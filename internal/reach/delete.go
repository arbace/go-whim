package reach

import "strings"

// What a deleting tool needs from the closure beyond the partition: where an
// entity is, who refers to it, and the facts its guards are stated in.  The
// closure itself still deletes nothing; internal/sweep's switch
// (WHIM_CLOSURE=1) asks these and keeps each tool's own deletion code.

// Span is the byte range of the construct that declares e: a member's
// StructDeclaration, an enumerator, a definition, a declaration.  0, 0 when
// the closure placed it nowhere.
func (e *Entity) Span() (int, int) { return e.start, e.end }

// Key is e's identity in the closure: two entities with one key are one.
func (e *Entity) Key() string { return e.key }

// Referrers is every entity that refers to e, reachable or not, in no order.
func (c *Closure) Referrers(e *Entity) []*Entity {
	out := make([]*Entity, 0, len(e.referrers))
	for k := range e.referrers {
		if r := c.byKey[k]; r != nil {
			out = append(out, r)
		}
	}
	return out
}

// Tag is the tagged struct, union or enum definition named tag, or nil.
func (c *Closure) Tag(tag string) *Entity {
	if e := c.byKey["s:"+tag]; e != nil {
		return e
	}
	return c.byKey["e:"+tag]
}

// ReadsSwap says ml_recover is defined: a struct layout is a disk format.
func (c *Closure) ReadsSwap() bool { return c.mlRecover }

// MemberDecl is a member's StructDeclaration span and how many members that
// one declaration declares.
func (c *Closure) MemberDecl(e *Entity) (start, end, count int) {
	d := c.memberDecl[e.key]
	return d[0], d[1], d[2]
}

// Siblings is every member of e's struct or union, e included, in order.
func (c *Closure) Siblings(e *Entity) []*Entity {
	var out []*Entity
	for _, k := range c.members[c.structOf[e.key]] {
		out = append(out, c.byKey[k])
	}
	return out
}

// Unresolved is every name the instrument met and could not place -- a
// designator's member, a member access joined to no member, the target of a
// reference charged to no entity -- as bare names and as IDs.  Nothing so
// named can be shown unreferenced, so a deleting tool refuses it.
func (c *Closure) Unresolved() map[string]bool {
	out := map[string]bool{}
	for _, f := range c.left {
		n := strings.TrimSuffix(strings.TrimPrefix(f.Fn, "."), ":")
		out[n] = true
		if i := strings.IndexByte(n, ':'); i >= 0 {
			out[n[i+1:]] = true
		}
	}
	return out
}
