package togo

// hseffects.go is what each function of the unit does beyond computing its
// result, as the Haskell printer needs it (doc/HASKELL-IDIOMS.md, item 5):
//
//   - pure: it touches no memory -- no pointer read or written, no object of
//     the segment, no local in a frame -- and calls only pure functions: a
//     Haskell function of its arguments, `musl_isdigit :: Int32 -> Bool`,
//     whose loops are pure recursion (item 11);
//   - editor: it reaches the segment, the host or a function pointer, or
//     calls a function that does: only then does it take the editor, `ed'`.
//
// Each is closed over the calls: a function that calls an impure function
// is impure, one that calls a function taking the editor takes it.  What the
// host calls back (Profile.HsExports) and what a runtime body names keep the
// whole signature, since their callers are written by hand.

import (
	"regexp"

	"github.com/arbace/go-whim/crefactor/cc"
)

// hsFx is one function's effects.
type hsFx struct {
	impure bool // touches memory, or calls what does
	editor bool // needs the editor
	calls  []string
}

// effects computes every defined function's effects.
func (h *hgen) effects() {
	h.fx = map[string]*hsFx{}
	for name, fd := range h.defined {
		h.fx[name] = h.directFx(fd)
	}
	// the functions whose callers are hand-written: the whole signature
	keep := map[string]bool{}
	for _, n := range h.g.p.HsExports {
		keep[n] = true
	}
	for _, rb := range h.g.p.RuntimeBodies {
		if rb.Hs == nil {
			continue
		}
		keep[rb.Name] = true
		for _, w := range hsWordRe.FindAllString(rb.Hs("()"), -1) {
			if h.defined[w] != nil {
				keep[w] = true
			}
		}
	}
	for n := range keep {
		if fx := h.fx[n]; fx != nil {
			fx.impure, fx.editor = true, true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, fx := range h.fx {
			for _, c := range fx.calls {
				cf := h.fx[c]
				if cf == nil {
					continue
				}
				if cf.impure && !fx.impure {
					fx.impure, changed = true, true
				}
				if cf.editor && !fx.editor {
					fx.editor, changed = true, true
				}
			}
		}
	}
}

var hsWordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_']*`)

// pure says the function name is a Haskell function of its arguments.
func (h *hgen) pure(name string) bool {
	fx := h.fx[name]
	return fx != nil && !fx.impure
}

// takesEd says the function name takes the editor.
func (h *hgen) takesEd(name string) bool {
	fx := h.fx[name]
	return fx == nil || fx.editor
}

// directFx is what fd does itself, and whom it calls.
func (h *hgen) directFx(fd *cc.FunctionDefinition) *hsFx {
	fx := &hsFx{}
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	if ft == nil || isAggr(ft.Result()) && !h.tupleRet(fd.Declarator.Name()) {
		fx.impure = true // a struct result is written through sret'
	}
	if ft != nil {
		for _, p := range ft.Parameters() {
			if t := p.Type(); t != nil && (isAggr(t) || t.Kind() == cc.Array) {
				fx.impure = true // a struct by value is copied into the frame
			}
		}
	}
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.Declarator:
			if x.Name() == "__func__" {
				break // the front end's own, in every body, unused
			}
			// a local in the frame: an array, a struct or a union
			if t := x.Type(); t != nil && !x.IsTypename() && !h.sval[x] && (isAggr(t) || t.Kind() == cc.Array) {
				fx.impure = true
			}
			if _, seg := h.segOff[h.g.a.declKey(x)]; seg && x.StorageDuration() == cc.Static && !x.IsParam() {
				fx.impure, fx.editor = true, true // a block-scope static is in the segment
			}
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				if d, ok := x.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() != cc.Function {
					if _, seg := h.segOff[h.g.a.declKey(d)]; seg {
						fx.impure, fx.editor = true, true
					}
				}
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionAddrof:
				if fnDesignator(x.CastExpression) == nil && !h.outArg[x] {
					fx.impure = true
				}
			case cc.UnaryExpressionDeref:
				if h.outParamOf(fd, x.CastExpression) < 0 {
					fx.impure = true
				}
			}
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionSelect:
				if !h.svalNamed(x.PostfixExpression) {
					fx.impure = true
				}
			case cc.PostfixExpressionIndex, cc.PostfixExpressionPSelect, cc.PostfixExpressionComplit:
				fx.impure = true
			case cc.PostfixExpressionCall:
				d := fnDesignator(x.PostfixExpression)
				switch {
				case d == nil:
					fx.impure, fx.editor = true, true
				case d.Name() == "__builtin_expect":
				case h.defined[d.Name()] != nil:
					fx.calls = append(fx.calls, d.Name())
					if ct, ok := d.Type().(*cc.FunctionType); ok && isAggr(ct.Result()) && !h.tupleRet(d.Name()) {
						fx.impure = true // its result goes into the frame
					}
				default:
					fx.impure, fx.editor = true, true
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(fd.CompoundStatement)
	return fx
}

// outParamOf is the index of fd's out-parameter e names, or -1.
func (h *hgen) outParamOf(fd *cc.FunctionDefinition, e cc.ExpressionNode) int {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return -1
	}
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	if ft == nil {
		return -1
	}
	for _, i := range h.outs[fd.Declarator.Name()] {
		if pd := ft.Parameters()[i].Declarator; pd != nil && p.ResolvedTo() == pd {
			return i
		}
	}
	return -1
}

// svalNamed says e names a struct local that is a value.
func (h *hgen) svalNamed(e cc.ExpressionNode) bool {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return false
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	return ok && h.sval[d]
}
