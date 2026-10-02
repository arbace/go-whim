package togo

// hsout.go is how the Haskell prints out-parameters as results
// (doc/HASKELL-IDIOMS.md, item 6): which they are, and which arguments and
// locals go with them, is decided in outparams.go, with the Scheme backend;
// here a function holds its out-parameters as values, keeps them live to
// every return, and a call binds what comes back --
// `getvcol :: ... -> Int32 -> IO (Int32, Int32)`, `(r, v1) <- getvcol ...
// v`.

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// outVars are the function's out-parameters, which it holds as values.
func (f *hfn) outVars() {
	f.isOut = map[*lvar]bool{}
	for _, i := range f.h.outs[f.name] {
		if i < len(f.lf.params) {
			v := f.lf.params[i]
			f.outs = append(f.outs, v)
			f.isOut[v] = true
		}
	}
}

// outDeref is the out-parameter e is *p of, or nil.
func (f *hfn) outDeref(e cc.ExpressionNode) *lvar {
	if len(f.outs) == 0 {
		return nil
	}
	u, ok := unparenE(e).(*cc.UnaryExpression)
	if !ok || u.Case != cc.UnaryExpressionDeref {
		return nil
	}
	return f.outNamed(u.CastExpression)
}

// outNamed is the out-parameter e names, or nil.
func (f *hfn) outNamed(e cc.ExpressionNode) *lvar {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return nil
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	if !ok {
		return nil
	}
	if v := f.lf.byDecl[d]; v != nil && f.isOut[v] {
		return v
	}
	return nil
}

// liveOuts has every out-parameter live in every block: its value goes out
// with every return.
func (f *hfn) liveOuts() {
	// and a struct that is a value, which the lowering's liveness does not
	// follow: its members go through every block
	vs := append([]*lvar{}, f.outs...)
	for s := range f.sv {
		vs = append(vs, s)
	}
	for _, v := range vs {
		for _, b := range f.lf.blocks {
			if f.live[b] == nil {
				f.live[b] = map[*lvar]bool{}
			}
			f.live[b][v] = true
		}
	}
}

// outCall is a call's out-arguments: for each, the caller's variable, in
// the callee's order.
func (f *hfn) outCall(call *cc.PostfixExpression) (map[int]*lvar, bool) {
	d := fnDesignator(call.PostfixExpression)
	if d == nil || len(f.h.outs[d.Name()]) == 0 {
		return nil, false
	}
	m := map[int]*lvar{}
	i := 0
	for l := call.ArgumentExpressionList; l != nil; l, i = l.ArgumentExpressionList, i+1 {
		if !f.h.isOut(d.Name(), i) {
			continue
		}
		u, x := addrOfLocal(l.AssignmentExpression)
		v := f.lf.byDecl[x]
		if u == nil || !f.h.outArg[u] || v == nil {
			f.no(call, "an out-argument that is not a local's address")
		}
		m[i] = v
	}
	return m, true
}
