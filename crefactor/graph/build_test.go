package graph

import (
	"bytes"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

const buildSample = `enum { FALSE, TRUE };
struct buf { struct buf *next; int n; };
typedef struct buf buf_T;
static buf_T *curbuf;
static int total = 0;
int g(int x);
int
f(int a)
{
    int total = a;
    if (a)
    {
        goto out;
    }
    total = g(total);
out:
    return total;
}
int g(int x) { return x; }
`

// Build: each name resolved where the place is -- a parameter, a local
// before the place over a file-scope object of the same name, an
// enumerator, a function, a typedef -- a member by the type selected from,
// a label; the typed edges that follow from the operands, and the forms
// they do not follow from listed in Untyped.
func TestBuild(t *testing.T) {
	_, _, v, _ := verbsOn(t, buildSample)
	e := v.Editor()
	ret := v.One("(return total)", "the return")
	ns, err := e.Build(ret, "(= total (call g (-> curbuf next n))) (if (== a FALSE) (block (goto out))) (cast (ptr buf_T) 0) (+ a 1)", nil)
	if err != nil {
		t.Fatal(err)
	}
	local := v.One("(def total int a)", "int total = a;")
	if local == nil {
		t.Fatal(v.Done())
	}
	store, iff, cast, sum := ns[0], ns[1], ns[2], ns[3]
	call := store.Kids[2]
	sel := call.Kids[2]
	intT := basicType(e.Graph(), []string{"int"})
	for _, c := range []struct {
		what string
		ok   bool
	}{
		{"total is the local, not the file's object", store.Kids[1].Ref() == local},
		{"the store is typed as total is", store.Type == intT},
		{"g is the prototype, the first declaration", call.Kids[1].Ref() == e.FileDecls("g")[0] && e.FileDecls("g")[0].Is("def")},
		{"the call is typed with g's result", call.Type == intT},
		{"next is struct buf's, through buf_T", sel.Kids[2].Ref() != nil && sel.Kids[2].Ref().Kids[0].Atom == "next"},
		{"n is struct buf's, through next's pointer", sel.Kids[3].Ref() != nil && sel.Kids[3].Ref().Kids[0].Atom == "n"},
		{"the selection is n's type", sel.Type == intT},
		{"a is the parameter", iff.Kids[1].Kids[1].Ref() == paramNamed(e.Defn("f"), "a")},
		{"FALSE is the enumerator", iff.Kids[1].Kids[2].Ref() != nil && iff.Kids[1].Kids[2].Ref().Kids[0].Atom == "FALSE"},
		{"the comparison is int", iff.Kids[1].Type == intT},
		{"out is the function's label", iff.Kids[2].Kids[1].Kids[1].Ref().Is("label")},
		{"the cast's buf_T is the typedef", cast.Kids[1].Kids[1].Ref() != nil && cast.Kids[1].Kids[1].Ref().Is("typedef")},
		{"a sum is not typed, and is listed", sum.Type == nil && len(e.Untyped) == 1 && e.Untyped[0] == sum},
	} {
		if !c.ok {
			t.Errorf("%s", c.what)
		}
	}
	if err := e.InsertBefore(ret, ns...); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	for _, n := range ns {
		if n.ID == 0 {
			t.Errorf("%s: no fresh id", Lisp(n))
		}
	}
	out, _ := e.Graph().C()
	if !bytes.Contains(out, []byte("    total = g(curbuf->next->n);\n    if (a == FALSE)\n")) {
		t.Errorf("the C view:\n%s", out)
	}

	// what it refuses: a name declared nowhere visible, a hole twice, a
	// member the type has not, a fragment that is FRAG's
	for _, c := range []struct{ src, refusal string }{
		{"(call nowhere)", "`nowhere` is declared nowhere visible"},
		{"(= x 1)", "`x` is declared nowhere visible"},
		{"(+ ?h ?h)", "used twice"},
		{"(-> curbuf missing)", "no member `missing`"},
		{"(macro \"MAX(a, b)\")", "FRAG's"},
		{"(goto nolabel)", "no label nolabel in f"},
	} {
		_, err := e.Build(ret, c.src, Bindings{"h": NewAtom("1")})
		if err == nil || !strings.Contains(err.Error(), c.refusal) {
			t.Errorf("%s: %v, want %q", c.src, err, c.refusal)
		}
	}
	// a local declared at or after the place is not visible; the file's is
	ns, err = e.Build(local, "total", nil)
	if err != nil || ns[0].Ref() == local || ns[0].Ref() != e.FileDecls("total")[0] {
		t.Errorf("before the local: %v, refers to %s", err, label(ns[0].Ref()))
	}
}

// The constructors, and a built node spliced where a new id is given.
func TestBuildConstructors(t *testing.T) {
	_, _, v, _ := verbsOn(t, buildSample)
	e := v.Editor()
	g := e.FileDecls("g")[0]
	ret := v.One("(return total)", "the return")
	call := e.Call(e.RefTo(g), NewAtom("2"))
	if call.Type != basicType(e.Graph(), []string{"int"}) {
		t.Error("the call is not typed with g's result")
	}
	if err := e.InsertBefore(ret, Void(call), Return(nil), Break()); err != nil {
		t.Fatal(err)
	}
	out, _ := e.Graph().C()
	if !bytes.Contains(out, []byte("    (void)g(2);\n    return;\n    break;\n")) {
		t.Errorf("the C view:\n%s", out)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
}

// Unwrap's clash check: a moved declaration that is declared again where
// it moves, that hides a declaration a later use names, or that shadows a
// parameter, is refused; one whose name a later scope declares anew is
// not.
func TestUnwrapClash(t *testing.T) {
	src := `int glob;
void use(int);
void
f(int p)
{
    int a = 0;
    if (p)
    {
        int a = 1;
        use(a);
    }
    if (p > 1)
    {
        int glob = 2;
        use(glob);
    }
    if (p > 2)
    {
        int p = 3;
        use(p);
    }
    if (p > 3)
    {
        int fresh = 4;
        use(fresh);
    }
    use(glob);
    {
        int fresh = 5;
        use(fresh);
    }
}
`
	for _, c := range []struct{ cond, refusal string }{
		{"p", "`a` is declared again where it would move"},
		{"(> p 1)", "`glob` after it names def glob, which the moved declaration would hide"},
		{"(> p 2)", "`p` is a parameter of f"},
		{"(> p 3)", ""},
	} {
		_, _, v, _ := verbsOn(t, src)
		v.FoldAlways(c.cond, 1, "x")
		err := v.Done()
		switch {
		case c.refusal == "" && err != nil:
			t.Errorf("%s: %v", c.cond, err)
		case c.refusal != "" && (err == nil || !strings.Contains(err.Error(), c.refusal)):
			t.Errorf("%s: %v, want %q", c.cond, err, c.refusal)
		}
	}
}

// The closure's opt-in splice: an if of a constant whose branch declares
// something is spliced with SpliceDeclaring where no name clashes, kept
// whole without it, and kept whole with it where a name clashes -- the
// text's unwrap, byte for byte where it is safe.
func TestFallOutSpliceDeclaring(t *testing.T) {
	src := `void use(int);
int k;
void
f(int p)
{
    if (p)
    {
        int a = 1;
        use(a);
    }
    if (p + 1)
    {
        int k = 2;
        use(k);
    }
    use(k);
}
`
	for _, c := range []struct {
		splice bool
		want   string
	}{
		{false, "    {\n        int a = 1;\n        use(a);\n    }\n    {\n        int k = 2;"},
		{true, "    int a = 1;\n    use(a);\n    {\n        int k = 2;"},
	} {
		_, _, v, _ := verbsOn(t, src)
		e := v.Editor()
		for _, s := range v.Find("(if _*)") {
			if err := e.Replace(s.Kids[1], NewAtom("1")); err != nil {
				t.Fatal(err)
			}
		}
		st, err := e.FallOut(FallOutOptions{SpliceDeclaring: c.splice})
		if err != nil || st.Branches != 2 {
			t.Fatalf("%v: %d branches", err, st.Branches)
		}
		out, _ := e.Graph().C()
		if !bytes.Contains(out, []byte(c.want)) {
			t.Errorf("SpliceDeclaring %v:\n%s", c.splice, out)
		}
		if err := e.Check(); err != nil {
			t.Fatal(err)
		}
	}
	// the spliced result is the text's FoldAlways where it is safe
	sameAsText(t, src, func(s []byte) ([]byte, error) { return edit.FoldAlways(s, edit.Head("if (p)"), 1) },
		func(v *Verbs) { v.FoldAlways("(if p (block (def a _ _) _*))", 1, "x") })
}

// TEXTQ: the text's counts, number for number, on the scope's C view; the
// graph's questions beside them.
func TestTextQueries(t *testing.T) {
	path, canon, v, _ := verbsOn(t, table)
	_ = path
	for _, name := range []string{"nv_op", "nv_error", "c", "always", "nv_cmds"} {
		if got, want := v.Mentions(name), edit.MentionCount(canon, name); got != want {
			t.Errorf("Mentions(%s) = %d, the text's %d", name, got, want)
		}
	}
	v.InFunction("main", func(v *Verbs) {
		if got, want := v.Mentions("nv_cmds"), 1; got != want {
			t.Errorf("in main: %d, want %d", got, want)
		}
		if got := v.TextQuery(`(\w+)\(always`, 1); len(got) != 1 || got[0] != "f" {
			t.Errorf("TextQuery: %q", got)
		}
	})
	if n := len(v.UsesOf("nv_op")); n != 2 {
		t.Errorf("UsesOf(nv_op) = %d, want 2 (the two rows)", n)
	}
	if n := len(v.UsesOutside("always", "main")); n != 0 {
		t.Errorf("UsesOutside(always, main) = %d", n)
	}
	if !v.Says("nv_cmds") || v.Says("nowhere") {
		t.Error("Says")
	}
	if b, err := v.Editor().Before("nv_error", "nv_cmds"); err != nil || !b {
		t.Errorf("Before: %v %v", b, err)
	}
	if _, err := v.Editor().Before("nowhere", "nv_cmds"); err == nil {
		t.Error("Before of nothing: no error")
	}
	_, _, w, _ := verbsOn(t, "const char *s = \"a b\";\nconst char *u = \"x\" \"y\";\nint main(void) { return 'c'; }\n")
	if got := w.Strings(); len(got) != 2 {
		var ss []string
		for _, s := range got {
			ss = append(ss, s.Atom)
		}
		t.Errorf("Strings: %q", ss)
	}
}
