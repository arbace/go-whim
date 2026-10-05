package graph

import (
	"fmt"
	"slices"
	"testing"
)

// hashOf hashes g or fails.
func hashOf(t *testing.T, g *Graph, opt HashOptions) *Hashes {
	t.Helper()
	h, err := g.Hash(opt)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// sameHashes says a and b, hashes of graphs with the same forms, give the
// nodes of the forms the same hashes, node by node in Walk's order.
func sameHashes(t *testing.T, what string, ga *Graph, a *Hashes, gb *Graph, b *Hashes) {
	t.Helper()
	var xs, ys []*Node
	for _, f := range ga.Forms {
		Walk(f, func(n *Node) bool { xs = append(xs, n); return true })
	}
	for _, f := range gb.Forms {
		Walk(f, func(n *Node) bool { ys = append(ys, n); return true })
	}
	if len(xs) != len(ys) {
		t.Fatalf("%s: %d nodes against %d", what, len(xs), len(ys))
	}
	for i := range xs {
		hx, okx := a.Hash(xs[i])
		hy, oky := b.Hash(ys[i])
		if okx != oky || hx != hy {
			t.Fatalf("%s: %s hashes %s against %s", what, label(xs[i]), hx.Short(), hy.Short())
		}
	}
}

// renumber gives every node of g a new id, in reverse, and reverses its type
// and external sections: what a graph's ids and section order are must not
// move a hash.
func renumber(g *Graph) {
	var ns []*Node
	g.Walk(func(n *Node) bool {
		if n.ID != 0 {
			ns = append(ns, n)
		}
		return true
	})
	for i, n := range ns {
		n.ID = ID(len(ns) - i + 1000)
	}
	slices.Reverse(g.Types)
	slices.Reverse(g.Externs)
}

// TestHashStable: the same program imported twice, read back from its
// Lisp, and renumbered, gives every node the same hash.
func TestHashStable(t *testing.T) {
	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			_, _, g := importSample(t, src)
			_, _, g2 := importSample(t, src)
			h, err := Read(g.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			hg := hashOf(t, g, HashOptions{})
			sameHashes(t, "imported twice", g, hg, g2, hashOf(t, g2, HashOptions{}))
			sameHashes(t, "read back", g, hg, h, hashOf(t, h, HashOptions{}))
			renumber(h)
			sameHashes(t, "renumbered", g, hg, h, hashOf(t, h, HashOptions{}))
			// the index the hashing keeps, by id where it can: lists with
			// no id and an id two nodes share (found by the node), and ids
			// too sparse for a slice
			for _, sparse := range []bool{false, true} {
				h, _ := Read(g.Lisp())
				i := 0
				h.Walk(func(n *Node) bool {
					i++
					switch {
					case sparse && n.ID != 0:
						n.ID = ID(1<<30 + i)
					case !sparse && n.list && i%3 == 0:
						n.ID = 0
					case !sparse && n.list && i%3 == 1:
						n.ID = 7
					}
					return true
				})
				sameHashes(t, fmt.Sprintf("ids spoilt, sparse %v", sparse), g, hg, h, hashOf(t, h, HashOptions{}))
			}
			if len(hg.Nodes) != g.Count().Nodes {
				t.Errorf("%d nodes hashed of %d", len(hg.Nodes), g.Count().Nodes)
			}
		})
	}
}

const hashSample = `
struct S { int a; long b; };
static int f(int x) { int a = x + 1; int c = 4; return a * 2 + c; }
static int g(void) { return f(3); }
static int p(int y);
static int q(void) { return p(2); }
static int p(int y) { return y + 1; }
static int h(void) { return 7; }
static int k(struct S *s) { return s->a; }
int main(void) { struct S s = {0}; return g() + h() + k(&s) + q(); }
`

// changed is the oracle: the nodes a change at the nodes `at` must move --
// those, the lists above them, and whatever names a moved node (an element,
// a refers edge, a typed edge, an edge of a token it holds), to a fixpoint.
// It reads only the containment and the edges, not the hash.
func changed(g *Graph, at []*Node, opt HashOptions) map[*Node]bool {
	moved := map[*Node]bool{}
	for _, n := range at {
		moved[n] = true
	}
	names := func(n *Node) bool {
		for _, r := range n.Refs {
			if moved[r] {
				return true
			}
		}
		return n.Type != nil && !opt.Untyped && moved[n.Type]
	}
	for grew := true; grew; {
		grew = false
		g.Walk(func(n *Node) bool {
			if !hashed(n) || moved[n] {
				return true
			}
			hit := names(n)
			for _, k := range n.Kids {
				if hit {
					break
				}
				if hashed(k) {
					hit = moved[k]
				} else {
					hit = names(k)
				}
			}
			if hit {
				moved[n] = true
				grew = true
			}
			return true
		})
	}
	return moved
}

// defn is the top-level form defining name.
func defn(g *Graph, name string) *Node {
	for _, f := range g.Forms {
		if f.Is("defn") && topName(f) == name || f.Is("struct") && len(f.Kids) > 2 && f.Kids[1].Atom == name {
			return f
		}
	}
	return nil
}

// TestHashChange: one token changed moves exactly the oracle's nodes --
// up the tree and through the edges -- and none else.
func TestHashChange(t *testing.T) {
	for _, c := range []struct {
		name     string
		token    string // the token changed, in the form named
		in       string
		to       string
		moved    []string // top-level forms that must move
		kept     []string // and must not
		opt      HashOptions
		untyped  bool
		wantSome int
	}{
		{name: "a constant in a body", token: "4", in: "f", to: "5",
			moved: []string{"f", "g", "main"}, kept: []string{"h", "k", "p", "q", "S"}},
		{name: "a body behind a prototype", token: "1", in: "p", to: "2",
			moved: []string{"p"}, kept: []string{"q", "main", "f", "g", "h", "k", "S"}},
		{name: "a member's type", token: "long", in: "S", to: "short",
			moved: []string{"S", "k", "main"}, kept: []string{"f", "g", "h", "p", "q"}},
		{name: "a member's type, untyped", token: "long", in: "S", to: "short", opt: HashOptions{Untyped: true},
			moved: []string{"S", "k", "main"}, kept: []string{"f", "g", "h", "p", "q"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, g := importSample(t, hashSample)
			g, err := Read(g.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			before := hashOf(t, g, c.opt)
			form := defn(g, c.in)
			if form == nil {
				t.Fatalf("no %s", c.in)
			}
			var tok, holder *Node
			Walk(form, func(n *Node) bool {
				for _, k := range n.Kids {
					if tok == nil && !hashed(k) && k.Atom == c.token {
						tok, holder = k, n
					}
				}
				return tok == nil
			})
			if tok == nil {
				t.Fatalf("no token %s in %s", c.token, c.in)
			}
			tok.Atom = c.to
			after := hashOf(t, g, c.opt)
			want := changed(g, []*Node{holder}, c.opt)
			got := map[*Node]bool{}
			for i, n := range before.Nodes {
				if before.Of[i] != after.Of[i] {
					got[n] = true
				}
			}
			for n := range want {
				if !got[n] {
					t.Errorf("%s should have moved", label(n))
				}
			}
			for n := range got {
				if !want[n] {
					t.Errorf("%s moved", label(n))
				}
			}
			for _, name := range c.moved {
				if !got[defn(g, name)] {
					t.Errorf("%s kept its hash", name)
				}
			}
			for _, name := range c.kept {
				if got[defn(g, name)] {
					t.Errorf("%s moved", name)
				}
			}
			t.Logf("%d of %d nodes moved", len(got), len(before.Nodes))
		})
	}
}

// TestHashCycles: a function calling itself, and two structs pointing at
// each other, are hashed as components; each member a hash of its own;
// the same cycle in another program, in the other order, the same hashes;
// and a change in one member moves them all.
func TestHashCycles(t *testing.T) {
	rec := `
static int fact(int n) { return n ? n * fact(n - 1) : 1; }
int main(void) { return fact(3); }
`
	_, _, g := importSample(t, rec)
	h := hashOf(t, g, HashOptions{})
	if h.Cyclic == 0 {
		t.Fatal("fact's recursion is no cycle")
	}
	f := defn(g, "fact")
	fh, _ := h.Hash(f)
	seen := map[Hash]bool{}
	Walk(f, func(n *Node) bool {
		if x, ok := h.Hash(n); ok {
			seen[x] = true
		}
		return true
	})
	t.Logf("fact: %d components, %d cyclic (%d nodes, the largest %d); fact %s", h.Components, h.Cyclic, h.InCycles, h.Largest, fh.Short())

	ab := `
struct A;
struct B;
struct A { struct B *b; int x; };
struct B { struct A *a; };
int main(void) { struct A v; struct B w; v.b = &w; w.a = &v; return v.x; }
`
	ba := `
static int unrelated(void) { return 1; }
struct A;
struct B;
struct B { struct A *a; };
struct A { struct B *b; int x; };
int main(void) { struct A v; struct B w; v.b = &w; w.a = &v; return v.x + unrelated(); }
`
	_, _, g1 := importSample(t, ab)
	_, _, g2 := importSample(t, ba)
	h1, h2 := hashOf(t, g1, HashOptions{}), hashOf(t, g2, HashOptions{})
	a1, _ := h1.Hash(defn(g1, "A"))
	b1, _ := h1.Hash(defn(g1, "B"))
	a2, _ := h2.Hash(defn(g2, "A"))
	b2, _ := h2.Hash(defn(g2, "B"))
	if a1 == b1 {
		t.Error("A and B, one cycle, one hash")
	}
	if a1 != a2 || b1 != b2 {
		t.Errorf("the cycle in another order, other hashes: A %s %s, B %s %s", a1.Short(), a2.Short(), b1.Short(), b2.Short())
	}
	if h1.Cyclic == 0 || h1.Largest < 2 {
		t.Errorf("A and B are no cycle: %d cyclic, the largest %d", h1.Cyclic, h1.Largest)
	}
	// a member of B changes: A moves too
	Walk(defn(g1, "B"), func(n *Node) bool {
		if !n.list && n.Atom == "a" {
			n.Atom = "aa"
		}
		return true
	})
	h3 := hashOf(t, g1, HashOptions{})
	if a3, _ := h3.Hash(defn(g1, "A")); a3 == a1 {
		t.Error("B changed and A, in its cycle, kept its hash")
	}
}
