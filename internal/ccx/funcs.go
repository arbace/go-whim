package ccx

import (
	"fmt"

	"github.com/arbace/go-whim/internal/cc"
)

// Go's func values compare only with nil.  FuncCompares partitions every ==
// and != whose operands are function pointers: against a null, or not.
func FuncCompares(ast *cc.AST) Result {
	res := Result{Title: "function pointer comparisons", Classes: map[string]int{}}
	isNull := func(e cc.ExpressionNode) bool {
		if v := e.Value(); v != nil && fmt.Sprint(v) == "0" {
			return true
		}
		if c, ok := unparen(e).(*cc.CastExpression); ok && c.Case == cc.CastExpressionCast {
			if v := c.CastExpression.Value(); v != nil && fmt.Sprint(v) == "0" {
				return true
			}
		}
		return false
	}
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		x, ok := n.(*cc.EqualityExpression)
		if !ok || (x.Case != cc.EqualityExpressionEq && x.Case != cc.EqualityExpressionNeq) {
			return
		}
		fp := false
		for _, o := range []cc.ExpressionNode{x.EqualityExpression, x.RelationalExpression} {
			if p, ok := o.Type().(*cc.PointerType); ok && p.Elem().Kind() == cc.Function {
				fp = true
			}
		}
		if !fp {
			return
		}
		if isNull(x.EqualityExpression) || isNull(x.RelationalExpression) {
			res.Classes["a function pointer tested against null"]++
			return
		}
		res.Left = append(res.Left, Finding{fn, x.Position().String(), "two function pointers compared: " + srcOrdered(x)})
	})
	return res
}
