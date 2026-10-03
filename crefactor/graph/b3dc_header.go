package graph

import (
	"fmt"
	"strings"
)

// DeleteForHeader deletes each of ds, the file's own top-level declaration
// of a name a header also declares -- a prototype the file wrote for a library
// function -- and points each use of it at the header's declaration instead,
// as an import of the text after makes it: the use is made again by FRAG at
// its place, an atom referring to an external node of the name (added if the
// graph has none), typed from the header, the expressions above it typed as
// the import types them.
//
// A use must stand in a top-level form below an include whose header
// declares the name, else the edit is refused before anything changes; or,
// with dangle, such a use is left dangling, for the collection to take with
// the code around it (a function nothing reaches once the cut is made).
// Refused, too, where the file declares the name elsewhere at top level
// (the uses would go to that declaration, not the header's), and where d is
// not a top-level declaration without a body.  The fragments also returns,
// asked after the deletions (a spot is a place in a list as it stands),
// are spliced in the same synthesized unit (a definition
// that calls the header's function, say), so that a phase pays for one.
// The result is the uses made again and the uses left dangling.
func (e *Editor) DeleteForHeader(ds []*Node, dangle bool, also func() []Frag) (rebound, dangling []*Node, err error) {
	for _, d := range ds {
		r, g, err := e.forHeader(d, dangle)
		if err != nil {
			return nil, nil, err
		}
		rebound, dangling = append(rebound, r...), append(dangling, g...)
	}
	for _, d := range ds {
		if err := e.Delete(d); err != nil {
			return nil, nil, fmt.Errorf("delete %s for the header: %v", topName(d), err)
		}
	}
	var more []Frag
	if also != nil {
		more = also()
	}
	if len(rebound) == 0 && len(more) == 0 {
		return nil, dangling, nil
	}
	fs := make([]Frag, 0, len(rebound)+len(more))
	for _, u := range rebound {
		fs = append(fs, Frag{At: e.SpotOf(u), Src: u.Atom})
	}
	made, err := e.SpliceC(append(fs, more...)...)
	if err != nil {
		return nil, nil, fmt.Errorf("delete for the header: %v", err)
	}
	out := make([]*Node, 0, len(rebound))
	for i, u := range rebound {
		ns := made[i]
		if len(ns) != 1 || ns[0].list || len(ns[0].Refs) != 1 || !e.isExtern(ns[0].Refs[0]) || !strings.HasPrefix(ns[0].Refs[0].Head(), "extern") {
			return nil, nil, fmt.Errorf("delete %s for the header: the use made again at #%d is not the header's", u.Atom, u.ID)
		}
		out = append(out, ns[0])
	}
	return out, dangling, nil
}

// forHeader is DeleteForHeader's question of one declaration: its uses
// the header takes, and those left dangling.
func (e *Editor) forHeader(d *Node, dangle bool) (rebound, dangling []*Node, err error) {
	name := topName(d)
	what := fmt.Sprintf("delete #%d for the header", d.ID)
	switch {
	case e.TopForm(d) != d || !d.Is("def"):
		return nil, nil, fmt.Errorf("%s: not a top-level declaration without a body (%s)", what, label(d))
	case name == "":
		return nil, nil, fmt.Errorf("%s: it declares no ordinary name", what)
	}
	what = fmt.Sprintf("delete %s for the header", name)
	for _, f := range e.g.Forms {
		if f != d && topName(f) == name {
			return nil, nil, fmt.Errorf("%s: the file declares it again (#%d, %s), so its uses would not be the header's",
				what, f.ID, label(f))
		}
	}
	first := -1
	for _, inc := range e.Includes() {
		h, err := HeaderOf(IncludeSpec(inc))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", what, err)
		}
		if h.Names[name] {
			first = e.formAt(inc)
			break
		}
	}
	if first < 0 {
		return nil, nil, fmt.Errorf("%s: no include's header declares it", what)
	}
	for _, u := range e.Uses(d) {
		f := e.TopForm(u)
		switch {
		case u.list:
			return nil, nil, fmt.Errorf("%s: a use #%d is not an atom (%s)", what, u.ID, label(u))
		case f != nil && e.formAt(f) > first:
			rebound = append(rebound, u)
		case dangle:
			dangling = append(dangling, u)
		default:
			where := "outside the forms"
			if f != nil {
				where = label(f)
			}
			return nil, nil, fmt.Errorf("%s: a use in %s is above every include that declares it", what, where)
		}
	}
	return rebound, dangling, nil
}
