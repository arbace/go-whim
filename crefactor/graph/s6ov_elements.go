package graph

import (
	"fmt"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// MEMBERS AND ELEMENTS ADDED (doc/GRAPH-MIGRATION.md, Step6 as built): what
// a struct that gains a member, and the positional initialisers of it that
// gain an element, need -- B3f wrote such a struct whole by FRAG (a member is
// no FRAG spot), which left the typed edges of every declaration naming the
// old definition at its type node; here the definition keeps its node, so
// every typed edge to it stays the import's.
//
//   - InsertMember adds a member, read from C-lisp `(NAME TYPE)` at the
//     definition's place, typed by its form as the importer types it (the
//     type nodes the graph lacks made and interned), beside an existing
//     member.  Refused: a name the struct has, a form whose type does not
//     follow (typeof, a computed size), and a place before the last
//     element of any positional initialiser of the struct -- an element
//     would then initialise another member.
//   - InsertElements adds elements to an initialiser list `(init ...)` at
//     position i: an array's (a row) or a struct's (refused past its
//     members, and after a designated element, whose successors' places
//     the designator says).

// InsertMember adds the member src, `(NAME TYPE)`, after (or before) the
// member at, and returns it.
func (e *Editor) InsertMember(at *Node, after bool, src string) (*Node, error) {
	def := e.Parent(at)
	if def == nil || !(def.Is("struct") || def.Is("union")) || !isMemberForm(e, at) {
		return nil, fmt.Errorf("insert member: #%d is not a member of a definition", at.ID)
	}
	forms, err := clisp.Read([]byte(src))
	if err != nil || len(forms) != 1 || !forms[0].IsList() || len(forms[0].List) < 2 || forms[0].List[0].IsList() {
		return nil, fmt.Errorf("insert member: %q is not one (NAME TYPE)", src)
	}
	name := forms[0].List[0].Atom
	if memberNamed(def, name) != nil {
		return nil, fmt.Errorf("insert member: %s already has a member %s", label(def), name)
	}
	ms := members(def)
	pos := indexIn(ms, at)
	if after {
		pos++
	}
	if def.Is("struct") {
		var bad error
		for _, f := range e.g.Forms {
			Walk(f, func(x *Node) bool {
				if bad == nil && x.Is("init") && e.initOf(x) == def && len(x.Kids)-1 > pos {
					bad = fmt.Errorf("insert member %s: a positional initialiser of %s has %d elements, so one would initialise it", name, label(def), len(x.Kids)-1)
				}
				return bad == nil
			})
		}
		if bad != nil {
			return nil, bad
		}
	}
	// the member's node, as the importer makes a member declaration
	b := &builder{e: e, p: def, i: indexIn(def.Kids, at), used: map[string]bool{}, local: map[string]*Node{}}
	m := NewList(NewAtom(name))
	for _, f := range forms[0].List[1:] {
		e.g.save(m)
		m.Kids = append(m.Kids, b.typ(f))
	}
	if b.err != nil {
		return nil, fmt.Errorf("insert member %s: %w", name, b.err)
	}
	tx := e.typeTx()
	t := tx.formType(m.Kids[1], false)
	if t == nil {
		return nil, fmt.Errorf("insert member %s: its type does not follow from its form", name)
	}
	e.g.save(m)
	m.Type = t
	i := indexIn(def.Kids, at)
	if after {
		i++
	}
	tx.commit()
	if err := e.spliceAs("insert", def, i, i, []*Node{m}, placeItem); err != nil {
		return nil, err
	}
	return m, nil
}

// initOf is the struct or union definition whose members an initialiser
// list's elements fill positionally, or nil.
func (e *Editor) initOf(x *Node) *Node {
	if t := e.initType(x); t.Is("struct") || t.Is("union") {
		return t
	}
	return nil
}

// initType is the type an initialiser list initialises, when known.
func (e *Editor) initType(x *Node) *Node {
	p := e.Parent(x)
	switch {
	case p == nil:
		return nil
	case p.Is("def"):
		return p.Type
	case p.Is("init"):
		pt := e.initType(p)
		switch {
		case pt.Is("array"):
			return pointee(pt)
		case pt.Is("struct"):
			k := indexIn(p.Kids, x) - 1
			ms := members(pt)
			if k >= 0 && k < len(ms) {
				return ms[k].Type
			}
		}
	}
	return nil
}

// InsertElements puts els into the initialiser list init before its i-th
// element (i == its length: at the end).
func (e *Editor) InsertElements(init *Node, i int, els ...*Node) error {
	if !init.Is("init") || !e.Live(init) {
		return fmt.Errorf("insert elements: #%d is not an initialiser list of the graph", init.ID)
	}
	n := len(init.Kids) - 1
	if i < 0 || i > n {
		return fmt.Errorf("insert elements: #%d has %d elements, none at %d", init.ID, n, i)
	}
	for _, k := range init.Kids[1 : i+1] {
		if k.Is("at") {
			return fmt.Errorf("insert elements: #%d has a designated element before %d", init.ID, i)
		}
	}
	if t := e.initType(init); t.Is("struct") && n+len(els) > len(members(t)) {
		return fmt.Errorf("insert elements: #%d would have %d elements, its struct %d members", init.ID, n+len(els), len(members(t)))
	}
	return e.spliceAs("insert", init, i+1, i+1, els, placeItem)
}

// ReplaceElement replaces an initialiser's element old by el, as Replace
// does; an initialiser list el is not listed in Untyped, since the importer
// types no initialiser list (only the expressions in it).
func (e *Editor) ReplaceElement(old, el *Node) error {
	p := e.Parent(old)
	if p == nil || !p.Is("init") {
		return fmt.Errorf("replace element: #%d is not an initialiser's element", old.ID)
	}
	if err := e.Replace(old, el); err != nil {
		return err
	}
	if el.Is("init") {
		e.typed(el)
	}
	return nil
}

// RederiveValue is Rederive from a selection n whose member is an array,
// typed as cc types such an expression where it is a value -- a pointer
// to the element: the operand of `[]`, an argument -- and as the array
// only under sizeof, _Alignof and `&`.  It says how many it typed.
func (e *Editor) RederiveValue(n *Node) int {
	if !(n.Is(".") || n.Is("->")) {
		return e.Rederive(n)
	}
	m := n.Args()[len(n.Args())-1].Ref()
	if m == nil || m.Type == nil || !m.Type.Is("array") {
		return e.Rederive(n)
	}
	t := m.Type
	switch p := e.Parent(n); {
	case p.Is("sizeof"), p.Is("sizeof-bare"), p.Is("alignof"), p.Is("alignof-bare"), p.Is("addr"):
	default:
		tx := e.typeTx()
		t = tx.pointer(pointee(t))
		tx.commit()
	}
	if n.Type == t {
		e.typed(n)
		return 0
	}
	e.g.save(n)
	n.Type = t
	e.typed(n)
	return 1 + e.Rederive(e.Parent(n))
}
