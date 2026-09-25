package togo

// lower.go is the lowered form: a C function as basic blocks, for a target
// with no statement that jumps -- no return before the end, no break,
// continue or goto, no switch that falls through -- and no variable that
// changes (Clojure, doc/CLOJURE.md; and Python, Lua or Scheme would share
// it).  Each block is a list of steps and one terminator: a jump, a branch
// on a condition, a switch on a value, or a return.
//
// A step does one thing: it assigns (C's `=`, `op=`, `++`, `--`, each its own
// step), evaluates a call for what it does, or gives a declared local its
// initial value.  What an expression does besides giving its value -- an
// assignment inside it, an increment, the comma's left operands, && and ||
// and ?: whose later operands do something -- is taken apart into steps
// before it, as the Java backend writes them into statements before it,
// and the value is then an expression with nothing left to do but read and
// call: the C node itself, read through the function's substitutions (sub),
// which put a temporary, or the lvalue an assignment stored to, where the
// node that did something was.  Calls stay where C has them, in C's order
// (the Java's): a call is taken apart only where it would be evaluated
// twice -- an lvalue that is read and then written.
//
// The form names nothing in any target: its types are the C's, and
// lowerC prints it back as C (lower_c.go), which is how it is tested.

import (
	"fmt"
	"sort"

	"github.com/arbace/go-whim/crefactor/cc"
)

// lvar is a function's variable: a C local or parameter, or a temporary the
// lowering made.
type lvar struct {
	name    string
	c       cc.Type        // its C type
	decl    *cc.Declarator // the C object it is; nil for a temporary
	param   bool
	temp    bool
	boolean bool   // a temporary holding a truth value (&&, ||)
	key     string // the analysis's key of a C object (declKey)
	pidx    int    // a parameter's position
}

// lexpr is a value in the lowered form: a C expression, read through the
// function's substitutions, or a variable.
type lexpr struct {
	n   cc.ExpressionNode
	v   *lvar
	raw bool // n is printed itself, not its substitution: a snapshot's own step
}

func (e lexpr) isZero() bool { return e.n == nil && e.v == nil }

// lop is what a step does.
type lop int

const (
	opSet      lop = iota // dst = e: a temporary's value (or a truth value: dst.boolean)
	opAssign              // lhs = e: C's assignment, a struct's a copy
	opAssignOp            // lhs op= e
	opIncDec              // lhs++ or lhs-- (inc says which), its value unused
	opEval                // e, a call, for what it does
	opInit                // dst's initializer, where C declares it: in, or a string for a char array
)

// lstep is one step of a block.
type lstep struct {
	op   lop
	dst  *lvar
	lhs  cc.ExpressionNode // an lvalue, read through the substitutions
	e    lexpr
	aop  string // opAssignOp's operator: + - * / % << >> & ^ |
	inc  bool
	in   *cc.Initializer
	at   cc.Node
	cdst string // opInit of a compound literal: its C type, for the C printer
}

// tkind is how a block ends.
type tkind int

const (
	tGoto   tkind = iota // to[0]
	tIf                  // cond ? to[0] : to[1]
	tSwitch              // on cond: cases[i] -> to[i], the last of to the default
	tRet                 // return ret (none: ret is zero)
	tFall                // the end of a function that has a result, reached: C's undefined value
)

type lterm struct {
	kind  tkind
	cond  lexpr
	to    []*lblock
	cases [][]int64 // tSwitch: the values that go to to[i]
	ret   lexpr
	at    cc.Node
}

// lblock is a basic block.
type lblock struct {
	id    int
	steps []lstep
	term  lterm
	preds []*lblock
	label string // a C label, or what made the block: for reading
	ended bool   // its terminator is set
}

func (b *lblock) succs() []*lblock { return b.term.to }

// lfn is one function lowered.
type lfn struct {
	fd     *cc.FunctionDefinition
	name   string
	ft     *cc.FunctionType
	params []*lvar
	vars   []*lvar // every variable: the parameters, the locals, the temporaries
	byDecl map[*cc.Declarator]*lvar
	sub    map[cc.ExpressionNode]lexpr
	blocks []*lblock // entry first; after lowering, the reachable ones in reverse postorder
	// the complit temporaries and the steps that make them
	statics []*cc.Declarator // block-scope statics, for the C printer

	// while lowering
	cur    *lblock
	brk    []*lblock
	cont   []*lblock
	sw     []*lswitch
	labels map[string]*lblock
	taken  map[string]bool
	ntemp  int
	p      *profile
	a      *an
	nblk   int
	rename func(string) string // a C name as the target writes it
}

type lswitch struct {
	b     *lblock // the block that switches
	kind  jk      // the promoted type of the value
	cases map[int64]*lblock
	order []int64
	def   *lblock
}

// lowerFunction lowers one function definition; a construct it has no rule
// for panics with unsupported, as the emitters do.
func lowerFunction(fd *cc.FunctionDefinition, a *an, p *profile, rename func(string) string) *lfn {
	d := fd.Declarator
	ft, _ := d.Type().(*cc.FunctionType)
	if rename == nil {
		rename = func(s string) string { return s }
	}
	f := &lfn{fd: fd, name: d.Name(), ft: ft, byDecl: map[*cc.Declarator]*lvar{}, sub: map[cc.ExpressionNode]lexpr{},
		labels: map[string]*lblock{}, taken: map[string]bool{}, p: p, a: a, rename: rename}
	if ft == nil {
		f.no(d, "a function of no function type")
	}
	if ft.IsVariadic() {
		f.no(d, "a variadic function")
	}
	// every name the body refers to that is not its own is taken
	var names func(cc.Node)
	names = func(n cc.Node) {
		if n == nil {
			return
		}
		if x, ok := n.(*cc.PrimaryExpression); ok && x.Case == cc.PrimaryExpressionIdent {
			switch dd := x.ResolvedTo().(type) {
			case *cc.Declarator:
				if !dd.IsParam() && dd.StorageDuration() != cc.Automatic {
					f.taken[rename(x.Token.SrcStr())] = true
				}
			case *cc.Enumerator:
				f.taken[rename(x.Token.SrcStr())] = true
			}
		}
		walkChildrenFn(n, names)
	}
	names(fd.CompoundStatement)
	for i, pr := range ft.Parameters() {
		if pr.Type() == nil || pr.Type().Kind() == cc.Void {
			continue
		}
		t := pr.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		pn := pr.Name()
		if pn == "" {
			pn = fmt.Sprintf("p%d", i)
		}
		v := &lvar{name: f.unique(rename(pn)), c: t, decl: pr.Declarator, param: true, pidx: i,
			key: fmt.Sprintf("param:%s:%d", d.Name(), i)}
		f.params = append(f.params, v)
		f.vars = append(f.vars, v)
		if pr.Declarator != nil {
			f.byDecl[pr.Declarator] = v
		}
	}
	f.cur = f.newBlock("entry")
	f.items(fd.CompoundStatement)
	if !f.cur.ended {
		if ft.Result() != nil && ft.Result().Kind() != cc.Void {
			f.end(lterm{kind: tFall})
		} else {
			f.end(lterm{kind: tRet})
		}
	}
	f.finish()
	return f
}

func (f *lfn) no(n cc.Node, format string, args ...any) {
	where := ""
	if n != nil {
		where = fmt.Sprintf(" at %d", n.Position().Line)
	}
	panic(unsupported{fmt.Sprintf(format, args...) + where})
}

func (f *lfn) unique(base string) string {
	name := base
	for i := 2; f.taken[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	f.taken[name] = true
	return name
}

func (f *lfn) newBlock(label string) *lblock {
	b := &lblock{id: f.nblk, label: label}
	f.nblk++
	f.blocks = append(f.blocks, b)
	return b
}

func (f *lfn) newTemp(t cc.Type) *lvar {
	for {
		f.ntemp++
		n := fmt.Sprintf("t%d", f.ntemp)
		if !f.taken[n] {
			f.taken[n] = true
			v := &lvar{name: n, c: t, temp: true}
			f.vars = append(f.vars, v)
			return v
		}
	}
}

// step adds a step to the current block.
func (f *lfn) step(s lstep) {
	f.cur.steps = append(f.cur.steps, s)
}

// end sets the current block's terminator and starts a block no jump
// reaches yet: what follows a jump in C is dead until a label.
func (f *lfn) end(t lterm) {
	f.cur.term = t
	f.cur.ended = true
	f.cur = f.newBlock("dead")
}

// jump ends the current block with a jump to b.
func (f *lfn) jump(b *lblock) { f.end(lterm{kind: tGoto, to: []*lblock{b}}) }

// branch ends the current block with a branch on c.
func (f *lfn) branch(c lexpr, t, e *lblock, at cc.Node) {
	f.end(lterm{kind: tIf, cond: c, to: []*lblock{t, e}, at: at})
}

// enter makes b the current block, the current one jumping to it.
func (f *lfn) enter(b *lblock) {
	if !f.cur.ended {
		f.jump(b)
	}
	f.cur = b
}

// declareLocal makes a C local's variable.
func (f *lfn) declareLocal(d *cc.Declarator) *lvar {
	if v, ok := f.byDecl[d]; ok {
		return v
	}
	v := &lvar{name: f.unique(f.rename(d.Name())), c: d.Type(), decl: d, key: f.a.declKey(d)}
	f.vars = append(f.vars, v)
	f.byDecl[d] = v
	return v
}

// --- statements -------------------------------------------------------------

func (f *lfn) items(cs *cc.CompoundStatement) {
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		it := l.BlockItem
		switch it.Case {
		case cc.BlockItemDecl:
			f.declaration(it.Declaration)
		case cc.BlockItemStmt:
			f.stmt(it.Statement)
		default:
			f.no(it, "a block item %v", it.Case)
		}
	}
}

func (f *lfn) declaration(d *cc.Declaration) {
	if d.Case != cc.DeclarationDecl {
		return
	}
	for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		id := l.InitDeclarator
		dd := id.Declarator
		t := dd.Type()
		if dd.IsTypename() || t.Kind() == cc.Function || dd.IsExtern() {
			continue
		}
		if dd.StorageDuration() == cc.Static {
			if dd.Name() == "__func__" {
				continue
			}
			f.statics = append(f.statics, dd) // hoisted: initialized with the file's objects
			continue
		}
		v := f.declareLocal(dd)
		if id.Initializer != nil {
			f.initStep(v, id.Initializer)
		}
	}
}

// initStep gives v its initializer's value where C declares it: an
// expression is an assignment; a braced list or a string a step of its own,
// which makes the object anew.
func (f *lfn) initStep(v *lvar, in *cc.Initializer) {
	if in.Case == cc.InitializerExpr && v.c.Kind() != cc.Array {
		e := in.AssignmentExpression
		f.step(lstep{op: opSet, dst: v, e: f.val(e), at: in})
		return
	}
	f.lowerInit(in)
	f.step(lstep{op: opInit, dst: v, in: in, at: in})
}

// lowerInit takes apart what an initializer's expressions do, before it.
func (f *lfn) lowerInit(in *cc.Initializer) {
	if in == nil {
		return
	}
	if in.Case == cc.InitializerExpr {
		f.child(in.AssignmentExpression)
		return
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		f.lowerInit(l.Initializer)
	}
}

func (f *lfn) stmt(s *cc.Statement) {
	if s == nil {
		return
	}
	switch s.Case {
	case cc.StatementCompound:
		f.items(s.CompoundStatement)
	case cc.StatementExpr:
		if s.ExpressionStatement.ExpressionList != nil {
			f.effect(s.ExpressionStatement.ExpressionList)
		}
	case cc.StatementSelection:
		f.selection(s.SelectionStatement)
	case cc.StatementIteration:
		f.iteration(s.IterationStatement)
	case cc.StatementJump:
		f.jumpStmt(s.JumpStatement)
	case cc.StatementLabeled:
		f.labeled(s.LabeledStatement)
	case cc.StatementAsm:
		f.no(s, "asm")
	default:
		f.no(s, "a statement %v", s.Case)
	}
}

func (f *lfn) labelBlock(name string) *lblock {
	b, ok := f.labels[name]
	if !ok {
		b = f.newBlock(name)
		f.labels[name] = b
	}
	return b
}

func (f *lfn) labeled(l *cc.LabeledStatement) {
	switch l.Case {
	case cc.LabeledStatementLabel:
		f.enter(f.labelBlock(l.Token.SrcStr()))
	case cc.LabeledStatementCaseLabel, cc.LabeledStatementDefault:
		if len(f.sw) == 0 {
			f.no(l, "a case label outside a switch")
		}
		sw := f.sw[len(f.sw)-1]
		b := f.newBlock("case")
		f.enter(b)
		if l.Case == cc.LabeledStatementDefault {
			sw.def = b
		} else {
			v, ok := intValue(l.ConstantExpression.Value())
			if !ok {
				f.no(l, "a case label of no integer value")
			}
			v = truncK(v, sw.kind)
			if _, dup := sw.cases[v]; !dup {
				sw.cases[v] = b
				sw.order = append(sw.order, v)
			}
		}
	default:
		f.no(l, "a case range")
	}
	f.stmt(l.Statement)
}

// intValue is a C constant's value as an int64.
func intValue(v cc.Value) (int64, bool) {
	switch x := v.(type) {
	case cc.Int64Value:
		return int64(x), true
	case cc.UInt64Value:
		return int64(x), true
	}
	return 0, false
}

// truncK is v converted to kind k: its bits, sign- or zero-extended.
func truncK(v int64, k jk) int64 {
	switch {
	case k.boolean:
		if v != 0 {
			return 1
		}
		return 0
	case k.size == 1 && k.signed:
		return int64(int8(v))
	case k.size == 1:
		return int64(uint8(v))
	case k.size == 2 && k.signed:
		return int64(int16(v))
	case k.size == 2:
		return int64(uint16(v))
	case k.size == 4 && k.signed:
		return int64(int32(v))
	case k.size == 4:
		return int64(uint32(v))
	}
	return v
}

func (f *lfn) selection(s *cc.SelectionStatement) {
	switch s.Case {
	case cc.SelectionStatementIf, cc.SelectionStatementIfElse:
		c := f.val(s.ExpressionList)
		then, join := f.newBlock("then"), f.newBlock("endif")
		els := join
		if s.Case == cc.SelectionStatementIfElse {
			els = f.newBlock("else")
		}
		f.branch(c, then, els, s)
		f.cur = then
		f.stmt(s.Statement)
		if s.Case == cc.SelectionStatementIfElse {
			if !f.cur.ended {
				f.jump(join)
			}
			f.cur = els
			f.stmt(s.Statement2)
		}
		f.enter(join)
	case cc.SelectionStatementSwitch:
		ck, ok := scalarKind(s.ExpressionList.Type())
		if !ok {
			f.no(s, "a switch on a %s", s.ExpressionList.Type())
		}
		v := f.val(s.ExpressionList)
		sw := &lswitch{b: f.cur, kind: promote(ck), cases: map[int64]*lblock{}}
		join := f.newBlock("endswitch")
		f.cur.term = lterm{kind: tSwitch, cond: v, at: s}
		f.cur.ended = true
		f.cur = f.newBlock("dead")
		f.sw = append(f.sw, sw)
		f.brk = append(f.brk, join)
		f.stmt(s.Statement)
		f.sw = f.sw[:len(f.sw)-1]
		f.brk = f.brk[:len(f.brk)-1]
		f.enter(join)
		def := sw.def
		if def == nil {
			def = join
		}
		// the cases in the order C lists them, each value once
		var to []*lblock
		var cases [][]int64
		idx := map[*lblock]int{}
		for _, cv := range sw.order {
			b := sw.cases[cv]
			if b == def {
				continue // the default's block: the default takes it
			}
			i, ok := idx[b]
			if !ok {
				i = len(to)
				idx[b] = i
				to = append(to, b)
				cases = append(cases, nil)
			}
			cases[i] = append(cases[i], cv)
		}
		sw.b.term.to = append(to, def)
		sw.b.term.cases = cases
	}
}

func (f *lfn) loopBody(s *cc.Statement, brk, cont *lblock) {
	f.brk = append(f.brk, brk)
	f.cont = append(f.cont, cont)
	f.stmt(s)
	f.brk = f.brk[:len(f.brk)-1]
	f.cont = f.cont[:len(f.cont)-1]
}

func (f *lfn) iteration(s *cc.IterationStatement) {
	switch s.Case {
	case cc.IterationStatementWhile:
		head, body, exit := f.newBlock("while"), f.newBlock("body"), f.newBlock("endwhile")
		f.enter(head)
		f.cond(s.ExpressionList, body, exit, s)
		f.cur = body
		f.loopBody(s.Statement, exit, head)
		f.enter(head)
		f.cur = exit
	case cc.IterationStatementDo:
		body, test, exit := f.newBlock("do"), f.newBlock("dowhile"), f.newBlock("enddo")
		f.enter(body)
		f.loopBody(s.Statement, exit, test)
		f.enter(test)
		f.cond(s.ExpressionList, body, exit, s)
		f.cur = exit
	case cc.IterationStatementFor, cc.IterationStatementForDecl:
		cond, post := s.ExpressionList2, s.ExpressionList3
		if s.Case == cc.IterationStatementForDecl {
			f.declaration(s.Declaration)
			cond, post = s.ExpressionList, s.ExpressionList2
		} else if s.ExpressionList != nil {
			f.effect(s.ExpressionList)
		}
		head, body, next, exit := f.newBlock("for"), f.newBlock("body"), f.newBlock("next"), f.newBlock("endfor")
		f.enter(head)
		if cond == nil {
			f.jump(body)
		} else {
			f.cond(cond, body, exit, s)
		}
		f.cur = body
		f.loopBody(s.Statement, exit, next)
		f.enter(next)
		if post != nil {
			f.effect(post)
		}
		f.enter(head)
		f.cur = exit
	default:
		f.no(s, "a loop %v", s.Case)
	}
}

// cond ends the current block with a branch on e to t or e: a constant's
// only way is a jump.
func (f *lfn) cond(e cc.ExpressionNode, t, el *lblock, at cc.Node) {
	switch {
	case isConstTrue(e):
		f.jump(t)
	case isConstFalse(e):
		f.jump(el)
	default:
		f.branch(f.val(e), t, el, at)
	}
}

func (f *lfn) jumpStmt(j *cc.JumpStatement) {
	switch j.Case {
	case cc.JumpStatementGoto:
		f.jump(f.labelBlock(j.Token2.SrcStr()))
	case cc.JumpStatementGotoExpr:
		f.no(j, "a computed goto")
	case cc.JumpStatementBreak:
		if len(f.brk) == 0 {
			f.no(j, "a break outside a loop or switch")
		}
		f.jump(f.brk[len(f.brk)-1])
	case cc.JumpStatementContinue:
		if len(f.cont) == 0 {
			f.no(j, "a continue outside a loop")
		}
		f.jump(f.cont[len(f.cont)-1])
	case cc.JumpStatementReturn:
		rt := f.ft.Result()
		if j.ExpressionList == nil {
			if rt != nil && rt.Kind() != cc.Void {
				f.end(lterm{kind: tFall, at: j})
				return
			}
			f.end(lterm{kind: tRet, at: j})
			return
		}
		if rt == nil || rt.Kind() == cc.Void {
			f.effect(j.ExpressionList)
			f.end(lterm{kind: tRet, at: j})
			return
		}
		f.end(lterm{kind: tRet, ret: f.val(j.ExpressionList), at: j})
	default:
		f.no(j, "a jump %v", j.Case)
	}
}

// --- expressions -------------------------------------------------------------

// hoisted says evaluating e does something that the lowering takes apart
// into steps: an assignment, an increment or decrement, a comma, a compound
// literal (storage made where C evaluates it).  A call is not: it stays
// where C has it.
func hoisted(e cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		switch x := n.(type) {
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec || x.Case == cc.PostfixExpressionComplit {
				found = true
				return
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
				found = true
				return
			}
			if x.Case == cc.UnaryExpressionSizeofExpr || x.Case == cc.UnaryExpressionSizeofType ||
				x.Case == cc.UnaryExpressionAlignofExpr || x.Case == cc.UnaryExpressionAlignofType {
				return // not evaluated
			}
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				found = true
				return
			}
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				found = true
				return
			}
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionStmt {
				found = true
				return
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(e)
	return found
}

// setSub records that node n's value is r.
func (f *lfn) setSub(n cc.ExpressionNode, r lexpr) {
	if r.v == nil && r.n == n {
		return
	}
	f.sub[n] = r
}

// child lowers a subexpression that stays in its parent: what it does is
// taken apart, and its value, when it is no longer the node itself, is the
// node's substitution.
func (f *lfn) child(e cc.ExpressionNode) {
	if e == nil {
		return
	}
	f.setSub(e, f.val(e))
}

// val lowers e for its value.
func (f *lfn) val(e cc.ExpressionNode) lexpr {
	if !hoisted(e) {
		return lexpr{n: e}
	}
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionExpr:
			r := f.val(x.ExpressionList)
			return r
		case cc.PrimaryExpressionStmt:
			f.no(x, "a statement expression")
		}
	case *cc.ConstantExpression:
		return f.val(x.ConditionalExpression)
	case *cc.ExpressionList:
		for x.ExpressionList != nil {
			f.effect(x.AssignmentExpression)
			x = x.ExpressionList
		}
		return f.val(x.AssignmentExpression)
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			f.assign(x)
			return lexpr{n: x.UnaryExpression} // C's value: the lvalue, read back
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			f.lvalue(x.PostfixExpression)
			t := f.newTemp(x.PostfixExpression.Type())
			f.step(lstep{op: opSet, dst: t, e: lexpr{n: x.PostfixExpression, raw: true}, at: x})
			f.step(lstep{op: opIncDec, lhs: x.PostfixExpression, inc: x.Case == cc.PostfixExpressionInc, at: x})
			return lexpr{v: t}
		case cc.PostfixExpressionComplit:
			t := f.newTemp(x.TypeName.Type())
			in := &cc.Initializer{Case: cc.InitializerInitList, InitializerList: x.InitializerList, Token: x.Token}
			f.lowerInit(in)
			f.step(lstep{op: opInit, dst: t, in: in, at: x})
			return lexpr{v: t}
		case cc.PostfixExpressionCall:
			f.child(x.PostfixExpression)
			for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
				f.child(l.AssignmentExpression)
			}
			return lexpr{n: e}
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			f.lvalue(x.UnaryExpression)
			f.step(lstep{op: opIncDec, lhs: x.UnaryExpression, inc: x.Case == cc.UnaryExpressionInc, at: x})
			return lexpr{n: x.UnaryExpression}
		}
	case *cc.LogicalAndExpression:
		return f.logical(e, true, x.LogicalAndExpression, x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		return f.logical(e, false, x.LogicalOrExpression, x.LogicalAndExpression)
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return f.ternary(x)
		}
	}
	// anything else: its operands, in C's order, and the node itself
	for _, c := range operands(e) {
		f.child(c)
	}
	return lexpr{n: e}
}

// operands are the subexpressions an expression evaluates, left to right:
// Java's order, which the emitters keep.  sizeof's operand is not evaluated.
func operands(e cc.ExpressionNode) []cc.ExpressionNode {
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionExpr {
			return []cc.ExpressionNode{x.ExpressionList}
		}
	case *cc.ConstantExpression:
		return []cc.ExpressionNode{x.ConditionalExpression}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			return []cc.ExpressionNode{x.PostfixExpression, x.ExpressionList}
		case cc.PostfixExpressionCall:
			r := []cc.ExpressionNode{x.PostfixExpression}
			for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
				r = append(r, l.AssignmentExpression)
			}
			return r
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect, cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			return []cc.ExpressionNode{x.PostfixExpression}
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionAddrof, cc.UnaryExpressionDeref, cc.UnaryExpressionPlus, cc.UnaryExpressionMinus,
			cc.UnaryExpressionCpl, cc.UnaryExpressionNot:
			return []cc.ExpressionNode{x.CastExpression}
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			return []cc.ExpressionNode{x.UnaryExpression}
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return []cc.ExpressionNode{x.CastExpression}
		}
	case *cc.MultiplicativeExpression:
		return []cc.ExpressionNode{x.MultiplicativeExpression, x.CastExpression}
	case *cc.AdditiveExpression:
		return []cc.ExpressionNode{x.AdditiveExpression, x.MultiplicativeExpression}
	case *cc.ShiftExpression:
		return []cc.ExpressionNode{x.ShiftExpression, x.AdditiveExpression}
	case *cc.RelationalExpression:
		return []cc.ExpressionNode{x.RelationalExpression, x.ShiftExpression}
	case *cc.EqualityExpression:
		return []cc.ExpressionNode{x.EqualityExpression, x.RelationalExpression}
	case *cc.AndExpression:
		return []cc.ExpressionNode{x.AndExpression, x.EqualityExpression}
	case *cc.ExclusiveOrExpression:
		return []cc.ExpressionNode{x.ExclusiveOrExpression, x.AndExpression}
	case *cc.InclusiveOrExpression:
		return []cc.ExpressionNode{x.InclusiveOrExpression, x.ExclusiveOrExpression}
	case *cc.LogicalAndExpression:
		return []cc.ExpressionNode{x.LogicalAndExpression, x.InclusiveOrExpression}
	case *cc.LogicalOrExpression:
		return []cc.ExpressionNode{x.LogicalOrExpression, x.LogicalAndExpression}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return []cc.ExpressionNode{x.LogicalOrExpression, x.ExpressionList, x.ConditionalExpression}
		}
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			return []cc.ExpressionNode{x.UnaryExpression, x.AssignmentExpression}
		}
	case *cc.ExpressionList:
		var r []cc.ExpressionNode
		for ; x != nil; x = x.ExpressionList {
			r = append(r, x.AssignmentExpression)
		}
		return r
	}
	return nil
}

// logical is && (and) or || whose right operand does something: a truth
// value in a temporary, the right side's steps only where C evaluates them.
func (f *lfn) logical(e cc.ExpressionNode, and bool, le, re cc.ExpressionNode) lexpr {
	if !hoisted(re) {
		f.child(le)
		return lexpr{n: e}
	}
	t := f.newTemp(e.Type())
	t.boolean = true
	f.step(lstep{op: opSet, dst: t, e: f.val(le), at: e})
	right, join := f.newBlock("rhs"), f.newBlock("endlogic")
	if and {
		f.branch(lexpr{v: t}, right, join, e)
	} else {
		f.branch(lexpr{v: t}, join, right, e)
	}
	f.cur = right
	f.step(lstep{op: opSet, dst: t, e: f.val(re), at: e})
	f.enter(join)
	return lexpr{v: t}
}

// ternary is ?: whose arms do something: the chosen arm's value in a
// temporary.
func (f *lfn) ternary(x *cc.ConditionalExpression) lexpr {
	if x.ExpressionList == nil {
		f.no(x, "a ?: with no middle operand")
	}
	if !hoisted(x.ExpressionList) && !hoisted(x.ConditionalExpression) {
		f.child(x.LogicalOrExpression)
		return lexpr{n: x}
	}
	if x.Type() == nil || x.Type().Kind() == cc.Void {
		f.no(x, "a ?: of no value")
	}
	c := f.val(x.LogicalOrExpression)
	t := f.newTemp(x.Type())
	a, b, join := f.newBlock("then"), f.newBlock("else"), f.newBlock("endcond")
	f.branch(c, a, b, x)
	f.cur = a
	f.step(lstep{op: opSet, dst: t, e: f.val(x.ExpressionList), at: x})
	f.jump(join)
	f.cur = b
	f.step(lstep{op: opSet, dst: t, e: f.val(x.ConditionalExpression), at: x})
	f.enter(join)
	return lexpr{v: t}
}

// impure says e calls something or does something else: it may not be
// evaluated twice.
func impure(e cc.Node) bool { return hasEffect(e) }

// snapshot is e, which may call, evaluated once into a temporary; e is then
// that temporary wherever it is read.
func (f *lfn) snapshot(e cc.ExpressionNode) {
	if e == nil || !impure(e) {
		return
	}
	r := f.val(e)
	if r.v != nil {
		f.setSub(e, r)
		return
	}
	t := f.newTemp(e.Type())
	r.raw = r.n == e
	f.step(lstep{op: opSet, dst: t, e: r, at: e})
	f.setSub(e, lexpr{v: t})
}

// lvalue lowers what an lvalue's address is made of, so that it can be read
// and written with nothing evaluated twice: a pointer, an index or a base
// that calls is a temporary.
func (f *lfn) lvalue(e cc.ExpressionNode) {
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			return
		case cc.PrimaryExpressionExpr:
			f.lvalue(x.ExpressionList)
			return
		}
	case *cc.ExpressionList:
		if x.ExpressionList == nil {
			f.lvalue(x.AssignmentExpression)
			return
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect:
			f.lvalue(x.PostfixExpression)
			return
		case cc.PostfixExpressionPSelect:
			f.snapshot(x.PostfixExpression)
			return
		case cc.PostfixExpressionIndex:
			if isPtrish(x.PostfixExpression.Type()) && x.PostfixExpression.Type().Kind() == cc.Array {
				f.lvalue(x.PostfixExpression)
			} else {
				f.snapshot(x.PostfixExpression)
			}
			f.snapshot(x.ExpressionList)
			return
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			f.snapshot(x.CastExpression)
			return
		}
	}
	f.no(e, "an assignment to %T", e)
}

// assign lowers an assignment as a step.
func (f *lfn) assign(x *cc.AssignmentExpression) {
	lhs := x.UnaryExpression
	f.lvalue(lhs)
	if x.Case == cc.AssignmentExpressionAssign {
		f.step(lstep{op: opAssign, lhs: lhs, e: f.val(x.AssignmentExpression), at: x})
		return
	}
	ops := map[cc.AssignmentExpressionCase]string{cc.AssignmentExpressionMul: "*", cc.AssignmentExpressionDiv: "/", cc.AssignmentExpressionMod: "%",
		cc.AssignmentExpressionAdd: "+", cc.AssignmentExpressionSub: "-", cc.AssignmentExpressionLsh: "<<", cc.AssignmentExpressionRsh: ">>",
		cc.AssignmentExpressionAnd: "&", cc.AssignmentExpressionXor: "^", cc.AssignmentExpressionOr: "|"}
	op, ok := ops[x.Case]
	if !ok {
		f.no(x, "an assignment %v", x.Case)
	}
	r := f.val(x.AssignmentExpression)
	if r.v == nil && impure(r.n) {
		// the right side's calls first, and then the lvalue read: gcc's
		// order, which Java's compound assignment does not keep (it reads
		// the lvalue first)
		t := f.newTemp(x.AssignmentExpression.Type())
		r.raw = true
		f.step(lstep{op: opSet, dst: t, e: r, at: x})
		r = lexpr{v: t}
	}
	f.step(lstep{op: opAssignOp, lhs: lhs, aop: op, e: r, at: x})
}

// effect lowers e for what it does; its value is not wanted.
func (f *lfn) effect(e cc.ExpressionNode) {
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionExpr {
			f.effect(x.ExpressionList)
			return
		}
	case *cc.ExpressionList:
		for ; x != nil; x = x.ExpressionList {
			f.effect(x.AssignmentExpression)
		}
		return
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			f.assign(x)
			return
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			f.lvalue(x.PostfixExpression)
			f.step(lstep{op: opIncDec, lhs: x.PostfixExpression, inc: x.Case == cc.PostfixExpressionInc, at: x})
			return
		case cc.PostfixExpressionCall:
			r := f.val(e)
			f.step(lstep{op: opEval, e: r, at: e})
			return
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
			f.lvalue(x.UnaryExpression)
			f.step(lstep{op: opIncDec, lhs: x.UnaryExpression, inc: x.Case == cc.UnaryExpressionInc, at: x})
			return
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast && x.Type().Kind() == cc.Void {
			f.effect(x.CastExpression)
			return
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond && (impure(x.ExpressionList) || impure(x.ConditionalExpression)) {
			c := f.val(x.LogicalOrExpression)
			a, b, join := f.newBlock("then"), f.newBlock("else"), f.newBlock("endcond")
			f.branch(c, a, b, x)
			f.cur = a
			f.effect(x.ExpressionList)
			f.jump(join)
			f.cur = b
			f.effect(x.ConditionalExpression)
			f.enter(join)
			return
		}
	case *cc.LogicalAndExpression:
		if impure(x.InclusiveOrExpression) {
			f.logicalEffect(true, x.LogicalAndExpression, x.InclusiveOrExpression)
			return
		}
	case *cc.LogicalOrExpression:
		if impure(x.LogicalAndExpression) {
			f.logicalEffect(false, x.LogicalOrExpression, x.LogicalAndExpression)
			return
		}
	}
	r := f.val(e)
	if r.v == nil && impure(r.n) {
		// a value that calls, computed for what the calls do
		f.step(lstep{op: opEval, e: r, at: e})
	}
}

// logicalEffect is `a && b;` or `a || b;`: b's effect where C evaluates it.
func (f *lfn) logicalEffect(and bool, le, re cc.ExpressionNode) {
	c := f.val(le)
	right, join := f.newBlock("rhs"), f.newBlock("endlogic")
	if and {
		f.branch(c, right, join, le)
	} else {
		f.branch(c, join, right, le)
	}
	f.cur = right
	f.effect(re)
	f.enter(join)
}

// --- the graph -----------------------------------------------------------

// finish drops the blocks nothing reaches, threads jumps through empty
// blocks, and orders the rest in reverse postorder with their
// predecessors.
func (f *lfn) finish() {
	// an empty block that only jumps is its target
	target := func(b *lblock) *lblock {
		seen := map[*lblock]bool{}
		for len(b.steps) == 0 && b.term.kind == tGoto && !seen[b] {
			seen[b] = true
			b = b.term.to[0]
		}
		return b
	}
	for _, b := range f.blocks {
		for i, s := range b.term.to {
			b.term.to[i] = target(s)
		}
	}
	entry := target(f.blocks[0])
	// reverse postorder from the entry
	var order []*lblock
	seen := map[*lblock]bool{}
	var dfs func(b *lblock)
	dfs = func(b *lblock) {
		seen[b] = true
		for _, s := range b.term.to {
			if !seen[s] {
				dfs(s)
			}
		}
		order = append(order, b)
	}
	dfs(entry)
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	for i, b := range order {
		b.id = i
		b.preds = nil
	}
	for _, b := range order {
		for _, s := range b.term.to {
			s.preds = append(s.preds, b)
		}
	}
	f.blocks = order
}

// liveVars is, per block, the variables live at its start, and the
// variables each block assigns: what a jump between blocks must carry.
func (f *lfn) usesDefs() (map[*lblock]map[*lvar]bool, map[*lblock]map[*lvar]bool) {
	uses := map[*lblock]map[*lvar]bool{}
	defs := map[*lblock]map[*lvar]bool{}
	for _, b := range f.blocks {
		u, d := map[*lvar]bool{}, map[*lvar]bool{}
		use := func(v *lvar) {
			if !d[v] {
				u[v] = true
			}
		}
		for _, s := range b.steps {
			for _, v := range f.stepReads(s) {
				use(v)
			}
			for _, v := range f.stepWrites(s) {
				d[v] = true
			}
		}
		for _, v := range f.termReads(b.term) {
			use(v)
		}
		uses[b], defs[b] = u, d
	}
	return uses, defs
}

// live is, per block, the variables live at its start.
func (f *lfn) live() map[*lblock]map[*lvar]bool {
	uses, defs := f.usesDefs()
	in := map[*lblock]map[*lvar]bool{}
	for _, b := range f.blocks {
		in[b] = map[*lvar]bool{}
	}
	for changed := true; changed; {
		changed = false
		for i := len(f.blocks) - 1; i >= 0; i-- {
			b := f.blocks[i]
			out := map[*lvar]bool{}
			for _, s := range b.term.to {
				for v := range in[s] {
					out[v] = true
				}
			}
			for v := range uses[b] {
				if !in[b][v] {
					in[b][v] = true
					changed = true
				}
			}
			for v := range out {
				if !defs[b][v] && !in[b][v] {
					in[b][v] = true
					changed = true
				}
			}
		}
	}
	return in
}

// exprVars are the variables an expression reads: its temporaries and the
// locals it names, through the substitutions.
func (f *lfn) exprVars(e lexpr) []*lvar {
	if e.v != nil {
		return []*lvar{e.v}
	}
	var vs []*lvar
	top := e.n
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if x, ok := n.(cc.ExpressionNode); ok && !(e.raw && n == top) {
			if r, ok := f.sub[x]; ok {
				vs = append(vs, f.exprVars(r)...)
				return
			}
		}
		if x, ok := n.(*cc.PrimaryExpression); ok && x.Case == cc.PrimaryExpressionIdent {
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
				if v, ok := f.byDecl[d]; ok {
					vs = append(vs, v)
				}
			}
			return
		}
		walkChildrenFn(n, rec)
	}
	rec(e.n)
	return vs
}

// lhsVar is the variable an lvalue that is a whole variable names, or nil.
func (f *lfn) lhsVar(e cc.ExpressionNode) *lvar {
	if r, ok := f.sub[e]; ok {
		if r.v != nil {
			return r.v
		}
		e = r.n
	}
	if p, ok := unparenE(e).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if d, ok := p.ResolvedTo().(*cc.Declarator); ok {
			return f.byDecl[d]
		}
	}
	return nil
}

// stepReads are the variables a step reads.
func (f *lfn) stepReads(s lstep) []*lvar {
	var vs []*lvar
	if !s.e.isZero() {
		vs = append(vs, f.exprVars(s.e)...)
	}
	if s.lhs != nil {
		// an lvalue reads what its address is made of -- and a whole
		// variable, when the step reads it before it writes it
		lv := f.lhsVar(s.lhs)
		if lv == nil || s.op == opAssignOp || s.op == opIncDec {
			vs = append(vs, f.exprVars(lexpr{n: s.lhs})...)
		}
	}
	if s.in != nil {
		var rec func(in *cc.Initializer)
		rec = func(in *cc.Initializer) {
			if in.Case == cc.InitializerExpr {
				vs = append(vs, f.exprVars(lexpr{n: in.AssignmentExpression})...)
				return
			}
			for l := in.InitializerList; l != nil; l = l.InitializerList {
				rec(l.Initializer)
			}
		}
		rec(s.in)
	}
	return vs
}

// stepWrites are the variables a step gives a new value.
func (f *lfn) stepWrites(s lstep) []*lvar {
	switch s.op {
	case opSet, opInit:
		return []*lvar{s.dst}
	case opAssign, opAssignOp, opIncDec:
		if v := f.lhsVar(s.lhs); v != nil {
			return []*lvar{v}
		}
	}
	return nil
}

func (f *lfn) termReads(t lterm) []*lvar {
	var vs []*lvar
	if !t.cond.isZero() {
		vs = append(vs, f.exprVars(t.cond)...)
	}
	if !t.ret.isZero() {
		vs = append(vs, f.exprVars(t.ret)...)
	}
	return vs
}

// sortedVars are a set of variables in the function's order.
func (f *lfn) sortedVars(set map[*lvar]bool) []*lvar {
	idx := map[*lvar]int{}
	for i, v := range f.vars {
		idx[v] = i
	}
	var vs []*lvar
	for v := range set {
		vs = append(vs, v)
	}
	sort.Slice(vs, func(a, b int) bool { return idx[vs[a]] < idx[vs[b]] })
	return vs
}
