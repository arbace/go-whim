package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// moRun is MemberOut on src imported and read back from its Lisp: the C
// view, the report, and the graph held to the import of its C view.
func moRun(t *testing.T, src string) (string, MemberOutResult) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.c")
	g, _, err := Import(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if g, err = Read(g.Lisp()); err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	r, err := e.MemberOut()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if len(e.Untyped) != 0 {
		t.Errorf("%d untyped", len(e.Untyped))
	}
	out, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	h, _, err := Import(path, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(g, h); err != nil {
		t.Errorf("not the import of its C view: %v", err)
	}
	if gcc, err := exec.LookPath("gcc"); err == nil {
		c := filepath.Join(dir, "out.c")
		os.WriteFile(c, out, 0o644)
		if b, err := exec.Command(gcc, "-fsyntax-only", c).CombinedOutput(); err != nil {
			t.Errorf("gcc: %v\n%s\n%s", err, b, out)
		}
	}
	return string(out), r
}

// crefactor/xform's TestMemberOut, its source and its expectations: a
// callee that reads and writes through the pointer and nothing else is
// taken; one that names the member itself, one that keeps the pointer, and
// a member whose address is kept elsewhere too are held.
func TestMemberOutGraph(t *testing.T) {
	src := `struct s { int a; int b; int c; int d; };
static int *kept;
static void bump(int *p) { *p += 1; }
static void peek(struct s *o, int *p) { *p = o->b + 1; }
static void keep(int *p) { kept = p; }
static void twice(int *p) { bump(p); bump(p); }
static int *elsewhere;
void f(struct s *o)
{
    bump(&o->a);
    twice(&o->a);
    peek(o, &o->b);
    keep(&o->c);
    bump(&o->d);
    elsewhere = &o->d;
}
`
	got, r := moRun(t, src)
	for _, want := range []string{
		"bump__a(o);", "twice__a(o);", // taken: every address of s.a goes
		"peek(o, &o->b);", // peek names b
		"keep(&o->c);",    // keep keeps the pointer
		"bump(&o->d);",    // d's address is kept elsewhere too
		"typeof(s0__->a) a0__ = s0__->a;",
		"    static void\nbump__a(struct s *s0__)\n{\n    typeof(s0__->a) a0__ = s0__->a;\n    bump(&a0__);\n    s0__->a = a0__;\n}\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
	want := []string{
		"2 arguments pass a member's address no more, through 2 functions",
		"held: 1 a callee that can name the member",
		"held: 1 a callee that keeps the pointer",
		"held: 1 a member whose address is kept elsewhere too",
	}
	if l := strings.Join(r.Lines(), "\n"); l != strings.Join(want, "\n") {
		t.Errorf("report:\n%s", l)
	}
}

// A local struct whose address goes nowhere but into the call is taken
// even where the callee names the member; the wrapper takes the struct's
// address, spelled by its typedef, and returns the callee's result.
func TestMemberOutLocal(t *testing.T) {
	src := `typedef struct { char *p; int n; } vars_T;
static int adv(char **pp) { vars_T other; other.p = *pp; *pp += 1; return other.p[0]; }
int g(char *s)
{
    vars_T v;
    v.p = s;
    v.n = adv(&v.p);
    return v.n;
}
`
	got, r := moRun(t, src)
	for _, want := range []string{
		"    static int\nadv__p(vars_T *s0__)\n{\n    typeof(s0__->p) p0__ = s0__->p;\n    int r__ = adv(&p0__);\n    s0__->p = p0__;\n    return r__;\n}\n",
		"v.n = adv__p(&v);",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
	if r.Taken != 1 || len(r.Wrappers) != 1 {
		t.Errorf("%v", r.Lines())
	}
}
