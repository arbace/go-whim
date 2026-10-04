package graph

import "github.com/arbace/go-whim/crefactor/clisp"

// PATTERNS on the graph's nodes: crefactor/clisp's, the same syntax --
// `_` any node, `_*` the rest of a list, `?name` a binding, which a second
// `?name` must equal as a form, `?name:P` a binding of a node P matches
// -- matched against nodes rather than forms,
// so that a match hands back the nodes themselves, with their ids and
// edges.

// Bindings are what a match bound, by name without its `?`.
type Bindings map[string]*Node

// Match says whether n matches the pattern p (clisp.MustPattern's), and
// what it bound.
func Match(p *clisp.Node, n *Node) (Bindings, bool) {
	var b Bindings
	if !match(p, n, &b) {
		return nil, false
	}
	if b == nil {
		b = Bindings{}
	}
	return b, true
}

// Matches is Match without the bindings.
func Matches(p *clisp.Node, n *Node) bool {
	var b Bindings
	return match(p, n, &b)
}

func match(p *clisp.Node, n *Node, b *Bindings) bool {
	if !p.IsList() {
		switch {
		case p.Atom == "_":
			return true
		case len(p.Atom) > 1 && p.Atom[0] == '?':
			name := p.Atom[1:]
			if old, ok := (*b)[name]; ok {
				return SameForm(old, n)
			}
			if *b == nil {
				*b = Bindings{}
			}
			(*b)[name] = n
			return true
		}
		return !n.list && n.Atom == p.Atom
	}
	if name, sub, ok := clisp.Shaped(p); ok {
		if !match(sub, n, b) {
			return false
		}
		if old, ok := (*b)[name]; ok {
			return SameForm(old, n)
		}
		if *b == nil {
			*b = Bindings{}
		}
		(*b)[name] = n
		return true
	}
	if !n.list {
		return false
	}
	for i, q := range p.List {
		if !q.IsList() && q.Atom == "_*" && i == len(p.List)-1 {
			return true
		}
		if i >= len(n.Kids) || !match(q, n.Kids[i], b) {
			return false
		}
	}
	return len(n.Kids) == len(p.List)
}

// SameForm says a and b are the same form: the same atoms in the same
// lists, whatever their ids and edges.
func SameForm(a, b *Node) bool {
	if a.list != b.list || a.Atom != b.Atom || len(a.Kids) != len(b.Kids) {
		return false
	}
	for i := range a.Kids {
		if !SameForm(a.Kids[i], b.Kids[i]) {
			return false
		}
	}
	return true
}

// Find is every node under root, root included, that f says yes to, in
// order.
func Find(root *Node, f func(*Node) bool) []*Node {
	var out []*Node
	Walk(root, func(n *Node) bool {
		if f(n) {
			out = append(out, n)
		}
		return true
	})
	return out
}

// Contains says the form p matches some node under n.
func Contains(n *Node, p *clisp.Node) bool {
	found := false
	Walk(n, func(x *Node) bool {
		found = found || Matches(p, x)
		return !found
	})
	return found
}
