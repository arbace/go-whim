package togo

// scm_fn.go is a lowered function printed as Scheme (scm.go says the
// frame): a top-level procedure of the editor and the C's parameters,
// whose blocks are local procedures -- joins and loops' heads -- of the
// variables live at their start, every jump a tail call: caprice's shape
// (hsshape.go), which Chez compiles as jumps.  A block's steps are
// bindings, each assignment to a variable a new binding of its name (let*,
// which a later value shadows), so no variable is ever set!; its
// terminator is a tail call of the next block, an if or a case of them, or
// the result.  A variable that is an array, a struct or a union, or whose
// address is taken, lives in the call's frame instead, read and written
// there.

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// sbind is a line of a block: a binding of names to an expression's
// values, or, with no names, an expression evaluated for what it does; lvl
// is what evaluating it does (scmPure, scmReads, scmCalls).
type sbind struct {
	names []string
	expr  string
	lvl   int
}

// sfn is one function being printed.
type sfn struct {
	s         *sgen
	lf        *lfn
	name      string
	mem       map[*lvar]int // a variable that lives in the frame -> its offset
	frameSize int
	ntmp      int
	sret      bool   // the result is a struct, written through sret
	ret       string // the Scheme kind of the result
	lines     []sbind

	base     map[*lvar]string // a variable's name
	used     map[string]bool
	cur      map[*lvar]string // a binding variable's value where the printing is: its name, or a constant
	entryCur map[*lvar]string

	live   map[*lblock]map[*lvar]bool
	start  *lblock
	fwd    map[*lblock]*lblock
	inline map[*lblock]bool
	loop   map[*lblock]bool
	fixed  map[*lvar]bool
	bnames map[*lblock]string
	tuple  bool
	outs   []*lvar
	sv     map[*lvar][]*lvar
	svOf   map[*lvar]*lvar
	extra  []*lvar
	isOut  map[*lvar]bool
	takes  bool // the function takes the editor
	framed bool // the call has a frame, which a return gives back
	locals map[*lvar]bool // the variables in the frame named in the body
	memory bool           // the body reads or writes memory by a name
}

// function prints one function definition, or says why it cannot.
func (s *sgen) function(fd *cc.FunctionDefinition) (src string, why string) {
	d := fd.Declarator
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(unsupported)
			if !ok {
				why = fmt.Sprint("panic: ", r)
				return
			}
			src, why = "", u.why
		}
	}()
	lf := lowerFunction(fd, s.g.a, s.g.p, scmName)
	f := &sfn{s: s, lf: lf, name: d.Name(), mem: map[*lvar]int{}, takes: s.takesEd(d.Name()), locals: map[*lvar]bool{}}
	ft := lf.ft
	f.tuple = s.h.tupleRet(d.Name())
	f.sret = isAggr(ft.Result()) && !f.tuple
	f.ret = scmTypeOf(ft.Result())
	f.outVars()
	f.structVars()
	f.placeVars(fd)
	f.nameVars()
	params := f.paramNames()
	head := "(define (" + strings.Join(append([]string{s.names[d.Name()]}, params...), " ") + ")"
	for _, rb := range s.g.p.RuntimeBodies {
		if rb.Name == d.Name() && rb.Scm != nil {
			// a rule of the runtime's, not a translation (Profile.RuntimeBodies)
			body := answerLayout(s.g.ast, rb.Scm(f.ret))
			return head + "\n  (let ([mem (ed-mem ed)])\n" + indent(body, 4) + "))\n", ""
		}
	}

	f.live = lf.live()
	f.liveOuts()
	// printed once, and again when the printing gave the call a frame its
	// locals had not (a struct a call returns, a struct value's address):
	// a return gives the frame back, and its value is read before
	placed, cases := f.frameSize, s.st.cases
	f.framed = placed > 0
	text := f.print()
	if f.frameSize > 0 && !f.framed {
		f.framed = true
		f.frameSize, f.ntmp, s.st.cases = placed, 0, cases
		text = f.print()
	}
	var binds []string
	if f.memory || scmUsesMem(text) {
		binds = append(binds, "[mem (ed-mem ed)]")
	}
	if f.frameSize > 0 {
		binds = append(binds, fmt.Sprintf("[fr (frame-push! ed %d)]", scmAlign(f.frameSize, 16)))
	}
	if f.frameSize > 0 {
		s.st.framed++
	}
	if len(f.outs) > 0 {
		s.st.values++
	}
	if f.tuple {
		s.st.tuples++
	}
	for _, bl := range lf.blocks {
		if f.local(bl) && f.loop[bl] {
			s.st.loops++
		} else if f.local(bl) {
			s.st.joins++
		}
	}
	var b strings.Builder
	b.WriteString(head + "\n")
	switch len(binds) {
	case 0:
		b.WriteString(indent(text, 2))
	default:
		kw := "let"
		if len(binds) > 1 {
			kw = "let*"
		}
		fmt.Fprintf(&b, "  (%s (%s)\n%s)", kw, strings.Join(binds, " "), indent(text, 4))
	}
	b.WriteString(")\n")
	return b.String(), ""
}

// print is the function's body: its local procedures, the parameters that
// live in the frame copied in, and its entry.
func (f *sfn) print() string {
	lf := f.lf
	f.shape()
	var locals []string
	for _, bl := range lf.blocks {
		if f.local(bl) {
			locals = append(locals, f.block(bl))
		}
	}
	f.cur = maps.Clone(f.entryCur)
	f.lines = nil
	for _, v := range lf.params {
		if _, ok := f.mem[v]; !ok {
			continue
		}
		a := f.localAddr(v)
		if isAggr(v.c) {
			f.emit("(mem-copy! %s %s %d)", a.base.val, f.argName(v), v.c.Size())
		} else {
			f.emit("%s", f.storeString(a, f.vtype(v), f.argName(v)))
		}
	}
	var forms []string
	for _, p := range f.lines {
		forms = append(forms, p.expr)
	}
	if f.inline[f.start] {
		forms = append(forms, f.body(f.start, 0)...)
	} else {
		forms = append(forms, f.jump(f.start))
	}
	// the variables in the frame the body names, by their names
	var defs []string
	for _, v := range lf.vars {
		if !f.locals[v] {
			continue
		}
		kind := scmAccess(scmTypeOf(v.c))
		if isAggr(v.c) || v.c.Kind() == cc.Array {
			kind = "agg"
		}
		defs = append(defs, fmt.Sprintf("(define-c-local %s &%s %s %d)", f.base[v], f.base[v], kind, f.mem[v]))
	}
	return strings.Join(append(append(defs, locals...), forms...), "\n")
}

// scmUsesMem says a function's text reads or writes memory: it binds mem.
func scmUsesMem(text string) bool {
	return strings.Contains(text, "(ld-") || strings.Contains(text, "(st-") || strings.Contains(text, "(mem-")
}

// paramNames are the procedure's parameters: the editor, the struct
// result's address, and the C's.
func (f *sfn) paramNames() []string {
	var ps []string
	if f.takes {
		ps = append(ps, "ed")
	}
	if f.sret {
		ps = append(ps, "sret")
	}
	for _, v := range f.lf.params {
		ps = append(ps, f.argName(v))
	}
	return ps
}

// argName is the name a parameter comes in by: its own, or, a parameter
// that lives in the frame, which its name reads, name.in.
func (f *sfn) argName(v *lvar) string {
	if _, ok := f.mem[v]; ok {
		return f.base[v] + ".in"
	}
	return f.base[v]
}

// placeVars decides which variables live in the frame: an array, a struct
// or a union, and a variable whose address is taken.
func (f *sfn) placeVars(fd *cc.FunctionDefinition) {
	taken := map[*cc.Declarator]bool{}
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof && !f.s.h.outArg[u] {
			if p, ok := unparenE(u.CastExpression).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
				if d, ok := p.ResolvedTo().(*cc.Declarator); ok {
					taken[d] = true
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(fd.CompoundStatement)
	for _, v := range f.lf.vars {
		if f.sv[v] != nil {
			continue
		}
		if isAggr(v.c) || v.c.Kind() == cc.Array || v.decl != nil && (taken[v.decl] || f.s.h.outLazy[v.decl]) {
			f.alloc(v, v.c)
		}
	}
}

// alloc gives v room in the frame.
func (f *sfn) alloc(v *lvar, t cc.Type) int {
	al := max(t.Align(), 1)
	off := (f.frameSize + al - 1) / al * al
	f.frameSize = off + int(max(t.Size(), 1))
	if v != nil {
		f.mem[v] = off
	}
	return off
}

// reg says v is a variable that is a binding, not in the frame.
func (f *sfn) reg(v *lvar) bool {
	_, inMem := f.mem[v]
	return !inMem
}

// nameVars gives each variable its name, and says what each binding
// variable is where the function starts: a parameter its name, any other
// its kind's zero, as C leaves it unset.
func (f *sfn) nameVars() {
	f.used = map[string]bool{"ed": true, "mem": true, "fr": true, "sret": true}
	for n := range f.lf.taken {
		f.used[n] = true
	}
	f.base = map[*lvar]string{}
	f.entryCur = map[*lvar]string{}
	vars := append(append([]*lvar{}, f.lf.vars...), f.extra...)
	for _, v := range f.lf.vars {
		f.base[v] = v.name // the lowering's: unique, and no name the body refers to
		f.used[v.name] = true
	}
	for _, v := range f.extra {
		n := scmName(v.name)
		for f.used[n] {
			n += "_"
		}
		f.used[n] = true
		f.base[v] = n
	}
	for _, v := range vars {
		if !f.reg(v) || f.sv[v] != nil {
			continue
		}
		if v.param {
			f.entryCur[v] = f.base[v]
		} else {
			f.entryCur[v] = scmZero(f.vtype(v))
		}
	}
}

// vtype is a variable's Scheme kind: a temporary that holds a truth value
// is a bool.
func (f *sfn) vtype(v *lvar) string {
	if v.boolean {
		return "bool"
	}
	t := v.c
	if f.isOut[v] {
		t = elemOf(t)
	}
	return scmTypeOf(t)
}

// scmZero is a kind's zero.
func scmZero(st string) string {
	switch st {
	case "bool":
		return "#f"
	case "void":
		return "(void)"
	}
	return "0"
}

// scmConstRe is a value a variable can be with no binding of its own: a
// literal, which no later binding shadows.
func scmConst(s string) bool {
	if s == "#t" || s == "#f" {
		return true
	}
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '-' {
		i = 1
	}
	if i == len(s) {
		return false
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// params are the variables a block is a procedure of: those live at its
// start that are bindings, in the function's order.
func (f *sfn) params(b *lblock) []*lvar {
	set := map[*lvar]bool{}
	var members []*lvar
	for v := range f.live[b] {
		if ms := f.sv[v]; ms != nil {
			members = append(members, v)
			continue
		}
		if f.reg(v) && !f.fixed[v] {
			set[v] = true
		}
	}
	out := f.lf.sortedVars(set)
	for _, s := range f.lf.sortedVars(sliceSet(members)) {
		out = append(out, f.sv[s]...)
	}
	return out
}

// jump is a tail call of b with the variables it is a procedure of.
func (f *sfn) jump(b *lblock) string {
	parts := []string{f.bname(b)}
	for _, v := range f.params(b) {
		parts = append(parts, f.valueOf(v))
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// valueOf is what v is where the printing is.
func (f *sfn) valueOf(v *lvar) string {
	s, ok := f.cur[v]
	if !ok {
		f.no(nil, "%s read where it has no value", v.name)
	}
	return s
}

// block is a block that is a local procedure: of what is live at its
// start.
func (f *sfn) block(b *lblock) string {
	head := []string{f.bname(b)}
	f.cur = map[*lvar]string{}
	for v := range f.fixed {
		f.cur[v] = f.base[v]
	}
	for _, v := range f.params(b) {
		f.cur[v] = f.base[v]
		head = append(head, f.base[v])
	}
	return "(define (" + strings.Join(head, " ") + ")\n" + indent(strings.Join(f.body(b, 0), "\n"), 2) + ")"
}

// body is block b's forms where the printing is (f.cur): its steps, and
// its terminator, into which a block reached by that one jump is written.
func (f *sfn) body(b *lblock, depth int) []string {
	saved := f.lines
	f.lines = nil
	tail := f.run(b, depth)
	out := scmRender(f.lines, tail)
	f.lines = saved
	return out
}

// run prints b's steps into the current lines and returns its
// terminator's expression.
func (f *sfn) run(b *lblock, depth int) string {
	if depth > 10000 {
		f.no(nil, "blocks written in place without end")
	}
	for _, s := range b.steps {
		f.step(s)
	}
	return f.term(b, depth)
}

// arm is the code that goes to s: a jump, or s written in place, one
// expression.
func (f *sfn) arm(s *lblock, depth int, cur map[*lvar]string) string {
	if !f.inline[s] {
		f.cur = cur
		return f.jump(s)
	}
	f.cur = maps.Clone(cur)
	return scmBegin(f.body(s, depth+1))
}

// scmBegin is forms as one expression.
func scmBegin(forms []string) string {
	if len(forms) == 1 {
		return forms[0]
	}
	return "(begin\n" + indent(strings.Join(forms, "\n"), 2) + ")"
}

// scmRender is a block's lines and its tail as body forms: the effects as
// they are, a run of bindings a let* (let-values for several values) whose
// body is the rest.
func scmRender(items []sbind, tail string) []string {
	var forms []string
	for i := 0; i < len(items); i++ {
		it := items[i]
		if it.names == nil {
			forms = append(forms, it.expr)
			continue
		}
		multi := len(it.names) != 1
		j := i
		for j < len(items) && items[j].names != nil && (len(items[j].names) != 1) == multi {
			j++
		}
		inner := scmRender(items[j:], tail)
		return append(forms, scmLet(items[i:j], multi, inner))
	}
	if tail != "" {
		forms = append(forms, tail)
	}
	return forms
}

// scmLet is a run of bindings around body.
func scmLet(bs []sbind, multi bool, body []string) string {
	kw := "let*"
	switch {
	case multi && len(bs) == 1:
		kw = "let-values"
	case multi:
		kw = "let*-values"
	case len(bs) == 1:
		kw = "let"
	}
	var b strings.Builder
	open := "(" + kw + " ("
	pad := strings.Repeat(" ", len(open))
	for i, x := range bs {
		name := x.names[0]
		if multi {
			name = "(" + strings.Join(x.names, " ") + ")"
		}
		item := "[" + name + " "
		lead := pad
		if i == 0 {
			lead = open
		}
		b.WriteString(lead + item + scmHang(x.expr, len(lead)+len(item)) + "]")
		if i < len(bs)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString(")\n" + indent(strings.Join(body, "\n"), 2) + ")")
	return b.String()
}

// scmHang is s whose lines after the first are indented by n: s's
// continuation where its first line follows n columns of text.
func scmHang(s string, n int) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	pad := strings.Repeat(" ", n)
	ls := strings.Split(s, "\n")
	for i := 1; i < len(ls); i++ {
		if ls[i] != "" {
			ls[i] = pad + ls[i]
		}
	}
	return strings.Join(ls, "\n")
}

// emit adds an effect to the current block.
func (f *sfn) emit(format string, args ...any) {
	f.lines = append(f.lines, sbind{expr: fmt.Sprintf(format, args...), lvl: scmCalls})
}

// bind adds a binding of name to x's value, its lines first.
func (f *sfn) bind(name string, x sx) {
	f.lines = append(f.lines, x.binds...)
	f.lines = append(f.lines, sbind{names: []string{name}, expr: x.val, lvl: x.lvl})
}

// flush adds a value's lines, and is its expression.
func (f *sfn) flush(v sx) string {
	f.lines = append(f.lines, v.binds...)
	return v.val
}

// step prints one step.
func (f *sfn) step(s lstep) {
	switch s.op {
	case opSet:
		v := f.lexpr(s.e)
		f.setVar(s.dst, v)
	case opAssign:
		v := f.lexpr(s.e)
		f.store(s.lhs, v)
	case opAssignOp:
		f.assignOp(s)
	case opIncDec:
		f.incDec(s)
	case opEval:
		// a call for what it does: its value, if any, not bound
		v := f.lexpr(s.e)
		f.lines = append(f.lines, v.binds...)
		if v.lvl > scmPure {
			f.lines = append(f.lines, sbind{expr: v.val, lvl: v.lvl})
		}
	case opInit:
		f.initVar(s)
	default:
		f.no(s.at, "a step %d", s.op)
	}
}

// setVar gives variable v the value x: a binding of its name, or, when x
// is a constant, x itself.
func (f *sfn) setVar(v *lvar, x sx) {
	if f.sv[v] != nil {
		f.copyStruct(v, x)
		return
	}
	if _, ok := f.mem[v]; ok {
		a := f.localAddr(v)
		if isAggr(v.c) && x.rec != nil {
			f.storeStruct(a, x.rec)
			return
		}
		if isAggr(v.c) {
			x = f.materialize(x, v.c)
			src := f.flush(x)
			f.emit("(mem-copy! %s %s %d)", a.base.val, src, v.c.Size())
			return
		}
		x = f.conv(x, f.vtype(v))
		val := f.flush(x)
		f.emit("%s", f.storeString(a, f.vtype(v), val))
		return
	}
	x = f.conv(x, f.vtype(v))
	n := f.base[v]
	if len(x.binds) == 0 && x.lvl == scmPure && (scmConst(x.val) || x.val == n) {
		f.cur[v] = x.val
		return
	}
	f.bind(n, x)
	f.cur[v] = n
}

// store writes x to the lvalue lhs.
func (f *sfn) store(lhs cc.ExpressionNode, x sx) {
	t := lhs.Type()
	if v := f.outDeref(lhs); v != nil {
		f.setVar(v, x)
		return
	}
	if m := f.memberVar(lhs); m != nil {
		f.setVar(m, x)
		return
	}
	if sv := f.structNamed(lhs); sv != nil {
		f.copyStruct(sv, x)
		return
	}
	if v := f.lf.lhsVar(lhs); v != nil && f.reg(v) {
		f.setVar(v, x)
		return
	}
	a := f.addrOf(lhs)
	if isAggr(t) && x.rec != nil {
		f.storeStruct(a, x.rec)
		return
	}
	if isAggr(t) && x.tup != nil {
		f.lines = append(f.lines, x.binds...)
		a.base = sx{val: f.once(a.base), st: "ptr"}
		st := t.(*cc.StructType)
		for i, v := range x.tup {
			fl := st.FieldByIndex(i)
			f.emit("%s", f.storeString(f.member(a, t, fl), scmTypeOf(fl.Type()), v))
		}
		return
	}
	if a.field != nil && a.field.IsBitfield() {
		f.no(lhs, "a bit field")
	}
	if !isAggr(t) {
		x = f.conv(x, scmTypeOf(t))
	}
	f.lines = append(f.lines, f.seq(&a.base, &x)...)
	if isAggr(t) {
		f.emit("(mem-copy! %s %s %d)", f.addrString(a), x.val, t.Size())
		return
	}
	f.emit("%s", f.storeString(a, scmTypeOf(t), x.val))
}

// once is x's value as an expression that can be evaluated again: x itself
// when it reads and calls nothing, else a temporary bound to it.
func (f *sfn) once(x sx) string {
	if x.lvl == scmPure && len(x.binds) == 0 {
		return x.val
	}
	f.lines = append(f.lines, x.binds...)
	if x.lvl == scmPure {
		return x.val
	}
	r := f.tmp()
	f.bind(r, sx{val: x.val, lvl: x.lvl})
	return r
}

// scmStore is the runtime's writer of a kind.
func scmStore(st string) string {
	switch st {
	case "bool":
		return "st-bool!"
	case "ptr":
		return "st-ptr!"
	}
	return "st-" + scmAccess(st) + "!"
}

// scmLoad is the runtime's reader of a kind.
func scmLoad(st string) string {
	switch st {
	case "bool":
		return "ld-bool"
	case "ptr":
		return "ld-ptr"
	}
	return "ld-" + scmAccess(st)
}

// scmAccess is i32 as the accessors name it, s32.
func scmAccess(st string) string {
	if strings.HasPrefix(st, "i") {
		return "s" + st[1:]
	}
	return st
}

// assignOp is lhs op= e: C's lhs = (T)(lhs op e), the operation in the
// operands' usual type.
func (f *sfn) assignOp(s lstep) {
	lt := s.lhs.Type()
	cur := f.readLval(s.lhs)
	r := f.lexpr(s.e)
	var nv sx
	if isPtrish(lt) {
		v := cur.v
		binds := f.seq(&v, &r)
		off := f.byteOff(r, elemSize(lt), s.aop == "-")
		nv = sx{binds: binds, val: scmAdd(v.val, off), st: "ptr", lvl: max(v.lvl, r.lvl)}
	} else {
		lk, _ := scalarKind(lt)
		rk, ok := scalarKind(s.e.typeOf())
		if !ok {
			rk = jInt
		}
		k := usualK(lk, rk)
		if s.aop == "<<" || s.aop == ">>" {
			k = promote(lk)
		}
		nv = f.arith(s.aop, cur.v, r, scmKindType(k))
	}
	cur.write(f, nv)
}

// incDec is lhs++ or lhs--.
func (f *sfn) incDec(s lstep) {
	lt := s.lhs.Type()
	cur := f.readLval(s.lhs)
	var nv sx
	switch st := scmTypeOf(lt); {
	case isPtrish(lt):
		d := elemSize(lt)
		if !s.inc {
			d = -d
		}
		nv = sx{binds: cur.v.binds, val: scmAdd(cur.v.val, strconv.FormatInt(d, 10)), st: "ptr", lvl: cur.v.lvl}
	case st == "bool":
		if s.inc {
			nv = sx{binds: cur.v.binds, val: "#t", st: "bool", konst: true, kv: 1}
		} else {
			nv = sx{binds: cur.v.binds, val: "(not " + cur.v.val + ")", st: "bool", lvl: cur.v.lvl}
		}
	default:
		// in the promoted type, converted back
		k := scmKindType(promote(mustKind(lt)))
		op := "+"
		if !s.inc {
			op = "-"
		}
		one := sx{val: "1", st: "i32", konst: true, kv: 1}
		nv = f.arith(op, cur.v, one, k)
	}
	cur.write(f, nv)
}

func mustKind(t cc.Type) jk {
	k, ok := scalarKind(t)
	if !ok {
		return jInt
	}
	return k
}

// slv is an lvalue read, and how it is written back: a binding, or memory
// at an address the read has already computed.
type slv struct {
	v   sx
	reg *lvar
	a   saddr
	t   cc.Type
}

func (f *sfn) readLval(e cc.ExpressionNode) slv {
	t := e.Type()
	if v := f.outDeref(e); v != nil {
		return slv{v: f.varRead(v), reg: v, t: t}
	}
	if m := f.memberVar(e); m != nil {
		return slv{v: f.varRead(m), reg: m, t: t}
	}
	if v := f.lf.lhsVar(e); v != nil && f.reg(v) {
		return slv{v: f.varRead(v), reg: v, t: t}
	}
	a := f.addrOf(e)
	if a.field != nil && a.field.IsBitfield() {
		f.no(e, "a bit field")
	}
	// the address once: read and written at the same place
	a.base = sx{val: f.once(a.base), st: "ptr"}
	st := scmTypeOf(t)
	return slv{v: sx{val: f.loadString(a, st), st: st, lvl: scmReads}, a: a, t: t}
}

func (l slv) write(f *sfn, x sx) {
	if l.reg != nil {
		f.setVar(l.reg, x)
		return
	}
	st := scmTypeOf(l.t)
	val := f.flush(f.conv(x, st))
	f.emit("%s", f.storeString(l.a, st, val))
}

// scmIdent says s is a name.
func scmIdent(s string) bool {
	return s != "" && !strings.ContainsAny(s, "() \n") && !scmConst(s)
}

// initVar is a declared local's initializer where C declares it: a braced
// list or a string, written into the variable's memory anew.
func (f *sfn) initVar(s lstep) {
	v := s.dst
	if f.sv[v] != nil {
		f.initStruct(v, s.in)
		return
	}
	off, ok := f.mem[v]
	if !ok {
		in := s.in
		for in != nil && in.Case == cc.InitializerInitList {
			if in.InitializerList == nil {
				f.setVar(v, sx{val: scmZero(f.vtype(v)), st: f.vtype(v), konst: true})
				return
			}
			in = in.InitializerList.Initializer
		}
		f.setVar(v, f.expr(in.AssignmentExpression))
		return
	}
	_ = off
	a := f.localAddr(v)
	f.emit("(mem-zero! %s %d)", a.base.val, v.c.Size())
	f.initInto(a.base.val, v.c, s.in)
}

// initInto writes initializer in, of an object of type t at base, into
// the frame: each value at its offset from the object's address.
func (f *sfn) initInto(base string, t cc.Type, in *cc.Initializer) {
	if in == nil {
		return
	}
	if in.Case == cc.InitializerExpr {
		e := in.AssignmentExpression
		at := int(in.Offset())
		it := in.Type()
		if it == nil {
			it = t
		}
		addr := scmAddr(base, at)
		if sv, ok := unparenE(e).Value().(cc.StringValue); ok && it.Kind() == cc.Array {
			n := min(int(it.Size()), len(sv))
			for i := 0; i < n; i++ {
				if sv[i] != 0 {
					f.emit("(st-u8! %s %d)", scmAddr(base, at+i), sv[i])
				}
			}
			return
		}
		if fl := in.Field(); fl != nil && fl.IsBitfield() {
			f.no(e, "a bit field's initializer")
		}
		if isAggr(it) {
			x := f.expr(e)
			if x.rec != nil {
				f.storeStruct(saddr{base: sx{val: base, st: "ptr"}, off: at}, x.rec)
				return
			}
			x = f.materialize(x, it)
			f.emit("(mem-copy! %s %s %d)", addr, f.flush(x), it.Size())
			return
		}
		st := scmTypeOf(it)
		x := f.expr(e)
		if x.konst && x.kv == 0 && !x.lit {
			return // the memory is zeroed
		}
		val := f.flush(f.conv(x, st))
		f.emit("(%s %s %s)", scmStore(st), addr, val)
		return
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		f.initInto(base, t, l.Initializer)
	}
}

// term is a block's terminator as an expression.
func (f *sfn) term(b *lblock, depth int) string {
	t := b.term
	switch t.kind {
	case tGoto:
		to := f.resolve(t.to[0])
		if f.inline[to] {
			return f.run(to, depth+1)
		}
		return f.jump(to)
	case tIf:
		c := f.truth(f.lexpr(t.cond))
		cv := f.flush(c)
		cur := f.cur
		then := f.arm(f.resolve(t.to[0]), depth, cur)
		els := f.arm(f.resolve(t.to[1]), depth, cur)
		return scmIf(cv, then, els)
	case tSwitch:
		x := f.lexpr(t.cond)
		if x.st == "bool" {
			x = f.conv(x, "i32")
		}
		v := f.flush(x)
		cur := f.cur
		var arms []string
		for i, vs := range t.cases {
			var labels []string
			for _, cv := range vs {
				labels = append(labels, scmLit(scmTrunc(cv, x.st), x.st))
			}
			arms = append(arms, scmArm("("+strings.Join(labels, " ")+")", f.arm(f.resolve(t.to[i]), depth, cur)))
		}
		arms = append(arms, scmArm("else", f.arm(f.resolve(t.to[len(t.to)-1]), depth, cur)))
		f.s.st.cases++
		return "(case " + v + "\n" + indent(strings.Join(arms, "\n"), 2) + ")"
	case tRet:
		switch {
		case t.ret.isZero():
			return f.result(scmZero(f.ret))
		case f.tuple:
			x := f.lexpr(t.ret)
			var vals []string
			for _, v := range f.structVals(x, f.lf.ft.Result()) {
				if f.framed {
					// read before the frame is given back
					vals = append(vals, f.once(v))
				} else {
					vals = append(vals, f.flush(v))
				}
			}
			return f.result("(values " + strings.Join(vals, " ") + ")")
		case f.sret:
			x := f.lexpr(t.ret)
			if x.rec != nil {
				f.storeStruct(saddr{base: sx{val: "sret", st: "ptr"}}, x.rec)
			} else {
				x = f.materialize(x, f.lf.ft.Result())
				f.emit("(mem-copy! sret %s %d)", f.flush(x), f.lf.ft.Result().Size())
			}
			return f.result("(void)")
		default:
			v := f.conv(f.lexpr(t.ret), f.ret)
			if f.framed {
				// computed before the frame is given back
				return f.result(f.once(v))
			}
			return f.result(f.flush(v))
		}
	case tFall:
		if f.sret || f.tuple {
			return f.result("(void)")
		}
		return f.result(scmZero(f.ret))
	}
	f.no(nil, "a terminator %d", t.kind)
	return ""
}

// scmIf is (if c a b), on one line when it fits.
func scmIf(c, a, b string) string {
	one := "(if " + c + " " + a + " " + b + ")"
	if !strings.Contains(one, "\n") && len(one) <= 100 {
		return one
	}
	return "(if " + scmHang(c, 4) + "\n" + indent(a, 4) + "\n" + indent(b, 4) + ")"
}

// scmArm is a case's clause.
func scmArm(labels, body string) string {
	one := "[" + labels + " " + body + "]"
	if !strings.Contains(one, "\n") && len(one) <= 100 {
		return one
	}
	return "[" + labels + "\n" + indent(body, 1) + "]"
}

// result is the function's result v, its frame given back first.
func (f *sfn) result(v string) string {
	if len(f.outs) > 0 {
		var parts []string
		if f.ret != "void" && !f.sret {
			parts = append(parts, v)
		}
		for _, o := range f.outs {
			parts = append(parts, f.valueOf(o))
		}
		if len(parts) == 1 {
			v = parts[0]
		} else {
			v = "(values " + strings.Join(parts, " ") + ")"
		}
	}
	if f.framed {
		f.emit("(frame-pop! ed fr)")
	}
	return v
}

func (f *sfn) no(n cc.Node, format string, args ...any) {
	where := ""
	if n != nil {
		where = fmt.Sprintf(" at %d", n.Position().Line)
	}
	panic(unsupported{fmt.Sprintf(format, args...) + where})
}

// tmp is a new name for a value the printer binds.
func (f *sfn) tmp() string {
	for {
		f.ntmp++
		n := "r" + strconv.Itoa(f.ntmp)
		if !f.used[n] {
			return n
		}
	}
}

// --- the shape: hsshape.go's ---------------------------------------------------

// shape decides, for the function being printed, where each block goes.
func (f *sfn) shape() {
	lf := f.lf
	f.fixed = f.unassigned()
	f.fwd = map[*lblock]*lblock{}
	for _, b := range lf.blocks {
		if len(b.steps) == 0 && b.term.kind == tGoto && b.term.to[0] != b {
			f.fwd[b] = b.term.to[0]
		}
	}
	f.start = f.resolve(lf.blocks[0])
	n := map[*lblock]int{f.start: 1}
	f.loop = map[*lblock]bool{}
	for _, b := range lf.blocks {
		if f.resolve(b) != b {
			continue
		}
		for _, s := range b.term.to {
			t := f.resolve(s)
			n[t]++
			if t.id <= b.id {
				f.loop[t] = true
			}
		}
	}
	f.inline = map[*lblock]bool{}
	for _, b := range lf.blocks {
		if f.resolve(b) == b && (n[b] == 1 && !f.loop[b] || f.onlyResult(b)) {
			f.inline[b] = true
		}
	}
	if f.bnames != nil {
		return // named in the first printing
	}
	f.bnames = map[*lblock]string{}
	for _, b := range lf.blocks {
		base := "join"
		if f.loop[b] {
			base = "loop"
		}
		name := base + strconv.Itoa(b.id)
		for f.used[name] {
			name += "_"
		}
		f.used[name] = true
		f.bnames[b] = name
	}
}

// onlyResult says b does nothing but return a constant or a binding: it is
// written where each jump to it is.
func (f *sfn) onlyResult(b *lblock) bool {
	if len(b.steps) > 0 || f.sret || f.framed {
		return false
	}
	switch t := b.term; t.kind {
	case tFall:
		return true
	case tRet:
		switch {
		case t.ret.isZero():
			return true
		case t.ret.v != nil:
			return f.reg(t.ret.v)
		case t.ret.n != nil:
			_, known := intValue(t.ret.n.Value())
			return known && !hasEffect(t.ret.n) && !f.hasSub(t.ret.n)
		}
	}
	return false
}

// resolve is where a jump to b goes: b, or, when b only jumps, where it
// jumps.
func (f *sfn) resolve(b *lblock) *lblock {
	seen := map[*lblock]bool{}
	for t := b; ; {
		next, ok := f.fwd[t]
		if !ok {
			return t
		}
		if seen[t] {
			return b
		}
		seen[t] = true
		t = next
	}
}

// local says b is printed as a local procedure.
func (f *sfn) local(b *lblock) bool {
	return f.resolve(b) == b && !f.inline[b]
}

// bname is a local procedure's name: a loop's head is a loop.
func (f *sfn) bname(b *lblock) string { return f.bnames[b] }

// unassigned are the parameters no step assigns: each is its one name
// everywhere in the function, and no local procedure takes it.
func (f *sfn) unassigned() map[*lvar]bool {
	set := map[*lvar]bool{}
	for _, b := range f.lf.blocks {
		for _, s := range b.steps {
			switch s.op {
			case opSet, opInit:
				set[s.dst] = true
			case opAssign, opAssignOp, opIncDec:
				if v := f.lf.lhsVar(s.lhs); v != nil {
					set[v] = true
				}
			}
		}
	}
	walkAddrs(f.lf.fd.CompoundStatement, func(u *cc.UnaryExpression, d *cc.Declarator) {
		if v, ok := f.lf.byDecl[d]; ok && f.s.h.outArg[u] {
			set[v] = true
		}
	})
	fixed := map[*lvar]bool{}
	for _, v := range f.lf.params {
		if !set[v] && f.reg(v) && !f.isOut[v] {
			fixed[v] = true
		}
	}
	return fixed
}

// --- out-parameters and struct values: hsout.go's and hsstruct.go's ------------

func (f *sfn) outVars() {
	f.isOut = map[*lvar]bool{}
	for _, i := range f.s.h.outs[f.name] {
		if i < len(f.lf.params) {
			v := f.lf.params[i]
			f.outs = append(f.outs, v)
			f.isOut[v] = true
		}
	}
}

func (f *sfn) outDeref(e cc.ExpressionNode) *lvar {
	if len(f.outs) == 0 {
		return nil
	}
	u, ok := unparenE(e).(*cc.UnaryExpression)
	if !ok || u.Case != cc.UnaryExpressionDeref {
		return nil
	}
	return f.outNamed(u.CastExpression)
}

func (f *sfn) outNamed(e cc.ExpressionNode) *lvar {
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

func (f *sfn) liveOuts() {
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

func (f *sfn) outCall(call *cc.PostfixExpression) (map[int]*lvar, bool) {
	d := fnDesignator(call.PostfixExpression)
	if d == nil || len(f.s.h.outs[d.Name()]) == 0 {
		return nil, false
	}
	m := map[int]*lvar{}
	i := 0
	for l := call.ArgumentExpressionList; l != nil; l, i = l.ArgumentExpressionList, i+1 {
		if !f.s.h.isOut(d.Name(), i) {
			continue
		}
		u, x := addrOfLocal(l.AssignmentExpression)
		v := f.lf.byDecl[x]
		if u == nil || !f.s.h.outArg[u] || v == nil {
			f.no(call, "an out-argument that is not a local's address")
		}
		m[i] = v
	}
	return m, true
}

func (f *sfn) structVars() {
	f.sv = map[*lvar][]*lvar{}
	f.svOf = map[*lvar]*lvar{}
	for _, v := range f.lf.vars {
		if v.decl == nil || !f.s.h.sval[v.decl] {
			continue
		}
		st := v.c.(*cc.StructType)
		for i := 0; i < st.NumFields(); i++ {
			fl := st.FieldByIndex(i)
			m := &lvar{name: v.name + "_" + fl.Name(), c: fl.Type()}
			f.sv[v] = append(f.sv[v], m)
			f.svOf[m] = v
			f.extra = append(f.extra, m)
		}
	}
}

func (f *sfn) memberVar(e cc.ExpressionNode) *lvar {
	if len(f.sv) == 0 {
		return nil
	}
	x, ok := unparenE(e).(*cc.PostfixExpression)
	if !ok || x.Case != cc.PostfixExpressionSelect {
		return nil
	}
	s := f.structNamed(x.PostfixExpression)
	if s == nil {
		return nil
	}
	for i, m := range f.sv[s] {
		if f.fieldOf(s, i).Name() == x.Field().Name() {
			return m
		}
	}
	f.no(x, "a member %s of a struct that is a value", x.Field().Name())
	return nil
}

func (f *sfn) structNamed(e cc.ExpressionNode) *lvar {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return nil
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	if !ok {
		return nil
	}
	if v := f.lf.byDecl[d]; v != nil && f.sv[v] != nil {
		return v
	}
	return nil
}

func (f *sfn) fieldOf(s *lvar, i int) *cc.Field {
	return s.c.(*cc.StructType).FieldByIndex(i)
}

// copyStruct gives struct s the value x: a call's values (x.tup), another
// struct that is a value (x.rec), or the struct at the address x is.
func (f *sfn) copyStruct(s *lvar, x sx) {
	if x.tup != nil {
		f.lines = append(f.lines, x.binds...)
		for i, m := range f.sv[s] {
			f.setVar(m, sx{val: x.tup[i], st: f.vtype(m)})
		}
		return
	}
	if x.rec != nil {
		// the members read first, then written: s may be x
		var vals []sx
		for i := range f.sv[s] {
			vals = append(vals, f.varRead(f.sv[x.rec][i]))
		}
		for i, m := range f.sv[s] {
			f.setVar(m, vals[i])
		}
		return
	}
	src := saddr{base: sx{val: f.once(x), st: "ptr"}}
	for i, m := range f.sv[s] {
		fl := f.fieldOf(s, i)
		f.setVar(m, f.readAt(f.member(src, s.c, fl), fl.Type()))
	}
}

// storeStruct writes struct s, a value, member by member at a.
func (f *sfn) storeStruct(a saddr, s *lvar) {
	a.base = sx{val: f.once(a.base), st: "ptr"}
	for i, m := range f.sv[s] {
		fl := f.fieldOf(s, i)
		st := scmTypeOf(fl.Type())
		val := f.flush(f.conv(f.varRead(m), st))
		f.emit("%s", f.storeString(f.member(a, s.c, fl), st, val))
	}
}

// initStruct is struct s's initializer where C declares it.
func (f *sfn) initStruct(s *lvar, in *cc.Initializer) {
	if in != nil && in.Case == cc.InitializerExpr {
		f.copyStruct(s, f.expr(in.AssignmentExpression))
		return
	}
	vals := map[int64]sx{}
	var walk func(in *cc.Initializer)
	walk = func(in *cc.Initializer) {
		if in == nil {
			return
		}
		if in.Case == cc.InitializerExpr {
			vals[in.Offset()] = f.expr(in.AssignmentExpression)
			return
		}
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			walk(l.Initializer)
		}
	}
	walk(in)
	for i, m := range f.sv[s] {
		fl := f.fieldOf(s, i)
		if x, ok := vals[fl.Offset()]; ok {
			f.setVar(m, x)
		} else {
			f.setVar(m, sx{val: scmZero(f.vtype(m)), st: f.vtype(m), konst: true})
		}
	}
}

// structVals are the members' values of x, a struct value of type t: a
// call's values, a struct that is a value's members, or the members read
// from the address x is -- reads only, which may be evaluated in any order.
func (f *sfn) structVals(x sx, t cc.Type) []sx {
	st := t.(*cc.StructType)
	var out []sx
	switch {
	case x.tup != nil:
		f.lines = append(f.lines, x.binds...)
		for i, n := range x.tup {
			out = append(out, sx{val: n, st: scmTypeOf(st.FieldByIndex(i).Type())})
		}
		return out
	case x.rec != nil:
		for i, m := range f.sv[x.rec] {
			fl := st.FieldByIndex(i)
			out = append(out, f.conv(f.varRead(m), scmTypeOf(fl.Type())))
		}
		return out
	}
	src := saddr{base: sx{val: f.once(x), st: "ptr"}}
	for i := 0; i < st.NumFields(); i++ {
		fl := st.FieldByIndex(i)
		out = append(out, f.readAt(f.member(src, t, fl), fl.Type()))
	}
	return out
}

// materialize is a struct value in memory: a call's values written into
// the frame, for what takes a struct's address.
func (f *sfn) materialize(x sx, t cc.Type) sx {
	if x.tup == nil {
		return x
	}
	binds := append([]sbind{}, x.binds...)
	off := f.alloc(nil, t)
	base := fmt.Sprintf("(fx+ fr %d)", off)
	st := t.(*cc.StructType)
	for i, v := range x.tup {
		fl := st.FieldByIndex(i)
		binds = append(binds, sbind{expr: fmt.Sprintf("(%s (fx+ fr %d) %s)", scmStore(scmTypeOf(fl.Type())), off+int(fl.Offset()), v), lvl: scmCalls})
	}
	return sx{binds: binds, val: base, st: "agg"}
}
