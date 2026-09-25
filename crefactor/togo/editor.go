package togo

import (
	"fmt"
	"github.com/arbace/go-whim/crefactor/cc"
	"go/format"
	"os"
	"strings"
)

// writeEditor writes editor.go whole, and refuses if any function or initial
// value has no rule.
func (g *gen) writeEditor(path string, skip map[string]bool) error {
	body, report := g.bodies(skip)
	if report != "" {
		return fmt.Errorf("functions the emitter cannot write:\n%s", report)
	}
	inits, failed := g.initializers()
	if len(failed) > 0 {
		return fmt.Errorf("initial values the emitter cannot write:\n%s", strings.Join(failed, "\n"))
	}
	var b strings.Builder
	b.WriteString(g.p.Header)
	b.WriteString(strings.TrimPrefix(g.typesText(), g.p.pkg()))
	b.WriteString("\n")
	b.WriteString(strings.TrimPrefix(g.globalsText(), g.p.pkg()))
	b.WriteString("\n// The initial values of the file-scope objects and the hoisted statics whose\n// C initializer is not all zeros.\nfunc init() {\n")
	b.WriteString(inits)
	b.WriteString("}\n\n")
	for _, rb := range g.p.RuntimeBodies {
		b.WriteString(g.runtimeBody(rb) + "\n")
	}
	b.WriteString(body)
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return fmt.Errorf("the generated file is not Go: %v", err)
	}
	return os.WriteFile(path, src, 0o644)
}

// runtimeBody is a function whose body is the runtime's rule, for the result
// type the C gives it: the rule is the runtime's; the signature is the C's.
func (g *gen) runtimeBody(rb RuntimeBody) string {
	for _, n := range g.ast.Scope.Nodes[rb.Name] {
		d, ok := n.(*cc.Declarator)
		if !ok {
			continue
		}
		ft, ok := d.Type().(*cc.FunctionType)
		if !ok {
			continue
		}
		return rb.Body(g.goType(ft.Result(), "ret:"+rb.Name))
	}
	return rb.Body("")
}
