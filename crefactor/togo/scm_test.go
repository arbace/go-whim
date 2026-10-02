package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// scmRuntime is whimsical's runtime, which the generated library imports.
const scmRuntime = "../../whimsical/whimsical/rt.ss"

// scmHarness is the tests' host: out, outs and a printf of what the tests
// print, the allocator of the tests that grow and the byte functions, each
// with the editor first, in the vector the core's host-names orders; and
// the program, which runs the C's run on a new editor.
const scmHarness = `(import (chezscheme) (whimsical rt) (editor))

(define (put s) (put-string (current-output-port) s) (newline))
(define (cstr ed p) (mem-string (ed-mem ed) p))

(define (out ed v) (put (number->string v)))
(define (outs ed p) (put (cstr ed p)))
(define (alloc ed n) (let-values ([(p used) (arena-alloc ed (max n 1))]) p))
(define (vim_free ed p) (void))
(define (memmove ed d s n) (bytevector-copy! (ed-mem ed) s (ed-mem ed) d n) d)
(define (memcpy ed d s n) (memmove ed d s n))
(define (memset ed d c n)
  (let ([m (ed-mem ed)])
    (do ([i 0 (+ i 1)]) ((= i n) d) (bytevector-u8-set! m (+ d i) (bitwise-and c 255)))))
(define (memcmp ed a b n)
  (let ([m (ed-mem ed)])
    (let loop ([i 0])
      (if (= i n)
          0
          (let ([x (bytevector-u8-ref m (+ a i))] [y (bytevector-u8-ref m (+ b i))])
            (cond [(< x y) -1] [(> x y) 1] [else (loop (+ i 1))]))))))

;; a printf of d, i, u, x, c, s and l, reading the arguments as C does
(define (wrap v bits signed)
  (let* ([m (expt 2 bits)] [y (mod v m)])
    (if (and signed (>= y (/ m 2))) (- y m) y)))
(define (outf ed fmt args)
  (let ([f (cstr ed fmt)] [o (open-output-string)])
    (let loop ([i 0] [args args])
      (cond
        [(= i (string-length f))
         (unless (null? args) (error 'outf "unused arguments" f))]
        [(char=? (string-ref f i) #\%)
         (let lp ([j (+ i 1)] [l 0])
           (if (char=? (string-ref f j) #\l)
               (lp (+ j 1) (+ l 1))
               (let ([conv (string-ref f j)])
                 (if (char=? conv #\%)
                     (begin (put-char o #\%) (loop (+ j 1) args))
                     (let ([v (car args)] [bits (if (> l 0) 64 32)])
                       (put-string o
                         (case conv
                           [(#\d #\i) (number->string (wrap v bits #t))]
                           [(#\u) (number->string (wrap v bits #f))]
                           [(#\x) (string-downcase (number->string (wrap v bits #f) 16))]
                           [(#\c) (string (integer->char (wrap v 8 #f)))]
                           [(#\s) (cstr ed v)]
                           [else (error 'outf "a conversion" conv)]))
                       (loop (+ j 1) (cdr args)))))))]
        [else (put-char o (string-ref f i)) (loop (+ i 1) args)]))
    (put (get-output-string o))))

(define procs
  (list (cons 'out out) (cons 'outs outs) (cons 'outf outf) (cons 'alloc alloc) (cons 'vim_free vim_free)
        (cons 'memmove memmove) (cons 'memcpy memcpy) (cons 'memset memset) (cons 'memcmp memcmp)))

(define glue
  (vector-map (lambda (n) (cond [(assq n procs) => cdr] [else (lambda args (error n "no such host function"))]))
              host-names))

(run (new-editor glue))
`

func requireScm(t *testing.T) {
	for _, tool := range []string{"gcc", "chez"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// scmProfile is a test's profile: run exported, which the harness calls.
func scmProfile(p Profile) Profile {
	p.ScmExports = append(append([]string{}, p.ScmExports...), "run")
	return p
}

// scmProgram translates src and returns the library and the refusals.
func scmProgram(t *testing.T, dir, src string, prof Profile) (string, string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "editor.ss")
	if rc := Run([]string{c, dir, "-scm", out}, io.Discard, scmProfile(prof)); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.ReadFile(out + ".refused")
	return string(b), string(r)
}

// scmRun runs the library with the harness under Chez, at optimize-level
// 2 -- safe: a fixnum operation on what is not one, an index out of the
// memory, is an error -- and returns what it prints.
func scmRun(t *testing.T, dir, prog string) ([]byte, error) {
	if err := os.WriteFile(filepath.Join(dir, "editor.ss"), []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ss"), []byte(scmHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := os.ReadFile(scmRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "whimsical"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "whimsical", "rt.ss"), rt, 0o644); err != nil {
		t.Fatal(err)
	}
	return bounded(120*time.Second, "", "chez", "--optimize-level", "2", "--libdirs", dir, "--program", filepath.Join(dir, "main.ss"))
}

// scmOutput runs the library and returns what it prints.
func scmOutput(t *testing.T, dir, prog string) string {
	b, err := scmRun(t, dir, prog)
	if err != nil {
		t.Fatalf("the Scheme: %v\n%s\n%s", err, b, numbered(prog))
	}
	return string(b)
}

// scmSame translates src, requires every function written, and requires the
// Scheme to print what the C prints; it returns the Scheme.
func scmSame(t *testing.T, src string, prof Profile, harness string) string {
	t.Parallel()
	requireScm(t)
	dir := t.TempDir()
	prog, refused := scmProgram(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutputWith(t, dir, src, harness)
	if got := scmOutput(t, dir, prog); got != want {
		t.Errorf("the Scheme prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(prog))
	}
	return prog
}

func TestScmIntegers(t *testing.T)  { scmSame(t, javaIntsC, Profile{}, javaHarnessC) }
func TestScmFlow(t *testing.T)      { scmSame(t, javaFlowC, Profile{}, javaHarnessC) }
func TestScmStrings(t *testing.T)   { scmSame(t, javaStringsC, Profile{}, javaHarnessC) }
func TestScmStructs(t *testing.T)   { scmSame(t, javaStructsC, Profile{}, javaHarnessC) }
func TestScmExtra(t *testing.T)     { scmSame(t, lowerExtraC, Profile{}, javaHarnessC) }
func TestScmVarargs(t *testing.T)   { scmSame(t, javaVarargsC, Profile{}, javaHarnessC) }
func TestScmPointers(t *testing.T)  { scmSame(t, javaPointersC, Profile{}, javaHarnessC) }
func TestScmGoto(t *testing.T)      { scmSame(t, javaGotoC, Profile{}, javaHarnessC) }
func TestScmShapes(t *testing.T)    { scmSame(t, cljShapesC, Profile{}, javaHarnessC) }
func TestScmNest(t *testing.T)      { scmSame(t, cljNestC, Profile{}, javaHarnessC) }
func TestScmMachine(t *testing.T)   { scmSame(t, cljMachineC, Profile{}, javaHarnessC) }
func TestScmDeadLabel(t *testing.T) { scmSame(t, javaDeadLabelC, Profile{}, javaHarnessC) }
func TestScmNames(t *testing.T)     { scmSame(t, javaNamesC, Profile{}, javaHarnessC) }
func TestScmProfile(t *testing.T)   { scmSame(t, javaProfileC, Profile{}, javaGrowHarnessC) }
func TestScmGrow(t *testing.T)      { scmSame(t, javaGrowC, Profile{}, javaGrowHarnessC) }
func TestScmSteps(t *testing.T)     { scmSame(t, javaStepsC, Profile{}, javaHarnessC) }
func TestScmMasks(t *testing.T)     { scmSame(t, javaMasksC, Profile{}, javaHarnessC) }
func TestScmLocals(t *testing.T)    { scmSame(t, javaLocalsC, Profile{}, javaHarnessC) }
func TestScmTables(t *testing.T)    { scmSame(t, cljTablesC, Profile{}, javaHarnessC) }
func TestScmCljNames(t *testing.T)  { scmSame(t, cljNamesC, Profile{}, javaHarnessC) }
func TestScmSplitRet(t *testing.T)  { scmSame(t, cljSplitReturnC, Profile{}, javaHarnessC) }
func TestScmIdioms(t *testing.T)    { scmSame(t, hsIdiomsC, Profile{}, javaHarnessC) }
func TestScmTuples(t *testing.T)    { scmSame(t, hsTupleC, Profile{}, javaHarnessC) }

// The same programs with every out-parameter and struct local in the frame,
// as C has them, and the pure functions without the editor: the switches
// change the printing, not what it prints.
func TestScmFrames(t *testing.T) {
	for _, c := range []struct{ name, src string }{{"idioms", hsIdiomsC}, {"tuples", hsTupleC}, {"structs", javaStructsC}, {"ints", javaIntsC}} {
		t.Run(c.name, func(t *testing.T) {
			scmSame(t, c.src, Profile{ScmNoOuts: true, ScmNoStructValues: true}, javaHarnessC)
		})
		t.Run(c.name+"-pure", func(t *testing.T) {
			scmSame(t, c.src, Profile{ScmPure: true}, javaHarnessC)
		})
	}
}

// The control: the comparison sees a translation that is wrong.  Each
// mutation undoes one of C's rules in the generated Scheme -- unsigned
// division done signed, an unsigned char widened with its sign, an unsigned
// shift done arithmetically, a struct's assignment not made -- and each
// must move the output.
func TestScmControl(t *testing.T) {
	requireScm(t)
	for _, c := range []struct {
		name, src string
		from      *regexp.Regexp
		repl      string
	}{
		{"unsigned division", javaIntsC, regexp.MustCompile(`\(u32/ (\w+) (\w+)\)`), "(->u32 (i32/ (->i32 $1) (->i32 $2)))"},
		{"unsigned widening", javaIntsC, regexp.MustCompile(`(\(define \(widen_uchar ed c\)\n\s*)\(fx\+ c 1\)`), "${1}(fx+ (->i8 c) 1)"},
		{"unsigned shift", javaIntsC, regexp.MustCompile(`\(u32>> (\w+) (\w+)\)`), "(->u32 (i32>> (->i32 $1) $2))"},
		{"struct copy", javaStructsC, regexp.MustCompile(`\(mem-copy! (\S+) (\S+) 56\)`), "(void)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			prog, _ := scmProgram(t, dir, c.src, Profile{})
			if !c.from.MatchString(prog) {
				t.Fatalf("the control's pattern %s is not in the Scheme:\n%s", c.from, numbered(prog))
			}
			bad := c.from.ReplaceAllString(prog, c.repl)
			want := cOutputWith(t, dir, c.src, javaHarnessC)
			got, _ := scmRun(t, dir, bad) // a failure moves the output too
			if string(got) == want {
				t.Errorf("the mutation %q moved nothing", c.name)
			}
		})
	}
}

var _ = strings.TrimSpace

// scmSignedC is signed arithmetic near its type's ends that does not
// overflow, which is all C defines: an int's on fixnums, a long's past
// Chez's 61-bit fixnums into bignums and back.
const scmSignedC = javaHost + `
static long big(long a, long b) { return a + b - b * 2 + b; }
static int edge(int a, int b) { return a - b + b * 1 - -b + -b; }
static long neg(long a) { return -a; }
void run(void) {
    out(big(0x7000000000000000L, 0x0fffffffffffffffL));
    out(big(-0x7fffffffffffffffL, -1));
    out(edge(2147483647, 1)); out(edge(-2147483647, -1));
    out(neg(-0x7fffffffffffffffL)); out(neg(0x1000000000000000L));
    out(-2147483647 * 1 - 1);
}
`

func TestScmSigned(t *testing.T) {
	prog := scmSame(t, scmSignedC, Profile{}, javaHarnessC)
	if regexp.MustCompile(`\((i32|i64)[-+*] `).MatchString(prog) {
		t.Errorf("signed arithmetic through a wrapping helper:\n%s", numbered(prog))
	}
}

// scmMemC moves, copies and fills bytes, overlapping both ways, through
// functions of its own that whim.Gen's Scheme writes as the runtime's
// (internal/whim/gen.go, doc/SCHEME-IDIOMS.md item 16): the bodies below
// are those, required to answer as the C's loops do.
const scmMemC = javaHost + `
void *musl_memmove(void *dest, const void *src, unsigned long n) {
    char *d = dest; const char *s = src;
    if (d == s) return d;
    if (d < s) { for (; n; n--) *d++ = *s++; } else { while (n) { n--; d[n] = s[n]; } }
    return dest;
}
void *musl_memcpy(void *dest, const void *src, unsigned long n) { char *d = dest; const char *s = src; for (; n; n--) *d++ = *s++; return dest; }
void *musl_memset(void *dest, int c, unsigned long n) { unsigned char *s = dest; for (; n; n--) *s++ = c; return dest; }
void run(void) {
    char b[32];
    musl_memset(b, 0, sizeof b);
    musl_memcpy(b, "abcdefghij", 10); outs(b);
    musl_memmove(b + 2, b, 6); outs(b);
    musl_memmove(b, b + 3, 5); outs(b);
    outs((char *)musl_memset(b + 4, 'z' + 256, 3) - 4);
    musl_memset(b + 1, 0, 2); outs(b); outs(b + 3);
    musl_memmove(b, b, 4); musl_memcpy(b, b + 3, 0); outs(b + 3);
}
`

func TestScmMemBodies(t *testing.T) {
	body := func(text string) func(string) string { return func(string) string { return text } }
	prof := Profile{RuntimeBodies: []RuntimeBody{
		{Name: "musl_memmove", Scm: body("(mem-copy! dest src n)\ndest")},
		{Name: "musl_memcpy", Scm: body("(mem-copy! dest src n)\ndest")},
		{Name: "musl_memset", Scm: body("(mem-fill! dest (->u8 c) n)\ndest")},
	}}
	prog := scmSame(t, scmMemC, prof, javaHarnessC)
	if !strings.Contains(prog, "(mem-copy! dest src n)") {
		t.Errorf("the runtime's bodies not written:\n%s", numbered(prog))
	}
}
