package graph

import (
	"fmt"
	"slices"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// RETYPE (doc/GRAPH-MIGRATION.md, B2c): a declaration's type changed --
// `int` to `bool`, `void *` to `T *`, a function's result -- in every
// declaration of the entity at once, and the typed edges the change
// invalidates found by the edges, never left silently wrong:
//
//   - WHAT IT TAKES.  An object (every file-scope declaration of it: an
//     extern and its definition), a local, a member, a typedef, a
//     parameter of a function (that parameter in every declaration of the
//     function), or a function's result (RetypeResult).  The new type is
//     C-lisp's type form, read at each declaration's place (BUILD's: typedef
//     names and tags resolved there), and replaces the old form there.
//   - WHAT CHANGES WITH IT.  A function whose parameter or result changes
//     changes type; its every use must be a call (a function whose address
//     is taken would no longer fit the pointer it goes to: refused, named).
//     A typedef's change reaches every declaration whose form names it, to
//     a fixed point (a parameter's reaching its function), and every cast
//     that names it.  Each such declaration is typed by what its form will
//     say; one whose form does not give its type node now (a form RETYPE
//     cannot read) loses its typed edge, listed in Untyped.
//   - THE EXPRESSIONS.  Above every use of every declaration that changed,
//     the expressions are typed again where the type follows plainly from
//     the operands (rederive: a call its callee's result, a selection its
//     member's, `&x`, `*p`, `p[i]`, an assignment, a comparison, a cast) and
//     otherwise CLEARED and listed in Untyped, up to the statement; the walk
//     stops where it finds a type unchanged.  No checker: what is listed is
//     step 6's to type again.  A cast that the change makes redundant, or a
//     value that no longer fits, is the caller's: the graph says what each
//     expression's type is, not whether C would accept it.
//
// Ids: the old type forms are superseded and the new given (the splice's
// act); the declarations keep theirs, their changed typed edges logged in
// a `retype` act; new type nodes are given in a `type` act.

// RetypeStats are what a RETYPE edit did.
type RetypeStats struct {
	Forms   int // type forms replaced
	Decls   int // declarations whose typed edge changed
	Exprs   int // expressions typed again
	Untyped int // expressions and declarations left without a typed edge
}

func (s RetypeStats) String() string {
	return fmt.Sprintf("%d type forms, %d declarations retyped, %d expressions typed again, %d untyped", s.Forms, s.Decls, s.Exprs, s.Untyped)
}

// Retype gives the declaration d the type typ, a C-lisp type form: d an
// object (every file-scope declaration of it), a local, a member, a
// typedef, or a parameter of a function (in every declaration of it).
func (e *Editor) Retype(d *Node, typ string) (RetypeStats, error) {
	if d == nil || !e.Live(d) {
		return RetypeStats{}, fmt.Errorf("retype: a declaration not in the graph")
	}
	var places []*Node // the declarations whose form is replaced
	var fns []*Node    // the functions whose type changes with them
	switch {
	case isFuncDecl(d):
		return RetypeStats{}, fmt.Errorf("retype: %s is a function: its result is RetypeResult's, a parameter's Retype's", label(d))
	case isParam(d):
		fn := d.up.up
		owner := e.Parent(fn)
		if owner == nil || !isFuncDecl(owner) || defType(owner) != fn {
			return RetypeStats{}, fmt.Errorf("retype: #%d is a parameter of a function type, not of a function", d.ID)
		}
		j := slices.Index(paramElems(fn), d)
		for _, f := range e.funcDecls(topName(owner)) {
			ps := paramElems(defType(f))
			if j >= len(ps) {
				return RetypeStats{}, fmt.Errorf("retype: %s's declarations disagree on its parameters", topName(owner))
			}
			places = append(places, ps[j])
			fns = append(fns, f)
		}
	case d.Is("def") && e.Function(d) == nil && !hasPrefix(d, "typedef"):
		for _, f := range e.FileDecls(topName(d)) {
			if f.Is("def") {
				places = append(places, f)
			}
		}
	case d.Is("def") || d.Is("typedef") || isMemberForm(e, d):
		places = []*Node{d}
	default:
		return RetypeStats{}, fmt.Errorf("retype: #%d (%s) is not a declaration RETYPE takes", d.ID, label(d))
	}
	forms := make([]*Node, len(places))
	for k, p := range places {
		forms[k] = declTypeForm(p)
	}
	return e.retypeForms(places, forms, fns, typ)
}

// RetypeResult gives the function fn the result typ, a C-lisp type form,
// in every declaration of it.
func (e *Editor) RetypeResult(fn, typ string) (RetypeStats, error) {
	ds := e.funcDecls(fn)
	if len(ds) == 0 {
		return RetypeStats{}, fmt.Errorf("retype: no function %s is declared", fn)
	}
	forms := make([]*Node, len(ds))
	for k, d := range ds {
		forms[k] = defType(d).Kids[2]
	}
	return e.retypeForms(ds, forms, ds, typ)
}

// retypeForms replaces each form of places by typ read there, then types
// what changed.  fns are the functions whose type changes with the forms.
func (e *Editor) retypeForms(places, forms, fns []*Node, typ string) (RetypeStats, error) {
	var st RetypeStats
	src, err := clisp.Read([]byte(typ))
	if err != nil || len(src) != 1 {
		return st, fmt.Errorf("retype: %q is not one type form", typ)
	}
	// the functions: called, never taken
	for _, f := range fns {
		if err := e.onlyCalled(f); err != nil {
			return st, err
		}
	}
	// the new forms, each read at its declaration's place
	news := make([]*Node, len(places))
	for k, d := range places {
		p, i := e.declPlace(d)
		b := &builder{e: e, p: p, i: i, used: map[string]bool{}, local: map[string]*Node{}}
		news[k] = b.typ(src[0])
		if b.err != nil {
			return st, fmt.Errorf("retype: %w", b.err)
		}
	}
	tx := e.typeTx()
	newT := map[*Node]*Node{}
	changed := []*Node{}
	change := func(d *Node, t *Node) {
		if _, ok := newT[d]; !ok {
			changed = append(changed, d)
		}
		newT[d] = t
	}
	formOf := map[*Node]*Node{} // a place's new form, for formType's hook below
	for k := range places {
		formOf[forms[k]] = news[k]
	}
	// what each place's declaration becomes
	for k, d := range places {
		if d.Is("defn") || isFuncDecl(d) {
			continue // a result: the function's type, below
		}
		change(d, tx.formType(news[k], isParam(d)))
	}
	// the typedefs' reach, to a fixed point, and the functions
	tx.typedef = func(td *Node) *Node {
		if t, ok := newT[td]; ok {
			return t
		}
		return nil
	}
	var casts []*Node
	for k := 0; k < len(changed); k++ {
		d := changed[k]
		if !(d.Is("typedef") || d.Is("def") && hasPrefix(d, "typedef")) {
			continue
		}
		for _, u := range e.Uses(d) {
			x := e.declAbove(u)
			if x == nil {
				if c := castAbove(e, u); c != nil {
					casts = append(casts, c)
					continue
				}
				return st, fmt.Errorf("retype: the typedef %s is named at #%d in %s outside a declaration or a cast", topName(d), u.ID, inFn(e, u))
			}
			if slices.Contains(places, x) {
				continue
			}
			plain := e.typeTx()
			if plain.completed(plain.formType(declTypeForm(x), isParam(x)), x.Type) != x.Type {
				change(x, nil) // a form RETYPE cannot read: untyped
			} else {
				change(x, tx.completed(tx.formType(declTypeForm(x), isParam(x)), x.Type))
			}
			if isParam(x) {
				if owner := e.Parent(x.up.up); owner != nil && isFuncDecl(owner) && defType(owner) == x.up.up && !slices.Contains(fns, owner) {
					if err := e.onlyCalled(owner); err != nil {
						return st, err
					}
					fns = append(fns, owner)
				}
			}
		}
	}
	// the functions' types: their forms as they will be
	for _, f := range fns {
		ft := defType(f)
		old, _, _, ok := funcParts(f.Type)
		if !ok {
			return st, fmt.Errorf("retype: %s's type is not a function's", label(f))
		}
		var params []*Node
		for j, p := range paramElems(ft) {
			if !p.list && p.Atom == "..." {
				continue
			}
			if t, ok := newT[p]; ok {
				params = append(params, t)
			} else if j < len(old) {
				params = append(params, old[j])
			}
		}
		if len(paramElems(ft)) == 0 {
			params = old // `(void)`: cc's one void parameter, as imported
		}
		_, variadic, result, _ := funcParts(f.Type)
		if nf := formOf[ft.Kids[2]]; nf != nil {
			result = tx.formType(nf, false)
		}
		change(f, tx.function(params, variadic, result))
	}
	// made: the forms, the types, the typed edges
	tx.commit()
	for k, f := range forms {
		if err := e.Replace(f, news[k]); err != nil {
			return st, err
		}
		e.typedAll(news[k])
		st.Forms++
	}
	act := Act{Op: "retype"}
	for _, d := range changed {
		if isParam(d) && paramName(d) == "" && d.Type == nil {
			continue // an unnamed parameter: no declarator, untyped as imported (Step6)
		}
		if d.Type != newT[d] {
			d.Type = newT[d]
			st.Decls++
			if d.ID != 0 {
				act.Moved = append(act.Moved, d.ID)
			}
		}
		if d.Type == nil {
			e.untype(d)
		} else {
			e.typed(d)
		}
	}
	e.Log = append(e.Log, act)
	before := len(e.Untyped)
	for _, d := range changed {
		for _, u := range e.Uses(d) {
			if u.up != nil {
				st.Exprs += tx.rederiveCount(u.up)
			}
		}
	}
	for _, c := range casts {
		st.Exprs += tx.rederiveCount(c)
	}
	tx.commit()
	st.Untyped = len(e.Untyped) - before
	return st, nil
}

// onlyCalled refuses a function used other than as a callee.
func (e *Editor) onlyCalled(f *Node) error {
	for _, d := range e.funcDecls(topName(f)) {
		for _, u := range e.Uses(d) {
			x := u
			p := e.Parent(x)
			for p != nil && p.Is("paren") {
				x, p = p, e.Parent(p)
			}
			if p == nil || !p.Is("call") || p.Kids[1] != x {
				return fmt.Errorf("retype: %s is used at #%d in %s other than as a callee: its type would no longer fit", topName(f), u.ID, inFn(e, u))
			}
		}
	}
	return nil
}

// castAbove is the cast or sizeof whose type form holds u, or nil.
func castAbove(e *Editor, u *Node) *Node {
	for x := u; ; {
		p := e.Parent(x)
		switch {
		case p == nil:
			return nil
		case (p.Is("cast") || p.Is("sizeof-type") || p.Is("alignof-type")) && p.Kids[1] == x:
			return p
		case !isTypeContext(p) && !p.Is("fn") && !isParamList(p) && !(p.up != nil && isParamList(p.up)):
			return nil
		}
		x = p
	}
}

// declPlace is where a declaration's type form is read: a local's own
// place, else its top-level form's.
func (e *Editor) declPlace(d *Node) (*Node, int) {
	if e.Function(d) != nil && isItemOf(e, d) {
		return e.index(d)
	}
	top := d
	for e.Parent(top) != nil {
		top = e.Parent(top)
	}
	return e.index(top)
}

// isItemOf says n is an item of a block, a body or the file.
func isItemOf(e *Editor, n *Node) bool {
	p, i := e.index(n)
	return i >= 0 && e.place(p, i) == placeItem
}

// rederiveCount is rederive, counting the expressions whose typed edge it
// changed.  It walks expression forms only: a node in a declaration's type
// form, a parameter list or a member is not one.
func (tx *typeTx) rederiveCount(n *Node) int {
	k := 0
	e := tx.e
	if n == nil || isTypeContext(n) || e.declAbove(n) != nil {
		return 0
	}
	for q := n; q != nil && q.up != nil && !e.isTop(q) && !e.isTop(q.up); q = q.up {
		if !q.list {
			continue
		}
		if !isExprForm(q) {
			return k
		}
		t := tx.decayAt(q, tx.derive(q)) // r1_build.go
		if t != nil && t == q.Type {
			e.typed(q)
			return k
		}
		q.Type = t
		k++
		if t == nil {
			e.untype(q)
		} else {
			e.typed(q)
		}
	}
	return k
}

// isExprForm says q is an expression's form.
func isExprForm(q *Node) bool {
	switch h := q.Head(); h {
	case "->", ".", "cast", "literal", "generic", "sizeof-type", "alignof-type", "stmt-expr", "label-addr", "macro":
		return true
	default:
		return exprHeads[h] || assignOps[h]
	}
}
