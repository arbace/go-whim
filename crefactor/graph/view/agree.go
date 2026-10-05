package view

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
)

// AGREEMENT.  crefactor/cc's check, which FRAG runs, resolves every name
// and types every expression, but it does not refuse what C's constraints
// on simple assignment refuse and a compiler reports: an integer stored in
// a pointer, a pointer in an integer, one struct in another or in a
// scalar.  An edit through a view is checked for them where it wrote:
// every assignment and initialised declaration among the nodes it made,
// its value's type against its target's, on the typed edges FRAG gave
// them.  Pointers are not told apart by what they point at (`void *` and
// the qualifiers make that a compiler's warning, not this refusal), and a
// type it cannot classify -- a header's -- is not judged.

// tclass is what an assignment's rule asks of a type: arith, bool,
// pointer, struct (with the struct), void, or "" when it cannot say.
type tclass struct {
	kind string
	agg  *graph.Node
}

func classOf(t *graph.Node) tclass {
	switch {
	case t == nil:
		return tclass{}
	case t.Is("basic"):
		words := ""
		for _, k := range t.Args() {
			words += " " + k.Atom
		}
		switch {
		case strings.Contains(words, "void"):
			return tclass{kind: "void"}
		case strings.Contains(words, "bool"):
			return tclass{kind: "bool"}
		}
		return tclass{kind: "arith"}
	case t.Is("pointer") || t.Is("array") || t.Is("function"):
		return tclass{kind: "pointer"}
	case t.Is("enum"):
		return tclass{kind: "arith"}
	case t.Is("struct") || t.Is("union"):
		return tclass{kind: "struct", agg: t}
	}
	return tclass{}
}

// valueClass is an expression's class, and whether it is a null pointer
// constant (0, nullptr).
func valueClass(x *graph.Node) (tclass, bool) {
	if x.IsList() {
		if x.Is("paren") && len(x.Kids) == 2 {
			return valueClass(x.Kids[1])
		}
		return classOf(x.Type), false
	}
	switch a := x.Atom; {
	case a == "0" || a == "nullptr":
		return tclass{kind: "arith"}, true
	case len(a) > 0 && (a[0] >= '0' && a[0] <= '9' || a[0] == '\'' || a[0] == '.'):
		return tclass{kind: "arith"}, false
	case strings.HasSuffix(a, "\"") && strings.Contains(a, "\""):
		return tclass{kind: "pointer"}, false
	}
	if d := x.Ref(); d != nil {
		return classOf(d.Type), false
	}
	return tclass{}, false
}

func (c tclass) String() string {
	if c.kind == "struct" {
		return fmt.Sprintf("%s %s", c.agg.Head(), graph.Tag(c.agg))
	}
	return c.kind
}

// agrees is the first assignment C's rules refuse among the nodes made
// and the forms above them, to their statement: an expression made is
// stored by what holds it.
func agrees(e *graph.Editor, made [][]*graph.Node) error {
	var err error
	seen := map[*graph.Node]bool{}
	for _, ns := range made {
		for _, n := range ns {
			var check []*graph.Node
			graph.Walk(n, func(x *graph.Node) bool { check = append(check, x); return true })
			for q := e.Parent(n); q != nil && !graph.IsStatement(n) && !seen[q]; q = e.Parent(q) {
				check = append(check, q)
				if graph.IsStatement(q) || e.Parent(q) != nil && e.Parent(q).Is("block") {
					break
				}
			}
			for _, x := range check {
				if err != nil || seen[x] {
					break
				}
				seen[x] = true
				var to tclass
				var v *graph.Node
				switch {
				case x.Is("=") && len(x.Kids) == 3:
					l, _ := valueClass(x.Kids[1])
					to, v = l, x.Kids[2]
				case x.Is("def") && x.Type != nil:
					to, v = classOf(x.Type), x.Kids[len(x.Kids)-1]
					if v == graph.DeclType(x) || v.Is("init") || graph.DeclAtom(x) == v {
						continue // no initialiser, or a list of them
					}
				default:
					continue
				}
				from, null := valueClass(v)
				if why := assignable(to, from, null); why != "" {
					c, _ := clisp.PrintExpr(graph.Lisp(x))
					if x.Is("def") {
						c, _ = clisp.PrintItems([]*clisp.Node{graph.Lisp(x)})
					}
					err = refuse("a type that does not agree: `%s` %s", strings.TrimSpace(c), why)
				}
			}
		}
	}
	return err
}

// assignable is why a value of class from cannot be stored in to, or "".
func assignable(to, from tclass, null bool) string {
	if to.kind == "" || from.kind == "" {
		return ""
	}
	bad := func() string { return fmt.Sprintf("stores a value of %s in %s", from, to) }
	switch to.kind {
	case "arith":
		if from.kind != "arith" && from.kind != "bool" {
			return bad()
		}
	case "bool":
		if from.kind == "struct" || from.kind == "void" {
			return bad()
		}
	case "pointer":
		if from.kind == "pointer" || null {
			return ""
		}
		return bad()
	case "struct":
		if from.kind != "struct" || from.agg != to.agg {
			return bad()
		}
	}
	return ""
}
