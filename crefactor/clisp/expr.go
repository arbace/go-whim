package clisp

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// THE PARENTHESES C NEEDS ARE NOT WRITTEN; THE ONES IT DOES NOT NEED ARE.  A
// prefix form carries its grouping, so `(* (+ a b) c)` is `(a + b) * c` and
// ToC writes the parentheses precedence asks for.  A pair the source has where
// precedence does not ask for one -- `(a * b) + c`, `return (x);` -- is kept
// as `(paren ...)`, since cemit keeps every pair the source wrote and the
// round trip is byte for byte.
//
// The levels are the grammar's: an operand is written bare when its form is
// at least the level its place asks for.
const (
	lvComma   = 1  // expression
	lvAssign  = 2  // assignment-expression
	lvCond    = 3  // conditional-expression (constant-expression)
	lvLOr     = 4  // ||
	lvLAnd    = 5  // &&
	lvOr      = 6  // |
	lvXor     = 7  // ^
	lvAnd     = 8  // &
	lvEq      = 9  // == !=
	lvRel     = 10 // < > <= >=
	lvShift   = 11 // << >>
	lvAdd     = 12 // + -
	lvMul     = 13 // * / %
	lvCast    = 14 // cast-expression
	lvUnary   = 15 // unary-expression
	lvPostfix = 16 // postfix-expression
	lvPrimary = 17 // primary-expression
)

// binaryLevel is a binary operator's level; assignment operators are lvAssign.
var binaryLevel = map[string]int{
	"||": lvLOr, "&&": lvLAnd, "|": lvOr, "^": lvXor, "&": lvAnd,
	"==": lvEq, "!=": lvEq, "<": lvRel, ">": lvRel, "<=": lvRel, ">=": lvRel,
	"<<": lvShift, ">>": lvShift, "+": lvAdd, "-": lvAdd, "*": lvMul, "/": lvMul, "%": lvMul,
	"=": lvAssign, "*=": lvAssign, "/=": lvAssign, "%=": lvAssign, "+=": lvAssign, "-=": lvAssign,
	"<<=": lvAssign, ">>=": lvAssign, "&=": lvAssign, "^=": lvAssign, "|=": lvAssign,
}

// unaryHeads are the prefix operators' forms; `-` and `+` of one operand are
// unary too.
var unaryHeads = map[string]bool{
	"addr": true, "deref": true, "!": true, "~": true, "pre++": true, "pre--": true,
	"sizeof": true, "sizeof-bare": true, "sizeof-type": true,
	"alignof": true, "alignof-bare": true, "alignof-type": true, "label-addr": true,
}

var postfixHeads = map[string]bool{
	"call": true, "index": true, ".": true, "->": true, "post++": true, "post--": true, "literal": true,
}

// level is the level of an expression form.
func level(n *Node) int {
	if !n.list {
		return lvPrimary
	}
	h := n.Head()
	switch {
	case h == "comma":
		return lvComma
	case h == "?":
		return lvCond
	case h == "cast":
		return lvCast
	case (h == "-" || h == "+") && len(n.List) == 2:
		return lvUnary
	case unaryHeads[h]:
		return lvUnary
	case postfixHeads[h]:
		return lvPostfix
	}
	if l, ok := binaryLevel[h]; ok {
		return l
	}
	return lvPrimary
}

// unwrap follows the grammar's one-child levels down to the node that says
// something, asking at each one whether the whole of it is a macro
// invocation -- the order cemit's expr asks in.
func (c *conv) unwrap(n cc.ExpressionNode) (cc.ExpressionNode, string, bool) {
	for n != nil {
		if s, ok := c.macro(n); ok {
			return nil, s, true
		}
		var next cc.ExpressionNode
		switch x := n.(type) {
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionPrimary {
				next = x.PrimaryExpression
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionPostfix {
				next = x.PostfixExpression
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionUnary {
				next = x.UnaryExpression
			}
		case *cc.MultiplicativeExpression:
			if x.Case == cc.MultiplicativeExpressionCast {
				next = x.CastExpression
			}
		case *cc.AdditiveExpression:
			if x.Case == cc.AdditiveExpressionMul {
				next = x.MultiplicativeExpression
			}
		case *cc.ShiftExpression:
			if x.Case == cc.ShiftExpressionAdd {
				next = x.AdditiveExpression
			}
		case *cc.RelationalExpression:
			if x.Case == cc.RelationalExpressionShift {
				next = x.ShiftExpression
			}
		case *cc.EqualityExpression:
			if x.Case == cc.EqualityExpressionRel {
				next = x.RelationalExpression
			}
		case *cc.AndExpression:
			if x.Case == cc.AndExpressionEq {
				next = x.EqualityExpression
			}
		case *cc.ExclusiveOrExpression:
			if x.Case == cc.ExclusiveOrExpressionAnd {
				next = x.AndExpression
			}
		case *cc.InclusiveOrExpression:
			if x.Case == cc.InclusiveOrExpressionXor {
				next = x.ExclusiveOrExpression
			}
		case *cc.LogicalAndExpression:
			if x.Case == cc.LogicalAndExpressionOr {
				next = x.InclusiveOrExpression
			}
		case *cc.LogicalOrExpression:
			if x.Case == cc.LogicalOrExpressionLAnd {
				next = x.LogicalAndExpression
			}
		case *cc.ConditionalExpression:
			if x.Case == cc.ConditionalExpressionLOr {
				next = x.LogicalOrExpression
			}
		case *cc.AssignmentExpression:
			if x.Case == cc.AssignmentExpressionCond {
				next = x.ConditionalExpression
			}
		case *cc.ConstantExpression:
			next = x.ConditionalExpression
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				next = x.AssignmentExpression
			}
		}
		if next == nil {
			return n, "", false
		}
		n = next
	}
	return nil, "", false
}

// expr is an expression's form in a place that asks for level want.
func (c *conv) expr(n cc.ExpressionNode, want int) *Node {
	x, text, isMacro := c.unwrap(n)
	if isMacro {
		return macroForm(text)
	}
	if x == nil {
		c.fail(nil, "a missing expression")
		return A("?")
	}
	if p, ok := x.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionExpr {
		in := c.expr(p.ExpressionList, lvComma)
		if level(in) < want {
			return in // ToC writes these parentheses
		}
		return L(A("paren"), in)
	}
	return c.form(x)
}

// bin is a binary operation, a left-nested run of one operator flattened:
// `a + b + c` is `(+ a b c)`.
func (c *conv) bin(op string, l, r cc.ExpressionNode) *Node {
	lv := binaryLevel[op]
	left := c.expr(l, lv)
	right := c.expr(r, lv+1)
	if lv != lvAssign && left.Is(op) && len(left.List) >= 3 {
		return left.add(right)
	}
	return L(A(op), left, right)
}

func (c *conv) form(n cc.ExpressionNode) *Node {
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent, cc.PrimaryExpressionInt, cc.PrimaryExpressionFloat,
			cc.PrimaryExpressionChar, cc.PrimaryExpressionLChar, cc.PrimaryExpressionString,
			cc.PrimaryExpressionLString:
			return A(tok(x.Token))
		case cc.PrimaryExpressionStmt:
			saved := c.noMacros
			c.noMacros = true
			f := L(A("stmt-expr")).add(c.blockItems(x.CompoundStatement.BlockItemList)...)
			c.noMacros = saved
			return f
		case cc.PrimaryExpressionGeneric:
			return c.generic(x.GenericSelection)
		}
		c.fail(x, "primary expression %v", x.Case)

	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			a := c.expr(x.PostfixExpression, lvPostfix)
			i := c.expr(x.ExpressionList, lvComma)
			if a.Is("index") {
				return a.add(i)
			}
			return L(A("index"), a, i)
		case cc.PostfixExpressionCall:
			f := L(A("call"), c.expr(x.PostfixExpression, lvPostfix))
			for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
				f.add(c.expr(l.AssignmentExpression, lvAssign))
			}
			return f
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			inner := c.expr(x.PostfixExpression, lvPostfix)
			sep := "."
			if x.Case == cc.PostfixExpressionPSelect {
				sep = "->"
			}
			name := tok(x.Token2)
			if !c.noMacros {
				var drop bool
				name, drop = c.m.Member(x.Token2)
				if drop {
					// the inner selection printed the member macro whole.
					return inner
				}
			}
			m := macroForm(name)
			if inner.Is(sep) && !m.list {
				return inner.add(m)
			}
			return L(A(sep), inner, m)
		case cc.PostfixExpressionInc:
			return L(A("post++"), c.expr(x.PostfixExpression, lvPostfix))
		case cc.PostfixExpressionDec:
			return L(A("post--"), c.expr(x.PostfixExpression, lvPostfix))
		case cc.PostfixExpressionComplit:
			// C23's storage classes stand before the type: `(literal static
			// (array int) 1 2)`.
			f := L(A("literal"))
			for _, s := range x.StorageClassSpecifiers {
				f.add(A(tok(s.Token)))
			}
			return f.add(c.typeName(x.TypeName)).add(c.initItems(x.InitializerList)...)
		}
		c.fail(x, "postfix expression %v", x.Case)

	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionInc:
			return L(A("pre++"), c.expr(x.UnaryExpression, lvUnary))
		case cc.UnaryExpressionDec:
			return L(A("pre--"), c.expr(x.UnaryExpression, lvUnary))
		case cc.UnaryExpressionAddrof:
			return L(A("addr"), c.expr(x.CastExpression, lvCast))
		case cc.UnaryExpressionDeref:
			return L(A("deref"), c.expr(x.CastExpression, lvCast))
		case cc.UnaryExpressionPlus:
			return L(A("+"), c.expr(x.CastExpression, lvCast))
		case cc.UnaryExpressionMinus:
			return L(A("-"), c.expr(x.CastExpression, lvCast))
		case cc.UnaryExpressionCpl:
			return L(A("~"), c.expr(x.CastExpression, lvCast))
		case cc.UnaryExpressionNot:
			return L(A("!"), c.expr(x.CastExpression, lvCast))
		case cc.UnaryExpressionSizeofExpr:
			return c.sizeof("sizeof", x.UnaryExpression)
		case cc.UnaryExpressionSizeofType:
			return L(A("sizeof-type"), c.typeName(x.TypeName))
		case cc.UnaryExpressionAlignofExpr:
			return c.sizeof("alignof", x.UnaryExpression)
		case cc.UnaryExpressionAlignofType:
			return L(A("alignof-type"), c.typeName(x.TypeName))
		case cc.UnaryExpressionLabelAddr:
			return L(A("label-addr"), A(tok(x.Token2)))
		}
		c.fail(x, "unary expression %v", x.Case)

	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return L(A("cast"), c.typeName(x.TypeName), c.expr(x.CastExpression, lvCast))
		}
		c.fail(x, "cast expression %v", x.Case)

	case *cc.MultiplicativeExpression:
		return c.bin(tok(x.Token), x.MultiplicativeExpression, x.CastExpression)
	case *cc.AdditiveExpression:
		return c.bin(tok(x.Token), x.AdditiveExpression, x.MultiplicativeExpression)
	case *cc.ShiftExpression:
		return c.bin(tok(x.Token), x.ShiftExpression, x.AdditiveExpression)
	case *cc.RelationalExpression:
		return c.bin(tok(x.Token), x.RelationalExpression, x.ShiftExpression)
	case *cc.EqualityExpression:
		return c.bin(tok(x.Token), x.EqualityExpression, x.RelationalExpression)
	case *cc.AndExpression:
		return c.bin("&", x.AndExpression, x.EqualityExpression)
	case *cc.ExclusiveOrExpression:
		return c.bin("^", x.ExclusiveOrExpression, x.AndExpression)
	case *cc.InclusiveOrExpression:
		return c.bin("|", x.InclusiveOrExpression, x.ExclusiveOrExpression)
	case *cc.LogicalAndExpression:
		return c.bin("&&", x.LogicalAndExpression, x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		return c.bin("||", x.LogicalOrExpression, x.LogicalAndExpression)
	case *cc.ConditionalExpression:
		return L(A("?"), c.expr(x.LogicalOrExpression, lvLOr), c.expr(x.ExpressionList, lvComma),
			c.expr(x.ConditionalExpression, lvCond))
	case *cc.AssignmentExpression:
		op := tok(x.Token)
		if _, ok := binaryLevel[op]; !ok {
			c.fail(x, "assignment operator %s", op)
		}
		return L(A(op), c.expr(x.UnaryExpression, lvUnary), c.expr(x.AssignmentExpression, lvAssign))
	case *cc.ExpressionList:
		f := L(A("comma"))
		for l := x; l != nil; l = l.ExpressionList {
			f.add(c.expr(l.AssignmentExpression, lvAssign))
		}
		return f
	default:
		c.fail(n, "expression node %T", n)
	}
	return A("?")
}

// sizeof is `(sizeof X)` for `sizeof(X)` -- the operand parenthesised, as
// the source nearly always has it -- and `(sizeof-bare X)` for an operand
// that is not; alignof the same.
func (c *conv) sizeof(kw string, operand cc.ExpressionNode) *Node {
	x, text, isMacro := c.unwrap(operand)
	if !isMacro {
		if p, ok := x.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionExpr {
			return L(A(kw), c.expr(p.ExpressionList, lvComma))
		}
		return L(A(kw+"-bare"), c.form(x))
	}
	return L(A(kw+"-bare"), macroForm(text))
}

func (c *conv) generic(n *cc.GenericSelection) *Node {
	f := L(A("generic"), c.expr(n.AssignmentExpression, lvAssign))
	for l := n.GenericAssociationList; l != nil; l = l.GenericAssociationList {
		a := l.GenericAssociation
		switch a.Case {
		case cc.GenericAssociationType:
			f.add(L(c.typeName(a.TypeName), c.expr(a.AssignmentExpression, lvAssign)))
		case cc.GenericAssociationDefault:
			f.add(L(A("default"), c.expr(a.AssignmentExpression, lvAssign)))
		default:
			c.fail(a, "generic association %v", a.Case)
		}
	}
	return f
}
