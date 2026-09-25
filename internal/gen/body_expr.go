package gen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// capture runs fn with its output going to a buffer of its own, and returns
// what it wrote.
func (f *fnEmit) capture(fn func()) string {
	old := f.out
	b := &strings.Builder{}
	f.out = b
	fn()
	f.out = old
	return b.String()
}

func unparenE(e cc.ExpressionNode) cc.ExpressionNode {
	for {
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				e = x.ExpressionList
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				e = x.AssignmentExpression
				continue
			}
		}
		return e
	}
}

// stripCasts is e without its casts.
func stripCasts(e cc.ExpressionNode) cc.ExpressionNode {
	for {
		e = unparenE(e)
		c, ok := e.(*cc.CastExpression)
		if !ok || c.Case != cc.CastExpressionCast {
			return e
		}
		e = c.CastExpression
	}
}

func isNullConst(e cc.ExpressionNode) bool {
	e = unparenE(e)
	if hasEffect(e) {
		return false
	}
	if p, ok := e.(*cc.PostfixExpression); ok && p.Case == cc.PostfixExpressionComplit {
		return false // storage of its own, whatever cc makes its value
	}
	if c, ok := e.(*cc.CastExpression); ok && c.Case == cc.CastExpressionCast {
		if t := c.Type(); t != nil && (t.Kind() == cc.Ptr) {
			return isNullConst(c.CastExpression)
		}
		return false
	}
	if t := e.Type(); t != nil && (cc.IsIntegerType(t) || t.Kind() == cc.Ptr) {
		if v := e.Value(); v != nil && fmt.Sprint(v) == "0" {
			return true
		}
	}
	return false
}

// ptrType is the Go type of a pointer- or array-valued expression, from the
// object it denotes.
func (f *fnEmit) exprGoType(e cc.ExpressionNode) string {
	t := e.Type()
	if t == nil {
		return ""
	}
	if t.Kind() == cc.Array {
		if p, ok := unparenE(e).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
			return f.ident(p).t
		}
		return f.g.goType(t, f.g.a.obj(e))
	}
	if t.Kind() == cc.Ptr {
		return f.objType(t, f.g.a.obj(e))
	}
	if s := scalar(t); s != "" {
		return s
	}
	return f.g.goType(t, "")
}

var callRe = regexp.MustCompile(`[A-Za-z_][\w]*\(`)
var pureCalls = map[string]bool{"int(": true, "int8(": true, "int16(": true, "int32(": true, "int64(": true, "uint16(": true, "uint32(": true, "uint64(": true, "byte(": true, "usize(": true,
	"At(": true, "Get(": true, "P(": true, "Nil(": true, "Add(": true, "Ref(": true, "B2i(": true, "Sub(": true}

// pure says an emitted expression calls nothing that has an effect, so it can
// be written twice.
func pure(s string) bool {
	for _, m := range callRe.FindAllString(s, -1) {
		if !pureCalls[m] {
			return false
		}
	}
	return true
}

func (f *fnEmit) index(v val) string {
	if v.boolean || f.g.canon(v.t) == "bool" {
		return "int(B2i(" + v.s + "))"
	}
	if v.konst {
		return v.s
	}
	if f.g.canon(v.t) == "int" {
		return v.s
	}
	return "int(" + v.s + ")"
}

// expr is e as a value; what it does besides is written before it.
func (f *fnEmit) expr(e cc.ExpressionNode) val {
	return f.exprTo(e, "")
}

// exprTo is expr with the Go type the value goes to, for what has no type of
// its own (a null, an allocation).
func (f *fnEmit) exprTo(e cc.ExpressionNode, to string) val {
	if isNullConst(e) && e.Type() != nil && e.Type().Kind() == cc.Ptr {
		return val{s: "nil", t: to, c: e.Type(), null: true}
	}
	v := f.exprTo1(e, to)
	if v.konst && !v.hasCv {
		switch c := e.Value().(type) {
		case cc.Int64Value:
			v.cv, v.hasCv = int64(c), true
		case cc.UInt64Value:
			v.cv, v.hasCv = int64(c), true
			if int64(c) < 0 || (e.Type() != nil && !cc.IsSignedInteger(e.Type()) && strings.Contains(v.s, "^")) {
				// Go's untyped arithmetic is not C's unsigned arithmetic
				v.s = strconv.FormatUint(uint64(c), 10)
				v.hasCv = int64(c) >= 0
			}
		}
	}
	if v.hasCv && digitSum.MatchString(v.s) {
		// `sizeof("") - 1` folds to `1 - 1`: say the number.  Named constants
		// and shifts stay as written; they carry the meaning.
		v.s = strconv.FormatInt(v.cv, 10)
	}
	return v
}

var digitSum = regexp.MustCompile(`^\(?[0-9]+ [-+] [0-9]+\)?$`)

func (f *fnEmit) exprTo1(e cc.ExpressionNode, to string) val {
	switch x := e.(type) {
	case *cc.ConstantExpression:
		return f.exprTo(x.ConditionalExpression, to)
	case *cc.ExpressionList:
		for x.ExpressionList != nil {
			f.exprStmt(x.AssignmentExpression)
			x = x.ExpressionList
		}
		return f.exprTo(x.AssignmentExpression, to)
	case *cc.PrimaryExpression:
		return f.primary(x, to)
	case *cc.PostfixExpression:
		return f.postfix(x, to)
	case *cc.UnaryExpression:
		return f.unary(x)
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return f.cast(x, to)
		}
	case *cc.MultiplicativeExpression:
		ops := map[cc.MultiplicativeExpressionCase]string{cc.MultiplicativeExpressionMul: "*", cc.MultiplicativeExpressionDiv: "/", cc.MultiplicativeExpressionMod: "%"}
		return f.binary(x, ops[x.Case], x.MultiplicativeExpression, x.CastExpression)
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
		return f.binary(x, "&", x.AndExpression, x.EqualityExpression)
	case *cc.ExclusiveOrExpression:
		return f.binary(x, "^", x.ExclusiveOrExpression, x.AndExpression)
	case *cc.InclusiveOrExpression:
		return f.binary(x, "|", x.InclusiveOrExpression, x.ExclusiveOrExpression)
	case *cc.RelationalExpression:
		ops := map[cc.RelationalExpressionCase]string{cc.RelationalExpressionLt: "<", cc.RelationalExpressionGt: ">", cc.RelationalExpressionLeq: "<=", cc.RelationalExpressionGeq: ">="}
		return f.compare(x, ops[x.Case], x.RelationalExpression, x.ShiftExpression)
	case *cc.EqualityExpression:
		op := "=="
		if x.Case == cc.EqualityExpressionNeq {
			op = "!="
		}
		return f.compare(x, op, x.EqualityExpression, x.RelationalExpression)
	case *cc.LogicalAndExpression:
		return f.logical(x, "&&", x.LogicalAndExpression, x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		return f.logical(x, "||", x.LogicalOrExpression, x.LogicalAndExpression)
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return f.ternary(x, to)
		}
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			lv := f.assign(x)
			return val{s: lv.get, t: lv.t, c: x.Type()}
		}
	}
	f.no(e, "an expression %T", e)
	return val{}
}

func (f *fnEmit) primary(x *cc.PrimaryExpression, to string) val {
	switch x.Case {
	case cc.PrimaryExpressionIdent:
		return f.ident(x)
	case cc.PrimaryExpressionInt:
		s := strings.TrimRight(x.Token.SrcStr(), "uUlL")
		return val{s: s, t: f.arith(x.Type()), c: x.Type(), konst: true}
	case cc.PrimaryExpressionChar:
		v, ok := x.Value().(cc.Int64Value)
		if !ok {
			f.no(x, "a character constant of value %T", x.Value())
		}
		s := strconv.FormatInt(int64(v), 10)
		if v >= 0x20 && v < 0x7f && v != '\'' && v != '\\' {
			s = "'" + string(rune(v)) + "'"
		}
		return val{s: s, t: "int32", c: x.Type(), konst: true}
	case cc.PrimaryExpressionString:
		sv, ok := x.Value().(cc.StringValue)
		if !ok {
			f.no(x, "a string of value %T", x.Value())
		}
		s := strings.TrimSuffix(string(sv), "\x00")
		return val{s: "S(" + goQuote(s) + ")", t: "Ptr[byte]", c: x.Type()}
	case cc.PrimaryExpressionExpr:
		v := f.exprTo(x.ExpressionList, to)
		if !v.null {
			v.s = paren(v.s)
		}
		return v
	}
	f.no(x, "a primary expression %v", x.Case)
	return val{}
}

// goQuote is a Go string literal of bytes, printable ASCII as itself.
func goQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\r':
			b.WriteString(`\r`)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\x%02x`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (f *fnEmit) fieldType(x *cc.PostfixExpression) string {
	fl := x.Field()
	if fl == nil {
		f.no(x, "a member with no field")
	}
	return f.g.goType(fl.Type(), fieldKey(fl))
}

func (f *fnEmit) postfix(x *cc.PostfixExpression, to string) val {
	switch x.Case {
	case cc.PostfixExpressionIndex:
		base := f.expr(x.PostfixExpression)
		ix := f.expr(x.ExpressionList)
		bt := f.g.canon(base.t)
		et := elemOfGo(base.t)
		switch {
		case strings.HasPrefix(bt, "Ptr["):
			if x.Type().Kind() == cc.Struct || x.Type().Kind() == cc.Union {
				return val{s: "*" + base.s + ".Ref(" + f.index(ix) + ")", t: et, c: x.Type()}
			}
			return val{s: base.s + ".At(" + f.index(ix) + ")", t: et, c: x.Type()}
		case strings.HasPrefix(bt, "["):
			return val{s: base.s + "[" + f.index(ix) + "]", t: et, c: x.Type()}
		}
		f.no(x, "an index into a %s", base.t)
	case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
		if x.Token2.SrcStr() == "ga_data" && strings.HasPrefix(f.g.canon(to), "Ptr[") {
			// read as the type it is converted to: the storage GaData makes
			return f.gadataGo(x, elemOfGo(to))
		}
		p := f.path(x)
		return val{s: p, t: f.fieldType(x), c: x.Type()}
	case cc.PostfixExpressionCall:
		return f.call(x, to)
	case cc.PostfixExpressionComplit:
		// a compound literal array: storage of its own, zeroed
		at, ok := x.TypeName.Type().(*cc.ArrayType)
		if !ok || at.Len() <= 0 {
			f.no(x, "a compound literal of a %s", x.TypeName.Type())
		}
		for l := x.InitializerList; l != nil; l = l.InitializerList {
			if l.Designation != nil || !zeroInit(l.Initializer) {
				f.no(x, "a compound literal that is not zeros")
			}
		}
		et := f.g.goType(at.Elem(), "")
		if isByteType(at.Elem()) {
			et = "byte"
		}
		return val{s: fmt.Sprintf("Mk[%s](%d)", et, at.Len()), t: "Ptr[" + et + "]", c: x.Type()}
	case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
		lv := f.lval(x.PostfixExpression)
		t := f.newTemp(lv.t)
		f.line("%s = %s", t, lv.get)
		f.incdec(lv, x.Case == cc.PostfixExpressionInc, x)
		return val{s: t, t: lv.t, c: x.Type()}
	}
	f.no(x, "a postfix expression %v", x.Case)
	return val{}
}

// path is an addressable Go expression for a struct member or a struct.
func (f *fnEmit) path(e cc.ExpressionNode) string {
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			return f.ident(x).s
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect:
			return f.path(x.PostfixExpression) + "." + GoName(x.Token2.SrcStr())
		case cc.PostfixExpressionPSelect:
			if u, ok := unparenE(x.PostfixExpression).(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
				return f.path(u.CastExpression) + "." + GoName(x.Token2.SrcStr())
			}
			b := f.expr(x.PostfixExpression)
			bt := f.g.canon(b.t)
			switch {
			case strings.HasPrefix(bt, "*"):
				return b.s + "." + GoName(x.Token2.SrcStr())
			case strings.HasPrefix(bt, "Ptr["):
				return b.s + ".P()." + GoName(x.Token2.SrcStr())
			case strings.HasPrefix(bt, "[]"):
				return b.s + "[0]." + GoName(x.Token2.SrcStr())
			}
			f.no(x, "a -> on a %s", b.t)
		case cc.PostfixExpressionCall:
			return f.call(x, "").s
		case cc.PostfixExpressionIndex:
			base := f.expr(x.PostfixExpression)
			ix := f.expr(x.ExpressionList)
			bt := f.g.canon(base.t)
			switch {
			case strings.HasPrefix(bt, "Ptr["):
				return base.s + ".Ref(" + f.index(ix) + ")"
			case strings.HasPrefix(bt, "["):
				return base.s + "[" + f.index(ix) + "]"
			}
			f.no(x, "an index into a %s", base.t)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			b := f.expr(x.CastExpression)
			bt := f.g.canon(b.t)
			switch {
			case strings.HasPrefix(bt, "*"):
				return "(*" + b.s + ")"
			case strings.HasPrefix(bt, "Ptr["):
				return b.s + ".P()"
			case strings.HasPrefix(bt, "[]"):
				return b.s + "[0]"
			}
		}
	}
	f.no(e, "a path through %T", e)
	return ""
}

// lvalue is somewhere a value is stored.
type lvalue struct {
	get string
	t   string
	set func(v string) string // the statement that stores v
	op  bool                  // get can take a compound assignment and ++
}

func (f *fnEmit) lval(e cc.ExpressionNode) lvalue {
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			v := f.ident(x)
			return lvalue{get: v.s, t: v.t, set: func(s string) string { return v.s + " = " + s }, op: true}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			p := f.path(x)
			return lvalue{get: p, t: f.fieldType(x), set: func(s string) string { return p + " = " + s }, op: true}
		case cc.PostfixExpressionIndex:
			base := f.expr(x.PostfixExpression)
			ix := f.expr(x.ExpressionList)
			i := f.index(ix)
			if !pure(i) {
				t := f.newTemp("int")
				f.line("%s = %s", t, i)
				i = t
			}
			bt := f.g.canon(base.t)
			et := elemOfGo(base.t)
			switch {
			case strings.HasPrefix(bt, "Ptr["):
				b := base.s
				return lvalue{get: b + ".At(" + i + ")", t: et, set: func(s string) string { return b + ".Set(" + i + ", " + s + ")" }}
			case strings.HasPrefix(bt, "["):
				g := base.s + "[" + i + "]"
				return lvalue{get: g, t: et, set: func(s string) string { return g + " = " + s }, op: true}
			}
			f.no(x, "a store into a %s", base.t)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			b := f.expr(x.CastExpression)
			bt := f.g.canon(b.t)
			et := f.exprGoType(x)
			switch {
			case strings.HasPrefix(bt, "Ptr["):
				return lvalue{get: b.s + ".Get()", t: et, set: func(s string) string { return b.s + ".Put(" + s + ")" }}
			case strings.HasPrefix(bt, "*"):
				g := "(*" + b.s + ")"
				return lvalue{get: g, t: et, set: func(s string) string { return "*" + b.s + " = " + s }, op: true}
			case strings.HasPrefix(bt, "["):
				g := b.s + "[0]"
				return lvalue{get: g, t: elemOfGo(b.t), set: func(s string) string { return g + " = " + s }, op: true}
			}
			f.no(x, "a store through a %s", b.t)
		}
	}
	f.no(e, "a store into %T", e)
	return lvalue{}
}

// incdec writes ++ or -- of an lvalue.
func (f *fnEmit) incdec(lv lvalue, inc bool, n cc.Node) {
	t := f.g.canon(lv.t)
	d := 1
	if !inc {
		d = -1
	}
	switch {
	case strings.HasPrefix(t, "Ptr["):
		f.line("%s", lv.set(fmt.Sprintf("%s.Add(%d)", lv.get, d)))
	case strings.HasPrefix(t, "[]") && inc:
		f.line("%s", lv.set(lv.get+"[1:]")) // a slice walks forward by reslicing
	case isIntGo(t):
		if lv.op {
			if inc {
				f.line("%s++", lv.get)
			} else {
				f.line("%s--", lv.get)
			}
			return
		}
		op := "+"
		if !inc {
			op = "-"
		}
		f.line("%s", lv.set(lv.get+" "+op+" 1"))
	default:
		f.no(n, "++ or -- of a %s", lv.t)
	}
}

// assign writes an assignment and returns where it stored.
func (f *fnEmit) assign(x *cc.AssignmentExpression) lvalue {
	lv := f.lval(x.UnaryExpression)
	if x.Case == cc.AssignmentExpressionAssign {
		r := f.exprTo(x.AssignmentExpression, lv.t)
		f.line("%s", lv.set(f.conv(r, lv.t)))
		return lv
	}
	ops := map[cc.AssignmentExpressionCase]string{cc.AssignmentExpressionMul: "*", cc.AssignmentExpressionDiv: "/", cc.AssignmentExpressionMod: "%",
		cc.AssignmentExpressionAdd: "+", cc.AssignmentExpressionSub: "-", cc.AssignmentExpressionLsh: "<<", cc.AssignmentExpressionRsh: ">>",
		cc.AssignmentExpressionAnd: "&", cc.AssignmentExpressionXor: "^", cc.AssignmentExpressionOr: "|"}
	op := ops[x.Case]
	r := f.expr(x.AssignmentExpression)
	lt := f.g.canon(lv.t)
	if strings.HasPrefix(lt, "Ptr[") && (op == "+" || op == "-") {
		n := f.index(r)
		if op == "-" {
			n = "-" + paren(n)
		}
		f.line("%s", lv.set(lv.get+".Add("+n+")"))
		return lv
	}
	if strings.HasPrefix(lt, "[]") && op == "+" {
		f.line("%s", lv.set(lv.get+"["+f.index(r)+":]")) // p += k walks a slice forward
		return lv
	}
	if !isIntGo(lt) {
		f.no(x, "%s= on a %s", op, lv.t)
	}
	rt := f.g.canon(r.t)
	var rhs string
	switch {
	case op == "<<" || op == ">>":
		rhs = r.s
		if r.boolean {
			rhs = "B2i(" + r.s + ")"
		}
	case (op == "/" || op == "%" || op == ">>") && !r.konst && usual(lt, rt) != promoted(lt) && usual(lt, rt) != lt:
		u := usual(lt, rt)
		f.line("%s", lv.set(fmt.Sprintf("%s(%s(%s) %s %s)", lv.t, u, lv.get, op, f.conv(r, u))))
		return lv
	default:
		rhs = f.conv(r, lv.t)
	}
	if lv.op {
		f.line("%s %s= %s", lv.get, op, rhs)
	} else {
		f.line("%s", lv.set(lv.get+" "+op+" "+paren(rhs)))
	}
	return lv
}

func (f *fnEmit) unary(x *cc.UnaryExpression) val {
	switch x.Case {
	case cc.UnaryExpressionPostfix:
		return f.expr(x.PostfixExpression)
	case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
		lv := f.lval(x.UnaryExpression)
		f.incdec(lv, x.Case == cc.UnaryExpressionInc, x)
		return val{s: lv.get, t: lv.t, c: x.Type()}
	case cc.UnaryExpressionAddrof:
		return f.addr(x.CastExpression)
	case cc.UnaryExpressionDeref:
		b := f.expr(x.CastExpression)
		bt := f.g.canon(b.t)
		switch {
		case strings.HasPrefix(bt, "func("):
			return b
		case strings.HasPrefix(bt, "Ptr["):
			if x.Type().Kind() == cc.Struct || x.Type().Kind() == cc.Union {
				return val{s: "*" + b.s + ".P()", t: elemOfGo(b.t), c: x.Type()}
			}
			return val{s: b.s + ".Get()", t: f.exprGoType(x), c: x.Type()}
		case strings.HasPrefix(bt, "*"):
			return val{s: "(*" + b.s + ")", t: elemOfGo(b.t), c: x.Type()}
		case strings.HasPrefix(bt, "["):
			return val{s: b.s + "[0]", t: elemOfGo(b.t), c: x.Type()}
		}
		f.no(x, "* of a %s", b.t)
	case cc.UnaryExpressionPlus:
		return f.expr(x.CastExpression)
	case cc.UnaryExpressionMinus, cc.UnaryExpressionCpl:
		v := f.expr(x.CastExpression)
		t := f.arith(x.Type())
		op := "-"
		if x.Case == cc.UnaryExpressionCpl {
			op = "^"
		}
		if !v.konst {
			return val{s: op + paren(f.conv(v, t)), t: t, c: x.Type()}
		}
		// a constant: its value is known, and conv writes it for an
		// unsigned type that the untyped negative would not fit
		return val{s: op + paren(v.s), t: t, c: x.Type(), konst: true}
	case cc.UnaryExpressionNot:
		v := f.expr(x.CastExpression)
		return val{s: f.falsity(v), t: "bool", c: x.Type(), boolean: true}
	case cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType:
		if v, ok := x.Value().(cc.UInt64Value); ok {
			return val{s: strconv.FormatUint(uint64(v), 10), t: "usize", c: x.Type(), konst: true}
		}
		if v, ok := x.Value().(cc.Int64Value); ok {
			return val{s: strconv.FormatInt(int64(v), 10), t: "usize", c: x.Type(), konst: true}
		}
	}
	f.no(x, "a unary expression %v", x.Case)
	return val{}
}

// addr is &e.
func (f *fnEmit) addr(e cc.ExpressionNode) val {
	e = unparenE(e)
	t := e.Type()
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			v := f.ident(x)
			if t.Kind() == cc.Function {
				return v
			}
			if strings.HasPrefix(f.g.canon(v.t), "Ptr[") && t.Kind() == cc.Array {
				return v
			}
			return val{s: "&" + v.s, t: "*" + v.t, c: e.Type()}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			base := f.expr(x.PostfixExpression)
			ix := f.expr(x.ExpressionList)
			bt := f.g.canon(base.t)
			switch {
			case strings.HasPrefix(bt, "Ptr["):
				if ix.konst && ix.s == "0" {
					return base
				}
				return val{s: base.s + ".Add(" + f.index(ix) + ")", t: base.t, c: e.Type()}
			case strings.HasPrefix(bt, "["):
				// an element's address can walk the array: a view of it
				v := "View(" + base.s + "[:])"
				if !(ix.konst && ix.s == "0") {
					v += ".Add(" + f.index(ix) + ")"
				}
				return val{s: v, t: "Ptr[" + elemOfGo(base.t) + "]", c: e.Type()}
			}
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			p := f.path(x)
			ft := f.fieldType(x)
			return val{s: "&" + p, t: "*" + ft, c: e.Type()}
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			return f.expr(x.CastExpression)
		}
	}
	f.no(e, "& of %T", e)
	return val{}
}

func (f *fnEmit) arithOperands(l, r val, t string) (string, string) {
	return paren(f.conv(l, t)), paren(f.conv(r, t))
}

func (f *fnEmit) binary(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) val {
	l, r := f.expr(le), f.expr(re)
	t := f.arith(x.Type())
	a, b := f.arithOperands(l, r, t)
	return val{s: a + " " + op + " " + b, t: t, c: x.Type(), konst: l.konst && r.konst}
}

func (f *fnEmit) shift(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) val {
	l, r := f.expr(le), f.expr(re)
	t := f.arith(x.Type())
	a := l.s
	if !l.konst {
		a = f.conv(l, t)
	}
	b := r.s
	if r.boolean {
		b = "B2i(" + r.s + ")"
	}
	return val{s: paren(a) + " " + op + " " + paren(b), t: t, c: x.Type(), konst: l.konst && r.konst}
}

func (f *fnEmit) additive(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) val {
	lt, rt := le.Type(), re.Type()
	lp := lt != nil && (lt.Kind() == cc.Ptr || lt.Kind() == cc.Array)
	rp := rt != nil && (rt.Kind() == cc.Ptr || rt.Kind() == cc.Array)
	switch {
	case lp && rp && op == "-":
		l, r := f.expr(le), f.expr(re)
		pt := l.t
		if !strings.HasPrefix(f.g.canon(pt), "Ptr[") {
			pt = r.t
		}
		return val{s: "int64(" + f.conv(l, pt) + ".Sub(" + f.conv(r, pt) + "))", t: "int64", c: x.Type()}
	case lp || rp:
		pe, ne := le, re
		if rp {
			pe, ne = re, le
		}
		p, n := f.expr(pe), f.expr(ne)
		if strings.HasPrefix(f.g.canon(p.t), "[]") && op == "+" {
			// a slice that walks forward: p + k is p[k:]
			return val{s: p.s + "[" + f.index(n) + ":]", t: p.t, c: x.Type()}
		}
		if strings.HasPrefix(f.g.canon(p.t), "[]") {
			p = val{s: "View(" + p.s + ")", t: "Ptr[" + elemOfGo(p.t) + "]", c: p.c}
		}
		if !strings.HasPrefix(f.g.canon(p.t), "Ptr[") {
			if strings.HasPrefix(f.g.canon(p.t), "[") {
				p = val{s: "View(" + p.s + "[:])", t: "Ptr[" + elemOfGo(p.t) + "]", c: p.c}
			} else {
				f.no(x, "arithmetic on a %s", p.t)
			}
		}
		i := f.index(n)
		if op == "-" {
			i = "-" + paren(i)
		}
		return val{s: p.s + ".Add(" + i + ")", t: p.t, c: x.Type()}
	}
	return f.binary(x, op, le, re)
}

func (f *fnEmit) compare(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) val {
	lt, rt := le.Type(), re.Type()
	lp := lt != nil && (lt.Kind() == cc.Ptr || lt.Kind() == cc.Array || lt.Kind() == cc.Function)
	rp := rt != nil && (rt.Kind() == cc.Ptr || rt.Kind() == cc.Array || rt.Kind() == cc.Function)
	if lp || rp {
		if isNullConst(re) || isNullConst(le) {
			pe := le
			if isNullConst(le) {
				pe = re
			}
			p := f.expr(pe)
			s := f.falsity(p)
			if op == "!=" {
				s = f.truth(p)
			}
			return val{s: s, t: "bool", c: x.Type(), boolean: true}
		}
		l, r := f.expr(le), f.expr(re)
		t := l.t
		if strings.HasPrefix(f.g.canon(r.t), "Ptr[") {
			t = r.t
		}
		a, b := f.conv(l, t), f.conv(r, t)
		if op == "==" || op == "!=" {
			return val{s: a + " " + op + " " + b, t: "bool", c: x.Type(), boolean: true}
		}
		m := map[string]string{"<": "Lt", ">": "Gt", "<=": "Le", ">=": "Ge"}[op]
		if !strings.HasPrefix(f.g.canon(t), "Ptr[") {
			f.no(x, "an ordered comparison of %s", t)
		}
		return val{s: a + "." + m + "(" + b + ")", t: "bool", c: x.Type(), boolean: true}
	}
	l, r := f.expr(le), f.expr(re)
	var t string
	switch {
	case l.konst && !r.konst:
		t = promoted(f.g.canon(r.t))
	case r.konst && !l.konst:
		t = promoted(f.g.canon(l.t))
	default:
		t = usual(f.arith(lt), f.arith(rt))
	}
	if l.boolean || r.boolean {
		t = "int32"
	}
	a, b := f.arithOperands(l, r, t)
	return val{s: a + " " + op + " " + b, t: "bool", c: x.Type(), boolean: true}
}

// logical is && or ||; what the right side does happens only when C would
// evaluate it.
func (f *fnEmit) logical(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) val {
	l := f.expr(le)
	var r val
	pre := f.capture(func() { r = f.expr(re) })
	if pre == "" {
		return val{s: paren(f.truth(l)) + " " + op + " " + paren(f.truth(r)), t: "bool", c: x.Type(), boolean: true}
	}
	t := f.newTemp("bool")
	f.line("%s = %s", t, f.truth(l))
	if op == "&&" {
		f.line("if %s {", t)
	} else {
		f.line("if !%s {", t)
	}
	f.indent++
	f.out.WriteString(indentText(pre, 1))
	f.line("%s = %s", t, f.truth(r))
	f.indent--
	f.line("}")
	return val{s: t, t: "bool", c: x.Type(), boolean: true}
}

// indentText indents captured lines by n more tabs.
func indentText(s string, n int) string {
	if s == "" {
		return s
	}
	p := strings.Repeat("\t", n)
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i := range lines {
		lines[i] = p + lines[i]
	}
	return strings.Join(lines, "\n") + "\n"
}

// ternary is ?: into a temporary; only the chosen arm is evaluated.
func (f *fnEmit) ternary(x *cc.ConditionalExpression, to string) val {
	c := f.expr(x.LogicalOrExpression)
	var a, b val
	var at, bt string
	f.indent++
	pa := f.capture(func() { a = f.exprTo(x.ExpressionList, to) })
	pb := f.capture(func() { b = f.exprTo(x.ConditionalExpression, to) })
	f.indent--
	t := to
	if t == "" {
		switch {
		case x.Type().Kind() == cc.Ptr:
			at, bt = a.t, b.t
			t = at
			if a.null || strings.HasPrefix(f.g.canon(bt), "Ptr[") {
				t = bt
			}
		case a.boolean && b.boolean:
			t = "bool"
		default:
			t = f.arith(x.Type())
		}
	}
	if t == "" {
		f.no(x, "a ?: of no type")
	}
	v := f.newTemp(t)
	f.line("if %s {", f.truth(c))
	f.out.WriteString(pa)
	f.indent++
	f.line("%s = %s", v, f.conv(a, t))
	f.indent--
	f.line("} else {")
	f.out.WriteString(pb)
	f.indent++
	f.line("%s = %s", v, f.conv(b, t))
	f.indent--
	f.line("}")
	return val{s: v, t: t, c: x.Type(), boolean: t == "bool"}
}

func (f *fnEmit) cast(x *cc.CastExpression, to string) val {
	tt := x.Type()
	inner := x.CastExpression
	it := inner.Type()
	switch {
	case tt.Kind() == cc.Void:
		f.exprStmt(inner)
		return val{s: "", t: ""}
	case scalar(tt) != "" && it != nil && scalar(it) != "":
		v := f.expr(inner)
		gt := f.arith(tt)
		if v.konst {
			if cv := x.Value(); cv != nil {
				switch n := cv.(type) {
				case cc.Int64Value:
					return val{s: strconv.FormatInt(int64(n), 10), t: gt, c: tt, konst: true}
				case cc.UInt64Value:
					return val{s: strconv.FormatUint(uint64(n), 10), t: gt, c: tt, konst: true}
				}
			}
		}
		return val{s: f.conv(v, gt), t: gt, c: tt}
	case tt.Kind() == cc.Ptr:
		pt := tt.(*cc.PointerType).Elem()
		si := stripCasts(inner)
		if memberOf(si) == "ga_data" {
			return f.gadata(si, pt)
		}
		if c, ok := si.(*cc.PostfixExpression); ok && c.Case == cc.PostfixExpressionCall && allocators[calleeName(c)] {
			return f.alloc(c, pt, to)
		}
		v := f.expr(inner)
		if pt.Kind() == cc.Void {
			return val{s: v.s, t: "any", c: tt}
		}
		want := to
		if want == "" {
			want = f.g.goType(tt, "")
			if isByteType(pt) {
				want = "Ptr[byte]"
			}
		}
		if isByteType(pt) && f.g.canon(v.t) == "Ptr[byte]" {
			return val{s: v.s, t: "Ptr[byte]", c: tt}
		}
		if f.g.canon(v.t) == f.g.canon(want) {
			return v
		}
		if elemOfGo(f.g.canon(v.t)) != "" && f.g.canon(elemOfGo(v.t)) == f.g.canon(elemOfGo(want)) {
			return val{s: f.conv(v, want), t: want, c: tt}
		}
		f.no(x, "a cast of %s to %s", v.t, tt)
	}
	f.no(x, "a cast to %s", tt)
	return val{}
}

var allocators = map[string]bool{"alloc": true, "alloc_clear": true, "lalloc": true, "lalloc_clear": true}

func calleeName(x *cc.PostfixExpression) string {
	if p, ok := unparenE(x.PostfixExpression).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
			return p.Token.SrcStr()
		}
	}
	return ""
}

func memberOf(e cc.ExpressionNode) string {
	if x, ok := unparenE(e).(*cc.PostfixExpression); ok && (x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionPSelect) {
		return x.Token2.SrcStr()
	}
	return ""
}

// gadata is (T *)gap->ga_data: the growarray's storage as Ptr[T].
func (f *fnEmit) gadata(e cc.ExpressionNode, elem cc.Type) val {
	x := unparenE(e).(*cc.PostfixExpression)
	var gap string
	if x.Case == cc.PostfixExpressionSelect {
		gap = "&" + f.path(x.PostfixExpression)
	} else {
		b := f.expr(x.PostfixExpression)
		gap = f.conv(b, "*garray_T")
	}
	et := f.g.goType(elem, "")
	if isByteType(elem) {
		et = "byte"
	}
	return val{s: "GaData[" + et + "](" + gap + ")", t: "Ptr[" + et + "]", c: e.Type()}
}

// gadataGo is gadata with the element's Go type.
func (f *fnEmit) gadataGo(x *cc.PostfixExpression, et string) val {
	var gap string
	if x.Case == cc.PostfixExpressionSelect {
		gap = "&" + f.path(x.PostfixExpression)
	} else {
		b := f.expr(x.PostfixExpression)
		gap = f.conv(b, "*garray_T")
	}
	return val{s: "GaData[" + et + "](" + gap + ")", t: "Ptr[" + et + "]", c: x.Type()}
}

// sizeCount is a size in bytes as a count of elements of Go type et whose C
// size is esize: n itself for bytes, k for k * sizeof(T), 1 for sizeof(T).
func (f *fnEmit) sizeCount(n cc.ExpressionNode, elem cc.Type) string {
	if isByteType(elem) || elem.Kind() == cc.Void {
		return f.index(f.expr(n))
	}
	es := elem.Size()
	b, known := int64(0), false
	switch x := constValue(n).(type) {
	case cc.Int64Value:
		b, known = int64(x), true
	case cc.UInt64Value:
		b, known = int64(x), true
	}
	if known && es > 0 && b%es == 0 {
		return strconv.FormatInt(b/es, 10)
	}
	if m, ok := unparenE(n).(*cc.MultiplicativeExpression); ok && m.Case == cc.MultiplicativeExpressionMul {
		for i, s := range []cc.ExpressionNode{m.MultiplicativeExpression, m.CastExpression} {
			u, ok := unparenE(s).(*cc.UnaryExpression)
			if ok && (u.Case == cc.UnaryExpressionSizeofType || u.Case == cc.UnaryExpressionSizeofExpr) {
				if v, ok := u.Value().(cc.UInt64Value); ok && int64(v) == es {
					other := m.CastExpression
					if i == 1 {
						other = m.MultiplicativeExpression
					}
					return f.index(f.expr(other))
				}
			}
		}
	}
	f.no(n, "a size that is no count of %s", elem)
	return ""
}

// alloc is an allocation: new(T) for one struct, Mk[T](n) for n elements,
// Alloc(n) for bytes.
func (f *fnEmit) alloc(c *cc.PostfixExpression, elem cc.Type, to string) val {
	var args []cc.ExpressionNode
	for l := c.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	if isByteType(elem) || elem == nil {
		return val{s: "Alloc(" + f.index(f.expr(args[0])) + ")", t: "Ptr[byte]", c: c.Type()}
	}
	et := f.g.goType(elem, "")
	want := to
	if want == "" {
		want = "*" + et
	}
	n := f.sizeCount(args[0], elem)
	if strings.HasPrefix(f.g.canon(want), "*") && n == "1" {
		return val{s: "new(" + et + ")", t: "*" + et, c: c.Type()}
	}
	pe := elemOfGo(want)
	if pe == "" {
		pe = et
	}
	if strings.HasPrefix(f.g.canon(want), "[]") {
		return val{s: "make([]" + pe + ", " + n + ")", t: "[]" + pe, c: c.Type()}
	}
	return val{s: "Mk[" + pe + "](" + n + ")", t: "Ptr[" + pe + "]", c: c.Type()}
}

// call is a function call, its arguments in the parameters' Go types.
func (f *fnEmit) call(x *cc.PostfixExpression, to string) val {
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	name := calleeName(x)
	switch name {
	case "musl_memmove", "musl_memcpy":
		d, s := f.memArg(args[0]), f.memArg(args[1])
		et := elemOfGo(f.g.canon(d.t))
		dt := "Ptr[" + et + "]"
		elem := elemCType(stripCasts(args[0]))
		return val{s: "Memmove(" + f.conv(d, dt) + ", " + f.conv(s, dt) + ", " + f.sizeCount(args[2], elem) + ")", t: dt, c: x.Type()}
	case "musl_memset":
		d := f.memArg(args[0])
		c := f.expr(args[1])
		elem := elemCType(stripCasts(args[0]))
		if isByteType(elem) {
			return val{s: "Memset(" + f.conv(d, "Ptr[byte]") + ", " + f.conv(c, "int32") + ", " + f.sizeCount(args[2], elem) + ")", t: "Ptr[byte]", c: x.Type()}
		}
		if isZeroConst(args[1]) {
			dt := f.g.canon(d.t)
			switch {
			case strings.HasPrefix(dt, "*"):
				f.line("%s = %s{}", deref(d.s), elemOfGo(d.t))
				return val{s: "", t: ""}
			case strings.HasPrefix(dt, "Ptr["):
				return val{s: "Zero(" + d.s + ", " + f.sizeCount(args[2], elem) + ")", t: "", c: x.Type()}
			case strings.HasPrefix(dt, "["):
				f.line("%s = %s{}", d.s, d.t)
				return val{s: "", t: ""}
			}
		}
		// every byte of every element is the fill byte
		f.fill(d, c, args[2], elem, x)
		return val{s: "", t: ""}
	case "musl_memcmp":
		if e := elemCType(stripCasts(args[0])); e != nil && (e.Kind() == cc.Struct || e.Kind() == cc.Union) {
			// two structs compared whole, where only equality is asked
			a, b := f.memArg(args[0]), f.memArg(args[1])
			if f.sizeCount(args[2], e) != "1" {
				f.no(x, "a memcmp of more than one struct")
			}
			return val{s: "B2i(" + f.conv(a, "*"+elemOfGo(a.t)) + " != " + f.conv(b, "*"+elemOfGo(a.t)) + ")", t: "int32", c: x.Type()}
		}
		a, b := f.memArg(args[0]), f.memArg(args[1])
		return val{s: "Memcmp(" + f.conv(a, "Ptr[byte]") + ", " + f.conv(b, "Ptr[byte]") + ", " + f.index(f.expr(args[2])) + ")", t: "int32", c: x.Type()}
	case "__builtin_expect":
		return f.exprTo(args[0], to)
	case "alloc", "alloc_clear", "lalloc", "lalloc_clear":
		if f.g.canon(to) == "[]byte" {
			// bytes for a pointer that only walks forward: a Go slice, zeroed
			return val{s: "make([]byte, " + f.index(f.expr(x.ArgumentExpressionList.AssignmentExpression)) + ")", t: "[]byte", c: x.Type()}
		}
		var elem cc.Type
		if !strings.HasPrefix(f.g.canon(to), "Ptr[byte]") && to != "" && to != "any" {
			elem = sizeofElem(args[0])
			if elem == nil {
				f.no(x, "an allocation of no element type for a %s", to)
			}
		}
		return f.alloc(x, elem, to)
	}
	var ft *cc.FunctionType
	var fn string
	var key string
	if name != "" {
		d := unparenE(x.PostfixExpression).(*cc.PrimaryExpression).ResolvedTo().(*cc.Declarator)
		ft, _ = d.Type().(*cc.FunctionType)
		fn = GoName(name)
		key = "param:" + name
	} else {
		callee := x.PostfixExpression
		if u, ok := unparenE(callee).(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionDeref {
			callee = u.CastExpression
		}
		ct := callee.Type()
		if p, ok := ct.(*cc.PointerType); ok {
			ft, _ = p.Elem().(*cc.FunctionType)
		}
		o := f.g.a.obj(callee)
		fn = f.expr(callee).s
		key = "fp:" + o
	}
	if ft == nil {
		f.no(x, "a call of no function type")
	}
	ps := ft.Parameters()
	if len(ps) == 1 && ps[0].Type().Kind() == cc.Void {
		ps = nil
	}
	var as []string
	for i, a := range args {
		if i < len(ps) {
			pkey := fmt.Sprintf("%s:%d", key, i)
			pt := f.g.goType(ps[i].Type(), pkey)
			v := f.exprTo(a, pt)
			if strings.HasPrefix(pt, "func(") && strings.HasPrefix(v.t, "func(") && f.g.canon(v.t) != f.g.canon(pt) {
				as = append(as, f.adapter(a, ps[i].Type(), pkey, v))
				continue
			}
			as = append(as, f.conv(v, pt))
			continue
		}
		if !ft.IsVariadic() {
			f.no(x, "more arguments than parameters")
		}
		v := f.expr(a)
		switch {
		case v.boolean:
			as = append(as, "B2i("+v.s+")")
		case v.konst:
			as = append(as, f.arith(a.Type())+"("+v.s+")")
		case v.null:
			as = append(as, "Ptr[byte]{}")
		case strings.HasPrefix(f.g.canon(v.t), "["):
			// an array decays to a pointer to its first element
			as = append(as, "View("+v.s+"[:])")
		default:
			as = append(as, v.s)
		}
	}
	rt := ""
	if name != "" {
		rt = f.g.goType(ft.Result(), "ret:"+name)
	} else {
		rt = f.g.goType(ft.Result(), "ret:"+key)
	}
	return val{s: fn + "(" + strings.Join(as, ", ") + ")", t: rt, c: x.Type()}
}

func elemCType(e cc.ExpressionNode) cc.Type {
	t := e.Type()
	switch x := t.(type) {
	case *cc.PointerType:
		return x.Elem()
	case *cc.ArrayType:
		return x.Elem()
	}
	return t
}

// exprStmt writes an expression evaluated for what it does.
func (f *fnEmit) exprStmt(e cc.ExpressionNode) {
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.ExpressionList:
		for ; x != nil; x = x.ExpressionList {
			f.exprStmt(x.AssignmentExpression)
		}
		return
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			f.assign(x)
			return
		}
	case *cc.PostfixExpression:
		if x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
			f.incdec(f.lval(x.PostfixExpression), x.Case == cc.PostfixExpressionInc, x)
			return
		}
		if x.Case == cc.PostfixExpressionCall {
			v := f.call(x, "")
			if v.s != "" {
				f.line("%s", v.s)
			}
			return
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
			f.incdec(f.lval(x.UnaryExpression), x.Case == cc.UnaryExpressionInc, x)
			return
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast && x.Type().Kind() == cc.Void {
			f.exprStmt(x.CastExpression)
			return
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			c := f.expr(x.LogicalOrExpression)
			f.line("if %s {", f.truth(c))
			f.indent++
			f.exprStmt(x.ExpressionList)
			f.indent--
			f.line("} else {")
			f.indent++
			f.exprStmt(x.ConditionalExpression)
			f.indent--
			f.line("}")
			return
		}
	case *cc.LogicalAndExpression, *cc.LogicalOrExpression:
		v := f.expr(x)
		f.line("_ = %s", v.s)
		return
	}
	v := f.expr(e)
	if v.s != "" {
		f.line("_ = %s", v.s)
	}
}

func isZeroConst(e cc.ExpressionNode) bool {
	if hasEffect(e) {
		return false
	}
	if p, ok := unparenE(e).(*cc.PostfixExpression); ok && p.Case == cc.PostfixExpressionComplit {
		return false // storage of its own, whatever cc makes its value
	}
	switch v := unparenE(e).Value().(type) {
	case cc.Int64Value:
		return v == 0
	case cc.UInt64Value:
		return v == 0
	}
	return false
}

// sizeofElem is T in sizeof(T), k * sizeof(T) or sizeof(T) * k.
func sizeofElem(n cc.ExpressionNode) cc.Type {
	st := func(e cc.ExpressionNode) cc.Type {
		u, ok := unparenE(e).(*cc.UnaryExpression)
		if !ok {
			return nil
		}
		switch u.Case {
		case cc.UnaryExpressionSizeofType:
			return u.TypeName.Type()
		case cc.UnaryExpressionSizeofExpr:
			return u.UnaryExpression.Type()
		}
		return nil
	}
	if t := st(n); t != nil {
		return t
	}
	if m, ok := unparenE(n).(*cc.MultiplicativeExpression); ok && m.Case == cc.MultiplicativeExpressionMul {
		if t := st(m.MultiplicativeExpression); t != nil {
			return t
		}
		return st(m.CastExpression)
	}
	return nil
}

// memArg is an argument of a function of bytes, seen through its casts; a
// growarray's storage is read as the type the cast gave it.
func (f *fnEmit) memArg(a cc.ExpressionNode) val {
	s := stripCasts(a)
	if x, ok := unparenE(s).(*cc.PostfixExpression); ok && memberOf(x) == "ga_data" {
		et := "byte"
		if e := elemCType(unparenE(a)); e != nil && !isByteType(e) && e.Kind() != cc.Void {
			et = f.g.goType(e, "")
		}
		return f.gadataGo(x, et)
	}
	return f.expr(s)
}

// repeated is the byte expression b repeated through an integer of k bytes.
func repeated(b string, k int64) string {
	switch k {
	case 1:
		return b
	case 2:
		return "uint16(" + b + ")*0x0101"
	case 4:
		return "uint32(" + b + ")*0x01010101"
	}
	return "uint64(" + b + ")*0x0101010101010101"
}

// fill is memset(p, c, n) of elements that are not bytes: every byte of each
// element is c's low byte, field by field for a struct of integers.
func (f *fnEmit) fill(d, c val, n cc.ExpressionNode, elem cc.Type, at cc.Node) {
	count := f.sizeCount(n, elem)
	if !strings.HasPrefix(f.g.canon(d.t), "Ptr[") {
		f.no(at, "a fill of a %s", d.t)
	}
	b := f.newTemp("byte")
	f.line("%s = %s", b, f.conv(c, "byte"))
	i := f.newTemp("int")
	f.line("for %s = 0; %s < %s; %s++ {", i, i, count, i)
	switch {
	case scalar(elem) != "" && elem.Kind() != cc.Float && elem.Kind() != cc.Double:
		et := elemOfGo(d.t)
		f.line("	%s.Set(%s, %s(%s))", d.s, i, et, repeated(b, elem.Size()))
	case elem.Kind() == cc.Struct:
		st := elem.(*cc.StructType)
		for k := 0; k < st.NumFields(); k++ {
			fl := st.FieldByIndex(k)
			if scalar(fl.Type()) == "" {
				f.no(at, "a fill of a struct with a %s", fl.Type())
			}
			f.line("	%s.Ref(%s).%s = %s(%s)", d.s, i, GoName(fl.Name()), f.g.goType(fl.Type(), fieldKey(fl)), repeated(b, fl.Type().Size()))
		}
	default:
		f.no(at, "a fill of %s", elem)
	}
	f.line("}")
}

// adapter is a function passed where a function pointer of another Go type
// is wanted -- the skeleton gave their parameters different pointer kinds --
// as a closure that converts each argument.
func (f *fnEmit) adapter(a cc.ExpressionNode, pt cc.Type, pkey string, v val) string {
	p, ok := unparenE(a).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		f.no(a, "a function pointer of another type that is not a function")
	}
	src := calleeNameOf(p)
	if src == "" {
		f.no(a, "a function pointer of another type that is not a function")
	}
	tf, _ := pt.(*cc.PointerType).Elem().(*cc.FunctionType)
	sf, _ := p.ResolvedTo().(*cc.Declarator).Type().(*cc.FunctionType)
	if tf == nil || sf == nil || len(tf.Parameters()) != len(sf.Parameters()) {
		f.no(a, "an adapter between functions of different arity")
	}
	var params, args []string
	for j, tp := range tf.Parameters() {
		tt := f.g.goType(tp.Type(), fmt.Sprintf("fp:%s:%d", pkey, j))
		st := f.g.goType(sf.Parameters()[j].Type(), fmt.Sprintf("param:%s:%d", src, j))
		n := fmt.Sprintf("a%d", j)
		params = append(params, n+" "+tt)
		args = append(args, f.conv(val{s: n, t: tt}, st))
	}
	rt := f.g.goType(tf.Result(), "ret:fp:"+pkey)
	sr := f.g.goType(sf.Result(), "ret:"+src)
	call := GoName(src) + "(" + strings.Join(args, ", ") + ")"
	if rt == "" {
		return "func(" + strings.Join(params, ", ") + ") { " + call + " }"
	}
	return "func(" + strings.Join(params, ", ") + ") " + rt + " { return " + f.conv(val{s: call, t: sr}, rt) + " }"
}

func calleeNameOf(p *cc.PrimaryExpression) string {
	if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
		return p.Token.SrcStr()
	}
	return ""
}

// hasEffect says evaluating e does something: a call, an assignment, an
// increment or decrement, or a comma, whose operands before the last are
// evaluated for what they do.  The front end folds `(emsg(...), NULL)` to the
// value of its last operand, so asking only for e's value -- is it a null
// pointer, a zero, a true constant -- dropped the error message (regatom's
// E64, `return (..., rc_did_emsg = TRUE, nullptr)`, went out as `return nil`).
func hasEffect(e cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		switch x := n.(type) {
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionCall || x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
				found = true
				return
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
				found = true
				return
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
		}
		walkChildrenFn(n, rec)
	}
	rec(e)
	return found
}

// constValue is e's value when evaluating e does nothing else; nil otherwise.
func constValue(e cc.ExpressionNode) cc.Value {
	if hasEffect(e) {
		return nil
	}
	return e.Value()
}
