package cut

import (
	"fmt"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
)

// DropLocal removes a buffer-local option field and its PLUMBING -- the
// declaration, the assignments, the checks and frees, and the get_varp case
// that hands its address out -- and refuses when anything else still names
// it.  It is a cut on the graph (crefactor/graph, doc/GRAPH.md step 4): it
// DELETES the field's declaration, and the editor's fall-out closure takes
// the plumbing -- the stores to the field (generic), the checks and frees
// of it (generic, told which functions act on their argument alone:
// whim.GraphFallOut), and the get_varp case (this cut's own rule).
//
// The refusal is the point.  Plumbing is what the phase may remove on its own;
// a use no rule takes is a READER, and a reader has to be dealt with by the
// phase before the field can go.  It also refuses when it found fewer than
// three sites, because the field, an initialiser and a get_varp case are the
// minimum shape -- fewer means the shape has moved and a silent partial cut
// would leave the struct and its users disagreeing.  It reports the sites as
// the text version it replaced did: the declaration and each statement, a
// get_varp case one.
func DropLocal(e *graph.Editor, field string, opt graph.FallOutOptions) (int, error) {
	var decls []*graph.Node
	for _, f := range e.Graph().Forms {
		graph.Walk(f, func(n *graph.Node) bool {
			if (n.Is("struct") || n.Is("union")) && len(graph.Members(n)) > 0 {
				for _, m := range graph.Members(n) {
					if declaresField(m, field) {
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

// The get_varp shapes: a case that hands out a field's address.  Not a
// generic fall-out rule -- a `return &field;` cannot go without the case
// that reaches it, which is a choice only this cut makes -- so it is the
// cut's own Rule.  ?f is the field.
//
// Each comes with its field's address spelled `&(curbuf->f)` or
// `&curbuf->f`: get_varp() writes the cases of 'equalprg' and
// 'keywordprg' without the parentheses, which phases 18 and 19 once took
// by a program of their own (whim18kp, whim19ep) before their droplocal
// could run.
var (
	plVarp = []*clisp.Node{
		clisp.MustPattern("(cast (ptr char_u) (addr (paren (-> curbuf ?f))))"),
		clisp.MustPattern("(cast (ptr char_u) (addr (-> curbuf ?f)))"),
	}
	plVarpBoth = []*clisp.Node{
		clisp.MustPattern("(? ?test (cast (ptr char_u) (addr (paren (-> curbuf ?f)))) (-> p var))"),
		clisp.MustPattern("(? ?test (cast (ptr char_u) (addr (-> curbuf ?f))) (-> p var))"),
	}
)

// matchAny is the bindings of the first of ps that matches n.
func matchAny(ps []*clisp.Node, n *graph.Node) (graph.Bindings, bool) {
	for _, p := range ps {
		if b, ok := graph.Match(p, n); ok {
			return b, true
		}
	}
	return nil, false
}

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
		b, ok := matchAny(plVarp, v)
		if !ok || b["f"].Atom != field {
			b, ok = matchAny(plVarpBoth, v)
			if !ok || b["f"].Atom != field ||
				!graph.Contains(b["test"], clisp.L(clisp.A("->"), clisp.A("curbuf"), clisp.A(field))) {
				return nil, nil
			}
		}
		prev := e.Sibling(it, -1)
		if prev == nil || !prev.Is("case") {
			return nil, nil
		}
		return []*graph.Node{prev, it}, nil
	}}
}

// declaresField says m is the field's member as DropLocal's declaration
// shape has it: `char_u *F;`, `int F;` or `long F;`.
func declaresField(m *graph.Node, field string) bool {
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
