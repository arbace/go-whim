package cemit

import (
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// expr prints an expression on one line, in the canonical spacing: one space
// either side of a binary operator, none between a unary operator and its
// operand, none inside brackets, one after a comma.
//
// PARENTHESES ARE THE SOURCE'S.  A printer that re-derived them from precedence
// would be deciding what the C means; this one prints the parentheses the tree
// has (PrimaryExpressionExpr) and no others, so the expression that comes back
// groups exactly as the one that went in.
func (e *emitter) expr(n cc.ExpressionNode) string {
	if n == nil {
		return ""
	}
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent, cc.PrimaryExpressionInt, cc.PrimaryExpressionFloat,
			cc.PrimaryExpressionChar, cc.PrimaryExpressionLChar, cc.PrimaryExpressionString,
			cc.PrimaryExpressionLString:
			return tok(x.Token)
		case cc.PrimaryExpressionExpr:
			return "(" + e.expr(x.ExpressionList) + ")"
		case cc.PrimaryExpressionStmt:
			return "(" + strings.TrimSpace(e.compoundInline(x.CompoundStatement)) + ")"
		case cc.PrimaryExpressionGeneric:
			return e.generic(x.GenericSelection)
		}
		e.fail(x, "primary expression %v", x.Case)
		return ""

	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionPrimary:
			return e.expr(x.PrimaryExpression)
		case cc.PostfixExpressionIndex:
			return e.expr(x.PostfixExpression) + "[" + e.expr(x.ExpressionList) + "]"
		case cc.PostfixExpressionCall:
			return e.expr(x.PostfixExpression) + "(" + e.args(x.ArgumentExpressionList) + ")"
		case cc.PostfixExpressionSelect:
			return e.expr(x.PostfixExpression) + "." + tok(x.Token2)
		case cc.PostfixExpressionPSelect:
			return e.expr(x.PostfixExpression) + "->" + tok(x.Token2)
		case cc.PostfixExpressionInc:
			return e.expr(x.PostfixExpression) + "++"
		case cc.PostfixExpressionDec:
			return e.expr(x.PostfixExpression) + "--"
		case cc.PostfixExpressionComplit:
			return "(" + e.typeName(x.TypeName) + "){" + e.initializerList(x.InitializerList) + "}"
		}
		e.fail(x, "postfix expression %v", x.Case)
		return ""

	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionPostfix:
			return e.expr(x.PostfixExpression)
		case cc.UnaryExpressionInc:
			return "++" + e.expr(x.UnaryExpression)
		case cc.UnaryExpressionDec:
			return "--" + e.expr(x.UnaryExpression)
		case cc.UnaryExpressionAddrof:
			return "&" + e.expr(x.CastExpression)
		case cc.UnaryExpressionDeref:
			return "*" + e.expr(x.CastExpression)
		case cc.UnaryExpressionPlus:
			return "+" + e.expr(x.CastExpression)
		case cc.UnaryExpressionMinus:
			return "-" + e.expr(x.CastExpression)
		case cc.UnaryExpressionCpl:
			return "~" + e.expr(x.CastExpression)
		case cc.UnaryExpressionNot:
			return "!" + e.expr(x.CastExpression)
		case cc.UnaryExpressionSizeofExpr:
			return "sizeof " + e.expr(x.UnaryExpression)
		case cc.UnaryExpressionSizeofType:
			return "sizeof(" + e.typeName(x.TypeName) + ")"
		case cc.UnaryExpressionAlignofExpr:
			return "alignof " + e.expr(x.UnaryExpression)
		case cc.UnaryExpressionAlignofType:
			return "alignof(" + e.typeName(x.TypeName) + ")"
		case cc.UnaryExpressionLabelAddr:
			return "&&" + tok(x.Token2)
		}
		e.fail(x, "unary expression %v", x.Case)
		return ""

	case *cc.CastExpression:
		switch x.Case {
		case cc.CastExpressionUnary:
			return e.expr(x.UnaryExpression)
		case cc.CastExpressionCast:
			return "(" + e.typeName(x.TypeName) + ")" + e.expr(x.CastExpression)
		}
		e.fail(x, "cast expression %v", x.Case)
		return ""

	case *cc.MultiplicativeExpression:
		if x.Case == cc.MultiplicativeExpressionCast {
			return e.expr(x.CastExpression)
		}
		return e.bin(x.MultiplicativeExpression, tok(x.Token), x.CastExpression)

	case *cc.AdditiveExpression:
		if x.Case == cc.AdditiveExpressionMul {
			return e.expr(x.MultiplicativeExpression)
		}
		return e.bin(x.AdditiveExpression, tok(x.Token), x.MultiplicativeExpression)

	case *cc.ShiftExpression:
		if x.Case == cc.ShiftExpressionAdd {
			return e.expr(x.AdditiveExpression)
		}
		return e.bin(x.ShiftExpression, tok(x.Token), x.AdditiveExpression)

	case *cc.RelationalExpression:
		if x.Case == cc.RelationalExpressionShift {
			return e.expr(x.ShiftExpression)
		}
		return e.bin(x.RelationalExpression, tok(x.Token), x.ShiftExpression)

	case *cc.EqualityExpression:
		if x.Case == cc.EqualityExpressionRel {
			return e.expr(x.RelationalExpression)
		}
		return e.bin(x.EqualityExpression, tok(x.Token), x.RelationalExpression)

	case *cc.AndExpression:
		if x.Case == cc.AndExpressionEq {
			return e.expr(x.EqualityExpression)
		}
		return e.bin(x.AndExpression, "&", x.EqualityExpression)

	case *cc.ExclusiveOrExpression:
		if x.Case == cc.ExclusiveOrExpressionAnd {
			return e.expr(x.AndExpression)
		}
		return e.bin(x.ExclusiveOrExpression, "^", x.AndExpression)

	case *cc.InclusiveOrExpression:
		if x.Case == cc.InclusiveOrExpressionXor {
			return e.expr(x.ExclusiveOrExpression)
		}
		return e.bin(x.InclusiveOrExpression, "|", x.ExclusiveOrExpression)

	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionOr {
			return e.expr(x.InclusiveOrExpression)
		}
		return e.bin(x.LogicalAndExpression, "&&", x.InclusiveOrExpression)

	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLAnd {
			return e.expr(x.LogicalAndExpression)
		}
		return e.bin(x.LogicalOrExpression, "||", x.LogicalAndExpression)

	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionLOr {
			return e.expr(x.LogicalOrExpression)
		}
		return e.expr(x.LogicalOrExpression) + " ? " + e.expr(x.ExpressionList) +
			" : " + e.expr(x.ConditionalExpression)

	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return e.expr(x.ConditionalExpression)
		}
		return e.bin(x.UnaryExpression, tok(x.Token), x.AssignmentExpression)

	case *cc.ExpressionList:
		var parts []string
		for l := x; l != nil; l = l.ExpressionList {
			parts = append(parts, e.expr(l.AssignmentExpression))
		}
		return strings.Join(parts, ", ")

	case *cc.ConstantExpression:
		return e.expr(x.ConditionalExpression)
	}
	e.fail(n, "expression node %T", n)
	return ""
}

func (e *emitter) bin(l cc.ExpressionNode, op string, r cc.ExpressionNode) string {
	return e.expr(l) + " " + op + " " + e.expr(r)
}

func (e *emitter) args(n *cc.ArgumentExpressionList) string {
	var parts []string
	for l := n; l != nil; l = l.ArgumentExpressionList {
		parts = append(parts, e.expr(l.AssignmentExpression))
	}
	return strings.Join(parts, ", ")
}

// generic prints `_Generic(expr, type: value, default: value)`, which the host
// half of the product uses to assert that two typedefs are the same type.
func (e *emitter) generic(n *cc.GenericSelection) string {
	var parts []string
	for l := n.GenericAssociationList; l != nil; l = l.GenericAssociationList {
		a := l.GenericAssociation
		switch a.Case {
		case cc.GenericAssociationType:
			parts = append(parts, e.typeName(a.TypeName)+": "+e.expr(a.AssignmentExpression))
		case cc.GenericAssociationDefault:
			parts = append(parts, "default: "+e.expr(a.AssignmentExpression))
		default:
			e.fail(a, "generic association %v", a.Case)
		}
	}
	return "_Generic(" + e.expr(n.AssignmentExpression) + ", " + strings.Join(parts, ", ") + ")"
}
