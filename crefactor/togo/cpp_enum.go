package togo

// cpp_enum.go is the C's enumerations as C++'s scoped ones where the C uses
// one as a type and not as a number (doc/CPP-IDIOMS.md, item 3): `enum
// class E : U { ... }; using enum E;` -- the enumerators still named as the
// C names them -- where every value of type E and every enumerator of E is
// only stored in, compared with, switched on, passed as, returned as or
// cast to and from E.  Any other use -- arithmetic, an index, a truth value,
// an increment, an argument of a variadic, a conditional -- leaves the
// enumeration its integer type, as milestone 1 has every one.

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// cppEnum is a C enumeration the backend decides about.
type cppEnum struct {
	name  string // its C++ name: the typedef's, or the tag
	t     *cc.EnumType
	why   string // why it stays an integer: "" for a scoped one
	where cc.Node
}

// enumKey is an enumeration's identity across the front end's copies of its
// type: its first enumerator's name.
func enumKey(t cc.Type) string {
	e, ok := t.(*cc.EnumType)
	if !ok || len(e.Enumerators()) == 0 {
		return ""
	}
	return e.Enumerators()[0].Token.SrcStr()
}

// scopedEnums decides which enumerations are scoped.
func (c *cppgen) scopedEnums() {
	c.enums = map[string]*cppEnum{}
	c.enumOf = map[string]string{} // an enumerator's name -> its enumeration's key
	// the named enumerations: a tag, or a typedef of an untagged one
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.EnumSpecifier:
			if x.Case == cc.EnumSpecifierDef {
				if et, ok := x.Type().(*cc.EnumType); ok {
					k := enumKey(et)
					if k != "" && c.enums[k] == nil {
						c.enums[k] = &cppEnum{t: et, name: x.Token2.SrcStr()}
						for _, en := range et.Enumerators() {
							c.enumOf[en.Token.SrcStr()] = k
						}
					}
				}
			}
		case *cc.Declaration:
			// typedef enum { ... } name_T
			for l := x.DeclarationSpecifiers; l != nil; l = l.DeclarationSpecifiers {
				if l.Case != cc.DeclarationSpecifiersTypeSpec || l.TypeSpecifier.Case != cc.TypeSpecifierEnum {
					continue
				}
				es := l.TypeSpecifier.EnumSpecifier
				if es.Case != cc.EnumSpecifierDef || es.Token2.SrcStr() != "" {
					continue
				}
				if il := x.InitDeclaratorList; il != nil && il.InitDeclaratorList == nil && il.InitDeclarator.Declarator.IsTypename() &&
					il.InitDeclarator.Declarator.Pointer == nil {
					rec(es)
					if e := c.enums[enumKey(es.Type())]; e != nil && e.name == "" {
						e.name = il.InitDeclarator.Declarator.Name()
					}
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if c.mine(tu.ExternalDeclaration) {
			rec(tu.ExternalDeclaration)
		}
	}
	for _, e := range c.enums {
		if e.name == "" {
			e.why = "unnamed"
		}
	}
	// every use, judged by its context
	parent := map[cc.Node]cc.Node{}
	var fn *cc.FunctionDefinition
	var walk func(n, p cc.Node)
	walk = func(n, p cc.Node) {
		if n == nil {
			return
		}
		parent[n] = p
		if f, ok := n.(*cc.FunctionDefinition); ok {
			fn = f
		}
		if x, ok := n.(cc.ExpressionNode); ok {
			c.judgeEnumUse(x, parent, fn)
		}
		walkChildrenFn(n, func(ch cc.Node) { walk(ch, n) })
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if c.mine(tu.ExternalDeclaration) {
			walk(tu.ExternalDeclaration, nil)
		}
	}
	for _, e := range c.enums {
		if e.why == "" {
			c.nScoped++
		}
	}
}

// enumOfExpr is the enumeration an expression's value is of: its type's,
// or, an enumerator, its own's; "" for none.
func (c *cppgen) enumOfExpr(n cc.ExpressionNode) string {
	s := strip(n)
	if p, ok := s.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if en, ok := p.ResolvedTo().(*cc.Enumerator); ok {
			return c.enumOf[en.Token.SrcStr()]
		}
	}
	if t := n.Type(); t != nil && t.Kind() == cc.Enum {
		return enumKey(t)
	}
	return ""
}

// judgeEnumUse rules an enumeration out when the expression n, of its
// value, is used as a number.
func (c *cppgen) judgeEnumUse(n cc.ExpressionNode, parent map[cc.Node]cc.Node, fn *cc.FunctionDefinition) {
	// only the outermost node of an expression's pass-through chain
	if p, ok := parent[n].(cc.ExpressionNode); ok && pass(p) == pass(n) {
		return
	}
	if q, ok := parent[n].(*cc.PrimaryExpression); ok && q.Case == cc.PrimaryExpressionExpr {
		return // judged as the parenthesis
	}
	k := c.enumOfExpr(n)
	if k == "" {
		// a value of no enumeration where one goes: that enumeration is a
		// number
		if slot := c.enumSlot(n, parent, fn); slot != "" {
			if e := c.enums[slot]; e != nil && e.why == "" {
				e.why, e.where = "a value of another type stored in it", n
			}
		}
		return
	}
	e := c.enums[k]
	if e == nil || e.why != "" {
		return
	}
	no := func(why string) { e.why, e.where = why, n }
	same := func(m cc.ExpressionNode) bool { return m != nil && c.enumOfExpr(m) == k }
	sameT := func(t cc.Type) bool { return t != nil && t.Kind() == cc.Enum && enumKey(t) == k }
	// the context, through the expression's own parentheses and the
	// grammar's pass-through productions
	var ctx cc.Node = parent[n]
	for {
		if p, ok := ctx.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionExpr {
			ctx = parent[ctx]
			continue
		}
		if x, ok := ctx.(cc.ExpressionNode); ok && pass(x) != x {
			ctx = parent[ctx]
			continue
		}
		break
	}
	switch x := ctx.(type) {
	case *cc.AssignmentExpression:
		if x.Token.SrcStr() != "=" {
			no("a compound assignment")
		} else if !same(x.UnaryExpression) || !same(x.AssignmentExpression) && !sameT(x.UnaryExpression.Type()) {
			no("an assignment of another type")
		}
	case *cc.EqualityExpression:
		if !same(x.EqualityExpression) || !same(x.RelationalExpression) {
			no("a comparison with another type")
		}
	case *cc.RelationalExpression:
		if !same(x.RelationalExpression) || !same(x.ShiftExpression) {
			no("a comparison with another type")
		}
	case *cc.CastExpression:
		// a cast to or from it: static_cast, which C++ allows
	case *cc.Initializer:
		if !sameT(x.Type()) {
			no("an initial value of another type")
		}
	case *cc.ArgumentExpressionList:
		call, _ := parent[c.argsHead(x, parent)].(*cc.PostfixExpression)
		if call == nil || !c.argIs(call, x, k) {
			no("an argument of another type")
		}
	case *cc.JumpStatement:
		if fn == nil || !sameT(fn.Declarator.Type().(*cc.FunctionType).Result()) {
			no("a result of another type")
		}
	case *cc.SelectionStatement:
		if x.Case != cc.SelectionStatementSwitch {
			no("a truth value")
		} else if !c.casesOf(x.Statement, k) {
			no("a switch with a case of another type")
		}
	case *cc.LabeledStatement:
		// a case label: its switch is judged by the switch
		if sw := enclosingSwitch(x, parent); sw == nil || c.enumOfExpr(sw.ExpressionList) != k {
			no("a case of another switch")
		}
	case *cc.ExpressionStatement:
	case *cc.UnaryExpression:
		if x.Case != cc.UnaryExpressionSizeofExpr {
			no("an operand of " + x.Case.String())
		}
	default:
		no("an operand of a " + nodeKind(ctx))
	}
}

func nodeKind(n cc.Node) string {
	if n == nil {
		return "nothing"
	}
	return cppNodeKind(n)
}

// cppNodeKind is what a node is, for the reasons.
func cppNodeKind(n cc.Node) string {
	s := ""
	switch n.(type) {
	case *cc.AdditiveExpression, *cc.MultiplicativeExpression:
		s = "arithmetic"
	case *cc.PostfixExpression:
		s = "postfix expression"
	case *cc.ConditionalExpression:
		s = "conditional"
	case *cc.LogicalAndExpression, *cc.LogicalOrExpression:
		s = "logical operator"
	default:
		s = "construct"
	}
	return s
}

// argsHead is the first node of an argument list.
func (c *cppgen) argsHead(l *cc.ArgumentExpressionList, parent map[cc.Node]cc.Node) cc.Node {
	var n cc.Node = l
	for {
		p, ok := parent[n].(*cc.ArgumentExpressionList)
		if !ok {
			return n
		}
		n = p
	}
}

// argIs reports whether the argument l of call is passed to a parameter of
// the enumeration k.
func (c *cppgen) argIs(call *cc.PostfixExpression, l *cc.ArgumentExpressionList, k string) bool {
	var ft *cc.FunctionType
	if d := fnDesig(call.PostfixExpression); d != nil {
		ft, _ = d.Type().(*cc.FunctionType)
	} else if t := call.PostfixExpression.Type(); t != nil {
		ft, _ = elemOf(t).(*cc.FunctionType)
	}
	if ft == nil {
		return false
	}
	i := 0
	for a := call.ArgumentExpressionList; a != nil && a != l; a = a.ArgumentExpressionList {
		i++
	}
	if i >= len(ft.Parameters()) {
		return false // variadic
	}
	t := ft.Parameters()[i].Type()
	return t != nil && t.Kind() == cc.Enum && enumKey(t) == k
}

// casesOf reports whether every case label of a switch's body is an
// enumerator of k.
func (c *cppgen) casesOf(body *cc.Statement, k string) bool {
	ok := true
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil || !ok {
			return
		}
		if s, isSel := n.(*cc.SelectionStatement); isSel && s.Case == cc.SelectionStatementSwitch {
			return // its own cases
		}
		if l, isL := n.(*cc.LabeledStatement); isL && (l.Case == cc.LabeledStatementCaseLabel || l.Case == cc.LabeledStatementRange) {
			if c.enumOfExpr(l.ConstantExpression) != k || l.Case == cc.LabeledStatementRange {
				ok = false
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(body)
	return ok
}

// enclosingSwitch is the switch a case label belongs to.
func enclosingSwitch(n cc.Node, parent map[cc.Node]cc.Node) *cc.SelectionStatement {
	for p := parent[n]; p != nil; p = parent[p] {
		if s, ok := p.(*cc.SelectionStatement); ok && s.Case == cc.SelectionStatementSwitch {
			return s
		}
	}
	return nil
}

// scoped is the scoped enumeration t is, or nil.
func (c *cppgen) scoped(t cc.Type) *cppEnum {
	if t == nil || t.Kind() != cc.Enum {
		return nil
	}
	if e := c.enums[enumKey(t)]; e != nil && e.why == "" {
		return e
	}
	return nil
}

// enumSlot is the enumeration of the slot an expression's value goes to:
// an assignment's left, a parameter, an initial value's object, a result.
func (c *cppgen) enumSlot(n cc.ExpressionNode, parent map[cc.Node]cc.Node, fn *cc.FunctionDefinition) string {
	if x, ok := strip(n).(*cc.CastExpression); ok && x.Case == cc.CastExpressionCast {
		return "" // a cast to it: static_cast
	}
	var ctx cc.Node = parent[n]
	for {
		if p, ok := ctx.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionExpr {
			ctx = parent[ctx]
			continue
		}
		if x, ok := ctx.(cc.ExpressionNode); ok && pass(x) != x {
			ctx = parent[ctx]
			continue
		}
		break
	}
	key := func(t cc.Type) string {
		if t != nil && t.Kind() == cc.Enum {
			return enumKey(t)
		}
		return ""
	}
	switch x := ctx.(type) {
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			return key(x.UnaryExpression.Type())
		}
	case *cc.EqualityExpression:
		return c.enumOfExpr(x.EqualityExpression) + c.enumOfExpr(x.RelationalExpression)
	case *cc.RelationalExpression:
		return c.enumOfExpr(x.RelationalExpression) + c.enumOfExpr(x.ShiftExpression)
	case *cc.Initializer:
		return key(x.Type())
	case *cc.ArgumentExpressionList:
		call, _ := parent[c.argsHead(x, parent)].(*cc.PostfixExpression)
		if call == nil {
			return ""
		}
		var ft *cc.FunctionType
		if d := fnDesig(call.PostfixExpression); d != nil {
			ft, _ = d.Type().(*cc.FunctionType)
		} else if t := call.PostfixExpression.Type(); t != nil {
			ft, _ = elemOf(t).(*cc.FunctionType)
		}
		i := 0
		for a := call.ArgumentExpressionList; a != nil && a != x; a = a.ArgumentExpressionList {
			i++
		}
		if ft != nil && i < len(ft.Parameters()) {
			return key(ft.Parameters()[i].Type())
		}
	case *cc.JumpStatement:
		if fn != nil {
			return key(fn.Declarator.Type().(*cc.FunctionType).Result())
		}
	}
	return ""
}
