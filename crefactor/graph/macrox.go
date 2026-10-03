package graph

import (
	"fmt"
	"strconv"
	"strings"
)

// MACROX (doc/GRAPH-MIGRATION.md, B2a).  A macro's invocation in C a
// fragment holds is made a node as the importer makes it (frag.go): an
// identifier-like one (`errno`, `nullptr`) an atom, any other `(macro
// "TEXT")`, each with a refers edge to every name its expansion uses.  And
// an invocation already in the graph, opaque, is EXPANDED: replaced by the
// C its expansion is -- the text a caller makes of its name and its
// arguments, read from the invocation's text (phase 42's MIN and MAX, from
// the header's own definition) -- which FRAG makes nodes at its place, an
// argument written twice in the expansion parsed twice: two nodes, no
// copy to make.

// MacroCall is an invocation's name and arguments, read from its text:
// `(macro "MIN(a, f(b, c))")` is MIN and [a f(b, c)]; one with no
// parentheses, none.  ok is false when n is no `(macro "...")`.
func MacroCall(n *Node) (name string, args []string, ok bool) {
	if !n.Is("macro") || len(n.Kids) != 2 || n.Kids[1].list {
		return "", nil, false
	}
	text, err := strconv.Unquote(n.Kids[1].Atom)
	if err != nil {
		return "", nil, false
	}
	return splitCall(text)
}

// splitCall reads `NAME(ARG, ...)` -- the arguments split at the commas
// outside parentheses, brackets, braces and literals -- or a bare NAME.
func splitCall(text string) (string, []string, bool) {
	text = strings.TrimSpace(text)
	i := 0
	for i < len(text) && identChar(text[i]) {
		i++
	}
	if i == 0 || !identStart(text[0]) {
		return "", nil, false
	}
	name := text[:i]
	rest := strings.TrimSpace(text[i:])
	if rest == "" {
		return name, nil, true
	}
	if rest[0] != '(' || rest[len(rest)-1] != ')' {
		return "", nil, false
	}
	inner := rest[1 : len(rest)-1]
	var args []string
	depth, start := 0, 0
	for k := 0; k < len(inner); k++ {
		switch c := inner[k]; c {
		case '"', '\'':
			k = skipLiteral(inner, k) - 1
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return "", nil, false
			}
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:k]))
				start = k + 1
			}
		}
	}
	if depth != 0 {
		return "", nil, false
	}
	if strings.TrimSpace(inner) != "" || len(args) > 0 {
		args = append(args, strings.TrimSpace(inner[start:]))
	}
	return name, args, true
}

// ExpandMacros replaces each invocation ns names by the C expand makes of
// its name and arguments, all in one synthesized import (SpliceC).  An
// expand that answers "" leaves its invocation as it is.
func (e *Editor) ExpandMacros(ns []*Node, expand func(name string, args []string) (string, error)) error {
	fs, err := e.macroFrags(ns, expand)
	if err != nil || len(fs) == 0 {
		return err
	}
	_, err = e.SpliceC(fs...)
	return err
}

// macroFrags are the fragments that expand the invocations ns.
func (e *Editor) macroFrags(ns []*Node, expand func(name string, args []string) (string, error)) ([]Frag, error) {
	var fs []Frag
	for _, n := range ns {
		name, args, ok := MacroCall(n)
		if !ok {
			return nil, fmt.Errorf("expand #%d (%s): not a macro's invocation with its text", n.ID, label(n))
		}
		src, err := expand(name, args)
		if err != nil {
			return nil, fmt.Errorf("expand #%d `%s`: %w", n.ID, name, err)
		}
		if src == "" {
			continue
		}
		fs = append(fs, Frag{At: e.SpotOf(n), Src: src})
	}
	return fs, nil
}

// ExpandMacros, the verb: the scope's invocations of the macros named,
// counted, each expanded by expand.
func (v *Verbs) ExpandMacros(names []string, expand func(name string, args []string) (string, error), n int, what string) {
	if v.Err != nil {
		return
	}
	want := map[string]bool{}
	for _, s := range names {
		want[s] = true
	}
	var ns []*Node
	for _, r := range v.roots() {
		Walk(r, func(x *Node) bool {
			if name, _, ok := MacroCall(x); ok && want[name] {
				ns = append(ns, x)
			}
			return true
		})
	}
	if len(ns) != n {
		v.Die("%s -- %d invocations, expected %d", what, len(ns), n)
		return
	}
	fs, err := v.e.macroFrags(ns, expand)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.spliceC(what, fs...)
}
