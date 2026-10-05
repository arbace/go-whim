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
// scalar -- nor a call with more arguments or fewer than its prototype
// says, nor a return with a value from a void function or without one from
// another.  An edit through a view is checked for them where it wrote:
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
	var order map[*graph.Node]int // the file's forms by place, made when a call asks
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
				case x.Is("call") && len(x.Kids) >= 2:
					if why := callAgrees(x); why != "" {
						c, _ := clisp.PrintExpr(graph.Lisp(x))
						err = refuse("a call that does not agree: `%s` %s", strings.TrimSpace(c), why)
					} else if why := declaredBefore(e, x, &order); why != "" {
						err = refuse("%s", why)
					}
					continue
				case x.Is("return"):
					if why := returnAgrees(e, x); why != "" {
						c, _ := clisp.PrintItems([]*clisp.Node{graph.Lisp(x)})
						err = refuse("a return that does not agree: `%s` %s", strings.TrimSpace(c), why)
					}
					continue
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

// fnType is the function type a call's callee has (through a pointer to
// one), or nil.
func fnType(callee *graph.Node) *graph.Node {
	t := callee.Type
	if callee.IsList() && callee.Is("paren") && len(callee.Kids) == 2 {
		return fnType(callee.Kids[1])
	}
	if t == nil && !callee.IsList() {
		if d := callee.Ref(); d != nil {
			t = d.Type // a name: its declaration's type
		}
	}
	if t != nil && t.Is("pointer") {
		t = operandOf(t)
	}
	if t == nil || !t.Is("function") || len(t.Kids) < 3 {
		return nil
	}
	return t
}

// callAgrees is why a call's arguments do not agree with its callee's
// prototype -- their count, or one's class against its parameter's, by
// simple assignment's rules -- or "".
func callAgrees(c *graph.Node) string {
	t := fnType(c.Kids[1])
	if t == nil {
		return ""
	}
	var params []*graph.Node
	variadic := false
	for _, p := range t.Kids[1].Kids {
		if !p.IsList() && p.Atom == "..." {
			variadic = true
			continue
		}
		params = append(params, p)
	}
	if len(params) == 1 && classOf(params[0].Type).kind == "void" {
		params = nil // (void): no parameters
	}
	args := c.Kids[2:]
	switch {
	case len(args) < len(params):
		return fmt.Sprintf("passes %s where the prototype takes %d", plural(len(args), "argument"), len(params))
	case len(args) > len(params) && !variadic:
		return fmt.Sprintf("passes %s where the prototype takes %d", plural(len(args), "argument"), len(params))
	}
	for i, p := range params {
		from, null := valueClass(args[i])
		if why := assignable(classOf(p.Type), from, null); why != "" {
			return fmt.Sprintf("argument %d %s", i+1, why)
		}
	}
	return ""
}

// returnAgrees is why a return does not agree with its function's result
// -- a value from a void function, none from another, or a value its
// result cannot hold -- or "".
func returnAgrees(e *graph.Editor, r *graph.Node) string {
	f := e.Function(r)
	if f == nil {
		return ""
	}
	t := f.Type // a definition's typed edge: its function type
	if t == nil || !t.Is("function") || len(t.Kids) < 3 {
		return ""
	}
	res := classOf(t.Kids[2].Type)
	switch {
	case len(r.Kids) < 2 && res.kind != "void" && res.kind != "":
		return fmt.Sprintf("returns no value from a function returning %s", res)
	case len(r.Kids) >= 2 && res.kind == "void":
		return "returns a value from a function returning void"
	case len(r.Kids) >= 2:
		from, null := valueClass(r.Kids[1])
		if why := assignable(res, from, null); why != "" {
			return why
		}
	}
	return ""
}

// callersAgree is the first call refused among the calls of the functions
// an edit declared or defined: a function's parameters changed leaves its
// callers', which the edit did not write, to agree with them.
func callersAgree(e *graph.Editor, made [][]*graph.Node) error {
	for _, ns := range made {
		for _, n := range ns {
			if !(n.Is("defn") || n.Is("def")) || n.Type == nil || !n.Type.Is("function") || !e.Live(n) {
				continue
			}
			for _, u := range e.Uses(n) {
				c := e.Parent(u)
				if c == nil || !c.Is("call") || len(c.Kids) < 2 || c.Kids[1] != u {
					continue
				}
				if why := callAgrees(c); why != "" {
					txt, _ := clisp.PrintExpr(graph.Lisp(c))
					in := ""
					if f := e.Function(c); f != nil {
						in = " in " + graph.DeclName(f)
					}
					return refuse("a call that does not agree with %s as edited: `%s`%s %s", graph.DeclName(n), strings.TrimSpace(txt), in, why)
				}
			}
		}
	}
	return nil
}

// declaredBefore is why a call names a function the file declares only
// after the top-level form the call is in, or "": cc's check resolves the
// call to the later declaration, and C23, which has no implicit
// declarations, refuses it (gcc: implicit declaration, then conflicting
// types) -- a function typed beside a view's and called from it.
func declaredBefore(e *graph.Editor, call *graph.Node, order *map[*graph.Node]int) string {
	callee := call.Kids[1]
	if callee.IsList() {
		return ""
	}
	d := callee.Ref()
	if d == nil || !(d.Is("defn") || d.Is("def")) {
		return ""
	}
	name := graph.DeclName(d)
	in := e.Function(call)
	if name == "" || in == nil {
		return ""
	}
	if *order == nil {
		*order = map[*graph.Node]int{}
		for i, f := range e.Graph().Forms {
			(*order)[f] = i
		}
	}
	at, ok := (*order)[in]
	if !ok {
		return ""
	}
	for _, f := range e.FileDecls(name) {
		if k, ok := (*order)[f]; ok && k <= at {
			return "" // declared before, or the calling function itself
		}
	}
	return fmt.Sprintf("%s is declared only after %s, which calls it: C needs a declaration first -- define it above, or declare it there", name, graph.DeclName(in))
}
