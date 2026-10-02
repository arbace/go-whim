package togo

// rs_fx.go is what each function of the unit does beyond computing its
// result, as the Rust signature needs it (doc/RUST-IDIOMS.md, items 2 and
// 7).  What the Rust shares with the Haskell and the Scheme is theirs
// (cfacts.go, effects.go): the functions whose callers are written by hand
// (cfacts.keep), the call graph, and which functions reach the editor --
// an object of the editor, the host, a function pointer, or a function
// that does: only those take `ed: *mut Editor` first.  What is the Rust's
// own is which are unsafe:
//
//   - a function does what Rust calls unsafe when it dereferences a raw
//     pointer (a member through a pointer, an element, `*p`), makes an
//     aggregate of zeroes or converts a function pointer -- a reference
//     parameter's dereference is safe (rs_refs.go) -- or reaches the
//     editor, or calls an unsafe function: only then is it an `unsafe fn`.
//     Rust's own checker holds this to the body: a function written safe
//     that does one of these does not compile.
//
// The Rust keeps one more signature whole than the others: a function used
// as a value, whose type is a function pointer's, the editor first.

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// analyses are what the Rust decides about the C as the Haskell and the
// Scheme do (cfacts.go): what the host calls back, what a function pointer
// holds and what a runtime body names keep their signatures.  The Rust
// keeps C's memory as C does, so it leaves the out-parameters and the
// struct locals out.
func (r *rgen) analyses() *cfacts {
	exports := append([]string(nil), r.g.p.RsExports...)
	for n := range r.g.a.addr {
		exports = append(exports, n)
	}
	var bodies []handBody
	for _, rb := range r.g.p.RuntimeBodies {
		if rb.Rs != nil {
			bodies = append(bodies, handBody{rb.Name, rb.Rs("()")})
		}
	}
	return newCFacts(r.g, cfactsOptions{exports: exports, bodies: bodies, noOuts: true, noStructValues: true})
}

// effects decides r.unsafe, once the reference parameters are known.
func (r *rgen) effects() {
	seed := map[string]bool{}
	for name, fd := range r.defined {
		if r.facts.keep[name] || r.facts.takesEd(name) || r.directUnsafe(fd) {
			seed[name] = true
		}
	}
	r.unsafe = r.facts.callersOf(seed)
}

// keepSigs are the functions whose signature is fixed from outside.
func (r *rgen) keepSigs() map[string]bool { return r.facts.keep }

// takesEd says the function name takes the editor: a host function, an
// unknown one, or a defined one that needs it.
func (r *rgen) takesEd(name string) bool { return r.facts.takesEd(name) }

// isSafe says the function name is a safe fn.
func (r *rgen) isSafe(name string) bool { return r.defined[name] != nil && !r.unsafe[name] }

// directUnsafe says fd itself does what Rust calls unsafe, beyond reaching
// the editor (which the shared analysis answers).
func (r *rgen) directUnsafe(fd *cc.FunctionDefinition) bool {
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	if ft == nil {
		return true
	}
	if rt := ft.Result(); rt != nil && (isAggr(rt) || rt.Kind() == cc.Array) {
		return true // its zero, where the end is reached, is core::mem::zeroed()
	}
	unsafe := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || unsafe {
			return
		}
		switch x := n.(type) {
		case *cc.Declarator:
			if x.Name() == "__func__" || x.IsTypename() {
				break
			}
			if t := x.Type(); t != nil && (isAggr(t) || t.Kind() == cc.Array) {
				unsafe = true // a local aggregate starts as core::mem::zeroed()
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionDeref && !r.isRefParam(x.CastExpression) {
				unsafe = true
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionCast && (isFnPtr(x.Type()) || isFnPtr(x.CastExpression.Type())) {
				unsafe = true // a transmute
			}
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionPSelect:
				if !r.isRefParam(x.PostfixExpression) {
					unsafe = true // a reference's member is safe (rs_refs.go)
				}
			case cc.PostfixExpressionIndex, cc.PostfixExpressionComplit:
				unsafe = true
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(fd.CompoundStatement)
	return unsafe
}
