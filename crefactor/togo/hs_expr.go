package togo

// hs_expr.go is an expression of the lowered form printed as Haskell: the
// lines that must run first -- each memory read and each call a binding of
// its own, in C's order (the Java's) -- and then a pure value.  An operand of
// &&, || or ?: that reads or calls runs only where C evaluates it: its lines
// go inside the branch.  A struct's value is its address; an array's, the
// address of its first element.

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// hv is a value: the lines before it, and the pure Haskell expression, of
// Haskell type ht ("agg" for a struct or union's address, "()" for none).
type hv struct {
	binds []string
	val   string
	ht    string
	act   string
	konst bool   // an integer constant, kv
	kv    int64  // its value
	bare  string // a constant's literal without its type, where the place fixes it
	lit   bool   // a string literal's address
	// a conversion's operand and its type: a conversion of this value
	// again may start from it (conv)
	from, fromHt string
	// a constant's C spelling (constSpelling), kept while its value is
	spell     string
	spellAtom bool
	rec       *lvar // a struct that is a value: its members are its value
}

// plain is v where its type is fixed by the place it goes: a constant's
// literal without the annotation.
func (v hv) plain() string {
	switch {
	case v.konst && v.bare != "":
		return v.bare
	case v.konst && !v.lit && !isPtrHt(v.ht) && v.val == hsLit(v.kv, v.ht):
		return hsBare(v.kv, v.ht)
	}
	return v.val
}

// haddr is an lvalue's address: a base address and a constant offset.
type haddr struct {
	base  hv
	off   int
	field *cc.Field // the member it is, when it is one
	// the names: the file-scope object it is in, and whether it is that
	// object whole; the members' offsets' names, and what they add up to
	obj    *hobj
	whole  bool
	syms   []string
	symOff int
}

// member is a with member fl of container type t added.
func (f *hfn) member(a haddr, t cc.Type, fl *cc.Field) haddr {
	a.off += int(fl.Offset())
	a.field = fl
	a.whole = false
	if n := f.h.memberName(t, fl); n != "" {
		a.syms = append(append([]string{}, a.syms...), n)
		a.symOff += int(fl.Offset())
	}
	return a
}

func (f *hfn) lexpr(r lexpr) hv {
	if r.v != nil {
		return f.varRead(r.v)
	}
	if r.raw {
		return f.node(r.n)
	}
	return f.expr(r.n)
}

// expr is e's value, read through the function's substitutions.
func (f *hfn) expr(e cc.ExpressionNode) hv {
	if f.lf != nil {
		if r, ok := f.lf.sub[e]; ok {
			return f.lexpr(r)
		}
	}
	return f.node(e)
}

func (f *hfn) hasSub(e cc.ExpressionNode) bool {
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
func (f *hfn) node(e cc.ExpressionNode) hv {
	if isNullConst(e) && e.Type() != nil && e.Type().Kind() == cc.Ptr {
		return hv{val: "nullPtr", ht: "P", konst: true}
	}
	if k, ok := scalarKind(e.Type()); ok && !hasEffect(e) && !f.hasSub(e) {
		if v, known := intValue(e.Value()); known {
			v = truncK(v, k)
			ht := hsKindType(k)
			if sp, atom := f.h.constSpelling(e, v); sp != "" && ht != "Bool" {
				return named(v, ht, sp, atom)
			}
			return hv{val: hsLit(v, ht), bare: hsBare(v, ht), ht: ht, konst: true, kv: v}
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
			return hv{val: hsString(string(sv)), ht: "P", lit: true}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionCall:
			return f.call(x)
		case cc.PostfixExpressionComplit:
			// in a file-scope object's initializer: an object of its own in
			// the segment (in a function, the lowering made it a temporary)
			if f.lf != nil {
				f.no(x, "a compound literal the lowering did not take apart")
			}
			t := x.TypeName.Type()
			off := f.h.anon(t)
			in := &cc.Initializer{Case: cc.InitializerInitList, InitializerList: x.InitializerList, Token: x.Token}
			f.initInto("(edSeg ed')", off, t, in)
			return f.readAt(haddr{base: hv{val: "(edSeg ed')", ht: "P"}, off: off}, t)
		case cc.PostfixExpressionIndex:
			// the element's own type: an element that is an array is its
			// address (grid[i] of int grid[3][4])
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
			// the member's own type: an array member's expression is its
			// decayed pointer, and its value the array's address
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
			a := f.addrOf(x.CastExpression)
			return f.addrVal(a)
		case cc.UnaryExpressionDeref:
			if v := f.outNamed(x.CastExpression); v != nil {
				return f.varRead(v) // an out-parameter's value
			}
			if et := elemOf(x.CastExpression.Type()); et != nil && et.Kind() == cc.Function || x.Type() != nil && x.Type().Kind() == cc.Function {
				return f.expr(x.CastExpression) // *fp is fp
			}
			return f.readAt(f.addrNode(x), x.Type())
		case cc.UnaryExpressionMinus:
			ht := f.h.hsType(x.Type())
			a := f.conv(f.expr(x.CastExpression), ht)
			return hv{binds: a.binds, val: "(negate " + a.val + ")", ht: ht}
		case cc.UnaryExpressionCpl:
			ht := f.h.hsType(x.Type())
			a := f.conv(f.expr(x.CastExpression), ht)
			return hv{binds: a.binds, val: "(complement " + a.val + ")", ht: ht}
		case cc.UnaryExpressionNot:
			a := f.truth(f.expr(x.CastExpression))
			return hv{binds: a.binds, val: "(not " + a.val + ")", ht: "Bool"}
		case cc.UnaryExpressionPlus:
			return f.conv(f.expr(x.CastExpression), f.h.hsType(x.Type()))
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			v := f.expr(x.CastExpression)
			t := x.Type()
			switch {
			case t.Kind() == cc.Void:
				return hv{binds: v.binds, val: "()", ht: "()"}
			case isAggr(t):
				return v
			}
			return f.conv(v, f.h.hsType(t))
		}
	case *cc.MultiplicativeExpression:
		ops := map[cc.MultiplicativeExpressionCase]string{cc.MultiplicativeExpressionMul: "*", cc.MultiplicativeExpressionDiv: "/", cc.MultiplicativeExpressionMod: "%"}
		return f.arith(ops[x.Case], f.expr(x.MultiplicativeExpression), f.expr(x.CastExpression), f.h.hsType(x.Type()))
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
		return f.arith(op, f.expr(x.ShiftExpression), f.expr(x.AdditiveExpression), f.h.hsType(x.Type()))
	case *cc.AndExpression:
		return f.arith("&", f.expr(x.AndExpression), f.expr(x.EqualityExpression), f.h.hsType(x.Type()))
	case *cc.ExclusiveOrExpression:
		return f.arith("^", f.expr(x.ExclusiveOrExpression), f.expr(x.AndExpression), f.h.hsType(x.Type()))
	case *cc.InclusiveOrExpression:
		return f.arith("|", f.expr(x.InclusiveOrExpression), f.expr(x.ExclusiveOrExpression), f.h.hsType(x.Type()))
	case *cc.RelationalExpression:
		ops := map[cc.RelationalExpressionCase]string{cc.RelationalExpressionLt: "<", cc.RelationalExpressionGt: ">", cc.RelationalExpressionLeq: "<=", cc.RelationalExpressionGeq: ">="}
		return f.compare(ops[x.Case], x.RelationalExpression, x.ShiftExpression)
	case *cc.EqualityExpression:
		op := "=="
		if x.Case == cc.EqualityExpressionNeq {
			op = "/="
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
	return hv{}
}

// fnDesignator is the function e names, or nil.
func fnDesignator(e cc.ExpressionNode) *cc.Declarator {
	if p, ok := unparenE(e).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
			return d
		}
	}
	return nil
}

// fnValue is a function's address: its index in the table.
func (f *hfn) fnValue(d *cc.Declarator) hv {
	return hv{val: fmt.Sprintf("(fnPtr %d)", f.h.fnPtrOf(d.Name())), ht: "P"}
}

// ident is a name's value.
func (f *hfn) ident(x *cc.PrimaryExpression) hv {
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
					return hv{val: "(i2p 1)", ht: "P", konst: true, kv: 1}
				}
				return f.varRead(v)
			}
		}
		a, ok := f.objAddr(f.h.g.a.declKey(d))
		if !ok {
			f.no(x, "an object with no place: %s", d.Name())
		}
		return f.readAt(a, d.Type())
	}
	f.no(x, "a name %s", x.Token.SrcStr())
	return hv{}
}

// varRead is a variable's value: its binding, or its memory in the frame.
func (f *hfn) varRead(v *lvar) hv {
	if f.sv[v] != nil {
		return hv{ht: "agg", rec: v}
	}
	if off, ok := f.mem[v]; ok {
		return f.readAt(haddr{base: hv{val: "fr'", ht: "P"}, off: off}, v.c)
	}
	return hv{val: f.valueOf(f.cur, v), ht: f.vtype(v)}
}

// addrVal is an address as a value.
func (f *hfn) addrVal(a haddr) hv {
	if a.off == 0 && len(a.syms) == 0 {
		return hv{binds: a.base.binds, val: a.base.val, ht: "P"}
	}
	return hv{binds: a.base.binds, val: fmt.Sprintf("(pAdd %s %s)", a.base.val, offStr(a)), ht: "P"}
}

// readAt is the value of the object of type t at a: a scalar read, or the
// address of an array, a struct or a union.
func (f *hfn) readAt(a haddr, t cc.Type) hv {
	switch {
	case t.Kind() == cc.Array || t.Kind() == cc.Function:
		return f.addrVal(a)
	case isAggr(t):
		v := f.addrVal(a)
		v.ht = "agg"
		return v
	}
	if a.field != nil && a.field.IsBitfield() {
		f.no(nil, "a bit field")
	}
	ht := f.h.hsType(t)
	r := f.tmp()
	read := fmt.Sprintf("%s <- rd%s %s %s", r, hsAccess(ht), a.base.val, offStr(a))
	if a.whole && a.obj != nil {
		read = fmt.Sprintf("%s <- %s ed'", r, a.obj.name)
	}
	binds := append(append([]string{}, a.base.binds...), read)
	return hv{binds: binds, val: r, ht: ht}
}

// addrOf is an lvalue's address.
func (f *hfn) addrOf(e cc.ExpressionNode) haddr {
	if f.lf != nil {
		if r, ok := f.lf.sub[e]; ok {
			if r.v != nil {
				off, ok := f.mem[r.v]
				if !ok {
					f.no(e, "the address of a variable not in memory")
				}
				return haddr{base: hv{val: "fr'", ht: "P"}, off: off}
			}
			if !r.raw {
				return f.addrOf(r.n)
			}
		}
	}
	return f.addrNode(e)
}

// addrNode is addrOf of e itself, its operands through the substitutions.
func (f *hfn) addrNode(e cc.ExpressionNode) haddr {
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
			return haddr{base: f.node(x)}
		case cc.PrimaryExpressionIdent:
			d, ok := x.ResolvedTo().(*cc.Declarator)
			if !ok {
				break
			}
			if f.lf != nil {
				if v, ok := f.lf.byDecl[d]; ok {
					off, inMem := f.mem[v]
					if !inMem {
						f.no(x, "the address of %s, which is not in memory", d.Name())
					}
					return haddr{base: hv{val: "fr'", ht: "P"}, off: off}
				}
			}
			a, ok := f.objAddr(f.h.g.a.declKey(d))
			if !ok {
				f.no(x, "an object with no place: %s", d.Name())
			}
			return a
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect:
			a := f.addrOf(x.PostfixExpression)
			return f.member(a, x.PostfixExpression.Type(), x.Field())
		case cc.PostfixExpressionPSelect:
			p := f.expr(x.PostfixExpression)
			return f.member(haddr{base: p}, x.PostfixExpression.Type(), x.Field())
		case cc.PostfixExpressionIndex:
			be, ie := x.PostfixExpression, x.ExpressionList
			if !isPtrish(be.Type()) {
				be, ie = ie, be
			}
			base := f.expr(be)
			size := elemSize(be.Type())
			i := f.expr(ie)
			if i.konst {
				return haddr{base: base, off: int(i.kv * size)}
			}
			binds := append(append([]string{}, base.binds...), i.binds...)
			return haddr{base: hv{binds: binds, val: fmt.Sprintf("(pAdd %s %s)", base.val, f.byteOff(i, size, false)), ht: "P"}}
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			return haddr{base: f.expr(x.CastExpression)}
		}
	}
	if isAggr(e.Type()) {
		return haddr{base: f.expr(e)} // a struct's value is its address
	}
	f.no(e, "the address of %T", e)
	return haddr{}
}

// hsLit is v as a literal of type ht.
func hsLit(v int64, ht string) string {
	switch {
	case ht == "Bool":
		if v != 0 {
			return "True"
		}
		return "False"
	case isPtrHt(ht):
		if v == 0 {
			return "nullPtr"
		}
		return fmt.Sprintf("(i2p %d)", uint64(v))
	case strings.HasPrefix(ht, "Word"):
		return fmt.Sprintf("(%d :: %s)", uint64(v)&hsMask(ht), ht)
	}
	return fmt.Sprintf("(%d :: %s)", v, ht)
}

func b2i64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// hsParen is a type as an argument: parenthesized when it is applied.
func hsParen(ht string) string {
	if strings.Contains(ht, " ") {
		return "(" + ht + ")"
	}
	return ht
}

// hsBare is v as a literal where the place fixes its type ht.
func hsBare(v int64, ht string) string {
	switch {
	case ht == "Bool" || isPtrHt(ht):
		return hsLit(v, ht)
	case strings.HasPrefix(ht, "Word"):
		return fmt.Sprint(uint64(v) & hsMask(ht))
	case v < 0:
		return fmt.Sprintf("(%d)", v)
	}
	return fmt.Sprint(v)
}

// hsString is a C string's bytes as a pointer to static memory: a
// primitive string literal, which Haskell stores as its bytes.
func hsString(s string) string {
	var b strings.Builder
	b.WriteString(`(Ptr "`)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "\\%d", c)
			if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
				b.WriteString(`\&`)
			}
		}
	}
	b.WriteString(`"#)`)
	return b.String()
}

// conv is v converted to Haskell type to, as C converts.
func (f *hfn) conv(v hv, to string) hv {
	if v.ht == to || to == "agg" || to == "()" || v.ht == "agg" {
		return v
	}
	if v.konst && !v.lit && isPtrHt(v.ht) && to == "Bool" {
		return hv{binds: v.binds, val: hsLit(b2i64(v.kv != 0), "Bool"), ht: "Bool", konst: true, kv: b2i64(v.kv != 0)}
	}
	if v.konst && !v.lit && !isPtrHt(v.ht) {
		k := v.kv
		if to == "Bool" && k != 0 {
			k = 1
		}
		if v.spell != "" && to != "Bool" && !isPtrHt(to) && hsBare(k, to) == fmt.Sprint(k) && k == v.kv {
			n := named(k, to, v.spell, v.spellAtom)
			n.binds = v.binds
			return n
		}
		return hv{binds: v.binds, val: hsLit(k, to), bare: hsBare(k, to), ht: to, konst: true, kv: k}
	}
	// a conversion of a conversion that kept every value is one conversion
	if v.from != "" && hsKeeps(v.fromHt, v.ht) {
		if v.fromHt == to {
			return hv{binds: v.binds, val: v.from, ht: to}
		}
		v = hv{binds: v.binds, val: v.from, ht: v.fromHt}
	}
	var s string
	switch {
	case isPtrHt(v.ht) && isPtrHt(to):
		// a pointer converted: castPtr where both types are fixed and
		// differ; a value of any pointer type (nullPtr, pAdd, a literal:
		// "P") takes the type it is used at
		if v.ht == "P" || to == "P" || f.h.canon(v.ht) == f.h.canon(to) {
			return hv{binds: v.binds, val: v.val, ht: to, konst: v.konst, kv: v.kv, lit: v.lit}
		}
		s = "(castPtr " + v.val + ")"
	case v.ht == "Bool" && isPtrHt(to):
		s = "(i2p (b2i " + v.val + "))"
	case v.ht == "Bool":
		s = fmt.Sprintf("(b2i %s :: %s)", v.val, to)
	case to == "Bool" && isPtrHt(v.ht):
		s = "(" + v.val + " /= nullPtr)"
	case to == "Bool":
		s = "(" + v.val + " /= 0)"
	case isPtrHt(v.ht):
		s = fmt.Sprintf("(fromIntegral (p2i %s) :: %s)", v.val, to)
	case isPtrHt(to):
		s = "(i2p (fromIntegral " + v.val + "))"
	default:
		s = fmt.Sprintf("(fromIntegral %s :: %s)", v.val, to)
		return hv{binds: v.binds, val: s, ht: to, from: v.val, fromHt: v.ht}
	}
	return hv{binds: v.binds, val: s, ht: to}
}

// hsKeeps says converting an integer of type a to type b keeps every value.
func hsKeeps(a, b string) bool {
	sa, ua, ok1 := hsIntType(a)
	sb, ub, ok2 := hsIntType(b)
	if !ok1 || !ok2 {
		return false
	}
	if !ua {
		return !ub && sb >= sa
	}
	return ub && sb >= sa || !ub && sb > sa
}

// hsIntType is an integer type's bits and whether it is unsigned.
func hsIntType(ht string) (int, bool, bool) {
	var n int
	switch {
	case strings.HasPrefix(ht, "Int"):
		if _, err := fmt.Sscan(ht[3:], &n); err == nil {
			return n, false, true
		}
	case strings.HasPrefix(ht, "Word"):
		if _, err := fmt.Sscan(ht[4:], &n); err == nil {
			return n, true, true
		}
	}
	return 0, false, false
}

// toInt is an integer value as an Int, pAdd's and shiftL's operand: C's
// conversion to a 64-bit offset, which fromIntegral from any integer type
// is.
func (f *hfn) toInt(v hv) string {
	switch {
	case v.konst:
		return hsOff(int(v.kv))
	case v.ht == "Bool":
		return "(b2i " + v.val + ")"
	case v.from != "" && hsKeeps(v.fromHt, v.ht):
		return "(fromIntegral " + v.from + ")"
	}
	return "(fromIntegral " + v.val + ")"
}

// byteOff is n elements of size bytes, negated when neg, as pAdd's offset.
func (f *hfn) byteOff(n hv, size int64, neg bool) string {
	if n.konst {
		k := n.kv * size
		if neg {
			k = -k
		}
		return hsOff(int(k))
	}
	s := f.toInt(n)
	if size != 1 {
		s = fmt.Sprintf("(%s * %d)", s, size)
	}
	if neg {
		s = "(negate " + s + ")"
	}
	return s
}

// truth is v as a condition.
func (f *hfn) truth(v hv) hv { return f.conv(v, "Bool") }

// arith is a op b in Haskell type ht: C's arithmetic on the operands'
// usual type, which the expression's is.
func (f *hfn) arith(op string, a, b hv, ht string) hv {
	a = f.conv(a, ht)
	shift := op == "<<" || op == ">>"
	if !shift {
		b = f.conv(b, ht)
	}
	binds := append(append([]string{}, a.binds...), b.binds...)
	av, bv := hsPair(a, b)
	if shift {
		av, bv = a.val, f.toInt(b)
	}
	var s string
	switch op {
	case "+", "-", "*":
		s = fmt.Sprintf("(%s %s %s)", av, op, bv)
	case "/":
		s = fmt.Sprintf("(quot %s %s)", av, bv)
	case "%":
		s = fmt.Sprintf("(rem %s %s)", av, bv)
	case "&":
		s = fmt.Sprintf("(%s .&. %s)", av, bv)
	case "|":
		s = fmt.Sprintf("(%s .|. %s)", av, bv)
	case "^":
		s = fmt.Sprintf("(xor %s %s)", av, bv)
	case "<<":
		s = fmt.Sprintf("(shiftL %s %s)", av, bv)
	case ">>":
		s = fmt.Sprintf("(shiftR %s %s)", av, bv)
	default:
		f.no(nil, "an operator %s", op)
	}
	return hv{binds: binds, val: s, ht: ht}
}

// hsPair is two operands of one type: a constant without its annotation
// when the other is not a constant, which fixes the type.
func hsPair(a, b hv) (string, string) {
	switch {
	case a.konst && !b.konst:
		return a.plain(), b.val
	case b.konst && !a.konst:
		return a.val, b.plain()
	}
	return a.val, b.val
}

// additive is + or -: of a pointer and an integer, of two pointers, or of
// two numbers.
func (f *hfn) additive(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) hv {
	lt, rt := le.Type(), re.Type()
	switch {
	case isPtrish(lt) && isPtrish(rt):
		a, b := f.expr(le), f.expr(re)
		binds := append(append([]string{}, a.binds...), b.binds...)
		ht := f.h.hsType(x.Type())
		return hv{binds: binds, val: fmt.Sprintf("(fromIntegral (quot (pSub %s %s) %d) :: %s)", a.val, b.val, elemSize(lt), ht), ht: ht}
	case isPtrish(lt) || isPtrish(rt):
		pe, ie := le, re
		if isPtrish(rt) {
			pe, ie = re, le
		}
		p := f.expr(pe)
		n := f.expr(ie)
		size := elemSize(pe.Type())
		binds := append(append([]string{}, p.binds...), n.binds...)
		if pe == le {
			binds = append(append([]string{}, p.binds...), n.binds...)
		} else {
			binds = append(append([]string{}, n.binds...), p.binds...)
		}
		return hv{binds: binds, val: fmt.Sprintf("(pAdd %s %s)", p.val, f.byteOff(n, size, op == "-")), ht: "P"}
	}
	return f.arith(op, f.expr(le), f.expr(re), f.h.hsType(x.Type()))
}

// compare is a comparison: of addresses, or of numbers in their usual type.
func (f *hfn) compare(op string, le, re cc.ExpressionNode) hv {
	a, b := f.expr(le), f.expr(re)
	var ht string
	if isPtrish(le.Type()) || isPtrish(re.Type()) || le.Type().Kind() == cc.Function || re.Type().Kind() == cc.Function {
		// one pointer type: the left's, when it is fixed
		ht = a.ht
		if !isPtrHt(ht) || ht == "P" {
			ht = b.ht
		}
		if !isPtrHt(ht) {
			ht = "P"
		}
	} else {
		lk, ok1 := scalarKind(le.Type())
		rk, ok2 := scalarKind(re.Type())
		if !ok1 || !ok2 {
			f.no(le, "a comparison of %s and %s", le.Type(), re.Type())
		}
		ht = hsKindType(usualK(lk, rk))
	}
	a, b = f.conv(a, ht), f.conv(b, ht)
	binds := append(append([]string{}, a.binds...), b.binds...)
	if isPtrHt(ht) && a.konst && b.konst && !a.lit && !b.lit && (op == "==" || op == "/=") {
		// two constant pointers: an out-parameter against null
		eq := a.kv == b.kv
		if op == "/=" {
			eq = !eq
		}
		return hv{binds: binds, val: hsLit(b2i64(eq), "Bool"), ht: "Bool", konst: true, kv: b2i64(eq)}
	}
	av, bv := hsPair(a, b)
	return hv{binds: binds, val: fmt.Sprintf("(%s %s %s)", av, op, bv), ht: "Bool"}
}

// logical is && (and) or ||: the right operand's lines run only when C
// evaluates it.
func (f *hfn) logical(and bool, le, re cc.ExpressionNode) hv {
	a := f.truth(f.expr(le))
	b := f.truth(f.expr(re))
	if len(b.binds) == 0 {
		op := "||"
		if and {
			op = "&&"
		}
		return hv{binds: a.binds, val: fmt.Sprintf("(%s %s %s)", a.val, op, b.val), ht: "Bool"}
	}
	r := f.tmp()
	var line string
	switch {
	case f.pure && and:
		line = fmt.Sprintf("let !%s = if %s then %s else False", r, a.val, pureExpr(b))
	case f.pure:
		line = fmt.Sprintf("let !%s = if %s then True else %s", r, a.val, pureExpr(b))
	case and:
		line = fmt.Sprintf("%s <- if %s then %s else pure False", r, a.val, doBlock(b))
	default:
		line = fmt.Sprintf("%s <- if %s then pure True else %s", r, a.val, doBlock(b))
	}
	return hv{binds: append(append([]string{}, a.binds...), line), val: r, ht: "Bool"}
}

// doBlock is a value's lines and value as one action.
func doBlock(v hv) string {
	if len(v.binds) == 0 {
		return "pure " + v.val
	}
	// a let on one line: its binding braced, or it would take the rest
	binds := make([]string, len(v.binds))
	for i, b := range v.binds {
		if strings.HasPrefix(b, "let !") {
			b = "let { " + strings.TrimPrefix(b, "let ") + " }"
		}
		binds[i] = b
	}
	last := binds[len(binds)-1]
	if strings.HasPrefix(last, v.val+" <- ") {
		// its last action's result is the value: the action itself
		act := strings.TrimPrefix(last, v.val+" <- ")
		if len(binds) == 1 {
			return "(" + act + ")"
		}
		return "(do { " + strings.Join(binds[:len(binds)-1], "; ") + "; " + act + " })"
	}
	return "(do { " + strings.Join(binds, "; ") + "; pure " + v.val + " })"
}

// outResult is a call with out-arguments: its result and the locals' new
// values, bound as a tuple, each local its new name -- or, a local in the
// frame, its slot written.
func (f *hfn) outResult(binds []string, line, ht string, pure bool, g string, outs map[int]*lvar) hv {
	var names []string
	r := ""
	if ht != "()" {
		r = f.tmp()
		names = append(names, r)
	}
	type upd struct {
		v    *lvar
		name string
	}
	var ups []upd
	for _, i := range f.h.outs[g] {
		o := outs[i]
		n := f.tmp()
		if f.reg(o) {
			n = f.fresh(o)
		}
		names = append(names, n)
		ups = append(ups, upd{o, n})
	}
	pat := strings.Join(names, ", ")
	if len(names) > 1 {
		pat = "(" + pat + ")"
	}
	if pure {
		binds = append(binds, "let !"+pat+" = "+line)
	} else {
		binds = append(binds, pat+" <- "+line)
	}
	for _, u := range ups {
		if f.reg(u.v) {
			f.cur[u.v] = u.name
			continue
		}
		off := f.mem[u.v]
		binds = append(binds, fmt.Sprintf("wr%s fr' %d %s", hsAccess(f.vtype(u.v)), off, u.name))
	}
	if r == "" {
		return hv{binds: binds, val: "()", ht: "()"}
	}
	return hv{binds: binds, val: r, ht: ht}
}

// pureExpr is a value and the lets before it as one expression.
func pureExpr(v hv) string {
	if len(v.binds) == 0 {
		return v.val
	}
	var bs []string
	for _, b := range v.binds {
		bs = append(bs, strings.TrimPrefix(b, "let "))
	}
	return "(let { " + strings.Join(bs, "; ") + " } in " + v.val + ")"
}

// ternary is ?:, each arm's lines only when it is chosen.
func (f *hfn) ternary(x *cc.ConditionalExpression) hv {
	c := f.truth(f.expr(x.LogicalOrExpression))
	ht := f.h.hsType(x.Type())
	a := f.conv(f.expr(x.ExpressionList), ht)
	b := f.conv(f.expr(x.ConditionalExpression), ht)
	if len(a.binds) == 0 && len(b.binds) == 0 {
		av, bv := hsPair(a, b)
		return hv{binds: c.binds, val: fmt.Sprintf("(if %s then %s else %s)", c.val, av, bv), ht: ht}
	}
	r := f.tmp()
	line := fmt.Sprintf("%s <- if %s then %s else %s", r, c.val, doBlock(a), doBlock(b))
	if f.pure {
		line = fmt.Sprintf("let !%s = if %s then %s else %s", r, c.val, pureExpr(a), pureExpr(b))
	}
	return hv{binds: append(append([]string{}, c.binds...), line), val: r, ht: ht}
}

// call is a call: of a function the unit defines, of the host's, or through
// a pointer.  A struct result is written into the frame, and its value is
// that address.
func (f *hfn) call(x *cc.PostfixExpression) hv {
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	var ft *cc.FunctionType
	var head string
	var binds []string
	d := fnDesignator(x.PostfixExpression)
	if d != nil && d.Name() == "__builtin_expect" && len(args) == 2 {
		// gcc's hint: its value is its first argument's
		return f.conv(f.expr(args[0]), f.h.hsType(x.Type()))
	}
	viaPtr := d == nil
	host := d != nil && f.h.defined[d.Name()] == nil
	if d != nil {
		ft, _ = d.Type().(*cc.FunctionType)
		if f.h.defined[d.Name()] != nil {
			head = f.h.names[d.Name()]
			if f.h.takesEd(d.Name()) {
				head += " ed'"
			}
		} else {
			head = f.h.host + "." + hsName(d.Name()) + " ed'"
		}
	} else {
		p := f.expr(x.PostfixExpression)
		binds = append(binds, p.binds...)
		t := x.PostfixExpression.Type()
		if pt, ok := t.(*cc.PointerType); ok {
			t = pt.Elem()
		}
		ft, _ = t.(*cc.FunctionType)
		head = p.val
	}
	if ft == nil {
		f.no(x, "a call of no function type")
	}
	params := ft.Parameters()
	if len(params) == 1 && (params[0].Type() == nil || params[0].Type().Kind() == cc.Void) {
		params = nil
	}
	var vals, rest []string
	outs, hasOuts := f.outCall(x)
	for i, a := range args {
		if o := outs[i]; o != nil {
			// an out-argument: the local's value goes in
			v := f.conv(f.varRead(o), f.vtype(o))
			binds = append(binds, v.binds...)
			vals = append(vals, v.plain())
			continue
		}
		if i < len(params) {
			pt := params[i].Type()
			if pt.Kind() == cc.Array {
				pt = pt.Decay()
			}
			want := f.h.hsParamType(pt)
			if host && isPtrHt(want) {
				want = "Ptr ()" // the host takes P
			}
			v := f.conv(f.expr(a), want)
			binds = append(binds, v.binds...)
			if viaPtr {
				vals = append(vals, v.val) // toRaw's, whose type it is
			} else {
				vals = append(vals, v.plain())
			}
			continue
		}
		v := f.expr(a)
		binds = append(binds, v.binds...)
		switch {
		case v.ht == "Bool":
			rest = append(rest, "VI (b2i "+v.val+")")
		case v.ht == "P" || v.ht == "Ptr ()":
			rest = append(rest, "VP "+v.val)
		case isPtrHt(v.ht):
			rest = append(rest, "VP (castPtr "+v.val+")")
		case strings.HasPrefix(v.ht, "Word") && v.konst:
			rest = append(rest, "VU "+hsBare(v.kv, "Word64"))
		case strings.HasPrefix(v.ht, "Word"):
			rest = append(rest, "VU (fromIntegral "+v.val+")")
		case v.konst:
			rest = append(rest, "VI "+hsBare(v.kv, "Int64"))
		default:
			rest = append(rest, "VI (fromIntegral "+v.val+")")
		}
	}
	if ft.IsVariadic() {
		vals = append(vals, "["+strings.Join(rest, ", ")+"]")
	}
	rt := ft.Result()
	if viaPtr {
		var raws []string
		for _, v := range vals {
			raws = append(raws, "toRaw "+v)
		}
		call := fmt.Sprintf("callPtr ed' %s [%s]", head, strings.Join(raws, ", "))
		if isAggr(rt) {
			f.no(x, "a struct result through a pointer")
		}
		ht := f.h.hsType(rt)
		if ht == "()" {
			return hv{binds: append(binds, call+" >> pure ()"), val: "()", ht: "()"}
		}
		r := f.tmp()
		return hv{binds: append(binds, fmt.Sprintf("%s <- (fromRaw <$> %s :: IO %s)", r, call, hsParen(ht))), val: r, ht: ht}
	}
	if isAggr(rt) {
		off := f.alloc(nil, rt)
		dst := fmt.Sprintf("(pAdd fr' %d)", off)
		line := strings.Join(append([]string{head, dst}, vals...), " ")
		return hv{binds: append(binds, line), val: dst, ht: "agg"}
	}
	line := strings.Join(append([]string{head}, vals...), " ")
	ht := f.h.hsType(rt)
	if host && isPtrHt(ht) {
		ht = "Ptr ()" // the host's P
	}
	pure := d != nil && f.h.pure(d.Name())
	if hasOuts {
		return f.outResult(binds, line, ht, pure, d.Name(), outs)
	}
	switch {
	case ht == "()" && pure:
		return hv{binds: binds, val: "()", ht: "()"} // nothing it does
	case ht == "()":
		return hv{binds: append(binds, line), val: "()", ht: "()"}
	case pure:
		// a function of its arguments: its value, bound as C evaluates it
		r := f.tmp()
		return hv{binds: append(binds, "let !"+r+" = "+line), val: r, ht: ht}
	}
	r := f.tmp()
	return hv{binds: append(binds, r+" <- "+line), val: r, ht: ht}
}
