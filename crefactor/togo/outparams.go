package togo

// outparams.go is out-parameters as results, as the backends that keep C's
// memory raw ask it (cfacts.go; doc/HASKELL-IDIOMS.md, item 6): a
// function's parameter `T *p` whose callee only reads and writes *p and asks
// whether p is null, and whose every caller passes `&x` of a local x of its
// own that nothing else takes the address of, is a value in and a value out
// -- in the Haskell `getvcol :: ... -> Int32 -> IO (Int32, Int32)`, `(r, v1)
// <- getvcol ... v`, in the Scheme multiple values -- and x a binding, not a
// slot in the frame.  An in-out parameter is the same thing, since the value
// goes in.  It is exact because x is reachable only through the parameter:
// the callee cannot see it by another way, so when the value comes back is
// when C's stores through p would be seen -- which is the proof phase 94
// (MemberOut) makes for members, simpler for locals.
//
// A caller whose call is where C evaluates lazily -- the right of && or ||,
// an arm of ?: -- keeps its x in the frame, read before the call and written
// after it, so that the value lives past the branch the call is in.

import (
	"fmt"
	"sort"

	"github.com/arbace/go-whim/crefactor/cc"
)

// outParams decides the out-parameters: c.outs, c.outArg and c.outLazy.
func (c *cfacts) outParams() {
	c.outs = map[string][]int{}
	c.outArg = map[*cc.UnaryExpression]bool{}
	c.outLazy = map[*cc.Declarator]bool{}
	// the candidates: what each callee does with the parameter
	cand := map[string]map[int]bool{}
	for name, fd := range c.defined {
		ft, _ := fd.Declarator.Type().(*cc.FunctionType)
		if ft == nil || ft.IsVariadic() || isAggr(ft.Result()) || c.keep[name] || c.g.a.addr[name] {
			continue
		}
		for i, p := range ft.Parameters() {
			if outCandidate(fd, p) {
				if cand[name] == nil {
					cand[name] = map[int]bool{}
				}
				cand[name][i] = true
			}
		}
	}
	// the calls: every one must pass &x of a local of the right type, and
	// each x once a call
	type site struct {
		u   *cc.UnaryExpression
		x   *cc.Declarator
		g   string
		i   int
		fd  *cc.FunctionDefinition
		lzy bool
	}
	var sites []site
	for _, fd := range c.defined {
		walkCalls(fd.CompoundStatement, false, func(call *cc.PostfixExpression, lazy bool) {
			d := fnDesignator(call.PostfixExpression)
			if d == nil || cand[d.Name()] == nil {
				return
			}
			g := d.Name()
			seen := map[*cc.Declarator]bool{}
			i := 0
			for l := call.ArgumentExpressionList; l != nil; l, i = l.ArgumentExpressionList, i+1 {
				if !cand[g][i] {
					continue
				}
				u, x := addrOfLocal(l.AssignmentExpression)
				ft := d.Type().(*cc.FunctionType)
				if x == nil || seen[x] || scalarClass(x.Type()) != scalarClass(elemOf(ft.Parameters()[i].Type())) && !(isPtrish(x.Type()) && isPtrish(elemOf(ft.Parameters()[i].Type()))) {
					delete(cand[g], i)
					continue
				}
				seen[x] = true
				sites = append(sites, site{u, x, g, i, fd, lazy})
			}
		})
	}
	// the locals: every address taken must be such an argument, or the
	// local escapes and its calls' parameters are not out-parameters
	for changed := true; changed; {
		changed = false
		good := map[*cc.UnaryExpression]bool{}
		for _, s := range sites {
			if cand[s.g][s.i] {
				good[s.u] = true
			}
		}
		escapes := map[*cc.Declarator]bool{}
		for _, fd := range c.defined {
			walkAddrs(fd.CompoundStatement, func(u *cc.UnaryExpression, x *cc.Declarator) {
				if !good[u] {
					escapes[x] = true
				}
			})
		}
		for _, s := range sites {
			if cand[s.g][s.i] && escapes[s.x] {
				delete(cand[s.g], s.i)
				changed = true
			}
		}
	}
	for g, is := range cand {
		for i := range is {
			c.outs[g] = append(c.outs[g], i)
		}
		sort.Ints(c.outs[g])
	}
	for _, s := range sites {
		if cand[s.g][s.i] {
			c.outArg[s.u] = true
			if s.lzy {
				c.outLazy[s.x] = true
			}
		}
	}
}

// isOut says parameter i of g is an out-parameter.
func (c *cfacts) isOut(g string, i int) bool {
	for _, j := range c.outs[g] {
		if j == i {
			return true
		}
	}
	return false
}

// outCandidate says fd uses its parameter p only as *p and in a test of
// whether it is null.
func outCandidate(fd *cc.FunctionDefinition, p *cc.Parameter) bool {
	t := p.Type()
	if t == nil || t.Kind() != cc.Ptr {
		return false
	}
	e := elemOf(t)
	if e == nil || isAggr(e) || e.Kind() == cc.Array || e.Kind() == cc.Function || e.Kind() == cc.Void {
		return false
	}
	pd := p.Declarator
	if pd == nil {
		return false
	}
	ok, used := true, false
	var rec func(n cc.Node, parent cc.Node)
	rec = func(n cc.Node, parent cc.Node) {
		if n == nil || !ok {
			return
		}
		if x, isP := n.(*cc.PrimaryExpression); isP && x.Case == cc.PrimaryExpressionIdent && x.ResolvedTo() == pd {
			used = true
			if !outUse(parent) {
				ok = false
			}
			return
		}
		walkChildrenFn(n, func(c cc.Node) { rec(c, outParent(n, parent)) })
	}
	rec(fd.CompoundStatement, nil)
	return ok && used
}

// outParent is the parent a child of n sees: a parenthesis is its own
// parent's.
func outParent(n, parent cc.Node) cc.Node {
	if x, ok := n.(*cc.PrimaryExpression); ok && x.Case == cc.PrimaryExpressionExpr {
		return parent
	}
	if x, ok := n.(*cc.ExpressionList); ok && x.ExpressionList == nil {
		return parent
	}
	return n
}

// outUse says the parameter's use under parent is one an out-parameter
// allows: *p, or a test of p against null.
func outUse(parent cc.Node) bool {
	switch x := parent.(type) {
	case *cc.UnaryExpression:
		return x.Case == cc.UnaryExpressionDeref || x.Case == cc.UnaryExpressionNot
	case *cc.EqualityExpression:
		return isNullConst(x.EqualityExpression) || isNullConst(x.RelationalExpression)
	}
	return false
}

// addrOfLocal is e as &x of a local or a parameter x, when it is.
func addrOfLocal(e cc.ExpressionNode) (*cc.UnaryExpression, *cc.Declarator) {
	u, ok := unparenE(e).(*cc.UnaryExpression)
	if !ok || u.Case != cc.UnaryExpressionAddrof {
		return nil, nil
	}
	p, ok := unparenE(u.CastExpression).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return nil, nil
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	if !ok || d.Type() == nil || d.StorageDuration() == cc.Static || d.Linkage() != cc.None {
		return nil, nil
	}
	if t := d.Type(); isAggr(t) || t.Kind() == cc.Array || t.Kind() == cc.Function {
		return nil, nil
	}
	return u, d
}

// walkCalls calls fn with every call under n, and whether C evaluates it
// only on a condition: the right of && or ||, an arm of ?:.
func walkCalls(n cc.Node, lazy bool, fn func(*cc.PostfixExpression, bool)) {
	if n == nil {
		return
	}
	switch x := n.(type) {
	case *cc.PostfixExpression:
		if x.Case == cc.PostfixExpressionCall {
			fn(x, lazy)
		}
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionLAnd {
			walkCalls(x.LogicalAndExpression, lazy, fn)
			walkCalls(x.InclusiveOrExpression, true, fn)
			return
		}
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLOr {
			walkCalls(x.LogicalOrExpression, lazy, fn)
			walkCalls(x.LogicalAndExpression, true, fn)
			return
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			walkCalls(x.LogicalOrExpression, lazy, fn)
			walkCalls(x.ExpressionList, true, fn)
			walkCalls(x.ConditionalExpression, true, fn)
			return
		}
	}
	walkChildrenFn(n, func(c cc.Node) { walkCalls(c, lazy, fn) })
}

// walkAddrs calls fn with every &x of a local under n.
func walkAddrs(n cc.Node, fn func(*cc.UnaryExpression, *cc.Declarator)) {
	if n == nil {
		return
	}
	if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
		if p, ok := unparenE(u.CastExpression).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
			if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() != cc.Function {
				fn(u, d)
			}
		}
	}
	walkChildrenFn(n, func(c cc.Node) { walkAddrs(c, fn) })
}

// scalarClass is what an out-parameter's type and its argument's must agree
// on: an integer's size and signedness, a bool, a pointer.
func scalarClass(t cc.Type) string {
	if t == nil {
		return "void"
	}
	switch t.Kind() {
	case cc.Void:
		return "void"
	case cc.Ptr, cc.Array, cc.Function:
		return "ptr"
	case cc.Struct, cc.Union:
		return "agg"
	}
	k, ok := scalarKind(t)
	switch {
	case !ok:
		return "?" + t.String()
	case k.boolean:
		return "bool"
	case k.signed:
		return fmt.Sprintf("s%d", k.size*8)
	}
	return fmt.Sprintf("u%d", k.size*8)
}
