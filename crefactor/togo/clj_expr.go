package togo

// clj_expr.go writes C's expressions in Clojure, from the lowered form: the
// C nodes that are left once what they do is taken apart, read through the
// function's substitutions.  Its decisions are the Java backend's (java_expr.go)
// -- which pointer class, which runtime call, where a struct is copied --
// on one difference: every C integer is a Clojure long holding the C value
// (doc/CLOJURE.md's contract), where the Java holds its bits in a primitive
// of the C type's size.  So
//
//   - a signed type's value is sign-extended, an unsigned type's narrower
//     than 64 bits zero-extended, and the usual conversions, ordering,
//     division and right shift are Clojure's own on them; a 64-bit unsigned
//     value is its bits, with Long/compareUnsigned, divideUnsigned and
//     unsigned-bit-shift-right where they differ;
//   - what can leave a type's range -- + - * << and unary - of a type
//     narrower than 64 bits, ~ of an unsigned one, a conversion to a
//     narrower type -- is brought back by the namespace's own i8/u8, i16/u16,
//     i32/u32;
//   - a Java array or a runtime pointer holds the bits in the primitive of
//     the element's size: a load widens them to the C value (unsigned ones
//     masked), a store narrows it (unchecked-byte, unchecked-short,
//     unchecked-int).

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// cv is an emitted value: its Clojure form, its Java type (the Java
// backend's vocabulary: int, BytePtr, Ptr<S_x>, byte[] ...) and its C type.
type cv struct {
	s     string
	t     string
	c     cc.Type
	konst bool
	cv    int64
	null  bool
	named bool // s names the constant cv
	kind  *jk  // the C kind of a value made here, whose c is nil
	typed bool // the Clojure compiler knows s's class: no hint is needed on it
}

// cljHint is the Clojure type hint of a Java type, "" for a primitive, a
// void * (Object) or none.
func cljHint(t string) string {
	switch t {
	case "", "byte", "short", "int", "long", "boolean", "void", "Object":
		return ""
	}
	if strings.HasSuffix(t, "[]") {
		switch t[:len(t)-2] {
		case "byte":
			return "bytes"
		case "short":
			return "shorts"
		case "int":
			return "ints"
		case "long":
			return "longs"
		case "boolean":
			return "booleans"
		}
		return "objects"
	}
	if strings.HasPrefix(t, "Ptr<") {
		return "Ptr"
	}
	if strings.HasPrefix(t, "Fn") {
		return "clojure.lang.IFn"
	}
	return t
}

// isIntJ says a Java type is an integer primitive: a Clojure long here.
func isIntJ(t string) bool {
	return t == "byte" || t == "short" || t == "int" || t == "long"
}

// tagged is v's form hinted with its class when the compiler would not
// know it: what a method call on it, or an overloaded static call, needs.
func tagged(v cv) string {
	h := cljHint(v.t)
	if v.typed || h == "" || v.null {
		return v.s
	}
	for _, sp := range []string{"(if ", "(let ", "(case ", "(do ", "(loop "} {
		if strings.HasPrefix(v.s, sp) {
			// a special form takes no hint: a local does
			return "(let [^" + h + " h__ " + v.s + "] h__)"
		}
	}
	return "^" + h + " " + v.s
}

// clit is the C value v converted to kind k, as a Clojure literal.
func clit(v int64, k jk) string {
	if k.boolean {
		if v != 0 {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(truncK(v, k))
}

// fits says every value of kind from is a value of kind to: the conversion
// changes no C value (and for 64 bits, no bits).
func fits(from, to jk) bool {
	if to.size == 8 {
		return true
	}
	if from.size == to.size && from.signed == to.signed {
		return true
	}
	return from.size < to.size && (!from.signed || to.signed)
}

// wrapK is s, a long, brought into kind k's range: C's conversion to k.
func wrapK(s string, k jk) string {
	switch {
	case k.size == 1 && k.signed:
		return "(i8 " + s + ")"
	case k.size == 1:
		return "(u8 " + s + ")"
	case k.size == 2 && k.signed:
		return "(i16 " + s + ")"
	case k.size == 2:
		return "(u16 " + s + ")"
	case k.size == 4 && k.signed:
		return "(i32 " + s + ")"
	case k.size == 4:
		return "(u32 " + s + ")"
	}
	return s
}

// numconvC is the value s of kind from as kind to.
func numconvC(s string, from, to jk) string {
	if fits(from, to) {
		return s
	}
	return wrapK(s, to)
}

// loadK is a C value of kind k from s, the bits a Java array or a runtime
// pointer holds for it.
func loadK(s string, k jk) string {
	switch {
	case k.boolean:
		return s
	case k.size == 1 && !k.signed:
		return "(bit-and " + s + " 0xff)"
	case k.size == 2 && !k.signed:
		return "(bit-and " + s + " 0xffff)"
	case k.size == 4 && !k.signed:
		return "(bit-and " + s + " 0xffffffff)"
	case k.size == 8:
		return s
	}
	return "(long " + s + ")"
}

// storeK is the bits a Java array or runtime pointer of kind k holds for
// the C value s.
func storeK(s string, k jk) string {
	switch {
	case k.boolean:
		return "(boolean " + s + ")"
	case k.size == 1:
		return "(unchecked-byte " + s + ")"
	case k.size == 2:
		return "(unchecked-short " + s + ")"
	case k.size == 4:
		return "(unchecked-int " + s + ")"
	}
	return s
}

// kindOfJ is the kind of the elements a Java primitive type holds, for a C
// type c when it is known.
func (f *cfn) kindOf(v cv) (jk, bool) {
	if v.kind != nil {
		return *v.kind, true
	}
	return scalarKind(v.c)
}

// convK is v as a scalar of kind k.
func (f *cfn) convK(v cv, k jk) string {
	if v.null {
		return clit(0, k)
	}
	if k.boolean {
		if v.t == "boolean" {
			return v.s
		}
		return f.truth(v)
	}
	if v.konst {
		if v.named && truncK(v.cv, k) == v.cv {
			return v.s
		}
		return clit(v.cv, k)
	}
	if v.t == "boolean" {
		return "(if " + v.s + " 1 0)"
	}
	from, ok := f.kindOf(v)
	if !ok {
		f.no(nil, "a cast between a pointer and an integer")
	}
	return numconvC(v.s, from, k)
}

// conv is v as Java type to, of C type tc.
func (f *cfn) conv(v cv, to string, tc cc.Type) string { return f.convV(v, to, tc).s }

// convV is conv, the value.
func (f *cfn) convV(v cv, to string, tc cc.Type) cv {
	if k, ok := scalarKind(tc); ok {
		return cv{s: f.convK(v, k), t: k.java(), c: tc, typed: true}
	}
	if v.null || v.konst && v.cv == 0 {
		return cv{s: "nil", t: to, c: tc, null: true}
	}
	if v.t == to {
		return v
	}
	if _, ok := scalarKind(v.c); ok || v.t == "boolean" {
		f.no(nil, "a cast between a pointer and an integer")
	}
	switch {
	case to == "Object":
		// into a void *: as it is, an array as the pointer it decays to
		if strings.HasSuffix(v.t, "[]") {
			return cv{s: ptrOverC(elemJ(v.t), tagged(v), "0"), t: ptrOfArray(elemJ(v.t)), c: tc, typed: true}
		}
		return cv{s: v.s, t: to, c: tc, typed: v.typed}
	case v.t == "Object" && strings.HasPrefix(to, "Ptr<"):
		return cv{s: "(Rt/ptr " + v.s + ")", t: to, c: tc, typed: true}
	case v.t == "Object" && isJPtr(to):
		return cv{s: v.s, t: to, c: tc}
	case v.t == "Object":
		return cv{s: "(Rt/obj " + v.s + ")", t: to, c: tc}
	}
	switch {
	case strings.HasSuffix(v.t, "[]") && ptrOfArray(elemJ(v.t)) == to:
		return cv{s: ptrOverC(elemJ(v.t), tagged(v), "0"), t: to, c: tc, typed: true}
	case strings.HasSuffix(v.t, "[]") && elemJ(v.t) == to:
		return cv{s: "(aget " + tagged(v) + " 0)", t: to, c: tc}
	case strings.HasPrefix(v.t, "Ptr<") && elemJ(v.t) == to:
		return cv{s: "(Ptr/ref " + v.s + ")", t: to, c: tc}
	case strings.HasPrefix(to, "Ptr<") && elemJ(to) == v.t:
		return cv{s: "(Ptr/one " + v.s + ")", t: to, c: tc, typed: true}
	}
	f.no(nil, "a pointer conversion: %s to %s", v.t, to)
	return cv{}
}

// ptrOverC is the runtime pointer to element k of the Java array a of
// elements of Java type e.
func ptrOverC(e, a, k string) string {
	switch e {
	case "byte":
		return "(BytePtr. " + a + " " + k + ")"
	case "short":
		return "(ShortPtr. " + a + " " + k + ")"
	case "int":
		return "(IntPtr. " + a + " " + k + ")"
	case "long":
		return "(LongPtr. " + a + " " + k + ")"
	case "boolean":
		return "(BoolPtr. " + a + " " + k + ")"
	}
	return "(Ptr. " + a + " " + k + ")"
}

// truth is v as a boolean value.
func (f *cfn) truth(v cv) string {
	switch {
	case v.t == "boolean":
		return v.s
	case v.konst:
		return fmt.Sprint(v.cv != 0)
	case v.null:
		return "false"
	case isPtrish(v.c) || v.c == nil || !isIntJ(v.t) && v.kind == nil:
		return "(some? " + v.s + ")"
	}
	return "(not (zero? " + v.s + "))"
}

// falsity is !v as a boolean value.
func (f *cfn) falsity(v cv) string {
	switch {
	case v.t == "boolean":
		return "(not " + v.s + ")"
	case v.konst:
		return fmt.Sprint(v.cv == 0)
	case v.null:
		return "true"
	case isPtrish(v.c) || v.c == nil || !isIntJ(v.t) && v.kind == nil:
		return "(nil? " + v.s + ")"
	}
	return "(zero? " + v.s + ")"
}

// cond is v as an if's test: a form, and whether the if's arms go the
// other way round -- (if (zero? x) else then) reads better than a not.
func (f *cfn) cond(v cv) (string, bool) {
	switch {
	case v.t == "boolean":
		if strings.HasPrefix(v.s, "(not ") && strings.HasSuffix(v.s, ")") && balancedC(v.s[5:len(v.s)-1]) {
			return v.s[5 : len(v.s)-1], true
		}
		return v.s, false
	case v.konst || v.null:
		return f.truth(v), false
	}
	return f.falsity(v), true
}

// balancedC says s's brackets balance, reading past strings.
func balancedC(s string) bool {
	d := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
			if d < 0 {
				return false
			}
		}
	}
	return d == 0
}

// index is v as an index or an offset: a long.
func (f *cfn) index(v cv) string { return f.convK(v, jk{size: 8, signed: true}) }

func (f *cfn) expr(e cc.ExpressionNode) cv { return f.exprTo(e, "") }

// exprTo is expr told the Java type the value goes to, for what has no type
// of its own (a null, a function, an allocation).
func (f *cfn) exprTo(e cc.ExpressionNode, to string) cv {
	if f.lf != nil {
		if r, ok := f.lf.sub[e]; ok {
			return f.lexprTo(r, to)
		}
	}
	return f.exprNode(e, to)
}

// lexprTo is a lowered value.
func (f *cfn) lexprTo(r lexpr, to string) cv {
	if r.v != nil {
		return f.varRef(r.v)
	}
	if r.raw {
		return f.exprNode(r.n, to)
	}
	return f.exprTo(r.n, to)
}

// exprNode is exprTo of the node itself, its operands through the
// substitutions.  Every integer constant is written as its value.
func (f *cfn) exprNode(e cc.ExpressionNode, to string) cv {
	if isNullConst(e) && e.Type() != nil && e.Type().Kind() == cc.Ptr {
		return cv{s: "nil", t: to, c: e.Type(), null: true}
	}
	if k, ok := scalarKind(e.Type()); ok && !hasEffect(e) && !f.hasSub(e) {
		if v, known := intValue(e.Value()); known {
			if p, ok := unparenE(e).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
				return f.primary(p, to)
			}
			return cv{s: clit(v, k), t: k.java(), c: e.Type(), konst: true, cv: truncK(v, k), typed: true}
		}
	}
	return f.exprTo1(e, to)
}

// hasSub says a substitution replaces a node under e.
func (f *cfn) hasSub(e cc.ExpressionNode) bool {
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

func (f *cfn) exprTo1(e cc.ExpressionNode, to string) cv {
	if x := f.gaMember(e); x != nil && unparenE(e) == cc.ExpressionNode(x) && isJPtr(to) {
		return f.gadata(x, to, e.Type())
	}
	if u, ok := e.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
		if p, ok := unparenE(u.CastExpression).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
			if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
				return f.fnValue(d, to, u)
			}
		}
	}
	switch x := e.(type) {
	case *cc.ConstantExpression:
		return f.exprTo(x.ConditionalExpression, to)
	case *cc.ExpressionList:
		if x.ExpressionList != nil {
			f.no(x, "a comma the lowering did not take apart")
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
		return f.binary(x, op, x.ShiftExpression, x.AdditiveExpression)
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
		return f.logical(x, "and", x.LogicalAndExpression, x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		return f.logical(x, "or", x.LogicalOrExpression, x.LogicalAndExpression)
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return f.ternary(x, to)
		}
	case *cc.AssignmentExpression:
		f.no(x, "an assignment the lowering did not take apart")
	}
	f.no(e, "an expression %T", e)
	return cv{}
}

func (f *cfn) primary(x *cc.PrimaryExpression, to string) cv {
	switch x.Case {
	case cc.PrimaryExpressionIdent:
		return f.identTo(x, to)
	case cc.PrimaryExpressionInt, cc.PrimaryExpressionChar:
		k, ok := scalarKind(x.Type())
		if !ok {
			f.no(x, "a constant of type %s", x.Type())
		}
		v, ok := intValue(x.Value())
		if !ok {
			f.no(x, "floating point")
		}
		return cv{s: clit(v, k), t: k.java(), c: x.Type(), konst: true, cv: truncK(v, k), typed: true}
	case cc.PrimaryExpressionString:
		sv, ok := x.Value().(cc.StringValue)
		if !ok {
			f.no(x, "a wide string")
		}
		return cv{s: "(BytePtr/lit " + cljQuote(strings.TrimSuffix(string(sv), "\x00")) + ")", t: "BytePtr", c: x.Type(), typed: true}
	case cc.PrimaryExpressionExpr:
		return f.exprTo(x.ExpressionList, to)
	case cc.PrimaryExpressionFloat:
		f.no(x, "floating point")
	case cc.PrimaryExpressionStmt:
		f.no(x, "a statement expression")
	}
	f.no(x, "a primary expression %v", x.Case)
	return cv{}
}

// cljQuote is a Clojure string of bytes: printable ASCII as itself, every
// other byte an octal escape, so that each char is one byte (BytePtr.lit).
func cljQuote(s string) string {
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
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\%03o`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// identTo is a name as a value.
func (f *cfn) identTo(x *cc.PrimaryExpression, to string) cv {
	switch d := x.ResolvedTo().(type) {
	case *cc.Declarator:
		t := d.Type()
		if t.Kind() == cc.Function {
			return f.fnValue(d, to, x)
		}
		if f.lf != nil {
			if v, ok := f.lf.byDecl[d]; ok {
				return f.varRef(v)
			}
		}
		if d.Name() == "__func__" {
			f.no(x, "__func__")
		}
		s := f.c.slotOf(d, f.lf)
		if s == nil {
			f.no(x, "a name with no declaration here: %s", d.Name())
		}
		return f.slotRead(s)
	case *cc.Enumerator:
		name := f.c.enumName(x.Token.SrcStr())
		k, ok := scalarKind(x.Type())
		if !ok {
			k = jInt
		}
		v, _ := intValue(d.Value())
		v = truncK(v, k)
		f.c.enums[name] = fmt.Sprintf("(def ^:const %s %s)\n", name, clit(v, k))
		return cv{s: name, t: k.java(), c: x.Type(), konst: true, named: true, cv: v, typed: true}
	}
	f.no(x, "an identifier resolved to %T", x.ResolvedTo())
	return cv{}
}

// slotRead is a file-scope object's value: its slot, or what its box holds.
func (f *cfn) slotRead(s *cslot) cv {
	g := "(g ed " + s.name + ")"
	if s.boxed {
		k, _ := scalarKind(s.c)
		if s.c.Kind() == cc.Ptr {
			return cv{s: "(aget " + g + " 0)", t: s.jt, c: s.c}
		}
		return cv{s: loadK("(aget "+g+" 0)", k), t: s.jt, c: s.c, typed: true}
	}
	return cv{s: g, t: s.jt, c: s.c, typed: true}
}

// varRef is a function's variable as a value.
func (f *cfn) varRef(v *lvar) cv {
	cvr := f.vars[v]
	if cvr == nil {
		f.no(nil, "a variable with no type: %s", v.name)
	}
	if cvr.boxed {
		k, _ := scalarKind(v.c)
		if v.c.Kind() == cc.Ptr {
			return cv{s: "(aget " + v.name + " 0)", t: cvr.jt, c: v.c}
		}
		return cv{s: loadK("(aget "+v.name+" 0)", k), t: cvr.jt, c: v.c, typed: true}
	}
	c := v.c
	if v.boolean {
		return cv{s: v.name, t: "boolean", c: nil, typed: true}
	}
	return cv{s: v.name, t: cvr.jt, c: c, typed: true}
}

// fnValue is function d used as a value of the function type to -- the
// function itself, one object, so that two uses of it compare equal as C's
// pointers do; or, when the pointer's parameters or result are other
// classes than the function's (the analysis gave a pointer to a struct a
// Ptr on one side), an adapter made once that converts them.
func (f *cfn) fnValue(d *cc.Declarator, to string, at cc.Node) cv {
	name := d.Name()
	ft := d.Type().(*cc.FunctionType)
	j := f.c.j
	own, why := j.fnIfaceK(ft, "param:"+name, "ret:"+name)
	if why != "" {
		f.no(at, "%s: in the signature of %s", why, name)
	}
	iface := own
	if _, ok := j.ifaceByName[to]; ok {
		iface = to
	}
	fn := f.c.fnName(name)
	if iface == own {
		return cv{s: fn, t: iface, c: d.Type().Pointer()}
	}
	key := name + "/" + iface
	if fld, ok := f.c.adapters[key]; ok {
		return cv{s: fld, t: iface, c: d.Type().Pointer()}
	}
	w := j.ifaceByName[iface]
	m := j.ifaceByName[own]
	if len(w.params) != len(m.params) {
		f.no(at, "a function pointer: %s as a function of %d parameters", name, len(w.params))
	}
	var ps, as []string
	for i, cp := range ft.Parameters() {
		if cp.Type() == nil || cp.Type().Kind() == cc.Void {
			continue
		}
		a := fmt.Sprintf("a%d", len(ps))
		ps = append(ps, a)
		pt := cp.Type()
		if pt.Kind() == cc.Array {
			pt = pt.Decay()
		}
		as = append(as, f.conv(cv{s: a, t: w.params[i], c: pt}, m.params[i], pt))
	}
	call := "(" + fn + " ed" + strings.Join(append([]string{""}, as...), " ") + ")"
	if m.result != "void" && w.result != m.result {
		call = f.conv(cv{s: call, t: m.result, c: ft.Result()}, w.result, ft.Result())
	}
	fld := "fp_" + fn + "_" + iface
	f.c.adapters[key] = fld
	f.c.adapterText = append(f.c.adapterText, fmt.Sprintf("(def ^:private %s (fn [ed %s] %s))\n", fld, strings.Join(ps, " "), call))
	return cv{s: fld, t: iface, c: d.Type().Pointer()}
}

// fieldInfo is a member's Clojure name, its Java type, and how it is held.
func (f *cfn) fieldInfo(x *cc.PostfixExpression) (*cc.Field, string, string) {
	fl := x.Field()
	if fl == nil {
		f.no(x, "a member with no field")
	}
	if fl.IsBitfield() {
		f.no(x, "a bitfield")
	}
	if fl.Name() == "" || fl.ParentType() == nil || (fl.ParentType().Kind() != cc.Struct && fl.ParentType().Kind() != cc.Union) {
		f.no(x, "a member of no struct")
	}
	return fl, f.c.memberName(fl), f.jt(fl.Type(), fieldKey(fl), "a member", fl.Name())
}

// member is s.x or p->x.
func (f *cfn) member(x *cc.PostfixExpression) cv {
	fl, name, ft := f.fieldInfo(x)
	obj := f.memberObj(x)
	switch {
	case f.c.j.boxedField[fieldKey(fl)]:
		box := "^" + cljHint(ft+"[]") + " (.-" + name + " " + tagged(obj) + ")"
		if fl.Type().Kind() == cc.Ptr {
			return cv{s: "(aget " + box + " 0)", t: ft, c: x.Type()}
		}
		k, _ := scalarKind(fl.Type())
		return cv{s: loadK("(aget "+box+" 0)", k), t: ft, c: x.Type(), typed: true}
	case isAggr(fl.Type()) || fl.Type().Kind() == cc.Array:
		return cv{s: "(.-" + name + " " + tagged(obj) + ")", t: ft, c: x.Type()}
	}
	_, scalar := scalarKind(fl.Type())
	return cv{s: "(." + name + " " + tagged(obj) + ")", t: ft, c: x.Type(), typed: scalar}
}

// memberBox is the one-element array a member whose address is taken is.
func (f *cfn) memberBox(x *cc.PostfixExpression) (string, string) {
	fl, name, ft := f.fieldInfo(x)
	if !f.c.j.boxedField[fieldKey(fl)] {
		f.no(x, "the address of a member")
	}
	return "^" + cljHint(ft+"[]") + " (.-" + name + " " + tagged(f.memberObj(x)) + ")", ft
}

// memberObj is the struct object s.x or p->x selects from, typed.
func (f *cfn) memberObj(x *cc.PostfixExpression) cv {
	if x.Case == cc.PostfixExpressionSelect {
		return f.expr(x.PostfixExpression)
	}
	b := f.expr(x.PostfixExpression)
	return f.structRef(b, x)
}

// structRef is the struct object a pointer value points at.
func (f *cfn) structRef(b cv, at cc.Node) cv {
	switch {
	case strings.HasPrefix(b.t, "Ptr<"):
		return cv{s: "(.get " + tagged(b) + ")", t: elemJ(b.t), c: elemOf(b.c)}
	case strings.HasSuffix(b.t, "[]"):
		return cv{s: "(aget " + tagged(b) + " 0)", t: elemJ(b.t), c: elemOf(b.c)}
	case b.null:
		f.no(at, "a member of NULL")
	}
	return b
}

// gaMember is e, through parentheses and casts, when it is the growarray's
// storage (Profile.GrowArray): gap->ga_data.
func (f *cfn) gaMember(e cc.ExpressionNode) *cc.PostfixExpression {
	if f.c.g.p.GrowArray.Data == "" {
		return nil
	}
	if f.hasSub(e) {
		// the storage is read through its object, which may be a temporary
	}
	x, ok := unparenE(stripCasts(e)).(*cc.PostfixExpression)
	if !ok || !f.c.g.gaData(memberOf(x)) {
		return nil
	}
	return x
}

// gadata is the growarray's storage x as the Java pointer type to: the
// accessor that makes or grows it as that type (Ga), as the Go's GaData[T].
func (f *cfn) gadata(x *cc.PostfixExpression, to string, c cc.Type) cv {
	if !isJPtr(to) {
		f.no(x, "the growarray's storage as a %s", to)
	}
	if f.c.g.p.GrowArray.MaxLen == "" {
		f.no(x, "a growarray with no MaxLen in the profile")
	}
	j := f.c.j
	h := j.gaUsed[to]
	if h == nil {
		st := x.PostfixExpression.Type()
		if x.Case == cc.PostfixExpressionPSelect {
			st = elemOf(st.Decay())
		}
		cls, why := j.structName(st)
		if why != "" {
			f.no(x, "%s", why)
		}
		name := "GA_" + regexp.MustCompile(`\W+`).ReplaceAllString(strings.TrimSuffix(to, ">"), "_")
		h = &gaHelper{name: name, class: cls}
		j.gaUsed[to] = h
	}
	return cv{s: "(" + h.name + " " + tagged(f.memberObj(x)) + ")", t: to, c: c, typed: true}
}

func (f *cfn) postfix(x *cc.PostfixExpression, to string) cv {
	switch x.Case {
	case cc.PostfixExpressionIndex:
		base, ix := f.indexOperands(x)
		i := f.index(ix)
		return f.elemAt(base, i, ix, x)
	case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
		return f.member(x)
	case cc.PostfixExpressionCall:
		return f.call(x, to)
	case cc.PostfixExpressionComplit:
		// storage of its own, made where C evaluates it, with its list
		t := x.TypeName.Type()
		jt := f.jt(t, "", "a compound literal")
		if !isAggr(t) && t.Kind() != cc.Array {
			f.no(x, "a compound literal of a %s", t)
		}
		return f.freshInit(t, jt, &cc.Initializer{Case: cc.InitializerInitList, InitializerList: x.InitializerList, Token: x.Token}, "")
	case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
		f.no(x, "an increment the lowering did not take apart")
	}
	f.no(x, "a postfix expression %v", x.Case)
	return cv{}
}

// elemAt is base[i].
func (f *cfn) elemAt(base cv, i string, ix cv, x cc.ExpressionNode) cv {
	switch {
	case strings.HasSuffix(base.t, "[]"):
		et := elemJ(base.t)
		s := "(aget " + tagged(base) + " " + i + ")"
		if k, ok := scalarKind(x.Type()); ok && scalarPtrElem(ptrOfArray(et)) != "" {
			return cv{s: loadK(s, k), t: et, c: x.Type(), typed: true}
		}
		return cv{s: s, t: et, c: x.Type()}
	case scalarPtrElem(base.t) != "":
		s := "(.at " + tagged(base) + " " + i + ")"
		k, _ := scalarKind(x.Type())
		return cv{s: loadK(s, k), t: elemJ(base.t), c: x.Type(), typed: true}
	case strings.HasPrefix(base.t, "Ptr<"):
		return cv{s: "(.at " + tagged(base) + " " + i + ")", t: elemJ(base.t), c: x.Type()}
	case ix.konst && ix.cv == 0 && isAggr(x.Type()):
		return cv{s: base.s, t: base.t, c: x.Type(), typed: base.typed} // p[0] of a pointer that walks nowhere
	}
	f.no(x, "an index into a %s", base.t)
	return cv{}
}

// indexOperands are a[i]'s array or pointer and index.
func (f *cfn) indexOperands(x *cc.PostfixExpression) (cv, cv) {
	if !isPtrish(x.PostfixExpression.Type()) {
		f.no(x, "an index written i[a]")
	}
	return f.expr(x.PostfixExpression), f.expr(x.ExpressionList)
}

// clv is somewhere a value is stored.
type clv struct {
	get cv
	t   string
	c   cc.Type
	set func(v string) cbind // the step that stores v, a value of the lvalue's C type
	agg bool                 // a struct: assigned by set()
}

func (f *cfn) lval(e cc.ExpressionNode) clv {
	if f.lf != nil {
		if r, ok := f.lf.sub[e]; ok {
			if r.v != nil {
				return f.varLval(r.v)
			}
			e = r.n
		}
	}
	e = unparenE(e)
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			d, _ := x.ResolvedTo().(*cc.Declarator)
			if d != nil && f.lf != nil {
				if v, ok := f.lf.byDecl[d]; ok {
					return f.varLval(v)
				}
			}
			if d != nil {
				if s := f.c.slotOf(d, f.lf); s != nil {
					return f.slotLval(s)
				}
			}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			fl, name, ft := f.fieldInfo(x)
			obj := tagged(f.memberObj(x))
			get := f.member(x)
			switch {
			case f.c.j.boxedField[fieldKey(fl)]:
				k, _ := scalarKind(fl.Type())
				box := "^" + cljHint(ft+"[]") + " (.-" + name + " " + obj + ")"
				return clv{get: get, t: ft, c: x.Type(), set: func(v string) cbind {
					if fl.Type().Kind() == cc.Ptr {
						return cbind{"_", "(aset " + box + " 0 " + v + ")"}
					}
					return cbind{"_", "(aset " + box + " 0 " + storeK(v, k) + ")"}
				}}
			case isAggr(fl.Type()):
				return clv{get: get, t: ft, c: x.Type(), agg: true}
			case fl.Type().Kind() == cc.Array:
				f.no(x, "an assignment to an array")
			}
			k, sc := scalarKind(fl.Type())
			return clv{get: get, t: ft, c: x.Type(), set: func(v string) cbind {
				if sc && k.boolean {
					v = "(boolean " + v + ")"
				}
				return cbind{"_", "(.set_" + name + " " + obj + " " + v + ")"}
			}}
		case cc.PostfixExpressionIndex:
			base, ix := f.indexOperands(x)
			i := f.index(ix)
			get := f.elemAt(base, i, ix, x)
			switch {
			case isAggr(x.Type()):
				return clv{get: get, t: get.t, c: x.Type(), agg: true}
			case strings.HasSuffix(base.t, "[]"):
				k, sc := scalarKind(x.Type())
				arr := tagged(base)
				return clv{get: get, t: get.t, c: x.Type(), set: func(v string) cbind {
					if sc {
						v = storeK(v, k)
					}
					return cbind{"_", "(aset " + arr + " " + i + " " + v + ")"}
				}}
			case isJPtr(base.t):
				k, sc := scalarKind(x.Type())
				p := tagged(base)
				return clv{get: get, t: get.t, c: x.Type(), set: func(v string) cbind {
					if sc {
						v = storeK(v, k)
					}
					return cbind{"_", "(.set " + p + " " + i + " " + v + ")"}
				}}
			}
			f.no(x, "a store into a %s", base.t)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			b := f.expr(x.CastExpression)
			get := f.derefOf(b, x)
			switch {
			case isAggr(x.Type()):
				return clv{get: get, t: get.t, c: x.Type(), agg: true}
			case strings.HasSuffix(b.t, "[]"):
				k, sc := scalarKind(x.Type())
				arr := tagged(b)
				return clv{get: get, t: get.t, c: x.Type(), set: func(v string) cbind {
					if sc {
						v = storeK(v, k)
					}
					return cbind{"_", "(aset " + arr + " 0 " + v + ")"}
				}}
			case isJPtr(b.t):
				k, sc := scalarKind(x.Type())
				p := tagged(b)
				return clv{get: get, t: get.t, c: x.Type(), set: func(v string) cbind {
					if sc {
						v = storeK(v, k)
					}
					return cbind{"_", "(.put " + p + " " + v + ")"}
				}}
			}
			f.no(x, "a store through a %s", b.t)
		}
	}
	f.no(e, "a store into %T", e)
	return clv{}
}

// varLval is a function's variable as somewhere a value is stored.
func (f *cfn) varLval(v *lvar) clv {
	cvr := f.vars[v]
	get := f.varRef(v)
	switch {
	case cvr.boxed:
		k, _ := scalarKind(v.c)
		return clv{get: get, t: cvr.jt, c: v.c, set: func(s string) cbind {
			if v.c.Kind() == cc.Ptr {
				return cbind{"_", "(aset " + v.name + " 0 " + s + ")"}
			}
			return cbind{"_", "(aset " + v.name + " 0 " + storeK(s, k) + ")"}
		}}
	case cvr.fixed:
		if v.c.Kind() == cc.Array {
			return clv{get: get, t: cvr.jt, c: v.c}
		}
		return clv{get: get, t: cvr.jt, c: v.c, agg: true}
	}
	return clv{get: get, t: get.t, c: v.c, set: func(s string) cbind {
		return cbind{v.name, s}
	}}
}

// slotLval is a file-scope object as somewhere a value is stored.
func (f *cfn) slotLval(s *cslot) clv {
	get := f.slotRead(s)
	g := "(g ed " + s.name + ")"
	switch {
	case s.boxed:
		k, _ := scalarKind(s.c)
		return clv{get: get, t: s.jt, c: s.c, set: func(v string) cbind {
			if s.c.Kind() == cc.Ptr {
				return cbind{"_", "(aset " + g + " 0 " + v + ")"}
			}
			return cbind{"_", "(aset " + g + " 0 " + storeK(v, k) + ")"}
		}}
	case isAggr(s.c):
		return clv{get: get, t: s.jt, c: s.c, agg: true}
	case s.c.Kind() == cc.Array:
		return clv{get: get, t: s.jt, c: s.c}
	}
	return clv{get: get, t: s.jt, c: s.c, set: func(v string) cbind {
		return cbind{"_", "(g! ed " + s.name + " " + v + ")"}
	}}
}

// derefOf is *b.
func (f *cfn) derefOf(b cv, at cc.ExpressionNode) cv {
	switch {
	case strings.HasSuffix(b.t, "[]"):
		s := "(aget " + tagged(b) + " 0)"
		if k, ok := scalarKind(at.Type()); ok && scalarPtrElem(ptrOfArray(elemJ(b.t))) != "" {
			return cv{s: loadK(s, k), t: elemJ(b.t), c: at.Type(), typed: true}
		}
		return cv{s: s, t: elemJ(b.t), c: at.Type()}
	case scalarPtrElem(b.t) != "":
		k, _ := scalarKind(at.Type())
		return cv{s: loadK("(.get "+tagged(b)+")", k), t: elemJ(b.t), c: at.Type(), typed: true}
	case strings.HasPrefix(b.t, "Ptr<"):
		return cv{s: "(.get " + tagged(b) + ")", t: elemJ(b.t), c: at.Type()}
	case isAggr(at.Type()):
		return cv{s: b.s, t: b.t, c: at.Type(), typed: b.typed} // *p of a pointer that walks nowhere: the object
	}
	f.no(at, "* of a %s", b.t)
	return cv{}
}

// incdec is the step of ++ or -- of an lvalue.
func (f *cfn) incdec(lv clv, inc bool, n cc.Node) cbind {
	d := "1"
	if !inc {
		d = "-1"
	}
	if lv.set == nil {
		f.no(n, "++ or -- of a %s", lv.t)
	}
	switch {
	case isJPtr(lv.t):
		return lv.set("(.add " + tagged(lv.get) + " " + d + ")")
	case lv.t == "boolean":
		f.no(n, "++ or -- of a _Bool")
	}
	k, ok := scalarKind(lv.c)
	if !ok {
		f.no(n, "++ or -- of a %s", lv.t)
	}
	op := "(inc "
	if !inc {
		op = "(dec "
	}
	pk := promote(k)
	return lv.set(numconvC(wrapK(op+lv.get.s+")", pk), pk, k))
}

// assign is the step of an assignment of r to lv.
func (f *cfn) assign(lv clv, r cv, at cc.Node) cbind {
	if lv.agg {
		return cbind{"_", "(.set " + tagged(lv.get) + " " + r.s + ")"} // C's assignment of a struct is a copy
	}
	if lv.set == nil {
		f.no(at, "an assignment to a %s", lv.t)
	}
	return lv.set(f.conv(r, lv.t, lv.c))
}

// assignOp is the step of lv op= r.
func (f *cfn) assignOp(lv clv, op string, r cv, at cc.Node) cbind {
	if lv.set == nil {
		f.no(at, "%s= on a %s", op, lv.t)
	}
	if isJPtr(lv.t) && (op == "+" || op == "-") {
		n := f.index(r)
		if op == "-" {
			n = "(- " + n + ")"
		}
		return lv.set("(.add " + tagged(lv.get) + " " + n + ")")
	}
	lk, ok := scalarKind(lv.c)
	if !ok {
		f.no(at, "%s= on a %s", op, lv.t)
	}
	rk, ok := scalarKind(r.c)
	if r.kind != nil {
		rk, ok = *r.kind, true
	}
	if !ok && r.t != "boolean" {
		f.no(at, "%s= of a %s", op, r.t)
	}
	if r.t == "boolean" {
		rk = jInt
	}
	common := usualK(lk, rk)
	if op == "<<" || op == ">>" {
		common = promote(lk)
	}
	res := f.arith(op, f.convK(lv.get, common), r, common)
	return lv.set(f.convK(cv{s: res, t: common.java(), kind: &common}, lk))
}

// arith is a op b in common kind k, a already converted: the value is in k's
// range.
func (f *cfn) arith(op, a string, bv cv, k jk) string {
	if op == "<<" || op == ">>" {
		b := f.convK(bv, jk{size: 8, signed: true})
		switch {
		case op == "<<":
			return wrapK("(bit-shift-left "+a+" "+b+")", k)
		case k.size == 8 && !k.signed:
			return "(unsigned-bit-shift-right " + a + " " + b + ")"
		}
		return "(bit-shift-right " + a + " " + b + ")"
	}
	b := f.convK(bv, k)
	switch op {
	case "+", "-", "*":
		return wrapK("("+op+" "+a+" "+b+")", k)
	case "/":
		if k.size == 8 && !k.signed {
			return "(Long/divideUnsigned " + a + " " + b + ")"
		}
		if k.signed {
			return wrapK("(quot "+a+" "+b+")", k)
		}
		return "(quot " + a + " " + b + ")"
	case "%":
		if k.size == 8 && !k.signed {
			return "(Long/remainderUnsigned " + a + " " + b + ")"
		}
		return "(rem " + a + " " + b + ")"
	case "&":
		return "(bit-and " + a + " " + b + ")"
	case "|":
		return "(bit-or " + a + " " + b + ")"
	case "^":
		return "(bit-xor " + a + " " + b + ")"
	}
	f.no(nil, "an operator %s", op)
	return ""
}

func (f *cfn) unary(x *cc.UnaryExpression) cv {
	switch x.Case {
	case cc.UnaryExpressionPostfix:
		return f.expr(x.PostfixExpression)
	case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
		f.no(x, "an increment the lowering did not take apart")
	case cc.UnaryExpressionAddrof:
		return f.addr(x.CastExpression, x)
	case cc.UnaryExpressionDeref:
		return f.derefOf(f.expr(x.CastExpression), x)
	case cc.UnaryExpressionPlus, cc.UnaryExpressionMinus, cc.UnaryExpressionCpl:
		v := f.expr(x.CastExpression)
		k, ok := scalarKind(x.Type())
		if !ok {
			f.no(x, "floating point")
		}
		s := f.convK(v, k)
		switch x.Case {
		case cc.UnaryExpressionMinus:
			s = wrapK("(- "+s+")", k)
		case cc.UnaryExpressionCpl:
			if k.signed {
				s = "(bit-not " + s + ")"
			} else {
				s = wrapK("(bit-not "+s+")", k)
			}
		}
		return cv{s: s, t: k.java(), c: x.Type(), typed: true}
	case cc.UnaryExpressionNot:
		v := f.expr(x.CastExpression)
		return cv{s: f.falsity(v), t: "boolean", c: x.Type(), typed: true}
	case cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType, cc.UnaryExpressionAlignofExpr, cc.UnaryExpressionAlignofType:
		f.no(x, "a sizeof of no constant value")
	}
	f.no(x, "a unary expression %v", x.Case)
	return cv{}
}

// addr is &e.
func (f *cfn) addr(e cc.ExpressionNode, at cc.Node) cv {
	if f.lf != nil {
		if r, ok := f.lf.sub[e]; ok && r.v == nil {
			e = r.n
		}
	}
	e = unparenE(e)
	t := at.(cc.ExpressionNode).Type()
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			d, _ := x.ResolvedTo().(*cc.Declarator)
			if d != nil && f.lf != nil {
				if v, ok := f.lf.byDecl[d]; ok {
					cvr := f.vars[v]
					switch {
					case cvr.boxed:
						return cv{s: ptrOverC(cvr.jt, v.name, "0"), t: ptrOfArray(cvr.jt), c: t, typed: true}
					case isAggr(v.c):
						return cv{s: v.name, t: cvr.jt, c: t, typed: true}
					}
				}
			}
			if d != nil {
				if s := f.c.slotOf(d, f.lf); s != nil {
					switch {
					case s.boxed:
						return cv{s: ptrOverC(s.jt, "(g ed "+s.name+")", "0"), t: ptrOfArray(s.jt), c: t, typed: true}
					case isAggr(s.c):
						return f.slotRead(s)
					}
				}
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
				return cv{s: ptrOverC(elemJ(base.t), tagged(base), i), t: ptrOfArray(elemJ(base.t)), c: t, typed: true}
			case isJPtr(base.t):
				if ix.konst && ix.cv == 0 {
					return cv{s: base.s, t: base.t, c: t, typed: base.typed}
				}
				return cv{s: "(.add " + tagged(base) + " " + i + ")", t: base.t, c: t, typed: true}
			}
			f.no(x, "the address of an element of a %s", base.t)
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			if isAggr(e.Type()) {
				v := f.member(x)
				return cv{s: v.s, t: v.t, c: t, typed: v.typed}
			}
			box, ft := f.memberBox(x)
			return cv{s: ptrOverC(ft, box, "0"), t: ptrOfArray(ft), c: t, typed: true}
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			v := f.expr(x.CastExpression)
			v.c = t
			return v
		}
	}
	f.no(e, "the address of a %T", e)
	return cv{}
}

func (f *cfn) binary(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) cv {
	l, r := f.expr(le), f.expr(re)
	k, ok := scalarKind(x.Type())
	if !ok {
		f.no(x, "floating point")
	}
	return cv{s: f.arith(op, f.convK(l, k), r, k), t: k.java(), c: x.Type(), typed: true}
}

func (f *cfn) additive(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) cv {
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
		a, b := f.convV(l, pt, nil), f.convV(r, pt, nil)
		return cv{s: "(.sub " + tagged(a) + " " + tagged(b) + ")", t: "long", c: x.Type(), typed: true}
	case lp || rp:
		pe, ne := le, re
		if rp {
			pe, ne = re, le
		}
		p, n := f.expr(pe), f.expr(ne)
		if strings.HasSuffix(p.t, "[]") {
			p = cv{s: ptrOverC(elemJ(p.t), tagged(p), "0"), t: ptrOfArray(elemJ(p.t)), c: p.c, typed: true}
		}
		if !isJPtr(p.t) {
			f.no(x, "arithmetic on a %s", p.t)
		}
		i := f.index(n)
		if op == "-" {
			i = "(- " + i + ")"
		}
		return cv{s: "(.add " + tagged(p) + " " + i + ")", t: p.t, c: x.Type(), typed: true}
	}
	return f.binary(x, op, le, re)
}

// ptrEqC is a == b for two pointer values of Java type t.
func ptrEqC(t, a, b string) string {
	switch {
	case scalarPtrElem(t) != "":
		return "(" + t + "/eq " + a + " " + b + ")"
	case strings.HasPrefix(t, "Ptr<"):
		return "(Ptr/eq " + a + " " + b + ")"
	}
	return "(identical? " + a + " " + b + ")" // references: a struct's, an array's, a function's
}

func (f *cfn) compare(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) cv {
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
			return cv{s: s, t: "boolean", c: x.Type(), typed: true}
		}
		l, r := f.exprTo(le, ""), f.exprTo(re, "")
		if isFnDesignator(re) {
			r = f.exprTo(re, l.t)
		} else if isFnDesignator(le) {
			l = f.exprTo(le, r.t)
		}
		if op == "==" || op == "!=" {
			var pv, ov cv
			switch {
			case strings.HasPrefix(l.t, "Ptr<") && elemJ(l.t) == r.t:
				pv, ov = l, r
			case strings.HasPrefix(r.t, "Ptr<") && elemJ(r.t) == l.t:
				pv, ov = r, l
			}
			if pv.t != "" {
				s := "(Ptr/is " + pv.s + " " + ov.s + ")"
				if op == "!=" {
					s = "(not " + s + ")"
				}
				return cv{s: s, t: "boolean", c: x.Type(), typed: true}
			}
		}
		t := l.t
		if strings.HasSuffix(t, "[]") && !strings.HasSuffix(r.t, "[]") {
			t = r.t
		}
		if op == "==" || op == "!=" {
			a, b := f.convV(l, t, nil), f.convV(r, t, nil)
			s := ptrEqC(t, tagged(a), tagged(b))
			if scalarPtrElem(t) == "" && !strings.HasPrefix(t, "Ptr<") {
				s = ptrEqC(t, a.s, b.s)
			}
			if op == "!=" {
				s = "(not " + s + ")"
			}
			return cv{s: s, t: "boolean", c: x.Type(), typed: true}
		}
		if strings.HasSuffix(t, "[]") {
			t = ptrOfArray(elemJ(t))
		}
		if !isJPtr(t) {
			f.no(x, "an ordered comparison of %s", t)
		}
		a, b := f.convV(l, t, nil), f.convV(r, t, nil)
		m := map[string]string{"<": "lt", ">": "gt", "<=": "le", ">=": "ge"}[op]
		return cv{s: "(." + m + " " + tagged(a) + " " + tagged(b) + ")", t: "boolean", c: x.Type(), typed: true}
	}
	l, r := f.expr(le), f.expr(re)
	kl, okl := scalarKind(lt)
	kr, okr := scalarKind(rt)
	if !okl || !okr {
		f.no(x, "floating point")
	}
	k := usualK(kl, kr)
	a, b := f.convK(l, k), f.convK(r, k)
	if k.size == 8 && !k.signed && op != "==" && op != "!=" {
		return cv{s: "(" + op + " (Long/compareUnsigned " + a + " " + b + ") 0)", t: "boolean", c: x.Type(), typed: true}
	}
	switch op {
	case "==":
		return cv{s: "(== " + a + " " + b + ")", t: "boolean", c: x.Type(), typed: true}
	case "!=":
		return cv{s: "(not (== " + a + " " + b + "))", t: "boolean", c: x.Type(), typed: true}
	}
	return cv{s: "(" + op + " " + a + " " + b + ")", t: "boolean", c: x.Type(), typed: true}
}

// logical is && (and) or || (or): what the right side does happens only
// when C would evaluate it -- a step the printer needs there stays in it.
func (f *cfn) logical(x cc.ExpressionNode, op string, le, re cc.ExpressionNode) cv {
	l := f.expr(le)
	var r cv
	pre := f.capture(func() { r = f.expr(re) })
	return cv{s: "(" + op + " " + f.truth(l) + " " + wrapPre(pre, f.truth(r)) + ")", t: "boolean", c: x.Type(), typed: true}
}

// wrapPre is value s after the steps pre, as one expression.
func wrapPre(pre []cbind, s string) string {
	if len(pre) == 0 {
		return s
	}
	return "(let [" + bindText(pre, " ") + "] " + s + ")"
}

// ternary is ?: -- only the chosen arm is evaluated.
func (f *cfn) ternary(x *cc.ConditionalExpression, to string) cv {
	c := f.expr(x.LogicalOrExpression)
	var a, b cv
	pa := f.capture(func() { a = f.exprTo(x.ExpressionList, to) })
	pb := f.capture(func() { b = f.exprTo(x.ConditionalExpression, to) })
	ct := x.Type()
	var t string
	switch {
	case ct.Kind() == cc.Void:
		f.no(x, "a ?: of no value")
	case isPtrish(ct) || isAggr(ct):
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
	conv := func(v cv) string {
		if ct == nil {
			return v.s
		}
		return f.conv(v, t, ct)
	}
	cs, swap := f.cond(c)
	as, bs := wrapPre(pa, conv(a)), wrapPre(pb, conv(b))
	if swap {
		as, bs = bs, as
	}
	_, scalar := scalarKind(ct)
	return cv{s: "(if " + cs + " " + as + " " + bs + ")", t: t, c: ct, typed: scalar || t == "boolean"}
}

func (f *cfn) cast(x *cc.CastExpression, to string) cv {
	tt := x.Type()
	inner := x.CastExpression
	it := inner.Type()
	if tt.Kind() == cc.Void {
		v := f.expr(inner)
		if v.s != "" && v.t != "void" {
			f.effectOf(v.s)
		}
		return cv{t: "void"}
	}
	if k, ok := scalarKind(tt); ok {
		if _, ok := scalarKind(it); !ok {
			f.no(x, "a cast between a pointer and an integer")
		}
		v := f.expr(inner)
		return cv{s: f.convK(v, k), t: k.java(), c: tt, typed: true}
	}
	if tt.Kind() != cc.Ptr {
		f.no(x, "a cast to %s", tt)
	}
	si := stripCasts(inner)
	if c, ok := si.(*cc.PostfixExpression); ok && c.Case == cc.PostfixExpressionCall && f.c.g.p.allocators[calleeName(c)] && !f.subbed(si) {
		return f.alloc(c, tt.(*cc.PointerType).Elem(), to)
	}
	if g := f.gaMember(inner); g != nil && tt.(*cc.PointerType).Elem().Kind() != cc.Void {
		want := f.jt(tt, "", "a cast")
		if isByteType(tt.(*cc.PointerType).Elem()) || tt.(*cc.PointerType).Elem().Kind() == cc.SChar {
			want = "BytePtr"
		} else if isJPtr(to) && elemJ(to) != "" && strings.HasPrefix(to, "Ptr<") == strings.HasPrefix(want, "Ptr<") {
			want = to
		}
		if !isJPtr(want) {
			want = ptrOfArray(want)
		}
		v := f.gadata(g, want, tt)
		if to != "" && to != want && !isJPtr(to) && elemJ(want) == to {
			return cv{s: "(Ptr/ref " + v.s + ")", t: to, c: tt}
		}
		return v
	}
	if _, ok := scalarKind(it); ok {
		f.no(x, "a cast between a pointer and an integer")
	}
	v := f.exprTo(inner, to)
	if v.null {
		return cv{s: "nil", t: to, c: tt, null: true}
	}
	want := to
	if want == "" {
		want = f.jt(tt, "", "a cast")
		if want != v.t && elemJ(want) == elemJ(v.t) {
			want = v.t
		}
	}
	if v.t == want {
		v.c = tt
		return v
	}
	if !sameTarget(tt, it) && v.t != "Object" && want != "Object" {
		f.no(x, "a pointer cast: %s to %s", it, tt)
	}
	return f.convV(v, want, nil)
}

// subbed says a substitution replaces e itself.
func (f *cfn) subbed(e cc.ExpressionNode) bool {
	if f.lf == nil {
		return false
	}
	_, ok := f.lf.sub[e]
	return ok
}

// alloc is an allocation (Profile.Allocators): a new struct for one, n of
// them, or n bytes -- storage the garbage collector owns.
func (f *cfn) alloc(c *cc.PostfixExpression, elem cc.Type, to string) cv {
	var args []cc.ExpressionNode
	for l := c.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	for _, a := range args[1:] {
		if impure(a) {
			f.effectOf(f.expr(a).s)
		}
	}
	if k, ok := scalarKind(elem); ok {
		cls := ptrClass(k)
		n := f.sizeCount(args[0], elem)
		return cv{s: "(" + cls + "/alloc " + n + ")", t: cls, c: c.Type(), typed: true}
	}
	switch elem.Kind() {
	case cc.Struct, cc.Union:
		sn := f.jt(elem, "")
		n := f.sizeCount(args[0], elem)
		if n == "1" && (to == "" || to == sn) {
			return cv{s: "(new-" + sn + ")", t: sn, c: c.Type(), typed: true}
		}
		return cv{s: "(Ptr. (array-" + sn + " " + n + ") 0)", t: "Ptr<" + sn + ">", c: c.Type(), typed: true}
	case cc.Ptr:
		et := elemJ(to)
		if !strings.HasPrefix(to, "Ptr<") {
			et = f.jt(elem, "")
		}
		n := f.sizeCount(args[0], elem)
		return cv{s: "(Ptr. (object-array " + n + ") 0)", t: "Ptr<" + et + ">", c: c.Type(), typed: true}
	}
	f.no(c, "an allocation of a %s", elem)
	return cv{}
}

// sizeCount is a size in bytes as a count of elements of C type elem.
func (f *cfn) sizeCount(n cc.ExpressionNode, elem cc.Type) string {
	if isByteType(elem) || elem.Kind() == cc.SChar {
		return f.index(f.expr(n))
	}
	es := elem.Size()
	if v, ok := intValue(constValue(n)); ok && !f.hasSub(n) {
		if es > 0 && v%es == 0 {
			return fmt.Sprint(v / es)
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
func (f *cfn) call(x *cc.PostfixExpression, to string) cv {
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	name := calleeName(x)
	p := f.c.g.p
	switch {
	case name == "":
		return f.callPtr(x, args)
	case name == "__builtin_expect":
		return f.exprTo(args[0], to)
	case p.frees[name]:
		// the garbage collector frees; what the argument does still happens
		for _, a := range args {
			if impure(a) {
				f.effectOf(f.expr(a).s)
			}
		}
		return cv{t: "void"}
	case p.byteMove(name), name == p.Bytes.Set, name == p.Bytes.Cmp:
		return f.bytesCall(x, name, args)
	case p.allocators[name]:
		if to == "BytePtr" || to == "Object" {
			return cv{s: "(BytePtr/alloc " + f.index(f.expr(args[0])) + ")", t: "BytePtr", c: x.Type(), typed: true}
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
	if why := f.c.j.sigWhy(name, ft); why != "" {
		f.no(x, "%s: in the signature of %s", why, name)
	}
	ps := ft.Parameters()
	if len(ps) == 1 && ps[0].Type().Kind() == cc.Void {
		ps = nil
	}
	if len(args) != len(ps) && !(ft.IsVariadic() && len(args) > len(ps)) {
		f.no(x, "a call with %d arguments to a function of %d parameters", len(args), len(ps))
	}
	var as, vs []string
	for i, a := range args {
		if i >= len(ps) {
			vs = append(vs, f.vararg(a))
			continue
		}
		pt := ps[i].Type()
		if pt.Kind() == cc.Array {
			pt = pt.Decay()
		}
		jt := f.jt(pt, fmt.Sprintf("param:%s:%d", name, i))
		v := f.exprTo(a, jt)
		s := f.conv(v, jt, pt)
		if isAggr(pt) {
			s = "(.copy " + tagged(cv{s: s, t: jt}) + ")" // a struct is passed by value
		}
		as = append(as, s)
	}
	if ft.IsVariadic() {
		as = append(as, "(object-array ["+strings.Join(vs, " ")+"])")
	}
	fn := f.c.fnName(name)
	if f.c.hostFns[name] {
		fn = f.c.hostNS + "/" + fn
	}
	call := "(" + fn + " ed" + strings.Join(append([]string{""}, as...), " ") + ")"
	return f.result(call, ft.Result(), "ret:"+name)
}

// result is a call's value, of C type rt: an integer made a long, a
// boolean as it comes, anything else of its class, untyped.
func (f *cfn) result(call string, rt cc.Type, key string) cv {
	if rt == nil || rt.Kind() == cc.Void {
		return cv{s: call, t: "void"}
	}
	t := f.jt(rt, key)
	if k, ok := scalarKind(rt); ok {
		if k.boolean {
			return cv{s: call, t: "boolean", c: rt}
		}
		return cv{s: "(long " + call + ")", t: t, c: rt, typed: true}
	}
	return cv{s: call, t: t, c: rt}
}

// callPtr is a call through a function pointer: a Clojure function of the
// editor and the arguments.
func (f *cfn) callPtr(x *cc.PostfixExpression, args []cc.ExpressionNode) cv {
	callee := x.PostfixExpression
	if u, ok := unparenE(callee).(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionDeref && !f.subbed(callee) {
		callee = u.CastExpression
	}
	var ft *cc.FunctionType
	if p, ok := callee.Type().(*cc.PointerType); ok {
		ft, _ = p.Elem().(*cc.FunctionType)
	}
	if ft == nil {
		f.no(x, "a call of no function type")
	}
	fv := f.expr(callee)
	w := f.c.j.ifaceByName[fv.t]
	if w == nil {
		f.no(x, "a function pointer: a call through a %s", fv.t)
	}
	var ps []cc.Type
	for _, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		t := p.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		ps = append(ps, t)
	}
	if len(args) != len(ps) || len(ps) != len(w.params) {
		f.no(x, "a call with %d arguments through a pointer to a function of %d parameters", len(args), len(ps))
	}
	var as []string
	for i, a := range args {
		v := f.exprTo(a, w.params[i])
		s := f.conv(v, w.params[i], ps[i])
		if isAggr(ps[i]) {
			s = "(.copy " + tagged(cv{s: s, t: w.params[i]}) + ")"
		}
		as = append(as, s)
	}
	call := "(" + fv.s + " ed" + strings.Join(append([]string{""}, as...), " ") + ")"
	return f.result(call, ft.Result(), "")
}

// vararg is an argument past a variadic function's parameters, as the Object
// array receives it: C's default promotions, then boxed as the Java editor
// boxes them -- an integer of int's size or less an Integer (an unsigned
// int zero-extended to a Long), a long a Long, a pointer as itself, an
// array as the pointer it decays to, NULL a nil.
func (f *cfn) vararg(a cc.ExpressionNode) string {
	v := f.expr(a)
	switch {
	case v.null:
		return "nil"
	case v.t == "boolean":
		return "(Integer/valueOf (int (if " + v.s + " 1 0)))"
	case strings.HasSuffix(v.t, "[]"):
		return ptrOverC(elemJ(v.t), tagged(v), "0")
	}
	k, ok := scalarKind(a.Type())
	if v.kind != nil {
		k, ok = *v.kind, true
	}
	if !ok {
		if isPtrish(a.Type()) {
			return v.s
		}
		f.no(a, "a variadic argument of type %s", a.Type())
	}
	p := promote(k)
	if p.size == 4 && !p.signed {
		p = jk{size: 8, signed: true}
	}
	s := f.convK(v, p)
	if p.size == 4 {
		return "(Integer/valueOf (unchecked-int " + s + "))"
	}
	return "(Long/valueOf " + s + ")"
}

// memArg is an argument of a function of bytes seen through its casts, and
// the C type of its elements (nil: the growarray's storage, bytes).
func (f *cfn) memArg(a cc.ExpressionNode) (cv, cc.Type) {
	if g := f.gaMember(a); g != nil && !f.subbed(a) {
		et := elemCType(unparenE(a))
		if et == nil || et.Kind() == cc.Void || isByteType(et) || et.Kind() == cc.SChar {
			return f.gadata(g, "BytePtr", a.Type()), cc.Type(nil)
		}
		want := f.jt(a.Type(), "", "a function of bytes' argument")
		if !isJPtr(want) {
			want = ptrOfArray(want)
		}
		return f.gadata(g, want, a.Type()), et
	}
	s := a
	if !f.subbed(a) {
		s = stripCasts(a)
	}
	if u, ok := unparenE(s).(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof && !f.subbed(s) {
		if in := unparenE(u.CastExpression); in.Type() != nil && in.Type().Kind() == cc.Array {
			return f.expr(in), in.Type().(*cc.ArrayType).Elem()
		}
	}
	return f.expr(s), elemCType(s)
}

// memView is a memArg as a pointer over its elements, or a struct object.
func memViewC(v cv) cv {
	if strings.HasSuffix(v.t, "[]") {
		return cv{s: ptrOverC(elemJ(v.t), tagged(v), "0"), t: ptrOfArray(elemJ(v.t)), c: v.c, typed: true}
	}
	return v
}

// bytesCall is memmove, memcpy, memset or memcmp (Profile.Bytes), in the
// elements the C's pointers point at before their cast to bytes.
func (f *cfn) bytesCall(x *cc.PostfixExpression, name string, args []cc.ExpressionNode) cv {
	if len(args) != 3 {
		f.no(x, "memmove, memset or memcmp of no bytes: %s with %d arguments", name, len(args))
	}
	p := f.c.g.p
	isBytes := func(t cc.Type) bool { return t == nil || isByteType(t) || t.Kind() == cc.SChar }
	d, de := f.memArg(args[0])
	switch {
	case p.byteMove(name):
		s, se := f.memArg(args[1])
		if isBytes(de) && isBytes(se) {
			n := f.index(f.expr(args[2]))
			return cv{s: "(Rt/memmove " + tagged(f.convV(d, "BytePtr", nil)) + " " + tagged(f.convV(s, "BytePtr", nil)) + " " + n + ")", t: "BytePtr", c: args[0].Type(), typed: true}
		}
		if de == nil || se == nil || !sameType(de, se) {
			f.no(x, "memmove, memset or memcmp of no bytes: from a %s to a %s", se, de)
		}
		return f.memmoveElems(x, memViewC(d), memViewC(s), de, args[2])
	case name == p.Bytes.Set:
		c := f.expr(args[1])
		if isBytes(de) {
			n := f.index(f.expr(args[2]))
			return cv{s: "(Rt/memset " + tagged(f.convV(d, "BytePtr", nil)) + " (unchecked-int " + f.convK(c, jInt) + ") " + n + ")", t: "BytePtr", c: args[0].Type(), typed: true}
		}
		return f.fill(x, memViewC(d), c, de, args[2])
	}
	b, be := f.memArg(args[1])
	if isBytes(de) && isBytes(be) {
		n := f.index(f.expr(args[2]))
		return cv{s: "(long (Rt/memcmp " + tagged(f.convV(d, "BytePtr", nil)) + " " + tagged(f.convV(b, "BytePtr", nil)) + " " + n + "))", t: "int", c: x.Type(), typed: true}
	}
	if de == nil || !isAggr(de) || be == nil || !sameType(de, be) || f.sizeCount(args[2], de) != "1" {
		f.no(x, "memmove, memset or memcmp of no bytes: a memcmp of %s and %s", de, be)
	}
	cls, why := f.c.j.structName(de)
	if why != "" {
		f.no(x, "%s", why)
	}
	f.c.j.needEq[cls] = true
	return cv{s: "(if (.eq " + f.memObj(d, cls) + " " + f.memObj(b, cls) + ") 0 1)", t: "int", c: x.Type(), typed: true}
}

// memObj is the one struct object a memArg points at, hinted cls.
func (f *cfn) memObj(v cv, cls string) string {
	switch {
	case strings.HasPrefix(v.t, "Ptr<"):
		return "^" + cls + " (Ptr/ref " + v.s + ")"
	case strings.HasSuffix(v.t, "[]"):
		return "^" + cls + " (aget " + tagged(v) + " 0)"
	}
	return tagged(v)
}

// memmoveElems is memmove of n bytes as elements of C type e.
func (f *cfn) memmoveElems(x *cc.PostfixExpression, d, s cv, e cc.Type, nb cc.ExpressionNode) cv {
	n := f.sizeCount(nb, e)
	switch {
	case isAggr(e) && !isJPtr(d.t) && !isJPtr(s.t):
		if n != "1" {
			f.no(x, "memmove, memset or memcmp of no bytes: %s structs through a plain pointer", n)
		}
		return cv{s: "(.set " + tagged(d) + " " + s.s + ")", t: d.t, c: x.Type()}
	case isAggr(e):
		d, s = f.memPtr(x, d), f.memPtr(x, s)
		return cv{s: "(Rt/moveStructs " + tagged(d) + " " + tagged(s) + " " + n + ")", t: d.t, c: x.Type(), typed: true}
	}
	if !isJPtr(d.t) || d.t != s.t {
		f.no(x, "memmove, memset or memcmp of no bytes: a memmove from %s to %s", s.t, d.t)
	}
	return cv{s: "(Rt/memmove " + tagged(d) + " " + tagged(s) + " " + n + ")", t: d.t, c: x.Type(), typed: true}
}

// memPtr is a struct memArg as a Ptr over its class.
func (f *cfn) memPtr(x *cc.PostfixExpression, v cv) cv {
	if strings.HasPrefix(v.t, "Ptr<") {
		return v
	}
	if isJPtr(v.t) || v.t == "" {
		f.no(x, "memmove, memset or memcmp of no bytes: structs through a %s", v.t)
	}
	return cv{s: "(Ptr/one " + v.s + ")", t: "Ptr<" + v.t + ">", c: v.c, typed: true}
}

// fill is memset of n bytes of c through elements of C type e.
func (f *cfn) fill(x *cc.PostfixExpression, d, c cv, e cc.Type, nb cc.ExpressionNode) cv {
	n := f.sizeCount(nb, e)
	zero := c.konst && c.cv == 0
	switch {
	case isAggr(e) && zero && !isJPtr(d.t):
		if n != "1" {
			f.no(x, "memmove, memset or memcmp of no bytes: %s structs through a plain pointer", n)
		}
		return cv{s: "(.zero " + tagged(d) + ")", t: d.t, c: x.Type()}
	case isAggr(e) && zero:
		return cv{s: "(Rt/zeroStructs " + tagged(f.memPtr(x, d)) + " " + n + ")", t: d.t, c: x.Type(), typed: true}
	case isAggr(e):
		// every member's every byte: a struct of scalars only
		p := f.memPtr(x, d)
		cls := elemJ(p.t)
		pt, k := f.tmpName("p"), f.tmpName("k")
		var sets []string
		for i, fl := range members(e) {
			if fl == nil {
				continue
			}
			fk, ok := scalarKind(fl.Type())
			if !ok || fl.IsBitfield() {
				f.no(x, "memmove, memset or memcmp of no bytes: a fill of a struct with a %s", fl.Type())
			}
			_ = i
			name := f.c.memberName(fl)
			obj := "^" + cls + " (.at " + pt + " " + k + ")"
			v := f.repeated(c, fk)
			if f.c.j.boxedField[fieldKey(fl)] {
				sets = append(sets, "(aset (.-"+name+" "+obj+") 0 "+storeK(v, fk)+")")
			} else {
				if fk.boolean {
					v = "(boolean " + v + ")"
				}
				sets = append(sets, "(.set_"+name+" "+obj+" "+v+")")
			}
		}
		return cv{s: "(let [" + pt + " " + tagged(p) + "] (dotimes [" + k + " " + n + "] " + strings.Join(sets, " ") + ") " + pt + ")", t: p.t, c: x.Type(), typed: true}
	case e.Kind() == cc.Ptr:
		if !zero {
			f.no(x, "memmove, memset or memcmp of no bytes: a fill of pointers with a byte that is not 0")
		}
		if !isJPtr(d.t) {
			f.no(x, "memmove, memset or memcmp of no bytes: a fill through a %s", d.t)
		}
		return cv{s: "(Rt/zero " + tagged(d) + " " + n + ")", t: d.t, c: x.Type(), typed: true}
	}
	k, ok := scalarKind(e)
	if !ok || !isJPtr(d.t) {
		f.no(x, "memmove, memset or memcmp of no bytes: a fill of %s through a %s", e, d.t)
	}
	return cv{s: "(Rt/fill " + tagged(d) + " " + storeK(f.repeated(c, k), k) + " " + n + ")", t: d.t, c: x.Type(), typed: true}
}

// repeated is the value of kind k whose every byte is c's low byte.
func (f *cfn) repeated(c cv, k jk) string {
	if c.konst {
		b := uint64(c.cv) & 0xff
		v := uint64(0)
		for i := 0; i < k.size; i++ {
			v = v<<8 | b
		}
		return clit(int64(v), k)
	}
	b := "(bit-and " + f.convK(c, jInt) + " 0xff)"
	switch {
	case k.boolean:
		return "(not (zero? " + b + "))"
	case k.size == 1:
		return wrapK(b, k)
	case k.size == 2:
		return wrapK("(* "+b+" 0x0101)", k)
	case k.size == 4:
		return wrapK("(* "+b+" 0x01010101)", k)
	}
	return "(* " + b + " 0x0101010101010101)"
}
