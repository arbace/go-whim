package graph

import (
	"fmt"
	"strconv"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// INITROW (doc/GRAPH-MIGRATION.md, B2b): the rows of an initialised table --
// an array's initialiser, vim's options[], cmdnames[], key_names_table[] --
// deleted, inserted and reordered, with what names a row BY ITS POSITION
// said again.  An element of an initialiser is a one-node place for the
// editor (Replace points a row's handler elsewhere; B0), but a row is not an
// item: Delete refuses it, because a row's position is its meaning wherever
// something indexes the table, and only ArrangeRows knows to say it again.
//
// A ROW'S POSITION is its index in the array: the one after the row before
// it, or what its designator says (`[CMD_read] = {...}`, `(at (idx K) ...)`,
// K evaluated as the sweep evaluates an enumerator's value; `[K ... L]` sets
// the next to L+1).  A row designated by a name does not move when another
// goes: the name's value places it.
//
// WHAT NAMES A POSITION, and is said again when its row moves:
//
//   - every subscript of the table by a decimal constant, anywhere in the
//     file -- `&highlight_tab[6]`, `(index T 6)`, found by the edges (the
//     subscripted name refers to one of the table's declarations);
//   - the enumerators the cut names as the table's indexes (RowIndex's
//     Enumerators: `ME_EXTRA_CMD`, which main_errors[] is read at through a
//     parameter, so no edge says it): one with a decimal value is written
//     with the new one; an implicit one must come out at the new position
//     by itself, its enum's other values held (RENUM's HoldValues);
//   - the arrays the cut names as permutations of the table (RowIndex's
//     Arrays: nv_cmd_idx[] for nv_cmds[]), every element a decimal constant.
//
// A position named but deleted is refused -- a subscript, a permutation's
// element -- except an index enumerator, which goes with its row (the row
// and its index are one thing: phase 51's main_errors[] and ME_GARBAGE),
// its declaration with it when it was its enum's only enumerator.  A
// position nothing names moves freely: a table read by a loop
// (key_names_table[], builtin_terminals[]) has no index to say.
//
// An array whose size is written (`T[13]`) is refused: its rows and its
// size are the cut's to say together.  The table's typed edge, an array of
// so many elements, is retargeted to the type node of the new count when the
// graph holds one and cleared otherwise (listed in Untyped: step 6's
// checker types it again).

// RowIndex is what names a table's rows by position, besides its
// subscripts by constants (found by the edges).
type RowIndex struct {
	Enumerators []*Node // enumerators whose values are row positions
	Arrays      []*Node // definitions of arrays whose elements are row positions
}

// RowsDone is what an arrangement of a table's rows did.
type RowsDone struct {
	Deleted, Inserted []*Node
	Moved             int     // rows whose position changed
	Sites             []*Node // the subscripts and permutation elements written with a new position
	Renumbered        []ValueMove
	GoneEnumerators   []*Node // index enumerators deleted with their rows
	GoneDecls         []*Node // their declarations, when they were their enum's only enumerator
}

// TableInit is the initialiser of the table def: its `(init ...)`, the rows
// its elements.
func TableInit(def *Node) *Node {
	if !def.Is("def") {
		return nil
	}
	if v := defValue(def); v != nil && v.Is("init") {
		return v
	}
	return nil
}

// positions are the rows' positions in the array, and whether each is
// known.
func rowPositions(rows []*Node, env map[string]int64) ([]int64, []bool, error) {
	pos := make([]int64, len(rows))
	ok := make([]bool, len(rows))
	var next int64
	known := true
	for i, r := range rows {
		p, k := next, known
		end := int64(-1)
		if r.Is("at") && len(r.Kids) > 2 {
			d := r.Kids[1]
			switch {
			case d.Is("idx") && (len(d.Kids) == 2 || len(d.Kids) == 3):
				p, k = evalForm(d.Kids[1], env)
				if len(d.Kids) == 3 && k {
					end, k = evalForm(d.Kids[2], env)
				}
			case !d.list && len(d.Atom) > 1 && d.Atom[0] == '.':
				return nil, nil, fmt.Errorf("row %d is designated by the member %s: not an array's row", i, d.Atom)
			default:
				k = false
			}
		}
		pos[i], ok[i] = p, k
		if end >= 0 {
			p = end
		}
		next, known = p+1, k
	}
	return pos, ok, nil
}

// tableEntity is the table's declarations: the file's, of its name, when it
// is the file's; itself when it is a local.
func (e *Editor) tableEntity(def *Node) map[*Node]bool {
	out := map[*Node]bool{def: true}
	if e.Parent(def) == nil {
		for _, d := range e.FileDecls(topName(def)) {
			out[d] = true
		}
	}
	return out
}

// ArrangeRows makes the table def's rows order: those it holds stay (moved
// if their place changed, keeping their ids), those it leaves out are
// deleted, new ones (BuildRows, not in the graph) are inserted with fresh
// ids -- and every position the file names (subscripts by constants, and
// ix) said again.  Everything is checked before anything is changed.
func (e *Editor) ArrangeRows(def *Node, order []*Node, ix RowIndex) (*RowsDone, error) {
	if !e.Live(def) {
		return nil, fmt.Errorf("arrange rows of #%d: not in the graph", def.ID)
	}
	name := topName(def)
	init := TableInit(def)
	if init == nil {
		return nil, fmt.Errorf("arrange rows of %s: not an initialised definition", name)
	}
	t := defType(def)
	if !t.Is("array") {
		return nil, fmt.Errorf("arrange rows of %s: not an array", name)
	}
	if len(t.Kids) > 2 {
		return nil, fmt.Errorf("arrange rows of %s: its size is written; say it with its rows", name)
	}
	old := init.Args()
	mine := map[*Node]int{}
	for i, r := range old {
		mine[r] = i
	}
	done := &RowsDone{}
	seen := map[*Node]bool{}
	for _, r := range order {
		if seen[r] {
			return nil, fmt.Errorf("arrange rows of %s: #%d (%s) twice", name, r.ID, label(r))
		}
		seen[r] = true
		if _, ok := mine[r]; ok {
			continue
		}
		if e.Live(r) {
			return nil, fmt.Errorf("arrange rows of %s: #%d (%s) is held elsewhere in the graph", name, r.ID, label(r))
		}
		done.Inserted = append(done.Inserted, r)
	}
	for _, r := range old {
		if !seen[r] {
			done.Deleted = append(done.Deleted, r)
		}
	}
	// the positions, before and after
	vals := e.EnumValues()
	env := map[string]int64{}
	for n, v := range vals {
		env[EnumeratorName(n)] = v
	}
	oldPos, oldOK, err := rowPositions(old, env)
	if err != nil {
		return nil, fmt.Errorf("arrange rows of %s: %w", name, err)
	}
	newPos, newOK, err := rowPositions(order, env)
	if err != nil {
		return nil, fmt.Errorf("arrange rows of %s: %w", name, err)
	}
	// what each old position becomes: -1 deleted, -2 not known
	moveTo := map[int64]int64{}
	for i, r := range old {
		if !oldOK[i] {
			continue
		}
		moveTo[oldPos[i]] = -1
		if !seen[r] {
			continue
		}
		for j, s := range order {
			if s == r {
				if newOK[j] {
					moveTo[oldPos[i]] = newPos[j]
				} else {
					moveTo[oldPos[i]] = -2
				}
			}
		}
	}
	for p, q := range moveTo {
		if q >= 0 && q != p {
			done.Moved++
		}
	}
	// where a constant position goes; an error when it cannot be said
	say := func(k int64, what string) (int64, error) {
		q, ok := moveTo[k]
		switch {
		case !ok && done.Moved == 0 && len(done.Inserted) == 0:
			return k, nil
		case !ok:
			return 0, fmt.Errorf("%s names position %d, which no row of %s has", what, k, name)
		case q == -1:
			return 0, fmt.Errorf("%s names the row at %d, which goes", what, k)
		case q == -2:
			return 0, fmt.Errorf("%s names the row at %d, whose new position cannot be computed", what, k)
		}
		return q, nil
	}
	ent := e.tableEntity(def)
	type site struct {
		at *Node
		to int64
	}
	var sites []site
	var serr error
	for _, f := range e.g.Forms {
		Walk(f, func(n *Node) bool {
			if serr != nil {
				return false
			}
			if !n.Is("index") || len(n.Kids) < 3 || n.Kids[1].list || !ent[n.Kids[1].Ref()] {
				return true
			}
			k := n.Kids[2]
			if k.list || !isDecimalInt(k.Atom) {
				return true
			}
			v, _ := parseInt(k.Atom)
			q, err := say(v, fmt.Sprintf("%s[%d]", name, v))
			if err != nil {
				serr = err
				return false
			}
			if q != v {
				sites = append(sites, site{k, q})
			}
			return true
		})
	}
	if serr != nil {
		return nil, fmt.Errorf("arrange rows of %s: %w", name, serr)
	}
	for _, a := range ix.Arrays {
		ai := TableInit(a)
		if ai == nil || !e.Live(a) {
			return nil, fmt.Errorf("arrange rows of %s: the permutation #%d (%s) is not an initialised definition in the graph", name, a.ID, label(a))
		}
		for _, k := range ai.Args() {
			if k.list || !isDecimalInt(k.Atom) {
				return nil, fmt.Errorf("arrange rows of %s: %s holds %s, not a decimal position", name, topName(a), label(k))
			}
			v, _ := parseInt(k.Atom)
			q, err := say(v, fmt.Sprintf("%s's element %d", topName(a), v))
			if err != nil {
				return nil, fmt.Errorf("arrange rows of %s: %w", name, err)
			}
			if q != v {
				sites = append(sites, site{k, q})
			}
		}
	}
	// the index enumerators: each enum's new order and values, held
	byEnum := map[*Node][]*Node{}
	var enums []*Node
	target := map[*Node]int64{}
	goneEn := map[*Node]bool{}
	for _, n := range ix.Enumerators {
		en, err := e.enumOf(n)
		if err != nil {
			return nil, fmt.Errorf("arrange rows of %s: %w", name, err)
		}
		v, ok := vals[n]
		if !ok {
			return nil, fmt.Errorf("arrange rows of %s: the index %s has no value that can be computed", name, EnumeratorName(n))
		}
		q, ok := moveTo[v]
		if ok && q == -1 {
			goneEn[n] = true
		} else if q, err = say(v, "the index "+EnumeratorName(n)); err != nil {
			return nil, fmt.Errorf("arrange rows of %s: %w", name, err)
		}
		target[n] = q
		if _, ok := byEnum[en]; !ok {
			enums = append(enums, en)
		}
		byEnum[en] = append(byEnum[en], n)
	}
	over := map[*Node][]*Node{}
	pin := map[*Node]int64{}
	var emptied []*Node
	for _, en := range enums {
		var keep []*Node
		for _, n := range Enumerators(en) {
			if !goneEn[n] {
				keep = append(keep, n)
			}
		}
		over[en] = keep
		if len(keep) == 0 {
			if e.Parent(en) != nil || tagOf(en) != "" {
				return nil, fmt.Errorf("arrange rows of %s: deleting its index enumerators would leave %s empty, and it is not an untagged declaration of its own", name, enumLabel(en))
			}
			emptied = append(emptied, en)
			continue
		}
		for _, n := range byEnum[en] {
			if goneEn[n] || target[n] == vals[n] {
				continue
			}
			if x := enumValue(n); x != nil {
				if x.list || !isDecimalInt(x.Atom) && !isHexInt(x.Atom) {
					return nil, fmt.Errorf("arrange rows of %s: the index %s's value is %s, not a constant to write the new one over", name, EnumeratorName(n), label(x))
				}
				pin[n] = target[n]
			}
		}
	}
	after := e.enumValues(over, pin)
	for _, en := range e.enumForms() {
		list, ok := over[en]
		if !ok {
			list = Enumerators(en)
		}
		for _, n := range list {
			b, okb := vals[n]
			a, oka := after[n]
			if want, isIx := target[n]; isIx {
				if !oka || a != want {
					return nil, fmt.Errorf("arrange rows of %s: the index %s would be %d, not %d: its value is implicit; write it first", name, EnumeratorName(n), a, want)
				}
				if a != b {
					done.Renumbered = append(done.Renumbered, ValueMove{n, b, a})
				}
				continue
			}
			if okb && oka && a != b {
				return nil, fmt.Errorf("arrange rows of %s: %s would move from %d to %d, and it is not an index of the table", name, EnumeratorName(n), b, a)
			}
		}
	}
	// apply: the rows, the sites, the enumerators, the type
	if err := e.spliceAs("rows", init, 1, len(init.Kids), order, placeItem); err != nil {
		return nil, err
	}
	for _, s := range sites {
		if err := e.Replace(s.at, NewAtom(strconv.FormatInt(s.to, 10))); err != nil {
			return nil, err
		}
		done.Sites = append(done.Sites, s.at)
	}
	for _, en := range enums {
		for _, n := range byEnum[en] {
			if v, ok := pin[n]; ok {
				if err := e.Replace(enumValue(n), NewAtom(strconv.FormatInt(v, 10))); err != nil {
					return nil, err
				}
			}
		}
		if keep := over[en]; len(keep) > 0 && len(keep) < len(Enumerators(en)) {
			from := len(en.Kids) - len(Enumerators(en))
			if err := e.spliceAs("renum", en, from, len(en.Kids), keep, placeItem); err != nil {
				return nil, err
			}
		}
	}
	for _, n := range ix.Enumerators {
		if goneEn[n] {
			done.GoneEnumerators = append(done.GoneEnumerators, n)
		}
	}
	for _, en := range emptied {
		if err := e.Delete(en); err != nil {
			return nil, err
		}
		done.GoneDecls = append(done.GoneDecls, en)
	}
	e.resize(def, newPos, newOK)
	return done, nil
}

func isHexInt(s string) bool {
	if len(s) < 3 || s[0] != '0' || s[1] != 'x' && s[1] != 'X' {
		return false
	}
	_, ok := parseInt(s)
	return ok
}

// resize points the table's typed edge at the array of its new count, or
// clears it.
func (e *Editor) resize(def *Node, pos []int64, ok []bool) {
	t := def.Type
	if t == nil || !t.Is("array") || len(t.Kids) != 3 {
		return
	}
	var n int64
	for i := range pos {
		if !ok[i] {
			e.g.save(def)
			def.Type = nil
			e.untype(def)
			return
		}
		n = max(n, pos[i]+1)
	}
	if strconv.FormatInt(n, 10) == t.Kids[1].Atom {
		return
	}
	elem := t.Kids[2].Type
	for _, x := range e.g.Types {
		if x.Is("array") && len(x.Kids) == 3 && x.Kids[1].Atom == strconv.FormatInt(n, 10) && x.Kids[2].Type == elem {
			e.g.save(def)
			def.Type = x
			return
		}
	}
	e.g.save(def)
	def.Type = nil
	e.untype(def)
}

// DeleteRows deletes the rows of the table def, saying again what names the
// rows after them by position.
func (e *Editor) DeleteRows(def *Node, rows []*Node, ix RowIndex) (*RowsDone, error) {
	init := TableInit(def)
	if init == nil {
		return nil, fmt.Errorf("delete rows of #%d (%s): not an initialised definition", def.ID, label(def))
	}
	gone := map[*Node]bool{}
	for _, r := range rows {
		gone[r] = true
	}
	var order []*Node
	for _, r := range init.Args() {
		if gone[r] {
			delete(gone, r)
			continue
		}
		order = append(order, r)
	}
	for r := range gone {
		return nil, fmt.Errorf("delete rows of %s: #%d (%s) is not one of its rows", topName(def), r.ID, label(r))
	}
	return e.ArrangeRows(def, order, ix)
}

// InsertRows puts the new rows (BuildRows) before the row before, or at the
// end for nil.
func (e *Editor) InsertRows(def *Node, before *Node, rows []*Node, ix RowIndex) (*RowsDone, error) {
	init := TableInit(def)
	if init == nil {
		return nil, fmt.Errorf("insert rows in #%d (%s): not an initialised definition", def.ID, label(def))
	}
	var order []*Node
	put := false
	for _, r := range init.Args() {
		if r == before {
			order = append(order, rows...)
			put = true
		}
		order = append(order, r)
	}
	if before != nil && !put {
		return nil, fmt.Errorf("insert rows in %s: #%d (%s) is not one of its rows", topName(def), before.ID, label(before))
	}
	if before == nil {
		order = append(order, rows...)
	}
	return e.ArrangeRows(def, order, ix)
}

// BuildRows reads src, C-lisp initialiser elements -- `(init E...)`, `(at
// DESIGNATOR... V)` with `.member` and `(idx K)`, or an expression -- as the
// rows of the table def they will be: names resolved where the table is,
// members by the type of its elements, holes as Build's.
func (e *Editor) BuildRows(def *Node, src string, holes Bindings) ([]*Node, error) {
	p, i := e.index(def)
	if i < 0 {
		return nil, fmt.Errorf("build rows of #%d: not in the graph", def.ID)
	}
	forms, err := clisp.Read([]byte(src))
	if err != nil {
		return nil, fmt.Errorf("build rows: %w", err)
	}
	b := &builder{e: e, p: p, i: i, holes: holes, used: map[string]bool{}, local: map[string]*Node{}}
	elem := pointee(def.Type)
	var out []*Node
	for _, f := range forms {
		n := b.element(f, elem)
		if b.err != nil {
			return nil, fmt.Errorf("build rows %s: %w", f, b.err)
		}
		out = append(out, n)
	}
	for _, n := range b.untyped {
		if n.Type == nil {
			e.untype(n)
		}
	}
	return out, nil
}

// element is an initialiser element of type t (nil: not known).
func (b *builder) element(f *clisp.Node, t *Node) *Node {
	if !f.IsList() {
		return b.expr(f)
	}
	switch f.Head() {
	case "init":
		n := NewList(NewAtom("init"))
		ms := []*Node(nil)
		if t != nil && (t.Is("struct") || t.Is("union")) {
			ms = members(t)
		}
		k := 0
		for _, x := range f.List[1:] {
			var et *Node
			switch {
			case t != nil && t.Is("array"):
				et = pointee(t)
			case k < len(ms):
				et = ms[k].Type
			}
			if x.IsList() && x.Head() == "at" {
				et = t      // a designator is the list's own
				k = len(ms) // positions after one are not followed
			}
			el := b.element(x, et)
			b.e.g.save(n)
			n.Kids = append(n.Kids, el)
			k++
		}
		return n
	case "at":
		if len(f.List) < 3 {
			return b.fail("a designation is (at DESIGNATOR... VALUE)")
		}
		n := NewList(NewAtom("at"))
		cur := t
		for _, d := range f.List[1 : len(f.List)-1] {
			switch {
			case !d.IsList() && len(d.Atom) > 1 && d.Atom[0] == '.':
				m := memberNamed(cur, d.Atom[1:])
				if m == nil {
					return b.fail("no member `%s` in the row's type", d.Atom[1:])
				}
				b.e.g.save(n)
				n.Kids = append(n.Kids, &Node{Atom: d.Atom, Refs: []*Node{m}})
				cur = m.Type
			case d.IsList() && d.Head() == "idx" && (len(d.List) == 2 || len(d.List) == 3):
				x := NewList(NewAtom("idx"))
				for _, k := range d.List[1:] {
					b.e.g.save(x)
					x.Kids = append(x.Kids, b.expr(k))
				}
				b.e.g.save(n)
				n.Kids = append(n.Kids, x)
				cur = pointee(cur)
			default:
				return b.fail("a designator is .member or (idx K), not %s", d)
			}
		}
		b.e.g.save(n)
		n.Kids = append(n.Kids, b.element(f.List[len(f.List)-1], cur))
		return n
	}
	return b.expr(f)
}

// ---- the verbs

// table is the scope's table (InTable), refusing elsewhere.
func (v *Verbs) table(what string) *Node {
	if v.scope == nil || TableInit(v.scope) == nil {
		v.Die("%s -- the scope is not a table (InTable)", what)
		return nil
	}
	return v.scope
}

// DeleteRows deletes the n rows of the scope's table the pattern matches
// (rows only: the initialiser's elements, not what they hold), saying again
// what names a position (INITROW).
func (v *Verbs) DeleteRows(pat string, n int, ix RowIndex, what string) *RowsDone {
	if v.Err != nil {
		return nil
	}
	t := v.table(what)
	if t == nil {
		return nil
	}
	p := v.pattern(pat, what)
	if p == nil {
		return nil
	}
	var rows []*Node
	for _, r := range TableInit(t).Args() {
		if Matches(p, r) {
			rows = append(rows, r)
		}
	}
	if len(rows) != n {
		v.Die("%s -- %d rows match, expected %d", what, len(rows), n)
		return nil
	}
	done, err := v.e.DeleteRows(t, rows, ix)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	v.Say(what)
	return done
}

// DeleteRowsEach deletes, as one arrangement, the rows of the scope's table
// the patterns match, each exactly one row.
func (v *Verbs) DeleteRowsEach(pats []string, ix RowIndex, what string) *RowsDone {
	if v.Err != nil {
		return nil
	}
	t := v.table(what)
	if t == nil {
		return nil
	}
	var rows []*Node
	for _, pat := range pats {
		r := v.Row(pat, what+": "+pat)
		if r == nil {
			return nil
		}
		rows = append(rows, r)
	}
	done, err := v.e.DeleteRows(t, rows, ix)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	v.Say(what)
	return done
}

// InsertRows puts the rows the template makes before the one row the
// pattern matches ("" : at the end) of the scope's table.
func (v *Verbs) InsertRows(before, tmpl string, ix RowIndex, what string) *RowsDone {
	if v.Err != nil {
		return nil
	}
	t := v.table(what)
	if t == nil {
		return nil
	}
	var at *Node
	if before != "" {
		if at = v.Row(before, what); at == nil {
			return nil
		}
	}
	rows, err := v.e.BuildRows(t, tmpl, nil)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	done, err := v.e.InsertRows(t, at, rows, ix)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	v.Say(what)
	return done
}
