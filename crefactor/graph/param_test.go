package graph

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// PARAM, RETYPE and MOVE (B2c) on graphs read back from their Lisp, no cc
// node behind them: each edit's C view held to the C written by hand,
// printed canonically, byte for byte; the invariants (Check) after it; the
// graph written and read back the same graph; and each refusal to its
// message, the graph untouched.

// b2c is src's graph read back from its Lisp, an editor on it, verbs to
// find its nodes by pattern, and the path the canonical text is printed
// under.
func b2c(t *testing.T, src string) (*Editor, *Verbs, string) {
	t.Helper()
	path, _, g := importSample(t, src)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	return e, NewVerbs("t", e, io.Discard), path
}

// holds requires the graph's C view to be want printed canonically, the
// invariants to hold, and the graph to read back from its Lisp the same.
func holds(t *testing.T, e *Editor, path, want string) {
	t.Helper()
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	canon, err := cemit.Canonical(path, []byte(want))
	if err != nil {
		t.Fatalf("the C written by hand does not print: %v", err)
	}
	got, err := e.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, canon) {
		t.Fatalf("%s\ngraph:\n%s\nwant:\n%s", firstDiff(got, canon), got, canon)
	}
	h, err := Read(e.Graph().Lisp())
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if err := Equal(e.Graph(), h); err != nil {
		t.Fatalf("read back: %v", err)
	}
}

// unchanged requires the graph's C view to be what it was: a refusal.
func unchanged(t *testing.T, e *Editor, before []byte, err error, saying string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), saying) {
		t.Fatalf("err = %v, want a refusal saying %q", err, saying)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.Graph().C(); !bytes.Equal(got, before) {
		t.Fatalf("a refusal changed the graph:\n%s", got)
	}
}

func cview(t *testing.T, e *Editor) []byte {
	t.Helper()
	b, err := e.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const paramSample = `static int g(int x, char *p, int y);
static int h(void);
int
g(int x, char *p, int y)
{
    return x + y;
}
int
main(void)
{
    char c = 0;
    int i = 0;
    return g(1, &c, 2) + g(3, (&c), i);
}
`

// A parameter dropped from every declaration and the argument from every
// call; the function's type node the new one's, interned; nothing untyped.
func TestDropParam(t *testing.T) {
	e, v, path := b2c(t, paramSample)
	g0 := e.FileDecls("g")[0]
	oldType := g0.Type
	st, err := e.DropParam("g", "p", ParamOptions{})
	if err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, strings.NewReplacer("char *p, ", "", "&c, ", "", "(&c), ", "").Replace(paramSample))
	if st.Params != 2 || st.Args != 2 || st.Calls != 2 || st.Retyped != 2 {
		t.Errorf("stats: %s", st)
	}
	defn := e.Defn("g")
	if g0.Type == oldType || defn.Type != g0.Type || len(e.Untyped) != 0 {
		t.Errorf("types: proto %s, defn %s, %d untyped", Lisp(g0.Type), Lisp(defn.Type), len(e.Untyped))
	}
	if ps, _, _, _ := funcParts(g0.Type); len(ps) != 2 {
		t.Errorf("the new type has %d parameters", len(ps))
	}
	// the calls keep their ids, and their type
	for _, c := range v.Find("(call g _ _)") {
		if c.ID == 0 || c.Type == nil {
			t.Errorf("call %s", Lisp(c))
		}
	}
	// a parameter list that empties says void
	e, _, path = b2c(t, `static int one(int x);
int one(int x) { return 1; }
int main(void) { return one(2); }
`)
	if _, err := e.DropParam("one", "x", ParamOptions{}); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `static int one(void);
int one(void) { return 1; }
int main(void) { return one(); }
`)
}

// What PARAM refuses, the graph untouched: a parameter still used (and,
// with Dangle, the use left dangling), an argument with a side effect, a
// function whose address goes where the edit does not follow, a `...`.
func TestDropParamRefuses(t *testing.T) {
	e, _, _ := b2c(t, paramSample)
	before := cview(t, e)
	_, err := e.DropParam("g", "x", ParamOptions{})
	unchanged(t, e, before, err, "parameter x is still used")
	_, err = e.DropParam("g", "y", ParamOptions{})
	unchanged(t, e, before, err, "parameter y is still used")
	// Dangle: the use stays, recorded
	if _, err := e.DropParam("g", "x", ParamOptions{Dangle: true}); err != nil {
		t.Fatal(err)
	}
	if d := e.Dangling(); len(d) != 1 || d[0].Use.Atom != "x" {
		t.Errorf("dangling: %v", d)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}

	e, _, _ = b2c(t, `static int g(int x, int y);
int g(int x, int y) { return x; }
int main(void) { int i = 0; return g(1, i++); }
`)
	before = cview(t, e)
	_, err = e.DropParam("g", "y", ParamOptions{})
	unchanged(t, e, before, err, "has a side effect")

	e, _, _ = b2c(t, `typedef int (*fp_T)(int, int);
static int g(int x, int y);
static fp_T tab[] = { g };
int g(int x, int y) { return x; }
int main(void) { return g(1, 2) + tab[0](3, 4); }
`)
	before = cview(t, e)
	_, err = e.DropParam("g", "y", ParamOptions{})
	unchanged(t, e, before, err, "in an initialiser")
	tab := e.FileDecls("tab")[0]
	_, err = e.DropParams([]ParamDrop{{Decl: tab, I: 1}}, ParamOptions{})
	unchanged(t, e, before, err, "the typedef fp_T: drop the parameter there")

	e, _, _ = b2c(t, `static int f(const char *fmt, ...);
int f(const char *fmt, ...) { return 0; }
int main(void) { return f("%d", 1); }
`)
	before = cview(t, e)
	_, err = e.DropParams([]ParamDrop{{Decl: e.Defn("f"), I: 1}}, ParamOptions{})
	unchanged(t, e, before, err, "DropArg's")
}

// A family through pointers: the function, the typedef its table is of,
// the table's call -- one edit; and a member's pointer with the functions
// it is set to and the parameter that hands it on.
func TestDropParamsFamily(t *testing.T) {
	src := `typedef int (*fp_T)(int, int);
static int g(int x, int y);
static fp_T tab[] = { g };
int g(int x, int y) { return x; }
int main(void) { return g(1, 2) + tab[0](3, 4); }
`
	e, _, path := b2c(t, src)
	td := e.FileDecls("fp_T")
	var typedef *Node
	for _, f := range e.Graph().Forms {
		if f.Is("typedef") && topName(f) == "fp_T" || f.Is("def") && hasPrefix(f, "typedef") && topName(f) == "fp_T" {
			typedef = f
		}
	}
	if typedef == nil {
		t.Fatalf("no typedef fp_T (%d decls)", len(td))
	}
	st, err := e.DropParams([]ParamDrop{{Decl: e.Defn("g"), I: 1}, {Decl: typedef, I: 1}}, ParamOptions{})
	if err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `typedef int (*fp_T)(int);
static int g(int x);
static fp_T tab[] = { g };
int g(int x) { return x; }
int main(void) { return g(1) + tab[0](3); }
`)
	if st.Calls != 2 || st.Retyped != 4 || len(e.Untyped) != 0 { // g twice, fp_T, tab
		t.Errorf("stats %s, %d untyped", st, len(e.Untyped))
	}
	if tab := e.FileDecls("tab")[0]; pointee(tab.Type) != typedef.Type {
		t.Errorf("tab is typed %s", Lisp(tab.Type))
	}

	src = `struct s { int (*cb)(int, void *); int n; };
static int h(int a, void *cookie);
static int run(int (*f)(int, void *), void *cookie);
int h(int a, void *cookie) { return a; }
int run(int (*f)(int, void *), void *cookie) { return f(1, cookie); }
int main(void)
{
    struct s v;
    v.cb = h;
    if (v.cb == h || v.cb != 0) { v.n = v.cb(2, 0); }
    return run(h, 0) + run(v.cb, 0) + h(3, 0);
}
`
	e, v, path := b2c(t, src)
	cb := v.One("(cb (ptr (fn (int ((ptr void))) int)))", "the member")
	runF := e.Defn("run")
	f := paramNamed(runF, "f")
	if cb == nil || f == nil {
		t.Fatal(v.Done())
	}
	before := cview(t, e)
	// h alone: the member it is stored in would not fit
	_, err = e.DropParam("h", "cookie", ParamOptions{})
	unchanged(t, e, before, err, "another type")
	// the family, and run's own cookie, which it hands to f
	st, err = e.DropParams([]ParamDrop{
		{Decl: e.Defn("h"), I: 1}, {Decl: cb, I: 1}, {Decl: f, I: 1}, {Decl: runF, I: 1},
	}, ParamOptions{})
	if err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `struct s { int (*cb)(int); int n; };
static int h(int a);
static int run(int (*f)(int));
int h(int a) { return a; }
int run(int (*f)(int)) { return f(1); }
int main(void)
{
    struct s v;
    v.cb = h;
    if (v.cb == h || v.cb != 0) { v.n = v.cb(2); }
    return run(h) + run(v.cb) + h(3);
}
`)
	if len(e.Untyped) != 0 {
		t.Errorf("%d untyped: %s", len(e.Untyped), Lisp(e.Untyped[0]))
	}
	sel := v.Find("(. v cb)")
	for _, s := range sel {
		if s.Type != cb.Type {
			t.Errorf("%s is typed %s, the member %s", Lisp(s), Lisp(s.Type), Lisp(cb.Type))
		}
	}
	_ = st
}

// An argument passed through `...`, dropped alone.
func TestDropArg(t *testing.T) {
	e, v, path := b2c(t, `static int f(const char *fmt, ...);
int f(const char *fmt, ...) { return 0; }
int main(void) { int i = 0; return f("%d %d", 1, i) + f("%d", i++); }
`)
	c := v.One(`(call f "%d %d" 1 i)`, "the call")
	before := cview(t, e)
	unchanged(t, e, before, e.DropArg(c, 0), "not one its callee takes through `...`")
	unchanged(t, e, before, e.DropArg(v.One(`(call f "%d" _)`, "the other"), 1), "side effect")
	if err := e.DropArg(c, 1); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `static int f(const char *fmt, ...);
int f(const char *fmt, ...) { return 0; }
int main(void) { int i = 0; return f("%d %d", i) + f("%d", i++); }
`)
}

// A parameter made a local: its node and id moved into the body as the
// first item, its uses unchanged.
func TestParamToLocal(t *testing.T) {
	src := `static int key(int c, int *got);
int
key(int c, int *got)
{
    c = 1;
    *got = c;
    return c;
}
int main(void) { int g = 0; return key(5, &g); }
`
	e, _, path := b2c(t, src)
	p := paramNamed(e.Defn("key"), "c")
	id, uses := p.ID, len(e.Uses(p))
	if _, err := e.ParamToLocal("key", "c"); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `static int key(int *got);
int
key(int *got)
{
    int c;
    c = 1;
    *got = c;
    return c;
}
int main(void) { int g = 0; return key(&g); }
`)
	if !e.Live(p) || p.ID != id || !p.Is("def") || len(e.Uses(p)) != uses || len(e.Untyped) != 0 {
		t.Errorf("the local: live %v, #%d (was #%d), %s, %d uses (was %d), %d untyped", e.Live(p), p.ID, id, Lisp(p), len(e.Uses(p)), uses, len(e.Untyped))
	}
}

// A parameter added: in every declaration, read there; the argument at
// every call, read at the call.
func TestAddParam(t *testing.T) {
	e, _, _ := b2c(t, paramSample)
	before := cview(t, e)
	_, err := e.AddParam("g", 1, "(n long)", func(c *Node) (string, Bindings) { return "nowhere", nil })
	unchanged(t, e, before, err, "declared nowhere visible")
	e, _, path := b2c(t, paramSample)
	st, err := e.AddParam("g", 1, "(n long)", func(c *Node) (string, Bindings) { return "7", nil })
	if err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, strings.NewReplacer("int x, char *p", "int x, long n, char *p", "g(1, ", "g(1, 7, ", "g(3, ", "g(3, 7, ").Replace(paramSample))
	if st.Params != 2 || st.Args != 2 || len(e.Untyped) != 0 {
		t.Errorf("stats %s, %d untyped", st, len(e.Untyped))
	}
	n := paramNamed(e.Defn("g"), "n")
	if n == nil || n.Type == nil || n.Type.Kids[1].Atom != "long" || n.ID == 0 {
		t.Errorf("the parameter: %v", n)
	}
	// it would hide the file's object the body names
	e, _, _ = b2c(t, `static int n = 1;
static int g(int x);
int g(int x) { return x + n; }
int main(void) { return g(1); }
`)
	before = cview(t, e)
	_, err = e.AddParam("g", 0, "(n int)", func(*Node) (string, Bindings) { return "0", nil })
	unchanged(t, e, before, err, "would hide")
}

// The verbs: each act reported, the first refusal stopping the rest.
func TestB2cVerbs(t *testing.T) {
	path, _, g := importSample(t, paramSample)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	v := NewVerbs("b2c", NewEditor(h), &log)
	v.DropParam("g", "p", "g takes no p")
	v.InFunction("main", func(v *Verbs) {
		v.Retype("(def i int 0)", "long", "i is a long")
		v.MoveBefore("(def i long 0)", "(def c char 0)", "i first")
	})
	v.DropParam("g", "x", "g takes no x")
	v.DropParam("g", "y", "never reached")
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "g takes no x -- param: g's parameter x is still used") {
		t.Fatalf("err = %v", err)
	}
	if want := "  b2c          g takes no p\n  b2c          i is a long\n  b2c          i first\n"; log.String() != want {
		t.Errorf("reported\n%s", log.String())
	}
	holds(t, v.Editor(), path, `static int g(int x, int y);
static int h(void);
int
g(int x, int y)
{
    return x + y;
}
int
main(void)
{
    long i = 0;
    char c = 0;
    return g(1, 2) + g(3, i);
}
`)
}
