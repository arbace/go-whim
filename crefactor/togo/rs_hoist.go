package togo

// rs_hoist.go takes C's side effects out of an expression where C's order
// lets it (doc/RUST-IDIOMS.md, item 6), so that the Rust says `*d = *src;
// d = d.wrapping_add(1);` where it said `*{ t1 = d; d = t1.wrapping_add(1);
// t1 } = ...`:
//
//   - an increment or decrement of a local of the function's own -- no
//     address taken, so that nothing but the function can see it -- named
//     nowhere else in the statement's expression and not in an operand C
//     may not evaluate (the right of && or ||, an arm of ?:), is done before
//     the statement (++x) or after it (x++), and the expression reads the
//     local: no other part of the expression can tell the difference;
//     where there is no after -- a condition, a return's value -- x++ reads
//     the local into a temporary first;
//   - an assignment that is the first thing a condition evaluates -- the
//     left operand all the way down, `(c = *p) != NUL` -- is done before
//     it, and the condition reads what it stored.
//
// A `while` whose condition does something is a `loop` that does it, then
// leaves when the condition is false (rs_fn.go).

import "github.com/arbace/go-whim/crefactor/cc"

// hoist finds what can be done outside e, records the values that stand
// for it in f.rsub, and returns the statements to write before e's and,
// where after, after it.  done forgets the values.
func (f *rfn) hoist(e cc.ExpressionNode, after, topOK bool) (pre, post []string) {
	if f.lowered_ {
		return nil, nil
	}
	count := map[*cc.Declarator]int{}
	var names func(n cc.Node)
	names = func(n cc.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
			if d, ok := p.ResolvedTo().(*cc.Declarator); ok {
				count[d]++
			}
		}
		walkChildrenFn(n, names)
	}
	names(e)
	// the top: what a statement of e is, through parentheses and (void)
	top := unparenE(e)
	for {
		c, ok := top.(*cc.CastExpression)
		if !ok || c.Case != cc.CastExpressionCast || c.Type() != nil && c.Type().Kind() != cc.Void {
			break
		}
		top = unparenE(c.CastExpression)
	}
	var visit func(n cc.Node, lazy, leading bool)
	visit = func(n cc.Node, lazy, leading bool) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				visit(x.ExpressionList, lazy, leading)
				return
			}
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				// a comma first evaluated: its left operands are statements
				// before, its value the last's
				if !leading || lazy || !topOK && cc.ExpressionNode(x) == top {
					return // a statement's own comma is its statements already
				}
				last := x
				for last.ExpressionList != nil {
					pre = append(pre, f.stmtsOf(last.AssignmentExpression)...)
					last = last.ExpressionList
				}
				visit(last.AssignmentExpression, lazy, leading)
				f.rsub[x] = f.expr(last.AssignmentExpression)
				return
			}
			visit(x.AssignmentExpression, lazy, leading)
			return
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
				if f.hoistInc(x, x.PostfixExpression, x.Case == cc.PostfixExpressionInc, true, top, topOK, lazy, after, count, &pre, &post) {
					return
				}
			case cc.PostfixExpressionCall:
				// the callee, then the arguments: none of them first in C's
				// order but by Rust's, so only the increments
				for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
					visit(l.AssignmentExpression, lazy, false)
				}
				return
			}
			walkChildrenFn(n, func(c cc.Node) { visit(c, lazy, leading && c == cc.Node(x.PostfixExpression)) })
			return
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
				if f.hoistInc(x, x.UnaryExpression, x.Case == cc.UnaryExpressionInc, false, top, topOK, lazy, after, count, &pre, &post) {
					return
				}
			case cc.UnaryExpressionAddrof, cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType:
				return
			}
			walkChildrenFn(n, func(c cc.Node) { visit(c, lazy, leading) })
			return
		case *cc.AssignmentExpression:
			if x.Case == cc.AssignmentExpressionCond {
				visit(x.ConditionalExpression, lazy, leading)
				return
			}
			// the lvalue and the value: their own increments first; the value
			// of a plain assignment is what Rust evaluates first, before the
			// place, when the place does nothing
			visit(x.UnaryExpression, lazy, false)
			visit(x.AssignmentExpression, lazy, leading && x.Case == cc.AssignmentExpressionAssign && !hasEffect(x.UnaryExpression))
			if leading && !lazy && (topOK || cc.ExpressionNode(x) != top) && !f.effect(x.UnaryExpression) {
				var lv lval
				b := f.capture(func() {
					var stmts []string
					stmts, lv = f.assigned(x)
					for _, s := range stmts {
						f.line("%s", s)
					}
				})
				pre = append(pre, rsLines(b)...)
				f.rsub[x] = lv.read()
			}
			return
		case *cc.LogicalAndExpression:
			if x.Case == cc.LogicalAndExpressionLAnd {
				visit(x.LogicalAndExpression, lazy, leading)
				visit(x.InclusiveOrExpression, true, false)
				return
			}
		case *cc.LogicalOrExpression:
			if x.Case == cc.LogicalOrExpressionLOr {
				visit(x.LogicalOrExpression, lazy, leading)
				visit(x.LogicalAndExpression, true, false)
				return
			}
		case *cc.ConditionalExpression:
			if x.Case == cc.ConditionalExpressionCond {
				visit(x.LogicalOrExpression, lazy, leading)
				visit(x.ExpressionList, true, false)
				visit(x.ConditionalExpression, true, false)
				return
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionCast {
				visit(x.CastExpression, lazy, leading)
				return
			}
		}
		// a binary operator: its left operand first
		first := true
		walkChildrenFn(n, func(c cc.Node) {
			if _, ok := c.(cc.ExpressionNode); !ok {
				return
			}
			visit(c, lazy, leading && first)
			first = false
		})
	}
	if f.rsub == nil {
		f.rsub = map[cc.ExpressionNode]rv{}
	}
	visit(e, false, true)
	return pre, post
}

// hoistInc takes an increment out, when it can: n is it, e its operand.
func (f *rfn) hoistInc(n cc.ExpressionNode, e cc.ExpressionNode, inc, postfix bool, top cc.ExpressionNode, topOK, lazy, after bool,
	count map[*cc.Declarator]int, pre, post *[]string) bool {
	if n == top && !topOK || lazy {
		return false
	}
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return false
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	if !ok || count[d] != 1 {
		return false
	}
	l := f.local[d]
	if l == nil || l.addr || l.t.Kind() == cc.Array || isAggr(l.t) {
		return false
	}
	if postfix && f.deadInc[e] {
		f.rsub[n] = rv{s: l.name, ty: f.r.constTy(declSlot(d), f.r.ty(l.t))} // the store is dead (rs_defer.go)
		return true
	}
	stmt := rsLines(f.capture(func() { f.incDecStmt(e, inc) }))
	ty := f.r.constTy(declSlot(d), f.r.ty(l.t))
	switch {
	case !postfix:
		*pre = append(*pre, stmt...)
		f.rsub[n] = rv{s: l.name, ty: ty}
	case after:
		*post = append(*post, stmt...)
		f.rsub[n] = rv{s: l.name, ty: ty}
	default:
		t := f.temp(l.t)
		if isConstPtr(ty) {
			f.retype(t, ty)
		}
		*pre = append(*pre, f.letTemp(t, l.name))
		*pre = append(*pre, stmt...)
		f.rsub[n] = rv{s: t, ty: ty}
	}
	return true
}

// done forgets what hoist recorded.
func (f *rfn) done() { f.rsub = nil }

// effect says evaluating e does something, beyond what hoist took out.
func (f *rfn) effect(e cc.Node) bool {
	if len(f.rsub) == 0 {
		return hasEffect(e)
	}
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		if x, ok := n.(cc.ExpressionNode); ok {
			if _, sub := f.rsub[x]; sub {
				return
			}
		}
		if hasEffectHere(n) {
			found = true
			return
		}
		walkChildrenFn(n, rec)
	}
	rec(e)
	return found
}

// hasEffectHere says n itself, not its operands, does something: hasEffect's
// cases.
func hasEffectHere(n cc.Node) bool {
	switch x := n.(type) {
	case *cc.PostfixExpression:
		return x.Case == cc.PostfixExpressionCall || x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec
	case *cc.UnaryExpression:
		return x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec
	case *cc.AssignmentExpression:
		return x.Case != cc.AssignmentExpressionCond
	case *cc.ExpressionList:
		return x.ExpressionList != nil
	}
	return false
}

// exprStatement is an expression statement: its increments of the
// function's own locals before or after it.
func (f *rfn) exprStatement(e cc.ExpressionNode) {
	if a, ok := unparenE(e).(*cc.AssignmentExpression); ok && f.deadStore[a] {
		// a store nothing reads (rs_defer.go): its value's effects alone
		if f.effect(a.AssignmentExpression) {
			f.exprStatement(a.AssignmentExpression)
		}
		return
	}
	pre, post := f.hoist(e, true, false)
	for _, s := range pre {
		f.line("%s", s)
	}
	f.exprStmt(e)
	for _, s := range post {
		f.line("%s", s)
	}
	f.done()
}

// hoists says hoist would take something out of e: a dry run, whose
// temporaries are given back.
func (f *rfn) hoists(e cc.ExpressionNode) bool {
	ntmp, norder := f.ntmp, len(f.order)
	var pre []string
	f.capture(func() { pre, _ = f.hoist(e, false, true) })
	f.done()
	for _, l := range f.order[norder:] {
		delete(f.taken, l.name)
	}
	f.ntmp, f.order = ntmp, f.order[:norder]
	return len(pre) > 0
}

// dryLines is what fn writes, as lines, its temporaries given back.
func (f *rfn) dryLines(fn func()) []string {
	ntmp, norder := f.ntmp, len(f.order)
	b := f.capture(fn)
	for _, l := range f.order[norder:] {
		delete(f.taken, l.name)
	}
	f.ntmp, f.order = ntmp, f.order[:norder]
	return rsLines(b)
}
