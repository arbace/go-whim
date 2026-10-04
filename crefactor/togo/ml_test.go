package togo

import (
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// mlRuntime is whiml's runtime, which the generated module opens.
const mlRuntime = "../../whiml/rt.ml"

// mlHostImpls are the tests' host functions, by name, each with the editor
// first: out, outs and a printf of what the tests print, the allocator of
// the tests that grow, and the byte functions.  The harness fills the
// module's glue record with those the program declares.
var mlHostImpls = map[string]string{
	"out":      `fun _ v -> Printf.printf "%d\n" v`,
	"outs":     `fun ed p -> print_endline (Rt.mem_string ed p)`,
	"outf":     `outf`,
	"alloc":    `fun ed n -> match Rt.arena_alloc ed (max n 1) with Some p, _ -> p | None, _ -> failwith "alloc"`,
	"vim_free": `fun _ _ -> ()`,
	"memmove":  `fun ed d s n -> Rt.mem_copy ed d s n; d`,
	"memcpy":   `fun ed d s n -> Rt.mem_copy ed d s n; d`,
	"memset":   `fun ed d c n -> Rt.mem_fill ed d c n; d`,
	"memcmp": `fun ed a b n ->
      let rec go i = if i = n then 0 else
        let x = Rt.ld_u8 ed (a + i) and y = Rt.ld_u8 ed (b + i) in
        if x < y then -1 else if x > y then 1 else go (i + 1) in go 0`,
}

// mlHarness is the program: a printf of d, i, u, x, c, s and l, reading the
// arguments as C does, and the run of the C's run on a new editor.
const mlHarness = `let wrap v bits signed =
  if bits = 64 then (if signed then Int64.to_string (Int64.of_int v) else Printf.sprintf "%Lu" (Int64.of_int v))
  else
    let m = 1 lsl bits in
    let y = v land (m - 1) in
    string_of_int (if signed && y >= m / 2 then y - m else y)

let outf ed fmt args =
  let f = Rt.mem_string ed fmt and b = Buffer.create 64 in
  let rec go i args =
    if i = String.length f then (if args <> [] then failwith "outf: unused arguments")
    else if f.[i] = '%' then begin
      let rec lp j l = if f.[j] = 'l' then lp (j + 1) (l + 1) else (j, l) in
      let j, l = lp (i + 1) 0 in
      match f.[j], args with
      | '%', _ -> Buffer.add_char b '%'; go (j + 1) args
      | c, v :: rest ->
          let bits = if l > 0 then 64 else 32 in
          Buffer.add_string b
            (match c with
             | 'd' | 'i' -> wrap v bits true
             | 'u' -> wrap v bits false
             | 'x' -> Printf.sprintf "%Lx" (Int64.of_int (if bits = 32 then v land 0xffffffff else v))
             | 'c' -> String.make 1 (Char.chr (v land 255))
             | 's' -> Rt.mem_string ed v
             | _ -> failwith "outf: a conversion");
          go (j + 1) rest
      | _, [] -> failwith "outf: too few arguments"
    end
    else (Buffer.add_char b f.[i]; go (i + 1) args)
  in
  go 0 args;
  print_endline (Buffer.contents b)
`

func requireMl(t *testing.T) {
	for _, tool := range []string{"gcc", "ocamlopt"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// mlProgram translates src and returns the module, its host's fields and
// the refusals.
func mlProgram(t *testing.T, dir, src string, prof Profile) (string, []string, string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "editor.ml")
	prof.ScmExports = append(append([]string{}, prof.ScmExports...), "run")
	if rc := Run([]string{c, dir, "-ml", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	h, _ := os.ReadFile(out + ".host")
	r, _ := os.ReadFile(out + ".refused")
	return string(b), strings.Fields(string(h)), string(r)
}

// mlRun compiles the module with the runtime and the harness and runs it,
// and returns what it prints.
func mlRun(t *testing.T, dir, prog string, host []string) ([]byte, error) {
	rt, err := os.ReadFile(mlRuntime)
	if err != nil {
		t.Fatal(err)
	}
	var main strings.Builder
	main.WriteString(mlHarness)
	main.WriteString("\nlet glue : Editor.glue = {\n")
	for _, h := range host {
		impl, ok := mlHostImpls[h]
		if !ok {
			impl = fmt.Sprintf("fun _ -> failwith %q", "no host function "+h)
		}
		fmt.Fprintf(&main, "  %s = (%s);\n", h, impl)
	}
	main.WriteString("}\n\nlet () = Editor.run (Editor.new_editor glue)\n")
	for name, b := range map[string]string{"rt.ml": string(rt), "editor.ml": prog, "main.ml": main.String()} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(dir, "ml")
	// the module with every warning on, and a warning a failure, as whiml's
	// build has it
	cmd := exec.Command("ocamlopt", "-I", "+unix", "rt.ml", "-w", "+a", "-warn-error", "+a", "editor.mli", "editor.ml", "-w", "-a", "main.ml", "-o", exe)
	cmd.Dir = dir
	if o, err := cmd.CombinedOutput(); err != nil {
		return o, fmt.Errorf("ocamlopt: %v", err)
	}
	return bounded(60*time.Second, "", exe)
}

// mlSame translates src, requires every function written, and requires the
// OCaml to print what the C prints; it returns the OCaml.
func mlSame(t *testing.T, src string, prof Profile, harness string) string {
	t.Parallel()
	requireMl(t)
	dir := t.TempDir()
	prog, host, refused := mlProgram(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutputWith(t, dir, src, harness)
	got, err := mlRun(t, dir, prog, host)
	if err != nil {
		t.Fatalf("the OCaml: %v\n%s\n%s", err, got, numbered(prog))
	}
	if !ml63Same(t, string(got), want) {
		t.Errorf("the OCaml prints\n%s\nthe C\n%s\n%s", diffLines(string(got), want), want, numbered(prog))
	}
	return prog
}

// ml63Same says the OCaml printed what the C printed, but where the C
// printed an integer past 2^62 in magnitude, which OCaml's 63-bit int
// cannot hold: there its low 63 bits (doc/OCAML.md, *Integers*).
func ml63Same(t *testing.T, got, want string) bool {
	if got == want {
		return true
	}
	gs, ws := strings.Split(got, "\n"), strings.Split(want, "\n")
	if len(gs) != len(ws) {
		return false
	}
	for i := range gs {
		if gs[i] == ws[i] {
			continue
		}
		w, ok := new(big.Int).SetString(ws[i], 10)
		g, ok2 := new(big.Int).SetString(gs[i], 10)
		lim := new(big.Int).Lsh(big.NewInt(1), 62)
		if !ok || !ok2 || new(big.Int).Abs(w).Cmp(lim) < 0 {
			return false
		}
		// w's low 63 bits, sign-extended
		m := new(big.Int).Lsh(big.NewInt(1), 63)
		low := new(big.Int).Mod(w, m)
		if low.Cmp(lim) >= 0 {
			low.Sub(low, m)
		}
		if low.Cmp(g) != 0 {
			return false
		}
		t.Logf("line %d: %s, past 63 bits: %s, its low 63", i+1, ws[i], gs[i])
	}
	return true
}

func TestMlIntegers(t *testing.T)   { mlSame(t, javaIntsC, Profile{}, javaHarnessC) }
func TestMlFlow(t *testing.T)       { mlSame(t, javaFlowC, Profile{}, javaHarnessC) }
func TestMlStrings(t *testing.T)    { mlSame(t, javaStringsC, Profile{}, javaHarnessC) }
func TestMlStructs(t *testing.T)    { mlSame(t, javaStructsC, Profile{}, javaHarnessC) }
func TestMlExtra(t *testing.T)      { mlSame(t, lowerExtraC, Profile{}, javaHarnessC) }
func TestMlVarargs(t *testing.T)    { mlSame(t, javaVarargsC, Profile{}, javaHarnessC) }
func TestMlPointers(t *testing.T)   { mlSame(t, javaPointersC, Profile{}, javaHarnessC) }
func TestMlGoto(t *testing.T)       { mlSame(t, javaGotoC, Profile{}, javaHarnessC) }
func TestMlShapes(t *testing.T)     { mlSame(t, cljShapesC, Profile{}, javaHarnessC) }
func TestMlNest(t *testing.T)       { mlSame(t, cljNestC, Profile{}, javaHarnessC) }
func TestMlMachine(t *testing.T)    { mlSame(t, cljMachineC, Profile{}, javaHarnessC) }
func TestMlDeadLabel(t *testing.T)  { mlSame(t, javaDeadLabelC, Profile{}, javaHarnessC) }
func TestMlNames(t *testing.T)      { mlSame(t, javaNamesC, Profile{}, javaHarnessC) }
func TestMlProfile(t *testing.T)    { mlSame(t, javaProfileC, Profile{}, javaGrowHarnessC) }
func TestMlGrow(t *testing.T)       { mlSame(t, javaGrowC, Profile{}, javaGrowHarnessC) }
func TestMlSteps(t *testing.T)      { mlSame(t, javaStepsC, Profile{}, javaHarnessC) }
func TestMlMasks(t *testing.T)      { mlSame(t, javaMasksC, Profile{}, javaHarnessC) }
func TestMlLocals(t *testing.T)     { mlSame(t, javaLocalsC, Profile{}, javaHarnessC) }
func TestMlTables(t *testing.T)     { mlSame(t, cljTablesC, Profile{}, javaHarnessC) }
func TestMlCljNames(t *testing.T)   { mlSame(t, cljNamesC, Profile{}, javaHarnessC) }
func TestMlSplitRet(t *testing.T)   { mlSame(t, cljSplitReturnC, Profile{}, javaHarnessC) }
func TestMlIdioms(t *testing.T)     { mlSame(t, hsIdiomsC, Profile{}, javaHarnessC) }
func TestMlTuples(t *testing.T)     { mlSame(t, hsTupleC, Profile{}, javaHarnessC) }
func TestMlSigned(t *testing.T)     { mlSame(t, mlSignedC, Profile{}, javaHarnessC) }
func TestMlCaseLabels(t *testing.T) { mlSame(t, scmCaseC, Profile{}, javaHarnessC) }

// mlSignedC is scmSignedC within OCaml's 63 bits: a long's arithmetic near
// 2^62, which is all the OCaml's int holds exactly.
const mlSignedC = javaHost + `
static long big(long a, long b) { return a + b - b * 2 + b; }
static int edge(int a, int b) { return a - b + b * 1 - -b + -b; }
static long neg(long a) { return -a; }
static unsigned long umax(void) { return (unsigned long)-1; }
static int ult(unsigned long a, unsigned long b) { return a < b; }
static unsigned long udiv(unsigned long a, unsigned long b) { return a / b; }
static unsigned long ushr(unsigned long a) { return a >> 1; }
void run(void) {
    out(big(0x3000000000000000L, 0x0fffffffffffffffL));
    out(big(-0x3fffffffffffffffL, -1));
    out(edge(2147483647, 1)); out(edge(-2147483647, -1));
    out(neg(-0x3fffffffffffffffL)); out(neg(0x1000000000000000L));
    out(-2147483647 * 1 - 1);
    out(ult(1, umax())); out(ult(umax(), 1)); out(udiv(umax() - 6, 0x4000000000000000UL));
    out(udiv(0x3ffffffffffffff0UL, 3)); out(ushr(umax())); out(ushr(0x3ffffffffffffffeUL)); out(umax() + 1);
}
`

// The control: the comparison sees a translation that is wrong.  Each
// mutation undoes one of C's rules in the generated OCaml -- an unsigned
// long's order taken signed, an unsigned char widened with its sign, an
// unsigned shift done arithmetically, a struct's assignment not made --
// and each must move the output.
func TestMlControl(t *testing.T) {
	requireMl(t)
	for _, c := range []struct {
		name, src string
		from      *regexp.Regexp
		repl      string
	}{
		{"unsigned order", mlSignedC, regexp.MustCompile(`u64_lt (\w+) (\w+)`), "$1 < $2"},
		{"unsigned widening", javaIntsC, regexp.MustCompile(`(let widen_uchar _?ed c =\n\s*)c \+ 1`), "${1}to_i8 c + 1"},
		{"unsigned shift", javaIntsC, regexp.MustCompile(`(\w+) lsr (\w+)`), "to_u32 (to_i32 $1 asr $2)"},
		{"struct copy", javaStructsC, regexp.MustCompile(`mem_copy ed (\S+) (\S+) 56`), "()"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			prog, host, _ := mlProgram(t, dir, c.src, Profile{})
			if !c.from.MatchString(prog) {
				t.Fatalf("the control's pattern %s is not in the OCaml:\n%s", c.from, numbered(prog))
			}
			bad := c.from.ReplaceAllString(prog, c.repl)
			want := cOutputWith(t, dir, c.src, javaHarnessC)
			got, _ := mlRun(t, dir, bad, host) // a failure moves the output too
			if string(got) == want {
				t.Errorf("the mutation %q moved nothing", c.name)
			}
		})
	}
}

// mlVariantsC is an enumeration the profile names a variant: a parameter,
// a result, a switch, comparisons, a member in memory (zeroed, read and
// written through the conversions) and a field of the state with an
// initial value that is not the first enumerator's.
const mlVariantsC = javaHost + `
typedef enum { RED, GREEN = 4, BLUE } color_T;
struct cell { int n; color_T c; };
static color_T last = GREEN;
static struct cell cells[3];
static color_T next(color_T c) { switch (c) { case RED: return GREEN; case GREEN: return BLUE; default: return RED; } }
static int code(color_T c) { if (c == BLUE) return 30; if (c != RED) return 20; return 10; }
void run(void) {
    color_T c = RED;
    out(code(cells[1].c));
    for (int i = 0; i < 3; i++) { cells[i].c = c; cells[i].n = i; c = next(c); }
    for (int i = 0; i < 3; i++) out(code(cells[i].c) + cells[i].n);
    out(code(last)); last = next(last); out(code(last));
}
`

func TestMlVariants(t *testing.T) {
	prog := mlSame(t, mlVariantsC, Profile{MlVariants: []string{"color_T"}}, javaHarnessC)
	for _, want := range []string{"type color = Red | Green | Blue", "| Red -> Green", "| Blue -> 30", "color_of_int", "int_of_color"} {
		if !strings.Contains(prog, want) {
			t.Errorf("no %q in the OCaml:\n%s", want, numbered(prog))
		}
	}
}

// An enumeration the C orders is refused: OCaml would order its variant by
// the constructors, polymorphically.
func TestMlVariantOrdered(t *testing.T) {
	requireMl(t)
	src := javaHost + `
typedef enum { LOW, HIGH } level_T;
static int high(level_T l) { return l > LOW; }
void run(void) { out(high(HIGH)); }
`
	dir := t.TempDir()
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	prof := Profile{MlVariants: []string{"level_T"}, ScmExports: []string{"run"}}
	if rc := Run([]string{c, dir, "-ml", filepath.Join(dir, "editor.ml")}, &log, prof); rc == 0 || !strings.Contains(log.String(), "level_T ordered") {
		t.Errorf("an ordered variant was not refused: %d\n%s", rc, log.String())
	}
}

// mlStrEqC is the profile's string comparisons tested for 0 against
// literals, each case written on an OCaml string (strEq): equal, not, a
// count shorter than the literal, as long, longer, folded.
const mlStrEqC = javaHost + `
static int scmp(const char *l, const char *r) { for (; *l && *l == *r; l++, r++) {} return *(unsigned char *)l - *(unsigned char *)r; }
static int sncmp(const char *l, const char *r, unsigned long n) { if (!n--) return 0; for (; *l && *r && n && *l == *r; l++, r++, n--) {} return *(unsigned char *)l - *(unsigned char *)r; }
static int low(int c) { return c >= 'A' && c <= 'Z' ? c | 32 : c; }
static int scasecmp(const char *l, const char *r) { for (; *l && *r && low(*l) == low(*r); l++, r++) {} return low(*l) - low(*r); }
static int sncasecmp(const char *l, const char *r, unsigned long n) { if (!n--) return 0; for (; *l && *r && n && low(*l) == low(*r); l++, r++, n--) {} return low(*l) - low(*r); }
static void t(const char *p) {
    out(scmp(p, "abc") == 0); out(scmp("abc", p) != 0); out(sncmp(p, "abc", 3) == 0);
    out(sncmp(p, "abcdef", 2) == 0); out(sncmp(p, "ab", 5) == 0);
    out(scasecmp(p, "ABC") == 0); out(sncasecmp(p, "aBcD", 3) == 0); out(scmp(p, "") == 0);
}
void run(void) { t("abc"); t("abcd"); t("ab"); t("Abc"); t(""); t("x"); }
`

func TestMlStrEq(t *testing.T) {
	prog := mlSame(t, mlStrEqC, Profile{MlStrings: StrFuncs{Cmp: "scmp", NCmp: "sncmp", CaseCmp: "scasecmp", NCaseCmp: "sncasecmp"}}, javaHarnessC)
	for _, want := range []string{`c_str_is ed p "abc"`, `has_prefix ed p "abc"`, `has_prefix ed p "ab"`, `c_str_is_ci ed p "ABC"`, `has_prefix_ci ed p "aBc"`, `c_str_is ed p ""`} {
		if !strings.Contains(prog, want) {
			t.Errorf("no %q in the OCaml:\n%s", want, numbered(prog))
		}
	}
	if strings.Contains(prog, "scmp ed p") || strings.Contains(prog, "sncmp ed p") {
		t.Errorf("a comparison against a literal left a call:\n%s", numbered(prog))
	}
}

// mlRecordsC is a struct the profile names a record: locals made, written,
// copied, initialized (all, and with {0} in a loop, which clears it each
// time), passed by pointer to a function that reads and writes it, and
// through a function pointer, whose table is the record's own.
const mlRecordsC = javaHost + `
typedef struct { int lnum; int fill; long height; _Bool top; } lineoff_T;
static void step(lineoff_T *lp) { lp->lnum++; lp->height += lp->fill; lp->top = !lp->top; }
static int sum(lineoff_T *lp) { return lp->lnum + lp->fill + (int)lp->height + lp->top * 100; }
static void (*steps[2])(lineoff_T *) = {step, step};
void run(void) {
    lineoff_T a, b;
    a.lnum = 1; a.fill = 2; a.height = 3; a.top = 0;
    b = a; step(&b); out(sum(&a)); out(sum(&b)); steps[1](&b); out(sum(&b));
    lineoff_T c = {5, 6, 7, 1}; step(&c); out(sum(&c));
    for (int i = 0; i < 3; i++) { lineoff_T d = {0}; d.lnum += i; step(&d); out(sum(&d)); }
}
`

func TestMlRecords(t *testing.T) {
	prog := mlSame(t, mlRecordsC, Profile{MlRecords: []string{"lineoff_T"}}, javaHarnessC)
	for _, want := range []string{"type lineoff = {", "lp.lnum <- lp.lnum + 1", "Lineoff_T.make ()", "Lineoff_T.copy_into", "Lineoff_T.clear", "call_ptr1v_0lineoff"} {
		if !strings.Contains(prog, want) {
			t.Errorf("no %q in the OCaml:\n%s", want, numbered(prog))
		}
	}
	if strings.Contains(prog, "frame_push") {
		t.Errorf("a record in the frame:\n%s", numbered(prog))
	}
}

// A struct whose member's address is taken, or that holds an array or a
// struct, keeps those members in memory of its own, in the frame: the rest
// are its fields.
const mlRecordMemC = javaHost + `
typedef struct { int x, y; } pt_T;
typedef struct { int a, b; pt_T at; char name[4]; } pair_T;
static void inc(int *p) { (*p)++; }
static int sum(pair_T *p) { return p->a + p->b + p->at.x * 10 + p->at.y * 100 + p->name[1]; }
void run(void) {
    pair_T p = {1, 2}; inc(&p.a); p.at.y = 3; p.name[1] = 7; out(sum(&p));
    pair_T q; q = p; q.b += 5; q.at.x = 4; out(sum(&q)); out(sum(&p));
    for (int i = 0; i < 2; i++) { pair_T r = {0}; r.b = i; out(sum(&r)); }
}
`

func TestMlRecordMem(t *testing.T) {
	prog := mlSame(t, mlRecordMemC, Profile{MlRecords: []string{"pair_T"}}, javaHarnessC)
	for _, want := range []string{"type pair = { mutable b : int; pair_mem : int }", "Pair_T.make (fr + ", "Pair_T.copy_into ed", "q.b <- q.b + 5"} {
		if !strings.Contains(prog, want) {
			t.Errorf("no %q in the OCaml:\n%s", want, numbered(prog))
		}
	}
}

// A struct the profile names whose pointers are compared is refused: a
// record's equality is its fields'.
func TestMlRecordCompared(t *testing.T) {
	requireMl(t)
	src := javaHost + `
typedef struct { int a, b; } pair_T;
static int same(pair_T *p, pair_T *q) { return p == q; }
void run(void) { pair_T p = {1, 2}; out(same(&p, &p)); }
`
	dir := t.TempDir()
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	prof := Profile{MlRecords: []string{"pair_T"}, ScmExports: []string{"run"}}
	if rc := Run([]string{c, dir, "-ml", filepath.Join(dir, "editor.ml")}, &log, prof); rc == 0 || !strings.Contains(log.String(), "pointers compared") {
		t.Errorf("a record compared was not refused: %d\n%s", rc, log.String())
	}
}
