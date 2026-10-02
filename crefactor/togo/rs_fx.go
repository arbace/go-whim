package togo

// rs_fx.go is what each function of the unit does beyond computing its
// result, as the Rust signature needs it (doc/RUST-IDIOMS.md, item 2) --
// the Haskell's hseffects.go, on Rust's terms:
//
//   - editor: it names an object of the editor, calls the host or a
//     function pointer, or calls a function that takes the editor: only
//     then is `ed: *mut Editor` its first parameter;
//   - unsafe: it does what Rust calls unsafe -- dereferences a raw pointer
//     (a member through a pointer, an element, `*p`, the editor's object),
//     makes an aggregate of zeroes, converts a function pointer, calls the
//     host, a function pointer or an unsafe function: only then is it an
//     `unsafe fn`.  Rust's own checker holds this to the body: a function
//     written safe that does one of these does not compile.
//
// Each is closed over the calls.  What is called by hand-written code
// (Profile.RsExports), what a runtime body names, and what is used as a
// value -- its type is a function pointer's, the editor first -- keep the
// whole signature.

import (
	"regexp"

	"github.com/arbace/go-whim/crefactor/cc"
)

// rsFx is one function's effects.
type rsFx struct {
	editor bool // needs the editor
	unsafe bool // does what Rust calls unsafe
	calls  []string
}

// effects computes every defined function's effects: r.fx.
func (r *rgen) effects() {
	r.fx = map[string]*rsFx{}
	for name, fd := range r.defined {
		r.fx[name] = r.directFx(fd)
	}
	for n := range r.keepSigs() {
		if fx := r.fx[n]; fx != nil {
			fx.editor, fx.unsafe = true, true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, fx := range r.fx {
			for _, c := range fx.calls {
				cf := r.fx[c]
				if cf == nil {
					continue
				}
				if cf.editor && !fx.editor {
					fx.editor, changed = true, true
				}
				if cf.unsafe && !fx.unsafe {
					fx.unsafe, changed = true, true
				}
			}
		}
	}
}

var rsWordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// keepSigs are the functions whose signature is fixed from outside: called
// by hand, named by a runtime body, or used as a value.
func (r *rgen) keepSigs() map[string]bool {
	keep := map[string]bool{}
	for _, n := range r.g.p.RsExports {
		keep[n] = true
	}
	for n := range r.g.a.addr {
		keep[n] = true
	}
	for _, rb := range r.g.p.RuntimeBodies {
		if rb.Rs == nil {
			continue
		}
		keep[rb.Name] = true
		for _, w := range rsWordRe.FindAllString(rb.Rs("()"), -1) {
			if r.defined[w] != nil {
				keep[w] = true
			}
		}
	}
	return keep
}

// takesEd says the function name takes the editor: a host function, an
// unknown one, or a defined one that needs it.
func (r *rgen) takesEd(name string) bool {
	fx := r.fx[name]
	return fx == nil || fx.editor
}

// isSafe says the function name is a safe fn.
func (r *rgen) isSafe(name string) bool {
	fx := r.fx[name]
	return fx != nil && !fx.unsafe
}

// directFx is what fd does itself, and whom it calls.
func (r *rgen) directFx(fd *cc.FunctionDefinition) *rsFx {
	fx := &rsFx{}
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	if ft == nil {
		fx.editor, fx.unsafe = true, true
		return fx
	}
	if rt := ft.Result(); rt != nil && (isAggr(rt) || rt.Kind() == cc.Array) {
		fx.unsafe = true // its zero, where the end is reached, is core::mem::zeroed()
	}
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.Declarator:
			if x.Name() == "__func__" || x.IsTypename() {
				break
			}
			t := x.Type()
			if t == nil {
				break
			}
			if x.StorageDuration() == cc.Static && !x.IsParam() && t.Kind() != cc.Function {
				fx.editor, fx.unsafe = true, true // a block-scope static is the editor's
				break
			}
			if isAggr(t) || t.Kind() == cc.Array {
				fx.unsafe = true // a local aggregate starts as core::mem::zeroed()
			}
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				if d, ok := x.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() != cc.Function {
					if _, obj := r.field[r.g.a.declKey(d)]; obj && (d.StorageDuration() == cc.Static || d.Linkage() != cc.None) {
						fx.editor, fx.unsafe = true, true
					}
				}
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionDeref {
				fx.unsafe = true
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionCast && (isFnPtr(x.Type()) || isFnPtr(x.CastExpression.Type())) {
				fx.unsafe = true // a transmute
			}
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionIndex, cc.PostfixExpressionPSelect, cc.PostfixExpressionComplit:
				fx.unsafe = true
			case cc.PostfixExpressionCall:
				d := fnDesignator(x.PostfixExpression)
				switch {
				case d == nil:
					fx.editor, fx.unsafe = true, true // through a pointer
				case d.Name() == "__builtin_expect":
				case r.defined[d.Name()] != nil:
					fx.calls = append(fx.calls, d.Name())
				default:
					fx.editor, fx.unsafe = true, true // the host's
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(fd.CompoundStatement)
	return fx
}
