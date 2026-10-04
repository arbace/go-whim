package graph

import (
	"strings"
	"testing"
)

// s6ovAsImported requires the graph to be the import of its C view: every
// edge where the importer puts it, every typed edge of its structure, and
// nothing listed untyped.
func s6ovAsImported(t *testing.T, e *Editor, path string) {
	t.Helper()
	c, err := e.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Import(path, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(e.Graph(), g); err != nil {
		t.Fatalf("not the import of its C view: %v\n%s", err, c)
	}
	if len(e.Untyped) != 0 {
		t.Fatalf("%d untyped: %s", len(e.Untyped), Lisp(e.Untyped[0]))
	}
}

// Phase 76a's shape: a member renamed and one added after it, the rows of
// a table given the element it initialises, a read of the old member pointed
// at the new one, its cast taken off and the types above it derived again
// -- an array member decayed where it is subscripted.
func TestS6ovMemberAndElements(t *testing.T) {
	src := `struct opt { char *name; char *def_val[2]; };
static struct opt opts[] = { {"a", {(char *)1L, (char *)0L}}, {"b", {"x", "y"}}, {0, {0, 0}} };
long f(int i) { return (long)opts[i].def_val[0]; }
int g(int i) { return sizeof opts[i].def_val; }
`
	e, v, path := b2c(t, src)
	m := v.One("(def_val (array 2 (ptr char)))", "def_val")
	if _, err := e.Rename(m, "def_str"); err != nil {
		t.Fatal(err)
	}
	num, err := e.InsertMember(m, true, "(def_num (array 2 long))")
	if err != nil {
		t.Fatal(err)
	}
	if num.Type == nil || !num.Type.Is("array") {
		t.Fatalf("def_num typed %v", num.Type)
	}
	for k, r := range TableInit(v.One("(def static opts _ _)", "opts")).Args() {
		var el *Node
		if k == 0 {
			el = NewList(NewAtom("init"), NewAtom("1L"), NewAtom("0L"))
		} else {
			el = NewList(NewAtom("init"), NewAtom("0L"), NewAtom("0L"))
		}
		if err := e.InsertElements(r, len(r.Args()), el); err != nil {
			t.Fatal(err)
		}
	}
	read := v.One("(cast long (index (. (index opts i) def_str) 0))", "the read")
	sel := read.Args()[1]
	if err := e.Replace(read, sel); err != nil {
		t.Fatal(err)
	}
	s := sel.Args()[0]
	if err := e.RetargetAs(s.Args()[1], 0, num); err != nil {
		t.Fatal(err)
	}
	e.RederiveValue(s)
	want := strings.NewReplacer(
		"char *def_val[2]; };", "char *def_str[2]; long def_num[2]; };",
		`{(char *)1L, (char *)0L}}`, `{(char *)1L, (char *)0L}, {1L, 0L}}`,
		`{"x", "y"}}`, `{"x", "y"}, {0L, 0L}}`,
		`{0, {0, 0}}`, `{0, {0, 0}, {0L, 0L}}`,
		"return (long)opts[i].def_val[0];", "return opts[i].def_num[0];",
		"sizeof opts[i].def_val", "sizeof opts[i].def_str").Replace(src)
	holds(t, e, path, want)
	s6ovAsImported(t, e, path)

	// refused: a name the struct has; a row that would outgrow its struct;
	// a member before an element a positional row holds
	if _, err := e.InsertMember(num, true, "(name int)"); err == nil {
		t.Error("a member named twice: not refused")
	}
	r := TableInit(v.One("(def static opts _ _)", "opts")).Args()[0]
	if err := e.InsertElements(r, len(r.Args()), NewAtom("0")); err == nil {
		t.Error("a row longer than its struct: not refused")
	}
	if _, err := e.InsertMember(m, false, "(early int)"); err == nil {
		t.Error("a member before a positional element: not refused")
	}
}

// An element replaced by an initialiser list is not listed untyped: the
// importer types none.
func TestS6ovReplaceElement(t *testing.T) {
	src := `struct v { int *i; long *l; };
struct o { char *name; struct v var; char *p; };
static int x;
static struct o opts[] = { {"a", {0, 0}, (char *)&x} };
`
	e, v, path := b2c(t, src)
	row := TableInit(v.One("(def static opts _ _)", "opts")).Args()[0]
	old := row.Args()[2]
	if err := e.ReplaceElement(old, NewList(NewAtom("init"), NewAtom("nullptr"))); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, strings.Replace(src, "(char *)&x}", "{nullptr}}", 1))
	if len(e.Untyped) != 0 {
		t.Fatalf("%d untyped", len(e.Untyped))
	}
	if err := e.ReplaceElement(v.One("(def static x int)", "x"), NewAtom("0")); err == nil {
		t.Error("a declaration as an element: not refused")
	}
}
