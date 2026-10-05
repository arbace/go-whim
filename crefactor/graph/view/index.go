// Package view projects a crefactor/graph graph to trees (doc/GRAPH.md,
// *Views, read-only*): a VIEW is a root, the edges that count as children,
// in which direction, where to stop, and how much context to show around
// each node it reaches.  Where following an edge would revisit a node the
// view already holds, the view holds a link instead, so every view is a
// tree and prints as Lisp: `#ID` before a node is its id, `@ID` a link or a
// refers edge, as the graph's own Lisp writes them.
//
// It reads the graph and changes nothing.  It names nothing in any one
// program: a root is found by its name in the file, by a struct's member,
// or by an id.
package view

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// An Index is what a view asks of a graph that the graph's edges hold the
// other way round: every node's parent, every node's uses (its incoming
// refers edges) and the nodes typed by it, the file's top-level forms by
// the name they declare.  It is built once, in one walk.
type Index struct {
	G       *graph.Graph
	byID    []*graph.Node
	parent  []*graph.Node   // by id: the list holding a node, nil at the top
	uses    [][]*graph.Node // by id: the nodes with a refers edge to it, in the source's order
	typedBy [][]*graph.Node // by id: the nodes with a typed edge to it
	top     map[string][]*graph.Node
	topSet  map[*graph.Node]bool
	rank    []int32 // by id: the node's place in the walk, the containment's order
	ranked  int32
}

// NewIndex indexes g.
func NewIndex(g *graph.Graph) *Index {
	var maxID graph.ID
	g.Walk(func(n *graph.Node) bool {
		maxID = max(maxID, n.ID)
		return true
	})
	ix := &Index{
		G: g, byID: make([]*graph.Node, maxID+1), parent: make([]*graph.Node, maxID+1),
		uses: make([][]*graph.Node, maxID+1), typedBy: make([][]*graph.Node, maxID+1),
		rank: make([]int32, maxID+1),
		top:  map[string][]*graph.Node{}, topSet: map[*graph.Node]bool{},
	}
	for _, s := range g.Sections() {
		for _, f := range s {
			ix.topSet[f] = true
			ix.walk(f, nil)
		}
	}
	for _, f := range g.Forms {
		if name := graph.DeclName(f); name != "" {
			ix.top[name] = append(ix.top[name], f)
		}
	}
	return ix
}

func (ix *Index) walk(n, parent *graph.Node) {
	if n.ID != 0 {
		ix.byID[n.ID] = n
		ix.parent[n.ID] = parent
		ix.ranked++
		ix.rank[n.ID] = ix.ranked
	}
	for _, r := range n.Refs {
		// an edge into a node an edit removed may name an id past the walk's
		if r.ID != 0 && n.ID != 0 && int(r.ID) < len(ix.uses) {
			ix.uses[r.ID] = append(ix.uses[r.ID], n)
		}
	}
	if t := n.Type; t != nil && t.ID != 0 && n.ID != 0 && int(t.ID) < len(ix.typedBy) {
		ix.typedBy[t.ID] = append(ix.typedBy[t.ID], n)
	}
	for _, k := range n.Kids {
		ix.walk(k, n)
	}
}

// Node is the node of an id, or nil.
func (ix *Index) Node(id graph.ID) *graph.Node {
	if int(id) < len(ix.byID) {
		return ix.byID[id]
	}
	return nil
}

// Parent is the list holding n, or nil for a top-level node (a form, a type
// node, an external) and for a token with no id.
func (ix *Index) Parent(n *graph.Node) *graph.Node {
	if n.ID == 0 || int(n.ID) >= len(ix.parent) {
		return nil
	}
	return ix.parent[n.ID]
}

// Uses is every node with a refers edge to n, in the source's order: the uses of a
// declaration, a member, a label.
func (ix *Index) Uses(n *graph.Node) []*graph.Node {
	if n.ID == 0 || int(n.ID) >= len(ix.uses) {
		return nil
	}
	return ix.uses[n.ID]
}

// TypedBy is every node with a typed edge to n.
func (ix *Index) TypedBy(n *graph.Node) []*graph.Node {
	if n.ID == 0 || int(n.ID) >= len(ix.typedBy) {
		return nil
	}
	return ix.typedBy[n.ID]
}

// IsTop says n is a top-level node: a form of the file, a type node, an
// external.
func (ix *Index) IsTop(n *graph.Node) bool { return ix.topSet[n] }

// Holder is the top-level node holding n: the function a statement is in,
// the definition an initialiser is in.
func (ix *Index) Holder(n *graph.Node) *graph.Node {
	for {
		p := ix.Parent(n)
		if p == nil {
			return n
		}
		n = p
	}
}

// Statement is the statement holding n: the nearest node whose parent is a
// block, a function's definition or a statement expression, or a top-level
// node.
func (ix *Index) Statement(n *graph.Node) *graph.Node {
	for {
		p := ix.Parent(n)
		if p == nil {
			return n
		}
		switch p.Head() {
		case "block", "defn", "stmt-expr":
			return n
		}
		n = p
	}
}

// Decls is n's declarations: every top-level form declaring the name a
// top-level n declares -- a function's prototypes and its definition, an
// object's declarations and its definition, which C makes one entity -- and
// n alone otherwise.
func (ix *Index) Decls(n *graph.Node) []*graph.Node {
	if ix.IsTop(n) {
		if name := graph.DeclName(n); name != "" && len(ix.top[name]) > 0 {
			return ix.top[name]
		}
	}
	return []*graph.Node{n}
}

// Rep is the node that stands for n's entity in a view: a function's
// definition, else its last declaration (an object's definition follows
// its declarations); n itself when it is not top-level.
func (ix *Index) Rep(n *graph.Node) *graph.Node {
	ds := ix.Decls(n)
	if len(ds) == 1 {
		return ds[0]
	}
	for _, d := range ds {
		if d.Is("defn") {
			return d
		}
	}
	return ds[len(ds)-1]
}

// InCall says n is what a call calls: `(call n ...)`.
func (ix *Index) InCall(n *graph.Node) bool {
	p := ix.Parent(n)
	return p != nil && p.Is("call") && len(p.Kids) > 1 && p.Kids[1] == n
}

// Name is how a view names a node: the name it declares, a struct's tag, a
// member's or a parameter's name, an atom's text, else its head.
func (ix *Index) Name(n *graph.Node) string {
	if n == nil {
		return ""
	}
	if !n.IsList() {
		return n.Atom
	}
	if name := graph.DeclName(n); name != "" {
		return name
	}
	switch n.Head() {
	case "struct", "union", "enum":
		if t := graph.Tag(n); t != "" {
			return n.Head() + " " + t
		}
		if p := ix.Parent(n); p != nil && p.Is("typedef") {
			return graph.DeclName(p)
		} else if p != nil && ix.Parent(p) != nil && graph.IsTypeDef(ix.Parent(p)) {
			return ix.Name(ix.Parent(p)) // an anonymous member: its struct's
		}
		return n.Head()
	}
	if strings.HasPrefix(n.Head(), "extern") || n.Is("undeclared") || n.Is("member") {
		if len(n.Kids) > 1 && !n.Kids[1].IsList() {
			return n.Kids[1].Atom
		}
	}
	if len(n.Kids) > 0 && !n.Kids[0].IsList() {
		if p := ix.Parent(n); p != nil {
			switch {
			case (p.Is("struct") || p.Is("union")) && graph.IsTypeDef(p):
				s := strings.TrimPrefix(strings.TrimPrefix(ix.Name(p), "struct "), "union ")
				return s + "." + n.Kids[0].Atom // a member
			case p.Is("enum") || isParams(ix, p):
				return n.Kids[0].Atom // an enumerator, a parameter
			}
		}
	}
	return n.Head()
}

// isParams says p is a function type's parameter list.
func isParams(ix *Index, p *graph.Node) bool {
	g := ix.Parent(p)
	return g != nil && (g.Is("fn") || g.Is("fn-ids")) && len(g.Kids) > 1 && g.Kids[1] == p
}

// Find is the node a view is rooted at, by what a user would type:
//
//	#ID          the node of that id
//	NAME         the file's declaration of NAME (a function's definition),
//	             else an enumerator, else what the headers declare
//	S.M          the member M of the struct or union S, a typedef name or a
//	             tag, M found in its anonymous members too
//	struct T     the struct (union, enum) whose tag is T
//	F/x          the parameters and locals named x in the function F
//
// More than one node -- a name shadowed in a function -- is every one.
func (ix *Index) Find(what string) ([]*graph.Node, error) {
	what = strings.TrimSpace(what)
	switch {
	case strings.HasPrefix(what, "#"):
		id, err := strconv.ParseUint(what[1:], 10, 32)
		if n := ix.Node(graph.ID(id)); err == nil && n != nil {
			return []*graph.Node{n}, nil
		}
		return nil, fmt.Errorf("no node %s", what)
	case strings.Contains(what, "/"):
		fn, local, _ := strings.Cut(what, "/")
		return ix.locals(fn, local)
	case strings.Contains(what, "."):
		s, m, _ := strings.Cut(what, ".")
		t, err := ix.Aggregate(s)
		if err != nil {
			return nil, err
		}
		if mem := findMember(t, m); mem != nil {
			return []*graph.Node{mem}, nil
		}
		return nil, fmt.Errorf("%s has no member %s", s, m)
	case strings.HasPrefix(what, "struct ") || strings.HasPrefix(what, "union ") || strings.HasPrefix(what, "enum "):
		kw, tag, _ := strings.Cut(what, " ")
		if t := ix.tagDef(kw, strings.TrimSpace(tag)); t != nil {
			return []*graph.Node{t}, nil
		}
		return nil, fmt.Errorf("no %s defined", what)
	}
	if ds := ix.top[what]; len(ds) > 0 {
		return []*graph.Node{ix.Rep(ds[0])}, nil
	}
	var found []*graph.Node
	for _, f := range ix.G.Forms {
		graph.Walk(f, func(n *graph.Node) bool {
			if n.Is("enum") {
				for _, e := range n.Args() {
					if e.IsList() && len(e.Kids) > 0 && e.Kids[0].Atom == what && e.ID != 0 {
						found = append(found, e)
					}
				}
			}
			return len(found) == 0
		})
		if len(found) > 0 {
			return found[:1], nil
		}
	}
	for _, e := range ix.G.Externs {
		if len(e.Kids) > 1 && e.Kids[1].Atom == what {
			return []*graph.Node{e}, nil
		}
	}
	if t := ix.tagDef("struct", what); t != nil {
		return []*graph.Node{t}, nil
	}
	if t := ix.tagDef("union", what); t != nil {
		return []*graph.Node{t}, nil
	}
	return nil, fmt.Errorf("nothing in the file declares %s", what)
}

// tagDef is the definition of struct (union, enum) tag, the first in the
// file.
func (ix *Index) tagDef(kw, tag string) *graph.Node {
	var def *graph.Node
	for _, f := range ix.G.Forms {
		graph.Walk(f, func(n *graph.Node) bool {
			if def == nil && n.Is(kw) && graph.Tag(n) == tag && graph.IsTypeDef(n) {
				def = n
			}
			return def == nil
		})
		if def != nil {
			break
		}
	}
	return def
}

// Aggregate is the struct, union or enum a name says: a typedef name's
// type, a tag's definition, or an external struct's node.
func (ix *Index) Aggregate(name string) (*graph.Node, error) {
	name = strings.TrimSpace(name)
	if strings.Contains(name, " ") || strings.HasPrefix(name, "#") {
		ns, err := ix.Find(name)
		if err != nil {
			return nil, err
		}
		return aggregateOf(ns[0])
	}
	for _, d := range ix.top[name] {
		if d.Is("typedef") {
			return aggregateOf(d)
		}
	}
	for _, kw := range []string{"struct", "union", "enum"} {
		if t := ix.tagDef(kw, name); t != nil {
			return t, nil
		}
	}
	for _, e := range ix.G.Externs {
		if strings.HasPrefix(e.Head(), "extern-") && len(e.Kids) > 1 && e.Kids[1].Atom == name {
			if t, err := aggregateOf(e); err == nil {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("no struct, union or enum is named %s", name)
}

// aggregateOf is the struct, union or enum n is or names.
func aggregateOf(n *graph.Node) (*graph.Node, error) {
	switch {
	case graph.IsTypeDef(n) || n.Is("extern-struct") || n.Is("extern-union") || n.Is("extern-enum"):
		return n, nil
	case n.Type != nil && n != n.Type:
		if t, err := aggregateOf(n.Type); err == nil {
			return t, nil
		}
	}
	return nil, fmt.Errorf("%s is not a struct, union or enum", n.Head())
}

// findMember is s's member m, in its anonymous members too.
func findMember(s *graph.Node, m string) *graph.Node {
	var ms []*graph.Node
	if graph.IsTypeDef(s) {
		ms = graph.Members(s)
	} else {
		ms = s.Args() // an external struct: (member NAME)
	}
	for _, x := range ms {
		switch {
		case x.Is("member") && len(x.Kids) > 1 && x.Kids[1].Atom == m:
			return x
		case len(x.Kids) > 0 && !x.Kids[0].IsList() && x.Kids[0].Atom == m:
			return x
		case len(x.Kids) > 0 && x.Kids[0].IsList() && graph.IsTypeDef(x.Kids[0]):
			if y := findMember(x.Kids[0], m); y != nil {
				return y
			}
		}
	}
	return nil
}

// locals is the parameters and locals named x in function fn.
func (ix *Index) locals(fn, x string) ([]*graph.Node, error) {
	var f *graph.Node
	for _, d := range ix.top[fn] {
		if d.Is("defn") {
			f = d
		}
	}
	if f == nil {
		return nil, fmt.Errorf("no function %s is defined", fn)
	}
	var out []*graph.Node
	graph.Walk(f, func(n *graph.Node) bool {
		if n == f || !n.IsList() || n.ID == 0 {
			return true
		}
		if n.Is("def") && graph.DeclName(n) == x {
			out = append(out, n)
		} else if p := ix.Parent(n); p != nil && isParams(ix, p) && len(n.Kids) > 1 && n.Kids[0].Atom == x {
			out = append(out, n)
		}
		return true
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("%s declares no %s", fn, x)
	}
	return out, nil
}

// inOrder sorts nodes in the source's order: the containment's, which is
// the ids' on a graph imported or read, and not after an edit has given
// new nodes fresh ids.
func (ix *Index) inOrder(ns []*graph.Node) {
	sort.SliceStable(ns, func(i, j int) bool { return ix.before(ns[i], ns[j]) })
}

// before says a comes before b in the containment's order.
func (ix *Index) before(a, b *graph.Node) bool { return ix.order(a) < ix.order(b) }

func (ix *Index) order(n *graph.Node) int32 {
	if n.ID != 0 && int(n.ID) < len(ix.rank) && ix.rank[n.ID] != 0 {
		return ix.rank[n.ID]
	}
	return ix.ranked + 1 + int32(n.ID) // not in the walk: after it, by id
}
