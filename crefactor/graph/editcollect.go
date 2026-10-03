package graph

import "slices"

// Collect is the collection (Collect) made through the editor, so that a
// caller can edit, collect and edit again on one index: what the
// collection cut goes out of the graph for the editor too -- each cut
// node's container cleared, so that neither it nor anything under it is
// Live -- a definition that took a top-level form's place is placed in the
// file, a pinned enumerator's value in its enumerator, and the act is
// logged, the ids it superseded in order: the record a cache keyed by id
// needs, as for any other edit.  The refers index needs nothing: a use the
// collection took is filtered out on reading, as an edit's are.
//
// Measured against collecting and indexing the graph anew (NewEditor),
// which is what a caller without this would do (doc/GRAPH.md, step 5): the
// fix-up is proportional to what the collection took, the new index to the
// whole graph.
func (e *Editor) Collect(opt CollectOptions) (CollectStats, error) {
	rec := &cutRecord{}
	st, err := collect(e.g, opt, rec)
	act := Act{Op: "collect"}
	kept := map[*Node]bool{}
	for _, s := range rec.swapped {
		form, def := s[0], s[1]
		def.up = e.top[0]
		form.up = nil
		kept[def] = true
		if def.ID != 0 {
			act.Moved = append(act.Moved, def.ID)
		}
	}
	for _, p := range rec.pinned {
		p[1].up = p[0]
	}
	// a form a definition took the place of went too, but for the definition
	cut := rec.gone
	for _, s := range rec.swapped {
		cut = append(cut, s[0])
	}
	roots := map[*Node]bool{}
	for _, n := range cut {
		roots[n] = true
	}
	for _, n := range cut {
		n.up = nil
		Walk(n, func(x *Node) bool {
			if kept[x] || x != n && roots[x] {
				return false
			}
			if x.ID != 0 {
				act.Gone = append(act.Gone, x.ID)
			}
			return true
		})
	}
	slices.Sort(act.Gone)
	slices.Sort(act.Moved)
	e.Log = append(e.Log, act)
	return st, err
}
