package togo

// rs_expr.go is a C expression printed as Rust: a value, of a Rust type that
// is the C type's (rs_types.go), every conversion C makes said with `as`,
// and what C does inside an expression -- an assignment's value, an
// increment, the comma's left operands -- a Rust block, which is an
// expression: `{ let t = x; x = t.wrapping_add(1); t }`.  An lvalue is a
// Rust place -- a local, `(*ed).object`, `*p`, `(*p).member` -- and its
// address `&raw mut` of it; an array is its first element's address
// wherever C converts it to one.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// rv is a value: a Rust expression of Rust type ty ("()" for none).
type rv struct {
	s     string
	ty    string
	prec  int
	konst bool   // an integer constant, kv
	kv    int64  // its value, of its type
	spell string // a named constant's name, of its own type (ty)
	chr   bool   // a character constant
	null  bool   // a null pointer constant
	str   []byte // a string literal's bytes, its NUL included: s is their address
}

// the precedences of Rust's expressions, tightest first
const (
	pPrim = iota // a path, a literal, a call, a member, a method call, ( )
	pUnary
	pAs
	pMul
	pAdd
	pShift
	pBitAnd
	pXor
	pBitOr
	pCmp
	pAnd
	pOr
	pLow // an if, a block
)

// wrap is v's text where an operand of precedence max goes.
func wrap(v rv, max int) string {
	if v.prec > max {
		return "(" + v.s + ")"
	}
	return v.s
}

// unparenRs drops the parentheses around a whole expression.
func unparenRs(s string) string {
	for len(s) > 1 && s[0] == '(' && s[len(s)-1] == ')' && rsBalanced(s[1:len(s)-1]) {
		s = s[1 : len(s)-1]
	}
	return s
}

// rsBalanced says s's brackets balance, outside its string and character
// literals.
func rsBalanced(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return false
			}
		case '"':
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '\'':
			// b'x' or b'\x': a character; 'label otherwise
			if i > 0 && s[i-1] == 'b' {
				for i++; i < len(s) && s[i] != '\''; i++ {
					if s[i] == '\\' {
						i++
					}
				}
			}
		}
	}
	return depth == 0
}

func isIntTy(ty string) bool {
	switch ty {
	case "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "isize", "usize":
		return true
	}
	return false
}

func isPtrTy(ty string) bool  { return strings.HasPrefix(ty, "*mut ") }
func isFnTy(ty string) bool   { return strings.HasPrefix(ty, "Option<") }
func isSigned(ty string) bool { return ty[0] == 'i' }

// tyKind is a Rust integer type's kind.
func tyKind(ty string) jk {
	if ty == "bool" {
		return jk{size: 1, boolean: true}
	}
	if ty == "isize" || ty == "usize" {
		return jk{size: 8, signed: ty[0] == 'i'}
	}
	n, _ := strconv.Atoi(ty[1:])
	return jk{size: n / 8, signed: ty[0] == 'i'}
}

// rsLit is v as a literal of ty, with no suffix: where the place fixes the
// type.
func rsLit(v int64, ty string) string {
	switch ty {
	case "bool":
		if v != 0 {
			return "true"
		}
		return "false"
	case "u8", "u16", "u32", "u64":
		return strconv.FormatUint(uint64(truncK(v, tyKind(ty))), 10)
	case "i64":
		if v == -1<<63 {
			return "i64::MIN"
		}
	}
	return strconv.FormatInt(v, 10)
}

// lit is a constant of ty: plain where the place fixes its type (fit), else
// with its type's suffix.
func lit(v int64, ty string, fit bool) rv {
	s := rsLit(v, ty)
	if !fit && ty != "bool" && !strings.Contains(s, "::") {
		s += ty
	}
	p := pPrim
	if strings.HasPrefix(s, "-") {
		p = pUnary
	}
	return rv{s: s, ty: ty, prec: p, konst: true, kv: v}
}

// konst is an integer constant of type ty, spelled as C spells it where it
// can be: a named constant, a character.
func (f *rfn) konst(e cc.ExpressionNode, v int64, ty string) rv {
	x := lit(v, ty, true)
	ue := unparenE(e)
	if p, ok := ue.(*cc.PrimaryExpression); ok {
		switch p.Case {
		case cc.PrimaryExpressionIdent:
			if en, ok := p.ResolvedTo().(*cc.Enumerator); ok {
				if c := f.r.consts[en.Token.SrcStr()]; c != nil {
					x.spell = c.name
					if c.ty == ty {
						x.s = c.name
						x.prec = pPrim
					}
				}
			}
		case cc.PrimaryExpressionChar:
			if v >= 32 && v < 127 && v != '\'' && v != '\\' {
				x.chr = true
				x.s, x.prec = "b'"+string(rune(v))+"' as "+ty, pAs
				if ty == "u8" {
					x.s, x.prec = "b'"+string(rune(v))+"'", pPrim
				}
			}
		}
	}
	return x
}

// fitK is a constant as a value of ty where the place fixes its type.
func fitK(v rv, ty string, fit bool) rv {
	k := truncK(v.kv, tyKind(ty))
	if v.spell != "" && v.ty == ty {
		return v
	}
	if v.chr && ty == "u8" {
		return rv{s: "b'" + string(rune(k)) + "'", ty: ty, konst: true, kv: k}
	}
	if v.chr && k == v.kv {
		return rv{s: "b'" + string(rune(k)) + "' as " + ty, ty: ty, prec: pAs, konst: true, kv: k}
	}
	if v.spell != "" {
		return rv{s: v.spell + " as " + ty, ty: ty, prec: pAs, konst: true, kv: k}
	}
	return lit(k, ty, fit)
}

// --- conversions -------------------------------------------------------------

// conv is v converted to Rust type to, as C converts it, where the place
// fixes the type (a constant is then a plain literal).
func (f *rfn) conv(v rv, to string) rv { return f.convFit(v, to, true) }

// convT is v converted to to, a constant with its type's suffix: where
// nothing fixes it -- a method's receiver.
func (f *rfn) convT(v rv, to string) rv { return f.convFit(v, to, false) }

func (f *rfn) convFit(v rv, to string, fit bool) rv {
	switch {
	case v.ty == to && !(v.konst && !fit && v.spell == ""):
		return v
	case v.konst && (isIntTy(to) || to == "bool") && !v.null:
		if to == "bool" {
			return lit(b2i64(v.kv != 0), "bool", true)
		}
		return fitK(v, to, fit)
	case v.null && isPtrTy(to):
		if fit {
			return rv{s: "null_mut()", ty: to}
		}
		return rv{s: "null_mut::<" + strings.TrimPrefix(to, "*mut ") + ">()", ty: to}
	case (v.null || v.konst && v.kv == 0) && isFnTy(to):
		return rv{s: "None", ty: to}
	case v.str != nil && isPtrTy(to):
		return rv{s: rsBytes(v.str) + ".as_ptr() as " + to, ty: to, prec: pAs}
	case to == "bool":
		return rv{s: f.truth(v), ty: "bool", prec: pCmp}
	case v.ty == "bool" && isIntTy(to):
		return rv{s: wrap(v, pUnary) + " as " + to, ty: to, prec: pAs}
	case isIntTy(v.ty) && isIntTy(to), isPtrTy(v.ty) && isPtrTy(to), isPtrTy(v.ty) && isIntTy(to), isIntTy(v.ty) && isPtrTy(to):
		return rv{s: wrap(v, pAs) + " as " + to, ty: to, prec: pAs}
	case isFnTy(v.ty) && isFnTy(to):
		return rv{s: "core::mem::transmute::<" + v.ty + ", " + to + ">(" + v.s + ")", ty: to}
	case isFnTy(v.ty) && isPtrTy(to):
		return rv{s: "fn_addr(" + v.s + ") as " + to, ty: to, prec: pAs}
	case isPtrTy(v.ty) && isFnTy(to):
		return rv{s: "core::mem::transmute::<" + v.ty + ", " + to + ">(" + v.s + ")", ty: to}
	case to == "()":
		return v
	}
	panic(unsupported{fmt.Sprintf("a conversion of %s to %s", v.ty, to)})
}

// nullish says v is C's null pointer constant: 0, or (void *)0.
func nullish(v rv) bool { return v.null || v.konst && v.kv == 0 }

// isFnPtr says t is a pointer to a function.
func isFnPtr(t cc.Type) bool {
	if t == nil || t.Kind() != cc.Ptr {
		return false
	}
	e := elemOf(t)
	return e != nil && e.Kind() == cc.Function
}

// truth is v as a Rust bool: C's v != 0.
func (f *rfn) truth(v rv) string {
	switch {
	case v.ty == "bool":
		return v.s
	case v.konst:
		if v.kv != 0 {
			return "true"
		}
		return "false"
	case v.null:
		return "false"
	case isIntTy(v.ty):
		return wrap(v, pBitOr) + " != 0"
	case isPtrTy(v.ty):
		return "!" + wrap(v, pPrim) + ".is_null()"
	case isFnTy(v.ty):
		return wrap(v, pPrim) + ".is_some()"
	}
	panic(unsupported{"the truth of a " + v.ty})
}

// rsBytes is a string literal's bytes as a Rust byte string.
func rsBytes(b []byte) string {
	var s strings.Builder
	s.WriteString(`b"`)
	for _, c := range b {
		switch {
		case c == '"':
			s.WriteString(`\"`)
		case c == '\\':
			s.WriteString(`\\`)
		case c == '\n':
			s.WriteString(`\n`)
		case c == '\t':
			s.WriteString(`\t`)
		case c == '\r':
			s.WriteString(`\r`)
		case c == 0:
			s.WriteString(`\0`)
		case c >= 32 && c < 127:
			s.WriteByte(c)
		default:
			fmt.Fprintf(&s, `\x%02x`, c)
		}
	}
	s.WriteString(`"`)
	return s.String()
}

// --- values -----------------------------------------------------------------

// vty is the Rust type of the value of an expression of C type t: an array
// its first element's address, a function its pointer.
func (f *rfn) vty(t cc.Type) string {
	if t == nil {
		return "()"
	}
	switch t.Kind() {
	case cc.Array:
		return "*mut " + f.r.rtype(elemOf(t), false, "")
	case cc.Function:
		return f.r.fnType(t, false)
	}
	return f.r.ty(t)
}

// expr is e's value.
func (f *rfn) expr(e cc.ExpressionNode) rv {
	if f.sub != nil && e != f.skip {
		if r, ok := f.sub[e]; ok {
			return f.lexpr(r)
		}
	}
	f.skip = nil
	t := e.Type()
	if isNullConst(e) && t != nil && t.Kind() == cc.Ptr {
		if isFnPtr(t) {
			return rv{s: "None", ty: f.vty(t), null: true}
		}
		return rv{s: "null_mut()", ty: f.vty(t), null: true}
	}
	if k, ok := scalarKind(t); ok && !hasEffect(e) {
		if v, known := intValue(e.Value()); known {
			return f.konst(e, truncK(v, k), rsKind(k))
		}
	}
	switch x := e.(type) {
	case *cc.ConstantExpression:
		return f.expr(x.ConditionalExpression)
	case *cc.ExpressionList:
		if x.ExpressionList == nil {
			return f.expr(x.AssignmentExpression)
		}
		// the comma: what the left does, then the right's value
		var parts []string
		for x.ExpressionList != nil {
			parts = append(parts, f.stmtsOf(x.AssignmentExpression)...)
			x = x.ExpressionList
		}
		v := f.expr(x.AssignmentExpression)
		return f.block(parts, v)
	case *cc.PrimaryExpression:
		return f.primary(x)
	case *cc.PostfixExpression:
		return f.postfix(x)
	case *cc.UnaryExpression:
		return f.unary(x)
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return f.cast(x)
		}
		return f.expr(x.UnaryExpression)
	case *cc.MultiplicativeExpression:
		ops := map[cc.MultiplicativeExpressionCase]string{cc.MultiplicativeExpressionMul: "*", cc.MultiplicativeExpressionDiv: "/", cc.MultiplicativeExpressionMod: "%"}
		return f.arith(x, ops[x.Case], x.MultiplicativeExpression, x.CastExpression)
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
		return f.shift(x, op, x.ShiftExpression, x.AdditiveExpression)
	case *cc.AndExpression:
		return f.arith(x, "&", x.AndExpression, x.EqualityExpression)
	case *cc.ExclusiveOrExpression:
		return f.arith(x, "^", x.ExclusiveOrExpression, x.AndExpression)
	case *cc.InclusiveOrExpression:
		return f.arith(x, "|", x.InclusiveOrExpression, x.ExclusiveOrExpression)
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
		return f.logical("&&", x.LogicalAndExpression, x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		return f.logical("||", x.LogicalOrExpression, x.LogicalAndExpression)
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return f.ternary(x)
		}
		return f.expr(x.LogicalOrExpression)
	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return f.expr(x.ConditionalExpression)
		}
		return f.assignValue(x)
	}
	f.no(e, "an expression %T", e)
	return rv{}
}

// block is a value whose statements run first: a Rust block.
func (f *rfn) block(stmts []string, v rv) rv {
	if len(stmts) == 0 {
		return v
	}
	s := "{ " + strings.Join(stmts, " ") + " " + unparenRs(v.s) + " }"
	if v.ty == "()" {
		s = "{ " + strings.Join(stmts, " ") + " }"
	}
	return rv{s: s, ty: v.ty, prec: pLow}
}

// stmtsOf is what e does, as statements: the comma's left operands.
func (f *rfn) stmtsOf(e cc.ExpressionNode) []string {
	b := f.capture(func() { f.exprStmt(e) })
	var out []string
	for _, l := range strings.Split(b, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (f *rfn) primary(x *cc.PrimaryExpression) rv {
	switch x.Case {
	case cc.PrimaryExpressionIdent:
		switch d := x.ResolvedTo().(type) {
		case *cc.Declarator:
			if d.Type() != nil && d.Type().Kind() == cc.Function {
				return f.fnValue(d)
			}
			t := d.Type()
			if t != nil && t.Kind() == cc.Array {
				return f.decay(f.place(x), t)
			}
			return rv{s: f.place(x), ty: f.r.ty(x.Type())}
		case *cc.Enumerator:
			v, _ := intValue(d.Value())
			k, _ := scalarKind(x.Type())
			return f.konst(x, truncK(v, k), rsKind(k))
		}
		f.no(x, "an identifier %s", x.Token.SrcStr())
	case cc.PrimaryExpressionExpr:
		v := f.expr(x.ExpressionList)
		return v
	case cc.PrimaryExpressionString:
		sv, ok := x.Value().(cc.StringValue)
		if !ok {
			f.no(x, "a wide string")
		}
		b := []byte(string(sv))
		if len(b) == 0 || b[len(b)-1] != 0 {
			b = append(b, 0)
		}
		ty := f.vty(x.Type())
		return rv{s: rsBytes(b) + ".as_ptr() as " + ty, ty: ty, prec: pAs, str: b}
	}
	f.no(x, "a primary expression %v", x.Case)
	return rv{}
}

// decay is an array's value: its first element's address.
func (f *rfn) decay(place string, t cc.Type) rv {
	return rv{s: "decay(&raw mut " + place + ")", ty: "*mut " + f.r.rtype(elemOf(t), false, "")}
}

// fnValue is a function's address.
func (f *rfn) fnValue(d *cc.Declarator) rv {
	ty := f.r.fnType(d.Type(), false)
	name, ok := f.r.fnName[d.Name()]
	if !ok {
		name = f.r.host + "::" + rsName(d.Name())
	}
	return rv{s: "Some(" + name + " as " + strings.TrimSuffix(strings.TrimPrefix(ty, "Option<"), ">") + ")", ty: ty}
}

// --- places -----------------------------------------------------------------

// place is an lvalue as a Rust place.
func (f *rfn) place(e cc.ExpressionNode) string {
	if f.sub != nil {
		if r, ok := f.sub[e]; ok {
			if r.v != nil {
				return f.vars[r.v].name
			}
			e = r.n
		}
	}
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			d, ok := x.ResolvedTo().(*cc.Declarator)
			if !ok {
				f.no(x, "a place that is no object")
			}
			if l := f.local[d]; l != nil {
				return l.name
			}
			if fl, ok := f.r.field[f.r.g.a.declKey(d)]; ok {
				return "(*ed)." + fl
			}
			f.no(x, "an object of no place: %s", d.Name())
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			return "*" + wrap(f.elemAddr(x), pPrim)
		case cc.PostfixExpressionSelect:
			return f.placeOf(x.PostfixExpression) + "." + rsName(x.Token2.SrcStr())
		case cc.PostfixExpressionPSelect:
			p := f.expr(x.PostfixExpression)
			return "(*" + wrap(p, pPrim) + ")." + rsName(x.Token2.SrcStr())
		case cc.PostfixExpressionComplit:
			a := f.complit(x)
			return "*" + wrap(a, pPrim)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			p := f.expr(x.CastExpression)
			if isFnTy(p.ty) {
				f.no(x, "a function's place")
			}
			return "*" + wrap(p, pPrim)
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionUnary {
			return f.place(x.UnaryExpression)
		}
	}
	f.no(e, "a place %T", e)
	return ""
}

// placeOf is a struct's place where a member of it is selected: a struct
// value that is no lvalue -- a call's result -- is its value.
func (f *rfn) placeOf(e cc.ExpressionNode) string {
	ue := unparenE(e)
	if p, ok := ue.(*cc.PostfixExpression); ok && p.Case == cc.PostfixExpressionCall {
		return wrap(f.expr(e), pPrim)
	}
	if a, ok := ue.(*cc.AssignmentExpression); ok && a.Case != cc.AssignmentExpressionCond {
		return wrap(f.expr(e), pPrim)
	}
	if c, ok := ue.(*cc.ConditionalExpression); ok && c.Case == cc.ConditionalExpressionCond {
		return wrap(f.expr(e), pPrim)
	}
	s := f.place(e)
	if strings.HasPrefix(s, "*") {
		return "(" + s + ")"
	}
	return s
}

// addrOf is an lvalue's address.
func (f *rfn) addrOf(e cc.ExpressionNode) rv {
	e = unparenE(e)
	ty := "*mut " + f.r.rtype(e.Type(), false, "")
	if e.Type() != nil && e.Type().Kind() == cc.Void {
		ty = "*mut c_void"
	}
	switch x := e.(type) {
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			p := f.expr(x.CastExpression)
			return f.conv(p, ty)
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			return f.elemAddr(x)
		case cc.PostfixExpressionComplit:
			return f.complit(x)
		}
	}
	return rv{s: "&raw mut " + f.place(e), ty: ty, prec: pUnary}
}

// elemAddr is a[i]'s address: the array's, or the pointer's, walked.
func (f *rfn) elemAddr(x *cc.PostfixExpression) rv {
	be, ie := cc.ExpressionNode(x.PostfixExpression), cc.ExpressionNode(x.ExpressionList)
	if !isPtrish(be.Type()) {
		be, ie = ie, be // i[a]
	}
	bt := be.Type()
	var base rv
	if bt.Kind() == cc.Array {
		// an array: in place, its first element's address
		base = rv{s: "decay(&raw mut " + f.place(be) + ")", ty: "*mut " + f.r.rtype(elemOf(bt), false, "")}
	} else {
		base = f.expr(be)
	}
	return f.ptrAdd(base, f.expr(ie), false)
}

// ptrAdd is p + n (or p - n): n elements on from p.
func (f *rfn) ptrAdd(p, n rv, sub bool) rv {
	if n.konst {
		k := n.kv
		if sub {
			k = -k
		}
		switch {
		case k == 0:
			return p
		case k > 0:
			return rv{s: wrap(p, pPrim) + ".wrapping_add(" + strconv.FormatInt(k, 10) + ")", ty: p.ty}
		default:
			return rv{s: wrap(p, pPrim) + ".wrapping_sub(" + strconv.FormatInt(-k, 10) + ")", ty: p.ty}
		}
	}
	if !isSigned(n.ty) {
		m := "wrapping_add"
		if sub {
			m = "wrapping_sub"
		}
		return rv{s: wrap(p, pPrim) + "." + m + "(" + unparenRs(f.conv(n, "usize").s) + ")", ty: p.ty}
	}
	off := unparenRs(f.conv(n, "isize").s)
	if sub {
		off = "(" + off + ").wrapping_neg()"
		if n.prec <= pAs {
			off = wrap(f.conv(n, "isize"), pPrim) + ".wrapping_neg()"
		}
	}
	return rv{s: wrap(p, pPrim) + ".wrapping_offset(" + off + ")", ty: p.ty}
}

// --- operators --------------------------------------------------------------

func (f *rfn) postfix(x *cc.PostfixExpression) rv {
	switch x.Case {
	case cc.PostfixExpressionPrimary:
		return f.expr(x.PrimaryExpression)
	case cc.PostfixExpressionIndex, cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
		t := x.Type()
		if x.Case == cc.PostfixExpressionIndex {
			// the element's own type: an element that is an array is its
			// address (grid[i] of int grid[3][4])
			be := x.PostfixExpression
			if !isPtrish(be.Type()) {
				be = x.ExpressionList
			}
			if et := elemOf(be.Type()); et != nil {
				t = et
			}
		} else {
			if fl := x.Field(); fl != nil && fl.Type() != nil {
				t = fl.Type()
			}
		}
		if t != nil && t.Kind() == cc.Array {
			if x.Case == cc.PostfixExpressionIndex {
				return f.conv(f.elemAddr(x), f.vty(t))
			}
			return f.decay(f.placeOf(x), t)
		}
		if x.Case == cc.PostfixExpressionIndex {
			a := f.elemAddr(x)
			return rv{s: "*" + wrap(a, pPrim), ty: f.r.ty(t), prec: pUnary}
		}
		if x.Case == cc.PostfixExpressionSelect {
			return rv{s: f.placeOf(x.PostfixExpression) + "." + rsName(x.Token2.SrcStr()), ty: f.r.ty(t)}
		}
		return rv{s: f.place(x), ty: f.r.ty(t)}
	case cc.PostfixExpressionCall:
		return f.call(x)
	case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
		return f.incDecValue(x.PostfixExpression, x.Case == cc.PostfixExpressionInc, true)
	case cc.PostfixExpressionComplit:
		a := f.complit(x)
		t := x.TypeName.Type()
		if t.Kind() == cc.Array {
			return f.conv(a, f.vty(t))
		}
		return rv{s: "*" + wrap(a, pPrim), ty: f.r.ty(t), prec: pUnary}
	}
	f.no(x, "a postfix expression %v", x.Case)
	return rv{}
}

// complit is a compound literal's address: a variable of the function's,
// given its value where C evaluates it.
func (f *rfn) complit(x *cc.PostfixExpression) rv {
	t := x.TypeName.Type()
	in := &cc.Initializer{Case: cc.InitializerInitList, InitializerList: x.InitializerList, Token: x.Token}
	if f.global {
		// in a file-scope object's initializer: an object of its own in the
		// editor, as long-lived as the object that points at it
		n := fmt.Sprintf("lit_%d", len(f.r.objects))
		f.r.objects = append(f.r.objects, &robj{key: "complit:" + n, field: n, t: t, what: n})
		f.r.field["complit:"+n] = n
		b := f.capture(func() { f.initInto("(*ed)."+n, t, in) })
		return f.block(rsLines(b), rv{s: "&raw mut (*ed)." + n, ty: "*mut " + f.r.ty(t), prec: pUnary})
	}
	n := f.temp(t)
	b := f.capture(func() {
		f.line("%s = %s;", n, f.zero(t))
		f.initInto(n, t, in)
	})
	return f.block(rsLines(b), rv{s: "&raw mut " + n, ty: "*mut " + f.r.ty(t), prec: pUnary})
}

// rsLines are a capture's statements, one a line.
func rsLines(b string) []string {
	var out []string
	for _, l := range strings.Split(b, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (f *rfn) unary(x *cc.UnaryExpression) rv {
	switch x.Case {
	case cc.UnaryExpressionPostfix:
		return f.expr(x.PostfixExpression)
	case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
		return f.incDecValue(x.UnaryExpression, x.Case == cc.UnaryExpressionInc, false)
	case cc.UnaryExpressionAddrof:
		if d := fnDesignator(x.CastExpression); d != nil {
			return f.fnValue(d)
		}
		a := f.addrOf(x.CastExpression)
		return f.conv(a, f.vty(x.Type()))
	case cc.UnaryExpressionDeref:
		t := x.Type()
		if t != nil && t.Kind() == cc.Function || isFnPtr(x.CastExpression.Type()) {
			return f.expr(x.CastExpression) // *fp is fp
		}
		p := f.expr(x.CastExpression)
		if t != nil && t.Kind() == cc.Array {
			return f.conv(p, f.vty(t))
		}
		return rv{s: "*" + wrap(p, pPrim), ty: f.r.ty(t), prec: pUnary}
	case cc.UnaryExpressionPlus:
		return f.conv(f.expr(x.CastExpression), f.vty(x.Type()))
	case cc.UnaryExpressionMinus:
		ty := f.vty(x.Type())
		v := f.convT(f.expr(x.CastExpression), ty)
		return rv{s: wrap(v, pPrim) + ".wrapping_neg()", ty: ty}
	case cc.UnaryExpressionCpl:
		ty := f.vty(x.Type())
		v := f.convT(f.expr(x.CastExpression), ty)
		return rv{s: "!" + wrap(v, pUnary), ty: ty, prec: pUnary}
	case cc.UnaryExpressionNot:
		v := f.expr(x.CastExpression)
		n := rsNot(f.truth(v), v)
		return rv{s: n, ty: "bool", prec: rsPrec(n)}
	}
	f.no(x, "a unary expression %v", x.Case)
	return rv{}
}

// rsNot is !c: a comparison turned round where it is one.
func rsNot(c string, v rv) string {
	if x := strings.TrimSuffix(c, " != 0"); x != c && rsBalanced(x) && rsPrec(x) < pCmp {
		// x != 0 turned round, when x is one operand of it
		return x + " == 0"
	}
	if strings.HasPrefix(c, "!") && strings.HasSuffix(c, ".is_null()") && !strings.ContainsAny(c[1:], " ") {
		return c[1:]
	}
	if c == "true" {
		return "false"
	}
	if c == "false" {
		return "true"
	}
	if isPrimaryRs(c) {
		return "!" + c
	}
	return "!(" + c + ")"
}

// isPrimaryRs says s is one operand: a name, a call, a member.
func isPrimaryRs(s string) bool {
	if s == "" || strings.HasPrefix(s, "*") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "!") || strings.HasPrefix(s, "&") || strings.HasPrefix(s, "{") {
		return false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '"':
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case ' ', '+', '-', '*', '/', '%', '<', '>', '=', '&', '|', '^':
			if depth == 0 {
				return false
			}
		}
	}
	return true
}

func (f *rfn) cast(x *cc.CastExpression) rv {
	t := x.Type()
	if t == nil || t.Kind() == cc.Void {
		return f.voidValue(x.CastExpression)
	}
	v := f.expr(x.CastExpression)
	return f.conv(v, f.vty(t))
}

// voidValue is an expression evaluated for what it does: a value of ().
func (f *rfn) voidValue(e cc.ExpressionNode) rv {
	stmts := f.stmtsOf(e)
	if len(stmts) == 0 {
		return rv{s: "()", ty: "()"}
	}
	return rv{s: "{ " + strings.Join(stmts, " ") + " }", ty: "()", prec: pLow}
}

// usual is the type C computes a binary operation of two integer operands
// in.
func (f *rfn) usual(l, r cc.ExpressionNode) jk {
	lk, ok1 := scalarKind(l.Type())
	rk, ok2 := scalarKind(r.Type())
	if !ok1 || !ok2 {
		f.no(l, "an arithmetic of %v and %v", l.Type(), r.Type())
	}
	return usualK(lk, rk)
}

var rsMethod = map[string]string{"+": "wrapping_add", "-": "wrapping_sub", "*": "wrapping_mul", "/": "wrapping_div", "%": "wrapping_rem"}

// arith is an arithmetic or bitwise operation of two integers.
func (f *rfn) arith(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) rv {
	k := f.usual(le, re)
	ty := rsKind(k)
	return f.binop(op, f.expr(le), f.expr(re), ty)
}

// binop is a op b in ty, both operands converted to it.
func (f *rfn) binop(op string, a, b rv, ty string) rv {
	if m, ok := rsMethod[op]; ok {
		av := f.convT(a, ty)
		bv := f.conv(b, ty)
		return rv{s: wrap(av, pPrim) + "." + m + "(" + unparenRs(bv.s) + ")", ty: ty}
	}
	prec := map[string]int{"&": pBitAnd, "^": pXor, "|": pBitOr}[op]
	av := f.convT(a, ty)
	bv := f.conv(b, ty)
	return rv{s: wrap(av, prec) + " " + op + " " + wrap(bv, prec-1), ty: ty, prec: prec}
}

func (f *rfn) shift(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) rv {
	lk, ok := scalarKind(le.Type())
	if !ok {
		f.no(x, "a shift of %v", le.Type())
	}
	ty := rsKind(promote(lk))
	a := f.convT(f.expr(le), ty)
	n := f.expr(re)
	if n.konst && n.kv >= 0 && n.kv < int64(tyKind(ty).size*8) {
		return rv{s: shiftOperand(a) + " " + op + " " + strconv.FormatInt(n.kv, 10), ty: ty, prec: pShift}
	}
	m := "wrapping_shl"
	if op == ">>" {
		m = "wrapping_shr"
	}
	return rv{s: wrap(a, pPrim) + "." + m + "(" + unparenRs(f.conv(n, "u32").s) + ")", ty: ty}
}

// shiftOperand is a shift's left operand: a cast in parentheses, which
// Rust would read as a type's generic arguments before <<.
func shiftOperand(v rv) string {
	if v.prec == pAs {
		return "(" + v.s + ")"
	}
	return wrap(v, pAdd)
}

// additive is + or -: of integers, or a pointer and an integer, or two
// pointers' difference.
func (f *rfn) additive(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) rv {
	lt, rt := le.Type(), re.Type()
	switch {
	case isPtrish(lt) && isPtrish(rt):
		a, b := f.expr(le), f.expr(re)
		b = f.conv(b, a.ty)
		return f.conv(rv{s: "pdiff(" + unparenRs(a.s) + ", " + unparenRs(b.s) + ")", ty: "i64"}, f.vty(x.Type()))
	case isPtrish(lt):
		return f.ptrAdd(f.expr(le), f.expr(re), op == "-")
	case isPtrish(rt):
		return f.ptrAdd(f.expr(re), f.expr(le), false)
	}
	return f.arith(x, op, le, re)
}

// compare is a comparison: of integers in their usual type, or of
// pointers.
func (f *rfn) compare(op string, le, re cc.ExpressionNode) rv {
	lt, rt := le.Type(), re.Type()
	if isPtrish(lt) || isPtrish(rt) || lt.Kind() == cc.Function || rt.Kind() == cc.Function {
		a, b := f.expr(le), f.expr(re)
		if (op == "==" || op == "!=") && (nullish(a) || nullish(b)) {
			p := a
			if nullish(a) {
				p = b
			}
			if isFnTy(p.ty) {
				if op == "==" {
					return rv{s: wrap(p, pPrim) + ".is_none()", ty: "bool"}
				}
				return rv{s: wrap(p, pPrim) + ".is_some()", ty: "bool"}
			}
			if op == "==" {
				return rv{s: wrap(p, pPrim) + ".is_null()", ty: "bool"}
			}
			return rv{s: "!" + wrap(p, pPrim) + ".is_null()", ty: "bool", prec: pUnary}
		}
		if isFnTy(a.ty) || isFnTy(b.ty) {
			a = rv{s: "fn_addr(" + a.s + ")", ty: "usize"}
			b = rv{s: "fn_addr(" + b.s + ")", ty: "usize"}
		} else if isIntTy(a.ty) {
			a = f.conv(a, b.ty)
		} else {
			b = f.conv(b, a.ty)
		}
		return rv{s: f.cmpOperand(a, true) + " " + op + " " + f.cmpOperand(b, false), ty: "bool", prec: pCmp}
	}
	k := f.usual(le, re)
	ty := rsKind(k)
	a, b := f.expr(le), f.expr(re)
	if a.ty == "bool" && b.ty == "bool" && (op == "==" || op == "!=") {
		return rv{s: wrap(a, pCmp-1) + " " + op + " " + wrap(b, pCmp-1), ty: "bool", prec: pCmp}
	}
	// a narrow value and a constant it can hold: compared in its own type,
	// which says the same (`*p == b'-'`, not `*p as i32 == 45`)
	if !a.konst && b.konst && isIntTy(a.ty) && a.ty != ty && truncK(b.kv, tyKind(a.ty)) == b.kv {
		ty = a.ty
	} else if a.konst && !b.konst && isIntTy(b.ty) && b.ty != ty && truncK(a.kv, tyKind(b.ty)) == a.kv {
		ty = b.ty
	}
	av, bv := f.conv(a, ty), f.conv(b, ty)
	if av.konst && bv.konst {
		av = f.convT(a, ty)
	}
	return rv{s: f.cmpOperand(av, true) + " " + op + " " + f.cmpOperand(bv, false), ty: "bool", prec: pCmp}
}

// cmpOperand is an operand of a comparison: a cast on the left in
// parentheses, which Rust would read as a type's generic arguments.
func (f *rfn) cmpOperand(v rv, left bool) string {
	if left && v.prec == pAs {
		return "(" + v.s + ")"
	}
	return wrap(v, pCmp-1)
}

func (f *rfn) logical(op string, le, re cc.ExpressionNode) rv {
	a := f.truth(f.expr(le))
	b := f.truth(f.expr(re))
	prec := pAnd
	if op == "||" {
		prec = pOr
	}
	av := rv{s: a, prec: rsPrec(a)}
	bv := rv{s: b, prec: rsPrec(b)}
	return rv{s: wrap(av, prec) + " " + op + " " + wrap(bv, prec-1), ty: "bool", prec: prec}
}

// rsPrec guesses the precedence of a Rust bool expression's text.
func rsPrec(s string) int {
	if isPrimaryRs(s) {
		return pPrim
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "if ") {
		return pLow
	}
	depth := 0
	worst := pPrim
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '"':
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		default:
			if depth != 0 {
				continue
			}
			switch {
			case strings.HasPrefix(s[i:], " || "):
				worst = max(worst, pOr)
			case strings.HasPrefix(s[i:], " && "):
				worst = max(worst, pAnd)
			case strings.HasPrefix(s[i:], " == "), strings.HasPrefix(s[i:], " != "), strings.HasPrefix(s[i:], " < "),
				strings.HasPrefix(s[i:], " > "), strings.HasPrefix(s[i:], " <= "), strings.HasPrefix(s[i:], " >= "):
				worst = max(worst, pCmp)
			case strings.HasPrefix(s[i:], " as "):
				worst = max(worst, pAs)
			case c == ' ':
				worst = max(worst, pBitOr)
			case i == 0 && (c == '!' || c == '*' || c == '-'):
				worst = max(worst, pUnary)
			}
		}
	}
	return worst
}

func (f *rfn) ternary(x *cc.ConditionalExpression) rv {
	ty := f.vty(x.Type())
	if x.Type() == nil || x.Type().Kind() == cc.Void {
		ty = "()"
	}
	if x.ExpressionList == nil {
		// a ?: b -- gcc's: a, unless it is zero
		f.no(x, "a ?: with no middle operand")
	}
	c := unparenRs(f.truth(f.expr(x.LogicalOrExpression)))
	if ty == "()" {
		a := f.voidValue(x.ExpressionList)
		b := f.voidValue(x.ConditionalExpression)
		return rv{s: "if " + c + " " + rsBraced(a.s) + " else " + rsBraced(b.s), ty: "()", prec: pLow}
	}
	a := f.conv(f.expr(x.ExpressionList), ty)
	b := f.conv(f.expr(x.ConditionalExpression), ty)
	if a.konst && b.konst {
		a = f.convT(a, ty)
	}
	return rv{s: "if " + c + " { " + unparenRs(a.s) + " } else { " + unparenRs(b.s) + " }", ty: ty, prec: pLow}
}

// rsBraced is a value as a block's body: braced once.
func rsBraced(s string) string {
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && rsBalanced(s[1:len(s)-1]) {
		return s
	}
	if s == "()" {
		return "{}"
	}
	return "{ " + s + "; }"
}

// --- calls ------------------------------------------------------------------

func (f *rfn) call(x *cc.PostfixExpression) rv {
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	var ft *cc.FunctionType
	var head string
	d := fnDesignator(x.PostfixExpression)
	if d != nil && d.Name() == "__builtin_expect" && len(args) == 2 {
		return f.expr(args[0]) // gcc's hint: its value is its first argument's
	}
	if d != nil {
		ft, _ = d.Type().(*cc.FunctionType)
		if n, ok := f.r.fnName[d.Name()]; ok {
			head = n
		} else {
			head = f.r.host + "::" + rsName(d.Name())
		}
	} else {
		p := f.expr(x.PostfixExpression)
		t := x.PostfixExpression.Type()
		if pt, ok := t.(*cc.PointerType); ok {
			t = pt.Elem()
		}
		ft, _ = t.(*cc.FunctionType)
		head = "(" + wrap(p, pPrim) + ".unwrap())"
	}
	if ft == nil {
		f.no(x, "a call of no function type")
	}
	params := ft.Parameters()
	if len(params) == 1 && (params[0].Type() == nil || params[0].Type().Kind() == cc.Void) {
		params = nil
	}
	vals := []string{"ed"}
	var rest []string
	for i, a := range args {
		if i < len(params) {
			pt := params[i].Type()
			if pt.Kind() == cc.Array {
				pt = pt.Decay()
			}
			v := f.conv(f.expr(a), f.r.ty(pt))
			vals = append(vals, unparenRs(v.s))
			continue
		}
		v := f.expr(a)
		switch {
		case v.ty == "bool":
			rest = append(rest, "VArg::I("+wrap(v, pUnary)+" as i64)")
		case isPtrTy(v.ty):
			rest = append(rest, "VArg::P("+unparenRs(f.conv(v, "*mut c_void").s)+")")
		case isIntTy(v.ty) && isSigned(promoteTy(v.ty)):
			rest = append(rest, "VArg::I("+unparenRs(f.conv(v, "i64").s)+")")
		case isIntTy(v.ty):
			rest = append(rest, "VArg::U("+unparenRs(f.conv(v, "u64").s)+")")
		default:
			f.no(a, "a variadic argument of %s", v.ty)
		}
	}
	if ft.IsVariadic() {
		vals = append(vals, "&["+strings.Join(rest, ", ")+"]")
	}
	ty := "()"
	if rt := ft.Result(); rt != nil && rt.Kind() != cc.Void {
		ty = f.r.ty(rt)
	}
	return rv{s: head + "(" + strings.Join(vals, ", ") + ")", ty: ty}
}

// promoteTy is C's integer promotion of a Rust integer type.
func promoteTy(ty string) string { return rsKind(promote(tyKind(ty))) }

// --- assignments ------------------------------------------------------------

// assignOps are the compound assignments' operators.
var assignOps = map[cc.AssignmentExpressionCase]string{
	cc.AssignmentExpressionMul: "*", cc.AssignmentExpressionDiv: "/", cc.AssignmentExpressionMod: "%",
	cc.AssignmentExpressionAdd: "+", cc.AssignmentExpressionSub: "-", cc.AssignmentExpressionLsh: "<<",
	cc.AssignmentExpressionRsh: ">>", cc.AssignmentExpressionAnd: "&", cc.AssignmentExpressionXor: "^",
	cc.AssignmentExpressionOr: "|",
}

// lval is an lvalue to read and write: its place, evaluated once -- through
// a pointer when evaluating it does something.
type lval struct {
	pre   []string // the statements that make the pointer
	place string
	ty    string // the Rust type of its value
	t     cc.Type
}

func (f *rfn) lval(e cc.ExpressionNode) lval {
	t := e.Type()
	ty := f.r.ty(t)
	if !hasEffect(e) || f.lowered_ {
		return lval{place: f.place(e), ty: ty, t: t}
	}
	a := f.addrOf(e)
	n := f.temp(nil)
	f.retype(n, "*mut "+ty)
	return lval{pre: []string{n + " = " + unparenRs(a.s) + ";"}, place: "*" + n, ty: ty, t: t}
}

// retype gives a temporary its Rust type.
func (f *rfn) retype(n, ty string) {
	for _, l := range f.order {
		if l.name == n {
			l.rty = ty
		}
	}
}

func (lv lval) read() rv {
	p := pPrim
	if strings.HasPrefix(lv.place, "*") {
		p = pUnary
	}
	return rv{s: lv.place, ty: lv.ty, prec: p}
}

// assigned is the new value of lv for C's assignment e (=, op=), as
// statements: lv's pointer, and the store.  It returns them, and the
// value stored.
func (f *rfn) assigned(x *cc.AssignmentExpression) ([]string, lval) {
	lv := f.lval(x.UnaryExpression)
	stmts := append([]string{}, lv.pre...)
	if x.Case == cc.AssignmentExpressionAssign {
		v := f.conv(f.expr(x.AssignmentExpression), lv.ty)
		f.markWrite(x.UnaryExpression)
		return append(stmts, lv.place+" = "+unparenRs(v.s)+";"), lv
	}
	op := assignOps[x.Case]
	r := f.expr(x.AssignmentExpression)
	if hasEffect(x.AssignmentExpression) && !r.konst {
		// the right side's calls first, and then the lvalue read: gcc's
		// order
		t := f.temp(x.AssignmentExpression.Type())
		stmts = append(stmts, t+" = "+unparenRs(r.s)+";")
		r = rv{s: t, ty: r.ty}
	}
	nv := f.compound(lv, op, r, x.AssignmentExpression.Type(), x)
	v := f.conv(nv, lv.ty)
	f.markWrite(x.UnaryExpression)
	return append(stmts, lv.place+" = "+unparenRs(v.s)+";"), lv
}

// compound is lv op r's value: C's lv = (T)(lv op r), the operation in
// the operands' usual type -- or in lv's own, where the low bits are the
// same (+ - * & | ^ of integers).
func (f *rfn) compound(lv lval, op string, r rv, rt cc.Type, x cc.Node) rv {
	cur := lv.read()
	if isPtrTy(lv.ty) {
		return f.ptrAdd(cur, r, op == "-")
	}
	lk, ok1 := scalarKind(lv.t)
	rk, ok2 := scalarKind(rt)
	if !ok1 || !ok2 {
		f.no(x, "a compound assignment of %v and %v", lv.t, rt)
	}
	ty := rsKind(usualK(lk, rk))
	switch op {
	case "+", "-", "*", "&", "|", "^":
		if lv.ty != "bool" {
			ty = lv.ty
		}
	case "<<", ">>":
		ty = rsKind(promote(lk))
	}
	return f.opIn(op, cur, r, ty, x)
}

// opIn is a op b computed in ty.
func (f *rfn) opIn(op string, a, b rv, ty string, x cc.Node) rv {
	if op == "<<" || op == ">>" {
		av := f.convT(a, ty)
		if b.konst && b.kv >= 0 && b.kv < int64(tyKind(ty).size*8) {
			return rv{s: shiftOperand(av) + " " + op + " " + strconv.FormatInt(b.kv, 10), ty: ty, prec: pShift}
		}
		m := "wrapping_shl"
		if op == ">>" {
			m = "wrapping_shr"
		}
		return rv{s: wrap(av, pPrim) + "." + m + "(" + unparenRs(f.conv(b, "u32").s) + ")", ty: ty}
	}
	return f.binop(op, a, b, ty)
}

// markWrite notes that a local is written: `mut`.
func (f *rfn) markWrite(e cc.ExpressionNode) {
	if l := f.identLocal(e); l != nil {
		l.mutated = true
	}
}

// assignValue is an assignment as a value: the value stored.
func (f *rfn) assignValue(x *cc.AssignmentExpression) rv {
	stmts, lv := f.assigned(x)
	return f.block(stmts, lv.read())
}

// incDec is ++ or -- of lv's value cur: its new value.
func (f *rfn) incDec(cur rv, t cc.Type, inc bool) rv {
	ty := cur.ty
	switch {
	case isPtrTy(ty):
		m := "wrapping_add"
		if !inc {
			m = "wrapping_sub"
		}
		return rv{s: wrap(cur, pPrim) + "." + m + "(1)", ty: ty}
	case ty == "bool":
		if inc {
			return rv{s: "true", ty: "bool"}
		}
		return rv{s: "!" + wrap(cur, pUnary), ty: "bool", prec: pUnary}
	}
	m := "wrapping_add"
	if !inc {
		m = "wrapping_sub"
	}
	return rv{s: wrap(cur, pPrim) + "." + m + "(1)", ty: ty}
}

// incDecValue is x++ (post) or ++x as a value.
func (f *rfn) incDecValue(e cc.ExpressionNode, inc, post bool) rv {
	lv := f.lval(e)
	f.markWrite(e)
	stmts := append([]string{}, lv.pre...)
	if post {
		t := f.temp(lv.t)
		stmts = append(stmts, t+" = "+lv.place+";")
		nv := f.incDec(rv{s: t, ty: lv.ty}, lv.t, inc)
		stmts = append(stmts, lv.place+" = "+unparenRs(nv.s)+";")
		return f.block(stmts, rv{s: t, ty: lv.ty})
	}
	nv := f.incDec(lv.read(), lv.t, inc)
	stmts = append(stmts, lv.place+" = "+unparenRs(nv.s)+";")
	return f.block(stmts, lv.read())
}

// --- statements of expressions ----------------------------------------------

// exprStmt writes an expression evaluated for what it does.
func (f *rfn) exprStmt(e cc.ExpressionNode) {
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.ExpressionList:
		for ; x != nil; x = x.ExpressionList {
			f.exprStmt(x.AssignmentExpression)
		}
		return
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			stmts, _ := f.assigned(x)
			for _, s := range stmts {
				f.line("%s", s)
			}
			return
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			f.incDecStmt(x.PostfixExpression, x.Case == cc.PostfixExpressionInc)
			return
		case cc.PostfixExpressionCall:
			v := f.call(x)
			if v.s != "" && hasEffect(x) {
				f.line("%s;", unparenRs(v.s))
			}
			return
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
			f.incDecStmt(x.UnaryExpression, x.Case == cc.UnaryExpressionInc)
			return
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast && (x.Type() == nil || x.Type().Kind() == cc.Void) {
			f.exprStmt(x.CastExpression)
			return
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond && x.ExpressionList != nil {
			c := unparenRs(f.truth(f.expr(x.LogicalOrExpression)))
			a := f.capture(func() { f.exprStmt(x.ExpressionList) })
			b := f.capture(func() { f.exprStmt(x.ConditionalExpression) })
			f.line("if %s {", c)
			f.out.WriteString(a)
			if strings.TrimSpace(b) != "" {
				f.line("} else {")
				f.out.WriteString(b)
			}
			f.line("}")
			return
		}
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionLAnd {
			c := unparenRs(f.truth(f.expr(x.LogicalAndExpression)))
			b := f.capture(func() { f.exprStmt(x.InclusiveOrExpression) })
			f.line("if %s {", c)
			f.out.WriteString(b)
			f.line("}")
			return
		}
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLOr {
			c := f.truth(f.expr(x.LogicalOrExpression))
			b := f.capture(func() { f.exprStmt(x.LogicalAndExpression) })
			f.line("if %s {", unparenRs(rsNot(c, rv{})))
			f.out.WriteString(b)
			f.line("}")
			return
		}
	}
	if !hasEffect(e) {
		return // nothing it does
	}
	v := f.expr(e)
	f.line("let _ = %s;", unparenRs(v.s))
}

// incDecStmt is x++ or x-- for what it does.
func (f *rfn) incDecStmt(e cc.ExpressionNode, inc bool) {
	lv := f.lval(e)
	f.markWrite(e)
	for _, s := range lv.pre {
		f.line("%s", s)
	}
	nv := f.incDec(lv.read(), lv.t, inc)
	f.line("%s = %s;", lv.place, unparenRs(nv.s))
}
