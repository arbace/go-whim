package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// THE NAMED VIEWS: specs a reader asks for by name, each a relation of the
// steps above (but type and def, which are shapes of their own).

// Named is every named view, as `whim view` lists them.
var Named = []string{"callers", "callees", "uses", "member", "type", "def"}

// The relations the named views are, as specs a user could write.
const (
	CallersSteps = "refers< call ^fn"
	CalleesSteps = "inside call refers"
	UsesSteps    = "refers< ^fn"
)

func mustSteps(s string) Relation {
	st, err := ParseSteps(s)
	if err != nil {
		panic(err)
	}
	return Steps(st)
}

// Options are what a named view is asked with.
type Options struct {
	Depth int  // the levels of children; 0 is the view's own default
	Show  Show // ShowStmt by default
	Stop  []string
}

func (o Options) depth(d int) int {
	if o.Depth > 0 {
		return o.Depth
	}
	if o.Depth < 0 {
		return 0 // no limit
	}
	return d
}

// Callers is who calls f: each function holding a call of it, with the
// calling statements, and who calls that, to the depth (2 by default).  A
// function the view holds already is a link: recursion ends there.
func Callers(ix *Index, f *graph.Node, o Options) *Tree {
	t := Build(ix, f, Spec{Name: "callers", Child: "in", Rel: mustSteps(CallersSteps), Depth: o.depth(2), Stop: o.Stop, Show: o.Show})
	calls, other := 0, 0
	for _, d := range ix.Decls(t.Node) {
		for _, u := range ix.Uses(d) {
			if ix.InCall(u) {
				calls++
			} else {
				other++
			}
		}
	}
	t.Notes = append(t.Notes, fmt.Sprintf("%s of %s in %s", plural(calls, "call"), ix.Name(t.Node), plural(len(t.Kids), "function")))
	if other > 0 {
		t.Notes = append(t.Notes, fmt.Sprintf("and %s not in a call, which `uses %s` shows", plural(other, "use"), ix.Name(t.Node)))
	}
	return t
}

// Callees is what f calls: each function a call in its body names, with
// the calling statements, and what that calls, to the depth (2 by
// default).  A call through a pointer is to the pointer: the parameter,
// the member, the object.
func Callees(ix *Index, f *graph.Node, o Options) *Tree {
	t := Build(ix, f, Spec{Name: "callees", Child: "calls", Rel: mustSteps(CalleesSteps), Depth: o.depth(2), Stop: o.Stop, Show: o.Show})
	t.Notes = append(t.Notes, fmt.Sprintf("%s calls %s", ix.Name(t.Node), plural(len(t.Kids), "function")))
	return t
}

// Uses is every use of a declaration -- an object, a function, an
// enumerator, a typedef, a parameter or local, a member -- grouped by the
// top-level form holding it, each with its statement and what the use is
// (Access).  Deeper than 1 (its default), the uses of each holder.
func Uses(ix *Index, n *graph.Node, o Options) *Tree {
	t := Build(ix, n, Spec{Name: "uses", Child: "in", Rel: mustSteps(UsesSteps), Depth: o.depth(1), Stop: o.Stop, Show: o.Show, Label: accessLabel})
	t.Notes = append(t.Notes, ix.Tally(t.Node, len(t.Kids)))
	return t
}

// Member is Uses of a struct's member, resolved by type: every read and
// write of it.
func Member(ix *Index, m *graph.Node, o Options) *Tree {
	t := Uses(ix, m, o)
	t.Head = "member"
	return t
}

// Tally says how many uses of n there are, in how many forms, and of what
// access.
func (ix *Index) Tally(n *graph.Node, holders int) string {
	counts := map[string]int{}
	total := 0
	for _, d := range ix.Decls(n) {
		for _, u := range ix.Uses(d) {
			counts[ix.Access(u)]++
			total++
		}
	}
	return fmt.Sprintf("%s of %s in %s: %s", plural(total, "use"), ix.Name(n), plural(holders, "top-level form"), tallyText(counts))
}

func tallyText(counts map[string]int) string {
	var ks []string
	for k := range counts {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return accessOrder(ks[i]) < accessOrder(ks[j]) })
	var parts []string
	for _, k := range ks {
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// The accesses, in the order a label lists them.
var accesses = []string{"call", "read", "write", "update", "addr", "init", "type", "unevaluated", "label"}

func accessOrder(a string) int {
	for i, x := range accesses {
		if x == a {
			return i
		}
	}
	return len(accesses)
}

// accessLabel is a context's label: the accesses of its uses, `+` between.
func accessLabel(ix *Index, uses []*graph.Node) string {
	seen := map[string]bool{}
	var ks []string
	for _, u := range uses {
		if a := ix.Access(u); !seen[a] {
			seen[a] = true
			ks = append(ks, a)
		}
	}
	sort.Slice(ks, func(i, j int) bool { return accessOrder(ks[i]) < accessOrder(ks[j]) })
	return strings.Join(ks, "+")
}

// Access is what a use does with what it names, read off the forms around
// it -- C's syntax, not an analysis of what a pointer reaches:
//
//	call         what a call calls
//	read         its value is taken
//	write        it is assigned: `(= USE ...)`, a member of it, an element
//	             of it when it is an array
//	update       read and written: `+=` and its kin, `++`, `--`
//	addr         its address is taken: what is done through it is not seen
//	init         a designator in an initialiser: `.m = ...`
//	type         a typedef name, a tag: a type is spelled
//	unevaluated  in sizeof, alignof or typeof
//	label        a goto's label
func (ix *Index) Access(u *graph.Node) string {
	if r := u.Ref(); r != nil {
		switch {
		case r.Is("typedef") || r.Is("extern-typedef") || graph.IsTypeDef(r) ||
			strings.HasPrefix(r.Head(), "extern-struct") || r.Is("extern-union") || r.Is("extern-enum"):
			return "type"
		case r.Is("label"):
			return "label"
		}
	}
	if ix.InCall(u) {
		return "call"
	}
	p := ix.Parent(u)
	if p == nil {
		return "read"
	}
	e := u
	switch {
	case p.Is("at"):
		return "init"
	case p.Is("->") || p.Is("."):
		if p.Kids[len(p.Kids)-1] != u && p.Is("->") {
			return "read" // p->a->b: a is read
		}
		e = p
	}
	for {
		q := ix.Parent(e)
		if q == nil {
			return "read"
		}
		first := len(q.Kids) > 1 && q.Kids[1] == e
		switch h := q.Head(); {
		case h == "paren":
		case h == "." && first:
		case h == "index" && first && isArray(typeOf(e)):
		case h == "=" && first:
			return "write"
		case first && isCompound(h), h == "post++", h == "post--", h == "pre++", h == "pre--":
			return "update"
		case h == "addr":
			return "addr"
		case h == "sizeof" || h == "sizeof-bare" || h == "alignof" || h == "alignof-bare" ||
			h == "typeof" || h == "typeof_unqual" || h == "__typeof__" || h == "__typeof":
			return "unevaluated"
		default:
			return "read"
		}
		e = q
	}
}

func isCompound(h string) bool {
	switch h {
	case "+=", "-=", "*=", "/=", "%=", "<<=", ">>=", "&=", "^=", "|=":
		return true
	}
	return false
}

// typeOf is a form's type node: its typed edge, or an atom's declaration's.
func typeOf(e *graph.Node) *graph.Node {
	if e.Type != nil {
		return e.Type
	}
	if r := e.Ref(); r != nil {
		return r.Type
	}
	return nil
}

func isArray(t *graph.Node) bool { return t != nil && t.Is("array") }

// operandOf is a pointer's or an array's element type node.
func operandOf(t *graph.Node) *graph.Node {
	if len(t.Kids) == 0 {
		return nil
	}
	return t.Kids[len(t.Kids)-1].Type
}

// reaches says type node t is s, or a pointer or array of it, at any depth.
func reaches(t, s *graph.Node) bool {
	for i := 0; t != nil && i < 64; i++ {
		if t == s {
			return true
		}
		if !t.Is("pointer") && !t.Is("array") {
			return false
		}
		t = operandOf(t)
	}
	return false
}

// Type is a struct, union or enum: its members (or enumerators) with how
// they are used, the functions taking it (by value or through pointers)
// and those returning it, the other structs' members of its type and the
// file's objects of it.
func Type(ix *Index, s *graph.Node, o Options) *Tree {
	t := &Tree{Head: "type", Node: s}
	label := "members"
	if s.Is("enum") || s.Is("extern-enum") {
		label = "enumerators"
	}
	ms := &Tree{Head: label}
	var flat func(s *graph.Node)
	flat = func(s *graph.Node) {
		var list []*graph.Node
		if graph.IsTypeDef(s) {
			list = graph.Members(s)
		} else {
			list = s.Args()
		}
		for _, m := range list {
			switch {
			case m.Is("static_assert") || !m.IsList():
			case len(m.Kids) > 0 && m.Kids[0].IsList() && graph.IsTypeDef(m.Kids[0]):
				flat(m.Kids[0]) // an anonymous struct or union: its members are s's
			default:
				e := &Tree{Head: "member", Node: m, Name: memberName(m)}
				if s.Is("enum") {
					e.Head = "enumerator"
				}
				if !m.Is("member") && len(m.Kids) > 1 && !s.Is("enum") {
					e.Inline = []*graph.Node{m.Kids[1]}
				}
				counts := map[string]int{}
				for _, u := range ix.Uses(m) {
					counts[ix.Access(u)]++
				}
				e.Attrs = usesAttr(counts)
				ms.Kids = append(ms.Kids, e)
			}
		}
	}
	if s.Is("extern-struct") || s.Is("extern-union") {
		ms.Notes = append(ms.Notes, "a header's: the members the file names")
	}
	flat(s)
	t.Kids = append(t.Kids, ms)

	taking, returning := &Tree{Head: "taking"}, &Tree{Head: "returning"}
	seen := map[*graph.Node]bool{}
	for _, f := range ix.G.Forms {
		ft := graph.DeclType(f)
		if ft == nil || !ft.Is("fn") || f.Is("typedef") {
			continue
		}
		rep := ix.Rep(f)
		if seen[rep] || rep != f {
			continue
		}
		seen[rep] = true
		var ps []*graph.Node
		if len(ft.Kids) > 1 {
			for _, p := range ft.Kids[1].Kids {
				if p.Type != nil && reaches(p.Type, s) {
					ps = append(ps, p)
				}
			}
		}
		if len(ps) > 0 {
			taking.Kids = append(taking.Kids, &Tree{Head: "fn", Node: f, Inline: ps})
		}
		if f.Type != nil && f.Type.Is("function") && len(f.Type.Kids) > 2 && reaches(f.Type.Kids[2].Type, s) {
			returning.Kids = append(returning.Kids, &Tree{Head: "fn", Node: f, Inline: []*graph.Node{ft.Kids[len(ft.Kids)-1]}})
		}
	}
	holding, objects := &Tree{Head: "held-in"}, &Tree{Head: "objects"}
	for _, f := range ix.G.Forms {
		graph.Walk(f, func(n *graph.Node) bool {
			if n.Is("defn") {
				return false
			}
			if graph.IsTypeDef(n) && n != s && (n.Is("struct") || n.Is("union")) {
				for _, m := range graph.Members(n) {
					if m.Type != nil && len(m.Kids) > 1 && !m.Kids[0].IsList() && reaches(m.Type, s) {
						holding.Kids = append(holding.Kids, &Tree{Head: "member", Node: m, Inline: []*graph.Node{m.Kids[1]}})
					}
				}
			}
			return true
		})
		if f.Is("def") && f.Type != nil && reaches(f.Type, s) && ix.Rep(f) == f {
			objects.Kids = append(objects.Kids, &Tree{Head: "def", Node: f, Inline: []*graph.Node{graph.DeclType(f)}})
		}
	}
	for _, g := range []*Tree{taking, returning, holding, objects} {
		if len(g.Kids) > 0 {
			t.Kids = append(t.Kids, g)
		}
	}
	t.Notes = append(t.Notes, fmt.Sprintf("%s: %d %s, taken by %s, returned by %d, held in %s, %s",
		ix.Name(s), len(ms.Kids), label, plural(len(taking.Kids), "function"), len(returning.Kids),
		plural(len(holding.Kids), "member"), plural(len(objects.Kids), "object")))
	return t
}

// usesAttr is a member's counts as a form: (uses N (read R) ...).
func usesAttr(counts map[string]int) string {
	total := 0
	var ks []string
	for k, v := range counts {
		total += v
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return accessOrder(ks[i]) < accessOrder(ks[j]) })
	var b strings.Builder
	fmt.Fprintf(&b, "(uses %d", total)
	for _, k := range ks {
		fmt.Fprintf(&b, " (%s %d)", k, counts[k])
	}
	b.WriteString(")")
	return b.String()
}

// Def is the containment view of one definition: its form, every node of
// it, nothing elided -- C-lisp, with ids when the printer shows them.
func Def(ix *Index, n *graph.Node) *graph.Node { return ix.Rep(n) }

// plural is n and the word, an s added but for one.
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// memberName is a member's or an enumerator's own name, its struct's
// aside.
func memberName(m *graph.Node) string {
	switch {
	case m.Is("member") && len(m.Kids) > 1:
		return m.Kids[1].Atom
	case len(m.Kids) > 0 && !m.Kids[0].IsList():
		return m.Kids[0].Atom
	}
	return ""
}
