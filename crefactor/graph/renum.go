package graph

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// RENUM (doc/GRAPH-MIGRATION.md, B2b): an enumeration's members deleted,
// moved, inserted -- with what that does to the values of the ones that
// stay said, and opted into.  The editor's Delete refuses an enumerator
// whose successor's value is implicit, since deleting it would move that
// value; ArrangeEnum is where a cut says what it wants instead:
//
//   - HoldValues: no enumerator of the file may change its value.  A set
//     of enumerators deleted together (a run and the implicit ones after
//     it, deleted last-first one by one with Delete) is one act here.
//   - Renumber: the implicit values move, as a text program's deletion of
//     the line moves them (`CMD_index` follows the rows the front cut);
//     every value that moved, in any enum of the file, is reported.
//   - PinValues: a survivor whose implicit value would move is written
//     with the value it had -- the sweep's rule, `= 4` after a deleted run
//     (crefactor/sweep's Prune, crefactor/graph's Collect), spelled as they
//     spell it: decimal, hexadecimal from 65536 up.
//
// The values are the sweep's: every enumerator of the file evaluated in
// the file's order, by name, as Prune evaluates its text (evalForm).  A
// value that cannot be computed is not an answer: a policy that needs it
// refuses.
type Values int

const (
	// HoldValues refuses an edit that would move any enumerator's value.
	HoldValues Values = iota
	// Renumber lets the implicit values move, and reports them.
	Renumber
	// PinValues writes a survivor whose implicit value would move with the
	// value it had.
	PinValues
)

func (v Values) String() string {
	return [...]string{"hold", "renumber", "pin"}[v]
}

// A ValueMove is an enumerator whose value an edit changed.
type ValueMove struct {
	Enumerator *Node
	Old, New   int64
}

// Renumbered is what an arrangement of an enum did.
type Renumbered struct {
	Deleted  []*Node     // the enumerators it removed
	Inserted []*Node     // the enumerators it brought in
	Moved    []ValueMove // the enumerators whose value changed (Renumber), in the file's order
	Pinned   []*Node     // the survivors written with the value they had (PinValues)
}

// IsEnumerator says n is an enumerator of an enum's definition.
func (e *Editor) IsEnumerator(n *Node) bool {
	p := e.Parent(n)
	if p == nil || !p.Is("enum") || !n.list {
		return false
	}
	for _, x := range body(p) {
		if x == n {
			return true
		}
	}
	return false
}

// Enumerators is an enum definition's enumerators, in order.
func Enumerators(enum *Node) []*Node {
	var out []*Node
	for _, x := range body(enum) {
		if x.list && !x.Is("@") {
			out = append(out, x)
		}
	}
	return out
}

// EnumeratorName is an enumerator's name.
func EnumeratorName(n *Node) string {
	if len(n.Kids) == 0 || n.Kids[0].list {
		return ""
	}
	return n.Kids[0].Atom
}

// NewEnumerator is `NAME` (value nil) or `NAME = value`, to be put in an
// enum by ArrangeEnum or InsertEnumerators.
func NewEnumerator(name string, value *Node) *Node {
	if value == nil {
		return NewList(NewAtom(name))
	}
	return NewList(NewAtom(name), value)
}

// enumForms are the enum definitions of the file outside function bodies,
// in the file's order: where Prune and Collect find their enumerators.
func (e *Editor) enumForms() []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		if f.Is("defn") {
			continue
		}
		Walk(f, func(n *Node) bool {
			if n.Is("enum") && isDefForm(n) {
				out = append(out, n)
			}
			return true
		})
	}
	return out
}

// enumValues evaluates every enumerator of the file, in the file's order,
// as Collect does; over, when given, stands in for an enum's enumerators,
// and pin for the value an enumerator is written with.  Unknown values are
// absent.
func (e *Editor) enumValues(over map[*Node][]*Node, pin map[*Node]int64) map[*Node]int64 {
	env := map[string]int64{}
	out := map[*Node]int64{}
	for _, en := range e.enumForms() {
		list, ok := over[en]
		if !ok {
			list = Enumerators(en)
		}
		var prev int64 = -1
		known := true
		for _, n := range list {
			if v, ok := pin[n]; ok {
				prev, known = v, true
			} else if x := enumValue(n); x != nil {
				prev, known = evalForm(x, env)
			} else {
				prev++
			}
			if known {
				out[n] = prev
				env[EnumeratorName(n)] = prev
			}
		}
	}
	return out
}

// EnumValues is every enumerator's value, as the sweep computes it; one
// that cannot be computed is absent.
func (e *Editor) EnumValues() map[*Node]int64 { return e.enumValues(nil, nil) }

// ArrangeEnum makes enum's enumerators order: those of enum it holds stay
// (moved, keeping their ids), those it leaves out are deleted (their uses
// left dangling, for the closure), new ones (NewEnumerator, not in the
// graph) are inserted with fresh ids.  how says what may happen to the
// values of the enumerators that stay, in this enum and every other.
func (e *Editor) ArrangeEnum(enum *Node, order []*Node, how Values) (*Renumbered, error) {
	if !e.Live(enum) || !enum.Is("enum") || !isDefForm(enum) {
		return nil, fmt.Errorf("arrange #%d (%s): not an enum's definition in the graph", enum.ID, label(enum))
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("arrange enum %s: an enum keeps one enumerator at least; delete its declaration", enumLabel(enum))
	}
	old := Enumerators(enum)
	mine := map[*Node]bool{}
	for _, n := range old {
		mine[n] = true
	}
	r := &Renumbered{}
	seen := map[*Node]bool{}
	names := map[string]bool{}
	for _, n := range order {
		switch {
		case seen[n]:
			return nil, fmt.Errorf("arrange enum %s: %s twice", enumLabel(enum), EnumeratorName(n))
		case mine[n]:
		case e.Live(n):
			return nil, fmt.Errorf("arrange enum %s: #%d (%s) is held elsewhere in the graph", enumLabel(enum), n.ID, label(n))
		default:
			name := EnumeratorName(n)
			if !isIdent(name) || len(n.Kids) > 2 {
				return nil, fmt.Errorf("arrange enum %s: a new enumerator is (NAME) or (NAME VALUE), not %s", enumLabel(enum), label(n))
			}
			if ds := e.Decls(name); len(ds) > 0 {
				return nil, fmt.Errorf("arrange enum %s: %s is declared already (#%d)", enumLabel(enum), name, ds[0].ID)
			}
			r.Inserted = append(r.Inserted, n)
		}
		if names[EnumeratorName(n)] {
			return nil, fmt.Errorf("arrange enum %s: two enumerators named %s", enumLabel(enum), EnumeratorName(n))
		}
		names[EnumeratorName(n)] = true
		seen[n] = true
	}
	for _, n := range old {
		if !seen[n] {
			r.Deleted = append(r.Deleted, n)
		}
	}
	// the values, before and as they would be
	before := e.enumValues(nil, nil)
	over := map[*Node][]*Node{enum: order}
	pin := map[*Node]int64{}
	if how == PinValues {
		// walk the new order: a survivor whose implicit value follows
		// another enumerator than it did -- the one after a deleted run, the
		// sweep's rule, or after a move or an insertion -- or would move, is
		// written with the one it had, and the implicit ones after it follow
		oldPrev := map[*Node]*Node{}
		for i, n := range old {
			if i > 0 {
				oldPrev[n] = old[i-1]
			}
		}
		for i, n := range order {
			if !mine[n] || enumValue(n) != nil {
				continue
			}
			var prev *Node
			if i > 0 {
				prev = order[i-1]
			}
			if prev == oldPrev[n] {
				after := e.enumValues(over, pin)
				if a, ok := after[n]; ok && a == before[n] {
					continue
				}
			}
			v, ok := before[n]
			if !ok {
				return nil, fmt.Errorf("arrange enum %s: %s's value cannot be computed, and it cannot be pinned", enumLabel(enum), EnumeratorName(n))
			}
			pin[n] = v
		}
	}
	after := e.enumValues(over, pin)
	for _, en := range e.enumForms() {
		list := Enumerators(en)
		if en == enum {
			list = order
		}
		for _, n := range list {
			b, okb := before[n]
			a, oka := after[n]
			if !okb || !oka || a == b {
				continue
			}
			if how != Renumber {
				return nil, fmt.Errorf("arrange enum %s: the value of %s would move from %d to %d (%s)",
					enumLabel(enum), EnumeratorName(n), b, a, how)
			}
			r.Moved = append(r.Moved, ValueMove{n, b, a})
		}
	}
	// apply: the pins in place, then the enumerators as one splice
	for _, n := range order {
		v, ok := pin[n]
		if !ok {
			continue
		}
		a := NewAtom(formatValue(v))
		a.up = n
		n.Kids = append(n.Kids, a)
		r.Pinned = append(r.Pinned, n)
	}
	if len(r.Pinned) > 0 {
		act := Act{Op: "pin"}
		for _, n := range r.Pinned {
			act.Moved = append(act.Moved, n.ID)
		}
		e.Log = append(e.Log, act)
	}
	from := len(enum.Kids) - len(old)
	if err := e.spliceAs("renum", enum, from, len(enum.Kids), order, placeItem); err != nil {
		return nil, err
	}
	return r, nil
}

func enumLabel(enum *Node) string {
	if t := tagOf(enum); t != "" {
		return t
	}
	if es := Enumerators(enum); len(es) > 0 {
		return "{" + EnumeratorName(es[0]) + ", ...}"
	}
	return "{}"
}

// enumOf is the enum definition holding the enumerator n.
func (e *Editor) enumOf(n *Node) (*Node, error) {
	if !e.IsEnumerator(n) {
		return nil, fmt.Errorf("#%d (%s) is not an enumerator in the graph", n.ID, label(n))
	}
	return e.Parent(n), nil
}

// DeleteEnumerators deletes the enumerators ens, of one enum or several, as
// one arrangement of each.  An enum they would leave empty is refused: its
// declaration is the cut's to delete.
func (e *Editor) DeleteEnumerators(ens []*Node, how Values) (*Renumbered, error) {
	gone := map[*Node]bool{}
	var enums []*Node
	for _, n := range ens {
		en, err := e.enumOf(n)
		if err != nil {
			return nil, fmt.Errorf("delete enumerators: %w", err)
		}
		if !gone[en] {
			enums = append(enums, en)
		}
		gone[en] = true
		gone[n] = true
	}
	all := &Renumbered{}
	for _, en := range enums {
		var keep []*Node
		for _, n := range Enumerators(en) {
			if !gone[n] {
				keep = append(keep, n)
			}
		}
		r, err := e.ArrangeEnum(en, keep, how)
		if err != nil {
			return nil, err
		}
		all.add(r)
	}
	return all, nil
}

// MoveEnumerators takes the enumerators ens out of their places and puts
// them, in that order, after the enumerator after, in the same enum.
func (e *Editor) MoveEnumerators(ens []*Node, after *Node, how Values) (*Renumbered, error) {
	en, err := e.enumOf(after)
	if err != nil {
		return nil, fmt.Errorf("move enumerators: %w", err)
	}
	moving := map[*Node]bool{}
	for _, n := range ens {
		m, err := e.enumOf(n)
		if err != nil {
			return nil, fmt.Errorf("move enumerators: %w", err)
		}
		if m != en {
			return nil, fmt.Errorf("move enumerators: %s is not in enum %s", EnumeratorName(n), enumLabel(en))
		}
		if n == after {
			return nil, fmt.Errorf("move enumerators: %s after itself", EnumeratorName(n))
		}
		moving[n] = true
	}
	var order []*Node
	for _, n := range Enumerators(en) {
		if moving[n] {
			continue
		}
		order = append(order, n)
		if n == after {
			order = append(order, ens...)
		}
	}
	return e.ArrangeEnum(en, order, how)
}

// InsertEnumerators puts the new enumerators ens (NewEnumerator) after the
// enumerator after.
func (e *Editor) InsertEnumerators(after *Node, ens []*Node, how Values) (*Renumbered, error) {
	en, err := e.enumOf(after)
	if err != nil {
		return nil, fmt.Errorf("insert enumerators: %w", err)
	}
	var order []*Node
	for _, n := range Enumerators(en) {
		order = append(order, n)
		if n == after {
			order = append(order, ens...)
		}
	}
	return e.ArrangeEnum(en, order, how)
}

func (r *Renumbered) add(o *Renumbered) {
	r.Deleted = append(r.Deleted, o.Deleted...)
	r.Inserted = append(r.Inserted, o.Inserted...)
	r.Moved = append(r.Moved, o.Moved...)
	r.Pinned = append(r.Pinned, o.Pinned...)
}

// String says what moved, for a report: `3 deleted, 89 values moved`.
func (r *Renumbered) String() string {
	var parts []string
	for _, p := range []struct {
		n    int
		what string
	}{{len(r.Deleted), "deleted"}, {len(r.Inserted), "inserted"}, {len(r.Moved), "values moved"}, {len(r.Pinned), "pinned"}} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.what))
		}
	}
	if len(parts) == 0 {
		return "nothing moved"
	}
	return strings.Join(parts, ", ")
}

// ---- the verbs

// enumeratorsNamed are the file's enumerators of the names, in that order,
// each declared once.
func (v *Verbs) enumeratorsNamed(names []string, what string) []*Node {
	var out []*Node
	for _, name := range names {
		var ens []*Node
		for _, d := range v.e.Decls(name) {
			if v.e.IsEnumerator(d) {
				ens = append(ens, d)
			}
		}
		if len(ens) != 1 {
			v.Die("%s -- %d enumerators named %s, expected 1", what, len(ens), name)
			return nil
		}
		out = append(out, ens[0])
	}
	return out
}

// DeleteEnumerators deletes the file's enumerators of the names, the values
// of the rest as how says (RENUM).
func (v *Verbs) DeleteEnumerators(names []string, how Values, what string) *Renumbered {
	if v.Err != nil {
		return nil
	}
	ens := v.enumeratorsNamed(names, what)
	if ens == nil {
		return nil
	}
	r, err := v.e.DeleteEnumerators(ens, how)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	v.Say(what)
	return r
}

// MoveEnumerators puts the file's enumerators of the names after the one
// named after, in their enum, the values of the rest as how says.
func (v *Verbs) MoveEnumerators(names []string, after string, how Values, what string) *Renumbered {
	if v.Err != nil {
		return nil
	}
	ens := v.enumeratorsNamed(append(append([]string(nil), names...), after), what)
	if ens == nil {
		return nil
	}
	r, err := v.e.MoveEnumerators(ens[:len(names)], ens[len(names)], how)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	v.Say(what)
	return r
}

// BuildValue reads src, one C-lisp expression, as the value of an
// enumerator to be put after the enumerator after: Build's names, and the
// enumerators of after's enum up to it, which C makes visible there.
func (e *Editor) BuildValue(after *Node, src string) (*Node, error) {
	en, err := e.enumOf(after)
	if err != nil {
		return nil, fmt.Errorf("build a value: %w", err)
	}
	top := en
	for p := e.Parent(top); p != nil; p = e.Parent(top) {
		top = p
	}
	p, i := e.index(top)
	forms, err := clisp.Read([]byte(src))
	if err != nil || len(forms) != 1 {
		return nil, fmt.Errorf("build a value: %s is not one form (%v)", src, err)
	}
	b := &builder{e: e, p: p, i: i, used: map[string]bool{}, local: map[string]*Node{}}
	for _, n := range Enumerators(en) {
		b.local[EnumeratorName(n)] = n
		if n == after {
			break
		}
	}
	n := b.expr(forms[0])
	if b.err != nil {
		return nil, fmt.Errorf("build %s: %w", src, b.err)
	}
	for _, x := range b.untyped {
		if x.Type == nil {
			e.untype(x)
		}
	}
	return n, nil
}
