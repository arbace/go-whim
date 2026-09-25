package ccx

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

func isByte(t cc.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case cc.Char, cc.SChar, cc.UChar:
		return true
	}
	return false
}

func ptrElem(t cc.Type) cc.Type {
	if t == nil {
		return nil
	}
	switch x := t.(type) {
	case *cc.PointerType:
		return x.Elem()
	case *cc.ArrayType:
		return x.Elem()
	}
	return nil
}

// srcText is the source of a node, for naming an expression.
func srcText(n cc.Node) string {
	var b strings.Builder
	var tok func(v reflect.Value)
	tok = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
			if v.IsNil() {
				return
			}
			if t, ok := v.Interface().(cc.Token); ok {
				b.WriteString(t.SrcStr())
				return
			}
			tok(v.Elem())
			return
		}
		if t, ok := v.Interface().(cc.Token); ok {
			b.WriteString(t.SrcStr())
			return
		}
		if v.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				tok(v.Field(i))
			}
		}
	}
	tok(reflect.ValueOf(n))
	return b.String()
}

// callee names the function an expression calls, if it is a direct call.
func callee(e cc.ExpressionNode) string {
	for {
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				e = x.ExpressionList
				continue
			}
			return ""
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				return ""
			}
			e = x.AssignmentExpression
			continue
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionCall {
				if p, ok := x.PostfixExpression.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
					return p.Token.SrcStr()
				}
			}
			return ""
		}
		return ""
	}
}

// memberName is the field an expression reads, if it is a member access.
func memberName(e cc.ExpressionNode) string {
	for {
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				e = x.ExpressionList
				continue
			}
			return ""
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				return ""
			}
			e = x.AssignmentExpression
			continue
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionPSelect {
				return x.Token2.SrcStr()
			}
			return ""
		}
		return ""
	}
}

// unparen strips parentheses from an expression.
func unparen(e cc.ExpressionNode) cc.ExpressionNode {
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

// Casts classifies every pointer cast by what it converts.  A cast of any
// pointer to char * handed straight to one of p.ByteFuncs is a view of the
// object as bytes.
func Casts(ast *cc.AST, p Profile) Result {
	allocators, byteFuncs := set(p.Allocators), set(p.ByteFuncs)
	growData := "growarray: " + p.GrowArray.Data + ", to the element type"
	classes := map[string]int{}
	var left []Finding
	// the casts that are arguments of a function of bytes
	byteArg := map[*cc.CastExpression]bool{}
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		x, ok := n.(*cc.PostfixExpression)
		if !ok || x.Case != cc.PostfixExpressionCall {
			return
		}
		p, ok := x.PostfixExpression.(*cc.PrimaryExpression)
		if !ok || p.Case != cc.PrimaryExpressionIdent || !byteFuncs[p.Token.SrcStr()] {
			return
		}
		for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
			if c, ok := unparen(l.AssignmentExpression).(*cc.CastExpression); ok {
				byteArg[c] = true
			}
		}
	})
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		x, ok := n.(*cc.CastExpression)
		if !ok || x.Case != cc.CastExpressionCast {
			return
		}
		to := x.Type()
		from := typeOf(x.CastExpression)
		if to == nil || from == nil || to.Kind() != cc.Ptr {
			return
		}
		te, fe := ptrElem(to), ptrElem(from)
		if fe != nil && fe.Kind() == cc.Void {
			if inner, ok := unparen(x.CastExpression).(*cc.CastExpression); ok && inner.Case == cc.CastExpressionCast {
				if v := inner.CastExpression.Value(); v != nil && fmt.Sprint(v) == "0" {
					classes["null of a type: nullptr cast to the pointer it is used as"]++
					return
				}
			}
		}
		if byteArg[x] && isByte(te) {
			classes["bytes: an object handed to a function of bytes (memmove, memset, memcmp)"]++
			return
		}
		switch {
		case from.Kind() == cc.Ptr || from.Kind() == cc.Array:
		default:
			if from.Kind() == cc.Function {
				classes["a function designator to its own pointer type"]++
				return
			}
			if cc.IsIntegerType(from) {
				if v := x.CastExpression.Value(); v != nil && fmt.Sprint(v) == "0" {
					classes["null: the constant 0 (nullptr)"]++
					return
				}
			}
			left = append(left, Finding{fn, x.Position().String(), fmt.Sprintf("(%s) of a %s", to, from)})
			return
		}
		switch {
		case isByte(te) && isByte(fe):
			classes["bytes: between char, unsigned char and signed char pointers"]++
		case te != nil && fe != nil && te.String() == fe.String():
			classes["no change: to the type it already has"]++
		case fe != nil && fe.Kind() == cc.Void && allocators[callee(x.CastExpression)]:
			classes["allocation: void * from an allocator, to the type allocated"]++
		case fe != nil && fe.Kind() == cc.Void && p.GrowArray.Data != "" && memberName(x.CastExpression) == p.GrowArray.Data:
			classes[growData]++
		case te != nil && te.Kind() == cc.Void:
			classes["to void *: handed to a function of bytes"]++
		case fe != nil && fe.Kind() == cc.Void && isByte(te):
			classes["from void * to bytes"]++
		default:
			left = append(left, Finding{fn, x.Position().String(), fmt.Sprintf("(%s) of %s: %s", to, from, srcText(x.CastExpression))})
		}
	})
	return Result{"pointer casts", classes, left}
}
