package graph

import (
	"fmt"
)

// EDITS UNDER CONSTRAINTS (doc/GRAPH.md, step 4).  An Editor holds a graph
// with the two indexes an edit needs -- each node's container, and each
// node's incoming refers and typed edges -- and makes the four operations:
// Delete, Replace, InsertBefore/InsertAfter, Retarget.  Each is checked
// where it is made:
//
//   - PLACES.  A node is deleted, or replaced by other than one node, only
//     where its container's grammar lets it go: an item of a block, a
//     statement expression or a function's body, a top-level form, a
//     struct's or union's member, an enumerator (not one whose successor's
//     value is implicit: deleting it would move that value), an if's else.
//     A body (an if's, a loop's, a switch's) is replaced by one block; any
//     other place (an operand, a condition, a declarator, a parameter -- a
//     function's parameters are its type) by exactly one node, and an
//     expression by an expression, never a statement.  The type and
//     external sections are the collection's and are not edited.
//   - CONTAINMENT.  A node is contained once: a replacement may move nodes
//     of the subtree it replaces, and holds no other node the graph has.
//   - REFERENCES.  A new node's refers and typed edges go to nodes the graph
//     holds (or to its own new nodes).  An edge INTO a node an edit removes
//     is not refused: it becomes a DANGLING record (Dangling), which the
//     fall-out closure (FallOut) must discharge -- by removing the use -- or
//     refuse, naming it.
//
// IDENTITY (the rule for which ids survive).  A node keeps its id while it is
// in the graph, wherever an edit moves it: the items an if's branch
// splices into its block keep theirs.  A node an edit removes takes its id
// with it -- the id is superseded, never given again.  A node an edit
// brings in gets a fresh id from the graph's IDs (a list, or an atom an
// edge starts from).  Every edit is logged (Log) as what it superseded,
// what it moved and what it gave, so that a view or a cache keyed by id
// knows what went stale.
//
// TYPES.  A deletion leaves every remaining typed edge right: an item or a
// member gone changes no other node's type (a struct's type is its form,
// which loses the member).  A replacement re-derives one thing only -- an
// expression replaced by one of the same type node (or a decimal literal
// replacing an `int`) leaves the expressions above it as they were --
// and otherwise CLEARS the typed edges of the expressions above it, up to
// the statement, and counts them (Untyped): unknown, never wrong.  A new
// expression without a typed edge is untyped too.  The re-check that would
// type them again is step 6's.
type Editor struct {
	g       *Graph
	first   []int32           // the refers edges by target id, as offsets into use: first[id] to first[id+1]
	use     []*Node           // the uses, grouped by target (removed uses are filtered on reading)
	extra   map[*Node][]*Node // refers edges an edit made, and those to a node with no id, by target
	typedBy map[*Node][]*Node // typed edges into the file's forms (a struct's, union's or enum's definition), by target
	top     [3]*Node          // the sections' sentinels: forms, types, externs
	removed []*Node           // every node an edit removed with incoming edges, in order: the candidates for dangling records
	emptied []*Node           // blocks an edit left with no item
	wasTop  map[*Node]bool    // top-level forms an edit removed

	// Log is every edit, in order.
	Log []Act
	// Untyped are the expressions an edit left without a typed edge.
	Untyped []*Node
	// Written are the nodes an edit put in or under which it changed
	// something: where the closure looks for what became constant.
	Written []*Node
}

// An Act is one edit: the ids it superseded, those of the nodes it moved
// (still in the graph, elsewhere) and those it gave.
type Act struct {
	Op               string
	Gone, Moved, New []ID
}

// A Dangling is a refers (or typed) edge whose target an edit removed while
// its use stayed: what the fall-out closure must discharge.
type Dangling struct {
	Use, Target *Node
	Typed       bool
}

// NewEditor indexes g for editing.
func NewEditor(g *Graph) *Editor {
	e := &Editor{
		g:       g,
		extra:   map[*Node][]*Node{},
		typedBy: map[*Node][]*Node{},
		wasTop:  map[*Node]bool{},
	}
	for i := range e.top {
		e.top[i] = &Node{Atom: [3]string{"(forms)", "(types)", "(externs)"}[i]}
	}
	// containment: each node's container, and the greatest id
	var maxID ID
	for i, s := range g.Sections() {
		for _, n := range s {
			n.up = e.top[i]
			Walk(n, func(x *Node) bool {
				maxID = max(maxID, x.ID)
				for _, k := range x.Kids {
					k.up = x
				}
				return true
			})
		}
	}
	// the refers edges by target id: counted, then placed
	e.first = make([]int32, maxID+2)
	total := 0
	g.Walk(func(x *Node) bool {
		for _, r := range x.Refs {
			if r.ID != 0 && r.ID <= maxID {
				e.first[r.ID+1]++
				total++
			}
		}
		return true
	})
	for i := 1; i < len(e.first); i++ {
		e.first[i] += e.first[i-1]
	}
	e.use = make([]*Node, total)
	next := append([]int32(nil), e.first...)
	g.Walk(func(x *Node) bool {
		for _, r := range x.Refs {
			if r.ID != 0 && r.ID <= maxID {
				e.use[next[r.ID]] = x
				next[r.ID]++
			} else {
				e.extra[r] = append(e.extra[r], x)
			}
		}
		if x.Type != nil && e.inFile(x.Type) {
			e.typedBy[x.Type] = append(e.typedBy[x.Type], x)
		}
		return true
	})
	return e
}

// usesOf are the nodes that refer, or referred, to t: some may have gone
// since, or been retargeted.
func (e *Editor) usesOf(t *Node) []*Node {
	var out []*Node
	if id := int(t.ID); id != 0 && id+1 < len(e.first) {
		out = e.use[e.first[id]:e.first[id+1]]
	}
	if x := e.extra[t]; len(x) > 0 {
		out = append(append([]*Node(nil), out...), x...)
	}
	return out
}

// Graph is the graph edited.
func (e *Editor) Graph() *Graph { return e.g }

// Live says n is in the graph.
func (e *Editor) Live(n *Node) bool {
	for x := n; x != nil; x = x.up {
		if e.isTop(x) {
			return true
		}
	}
	return false
}

func (e *Editor) isTop(x *Node) bool { return x == e.top[0] || x == e.top[1] || x == e.top[2] }

// Parent is n's container; nil for a section's top-level node, or a node
// not in the graph.
func (e *Editor) Parent(n *Node) *Node {
	if !e.Live(n) || e.isTop(n.up) {
		return nil
	}
	return n.up
}

// Uses are the live nodes whose refers edges go to t.
func (e *Editor) Uses(t *Node) []*Node {
	var out []*Node
	for _, u := range e.usesOf(t) {
		if e.Live(u) && refersTo(u, t) {
			out = append(out, u)
		}
	}
	return out
}

func refersTo(u, t *Node) bool {
	for _, r := range u.Refs {
		if r == t {
			return true
		}
	}
	return false
}

// kids is the list p holds its elements in: a node's Kids, or a section.
func (e *Editor) kids(p *Node) []*Node {
	switch p {
	case e.top[0]:
		return e.g.Forms
	case e.top[1]:
		return e.g.Types
	case e.top[2]:
		return e.g.Externs
	}
	return p.Kids
}

func (e *Editor) setKids(p *Node, k []*Node) {
	switch p {
	case e.top[0]:
		e.g.Forms = k
	case e.top[1]:
		e.g.Types = k
	case e.top[2]:
		e.g.Externs = k
	default:
		p.Kids = k
	}
}

// index is n's place in its container, -1 when it is not in the graph.
func (e *Editor) index(n *Node) (*Node, int) {
	if !e.Live(n) {
		return nil, -1
	}
	p := n.up
	for i, k := range e.kids(p) {
		if k == n {
			return p, i
		}
	}
	return nil, -1
}

// Sibling is the node k places after n in its container (before it, for k
// negative), or nil.
func (e *Editor) Sibling(n *Node, k int) *Node {
	p, i := e.index(n)
	if i < 0 || i+k < 0 || i+k >= len(e.kids(p)) {
		return nil
	}
	return e.kids(p)[i+k]
}

// Function is the top-level form n is in, when it is a function's
// definition.
func (e *Editor) Function(n *Node) *Node {
	if !e.Live(n) {
		return nil
	}
	for {
		p := n.up
		if e.isTop(p) {
			if p == e.top[0] && n.Is("defn") {
				return n
			}
			return nil
		}
		n = p
	}
}

// Item is the statement or declaration n is in: the nearest node at or
// above n that is an item of a block, a statement expression, a function's
// body or the file.  nil when there is none.
func (e *Editor) Item(n *Node) *Node {
	for {
		p, i := e.index(n)
		if i < 0 {
			return nil
		}
		if e.place(p, i) == placeItem {
			return n
		}
		n = p
	}
}

// The places a container has for an element.
const (
	placeFixed    = iota // exactly one node, of its kind
	placeItem            // any number of items
	placeOptional        // an if's else: one or none
	placeBody            // one block
	placeMember          // a struct's or union's member: one or none
	placeEnum            // an enumerator: one or none
	placeSection         // the types and externs: not edited
)

// place is what the container p allows at element i.
func (e *Editor) place(p *Node, i int) int {
	switch p {
	case e.top[0]:
		return placeItem
	case e.top[1], e.top[2]:
		return placeSection
	}
	if p.up == e.top[1] || p.up == e.top[2] {
		return placeSection
	}
	switch p.Head() {
	case "block":
		if i == 1 && p.Kids[1].Is("@") {
			return placeFixed
		}
		if i >= 1 {
			return placeItem
		}
	case "stmt-expr":
		if i >= 1 {
			return placeItem
		}
	case "defn":
		if i >= defnItemsAt(p) {
			return placeItem
		}
	case "if":
		switch i {
		case 2:
			return placeBody
		case 3:
			return placeOptional
		}
	case "while", "switch":
		if i == 2 {
			return placeBody
		}
	case "do":
		if i == 1 {
			return placeBody
		}
	case "for":
		if i == 4 {
			return placeBody
		}
	case "struct", "union":
		for _, m := range members(p) {
			if m == p.Kids[i] {
				return placeMember
			}
		}
	case "enum":
		for _, m := range body(p) {
			if m == p.Kids[i] {
				return placeEnum
			}
		}
	}
	// a section's operand, under a type or an external, is the section's
	for q := p; ; {
		r := q.up
		if r == nil || r == e.top[0] {
			break
		}
		if r == e.top[1] || r == e.top[2] {
			return placeSection
		}
		q = r
	}
	return placeFixed
}

// defnItemsAt is the index of a function definition's first item.
func defnItemsAt(f *Node) int {
	i := defNameAt(f) + 2
	for i < len(f.Kids) && (isAttrForm(f.Kids[i]) || f.Kids[i].Is("kr-params")) {
		i++
	}
	return i
}

// stmtHeads are the forms that are statements or declarations, never
// expressions (C-lisp's).
var stmtHeads = map[string]bool{
	"block": true, "if": true, "switch": true, "while": true, "do": true, "for": true,
	"return": true, "break": true, "continue": true, "goto": true, "goto*": true,
	"label": true, "case": true, "case-range": true, "default": true, "attributed": true,
	"stmt-attr": true, "empty": true, "def": true, "typedef": true, "defn": true,
	"verbatim": true, "static_assert": true, "declare": true, "macro-decl": true,
}

// IsStatement says n is a statement's or a declaration's form, not an
// expression's: an expression is also a statement, as C's expression
// statement, and is not one of these.
func IsStatement(n *Node) bool { return n.list && stmtHeads[n.Head()] }

// Delete removes n and everything it contains.
func (e *Editor) Delete(n *Node) error {
	p, i := e.index(n)
	if i < 0 {
		return fmt.Errorf("delete #%d: not in the graph", n.ID)
	}
	return e.splice("delete", p, i, i+1, nil)
}

// Replace puts with in old's place, old and what it contains removed but
// for the nodes with moves out of it.
func (e *Editor) Replace(old *Node, with ...*Node) error {
	p, i := e.index(old)
	if i < 0 {
		return fmt.Errorf("replace #%d: not in the graph", old.ID)
	}
	return e.splice("replace", p, i, i+1, with)
}

// InsertBefore puts ns before the item at.
func (e *Editor) InsertBefore(at *Node, ns ...*Node) error {
	p, i := e.index(at)
	if i < 0 {
		return fmt.Errorf("insert before #%d: not in the graph", at.ID)
	}
	return e.splice("insert", p, i, i, ns)
}

// InsertAfter puts ns after the item at.
func (e *Editor) InsertAfter(at *Node, ns ...*Node) error {
	p, i := e.index(at)
	if i < 0 {
		return fmt.Errorf("insert after #%d: not in the graph", at.ID)
	}
	return e.splice("insert", p, i+1, i+1, ns)
}

// Retarget makes use's i-th refers edge go to to.  A use that spells a
// name must spell to's: the C view prints the name, not the edge.
func (e *Editor) Retarget(use *Node, i int, to *Node) error {
	switch {
	case !e.Live(use):
		return fmt.Errorf("retarget #%d: not in the graph", use.ID)
	case i < 0 || i >= len(use.Refs):
		return fmt.Errorf("retarget #%d: it has no refers edge %d", use.ID, i)
	case !e.Live(to):
		return fmt.Errorf("retarget #%d to #%d: the target is not in the graph", use.ID, to.ID)
	}
	if name := declName(to); !use.list && name != "" && name != use.Atom {
		return fmt.Errorf("retarget #%d `%s` to #%d: it declares `%s`", use.ID, use.Atom, to.ID, name)
	}
	use.Refs[i] = to
	e.extra[to] = append(e.extra[to], use)
	e.Log = append(e.Log, Act{Op: "retarget", Moved: []ID{use.ID}})
	return nil
}

// splice replaces the elements lo..hi of p with with, checked.
func (e *Editor) splice(op string, p *Node, lo, hi int, with []*Node) error {
	ks := e.kids(p)
	what := func() string {
		if lo < hi {
			return fmt.Sprintf("%s #%d (%s)", op, ks[lo].ID, label(ks[lo]))
		}
		return fmt.Sprintf("%s in #%d (%s)", op, p.ID, label(p))
	}
	// the place
	var pl int
	if lo == hi {
		pl = e.insertPlace(p, lo)
	} else {
		pl = e.place(p, lo)
	}
	switch {
	case pl == placeSection:
		return fmt.Errorf("%s: the type and external sections are the collection's", what())
	case lo == hi && pl != placeItem:
		return fmt.Errorf("%s: #%d (%s) has no place for an item there", what(), p.ID, label(p))
	case pl == placeItem:
	case pl == placeOptional, pl == placeMember:
		if len(with) > 1 {
			return fmt.Errorf("%s: one node or none in that place of #%d (%s)", what(), p.ID, label(p))
		}
	case pl == placeEnum:
		if len(with) > 1 {
			return fmt.Errorf("%s: one enumerator or none in that place", what())
		}
		if len(with) == 0 {
			if next := e.nextEnumerator(p, lo); next != nil && len(next.Kids) < 2 {
				return fmt.Errorf("%s: the value of #%d (%s), after it, is implicit and would move", what(), next.ID, label(next))
			}
		}
	case pl == placeBody:
		if len(with) != 1 || !with[0].Is("block") {
			return fmt.Errorf("%s: the body of #%d (%s) is one block", what(), p.ID, label(p))
		}
	default:
		if len(with) != 1 {
			return fmt.Errorf("%s: the place in #%d (%s) holds exactly one node", what(), p.ID, label(p))
		}
		if !IsStatement(ks[lo]) && IsStatement(with[0]) {
			return fmt.Errorf("%s: an expression's place in #%d (%s) cannot hold a %s", what(), p.ID, label(p), with[0].Head())
		}
	}
	// containment: what goes, what moves, what is new
	old := map[*Node]bool{}
	for _, n := range ks[lo:hi] {
		Walk(n, func(x *Node) bool { old[x] = true; return true })
	}
	seen := map[*Node]bool{}
	var fresh []*Node
	var err error
	for _, w := range with {
		Walk(w, func(x *Node) bool {
			switch {
			case err != nil:
			case seen[x]:
				err = fmt.Errorf("%s: #%d (%s) is in the replacement twice", what(), x.ID, label(x))
			case e.Live(x) && !old[x]:
				err = fmt.Errorf("%s: #%d (%s) is held elsewhere in the graph: a node is contained once", what(), x.ID, label(x))
			case !e.Live(x) && x.ID != 0 && !old[x]:
				// a node removed earlier comes back with its id: refused,
				// an id is never given twice
				err = fmt.Errorf("%s: #%d (%s) was removed by an earlier edit; its id is superseded", what(), x.ID, label(x))
			}
			seen[x] = true
			if !e.Live(x) {
				fresh = append(fresh, x)
			}
			return err == nil
		})
	}
	if err != nil {
		return err
	}
	for _, x := range fresh {
		for _, r := range x.Refs {
			if !e.Live(r) && !seen[r] || old[r] && !seen[r] {
				return fmt.Errorf("%s: a refers edge from %s to #%d (%s), which the graph does not hold", what(), label(x), r.ID, label(r))
			}
		}
		if t := x.Type; t != nil && (!e.Live(t) && !seen[t] || old[t] && !seen[t]) {
			return fmt.Errorf("%s: a typed edge from %s to #%d, which the graph does not hold", what(), label(x), t.ID)
		}
	}
	// the types above an expression replaced
	var oldOne *Node
	if hi == lo+1 {
		oldOne = ks[lo]
	}
	// apply
	act := Act{Op: op}
	if p == e.top[0] {
		for _, n := range ks[lo:hi] {
			e.wasTop[n] = true
		}
	}
	var gone []*Node
	for x := range old {
		if !seen[x] {
			gone = append(gone, x)
		}
	}
	for _, x := range gone {
		x.up = nil
		if x.ID != 0 {
			act.Gone = append(act.Gone, x.ID)
		}
		if len(e.usesOf(x)) > 0 || len(e.typedBy[x]) > 0 {
			e.removed = append(e.removed, x)
		}
	}
	nk := make([]*Node, 0, len(ks)-(hi-lo)+len(with))
	nk = append(nk, ks[:lo]...)
	nk = append(nk, with...)
	nk = append(nk, ks[hi:]...)
	e.setKids(p, nk)
	for _, w := range with {
		w.up = p
		Walk(w, func(x *Node) bool {
			for _, k := range x.Kids {
				k.up = x
			}
			return true
		})
	}
	for _, x := range fresh {
		if x.ID == 0 && (x.list || len(x.Refs) > 0) {
			e.g.Fresh(x)
			act.New = append(act.New, x.ID)
		} else if x.ID != 0 {
			e.g.ids.Saw(x.ID)
		}
		for _, r := range x.Refs {
			e.extra[r] = append(e.extra[r], x)
		}
		if x.Type != nil && e.inFile(x.Type) {
			e.typedBy[x.Type] = append(e.typedBy[x.Type], x)
		}
	}
	for x := range old {
		if seen[x] && x.ID != 0 {
			act.Moved = append(act.Moved, x.ID)
		}
	}
	e.Log = append(e.Log, act)
	// what became empty, what was written, what lost its type
	if len(with) == 0 && p.Is("block") && len(blockItems(p)) == 0 {
		e.emptied = append(e.emptied, p)
	}
	if p != e.top[0] {
		e.Written = append(e.Written, p)
	}
	e.Written = append(e.Written, with...)
	if oldOne != nil && len(with) == 1 && e.place(p, lo) == placeFixed && !IsStatement(oldOne) {
		e.retype(p, oldOne, with[0])
	}
	return nil
}

// retype keeps or clears the typed edges above an expression replaced.
func (e *Editor) retype(p, old, new *Node) {
	if new.list && new.Type == nil && !IsStatement(new) {
		e.Untyped = append(e.Untyped, new)
	}
	if sameType(old, new) {
		return
	}
	for q := p; q != nil && q.Type != nil && !IsStatement(q) && q.up != e.top[0]; q = e.Parent(q) {
		q.Type = nil
		e.Untyped = append(e.Untyped, q)
	}
}

// sameType says new is known to have old's type: the same type node, or
// a decimal literal in an int's place, or an atom for an atom with the
// same edge.
func sameType(old, new *Node) bool {
	switch {
	case old.Type != nil && new.Type == old.Type:
		return true
	case !new.list && isDecimalInt(new.Atom) && old.Type != nil && old.Type.Is("basic") &&
		len(old.Type.Kids) == 2 && old.Type.Kids[1].Atom == "int":
		return true
	case !old.list && !new.list && old.Type == nil && new.Type == nil:
		if isDecimalInt(old.Atom) && isDecimalInt(new.Atom) {
			return true
		}
		return old.Ref() != nil && old.Ref() == new.Ref()
	}
	return false
}

// isDecimalInt is a decimal constant of type int: no suffix, below 2^31.
func isDecimalInt(s string) bool {
	if s == "" || len(s) > 10 || s[0] == '0' && len(s) > 1 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	v, ok := parseInt(s)
	return ok && v < 1<<31
}

func (e *Editor) inFile(n *Node) bool {
	for {
		p := n.up
		if p == nil {
			return false
		}
		if p == e.top[0] {
			return true
		}
		if p == e.top[1] || p == e.top[2] {
			return false
		}
		n = p
	}
}

// nextEnumerator is the enumerator after element i of the enum p.
func (e *Editor) nextEnumerator(p *Node, i int) *Node {
	if i+1 < len(p.Kids) {
		if n := p.Kids[i+1]; n.list && !n.Is("@") {
			return n
		}
	}
	return nil
}

// blockItems are a block's items, after its attributes.
func blockItems(b *Node) []*Node {
	items := b.Kids[1:]
	if len(items) > 0 && items[0].Is("@") {
		items = items[1:]
	}
	return items
}

// Dangling are the edges into removed nodes whose uses are still in the
// graph: what the closure has not yet discharged.
func (e *Editor) Dangling() []Dangling {
	var out []Dangling
	seen := map[*Node]bool{}
	for _, t := range e.removed {
		if seen[t] || e.Live(t) {
			continue
		}
		seen[t] = true
		for _, u := range e.usesOf(t) {
			if e.Live(u) && refersTo(u, t) {
				out = append(out, Dangling{Use: u, Target: t})
			}
		}
		for _, u := range e.typedBy[t] {
			if e.Live(u) && u.Type == t {
				out = append(out, Dangling{Use: u, Target: t, Typed: true})
			}
		}
	}
	return out
}

// live says d is still a dangling edge.
func (e *Editor) live(d Dangling) bool {
	if !e.Live(d.Use) || e.Live(d.Target) {
		return false
	}
	if d.Typed {
		return d.Use.Type == d.Target
	}
	return refersTo(d.Use, d.Target)
}

// Check is the whole graph held to the invariants, for a test: every node
// contained once and indexed where it is, every edge to a node the graph
// holds or a dangling record, every typed edge to a held node or recorded.
func (e *Editor) Check() error {
	seen := map[*Node]*Node{}
	var err error
	for i, s := range e.g.Sections() {
		for _, top := range s {
			if top.up != e.top[i] {
				return fmt.Errorf("#%d (%s): a top-level node the index does not place there", top.ID, label(top))
			}
			Walk(top, func(x *Node) bool {
				for _, k := range x.Kids {
					if q, ok := seen[k]; ok && err == nil {
						err = fmt.Errorf("#%d (%s) is contained twice, in #%d and #%d", k.ID, label(k), q.ID, x.ID)
					}
					seen[k] = x
					if k.up != x && err == nil {
						err = fmt.Errorf("#%d (%s): the index does not place it in #%d", k.ID, label(k), x.ID)
					}
				}
				return err == nil
			})
		}
	}
	if err != nil {
		return err
	}
	dangling := map[[2]*Node]bool{}
	for _, d := range e.Dangling() {
		dangling[[2]*Node{d.Use, d.Target}] = true
	}
	e.g.Walk(func(x *Node) bool {
		for _, r := range x.Refs {
			if !e.Live(r) && !dangling[[2]*Node{x, r}] && err == nil {
				err = fmt.Errorf("%s refers to #%d, which the graph does not hold, and no record says so", label(x), r.ID)
			}
		}
		if t := x.Type; t != nil && !e.Live(t) && !dangling[[2]*Node{x, t}] && err == nil {
			err = fmt.Errorf("%s has a typed edge to #%d, which the graph does not hold", label(x), t.ID)
		}
		return err == nil
	})
	return err
}

// label names a node for a message: its head, or its atom.
func label(n *Node) string {
	switch {
	case n == nil:
		return "nil"
	case !n.list:
		return "`" + n.Atom + "`"
	case declName(n) != "" && (n.Is("def") || n.Is("defn") || n.Is("typedef")):
		return n.Head() + " " + declName(n)
	case n.Head() != "":
		return n.Head()
	}
	return "list"
}

// FileDecls are the top-level declarations of name: a function's
// prototypes and definition, an object's declarations.
func (e *Editor) FileDecls(name string) []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		if (f.Is("def") || f.Is("defn")) && !hasPrefix(f, "typedef") && declName(f) == name {
			out = append(out, f)
		}
	}
	return out
}

// Defn is the definition of the function name, or nil.
func (e *Editor) Defn(name string) *Node {
	for _, f := range e.g.Forms {
		if f.Is("defn") && declName(f) == name {
			return f
		}
	}
	return nil
}

// Body is a function definition's items.
func Body(f *Node) []*Node { return f.Kids[defnItemsAt(f):] }

// DeclName is the name a def, defn, typedef, parameter, member, enumerator
// or label form declares, or "".
func DeclName(n *Node) string { return declName(n) }

// Members are a struct or union definition's members.
func Members(n *Node) []*Node { return members(n) }

// insertPlace is what p allows a new element at i, before its element i:
// an item where its items are.
func (e *Editor) insertPlace(p *Node, i int) int {
	switch {
	case p == e.top[0]:
		return placeItem
	case p == e.top[1] || p == e.top[2]:
		return placeSection
	case p.Is("block"):
		if i >= 2 || i == 1 && (len(p.Kids) < 2 || !p.Kids[1].Is("@")) {
			return placeItem
		}
	case p.Is("stmt-expr"):
		if i >= 1 {
			return placeItem
		}
	case p.Is("defn"):
		if i >= defnItemsAt(p) {
			return placeItem
		}
	}
	return placeFixed
}
