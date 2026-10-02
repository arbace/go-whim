package togo

// rs_defer.go is a local declared with no value where rustc can see every
// read come after a store (doc/RUST-IDIOMS.md, item 5): `let col: colnr_T;`
// for the top's `let mut col: colnr_T = 0;` whose zero nothing reads.  What
// sinkDecls cannot place in one block -- a local first given its value in
// both arms of an `if`, or before a loop -- keeps its declaration at the
// top, and loses only the zero.
//
// The analysis is rustc's own on C's statements: a path-insensitive walk in
// evaluation order of whether the local is definitely stored (every read
// must come after) and maybe stored (a store then needs `mut`), joining the
// arms of an if, and a loop's body taken twice, as its second time round
// sees the first's stores.  A function with a goto, a switch naming the
// local, or a local whose address is taken keeps its zero.  Either way the
// claim is rustc's to check: a read it cannot see initialized, or a `mut`
// it finds missing or unneeded, does not compile.

import "github.com/arbace/go-whim/crefactor/cc"

// dstate is what a path knows of the local: stored on every path, or on
// some.
type dstate struct{ def, may bool }

func (a dstate) join(b dstate) dstate { return dstate{a.def && b.def, a.may || b.may} }

// defer_ is the walk for one local.
type defer_ struct {
	d       *cc.Declarator
	bad     bool // read before a store, or a use the walk does not follow
	needMut bool
	loops   []*dloop // the loops the walk is in
}

// deferred says the local l can be declared with no value, and whether it
// needs `mut`.
func (f *rfn) deferred(l *rlocal) (ok, mut bool) {
	if l.d == nil || l.param || l.temp || l.addr || isAggr(l.t) || l.t.Kind() == cc.Array || l.t.Kind() == cc.Union {
		return false, false
	}
	if rsHasGoto(f.fd.CompoundStatement) {
		return false, false
	}
	w := &defer_{d: l.d}
	_, div := w.items(f.fd.CompoundStatement, dstate{})
	_ = div
	return !w.bad, w.needMut
}

// rsHasGoto says n holds a goto.
func rsHasGoto(n cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		if j, ok := n.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementGoto {
			found = true
			return
		}
		walkChildrenFn(n, rec)
	}
	rec(n)
	return found
}

// names says n names the local.
func (w *defer_) names(n cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		if p, ok := n.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent && p.ResolvedTo() == w.d {
			found = true
			return
		}
		walkChildrenFn(n, rec)
	}
	rec(n)
	return found
}

// isMe says e is the local itself.
func (w *defer_) isMe(e cc.ExpressionNode) bool {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	return ok && p.Case == cc.PrimaryExpressionIdent && p.ResolvedTo() == w.d
}

func (w *defer_) store(s dstate) dstate {
	if s.may {
		w.needMut = true
	}
	return dstate{true, true}
}

// expr walks an expression in Rust's order of evaluation.
func (w *defer_) expr(n cc.Node, s dstate) dstate {
	if n == nil || w.bad {
		return s
	}
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent && x.ResolvedTo() == w.d {
			if !s.def {
				w.bad = true
			}
			return s
		}
	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return w.expr(x.ConditionalExpression, s)
		}
		if w.isMe(x.UnaryExpression) {
			s = w.expr(x.AssignmentExpression, s)
			if x.Case == cc.AssignmentExpressionAssign {
				return w.store(s)
			}
			if !s.def {
				w.bad = true
			}
			w.needMut = true
			return s
		}
		s = w.expr(x.AssignmentExpression, s) // the value first, then the place
		return w.expr(x.UnaryExpression, s)
	case *cc.PostfixExpression:
		if (x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec) && w.isMe(x.PostfixExpression) {
			if !s.def {
				w.bad = true
			}
			w.needMut = true
			return s
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			if w.isMe(x.UnaryExpression) {
				if !s.def {
					w.bad = true
				}
				w.needMut = true
				return s
			}
		case cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType:
			return s
		}
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionLAnd {
			s = w.expr(x.LogicalAndExpression, s)
			return s.join(w.expr(x.InclusiveOrExpression, s))
		}
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLOr {
			s = w.expr(x.LogicalOrExpression, s)
			return s.join(w.expr(x.LogicalAndExpression, s))
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			s = w.expr(x.LogicalOrExpression, s)
			return w.expr(x.ExpressionList, s).join(w.expr(x.ConditionalExpression, s))
		}
	}
	walkChildrenFn(n, func(c cc.Node) { s = w.expr(c, s) })
	return s
}

// items walks a block's items; div says it cannot complete.
func (w *defer_) items(cs *cc.CompoundStatement, s dstate) (dstate, bool) {
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		it := l.BlockItem
		switch it.Case {
		case cc.BlockItemDecl:
			d := it.Declaration
			if d == nil || d.Case != cc.DeclarationDecl {
				continue
			}
			for il := d.InitDeclaratorList; il != nil; il = il.InitDeclaratorList {
				id := il.InitDeclarator
				if id.Initializer == nil {
					continue
				}
				s = w.expr(id.Initializer, s)
				if id.Declarator == w.d {
					s = w.store(s)
				}
			}
		case cc.BlockItemStmt:
			var div bool
			s, div = w.stmt(it.Statement, s)
			if div {
				return s, true
			}
		}
	}
	return s, false
}

// stmt walks a statement; div says it cannot complete.
func (w *defer_) stmt(st *cc.Statement, s dstate) (dstate, bool) {
	if st == nil || w.bad {
		return s, false
	}
	switch st.Case {
	case cc.StatementCompound:
		return w.items(st.CompoundStatement, s)
	case cc.StatementExpr:
		if e := st.ExpressionStatement.ExpressionList; e != nil {
			s = w.expr(e, s)
		}
		return s, false
	case cc.StatementLabeled:
		if w.names(st) {
			w.bad = true // a case of a switch: not followed
		}
		return w.stmt(st.LabeledStatement.Statement, s)
	case cc.StatementJump:
		j := st.JumpStatement
		switch {
		case j.Case == cc.JumpStatementReturn && j.ExpressionList != nil:
			w.expr(j.ExpressionList, s)
		case j.Case == cc.JumpStatementBreak && len(w.loops) > 0:
			l := w.loops[len(w.loops)-1]
			if l.brkSet {
				l.brk = l.brk.join(s)
			} else {
				l.brk, l.brkSet = s, true
			}
		case j.Case == cc.JumpStatementContinue && w.loop() != nil:
			l := w.loop()
			switch {
			case l.once:
				// do { } while (0): its condition is false, so out
				if l.brkSet {
					l.brk = l.brk.join(s)
				} else {
					l.brk, l.brkSet = s, true
				}
			case l.contSet:
				l.cont = l.cont.join(s)
			default:
				l.cont, l.contSet = s, true
			}
		}
		return s, true
	case cc.StatementSelection:
		x := st.SelectionStatement
		if x.Case == cc.SelectionStatementSwitch {
			return w.switchStmt(x, s)
		}
		s = w.expr(x.ExpressionList, s)
		a, adiv := w.stmt(x.Statement, s)
		b, bdiv := s, false
		if x.Case == cc.SelectionStatementIfElse {
			b, bdiv = w.stmt(x.Statement2, s)
		}
		switch {
		case adiv && bdiv:
			return s, true
		case adiv:
			return b, false
		case bdiv:
			return a, false
		}
		return a.join(b), false
	case cc.StatementIteration:
		x := st.IterationStatement
		var cond, step cc.ExpressionNode
		switch x.Case {
		case cc.IterationStatementDo:
			if isConstFalse(x.ExpressionList) {
				// do { ... } while (0): a block its breaks leave
				acc := &dloop{once: true}
				w.loops = append(w.loops, acc)
				end, div := w.stmt(x.Statement, s)
				w.loops = w.loops[:len(w.loops)-1]
				switch {
				case acc.brkSet && div:
					return acc.brk, false
				case acc.brkSet:
					return end.join(acc.brk), false
				}
				return end, div
			}
			// the body at least once, then as a loop
			body := w.loopBody(x.Statement, nil, s)
			if rsLoopLeaves(x.Statement) {
				return dstate{s.def, body.may}, false
			}
			return w.expr(x.ExpressionList, body), false
		case cc.IterationStatementWhile:
			cond = x.ExpressionList
		case cc.IterationStatementFor:
			s = w.expr(x.ExpressionList, s)
			cond, step = x.ExpressionList2, x.ExpressionList3
		case cc.IterationStatementForDecl:
			s, _ = w.items(&cc.CompoundStatement{BlockItemList: &cc.BlockItemList{BlockItem: &cc.BlockItem{Case: cc.BlockItemDecl, Declaration: x.Declaration}}}, s)
			cond, step = x.ExpressionList, x.ExpressionList2
		}
		s = w.expr(cond, s)
		body := w.loopBody(x.Statement, step, s)
		if cond != nil {
			body = w.expr(cond, body)
		}
		return dstate{s.def, body.may}, false
	}
	w.bad = true
	return s, false
}

// loopBody walks a loop's body and step twice -- the second time as the
// first left it, as rustc sees a store the second time round -- and
// returns what the first time left: its end and its continues joined, and
// with may, what its breaks stored too.
func (w *defer_) loopBody(body *cc.Statement, step cc.ExpressionNode, s dstate) dstate {
	acc := &dloop{}
	w.loops = append(w.loops, acc)
	one, div := w.stmt(body, s)
	w.loops = w.loops[:len(w.loops)-1]
	switch {
	case acc.contSet && div:
		one = acc.cont
	case acc.contSet:
		one = one.join(acc.cont)
	case div:
		one = s // no way round but by the start
	}
	one = w.expr(step, one)
	w.loops = append(w.loops, &dloop{})
	again, _ := w.stmt(body, s.join(one))
	w.loops = w.loops[:len(w.loops)-1]
	w.expr(step, again)
	if acc.brkSet {
		one.may = one.may || acc.brk.may
	}
	return one
}

// dloop is what a loop's breaks and continues left.
type dloop struct {
	brk, cont       dstate
	brkSet, contSet bool
	sw              bool // a switch's: a break's, not a continue's
	once            bool // do { } while (0)'s: a continue leaves it
}

// rsLoopLeaves says a loop's body has a break or a continue of its own.
func rsLoopLeaves(s cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		switch x := n.(type) {
		case *cc.IterationStatement:
			return
		case *cc.SelectionStatement:
			if x.Case == cc.SelectionStatementSwitch {
				// its breaks are its own; a continue is the loop's
				if rsHasContinue(x) {
					found = true
				}
				return
			}
		case *cc.JumpStatement:
			if x.Case == cc.JumpStatementBreak || x.Case == cc.JumpStatementContinue {
				found = true
			}
			return
		}
		walkChildrenFn(n, rec)
	}
	walkChildrenFn(s, rec)
	return found
}

// switchStmt walks a switch: each case entered from the switch's head, or
// fallen into from the case before; out at a break, at the end, or -- with
// no default -- past every case, as the match's `_ => {}` is.
func (w *defer_) switchStmt(x *cc.SelectionStatement, s dstate) (dstate, bool) {
	s = w.expr(x.ExpressionList, s)
	if !w.names(x.Statement) {
		return s, false
	}
	if x.Statement.Case != cc.StatementCompound {
		w.bad = true
		return s, false
	}
	acc := &dloop{sw: true}
	w.loops = append(w.loops, acc)
	defer func() { w.loops = w.loops[:len(w.loops)-1] }()
	cur, live, hasDefault := s, false, false
	for l := x.Statement.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
		labels, st := caseChain(l.BlockItem)
		if len(labels) > 0 {
			for _, lb := range labels {
				if lb.Case == cc.LabeledStatementDefault {
					hasDefault = true
				}
			}
			if live {
				cur = cur.join(s)
			} else {
				cur, live = s, true
			}
			var div bool
			cur, div = w.stmt(st, cur)
			live = !div
			continue
		}
		if !live {
			continue
		}
		var div bool
		cur, div = w.items(&cc.CompoundStatement{BlockItemList: &cc.BlockItemList{BlockItem: l.BlockItem}}, cur)
		live = !div
	}
	var out dstate
	have := false
	add := func(d dstate) {
		if have {
			out = out.join(d)
		} else {
			out, have = d, true
		}
	}
	if live {
		add(cur)
	}
	if acc.brkSet {
		add(acc.brk)
	}
	if !hasDefault {
		add(s)
	}
	if !have {
		return s, true
	}
	return out, false
}

// loop is the innermost loop the walk is in, past its switches.
func (w *defer_) loop() *dloop {
	for i := len(w.loops) - 1; i >= 0; i-- {
		if !w.loops[i].sw {
			return w.loops[i]
		}
	}
	return nil
}
