package togo

// cquery.go is small questions about the C that more than one backend asks
// -- the Haskell, the Scheme, the Rust, the Java, the Clojure -- kept apart
// from any one backend's printing, so that changing a printer cannot move
// another's output.

import (
	"fmt"
	"regexp"

	"github.com/arbace/go-whim/crefactor/cc"
)

// fnDesignator is the function e names, or nil.
func fnDesignator(e cc.ExpressionNode) *cc.Declarator {
	if p, ok := unparenE(e).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
			return d
		}
	}
	return nil
}

// b2i64 is a truth as C's int.
func b2i64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// sliceSet is the variables vs as a set.
func sliceSet(vs []*lvar) map[*lvar]bool {
	m := map[*lvar]bool{}
	for _, v := range vs {
		m[v] = true
	}
	return m
}

// elemSize is the size of what a pointer or array points at: 1 for void,
// as gcc has it.
func elemSize(t cc.Type) int64 {
	e := elemOf(t)
	if e == nil || e.Kind() == cc.Void || e.Kind() == cc.Function {
		return 1
	}
	return e.Size()
}

// layoutMarkRe is a runtime body's question about the C's layout: {{sizeof
// T}} or {{offsetof T m}}, T a typedef's name.
var layoutMarkRe = regexp.MustCompile(`\{\{(sizeof|offsetof) ([A-Za-z_][A-Za-z_0-9]*)(?: ([A-Za-z_][A-Za-z_0-9]*))?\}\}`)

// layoutMarks answers a runtime body's questions about the C's layout from
// the front end's: a body is written once, and the sizes are the unit's.
func (r *rgen) layoutMarks(body string) string { return answerLayout(r.g.ast, body) }

// answerLayout answers a body's {{sizeof T}} and {{offsetof T m}} from the
// front end's layout of ast.
func answerLayout(ast *cc.AST, body string) string {
	return layoutMarkRe.ReplaceAllStringFunc(body, func(m string) string {
		g := layoutMarkRe.FindStringSubmatch(m)
		var t cc.Type
		for _, n := range ast.Scope.Nodes[g[2]] {
			if d, ok := n.(*cc.Declarator); ok && d.IsTypename() {
				t = d.Type()
			}
		}
		if t == nil {
			panic(unsupported{"a runtime body's " + m + ": no type " + g[2]})
		}
		if g[1] == "sizeof" {
			return fmt.Sprint(t.Size())
		}
		var st *cc.StructType
		switch x := t.(type) {
		case *cc.StructType:
			st = x
		}
		if st == nil {
			panic(unsupported{"a runtime body's " + m + ": " + g[2] + " is not a struct"})
		}
		fl := st.FieldByName(g[3])
		if fl == nil {
			panic(unsupported{"a runtime body's " + m + ": no member " + g[3]})
		}
		return fmt.Sprint(fl.Offset())
	})
}
