package graph

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// PARAM (doc/GRAPH-MIGRATION.md, B2c): a function's parameter dropped -- or
// added -- with the argument at every call, as ONE edit, checked by the
// edges before anything moves, refused naming what it cannot follow.
//
//   - WHAT CHANGES TYPE.  A drop names a FUNCTION TYPE FORM by its
//     declaration: a function (every declaration of it, its prototypes and
//     its definition, the block-scope ones too), a parameter of a function
//     (that parameter in every declaration), or a member, an object, a local
//     or a typedef whose type is a pointer to a function or an array of them
//     (the fn form under the derivations).  Then, to a fixed point: a
//     declaration whose type form names a typedef that changes changes; a
//     function one of whose parameters changes changes.  Each such
//     declaration's new type is what its form will say (formType, with the
//     drops and the typedefs' new types in it) -- held first to its form as
//     it stands, which must give its type node now, so that a form PARAM
//     cannot read is refused rather than mistyped.
//   - EVERY USE IS ONE IT CAN FOLLOW.  A use of a declaration whose type
//     changes -- through `()`, `*`, `&`, `[i]` and a member's selection --
//     is a CALLEE (the call loses the arguments the callee's fn form loses);
//     an argument of a call whose callee's parameter has the same new type,
//     or is itself an argument being dropped; one side of `=`, `==`, `!=`, a
//     return, a declaration's value or an initialiser's element, the other
//     side of the same new type (a function decays to the pointer) or a null
//     constant; or a truth test.  Anything else -- a cast, an operand of
//     arithmetic, a function's address passed through `...` -- is refused,
//     named: that is what a call through a pointer PARAM was not told of
//     looks like.  The arguments of a call the edit makes of a function
//     whose parameter changes type are held to that parameter's new type the
//     same way.
//   - THE ARGUMENT'S SIDE EFFECTS.  A dropped argument must be free of them
//     (no store, no call, nothing kept as text): the text verbs dropped
//     whatever was there, and the graph refuses what would lose an effect.
//   - THE PARAMETER'S USES.  A dropped parameter of a definition may have
//     no live use but in an argument the same edit drops; with Dangle the
//     others stay, dangling, for the closure or the collection to take (an
//     unused local's initialiser), and Dangling names them.
//
// Then the arguments and the parameters go through the editor's splice,
// sanctioned for argument and parameter lists alone (a list that empties
// says `void`); the type nodes the new types need are made and interned;
// each declaration's typed edge is its new type; and the expressions above
// every use are typed again (rederive): a call's type is its callee's
// result, unchanged, so a drop on directly called functions leaves nothing
// untyped.  Ids: what goes is superseded, nothing is moved, the new type
// nodes are given; ParamToLocal keeps the parameter's id, moved into the
// body as a local.

// A ParamDrop is a parameter to drop: the I-th of the function type the
// declaration Decl says -- a function's (any of its declarations), a
// parameter's of a function, or a member's, object's, local's or typedef's
// whose type is a pointer to a function or an array of them.
type ParamDrop struct {
	Decl *Node
	I    int
}

// ParamOptions are what a PARAM edit is told.
type ParamOptions struct {
	// Dangle leaves the live uses of a dropped parameter that are not in a
	// dropped argument, dangling, for the closure or the collection; without
	// it they refuse the edit.
	Dangle bool
}

// ParamStats are what a PARAM edit did.
type ParamStats struct {
	Params  int // parameter forms removed or added, every declaration counted
	Args    int // arguments removed or added
	Calls   int // calls edited
	Retyped int // declarations given a new type node
}

func (s ParamStats) String() string {
	return fmt.Sprintf("%d parameters, %d arguments at %d calls, %d declarations retyped", s.Params, s.Args, s.Calls, s.Retyped)
}

// paramEdit is one PARAM edit, checked, then made.
type paramEdit struct {
	e      *Editor
	tx     *typeTx
	opt    ParamOptions
	sites  map[*Node][]int // a fn form, the parameters it loses
	owner  map[*Node]*Node // a site's fn form, its declaration
	order  []*Node         // the fn forms, in the order they were named
	tc     map[*Node]bool  // the declarations whose type changes
	tcList []*Node
	newT   map[*Node]*Node // their new types
	calls  map[*Node][]int // the calls edited, the arguments they lose
	callsL []*Node         // the calls, in order
	keep   map[*Node]bool  // parameters moved, not removed (ParamToLocal)
	gone   map[*Node]bool  // the arguments that go
	tdDone map[*Node]bool  // typedefs whose new type is computed
	tdNew  map[*Node]*Node // and that type
	flows  [][2]*Node      // argument slots of changed parameters to check
	refuse func(format string, a ...any) error
}

func (e *Editor) newParamEdit(opt ParamOptions) *paramEdit {
	pe := &paramEdit{e: e, tx: e.typeTx(), opt: opt, sites: map[*Node][]int{}, owner: map[*Node]*Node{},
		tc: map[*Node]bool{}, newT: map[*Node]*Node{}, calls: map[*Node][]int{}, keep: map[*Node]bool{},
		gone: map[*Node]bool{}, tdDone: map[*Node]bool{}, tdNew: map[*Node]*Node{}}
	pe.tx.drops = pe.sites
	pe.tx.typedef = pe.typedefType
	pe.refuse = func(format string, a ...any) error { return fmt.Errorf("param: "+format, a...) }
	return pe
}

// DropParams drops every parameter drops names, as one edit.
func (e *Editor) DropParams(drops []ParamDrop, opt ParamOptions) (ParamStats, error) {
	pe := e.newParamEdit(opt)
	if err := pe.plan(drops); err != nil {
		return ParamStats{}, err
	}
	return pe.commit()
}

// DropParam drops the parameter named param of the function fn, in every
// declaration of it, with the argument at every call.
func (e *Editor) DropParam(fn, param string, opt ParamOptions) (ParamStats, error) {
	d, i, err := e.paramOf(fn, param)
	if err != nil {
		return ParamStats{}, err
	}
	return e.DropParams([]ParamDrop{{Decl: d, I: i}}, opt)
}

// paramOf is a declaration of the function fn, and the index of its
// parameter named param.
func (e *Editor) paramOf(fn, param string) (*Node, int, error) {
	ds := e.funcDecls(fn)
	if len(ds) == 0 {
		return nil, 0, fmt.Errorf("param: no function %s is declared", fn)
	}
	for _, d := range ds {
		for i, p := range paramElems(defType(d)) {
			if paramName(p) == param {
				return d, i, nil
			}
		}
	}
	return nil, 0, fmt.Errorf("param: %s has no parameter named %s", fn, param)
}

// ParamIndex is the index of the parameter named param in the function
// fn's declarations, or -1.
func (e *Editor) ParamIndex(fn, param string) int {
	_, i, err := e.paramOf(fn, param)
	if err != nil {
		return -1
	}
	return i
}

// funcDecls are every declaration of the function name: the file's
// prototypes and definition, and the block-scope prototypes, in order.
func (e *Editor) funcDecls(name string) []*Node {
	if !e.blockFnsOK {
		e.s6indexBlockFns()
	}
	inner := map[*Node][]*Node{}
	for _, n := range e.blockFns[name] {
		if e.Live(n) && topName(n) == name && defType(n).Is("fn") {
			top := n
			for !e.isTop(top.up) {
				top = top.up
			}
			inner[top] = append(inner[top], n)
		}
	}
	var out []*Node
	for _, f := range e.g.Forms {
		if (f.Is("def") || f.Is("defn")) && topName(f) == name && defType(f).Is("fn") && !hasPrefix(f, "typedef") {
			out = append(out, f)
		}
		out = append(out, inner[f]...)
	}
	return out
}

// isFuncDecl says d declares a function: a defn, or a def of a fn type.
func isFuncDecl(d *Node) bool {
	return d.Is("defn") || d.Is("def") && !hasPrefix(d, "typedef") && defType(d).Is("fn")
}

// declTypeForm is a declaration's type form: a def's, typedef's or defn's, a
// parameter's, a member's.
func declTypeForm(d *Node) *Node {
	if d.Is("def") || d.Is("typedef") || d.Is("defn") {
		return defType(d)
	}
	if d.up != nil && isParamList(d.up) {
		return paramTypeForm(d)
	}
	if d.list && len(d.Kids) >= 2 {
		return d.Kids[1] // a member
	}
	return nil
}

// isParam says d is a parameter of a fn form.
func isParam(d *Node) bool { return d.up != nil && isParamList(d.up) }

// fnUnder is the fn form a pointer-to-function declaration's type form says
// -- through ptr, paren and array -- or the typedef that names it.
func fnUnder(t *Node) (fn, typedef *Node) {
	for t != nil {
		switch {
		case t.Is("fn"):
			return t, nil
		case t.Is("ptr") || t.Is("paren") || t.Is("name-attr"):
			t = t.Kids[1]
		case t.Is("array"):
			t = t.Kids[len(t.Kids)-1]
		case !t.list:
			if d := t.Ref(); d != nil && (d.Is("typedef") || d.Is("def") && hasPrefix(d, "typedef")) {
				return nil, d
			}
			return nil, nil
		default:
			var named *Node
			for _, k := range t.Kids {
				if !k.list && k.Ref() != nil {
					named = k
				}
			}
			t = named
		}
	}
	return nil, nil
}

// site records that the fn form fn of the declaration d loses parameter i.
func (pe *paramEdit) site(d, fn *Node, i int) error {
	ps := paramElems(fn)
	if i < 0 || i >= len(ps) {
		return pe.refuse("%s has no parameter %d (it has %d)", label(d), i, len(ps))
	}
	if !ps[i].list && ps[i].Atom == "..." {
		return pe.refuse("parameter %d of %s is its `...`: an argument of a variadic call is DropArg's", i, label(d))
	}
	if !slices.Contains(pe.sites[fn], i) {
		if _, ok := pe.sites[fn]; !ok {
			pe.order = append(pe.order, fn)
		}
		pe.sites[fn] = append(pe.sites[fn], i)
		sort.Ints(pe.sites[fn])
	}
	pe.owner[fn] = d
	pe.change(d)
	return nil
}

// change records that d's type changes.
func (pe *paramEdit) change(d *Node) {
	if !pe.tc[d] {
		pe.tc[d] = true
		pe.tcList = append(pe.tcList, d)
	}
}

// add expands one drop to its fn forms.
func (pe *paramEdit) add(d ParamDrop) error {
	e := pe.e
	switch n := d.Decl; {
	case n == nil || !e.Live(n):
		return pe.refuse("a drop names a declaration not in the graph")
	case isFuncDecl(n):
		for _, f := range e.funcDecls(topName(n)) {
			if err := pe.site(f, defType(f), d.I); err != nil {
				return err
			}
		}
	case isParam(n):
		fn := n.up.up
		owner := e.Parent(fn)
		if owner == nil || !isFuncDecl(owner) || defType(owner) != fn {
			return pe.refuse("#%d is a parameter of a function type, not of a function: drop it by the pointer's declaration", n.ID)
		}
		j := slices.Index(paramElems(fn), n)
		for _, f := range e.funcDecls(topName(owner)) {
			ps := paramElems(defType(f))
			if j >= len(ps) {
				return pe.refuse("%s's declarations disagree on its parameters", topName(owner))
			}
			p := ps[j]
			if err := pe.pointer(p, d.I); err != nil {
				return err
			}
		}
	default:
		return pe.pointer(n, d.I)
	}
	return nil
}

// pointer is a drop on a declaration of a pointer-to-function type.
func (pe *paramEdit) pointer(d *Node, i int) error {
	t := declTypeForm(d)
	fn, td := fnUnder(t)
	switch {
	case fn != nil:
		return pe.site(d, fn, i)
	case td != nil:
		return pe.refuse("the type of #%d (%s) is the typedef %s: drop the parameter there", d.ID, label(d), topName(td))
	}
	return pe.refuse("#%d (%s) is not of a function type, or a pointer to one", d.ID, label(d))
}

// typedefType is the new type of a typedef the edit changes, for formType.
func (pe *paramEdit) typedefType(d *Node) *Node {
	if !pe.tc[d] {
		return nil
	}
	if !pe.tdDone[d] {
		pe.tdDone[d] = true // a typedef cannot name itself
		pe.tdNew[d] = pe.tx.formType(defType(d), false)
	}
	return pe.tdNew[d]
}

// plan expands the drops, finds what changes type, and checks every use.
func (pe *paramEdit) plan(drops []ParamDrop) error {
	for _, d := range drops {
		if err := pe.add(d); err != nil {
			return err
		}
	}
	if err := pe.closeTypes(); err != nil {
		return err
	}
	for _, calls := range []bool{true, false} {
		for _, d := range pe.tcList {
			if err := pe.uses(d, calls); err != nil {
				return err
			}
		}
	}
	if err := pe.checkFlows(); err != nil {
		return err
	}
	return pe.checkArgsAndParams()
}

// closeTypes finds every declaration whose type changes, to a fixed point,
// and its new type, each held first to its form as it stands.
func (pe *paramEdit) closeTypes() error {
	e := pe.e
	for k := 0; k < len(pe.tcList); k++ {
		d := pe.tcList[k]
		if d.Is("typedef") || d.Is("def") && hasPrefix(d, "typedef") {
			for _, u := range e.Uses(d) {
				up := e.declAbove(u)
				if up == nil {
					return pe.refuse("the typedef %s, whose type changes, is named at #%d in %s outside a declaration (a cast, a sizeof)",
						topName(d), u.ID, inFn(e, u))
				}
				pe.change(up)
			}
		}
		if isParam(d) {
			if owner := e.Parent(d.up.up); owner != nil && isFuncDecl(owner) && defType(owner) == d.up.up {
				pe.change(owner)
			}
		}
	}
	// the new types, each declaration's form held to its type first
	plain := e.typeTx()
	for _, d := range pe.tcList {
		t := declTypeForm(d)
		param := isParam(d)
		if old := plain.completed(plain.formType(t, param), d.Type); old != d.Type {
			return pe.refuse("the type form of #%d (%s) does not say its type node: PARAM cannot follow it", d.ID, label(d))
		}
		nt := pe.tx.completed(pe.tx.formType(t, param), d.Type)
		if nt == nil {
			return pe.refuse("the new type of #%d (%s) is not one PARAM can say", d.ID, label(d))
		}
		pe.newT[d] = nt
	}
	return nil
}

// declAbove is the declaration whose type form holds the use u of a
// typedef -- the innermost: a parameter, a member, a def -- or nil when u
// is in an expression.
func (e *Editor) declAbove(u *Node) *Node {
	for x := u; ; {
		p := e.Parent(x)
		switch {
		case p == nil:
			return nil
		case (p.Is("def") || p.Is("typedef") || p.Is("defn")) && defType(p) == x:
			return p
		case isParamList(p):
			if paramTypeForm(x) == x {
				return x // a parameter that is its type alone, unnamed
			}
			return x
		case isMemberForm(e, p) && p.Kids[1] == x:
			return p
		case p.list && p.up != nil && isParamList(p.up) && paramTypeForm(p) == x:
			return p
		case !isTypeContext(p) && !p.Is("fn") && !isParamList(p):
			return nil
		}
		x = p
	}
}

// inFn names the function n is in, for a message.
func inFn(e *Editor, n *Node) string {
	if f := e.Function(n); f != nil {
		return topName(f)
	}
	return "the file"
}

// typeNow is a declaration's type as it will be.
func (pe *paramEdit) typeNow(d *Node) *Node {
	if t, ok := pe.newT[d]; ok {
		return t
	}
	return d.Type
}

// desig is an expression that designates a declaration -- an identifier,
// a member selected, through (), *, &, [i] -- that declaration and the
// expression's type as it will be; nil when it is not one.
func (pe *paramEdit) desig(x *Node) (*Node, *Node) {
	if !x.list {
		if d := x.Ref(); d != nil {
			return d, pe.typeNow(d)
		}
		return nil, nil
	}
	switch x.Head() {
	case "paren":
		return pe.desig(x.Kids[1])
	case "deref":
		d, t := pe.desig(x.Kids[1])
		if t.Is("function") {
			return d, t
		}
		return d, pointee(t)
	case "addr":
		d, t := pe.desig(x.Kids[1])
		if t == nil {
			return nil, nil
		}
		return d, pe.tx.pointer(t)
	case "index":
		d, t := pe.desig(x.Kids[1])
		return d, pointee(t)
	case "->", ".":
		if m := x.Kids[len(x.Kids)-1].Ref(); m != nil {
			return m, pe.typeNow(m)
		}
	}
	return nil, nil
}

// isNull says x is a null pointer constant: nullptr, 0, (void *)0.
func isNull(x *Node) bool {
	switch {
	case !x.list:
		return x.Atom == "nullptr" || x.Atom == "0" || x.Atom == "NULL"
	case x.Is("paren"):
		return isNull(x.Kids[1])
	case x.Is("cast") && len(x.Kids) == 3:
		return isNull(x.Kids[2])
	}
	return false
}

// sameFlow says a value of type b may go where a is, as the edit leaves
// them: the same type node, a function where a pointer to it is.
func sameFlow(a, b *Node) bool {
	switch {
	case a == nil || b == nil:
		return false
	case a == b:
		return true
	case a.Is("pointer") && pointee(a) == b && b.Is("function"):
		return true
	case b.Is("pointer") && pointee(b) == a && a.Is("function"):
		return true
	}
	return false
}

// uses checks every use of a declaration whose type changes: on the first
// pass the calls it is the callee of (so that the arguments they lose are
// known), on the second every other use.
func (pe *paramEdit) uses(d *Node, calls bool) error {
	e := pe.e
	if d.Is("typedef") || d.Is("def") && hasPrefix(d, "typedef") {
		return nil // its uses are declarations, which changed with it
	}
	for _, u := range e.Uses(d) {
		if !calls && pe.within(u) {
			continue // in an argument the edit drops
		}
		if err := pe.use(u, calls); err != nil {
			return err
		}
	}
	return nil
}

// within says n is in an argument the edit drops.
func (pe *paramEdit) within(n *Node) bool {
	for x := n; x != nil && !pe.e.isTop(x); x = x.up {
		if pe.gone[x] {
			return true
		}
	}
	return false
}

// use checks one use: what designates it, and where that goes -- on the
// first pass only the calls it is the callee of.
func (pe *paramEdit) use(u *Node, calls bool) error {
	e := pe.e
	x := u
	if p := e.Parent(u); p != nil && (p.Is("->") || p.Is(".")) {
		if p.Kids[len(p.Kids)-1] != u {
			if calls {
				return nil
			}
			return pe.refuse("#%d `%s` in %s is selected through", u.ID, u.Atom, inFn(e, u))
		}
		x = p
	}
	for {
		p := e.Parent(x)
		if p == nil {
			break
		}
		if p.Is("paren") || p.Is("deref") || p.Is("addr") || p.Is("index") && p.Kids[1] == x {
			x = p
			continue
		}
		break
	}
	p := e.Parent(x)
	if p == nil {
		return pe.refuse("#%d `%s`: no place", u.ID, u.Atom)
	}
	idx := slices.Index(p.Kids, x)
	if calls {
		if p.Is("call") && idx == 1 {
			return pe.callee(p)
		}
		return nil
	}
	_, t := pe.desig(x)
	where := func() error {
		return pe.refuse("#%d `%s` in %s, whose type changes, is used where PARAM cannot follow it (in %s #%d)",
			u.ID, u.Atom, inFn(e, u), label(p), p.ID)
	}
	if t == nil {
		return where()
	}
	switch h := p.Head(); {
	case h == "call" && idx == 1:
		return nil // the first pass's
	case h == "call":
		pe.flows = append(pe.flows, [2]*Node{p, x})
		return nil
	case h == "=" || h == "==" || h == "!=":
		other := p.Kids[3-idx]
		if isNull(other) && (h != "=" || idx == 1) {
			return nil
		}
		if _, ot := pe.desig(other); sameFlow(ot, t) {
			return nil
		}
		return pe.refuse("#%d `%s` in %s meets #%d in a `%s` of another type as the edit leaves them", u.ID, u.Atom, inFn(e, u), other.ID, h)
	case h == "!" || h == "&&" || h == "||":
		return nil
	case (h == "if" || h == "while" || h == "switch") && idx == 1, h == "do" && idx == 2, h == "for" && idx == 2, h == "?" && idx == 1:
		return nil
	case h == "return":
		if f := e.Function(p); f != nil {
			if r := resultType(pe.typeNow(f)); sameFlow(r, t) {
				return nil
			}
		}
		return pe.refuse("#%d `%s` is returned by %s, whose result is another type", u.ID, u.Atom, inFn(e, u))
	case h == "def" && idx > defNameAt(p)+1:
		if sameFlow(pe.typeNow(p), t) {
			return nil
		}
		return pe.refuse("#%d `%s` initialises %s, of another type", u.ID, u.Atom, label(p))
	case h == "init":
		if sameFlow(pe.slot(p, idx-1), t) {
			return nil
		}
		return pe.refuse("#%d `%s` in an initialiser of #%d goes where another type is", u.ID, u.Atom, p.ID)
	case h == "sizeof" || h == "sizeof-bare":
		return nil
	}
	return where()
}

// slot is the type, as the edit leaves it, of element k of the initialiser
// in: an array's element, a struct's k-th member's.
func (pe *paramEdit) slot(in *Node, k int) *Node {
	e := pe.e
	p := e.Parent(in)
	var t *Node
	switch {
	case p == nil:
		return nil
	case p.Is("def"):
		t = pe.typeNow(p)
	case p.Is("init"):
		t = pe.slot(p, slices.Index(p.Kids, in)-1)
	default:
		return nil
	}
	if slices.ContainsFunc(in.Kids[1:], func(x *Node) bool { return x.Is("at") }) {
		return nil // designated: not followed
	}
	switch {
	case t.Is("array"):
		return pointee(t)
	case t.Is("struct"):
		ms := members(t)
		if k < len(ms) {
			return pe.typeNow(ms[k])
		}
	}
	return nil
}

// callDrops are the arguments a call loses: its callee's fn form's drops.
func (pe *paramEdit) callDrops(c *Node) []int {
	d, _ := pe.desig(c.Kids[1])
	if d == nil {
		return nil
	}
	return pe.dropsOf(d)
}

// dropsOf are the parameters the function type d declares loses.
func (pe *paramEdit) dropsOf(d *Node) []int {
	t := declTypeForm(d)
	if t == nil {
		return nil
	}
	if isFuncDecl(d) {
		return pe.sites[t]
	}
	fn, td := fnUnder(t)
	switch {
	case fn != nil:
		return pe.sites[fn]
	case td != nil:
		return pe.dropsOf(td)
	}
	return nil
}

// callee records a call whose callee's type changes, and the arguments it
// loses and keeps.
func (pe *paramEdit) callee(c *Node) error {
	drops := pe.callDrops(c)
	if old, ok := pe.calls[c]; ok {
		if !slices.Equal(old, drops) {
			return pe.refuse("call #%d in %s: its callee loses two different sets of parameters", c.ID, inFn(pe.e, c))
		}
		return nil
	}
	pe.calls[c] = drops
	pe.callsL = append(pe.callsL, c)
	for _, j := range drops {
		if 2+j >= len(c.Kids) {
			return pe.refuse("call #%d in %s has no argument %d", c.ID, inFn(pe.e, c), j)
		}
		pe.markGone(c.Kids[2+j])
	}
	// the arguments it keeps, held to its parameters' types later
	for j := 2; j < len(c.Kids); j++ {
		if !slices.Contains(drops, j-2) {
			pe.flows = append(pe.flows, [2]*Node{c, c.Kids[j]})
		}
	}
	return nil
}

func (pe *paramEdit) markGone(a *Node) { pe.gone[a] = true }

// checkFlows holds each argument a call keeps to its parameter, as the
// edit leaves them, where either's type changes.
func (pe *paramEdit) checkFlows() error {
	e := pe.e
	seen := map[*Node]bool{}
	for _, fl := range pe.flows {
		c, a := fl[0], fl[1]
		if seen[a] || pe.gone[a] || pe.within(c) {
			continue
		}
		seen[a] = true
		_, nf := pe.desig(c.Kids[1])
		of := typeOf(c.Kids[1])
		if nf.Is("pointer") {
			nf = pointee(nf)
		}
		if of.Is("pointer") {
			of = pointee(of)
		}
		nparams, nvar, _, ok1 := funcParts(nf)
		oparams, _, _, ok2 := funcParts(of)
		if !ok1 || !ok2 {
			return pe.refuse("call #%d in %s: its callee's type is not a function's", c.ID, inFn(e, c))
		}
		j := slices.Index(c.Kids, a) - 2 // among all the arguments
		k := 0                           // among those kept
		for i := 2; i < len(c.Kids) && c.Kids[i] != a; i++ {
			if !pe.gone[c.Kids[i]] {
				k++
			}
		}
		_, at := pe.desig(a)
		argChanged := at != nil && at != typeOf(a)
		if k >= len(nparams) {
			if nvar && argChanged {
				return pe.refuse("call #%d in %s passes #%d, whose type changes, through `...`", c.ID, inFn(e, c), a.ID)
			}
			continue
		}
		paramChanged := j >= len(oparams) || oparams[j] != nparams[k]
		switch {
		case !argChanged && !paramChanged, isNull(a), sameFlow(nparams[k], at):
			continue
		}
		return pe.refuse("call #%d in %s: argument #%d goes to a parameter of another type as the edit leaves them", c.ID, inFn(e, c), a.ID)
	}
	return nil
}

// checkArgsAndParams: the arguments that go are free of side effects, and
// a dropped parameter of a definition has no use left but in them.
func (pe *paramEdit) checkArgsAndParams() error {
	e := pe.e
	c := &closure{e: e, pureFn: map[string]bool{}}
	for _, call := range pe.callsL {
		for _, j := range pe.calls[call] {
			if a := call.Kids[2+j]; !c.pure(a) {
				return pe.refuse("call #%d in %s: argument %d (#%d) has a side effect, which dropping it would lose", call.ID, inFn(e, call), j, a.ID)
			}
		}
	}
	for _, fn := range pe.order {
		d := pe.owner[fn]
		if !d.Is("defn") || defType(d) != fn {
			continue
		}
		ps := paramElems(fn)
		for _, i := range pe.sites[fn] {
			p := ps[i]
			if pe.keep[p] {
				continue
			}
			for _, u := range e.Uses(p) {
				if pe.within(u) || pe.opt.Dangle {
					continue
				}
				return pe.refuse("%s's parameter %s is still used at #%d: rewrite its uses first, or let them dangle (Dangle)",
					topName(d), paramName(p), u.ID)
			}
		}
	}
	return nil
}

// commit makes the edit: the types, the arguments, the parameters, the
// typed edges.
func (pe *paramEdit) commit() (ParamStats, error) {
	e := pe.e
	var st ParamStats
	pe.tx.commit()
	e.argLists = true
	defer func() { e.argLists = false }()
	for _, c := range pe.callsL {
		ds := pe.calls[c]
		for k := len(ds) - 1; k >= 0; k-- {
			if err := e.Delete(c.Kids[2+ds[k]]); err != nil {
				return st, err
			}
			st.Args++
		}
		if len(ds) > 0 {
			st.Calls++
		}
	}
	for _, fn := range pe.order {
		ps := paramElems(fn)
		ds := pe.sites[fn]
		for k := len(ds) - 1; k >= 0; k-- {
			p := ps[ds[k]]
			if pe.keep[p] {
				if err := pe.toLocal(p); err != nil {
					return st, err
				}
			} else if err := e.Delete(p); err != nil {
				return st, err
			}
			st.Params++
		}
		if len(fn.Kids[1].Kids) == 0 {
			if err := e.splice("insert", fn.Kids[1], 0, 0, []*Node{NewAtom("void")}); err != nil {
				return st, err
			}
		}
	}
	e.argLists = false
	act := Act{Op: "retype"}
	for _, d := range pe.tcList {
		if d.Type != pe.newT[d] {
			d.Type = pe.newT[d]
			if d.ID != 0 {
				act.Moved = append(act.Moved, d.ID)
			}
			st.Retyped++
		}
	}
	e.Log = append(e.Log, act)
	for _, d := range pe.tcList {
		for _, u := range e.Uses(d) {
			if u.up != nil {
				pe.tx.rederive(u.up)
			}
		}
	}
	pe.tx.commit()
	return st, nil
}

// toLocal moves the parameter p of its definition into the body as the
// first item, `(def NAME TYPE ATTR...)`, its id and edges kept.
func (pe *paramEdit) toLocal(p *Node) error {
	e := pe.e
	pl := p.up
	f := e.Parent(pl.up)
	i := slices.Index(pl.Kids, p)
	pl.Kids = slices.Delete(slices.Clone(pl.Kids), i, i+1)
	p.Kids = append([]*Node{NewAtom("def")}, p.Kids...)
	p.Kids[0].up = p
	at := defnItemsAt(f)
	f.Kids = slices.Insert(slices.Clone(f.Kids), at, p)
	p.up = f
	e.Written = append(e.Written, pl, f)
	e.touched = append(e.touched, pl, f)
	e.Log = append(e.Log, Act{Op: "param-to-local", Moved: []ID{p.ID}})
	return nil
}

// ParamToLocal makes the parameter param of the function fn a local of its
// definition, declared first in its body: every declaration of fn loses it
// and every call its argument, as DropParam; its uses stay, and refer to
// the same node, the same id, now a `def`.  Refused for a parameter whose
// type a parameter's place adjusts (an array, a function).
func (e *Editor) ParamToLocal(fn, param string) (ParamStats, error) {
	f := e.Defn(fn)
	if f == nil {
		return ParamStats{}, fmt.Errorf("param: %s is not defined", fn)
	}
	p := paramNamed(f, param)
	if p == nil {
		return ParamStats{}, fmt.Errorf("param: %s has no parameter named %s", fn, param)
	}
	if t := paramTypeForm(p); t.Is("array") || t.Is("fn") {
		return ParamStats{}, fmt.Errorf("param: %s's parameter %s is %s: its type is adjusted as a parameter", fn, param, t.Head())
	}
	pe := e.newParamEdit(ParamOptions{})
	pe.keep[p] = true
	if err := pe.plan([]ParamDrop{{Decl: f, I: slices.Index(paramElems(defType(f)), p)}}); err != nil {
		return ParamStats{}, err
	}
	return pe.commit()
}

// DropArg drops the argument i of the call c, where its callee takes it
// through `...`: the call alone changes, no type.  The argument must be
// free of side effects; a format string that counted it is the caller's.
func (e *Editor) DropArg(c *Node, i int) error {
	if !e.Live(c) || !c.Is("call") {
		return fmt.Errorf("param: #%d is not a call in the graph", c.ID)
	}
	ft := typeOf(c.Kids[1])
	if ft.Is("pointer") {
		ft = pointee(ft)
	}
	params, variadic, _, ok := funcParts(ft)
	switch {
	case !ok:
		return fmt.Errorf("param: call #%d: its callee's type is not known", c.ID)
	case !variadic || i < len(params):
		return fmt.Errorf("param: call #%d: argument %d is not one its callee takes through `...`: drop the parameter", c.ID, i)
	case 2+i >= len(c.Kids):
		return fmt.Errorf("param: call #%d has no argument %d", c.ID, i)
	}
	if a := c.Kids[2+i]; !(&closure{e: e, pureFn: map[string]bool{}}).pure(a) {
		return fmt.Errorf("param: call #%d: argument %d has a side effect, which dropping it would lose", c.ID, i)
	}
	e.argLists = true
	defer func() { e.argLists = false }()
	return e.Delete(c.Kids[2+i])
}

// AddParam adds the parameter param -- C-lisp's `(NAME TYPE ATTR...)`,
// read in each declaration's place -- as parameter i of every declaration
// of the function fn, and at every call the argument arg says, read at the
// call (holes its bindings).  Every use of fn must be a call: a function
// whose address is taken would no longer fit the pointer it went to.  The
// name must neither be declared again at the top of the definition's body
// nor hide a declaration the body names.
func (e *Editor) AddParam(fn string, i int, param string, arg func(c *Node) (string, Bindings)) (ParamStats, error) {
	var st ParamStats
	ds := e.funcDecls(fn)
	if len(ds) == 0 {
		return st, fmt.Errorf("param: no function %s is declared", fn)
	}
	forms, err := clisp.Read([]byte(param))
	if err != nil || len(forms) != 1 || !forms[0].IsList() || len(forms[0].List) < 2 || forms[0].List[0].IsList() {
		return st, fmt.Errorf("param: %q is not one (NAME TYPE ATTR...)", param)
	}
	name := forms[0].List[0].Atom
	tx := e.typeTx()
	// the uses: calls only
	var calls []*Node
	for _, d := range ds {
		for _, u := range e.Uses(d) {
			p := e.Parent(u)
			for p != nil && p.Is("paren") {
				u, p = p, e.Parent(p)
			}
			if p == nil || !p.Is("call") || p.Kids[1] != u {
				return st, fmt.Errorf("param: %s is used at #%d in %s other than as a callee: its type would no longer fit", fn, u.ID, inFn(e, u))
			}
			calls = append(calls, p)
		}
	}
	// the name in the definition
	if f := e.Defn(fn); f != nil {
		if paramNamed(f, name) != nil {
			return st, fmt.Errorf("param: %s already has a parameter %s", fn, name)
		}
		for _, it := range Body(f) {
			if (it.Is("def") || it.Is("typedef")) && topName(it) == name {
				return st, fmt.Errorf("param: %s declares %s at the top of its body", fn, name)
			}
		}
		var hidden *Node
		Walk(f, func(n *Node) bool {
			for _, r := range n.Refs {
				if hidden == nil && ordinaryName(r) == name && e.Function(r) == nil && !isMember(e, r) {
					hidden = n
				}
			}
			return hidden == nil
		})
		if hidden != nil {
			return st, fmt.Errorf("param: %s's body names the file's %s at #%d, which the parameter would hide", fn, name, hidden.ID)
		}
	}
	// the parameter, built in each declaration's place
	type made struct {
		d, p *Node
		t    *Node
	}
	var ms []made
	plain := e.typeTx()
	for _, d := range ds {
		fnf := defType(d)
		pp, pi := e.index(d)
		b := &builder{e: e, p: pp, i: pi, used: map[string]bool{}, local: map[string]*Node{}}
		p := NewList(NewAtom(name), b.typ(forms[0].List[1]))
		for _, a := range forms[0].List[2:] {
			if !a.IsList() || a.Head() != "attr" && a.Head() != "std-attr" {
				return st, fmt.Errorf("param: %q: after the type only attributes", param)
			}
			p.Kids = append(p.Kids, attrNode(a))
		}
		if b.err != nil {
			return st, fmt.Errorf("param: %w", b.err)
		}
		pt := tx.formType(p.Kids[1], true)
		if pt == nil {
			return st, fmt.Errorf("param: the type of %s is not one PARAM can say", param)
		}
		p.Type = pt
		if plain.formType(fnf, false) != d.Type {
			return st, fmt.Errorf("param: the type form of %s does not say its type node", label(d))
		}
		if i < 0 || i > len(paramElems(fnf)) {
			return st, fmt.Errorf("param: %s has no place %d for a parameter", fn, i)
		}
		ms = append(ms, made{d, p, pt})
	}
	// the arguments, built at each call
	args := make([]*Node, len(calls))
	for k, c := range calls {
		src, holes := arg(c)
		ns, err := e.Build(c, src, holes)
		if err != nil {
			return st, fmt.Errorf("param: the argument at call #%d: %w", c.ID, err)
		}
		if len(ns) != 1 || IsStatement(ns[0]) {
			return st, fmt.Errorf("param: the argument at call #%d is not one expression", c.ID)
		}
		if 2+i > len(c.Kids) {
			return st, fmt.Errorf("param: call #%d has fewer than %d arguments", c.ID, i)
		}
		args[k] = ns[0]
	}
	// made
	newT := map[*Node]*Node{}
	for _, m := range ms {
		params, variadic, result, ok := funcParts(m.d.Type)
		if !ok {
			return st, fmt.Errorf("param: %s's type is not a function's", label(m.d))
		}
		if len(params) == 1 && params[0] != nil && params[0].Is("basic") && len(params[0].Kids) == 2 && params[0].Kids[1].Atom == "void" {
			params = nil // `(void)`: no parameter to insert beside
		}
		params = slices.Insert(slices.Clone(params), i, m.t)
		newT[m.d] = tx.function(params, variadic, result)
	}
	tx.commit()
	e.argLists = true
	defer func() { e.argLists = false }()
	for _, m := range ms {
		pl := defType(m.d).Kids[1]
		var err error
		if len(paramElems(defType(m.d))) == 0 && len(pl.Kids) == 1 {
			err = e.Replace(pl.Kids[0], m.p)
		} else {
			err = e.splice("insert", pl, i, i, []*Node{m.p})
		}
		if err != nil {
			return st, err
		}
		e.typedAll(m.p)
		st.Params++
	}
	for k, c := range calls {
		if err := e.splice("insert", c, 2+i, 2+i, []*Node{args[k]}); err != nil {
			return st, err
		}
		st.Args++
		st.Calls++
	}
	e.argLists = false
	act := Act{Op: "retype"}
	for _, m := range ms {
		m.d.Type = newT[m.d]
		act.Moved = append(act.Moved, m.d.ID)
		st.Retyped++
	}
	e.Log = append(e.Log, act)
	return st, nil
}

// attrNode is an attribute form as nodes: its atoms tokens.
func attrNode(a *clisp.Node) *Node {
	if !a.IsList() {
		return NewAtom(a.Atom)
	}
	n := NewList()
	for _, k := range a.List {
		n.Kids = append(n.Kids, attrNode(k))
	}
	return n
}

// String names a drop's declaration and parameter, for a report.
func (d ParamDrop) String() string {
	if d.Decl == nil {
		return "nothing"
	}
	return strings.TrimSpace(fmt.Sprintf("%s's parameter %d", label(d.Decl), d.I))
}
