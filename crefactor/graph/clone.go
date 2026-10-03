package graph

// CLONE (doc/GRAPH-MIGRATION.md, B2a): a node used twice -- a format
// argument passed twice, a goto's tail copied over each goto, a macro's
// operand written twice in its expansion -- is a copy the second time,
// since a node is contained once.

// Clone is a copy of n and everything it contains, with no ids: the edit
// that puts it in the graph gives it fresh ones, as it gives a built node.
// Its edges are n's: a refers or typed edge to a node outside n goes where
// n's goes (the same declaration, the same type node), and one to a node
// inside n -- a use of a local n declares, a goto to a label n holds, an
// expression typed with a struct n defines -- goes to that node's copy.
func Clone(n *Node) *Node {
	copies := map[*Node]*Node{}
	var cp func(x *Node) *Node
	cp = func(x *Node) *Node {
		y := &Node{Atom: x.Atom, list: x.list, Type: x.Type}
		copies[x] = y
		if x.list {
			y.Kids = make([]*Node, len(x.Kids))
			for i, k := range x.Kids {
				y.Kids[i] = cp(k)
			}
		}
		return y
	}
	c := cp(n)
	var edges func(x *Node)
	edges = func(x *Node) {
		y := copies[x]
		if len(x.Refs) > 0 {
			y.Refs = make([]*Node, len(x.Refs))
			for i, r := range x.Refs {
				if c, ok := copies[r]; ok {
					r = c
				}
				y.Refs[i] = r
			}
		}
		if c, ok := copies[x.Type]; ok {
			y.Type = c
		}
		for _, k := range x.Kids {
			edges(k)
		}
	}
	edges(n)
	return c
}
