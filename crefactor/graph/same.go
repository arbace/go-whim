package graph

import "fmt"

// SameGraph says a and b are the same graph but for the ids: the same
// forms, every refers edge to corresponding nodes (an external one by its
// name), every typed edge to a type node of the same structure.  It is
// what holds a graph an edit made -- FRAG's above all -- to the import of
// its C view: Equal is identity by id, and an import gives other ids.
func SameGraph(a, b *Graph) error {
	m := map[*Node]*Node{}
	var pair func(x, y *Node) error
	pair = func(x, y *Node) error {
		if x.list != y.list || x.Atom != y.Atom || len(x.Kids) != len(y.Kids) {
			return fmt.Errorf("%s against %s", Lisp(x), Lisp(y))
		}
		m[x] = y
		for i := range x.Kids {
			if err := pair(x.Kids[i], y.Kids[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if len(a.Forms) != len(b.Forms) {
		return fmt.Errorf("%d forms against %d", len(a.Forms), len(b.Forms))
	}
	for i := range a.Forms {
		if err := pair(a.Forms[i], b.Forms[i]); err != nil {
			return err
		}
	}
	bx := map[string]*Node{}
	for _, x := range b.Externs {
		bx[externKey(x)] = x
	}
	for _, x := range a.Externs {
		if y := bx[externKey(x)]; y != nil {
			m[x] = y
			for _, k := range x.Kids {
				if k.Is("member") {
					for _, l := range y.Kids {
						if l.Is("member") && l.Kids[1].Atom == k.Kids[1].Atom {
							m[k] = l
						}
					}
				}
			}
		}
	}
	am, bm := map[*Node]string{}, map[*Node]string{}
	akey := func(t *Node) string {
		return typeKey(t, am, func(x *Node) string { return identOf(m[x]) })
	}
	bkey := func(t *Node) string { return typeKey(t, bm, identOf) }
	var err error
	a.Walk(func(x *Node) bool {
		y := m[x]
		if y == nil || err != nil {
			return err == nil
		}
		if len(x.Refs) != len(y.Refs) {
			err = fmt.Errorf("%s: %d refers edges against %d", label(x), len(x.Refs), len(y.Refs))
			return false
		}
		for i, r := range x.Refs {
			if m[r] != y.Refs[i] {
				err = fmt.Errorf("%s in %s: refers to %s, the import to %s", label(x), Lisp(x), label(r), label(y.Refs[i]))
				return false
			}
		}
		if (x.Type == nil) != (y.Type == nil) {
			err = fmt.Errorf("%s: typed %v against %v", Lisp(x), x.Type != nil, y.Type != nil)
			return false
		}
		if x.Type != nil && akey(x.Type) != bkey(y.Type) {
			err = fmt.Errorf("%s: typed %s against %s", Lisp(x), akey(x.Type), bkey(y.Type))
			return false
		}
		return true
	})
	return err
}
