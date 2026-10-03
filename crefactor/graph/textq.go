package graph

import (
	"fmt"
	"regexp"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
)

// TEXTQ (doc/GRAPH-MIGRATION.md, B0): the text programs' assertions, asked of
// the graph, so that a converted phase asserts what its text version did.
// Two kinds, and a converter chooses:
//
//   - THE TEXT'S OWN QUESTION, EXACTLY: Text is the scope's C view -- the
//     scope's top-level form, or the whole file, printed as the C view
//     prints it, which is the text the text program counted on -- and
//     Mentions, TextCount and TextQuery ask it what edit.E's Mentions,
//     CountIs and Query asked, by the same code (crefactor/edit's
//     MentionCount, regexp).  The numbers are the text's, whatever a name's
//     mentions are: declarations, string contents, a macro's text.  A
//     function prints in well under a millisecond, the file in 50-90 ms.
//   - THE GRAPH'S QUESTION, which is usually what the count was a proxy for:
//     UsesOf (the uses of a name's declarations, by edge), UsesOutside
//     (those outside some functions), Says (an atom spelled so), Strings
//     (the string literals), Rows and Row (a table's initialiser elements),
//     Before (the order of two top-level declarations).
//
// The text-only questions -- a body's line count, blank-line runs, the
// directives on the first N lines -- have no counterpart: there are no lines.

// Text is the C view of the scope: its top-level form, or the file.
func (v *Verbs) Text() []byte {
	var forms []*clisp.Node
	if v.scope != nil {
		top := v.scope
		for p := v.e.Parent(top); p != nil; p = v.e.Parent(top) {
			top = p
		}
		forms = []*clisp.Node{Lisp(top)}
	} else {
		forms = make([]*clisp.Node, len(v.e.g.Forms))
		for i, f := range v.e.g.Forms {
			forms[i] = Lisp(f)
		}
	}
	out, err := clisp.Print(forms)
	if err != nil {
		v.Die("the C view: %v", err)
		return nil
	}
	return out
}

// Mentions counts the name as a whole word in the scope's C view: edit.E's
// Mentions, number for number.
func (v *Verbs) Mentions(name string) int { return edit.MentionCount(v.Text(), name) }

// TextCount is how many times the regular expression matches the scope's C
// view.
func (v *Verbs) TextCount(re string) int {
	x, err := regexp.Compile(re)
	if err != nil {
		v.Die("%s -- %v", re, err)
		return 0
	}
	return edit.CountMatches(x, v.Text())
}

// TextCountIs is edit.E's CountIs on the scope's C view.
func (v *Verbs) TextCountIs(re string, n int, what string) {
	if v.Err != nil {
		return
	}
	if k := v.TextCount(re); v.Err == nil && k != n {
		v.Die("%s -- matched %d times, expected %d", what, k, n)
	}
}

// TextQuery is edit.E's Query on the scope's C view: the group of every
// match, in order.
func (v *Verbs) TextQuery(re string, group int) []string {
	x, err := regexp.Compile(re)
	if err != nil {
		v.Die("%s -- %v", re, err)
		return nil
	}
	t := v.Text()
	var out []string
	for _, m := range edit.AllSubmatchIndex(x, t) {
		if m[2*group] >= 0 {
			out = append(out, string(t[m[2*group]:m[2*group+1]]))
		} else {
			out = append(out, "")
		}
	}
	return out
}

// Decls are the file's declarations of the ordinary name: its top-level
// defs, defns and typedefs, an enumerator, an external.
func (e *Editor) Decls(name string) []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		if topName(f) == name {
			out = append(out, f)
		} else if en := enumeratorIn(f, name); en != nil {
			out = append(out, en)
		}
	}
	for _, x := range e.g.Externs {
		if ordinaryName(x) == name {
			out = append(out, x)
		}
	}
	return out
}

// UsesOf are the uses in the scope of the file's declarations of name, in
// the file's order.
func (v *Verbs) UsesOf(name string) []*Node {
	ds := map[*Node]bool{}
	for _, d := range v.e.Decls(name) {
		ds[d] = true
	}
	var out []*Node
	for _, r := range v.roots() {
		Walk(r, func(n *Node) bool {
			for _, t := range n.Refs {
				if ds[t] {
					out = append(out, n)
					break
				}
			}
			return true
		})
	}
	return out
}

// UsesOutside are the uses of name's declarations in the scope outside the
// functions named: "every use left is in one of these".
func (v *Verbs) UsesOutside(name string, fns ...string) []*Node {
	in := map[string]bool{}
	for _, f := range fns {
		in[f] = true
	}
	var out []*Node
	for _, u := range v.UsesOf(name) {
		if f := v.e.Function(u); f == nil || !in[topName(f)] {
			out = append(out, u)
		}
	}
	return out
}

// Says says some atom of the scope is spelled name: an identifier, a
// keyword, a number -- not inside a string or a macro's text.
func (v *Verbs) Says(name string) bool {
	found := false
	for _, r := range v.roots() {
		Walk(r, func(n *Node) bool {
			found = found || !n.list && n.Atom == name
			return !found
		})
	}
	return found
}

// Strings are the scope's string literals, each an atom as C spells it,
// quotes and escapes included (`"abc"`, `L"x"`); not the texts the forms
// carry verbatim, written as one (an include's, a macro's, an attribute's).
func (v *Verbs) Strings() []*Node {
	var out []*Node
	for _, r := range v.roots() {
		Walk(r, func(n *Node) bool {
			if verbatimHeads[n.Head()] {
				return false
			}
			if !n.list && isStringAtom(n.Atom) {
				out = append(out, n)
			}
			return true
		})
	}
	return out
}

// verbatimHeads are the forms whose string atom is a text, not a literal.
var verbatimHeads = map[string]bool{"include": true, "macro": true, "macro-decl": true, "verbatim": true, "attr-text": true, "asm-label": true}

func isStringAtom(s string) bool {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			return true
		case c == 'L' || c == 'u' || c == 'U' || c == '8':
		default:
			return false
		}
	}
	return false
}

// Rows are the elements of the initialiser of the scope, a table's
// definition (InTable): its rows, in order.
func (v *Verbs) Rows() []*Node {
	if v.scope == nil || !v.scope.Is("def") {
		v.Die("rows: the scope is not a table (InTable)")
		return nil
	}
	val := defValue(v.scope)
	if val == nil || !val.Is("init") {
		v.Die("rows: %s has no initialiser", topName(v.scope))
		return nil
	}
	return val.Args()
}

// Row is the one row of the scope's table the pattern matches: `(init
// '!' _*)` a row found by its first element, `(at .name _)` by its
// designator, `(init _ "abc" _*)` by its string.
func (v *Verbs) Row(pat, what string) *Node {
	rows := v.Rows()
	if v.Err != nil {
		return nil
	}
	p := v.pattern(pat, what)
	if p == nil {
		return nil
	}
	var ms []*Node
	for _, r := range rows {
		if Matches(p, r) {
			ms = append(ms, r)
		}
	}
	if len(ms) != 1 {
		v.Die("%s -- %d rows match, expected 1", what, len(ms))
		return nil
	}
	return ms[0]
}

// Before says the first top-level declaration of a comes before the first
// of b in the file; an error names one that is not there.
func (e *Editor) Before(a, b string) (bool, error) {
	at := map[string]int{}
	for i, f := range e.g.Forms {
		for _, n := range []string{a, b} {
			if _, ok := at[n]; !ok && topName(f) == n {
				at[n] = i
			}
		}
	}
	for _, n := range []string{a, b} {
		if _, ok := at[n]; !ok {
			return false, fmt.Errorf("%s is not declared at file scope", n)
		}
	}
	return at[a] < at[b], nil
}
