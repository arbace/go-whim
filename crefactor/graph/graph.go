// Package graph holds a C translation unit as ONE resolved, typed graph
// (doc/GRAPH.md): the program the pipeline's text is a printing of.
//
// NODES.  The file's containment is C-lisp's (crefactor/clisp, SPEC.md):
// every form a list, every token an atom, in C's order -- the one spanning
// tree the C view and the Lisp view walk.  A list is a node, with an id; an
// atom is a node, with an id, when it is one end of an edge (a use of a
// name), and otherwise a token of its form.  Beside the file there are the
// TYPE nodes -- `(basic int)`, `(pointer @:T)`, `(array 10 @:T)`,
// `(function (@:P ...) @:R)`; a struct, union or enum type is its
// definition's own form in the file, the node its members hang from -- and
// the EXTERNAL nodes, what the system headers declare that the file uses
// (`(extern errno)`, `(extern-typedef size_t)`, `(extern-struct stat
// (member st_size))`), typed from crefactor/cc's view of the headers.
//
// EDGES.  Contains: a list's elements, in order.  Refers: a use to its
// declaration -- an identifier to its object, function, parameter, local,
// enumerator or extern; a member to the member, by the type of the
// expression it selects from (cc.Translate's resolution, which is what
// C-lisp's untyped resolver lacked); a tag to its struct, union or enum;
// a typedef name to its typedef; a label to its label; a macro's invocation
// to every name its expansion uses.  Typed: an expression's form, and a
// declaration's, to its type node.
//
// IDENTITY.  Every node with an id keeps it through an edit; a node an edit
// makes takes a fresh one (Graph.Fresh).  The ids are given by an IDs, one
// interface, so that sequential ids -- what Import gives, in the
// containment's order -- can become content addresses without the rest
// moving.
//
// It is built once, from cc (Import), and printed as C (C, the containment
// view in cemit's spelling, byte for byte), written as Lisp (Write) and
// read back without parsing C (Read); Collect is the sweep as garbage
// collection.  It names nothing in any one program: what a caller's
// program needs -- the sweep's roots and guards -- it is told.
package graph

import (
	"fmt"
)

// An ID names a node.  0 is no id: an atom that is a token of its form, or
// a type operand.
type ID uint32

// IDs gives a graph's nodes their ids.
type IDs interface {
	// Next is a fresh id.
	Next() ID
	// Saw records an id already given, so that Next never repeats it.
	Saw(ID)
}

// Sequential is the IDs Import uses: 1, 2, 3, ... in the order asked.
type Sequential struct{ last ID }

// Next is one more than the greatest id given or seen.
func (s *Sequential) Next() ID { s.last++; return s.last }

// Saw records id.
func (s *Sequential) Saw(id ID) {
	if id > s.last {
		s.last = id
	}
}

// Last is the greatest id given or seen (Lasting): what the Lisp records,
// so that a graph read back never gives an id an edit superseded.
func (s *Sequential) Last() ID { return s.last }

// A Node is a list or an atom, with its edges.
type Node struct {
	ID   ID
	Atom string  // an atom's text: C's own token, or a C-lisp head
	Kids []*Node // a list's elements: the contains edges, in order
	Refs []*Node // refers edges
	Type *Node   // the typed edge
	list bool
	up   *Node // the container, while an Editor holds the graph (edit.go)
}

// NewList is a list of kids.
func NewList(kids ...*Node) *Node { return &Node{Kids: kids, list: true} }

// NewAtom is an atom.
func NewAtom(s string) *Node { return &Node{Atom: s} }

// IsList reports whether n is a list.
func (n *Node) IsList() bool { return n != nil && n.list }

// Head is a list's first element when it is an atom, else "".
func (n *Node) Head() string {
	if n == nil || !n.list || len(n.Kids) == 0 || n.Kids[0].list {
		return ""
	}
	return n.Kids[0].Atom
}

// Is reports whether n is a list headed by h.
func (n *Node) Is(h string) bool { return n.Head() == h }

// Args is a list's elements after its head.
func (n *Node) Args() []*Node {
	if n == nil || !n.list || len(n.Kids) == 0 {
		return nil
	}
	return n.Kids[1:]
}

// Ref is n's one refers edge, or nil.
func (n *Node) Ref() *Node {
	if len(n.Refs) == 0 {
		return nil
	}
	return n.Refs[0]
}

// A Graph is a translation unit.
type Graph struct {
	Forms   []*Node // the file's top-level forms, in order
	Types   []*Node // the type nodes
	Externs []*Node // what the headers declare that the file uses
	ids     IDs
	journal *Journal // journal.go: the edits being recorded, or nil
}

// IDs is the graph's id allocator.
func (g *Graph) IDs() IDs { return g.ids }

// Fresh gives n a new id and returns it.
func (g *Graph) Fresh(n *Node) *Node {
	n.ID = g.ids.Next()
	return n
}

// Sections are the graph's three trees, in the order ids are given and the
// Lisp is written.
func (g *Graph) Sections() [3][]*Node { return [3][]*Node{g.Forms, g.Types, g.Externs} }

// Walk calls f on every node of every section, a list before its
// elements, and does not descend below a node f returns false for.
func (g *Graph) Walk(f func(*Node) bool) {
	for _, s := range g.Sections() {
		for _, n := range s {
			Walk(n, f)
		}
	}
}

// Walk calls f on n and every node it contains, in order, and does not
// descend below a node f returns false for.
func Walk(n *Node, f func(*Node) bool) {
	if !f(n) {
		return
	}
	for _, k := range n.Kids {
		Walk(k, f)
	}
}

// Index is every node with an id, by id.
func (g *Graph) Index() map[ID]*Node {
	m := map[ID]*Node{}
	g.Walk(func(n *Node) bool {
		if n.ID != 0 {
			m[n.ID] = n
		}
		return true
	})
	return m
}

// Counts is what a graph holds.
type Counts struct {
	Nodes, Lists, Atoms, Tokens int // nodes with an id; lists; atoms with an id; atoms without
	Refers, Typed               int // edges
	Types, Externs              int // nodes in those sections
	TopLevel                    int
	Dangling                    int // edges to a node no section holds
}

// Count counts g.
func (g *Graph) Count() Counts {
	var c Counts
	c.TopLevel = len(g.Forms)
	held := map[*Node]bool{}
	g.Walk(func(n *Node) bool { held[n] = true; return true })
	for i, s := range g.Sections() {
		for _, top := range s {
			Walk(top, func(n *Node) bool {
				switch {
				case n.list:
					c.Lists++
				case n.ID != 0:
					c.Atoms++
				default:
					c.Tokens++
				}
				if n.ID != 0 {
					c.Nodes++
					switch i {
					case 1:
						c.Types++
					case 2:
						c.Externs++
					}
				}
				c.Refers += len(n.Refs)
				for _, r := range n.Refs {
					if !held[r] {
						c.Dangling++
					}
				}
				if n.Type != nil {
					c.Typed++
					if !held[n.Type] {
						c.Dangling++
					}
				}
				return true
			})
		}
	}
	return c
}

func (c Counts) String() string {
	return fmt.Sprintf("%d nodes (%d lists, %d atoms; %d tokens beside them), %d refers edges, %d typed edges, "+
		"%d type nodes, %d external nodes, %d top-level forms, %d edges dangling",
		c.Nodes, c.Lists, c.Atoms, c.Tokens, c.Refers, c.Typed, c.Types, c.Externs, c.TopLevel, c.Dangling)
}

// Equal says a and b are the same graph: the same nodes by id, each with the
// same text, elements, and edges to the same ids.  The first difference is
// the error.
func Equal(a, b *Graph) error {
	as, bs := a.Sections(), b.Sections()
	names := []string{"forms", "types", "externs"}
	for i := range as {
		if len(as[i]) != len(bs[i]) {
			return fmt.Errorf("%s: %d against %d", names[i], len(as[i]), len(bs[i]))
		}
		for j := range as[i] {
			if err := same(as[i][j], bs[i][j]); err != nil {
				return fmt.Errorf("%s %d: %w", names[i], j, err)
			}
		}
	}
	return nil
}

func same(a, b *Node) error {
	switch {
	case a.ID != b.ID:
		return fmt.Errorf("id %d against %d", a.ID, b.ID)
	case a.list != b.list || a.Atom != b.Atom:
		return fmt.Errorf("#%d: %q against %q", a.ID, a.Atom, b.Atom)
	case len(a.Kids) != len(b.Kids):
		return fmt.Errorf("#%d: %d elements against %d", a.ID, len(a.Kids), len(b.Kids))
	case len(a.Refs) != len(b.Refs):
		return fmt.Errorf("#%d: %d refers edges against %d", a.ID, len(a.Refs), len(b.Refs))
	case (a.Type == nil) != (b.Type == nil):
		return fmt.Errorf("#%d: a typed edge on one side only", a.ID)
	case a.Type != nil && (a.Type.ID != b.Type.ID || a.Type.ID == 0):
		return fmt.Errorf("#%d: typed @:%d against @:%d", a.ID, a.Type.ID, b.Type.ID)
	}
	for i := range a.Refs {
		if a.Refs[i].ID != b.Refs[i].ID || a.Refs[i].ID == 0 {
			return fmt.Errorf("#%d: refers @%d against @%d", a.ID, a.Refs[i].ID, b.Refs[i].ID)
		}
	}
	for i := range a.Kids {
		if err := same(a.Kids[i], b.Kids[i]); err != nil {
			return err
		}
	}
	return nil
}

// Number gives every node that needs an id and has none a fresh one, in
// order: the lists, and the atoms an edge starts from, of every section.
func (g *Graph) Number() {
	g.Walk(func(n *Node) bool {
		if n.ID == 0 && (n.list || len(n.Refs) > 0) {
			n.ID = g.ids.Next()
		}
		return true
	})
}
