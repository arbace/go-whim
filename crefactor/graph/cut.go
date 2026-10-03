package graph

import (
	"fmt"
	"strconv"
)

// ---- the locals nothing reads (Prune's unusedLocals)

// A deadLocal is a block's declaration nothing in its function uses.
type deadLocal struct {
	in   *Node // the block, statement expression or function it is an item of
	form *Node
}

// unusedLocals is every local of a live function that no use in it refers
// to, where no use of its name went unresolved.
func (c *collector) unusedLocals() []deadLocal {
	var out []deadLocal
	for _, d := range c.decls {
		if d.fn == nil || !d.fn.live {
			continue
		}
		f := d.form
		uses := map[*Node]int{}
		unresolved := map[string]bool{}
		Walk(f, func(n *Node) bool {
			for _, t := range n.Refs {
				uses[t]++
				if t.Is("undeclared") {
					unresolved[t.Kids[1].Atom] = true
				}
			}
			return true
		})
		Walk(f, func(n *Node) bool {
			var items []*Node
			switch {
			case n == f:
				items = defnBody(f)
			case n.Is("block") || n.Is("stmt-expr"):
				items = n.Kids[1:]
			default:
				return true
			}
			for _, it := range items {
				if !it.Is("def") || !isLocalCandidate(it) {
					continue
				}
				name := it.Kids[defNameAt(it)].Atom
				if uses[it] == 0 && !unresolved[name] {
					out = append(out, deadLocal{in: n, form: it})
				}
			}
			return true
		})
	}
	return out
}

// defnBody is a function definition's items: after its name, its type, its
// attributes and its K&R parameter declarations.
func defnBody(f *Node) []*Node {
	i := defNameAt(f) + 2
	for i < len(f.Kids) && (isAttrForm(f.Kids[i]) || f.Kids[i].Is("kr-params")) {
		i++
	}
	return f.Kids[i:]
}

// isLocalCandidate is Prune's test of a block's declaration: not a typedef,
// not extern, nothing braced in its specifiers, not `__auto_type`'s.
func isLocalCandidate(d *Node) bool {
	if hasPrefix(d, "typedef") || hasPrefix(d, "extern") || hasPrefix(d, "__auto_type") {
		return false
	}
	braced := false
	for _, p := range prefixOf(d) {
		braced = braced || hasBrace(p)
	}
	if t := defType(d); t != nil {
		braced = braced || hasBrace(base(t, nil))
	}
	return !braced
}

// hasBrace says a specifier's C has a `{`.
func hasBrace(n *Node) bool {
	found := false
	Walk(n, func(x *Node) bool {
		if isDefForm(x) || x.Is("stmt-expr") || x.Is("literal") || x.Is("init") {
			found = true
		}
		return !found
	})
	return found
}

// ---- cutting

func (c *collector) count(e *ent) {
	switch e.kind {
	case 'F':
		c.st.Funcs++
	case 'O':
		c.st.Objects++
	case 'P':
		c.st.Protos++
	case 'T':
		c.st.Typedefs++
	case 'S', 'E', 'D':
		c.st.Tags++
	case 'M':
		c.st.Members++
	case 'N':
		c.st.Enumerators++
	}
}

// cut deletes what the closure did not reach, and the dead locals: a
// top-level form, a member, an enumerator (the survivor after a deleted run
// pinned to its value), a local's declaration.  A declaration whose item
// is dead and whose tag lives becomes the tag's definition alone.
func (c *collector) cut(dead []deadLocal, orphans []*Node) error {
	gone := map[*Node]bool{}
	swap := map[*Node]*Node{}
	var inner func(t *ent)
	inner = func(t *ent) {
		switch t.kind {
		case 'S':
			for _, m := range t.members {
				if !m.live {
					c.count(m)
					gone[m.form] = true
					continue
				}
				for _, n := range m.nested {
					if n.live {
						inner(n)
					}
				}
			}
		case 'E':
			deleted := false
			for _, n := range t.members {
				if !n.live {
					c.count(n)
					gone[n.form] = true
					deleted = true
					continue
				}
				if deleted && !n.explicit {
					v := NewAtom(formatValue(n.val))
					n.form.Kids = append(n.form.Kids, v)
					c.st.Pinned++
					if c.rec != nil {
						c.rec.pinned = append(c.rec.pinned, [2]*Node{n.form, v})
					}
				}
				deleted = false
			}
		}
	}
	for _, d := range c.decls {
		if d.keep {
			continue
		}
		if d.fn != nil {
			if !d.fn.live {
				c.count(d.fn)
				gone[d.form] = true
			}
			continue
		}
		liveItems, liveTags := 0, 0
		for _, e := range d.items {
			if e.live {
				liveItems++
			}
		}
		for _, t := range d.tags {
			if t.live || t.kind == 'D' && c.live[t.key] {
				liveTags++
			}
		}
		if liveItems == 0 && liveTags == 0 {
			if len(d.items) == 0 && len(d.tags) == 0 {
				continue
			}
			for _, e := range d.items {
				c.count(e)
			}
			for _, t := range d.tags {
				c.count(t)
			}
			gone[d.form] = true
			continue
		}
		for _, t := range d.tags {
			if t.live {
				inner(t)
			}
		}
		if len(d.items) == 0 || liveItems > 0 {
			continue
		}
		// The tag it defines lives and nothing it declares does: what is
		// left is the definition alone, `struct foo { ... };`.
		t := d.tags[0]
		if t.kind == 'D' {
			return fmt.Errorf("%s: a declaration of a dead item and a live tag it does not define: the text's cut overlaps", DeclLabel(d.form))
		}
		for _, e := range d.items {
			c.count(e)
		}
		swap[d.form] = t.form
	}
	for _, l := range dead {
		c.st.Locals++
		gone[l.form] = true
	}
	for _, o := range orphans {
		c.st.Fallthroughs++
		gone[o] = true
	}
	if c.rec != nil {
		for n := range gone {
			c.rec.gone = append(c.rec.gone, n)
		}
		for f, s := range swap {
			c.rec.swapped = append(c.rec.swapped, [2]*Node{f, s})
		}
	}
	forms := c.g.Forms[:0]
	for _, f := range c.g.Forms {
		if gone[f] {
			continue
		}
		if s, ok := swap[f]; ok {
			f = s
		}
		forms = append(forms, f)
	}
	c.g.Forms = forms
	for _, f := range c.g.Forms {
		Walk(f, func(n *Node) bool {
			if !n.list {
				return false
			}
			kept := n.Kids[:0]
			for _, k := range n.Kids {
				if !gone[k] {
					kept = append(kept, k)
				}
			}
			for i := len(kept); i < len(n.Kids); i++ {
				n.Kids[i] = nil
			}
			n.Kids = kept
			return true
		})
	}
	return nil
}

// DeclLabel is a top-level form, for a message: its head and name.
func DeclLabel(f *Node) string {
	if name := topName(f); name != "" {
		return f.Head() + " " + name
	}
	if t := tagOf(f); t != "" {
		return f.Head() + " " + t
	}
	return f.Head()
}

// formatValue spells a pinned value as Prune does: decimal, hexadecimal
// from 65536 up.
func formatValue(v int64) string {
	if v >= 65536 {
		return "0x" + strconv.FormatInt(v, 16)
	}
	return strconv.FormatInt(v, 10)
}

// unreached drops the type and external nodes nothing in the file reaches
// any more, and says how many of each; rec, when given, records them.
func (g *Graph) unreached(rec *cutRecord) (types, externs int) {
	reached := map[*Node]bool{}
	var mark func(n *Node)
	mark = func(n *Node) {
		if n == nil || reached[n] {
			return
		}
		reached[n] = true
		for _, r := range n.Refs {
			mark(r)
		}
		mark(n.Type)
		for _, k := range n.Kids {
			mark(k)
		}
	}
	for _, f := range g.Forms {
		Walk(f, func(n *Node) bool {
			for _, r := range n.Refs {
				mark(r)
			}
			mark(n.Type)
			return true
		})
	}
	// an external struct lives when a member of it does
	for _, x := range g.Externs {
		for _, k := range x.Kids {
			if reached[k] {
				mark(x)
			}
		}
	}
	keep := func(s []*Node) ([]*Node, int) {
		out := s[:0]
		n := 0
		for _, x := range s {
			if reached[x] {
				out = append(out, x)
			} else {
				n++
				if rec != nil {
					rec.gone = append(rec.gone, x)
				}
			}
		}
		return out, n
	}
	g.Types, types = keep(g.Types)
	g.Externs, externs = keep(g.Externs)
	return types, externs
}
