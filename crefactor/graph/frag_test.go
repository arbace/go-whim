package graph

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
)

const fragSample = `#include <errno.h>
#include <stddef.h>
#include <string.h>
#include <sys/param.h>
#include <stdarg.h>
enum { FALSE, TRUE };
struct buf { struct buf *next; int n; };
struct win { struct win *next; struct buf *w_buffer; int n; };
typedef struct buf buf_T;
static buf_T *curbuf;
static int total = 0;
int g(int x);
static int sum(int n, ...);
int
f(int a)
{
    int k = a;
    if (a)
    {
        k = g(k);
    }
    total = k;
    return total;
}
int g(int x) { return x + total; }
static int sum(int n, ...)
{
    va_list ap;
    int s = 0;
    va_start(ap, n);
    while (n-- > 0)
    {
        s += va_arg(ap, int);
    }
    va_end(ap);
    return s;
}
int
pick(struct win *wp, int lo, int hi)
{
    int w = MIN(lo, hi);
    return MAX(w, wp->n) + MIN(hi, 3);
}
`

// fragOn is the sample's graph, read back from its Lisp (no cc node behind
// it), and its editor.
func fragOn(t *testing.T, src string) (string, []byte, *Editor) {
	t.Helper()
	path, canon, v, _ := verbsOn(t, src)
	return path, canon, v.Editor()
}

// asImported holds the edited graph to the text: its C view is want, and
// it is the graph an import of that C gives, ids aside.
func asImported(t *testing.T, path string, e *Editor, want []byte) {
	t.Helper()
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	got, err := e.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	if want != nil && !bytes.Equal(got, want) {
		t.Fatalf("%s\ngraph:\n%s\ntext:\n%s", firstDiff(got, want), got, want)
	}
	h, _, err := Import(path, got)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(e.Graph(), h); err != nil {
		t.Fatalf("not the import of its C view: %v", err)
	}
	if c := e.Graph().Count(); c.Dangling != 0 {
		t.Fatalf("%d edges dangle", c.Dangling)
	}
}

// textDo is a text verb's result, printed canonically.
func textDo(t *testing.T, path string, canon []byte, act func(*edit.E)) []byte {
	t.Helper()
	var log bytes.Buffer
	x := edit.New("tiny", canon, &log)
	act(x)
	out, err := x.Done()
	if err != nil {
		t.Fatal(err)
	}
	want, err := cemit.Canonical(path, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return want
}

// A whole body: a parameter, the file's object over nothing, a local, a
// call, members by type (two structs have `next` and `n`), a label and its
// goto, a header's macro (errno), a builtin, a header's function.
func TestFragBody(t *testing.T) {
	path, canon, e := fragOn(t, fragSample)
	body := `    struct win *wp = nullptr;
    int k = a + (int)strlen("x");
    if (k == 0)
    {
        goto out;
    }
    errno = 0;
    k = (int)__builtin_offsetof(struct win, n) + curbuf->next->n;
    if (wp != nullptr && wp->next->w_buffer == curbuf)
    {
        return wp->n;
    }
out:
    return g(k) + sum(2, k, total);
`
	want := textDo(t, path, canon, func(x *edit.E) { x.Body("f", body, "f") })
	v := NewVerbs("tiny", e, &bytes.Buffer{})
	maxBefore := maxID(e.Graph())
	v.BodyC("f", body, "f's body")
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, want)
	// fresh ids, from the graph's IDs
	last := e.Log[len(e.Log)-1]
	for _, a := range e.Log {
		if a.Op == "replace" {
			last = a
		}
	}
	if len(last.New) == 0 || len(last.Gone) == 0 {
		t.Fatalf("the splice gave %d ids and superseded %d", len(last.New), len(last.Gone))
	}
	for _, id := range last.New {
		if id <= maxBefore {
			t.Fatalf("id %d given again (the greatest was %d)", id, maxBefore)
		}
	}
	// a few edges: the parameter, the headers', the file's object
	f := e.Defn("f")
	got := map[string]string{}
	Walk(f, func(n *Node) bool {
		if !n.list && n.Ref() != nil {
			got[n.Atom] = n.Ref().Head()
			if n.Ref().list && len(n.Ref().Kids) > 1 && !n.Ref().Kids[0].list && n.Ref().Kids[0].Atom == n.Atom {
				got[n.Atom] = "param"
			}
		}
		return true
	})
	for name, head := range map[string]string{"a": "param", "strlen": "extern", "errno": "extern",
		"curbuf": "def", "out": "label", "g": "def", "sum": "def"} {
		if got[name] != head {
			t.Errorf("%s refers to a %s, want a %s", name, got[name], head)
		}
	}
	// the builtin is a macro's invocation, its edge to struct win's n, by
	// the type the importer resolves it with
	off := One(f, "(macro _)")
	win := e.tagDef("struct", "win")
	if off == nil || off.Ref() != memberNamed(win, "n") {
		t.Errorf("__builtin_offsetof(struct win, n) does not refer to struct win's n")
	}
}

func maxID(g *Graph) ID {
	var m ID
	g.Walk(func(n *Node) bool { m = max(m, n.ID); return true })
	return m
}

// A run of items replaced: the fragment declares a local `total`, and the
// use after the run that referred to the file's object refers to it now;
// and items inserted, before and after.
func TestFragRun(t *testing.T) {
	path, canon, e := fragOn(t, fragSample)
	v := NewVerbs("tiny", e, &bytes.Buffer{})
	v.InFunction("f", func(v *Verbs) {
		v.LiteralC("if (a) { k = g(k); } total = k;", "int total = k * 2;\nk = total;", 1, "a run")
		v.BeforeC("(return total)", "k++;", 1, "before the return")
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	want := textDo(t, path, canon, func(x *edit.E) {
		x.Literal("    if (a)\n    {\n        k = g(k);\n    }\n    total = k;\n", "    int total = k * 2;\n    k = total;\n", 1, "a run")
		x.Literal("    return total;\n", "    k++;\n    return total;\n", 1, "before")
	})
	asImported(t, path, e, want)
	ret := One(e.Defn("f"), "(return total)")
	if ret == nil || !ret.Kids[1].Ref().Is("def") || e.Function(ret.Kids[1].Ref()) == nil {
		t.Fatalf("the return's total refers to %s, not the new local", label(ret.Kids[1].Ref()))
	}
}

// One returns the one node under root the pattern matches, or nil.
func One(root *Node, pat string) *Node {
	p := clisp.MustPattern(pat)
	ms := Find(root, func(n *Node) bool { return Matches(p, n) })
	if len(ms) != 1 {
		return nil
	}
	return ms[0]
}

// An expression's place, with a hole: the hole's node moved in, its
// parentheses kept only where an operator needs them; and an
// initialiser-like place.
func TestFragExprHoles(t *testing.T) {
	path, _, e := fragOn(t, fragSample)
	v := NewVerbs("tiny", e, &bytes.Buffer{})
	v.InFunction("f", func(v *Verbs) {
		v.ReplaceAtC("(= total ?x)", "x", "2 * ($x)", 1, "the store, doubled")
		v.ReplaceAtC("(def k int ?v)", "v", "g($v + 1)", 1, "the local, through g")
	})
	v.InFunction("g", func(v *Verbs) {
		v.ReplaceAtC("(return ?r)", "r", "$r * $r", 1, "a hole twice: a copy")
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, nil)
	out, _ := e.Graph().C()
	for _, s := range []string{"total = 2 * (k);", "int k = g(a + 1);", "return (x + total) * (x + total);"} {
		if !bytes.Contains(out, []byte(s)) {
			t.Errorf("no %q in\n%s", s, out)
		}
	}
	// the hole kept its id
	if One(e.Defn("f"), "(= total (* 2 (paren k)))") == nil {
		t.Fatalf("the store is not (= total (* 2 (paren k)))")
	}
}

// External declarations: a prototype that comes first takes the uses of
// the declaration that was first; a new function definition; a new type
// node and a new external.
func TestFragTop(t *testing.T) {
	path, canon, e := fragOn(t, fragSample)
	v := NewVerbs("tiny", e, &bytes.Buffer{})
	top := "static int g(int x);\n\nstatic char ***triple(void)\n{\n    return nullptr;\n}\n\nstatic size_t len2(const char *s)\n{\n    return strlen(s) + strspn(s, \"ab\");\n}\n"
	v.TopBeforeC("total", top, "before total")
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	want := textDo(t, path, canon, func(x *edit.E) {
		x.Literal("static int total = 0;\n", strings.ReplaceAll(top, "static int g(int x);", "int g(int x);")+"\nstatic int total = 0;\n", 1, "top")
	})
	_ = want // the prototype's storage class differs on purpose: the text below is the graph's own
	asImported(t, path, e, nil)
	proto := e.FileDecls("g")[0]
	if !hasPrefix(proto, "static") {
		t.Fatalf("the first declaration of g is %s", Lisp(proto))
	}
	if call := One(e.Defn("f"), "(call g k)"); call == nil || call.Kids[1].Ref() != proto {
		t.Fatalf("f's call of g does not refer to the new first declaration")
	}
}

// A body's place (an if's), and several fragments in one import, two in
// one list.
func TestFragMany(t *testing.T) {
	path, canon, e := fragOn(t, fragSample)
	iff := One(e.Defn("f"), "(if a _*)")
	ret := One(e.Defn("g"), "(return _)")
	store := One(e.Defn("f"), "(= total k)")
	_, err := e.SpliceC(
		Frag{At: e.SpotOf(iff.Kids[2]), Src: "{\n    k = 7;\n    k += a;\n}"},
		Frag{At: e.SpotOf(ret), Src: "if (x < 0)\n    return -x;\nreturn x;"},
		Frag{At: e.SpotBefore(store), Src: "k--;"},
		Frag{At: e.SpotAfter(store), Src: "k++;"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := textDo(t, path, canon, func(x *edit.E) {
		x.Literal("        k = g(k);\n", "        k = 7;\n        k += a;\n", 1, "then")
		x.Literal("    total = k;\n", "    k--;\n    total = k;\n    k++;\n", 1, "around")
	})
	_ = want
	asImported(t, path, e, nil)
	out, _ := e.Graph().C()
	for _, s := range []string{"        k = 7;\n        k += a;\n", "    k--;\n    total = k;\n    k++;\n", "    if (x < 0)\n    {\n        return -x;\n    }\n    return x;\n"} {
		if !bytes.Contains(out, []byte(s)) {
			t.Errorf("no %q in\n%s", s, out)
		}
	}
}

// MACROX: invocations in a fragment made nodes as the importer makes them,
// and an invocation in the graph expanded, its arguments read from its
// text: each MIN and MAX is the header's own expansion, as phase 42's text
// expansion writes it.
func TestFragMacros(t *testing.T) {
	path, canon, e := fragOn(t, fragSample)
	v := NewVerbs("tiny", e, &bytes.Buffer{})
	tmpl := map[string]string{"MIN": "(((A)<(B))?(A):(B))", "MAX": "(((A)>(B))?(A):(B))"}
	expand := func(name string, args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("%d arguments", len(args))
		}
		return strings.NewReplacer("A", args[0], "B", args[1]).Replace(tmpl[name]), nil
	}
	v.InFunction("pick", func(v *Verbs) {
		v.ExpandMacros([]string{"MIN", "MAX"}, expand, 3, "MIN and MAX expanded")
	})
	v.InFunction("sum", func(v *Verbs) {
		v.ReplaceC("(+= s _)", "s += va_arg(ap, int) * 2;", 1, "a va_arg in a fragment")
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	want := textDo(t, path, canon, func(x *edit.E) {
		x.Literal("MIN(lo, hi)", "(((lo)<(hi))?(lo):(hi))", 1, "")
		x.Literal("MAX(w, wp->n)", "(((w)>(wp->n))?(w):(wp->n))", 1, "")
		x.Literal("MIN(hi, 3)", "(((hi)<(3))?(hi):(3))", 1, "")
		x.Literal("s += va_arg(ap, int);", "s += va_arg(ap, int) * 2;", 1, "")
	})
	asImported(t, path, e, want)
	m := One(e.Defn("sum"), "(+= s (* (macro _) 2))")
	if m == nil || len(m.Kids[2].Kids[1].Refs) == 0 {
		t.Fatalf("the va_arg is not a macro node with its expansion's edges")
	}
	if name, args, ok := MacroCall(NewList(NewAtom("macro"), NewAtom(`"F(a, g(b, c), \"x,y\")"`))); !ok || name != "F" || len(args) != 3 || args[1] != "g(b, c)" {
		t.Fatalf("MacroCall: %s %q %v", name, args, ok)
	}
}

// What FRAG refuses, at the spot, and leaves the graph as it was.
func TestFragRefusals(t *testing.T) {
	for _, c := range []struct {
		src, saying string
	}{
		{"nowhere(1);", "`nowhere` is declared nowhere visible"},
		{"k = ;", "frag 1 line 1"},
		{"curbuf->nope = 1;", "frag 1 line 1"},
		{"#include <stdio.h>\nk = 1;", "INCLUDE's"},
		{"k = $missing;", "bound to nothing"},
	} {
		path, canon, e := fragOn(t, fragSample)
		store := One(e.Defn("f"), "(= total k)")
		_, err := e.SpliceC(Frag{At: e.SpotOf(store), Src: c.src})
		if err == nil || !strings.Contains(err.Error(), c.saying) {
			t.Errorf("%q: %v, want %q", c.src, err, c.saying)
		}
		asImported(t, path, e, canon)
	}
	// an expression that does not stand as one node in its place
	path, canon, e := fragOn(t, fragSample)
	x := One(e.Defn("g"), "(+ x total)")
	_, err := e.SpliceC(Frag{At: e.SpotOf(x.Kids[1]), Src: "x, 1"})
	if err == nil || !strings.Contains(err.Error(), "did not stand where it was written") {
		t.Errorf("a comma in an operand: %v", err)
	}
	asImported(t, path, e, canon)
}

// CLONE: a copy's edges inside it go to its own nodes, the others where the
// original's go; and Build's hole used twice is a copy.
func TestClone(t *testing.T) {
	path, _, e := fragOn(t, fragSample)
	w := One(e.Defn("sum"), "(while _*)")
	c := Clone(w)
	var inner, outer int
	Walk(c, func(n *Node) bool {
		if n.ID != 0 {
			t.Fatalf("a clone has an id")
		}
		for _, r := range n.Refs {
			if within(c, r) {
				inner++
			} else {
				outer++
			}
			if within(w, r) {
				t.Fatalf("a clone refers into the original")
			}
		}
		return true
	})
	if outer == 0 {
		t.Fatalf("the clone refers to nothing outside it")
	}
	if err := e.InsertAfter(w, c); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, nil)
	// a declaration and its use inside: the use goes to the copy
	blk := NewList(NewAtom("block"))
	d := NewList(NewAtom("def"), NewAtom("q"), NewAtom("int"), NewAtom("0"))
	u := &Node{Atom: "q", Refs: []*Node{d}}
	blk.Kids = append(blk.Kids, d, NewList(NewAtom("post++"), u))
	cb := Clone(blk)
	if cb.Kids[2].Kids[1].Ref() != cb.Kids[1] {
		t.Fatalf("the copy's use refers to %s", label(cb.Kids[2].Kids[1].Ref()))
	}
	// Build: a hole twice
	_, _, v := func() (string, []byte, *Verbs) { p, c, v, _ := verbsOn(t, buildSample); return p, c, v }()
	ret := v.One("(return total)", "the return")
	ns, err := v.Editor().Build(ret, "(return (+ ?h ?h))", Bindings{"h": ret.Kids[1]})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Editor().Replace(ret, ns...); err != nil {
		t.Fatal(err)
	}
	sumN := ns[0].Kids[1]
	if sumN.Kids[1] == sumN.Kids[2] || sumN.Kids[1].Ref() != sumN.Kids[2].Ref() {
		t.Fatalf("the two uses are not two nodes with one target")
	}
	if err := v.Editor().Check(); err != nil {
		t.Fatal(err)
	}
}

func within(root, n *Node) bool {
	found := false
	Walk(root, func(x *Node) bool { found = found || x == n; return !found })
	return found
}

// Together: the FRAG acts of a batch made in one import, the C the acts
// make one by one; reported at the end in order; a refusal names its act.
func TestFragTogether(t *testing.T) {
	acts := func(v *Verbs) {
		v.BodyC("g", "return x * 2;", "g doubles")
		v.InFunction("f", func(v *Verbs) {
			v.LiteralC("total = k;", "total = k + 1;", 1, "f adds one")
			v.ReplaceAtC("(return ?r)", "r", "$r - 1", 1, "f takes one")
		})
		v.TopAfterC("sum", "static int three(void) { return g(1) + 1; }", "three")
	}
	path, _, one := fragOn(t, fragSample)
	var log1 bytes.Buffer
	v1 := NewVerbs("tiny", one, &log1)
	acts(v1)
	if err := v1.Done(); err != nil {
		t.Fatal(err)
	}
	_, _, two := fragOn(t, fragSample)
	var log2 bytes.Buffer
	v2 := NewVerbs("tiny", two, &log2)
	v2.Together(acts)
	if err := v2.Done(); err != nil {
		t.Fatal(err)
	}
	a, _ := one.Graph().C()
	asImported(t, path, two, a)
	if log1.String() != log2.String() {
		t.Errorf("the report:\n%s\nagainst\n%s", log2.String(), log1.String())
	}
	_, _, three := fragOn(t, fragSample)
	var log3 bytes.Buffer
	v3 := NewVerbs("tiny", three, &log3)
	v3.Together(func(v *Verbs) {
		v.BodyC("g", "return x * 2;", "g doubles")
		v.BodyC("f", "return nowhere;", "f is refused")
	})
	if err := v3.Done(); err == nil || !strings.Contains(err.Error(), "f is refused") || log3.Len() != 0 {
		t.Errorf("a batch's refusal: %v, reported %q", err, log3.String())
	}
}

// The control: one refers edge moved to another declaration of the same
// name, and SameGraph says the graph is not the import of its C view.
func TestSameGraphControl(t *testing.T) {
	path, canon, e := fragOn(t, fragSample)
	asImported(t, path, e, canon)
	st := One(e.Defn("f"), "(= total k)")
	h, _, err := Import(path, canon)
	if err != nil {
		t.Fatal(err)
	}
	st.Kids[2].Refs[0] = e.FileDecls("total")[0] // k, the local, made the file's total
	if err := SameGraph(e.Graph(), h); err == nil {
		t.Fatal("SameGraph did not see a retargeted edge")
	}
}

// Names resolve as cc resolves them, which is not "the first declaration":
// a use after a function's definition refers to the definition when its
// prototype names no parameter.  A fragment after the definition calls it
// so; and a definition replaced by a top-level fragment takes with it the
// uses of the old one, every form using the name printed whole for the
// synthesized import to say so.
func TestFragResolvesAsCC(t *testing.T) {
	src := `static int h(int);
int before(void) { return h(0); }
static int h(int y) { return y; }
int after(void) { return h(1); }
int other(void) { return 2; }
`
	path, _, e := fragOn(t, src)
	defn := e.Defn("h")
	proto := e.FileDecls("h")[0]
	if One(e.Defn("after"), "(call h 1)").Kids[1].Ref() != defn || One(e.Defn("before"), "(call h 0)").Kids[1].Ref() != proto {
		t.Fatal("the import does not resolve as this test says cc does")
	}
	ns, err := e.SpliceC(Frag{At: e.SpotBody(e.Defn("other")), Src: "return h(2);"})
	if err != nil {
		t.Fatal(err)
	}
	if ns[0][0].Kids[1].Kids[1].Ref() != defn {
		t.Fatalf("other's h refers to %s", Lisp(ns[0][0].Kids[1].Kids[1].Ref()))
	}
	asImported(t, path, e, nil)
	ns, err = e.SpliceC(Frag{At: e.SpotOf(defn), Src: "static int h(int z)\n{\n    return z + 1;\n}"})
	if err != nil {
		t.Fatal(err)
	}
	if One(e.Defn("after"), "(call h 1)").Kids[1].Ref() != ns[0][0] {
		t.Fatal("after's h does not refer to the new definition")
	}
	asImported(t, path, e, nil)
}
