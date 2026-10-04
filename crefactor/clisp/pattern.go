package clisp

import (
	"fmt"
	"strings"
)

// PATTERNS.  A pattern is a form written in C-lisp, read once, that matches
// forms the way the text verbs' regular expressions match lines -- but on the
// tree, so that spacing, line breaks and the parentheses C needs are not part
// of the question:
//
//	_        any one node
//	_*       any number of nodes, the rest of a list (last in it only)
//	?name    any one node, bound to name; a second ?name must be equal to it
//	?name:P  a node that matches the pattern P, bound to name: P a list,
//	         `(call ?f:(paren _) _*)`, or an atom, `?k:nullptr`
//
// and any other atom matches itself.  `(= (-> buf ?f) _)` is every assignment
// to a member of buf, the member bound.
//
// The reader reads `?name:(...)` as the atom `?name:` and the list after
// it; Pattern makes the two one node, a SHAPED binding: the list
// `(?name: P)`, whose head no form has (doc/C-LISP.md: SPEC.md, *Patterns*).

// Pattern reads a pattern, which must be one form.
func Pattern(src string) (*Node, error) {
	fs, err := Read([]byte(src))
	if err != nil {
		return nil, err
	}
	if fs, err = shapes(fs); err != nil {
		return nil, fmt.Errorf("clisp: the pattern %q: %v", src, err)
	}
	if len(fs) != 1 {
		return nil, fmt.Errorf("clisp: a pattern is one form, %q is %d", src, len(fs))
	}
	return fs[0], nil
}

// shapes makes each `?name:` and the element after it, and each atom
// `?name:P`, the shaped binding `(?name: P)`, in xs and every list below.
func shapes(xs []*Node) ([]*Node, error) {
	var out []*Node
	for i := 0; i < len(xs); i++ {
		x := xs[i]
		if x.list {
			kids, err := shapes(x.List)
			if err != nil {
				return nil, err
			}
			out = append(out, L(kids...))
			continue
		}
		name, rest, ok := shapedAtom(x.Atom)
		if !ok {
			out = append(out, x)
			continue
		}
		var sub *Node
		if rest != "" {
			sub = A(rest)
		} else {
			if i+1 == len(xs) {
				return nil, fmt.Errorf("?%s: has no pattern after it", name)
			}
			i++
			sx, err := shapes([]*Node{xs[i]})
			if err != nil {
				return nil, err
			}
			sub = sx[0]
		}
		if !sub.list && sub.Atom == "_*" {
			return nil, fmt.Errorf("?%s: binds one node, and _* is a run of them", name)
		}
		out = append(out, L(A("?"+name+":"), sub))
	}
	return out, nil
}

// shapedAtom splits `?name:` or `?name:P` into name and P.
func shapedAtom(s string) (name, rest string, ok bool) {
	if len(s) < 3 || s[0] != '?' {
		return "", "", false
	}
	i := strings.IndexByte(s, ':')
	if i < 2 {
		return "", "", false
	}
	return s[1:i], s[i+1:], true
}

// Shaped says p is a shaped binding `(?name: P)`, and gives name and P.
func Shaped(p *Node) (name string, sub *Node, ok bool) {
	if p == nil || !p.list || len(p.List) != 2 || p.List[0].list {
		return "", nil, false
	}
	h := p.List[0].Atom
	if len(h) < 3 || h[0] != '?' || h[len(h)-1] != ':' || strings.IndexByte(h, ':') != len(h)-1 {
		return "", nil, false
	}
	return h[1 : len(h)-1], p.List[1], true
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
	if name, sub, ok := Shaped(p); ok {
		if !match(sub, n, b) {
			return false
		}
		if old, ok := (*b)[name]; ok {
			return Equal(old, n)
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
	if name, sub, ok := Shaped(p); ok {
		if v, ok := b[name]; ok {
			return Clone(v)
		}
		return Subst(sub, b)
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
