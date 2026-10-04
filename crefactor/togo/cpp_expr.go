package togo

// cpp_expr.go prints the C's expressions as C++: each as the C wrote it,
// its own parentheses and no others, with what C++ spells otherwise -- a
// function a pointer to the editor's member and a call through one
// `(this->*p)(...)`, C23's nullptr, true and false as written, a hoisted
// static by its field's name -- and every conversion C makes implicitly and
// C++ does not written as a cast where C made it.

import (
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// pass descends through the grammar's pass-through productions to the node
// that is the expression.
func pass(n cc.ExpressionNode) cc.ExpressionNode {
	for {
		switch x := n.(type) {
		case *cc.ConstantExpression:
			n = x.ConditionalExpression
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				return n
			}
			n = x.ConditionalExpression
		case *cc.ConditionalExpression:
			if x.Case != cc.ConditionalExpressionLOr {
				return n
			}
			n = x.LogicalOrExpression
		case *cc.LogicalOrExpression:
			if x.Case != cc.LogicalOrExpressionLAnd {
				return n
			}
			n = x.LogicalAndExpression
		case *cc.LogicalAndExpression:
			if x.Case != cc.LogicalAndExpressionOr {
				return n
			}
			n = x.InclusiveOrExpression
		case *cc.InclusiveOrExpression:
			if x.Case != cc.InclusiveOrExpressionXor {
				return n
			}
			n = x.ExclusiveOrExpression
		case *cc.ExclusiveOrExpression:
			if x.Case != cc.ExclusiveOrExpressionAnd {
				return n
			}
			n = x.AndExpression
		case *cc.AndExpression:
			if x.Case != cc.AndExpressionEq {
				return n
			}
			n = x.EqualityExpression
		case *cc.EqualityExpression:
			if x.Case != cc.EqualityExpressionRel {
				return n
			}
			n = x.RelationalExpression
		case *cc.RelationalExpression:
			if x.Case != cc.RelationalExpressionShift {
				return n
			}
			n = x.ShiftExpression
		case *cc.ShiftExpression:
			if x.Case != cc.ShiftExpressionAdd {
				return n
			}
			n = x.AdditiveExpression
		case *cc.AdditiveExpression:
			if x.Case != cc.AdditiveExpressionMul {
				return n
			}
			n = x.MultiplicativeExpression
		case *cc.MultiplicativeExpression:
			if x.Case != cc.MultiplicativeExpressionCast {
				return n
			}
			n = x.CastExpression
		case *cc.CastExpression:
			if x.Case != cc.CastExpressionUnary {
				return n
			}
			n = x.UnaryExpression
		case *cc.UnaryExpression:
			if x.Case != cc.UnaryExpressionPostfix {
				return n
			}
			n = x.PostfixExpression
		case *cc.PostfixExpression:
			if x.Case != cc.PostfixExpressionPrimary {
				return n
			}
			n = x.PrimaryExpression
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				return n
			}
			n = x.AssignmentExpression
		default:
			return n
		}
	}
}

// strip is pass, and through the expression's own parentheses.
func strip(n cc.ExpressionNode) cc.ExpressionNode {
	for {
		n = pass(n)
		if p, ok := n.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionExpr {
			n = p.ExpressionList
			continue
		}
		return n
	}
}

// fnDesig is the function an expression designates, or nil.
func fnDesig(n cc.ExpressionNode) *cc.Declarator {
	if p, ok := strip(n).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
			return d
		}
	}
	return nil
}

// macro is the C23 keyword the front end expanded at n -- nullptr, true or
// false, which its predefined macros spell ((void*)0), 1 and 0 -- or "".
func (c *cppgen) macro(n cc.Node) string {
	p := n.Position()
	if p.Line <= 0 || p.Line >= len(c.lineAt) {
		return ""
	}
	off := c.lineAt[p.Line] + p.Column - 1
	if off < 0 || off >= len(c.src) {
		return ""
	}
	for _, w := range []string{"nullptr", "true", "false"} {
		if strings.HasPrefix(string(c.src[off:min(off+len(w)+1, len(c.src))]), w) {
			rest := c.src[off+len(w):]
			if len(rest) == 0 || !isIdentByte(rest[0]) {
				return w
			}
		}
	}
	return ""
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// isNull reports whether n is a null pointer constant C++ takes as one: 0
// or nullptr.
func (c *cppgen) isNull(n cc.ExpressionNode) bool {
	s := strip(n)
	if c.macro(s) == "nullptr" {
		return true
	}
	if p, ok := s.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionInt {
		v, ok := intValue(p.Value())
		return ok && v == 0
	}
	return false
}

// isStrLit reports whether n is a string literal.
func isStrLit(n cc.ExpressionNode) bool {
	p, ok := strip(n).(*cc.PrimaryExpression)
	return ok && p.Case == cc.PrimaryExpressionString
}

// expr prints an expression.
func (c *cppgen) expr(n cc.ExpressionNode) string {
	if n == nil {
		return ""
	}
	switch m := c.macro(n); m {
	case "nullptr", "true", "false":
		// the whole node is the keyword when it is the expansion
		if c.isExpansion(n, m) {
			return m
		}
	}
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			return c.ident(x)
		case cc.PrimaryExpressionInt, cc.PrimaryExpressionFloat, cc.PrimaryExpressionChar,
			cc.PrimaryExpressionLChar, cc.PrimaryExpressionString, cc.PrimaryExpressionLString:
			return x.Token.SrcStr()
		case cc.PrimaryExpressionExpr:
			return "(" + c.expr(x.ExpressionList) + ")"
		}
		c.fail(x, "primary expression %v", x.Case)
		return ""

	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionPrimary:
			return c.expr(x.PrimaryExpression)
		case cc.PostfixExpressionIndex:
			return c.expr(x.PostfixExpression) + "[" + c.expr(x.ExpressionList) + "]"
		case cc.PostfixExpressionCall:
			return c.call(x)
		case cc.PostfixExpressionSelect:
			return c.expr(x.PostfixExpression) + "." + cppName(x.Token2.SrcStr())
		case cc.PostfixExpressionPSelect:
			return c.expr(x.PostfixExpression) + "->" + cppName(x.Token2.SrcStr())
		case cc.PostfixExpressionInc:
			return c.expr(x.PostfixExpression) + "++"
		case cc.PostfixExpressionDec:
			return c.expr(x.PostfixExpression) + "--"
		case cc.PostfixExpressionComplit:
			return c.complit(x)
		}
		c.fail(x, "postfix expression %v", x.Case)
		return ""

	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionPostfix:
			return c.expr(x.PostfixExpression)
		case cc.UnaryExpressionInc:
			return "++" + c.expr(x.UnaryExpression)
		case cc.UnaryExpressionDec:
			return "--" + c.expr(x.UnaryExpression)
		case cc.UnaryExpressionAddrof:
			if fnDesig(x.CastExpression) != nil {
				// a function's address: the member's
				return c.expr(x.CastExpression)
			}
			return prefixOp("&", c.expr(x.CastExpression))
		case cc.UnaryExpressionDeref:
			if fnDesig(x.CastExpression) != nil || cppFnPtr(x.CastExpression.Type()) {
				// *f and *p of a function are the function: C++ has no
				// such deref of a pointer to member
				return c.expr(x.CastExpression)
			}
			return "*" + c.expr(x.CastExpression)
		case cc.UnaryExpressionPlus:
			return prefixOp("+", c.expr(x.CastExpression))
		case cc.UnaryExpressionMinus:
			return prefixOp("-", c.expr(x.CastExpression))
		case cc.UnaryExpressionCpl:
			return "~" + c.expr(x.CastExpression)
		case cc.UnaryExpressionNot:
			return "!" + c.expr(x.CastExpression)
		case cc.UnaryExpressionSizeofExpr:
			if in := c.expr(x.UnaryExpression); strings.HasPrefix(in, "(") {
				return "sizeof" + in
			} else {
				return "sizeof " + in
			}
		case cc.UnaryExpressionSizeofType:
			return "sizeof(" + c.typeName(x.TypeName) + ")"
		case cc.UnaryExpressionAlignofExpr:
			return "alignof(decltype(" + c.expr(x.UnaryExpression) + "))"
		case cc.UnaryExpressionAlignofType:
			return "alignof(" + c.typeName(x.TypeName) + ")"
		}
		c.fail(x, "unary expression %v", x.Case)
		return ""

	case *cc.CastExpression:
		switch x.Case {
		case cc.CastExpressionUnary:
			return c.expr(x.UnaryExpression)
		case cc.CastExpressionCast:
			return "(" + c.typeName(x.TypeName) + ")" + c.expr(x.CastExpression)
		}
		c.fail(x, "cast expression %v", x.Case)
		return ""

	case *cc.MultiplicativeExpression:
		if x.Case == cc.MultiplicativeExpressionCast {
			return c.expr(x.CastExpression)
		}
		return c.bin(x.MultiplicativeExpression, x.Token.SrcStr(), x.CastExpression)
	case *cc.AdditiveExpression:
		if x.Case == cc.AdditiveExpressionMul {
			return c.expr(x.MultiplicativeExpression)
		}
		return c.bin(x.AdditiveExpression, x.Token.SrcStr(), x.MultiplicativeExpression)
	case *cc.ShiftExpression:
		if x.Case == cc.ShiftExpressionAdd {
			return c.expr(x.AdditiveExpression)
		}
		return c.bin(x.ShiftExpression, x.Token.SrcStr(), x.AdditiveExpression)
	case *cc.RelationalExpression:
		if x.Case == cc.RelationalExpressionShift {
			return c.expr(x.ShiftExpression)
		}
		return c.cmp(x.RelationalExpression, x.Token.SrcStr(), x.ShiftExpression)
	case *cc.EqualityExpression:
		if x.Case == cc.EqualityExpressionRel {
			return c.expr(x.RelationalExpression)
		}
		return c.cmp(x.EqualityExpression, x.Token.SrcStr(), x.RelationalExpression)
	case *cc.AndExpression:
		if x.Case == cc.AndExpressionEq {
			return c.expr(x.EqualityExpression)
		}
		return c.bin(x.AndExpression, "&", x.EqualityExpression)
	case *cc.ExclusiveOrExpression:
		if x.Case == cc.ExclusiveOrExpressionAnd {
			return c.expr(x.AndExpression)
		}
		return c.bin(x.ExclusiveOrExpression, "^", x.AndExpression)
	case *cc.InclusiveOrExpression:
		if x.Case == cc.InclusiveOrExpressionXor {
			return c.expr(x.ExclusiveOrExpression)
		}
		return c.bin(x.InclusiveOrExpression, "|", x.ExclusiveOrExpression)
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionOr {
			return c.expr(x.InclusiveOrExpression)
		}
		return c.bin(x.LogicalAndExpression, "&&", x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLAnd {
			return c.expr(x.LogicalAndExpression)
		}
		return c.bin(x.LogicalOrExpression, "||", x.LogicalAndExpression)

	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionLOr {
			return c.expr(x.LogicalOrExpression)
		}
		t := x.Type()
		if t != nil && t.Kind() == cc.Ptr {
			return c.expr(x.LogicalOrExpression) + " ? " + c.conv(x.ExpressionList, t) + " : " + c.conv(x.ConditionalExpression, t)
		}
		return c.expr(x.LogicalOrExpression) + " ? " + c.expr(x.ExpressionList) + " : " + c.expr(x.ConditionalExpression)

	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return c.expr(x.ConditionalExpression)
		}
		op := x.Token.SrcStr()
		if op == "=" {
			return c.expr(x.UnaryExpression) + " = " + c.conv(x.AssignmentExpression, x.UnaryExpression.Type())
		}
		return c.bin(x.UnaryExpression, op, x.AssignmentExpression)

	case *cc.ExpressionList:
		var parts []string
		for l := x; l != nil; l = l.ExpressionList {
			parts = append(parts, c.expr(l.AssignmentExpression))
		}
		return strings.Join(parts, ", ")

	case *cc.ConstantExpression:
		return c.expr(x.ConditionalExpression)
	}
	c.fail(n, "expression node %T", n)
	return ""
}

// isExpansion reports whether n is all of the keyword's expansion: the
// paren of ((void*)0), or the constant of 1 and 0.
func (c *cppgen) isExpansion(n cc.ExpressionNode, m string) bool {
	switch m {
	case "nullptr":
		p, ok := n.(*cc.PrimaryExpression)
		return ok && p.Case == cc.PrimaryExpressionExpr
	default:
		p, ok := n.(*cc.PrimaryExpression)
		return ok && p.Case == cc.PrimaryExpressionInt
	}
}

func prefixOp(op, operand string) string {
	if operand != "" && op[len(op)-1] == operand[0] && strings.ContainsRune("+-&", rune(operand[0])) {
		return op + " " + operand
	}
	return op + operand
}

func (c *cppgen) bin(l cc.ExpressionNode, op string, r cc.ExpressionNode) string {
	return c.expr(l) + " " + op + " " + c.expr(r)
}

// cmp is a comparison: pointers of types C compares and C++ does not are
// converted to the left's.
func (c *cppgen) cmp(l cc.ExpressionNode, op string, r cc.ExpressionNode) string {
	lt, rt := l.Type(), r.Type()
	if lt != nil && rt != nil && lt.Kind() == cc.Ptr && rt.Kind() == cc.Ptr && !c.isNull(r) && !c.isNull(l) {
		le, re := elemOf(lt), elemOf(rt)
		if le.Kind() != cc.Void && re.Kind() != cc.Void && c.canon(le, false) != c.canon(re, false) {
			c.nCasts++
			return c.expr(l) + " " + op + " " + "(" + c.typeStr(lt.Decay()) + ")" + c.paren(r)
		}
	}
	return c.bin(l, op, r)
}

// isFnPtr reports whether t is a pointer to a function.
func cppFnPtr(t cc.Type) bool {
	if t == nil || t.Kind() != cc.Ptr {
		return false
	}
	e := elemOf(t)
	return e != nil && e.Kind() == cc.Function
}

// ident prints a name: a hoisted static's field, a function as a value the
// member's address.
func (c *cppgen) ident(x *cc.PrimaryExpression) string {
	name := x.Token.SrcStr()
	switch d := x.ResolvedTo().(type) {
	case *cc.Declarator:
		if f, ok := c.field[d]; ok {
			return f
		}
		if h, ok := c.hoist[d]; ok {
			return h
		}
		if d.Type() != nil && d.Type().Kind() == cc.Function && c.isMember(name) {
			return "&Editor::" + cppName(name)
		}
	}
	return cppName(name)
}

// isMember reports whether name is one of the editor's functions.
func (c *cppgen) isMember(name string) bool {
	return c.defined[name] != nil || c.hostFns[name] != nil
}

// call prints a call: a function's by its name, a pointer's through the
// editor, each argument converted to its parameter's type.
func (c *cppgen) call(x *cc.PostfixExpression) string {
	callee := x.PostfixExpression
	var ft *cc.FunctionType
	var head string
	if d := fnDesig(callee); d != nil {
		ft, _ = d.Type().(*cc.FunctionType)
		head = cppName(d.Name())
		if !c.isMember(d.Name()) {
			head = d.Name()
		}
	} else {
		t := callee.Type()
		if t != nil && t.Kind() == cc.Ptr {
			ft, _ = elemOf(t).(*cc.FunctionType)
		} else {
			ft, _ = t.(*cc.FunctionType)
		}
		// (*p)(...) and p(...) alike: the pointer's
		p := strip(callee)
		for {
			u, ok := p.(*cc.UnaryExpression)
			if !ok || u.Case != cc.UnaryExpressionDeref {
				break
			}
			p = strip(u.CastExpression)
		}
		head = "(this->*" + c.expr(p) + ")"
	}
	var args []string
	i := 0
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		var pt cc.Type
		if ft != nil && i < len(ft.Parameters()) {
			pt = ft.Parameters()[i].Type()
		}
		if pt != nil && pt.Kind() != cc.Void {
			args = append(args, c.conv(l.AssignmentExpression, pt.Decay()))
		} else {
			args = append(args, c.expr(l.AssignmentExpression))
		}
		i++
	}
	return head + "(" + strings.Join(args, ", ") + ")"
}

// complit is a compound literal.  At file scope, C's is an object of static
// storage: the editor's field, named.
func (c *cppgen) complit(x *cc.PostfixExpression) string {
	t := x.TypeName.Type()
	if !c.inClass && t.Kind() == cc.Array {
		// C's lives to the end of its block, C++'s temporary to the end of
		// its expression: a local, declared before the statement
		c.nlit++
		name := "complit__" + strconv.Itoa(c.nlit)
		c.pre = append(c.pre, c.typeDecl1(t, name)+" = {"+c.initList(x.InitializerList)+"};")
		return name
	}
	if !c.inClass {
		return "(" + c.typeName(x.TypeName) + ")" + "{" + c.initList(x.InitializerList) + "}"
	}
	c.nlit++
	name := "complit__" + strconv.Itoa(c.nlit)
	c.complits = append(c.complits, c.typeDecl1(t, name)+" = {"+c.initList(x.InitializerList)+"};")
	return name
}

// paren is n printed where a cast's operand goes.
func (c *cppgen) paren(n cc.ExpressionNode) string {
	s := c.expr(n)
	switch p := pass(n).(type) {
	case *cc.PrimaryExpression, *cc.PostfixExpression:
		return s
	case *cc.UnaryExpression:
		if p.Case != cc.UnaryExpressionSizeofExpr {
			return s
		}
	case *cc.CastExpression:
		return s
	}
	return "(" + s + ")"
}

// conv prints n converted to t as C converts it when it assigns: a cast
// where C++ would not convert.
func (c *cppgen) conv(n cc.ExpressionNode, t cc.Type) string {
	if c.needsCast(n, t) {
		c.nCasts++
		return "(" + c.typeStr(t) + ")" + c.paren(n)
	}
	return c.expr(n)
}

// needsCast reports whether C++ would not convert n to t implicitly where C
// does.
func (c *cppgen) needsCast(n cc.ExpressionNode, t cc.Type) bool {
	if t == nil || n == nil {
		return false
	}
	from := n.Type()
	if from == nil {
		return false
	}
	if t.Kind() != cc.Ptr {
		return false
	}
	if c.isNull(n) {
		return false
	}
	te := elemOf(t)
	if d := fnDesig(n); d != nil {
		return c.canon(te, false) != c.canon(d.Type(), false)
	}
	switch from.Kind() {
	case cc.Ptr, cc.Array:
	case cc.Function:
		return true
	default:
		return true // an integer where a pointer goes
	}
	fe := elemOf(from)
	if fe == nil || te == nil {
		return false
	}
	fconst := fe.Attributes().IsConst() || isStrLit(n)
	tconst := te.Attributes().IsConst()
	switch {
	case te.Kind() == cc.Void:
		return fconst && !tconst || fe.Kind() == cc.Function
	case fe.Kind() == cc.Void:
		return true
	case fconst && !tconst:
		return true
	}
	return c.canon(fe, false) != c.canon(te, false)
}

// canon is a type's identity, typedefs seen through: two types C++ takes as
// one (qualifiers at the top aside, unless quals) have one.
func (c *cppgen) canon(t cc.Type, quals bool) string {
	if t == nil {
		return "?"
	}
	q := ""
	if quals && t.Attributes().IsConst() {
		q = "const "
	}
	switch t.Kind() {
	case cc.Ptr:
		return q + "*" + c.canon(elemOf(t), true)
	case cc.Array:
		return q + "[]" + c.canon(elemOf(t), true)
	case cc.Function:
		f := t.(*cc.FunctionType)
		s := "fn("
		for _, p := range f.Parameters() {
			s += c.canon(p.Type(), false) + ","
		}
		if f.IsVariadic() {
			s += "..."
		}
		return s + ")" + c.canon(f.Result(), false)
	case cc.Struct:
		st := t.(*cc.StructType)
		if tag := tagStr(st.Tag()); tag != "" {
			return q + "struct " + tag
		}
		return q + "struct@" + t.String()
	case cc.Union:
		ut := t.(*cc.UnionType)
		if tag := tagStr(ut.Tag()); tag != "" {
			return q + "union " + tag
		}
		return q + "union@" + t.String()
	}
	return q + c.scalar(t)
}

// cppEffects reports whether n has a side effect or calls a function: an
// increment, a decrement, an assignment or a call.
func cppEffects(n cc.Node) bool {
	found := false
	var r func(n cc.Node)
	r = func(n cc.Node) {
		if found || n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionInc, cc.PostfixExpressionDec, cc.PostfixExpressionCall:
				found = true
				return
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
				found = true
				return
			case cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType:
				return
			}
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				found = true
				return
			}
		}
		walkChildrenFn(n, r)
	}
	r(n)
	return found
}

// cppOrderMatters reports whether the left of an assignment, evaluated
// after the right (C++17's order) rather than before it (gcc's C), could
// give another answer: the left changes an object the right reads, or one
// a function the right calls could reach -- one that is not a local whose
// address nothing takes -- or the left calls a function and the right has
// an effect.
func cppOrderMatters(l, r cc.ExpressionNode) bool {
	if !cppEffects(l) || !cppEffects(r) {
		return false
	}
	var changed []*cc.Declarator
	reach := false // the left changes what a call could see, or calls
	var target func(n cc.ExpressionNode)
	target = func(n cc.ExpressionNode) {
		// the object an increment or an assignment changes: its root name
		n = strip(n)
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
				changed = append(changed, d)
				if d.AddressTaken() || !d.IsParam() && d.Linkage() != cc.None || d.IsStatic() {
					reach = true
				}
				return
			}
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionSelect {
				target(x.PostfixExpression)
				return
			}
		}
		reach = true // through a pointer: anything
	}
	var walk func(n cc.Node)
	walk = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
				target(x.PostfixExpression)
			case cc.PostfixExpressionCall:
				reach = true
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
				target(x.UnaryExpression)
			}
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				target(x.UnaryExpression)
			}
		}
		walkChildrenFn(n, walk)
	}
	walk(l)
	calls := false
	reads := false
	var rw func(n cc.Node)
	rw = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionCall {
				calls = true
			}
		case *cc.PrimaryExpression:
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
				for _, c := range changed {
					if c == d {
						reads = true
					}
				}
			}
		}
		walkChildrenFn(n, rw)
	}
	rw(r)
	return reads || reach && (calls || cppEffects(r))
}
