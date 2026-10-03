package graph

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const editSample = `
struct opt { int f; char *s; int keep; };
enum { A, B, C = 5, D };
static struct opt *cur;
static int counter = 0;
static int flag = 0;
static void nothing(int x);
static void nothing(int x) { }
static char *strsave(const char *s) { return (char *)s; }
static void clear(char **p) { *p = 0; }
static int get(int c)
{
    switch (c) {
    case 1:
        return cur->f;
    default:
        break;
    }
    return 0;
}
int main(void)
{
    int ready = 1;
    nothing(1);
    (void)nothing(2);
    ++counter;
    cur->s = strsave("x");
    clear(&cur->s);
    if (ready) { nothing(3); }
    if (flag) { cur->keep = 1; } else { cur->keep = A + D; }
    while (flag) { cur->keep++; }
    return get(1) + (flag ? 4 : 5);
}
`

// editGraph is the sample's graph, read back from its Lisp: no cc node is
// behind what the edits see.
func editGraph(t *testing.T) (string, *Graph, *Editor) {
	t.Helper()
	path, _, g := importSample(t, editSample)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return path, h, NewEditor(h)
}

func find(g *Graph, f func(*Node) bool) *Node {
	var out *Node
	g.Walk(func(n *Node) bool {
		if out == nil && f(n) {
			out = n
		}
		return out == nil
	})
	return out
}

func member(g *Graph, name string) *Node {
	return find(g, func(n *Node) bool {
		return n.list && len(n.Kids) == 2 && !n.Kids[0].list && n.Kids[0].Atom == name && n.Kids[1].Atom == "int" ||
			n.list && len(n.Kids) == 2 && n.Kids[0].Atom == name && n.Kids[1].Is("ptr")
	})
}

// The places: what a container lets go, and what it refuses, named.
func TestEditPlaces(t *testing.T) {
	_, g, e := editGraph(t)
	iff := find(g, func(n *Node) bool { return n.Is("if") && n.Kids[1].Atom == "ready" })
	call := find(g, func(n *Node) bool { return n.Is("call") && n.Kids[2].Atom == "1" && n.Kids[1].Atom == "nothing" })
	for _, c := range []struct {
		what string
		err  error
	}{
		{"an if's condition", e.Delete(iff.Kids[1])},
		{"an if's body", e.Delete(iff.Kids[2])},
		{"a call's callee", e.Delete(call.Kids[1])},
		{"a function's parameters", e.Delete(e.Defn("get").Kids[3].Kids[1])},
		{"a type node", e.Delete(g.Types[0])},
		{"an enumerator before an implicit one", e.Delete(find(g, func(n *Node) bool { return n.list && len(n.Kids) == 1 && n.Kids[0].Atom == "A" }))},
		{"a statement in an expression's place", e.Replace(call.Kids[2], NewList(NewAtom("return")))},
		{"a body replaced by a non-block", e.Replace(iff.Kids[2], NewAtom("x"))},
		{"a node held elsewhere", e.Replace(iff.Kids[1], call.Kids[2])},
		{"a refers edge to nothing the graph holds", e.Replace(iff.Kids[1], &Node{Atom: "ghost", Refs: []*Node{NewList(NewAtom("def"))}})},
		{"a use retargeted to another name", e.Retarget(iff.Kids[1], 0, e.FileDecls("counter")[0])},
	} {
		if c.err == nil {
			t.Errorf("%s: not refused", c.what)
		} else {
			t.Logf("%s: %v", c.what, c.err)
		}
	}
	if err := e.Check(); err != nil {
		t.Fatalf("after the refusals: %v", err)
	}
	// what is allowed: an item, an else, a member, a top-level form
	if err := e.Delete(e.Item(call)); err != nil {
		t.Errorf("an item: %v", err)
	}
	other := find(g, func(n *Node) bool { return n.Is("if") && n.Kids[1].Atom == "flag" })
	if err := e.Delete(other.Kids[3]); err != nil {
		t.Errorf("an else: %v", err)
	}
	if err := e.Delete(member(g, "keep")); err != nil {
		t.Errorf("a member: %v", err)
	}
	if n := len(e.Dangling()); n != 2 {
		t.Errorf("the member's two uses left dangle: %d records", n)
	}
	if err := e.Check(); err != nil {
		t.Fatalf("after the edits: %v", err)
	}
}

// The id rule: a moved node keeps its id, a removed one's is superseded
// and never given again, a new node takes a fresh one; and the types above
// a replacement.
func TestEditIDs(t *testing.T) {
	_, g, e := editGraph(t)
	maxID := ID(0)
	g.Walk(func(n *Node) bool { maxID = max(maxID, n.ID); return true })
	iff := find(g, func(n *Node) bool { return n.Is("if") && n.Kids[1].Atom == "flag" })
	keep := blockItems(iff.Kids[3])[0] // cur->keep = A + D; moved out of the else
	keepID, ifID := keep.ID, iff.ID
	ne := NewList(NewAtom("="), NewAtom("x"), NewAtom("1"))
	if err := e.Replace(iff, keep, ne); err != nil {
		t.Fatal(err)
	}
	act := e.Log[len(e.Log)-1]
	if keep.ID != keepID || !containsID(act.Moved, keepID) || !containsID(act.Gone, ifID) || ne.ID <= maxID || !containsID(act.New, ne.ID) {
		t.Errorf("ids: kept %d (was %d), the act %+v, new #%d", keep.ID, keepID, act, ne.ID)
	}
	if err := e.Replace(e.Item(keep), iff); err == nil {
		t.Error("a removed node brought back with its id: not refused")
	}
	// A + D (int) replaced by 6: the same type, the assignment keeps its own
	sum := keep.Kids[2]
	if err := e.Replace(sum, NewAtom("6")); err != nil {
		t.Fatal(err)
	}
	if keep.Type == nil || len(e.Untyped) != 0 {
		t.Errorf("a decimal int for an int: the store's type kept (%v), %d untyped", keep.Type != nil, len(e.Untyped))
	}
	// 6 replaced by a new expression no one typed: what is above it is unknown
	if err := e.Replace(keep.Kids[2], NewList(NewAtom("+"), NewAtom("x"), NewAtom("y"))); err != nil {
		t.Fatal(err)
	}
	if keep.Type != nil || len(e.Untyped) != 2 {
		t.Errorf("an untyped expression: the store's type cleared (%v), %d untyped, want 2", keep.Type == nil, len(e.Untyped))
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
}

func containsID(ids []ID, id ID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// The closure: each generic rule, then the collection, on the graph read
// back; the result held to the sweep and print of the C written by hand.
func TestFallOut(t *testing.T) {
	path, g, e := editGraph(t)
	for _, name := range []string{"nothing", "counter", "flag"} {
		for _, d := range e.FileDecls(name) {
			if err := e.Delete(d); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := e.Delete(member(g, "s")); err != nil {
		t.Fatal(err)
	}
	st, err := e.FallOut(FallOutOptions{Pure: []string{"strsave"}, Through: []string{"clear"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	rules := map[string]int{}
	for _, r := range st.Removed {
		rules[r.Rule]++
	}
	if rules["call"] != 3 || rules["store"] != 2 || rules["through"] != 1 || rules["empty"] != 1 || st.Values != 3 || st.Branches != 3 {
		t.Errorf("the rules: %v, %d values, %d folded, %d branches", rules, st.Values, st.Folded, st.Branches)
	}
	if _, err := Collect(g, testCollect); err != nil {
		t.Fatal(err)
	}
	got, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	want := oracle(t, path, []byte(`
struct opt { int f; int keep; };
enum { A, B, C = 5, D };
static struct opt *cur;
static int get(int c)
{
    switch (c) {
    case 1:
        return cur->f;
    default:
        break;
    }
    return 0;
}
int main(void)
{
    cur->keep = A + D;
    return get(1) + 5;
}
`))
	if !bytes.Equal(got, want) {
		t.Fatalf("%s\ngot:\n%s\nwant:\n%s", firstDiff(got, want), got, want)
	}
	if c := g.Count(); c.Dangling != 0 {
		t.Errorf("%d edges dangle", c.Dangling)
	}
}

// What the closure refuses, named: a call whose value is used, a reader of
// a deleted member; and a block it empties, left when told.
func TestFallOutRefuses(t *testing.T) {
	_, _, e := editGraph(t)
	for _, d := range e.FileDecls("get") {
		if err := e.Delete(d); err != nil {
			t.Fatal(err)
		}
	}
	_, err := e.FallOut(FallOutOptions{})
	var u *Unhandled
	if !errors.As(err, &u) || !strings.Contains(err.Error(), "`get` in main") {
		t.Errorf("a call whose value is used: %v", err)
	}
	_, g, e := editGraph(t)
	if err := e.Delete(member(g, "f")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.FallOut(FallOutOptions{}); !errors.As(err, &u) || !strings.Contains(err.Error(), "`f` in get") {
		t.Errorf("a reader: %v", err)
	}
	_, _, e = editGraph(t)
	for _, d := range e.FileDecls("nothing") {
		if err := e.Delete(d); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.FallOut(FallOutOptions{KeepEmpty: true}); err != nil {
		t.Fatal(err)
	}
	out, _ := e.Graph().C()
	if !bytes.Contains(out, []byte("if (ready)\n    {\n    }")) {
		t.Errorf("KeepEmpty: the emptied block is not left:\n%s", out)
	}
}

// Insertion: new items take fresh ids where items are, and nowhere else;
// a refers edge from a new node is indexed, so that deleting its target
// leaves it dangling.
func TestEditInsert(t *testing.T) {
	_, g, e := editGraph(t)
	ret := find(g, func(n *Node) bool { return n.Is("return") && n.Kids[1].Is("+") })
	counter := e.FileDecls("counter")[0]
	use := &Node{Atom: "counter", Refs: []*Node{counter}}
	st := NewList(NewAtom("pre++"), use)
	if err := e.InsertBefore(ret, st); err != nil {
		t.Fatal(err)
	}
	if st.ID == 0 || use.ID == 0 || e.Sibling(ret, -1) != st {
		t.Errorf("inserted: #%d, #%d", st.ID, use.ID)
	}
	if err := e.InsertAfter(ret.Kids[1], NewAtom("x")); err == nil {
		t.Error("an insertion into an expression: not refused")
	}
	if err := e.InsertAfter(g.Forms[len(g.Forms)-1], NewList(NewAtom("def"), NewAtom("y"), NewAtom("int"))); err != nil {
		t.Errorf("a top-level form: %v", err)
	}
	if err := e.Delete(counter); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range e.Dangling() {
		found = found || d.Use == use
	}
	if !found {
		t.Error("the inserted use of a deleted object does not dangle")
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	out, _ := g.C()
	if !bytes.Contains(out, []byte("++counter;\n    return")) || !bytes.HasSuffix(out, []byte("int y;\n")) {
		t.Errorf("the C view:\n%s", out)
	}
}
