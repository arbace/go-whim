package togo

// java_expr.go writes C's expressions in Java.  What an expression does
// besides giving its value -- an assignment, ++, a call inside && -- is
// written as statements before it, into temporaries, as the Go emitter does;
// the value is then an expression with nothing left to do.

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// jval is an emitted expression: its Java source, its Java type and its C
// type.  konst is a C constant whose value is cv; named says s is a Java int
// constant of that value worth keeping (an enumerator, a character).
type jval struct {
	s     string
	t     string
	c     cc.Type
	konst bool
	named bool
	null  bool
	cv    int64
	kind  *jk // the C kind of a value made here from Java text, whose c is nil
}

// jparen wraps a composite expression, reading past string and character
// literals.
func jparen(s string) string {
	simple, depth := true, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			for i++; i < len(s) && s[i] != c; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ' ':
			if depth == 0 {
				simple = false
			}
		case '-', '+', '!', '~':
			if i == 0 {
				simple = false
			}
		}
	}
	if simple || (strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && jbalanced(s[1:len(s)-1])) {
		return s
	}
	return "(" + s + ")"
}

// jbalanced says s's parentheses balance, reading past literals.
func jbalanced(s string) bool {
	d := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			for i++; i < len(s) && s[i] != c; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '(':
			d++
		case ')':
			d--
			if d < 0 {
				return false
			}
		}
	}
	return d == 0
}

// isPtrish: the C type is a pointer or an array, which Java holds by
// reference.
func isPtrish(t cc.Type) bool {
	return t != nil && (t.Kind() == cc.Ptr || t.Kind() == cc.Array)
}

// numconv is the Java expression s of kind from as kind to: C's conversion
// of integers, on their bits.
func numconv(s string, from, to jk) string {
	if from.size == to.size {
		return s
	}
	if to.size > from.size {
		if from.signed {
			return "(" + to.java() + ") " + jparen(s)
		}
		switch {
		case from.size == 1 && to.size == 2:
			return "(short) (" + s + " & 0xff)"
		case from.size == 1 && to.size == 4:
			return "(" + s + " & 0xff)"
		case from.size == 1:
			return "(" + s + " & 0xffL)"
		case from.size == 2 && to.size == 4:
			return "(" + s + " & 0xffff)"
		case from.size == 2:
			return "(" + s + " & 0xffffL)"
		}
		return "Integer.toUnsignedLong(" + s + ")"
	}
	return "(" + to.java() + ") " + jparen(s)
}

// convK is v as a scalar of kind k.
func (f *jfn) convK(v jval, k jk) string {
	if v.null {
		return jlit(0, k)
	}
	if k.boolean {
		if v.t == "boolean" {
			return v.s
		}
		return f.truth(v)
	}
	if v.konst {
		if v.named && k.size == 4 && int64(int32(v.cv)) == v.cv {
			return v.s
		}
		return jlit(v.cv, k)
	}
	if v.t == "boolean" {
		return numconv("("+v.s+" ? 1 : 0)", jInt, k)
	}
	from, ok := scalarKind(v.c)
	if v.kind != nil {
		from, ok = *v.kind, true
	}
	if !ok {
		f.no(nil, "a cast between a pointer and an integer")
	}
	return numconv(v.s, from, k)
}

// conv is v as Java type to, of C type tc.
func (f *jfn) conv(v jval, to string, tc cc.Type) string {
	if k, ok := scalarKind(tc); ok {
		return f.convK(v, k)
	}
	if v.null || v.konst && v.cv == 0 {
		return "null" // a 0 is a null pointer constant
	}
	if v.t == to {
		return v.s
	}
	if _, ok := scalarKind(v.c); ok || v.t == "boolean" {
		f.no(nil, "a cast between a pointer and an integer")
	}
	switch {
	case strings.HasSuffix(v.t, "[]") && ptrOfArray(elemJ(v.t)) == to:
		// an array decays to a pointer to its first element
		return ptrOver(elemJ(v.t), v.s, "0")
	case strings.HasSuffix(v.t, "[]") && elemJ(v.t) == to:
		return v.s + "[0]" // ... held as a plain reference
	case strings.HasPrefix(v.t, "Ptr<") && elemJ(v.t) == to:
		return ptrRef(v.s)
	case strings.HasPrefix(to, "Ptr<") && elemJ(to) == v.t:
		return "Ptr.one(" + v.s + ")"
	}
	f.no(nil, "a pointer conversion: %s to %s", v.t, to)
	return ""
}

// ptrRef is *p for a Ptr p held as a plain reference: new Ptr<T>(a, k) is
// a[k], and p.add(k) is p.at(k).
func ptrRef(s string) string {
	if strings.HasPrefix(s, "new Ptr<") {
		if i := strings.Index(s, ">("); i > 0 && strings.HasSuffix(s, ")") {
			args := s[i+2 : len(s)-1]
			if c := topComma(args); c > 0 && jbalanced(args) {
				return args[:c] + "[" + strings.TrimSpace(args[c+1:]) + "]"
			}
		}
	}
	if strings.HasSuffix(s, ")") {
		if i := strings.LastIndex(s, ".add("); i > 0 && jbalanced(s[i+5:len(s)-1]) && jpure(s[:i]) && jbalanced(s[:i]) {
			return s[:i] + ".at(" + s[i+5:len(s)-1] + ")"
		}
	}
	return "Ptr.ref(" + s + ")"
}

// topComma is the index of the first comma outside brackets and literals.
func topComma(s string) int {
	d := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			for i++; i < len(s) && s[i] != c; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
		case ',':
			if d == 0 {
				return i
			}
		}
	}
	return -1
}

// truth is v as a Java condition.
func (f *jfn) truth(v jval) string {
	switch {
	case v.t == "boolean":
		return v.s
	case v.konst:
		return fmt.Sprint(v.cv != 0)
	case v.null:
		return "false"
	case isPtrish(v.c) || v.c == nil:
		return jparen(v.s) + " != null"
	}
	return jparen(v.s) + " != 0"
}

// falsity is !v as a Java condition.
func (f *jfn) falsity(v jval) string {
	switch {
	case v.t == "boolean":
		return jnot(v.s)
	case v.konst:
		return fmt.Sprint(v.cv == 0)
	case v.null:
		return "true"
	case isPtrish(v.c) || v.c == nil:
		return jparen(v.s) + " == null"
	}
	return jparen(v.s) + " == 0"
}

func jnot(s string) string {
	if strings.HasPrefix(s, "!") && jparen(s[1:]) == s[1:] {
		return s[1:]
	}
	return "!" + jparen(s)
}

var jcallRe = regexp.MustCompile(`\.?[A-Za-z_]\w*\(`)
var jpureCalls = map[string]bool{".get(": true, ".at(": true, ".add(": true, ".of(": true, ".lit(": true, ".ref(": true,
	".eq(": true, ".sub(": true, ".lt(": true, ".le(": true, ".gt(": true, ".ge(": true, ".divideUnsigned(": true,
	".remainderUnsigned(": true, ".compareUnsigned(": true, ".toUnsignedLong(": true, ".one(": true,
	"BytePtr(": true, "ShortPtr(": true, "IntPtr(": true, "LongPtr(": true, "BoolPtr(": true}

// pure says an emitted expression calls nothing that has an effect, so it can
// be written twice or dropped.
func jpure(s string) bool {
	for _, m := range jcallRe.FindAllString(s, -1) {
		if !jpureCalls[m] {
			return false
		}
	}
	return true
}

// index is v as a Java int, for an index or an offset.
func (f *jfn) index(v jval) string { return f.convK(v, jInt) }

func (f *jfn) expr(e cc.ExpressionNode) jval { return f.exprTo(e, "") }

// exprTo is expr told the Java type the value goes to, for what has no type
// of its own (a null).  Every integer constant is written as its value: the
// C arithmetic that made it is not Java's.
func (f *jfn) exprTo(e cc.ExpressionNode, to string) jval {
	if isNullConst(e) && e.Type() != nil && e.Type().Kind() == cc.Ptr {
		return jval{s: "null", t: to, c: e.Type(), null: true}
	}
	if k, ok := scalarKind(e.Type()); ok && !hasEffect(e) {
		var cv int64
		known := true
		switch x := e.Value().(type) {
		case cc.Int64Value:
			cv = int64(x)
		case cc.UInt64Value:
			cv = int64(x)
		default:
			known = false
		}
		if known {
			switch x := unparenE(e).(type) {
			case *cc.PrimaryExpression:
				if x.Case == cc.PrimaryExpressionChar || x.Case == cc.PrimaryExpressionIdent {
					return f.primary(x, to)
				}
			}
			return jval{s: jlit(cv, k), t: k.java(), c: e.Type(), konst: true, cv: cv}
		}
	}
	return f.exprTo1(e, to)
}

func (f *jfn) exprTo1(e cc.ExpressionNode, to string) jval {
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
			return jval{s: lv.get, t: lv.t, c: lv.c}
		}
	}
	f.no(e, "an expression %T", e)
	return jval{}
}

func (f *jfn) primary(x *cc.PrimaryExpression, to string) jval {
	switch x.Case {
	case cc.PrimaryExpressionIdent:
		return f.ident(x)
	case cc.PrimaryExpressionInt, cc.PrimaryExpressionChar:
		k, ok := scalarKind(x.Type())
		if !ok {
			f.no(x, "a constant of type %s", x.Type())
		}
		var cv int64
		switch v := x.Value().(type) {
		case cc.Int64Value:
			cv = int64(v)
		case cc.UInt64Value:
			cv = int64(v)
		default:
			f.no(x, "floating point")
		}
		if x.Case == cc.PrimaryExpressionChar && cv >= 0x20 && cv < 0x7f && cv != '\'' && cv != '\\' {
			return jval{s: "'" + string(rune(cv)) + "'", t: k.java(), c: x.Type(), konst: true, named: true, cv: cv}
		}
		return jval{s: jlit(cv, k), t: k.java(), c: x.Type(), konst: true, cv: cv}
	case cc.PrimaryExpressionString:
		sv, ok := x.Value().(cc.StringValue)
		if !ok {
			f.no(x, "a wide string")
		}
		return jval{s: "BytePtr.lit(" + javaQuote(strings.TrimSuffix(string(sv), "\x00")) + ")", t: "BytePtr", c: x.Type()}
	case cc.PrimaryExpressionExpr:
		v := f.exprTo(x.ExpressionList, to)
		if !v.null && !v.named {
			v.s = jparen(v.s)
		}
		return v
	case cc.PrimaryExpressionFloat:
		f.no(x, "floating point")
	}
	f.no(x, "a primary expression %v", x.Case)
	return jval{}
}

// ident is a name as a value.
func (f *jfn) ident(x *cc.PrimaryExpression) jval {
	switch d := x.ResolvedTo().(type) {
	case *cc.Declarator:
		t := d.Type()
		if t.Kind() == cc.Function {
			f.no(x, "a function pointer: %s used as a value", d.Name())
		}
		if l, ok := f.byDecl[d]; ok {
			return jval{s: l.ref(), t: l.typ, c: t}
		}
		if d.Name() == "__func__" {
			f.no(x, "__func__")
		}
		key := f.j.g.a.declKey(d)
		var name string
		switch {
		case strings.HasPrefix(key, "static:"):
			name = f.j.staticName(strings.SplitN(strings.TrimPrefix(key, "static:"), ".", 2)[0], d.Name())
		case strings.HasPrefix(key, "global:"):
			name = f.j.jName(d.Name())
		default:
			f.no(x, "a name with no declaration here: %s (%s)", d.Name(), key)
		}
		jt := f.jt(t, key, "a global", d.Name())
		if f.j.isBoxed(d) {
			name += "[0]"
		}
		return jval{s: name, t: jt, c: t}
	case *cc.Enumerator:
		name := f.j.jName(x.Token.SrcStr())
		k, ok := scalarKind(x.Type())
		if !ok {
			k = jInt
		}
		var cv int64
		switch v := d.Value().(type) {
		case cc.Int64Value:
			cv = int64(v)
		case cc.UInt64Value:
			cv = int64(v)
		}
		f.j.enums[name] = fmt.Sprintf("    static final %s %s = %s;\n", k.java(), name, jlit(cv, k))
		return jval{s: name, t: k.java(), c: x.Type(), konst: true, named: k.size == 4, cv: cv}
	}
	f.no(x, "an identifier resolved to %T", x.ResolvedTo())
	return jval{}
}

// fieldOf is a member's Java name and type, or the function refused.
func (f *jfn) fieldOf(x *cc.PostfixExpression) (string, string) {
	fl := x.Field()
	if fl == nil {
		f.no(x, "a member with no field")
	}
	if f.j.bitfield[fl] {
		f.no(x, "a bitfield")
	}
	if fl.Name() == "" || fl.ParentType() == nil || fl.ParentType().Kind() != cc.Struct {
		f.no(x, "a union")
	}
	return f.j.jName(fl.Name()), f.jt(fl.Type(), fieldKey(fl), "a member", fl.Name())
}

// member is s.x or p->x: the struct's object, then its member.
func (f *jfn) member(x *cc.PostfixExpression) jval {
	name, ft := f.fieldOf(x)
	var obj string
	if x.Case == cc.PostfixExpressionSelect {
		obj = f.expr(x.PostfixExpression).s
	} else {
		obj = f.structRef(f.expr(x.PostfixExpression), x)
	}
	return jval{s: obj + "." + name, t: ft, c: x.Type()}
}

// structRef is the struct object a pointer value points at.
func (f *jfn) structRef(b jval, at cc.Node) string {
	switch {
	case strings.HasPrefix(b.t, "Ptr<"):
		return b.s + ".get()"
	case strings.HasSuffix(b.t, "[]"):
		return b.s + "[0]"
	case b.null:
		f.no(at, "a member of NULL")
	}
	return b.s
}

func (f *jfn) postfix(x *cc.PostfixExpression, to string) jval {
	switch x.Case {
	case cc.PostfixExpressionIndex:
		base, ix := f.indexOperands(x)
		i := f.index(ix)
		switch {
		case strings.HasSuffix(base.t, "[]"):
			return jval{s: base.s + "[" + i + "]", t: elemJ(base.t), c: x.Type()}
		case scalarPtrElem(base.t) != "" || strings.HasPrefix(base.t, "Ptr<"):
			return jval{s: base.s + ".at(" + i + ")", t: elemJ(base.t), c: x.Type()}
		case ix.konst && ix.cv == 0 && x.Type().Kind() == cc.Struct:
			return jval{s: base.s, t: base.t, c: x.Type()} // p[0] of a pointer that walks nowhere
		}
		f.no(x, "an index into a %s", base.t)
	case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
		return f.member(x)
	case cc.PostfixExpressionCall:
		return f.call(x, to)
	case cc.PostfixExpressionComplit:
		f.no(x, "a compound literal")
	case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
		lv := f.lval(x.PostfixExpression)
		t := f.newTemp(lv.t, lv.c)
		f.stmt1("%s = %s", t, lv.get)
		f.incdec(lv, x.Case == cc.PostfixExpressionInc, x)
		return jval{s: t, t: lv.t, c: lv.c}
	}
	f.no(x, "a postfix expression %v", x.Case)
	return jval{}
}

// indexOperands are a[i]'s array or pointer and index, in either order.
func (f *jfn) indexOperands(x *cc.PostfixExpression) (jval, jval) {
	if !isPtrish(x.PostfixExpression.Type()) {
		f.no(x, "an index written i[a]")
	}
	return f.expr(x.PostfixExpression), f.expr(x.ExpressionList)
}

// jlv is somewhere a value is stored.
type jlv struct {
	get string
	t   string
	c   cc.Type
	set func(v string) string // the statement that stores v
	op  bool                  // get is a Java variable: it takes op= and ++
}

func (f *jfn) lval(e cc.ExpressionNode) jlv {
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			v := f.ident(x)
			return jlv{get: v.s, t: v.t, c: v.c, set: func(s string) string { return v.s + " = " + s }, op: true}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			v := f.member(x)
			if !jpure(v.s) {
				f.no(x, "a store through a member of a call's result")
			}
			return jlv{get: v.s, t: v.t, c: v.c, set: func(s string) string { return v.s + " = " + s }, op: true}
		case cc.PostfixExpressionIndex:
			base, ix := f.indexOperands(x)
			i := f.index(ix)
			if !jpure(i) {
				t := f.newTemp("int", nil)
				f.stmt1("%s = %s", t, i)
				i = t
			}
			b := f.pureBase(base)
			switch {
			case strings.HasSuffix(b.t, "[]"):
				g := b.s + "[" + i + "]"
				return jlv{get: g, t: elemJ(b.t), c: x.Type(), set: func(s string) string { return g + " = " + s }, op: true}
			case scalarPtrElem(b.t) != "" || strings.HasPrefix(b.t, "Ptr<"):
				return jlv{get: b.s + ".at(" + i + ")", t: elemJ(b.t), c: x.Type(), set: func(s string) string { return b.s + ".set(" + i + ", " + s + ")" }}
			case ix.konst && ix.cv == 0 && x.Type().Kind() == cc.Struct:
				return jlv{get: b.s, t: b.t, c: x.Type(), set: func(s string) string { return b.s + ".set(" + s + ")" }}
			}
			f.no(x, "a store into a %s", b.t)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			b := f.pureBase(f.expr(x.CastExpression))
			switch {
			case strings.HasSuffix(b.t, "[]"):
				g := b.s + "[0]"
				return jlv{get: g, t: elemJ(b.t), c: x.Type(), set: func(s string) string { return g + " = " + s }, op: true}
			case scalarPtrElem(b.t) != "" || strings.HasPrefix(b.t, "Ptr<"):
				return jlv{get: b.s + ".get()", t: elemJ(b.t), c: x.Type(), set: func(s string) string { return b.s + ".put(" + s + ")" }}
			case x.Type().Kind() == cc.Struct:
				return jlv{get: b.s, t: b.t, c: x.Type(), set: func(s string) string { return b.s + ".set(" + s + ")" }}
			}
			f.no(x, "a store through a %s", b.t)
		}
	}
	f.no(e, "a store into %T", e)
	return jlv{}
}

// pureBase is a pointer or array value that may be written twice: itself,
// or a temporary holding it.
func (f *jfn) pureBase(b jval) jval {
	if jpure(b.s) {
		return b
	}
	t := f.newTemp(b.t, b.c)
	f.stmt1("%s = %s", t, b.s)
	b.s = t
	return b
}

// isJPtr says a Java type is one of the runtime's pointers.
func isJPtr(t string) bool {
	return scalarPtrElem(t) != "" || strings.HasPrefix(t, "Ptr<")
}

// incdec writes ++ or -- of an lvalue.
func (f *jfn) incdec(lv jlv, inc bool, n cc.Node) {
	d := "1"
	if !inc {
		d = "-1"
	}
	switch {
	case isJPtr(lv.t):
		f.stmt1("%s", lv.set(lv.get+".add("+d+")"))
	case lv.t == "boolean":
		f.no(n, "++ or -- of a _Bool")
	case lv.op:
		if inc {
			f.stmt1("%s++", lv.get)
		} else {
			f.stmt1("%s--", lv.get)
		}
	default:
		k, ok := scalarKind(lv.c)
		if !ok {
			f.no(n, "++ or -- of a %s", lv.t)
		}
		op := " + 1"
		if !inc {
			op = " - 1"
		}
		f.stmt1("%s", lv.set(numconv(lv.get+op, promote(k), k)))
	}
}

// assign writes an assignment and returns where it stored.
func (f *jfn) assign(x *cc.AssignmentExpression) jlv {
	lv := f.lval(x.UnaryExpression)
	if x.Case == cc.AssignmentExpressionAssign {
		r := f.exprTo(x.AssignmentExpression, lv.t)
		if lv.c.Kind() == cc.Struct {
			f.stmt1("%s.set(%s)", lv.get, r.s) // C's assignment of a struct is a copy
			return lv
		}
		f.stmt1("%s", lv.set(f.conv(r, lv.t, lv.c)))
		return lv
	}
	ops := map[cc.AssignmentExpressionCase]string{cc.AssignmentExpressionMul: "*", cc.AssignmentExpressionDiv: "/", cc.AssignmentExpressionMod: "%",
		cc.AssignmentExpressionAdd: "+", cc.AssignmentExpressionSub: "-", cc.AssignmentExpressionLsh: "<<", cc.AssignmentExpressionRsh: ">>",
		cc.AssignmentExpressionAnd: "&", cc.AssignmentExpressionXor: "^", cc.AssignmentExpressionOr: "|"}
	op := ops[x.Case]
	r := f.expr(x.AssignmentExpression)
	if isJPtr(lv.t) && (op == "+" || op == "-") {
		n := f.index(r)
		if op == "-" {
			n = "-" + jparen(n)
		}
		f.stmt1("%s", lv.set(lv.get+".add("+n+")"))
		return lv
	}
	lk, ok := scalarKind(lv.c)
	if !ok {
		f.no(x, "%s= on a %s", op, lv.t)
	}
	rk, ok := scalarKind(r.c)
	if !ok && r.t != "boolean" {
		f.no(x, "%s= of a %s", op, r.t)
	}
	if r.t == "boolean" {
		rk = jInt
	}
	common := usualK(lk, rk)
	if op == "<<" || op == ">>" {
		common = promote(lk)
	}
	direct := lv.op && !lk.boolean
	switch op {
	case "/", "%":
		direct = direct && common.signed && lk.size == common.size
	case ">>":
		direct = direct && lk.size >= 4
	}
	if direct {
		rhs := f.convK(r, common)
		jop := op
		if op == "<<" || op == ">>" {
			rhs = f.convK(r, jInt)
			if op == ">>" && !common.signed {
				jop = ">>>"
			}
		}
		f.stmt1("%s %s= %s", lv.get, jop, rhs)
		return lv
	}
	// lv = (T) (lv op r), in C's common type
	lval := jval{s: lv.get, t: lv.t, c: lv.c}
	res := f.arith(op, f.convK(lval, common), r, common)
	f.stmt1("%s", lv.set(f.convK(jval{s: res, t: common.java(), kind: &common}, lk)))
	return lv
}

// arith is a op b in common kind k, a already converted: C's arithmetic on
// Java's bits, unsigned where C's is.
func (f *jfn) arith(op, a string, bv jval, k jk) string {
	if op == "<<" || op == ">>" {
		b := f.convK(bv, jInt)
		if op == ">>" && !k.signed {
			op = ">>>"
		}
		return jparen(a) + " " + op + " " + jparen(b)
	}
	b := f.convK(bv, k)
	if !k.signed && (op == "/" || op == "%") {
		cls := "Integer"
		if k.size == 8 {
			cls = "Long"
		}
		fn := "divideUnsigned"
		if op == "%" {
			fn = "remainderUnsigned"
		}
		return cls + "." + fn + "(" + a + ", " + b + ")"
	}
	return jparen(a) + " " + op + " " + jparen(b)
}

func (f *jfn) unary(x *cc.UnaryExpression) jval {
	switch x.Case {
	case cc.UnaryExpressionPostfix:
		return f.expr(x.PostfixExpression)
	case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
		lv := f.lval(x.UnaryExpression)
		f.incdec(lv, x.Case == cc.UnaryExpressionInc, x)
		return jval{s: lv.get, t: lv.t, c: lv.c}
	case cc.UnaryExpressionAddrof:
		return f.addr(x.CastExpression, x)
	case cc.UnaryExpressionDeref:
		b := f.expr(x.CastExpression)
		switch {
		case strings.HasSuffix(b.t, "[]"):
			return jval{s: b.s + "[0]", t: elemJ(b.t), c: x.Type()}
		case isJPtr(b.t):
			return jval{s: b.s + ".get()", t: elemJ(b.t), c: x.Type()}
		case x.Type().Kind() == cc.Struct:
			return jval{s: b.s, t: b.t, c: x.Type()} // *p of a pointer that walks nowhere: the object
		}
		f.no(x, "* of a %s", b.t)
	case cc.UnaryExpressionPlus, cc.UnaryExpressionMinus, cc.UnaryExpressionCpl:
		v := f.expr(x.CastExpression)
		k, ok := scalarKind(x.Type())
		if !ok {
			f.no(x, "floating point")
		}
		s := f.convK(v, k)
		switch x.Case {
		case cc.UnaryExpressionMinus:
			s = "-(" + s + ")"
		case cc.UnaryExpressionCpl:
			s = "~(" + s + ")"
		}
		return jval{s: s, t: k.java(), c: x.Type()}
	case cc.UnaryExpressionNot:
		v := f.expr(x.CastExpression)
		return jval{s: f.falsity(v), t: "boolean", c: x.Type()}
	case cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType, cc.UnaryExpressionAlignofExpr, cc.UnaryExpressionAlignofType:
		f.no(x, "a sizeof of no constant value")
	}
	f.no(x, "a unary expression %v", x.Case)
	return jval{}
}

// addr is &e.
func (f *jfn) addr(e cc.ExpressionNode, at cc.Node) jval {
	e = unparenE(e)
	t := at.(cc.ExpressionNode).Type()
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			d, _ := x.ResolvedTo().(*cc.Declarator)
			v := f.ident(x)
			switch {
			case d != nil && f.j.isBoxed(d):
				box := strings.TrimSuffix(v.s, "[0]")
				return jval{s: ptrOver(v.t, box, "0"), t: ptrOfArray(v.t), c: t}
			case e.Type().Kind() == cc.Struct:
				return jval{s: v.s, t: v.t, c: t}
			}
			f.no(x, "the address of a %s", e.Type())
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			base, ix := f.indexOperands(x)
			i := f.index(ix)
			switch {
			case strings.HasSuffix(base.t, "[]"):
				return jval{s: ptrOver(elemJ(base.t), base.s, i), t: ptrOfArray(elemJ(base.t)), c: t}
			case isJPtr(base.t):
				if ix.konst && ix.cv == 0 {
					return jval{s: base.s, t: base.t, c: t}
				}
				return jval{s: base.s + ".add(" + i + ")", t: base.t, c: t}
			}
			f.no(x, "the address of an element of a %s", base.t)
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			if e.Type().Kind() == cc.Struct {
				v := f.member(x)
				return jval{s: v.s, t: v.t, c: t}
			}
			f.no(x, "the address of a member")
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			v := f.expr(x.CastExpression)
			v.c = t
			return v
		}
	}
	f.no(e, "the address of a %T", e)
	return jval{}
}

func (f *jfn) binary(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) jval {
	l, r := f.expr(le), f.expr(re)
	k, ok := scalarKind(x.Type())
	if !ok {
		f.no(x, "floating point")
	}
	return jval{s: f.arith(op, f.convK(l, k), r, k), t: k.java(), c: x.Type()}
}

func (f *jfn) shift(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) jval {
	return f.binary(x, op, le, re)
}

func (f *jfn) additive(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) jval {
	lp, rp := isPtrish(le.Type()), isPtrish(re.Type())
	switch {
	case lp && rp && op == "-":
		l, r := f.expr(le), f.expr(re)
		pt := l.t
		if strings.HasSuffix(pt, "[]") {
			pt = r.t
		}
		if strings.HasSuffix(pt, "[]") {
			pt = ptrOfArray(elemJ(pt))
		}
		if !isJPtr(pt) {
			f.no(x, "a pointer difference of a %s", pt)
		}
		return jval{s: f.conv(l, pt, nil) + ".sub(" + f.conv(r, pt, nil) + ")", t: "long", c: x.Type()}
	case lp || rp:
		pe, ne := le, re
		if rp {
			pe, ne = re, le
		}
		p, n := f.expr(pe), f.expr(ne)
		if strings.HasSuffix(p.t, "[]") {
			p = jval{s: ptrOver(elemJ(p.t), p.s, "0"), t: ptrOfArray(elemJ(p.t)), c: p.c}
		}
		if !isJPtr(p.t) {
			f.no(x, "arithmetic on a %s", p.t)
		}
		i := f.index(n)
		if op == "-" {
			i = "-" + jparen(i)
		}
		return jval{s: p.s + ".add(" + i + ")", t: p.t, c: x.Type()}
	}
	return f.binary(x, op, le, re)
}

// ptrEq is a == b for two pointer values of Java type t.
func ptrEq(t, a, b string) string {
	switch {
	case scalarPtrElem(t) != "":
		return t + ".eq(" + a + ", " + b + ")"
	case strings.HasPrefix(t, "Ptr<"):
		return "Ptr.eq(" + a + ", " + b + ")"
	}
	return a + " == " + b // references: a struct's, or an array's
}

func (f *jfn) compare(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) jval {
	lt, rt := le.Type(), re.Type()
	if isPtrish(lt) || isPtrish(rt) || (lt != nil && lt.Kind() == cc.Function) || (rt != nil && rt.Kind() == cc.Function) {
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
			return jval{s: s, t: "boolean", c: x.Type()}
		}
		l, r := f.expr(le), f.expr(re)
		t := l.t
		if strings.HasSuffix(t, "[]") && !strings.HasSuffix(r.t, "[]") {
			t = r.t
		}
		a, b := f.conv(l, t, nil), f.conv(r, t, nil)
		if op == "==" || op == "!=" {
			s := ptrEq(t, a, b)
			if op == "!=" {
				s = jnot(s)
			}
			return jval{s: s, t: "boolean", c: x.Type()}
		}
		if strings.HasSuffix(t, "[]") {
			t = ptrOfArray(elemJ(t))
			a, b = f.conv(l, t, nil), f.conv(r, t, nil)
		}
		if !isJPtr(t) {
			f.no(x, "an ordered comparison of %s", t)
		}
		m := map[string]string{"<": "lt", ">": "gt", "<=": "le", ">=": "ge"}[op]
		return jval{s: a + "." + m + "(" + b + ")", t: "boolean", c: x.Type()}
	}
	l, r := f.expr(le), f.expr(re)
	kl, okl := scalarKind(lt)
	kr, okr := scalarKind(rt)
	if !okl || !okr {
		f.no(x, "floating point")
	}
	k := usualK(kl, kr)
	a, b := f.convK(l, k), f.convK(r, k)
	if !k.signed && op != "==" && op != "!=" {
		cls := "Integer"
		if k.size == 8 {
			cls = "Long"
		}
		return jval{s: cls + ".compareUnsigned(" + a + ", " + b + ") " + op + " 0", t: "boolean", c: x.Type()}
	}
	return jval{s: jparen(a) + " " + op + " " + jparen(b), t: "boolean", c: x.Type()}
}

// logical is && or ||; what the right side does happens only when C would
// evaluate it.
func (f *jfn) logical(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) jval {
	l := f.expr(le)
	var r jval
	f.indent++
	pre := f.capture(func() { r = f.expr(re) })
	f.indent--
	if pre == "" {
		return jval{s: jparen(f.truth(l)) + " " + op + " " + jparen(f.truth(r)), t: "boolean", c: x.Type()}
	}
	t := f.newTemp("boolean", nil)
	f.stmt1("%s = %s", t, f.truth(l))
	if op == "&&" {
		f.line("if (%s) {", t)
	} else {
		f.line("if (!%s) {", t)
	}
	f.out.WriteString(pre)
	f.indent++
	f.stmt1("%s = %s", t, f.truth(r))
	f.indent--
	f.line("}")
	return jval{s: t, t: "boolean", c: x.Type()}
}

// ternary is ?: -- Java's own when neither arm does anything, else an if
// into a temporary; only the chosen arm is evaluated.
func (f *jfn) ternary(x *cc.ConditionalExpression, to string) jval {
	c := f.expr(x.LogicalOrExpression)
	var a, b jval
	f.indent++
	pa := f.capture(func() { a = f.exprTo(x.ExpressionList, to) })
	pb := f.capture(func() { b = f.exprTo(x.ConditionalExpression, to) })
	f.indent--
	ct := x.Type()
	var t string
	switch {
	case ct.Kind() == cc.Void:
		f.no(x, "a ?: of no value")
	case isPtrish(ct) || ct.Kind() == cc.Struct:
		t = to
		if t == "" {
			t = a.t
			if a.null || strings.HasSuffix(t, "[]") {
				t = b.t
			}
		}
		if strings.HasSuffix(t, "[]") {
			t = ptrOfArray(elemJ(t))
		}
	case a.t == "boolean" && b.t == "boolean":
		t = "boolean"
		ct = nil
	default:
		k, ok := scalarKind(ct)
		if !ok {
			f.no(x, "floating point")
		}
		t = k.java()
	}
	conv := func(v jval) string {
		if ct == nil {
			return v.s
		}
		return f.conv(v, t, ct)
	}
	if pa == "" && pb == "" {
		return jval{s: "(" + f.truth(c) + " ? " + conv(a) + " : " + conv(b) + ")", t: t, c: ct}
	}
	v := f.newTemp(t, ct)
	f.line("if (%s) {", f.truth(c))
	f.out.WriteString(pa)
	f.indent++
	f.stmt1("%s = %s", v, conv(a))
	f.indent--
	f.line("} else {")
	f.out.WriteString(pb)
	f.indent++
	f.stmt1("%s = %s", v, conv(b))
	f.indent--
	f.line("}")
	return jval{s: v, t: t, c: ct}
}

func (f *jfn) cast(x *cc.CastExpression, to string) jval {
	tt := x.Type()
	inner := x.CastExpression
	it := inner.Type()
	if tt.Kind() == cc.Void {
		f.exprStmt(inner)
		return jval{}
	}
	if k, ok := scalarKind(tt); ok {
		if _, ok := scalarKind(it); !ok {
			f.no(x, "a cast between a pointer and an integer")
		}
		v := f.expr(inner)
		return jval{s: f.convK(v, k), t: k.java(), c: tt}
	}
	if tt.Kind() != cc.Ptr {
		f.no(x, "a cast to %s", tt)
	}
	si := stripCasts(inner)
	if c, ok := si.(*cc.PostfixExpression); ok && c.Case == cc.PostfixExpressionCall && f.j.g.p.allocators[calleeName(c)] {
		return f.alloc(c, tt.(*cc.PointerType).Elem(), to)
	}
	if _, ok := scalarKind(it); ok {
		f.no(x, "a cast between a pointer and an integer")
	}
	v := f.exprTo(inner, to)
	if v.null {
		return jval{s: "null", t: to, c: tt, null: true}
	}
	want := to
	if want == "" {
		want = f.jt(tt, "", "a cast")
		if want != v.t && elemJ(want) == elemJ(v.t) {
			want = v.t // the same target; the cast's own class is the operand's
		}
	}
	if v.t == want {
		v.c = tt
		return v
	}
	if !sameTarget(tt, it) {
		f.no(x, "a pointer cast: %s to %s", it, tt)
	}
	return jval{s: f.conv(v, want, nil), t: want, c: tt}
}

// alloc is an allocation (Profile.Allocators): a new struct for one, n of
// them or n bytes -- storage the garbage collector owns.
func (f *jfn) alloc(c *cc.PostfixExpression, elem cc.Type, to string) jval {
	var args []cc.ExpressionNode
	for l := c.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	// sized by the first argument (Profile.Allocators); what the others do
	// still happens
	for _, a := range args[1:] {
		if hasEffect(a) {
			f.exprStmt(a)
		}
	}
	if k, ok := scalarKind(elem); ok {
		cls := ptrClass(k)
		n := f.sizeCount(args[0], elem)
		return jval{s: cls + ".alloc(" + n + ")", t: cls, c: c.Type()}
	}
	switch elem.Kind() {
	case cc.Struct:
		sn := f.jt(elem, "")
		n := f.sizeCount(args[0], elem)
		if n == "1" && (to == "" || to == sn) {
			return jval{s: "new " + sn + "()", t: sn, c: c.Type()}
		}
		return jval{s: "new Ptr<" + sn + ">(" + sn + ".array(" + n + "), 0)", t: "Ptr<" + sn + ">", c: c.Type()}
	case cc.Ptr:
		// the elements' class is the destination's, which the analysis knows
		et := elemJ(to)
		if !strings.HasPrefix(to, "Ptr<") {
			et = f.jt(elem, "")
		}
		n := f.sizeCount(args[0], elem)
		return jval{s: "new Ptr<" + et + ">(new " + raw(et) + "[" + n + "], 0)", t: "Ptr<" + et + ">", c: c.Type()}
	}
	f.no(c, "an allocation of a %s", elem)
	return jval{}
}

// sizeCount is a size in bytes as a count of elements of C type elem: the
// size itself for bytes, k for k * sizeof(T), 1 for sizeof(T) -- as an int.
func (f *jfn) sizeCount(n cc.ExpressionNode, elem cc.Type) string {
	if isByteType(elem) || elem.Kind() == cc.SChar {
		return f.index(f.expr(n))
	}
	es := elem.Size()
	switch x := constValue(n).(type) {
	case cc.Int64Value:
		if es > 0 && int64(x)%es == 0 {
			return fmt.Sprint(int64(x) / es)
		}
	case cc.UInt64Value:
		if es > 0 && int64(x)%es == 0 {
			return fmt.Sprint(int64(x) / es)
		}
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

// call is a function call, its arguments in the parameters' Java types.
func (f *jfn) call(x *cc.PostfixExpression, to string) jval {
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	name := calleeName(x)
	p := f.j.g.p
	switch {
	case name == "":
		f.no(x, "a function pointer: a call through one")
	case name == "__builtin_expect":
		return f.exprTo(args[0], to)
	case p.frees[name]:
		// the garbage collector frees; what the argument does still happens
		for _, a := range args {
			if hasEffect(a) {
				f.exprStmt(a)
			}
		}
		return jval{}
	case p.byteMove(name), name == p.Bytes.Set, name == p.Bytes.Cmp:
		return f.bytesCall(x, name, args)
	case p.allocators[name]:
		// C's void * converts to the destination's pointer: its type is
		// the destination's, and the size says the element's
		if to == "BytePtr" {
			return jval{s: "BytePtr.alloc(" + f.convK(f.expr(args[0]), jk{size: 8}) + ")", t: "BytePtr", c: x.Type()}
		}
		if to == "" {
			f.no(x, "an allocation of no element type")
		}
		elem := sizeofElem(args[0])
		if elem == nil {
			f.no(x, "an allocation of no element type")
		}
		return f.alloc(x, elem, to)
	}
	d := unparenE(x.PostfixExpression).(*cc.PrimaryExpression).ResolvedTo().(*cc.Declarator)
	ft, _ := d.Type().(*cc.FunctionType)
	if ft == nil {
		f.no(x, "a call of no function type")
	}
	if ft.IsVariadic() {
		f.no(x, "a variadic call: %s", name)
	}
	if why := f.j.sigWhy(name, ft); why != "" {
		f.no(x, "%s: in the signature of %s", why, name)
	}
	ps := ft.Parameters()
	if len(ps) == 1 && ps[0].Type().Kind() == cc.Void {
		ps = nil
	}
	if len(args) != len(ps) {
		f.no(x, "a call with %d arguments to a function of %d parameters", len(args), len(ps))
	}
	var as []string
	for i, a := range args {
		pt := ps[i].Type()
		if pt.Kind() == cc.Array {
			pt = pt.Decay()
		}
		jt := f.jt(pt, fmt.Sprintf("param:%s:%d", name, i))
		v := f.exprTo(a, jt)
		s := f.conv(v, jt, pt)
		if pt.Kind() == cc.Struct {
			s += ".copy()" // a struct is passed by value
		}
		as = append(as, s)
	}
	rt := f.jt(ft.Result(), "ret:"+name)
	return jval{s: f.j.jName(name) + "(" + strings.Join(as, ", ") + ")", t: rt, c: ft.Result()}
}

// bytesCall is memmove, memcpy, memset or memcmp (Profile.Bytes) of bytes,
// the runtime's Rt methods.
func (f *jfn) bytesCall(x *cc.PostfixExpression, name string, args []cc.ExpressionNode) jval {
	byteArg := func(a cc.ExpressionNode) string {
		s := stripCasts(a)
		et := elemCType(s)
		if !isPtrish(s.Type()) || et == nil || !isByteType(et) && et.Kind() != cc.SChar {
			f.no(x, "memmove, memset or memcmp of no bytes: of a %s", s.Type())
		}
		return f.conv(f.expr(s), "BytePtr", nil)
	}
	if len(args) != 3 {
		f.no(x, "memmove, memset or memcmp of no bytes: %s with %d arguments", name, len(args))
	}
	n := f.convK(f.expr(args[2]), jk{size: 8})
	p := f.j.g.p
	switch {
	case p.byteMove(name):
		return jval{s: "Rt.memmove(" + byteArg(args[0]) + ", " + byteArg(args[1]) + ", " + n + ")", t: "BytePtr", c: args[0].Type()}
	case name == p.Bytes.Set:
		return jval{s: "Rt.memset(" + byteArg(args[0]) + ", " + f.convK(f.expr(args[1]), jInt) + ", " + n + ")", t: "BytePtr", c: args[0].Type()}
	}
	return jval{s: "Rt.memcmp(" + byteArg(args[0]) + ", " + byteArg(args[1]) + ", " + n + ")", t: "int", c: x.Type()}
}

// exprStmt writes an expression evaluated for what it does.
func (f *jfn) exprStmt(e cc.ExpressionNode) {
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
			f.line("if (%s) {", f.truth(c))
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
	}
	v := f.expr(e)
	f.discard(v.s)
}

// discard writes a value computed for what computing it does: a call as
// itself, anything else that calls through Rt.use, and nothing when it does
// nothing.
func (f *jfn) discard(s string) {
	if s == "" || jpure(s) {
		return
	}
	for strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && jbalanced(s[1:len(s)-1]) {
		s = s[1 : len(s)-1]
	}
	if isOneCall(s) {
		f.stmt1("%s", s)
		return
	}
	f.stmt1("Rt.use(%s)", s)
}

// isOneCall says s is one call, name(args), and nothing after it.
var oneCallRe = regexp.MustCompile(`^[A-Za-z_][\w.]*\(`)

func isOneCall(s string) bool {
	m := oneCallRe.FindString(s)
	if m == "" || !strings.HasSuffix(s, ")") {
		return false
	}
	return jbalanced(s[len(m) : len(s)-1])
}
