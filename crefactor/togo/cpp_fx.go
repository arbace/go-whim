package togo

// cpp_fx.go is what the C++ backend decides about the core's functions as
// members (doc/CPP-IDIOMS.md): which are static -- they reach no field of
// the editor, no host function and no function pointer, closed over their
// calls (effects.go, the Rust's and the Scheme's) -- which are
// [[nodiscard]] -- every call uses the result -- and which are private.

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// memberFacts decides c.static and c.nodiscard.
func (c *cppgen) memberFacts() {
	exports := append([]string(nil), c.g.p.CppExports...)
	for n := range c.g.a.addr {
		exports = append(exports, n) // a pointer to a member: not static
	}
	var bodies []handBody
	for _, rb := range c.g.p.RuntimeBodies {
		if rb.Cpp != nil {
			bodies = append(bodies, handBody{rb.Name, rb.Cpp("void")})
		}
	}
	facts := newCFacts(c.g, cfactsOptions{exports: exports, bodies: bodies, noOuts: true, noStructValues: true})
	c.static = map[string]bool{}
	for name := range c.defined {
		if !facts.takesEd(name) && !c.g.a.addr[name] {
			c.static[name] = true
		}
	}
	// a result some call drops is not [[nodiscard]]; nor the hand-written
	// code's, which this does not see
	dropped := map[string]bool{}
	for _, n := range c.g.p.CppExports {
		dropped[n] = true
	}
	var rec func(n cc.Node)
	dropCall := func(e cc.ExpressionNode) {
		if call, ok := strip(e).(*cc.PostfixExpression); ok && call.Case == cc.PostfixExpressionCall {
			if d := fnDesig(call.PostfixExpression); d != nil {
				dropped[d.Name()] = true
			}
		}
	}
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.ExpressionStatement:
			if x.ExpressionList != nil {
				for l, ok := x.ExpressionList.(*cc.ExpressionList); ok && l != nil; l = l.ExpressionList {
					dropCall(l.AssignmentExpression)
				}
				dropCall(x.ExpressionList)
			}
		case *cc.ExpressionList:
			// a comma's left operands
			for l := x; l != nil && l.ExpressionList != nil; l = l.ExpressionList {
				dropCall(l.AssignmentExpression)
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionCast && x.TypeName.Type() != nil && x.TypeName.Type().Kind() == cc.Void {
				dropCall(x.CastExpression)
			}
		case *cc.IterationStatement:
			if x.Case == cc.IterationStatementFor {
				dropCall(x.ExpressionList)
				dropCall(x.ExpressionList3)
			}
			if x.Case == cc.IterationStatementForDecl {
				dropCall(x.ExpressionList2)
			}
		}
		walkChildrenFn(n, rec)
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		rec(tu.ExternalDeclaration)
	}
	c.nodiscard = map[string]bool{}
	for name, fd := range c.defined {
		ft, ok := fd.Declarator.Type().(*cc.FunctionType)
		if ok && ft.Result() != nil && ft.Result().Kind() != cc.Void && !dropped[name] {
			c.nodiscard[name] = true
		}
	}
}
