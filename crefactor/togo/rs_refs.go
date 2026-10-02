package togo

// rs_refs.go is pointer parameters as Rust references (doc/RUST-IDIOMS.md,
// item 6): `fn equalpos(a: pos_T, b: pos_T)` taking `a: &pos_T`, `fn
// clearpos(pos: &mut pos_T)` -- where the reference's promise is provably
// kept, which in Rust is the whole of it: while it lives, nothing else
// reads or writes what a `&mut` points at, and nothing writes what a `&`
// points at.  The proof is that the function is memory-local:
//
//   - it takes no editor (effects.go), so it reaches no object of the
//     editor's, no host and no function pointer;
//   - every pointer it dereferences is one of its reference parameters --
//     `p->m`, `*p`, `p->m.n` -- or an array of its own;
//   - every function it calls is memory-local too.
//
// So while it runs, memory is touched only through its references.  A
// parameter is a reference when the function uses it only to reach its
// member or its value -- never compared, stepped, stored, cast, tested for
// null, its address handed on but as another such reference -- and no
// member it reaches is an array (which would decay to a raw pointer into
// it).  It is `&mut` when it is written through, else `&`.  A function with
// a `&mut` has no other reference parameter, so no two of its references
// can overlap; and every call passes `&x` of an lvalue -- never null --
// or a reference of the caller's own.  What is kept from outside
// (keepSigs) keeps its raw pointers.  The decisions are a greatest fixed
// point: a parameter that fails takes with it what relied on it.

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// rsRef is a reference parameter: whether it is written through.
type rsRef struct{ mut bool }

// refArg is a call that passes something to a reference parameter: the
// callee and the index, and what is passed.
type refArg struct {
	caller string
	callee string
	i      int
	arg    cc.ExpressionNode
}

// references decides r.refs: for each function, its reference parameters.
func (r *rgen) references() {
	r.refs = map[string]map[int]*rsRef{}
	r.refArgs = map[cc.ExpressionNode]*rsRef{}
	keep := r.keepSigs()
	params := map[string][]*cc.Declarator{} // a function's parameters' declarators
	local := map[string]bool{}              // memory-local, so far
	for name, fd := range r.defined {
		ft, _ := fd.Declarator.Type().(*cc.FunctionType)
		if ft == nil || ft.IsVariadic() || keep[name] || r.takesEd(name) {
			continue
		}
		local[name] = true
		var ds []*cc.Declarator
		for i, p := range ft.Parameters() {
			ds = append(ds, p.Declarator)
			if refCandidate(p) {
				if r.refs[name] == nil {
					r.refs[name] = map[int]*rsRef{}
				}
				r.refs[name][i] = &rsRef{}
			}
		}
		params[name] = ds
	}
	isRef := func(fn string, d *cc.Declarator) (int, bool) {
		for i, pd := range params[fn] {
			if pd != nil && pd == d && r.refs[fn][i] != nil {
				return i, true
			}
		}
		return -1, false
	}
	for changed := true; changed; {
		changed = false
		drop := func(fn string) {
			if local[fn] {
				delete(local, fn)
				changed = true
			}
			if len(r.refs[fn]) > 0 {
				delete(r.refs, fn)
				changed = true
			}
		}
		var args []refArg
		for name := range local {
			fd := r.defined[name]
			ok, as, writes, dropped := r.refUses(name, fd, params[name], local, isRef)
			if dropped {
				changed = true
			}
			if !ok {
				drop(name)
				continue
			}
			for i, ref := range r.refs[name] {
				if writes[i] && !ref.mut {
					ref.mut = true
					changed = true
				}
			}
			args = append(args, as...)
		}
		// the passes to a reference: from a mut one, the caller's is mut;
		// an argument that is not &x nor a reference drops the parameter
		for _, a := range args {
			ref := r.refs[a.callee][a.i]
			if ref == nil {
				continue
			}
			if i, ok := r.refParamArg(a.caller, a.arg, isRef); ok && ref.mut {
				if cr := r.refs[a.caller][i]; cr != nil && !cr.mut {
					cr.mut = true
					changed = true
				}
			}
		}
		// every call of a function with references, wherever it is
		for name, fd := range r.defined {
			walkCalls(fd.CompoundStatement, false, func(call *cc.PostfixExpression, _ bool) {
				d := fnDesignator(call.PostfixExpression)
				if d == nil || len(r.refs[d.Name()]) == 0 {
					return
				}
				g := d.Name()
				i := 0
				for l := call.ArgumentExpressionList; l != nil; l, i = l.ArgumentExpressionList, i+1 {
					if r.refs[g][i] == nil {
						continue
					}
					if !r.refArgOK(name, l.AssignmentExpression, r.defined[g], i, isRef) {
						delete(r.refs[g], i)
						changed = true
					}
				}
			})
		}
		// no two references of a function may overlap where one writes
		for name, rs := range r.refs {
			mut := false
			for _, ref := range rs {
				mut = mut || ref.mut
			}
			if mut && len(rs) > 1 {
				drop(name)
			}
			if len(rs) == 0 {
				delete(r.refs, name)
			}
		}
	}
	// what each call passes, for the printer
	for _, fd := range r.defined {
		walkCalls(fd.CompoundStatement, false, func(call *cc.PostfixExpression, _ bool) {
			d := fnDesignator(call.PostfixExpression)
			if d == nil {
				return
			}
			i := 0
			for l := call.ArgumentExpressionList; l != nil; l, i = l.ArgumentExpressionList, i+1 {
				if ref := r.refs[d.Name()][i]; ref != nil {
					r.refArgs[l.AssignmentExpression] = ref
				}
			}
		})
	}
	r.refParams = map[*cc.Declarator]*rsRef{}
	for name, rs := range r.refs {
		for i, ref := range rs {
			if pd := params[name][i]; pd != nil {
				r.refParams[pd] = ref
			}
		}
	}
}

// refCandidate says a parameter's type could be a reference's: a pointer
// to an object that is no array, no function and not void.
func refCandidate(p *cc.Parameter) bool {
	t := p.Type()
	if t == nil || t.Kind() != cc.Ptr || p.Declarator == nil {
		return false
	}
	e := elemOf(t)
	return e != nil && e.Kind() != cc.Array && e.Kind() != cc.Function && e.Kind() != cc.Void
}

// refParamArg says an argument is the caller's own reference parameter,
// and which.
func (r *rgen) refParamArg(caller string, e cc.ExpressionNode, isRef func(string, *cc.Declarator) (int, bool)) (int, bool) {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return -1, false
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	if !ok {
		return -1, false
	}
	return isRef(caller, d)
}

// refArgOK says an argument may be passed to callee's reference parameter
// i: &x of an lvalue of the parameter's own type, or the caller's own
// reference parameter.
func (r *rgen) refArgOK(caller string, e cc.ExpressionNode, callee *cc.FunctionDefinition, i int, isRef func(string, *cc.Declarator) (int, bool)) bool {
	ft, _ := callee.Declarator.Type().(*cc.FunctionType)
	pt := ft.Parameters()[i].Type()
	if e.Type() == nil || r.ty(e.Type()) != r.ty(pt) {
		return false
	}
	if _, ok := r.refParamArg(caller, e, isRef); ok {
		return true
	}
	u, ok := unparenE(e).(*cc.UnaryExpression)
	if !ok || u.Case != cc.UnaryExpressionAddrof {
		return false
	}
	// the lvalue: no array element through a pointer of the caller's that
	// is a reference (that would be a raw pointer into it), nothing that
	// does anything
	if hasEffect(u.CastExpression) {
		return false
	}
	return true
}

// refUses checks a memory-local candidate's body: whether it still is one,
// the passes of its references (and of &p->m) to callees, and which of its
// references it writes through.
func (r *rgen) refUses(name string, fd *cc.FunctionDefinition, params []*cc.Declarator, local map[string]bool,
	isRef func(string, *cc.Declarator) (int, bool)) (ok bool, args []refArg, writes map[int]bool, dropped bool) {
	ok = true
	writes = map[int]bool{}
	bad := map[int]bool{}
	refOf := func(e cc.ExpressionNode) (int, bool) {
		p, isP := unparenE(e).(*cc.PrimaryExpression)
		if !isP || p.Case != cc.PrimaryExpressionIdent {
			return -1, false
		}
		d, isD := p.ResolvedTo().(*cc.Declarator)
		if !isD {
			return -1, false
		}
		return isRef(name, d)
	}
	// rootRef is the reference a place is reached through -- p->m, (*p).m,
	// *p, p->m.n -- with no array on the way.
	var rootRef func(e cc.ExpressionNode) (int, bool)
	rootRef = func(e cc.ExpressionNode) (int, bool) {
		e = unparenE(e)
		if t := e.Type(); t != nil && t.Kind() == cc.Array {
			return -1, false
		}
		switch x := e.(type) {
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionPSelect:
				return refOf(x.PostfixExpression)
			case cc.PostfixExpressionSelect:
				return rootRef(x.PostfixExpression)
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionDeref {
				return refOf(x.CastExpression)
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionUnary {
				return rootRef(x.UnaryExpression)
			}
		}
		return -1, false
	}
	// localArray says e is an array of the function's own: a local, or a
	// member of a local struct
	var localArray func(e cc.ExpressionNode) bool
	localArray = func(e cc.ExpressionNode) bool {
		e = unparenE(e)
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				d, isD := x.ResolvedTo().(*cc.Declarator)
				return isD && d.StorageDuration() == cc.Automatic && !d.IsParam()
			}
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionSelect {
				return localArray(x.PostfixExpression)
			}
			if x.Case == cc.PostfixExpressionIndex {
				return localArray(x.PostfixExpression)
			}
		}
		return false
	}
	written := func(e cc.ExpressionNode) {
		if i, isR := rootRef(e); isR {
			writes[i] = true
		}
	}
	// the uses of each reference that are allowed are marked here, by the
	// node of the parameter's name; any other is a use that is not
	allowed := map[*cc.PrimaryExpression]bool{}
	passed := map[*cc.UnaryExpression]bool{} // &p->m handed to a reference
	allow := func(e cc.ExpressionNode) {
		e = unparenE(e)
		for {
			switch x := e.(type) {
			case *cc.PrimaryExpression:
				allowed[x] = true
				return
			case *cc.PostfixExpression:
				if x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionPSelect {
					e = unparenE(x.PostfixExpression)
					continue
				}
			case *cc.UnaryExpression:
				if x.Case == cc.UnaryExpressionDeref {
					e = unparenE(x.CastExpression)
					continue
				}
			}
			return
		}
	}
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionPSelect:
				if _, isR := rootRef(x); isR {
					allow(x)
				} else if t := x.Type(); t != nil && t.Kind() == cc.Array {
					if i, isR := refOf(x.PostfixExpression); isR {
						bad[i] = true // an array in it decays to a raw pointer
					} else {
						ok = false
					}
				} else {
					ok = false // a pointer that is no reference
				}
			case cc.PostfixExpressionIndex:
				be := x.PostfixExpression
				if !isPtrish(be.Type()) {
					be = x.ExpressionList
				}
				if be.Type() == nil || be.Type().Kind() != cc.Array || !localArray(be) {
					ok = false
				}
			case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
				written(x.PostfixExpression)
			case cc.PostfixExpressionComplit:
			case cc.PostfixExpressionCall:
				d := fnDesignator(x.PostfixExpression)
				if d != nil && r.defined[d.Name()] != nil && !local[d.Name()] {
					ok = false
				}
				if d != nil && r.defined[d.Name()] != nil {
					i := 0
					for l := x.ArgumentExpressionList; l != nil; l, i = l.ArgumentExpressionList, i+1 {
						a := l.AssignmentExpression
						ref := r.refs[d.Name()][i]
						if ref == nil {
							continue
						}
						if pi, isR := refOf(a); isR {
							allow(a)
							if ref.mut {
								writes[pi] = true
							}
						} else if u, isU := unparenE(a).(*cc.UnaryExpression); isU && u.Case == cc.UnaryExpressionAddrof {
							if pi, isR := rootRef(u.CastExpression); isR {
								allow(u.CastExpression)
								passed[u] = true
								if ref.mut {
									writes[pi] = true
								}
							}
						}
						args = append(args, refArg{caller: name, callee: d.Name(), i: i, arg: a})
					}
				}
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionDeref:
				if _, isR := rootRef(x); isR {
					allow(x)
				} else {
					ok = false
				}
			case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
				written(x.UnaryExpression)
			case cc.UnaryExpressionAddrof:
				// &p->m but as a reference handed on: a raw pointer into
				// the reference, which may outlive it
				if i, isR := rootRef(x.CastExpression); isR && !passed[x] {
					bad[i] = true
				}
			}
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				written(x.UnaryExpression)
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(fd.CompoundStatement)
	// any use of a reference's name not allowed above drops it
	var names func(n cc.Node)
	names = func(n cc.Node) {
		if n == nil {
			return
		}
		if p, isP := n.(*cc.PrimaryExpression); isP && p.Case == cc.PrimaryExpressionIdent && !allowed[p] {
			if d, isD := p.ResolvedTo().(*cc.Declarator); isD {
				if i, isR := isRef(name, d); isR {
					bad[i] = true
				}
			}
		}
		walkChildrenFn(n, names)
	}
	names(fd.CompoundStatement)
	for i := range bad {
		if r.refs[name][i] != nil {
			delete(r.refs[name], i)
			dropped = true // its derefs are of a raw pointer now: the next round sees them
		}
	}
	return ok, args, writes, dropped
}

// isRefParam says e names a reference parameter.
func (r *rgen) isRefParam(e cc.ExpressionNode) bool {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return false
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	return ok && r.refParams[d] != nil
}

// rsType is a reference's Rust type, for the raw pointer type ptr.
func (ref *rsRef) rsType(ptr string) string {
	if ref.mut {
		return "&mut " + strings.TrimPrefix(ptr, "*mut ")
	}
	return "&" + strings.TrimPrefix(ptr, "*mut ")
}

// refArg is what a call passes to a reference parameter: the caller's own
// reference, or `&mut x` of the lvalue whose address C passes.
func (f *rfn) refArg(a cc.ExpressionNode, ref *rsRef) string {
	if f.r.isRefParam(a) {
		return f.expr(a).s
	}
	u := unparenE(a).(*cc.UnaryExpression)
	if ref.mut {
		return "&mut " + f.place(u.CastExpression)
	}
	return "&" + f.place(u.CastExpression)
}
