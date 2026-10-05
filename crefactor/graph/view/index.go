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
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// An Index is what a view asks of a graph that the graph's edges hold the
// other way round: every node's parent, every node's uses (its incoming
// refers edges) and the nodes typed by it, the file's top-level forms by
// the name they declare.  It is built once, in one walk, and kept by
// top-level node: each one's nodes and edges are its own entries, so that
// after an edit only the top-level nodes it changed are walked again
// (Update), where a new index walks every node (33 ms on whim-vim.c's).
type Index struct {
	G       *graph.Graph
	byID    []*graph.Node
	parent  []*graph.Node    // by id: the list holding a node, nil at the top
	topOf   []*graph.Node    // by id: the top-level node holding it
	local   []int32          // by id: its place in its top-level node's walk
	uses    [][]*graph.Node  // by id: the nodes with a refers edge to it
	typedBy [][]*graph.Node  // by id: the nodes with a typed edge to it
	dirty   []bool           // by id: uses and typedBy to sort again by the source's order
	dirties []graph.ID       // the ids dirty marks
	gen     []uint32         // by id: the Update that last walked it
	update  uint32           // the Update running
	secs    [3][]*graph.Node // the sections as last indexed, copied: a deletion may shift the graph's in place
	top     map[string][]*graph.Node
	topSet  map[*graph.Node]bool
	place   map[*graph.Node][2]int // a top-level node's section and place in it
	held    map[*graph.Node]*held  // what each top-level node put in the index
	kept    bool                   // walking in Update: what add appends is sorted after
}

// held is what one top-level node's walk put in the index: its nodes'
// ids, and the edges out of them, so that they can be taken out again.
type held struct {
	ids        []graph.ID
	refs, typd [][2]*graph.Node // (user, target)
}

// NewIndex indexes g.
func NewIndex(g *graph.Graph) *Index {
	var maxID graph.ID
	g.Walk(func(n *graph.Node) bool {
		maxID = max(maxID, n.ID)
		return true
	})
	ix := &Index{G: g, topSet: map[*graph.Node]bool{}, place: map[*graph.Node][2]int{}, held: map[*graph.Node]*held{}}
	ix.grow(maxID)
	for si, sec := range g.Sections() {
		ix.secs[si] = slices.Clone(sec) // its own: an edit may shift the graph's in place
		for k, f := range sec {
			ix.topSet[f] = true
			ix.place[f] = [2]int{si, k}
			ix.add(f)
		}
	}
	ix.names()
	return ix
}

// grow makes the by-id tables hold id.
func (ix *Index) grow(id graph.ID) {
	n := int(id) + 1
	if n <= len(ix.byID) {
		return
	}
	n = max(n, len(ix.byID)+len(ix.byID)/4)
	ext := func(xs []*graph.Node) []*graph.Node { return append(xs, make([]*graph.Node, n-len(xs))...) }
	ix.byID, ix.parent, ix.topOf = ext(ix.byID), ext(ix.parent), ext(ix.topOf)
	ix.local = append(ix.local, make([]int32, n-len(ix.local))...)
	ix.uses = append(ix.uses, make([][]*graph.Node, n-len(ix.uses))...)
	ix.typedBy = append(ix.typedBy, make([][]*graph.Node, n-len(ix.typedBy))...)
	ix.dirty = append(ix.dirty, make([]bool, n-len(ix.dirty))...)
	ix.gen = append(ix.gen, make([]uint32, n-len(ix.gen))...)
}

// names indexes the file's forms by the names they declare.
func (ix *Index) names() {
	ix.top = map[string][]*graph.Node{}
	for _, f := range ix.G.Forms {
		if name := graph.DeclName(f); name != "" {
			ix.top[name] = append(ix.top[name], f)
		}
	}
}

// add walks the top-level node f into the index.
func (ix *Index) add(f *graph.Node) {
	h := &held{}
	ix.held[f] = h
	var rank int32
	var walk func(n, parent *graph.Node)
	walk = func(n, parent *graph.Node) {
		if n.ID != 0 {
			ix.grow(n.ID)
			ix.byID[n.ID], ix.parent[n.ID], ix.topOf[n.ID] = n, parent, f
			rank++
			ix.local[n.ID] = rank
			ix.gen[n.ID] = ix.update
			h.ids = append(h.ids, n.ID)
		}
		for _, r := range n.Refs {
			if r.ID != 0 && n.ID != 0 {
				ix.grow(r.ID)
				ix.uses[r.ID] = append(ix.uses[r.ID], n)
				ix.mark(r.ID)
				h.refs = append(h.refs, [2]*graph.Node{n, r})
			}
		}
		if t := n.Type; t != nil && t.ID != 0 && n.ID != 0 {
			ix.grow(t.ID)
			ix.typedBy[t.ID] = append(ix.typedBy[t.ID], n)
			ix.mark(t.ID)
			h.typd = append(h.typd, [2]*graph.Node{n, t})
		}
		for _, k := range n.Kids {
			walk(k, n)
		}
	}
	walk(f, nil)
}

// merged is a list of users in the source's order again: those of the
// top-level nodes this Update walked, sorted, each one's run placed among
// the rest by a binary search -- they kept their order, and a list of
// thousands (the uses of int) is neither sorted nor compared whole for the
// few an edit changed.
func (ix *Index) merged(xs []*graph.Node, _ map[*graph.Node]bool) []*graph.Node {
	var keep, fresh []*graph.Node
	for _, u := range xs {
		if ix.gen[u.ID] == ix.update {
			fresh = append(fresh, u)
		} else {
			keep = append(keep, u)
		}
	}
	if len(fresh) == 0 {
		return xs
	}
	ix.inOrder(fresh)
	out := make([]*graph.Node, 0, len(xs))
	at := 0
	for i := 0; i < len(fresh); {
		j := i + 1
		for j < len(fresh) && ix.topOf[fresh[j].ID] == ix.topOf[fresh[i].ID] {
			j++
		}
		p := at + sort.Search(len(keep)-at, func(k int) bool { return ix.before(fresh[i], keep[at+k]) })
		out = append(out, keep[at:p]...)
		out = append(out, fresh[i:j]...)
		at, i = p, j
	}
	return append(out, keep[at:]...)
}

// mark notes, in Update, that id's lists are to be sorted again.
func (ix *Index) mark(id graph.ID) {
	if ix.kept && !ix.dirty[id] {
		ix.dirty[id] = true
		ix.dirties = append(ix.dirties, id)
	}
}

// drop takes what f's walk put in the index out of it.
func (ix *Index) drop(f *graph.Node) {
	h := ix.held[f]
	if h == nil {
		return
	}
	delete(ix.held, f)
	for _, id := range h.ids {
		if ix.topOf[id] == f {
			ix.byID[id], ix.parent[id], ix.topOf[id], ix.local[id] = nil, nil, nil, 0
		}
	}
	gone := map[*graph.Node]bool{}
	for _, e := range h.refs {
		gone[e[0]] = true
	}
	for _, e := range h.typd {
		gone[e[0]] = true
	}
	strip := func(xs []*graph.Node) []*graph.Node {
		out := xs[:0]
		for _, u := range xs {
			if !gone[u] {
				out = append(out, u)
			}
		}
		return out
	}
	targets := map[graph.ID]bool{}
	for _, e := range h.refs {
		targets[e[1].ID] = true
	}
	for _, e := range h.typd {
		targets[e[1].ID] = true
	}
	for t := range targets {
		ix.uses[t] = strip(ix.uses[t])
		ix.typedBy[t] = strip(ix.typedBy[t])
		ix.mark(t)
	}
}

// Update brings the index to the graph after an edit, or its undo, made
// in place: changed is every node the edit changed that the index held
// before it (a journal's Saved).  The top-level nodes holding them are
// walked again, those a section gained are walked, those it lost taken
// out; nothing else is visited.  It returns how many were walked.
func (ix *Index) Update(changed []*graph.Node) int {
	ix.update++
	ix.kept = true
	defer func() { ix.kept = false }()
	again := map[*graph.Node]bool{}
	for _, n := range changed {
		if n.ID != 0 && int(n.ID) < len(ix.byID) && ix.byID[n.ID] == n && ix.topOf[n.ID] != nil {
			again[ix.topOf[n.ID]] = true
		}
	}
	// the sections: those not as they were are placed again, and what
	// they gained or lost is walked or taken out
	names := false
	for si, sec := range ix.G.Sections() {
		if slices.Equal(sec, ix.secs[si]) {
			continue
		}
		names = names || si == 0
		now := map[*graph.Node]bool{}
		for k, f := range sec {
			now[f] = true
			ix.place[f] = [2]int{si, k}
			if !ix.topSet[f] {
				again[f] = true // gained
			}
		}
		for _, f := range ix.secs[si] {
			if !now[f] {
				again[f] = true // lost
				delete(ix.topSet, f)
				delete(ix.place, f)
			}
		}
		for f := range now {
			ix.topSet[f] = true
		}
		ix.secs[si] = slices.Clone(sec)
	}
	for f := range again {
		ix.drop(f)
	}
	for f := range again {
		if ix.topSet[f] {
			ix.add(f)
			names = names || ix.place[f][0] == 0
		}
	}
	for _, t := range ix.dirties {
		ix.uses[t] = ix.merged(ix.uses[t], again)
		ix.typedBy[t] = ix.merged(ix.typedBy[t], again)
		ix.dirty[t] = false
	}
	ix.dirties = ix.dirties[:0]
	if names {
		ix.names()
	}
	return len(again)
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

// before says a comes before b in the containment's order: their
// top-level nodes' sections and places, then their places in them.
func (ix *Index) before(a, b *graph.Node) bool {
	ka, kb := ix.order(a), ix.order(b)
	for i := range ka {
		if ka[i] != kb[i] {
			return ka[i] < kb[i]
		}
	}
	return false
}

func (ix *Index) order(n *graph.Node) [4]int {
	if n.ID != 0 && int(n.ID) < len(ix.topOf) && ix.topOf[n.ID] != nil {
		p := ix.place[ix.topOf[n.ID]]
		return [4]int{0, p[0], p[1], int(ix.local[n.ID])}
	}
	return [4]int{1, int(n.ID)} // not in the walk: after it, by id
}

// sameIndex is where two indexes of one graph differ, or nil: an index
// kept by Update against a new one (the tests' checkWhole).
func sameIndex(a, b *Index) error {
	n := max(len(a.byID), len(b.byID))
	at := func(xs []*graph.Node, i int) *graph.Node {
		if i < len(xs) {
			return xs[i]
		}
		return nil
	}
	list := func(xs [][]*graph.Node, i int) []*graph.Node {
		if i < len(xs) {
			return xs[i]
		}
		return nil
	}
	for i := 1; i < n; i++ {
		if at(a.byID, i) != at(b.byID, i) {
			return fmt.Errorf("node #%d: kept %v, new %v", i, at(a.byID, i) != nil, at(b.byID, i) != nil)
		}
		if at(a.byID, i) == nil {
			continue
		}
		if at(a.parent, i) != at(b.parent, i) {
			return fmt.Errorf("#%d's parent differs", i)
		}
		for k, xs := range [2][2][]*graph.Node{{list(a.uses, i), list(b.uses, i)}, {list(a.typedBy, i), list(b.typedBy, i)}} {
			if len(xs[0]) != len(xs[1]) {
				return fmt.Errorf("#%d: %d %s kept, %d new", i, len(xs[0]), [2]string{"uses", "typed"}[k], len(xs[1]))
			}
			for j := range xs[0] {
				if xs[0][j] != xs[1][j] {
					return fmt.Errorf("#%d: its %s in another order (at %d: #%d, new #%d)", i, [2]string{"uses", "typed"}[k], j, xs[0][j].ID, xs[1][j].ID)
				}
			}
		}
	}
	if len(a.topSet) != len(b.topSet) {
		return fmt.Errorf("%d top-level nodes kept, %d new", len(a.topSet), len(b.topSet))
	}
	for f := range a.topSet {
		if !b.topSet[f] {
			return fmt.Errorf("#%d is top-level in the kept index alone", f.ID)
		}
	}
	if len(a.top) != len(b.top) {
		return fmt.Errorf("%d names kept, %d new", len(a.top), len(b.top))
	}
	return nil
}

// verify panics, under the tests' checkWhole, when ix is not what a new
// index of its graph would be.
func (ix *Index) verify(after string) {
	if checkWhole {
		if err := sameIndex(ix, NewIndex(ix.G)); err != nil {
			panic(fmt.Sprintf("after %s, the index kept by Update is not a new one's: %v", after, err))
		}
	}
}
