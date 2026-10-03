package graph

import "fmt"

// THE DECLARING-BLOCK SPLICE (doc/GRAPH-MIGRATION.md: the byte risk of the
// text's unwrap).  The text verbs that keep a branch -- FoldNever's else,
// FoldAlways's then, a walk folded -- splice the branch's lines into the
// block around it whatever they declare; the closure's if-fold keeps a block
// that declares anything whole.  Unwrap is the splice, made safe by the
// edges: a block's items take its place when NO NAME CLASHES -- none of the
// names its items declare is declared again at the level they move to (a
// redeclaration), or names another declaration in what follows them there
// (which the moved one would then hide).  Either is refused, named.  A block
// whose items declare a struct, union or enum type, or that carries
// attributes, is refused too: the tag's scope would move with it.

// Unwrap puts the items of the block b -- held by old, or old itself -- in
// old's place, refusing a name clash.  Where old's place holds one node (an
// else, a body), b takes it whole.
func (e *Editor) Unwrap(old, b *Node) error { return e.unwrap(old, nil, b) }

// unwrap is Unwrap with the nodes pre put before b's items.
func (e *Editor) unwrap(old *Node, pre []*Node, b *Node) error {
	p, i := e.index(old)
	if i < 0 {
		return fmt.Errorf("unwrap #%d: not in the graph", old.ID)
	}
	if e.place(p, i) != placeItem {
		if len(pre) > 0 {
			return fmt.Errorf("unwrap #%d (%s): its place holds one node", old.ID, label(old))
		}
		return e.Replace(old, b)
	}
	if !b.Is("block") {
		return e.Replace(old, append(pre, b)...)
	}
	if len(b.Kids) > 1 && b.Kids[1].Is("@") {
		return fmt.Errorf("unwrap #%d (%s): the block carries attributes", old.ID, label(old))
	}
	items := append(append([]*Node(nil), pre...), blockItems(b)...)
	if err := e.Clash(p, i, items); err != nil {
		return fmt.Errorf("unwrap #%d (%s): %w", old.ID, label(old), err)
	}
	return e.Replace(old, items...)
}

// Clash says why items cannot take the place of element i of p, a list of
// items (a block, a function's body, a statement expression): a name one of
// them declares that p declares too, or that a use after the place resolves
// to another declaration of; nil when none does.
func (e *Editor) Clash(p *Node, i int, items []*Node) error {
	declared := map[string]*Node{}
	mine := map[*Node]bool{}
	for _, it := range items {
		if !it.Is("def") && !it.Is("typedef") {
			continue
		}
		if t := defType(it); t != nil && definesType(t) {
			return fmt.Errorf("%s declares a type of its own", label(it))
		}
		if name := topName(it); name != "" {
			declared[name] = it
			mine[it] = true
		}
	}
	if len(declared) == 0 {
		return nil
	}
	ks := e.kids(p)
	for j, k := range ks {
		if j == i || e.place(p, j) != placeItem {
			continue
		}
		if (k.Is("def") || k.Is("typedef")) && declared[topName(k)] != nil {
			return fmt.Errorf("`%s` is declared again where it would move (%s)", topName(k), label(k))
		}
	}
	if p.Is("defn") {
		for name := range declared {
			if paramNamed(p, name) != nil {
				return fmt.Errorf("`%s` is a parameter of %s", name, topName(p))
			}
		}
	}
	// what follows: a use of another declaration of a moved name, declared
	// outside what follows (inside it, that one hides the moved one anyway)
	after := map[*Node]bool{}
	for _, k := range ks[i+1:] {
		after[k] = true
	}
	var err error
	for _, k := range ks[i+1:] {
		Walk(k, func(x *Node) bool {
			for _, r := range x.Refs {
				if err != nil {
					return false
				}
				name := ordinaryName(r)
				if name == "" || declared[name] == nil || mine[r] || isMember(e, r) || e.within(r, after) {
					continue
				}
				err = fmt.Errorf("`%s` after it names %s, which the moved declaration would hide", name, label(r))
			}
			return err == nil
		})
	}
	return err
}

// within says n is under one of the nodes of set.
func (e *Editor) within(n *Node, set map[*Node]bool) bool {
	for x := n; x != nil && !e.isTop(x); x = x.up {
		if set[x] {
			return true
		}
	}
	return false
}

// isMember says r is a struct's or union's member: not in the ordinary name
// space.
func isMember(e *Editor, r *Node) bool {
	p := e.Parent(r)
	return p != nil && (p.Is("struct") || p.Is("union"))
}

// definesType says a type form defines a struct, union or enum: a tag or
// enumerators would move with its declaration.
func definesType(t *Node) bool {
	found := false
	Walk(t, func(n *Node) bool {
		found = found || isDefForm(n)
		return !found
	})
	return found
}

// ReplaceRun replaces the items first through last, which one list holds in
// that order, by with: a run of statements made one, or none.
func (e *Editor) ReplaceRun(first, last *Node, with ...*Node) error {
	p, lo := e.index(first)
	q, hi := e.index(last)
	switch {
	case lo < 0 || hi < 0:
		return fmt.Errorf("replace the run #%d..#%d: not in the graph", first.ID, last.ID)
	case p != q:
		return fmt.Errorf("replace the run #%d..#%d: not in one list", first.ID, last.ID)
	case hi < lo:
		return fmt.Errorf("replace the run #%d..#%d: its end comes before its start", first.ID, last.ID)
	}
	for j := lo; j <= hi; j++ {
		if e.place(p, j) != placeItem {
			return fmt.Errorf("replace the run #%d..#%d: #%d (%s) is not an item", first.ID, last.ID, e.kids(p)[j].ID, label(e.kids(p)[j]))
		}
	}
	return e.splice("replace", p, lo, hi+1, with)
}
