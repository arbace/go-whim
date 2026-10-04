package togo

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The C++ backend's tests: the foreign C programs the other backends'
// tests translate, written as C++ by -cpp, compiled by g++ with every
// and required to print what gcc's
// build of the C prints (their C is warned of as C, on purpose: the
// core's build is what holds the warnings to none).  The C side of each test's host -- out, outs, a
// printf, an allocator -- is the C harness itself, compiled by gcc; the
// editor's host functions are a line of glue to it each.

// cppFlags are g++'s for a test's program: whim++'s (wpp.Flags), and a
// warning an error.
var cppFlags = []string{"-std=c++23", "-O2", "-w"}

func requireCpp(t *testing.T) {
	for _, tool := range []string{"gcc", "g++"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// cppProgram translates src and returns the source and the header, the
// host's signatures and the refusals.
func cppProgram(t *testing.T, dir, src string, prof Profile) (cpp, hpp, host, refused string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "editor.cpp")
	prof.CppExports = append(append([]string{}, prof.CppExports...), "run")
	if rc := Run([]string{c, dir, "-cpp", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return read(out), read(filepath.Join(dir, "editor.hpp")), read(out + ".host"), read(out + ".refused")
}

// cppGlue is the editor's host functions, each a call of the C harness's
// function of the name, and main: an editor, run.
func cppGlue(host string) string {
	var ext, defs strings.Builder
	ext.WriteString("#include \"editor.hpp\"\n#include <cstdarg>\n#include <cstdio>\n#include <memory>\n\nextern \"C\" {\n")
	for _, l := range strings.Split(strings.TrimSpace(host), "\n") {
		if l == "" {
			continue
		}
		f := strings.Split(l, "|")
		ret, name, ps := f[0], f[1], f[2:]
		if len(ps) == 1 && ps[0] == "" {
			ps = nil
		}
		variadic := len(ps) > 0 && ps[len(ps)-1] == "..."
		var params, args []string
		for i, p := range ps {
			if p == "..." {
				params = append(params, "...")
				continue
			}
			params = append(params, cppParamDecl(p, fmt.Sprintf("p%d", i)))
			args = append(args, fmt.Sprintf("p%d", i))
		}
		fmt.Fprintf(&ext, "%s %s(%s);\n", ret, name, strings.Join(params, ", "))
		if variadic {
			// the harness's printf: vprintf and a newline, as its outf
			fmt.Fprintf(&defs, "%s whimpp::Editor::%s(%s)\n{\n    va_list ap;\n    va_start(ap, p%d);\n    std::vprintf(p%d, ap);\n    va_end(ap);\n    std::printf(\"\\n\");\n}\n", ret, name, strings.Join(params, ", "), len(args)-1, len(args)-1)
			continue
		}
		r := "return "
		if ret == "void" {
			r = ""
		}
		fmt.Fprintf(&defs, "%s whimpp::Editor::%s(%s)\n{\n    %s::%s(%s);\n}\n", ret, name, strings.Join(params, ", "), r, name, strings.Join(args, ", "))
	}
	// the harness's main, renamed and never called, calls run
	ext.WriteString("void run(void) {}\n}\n\n")
	return ext.String() + defs.String() + "\nint main()\n{\n    auto ed = std::make_unique<whimpp::Editor>();\n    ed->run();\n    return 0;\n}\n"
}

// cppParamDecl declares a parameter of type t named n.
func cppParamDecl(t, n string) string {
	if i := strings.Index(t, "(Editor::*"); i >= 0 {
		return t[:i+len("(Editor::*")] + n + t[i+len("(Editor::*"):]
	}
	return t + " " + n
}

// cppRun compiles the program with the glue, and the C harness without its
// main, and runs it, and returns what it prints.
func cppRun(t *testing.T, dir, cpp, hpp, host, harness string) ([]byte, error) {
	files := map[string]string{"editor.cpp": cpp, "editor.hpp": hpp, "glue.cpp": cppGlue(host), "harness.c": harness,
		"rt.hpp": cppRtHpp}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) ([]byte, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		return cmd.CombinedOutput()
	}
	if o, err := run("gcc", "-w", "-O0", "-Dmain=harness_main", "-c", "harness.c", "-o", "harness.o"); err != nil {
		return o, fmt.Errorf("gcc: %v", err)
	}
	if o, err := run("g++", append(append([]string{}, cppFlags...), "-c", "editor.cpp", "-o", "editor.o")...); err != nil {
		return o, fmt.Errorf("g++ editor.cpp: %v", err)
	}
	if o, err := run("g++", "-std=c++23", "-w", "-c", "glue.cpp", "-o", "glue.o"); err != nil {
		return o, fmt.Errorf("g++ glue.cpp: %v", err)
	}
	exe := filepath.Join(dir, "cpp")
	if o, err := run("g++", "-o", exe, "editor.o", "glue.o", "harness.o"); err != nil {
		return o, fmt.Errorf("g++ link: %v", err)
	}
	return bounded(60*time.Second, "", exe)
}

// cppRtHpp is the runtime's header the generated source includes; the
// tests' programs run no chunks.
const cppRtHpp = "#pragma once\n"

// cppSame translates src, requires every function written, and requires
// the C++ to print what the C prints; it returns the C++.
func cppSame(t *testing.T, src string, prof Profile, harness string) string {
	t.Parallel()
	requireCpp(t)
	dir := t.TempDir()
	cpp, hpp, host, refused := cppProgram(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(cpp))
	}
	want := cOutputWith(t, dir, src, harness)
	got, err := cppRun(t, dir, cpp, hpp, host, harness)
	if err != nil {
		t.Fatalf("the C++: %v\n%s\n%s\n%s", err, got, numbered(hpp), numbered(cpp))
	}
	if string(got) != want {
		t.Errorf("the C++ prints\n%s\nthe C\n%s\n%s", diffLines(string(got), want), want, numbered(cpp))
	}
	return cpp
}

func TestCppIntegers(t *testing.T)   { cppSame(t, javaIntsC, Profile{}, javaHarnessC) }
func TestCppFlow(t *testing.T)       { cppSame(t, javaFlowC, Profile{}, javaHarnessC) }
func TestCppStrings(t *testing.T)    { cppSame(t, javaStringsC, Profile{}, javaHarnessC) }
func TestCppStructs(t *testing.T)    { cppSame(t, javaStructsC, Profile{}, javaHarnessC) }
func TestCppExtra(t *testing.T)      { cppSame(t, lowerExtraC, Profile{}, javaHarnessC) }
func TestCppVarargs(t *testing.T)    { cppSame(t, javaVarargsC, Profile{}, javaHarnessC) }
func TestCppPointers(t *testing.T)   { cppSame(t, javaPointersC, Profile{}, javaHarnessC) }
func TestCppGoto(t *testing.T)       { cppSame(t, javaGotoC, Profile{}, javaHarnessC) }
func TestCppShapes(t *testing.T)     { cppSame(t, cljShapesC, Profile{}, javaHarnessC) }
func TestCppNest(t *testing.T)       { cppSame(t, cljNestC, Profile{}, javaHarnessC) }
func TestCppMachine(t *testing.T)    { cppSame(t, cljMachineC, Profile{}, javaHarnessC) }
func TestCppDeadLabel(t *testing.T)  { cppSame(t, javaDeadLabelC, Profile{}, javaHarnessC) }
func TestCppNames(t *testing.T)      { cppSame(t, javaNamesC, Profile{}, javaHarnessC) }
func TestCppProfile(t *testing.T)    { cppSame(t, javaProfileC, Profile{}, javaGrowHarnessC) }
func TestCppGrow(t *testing.T)       { cppSame(t, javaGrowC, Profile{}, javaGrowHarnessC) }
func TestCppSteps(t *testing.T)      { cppSame(t, javaStepsC, Profile{}, javaHarnessC) }
func TestCppMasks(t *testing.T)      { cppSame(t, javaMasksC, Profile{}, javaHarnessC) }
func TestCppLocals(t *testing.T)     { cppSame(t, javaLocalsC, Profile{}, javaHarnessC) }
func TestCppTables(t *testing.T)     { cppSame(t, cljTablesC, Profile{}, javaHarnessC) }
func TestCppCljNames(t *testing.T)   { cppSame(t, cljNamesC, Profile{}, javaHarnessC) }
func TestCppSplitRet(t *testing.T)   { cppSame(t, cljSplitReturnC, Profile{}, javaHarnessC) }
func TestCppIdioms(t *testing.T)     { cppSame(t, hsIdiomsC, Profile{}, javaHarnessC) }
func TestCppTuples(t *testing.T)     { cppSame(t, hsTupleC, Profile{}, javaHarnessC) }
func TestCppSigned(t *testing.T)     { cppSame(t, mlSignedC, Profile{}, javaHarnessC) }
func TestCppCaseLabels(t *testing.T) { cppSame(t, scmCaseC, Profile{}, javaHarnessC) }
func TestCppOwn(t *testing.T)        { cppSame(t, cppOwnC, Profile{}, javaGrowHarnessC) }

// cppOwnC is what C does and C++ says otherwise, each once: a void * to
// another pointer, a string literal to char *, char * and unsigned char *,
// an enumeration's arithmetic, a jump past an initial value and a case
// past a scalar the switch leaves unset, a function pointer table and a
// call through one, a block's static, a file-scope compound literal, a
// keyword of C++'s as a name, a braced list that narrows, {0}.
const cppOwnC = javaHost + `
void *alloc(unsigned long n);
typedef unsigned char uchar;
enum color { RED, GREEN = 5, BLUE };
typedef enum { LOW = -1, HIGH = 1 } level_T;
struct op { const char *name; int (*fn)(int); };
struct buf { uchar *b; int n; };
static int twice(int x) { return 2 * x; }
static int neg(int x) { return -x; }
static struct op ops[] = { { "twice", twice }, { "neg", neg }, { 0, 0 } };
static struct buf empty = { (uchar[1]){0}, 0 };
static int counter(void) { static int n = 10; return n++; }
static int jumpy(int k) {
    int r = 0;
    if (k > 2) goto late;
    int v = k * 3;
    r = v;
late:
    r += 1;
    return r;
}
static int sw(int k) {
    switch (k) {
        int t;
    case 1:
        t = 7;
        return t;
    default:
        return -1;
    }
}
void run(void) {
    char *s = "literal";
    uchar *u = (uchar *)s;
    void *p = s;
    char *back = p;
    int new = 3, class = 4;
    enum color c = GREEN;
    c++;
    level_T lv = LOW;
    unsigned char narrow[2] = { new, class };
    struct buf zero = {0};
    long big = 300;
    char small[2] = { (char)big, 1 };
    int (*f)(int) = neg;
    outs(back);
    out(u[1]);
    out(c);
    out(lv < 0);
    out(narrow[0] + narrow[1]);
    out(zero.n + small[1]);
    for (struct op *o = ops; o->name; o++) { outs(o->name); out(o->fn(21)); }
    out(f(4));
    out((*f)(5));
    out(f == neg);
    out(counter()); out(counter());
    out(jumpy(1)); out(jumpy(5));
    out(sw(1)); out(sw(2));
    out(empty.b[0] + empty.n);
    uchar *q = alloc(4);
    q[3] = 9;
    out(q[3]);
}
`

// The control: the comparison sees a translation that is wrong.  Each
// mutation undoes one of C's rules in the generated C++ -- an unsigned
// char read as a char, a function pointer's call made of another, a
// block's static made every call's -- and each must move the output.
func TestCppControl(t *testing.T) {
	requireCpp(t)
	for _, c := range []struct {
		name, src string
		from      *regexp.Regexp
		repl      string
	}{
		{"unsigned widening", cppOwnC, regexp.MustCompile(`out\(u\[1\]\)`), "out((signed char)u[1] - 200)"},
		{"function pointer", cppOwnC, regexp.MustCompile(`out\(\(this->\*o->fn\)\(21\)\)`), "out((this->*f)(21))"},
		{"static", cppOwnC, regexp.MustCompile(`return counter__n\+\+;`), "int counter__n = 10;\n    return counter__n++;"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			cpp, hpp, host, _ := cppProgram(t, dir, c.src, Profile{})
			if !c.from.MatchString(cpp) {
				t.Fatalf("the control's pattern %s is not in the C++:\n%s", c.from, numbered(cpp))
			}
			bad := c.from.ReplaceAllString(cpp, c.repl)
			want := cOutputWith(t, dir, c.src, javaGrowHarnessC)
			got, _ := cppRun(t, dir, bad, hpp, host, javaGrowHarnessC)
			if string(got) == want {
				t.Errorf("the mutation %q moved nothing", c.name)
			}
		})
	}
}
