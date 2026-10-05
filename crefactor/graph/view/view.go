package view

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// A Step is one move of a view's relation, from a set of nodes to another.
type Step struct {
	Kind string // refers, typed, contains, inside; holder, statement; call
	In   bool   // against the edge: refers< is a declaration's uses
}

// The steps a spec is written in, one word each (ParseSteps):
//
//	refers   a use to its declaration (the entity's: a function's definition)
//	refers<  a declaration to its uses -- every declaration of the entity's
//	typed    a node to its type          typed<     a type to what it types
//	contains a list to its elements      contains<  a node to its parent
//	inside   a node to every node it contains, at any depth
//	^fn      a node to the top-level form holding it
//	^stmt    a node to the statement holding it
//	call     keep the nodes a call calls: (call NODE ...)
//
// A view's CONTEXT is the use an edge was crossed at: where a refers edge
// (or a typed one) was followed, the node it starts from, whichever way it
// was followed; that is what is shown around a child.
func ParseSteps(spec string) ([]Step, error) {
	var out []Step
	for _, w := range strings.Fields(spec) {
		s := Step{}
		if strings.HasSuffix(w, "<") {
			s.In = true
			w = strings.TrimSuffix(w, "<")
		}
		switch w {
		case "refers", "typed", "contains":
			s.Kind = w
		case "inside", "^fn", "^stmt", "call":
			if s.In {
				return nil, fmt.Errorf("%s< has no direction", w)
			}
			s.Kind = map[string]string{"inside": "inside", "^fn": "holder", "^stmt": "statement", "call": "call"}[w]
		default:
			return nil, fmt.Errorf("no step %q: refers, typed, contains (each with < for against the edge), inside, ^fn, ^stmt, call", w)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("a spec of no steps")
	}
	return out, nil
}

func (s Step) String() string {
	w := map[string]string{"holder": "^fn", "statement": "^stmt"}[s.Kind]
	if w == "" {
		w = s.Kind
	}
	if s.In {
		w += "<"
	}
	return w
}

// A Hit is a child a relation reached, and the uses it was reached at.
type Hit struct {
	To  *graph.Node
	Via []*graph.Node // in the source's order
}

// A Relation is a view's children of a node, in the source's order.
type Relation func(ix *Index, n *graph.Node) []Hit

// Steps is the relation of a list of steps.
func Steps(steps []Step) Relation {
	return func(ix *Index, n *graph.Node) []Hit {
		type at struct{ n, via *graph.Node }
		cur := []at{{n, nil}}
		for _, s := range steps {
			var next []at
			seen := map[at]bool{}
			add := func(a at) {
				if a.n != nil && !seen[a] {
					seen[a] = true
					next = append(next, a)
				}
			}
			for _, a := range cur {
				switch s.Kind {
				case "refers":
					if s.In {
						for _, d := range ix.Decls(a.n) {
							for _, u := range ix.Uses(d) {
								add(at{u, u})
							}
						}
					} else {
						for _, r := range a.n.Refs {
							add(at{ix.Rep(r), a.n})
						}
					}
				case "typed":
					if s.In {
						for _, u := range ix.TypedBy(a.n) {
							add(at{u, u})
						}
					} else if a.n.Type != nil {
						add(at{a.n.Type, a.n})
					}
				case "contains":
					if s.In {
						add(at{ix.Parent(a.n), a.via})
					} else {
						for _, k := range a.n.Kids {
							if k.ID != 0 {
								add(at{k, a.via})
							}
						}
					}
				case "inside":
					graph.Walk(a.n, func(k *graph.Node) bool {
						if k != a.n && k.ID != 0 {
							add(at{k, a.via})
						}
						return true
					})
				case "holder":
					add(at{ix.Rep(ix.Holder(a.n)), a.via})
				case "statement":
					add(at{ix.Statement(a.n), a.via})
				case "call":
					if ix.InCall(a.n) {
						add(a)
					}
				}
			}
			cur = next
		}
		return hits(ix, cur, func(a at) (*graph.Node, *graph.Node) { return a.n, a.via })
	}
}

// hits groups what a relation reached by node, each with its uses.
func hits[T any](ix *Index, xs []T, f func(T) (*graph.Node, *graph.Node)) []Hit {
	index := map[*graph.Node]int{}
	var out []Hit
	for _, x := range xs {
		n, via := f(x)
		i, ok := index[n]
		if !ok {
			i = len(out)
			index[n] = i
			out = append(out, Hit{To: n})
		}
		if via != nil {
			out[i].Via = append(out[i].Via, via)
		}
	}
	for i := range out {
		ix.inOrder(out[i].Via)
		out[i].Via = dedupe(out[i].Via)
	}
	sortHits(ix, out)
	return out
}

func dedupe(ns []*graph.Node) []*graph.Node {
	out := ns[:0]
	for i, n := range ns {
		if i == 0 || n != ns[i-1] {
			out = append(out, n)
		}
	}
	return out
}

func sortHits(ix *Index, hs []Hit) {
	// insertion in the source's order keeps it stable and the lists are short
	for i := 1; i < len(hs); i++ {
		for j := i; j > 0 && ix.before(hs[j].To, hs[j-1].To); j-- {
			hs[j], hs[j-1] = hs[j-1], hs[j]
		}
	}
}

// Show is how much of the graph a view shows around a use.
type Show int

const (
	ShowStmt Show = iota // the statement holding it, the blocks it is not in elided
	ShowNode             // the use alone: a call, a member's selection, an atom
	ShowFn               // the whole top-level form holding it
	ShowNone             // nothing: the children alone
)

// ParseShow reads node, stmt, fn or none.
func ParseShow(s string) (Show, error) {
	switch s {
	case "stmt":
		return ShowStmt, nil
	case "node":
		return ShowNode, nil
	case "fn":
		return ShowFn, nil
	case "none":
		return ShowNone, nil
	}
	return 0, fmt.Errorf("--show is node, stmt, fn or none, not %q", s)
}

// A Spec is a view: what it is called, its relation, how deep it goes,
// where it stops and what it shows.
type Spec struct {
	Name  string   // the view's head: callers, uses, ...
	Child string   // a child's head: in, calls, ...
	Rel   Relation // a node's children
	Depth int      // the levels of children below the root; 0 is no limit
	Stop  []string // heads of nodes a view shows and does not expand
	Show  Show
	// Label, when there is one, says what each context is: a member's
	// read or write.  It is handed the uses in that context.
	Label func(ix *Index, uses []*graph.Node) string
}

// A Tree is a view: the root, and below it its children, each with the
// contexts it was reached through.
type Tree struct {
	Head     string        // the view's name at the root, a child's head below it
	Node     *graph.Node   // the node this entry is
	Link     bool          // the view holds Node already: a link, not expanded
	Name     string        // the node's name, when not Index.Name's
	Inline   []*graph.Node // forms shown on the entry's line, after its name: a member's type
	Attrs    string        // Lisp written after them: counts
	Contexts []Context
	Kids     []*Tree
	Notes    []string // comments printed above the entry: counts, what was left out
}

// A Context is a form shown around a child's uses.
type Context struct {
	Label string      // what the uses are, when the view says
	Form  *graph.Node // the use alone, its statement, or its function
	Uses  []*graph.Node
	Whole bool // shown whole, nothing elided
}

// Build is the view of root by spec: the root, its children, theirs, to
// the spec's depth, a revisit a link.  It is deterministic: children in id
// order, which is the source's, depth first.
func Build(ix *Index, root *graph.Node, spec Spec) *Tree {
	root = ix.Rep(root)
	t := &Tree{Head: spec.Name, Node: root}
	held := map[*graph.Node]bool{root: true}
	stop := map[string]bool{}
	for _, s := range spec.Stop {
		stop[s] = true
	}
	var expand func(t *Tree, depth int)
	expand = func(t *Tree, depth int) {
		for _, h := range spec.Rel(ix, t.Node) {
			c := &Tree{Head: spec.Child, Node: h.To, Contexts: Contexts(ix, h.Via, spec.Show, spec.Label)}
			t.Kids = append(t.Kids, c)
			if held[h.To] {
				c.Link = true
				continue
			}
			held[h.To] = true
			if (spec.Depth == 0 || depth < spec.Depth) && !stop[h.To.Head()] {
				expand(c, depth+1)
			}
		}
	}
	expand(t, 1)
	return t
}

// Contexts are the forms around uses: one for each statement (or
// function) holding any, in the source's order, each with the uses it holds.
func Contexts(ix *Index, uses []*graph.Node, show Show, label func(*Index, []*graph.Node) string) []Context {
	if show == ShowNone || len(uses) == 0 {
		return nil
	}
	var out []Context
	at := map[*graph.Node]int{}
	for _, u := range uses {
		var f *graph.Node
		switch show {
		case ShowNode:
			f = ix.UseForm(u)
		case ShowFn:
			f = ix.Holder(u)
		default:
			f = ix.Statement(u)
		}
		i, ok := at[f]
		if !ok {
			i = len(out)
			at[f] = i
			out = append(out, Context{Form: f, Whole: show == ShowFn || show == ShowNode})
		}
		out[i].Uses = append(out[i].Uses, u)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && ix.before(out[j].Form, out[j-1].Form); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if label != nil {
		for i := range out {
			out[i].Label = label(ix, out[i].Uses)
		}
	}
	return out
}

// UseForm is a use with the least around it that says what it is: the call
// of what a call calls, the selection a member is selected by, the atom.
func (ix *Index) UseForm(u *graph.Node) *graph.Node {
	p := ix.Parent(u)
	switch {
	case p == nil:
		return u
	case ix.InCall(u):
		return p
	case (p.Is("->") || p.Is(".")) && len(p.Kids) > 2 && p.Kids[1] != u:
		return p
	case p.Is("at"):
		return p
	}
	return u
}

// Count is the number of children in t, and of contexts, at every level,
// links included.
func (t *Tree) Count() (kids, contexts int) {
	contexts = len(t.Contexts)
	for _, k := range t.Kids {
		a, b := k.Count()
		kids += 1 + a
		contexts += b
	}
	return kids, contexts
}

// idText is an id as the marks write it.
func idText(id graph.ID) string { return strconv.FormatUint(uint64(id), 10) }
