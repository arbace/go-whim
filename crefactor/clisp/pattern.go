package clisp

import "fmt"

// PATTERNS.  A pattern is a form written in C-lisp, read once, that matches
// forms the way the text verbs' regular expressions match lines -- but on the
// tree, so that spacing, line breaks and the parentheses C needs are not part
// of the question:
//
//	_        any one node
//	_*       any number of nodes, the rest of a list (last in it only)
//	?name    any one node, bound to name; a second ?name must be equal to it
//
// and any other atom matches itself.  `(= (-> buf ?f) _)` is every assignment
// to a member of buf, the member bound.

// Pattern reads a pattern, which must be one form.
func Pattern(src string) (*Node, error) {
	fs, err := Read([]byte(src))
	if err != nil {
		return nil, err
	}
	if len(fs) != 1 {
		return nil, fmt.Errorf("clisp: a pattern is one form, %q is %d", src, len(fs))
	}
	return fs[0], nil
}

// MustPattern is Pattern, panicking on a pattern that does not read: the
// patterns are the program's own literals.
func MustPattern(src string) *Node {
	p, err := Pattern(src)
	if err != nil {
		panic(err)
	}
	return p
}

// Bindings are what a match bound, by name without its `?`.
type Bindings map[string]*Node

// Match says whether n matches the pattern p, and what it bound.
func Match(p, n *Node) (Bindings, bool) {
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
func Matches(p, n *Node) bool {
	var b Bindings
	return match(p, n, &b)
}

// match binds into *b, made on the first binding: most nodes a pattern is
// tried on fail at their head, and allocate nothing.
func match(p, n *Node, b *Bindings) bool {
	if !p.list {
		switch {
		case p.Atom == "_":
			return true
		case len(p.Atom) > 1 && p.Atom[0] == '?':
			name := p.Atom[1:]
			if old, ok := (*b)[name]; ok {
				return Equal(old, n)
			}
			if *b == nil {
				*b = Bindings{}
			}
			(*b)[name] = n
			return true
		}
		return !n.list && n.Atom == p.Atom
	}
	if !n.list {
		return false
	}
	for i, q := range p.List {
		if !q.list && q.Atom == "_*" && i == len(p.List)-1 {
			return true
		}
		if i >= len(n.List) || !match(q, n.List[i], b) {
			return false
		}
	}
	return len(n.List) == len(p.List)
}

// Subst is the pattern with its ?names replaced by copies of what b binds:
// the form a rewrite puts in a matched form's place.
func Subst(p *Node, b Bindings) *Node {
	if !p.list {
		if len(p.Atom) > 1 && p.Atom[0] == '?' {
			if v, ok := b[p.Atom[1:]]; ok {
				return Clone(v)
			}
		}
		return A(p.Atom)
	}
	out := L()
	for _, x := range p.List {
		out.List = append(out.List, Subst(x, b))
	}
	return out
}

// FindPattern is Find of the nodes matching p.
func FindPattern(root *Node, p *Node) []*Cursor {
	return Find(root, func(n *Node) bool { return Matches(p, n) })
}
