// Package graphcut is step 4 of doc/GRAPH.md: two of the pipeline's cuts
// written as deletions on crefactor/graph's graph, their fall-out the
// graph's closure (Editor.FallOut), their sweep its collection, and held
// byte for byte to the snapshots the text versions made.  The plan does not
// run it; internal/cut and internal/phase/024 are the cuts the pipeline
// runs.  It is whim's: it names vim's fields, functions and shapes.
package graphcut

import (
	"fmt"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
)

// The get_varp shapes: a case that hands out a field's address.  Not a
// generic fall-out rule -- a `return &field;` cannot go without the case
// that reaches it, which is a choice only this cut makes -- so it is the
// cut's own Rule.  ?f is the field.
var (
	plVarp     = clisp.MustPattern("(cast (ptr char_u) (addr (paren (-> curbuf ?f))))")
	plVarpBoth = clisp.MustPattern("(? ?test (cast (ptr char_u) (addr (paren (-> curbuf ?f)))) (-> p var))")
)

// controlKeepCase is the control: set, the get_varp rule leaves the case
// label of a case it cuts -- C still, a label falling into the next case,
// and a change the byte comparison must catch.
var controlKeepCase bool

// plumbingTypes are the declared types DropLocal's declaration has.
var plumbingTypes = []*clisp.Node{
	clisp.MustPattern("(ptr char_u)"), clisp.A("int"), clisp.A("long"),
}

// getVarp is the cut's rule for field: a get_varp case returning its
// address goes, with the case label before it.
func getVarp(field string) graph.Rule {
	return graph.Rule{Name: "get_varp", Take: func(e *graph.Editor, d graph.Dangling) ([]*graph.Node, error) {
		it := e.Item(d.Use)
		if it == nil || !it.Is("return") || len(it.Kids) != 2 {
			return nil, nil
		}
		v := it.Kids[1]
		b, ok := graph.Match(plVarp, v)
		if !ok || b["f"].Atom != field {
			b, ok = graph.Match(plVarpBoth, v)
			if !ok || b["f"].Atom != field ||
				!graph.Contains(b["test"], clisp.L(clisp.A("->"), clisp.A("curbuf"), clisp.A(field))) {
				return nil, nil
			}
		}
		prev := e.Sibling(it, -1)
		if prev == nil || !prev.Is("case") {
			return nil, nil
		}
		if controlKeepCase {
			return []*graph.Node{it}, nil
		}
		return []*graph.Node{prev, it}, nil
	}}
}

// DropLocal is internal/cut's DropLocal on the graph: it DELETES a
// buffer-local option field -- its declaration -- and the fall-out closure
// takes its plumbing: the stores to it (generic), the checks and frees of
// it (generic, told which functions act on their argument alone), and the
// get_varp case that hands its address out (the cut's own rule).  It
// refuses as the text version does: on no field, on fewer than three sites,
// and on a use no rule takes -- a reader, which the phase must deal with
// before the field can go.  It reports the sites as the text does: the
// declaration and each statement, a get_varp case one.
func DropLocal(e *graph.Editor, field string, opt graph.FallOutOptions) (int, error) {
	var decls []*graph.Node
	for _, f := range e.Graph().Forms {
		graph.Walk(f, func(n *graph.Node) bool {
			if (n.Is("struct") || n.Is("union")) && len(graph.Members(n)) > 0 {
				for _, m := range graph.Members(n) {
					if declares(m, field) {
						decls = append(decls, m)
					}
				}
			}
			return true
		})
		if f.Is("def") && len(f.Kids) == 3 && !f.Kids[1].IsList() && f.Kids[1].Atom == field && plumbing(f.Kids[2]) {
			decls = append(decls, f)
		}
	}
	if len(decls) == 0 {
		return 0, fmt.Errorf("droplocal: there is no %s here", field)
	}
	for _, d := range decls {
		if err := e.Delete(d); err != nil {
			return 0, fmt.Errorf("droplocal: %s: %w", field, err)
		}
	}
	opt.Rules = append([]graph.Rule{getVarp(field)}, opt.Rules...)
	st, err := e.FallOut(opt)
	if u, ok := err.(*graph.Unhandled); ok {
		return 0, fmt.Errorf("droplocal: %s still has %d mentions after the plumbing "+
			"went -- those are readers, and the phase has to deal with them before the "+
			"field can go (%v)", field, u.Left, u)
	}
	if err != nil {
		return 0, fmt.Errorf("droplocal: %s: %w", field, err)
	}
	n := len(decls)
	for _, r := range st.Removed {
		if !r.Item.Is("case") {
			n++
		}
	}
	if n < 3 {
		return 0, fmt.Errorf("droplocal: %s: only %d plumbing sites, expected at "+
			"least the field, an initialiser and a get_varp case -- the shape has moved",
			field, n)
	}
	return n, nil
}

// declares says m is the field's member as DropLocal's declaration shape
// has it: `char_u *F;`, `int F;` or `long F;`.
func declares(m *graph.Node, field string) bool {
	return len(m.Kids) == 2 && !m.Kids[0].IsList() && m.Kids[0].Atom == field && plumbing(m.Kids[1])
}

func plumbing(t *graph.Node) bool {
	for _, p := range plumbingTypes {
		if graph.Matches(p, t) {
			return true
		}
	}
	return false
}
