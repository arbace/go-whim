package togo

// scm_expr.go is an expression of the lowered form printed as Scheme: the
// bindings that must come first -- each memory read and each call a
// binding of its own, in C's order (the Java's), since Scheme leaves the
// order of a call's arguments open -- and then a value with nothing left to
// do but compute.  An operand of &&, || or ?: that reads or calls is
// evaluated only where C evaluates it: its bindings go inside the and, the
// or, the if.  A struct's value is its address; an array's, the address of
// its first element.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// sx is a value: the lines before it, and the Scheme expression, of kind
// st (i8 to u64, bool, ptr, agg for a struct or union's address, void),
// and what evaluating the expression does (lvl): nothing, reads memory,
// or calls.
type sx struct {
	binds []sbind
	val   string
	st    string
	lvl   int
	konst bool     // an integer constant, kv
	kv    int64    // its value
	spell string   // its C name, when it has one and is it
	lit   bool     // a string literal's address
	rec   *lvar    // a struct that is a value: its members are its value
	tup   []string // a struct result returned as values: its members' names
}

// What evaluating an expression does: nothing, reads of memory -- which
// may be evaluated in any order among themselves -- or a call, which may
// do anything.
const (
	scmPure = iota
	scmReads
	scmCalls
)

// seq readies the operands of one combination, in C's order: their lines
// come first, in order, and an operand that reads or calls stays where it
// is used only where Scheme, which evaluates a combination's operands in
// an order of its own, cannot reorder it against another's -- reads
// commute with reads, nothing else does; the rest are bound to
// temporaries first, in C's order.  It returns the lines, and leaves each
// operand its expression.
func (f *sfn) seq(ops ...*sx) []sbind {
	var out []sbind
	var inline []int // operands left where they are that read or call
	bindOp := func(i int) {
		r := f.tmp()
		out = append(out, sbind{names: []string{r}, expr: ops[i].val, lvl: ops[i].lvl})
		ops[i].val, ops[i].lvl = r, scmPure
	}
	for i, op := range ops {
		if len(op.binds) > 0 {
			// its lines run before the combination: what is left in place
			// before it, and what names a variable its lines bind anew, is
			// bound before them
			rebound := scmRebinds(op.binds)
			var keep []int
			for _, j := range inline {
				bindOp(j)
			}
			for j := 0; j < i; j++ {
				if ops[j].lvl == scmPure && len(rebound) > 0 && scmMentions(ops[j].val, rebound) {
					bindOp(j)
				}
			}
			inline = keep
		}
		out = append(out, op.binds...)
		op.binds = nil
		if op.lvl > scmPure {
			inline = append(inline, i)
		}
	}
	// a call in place: no other operand in place before it, and none that
	// reads after it
	last := -1
	for k, i := range inline {
		if ops[i].lvl == scmCalls {
			last = k
		}
	}
	if last >= 0 && len(inline) > 1 {
		upto := last
		if last == len(inline)-1 {
			upto = last - 1
		}
		for k := 0; k <= upto; k++ {
			bindOp(inline[k])
		}
	}
	return out
}

// scmRebinds are the names lines bind that are not the printer's
// temporaries: variables of the C, given a new value.
func scmRebinds(bs []sbind) map[string]bool {
	m := map[string]bool{}
	for _, b := range bs {
		for _, n := range b.names {
			if !scmTemp(n) {
				m[n] = true
			}
		}
	}
	return m
}

// scmTemp says n is a temporary of the printer's: r and a number.
func scmTemp(n string) bool {
	if len(n) < 2 || n[0] != 'r' {
		return false
	}
	for _, c := range n[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// scmMentions says the expression e names one of names.
func scmMentions(e string, names map[string]bool) bool {
	for _, t := range strings.FieldsFunc(e, func(r rune) bool { return r == ' ' || r == '(' || r == ')' || r == '\n' || r == '[' || r == ']' }) {
		if names[t] {
			return true
		}
	}
	return false
}

// bindsLvl is what lines do, the most of them.
func bindsLvl(bs []sbind) int {
	l := scmPure
	for _, b := range bs {
		l = max(l, b.lvl)
	}
	return l
}

// saddr is an lvalue's address: a base address and a constant offset;
// and how the C names it, where it does: a whole scalar object or local
// (whole: read by its name, written by set!), or a member of the struct at
// base (root, the struct's name, and path, the members' from it to off: a
// record's accessor, root.path).
type saddr struct {
	base  sx
	off   int
	field *cc.Field
	whole string
	root  string
	path  []string
}

// member is a with member fl of the struct or union ct added.
func (f *sfn) member(a saddr, ct cc.Type, fl *cc.Field) saddr {
	if ct != nil && (ct.Kind() == cc.Ptr || ct.Kind() == cc.Array) {
		ct = elemOf(ct)
	}
	switch {
	case fl.Name() == "":
	case a.root != "":
		return saddr{base: a.base, off: a.off + int(fl.Offset()), field: fl, root: a.root, path: append(append([]string{}, a.path...), fl.Name())}
	default:
		if root := f.s.structName(ct); root != "" {
			return saddr{base: f.addrVal(a), off: int(fl.Offset()), field: fl, root: root, path: []string{fl.Name()}}
		}
	}
	a.off += int(fl.Offset())
	a.field, a.whole, a.root, a.path = fl, "", "", nil
	return a
}

// accessor is a's member's accessor, defined for kind st (agg: its
// address alone), or "" when it has none.
func (f *sfn) accessor(a saddr, st string) string {
	if a.root == "" {
		return ""
	}
	return f.s.useMember(a.root+"."+strings.Join(a.path, "."), st, a.off)
}

// addrString is a's address as an expression.
func (f *sfn) addrString(a saddr) string {
	if n := f.accessor(a, "agg"); n != "" {
		return "(" + n + "& " + a.base.val + ")"
	}
	return scmAddr(a.base.val, a.off)
}

// loadString is the read of a scalar of kind st at a.
func (f *sfn) loadString(a saddr, st string) string {
	f.memory = true
	if a.whole != "" && a.off == 0 {
		return a.whole
	}
	if n := f.accessor(a, st); n != "" {
		return "(" + n + " " + a.base.val + ")"
	}
	return "(" + scmLoad(st) + " " + scmAddr(a.base.val, a.off) + ")"
}

// storeString is the write of val, of kind st, at a.
func (f *sfn) storeString(a saddr, st, val string) string {
	f.memory = true
	if a.whole != "" && a.off == 0 {
		return "(set! " + a.whole + " " + val + ")"
	}
	if n := f.accessor(a, st); n != "" {
		return "(" + n + "-set! " + a.base.val + " " + val + ")"
	}
	return "(" + scmStore(st) + " " + scmAddr(a.base.val, a.off) + " " + val + ")"
}

// localAddr is the address of v, a variable in the frame: by its name.
func (f *sfn) localAddr(v *lvar) saddr {
	f.locals[v] = true
	if isAggr(v.c) || v.c.Kind() == cc.Array {
		return saddr{base: sx{val: f.base[v], st: "ptr"}}
	}
	return saddr{base: sx{val: "&" + f.base[v], st: "ptr"}, whole: f.base[v]}
}

// scmAddr is base plus off, folded where both are numbers.
func scmAddr(base string, off int) string {
	if off == 0 {
		return base
	}
	if scmConst(base) {
		b, _ := strconv.ParseInt(base, 10, 64)
		return strconv.FormatInt(b+int64(off), 10)
	}
	return fmt.Sprintf("(fx+ %s %d)", base, off)
}

// scmAdd is p plus the offset expression off.
func scmAdd(p, off string) string {
	if off == "0" {
		return p
	}
	if scmConst(p) && scmConst(off) {
		a, _ := strconv.ParseInt(p, 10, 64)
		b, _ := strconv.ParseInt(off, 10, 64)
		return strconv.FormatInt(a+b, 10)
	}
	return "(fx+ " + p + " " + off + ")"
}

func (f *sfn) lexpr(r lexpr) sx {
	if r.v != nil {
		return f.varRead(r.v)
	}
	if r.raw {
		return f.node(r.n)
	}
	return f.expr(r.n)
}

// expr is e's value, read through the function's substitutions.
func (f *sfn) expr(e cc.ExpressionNode) sx {
	if f.lf != nil {
		if r, ok := f.lf.sub[e]; ok {
			return f.lexpr(r)
		}
	}
	return f.node(e)
}

func (f *sfn) hasSub(e cc.ExpressionNode) bool {
	if f.lf == nil || len(f.lf.sub) == 0 {
		return false
	}
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		if x, ok := n.(cc.ExpressionNode); ok {
			if _, ok := f.lf.sub[x]; ok {
				found = true
				return
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(e)
	return found
}

// node is e itself, its operands through the substitutions; an integer
// constant is its value.
func (f *sfn) node(e cc.ExpressionNode) sx {
	if isNullConst(e) && e.Type() != nil && e.Type().Kind() == cc.Ptr {
		return sx{val: "0", st: "ptr", konst: true}
	}
	if k, ok := scalarKind(e.Type()); ok && !hasEffect(e) && !f.hasSub(e) {
		if v, known := intValue(e.Value()); known {
			st := scmKindType(k)
			t := scmTrunc(v, st)
			if sp := f.s.constSpelling(e, scmLit(t, st)); sp != "" && t == v && st != "bool" {
				return sx{val: sp, st: st, konst: true, kv: t, spell: sp}
			}
			return sx{val: scmLit(t, st), st: st, konst: true, kv: t}
		}
	}
	switch x := e.(type) {
	case *cc.ConstantExpression:
		return f.expr(x.ConditionalExpression)
	case *cc.ExpressionList:
		if x.ExpressionList != nil {
			f.no(x, "a comma the lowering did not take apart")
		}
		return f.expr(x.AssignmentExpression)
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			return f.ident(x)
		case cc.PrimaryExpressionExpr:
			return f.expr(x.ExpressionList)
		case cc.PrimaryExpressionString:
			sv, ok := x.Value().(cc.StringValue)
			if !ok {
				f.no(x, "a wide string")
			}
			addr := f.s.literal(string(sv))
			return sx{val: fmt.Sprintf("(c-str %d %s)", addr, scmString([]byte(strings.TrimSuffix(string(sv), "\x00")))), st: "ptr", lit: true, konst: true, kv: int64(addr)}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionCall:
			return f.call(x)
		case cc.PostfixExpressionComplit:
			f.no(x, "a compound literal the lowering did not take apart")
		case cc.PostfixExpressionIndex:
			be := x.PostfixExpression
			if !isPtrish(be.Type()) {
				be = x.ExpressionList
			}
			t := x.Type()
			if et := elemOf(be.Type()); et != nil {
				t = et
			}
			return f.readAt(f.addrNode(x), t)
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			if m := f.memberVar(x); m != nil {
				return f.varRead(m)
			}
			t := x.Type()
			if fl := x.Field(); fl != nil && fl.Type() != nil {
				t = fl.Type()
			}
			return f.readAt(f.addrNode(x), t)
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionAddrof:
			if d := fnDesignator(x.CastExpression); d != nil {
				return f.fnValue(d)
			}
			return f.addrVal(f.addrOf(x.CastExpression))
		case cc.UnaryExpressionDeref:
			if v := f.outNamed(x.CastExpression); v != nil {
				return f.varRead(v)
			}
			if et := elemOf(x.CastExpression.Type()); et != nil && et.Kind() == cc.Function || x.Type() != nil && x.Type().Kind() == cc.Function {
				return f.expr(x.CastExpression) // *fp is fp
			}
			return f.readAt(f.addrNode(x), x.Type())
		case cc.UnaryExpressionMinus:
			st := scmTypeOf(x.Type())
			a := f.conv(f.expr(x.CastExpression), st)
			return sx{binds: a.binds, val: scmNeg(st, a.val), st: st, lvl: a.lvl}
		case cc.UnaryExpressionCpl:
			st := scmTypeOf(x.Type())
			a := f.conv(f.expr(x.CastExpression), st)
			var s string
			switch st {
			case "i32":
				s = "(fxnot " + a.val + ")"
			case "u32":
				s = "(u32~ " + a.val + ")"
			case "i64":
				s = "(bitwise-not " + a.val + ")"
			default:
				s = "(u64~ " + a.val + ")"
			}
			return sx{binds: a.binds, val: s, st: st, lvl: a.lvl}
		case cc.UnaryExpressionNot:
			a := f.truth(f.expr(x.CastExpression))
			return sx{binds: a.binds, val: scmNegate(a.val), st: "bool", lvl: a.lvl}
		case cc.UnaryExpressionPlus:
			return f.conv(f.expr(x.CastExpression), scmTypeOf(x.Type()))
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			v := f.expr(x.CastExpression)
			t := x.Type()
			switch {
			case t.Kind() == cc.Void:
				return sx{binds: v.binds, val: v.val, st: "void", lvl: v.lvl}
			case isAggr(t):
				return v
			}
			return f.conv(v, scmTypeOf(t))
		}
	case *cc.MultiplicativeExpression:
		ops := map[cc.MultiplicativeExpressionCase]string{cc.MultiplicativeExpressionMul: "*", cc.MultiplicativeExpressionDiv: "/", cc.MultiplicativeExpressionMod: "%"}
		return f.arith(ops[x.Case], f.expr(x.MultiplicativeExpression), f.expr(x.CastExpression), scmTypeOf(x.Type()))
	case *cc.AdditiveExpression:
		op := "+"
		if x.Case == cc.AdditiveExpressionSub {
			op = "-"
		}
		return f.additive(x, op, x.AdditiveExpression, x.MultiplicativeExpression)
	case *cc.ShiftExpression:
		op := "<<"
		if x.Case == cc.ShiftExpressionRsh {
			op = ">>"
		}
		return f.arith(op, f.expr(x.ShiftExpression), f.expr(x.AdditiveExpression), scmTypeOf(x.Type()))
	case *cc.AndExpression:
		return f.arith("&", f.expr(x.AndExpression), f.expr(x.EqualityExpression), scmTypeOf(x.Type()))
	case *cc.ExclusiveOrExpression:
		return f.arith("^", f.expr(x.ExclusiveOrExpression), f.expr(x.AndExpression), scmTypeOf(x.Type()))
	case *cc.InclusiveOrExpression:
		return f.arith("|", f.expr(x.InclusiveOrExpression), f.expr(x.ExclusiveOrExpression), scmTypeOf(x.Type()))
	case *cc.RelationalExpression:
		ops := map[cc.RelationalExpressionCase]string{cc.RelationalExpressionLt: "<", cc.RelationalExpressionGt: ">", cc.RelationalExpressionLeq: "<=", cc.RelationalExpressionGeq: ">="}
		return f.compare(ops[x.Case], x.RelationalExpression, x.ShiftExpression)
	case *cc.EqualityExpression:
		op := "=="
		if x.Case == cc.EqualityExpressionNeq {
			op = "!="
		}
		return f.compare(op, x.EqualityExpression, x.RelationalExpression)
	case *cc.LogicalAndExpression:
		return f.logical(true, x.LogicalAndExpression, x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		return f.logical(false, x.LogicalOrExpression, x.LogicalAndExpression)
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return f.ternary(x)
		}
	case *cc.AssignmentExpression:
		f.no(x, "an assignment the lowering did not take apart")
	}
	f.no(e, "an expression %T", e)
	return sx{}
}

// fnValue is a function's address: its index in the table.
func (f *sfn) fnValue(d *cc.Declarator) sx {
	i := f.s.fnPtrOf(d.Name())
	return sx{val: fmt.Sprintf("(fn-ptr %d)", i), st: "ptr"}
}

// ident is a name's value.
func (f *sfn) ident(x *cc.PrimaryExpression) sx {
	switch d := x.ResolvedTo().(type) {
	case *cc.Declarator:
		if d.Type() != nil && d.Type().Kind() == cc.Function {
			return f.fnValue(d)
		}
		if f.lf != nil {
			if v, ok := f.lf.byDecl[d]; ok {
				if f.isOut[v] {
					// an out-parameter itself, asked whether it is null: every
					// caller passes a local's address
					return sx{val: "1", st: "ptr", konst: true, kv: 1}
				}
				return f.varRead(v)
			}
		}
		a, ok := f.objAddr(f.s.g.a.declKey(d))
		if !ok {
			f.no(x, "an object with no place: %s", d.Name())
		}
		return f.readAt(a, d.Type())
	}
	f.no(x, "a name %s", x.Token.SrcStr())
	return sx{}
}

// objAddr is the address of the file-scope object of key, by its name: a
// constant.
func (f *sfn) objAddr(key string) (saddr, bool) {
	o, ok := f.s.useObject(key)
	if !ok {
		return saddr{}, false
	}
	base := sx{val: "&" + o.name, st: "ptr", konst: true, kv: int64(o.addr)}
	if o.kind == "agg" {
		base.val = o.name
		return saddr{base: base}, true
	}
	return saddr{base: base, whole: o.name}, true
}

// varRead is a variable's value: its binding, or its memory in the frame.
func (f *sfn) varRead(v *lvar) sx {
	if f.sv[v] != nil {
		return sx{st: "agg", rec: v}
	}
	if _, ok := f.mem[v]; ok {
		return f.readAt(f.localAddr(v), v.c)
	}
	val := f.valueOf(v)
	st := f.vtype(v)
	x := sx{val: val, st: st}
	if scmConst(val) {
		x.konst = true
		if val == "#t" {
			x.kv = 1
		} else if val != "#f" {
			x.kv, _ = strconv.ParseInt(val, 10, 64)
		}
	}
	return x
}

// addrVal is an address as a value.
func (f *sfn) addrVal(a saddr) sx {
	val := f.addrString(a)
	x := sx{binds: a.base.binds, val: val, st: "ptr", lvl: a.base.lvl}
	if a.base.konst && a.root == "" {
		x.konst, x.kv = true, a.base.kv+int64(a.off)
	}
	if scmConst(val) {
		x.konst = true
		x.kv, _ = strconv.ParseInt(val, 10, 64)
	}
	return x
}

// readAt is the value of the object of type t at a: a scalar read, or the
// address of an array, a struct or a union.
func (f *sfn) readAt(a saddr, t cc.Type) sx {
	switch {
	case t.Kind() == cc.Array || t.Kind() == cc.Function:
		return f.addrVal(a)
	case isAggr(t):
		v := f.addrVal(a)
		v.st = "agg"
		return v
	}
	if a.field != nil && a.field.IsBitfield() {
		f.no(nil, "a bit field")
	}
	st := scmTypeOf(t)
	return sx{binds: a.base.binds, val: f.loadString(a, st), st: st, lvl: max(scmReads, a.base.lvl)}
}

// addrOf is an lvalue's address.
func (f *sfn) addrOf(e cc.ExpressionNode) saddr {
	if f.lf != nil {
		if r, ok := f.lf.sub[e]; ok {
			if r.v != nil {
				if _, ok := f.mem[r.v]; !ok {
					f.no(e, "the address of a variable not in memory")
				}
				return f.localAddr(r.v)
			}
			if !r.raw {
				return f.addrOf(r.n)
			}
		}
	}
	return f.addrNode(e)
}

// addrNode is addrOf of e itself, its operands through the substitutions.
func (f *sfn) addrNode(e cc.ExpressionNode) saddr {
	switch x := e.(type) {
	case *cc.ExpressionList:
		if x.ExpressionList == nil {
			return f.addrOf(x.AssignmentExpression)
		}
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionExpr:
			return f.addrOf(x.ExpressionList)
		case cc.PrimaryExpressionString:
			return saddr{base: f.node(x)}
		case cc.PrimaryExpressionIdent:
			d, ok := x.ResolvedTo().(*cc.Declarator)
			if !ok {
				break
			}
			if f.lf != nil {
				if v, ok := f.lf.byDecl[d]; ok {
					if _, inMem := f.mem[v]; !inMem {
						f.no(x, "the address of %s, which is not in memory", d.Name())
					}
					return f.localAddr(v)
				}
			}
			a, ok := f.objAddr(f.s.g.a.declKey(d))
			if !ok {
				f.no(x, "an object with no place: %s", d.Name())
			}
			return a
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect:
			return f.member(f.addrOf(x.PostfixExpression), x.PostfixExpression.Type(), x.Field())
		case cc.PostfixExpressionPSelect:
			return f.member(saddr{base: f.expr(x.PostfixExpression)}, x.PostfixExpression.Type(), x.Field())
		case cc.PostfixExpressionIndex:
			be, ie := x.PostfixExpression, x.ExpressionList
			if !isPtrish(be.Type()) {
				be, ie = ie, be
			}
			base := f.expr(be)
			size := elemSize(be.Type())
			i := f.expr(ie)
			if i.konst {
				return saddr{base: base, off: int(i.kv * size)}
			}
			var binds []sbind
			if be == x.PostfixExpression {
				binds = f.seq(&base, &i)
			} else {
				binds = f.seq(&i, &base)
			}
			return saddr{base: sx{binds: binds, val: scmAdd(base.val, f.byteOff(i, size, false)), st: "ptr", lvl: max(base.lvl, i.lvl)}}
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			return saddr{base: f.expr(x.CastExpression)}
		}
	}
	if isAggr(e.Type()) {
		return saddr{base: f.materialize(f.expr(e), e.Type())}
	}
	f.no(e, "the address of %T", e)
	return saddr{}
}

// conv is v converted to kind to, as C converts.
func (f *sfn) conv(v sx, to string) sx {
	from := v.st
	if from == to || to == "agg" || to == "void" || from == "agg" || from == "void" {
		return v
	}
	if v.konst && !v.lit {
		k := v.kv
		switch {
		case to == "bool":
			k = b2i64(k != 0)
		case to == "ptr":
		default:
			k = scmTrunc(k, to)
		}
		if v.spell != "" && k == v.kv && to != "bool" && to != "ptr" {
			// the C's name for it, while it is its value
			return sx{binds: v.binds, val: v.spell, st: to, konst: true, kv: k, spell: v.spell}
		}
		return sx{binds: v.binds, val: scmLit(k, to), st: to, konst: true, kv: k}
	}
	out := sx{binds: v.binds, st: to, lvl: v.lvl}
	switch {
	case from == "bool":
		out.val = "(b->i " + v.val + ")"
	case to == "bool" && (from == "ptr" || !scmWide(from)):
		out.val = "(not (fxzero? " + v.val + "))"
	case to == "bool":
		out.val = "(not (eqv? " + v.val + " 0))"
	case from == "ptr" && scmWide(to):
		out.val = v.val
	case from == "ptr":
		out.val = "(->" + to + " " + v.val + ")"
	case to == "ptr" && from == "u64":
		out.val = "(->i64 " + v.val + ")"
	case to == "ptr":
		out.val = v.val
		out.lit, out.konst, out.kv = v.lit, v.konst, v.kv
	case scmKeeps(from, to):
		out.val = v.val
	default:
		out.val = "(->" + to + " " + v.val + ")"
	}
	return out
}

// scmKeeps says converting an integer of kind a to kind b keeps every value.
func scmKeeps(a, b string) bool {
	sa, siga, ok1 := scmBits(a)
	sb, sigb, ok2 := scmBits(b)
	if !ok1 || !ok2 {
		return false
	}
	if siga {
		return sigb && sb >= sa
	}
	return !sigb && sb >= sa || sigb && sb > sa
}

// toInt is an integer value as an offset's fixnum: an unsigned long's bits
// as a signed long, C's conversion to ptrdiff_t.
func (f *sfn) toInt(v sx) string {
	switch {
	case v.konst:
		return strconv.FormatInt(v.kv, 10)
	case v.st == "bool":
		return "(b->i " + v.val + ")"
	case v.st == "u64":
		return "(->i64 " + v.val + ")"
	}
	return v.val
}

// byteOff is n elements of size bytes, negated when neg, as an offset.
func (f *sfn) byteOff(n sx, size int64, neg bool) string {
	if n.konst {
		k := n.kv * size
		if neg {
			k = -k
		}
		return strconv.FormatInt(k, 10)
	}
	s := f.toInt(n)
	if size != 1 {
		s = fmt.Sprintf("(fx* %s %d)", s, size)
	}
	if neg {
		s = "(fx- 0 " + s + ")"
	}
	return s
}

// truth is v as a condition.
func (f *sfn) truth(v sx) sx { return f.conv(v, "bool") }

// arith is a op b in kind st: C's arithmetic on the operands' usual type,
// which the expression's is.
func (f *sfn) arith(op string, a, b sx, st string) sx {
	a = f.conv(a, st)
	shift := op == "<<" || op == ">>"
	if !shift {
		b = f.conv(b, st)
	}
	binds := f.seq(&a, &b)
	bv := b.val
	if shift {
		bv = f.toInt(b)
	}
	var s string
	switch op {
	case "+", "-", "*":
		s = "(" + scmSignedOp(st, op) + " " + a.val + " " + bv + ")"
	case "/", "%", "<<", ">>":
		s = "(" + st + op + " " + a.val + " " + bv + ")"
	case "&", "|", "^":
		fn := map[string]string{"&": "fxand", "|": "fxior", "^": "fxxor"}[op]
		if scmWide(st) {
			fn = map[string]string{"&": "bitwise-and", "|": "bitwise-ior", "^": "bitwise-xor"}[op]
		}
		s = "(" + fn + " " + a.val + " " + bv + ")"
	default:
		f.no(nil, "an operator %s", op)
	}
	return sx{binds: binds, val: s, st: st, lvl: max(a.lvl, b.lvl)}
}

// scmSignedOp is the operator of + - or * in kind st. Signed arithmetic
// is C's as C means it (doc/SCHEME-IDIOMS.md, item 15): an overflow is
// undefined, so an int's is fixnum arithmetic, whose operands' range
// keeps it a fixnum, and a long's Scheme's own; unsigned arithmetic wraps,
// as C defines it, through the runtime's u32+ and u64+.
func scmSignedOp(st, op string) string {
	switch st {
	case "i32":
		return "fx" + op
	case "i64":
		return op
	}
	return st + op
}

// scmNeg is -v in kind st.
func scmNeg(st, v string) string {
	switch st {
	case "i32":
		return "(fx- " + v + ")"
	case "i64":
		return "(- " + v + ")"
	}
	return "(" + st + "- 0 " + v + ")"
}

// additive is + or -: of a pointer and an integer, of two pointers, or of
// two numbers.
func (f *sfn) additive(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) sx {
	lt, rt := le.Type(), re.Type()
	switch {
	case isPtrish(lt) && isPtrish(rt):
		a, b := f.expr(le), f.expr(re)
		binds := f.seq(&a, &b)
		d := "(fx- " + a.val + " " + b.val + ")"
		if sz := elemSize(lt); sz != 1 {
			d = fmt.Sprintf("(fxquotient %s %d)", d, sz)
		}
		return f.conv(sx{binds: binds, val: d, st: "i64", lvl: max(a.lvl, b.lvl)}, scmTypeOf(x.Type()))
	case isPtrish(lt) || isPtrish(rt):
		pe, ie := le, re
		if isPtrish(rt) {
			pe, ie = re, le
		}
		p := f.expr(pe)
		n := f.expr(ie)
		size := elemSize(pe.Type())
		var binds []sbind
		if pe == le {
			binds = f.seq(&p, &n)
		} else {
			binds = f.seq(&n, &p)
		}
		return sx{binds: binds, val: scmAdd(p.val, f.byteOff(n, size, op == "-")), st: "ptr", lvl: max(p.lvl, n.lvl)}
	}
	return f.arith(op, f.expr(le), f.expr(re), scmTypeOf(x.Type()))
}

// compare is a comparison: of addresses, or of numbers in their usual type.
func (f *sfn) compare(op string, le, re cc.ExpressionNode) sx {
	a, b := f.expr(le), f.expr(re)
	if op == "==" || op == "!=" {
		// a truth value against 0 or 1 (FALSE, TRUE, OK, FAIL): itself
		// or its negation
		x, k := a, b
		if b.st == "bool" && !a.lit && a.konst {
			x, k = b, a
		}
		if x.st == "bool" && k.konst && !k.lit && k.st != "ptr" && (k.kv == 0 || k.kv == 1) && len(k.binds) == 0 {
			v := x.val
			if (op == "==") != (k.kv == 1) {
				v = scmNegate(v)
			}
			return sx{binds: x.binds, val: v, st: "bool", lvl: x.lvl}
		}
	}
	var st string
	if isPtrish(le.Type()) || isPtrish(re.Type()) || le.Type().Kind() == cc.Function || re.Type().Kind() == cc.Function {
		st = "ptr"
	} else {
		lk, ok1 := scalarKind(le.Type())
		rk, ok2 := scalarKind(re.Type())
		if !ok1 || !ok2 {
			f.no(le, "a comparison of %s and %s", le.Type(), re.Type())
		}
		st = scmKindType(usualK(lk, rk))
	}
	a, b = f.conv(a, st), f.conv(b, st)
	binds := f.seq(&a, &b)
	if st == "ptr" && a.konst && b.konst && !a.lit && !b.lit && (op == "==" || op == "!=") {
		eq := a.kv == b.kv
		if op == "!=" {
			eq = !eq
		}
		return sx{binds: binds, val: scmLit(b2i64(eq), "bool"), st: "bool", konst: true, kv: b2i64(eq)}
	}
	var s string
	switch {
	case op == "==" && scmWide(st):
		s = "(= " + a.val + " " + b.val + ")"
	case op == "==":
		s = "(fx=? " + a.val + " " + b.val + ")"
	case op == "!=" && scmWide(st):
		s = "(not (= " + a.val + " " + b.val + "))"
	case op == "!=":
		s = "(not (fx=? " + a.val + " " + b.val + "))"
	case scmWide(st):
		s = "(" + op + " " + a.val + " " + b.val + ")"
	default:
		s = "(fx" + op + "? " + a.val + " " + b.val + ")"
	}
	return sx{binds: binds, val: s, st: "bool", lvl: max(a.lvl, b.lvl)}
}

// scmExpr is a value and the bindings before it as one expression.
func scmExpr(binds []sbind, val string) string {
	return scmBegin(scmRender(binds, []string{val}))
}

// logical is && (and) or ||: the right operand, its bindings with it,
// evaluated only where C evaluates it.
func (f *sfn) logical(and bool, le, re cc.ExpressionNode) sx {
	a := f.truth(f.expr(le))
	b := f.truth(f.expr(re))
	kw := "or"
	if and {
		kw = "and"
	}
	lvl := max(a.lvl, b.lvl, bindsLvl(b.binds))
	if len(b.binds) == 0 {
		return sx{binds: a.binds, val: "(" + kw + " " + a.val + " " + b.val + ")", st: "bool", lvl: lvl}
	}
	return sx{binds: a.binds, val: "(" + kw + " " + a.val + "\n" + indent(scmExpr(b.binds, b.val), 2+len(kw)) + ")", st: "bool", lvl: lvl}
}

// ternary is ?:, each arm's bindings with it, evaluated only when it is
// chosen.
func (f *sfn) ternary(x *cc.ConditionalExpression) sx {
	c := f.truth(f.expr(x.LogicalOrExpression))
	st := scmTypeOf(x.Type())
	a := f.conv(f.expr(x.ExpressionList), st)
	b := f.conv(f.expr(x.ConditionalExpression), st)
	if st == "agg" {
		a, b = f.materialize(a, x.Type()), f.materialize(b, x.Type())
	}
	lvl := max(c.lvl, a.lvl, b.lvl, bindsLvl(a.binds), bindsLvl(b.binds))
	return sx{binds: c.binds, val: scmIf(c.val, scmExpr(a.binds, a.val), scmExpr(b.binds, b.val)), st: st, lvl: lvl}
}

// call is a call: of a function the unit defines, of the host's, or through
// a pointer.  A struct result is written into the frame, and its value is
// that address; a struct of scalars comes back as values.
func (f *sfn) call(x *cc.PostfixExpression) sx {
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	var ft *cc.FunctionType
	var head []string
	d := fnDesignator(x.PostfixExpression)
	if d != nil && d.Name() == "__builtin_expect" && len(args) == 2 {
		return f.conv(f.expr(args[0]), scmTypeOf(x.Type()))
	}
	viaPtr := d == nil
	// the operands, in C's order: the pointer called through, the
	// arguments, the variadic ones
	var ops []*sx
	var p sx
	if d != nil {
		ft, _ = d.Type().(*cc.FunctionType)
		head = []string{f.s.names[d.Name()]}
		if f.s.defined[d.Name()] == nil || f.s.takesEd(d.Name()) {
			head = append(head, "ed")
		}
	} else {
		p = f.expr(x.PostfixExpression)
		ops = append(ops, &p)
		t := x.PostfixExpression.Type()
		if pt, ok := t.(*cc.PointerType); ok {
			t = pt.Elem()
		}
		ft, _ = t.(*cc.FunctionType)
	}
	if ft == nil {
		f.no(x, "a call of no function type")
	}
	params := ft.Parameters()
	if len(params) == 1 && (params[0].Type() == nil || params[0].Type().Kind() == cc.Void) {
		params = nil
	}
	var vals, rest []*sx
	outs, hasOuts := f.outCall(x)
	for i, a := range args {
		if o := outs[i]; o != nil {
			v := f.conv(f.varRead(o), f.vtype(o))
			vals = append(vals, &v)
			ops = append(ops, &v)
			continue
		}
		if i < len(params) {
			pt := params[i].Type()
			want := scmTypeOf(pt)
			v := f.conv(f.expr(a), want)
			if v.tup != nil {
				v = f.materialize(v, pt)
			}
			if viaPtr && want == "bool" {
				v.val = "(b->i " + v.val + ")"
			}
			vals = append(vals, &v)
			ops = append(ops, &v)
			continue
		}
		v := f.expr(a)
		if v.tup != nil {
			f.no(a, "a struct passed to a variadic function")
		}
		if v.st == "bool" {
			v.val = "(b->i " + v.val + ")"
		}
		rest = append(rest, &v)
		ops = append(ops, &v)
	}
	binds := f.seq(ops...)
	lvl := scmCalls
	var as []string
	for _, v := range vals {
		as = append(as, v.val)
	}
	if ft.IsVariadic() {
		var rs []string
		for _, v := range rest {
			rs = append(rs, v.val)
		}
		if len(rs) == 0 {
			as = append(as, "'()")
		} else {
			as = append(as, "(list "+strings.Join(rs, " ")+")")
		}
	}
	if viaPtr {
		head = []string{"call-ptr", p.val, "ed"}
	}
	rt := ft.Result()
	st := scmTypeOf(rt)
	if viaPtr {
		if isAggr(rt) {
			f.no(x, "a struct result through a pointer")
		}
		call := "(" + strings.Join(append(head, as...), " ") + ")"
		if st == "bool" {
			call = "(not (eqv? " + call + " 0))"
		}
		return sx{binds: binds, val: call, st: st, lvl: lvl}
	}
	if isAggr(rt) && f.s.facts.tupleRet(d.Name()) {
		var names []string
		stt := rt.(*cc.StructType)
		for i := 0; i < stt.NumFields(); i++ {
			names = append(names, f.tmp())
		}
		line := "(" + strings.Join(append(head, as...), " ") + ")"
		return sx{binds: append(binds, sbind{names: names, expr: line, lvl: lvl}), st: "agg", tup: names}
	}
	if isAggr(rt) {
		off := f.alloc(nil, rt)
		dst := fmt.Sprintf("(fx+ fr %d)", off)
		line := "(" + strings.Join(append(append(append([]string{}, head...), dst), as...), " ") + ")"
		return sx{binds: append(binds, sbind{expr: line, lvl: lvl}), val: dst, st: "agg"}
	}
	line := "(" + strings.Join(append(head, as...), " ") + ")"
	if hasOuts {
		return f.outResult(binds, line, st, d.Name(), outs)
	}
	return sx{binds: binds, val: line, st: st, lvl: lvl}
}

// outResult is a call with out-arguments: its result and the locals' new
// values, bound together, each local its own name again -- or, a local in
// the frame, its slot written.
func (f *sfn) outResult(binds []sbind, line, st string, g string, outs map[int]*lvar) sx {
	var names []string
	r := ""
	if st != "void" {
		r = f.tmp()
		names = append(names, r)
	}
	type upd struct {
		v    *lvar
		name string
	}
	var ups []upd
	for _, i := range f.s.facts.outs[g] {
		o := outs[i]
		n := f.tmp()
		if f.reg(o) {
			n = f.base[o]
			binds = append(binds, f.aliases(n)...)
		}
		names = append(names, n)
		ups = append(ups, upd{o, n})
	}
	binds = append(binds, sbind{names: names, expr: line, lvl: scmCalls})
	for _, u := range ups {
		if f.reg(u.v) {
			f.cur[u.v] = u.name
			continue
		}
		off := f.mem[u.v]
		binds = append(binds, sbind{expr: fmt.Sprintf("(%s (fx+ fr %d) %s)", scmStore(f.vtype(u.v)), off, u.name), lvl: scmCalls})
	}
	if r == "" {
		return sx{binds: binds, val: "(void)", st: "void"}
	}
	return sx{binds: binds, val: r, st: st}
}

// constSpelling is the C's name for the constant expression e of value v
// (its literal), when it has one: an enumerator, (define NAME v) in the
// library, or a character constant, (ch #\a); "" else.
func (s *sgen) constSpelling(e cc.ExpressionNode, v string) string {
	for {
		switch x := e.(type) {
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				return ""
			}
			e = x.AssignmentExpression
			continue
		case *cc.ConstantExpression:
			e = x.ConditionalExpression
			continue
		case *cc.PrimaryExpression:
			switch x.Case {
			case cc.PrimaryExpressionExpr:
				e = x.ExpressionList
				continue
			case cc.PrimaryExpressionIdent:
				en, ok := x.ResolvedTo().(*cc.Enumerator)
				if !ok {
					return ""
				}
				n := scmName(en.Token.SrcStr())
				if have, ok := s.enums[n]; ok && have != v {
					return ""
				}
				s.enums[n] = v
				return n
			case cc.PrimaryExpressionChar:
				if k, err := strconv.ParseInt(v, 10, 64); err == nil {
					if c := scmChar(k); c != "" {
						return "(ch " + c + ")"
					}
				}
			}
		}
		return ""
	}
}

// scmChar is v as a Scheme character a reader reads, when it is one: a
// printable character but those that delimit, and the named ones.
func scmChar(v int64) string {
	switch v {
	case ' ':
		return `#\space`
	case '\t':
		return `#\tab`
	case '\n':
		return `#\newline`
	case '\r':
		return `#\return`
	case 27:
		return `#\esc`
	case 127:
		return `#\delete`
	case 8:
		return `#\backspace`
	case 7:
		return `#\alarm`
	case 0:
		return `#\nul`
	}
	if v > ' ' && v < 0x7f && !strings.ContainsRune("()[]{}\";'`,|#\\", rune(v)) {
		return `#\` + string(rune(v))
	}
	if v > 0 && v < 0x80 {
		return fmt.Sprintf(`#\x%x`, v)
	}
	return ""
}

// scmNot is X when c is (not X), the whole of it.
func scmNot(c string) (string, bool) {
	if !strings.HasPrefix(c, "(not ") || scmFormEnd(c, 0) != len(c) {
		return "", false
	}
	return c[len("(not ") : len(c)-1], true
}

// scmNegate is (not c), or X when c is (not X).
func scmNegate(c string) string {
	if x, ok := scmNot(c); ok {
		return x
	}
	return "(not " + c + ")"
}
