package graph

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// R1's library (doc/GRAPH-MIGRATION.md, *R1 as built*): MARKFOLD, a name's
// lines (FormLines and the declarations it is asked of), and the decayed
// operands BUILD and RETYPE now type as cc's check does.  Each result is
// held to the C a reader writes by hand, printed canonically, and -- after
// the re-check the pipeline runs before a collection -- to the import of
// its C view.

// r1Same requires e's C view to be want printed canonically and, re-checked,
// the import of that C view.
func r1Same(t *testing.T, e *Editor, path, want string) {
	t.Helper()
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	got, err := e.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	if want != "" {
		canon, err := cemit.Canonical(filepath.Join(t.TempDir(), "w.c"), []byte(want))
		if err != nil {
			t.Fatalf("the expected C does not print: %v", err)
		}
		if string(got) != string(canon) {
			t.Fatalf("%s\ngraph:\n%s\nwant:\n%s", firstDiff(got, canon), got, canon)
		}
	}
	e.Recheck()
	g, _, err := Import(path, got)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(e.Graph(), g); err != nil {
		t.Fatalf("not the import of its C view: %v\n%s", err, got)
	}
}

const r1MarkSrc = `enum { FALSE = 0, TRUE = 1 };
enum { DBCS_JPN = 932 };
static int on = 1;
static int dbcs = 0;
int f(void);
int g(void);
void a(void);
void b(void);
int h(int x, int s, int c)
{
    int y, z, w, q, r;
    if (on)
    {
        a();
    }
    else
    {
        b();
    }
    if (x && !on)
    {
        a();
    }
    y = dbcs == DBCS_JPN ? 1 : 2;
    if (f() || on || g())
    {
        b();
    }
    z = x || (on && s);
    w = !dbcs;
    while (dbcs)
    {
        a();
    }
    if (x)
    {
        a();
    }
    else if (on)
    {
        b();
    }
    else
    {
        a();
    }
    if (c)
    {
        q = 1;
    }
    r = on && s;
    q = on && s;
    return (on);
}
`

// TestR1MarkFold folds the constants of a sample as the text simplifier
// did: the groups through `||`, `&&`, `!`, `?:` and a comparison with zero,
// the parentheses the text kept, an impure operand before a deciding
// constant kept, the statements on a constant folded, a right side after a
// braced statement whose block assigns left as the text left it, and a
// keyword's parentheses not taken off the constant they hold.
func TestR1MarkFold(t *testing.T) {
	e, _, path := b2c(t, r1MarkSrc)
	fn := e.Defn("h")
	mf := &MarkFold{Marks: map[*Node]Mark{}, NonZero: func(a string) bool { return strings.HasPrefix(a, "DBCS_") }}
	ns, err := e.Build(Body(fn)[1], "TRUE FALSE", nil)
	if err != nil {
		t.Fatal(err)
	}
	tr, fa := ns[0], ns[1]
	mf.Make = func(k Mark) *Node {
		switch k {
		case MarkTrue:
			return Clone(tr)
		case MarkFalse:
			return Clone(fa)
		}
		return Literal(0)
	}
	for _, name := range []string{"on", "dbcs"} {
		k := MarkTrue
		if name == "dbcs" {
			k = MarkZero
		}
		d := e.FileDecls(name)[0]
		for _, u := range append([]*Node(nil), e.Uses(d)...) {
			n := mf.Make(k)
			if err := e.Replace(u, n); err != nil {
				t.Fatal(err)
			}
			mf.Marks[n] = k
		}
		if err := e.Delete(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.FoldMarks(mf, fn); err != nil {
		t.Fatal(err)
	}
	r1Same(t, e, path, `enum { FALSE = 0, TRUE = 1 };
enum { DBCS_JPN = 932 };
int f(void);
int g(void);
void a(void);
void b(void);
int h(int x, int s, int c)
{
    int y, z, w, q, r;
    a();
    y = 2;
    if (f() || TRUE)
    {
        b();
    }
    z = x || (s);
    w = TRUE;
    if (x)
    {
        a();
    }
    else
    {
        b();
    }
    if (c)
    {
        q = 1;
    }
    r = TRUE && s;
    q = s;
    return (TRUE);
}
`)
	if mf.Folds != 4 {
		t.Fatalf("%d statements folded, expected 4", mf.Folds)
	}
}

// TestR1FormLines: the lines of the forms that say a member, each with the
// function it is in -- the return type's line the file scope's, as a text
// program reading heads at column 0 saw it -- and the declarations a name's
// lines are asked of.
func TestR1FormLines(t *testing.T) {
	e, _, _ := b2c(t, `struct s { int m; int n; };
struct t { int m; };
static int
f(struct s *p, int m)
{
    int k = m;
    return p->m + k;
}
int g(struct t *q) { return q->m; }
int h(struct s *p) { return p->n; }
`)
	ms := e.MemberDecls("m")
	if len(ms) != 2 {
		t.Fatalf("%d members m, expected 2", len(ms))
	}
	ls, err := e.FormLines(e.AndUses(ms[0])...)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range ls {
		if strings.Contains(l.Text, "m") {
			got = append(got, l.Func+"|"+strings.TrimSpace(l.Text))
		}
	}
	want := []string{"|int m;", "f|f(struct s *p, int m)", "f|int k = m;", "f|return p->m + k;"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if ds := e.LocalDecls(e.Defn("f"), "m"); len(ds) != 1 || len(e.Uses(ds[0])) != 1 {
		t.Fatalf("f's parameter m: %d declarations", len(ds))
	}
	if ds := e.LocalDecls(e.Defn("f"), "k"); len(ds) != 1 || !ds[0].Is("def") {
		t.Fatal("f's local k is not found")
	}
	c, err := ItemsC(Body(e.Defn("f"))[:1])
	if err != nil || strings.TrimSpace(c) != "int k = m;" {
		t.Fatalf("ItemsC: %q %v", c, err)
	}
}

// TestR1Decay: BUILD types a subscripted array member, and RETYPE the
// selections above a member given an array type whose size is an
// enumerator, as cc's check types them -- decayed, a pointer the graph did
// not hold made -- so that, re-checked, the graph is the import of its C.
func TestR1Decay(t *testing.T) {
	e, v, path := b2c(t, `enum { N = 4 };
struct e { int x; };
struct s { struct e arr[2]; int one[1]; };
int f(struct s *p) { p->one[0] = 3; return p->arr[1].x; }
`)
	at := v.One("(= (index (-> p one) 0) 3)", "the store")
	if v.Failed() {
		t.Fatal(v.Err)
	}
	ns, err := e.Build(at, "(= (. (index (-> p arr) 0) x) 1)", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.InsertAfter(at, ns...); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Retype(e.MemberDecls("one")[0], "(array N int)"); err != nil {
		t.Fatal(err)
	}
	r1Same(t, e, path, `enum { N = 4 };
struct e { int x; };
struct s { struct e arr[2]; int one[N]; };
int f(struct s *p) { p->one[0] = 3; p->arr[0].x = 1; return p->arr[1].x; }
`)
}
