package togo

// effects.go is what each function of the unit does beyond computing its
// result, as the backends that keep C's memory raw ask it (cfacts.go;
// doc/HASKELL-IDIOMS.md, item 5, and doc/SCHEME-IDIOMS.md):
//
//   - pure: it touches no memory -- no pointer read or written, no object of
//     the segment, no local in a frame -- and calls only pure functions: a
//     function of its arguments, `musl_isdigit :: Int32 -> Bool` in the
//     Haskell, whose loops are pure recursion;
//   - editor: it reaches the segment, the host or a function pointer, or
//     calls a function that does: only then does it take the editor.
//
// Each is closed over the calls: a function that calls an impure function
// is impure, one that calls a function taking the editor takes it.  What the
// host calls back and what a runtime body names (cfacts.keep) keep the
// whole signature, since their callers are written by hand.
//
// The Rust (rs_fx.go) asks the editor of it, and closes its own judgement
// -- what Rust calls unsafe -- over the same calls (callersOf).

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// cfx is one function's effects.
type cfx struct {
	impure bool // touches memory, or calls what does
	editor bool // needs the editor
	calls  []string
}

// effects computes every defined function's effects.
func (c *cfacts) effects() {
	c.fx = map[string]*cfx{}
	for name, fd := range c.defined {
		c.fx[name] = c.directFx(fd)
	}
	// the functions whose callers are hand-written: the whole signature
	for n := range c.keep {
		if fx := c.fx[n]; fx != nil {
			fx.impure, fx.editor = true, true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, fx := range c.fx {
			for _, callee := range fx.calls {
				cf := c.fx[callee]
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

// pure says the function name is a function of its arguments alone.
func (c *cfacts) pure(name string) bool {
	fx := c.fx[name]
	return fx != nil && !fx.impure
}

// takesEd says the function name takes the editor.
func (c *cfacts) takesEd(name string) bool {
	fx := c.fx[name]
	return fx == nil || fx.editor
}

// callersOf is seed closed over the calls: the defined functions that are
// in it, or call one that is.  A backend's own judgement of what a body
// does is closed over the same call graph as the shared ones (rs_fx.go).
func (c *cfacts) callersOf(seed map[string]bool) map[string]bool {
	in := map[string]bool{}
	for n := range seed {
		if c.fx[n] != nil {
			in[n] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for n, fx := range c.fx {
			if in[n] {
				continue
			}
			for _, callee := range fx.calls {
				if in[callee] {
					in[n], changed = true, true
					break
				}
			}
		}
	}
	return in
}

// directFx is what fd does itself, and whom it calls.
func (c *cfacts) directFx(fd *cc.FunctionDefinition) *cfx {
	fx := &cfx{}
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	if ft == nil || isAggr(ft.Result()) && !c.tupleRet(fd.Declarator.Name()) {
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
			if t := x.Type(); t != nil && !x.IsTypename() && !c.sval[x] && (isAggr(t) || t.Kind() == cc.Array) {
				fx.impure = true
			}
			if _, seg := c.segOff[c.g.a.declKey(x)]; seg && x.StorageDuration() == cc.Static && !x.IsParam() {
				fx.impure, fx.editor = true, true // a block-scope static is in the segment
			}
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				if d, ok := x.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() != cc.Function {
					if _, seg := c.segOff[c.g.a.declKey(d)]; seg {
						fx.impure, fx.editor = true, true
					}
				}
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionAddrof:
				if fnDesignator(x.CastExpression) == nil && !c.outArg[x] {
					fx.impure = true
				}
			case cc.UnaryExpressionDeref:
				if c.outParamOf(fd, x.CastExpression) < 0 {
					fx.impure = true
				}
			}
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionSelect:
				if !c.svalNamed(x.PostfixExpression) {
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
				case c.defined[d.Name()] != nil:
					fx.calls = append(fx.calls, d.Name())
					if ct, ok := d.Type().(*cc.FunctionType); ok && isAggr(ct.Result()) && !c.tupleRet(d.Name()) {
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
func (c *cfacts) outParamOf(fd *cc.FunctionDefinition, e cc.ExpressionNode) int {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return -1
	}
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	if ft == nil {
		return -1
	}
	for _, i := range c.outs[fd.Declarator.Name()] {
		if pd := ft.Parameters()[i].Declarator; pd != nil && p.ResolvedTo() == pd {
			return i
		}
	}
	return -1
}

// svalNamed says e names a struct local that is a value.
func (c *cfacts) svalNamed(e cc.ExpressionNode) bool {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return false
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	return ok && c.sval[d]
}
