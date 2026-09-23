package ccx

import (
	"fmt"

	"github.com/arbace/go-whim/internal/cc"
)

// Go's goto may not jump into a block, and may not jump over a variable's
// declaration.  The second an emitter settles by declaring a function's locals
// at its top, as C89 did; the first it cannot settle without restructuring.
// Gotos partitions every goto by where its label is: in a block the goto is
// also in, or not.
func Gotos(ast *cc.AST) Result {
	res := Result{Title: "gotos", Classes: map[string]int{}}
	type site struct {
		fn, label, where string
		chain            []*cc.CompoundStatement
	}
	var gotos []site
	labels := map[string][]*cc.CompoundStatement{}
	var visit func(n cc.Node, fn string, chain []*cc.CompoundStatement)
	visit = func(n cc.Node, fn string, chain []*cc.CompoundStatement) {
		switch x := n.(type) {
		case *cc.FunctionDefinition:
			fn = x.Declarator.Name()
		case *cc.CompoundStatement:
			chain = append(append([]*cc.CompoundStatement{}, chain...), x)
		case *cc.LabeledStatement:
			if x.Case == cc.LabeledStatementLabel {
				labels[fn+":"+x.Token.SrcStr()] = chain
			}
		case *cc.JumpStatement:
			if x.Case == cc.JumpStatementGoto {
				gotos = append(gotos, site{fn, x.Token2.SrcStr(), x.Position().String(), chain})
			}
		}
		for _, c := range children(n) {
			visit(c, fn, chain)
		}
	}
	visit(ast.TranslationUnit, "", nil)
	for _, g := range gotos {
		lc, ok := labels[g.fn+":"+g.label]
		if !ok {
			res.Left = append(res.Left, Finding{g.fn, g.where, "goto " + g.label + ": no such label"})
			continue
		}
		in := map[*cc.CompoundStatement]bool{}
		for _, b := range g.chain {
			in[b] = true
		}
		into := false
		for _, b := range lc {
			into = into || !in[b]
		}
		if into {
			res.Left = append(res.Left, Finding{g.fn, g.where, fmt.Sprintf("goto %s jumps into a block", g.label)})
			continue
		}
		res.Classes["a goto to a label in a block it is in"]++
	}
	return res
}
