package graph

import "slices"

// THE JOURNAL: undo without the copy (doc/GRAPH.md, *Undo*).  An edit made
// on the graph in place is undone by putting back what it changed: from
// Begin, every node an edit is about to change is saved once -- its atom,
// its kids, its refers edges, its typed edge, its id -- and the three
// sections and the id allocator with them, so that Undo restores the graph
// Begin was given, pointer for pointer: a node an untouched form refers to
// is the same node after.  Nodes an edit made are not restored; nothing
// reaches them once their containers are.  An Editor's indexes are not
// journaled: Undo makes them again (NewEditor's 18 ms on whim-vim.c, where
// the copy Undo spares is 130 ms of printing and reading the graph's Lisp).
//
// Every assignment to a node's fields in the editor's code is preceded by
// its save; internal/graphcheck's TestUndoPhases holds that to every phase
// of the pipeline (each run on its snapshot under a journal, undone, the
// graph's Lisp the snapshot's byte for byte), so that a site that forgets
// is found.

// A Journal is what the edits since Begin changed.
type Journal struct {
	g        *Graph
	saved    map[*Node]bool
	nodes    []nodeState
	sections [3][]*Node
	ids      IDs
	done     bool
}

type nodeState struct {
	n          *Node
	atom       string
	kids, refs []*Node
	typ        *Node
	id         ID
}

// Begin starts a journal of g's edits; g keeps one at a time, and an
// edit made while none is kept is not undone.
func (g *Graph) Begin() *Journal {
	j := &Journal{g: g, saved: map[*Node]bool{}}
	for i, s := range g.Sections() {
		j.sections[i] = slices.Clone(s)
	}
	if s, ok := g.ids.(*Sequential); ok {
		c := *s
		j.ids = &c
	}
	g.journal = j
	return j
}

// End stops the journal and keeps what it recorded, for an Undo later;
// the edits after are not recorded (Undo then would put back these
// alone, over them).
func (j *Journal) End() {
	if j.g.journal == j {
		j.g.journal = nil
	}
}

// Changed is how many nodes the journal saved.
func (j *Journal) Changed() int { return len(j.nodes) }

// save keeps n's fields, the first time an edit changes them.
func (g *Graph) save(n *Node) {
	j := g.journal
	if j == nil || n == nil || j.saved[n] {
		return
	}
	j.saved[n] = true
	j.nodes = append(j.nodes, nodeState{n: n, atom: n.Atom, kids: slices.Clone(n.Kids), refs: slices.Clone(n.Refs), typ: n.Type, id: n.ID})
}

// Undo puts g back as it was at Begin.  An Editor made on g before is
// stale after: make another.
func (j *Journal) Undo() {
	g := j.g
	if g.journal == j {
		g.journal = nil
	}
	if j.done {
		return
	}
	j.done = true
	for i := len(j.nodes) - 1; i >= 0; i-- {
		s := j.nodes[i]
		s.n.Atom, s.n.Kids, s.n.Refs, s.n.Type, s.n.ID = s.atom, s.kids, s.refs, s.typ, s.id
	}
	g.Forms, g.Types, g.Externs = j.sections[0], j.sections[1], j.sections[2]
	if j.ids != nil {
		g.ids = j.ids
	}
}

// Recording is the journal g keeps now, or nil.
func (g *Graph) Recording() *Journal { return g.journal }

// Saved is every node the journal saved: every node the edits since Begin
// changed that was there before them.
func (j *Journal) Saved() []*Node {
	out := make([]*Node, len(j.nodes))
	for i, s := range j.nodes {
		out[i] = s.n
	}
	return out
}
