package graph

import (
	"fmt"
	"slices"
)

// MOVE (doc/GRAPH-MIGRATION.md, B2c): a node moved to another place --
// a statement or a run of them to another block, a local to an outer one,
// a function before its first use, an expression into a declaration's
// value -- KEEPING its ids and its edges, and refused when a reference
// would no longer resolve from where it goes:
//
//   - PLACES.  MoveBefore, MoveAfter and MoveRun move items (a block's, a
//     function body's, a statement expression's, the file's) beside another
//     item, a file's form among the file's and a statement among
//     statements; MoveTo puts any node in the place of another, `to` (a
//     placeholder, superseded), the place it leaves taken by `fill`, or left
//     empty where an item or an else may go.  A node moves into neither
//     itself nor what it holds.
//   - NAMES.  Every edge that crosses the moved nodes' edge -- a use inside
//     them of a declaration outside, a use outside of a declaration inside,
//     and a use, where they go, of another declaration of a name they
//     declare -- must resolve from its new place to the declaration it
//     refers to (the scopes before it, innermost first, then the file:
//     BUILD's resolution, the importer's).  Where it resolves to another
//     file-scope declaration of the same name -- one entity in C, the
//     importer's edge going to the first -- the edge is retargeted there,
//     logged; anything else refuses, naming the use.  A label must stay in
//     its function; a member or a tag used at file scope, after its
//     definition.  A declaration that would be declared twice in one block
//     refuses.
//   - CONTROL.  A `break`, `continue`, `case` or `default` inside the moved
//     nodes that binds outside them must bind to the same loop or switch
//     from where they go; a `return` must stay in its function.
//
// It is checked on the graph as the move leaves it, and undone when it
// refuses: nothing is logged, no id given.  Ids: every node moved keeps
// its own (a `move` act lists them); MoveTo's placeholder is superseded and
// its fill given, as a Replace's.  Types do not change with a place, but
// the expressions above the moved node and above the fill are typed again
// (rederive) where they held the node before.

// MoveBefore moves the item n before the item at.
func (e *Editor) MoveBefore(n, at *Node) error { return e.MoveRun(n, n, at, false) }

// MoveAfter moves the item n after the item at.
func (e *Editor) MoveAfter(n, at *Node) error { return e.MoveRun(n, n, at, true) }

// MoveRun moves the items first through last, which one list holds in that
// order, before the item at, or after it.
func (e *Editor) MoveRun(first, last, at *Node, after bool) error {
	p1, lo := e.index(first)
	q1, hi := e.index(last)
	p2, j := e.index(at)
	what := fmt.Sprintf("move #%d..#%d", first.ID, last.ID)
	switch {
	case lo < 0 || hi < 0 || j < 0:
		return fmt.Errorf("%s: not in the graph", what)
	case p1 != q1:
		return fmt.Errorf("%s: not in one list", what)
	case hi < lo:
		return fmt.Errorf("%s: its end comes before its start", what)
	case e.place(p2, j) != placeItem:
		return fmt.Errorf("%s: #%d (%s) is not an item", what, at.ID, label(at))
	case (p1 == e.top[0]) != (p2 == e.top[0]):
		return fmt.Errorf("%s: a file's form moves among the file's, a statement among statements", what)
	}
	run := slices.Clone(e.kids(p1)[lo : hi+1])
	set := map[*Node]bool{}
	for k, n := range run {
		if e.place(p1, lo+k) != placeItem {
			return fmt.Errorf("%s: #%d (%s) is not an item", what, n.ID, label(n))
		}
		set[n] = true
	}
	if set[at] || e.within(p2, set) {
		return fmt.Errorf("%s: into what it holds", what)
	}
	bind := e.binders(run, set)
	// the move, tentatively
	ks := slices.Delete(slices.Clone(e.kids(p1)), lo, hi+1)
	e.setKids(p1, ks)
	_, j = e.index(at)
	if after {
		j++
	}
	e.setKids(p2, slices.Insert(slices.Clone(e.kids(p2)), j, run...))
	for _, n := range run {
		n.up = p2
	}
	undo := func() {
		e.setKids(p2, slices.Delete(slices.Clone(e.kids(p2)), j, j+len(run)))
		e.setKids(p1, slices.Insert(slices.Clone(e.kids(p1)), lo, run...))
		for _, n := range run {
			n.up = p1
		}
	}
	re, err := e.moveChecks(run, set, bind, p2)
	if err != nil {
		undo()
		return fmt.Errorf("%s: %w", what, err)
	}
	e.moved("move", run, p1, p2, re)
	if p1.Is("block") && len(blockItems(p1)) == 0 {
		e.emptied = append(e.emptied, p1)
	}
	return nil
}

// MoveTo puts n in the place of to, which goes (superseded), and fill in
// the place n leaves -- nil to leave it empty where an item or an else may
// go.  fill is new: built (Build) where n stands, before the move.
func (e *Editor) MoveTo(n, to, fill *Node) error {
	pn, in := e.index(n)
	pt, it := e.index(to)
	what := fmt.Sprintf("move #%d (%s) to #%d", n.ID, label(n), to.ID)
	switch {
	case in < 0 || it < 0:
		return fmt.Errorf("%s: not in the graph", what)
	case n == to || e.within(to, map[*Node]bool{n: true}) || e.within(n, map[*Node]bool{to: true}):
		return fmt.Errorf("%s: into what it holds, or out of what it goes with", what)
	case (pn == e.top[0]) != (pt == e.top[0]):
		return fmt.Errorf("%s: a file's form moves among the file's, a statement among statements", what)
	}
	if err := e.fits(pt, it, to, n); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	switch pl := e.place(pn, in); {
	case fill == nil && pl != placeItem && pl != placeOptional:
		return fmt.Errorf("%s: its place in #%d (%s) holds one node: say what fills it", what, pn.ID, label(pn))
	case fill != nil:
		if err := e.fits(pn, in, n, fill); err != nil {
			return fmt.Errorf("%s: the fill: %w", what, err)
		}
		bad := false
		Walk(fill, func(x *Node) bool { bad = bad || e.Live(x) || x.ID != 0; return !bad })
		if bad {
			return fmt.Errorf("%s: the fill is not new", what)
		}
	}
	set := map[*Node]bool{n: true}
	bind := e.binders([]*Node{n}, set)
	swap := func() {
		e.kids(pn)[in], e.kids(pt)[it] = e.kids(pt)[it], e.kids(pn)[in]
		n.up, to.up = to.up, n.up
	}
	swap()
	re, err := e.moveChecks([]*Node{n}, set, bind, pt)
	if err == nil {
		if fill != nil {
			err = e.Replace(to, fill)
		} else {
			err = e.Delete(to)
		}
	}
	if err != nil {
		swap()
		return fmt.Errorf("%s: %w", what, err)
	}
	e.moved("move", []*Node{n}, pn, pt, re)
	tx := e.typeTx()
	tx.rederive(pt)
	if fill != nil {
		tx.rederive(pn)
	}
	tx.commit()
	return nil
}

// fits says whether with may stand in the place of old, element i of p.
func (e *Editor) fits(p *Node, i int, old, with *Node) error {
	switch e.place(p, i) {
	case placeItem:
		return nil
	case placeBody:
		if !with.Is("block") {
			return fmt.Errorf("the body of #%d (%s) is one block", p.ID, label(p))
		}
	case placeOptional:
		if !with.Is("block") && !with.Is("if") {
			return fmt.Errorf("an else is a block or an if")
		}
	case placeFixed:
		if IsStatement(old) != IsStatement(with) {
			return fmt.Errorf("#%d (%s) cannot stand where #%d (%s) is", with.ID, label(with), old.ID, label(old))
		}
	default:
		return fmt.Errorf("#%d (%s) holds no such place", p.ID, label(p))
	}
	return nil
}

// A binding is what a jump or a label inside moved nodes binds to outside
// them: the loop or switch, or the function.
type binding struct {
	n, to *Node
}

// binders are the bindings out of the moved nodes run (set).
func (e *Editor) binders(run []*Node, set map[*Node]bool) []binding {
	var out []binding
	for _, r := range run {
		Walk(r, func(x *Node) bool {
			switch {
			case x.Is("break") || x.Is("continue") || isCaseLabel(x):
				if b := e.binder(x); b != nil && !e.within(b, set) {
					out = append(out, binding{x, b})
				}
			case x.Is("return"):
				out = append(out, binding{x, e.Function(x)})
			}
			return true
		})
	}
	return out
}

// binder is the loop or switch a break, continue or case label binds to.
func (e *Editor) binder(x *Node) *Node {
	for q := e.Parent(x); q != nil; q = e.Parent(q) {
		switch {
		case isLoop(q) && !x.Is("case") && !x.Is("case-range") && !x.Is("default"):
			return q
		case q.Is("switch") && !x.Is("continue"):
			return q
		case q.Is("defn"):
			return nil
		}
	}
	return nil
}

// A retarget is an edge a move sends to another declaration of its name.
type retarget struct {
	use *Node
	i   int
	to  *Node
}

// moveChecks holds the graph, the move made, to the rules: every edge
// across the moved nodes resolves, every binding is the same.  The
// retargets are the edges that resolve to another file-scope declaration
// of their name.
func (e *Editor) moveChecks(run []*Node, set map[*Node]bool, bind []binding, dest *Node) ([]retarget, error) {
	for _, b := range bind {
		now := e.Function(b.n)
		if !b.n.Is("return") {
			now = e.binder(b.n)
		}
		if now != b.to {
			return nil, fmt.Errorf("#%d (%s) would bind to %s, not to #%d (%s)", b.n.ID, label(b.n), label(now), b.to.ID, label(b.to))
		}
	}
	var re []retarget
	seen := map[retarget]bool{}
	check := func(u *Node, k int, t *Node) error {
		r, err := e.resolvesFrom(u, t)
		if err != nil {
			return err
		}
		if x := (retarget{u, k, r}); r != t && !seen[x] {
			seen[x] = true
			re = append(re, x)
		}
		return nil
	}
	// a redeclaration where they go
	declared := map[string]bool{}
	for _, n := range run {
		if (n.Is("def") || n.Is("typedef")) && dest != e.top[0] {
			declared[topName(n)] = true
		}
	}
	for _, k := range e.kids(dest) {
		if !set[k] && (k.Is("def") || k.Is("typedef")) && declared[topName(k)] {
			return nil, fmt.Errorf("`%s` is declared again where it would move (#%d)", topName(k), k.ID)
		}
	}
	if dest.Is("defn") {
		for name := range declared {
			if paramNamed(dest, name) != nil {
				return nil, fmt.Errorf("`%s` is a parameter of %s, where it would move", name, topName(dest))
			}
		}
	}
	// the uses inside, of what is outside; and the uses outside, of what
	// is inside
	var err error
	for _, n := range run {
		Walk(n, func(x *Node) bool {
			for k, t := range x.Refs {
				if err == nil && !e.within(t, set) {
					err = check(x, k, t)
				}
			}
			if err == nil {
				for _, u := range e.Uses(x) {
					if !e.within(u, set) {
						for k, t := range u.Refs {
							if t == x && err == nil {
								err = check(u, k, t)
							}
						}
					}
				}
			}
			return err == nil
		})
		if err != nil {
			return nil, err
		}
	}
	// the uses of other declarations of a name the moved nodes declare,
	// where they go: hidden now, or no longer the first
	names := map[string]bool{}
	for _, n := range run {
		if name := topName(n); name != "" {
			names[name] = true
		}
	}
	if len(names) > 0 {
		scope := e.g.Forms
		if !e.isTop(dest) {
			if f := e.Function(dest); f != nil {
				scope = []*Node{f}
			}
		}
		for _, s := range scope {
			Walk(s, func(x *Node) bool {
				if e.within(x, set) {
					return false
				}
				for k, t := range x.Refs {
					if err == nil && !e.within(t, set) && names[ordinaryName(t)] && e.inFile(t) && !isMember(e, t) {
						err = check(x, k, t)
					}
				}
				return err == nil
			})
		}
	}
	return re, err
}

// resolvesFrom is the declaration the use u's edge to t resolves to from
// where u now is: t, or another file-scope declaration of its name; an
// error when the name would resolve to nothing or to another entity.
func (e *Editor) resolvesFrom(u, t *Node) (*Node, error) {
	switch {
	case !e.inFile(t):
		return t, nil // a type, an external
	case t.Is("label"):
		if e.Function(u) != e.Function(t) {
			return nil, fmt.Errorf("the goto #%d would leave the function of its label %s", u.ID, labelName(t))
		}
		return t, nil
	case isMember(e, t) || isDefForm(t):
		if e.topIndex(t) > e.topIndex(u) {
			return nil, fmt.Errorf("#%d (%s) would be used at #%d before its definition", t.ID, label(t), u.ID)
		}
		return t, nil
	}
	name := ordinaryName(t)
	if name == "" {
		return nil, fmt.Errorf("#%d (%s), which #%d refers to, is not a declaration MOVE can follow", t.ID, label(t), u.ID)
	}
	r := e.Resolve(u, name)
	switch {
	case r == t:
		return t, nil
	case r != nil && e.fileScope(r) && e.fileScope(t):
		return r, nil
	case r == nil:
		return nil, fmt.Errorf("`%s` at #%d in %s would resolve to nothing", name, u.ID, inFn(e, u))
	}
	return nil, fmt.Errorf("`%s` at #%d in %s would resolve to #%d (%s), not #%d (%s)", name, u.ID, inFn(e, u), r.ID, label(r), t.ID, label(t))
}

// fileScope says d is declared at file scope: a top-level form, or an
// enumerator of one outside a function.
func (e *Editor) fileScope(d *Node) bool {
	if !e.Live(d) || !e.inFile(d) {
		return false
	}
	f := e.Function(d)
	return f == nil || f == d
}

// topIndex is the place in the file of the top-level form n is in.
func (e *Editor) topIndex(n *Node) int {
	top := n
	for e.Parent(top) != nil {
		top = e.Parent(top)
	}
	return slices.Index(e.g.Forms, top)
}

// labelName is a label's name.
func labelName(l *Node) string {
	if len(l.Kids) > 1 {
		return l.Kids[1].Atom
	}
	return "?"
}

// moved commits a move: the retargets made, the act logged.
func (e *Editor) moved(op string, run []*Node, from, to *Node, re []retarget) {
	act := Act{Op: op}
	for _, n := range run {
		Walk(n, func(x *Node) bool {
			if x.ID != 0 {
				act.Moved = append(act.Moved, x.ID)
			}
			return true
		})
	}
	e.Log = append(e.Log, act)
	for _, r := range re {
		r.use.Refs[r.i] = r.to
		e.extra[r.to] = append(e.extra[r.to], r.use)
		e.Log = append(e.Log, Act{Op: "retarget", Moved: []ID{r.use.ID}})
	}
	for _, p := range []*Node{from, to} {
		if !e.isTop(p) {
			e.Written = append(e.Written, p)
			e.touched = append(e.touched, p)
		}
	}
}
