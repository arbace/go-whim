package togo

// java_da.go is Java's definite assignment for the printer's locals
// (doc/JAVA-IDIOMS.md, item 2): a local declared where C declares it with no
// initializer is written `int n;`, not `int n = 0;`, when no path from its
// declaration reads it before a store to it.  The analysis is at most as
// bold as JLS 16's -- a store in a loop does not count after the loop, a
// store behind && or ?: does not count, a switch's case is entered with
// what the switch had -- so javac accepts every local it passes, and one it
// cannot is a compile error, not a wrong editor.

import "github.com/arbace/go-whim/crefactor/cc"

// daFirst says no path from d's declaration to the end of its block reads d
// before storing to it.
func (f *jfn) daFirst(d *cc.Declarator) bool {
	if f.par == nil {
		f.par = map[cc.Node]cc.Node{}
		var rec func(cc.Node)
		rec = func(n cc.Node) {
			walkChildrenFn(n, func(c cc.Node) {
				f.par[c] = n
				rec(c)
			})
		}
		rec(f.fbody)
	}
	// the declaration's block item, and what follows it in its block
	var n cc.Node = d
	for n != nil {
		if _, ok := n.(*cc.BlockItem); ok {
			break
		}
		n = f.par[n]
	}
	list, ok := f.par[n].(*cc.BlockItemList)
	if !ok {
		return false
	}
	w := &jda{d: d}
	u := true
	for l := list.BlockItemList; l != nil && !w.fail; l = l.BlockItemList {
		u = w.item(l.BlockItem, u)
	}
	return !w.fail
}

// jda follows whether d may still be unassigned (u): a read then fails, a
// store clears it; what follows a jump is reached with nothing (false), a
// case label with what its switch had.
type jda struct {
	d       *cc.Declarator
	fail    bool
	switch_ []bool
}

func (w *jda) names(n cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(m cc.Node) {
		if found || m == nil {
			return
		}
		if pe, ok := m.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && pe.ResolvedTo() == cc.Node(w.d) {
			found = true
			return
		}
		walkChildrenFn(m, rec)
	}
	rec(n)
	return found
}

func (w *jda) item(bi *cc.BlockItem, u bool) bool {
	if bi.Statement == nil {
		if u && w.names(bi) {
			w.fail = true
		}
		return u
	}
	return w.stmt(bi.Statement, u)
}

// expr is an expression C evaluates whole: a store of d that is its only
// name of d, `d = R` at its top, clears u; any other name of d while u is a
// read.
func (w *jda) expr(e cc.Node, u bool) bool {
	if e == nil {
		return u
	}
	x := unparenE(e.(cc.ExpressionNode))
	if l, ok := x.(*cc.ExpressionList); ok && l.ExpressionList != nil {
		// a comma: its operands in order
		for ; l != nil; l = l.ExpressionList {
			u = w.expr(l.AssignmentExpression, u)
		}
		return u
	}
	if as, ok := x.(*cc.AssignmentExpression); ok && as.Case == cc.AssignmentExpressionAssign {
		if p, ok := unparenE(as.UnaryExpression).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent && p.ResolvedTo() == cc.Node(w.d) {
			if u && w.names(as.AssignmentExpression) {
				w.fail = true
			}
			return false
		}
	}
	if u && w.names(e) {
		w.fail = true
	}
	return u
}

func (w *jda) stmt(st *cc.Statement, u bool) bool {
	if w.fail {
		return u
	}
	switch st.Case {
	case cc.StatementCompound:
		for l := st.CompoundStatement.BlockItemList; l != nil && !w.fail; l = l.BlockItemList {
			u = w.item(l.BlockItem, u)
		}
		return u
	case cc.StatementExpr:
		if st.ExpressionStatement.ExpressionList == nil {
			return u
		}
		return w.expr(st.ExpressionStatement.ExpressionList, u)
	case cc.StatementSelection:
		sel := st.SelectionStatement
		if u && w.names(sel.ExpressionList) {
			w.fail = true
			return u
		}
		switch sel.Case {
		case cc.SelectionStatementIf:
			return w.stmt(sel.Statement, u) || u
		case cc.SelectionStatementIfElse:
			a := w.stmt(sel.Statement, u)
			b := w.stmt(sel.Statement2, u)
			return a || b
		default: // a switch: every case entered with what it had
			w.switch_ = append(w.switch_, u)
			w.stmt(sel.Statement, u)
			w.switch_ = w.switch_[:len(w.switch_)-1]
			return u
		}
	case cc.StatementIteration:
		// a loop: its stores do not hold after it, and each turn starts
		// from what the loop was entered with
		if u && w.names(st) {
			w.fail = true
		}
		return u
	case cc.StatementJump:
		js := st.JumpStatement
		if js.Case == cc.JumpStatementReturn && js.ExpressionList != nil && u && w.names(js.ExpressionList) {
			w.fail = true
		}
		if js.Case == cc.JumpStatementGoto || js.Case == cc.JumpStatementGotoExpr {
			w.fail = true
		}
		return false
	case cc.StatementLabeled:
		ls := st.LabeledStatement
		if ls.Case == cc.LabeledStatementLabel || len(w.switch_) == 0 {
			w.fail = true
			return u
		}
		return w.stmt(ls.Statement, u || w.switch_[len(w.switch_)-1])
	}
	if u && w.names(st) {
		w.fail = true
	}
	return u
}

// inlineStep says a ++ or -- of the variable e (its place lv), at n, may be
// Java's own in the expression, `a[i++]`, where the Go had to take it out
// before the statement (doc/JAVA-IDIOMS.md, item 9).  Java evaluates left
// to right, and C leaves unsequenced only what a valid C program does not
// do: read or write the variable elsewhere in the full expression.  So it
// may be inline when the full expression names the variable only there,
// and, for a field, calls nothing, which could read it.  A number held
// plainly: not a pointer (immutable, so walked by a new one), not a bool,
// not boxed.
func (f *jfn) inlineStep(e cc.ExpressionNode, lv jlv, n cc.Node) bool {
	if !lv.op || isJPtr(lv.t) || lv.t == "boolean" || f.fbody == nil {
		return false
	}
	return f.alone(e, n)
}

// inlineAssign says a plain = to a variable, x, may be Java's own
// assignment expression, `(p = ml_get(...)) != null`, where the Go took it
// out before the statement: by inlineStep's rule, the assignment's own
// right side free to name the variable, which C and Java both read before
// the store.  A scalar or a reference held plainly, not a struct.
func (f *jfn) inlineAssign(x *cc.AssignmentExpression, lv jlv) bool {
	if x.Case != cc.AssignmentExpressionAssign || !lv.op || isAggr(lv.c) || f.fbody == nil {
		return false
	}
	return f.alone(x.UnaryExpression, x)
}

// alone says the variable e names may be written by n in place: e is an
// identifier of a local or a field, not boxed, and its full expression
// names it elsewhere, or (for a field) calls anything, only where C
// sequences it with n (an operand of && || ?: or comma apart from n's) --
// Java evaluates left to right, and C leaves unsequenced only what a valid
// program does not do.
func (f *jfn) alone(e cc.ExpressionNode, n cc.Node) bool {
	d := identDecl(unparenE(e))
	if d == nil || f.j.isBoxed(d) {
		return false
	}
	local := d.StorageDuration() == cc.Automatic
	if !local && d.StorageDuration() != cc.Static {
		return false
	}
	if f.par == nil {
		f.par = map[cc.Node]cc.Node{}
		var rec func(cc.Node)
		rec = func(m cc.Node) {
			walkChildrenFn(m, func(c cc.Node) {
				f.par[c] = m
				rec(c)
			})
		}
		rec(f.fbody)
	}
	// the full expression: up to the first node that is none
	root := n
	for {
		p := f.par[root]
		if p == nil {
			return false
		}
		if _, ok := p.(cc.ExpressionNode); !ok {
			break
		}
		root = p
	}
	inside := func(m cc.Node) bool {
		for ; m != nil; m = f.par[m] {
			if m == n {
				return true
			}
		}
		return false
	}
	ok := true
	walkNodes(root, func(m cc.Node) {
		if !ok || inside(m) {
			return
		}
		touch := identDecl(m) == d
		if c, is := m.(*cc.PostfixExpression); is && c.Case == cc.PostfixExpressionCall && !local {
			touch = true
		}
		if touch && !f.sequenced(m, n) {
			ok = false
		}
	})
	return ok
}

// sequenced says a and b are in different operands of an && || ?: or comma:
// C orders them, as Java does.
func (f *jfn) sequenced(a, b cc.Node) bool {
	up := map[cc.Node]cc.Node{} // an ancestor of a -> its child on a's path
	for c, p := a, f.par[a]; p != nil; c, p = p, f.par[p] {
		up[p] = c
	}
	for c, p := b, f.par[b]; p != nil; c, p = p, f.par[p] {
		ca, shared := up[p]
		if !shared {
			continue
		}
		// p is the lowest common ancestor; ca and c its children
		if ca == c {
			return false
		}
		switch x := p.(type) {
		case *cc.LogicalAndExpression:
			return x.Case == cc.LogicalAndExpressionLAnd
		case *cc.LogicalOrExpression:
			return x.Case == cc.LogicalOrExpressionLOr
		case *cc.ConditionalExpression:
			return x.Case == cc.ConditionalExpressionCond
		case *cc.ExpressionList:
			return true
		}
		return false
	}
	return false
}
