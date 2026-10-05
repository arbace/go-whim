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
	ua, err := a.UnitHashes(opt)
	if err != nil {
		return nil, err
	}
	ub, err := b.UnitHashes(opt)
	if err != nil {
		return nil, err
	}
	na, nb := units(a), units(b)
	u := CompareUnits(ua, ub)
	c := &Changes{Kept: u.Kept}
	for _, i := range u.Removed {
		c.Removed = append(c.Removed, Change{Label: ua[i].Label, Before: na[i]})
	}
	for _, i := range u.Added {
		c.Added = append(c.Added, Change{Label: ub[i].Label, After: nb[i]})
	}
	for _, p := range u.Changed {
		c.Changed = append(c.Changed, Change{Label: ub[p[1]].Label, Before: na[p[0]], After: nb[p[1]]})
	}
	return c, nil
}

// units are the nodes Compare compares: the top-level forms, then the
// external nodes.
func units(g *Graph) []*Node { return append(append([]*Node(nil), g.Forms...), g.Externs...) }

// A UnitHash is a top-level form or external node as Compare sees it,
// apart from the graph: its id, its label (UnitLabel) and its hash.  A
// graph's are what it was once an editor has moved on: the build log keeps
// a boundary's, and compares the next boundary's with them.
type UnitHash struct {
	ID    ID
	Label string
	Hash  Hash
}

// UnitHashes are g's top-level forms and external nodes, in that order,
// each with its hash under opt.
func (g *Graph) UnitHashes(opt HashOptions) ([]UnitHash, error) {
	hs, err := g.Hash(opt)
	if err != nil {
		return nil, err
	}
	ns := units(g)
	us := make([]UnitHash, len(ns))
	for i, n := range ns {
		h, ok := hs.Hash(n)
		if !ok {
			return nil, fmt.Errorf("compare: %s has no hash", UnitLabel(n))
		}
		us[i] = UnitHash{ID: n.ID, Label: UnitLabel(n), Hash: h}
	}
	return us, nil
}

// UnitChanges are Changes by position: Added and Changed's second in b,
// Removed and Changed's first in a, each list in b's order (Removed in
// a's).
type UnitChanges struct {
	Added, Removed []int
	Changed        [][2]int
	Kept           int
}

// CompareUnits is Compare on two graphs' UnitHashes: a unit of a is the
// same unit of b when it has the same id and label, or else, among those
// left, the same label, in the order the files hold them.
func CompareUnits(a, b []UnitHash) *UnitChanges {
	byID := map[ID]int{}
	for i, x := range a {
		if x.ID != 0 {
			byID[x.ID] = i
		}
	}
	paired := make([]int, len(b)) // b's unit: a's, +1; 0 for none
	used := make([]bool, len(a))
	for j, y := range b {
		if i, ok := byID[y.ID]; y.ID != 0 && ok && a[i].Label == y.Label {
			paired[j], used[i] = i+1, true
		}
	}
	byLabel := map[string][]int{}
	for i, x := range a {
		if !used[i] {
			byLabel[x.Label] = append(byLabel[x.Label], i)
		}
	}
	for j, y := range b {
		if paired[j] != 0 {
			continue
		}
		if q := byLabel[y.Label]; len(q) > 0 {
			paired[j], used[q[0]] = q[0]+1, true
			byLabel[y.Label] = q[1:]
		}
	}
	c := &UnitChanges{}
	for i := range a {
		if !used[i] {
			c.Removed = append(c.Removed, i)
		}
	}
	for j, y := range b {
		i := paired[j] - 1
		switch {
		case i < 0:
			c.Added = append(c.Added, j)
		case a[i].Hash != y.Hash:
			c.Changed = append(c.Changed, [2]int{i, j})
		default:
			c.Kept++
		}
	}
	return c
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
