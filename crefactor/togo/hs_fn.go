package togo

// hs_fn.go is a lowered function printed as Haskell (hs.go says the
// frame): a top-level function in IO of the editor and the C's parameters,
// whose blocks are local functions -- join points -- of the variables live
// at their start.  A block's steps are a do block's lines, each assignment
// to a variable a new binding of its name, so the variables need no cells;
// its terminator is a tail call of the next block, an if or a case of
// them, or the result.  A variable that is an array, a struct or a union,
// or whose address is taken, lives in the call's frame instead, read and
// written there.

import (
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// hfn is one function being printed.
type hfn struct {
	h         *hgen
	lf        *lfn
	name      string
	mem       map[*lvar]int // a variable that lives in the frame -> its offset
	frameSize int
	ntmp      int
	sret      bool   // the result is a struct, written through sret'
	ret       string // the Haskell type of the result
	lines     []string

	// the names: every binding is a name of its own, so none shadows another
	// (hsnames.go)
	base     map[*lvar]string // a variable's first name: its C name, unless taken
	ver      map[*lvar]int
	used     map[string]bool
	cur      map[*lvar]string // a binding variable's value where the printing is: a name, or an atom
	entryCur map[*lvar]string // the same, where the function starts

	// the shape (hsshape.go)
	live   map[*lblock]map[*lvar]bool
	start  *lblock
	fwd    map[*lblock]*lblock // a block that only jumps -> where it goes
	inline map[*lblock]bool    // reached by one jump: written there
	loop   map[*lblock]bool    // a loop's head: a jump back reaches it
	fixed  map[*lvar]bool      // parameters never assigned: read from the scope
	pure   bool                // a function of its arguments (hseffects.go)
	tuple  bool                // its struct result is a tuple (hsstruct.go)
	outs   []*lvar             // the out-parameters, as values (hsout.go)
	sv     map[*lvar][]*lvar   // a struct that is a value -> its members' variables (hsstruct.go)
	svOf   map[*lvar]*lvar
	extra  []*lvar // variables the lowering did not make: the members
	isOut  map[*lvar]bool

	image []byte // initGlobals: the segment's constant bytes (hsnamed.go)
}

// function prints one function definition, or says why it cannot.
func (h *hgen) function(fd *cc.FunctionDefinition) (src string, why string) {
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
	lf := lowerFunction(fd, h.g.a, h.g.p, hsName)
	f := &hfn{h: h, lf: lf, name: d.Name(), mem: map[*lvar]int{}, pure: h.pure(d.Name())}
	for _, rb := range h.g.p.RuntimeBodies {
		if rb.Name == d.Name() && rb.Hs != nil {
			// a rule of the runtime's, not a translation (Profile.RuntimeBodies)
			sig, params := h.signature(d, lf)
			body := h.layoutMarks(rb.Hs(f.h.hsType(lf.ft.Result())))
			return fmt.Sprintf("%s\n%s %s= do\n%s\n", sig, h.names[d.Name()], strings.Join(append(params, ""), " "), indent(body, 2)), ""
		}
	}
	ft := lf.ft
	f.tuple = h.tupleRet(d.Name())
	f.sret = isAggr(ft.Result()) && !f.tuple
	f.ret = f.h.hsType(ft.Result())
	f.outVars()
	f.structVars()
	f.placeVars(fd)
	f.nameVars()
	sig, params := h.signature(d, lf)
	first := 0
	if h.takesEd(d.Name()) {
		first++ // ed' is the first
	}
	if f.sret {
		first++ // then sret'
	}
	for i, v := range lf.params {
		if first+i < len(params) {
			params[first+i] = f.base[v]
		}
	}

	// the blocks: those reached by one jump written where the jump is, the
	// rest -- joins and loops' heads -- local functions of what is live at
	// their start (hsshape.go)
	f.live = lf.live()
	f.liveOuts()
	f.shape()
	var b strings.Builder
	b.WriteString(sig + "\n")
	fmt.Fprintf(&b, "%s %s=", h.names[d.Name()], strings.Join(append(params, ""), " "))
	var body []string
	// the parameters that live in the frame: copied in
	for _, v := range lf.params {
		off, ok := f.mem[v]
		if !ok {
			continue
		}
		if isAggr(v.c) {
			body = append(body, fmt.Sprintf("copyMem (pAdd fr' %d) %s %d", off, f.base[v], v.c.Size()))
		} else {
			body = append(body, fmt.Sprintf("wr%s fr' %d %s", hsAccess(f.h.hsType(v.c)), off, f.base[v]))
		}
	}
	var locals []string
	for _, bl := range lf.blocks {
		if f.local(bl) {
			locals = append(locals, f.block(bl))
		}
	}
	f.cur = f.entryCur
	start := []string{f.jump(f.start)}
	if f.inline[f.start] {
		start = f.body(f.start, 0)
	}
	if len(locals) > 0 {
		body = append(body, "let")
		for _, l := range locals {
			body = append(body, indent(l, 2))
		}
	}
	if f.pure {
		// a function of its arguments: no do, lets and an expression
		if len(locals) > 0 {
			body = append(body, "in "+strings.Join(indentRest(f.pureBlock(start), 3), "\n"))
		} else {
			body = f.pureBlock(start)
		}
		fmt.Fprintf(&b, "\n%s\n", indent(strings.Join(body, "\n"), 2))
		return hsTidy(b.String()), ""
	}
	body = append(body, start...)
	text := "do\n" + indent(strings.Join(body, "\n"), 2)
	if f.frameSize > 0 {
		fmt.Fprintf(&b, " frame %d $ \\fr' -> %s\n", f.frameSize, text)
	} else {
		fmt.Fprintf(&b, " %s\n", text)
	}
	return hsTidy(b.String()), ""
}

// placeVars decides which variables live in the frame: an array, a struct
// or a union, and a variable whose address is taken.
func (f *hfn) placeVars(fd *cc.FunctionDefinition) {
	taken := map[*cc.Declarator]bool{}
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof && !f.h.outArg[u] {
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
			continue // its members are bindings
		}
		if isAggr(v.c) || v.c.Kind() == cc.Array || v.decl != nil && (taken[v.decl] || f.h.outLazy[v.decl]) {
			f.alloc(v, v.c)
		}
	}
}

// alloc gives v room in the frame.
func (f *hfn) alloc(v *lvar, t cc.Type) int {
	al := max(t.Align(), 1)
	off := (f.frameSize + al - 1) / al * al
	f.frameSize = off + int(max(t.Size(), 1))
	if v != nil {
		f.mem[v] = off
	}
	return off
}

// reg says v is a variable that is a Haskell binding, not in the frame.
func (f *hfn) reg(v *lvar) bool {
	_, inMem := f.mem[v]
	return !inMem
}

// params are the variables a block is a function of: those live at its
// start that are bindings, in the function's order.
func (f *hfn) params(b *lblock, live map[*lblock]map[*lvar]bool) []*lvar {
	set := map[*lvar]bool{}
	var members []*lvar
	for v := range live[b] {
		if ms := f.sv[v]; ms != nil {
			members = append(members, v)
			continue
		}
		if f.reg(v) && !f.fixed[v] {
			set[v] = true
		}
	}
	out := f.lf.sortedVars(set)
	// a struct that is a value: its members, in the struct's order
	for _, s := range f.lf.sortedVars(sliceSet(members)) {
		out = append(out, f.sv[s]...)
	}
	return out
}

// jump is a tail call of b with the variables it is a function of, each
// under its name where the jump is.
func (f *hfn) jump(b *lblock) string {
	parts := []string{f.bname(b)}
	for _, v := range f.params(b, f.live) {
		parts = append(parts, f.valueOf(f.cur, v))
	}
	return strings.Join(parts, " ")
}

// vtype is a variable's Haskell type: a temporary that holds a truth value
// is a Bool.
func (f *hfn) vtype(v *lvar) string {
	if v.boolean {
		return "Bool"
	}
	t := v.c
	if f.isOut[v] {
		t = elemOf(t) // the value it points at
	}
	if t.Kind() == cc.Array {
		t = t.Decay()
	}
	return f.h.hsType(t)
}

func sliceSet(vs []*lvar) map[*lvar]bool {
	m := map[*lvar]bool{}
	for _, v := range vs {
		m[v] = true
	}
	return m
}

// hsZero is a type's zero.
func hsZero(ht string) string {
	switch {
	case ht == "Bool":
		return "False"
	case isPtrHt(ht):
		return "nullPtr"
	case ht == "()":
		return "()"
	}
	return "(0 :: " + ht + ")"
}

// block is a block that is a local function: of what is live at its
// start, each a name of its own.
func (f *hfn) block(b *lblock) string {
	head := []string{f.bname(b)}
	f.cur = map[*lvar]string{}
	for v := range f.fixed {
		f.cur[v] = f.base[v]
	}
	for _, v := range f.params(b, f.live) {
		n := f.fresh(v)
		f.cur[v] = n
		head = append(head, "!"+n)
	}
	if f.pure {
		return strings.Join(head, " ") + " =\n" + indent(strings.Join(f.pureBlock(f.body(b, 0)), "\n"), 2)
	}
	return strings.Join(head, " ") + " = do\n" + indent(strings.Join(f.body(b, 0), "\n"), 2)
}

// pureBlock is a block's lines in a pure function as one expression: its
// lets, and the expression they are in.
func (f *hfn) pureBlock(ls []string) []string {
	var binds []string
	i := 0
	for i < len(ls) && strings.HasPrefix(ls[i], "let !") {
		binds = append(binds, strings.TrimPrefix(ls[i], "let "))
		i++
	}
	rest := ls[i:]
	for _, l := range rest {
		if strings.HasPrefix(l, "let ") || strings.Contains(l, " <- ") {
			f.no(nil, "a pure function's line that is an action: %s", l)
		}
	}
	if len(rest) == 0 {
		f.no(nil, "a pure function's block with no value")
	}
	if len(binds) == 0 {
		return rest
	}
	out := []string{"let " + binds[0]}
	for _, b := range binds[1:] {
		out = append(out, "    "+b)
	}
	return append(out, indentRest(append([]string{"in " + rest[0]}, rest[1:]...), 3)...)
}

// indentRest indents every line but the first by n.
func indentRest(ls []string, n int) []string {
	out := append([]string{}, ls...)
	pad := strings.Repeat(" ", n)
	for i := 1; i < len(out); i++ {
		out[i] = strings.ReplaceAll(pad+out[i], "\n", "\n"+pad)
	}
	return out
}

// body is block b's lines where the printing is (f.cur): its steps, and its
// terminator, into which a block reached by that one jump is written.
func (f *hfn) body(b *lblock, depth int) []string {
	if depth > 10000 {
		f.no(nil, "blocks written in place without end")
	}
	saved := f.lines
	f.lines = nil
	for _, s := range b.steps {
		f.step(s)
	}
	f.term(b, depth)
	out := f.lines
	f.lines = saved
	return out
}

// arm is the code that goes to s: a jump, or s written in place -- on the
// same line when it is one line, else a do block below.  cur is the names
// where the arm starts; an arm's own bindings are its alone.
func (f *hfn) arm(s *lblock, depth int, cur map[*lvar]string) string {
	if !f.inline[s] {
		f.cur = cur
		return f.jump(s)
	}
	f.cur = maps.Clone(cur)
	ls := f.body(s, depth+1)
	if f.pure {
		ls = f.pureBlock(ls)
	}
	if len(ls) == 1 && !strings.Contains(ls[0], "\n") {
		return ls[0]
	}
	if f.pure {
		return "\n" + indent(strings.Join(ls, "\n"), 2)
	}
	return "do\n" + indent(strings.Join(ls, "\n"), 2)
}

// armed is a keyword and its arm: on one line, or the arm below it.
func armed(kw, arm string) string {
	if strings.HasPrefix(arm, "\n") {
		return kw + arm
	}
	return kw + " " + arm
}

// emit adds a line to the current block.
func (f *hfn) emit(format string, args ...any) {
	f.lines = append(f.lines, fmt.Sprintf(format, args...))
}

// flush adds a value's binds.
func (f *hfn) flush(v hv) string {
	f.lines = append(f.lines, v.binds...)
	return v.val
}

// flushPlain is flush of a value whose type the place it goes fixes: a
// literal without its annotation.
func (f *hfn) flushPlain(v hv) string {
	f.lines = append(f.lines, v.binds...)
	return v.plain()
}

// step prints one step.
func (f *hfn) step(s lstep) {
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
		v := f.lexpr(s.e)
		f.lines = append(f.lines, v.binds...)
		if v.act != "" {
			f.emit("%s", v.act)
		}
	case opInit:
		f.initVar(s)
	default:
		f.no(s.at, "a step %d", s.op)
	}
}

// setVar gives variable v the value x: a binding of a name of its own (a
// let, strict as C's assignment), or, when x is a name or a literal, x
// itself.
func (f *hfn) setVar(v *lvar, x hv) {
	if f.sv[v] != nil {
		f.copyStruct(v, x)
		return
	}
	if off, ok := f.mem[v]; ok {
		if isAggr(v.c) {
			src := f.flush(x)
			f.emit("copyMem (pAdd fr' %d) %s %d", off, src, v.c.Size())
			return
		}
		x = f.conv(x, f.vtype(v))
		val := f.flushPlain(x)
		f.emit("wr%s fr' %d %s", hsAccess(f.vtype(v)), off, val)
		return
	}
	x = f.conv(x, f.vtype(v))
	val := f.flush(x)
	if hsAtom(val) {
		// a name or a literal: the variable is it, with no binding
		f.cur[v] = val
		return
	}
	n := f.fresh(v)
	f.emit("let !%s = %s", n, hsUnparen(val))
	f.cur[v] = n
}

// store writes x to the lvalue lhs.
func (f *hfn) store(lhs cc.ExpressionNode, x hv) {
	t := lhs.Type()
	if v := f.outDeref(lhs); v != nil {
		f.setVar(v, x)
		return
	}
	if m := f.memberVar(lhs); m != nil {
		f.setVar(m, x)
		return
	}
	if s := f.structNamed(lhs); s != nil {
		f.copyStruct(s, x)
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
		// a tuple's members written where they go
		f.lines = append(f.lines, x.binds...)
		base := hv{val: f.flush(a.base), ht: "P"}
		st := t.(*cc.StructType)
		for i, v := range x.tup {
			fl := st.FieldByIndex(i)
			ma := f.member(haddr{base: base, off: a.off, syms: a.syms, symOff: a.symOff}, t, fl)
			ht := f.h.hsType(fl.Type())
			f.emit("wr%s %s %s %s", hsAccess(ht), base.val, offStr(ma), v)
		}
		return
	}
	base := f.flush(a.base)
	if isAggr(t) {
		src := f.flush(x)
		f.emit("copyMem %s %s %d", f.addrVal(haddr{base: hv{val: base, ht: "P"}, off: a.off, syms: a.syms, symOff: a.symOff}).val, src, t.Size())
		return
	}
	if a.field != nil && a.field.IsBitfield() {
		f.no(lhs, "a bit field")
	}
	ht := f.h.hsType(t)
	val := f.flushPlain(f.conv(x, ht))
	if a.whole && a.obj != nil {
		f.emit("set'%s ed' %s", a.obj.name, val)
		return
	}
	f.emit("wr%s %s %s %s", hsAccess(ht), base, offStr(a), val)
}

// assignOp is lhs op= e: C's lhs = (T)(lhs op e), the operation in the
// operands' usual type.
func (f *hfn) assignOp(s lstep) {
	lt := s.lhs.Type()
	cur := f.readLval(s.lhs)
	r := f.lexpr(s.e)
	var nv hv
	if isPtrish(lt) {
		// pointer += or -= an integer
		off := f.byteOff(r, elemSize(lt), s.aop == "-")
		nv = hv{binds: append(cur.v.binds, r.binds...), val: fmt.Sprintf("(pAdd %s %s)", cur.v.val, off), ht: "P"}
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
		nv = f.arith(s.aop, cur.v, r, hsKindType(k))
	}
	cur.write(f, nv)
}

// incDec is lhs++ or lhs--.
func (f *hfn) incDec(s lstep) {
	lt := s.lhs.Type()
	cur := f.readLval(s.lhs)
	var nv hv
	switch {
	case isPtrish(lt):
		d := elemSize(lt)
		if !s.inc {
			d = -d
		}
		nv = hv{binds: cur.v.binds, val: fmt.Sprintf("(pAdd %s %s)", cur.v.val, hsOff(int(d))), ht: "P"}
	case f.h.hsType(lt) == "Bool":
		// ++ on a bool is true; -- is its negation
		if s.inc {
			nv = hv{binds: cur.v.binds, val: "True", ht: "Bool"}
		} else {
			nv = hv{binds: cur.v.binds, val: "(not " + cur.v.val + ")", ht: "Bool"}
		}
	default:
		ht := f.h.hsType(lt)
		op := "+"
		if !s.inc {
			op = "-"
		}
		nv = hv{binds: cur.v.binds, val: fmt.Sprintf("(%s %s 1)", cur.v.val, op), ht: ht}
	}
	cur.write(f, nv)
}

// hlv is an lvalue read, and how it is written back: a binding, or memory
// at an address the read has already computed.
type hlv struct {
	v    hv
	reg  *lvar
	base string
	a    haddr
	t    cc.Type
}

func (f *hfn) readLval(e cc.ExpressionNode) hlv {
	t := e.Type()
	if v := f.outDeref(e); v != nil {
		return hlv{v: f.varRead(v), reg: v, t: t}
	}
	if m := f.memberVar(e); m != nil {
		return hlv{v: f.varRead(m), reg: m, t: t}
	}
	if v := f.lf.lhsVar(e); v != nil && f.reg(v) {
		return hlv{v: f.varRead(v), reg: v, t: t}
	}
	a := f.addrOf(e)
	base := f.flush(a.base)
	if a.field != nil && a.field.IsBitfield() {
		f.no(e, "a bit field")
	}
	ht := f.h.hsType(t)
	r := f.tmp()
	if a.whole && a.obj != nil {
		f.emit("%s <- %s ed'", r, a.obj.name)
	} else {
		f.emit("%s <- rd%s %s %s", r, hsAccess(ht), base, offStr(a))
	}
	return hlv{v: hv{val: r, ht: ht}, base: base, a: a, t: t}
}

func (l hlv) write(f *hfn, x hv) {
	if l.reg != nil {
		f.setVar(l.reg, x)
		return
	}
	ht := f.h.hsType(l.t)
	val := f.flushPlain(f.conv(x, ht))
	if l.a.whole && l.a.obj != nil {
		f.emit("set'%s ed' %s", l.a.obj.name, val)
		return
	}
	f.emit("wr%s %s %s %s", hsAccess(ht), l.base, offStr(l.a), val)
}

// initVar is a declared local's initializer where C declares it: a braced
// list or a string, written into the variable's memory anew.
func (f *hfn) initVar(s lstep) {
	v := s.dst
	if f.sv[v] != nil {
		f.initStruct(v, s.in)
		return
	}
	off, ok := f.mem[v]
	if !ok {
		// a scalar with braces: its one value
		in := s.in
		for in != nil && in.Case == cc.InitializerInitList {
			if in.InitializerList == nil {
				f.setVar(v, hv{val: hsZero(f.vtype(v)), ht: f.vtype(v)})
				return
			}
			in = in.InitializerList.Initializer
		}
		f.setVar(v, f.expr(in.AssignmentExpression))
		return
	}
	f.emit("fillMem (pAdd fr' %d) 0 %d", off, v.c.Size())
	f.initInto(fmt.Sprintf("(pAdd fr' %d)", off), 0, v.c, s.in)
}

// initInto writes initializer in, of an object of type t at base+off.  The
// C front end gives each initializer its offset in the object.
func (f *hfn) initInto(base string, off int, t cc.Type, in *cc.Initializer) {
	if in == nil {
		return
	}
	if in.Case == cc.InitializerExpr {
		e := in.AssignmentExpression
		at := off + int(in.Offset())
		it := in.Type()
		if it == nil {
			it = t
		}
		img := f.image != nil && base == "(edSeg ed')"
		if sv, ok := unparenE(e).Value().(cc.StringValue); ok && it.Kind() == cc.Array {
			// a char array from a string: its bytes, as many as fit
			n := min(int(it.Size()), len(sv))
			for i := 0; i < n; i++ {
				switch {
				case sv[i] == 0:
				case img:
					f.poke(at+i, 1, uint64(sv[i]))
				default:
					f.emit("wrW8 %s %d %d", base, at+i, sv[i])
				}
			}
			return
		}
		if fl := in.Field(); fl != nil && fl.IsBitfield() {
			f.no(e, "a bit field's initializer")
		}
		if isAggr(it) {
			if x := f.expr(e); x.rec != nil || x.tup != nil {
				if x.tup != nil {
					x = f.materialize(x, it)
					f.emit("copyMem (pAdd %s %d) %s %d", base, at, f.flush(x), it.Size())
					return
				}
				f.storeStruct(haddr{base: hv{val: base, ht: "P"}, off: at}, x.rec)
				return
			}
			src := f.flush(f.expr(e))
			f.emit("copyMem (pAdd %s %d) %s %d", base, at, src, it.Size())
			return
		}
		ht := f.h.hsType(it)
		x := f.expr(e)
		if x.konst && x.kv == 0 && !x.lit {
			return // the memory is zeroed
		}
		if img && x.konst && !x.lit && !isPtrHt(ht) && len(x.binds) == 0 {
			// a constant: its bytes in the image
			k := f.conv(x, ht).kv
			if ht == "Bool" && k != 0 {
				k = 1
			}
			f.poke(at, int(it.Size()), uint64(k))
			return
		}
		val := f.flushPlain(f.conv(x, ht))
		f.emit("wr%s %s %d %s", hsAccess(ht), base, at, val)
		return
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		f.initInto(base, off, t, l.Initializer)
	}
}

// term prints a block's terminator.
func (f *hfn) term(b *lblock, depth int) {
	t := b.term
	switch t.kind {
	case tGoto:
		to := f.resolve(t.to[0])
		if f.inline[to] {
			// straight on: its lines are this block's
			f.lines = append(f.lines, f.body(to, depth+1)...)
			return
		}
		f.emit("%s", f.jump(to))
	case tIf:
		c := f.truth(f.lexpr(t.cond))
		cv := f.flush(c)
		cur := f.cur
		then := f.arm(f.resolve(t.to[0]), depth, cur)
		els := f.arm(f.resolve(t.to[1]), depth, cur)
		f.emit("if %s", hsUnparen(cv))
		f.emit("%s", indent(armed("then", then), 2))
		f.emit("%s", indent(armed("else", els), 2))
	case tSwitch:
		x := f.lexpr(t.cond)
		ht := x.ht
		if ht == "Bool" {
			x = f.conv(x, "Int32")
			ht = "Int32"
		}
		v := f.flush(x)
		cur := f.cur
		f.emit("case (%s :: %s) of", v, ht)
		for i, vs := range t.cases {
			for _, cv := range vs {
				f.emit("%s", indent(armed(f.h.hsLabel(cv, ht, t.labels[cv])+" ->", f.arm(f.resolve(t.to[i]), depth, cur)), 2))
			}
		}
		f.emit("%s", indent(armed("_ ->", f.arm(f.resolve(t.to[len(t.to)-1]), depth, cur)), 2))
	case tRet:
		switch {
		case t.ret.isZero():
			f.result(hsZero(f.ret))
		case f.tuple:
			x := f.lexpr(t.ret)
			f.result("(" + strings.Join(f.structVals(x, f.lf.ft.Result()), ", ") + ")")
		case f.sret:
			if x := f.lexpr(t.ret); x.rec != nil || x.tup != nil {
				if x.tup != nil {
					x = f.materialize(x, f.lf.ft.Result())
					f.emit("copyMem sret' %s %d", f.flush(x), f.lf.ft.Result().Size())
					f.emit("pure ()")
					return
				}
				f.storeStruct(haddr{base: hv{val: "sret'", ht: "P"}}, x.rec)
				f.emit("pure ()")
				return
			}
			src := f.flush(f.lexpr(t.ret))
			f.emit("copyMem sret' %s %d", src, f.lf.ft.Result().Size())
			f.emit("pure ()")
		default:
			v := f.flushPlain(f.conv(f.lexpr(t.ret), f.ret))
			f.result(v)
		}
	case tFall:
		if f.sret {
			f.emit("pure ()")
		} else {
			f.result(hsZero(f.ret))
		}
	}
}

// result is the function's result v: its value, in IO or not.
func (f *hfn) result(v string) {
	if len(f.outs) > 0 {
		// the out-parameters' values beside the result
		var parts []string
		if f.ret != "()" {
			parts = append(parts, v)
		}
		for _, o := range f.outs {
			parts = append(parts, f.valueOf(f.cur, o))
		}
		v = strings.Join(parts, ", ")
		if len(parts) > 1 {
			v = "(" + v + ")"
		}
	}
	if f.pure && len(f.outs) > 0 {
		f.emit("%s", v) // a tuple
		return
	}
	if f.pure {
		f.emit("%s", hsUnparen(v))
		return
	}
	f.emit("pure %s", v)
}

// hsPattern is a case value as a pattern of type ht.
func hsPattern(v int64, ht string) string {
	if strings.HasPrefix(ht, "Word") {
		return fmt.Sprint(uint64(v) & hsMask(ht))
	}
	return fmt.Sprint(v)
}

// hsMask is the bits of an unsigned type.
func hsMask(ht string) uint64 {
	switch ht {
	case "Word8":
		return 0xff
	case "Word16":
		return 0xffff
	case "Word32":
		return 0xffffffff
	}
	return ^uint64(0)
}

// elemSize is the size of what a pointer or array points at: 1 for void,
// as gcc has it.
func elemSize(t cc.Type) int64 {
	e := elemOf(t)
	if e == nil || e.Kind() == cc.Void || e.Kind() == cc.Function {
		return 1
	}
	return e.Size()
}

func (f *hfn) no(n cc.Node, format string, args ...any) {
	where := ""
	if n != nil {
		where = fmt.Sprintf(" at %d", n.Position().Line)
	}
	panic(unsupported{fmt.Sprintf(format, args...) + where})
}

func (f *hfn) tmp() string {
	for {
		f.ntmp++
		n := fmt.Sprintf("r'%d", f.ntmp)
		if !f.used[n] {
			return n
		}
	}
}

// hsOff is a byte offset as an argument: a negative one in parentheses.
func hsOff(n int) string {
	if n < 0 {
		return fmt.Sprintf("(%d)", n)
	}
	return fmt.Sprint(n)
}

// hsMarkRe is a runtime body's question about the C's layout: {{sizeof T}}
// or {{offsetof T m}}, T a typedef's name.
var hsMarkRe = regexp.MustCompile(`\{\{(sizeof|offsetof) ([A-Za-z_][A-Za-z_0-9]*)(?: ([A-Za-z_][A-Za-z_0-9]*))?\}\}`)

// layoutMarks answers a runtime body's questions about the C's layout from
// the front end's: a body is written once, and the sizes are the unit's.
func (h *hgen) layoutMarks(body string) string { return answerLayout(h.g.ast, body) }

// layoutMarks answers a runtime body's questions about the C's layout.
func (r *rgen) layoutMarks(body string) string { return answerLayout(r.g.ast, body) }

// answerLayout answers a body's {{sizeof T}} and {{offsetof T m}} from the
// front end's layout of ast.
func answerLayout(ast *cc.AST, body string) string {
	return hsMarkRe.ReplaceAllStringFunc(body, func(m string) string {
		g := hsMarkRe.FindStringSubmatch(m)
		var t cc.Type
		for _, n := range ast.Scope.Nodes[g[2]] {
			if d, ok := n.(*cc.Declarator); ok && d.IsTypename() {
				t = d.Type()
			}
		}
		if t == nil {
			panic(unsupported{"a runtime body's " + m + ": no type " + g[2]})
		}
		if g[1] == "sizeof" {
			return fmt.Sprint(t.Size())
		}
		var st *cc.StructType
		switch x := t.(type) {
		case *cc.StructType:
			st = x
		}
		if st == nil {
			panic(unsupported{"a runtime body's " + m + ": " + g[2] + " is not a struct"})
		}
		fl := st.FieldByName(g[3])
		if fl == nil {
			panic(unsupported{"a runtime body's " + m + ": no member " + g[3]})
		}
		return fmt.Sprint(fl.Offset())
	})
}
