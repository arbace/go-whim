package view

import (
	"github.com/arbace/go-whim/crefactor/graph"
)

// AT THE CURSOR, AND UNDO (doc/GRAPH.md, *The view at the cursor, and
// undo*): what an editor over the views needs past the first gate.  A
// position in a printed view names a node through the span table; the
// entity that node is about is the root of the view an editor opens there,
// built in milliseconds from the index.  And a session's edits are made on
// its graph in place, each under a journal, and undone the other way round,
// with no copy of the graph kept.

// Cursor is the span at byte pos of a view's text: the innermost form or
// name holding it, else the innermost context or entry; -1 for none (a
// note, the space between entries).
func Cursor(spans []Span, pos int) int {
	best := -1
	for i, s := range spans {
		if s.Node == nil || pos < s.Start || pos >= s.End {
			continue
		}
		if best < 0 || better(s, spans[best]) {
			best = i
		}
	}
	return best
}

// better: a form or a name before a context or an entry, then the
// narrower.
func better(s, b Span) bool {
	fs, fb := s.Kind == SpanForm || s.Kind == SpanName, b.Kind == SpanForm || b.Kind == SpanName
	if fs != fb {
		return fs
	}
	return s.Start >= b.Start && s.End <= b.End
}

// Entity is what n is about, the root of the view at a cursor on it: the
// entity a use refers to (a call's callee for the call), a declaration's
// own entity, else the nearest of these around n -- a statement's
// function, a member's struct.  A top-level entity is its Rep.
func (ix *Index) Entity(n *graph.Node) *graph.Node {
	for x := n; x != nil; x = ix.Parent(x) {
		if t := referent(x); t != nil {
			return ix.Rep(t)
		}
		if x.Is("call") && len(x.Kids) > 1 {
			if t := referent(x.Kids[1]); t != nil {
				return ix.Rep(t)
			}
		}
		if x.ID != 0 && (ix.IsTop(x) || graph.DeclName(x) != "" || isMemberNode(ix, x)) {
			return ix.Rep(x)
		}
	}
	return nil
}

// referent is the node an atom's refers edge names, or nil.
func referent(x *graph.Node) *graph.Node {
	if x.IsList() || len(x.Refs) == 0 {
		return nil
	}
	return x.Refs[0]
}

// isMemberNode: a struct's or union's member (whim view's test).
func isMemberNode(ix *Index, n *graph.Node) bool {
	p := ix.Parent(n)
	return n.Is("member") || p != nil && graph.IsTypeDef(p) && (p.Is("struct") || p.Is("union"))
}

// EntityAt is the entity at byte pos of a view's text (Cursor, then Entity),
// and the node the cursor is on; nil, nil when the position names none.
func EntityAt(ix *Index, spans []Span, pos int) (entity, on *graph.Node) {
	i := Cursor(spans, pos)
	if i < 0 {
		return nil, nil
	}
	on = spans[i].Node
	// an atom with no id (a head, a literal) has no parent in the index:
	// its span's parents are its forms
	for ; i >= 0; i = spans[i].Parent {
		if n := spans[i].Node; n != nil {
			if e := ix.Entity(n); e != nil {
				return e, on
			}
		}
	}
	return nil, on
}

// A Session is a graph edited through its views in place, every edit
// under a journal of its own, so that the edits are undone the other way
// round with no copy of the graph kept.  An Index or Editor made before an
// edit or an undo is stale after it.
type Session struct {
	G    *graph.Graph
	done []*graph.Journal
}

// NewSession edits g.
func NewSession(g *graph.Graph) *Session { return &Session{G: g} }

// Edit is view.Edit on the session's graph, in place; a refusal leaves the
// graph as it was and is not an edit to undo.
func (s *Session) Edit(render Render, edited string, opt EditOptions) (*Result, error) {
	opt.InPlace = true
	r, err := Edit(s.G, render, edited, opt)
	if err != nil {
		return nil, err
	}
	s.done = append(s.done, r.Journal)
	return r, nil
}

// Do makes one edit of the session by the editor's own verbs: f on an
// editor of the graph, under a journal, then the re-check and the
// invariants of the forms it changed; refused, nothing is left of it.
func (s *Session) Do(f func(e *graph.Editor) error) (*graph.Journal, error) {
	j := s.G.Begin()
	e := graph.NewEditor(s.G)
	err := f(e)
	if err == nil {
		if d := e.Dangling(); len(d) > 0 {
			err = refuse("#%d (%s) refers to #%d, which the edit deleted", d[0].Use.ID, label(d[0].Use), d[0].Target.ID)
		}
	}
	if err == nil {
		if rc := e.Recheck(); rc.Left > 0 {
			err = refuse("%d expressions the edit wrote have no type the re-check derives", rc.Left)
		}
	}
	if err == nil {
		if cerr := e.CheckForms(e.TopForms(j.Saved())); cerr != nil {
			err = refuse("the graph's invariants: %v", cerr)
		}
	}
	j.End()
	if err != nil {
		j.Undo()
		return nil, err
	}
	s.done = append(s.done, j)
	return j, nil
}

// Undo undoes the last edit not yet undone; false when there is none.
func (s *Session) Undo() bool {
	if len(s.done) == 0 {
		return false
	}
	j := s.done[len(s.done)-1]
	s.done = s.done[:len(s.done)-1]
	j.Undo()
	return true
}

// Edits is how many edits there are to undo.
func (s *Session) Edits() int { return len(s.done) }
