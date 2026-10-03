package graph

import (
	"fmt"
	"sort"
	"strings"
)

// INCLUDE AND THE LINE (doc/GRAPH-MIGRATION.md, B2e).  An `#include` is a
// top-level form, `(include "<stdio.h>")`, with an id like every list.
// THE FIRST INCLUDE FORM IS A LINE: the forms above it are the CORE, the
// forms from it on the HOST -- a convention of the code base the graph is
// told nothing about, only that the first include form is where a file's
// own code stops being able to name a header's (crefactor/graph names
// nothing in vim).  A graph with no include form has no line: every form is
// the core's.
//
// THE RULE AN INCLUDE EDIT KEEPS (the extern rule).  What the file takes
// from the headers is of two kinds:
//
//   - a DECLARATION: an external node (`(extern getenv)`, `(extern-typedef
//     time_t)`, `(extern-struct termios ...)`, an `extern-enumerator`) with
//     a live use that spells it -- an atom spelled its name, or the
//     `(struct NAME)` that names its tag.  Its uses through a macro (a
//     `(macro "...")` form, or an atom spelled otherwise, `errno` referring
//     to `__errno_location`) are the macro's business;
//   - a MACRO: a token the file says that some include's header defines as
//     a macro -- an identifier atom with no refers edge that is not a name
//     the file declares, an atom whose edge goes to an external node spelled
//     otherwise, or an identifier in a macro invocation's text.  C23's
//     keywords are the language's, even where a header defines them for an
//     older C (`bool` in <stdbool.h>).
//
// A name is PROVIDED where an include form above its first use has a
// header (HeaderOf: the header parsed by cc on its own) that declares it --
// or defines it, for a macro.  The compiler's own names (`__builtin_*`,
// what a unit with no include declares or defines) need no header.  And an
// include's macros must not take a name the file has as its own below it --
// a COLLISION: `INT_MAX` declared by the file as an enumerator below
// <limits.h> is that header's macro there, a syntax error; a use of the
// file's own `INT_MAX` below it would silently become the macro.
//
// An edit refuses where it would leave a name that was provided unprovided,
// or make a collision that was not there: the rule is held RELATIVE to the
// graph before the edit, so a graph that already says a name no include
// provides is not refused for it, only never made worse.  So:
//
//   - a removed include's names still used must come from another include
//     above their first use, or the removal is refused, naming them;
//   - an added include's macros must not collide with the file's names
//     below it;
//   - a form moved above an include (into the core, above the line) must
//     not take with it a use of what only that include provides.
//
// Two edits go further on purpose, each where a token changes what it
// names and the program still compiles -- what a compiler's silence
// accepts: DeleteIncludeRebind, where a removed header's macro is a name
// the file declares itself (its tokens become uses of the file's
// declaration), and InsertIncludeRebind, its inverse (the file's uses below
// the new include become the macro's tokens).  Each returns the tokens it
// changed.
//
// The external nodes stay what they are: a name a removed header provided
// and another header provides still refers to its external node, typed as
// cc typed it at import.  That the two headers' declarations of it agree is
// not checked (cc's check of the file would; the graph does not re-check).
//
// A MOVE of top-level forms keeps their ids (an act `move`, every id moved)
// and keeps C's declare-before-use: a refers edge whose declaration was in a
// form above its use's form stays so, or the move is refused -- and the
// rule above. That is MOVE's rule for top-level forms, made here for the
// line (B2c's MOVE is the general one: items in blocks, runs, bodies).

// IsInclude says n is an include form: `(include "<stdio.h>")`.
func IsInclude(n *Node) bool {
	return n.Is("include") && len(n.Kids) == 2 && !n.Kids[1].list
}

// IncludeSpec is an include form's operand as written: `<stdio.h>`.
func IncludeSpec(n *Node) string {
	if !IsInclude(n) {
		return ""
	}
	return unquote(n.Kids[1].Atom)
}

// NewInclude is a new include form for spec, `<stdio.h>` or `"x.h"`: its
// id is given where it is inserted.
func NewInclude(spec string) *Node {
	return NewList(NewAtom("include"), NewAtom(quoteAtom(spec)))
}

// quoteAtom writes s as a C string literal's atom, as C-lisp quotes.
func quoteAtom(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Includes are the file's include forms, in order.
func (e *Editor) Includes() []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		if IsInclude(f) {
			out = append(out, f)
		}
	}
	return out
}

// FirstInclude is the first include form: the line between the core above
// it and the host. nil when the file has none.
func (e *Editor) FirstInclude() *Node {
	for _, f := range e.g.Forms {
		if IsInclude(f) {
			return f
		}
	}
	return nil
}

// TopForm is the top-level form n is in (n itself, for one); nil for a
// node of the type or external sections, or one not in the graph.
func (e *Editor) TopForm(n *Node) *Node {
	if !e.Live(n) {
		return nil
	}
	for n.up != nil && !e.isTop(n.up) {
		n = n.up
	}
	if n.up != e.top[0] {
		return nil
	}
	return n
}

// lineAt is the first include form's index in the forms, len(Forms) when
// there is none.
func (e *Editor) lineAt() int {
	for i, f := range e.g.Forms {
		if IsInclude(f) {
			return i
		}
	}
	return len(e.g.Forms)
}

// Core is the forms above the first include form: all of them when there
// is none.
func (e *Editor) Core() []*Node { return e.g.Forms[:e.lineAt()] }

// Host is the forms from the first include form on, the includes with
// them: none when there is no include form.
func (e *Editor) Host() []*Node { return e.g.Forms[e.lineAt():] }

// InCore says n is in a form above the line; InHost, at or below it.  A node
// not in the forms is in neither.
func (e *Editor) InCore(n *Node) bool {
	f := e.TopForm(n)
	return f != nil && e.formAt(f) < e.lineAt()
}

// InHost says n is in a form at or below the line: the first include
// form, or one after it.
func (e *Editor) InHost(n *Node) bool {
	f := e.TopForm(n)
	return f != nil && e.formAt(f) >= e.lineAt()
}

// formAt is a top-level form's index, -1 when it is not one.
func (e *Editor) formAt(f *Node) int {
	for i, x := range e.g.Forms {
		if x == f {
			return i
		}
	}
	return -1
}

// A HeaderUse is a name the file takes from the headers: where it is first
// used, and the include above that which provides it (nil: none does).
type HeaderUse struct {
	Name  string // an ordinary name, `struct NAME` for a tag, or a macro's
	Macro bool   // a macro's name, not a declaration's
	First *Node  // the first top-level form that uses it
	From  *Node  // the first include form above First whose header provides it
}

func (u HeaderUse) String() string {
	kind := "declaration"
	if u.Macro {
		kind = "macro"
	}
	where := "?"
	if u.First != nil {
		where = label(u.First)
	}
	return fmt.Sprintf("%s `%s`, first used in %s", kind, u.Name, where)
}

// A Collision is an include's macro over a name the file declares below it.
type Collision struct {
	Include *Node
	Name    string
	Form    *Node // the first form below the include that declares or names it as its own
	Use     bool  // the file only uses the name below the include (it declares it above)
}

func (c Collision) String() string {
	what := "declares"
	if c.Use {
		what = "uses as its own declaration's"
	}
	where := "a form"
	if c.Form != nil {
		where = label(c.Form)
	}
	return fmt.Sprintf("%s defines `%s` as a macro, which %s below it %s", IncludeSpec(c.Include), c.Name, where, what)
}

// HeaderUses are the names the file takes from the headers, in the order
// of their first use, each with the include that provides it.
func (e *Editor) HeaderUses() ([]HeaderUse, error) {
	s, err := e.headerState()
	if err != nil {
		return nil, err
	}
	var out []HeaderUse
	for _, u := range s.uses(nil) {
		if u.From == nil && strings.Contains(u.Name, " ") && !s.anyProvides(u.Name) {
			continue // a tag no header has: the file's own incomplete type
		}
		out = append(out, u)
	}
	return out, nil
}

// anyProvides says some include of the file declares name.
func (s *headerState) anyProvides(name string) bool {
	for _, inc := range s.incs {
		if s.hdr[inc].Names[name] {
			return true
		}
	}
	return false
}

// Missing are the names that are provided now and would not be with the
// include forms without gone: what deleting them would break.
func (e *Editor) Missing(without ...*Node) ([]HeaderUse, error) {
	s, err := e.headerState()
	if err != nil {
		return nil, err
	}
	gone := map[*Node]bool{}
	for _, w := range without {
		if !IsInclude(w) || e.TopForm(w) != w {
			return nil, fmt.Errorf("missing without #%d (%s): not an include form of the file", w.ID, label(w))
		}
		gone[w] = true
	}
	return s.missing(gone), nil
}

// Collisions are the include forms' macros over names the file declares,
// or uses as its own, below them.
func (e *Editor) Collisions() ([]Collision, error) {
	s, err := e.headerState()
	if err != nil {
		return nil, err
	}
	return s.collisions(), nil
}

// SpareIncludes are the include forms the file can do without, by the
// rule: each one whose deletion alone leaves every name provided; then, if
// those together leave one unprovided, the largest set of them a fold from
// the bottom keeps out (each tried with the ones below it that went) -- the
// order of crefactor/xform's Includes, which asks the compiler instead.
// The forms are not deleted.
func (e *Editor) SpareIncludes() ([]*Node, error) {
	s, err := e.headerState()
	if err != nil {
		return nil, err
	}
	var alone []*Node
	for _, inc := range s.incs {
		if len(s.missing(map[*Node]bool{inc: true})) == 0 {
			alone = append(alone, inc)
		}
	}
	all := map[*Node]bool{}
	for _, inc := range alone {
		all[inc] = true
	}
	if len(s.missing(all)) == 0 {
		return alone, nil
	}
	keep := map[*Node]bool{}
	for i := len(alone) - 1; i >= 0; i-- {
		keep[alone[i]] = true
		if len(s.missing(keep)) > 0 {
			delete(keep, alone[i])
		}
	}
	var out []*Node
	for _, inc := range alone {
		if keep[inc] {
			out = append(out, inc)
		}
	}
	return out, nil
}

// InsertIncludeBefore puts a new include of spec before the top-level form
// at, refused where its macros would collide with the file's names below
// it, or where the file includes spec already.
func (e *Editor) InsertIncludeBefore(at *Node, spec string) (*Node, error) {
	return e.insertInclude(at, spec, false)
}

// InsertIncludeAfter puts a new include of spec after the top-level form
// at, refused as InsertIncludeBefore.
func (e *Editor) InsertIncludeAfter(at *Node, spec string) (*Node, error) {
	return e.insertInclude(at, spec, true)
}

// newInclude checks an include of spec may go next to the top-level form
// at: a header cc parses, not included already.
func (e *Editor) newInclude(at *Node, spec string) (string, *Header, error) {
	what := fmt.Sprintf("include %s", spec)
	if e.TopForm(at) != at {
		return what, nil, fmt.Errorf("%s: #%d (%s) is not a top-level form", what, at.ID, label(at))
	}
	for _, inc := range e.Includes() {
		if IncludeSpec(inc) == spec {
			return what, nil, fmt.Errorf("%s: the file includes it already, #%d", what, inc.ID)
		}
	}
	h, err := HeaderOf(spec)
	if err != nil {
		return what, nil, fmt.Errorf("%s: %v", what, err)
	}
	return what, h, nil
}

func (e *Editor) insertInclude(at *Node, spec string, after bool) (*Node, error) {
	what, _, err := e.newInclude(at, spec)
	if err != nil {
		return nil, err
	}
	n := NewInclude(spec)
	err = e.guarded(what, func() error {
		if after {
			return e.InsertAfter(at, n)
		}
		return e.InsertBefore(at, n)
	}, func() {
		e.g.Forms = removeForm(e.g.Forms, n)
		n.up = nil
		e.Log = e.Log[:len(e.Log)-1]
	})
	if err != nil {
		return nil, err
	}
	return n, nil
}

// InsertIncludeRebind inserts an include of spec as InsertIncludeBefore or
// (after) InsertIncludeAfter does, but where one of its macros is a name the
// file only USES below it (declaring it above), each such use becomes the
// macro -- an atom with no refers edge, what the importer makes of a
// constant -- instead of refusing: DeleteIncludeRebind's inverse.  A macro
// over a name the file declares below it still refuses, and so does one
// whose replacement names something (it would need edges this does not
// make).  It returns the include and the new tokens.
func (e *Editor) InsertIncludeRebind(at *Node, spec string, after bool) (*Node, []*Node, error) {
	what, h, err := e.newInclude(at, spec)
	if err != nil {
		return nil, nil, err
	}
	s, err := e.headerState()
	if err != nil {
		return nil, nil, err
	}
	pos := s.pos[at] // the new include is between pos and pos+1 (after), pos-1 and pos (before)
	below := pos
	if !after {
		below = pos - 1
	}
	names := map[string]bool{}
	for m := range h.Macros {
		if c23Keywords[m] {
			continue
		}
		if d, ok := s.ownDecl[m]; ok && d > below {
			return nil, nil, fmt.Errorf("%s: it defines `%s` as a macro, which the file declares below it", what, m)
		}
		if u, ok := s.own[m]; ok && u > below {
			if !h.Bare[m] {
				return nil, nil, fmt.Errorf("%s: it defines `%s` as a macro whose replacement names something, over the file's uses of its own `%s` below it", what, m, m)
			}
			names[m] = true
		}
	}
	var uses []*Node
	for i := below + 1; i < len(s.forms); i++ {
		Walk(s.forms[i], func(n *Node) bool {
			if !n.list && names[n.Atom] && n.Ref() != nil && !isExtern(n.Ref()) {
				uses = append(uses, n)
			}
			return true
		})
	}
	inc := NewInclude(spec)
	if after {
		err = e.InsertAfter(at, inc)
	} else {
		err = e.InsertBefore(at, inc)
	}
	if err != nil {
		return nil, nil, err
	}
	var out []*Node
	for _, u := range uses {
		tok := NewAtom(u.Atom)
		if err := e.Replace(u, tok); err != nil {
			return inc, out, fmt.Errorf("%s: %v", what, err)
		}
		out = append(out, tok)
	}
	return inc, out, nil
}

// DeleteInclude deletes an include form, refused where a name it provides
// would be left unprovided (Missing).
func (e *Editor) DeleteInclude(inc *Node) error {
	if !IsInclude(inc) || e.TopForm(inc) != inc {
		return fmt.Errorf("delete include #%d (%s): not an include form of the file", inc.ID, label(inc))
	}
	miss, err := e.Missing(inc)
	if err != nil {
		return err
	}
	if len(miss) > 0 {
		return fmt.Errorf("delete include %s: %s", IncludeSpec(inc), unprovided(miss))
	}
	return e.Delete(inc)
}

// DeleteIncludeRebind deletes an include form as DeleteInclude does, but
// where a macro it provides is a name the file also declares as its own,
// the tokens that were the macro and are no longer provided become uses of
// the file's declaration (each a new atom with its refers edge, by
// Resolve: what C now makes of them) instead of refusing.  A declaration,
// or a macro the file does not declare, or one said inside a macro
// invocation's text, still refuses.  It returns the new uses.
//
// This is what a compiler's silence accepts and the rule does not: the
// program still compiles, and those tokens now say something else.
func (e *Editor) DeleteIncludeRebind(inc *Node) ([]*Node, error) {
	if !IsInclude(inc) || e.TopForm(inc) != inc {
		return nil, fmt.Errorf("delete include #%d (%s): not an include form of the file", inc.ID, label(inc))
	}
	s, err := e.headerState()
	if err != nil {
		return nil, err
	}
	gone := map[*Node]bool{inc: true}
	miss := s.missing(gone)
	what := "delete include " + IncludeSpec(inc)
	names := map[string]bool{}
	var hard []HeaderUse
	for _, u := range miss {
		if _, own := s.ownDecl[u.Name]; u.Macro && own {
			names[u.Name] = true
		} else {
			hard = append(hard, u)
		}
	}
	if len(hard) > 0 {
		return nil, fmt.Errorf("%s: %s", what, unprovided(hard))
	}
	// the tokens that would no longer be the macro, each resolved where it is
	type rebind struct{ atom, decl *Node }
	var rb []rebind
	declaring := map[*Node]bool{}
	for i, f := range s.forms {
		if IsInclude(f) || f.Is("directive") {
			continue
		}
		declaringAtoms(f, declaring)
		Walk(f, func(n *Node) bool {
			if err != nil {
				return false
			}
			if n.list {
				if (n.Is("macro") || n.Is("macro-decl") || n.Is("verbatim")) && len(n.Kids) > 1 && !n.Kids[1].list {
					for _, id := range identsIn(unquote(n.Kids[1].Atom)) {
						if names[id] && s.providedAbove(id, true, i, gone) == nil && s.providedAbove(id, true, i, nil) != nil {
							err = fmt.Errorf("%s: `%s`, its macro, is said in %s's text, which cannot be made a use", what, id, label(n))
						}
					}
				}
				return true
			}
			if !names[n.Atom] || declaring[n] || n.Ref() != nil ||
				s.providedAbove(n.Atom, true, i, nil) == nil || s.providedAbove(n.Atom, true, i, gone) != nil {
				return true
			}
			d := e.Resolve(n, n.Atom)
			if d == nil || isExtern(d) {
				err = fmt.Errorf("%s: `%s` in %s would name nothing the file declares there", what, n.Atom, label(f))
				return false
			}
			rb = append(rb, rebind{n, d})
			return true
		})
		if err != nil {
			return nil, err
		}
	}
	if err := e.Delete(inc); err != nil {
		return nil, err
	}
	var out []*Node
	for _, r := range rb {
		use := &Node{Atom: r.atom.Atom, Refs: []*Node{r.decl}}
		if err := e.Replace(r.atom, use); err != nil {
			return out, fmt.Errorf("%s: %v", what, err)
		}
		out = append(out, use)
	}
	return out, nil
}

// unprovided says what a refused edit would have left unprovided.
func unprovided(miss []HeaderUse) string {
	var b strings.Builder
	for i, m := range miss {
		if i == 3 {
			fmt.Fprintf(&b, "; and %d more", len(miss)-3)
			break
		}
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(m.String())
	}
	return fmt.Sprintf("%d name(s) would be provided by no include above their first use: %s", len(miss), b.String())
}

// MoveFormsBefore moves the top-level forms ns, in that order, to just
// before the top-level form at, keeping their ids: refused where a
// declaration would no longer be above a use it was above, or where the
// headers' rule would break (a name left unprovided above its first use, a
// new collision).
func (e *Editor) MoveFormsBefore(at *Node, ns ...*Node) error { return e.moveForms(at, false, ns) }

// MoveFormsAfter moves the top-level forms ns, in that order, to just
// after the top-level form at, refused as MoveFormsBefore.
func (e *Editor) MoveFormsAfter(at *Node, ns ...*Node) error { return e.moveForms(at, true, ns) }

// MoveToHost moves the top-level forms ns below the line: to just after
// the include forms that begin the host (the last of the run the first
// include form starts).  Refused as MoveFormsBefore, and where there is no
// line.
func (e *Editor) MoveToHost(ns ...*Node) error {
	first := e.FirstInclude()
	if first == nil {
		return fmt.Errorf("move to the host: the file has no include form, so no line")
	}
	last := first
	for i := e.formAt(first) + 1; i < len(e.g.Forms) && IsInclude(e.g.Forms[i]); i++ {
		last = e.g.Forms[i]
	}
	return e.MoveFormsAfter(last, ns...)
}

// MoveToCore moves the top-level forms ns above the line: to just before
// the first include form.  Refused as MoveFormsBefore -- a form that uses
// what only a header provides cannot go above every include -- and where
// there is no line.
func (e *Editor) MoveToCore(ns ...*Node) error {
	first := e.FirstInclude()
	if first == nil {
		return fmt.Errorf("move to the core: the file has no include form, so no line")
	}
	return e.MoveFormsBefore(first, ns...)
}

func (e *Editor) moveForms(at *Node, after bool, ns []*Node) error {
	what := fmt.Sprintf("move %d form(s)", len(ns))
	if len(ns) == 0 {
		return fmt.Errorf("%s: nothing to move", what)
	}
	if e.TopForm(at) != at {
		return fmt.Errorf("%s: #%d (%s) is not a top-level form", what, at.ID, label(at))
	}
	moving := map[*Node]bool{}
	for _, n := range ns {
		switch {
		case e.TopForm(n) != n:
			return fmt.Errorf("%s: #%d (%s) is not a top-level form", what, n.ID, label(n))
		case moving[n]:
			return fmt.Errorf("%s: #%d (%s) is named twice", what, n.ID, label(n))
		case n == at:
			return fmt.Errorf("%s: #%d (%s) is the place itself", what, n.ID, label(n))
		}
		moving[n] = true
	}
	before := append([]*Node(nil), e.g.Forms...)
	var nf []*Node
	for _, f := range before {
		if moving[f] {
			continue
		}
		if f == at && !after {
			nf = append(nf, ns...)
		}
		nf = append(nf, f)
		if f == at && after {
			nf = append(nf, ns...)
		}
	}
	if err := e.orderKept(what, before, nf, moving); err != nil {
		return err
	}
	act := Act{Op: "move"}
	for _, n := range ns {
		act.Moved = append(act.Moved, n.ID)
	}
	return e.guarded(what, func() error {
		e.g.Forms = nf
		e.Log = append(e.Log, act)
		return nil
	}, func() {
		e.g.Forms = before
		e.Log = e.Log[:len(e.Log)-1]
	})
}

// orderKept refuses a move that would put a declaration below a use it was
// above: every refers edge from or into a moved form, between two
// top-level forms of the file.
func (e *Editor) orderKept(what string, before, after []*Node, moving map[*Node]bool) error {
	pb, pa := map[*Node]int{}, map[*Node]int{}
	for i, f := range before {
		pb[f] = i
	}
	for i, f := range after {
		pa[f] = i
	}
	check := func(use, decl *Node) error {
		fu, fd := e.TopForm(use), e.TopForm(decl)
		if fu == nil || fd == nil || fu == fd {
			return nil
		}
		if pb[fd] < pb[fu] && pa[fd] > pa[fu] {
			return fmt.Errorf("%s: %s, used in %s, would no longer be declared above that use", what, label(decl), label(fu))
		}
		return nil
	}
	for _, f := range before {
		if !moving[f] {
			continue
		}
		var err error
		Walk(f, func(x *Node) bool {
			if err != nil {
				return false
			}
			for _, r := range x.Refs {
				if err = check(x, r); err != nil {
					return false
				}
			}
			if x.list {
				for _, u := range e.Uses(x) {
					if err = check(u, x); err != nil {
						return false
					}
				}
			}
			return true
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// guarded makes an edit and holds it to the headers' rule against the graph
// before it: a name provided before and not after, or a collision after and
// not before, undoes the edit and refuses it.
func (e *Editor) guarded(what string, do func() error, undo func()) error {
	s0, err := e.headerState()
	if err != nil {
		return fmt.Errorf("%s: %v", what, err)
	}
	if err := do(); err != nil {
		return err
	}
	s1, err := e.headerState()
	if err != nil {
		undo()
		return fmt.Errorf("%s: %v", what, err)
	}
	var lost []HeaderUse
	had := map[string]bool{}
	for _, u := range s0.uses(nil) {
		if u.From != nil {
			had[useKey(u)] = true
		}
	}
	for _, u := range s1.uses(nil) {
		if u.From == nil && had[useKey(u)] {
			lost = append(lost, u)
		}
	}
	if len(lost) > 0 {
		undo()
		return fmt.Errorf("%s: %s", what, unprovided(lost))
	}
	old := map[string]bool{}
	for _, c := range s0.collisions() {
		old[IncludeSpec(c.Include)+" "+c.Name] = true
	}
	var fresh []Collision
	named := map[string]bool{}
	for _, c := range s1.collisions() {
		if !old[IncludeSpec(c.Include)+" "+c.Name] && !named[c.Name] {
			named[c.Name] = true
			fresh = append(fresh, c)
		}
	}
	if len(fresh) > 0 {
		undo()
		return &CollisionError{What: what, Collisions: fresh}
	}
	return nil
}

// A CollisionError is an edit refused for the names an include's macros
// would take from the file: each name once, with the first include that
// takes it.
type CollisionError struct {
	What       string
	Collisions []Collision
}

func (c *CollisionError) Error() string {
	var b strings.Builder
	for i, x := range c.Collisions {
		if i == 20 {
			fmt.Fprintf(&b, ", and %d more", len(c.Collisions)-20)
			break
		}
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s (%s)", x.Name, IncludeSpec(x.Include))
	}
	return fmt.Sprintf("%s: %d name(s) the file declares as its own would be an include's macro below it: %s; the first: %s",
		c.What, len(c.Collisions), b.String(), c.Collisions[0])
}

// Names are the names the refused edit's includes would take.
func (c *CollisionError) Names() []string {
	var out []string
	for _, x := range c.Collisions {
		out = append(out, x.Name)
	}
	return out
}

func useKey(u HeaderUse) string {
	if u.Macro {
		return "macro " + u.Name
	}
	return u.Name
}

func removeForm(fs []*Node, n *Node) []*Node {
	out := fs[:0:0]
	for _, f := range fs {
		if f != n {
			out = append(out, f)
		}
	}
	return out
}

// ---- the headers' state of a graph

// headerState is what the rule needs, computed from the forms as they are:
// the include forms and their headers, each name taken from the headers
// with the forms that use it, and the file's own names with the forms
// that declare or name them.
type headerState struct {
	forms   []*Node
	pos     map[*Node]int
	incs    []*Node
	hdr     map[*Node]*Header
	decl    map[string]int // an external's name -> the first form that uses it
	macro   map[string]int // a token -> the first form that says it, for the macros the includes define
	own     map[string]int // a name the file declares or names as its own -> the LAST form that does
	ownDecl map[string]int // a name the file declares -> the LAST form that does
}

// c23Keywords are C23's keywords: a header's macro of one (`bool`) is the
// language's in the C the graph is of, never a name the file takes from it.
var c23Keywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`alignas alignof auto bool break case char const constexpr
		continue default do double else enum extern false float for goto if inline int long
		nullptr register restrict return short signed sizeof static static_assert struct switch
		thread_local true typedef typeof typeof_unqual union unsigned void volatile while
		_Alignas _Alignof _Atomic _BitInt _Bool _Complex _Decimal128 _Decimal32 _Decimal64
		_Generic _Imaginary _Noreturn _Static_assert _Thread_local`) {
		c23Keywords[k] = true
	}
}

func (e *Editor) headerState() (*headerState, error) {
	s := &headerState{
		forms: e.g.Forms, pos: map[*Node]int{}, hdr: map[*Node]*Header{},
		decl: map[string]int{}, macro: map[string]int{}, own: map[string]int{}, ownDecl: map[string]int{},
	}
	macros := map[string]bool{}
	for i, f := range s.forms {
		s.pos[f] = i
		if IsInclude(f) {
			h, err := HeaderOf(IncludeSpec(f))
			if err != nil {
				return nil, err
			}
			s.incs = append(s.incs, f)
			s.hdr[f] = h
			for m := range h.Macros {
				macros[m] = true
			}
		}
	}
	last := func(m map[string]int, name string, i int) {
		if j, ok := m[name]; !ok || i > j {
			m[name] = i
		}
	}
	first := func(m map[string]int, name string, i int) {
		if j, ok := m[name]; !ok || i < j {
			m[name] = i
		}
	}
	// the declarations: an external's uses that spell it
	for _, x := range e.g.Externs {
		name, spelled := externName(x)
		if name == "" || compilerProvides(name) {
			continue
		}
		for _, u := range e.Uses(x) {
			if !spelled(u) {
				continue
			}
			if f := e.TopForm(u); f != nil {
				first(s.decl, name, s.pos[f])
			}
		}
	}
	// the tokens, and the file's own names
	declaring := map[*Node]bool{}
	for i, f := range s.forms {
		if f.Is("include") || f.Is("directive") {
			continue
		}
		declaringAtoms(f, declaring)
		Walk(f, func(n *Node) bool {
			if n.list {
				switch n.Head() {
				case "macro", "macro-decl", "verbatim", "attr-text", "asm-label":
					if len(n.Kids) > 1 && !n.Kids[1].list {
						for _, id := range identsIn(unquote(n.Kids[1].Atom)) {
							if macros[id] && !c23Keywords[id] && !compilerProvides(id) {
								first(s.macro, id, i)
							}
						}
					}
				}
				return true
			}
			if !isIdentText(n.Atom) || c23Keywords[n.Atom] {
				return true
			}
			used := false
			if r := n.Ref(); r != nil {
				if !isExtern(r) {
					used = true
				} else if name, spelled := externName(r); name != "" && spelled(n) {
					return true // the declaration's route
				}
			}
			if declaring[n] || used {
				last(s.own, n.Atom, i)
				if declaring[n] {
					last(s.ownDecl, n.Atom, i)
				}
				return true
			}
			if macros[n.Atom] && !compilerProvides(n.Atom) {
				first(s.macro, n.Atom, i)
			}
			return true
		})
	}
	return s, nil
}

// externName is the name an external node is provided by -- an ordinary
// name, or `struct NAME` for a tag -- and which of its uses spell it.  ""
// for one no header declares (an undeclared name, an anonymous tag, a
// member: its struct's).
func externName(x *Node) (string, func(*Node) bool) {
	if len(x.Kids) < 2 || x.Kids[1].list {
		return "", nil
	}
	name := x.Kids[1].Atom
	switch h := x.Head(); h {
	case "extern", "extern-typedef", "extern-enumerator":
		return name, func(u *Node) bool { return !u.list && u.Atom == name }
	case "extern-struct", "extern-union", "extern-enum":
		kw := strings.TrimPrefix(h, "extern-")
		return kw + " " + name, func(u *Node) bool {
			return u.Is(kw) && len(u.Kids) >= 2 && !u.Kids[1].list && u.Kids[1].Atom == name
		}
	}
	return "", nil
}

// declaringAtoms adds to set the atoms in f that are names it declares: a
// def's, typedef's or defn's, a parameter's, a member's, an enumerator's,
// a tag's, a label's.
func declaringAtoms(f *Node, set map[*Node]bool) {
	Walk(f, func(n *Node) bool {
		if !n.list || len(n.Kids) == 0 {
			return true
		}
		switch n.Head() {
		case "def", "typedef", "defn":
			if i := defNameAt(n); i > 0 {
				set[n.Kids[i]] = true
			}
		case "struct", "union", "enum":
			if len(n.Kids) > 1 && !n.Kids[1].list {
				set[n.Kids[1]] = true
			}
			if n.Is("enum") {
				for _, m := range body(n) {
					if m.list && len(m.Kids) > 0 && !m.Kids[0].list {
						set[m.Kids[0]] = true
					}
				}
			} else {
				for _, m := range members(n) {
					if memberDeclName(m) != "" {
						set[m.Kids[0]] = true
					}
				}
			}
		case "fn":
			if len(n.Kids) > 1 && n.Kids[1].list {
				for _, p := range n.Kids[1].Kids {
					if p.list && len(p.Kids) >= 2 && !p.Kids[0].list {
						set[p.Kids[0]] = true
					}
				}
			}
		case "label":
			if len(n.Kids) == 2 && !n.Kids[1].list {
				set[n.Kids[1]] = true
			}
		}
		return true
	})
}

// identsIn are the identifiers of a text, outside its string and character
// literals.
func identsIn(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(s) && s[j] != c {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			i = j + 1
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			out = append(out, s[i:j])
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(s) && (s[j] == '_' || s[j] == '.' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			i = j
		default:
			i++
		}
	}
	return out
}

// providedAbove is the first include form before position at whose header
// provides name (as a macro, or a declaration), not counting gone.
func (s *headerState) providedAbove(name string, macro bool, at int, gone map[*Node]bool) *Node {
	for _, inc := range s.incs {
		if s.pos[inc] >= at {
			break
		}
		if gone[inc] {
			continue
		}
		h := s.hdr[inc]
		if macro && h.Macros[name] || !macro && h.Names[name] {
			return inc
		}
	}
	return nil
}

// uses are the names taken from the headers, the includes gone taken away.
func (s *headerState) uses(gone map[*Node]bool) []HeaderUse {
	var out []HeaderUse
	add := func(m map[string]int, macro bool) {
		for name, at := range m {
			out = append(out, HeaderUse{Name: name, Macro: macro, First: s.forms[at], From: s.providedAbove(name, macro, at, gone)})
		}
	}
	add(s.decl, false)
	add(s.macro, true)
	sort.Slice(out, func(i, j int) bool {
		a, b := s.pos[out[i].First], s.pos[out[j].First]
		if a != b {
			return a < b
		}
		if out[i].Macro != out[j].Macro {
			return !out[i].Macro
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// missing are the names provided now and not with gone taken away.
func (s *headerState) missing(gone map[*Node]bool) []HeaderUse {
	var out []HeaderUse
	for _, u := range s.uses(nil) {
		if u.From == nil {
			continue
		}
		if s.providedAbove(u.Name, u.Macro, s.pos[u.First], gone) == nil {
			u.From = nil
			out = append(out, u)
		}
	}
	return out
}

// collisions are each include's macros over the file's own names below it.
func (s *headerState) collisions() []Collision {
	var out []Collision
	for _, inc := range s.incs {
		var names []string
		for m := range s.hdr[inc].Macros {
			if last, ok := s.own[m]; ok && last > s.pos[inc] && !c23Keywords[m] {
				names = append(names, m)
			}
		}
		sort.Strings(names)
		for _, m := range names {
			d, ok := s.ownDecl[m]
			out = append(out, Collision{Include: inc, Name: m, Form: s.firstOwnBelow(m, s.pos[inc]), Use: !ok || d <= s.pos[inc]})
		}
	}
	return out
}

// firstOwnBelow is the first form below position at that declares or names
// name as the file's own (the last such form when it cannot say).
func (s *headerState) firstOwnBelow(name string, at int) *Node {
	for i := at + 1; i < len(s.forms); i++ {
		found := false
		Walk(s.forms[i], func(n *Node) bool {
			if !found && !n.list && n.Atom == name {
				found = true
			}
			return !found
		})
		if found {
			return s.forms[i]
		}
	}
	return nil
}

// FormsC is the C view of some top-level forms alone, in the order given:
// the Core's or the Host's, printed as the file prints them.
func FormsC(forms []*Node) ([]byte, error) {
	return (&Graph{Forms: forms}).C()
}
