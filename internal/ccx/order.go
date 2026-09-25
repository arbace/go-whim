package ccx

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// effects is what evaluating an expression does: the objects it writes and
// reads, named by their source (an lvalue's text), and the functions it calls.
type effects struct {
	writes, reads map[string]bool
	calls         []string
}

func newEffects() *effects { return &effects{writes: map[string]bool{}, reads: map[string]bool{}} }

func (e *effects) add(o *effects) {
	for k := range o.writes {
		e.writes[k] = true
	}
	for k := range o.reads {
		e.reads[k] = true
	}
	e.calls = append(e.calls, o.calls...)
}

// eff computes the effects of an expression, and reports every unsequenced
// pair of operands whose effects collide.
type orderCheck struct {
	fn      string
	classes map[string]int
	left    []Finding
}

func (c *orderCheck) pair(n cc.Node, ops []*effects, what string) {
	for i := 0; i < len(ops); i++ {
		for j := i + 1; j < len(ops); j++ {
			a, b := ops[i], ops[j]
			for k := range a.writes {
				if b.writes[k] || b.reads[k] {
					c.left = append(c.left, Finding{c.fn, n.Position().String(), fmt.Sprintf("%s: one operand writes %s and another uses it", what, k)})
				}
			}
			for k := range b.writes {
				if a.reads[k] {
					c.left = append(c.left, Finding{c.fn, n.Position().String(), fmt.Sprintf("%s: one operand writes %s and another reads it", what, k)})
				}
			}
			ia, ib := impure(a.calls), impure(b.calls)
			if len(ia) > 0 && len(ib) > 0 {
				c.left = append(c.left, Finding{c.fn, n.Position().String(), fmt.Sprintf("%s: calls on both sides, %v and %v", what, ia, ib)})
			} else if len(a.calls)+len(b.calls) > 0 {
				c.classes["unsequenced operands, at most one of them calling a function with effects"]++
			}
		}
	}
}

var pureSet map[string]bool

func impure(calls []string) []string {
	var r []string
	for _, c := range calls {
		if !pureSet[c] {
			r = append(r, c)
		}
	}
	return r
}

// lvalue names the object an lvalue expression designates, by its source.
func lvalue(e cc.ExpressionNode) string {
	return strings.Join(strings.Fields(srcText(unparen(e))), "")
}

func (c *orderCheck) eff(n cc.Node) *effects {
	e := newEffects()
	if n == nil {
		return e
	}
	switch x := n.(type) {
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			l, r := c.eff(x.UnaryExpression), c.eff(x.AssignmentExpression)
			// the operands' value computations are unsequenced with each other
			c.pair(x, []*effects{l, r}, "assignment")
			e.add(l)
			e.add(r)
			e.writes[lvalue(x.UnaryExpression)] = true
			return e
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
			e.add(c.eff(x.UnaryExpression))
			e.writes[lvalue(x.UnaryExpression)] = true
			return e
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			e.add(c.eff(x.PostfixExpression))
			e.writes[lvalue(x.PostfixExpression)] = true
			return e
		case cc.PostfixExpressionCall:
			ops := []*effects{c.eff(x.PostfixExpression)}
			for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
				ops = append(ops, c.eff(l.AssignmentExpression))
			}
			c.pair(x, ops, "call arguments")
			for _, o := range ops {
				e.add(o)
			}
			if name := callee(x); name != "" {
				e.calls = append(e.calls, name)
			} else {
				e.calls = append(e.calls, "(indirect)")
			}
			return e
		}
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			e.reads[x.Token.SrcStr()] = true
			return e
		}
	case *cc.LogicalAndExpression, *cc.LogicalOrExpression, *cc.ConditionalExpression, *cc.ExpressionList:
		// sequenced: each operand is a full expression of its own for this
		// check, and none races another
		for _, ch := range children(n) {
			e.add(c.eff(ch))
		}
		return e
	case *cc.AdditiveExpression, *cc.MultiplicativeExpression, *cc.ShiftExpression, *cc.RelationalExpression,
		*cc.EqualityExpression, *cc.AndExpression, *cc.ExclusiveOrExpression, *cc.InclusiveOrExpression:
		ch := children(n)
		var ops []*effects
		for _, k := range ch {
			ops = append(ops, c.eff(k))
		}
		if len(ops) == 2 {
			c.pair(n, ops, fmt.Sprintf("%T", n)[4:])
		}
		for _, o := range ops {
			e.add(o)
		}
		return e
	}
	for _, ch := range children(n) {
		e.add(c.eff(ch))
	}
	return e
}

// children are the direct node children of n.
func children(n cc.Node) []cc.Node {
	var r []cc.Node
	first := true
	walkDepth(n, func(m cc.Node) bool {
		if first {
			first = false
			return true
		}
		r = append(r, m)
		return false
	})
	return r
}

// Order finds every unsequenced pair of operands whose effects collide.
func Order(ast *cc.AST, p Profile) Result {
	pureSet = PureFuncs(ast, p.PureCalls)
	np := 0
	for _, v := range pureSet {
		if v {
			np++
		}
	}
	c := &orderCheck{classes: map[string]int{}}
	n := 0
	walk(ast.TranslationUnit, "", func(m cc.Node, fn string) {
		// each full expression once: the expression of a statement, a
		// condition, a return, an initializer
		var full cc.Node
		switch x := m.(type) {
		case *cc.ExpressionStatement:
			if x.ExpressionList != nil {
				full = x.ExpressionList
			}
		case *cc.SelectionStatement:
			full = x.ExpressionList
		case *cc.IterationStatement:
			full = x.ExpressionList
		case *cc.JumpStatement:
			if x.ExpressionList != nil {
				full = x.ExpressionList
			}
		case *cc.Initializer:
			if x.Case == cc.InitializerExpr && fn != "" {
				full = x.AssignmentExpression
			}
		}
		if full == nil {
			return
		}
		c.fn = fn
		c.eff(full)
		n++
	})
	c.classes[fmt.Sprintf("full expressions examined (%d); functions with no effect outside their frame: %d", n, np)] = 0
	sort.Slice(c.left, func(i, j int) bool { return c.left[i].Where < c.left[j].Where })
	return Result{"evaluation order", c.classes, c.left}
}
