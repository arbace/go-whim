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
