package graph

import "fmt"

// WHAT CHANGED BETWEEN TWO GRAPHS, by their content hashes (hash.go): the
// top-level forms and external nodes of a that b does not hold, those of b
// a does not, and those both hold whose hash moved.  A form of a is the
// same form of b when it has the same id -- ids carry from phase to phase
// -- or else, among those left, the same label (DeclLabel's head and name),
// in the order the files hold them: a graph imported anew has other ids.
// The type nodes are left out: a type that changes moves what it types.
//
// With HashOptions.Nominal an edge to a type's definition is its name, so
// a struct that loses a member changes itself and not every form naming it.

// A Change is one top-level node: its label, and the node in a (nil when
// added) and in b (nil when removed).
type Change struct {
	Label  string
	Before *Node
	After  *Node
}

// Changes are what moved, in each list in b's order (Removed in a's).
type Changes struct {
	Added, Removed, Changed []Change
	Kept                    int // the nodes both hold with the same hash
}

// Compare says what changed from a to b.
func Compare(a, b *Graph, opt HashOptions) (*Changes, error) {
	ha, err := a.Hash(opt)
	if err != nil {
		return nil, err
	}
	hb, err := b.Hash(opt)
	if err != nil {
		return nil, err
	}
	units := func(g *Graph) []*Node { return append(append([]*Node(nil), g.Forms...), g.Externs...) }
	ua, ub := units(a), units(b)
	byID := map[ID]*Node{}
	for _, n := range ua {
		if n.ID != 0 {
			byID[n.ID] = n
		}
	}
	paired := map[*Node]*Node{} // b's node: a's
	used := map[*Node]bool{}
	for _, n := range ub {
		if m := byID[n.ID]; n.ID != 0 && m != nil && UnitLabel(m) == UnitLabel(n) {
			paired[n], used[m] = m, true
		}
	}
	byLabel := map[string][]*Node{}
	for _, m := range ua {
		if !used[m] {
			byLabel[UnitLabel(m)] = append(byLabel[UnitLabel(m)], m)
		}
	}
	for _, n := range ub {
		if paired[n] != nil {
			continue
		}
		if q := byLabel[UnitLabel(n)]; len(q) > 0 {
			paired[n], used[q[0]] = q[0], true
			byLabel[UnitLabel(n)] = q[1:]
		}
	}
	c := &Changes{}
	for _, m := range ua {
		if !used[m] {
			c.Removed = append(c.Removed, Change{Label: UnitLabel(m), Before: m})
		}
	}
	for _, n := range ub {
		m := paired[n]
		if m == nil {
			c.Added = append(c.Added, Change{Label: UnitLabel(n), After: n})
			continue
		}
		x, okA := ha.Hash(m)
		y, okB := hb.Hash(n)
		if !okA || !okB {
			return nil, fmt.Errorf("compare: %s has no hash", UnitLabel(n))
		}
		if x != y {
			c.Changed = append(c.Changed, Change{Label: UnitLabel(n), Before: m, After: n})
		} else {
			c.Kept++
		}
	}
	return c, nil
}

// UnitLabel is a top-level node for a report: DeclLabel's head and name, an
// include its header, an unnamed enum its first enumerator, an external
// node its name.
func UnitLabel(f *Node) string {
	l := DeclLabel(f)
	if l != f.Head() || len(f.Kids) < 2 {
		return l
	}
	switch {
	case f.Is("enum"):
		for _, k := range f.Kids[1:] {
			if k.list && k.Head() != ":" && k.Head() != "" {
				return l + " " + k.Head()
			}
		}
	case !f.Kids[1].list:
		return l + " " + f.Kids[1].Atom
	}
	return l
}
