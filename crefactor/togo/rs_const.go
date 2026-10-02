package togo

// rs_const.go is read-only pointers as `*const T` (doc/RUST-IDIOMS.md, item
// 10).  The core keeps almost no C `const`, so it is inferred: a pointer
// slot -- a parameter, a local, an object of the editor, a struct's
// member, a function's result -- is `*const` when nothing is written
// through it and its value goes nowhere that is `*mut`.  A greatest fixed
// point, computed as its complement, the least set of slots that must be
// `*mut`:
//
//   - a slot is written through when a place reached through its value --
//     `*p`, `p[i]`, `p->m`, `p->m.n`, `p->a[i]` -- is assigned, incremented
//     or has its address taken (`&p->m`, which Rust says `&raw mut`); and a
//     slot whose own address is taken is `*mut`, since `&raw mut p` would
//     otherwise point at a `*const`;
//   - a value flows where it is assigned, initializes, is passed, returned:
//     into a slot, which must be `*mut` if that one is; or into a place that
//     is no slot -- an element of an array of pointers, the pointee of a
//     pointer to a pointer, a host function's or a function pointer's
//     parameter, an integer -- which is `*mut`.  A variadic argument is
//     read (the one variadic callee is vim's printf, which has no %n):
//     VArg::P holds a `*const`.
//
// An array reached through a pointer is that pointer walked: its address
// is `decay_const(&raw const (*p).a)` where p is `*const`.
//
// An expression's value comes from its heads: the slot it names, a
// member's, a call's result; through a cast between pointers, pointer
// arithmetic, an increment, an assignment's value (the left side's), the
// comma's last and both arms of ?:.  A value whose heads are all `*mut` is
// `*mut`; a `*mut` value goes into a `*const` slot as Rust coerces it.
// What is fixed from outside (keepSigs), a reference parameter (rs_refs.go),
// a slot whose type a typedef names and the slots of a function printed
// from the lowered form (its temporaries are `*mut`) keep `*mut`.  rustc checks the
// result: a write through a `*const`, or a `*const` where a `*mut` is
// wanted, does not compile, since nothing here casts a `*const` to a `*mut`.

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// rsConstA is the analysis' state.
type rsConstA struct {
	r      *rgen
	cand   map[string]bool     // a slot that could be *const: its declared type is *mut T
	mut    map[string]bool     // a slot that must be *mut
	into   map[string][]string // a slot -> the slots whose values flow into it
	params map[string]bool     // the slots that are parameters
	fn     string              // the function being walked
}

// constness decides r.constSlots.
func (r *rgen) constness() {
	a := &rsConstA{r: r, cand: map[string]bool{}, mut: map[string]bool{}, into: map[string][]string{}, params: map[string]bool{}}
	for _, o := range r.objects {
		a.slot("o:"+o.key, o.t, true)
	}
	for _, n := range r.structOrder {
		for _, fl := range fields(r.structType[n]) {
			a.slot(memberSlot(fl), fl.Type(), true)
		}
	}
	keep := r.keepSigs()
	for name, fd := range r.defined {
		ft, _ := fd.Declarator.Type().(*cc.FunctionType)
		if ft == nil {
			continue
		}
		fixed := keep[name] || r.loweredFns[name]
		a.slot("r:"+name, ft.Result(), !keep[name])
		for _, p := range ft.Parameters() {
			if p.Declarator == nil {
				continue
			}
			a.slot(declSlot(p.Declarator), p.Type(), !fixed && r.refParams[p.Declarator] == nil)
			a.params[declSlot(p.Declarator)] = true
		}
		var decls func(n cc.Node)
		decls = func(n cc.Node) {
			if n == nil {
				return
			}
			if id, ok := n.(*cc.InitDeclarator); ok && id.Declarator != nil && id.Declarator.StorageDuration() == cc.Automatic {
				a.slot(declSlot(id.Declarator), id.Declarator.Type(), !r.loweredFns[name])
			}
			walkChildrenFn(n, decls)
		}
		decls(fd.CompoundStatement)
	}
	for name, fd := range r.defined {
		a.fn = name
		a.walk(fd.CompoundStatement)
	}
	// the objects' initial values: an address taken, a value stored
	a.fn = ""
	for _, o := range r.objects {
		if o.in != nil {
			a.walk(o.in)
			a.initFlows(o.in, "o:"+o.key, true)
		}
	}
	// what must be *mut, closed backwards over the flows
	var work []string
	for s := range a.mut {
		work = append(work, s)
	}
	for len(work) > 0 {
		s := work[len(work)-1]
		work = work[:len(work)-1]
		for _, src := range a.into[s] {
			if !a.mut[src] {
				a.mut[src] = true
				work = append(work, src)
			}
		}
	}
	r.constSlots = map[string]bool{}
	n, of := map[string]int{}, map[string]int{}
	for s := range a.cand {
		k := s[:1]
		if a.params[s] {
			k = "p"
		}
		of[k]++
		if !a.mut[s] {
			r.constSlots[s] = true
			n[k]++
		}
	}
	r.constCount = fmt.Sprintf("%d of %d pointers *const: %d/%d parameters, %d/%d locals, %d/%d results, %d/%d members, %d/%d objects",
		len(r.constSlots), len(a.cand), n["p"], of["p"], n["d"], of["d"], n["r"], of["r"], n["m"], of["m"], n["o"], of["o"])
}

// declSlot is a parameter's or a local's slot; memberSlot a member's.
func declSlot(d *cc.Declarator) string { return fmt.Sprintf("d:%p", d) }
func memberSlot(fl *cc.Field) string   { return fmt.Sprintf("m:%p", fl) }

// slot registers a slot of C type t: a candidate when its declared Rust
// type is a raw pointer to data and ok, else *mut.
func (a *rsConstA) slot(key string, t cc.Type, ok bool) {
	if t == nil || t.Kind() != cc.Ptr || isFnPtr(t) {
		return
	}
	if _, seen := a.cand[key]; seen {
		if !ok {
			a.mut[key] = true
		}
		return
	}
	a.cand[key] = true
	if !ok || a.r.aliased(t) {
		a.mut[key] = true
	}
}

// objSlot is the slot an identifier names: a local's, a parameter's, or
// the editor's object's; "" for none.
func (a *rsConstA) objSlot(d *cc.Declarator) string {
	if d == nil || d.Type() == nil || d.Type().Kind() != cc.Ptr {
		return ""
	}
	if d.StorageDuration() == cc.Automatic || d.IsParam() {
		return declSlot(d)
	}
	if _, ok := a.r.field[a.r.g.a.declKey(d)]; ok {
		return "o:" + a.r.g.a.declKey(d)
	}
	return ""
}

// placeSlot is the slot an lvalue is: a variable, an object or a member of
// pointer type; "" for a place that is no slot.
func (a *rsConstA) placeSlot(e cc.ExpressionNode) string {
	e = unparenE(e)
	if t := e.Type(); t == nil || t.Kind() != cc.Ptr {
		return ""
	}
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			d, _ := x.ResolvedTo().(*cc.Declarator)
			return a.objSlot(d)
		}
		if x.Case == cc.PrimaryExpressionExpr {
			return a.placeSlot(x.ExpressionList)
		}
	case *cc.PostfixExpression:
		if (x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionPSelect) && x.Field() != nil {
			s := memberSlot(x.Field())
			if a.cand != nil {
				a.slot(s, x.Field().Type(), true)
			}
			return s
		}
		if x.Case == cc.PostfixExpressionPrimary {
			return a.placeSlot(x.PrimaryExpression)
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionUnary {
			return a.placeSlot(x.UnaryExpression)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionPostfix {
			return a.placeSlot(x.PostfixExpression)
		}
	}
	return ""
}

// heads are the slots a pointer value comes from.
func (a *rsConstA) heads(e cc.ExpressionNode) []string {
	if e == nil {
		return nil
	}
	e = unparenE(e)
	if x, ok := e.(*cc.PostfixExpression); ok {
		if t := rsPlaceType(x); t != nil && t.Kind() == cc.Array {
			// an array reached through a pointer: its address is the
			// pointer's, walked (decay_const where that one is *const)
			if b := basePtr(x); b != nil {
				return a.heads(b)
			}
			return nil
		}
	}
	if t := e.Type(); t == nil || t.Kind() != cc.Ptr {
		return nil
	}
	switch x := e.(type) {
	case *cc.ExpressionList:
		for x.ExpressionList != nil {
			x = x.ExpressionList
		}
		return a.heads(x.AssignmentExpression)
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			if s := a.placeSlot(x); s != "" {
				return []string{s}
			}
		case cc.PrimaryExpressionExpr:
			return a.heads(x.ExpressionList)
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionPrimary:
			return a.heads(x.PrimaryExpression)
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			if s := a.placeSlot(x); s != "" {
				return []string{s}
			}
		case cc.PostfixExpressionCall:
			d := fnDesignator(x.PostfixExpression)
			if d == nil {
				return nil
			}
			if d.Name() == "__builtin_expect" && x.ArgumentExpressionList != nil {
				return a.heads(x.ArgumentExpressionList.AssignmentExpression)
			}
			if a.r.defined[d.Name()] != nil {
				return []string{"r:" + d.Name()}
			}
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			return a.heads(x.PostfixExpression)
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionPostfix:
			return a.heads(x.PostfixExpression)
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			return a.heads(x.UnaryExpression)
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionUnary {
			return a.heads(x.UnaryExpression)
		}
		if t := x.CastExpression.Type(); t != nil && t.Kind() == cc.Ptr && !isFnPtr(t) && !isFnPtr(x.Type()) {
			return a.heads(x.CastExpression)
		}
	case *cc.AdditiveExpression:
		if x.Case == cc.AdditiveExpressionAdd || x.Case == cc.AdditiveExpressionSub {
			if t := x.AdditiveExpression.Type(); t != nil && t.Kind() == cc.Ptr {
				return a.heads(x.AdditiveExpression)
			}
			if t := x.MultiplicativeExpression.Type(); t != nil && t.Kind() == cc.Ptr {
				return a.heads(x.MultiplicativeExpression)
			}
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return append(a.heads(x.ExpressionList), a.heads(x.ConditionalExpression)...)
		}
		return a.heads(x.LogicalOrExpression)
	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return a.heads(x.ConditionalExpression)
		}
		if s := a.placeSlot(x.UnaryExpression); s != "" {
			return []string{s}
		}
	case *cc.ConstantExpression:
		return a.heads(x.ConditionalExpression)
	}
	return nil
}

// flow says e's value goes into slot dst: "" is a place that is *mut.
func (a *rsConstA) flow(e cc.ExpressionNode, dst string) {
	if !a.cand[dst] {
		dst = "" // no slot the analysis knows: *mut
	}
	for _, h := range a.heads(e) {
		if dst == "" {
			a.mut[h] = true
		} else {
			a.into[dst] = append(a.into[dst], h)
		}
	}
}

// basePtr is the pointer a place is reached through: nil for a variable,
// an object, or a member of one.
func basePtr(e cc.ExpressionNode) cc.ExpressionNode {
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionExpr {
			return basePtr(x.ExpressionList)
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionPrimary:
			return basePtr(x.PrimaryExpression)
		case cc.PostfixExpressionSelect:
			return basePtr(x.PostfixExpression)
		case cc.PostfixExpressionPSelect:
			return x.PostfixExpression
		case cc.PostfixExpressionIndex:
			be := cc.ExpressionNode(x.PostfixExpression)
			if !isPtrish(be.Type()) {
				be = x.ExpressionList
			}
			if be.Type() != nil && be.Type().Kind() == cc.Array {
				return basePtr(be)
			}
			return be
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionPostfix:
			return basePtr(x.PostfixExpression)
		case cc.UnaryExpressionDeref:
			return x.CastExpression
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionUnary {
			return basePtr(x.UnaryExpression)
		}
	}
	return nil
}

// written says the place e is written, or its address taken: what it is
// reached through is *mut, and so is the place itself where it is a slot
// whose address is taken.
func (a *rsConstA) written(e cc.ExpressionNode, addr bool) {
	if b := basePtr(e); b != nil {
		for _, h := range a.heads(b) {
			a.mut[h] = true
		}
	}
	if addr {
		if s := a.placeSlot(e); s != "" {
			a.mut[s] = true
		}
	}
}

// walk finds the writes and the flows of a function's body.
func (a *rsConstA) walk(n cc.Node) {
	if n == nil {
		return
	}
	switch x := n.(type) {
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			a.written(x.UnaryExpression, false)
			if x.Case == cc.AssignmentExpressionAssign {
				if t := x.UnaryExpression.Type(); t != nil && t.Kind() == cc.Ptr {
					a.flow(x.AssignmentExpression, a.placeSlot(x.UnaryExpression))
				}
			}
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			a.written(x.UnaryExpression, false)
		case cc.UnaryExpressionAddrof:
			if fnDesignator(x.CastExpression) == nil {
				a.written(x.CastExpression, true)
			}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			a.written(x.PostfixExpression, false)
		case cc.PostfixExpressionCall:
			a.callFlows(x)
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			if t := x.Type(); t != nil && t.Kind() != cc.Ptr && t.Kind() != cc.Void {
				a.flow(x.CastExpression, "") // a pointer as an integer
			}
		}
	case *cc.JumpStatement:
		if x.Case == cc.JumpStatementReturn && x.ExpressionList != nil {
			a.flow(x.ExpressionList, "r:"+a.fn)
		}
	case *cc.InitDeclarator:
		if x.Initializer != nil && x.Declarator != nil {
			a.initFlows(x.Initializer, declSlot(x.Declarator), true)
		}
	}
	walkChildrenFn(n, a.walk)
}

// initFlows are an initializer's flows: a scalar's into its slot, a
// list's each into its member's, or into an element, which is no slot.
func (a *rsConstA) initFlows(in *cc.Initializer, dst string, top bool) {
	if in == nil {
		return
	}
	if in.Case == cc.InitializerExpr {
		if !top {
			dst = ""
			if fl := in.Field(); fl != nil && fl.Type() != nil && fl.Type().Kind() == cc.Ptr {
				dst = memberSlot(fl)
			}
		}
		a.flow(in.AssignmentExpression, dst)
		return
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		a.initFlows(l.Initializer, "", false)
	}
}

// callFlows are a call's arguments' flows: into the callee's parameters
// where it is defined, else into what is *mut.
func (a *rsConstA) callFlows(x *cc.PostfixExpression) {
	d := fnDesignator(x.PostfixExpression)
	var params []*cc.Parameter
	nfixed := -1 // a variadic callee's fixed parameters: the rest are read (VArg)
	if d != nil {
		if ft, ok := d.Type().(*cc.FunctionType); ok && ft.IsVariadic() {
			nfixed = len(ft.Parameters())
		}
		if fd := a.r.defined[d.Name()]; fd != nil {
			if ft, ok := fd.Declarator.Type().(*cc.FunctionType); ok {
				params = ft.Parameters()
			}
		}
		if d.Name() == "__builtin_expect" {
			return
		}
	}
	i := 0
	for l := x.ArgumentExpressionList; l != nil; l, i = l.ArgumentExpressionList, i+1 {
		if nfixed >= 0 && i >= nfixed {
			continue
		}
		dst := ""
		if i < len(params) && params[i].Declarator != nil {
			dst = declSlot(params[i].Declarator)
		}
		a.flow(l.AssignmentExpression, dst)
	}
}

// --- the printer's side ---------------------------------------------------

// constTy is ty, a raw pointer's type, as *const where the slot is.
func (r *rgen) constTy(slot, ty string) string {
	if slot != "" && r.constSlots[slot] && strings.HasPrefix(ty, "*mut ") {
		return "*const " + strings.TrimPrefix(ty, "*mut ")
	}
	return ty
}

// isConstPtr says ty is a *const T.
func isConstPtr(ty string) bool { return strings.HasPrefix(ty, "*const ") }

// ptrElem is a raw pointer type's pointee.
func ptrElem(ty string) string {
	if s, ok := strings.CutPrefix(ty, "*mut "); ok {
		return s
	}
	return strings.TrimPrefix(ty, "*const ")
}

// asConst is a raw pointer type as *const.
func asConst(ty string) string { return "*const " + ptrElem(ty) }

// slotOf is the slot an expression the printer reads names, for its type:
// a variable, an object, a member, a call's result.
func (f *rfn) slotOf(e cc.ExpressionNode) string {
	a := &rsConstA{r: f.r}
	if p, ok := unparenE(e).(*cc.PostfixExpression); ok && p.Case == cc.PostfixExpressionCall {
		if d := fnDesignator(p.PostfixExpression); d != nil && f.r.defined[d.Name()] != nil {
			return "r:" + d.Name()
		}
		return ""
	}
	return a.placeSlot(e)
}

// paramSlot is the slot of parameter i of the function d names, where it
// is defined; "" otherwise.
func (f *rfn) paramSlot(d *cc.Declarator, i int) string {
	if d == nil {
		return ""
	}
	fd := f.r.defined[d.Name()]
	if fd == nil {
		return ""
	}
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	if ft == nil || i >= len(ft.Parameters()) || ft.Parameters()[i].Declarator == nil {
		return ""
	}
	return declSlot(ft.Parameters()[i].Declarator)
}

// slotTy is ty, the Rust type of e's value, as *const where e names a
// *const slot.
func (f *rfn) slotTy(e cc.ExpressionNode, ty string) string {
	if !isPtrTy(ty) || len(f.r.constSlots) == 0 {
		return ty
	}
	return f.r.constTy(f.slotOf(e), ty)
}

// rsMirror is a comparison with its operands swapped.
var rsMirror = map[string]string{"==": "==", "!=": "!=", "<": ">", ">": "<", "<=": ">=", ">=": "<="}

// rsPlaceType is the type of the place a member or an element is, before
// C converts an array to its first element's address: postfix's own.
func rsPlaceType(x *cc.PostfixExpression) cc.Type {
	switch x.Case {
	case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
		if fl := x.Field(); fl != nil && fl.Type() != nil {
			return fl.Type()
		}
		return x.Type()
	case cc.PostfixExpressionIndex:
		be := cc.ExpressionNode(x.PostfixExpression)
		if !isPtrish(be.Type()) {
			be = x.ExpressionList
		}
		if et := elemOf(be.Type()); et != nil {
			return et
		}
		return x.Type()
	}
	return nil
}

// decayAt is the array at place, of type t, which the expression x is, as
// its first element's address: through a *const pointer a *const one,
// `decay_const(&raw const (*p).a)`, else decay's.
func (f *rfn) decayAt(x cc.ExpressionNode, place string, t cc.Type) rv {
	if b := basePtr(x); b != nil && f.r.anyConst(b) {
		return rv{s: "decay_const(&raw const " + place + ")", ty: "*const " + f.r.rtype(elemOf(t), false, "")}
	}
	return f.decay(place, t)
}

// anyConst says a pointer value comes from a *const slot: its Rust type is
// *const.
func (r *rgen) anyConst(e cc.ExpressionNode) bool {
	if len(r.constSlots) == 0 {
		return false
	}
	a := &rsConstA{r: r}
	for _, h := range a.heads(e) {
		if r.constSlots[h] {
			return true
		}
	}
	return false
}

// aliased says a declaration of type t says it by a typedef's alias,
// which cannot be made *const (rtype's pretty spelling).
func (r *rgen) aliased(t cc.Type) bool {
	td := t.Typedef()
	if td == nil {
		return false
	}
	_, ok := r.aliasOf[td]
	return ok
}
