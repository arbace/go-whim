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

// rsRuntime is whimsy's runtime, which the generated module uses.
const rsRuntime = "../../whimsy/src/rt.rs"

// rsHarness is the tests' host: out, outs and a printf of what the tests
// print, and the allocator of the tests that grow, each with the editor
// first.
const rsHarness = `use crate::editor::Editor;
use crate::rt::VArg;
use core::ffi::c_void;
use std::io::Write;

unsafe fn cstr(p: *const u8) -> Vec<u8> {
    let mut v = Vec::new();
    let mut i = 0;
    while *p.add(i) != 0 {
        v.push(*p.add(i));
        i += 1;
    }
    v
}

fn put(b: &[u8]) {
    let mut o = std::io::stdout().lock();
    o.write_all(b).unwrap();
    o.write_all(b"\n").unwrap();
}

pub unsafe fn out(_ed: *mut Editor, v: i64) {
    put(v.to_string().as_bytes());
}

pub unsafe fn outs(_ed: *mut Editor, s: *mut i8) {
    put(&cstr(s as *const u8));
}

pub unsafe fn alloc(_ed: *mut Editor, n: u64) -> *mut c_void {
    std::alloc::alloc_zeroed(std::alloc::Layout::from_size_align(n.max(1) as usize, 16).unwrap()) as *mut c_void
}

pub unsafe fn vim_free(_ed: *mut Editor, _p: *mut c_void) {}

pub unsafe fn memmove(_ed: *mut Editor, d: *mut c_void, s: *mut c_void, n: u64) -> *mut c_void {
    core::ptr::copy(s as *const u8, d as *mut u8, n as usize);
    d
}

pub unsafe fn memcpy(ed: *mut Editor, d: *mut c_void, s: *mut c_void, n: u64) -> *mut c_void {
    memmove(ed, d, s, n)
}

pub unsafe fn memset(_ed: *mut Editor, d: *mut c_void, c: i32, n: u64) -> *mut c_void {
    core::ptr::write_bytes(d as *mut u8, c as u8, n as usize);
    d
}

pub unsafe fn memcmp(_ed: *mut Editor, a: *mut c_void, b: *mut c_void, n: u64) -> i32 {
    for i in 0..n as usize {
        let (x, y) = (*(a as *const u8).add(i), *(b as *const u8).add(i));
        if x != y {
            return if x < y { -1 } else { 1 };
        }
    }
    0
}

/// a printf of d, i, u, x, c, s and l, reading the arguments as C does
pub unsafe fn outf(_ed: *mut Editor, fmt: *mut i8, args: &[VArg]) {
    let f = cstr(fmt as *const u8);
    let mut b: Vec<u8> = Vec::new();
    let mut k = 0;
    let mut i = 0;
    while i < f.len() {
        let c = f[i];
        i += 1;
        if c != b'%' {
            b.push(c);
            continue;
        }
        let mut l = 0;
        while f[i] == b'l' {
            l += 1;
            i += 1;
        }
        let conv = f[i];
        i += 1;
        if conv == b'%' {
            b.push(b'%');
            continue;
        }
        let a = args[k];
        k += 1;
        let v = a.bits();
        let s = match conv {
            b'd' | b'i' => if l > 0 { (v as i64).to_string() } else { (v as i32).to_string() },
            b'u' => if l > 0 { v.to_string() } else { (v as u32).to_string() },
            b'x' => if l > 0 { format!("{:x}", v) } else { format!("{:x}", v as u32) },
            b'c' => String::from_utf8_lossy(&[v as u8]).into_owned(),
            b's' => String::from_utf8_lossy(&cstr(a.ptr())).into_owned(),
            _ => panic!("outf: %{}", conv as char),
        };
        b.extend_from_slice(s.as_bytes());
    }
    assert_eq!(k, args.len(), "unused arguments");
    put(&b);
}
`

const rsLib = "pub mod rt;\npub mod host;\npub mod editor;\n"

const rsMain = "fn main() {\n    let ed = prog::editor::new_editor();\n    unsafe { prog::editor::run(ed) }\n}\n"

func requireRs(t *testing.T) {
	for _, tool := range []string{"gcc", "rustc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// rsProgram translates src and returns the module and the refusals.
func rsProgram(t *testing.T, dir, src string, prof Profile) (string, string) {
	if prof.RsExports == nil {
		prof.RsExports = []string{"run"} // what the harness's main calls, the editor first
	}
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "editor.rs")
	if rc := Run([]string{c, dir, "-rs", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.ReadFile(out + ".refused")
	return string(b), string(r)
}

// rsBuild compiles the module with the harness, warnings denied, into
// dir/prog.
func rsBuild(t *testing.T, dir, prog string) string {
	files := map[string]string{"editor.rs": prog, "host.rs": rsHarness, "lib.rs": rsLib, "main.rs": rsMain}
	rt, err := os.ReadFile(rsRuntime)
	if err != nil {
		t.Fatal(err)
	}
	files["rt.rs"] = string(rt)
	for n, s := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o, err := bounded(300*time.Second, "", "rustc", "--edition", "2021", "-D", "warnings", "--crate-type", "rlib", "--crate-name", "prog",
		"-C", "opt-level=0", "--out-dir", dir, filepath.Join(dir, "lib.rs"))
	if err != nil {
		t.Fatalf("rustc: %v\n%s\n%s", err, o, numbered(prog))
	}
	exe := filepath.Join(dir, "rsprog")
	o, err = bounded(300*time.Second, "", "rustc", "--edition", "2021", "-D", "warnings", "-C", "opt-level=0",
		"--extern", "prog="+filepath.Join(dir, "libprog.rlib"), "-o", exe, filepath.Join(dir, "main.rs"))
	if err != nil {
		t.Fatalf("rustc main: %v\n%s", err, o)
	}
	return exe
}

// rsOutput compiles the module with the harness and returns what it prints.
func rsOutput(t *testing.T, dir, prog string) string {
	exe := rsBuild(t, dir, prog)
	b, err := bounded(60*time.Second, "", exe)
	if err != nil {
		t.Fatalf("the Rust: %v\n%s\n%s", err, b, numbered(prog))
	}
	return string(b)
}

// rsSame translates src, requires every function written, and requires the
// Rust to print what the C prints; it returns the Rust.
func rsSame(t *testing.T, src string, prof Profile, harness string) string {
	t.Parallel()
	requireRs(t)
	dir := t.TempDir()
	prog, refused := rsProgram(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutputWith(t, dir, src, harness)
	if got := rsOutput(t, dir, prog); got != want {
		t.Errorf("the Rust prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(prog))
	}
	return prog
}

func TestRsIntegers(t *testing.T)  { rsSame(t, javaIntsC, Profile{}, javaHarnessC) }
func TestRsFlow(t *testing.T)      { rsSame(t, javaFlowC, Profile{}, javaHarnessC) }
func TestRsStrings(t *testing.T)   { rsSame(t, javaStringsC, Profile{}, javaHarnessC) }
func TestRsStructs(t *testing.T)   { rsSame(t, javaStructsC, Profile{}, javaHarnessC) }
func TestRsExtra(t *testing.T)     { rsSame(t, lowerExtraC, Profile{}, javaHarnessC) }
func TestRsVarargs(t *testing.T)   { rsSame(t, javaVarargsC, Profile{}, javaHarnessC) }
func TestRsPointers(t *testing.T)  { rsSame(t, javaPointersC, Profile{}, javaHarnessC) }
func TestRsGoto(t *testing.T)      { rsSame(t, javaGotoC, Profile{}, javaHarnessC) }
func TestRsShapes(t *testing.T)    { rsSame(t, cljShapesC, Profile{}, javaHarnessC) }
func TestRsNest(t *testing.T)      { rsSame(t, cljNestC, Profile{}, javaHarnessC) }
func TestRsMachine(t *testing.T)   { rsSame(t, cljMachineC, Profile{}, javaHarnessC) }
func TestRsDeadLabel(t *testing.T) { rsSame(t, javaDeadLabelC, Profile{}, javaHarnessC) }
func TestRsNames(t *testing.T)     { rsSame(t, javaNamesC, Profile{}, javaHarnessC) }
func TestRsProfile(t *testing.T)   { rsSame(t, javaProfileC, Profile{}, javaGrowHarnessC) }
func TestRsGrow(t *testing.T)      { rsSame(t, javaGrowC, Profile{}, javaGrowHarnessC) }
func TestRsSteps(t *testing.T)     { rsSame(t, javaStepsC, Profile{}, javaHarnessC) }
func TestRsMasks(t *testing.T)     { rsSame(t, javaMasksC, Profile{}, javaHarnessC) }
func TestRsLocals(t *testing.T)    { rsSame(t, javaLocalsC, Profile{}, javaHarnessC) }
func TestRsTables(t *testing.T)    { rsSame(t, cljTablesC, Profile{}, javaHarnessC) }
func TestRsCljNames(t *testing.T)  { rsSame(t, cljNamesC, Profile{}, javaHarnessC) }
func TestRsSplitRet(t *testing.T)  { rsSame(t, cljSplitReturnC, Profile{}, javaHarnessC) }

// The control: the comparison sees a translation that is wrong.  Each
// mutation undoes one of C's rules in the generated Rust -- unsigned
// division done signed, an unsigned char widened with its sign, an unsigned
// shift done arithmetically, a struct's assignment not made -- and each
// must move the output.
func TestRsControl(t *testing.T) {
	requireRs(t)
	for _, c := range []struct {
		name, src string
		from      *regexp.Regexp
		repl      string
	}{
		{"unsigned division", javaIntsC, regexp.MustCompile(`(fn udiv\(.*\n\s*(?:return )?)a / b(;?)\n`), "${1}((a as i32) / (b as i32)) as u32${2}\n"},
		{"unsigned widening", javaIntsC, regexp.MustCompile(`(fn widen_uchar\(.*\n\s*(?:return )?)c as i32 \+ 1(;?)\n`), "${1}c as i8 as i32 + 1${2}\n"},
		{"unsigned shift", javaIntsC, regexp.MustCompile(`(fn ushr\(.*\n\s*(?:return )?)a\.wrapping_shr\(n as u32\)(;?)\n`), "${1}(a as i32).wrapping_shr(n as u32) as u32${2}\n"},
		{"struct copy", javaStructsC, regexp.MustCompile(`\(\*ed\)\.g2 = \(\*ed\)\.g1;`), "let _ = (*ed).g1;"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			prog, _ := rsProgram(t, dir, c.src, Profile{})
			if !c.from.MatchString(prog) {
				t.Fatalf("the control's pattern %s is not in the Rust:\n%s", c.from, numbered(prog))
			}
			bad := c.from.ReplaceAllString(prog, c.repl)
			want := cOutputWith(t, dir, c.src, javaHarnessC)
			exe := rsBuild(t, dir, bad)
			got, _ := bounded(60*time.Second, "", exe) // a crash moves the output too
			if string(got) == want {
				t.Errorf("the mutation %q moved nothing", c.name)
			}
		})
	}
}

// numberedRs is unused when the tests pass; strings keeps the import.
var _ = strings.TrimSpace

// rsArithC is C's arithmetic where it is defined to wrap -- unsigned
// operands, a narrow type's increment or compound assignment, a compound
// assignment computed in a wider type -- beside the arithmetic the backend
// writes as Rust's plain operators: a byte less a character, a mask, a
// remainder, signed arithmetic in int and long.  The tests compile at
// opt-level 0, where Rust's plain operators panic on an overflow: a plain
// operator where C wraps is a panic here (doc/RUST-IDIOMS.md, items 3-4).
const rsArithC = javaHost + `
static unsigned int u = 0;
static unsigned char uc = 255;
static signed char sc = 127;
static short sh = 32767;
static unsigned long long ull = 0;

int digit(const char *s) { return *s - '0'; }
int low7(int c) { return (c & 0x7f) + 1; }
int rem10(unsigned int x) { return x % 10 + 1; }
int sdiv(int a, int b) { return a / b; }
int srem(int a, int b) { return a % b; }
long long ldiv_(long long a, long long b) { return a / b + a % b; }
unsigned int udiv(unsigned int a, unsigned int b) { return a / b + a % b; }
int neg(int x) { return -x; }
int negc(signed char c) { return -c; }
int sum(int n) { int s = 0; for (int i = 0; i < n; i++) s += i * i; return s; }
long long lsum(long long a, int b) { a -= b; a *= 3; return a; }

void run(void)
{
    u -= 1; out(u);
    u += 2; out(u);
    uc++; out(uc);
    uc--; out(uc);
    sc++; out(sc);
    sc += 1; out(sc);
    sh += 1; out(sh);
    ull -= 1; out((long long)(ull >> 1));
    int x = 2147483647;
    x += 1LL; out(x);
    unsigned char b = 200;
    b += 100; out(b);
    b *= 3; out(b);
    out(digit("7"));
    out(low7(255));
    out(rem10(4294967295u));
    out(sdiv(-7, 2)); out(srem(-7, 2));
    out(ldiv_(-9000000000LL, 7));
    out(udiv(4294967295u, 7));
    out(neg(5)); out(negc(-128));
    out(sum(100));
    out(lsum(10, 4));
}
`

func TestRsArith(t *testing.T) {
	prog := rsSame(t, rsArithC, Profile{}, javaHarnessC)
	// the effects (rs_fx.go): a function of its arguments is a safe fn of
	// no editor, one that dereferences an unsafe fn of no editor, one that
	// names the editor's objects an unsafe fn of the editor
	for _, sig := range []string{"pub fn neg(x: i32) -> i32 {", "pub unsafe fn digit(s: *const i8) -> i32 {", "pub unsafe fn run(ed: *mut Editor) {"} {
		if !strings.Contains(prog, sig) {
			t.Errorf("no %q in the Rust:\n%s", sig, numbered(prog))
		}
	}
}

// rsHoistC is C's side effects inside expressions, which the backend takes
// out where C's order lets it (doc/RUST-IDIOMS.md, item 6): a local's own
// increments, an assignment a condition evaluates first, a while whose
// condition does something -- beside what stays: a local named twice, an
// increment C may not evaluate, a global, a local whose address is taken.
const rsHoistC = javaHost + `
static int g;
static int seen(int v) { return v + g; }
static void bump(int *p) { (*p)++; }
struct pr { int r; int a; };
static struct pr two(int v) { struct pr p = { v, v * 2 }; return p; }

void copy(char *d, const char *s) { while ((*d++ = *s++) != 0) ; }
int count(const char *p) { int n = 0; int c; while ((c = *p++) != 0) { if (c == 'x') continue; n++; } return n; }
int down(int n) { int s = 0; while (n--) { if (n == 2) continue; s += n; } return s; }
int post(int n) { return n++; }

void run(void)
{
    char buf[8];
    copy(buf, "abc");
    outs(buf);
    out(count("axbxc"));
    out(down(5));
    out(post(7));
    int a[4] = {1, 2, 3, 4}, b[4], i = 0, j = 0;
    while (i < 4) b[i++] = a[j++] * 10;
    out(b[3]); out(i); out(j);
    int n = 2;
    if (n++ == 2) out(n);
    if (++n == 4) out(n);
    int k = n--;
    out(k); out(n);
    int y = 1;
    int z = y++ + y;
    out(z);
    int w = 0;
    if (w && i++) out(1);
    out(i);
    g = 5;
    out(seen(g++)); out(g);
    int q = 1;
    bump(&q);
    int r = q++ + 0;
    out(r); out(q);
    int c;
    if ((c = seen(1)) > 5) out(c);
    out(seen(i++)); out(i);
    int a2; struct pr o; int r2;
    r2 = (o = two(3), a2 = o.a, o.r);
    out(r2); out(a2);
    if ((o = two(4), a2 = o.a, o.r) == 4) out(a2);
    (void)(a2 += 1, o = two(a2));
    out(a2);
    out(o.r);
}
`

func TestRsHoist(t *testing.T) {
	prog := rsSame(t, rsHoistC, Profile{}, javaHarnessC)
	for _, s := range []string{"let k: i32 = n;\n    n -= 1;", "loop {", "let c: i32 = seen(ed, 1);"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
}

// rsRefsC is pointer parameters that are Rust references (rs_refs.go): a
// function that touches memory only through its parameters, `&mut` where
// it writes through one, `&` where it only reads -- beside what stays raw:
// a parameter compared, a function that names a global, two parameters
// where one is written.
const rsRefsC = javaHost + `
struct pt { int x; int y; struct { int a; } in; };
static int g;
static void clear(struct pt *p) { p->x = 0; p->y = 0; p->in.a = 0; }
static void init(struct pt *p, int v) { clear(p); p->x = v; p->y = v * 2; }
static int sum(const struct pt *a, const struct pt *b) { return a->x + b->y; }
static void bumpa(int *a) { *a += 1; }
static int same(struct pt *a, struct pt *b) { return a == b; }
static void touch(struct pt *p) { p->x = g; }
static void swapxy(struct pt *a, struct pt *b) { int t = a->x; a->x = b->y; b->y = t; }

void run(void)
{
    struct pt p, q;
    init(&p, 3);
    init(&q, 5);
    out(sum(&p, &q));
    bumpa(&p.in.a);
    out(p.in.a);
    out(same(&p, &p));
    g = 9;
    touch(&q);
    out(q.x);
    swapxy(&p, &q);
    out(p.x); out(q.y);
}
`

func TestRsRefs(t *testing.T) {
	prog := rsSame(t, rsRefsC, Profile{}, javaHarnessC)
	for _, s := range []string{"fn clear(p: &mut pt)", "fn init(p: &mut pt, v: i32)", "fn sum(a: &pt, b: &pt)", "fn bumpa(a: &mut i32)",
		"init(&mut p, 3)", "bumpa(&mut p.in_.a)", "p.x = v;"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
	for _, s := range []string{"fn same(a: *const pt", "fn touch(ed: *mut Editor, p: *mut pt)", "fn swapxy(a: *mut pt, b: *mut pt)"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
}

// rsDeferC is locals first given a value in both arms of an if, in every
// case of a switch, or before a loop -- declared with no value, `mut` only
// where a path stores twice (rs_defer.go) -- beside one a path reads
// before any store, which keeps its zero.
const rsDeferC = javaHost + `
int arms(int c) { int x; if (c > 0) x = 1; else x = -1; return x; }
int cases(int c) { int y; switch (c) { case 1: y = 10; break; case 2: case 3: y = 20; break; default: y = 0; } return y; }
int loop(int n) { int s, i; s = 0; for (i = 0; i < n; i++) s += i; return s; }
int maybe(int c) { int z; if (c) z = 5; while (c-- > 0) z = c; return c > 100 ? z : 0; }
int once(int c) { int w; do { if (c) { w = 1; break; } w = 2; } while (0); return w; }

void run(void)
{
    out(arms(3)); out(arms(-3));
    out(cases(1)); out(cases(3)); out(cases(9));
    out(loop(5));
    out(maybe(0));
    out(once(0)); out(once(1));
}
`

func TestRsDefer(t *testing.T) {
	prog := rsSame(t, rsDeferC, Profile{}, javaHarnessC)
	for _, s := range []string{"let x: i32;", "let y: i32;", "let w: i32;", "let mut z: i32 = 0;"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
}

// rsConstC is pointers never written through (doc/RUST-IDIOMS.md, item 10):
// a string walked and compared, a member and a result that only carry a
// pointer to be read, an object read through, a ?: of both kinds, a
// variadic argument (read by the printf) -- each *const -- beside what
// must stay *mut: a pointer written through, one whose value goes where a
// write is made (a member that is, a parameter that is, a host's), one
// whose own address is taken, one an array of a struct is written
// through, and the pointers of a function printed from the lowered form;
// an array read through a *const one is decay_const's.
const rsConstC = javaHost + `void outf(const char *fmt, ...);
struct item { const char *name; char *buf; char tag[4]; int n; };
static char store[16];
static char *cursor;
static const char *greeting = "hello";

static int length(const char *s) { const char *p = s; while (*p) p++; return p - s; }
static const char *find(const char *s, int c) { for (; *s; s++) if (*s == c) return s; return 0; }
static int before(char *mut_end, const char *p) { return p < mut_end; }
static void fill(char *d, const char *s) { if (d == s) return; while ((*d++ = *s++) != 0) ; }
static const char *pick(int c, const char *a, char *b) { return c ? a : b; }
static int tag0(struct item *it) { return it->tag[0]; }
static void settag(struct item *it, char c) { it->tag[1] = c; }
static void put(char **slot, char *v) { *slot = v; }
static int back(const char *s) { int n = 0; const char *p = s; again: if (*p) { p++; n++; goto again; } return n; }

void run(void)
{
    struct item it;
    char *w;
    it.name = "name";
    it.buf = store;
    it.tag[0] = 't';
    fill(it.buf, it.name);
    outs(it.buf);
    out(length(it.name));
    out(find(greeting, 'l') - greeting);
    out(find(greeting, 'z') == 0);
    out(before(store + 4, store));
    out(before(store, store + 4));
    out(length(pick(1, greeting, store)));
    out(length(pick(0, greeting, store)));
    settag(&it, 0);
    out(tag0(&it));
    put(&cursor, store + 1);
    *cursor = 'X';
    outs(store);
    w = store;
    put(&w, store + 2);
    outs(w);
    out(back("abc"));
    outf("%s %d", greeting, length(greeting));
}
`

func TestRsConst(t *testing.T) {
	prog := rsSame(t, rsConstC, Profile{}, javaHarnessC)
	for _, s := range []string{"fn length(s: *const i8)", "let mut p: *const i8 = s;", "fn find(mut s: *const i8, c: i32) -> *const i8",
		"pub name: *const i8,", "fn pick(c: i32, a: *const i8, b: *const i8) -> *const i8", "pub greeting: *const i8,",
		"fn before(mut_end: *const i8, p: *const i8)", "if s == d {", "fn tag0(it: *const item)", "decay_const(&raw const (*it).tag)", "VArg::P((*ed).greeting as *const c_void)"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
	// written through, passed where it is, its address taken, an array
	// reached through it, the lowered form's
	for _, s := range []string{"fn fill(mut d: *mut i8, mut s: *const i8)", "pub buf: *mut i8,", "fn settag(it: *mut item, c: i8)",
		"fn put(slot: &mut *mut i8, v: *mut i8)", "pub cursor: *mut i8,", "fn back(s: *mut i8)"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
}

// rsTempsC is what needs a temporary: an object's increment as a value, a
// compound assignment whose right side calls, an lvalue whose evaluation
// does something -- each temporary declared where it is given its value,
// `let t1: T = v;`, in the block that uses it, and none at the top.
const rsTempsC = javaHost + `
struct ctr { int n; int a[4]; };
static int serial;
static struct ctr c;
static int bump(void) { serial += 10; return serial; }
static int next(void) { return serial++; }
static struct ctr *pick(void) { c.n++; return &c; }

void run(void)
{
    int i = 0, x;
    x = next() + next();
    out(x); out(serial);
    c.a[1] += bump();
    out(c.a[1]);
    pick()->a[i++] += 3;
    out(c.a[0]); out(c.n); out(i);
    x = pick()->n++;
    out(x); out(c.n);
}
`

func TestRsTemps(t *testing.T) {
	prog := rsSame(t, rsTempsC, Profile{}, javaHarnessC)
	for _, s := range []string{"{ let t1: i32 = (*ed).serial; (*ed).serial = t1 + 1; t1 }", "let t1: i32 = bump(ed);", "let t2: *mut i32 = ", "let t3: *mut i32 = &raw mut (*pick(ed)).n;"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
	if regexp.MustCompile(`(?m)^    let mut t\d+: `).MatchString(prog) {
		t.Errorf("a temporary declared at a function's top:\n%s", numbered(prog))
	}
}

// rsLiveC is what rustc's own analyses see that the printer now sees too
// (doc/RUST-IDIOMS.md, item 13): a local first stored before a loop with
// no condition breaks, before a goto, in the left of an &&, in a do's
// body; a struct stored whole; a local whose address is taken after its
// store -- each declared with no value -- and C's dead stores: an
// initializer nothing reads, a parameter's value nothing reads, a store
// nothing reads, an increment a return makes -- not written -- and one to
// a local whose address is taken, which rustc does not follow: kept, the
// function's `#[expect(unused_assignments)]`.  The module allows no
// unused_assignments: rustc holds every claim.
const rsLiveC = javaHost + `
struct pt { int a; int b; };
static struct pt mk(int c) { struct pt p; p.a = c; p.b = c + 1; return p; }
static void bump(int *p) { *p += 1; }
static int last;
static void set(int *p, int v) { *p = v; last = v; }
static int forever(int n) { int x; for (;;) { if (n > 3) { x = n; break; } n++; } return x; }
static int jump(int c) { int y; if (c) { y = 1; goto out; } y = 2; out: return y; }
static int lazy(int c) { int z; if (c > 0 && (z = c * 2) > 2) return z; return 0; }
static int whole(int c) { struct pt p; if (c) p = mk(c); else p = mk(-c); return p.a + p.b; }
static int addr(int c) { int w; if (c) w = 1; else w = 2; bump(&w); return w; }
static int deadinit(int c) { int k = 0; k = c + 1; return k; }
static int param(int n, int m) { n = m * 2; return n; }
static int deadstore(int c) { int q = c; q = q + 1; out(q); q = 5; return c; }
static int retinc(int n) { return n++; }
static int addrdead(int c) { int v; set(&v, c); int r = v; v = 0; return r; }
static int dowhile(int n) { int s; do { s = n; n--; if (n < 0) break; } while (n > 5); return s; }

void run(void)
{
    out(forever(1)); out(jump(0)); out(jump(1)); out(lazy(3)); out(lazy(1));
    out(whole(4)); out(addr(6)); out(deadinit(7)); out(param(1, 4));
    out(deadstore(9)); out(retinc(11)); out(addrdead(12)); out(dowhile(9)); out(dowhile(2));
}
`

func TestRsLive(t *testing.T) {
	prog := rsSame(t, rsLiveC, Profile{}, javaHarnessC)
	for _, s := range []string{"let x: i32;", "let y: i32;", "let z: i32;", "let p: pt;", "let mut w: i32;", "let k: i32 = c + 1;",
		"fn param(_n: i32, m: i32)", "let n: i32;", "#[expect(unused_assignments)]\npub unsafe fn addrdead(ed: *mut Editor", "let mut s: i32;",
		"pub fn retinc(n: i32) -> i32 {\n    n\n}", "#![allow(non_snake_case, non_camel_case_types, non_upper_case_globals)]"} {
		if !strings.Contains(prog, s) {
			t.Errorf("no %q in the Rust:\n%s", s, numbered(prog))
		}
	}
	if strings.Contains(prog, "q = 5") {
		t.Errorf("a dead store written:\n%s", numbered(prog))
	}
}
