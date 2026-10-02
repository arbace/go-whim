package togo

// rs_defer.go is a local declared with no value where rustc can see every
// read come after a store (doc/RUST-IDIOMS.md, item 5): `let col: colnr_T;`
// for the top's `let mut col: colnr_T = 0;` whose zero nothing reads.  What
// sinkDecls cannot place in one block -- a local first given its value in
// both arms of an `if`, or before a loop -- keeps its declaration at the
// top, and loses only the zero.
//
// The analysis is rustc's own on C's statements: a walk in evaluation order
// of whether the local is definitely stored (every read must come after)
// and maybe stored (a store then needs `mut`), joining the arms of an if,
// a condition's true and false ways apart (the right of && only where the
// left is true), a loop's body taken twice, as its second time round sees
// the first's stores, a loop with no condition left only by its breaks,
// and a goto's state joined at its label (item 13).  An address taken, a
// member or an element written, or an array reached are reads that need
// `mut`.  A switch naming the local, a label inside a statement and a
// goto back are not followed: the local keeps its zero.  Either way the
// claim is rustc's to check: a read it cannot see initialized, or a `mut`
// it finds missing or unneeded, does not compile.
//
// The same walk, following one store's value (live), finds C's dead
// stores (deadStores, deadInits, the parameters' values, deadIncs): rustc
// counts each as a value assigned and never read; each is not written --
// but one to a local whose address is taken, which a pointer may read:
// that one is kept, and its function says `#[expect(unused_assignments)]`,
// which rustc holds to its own count.  So the module allows no
// unused_assignments.

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
	// what the gotos to each label left (a goto is a forward jump to a
	// label of a block holding it: a labeled block it breaks)
	gotos  map[string]dstate
	joined map[*cc.LabeledStatement]bool // the labels of a block's items, whose gotos it joins
	backTo map[string]bool               // the labels a goto after them reaches: not followed
	// sharedRef says an argument is passed to a & reference: `&x`, no mut
	sharedRef func(cc.ExpressionNode) bool
	skipInit  bool // the local's initializer is not written: no store
	// the stores not written (deadStores), and the liveness walk's: the
	// store under test, and whether its value is read
	dead     map[*cc.AssignmentExpression]bool
	deadInc  map[cc.ExpressionNode]bool
	live     cc.Node
	liveRead bool
}

// deferred says the local l can be declared with no value, and whether it
// needs `mut`.
func (f *rfn) deferred(l *rlocal) (ok, mut bool) {
	if l.d == nil || l.param || l.temp {
		return false, false
	}
	return f.deferredFrom(l)
}

// deferredFrom is deferred's walk, of a local or of a parameter.
func (f *rfn) deferredFrom(l *rlocal) (ok, mut bool) {
	w := &defer_{d: l.d, skipInit: l.deadInit, gotos: map[string]dstate{}, joined: map[*cc.LabeledStatement]bool{}, backTo: rsBackGotos(f.fd), dead: f.deadStore, deadInc: f.deadInc,
		sharedRef: func(e cc.ExpressionNode) bool { ref := f.r.refArgs[e]; return ref != nil && !ref.mut }}
	_, div := w.items(f.fd.CompoundStatement, dstate{})
	_ = div
	return !w.bad, w.needMut
}

// rsBackGotos are the labels of fd a goto after them reaches.
func rsBackGotos(fd *cc.FunctionDefinition) map[string]bool {
	at := map[string]int{}
	back := map[string]bool{}
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.LabeledStatement:
			if x.Case == cc.LabeledStatementLabel {
				at[x.Token.SrcStr()] = len(at) + 1
			}
		case *cc.JumpStatement:
			if x.Case == cc.JumpStatementGoto && at[x.Token2.SrcStr()] > 0 {
				back[x.Token2.SrcStr()] = true
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(fd.CompoundStatement)
	return back
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

// storeAt is a store of the local at node n.  In the liveness walk (live)
// may is whether the value the store under test stored is the local's.
func (w *defer_) storeAt(s dstate, n cc.Node) dstate {
	if w.live != nil {
		return dstate{true, n == w.live}
	}
	if s.may {
		w.needMut = true
	}
	return dstate{true, true}
}

// reStore is the local stored again from its own value (x++, x += v).
func (w *defer_) reStore(s dstate, n cc.Node) dstate {
	if w.live != nil {
		s.may = n == w.live
	}
	return s
}

// read is a read of the local: before any store a walk the deferral
// cannot follow; in the liveness walk, a read of the store under test's
// value where it may be the local's.
func (w *defer_) read(s dstate) {
	if w.live != nil {
		if s.may {
			w.liveRead = true
		}
		return
	}
	if !s.def {
		w.bad = true
	}
}

// memberOfMe says e is a member of the local, through members only.
func (w *defer_) memberOfMe(e cc.ExpressionNode) bool {
	p, ok := unparenE(e).(*cc.PostfixExpression)
	for ok && p.Case == cc.PostfixExpressionSelect {
		if w.isMe(p.PostfixExpression) {
			return true
		}
		p, ok = unparenE(p.PostfixExpression).(*cc.PostfixExpression)
	}
	return false
}

// expr walks an expression in Rust's order of evaluation.
func (w *defer_) expr(n cc.Node, s dstate) dstate {
	if n == nil || w.bad {
		return s
	}
	w.mutUse(n)
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent && x.ResolvedTo() == w.d {
			w.read(s)
			return s
		}
	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return w.expr(x.ConditionalExpression, s)
		}
		if w.dead[x] {
			return w.expr(x.AssignmentExpression, s) // not written: its value's effects alone
		}
		if w.isMe(x.UnaryExpression) {
			s = w.expr(x.AssignmentExpression, s)
			if x.Case == cc.AssignmentExpressionAssign {
				return w.storeAt(s, x)
			}
			w.read(s)
			w.needMut = true
			return w.reStore(s, x)
		}
		if w.live != nil && x.Case == cc.AssignmentExpressionAssign && w.memberOfMe(x.UnaryExpression) {
			// a member stored: the local's value is not read
			s = w.expr(x.AssignmentExpression, s)
			if cc.Node(x) == w.live {
				s.may = true
			}
			return s
		}
		s = w.expr(x.AssignmentExpression, s) // the value first, then the place
		return w.expr(x.UnaryExpression, s)
	case *cc.PostfixExpression:
		if (x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec) && w.isMe(x.PostfixExpression) {
			w.read(s)
			if w.deadInc[x.PostfixExpression] {
				return s // not written: a read
			}
			w.needMut = true
			return w.reStore(s, x)
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			if w.isMe(x.UnaryExpression) {
				w.read(s)
				w.needMut = true
				return w.reStore(s, x)
			}
		case cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType:
			return s
		}
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionLAnd {
			t, f := w.cond(x, s)
			return t.join(f)
		}
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLOr {
			t, f := w.cond(x, s)
			return t.join(f)
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			t, f := w.cond(x.LogicalOrExpression, s)
			return w.expr(x.ExpressionList, t).join(w.expr(x.ConditionalExpression, f))
		}
	}
	walkChildrenFn(n, func(c cc.Node) { s = w.expr(c, s) })
	return s
}

// items walks a block's items; div says it cannot complete.
func (w *defer_) items(cs *cc.CompoundStatement, s dstate) (dstate, bool) {
	live := true
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		it := l.BlockItem
		if it.Case == cc.BlockItemStmt {
			// a label the gotos before it reach: what they left joins
			for st := it.Statement; st != nil && st.Case == cc.StatementLabeled && st.LabeledStatement.Case == cc.LabeledStatementLabel; st = st.LabeledStatement.Statement {
				name := st.LabeledStatement.Token.SrcStr()
				w.joined[st.LabeledStatement] = true
				if g, ok := w.gotos[name]; ok {
					delete(w.gotos, name) // the next time round collects them anew
					if live {
						s = s.join(g)
					} else {
						s, live = g, true
					}
				}
			}
		}
		if !live {
			continue // dead: not written
		}
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
				if id.Declarator == w.d && !w.skipInit {
					s = w.storeAt(s, nil)
				}
			}
		case cc.BlockItemStmt:
			var div bool
			s, div = w.stmt(it.Statement, s)
			if div {
				live = false
			}
		}
	}
	return s, !live
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
		if ls := st.LabeledStatement; ls.Case == cc.LabeledStatementLabel {
			if !w.joined[ls] {
				w.bad = true // a label inside a statement: not followed
			}
			return w.stmt(ls.Statement, s)
		}
		if w.names(st) {
			w.bad = true // a case of a switch: not followed
		}
		return w.stmt(st.LabeledStatement.Statement, s)
	case cc.StatementJump:
		j := st.JumpStatement
		switch {
		case j.Case == cc.JumpStatementGoto:
			name := j.Token2.SrcStr()
			if w.backTo[name] {
				w.bad = true // a goto back: not followed
			}
			if g, ok := w.gotos[name]; ok {
				w.gotos[name] = g.join(s)
			} else {
				w.gotos[name] = s
			}
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
		t, f := w.cond(x.ExpressionList, s)
		s = t.join(f)
		a, adiv := w.stmt(x.Statement, t)
		b, bdiv := f, false
		if x.Case == cc.SelectionStatementIfElse {
			b, bdiv = w.stmt(x.Statement2, f)
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
			body, brk, brkSet := w.loopBody(x.Statement, nil, s)
			if isConstTrue(x.ExpressionList) {
				return forever(s, brk, brkSet)
			}
			// out where the condition is false, or at a break: the body ran
			// once at least
			_, out := w.cond(x.ExpressionList, body)
			if brkSet {
				out = out.join(brk)
			}
			return out, false
		case cc.IterationStatementWhile:
			cond = x.ExpressionList
		case cc.IterationStatementFor:
			s = w.expr(x.ExpressionList, s)
			cond, step = x.ExpressionList2, x.ExpressionList3
		case cc.IterationStatementForDecl:
			s, _ = w.items(&cc.CompoundStatement{BlockItemList: &cc.BlockItemList{BlockItem: &cc.BlockItem{Case: cc.BlockItemDecl, Declaration: x.Declaration}}}, s)
			cond, step = x.ExpressionList, x.ExpressionList2
		}
		t, f := w.cond(cond, s)
		s = t.join(f)
		body, brk, brkSet := w.loopBody(x.Statement, step, t)
		if cond == nil || isConstTrue(cond) && !isConstFalse(cond) {
			return forever(s, brk, brkSet)
		}
		// out where the condition is false, first or after a time round, or
		// at a break
		_, again := w.cond(cond, body)
		out := f.join(again)
		if brkSet {
			out = out.join(brk)
		}
		return out, false
	}
	w.bad = true
	return s, false
}

// loopBody walks a loop's body and step twice -- the second time as the
// first left it, as rustc sees a store the second time round -- and
// returns what the first time left: its end and its continues joined, and
// with may, what its breaks stored too.
func (w *defer_) loopBody(body *cc.Statement, step cc.ExpressionNode, s dstate) (dstate, dstate, bool) {
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
	acc2 := &dloop{}
	w.loops = append(w.loops, acc2)
	again, _ := w.stmt(body, s.join(one))
	w.loops = w.loops[:len(w.loops)-1]
	w.expr(step, again)
	brk, brkSet := acc.brk, acc.brkSet
	if acc2.brkSet {
		if brkSet {
			brk = brk.join(acc2.brk)
		} else {
			brk, brkSet = acc2.brk, true
		}
	}
	if brkSet {
		one.may = one.may || brk.may
	}
	return one, brk, brkSet
}

// dloop is what a loop's breaks and continues left.
type dloop struct {
	brk, cont       dstate
	brkSet, contSet bool
	sw              bool // a switch's: a break's, not a continue's
	once            bool // do { } while (0)'s: a continue leaves it
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

// forever is what a loop with no condition leaves: what its breaks left,
// the only way out -- Rust's `loop`, which completes only by a break.
func forever(s, brk dstate, brkSet bool) (dstate, bool) {
	if !brkSet {
		return s, true
	}
	return brk, false
}

// rootOf says e is the local or a place in it -- a member, an element of
// an array of it -- and whether an array is on the way, which Rust
// reaches through `&raw mut` (decay).
func (w *defer_) rootOf(e cc.ExpressionNode) (me, viaArray bool) {
	for {
		e = unparenE(e)
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent && x.ResolvedTo() == w.d {
				t := w.d.Type()
				return true, viaArray || t != nil && t.Kind() == cc.Array
			}
			return false, false
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionSelect:
				if t := rsPlaceType(x); t != nil && t.Kind() == cc.Array {
					viaArray = true
				}
				e = x.PostfixExpression
				continue
			case cc.PostfixExpressionIndex:
				be := cc.ExpressionNode(x.PostfixExpression)
				if !isPtrish(be.Type()) {
					be = x.ExpressionList
				}
				if t := unparenE(be).Type(); t != nil && t.Kind() == cc.Array {
					viaArray = true
					e = be
					continue
				}
			}
		}
		return false, false
	}
}

// mutUse notes what makes the local's binding `mut` beside a second store:
// a member or an element written, an address taken (but a & reference's),
// an array of it reached (`&raw mut`).
func (w *defer_) mutUse(n cc.Node) {
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		if me, arr := w.rootOf(x); me && arr {
			w.needMut = true
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect, cc.PostfixExpressionIndex:
			if me, arr := w.rootOf(x); me && arr {
				w.needMut = true
			}
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			if me, _ := w.rootOf(x.PostfixExpression); me {
				w.needMut = true
			}
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			if me, _ := w.rootOf(x.UnaryExpression); me {
				w.needMut = true
			}
		case cc.UnaryExpressionAddrof:
			if me, _ := w.rootOf(x.CastExpression); me && !(w.sharedRef != nil && w.sharedRef(x)) {
				w.needMut = true
			}
		}
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond && !w.isMe(x.UnaryExpression) {
			if me, _ := w.rootOf(x.UnaryExpression); me {
				w.needMut = true
			}
		}
	}
}

// deadInits finds the locals whose initializer's value nothing reads --
// every read comes after a store, as the walk above sees it with the
// initializer not a store -- and whose initializer does nothing: it is
// not written, and the local is declared with no value.  rustc counted
// each as a value assigned and never read.
func (f *rfn) deadInits() {
	inits := map[*cc.Declarator]*cc.Initializer{}
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if id, ok := n.(*cc.InitDeclarator); ok && id.Initializer != nil {
			inits[id.Declarator] = id.Initializer
		}
		walkChildrenFn(n, rec)
	}
	rec(f.fd.CompoundStatement)
	for _, l := range f.order {
		in := inits[l.d]
		if l.param || l.d == nil || in == nil || in.Case != cc.InitializerExpr || hasEffect(in.AssignmentExpression) {
			continue
		}
		l.deadInit = true
		if ok, _ := f.deferred(l); !ok {
			l.deadInit = false
		}
	}
	// a parameter whose value is never read: every read comes after a store
	for _, l := range f.order {
		if !l.param || l.d == nil || f.r.refParams[l.d] != nil {
			continue
		}
		if ok, mut := f.deferredFrom(l); ok && l.mutated {
			l.deadParam, l.deadMut = true, mut
		}
	}
}

// cond walks a condition, and returns what it leaves where it is true and
// where it is false: the right of && is walked only where the left is
// true, the right of || where it is false -- as rustc's branches see it.
func (w *defer_) cond(n cc.ExpressionNode, s dstate) (t, f dstate) {
	if n == nil {
		return s, s
	}
	switch x := unparenE(n).(type) {
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionLAnd {
			lt, lf := w.cond(x.LogicalAndExpression, s)
			rt, rf := w.cond(x.InclusiveOrExpression, lt)
			return rt, lf.join(rf)
		}
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLOr {
			lt, lf := w.cond(x.LogicalOrExpression, s)
			rt, rf := w.cond(x.LogicalAndExpression, lf)
			return lt.join(rt), rf
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionNot {
			f, t := w.cond(x.CastExpression, s)
			return t, f
		}
	}
	s = w.expr(n, s)
	return s, s
}

// deadStores finds the stores to a local whose value nothing reads -- an
// expression statement `x = v;`, `x op= v;` or `s.m = v;` of a local
// whose address is never taken, after which every path stores again, or
// ends, before a read -- the liveness walk above, from the function's
// start, with the store under test the only one whose value is followed.
// Such a store is C's own dead code, which rustc counts as a value
// assigned and never read: it is not written, its value's effects alone
// are (exprStatement).
func (f *rfn) deadStores() {
	f.deadIncs()
	var cands []*cc.AssignmentExpression
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if st, ok := n.(*cc.ExpressionStatement); ok && st.ExpressionList != nil {
			if a, ok := unparenE(st.ExpressionList).(*cc.AssignmentExpression); ok && a.Case != cc.AssignmentExpressionCond {
				cands = append(cands, a)
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(f.fd.CompoundStatement)
	for _, a := range cands {
		l := f.storeTarget(a.UnaryExpression)
		if l == nil || isAggr(l.t) && a.Case != cc.AssignmentExpressionAssign {
			continue
		}
		w := &defer_{d: l.d, gotos: map[string]dstate{}, joined: map[*cc.LabeledStatement]bool{}, backTo: rsBackGotos(f.fd),
			dead: f.deadStore, deadInc: f.deadInc, live: a}
		w.items(f.fd.CompoundStatement, dstate{})
		if !w.bad && !w.liveRead && l.addr {
			f.expectDead = true // written: a pointer may read it
			continue
		}
		if !w.bad && !w.liveRead {
			if f.deadStore == nil {
				f.deadStore = map[*cc.AssignmentExpression]bool{}
			}
			f.deadStore[a] = true
		}
	}
	if len(f.deadStore) > 0 || len(f.deadInc) > 0 {
		for _, l := range f.order {
			l.read, l.mutated, l.addr = false, false, false
		}
		f.uses()
	}
}

// storeTarget is the local a store's place is: the local itself, of a
// scalar or a pointer, or a member of a struct local.
func (f *rfn) storeTarget(e cc.ExpressionNode) *rlocal {
	e = unparenE(e)
	if p, ok := e.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		d, _ := p.ResolvedTo().(*cc.Declarator)
		l := f.local[d]
		if l == nil || l.d == nil || isAggr(l.t) || l.t.Kind() == cc.Array {
			return nil
		}
		return l
	}
	for {
		x, ok := e.(*cc.PostfixExpression)
		if !ok || x.Case != cc.PostfixExpressionSelect {
			return nil
		}
		e = unparenE(x.PostfixExpression)
		if p, ok := e.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
			d, _ := p.ResolvedTo().(*cc.Declarator)
			l := f.local[d]
			if l == nil || l.d == nil || !isAggr(l.t) {
				return nil
			}
			return l
		}
	}
}

// deadIncs finds x++ and x-- in a return's value, of a local whose address
// is never taken, named once there: the function returns, so the store is
// dead -- C's `return n++;` -- and the value read is the local's.
func (f *rfn) deadIncs() {
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if j, ok := n.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementReturn && j.ExpressionList != nil {
			count := map[*cc.Declarator]int{}
			var names func(cc.Node)
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
			names(j.ExpressionList)
			var incs func(cc.Node)
			incs = func(n cc.Node) {
				if n == nil {
					return
				}
				if x, ok := n.(*cc.PostfixExpression); ok && (x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec) {
					if p, ok := unparenE(x.PostfixExpression).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
						d, _ := p.ResolvedTo().(*cc.Declarator)
						if l := f.local[d]; l != nil && !l.addr && count[d] == 1 && !isAggr(l.t) && l.t.Kind() != cc.Array {
							if f.deadInc == nil {
								f.deadInc = map[cc.ExpressionNode]bool{}
							}
							f.deadInc[x.PostfixExpression] = true
						}
					}
				}
				walkChildrenFn(n, incs)
			}
			incs(j.ExpressionList)
			return
		}
		walkChildrenFn(n, rec)
	}
	rec(f.fd.CompoundStatement)
}
