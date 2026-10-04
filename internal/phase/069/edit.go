package p069

// Whim phase 69 (formerly 141) -- regrepeat() does not jump into a case.  See GOAL.md.
//
// Seventeen character classes set their mask and jumped to do_class, a label
// inside the \s case (internal/gen/FINDINGS.md, 11).  All eighteen now share one case
// that sets mask and testval in a switch on the same opcode, then runs the
// unchanged loop.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim69", Edit) }

// Edit takes the jumps into a case out of regrepeat().
//
// The character classes \s \S \d \D ... \u \U were eighteen pairs of case
// labels: \s set its mask and testval and ran into the class loop, labelled
// do_class, and each of the other seventeen set its own and jumped to the
// label from further down the switch.  Go cannot jump into a case, and the
// transpilation restructured it by hand (internal/gen/FINDINGS.md, 11).  Here all
// thirty-six labels lead to one case that sets the two variables in a switch
// on the same opcode, then runs the loop: the same assignments for each
// opcode, the loop unchanged, and no goto.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the run of items the switch
// holds is read as the text was -- \s's two labels, its assignment and
// do_class before the loop and its break, then the seventeen classes, four
// items each, jumping to the label -- the head written anew by FRAG (the
// thirty-six labels and the switch on the opcode, each class's assignment
// its own C), the seventeen runs deleted; the loop and its break keep their
// nodes.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("doclass", e, w)
	v.InFunction("regrepeat", func(v *graph.Verbs) {
		lbl := v.One("(label do_class)", "regrepeat()'s do_class label")
		if v.Failed() {
			return
		}
		items := e.Parent(lbl).Kids
		at := -1
		for i, n := range items {
			if n == lbl {
				at = i
			}
		}
		is := func(n *graph.Node, pat string) (graph.Bindings, bool) {
			p, err := clisp.Pattern(pat)
			if err != nil {
				v.Die("%v", err)
				return nil, false
			}
			return graph.Match(p, n)
		}
		head := func(k int, pat string) bool {
			if at+k < 0 || at+k >= len(items) {
				return false
			}
			_, ok := is(items[at+k], pat)
			return ok
		}
		if at < 3 || !head(-3, "(case RE_WHITE)") || !head(-2, "(case (+ RE_WHITE ADD_NL))") ||
			!head(-1, "(= testval (= mask RI_WHITE))") || !head(1, "(while _ _)") || !head(2, "(break)") {
			v.Die("regrepeat(): the \\s case and its do_class label are not where this phase expects them")
			return
		}
		type cls struct{ op, assign string }
		all := []cls{{"RE_WHITE", "testval = mask = RI_WHITE;"}}
		first := at + 3
		k := first
		for ; k+3 < len(items); k += 4 {
			a, ok1 := is(items[k], "(case ?op)")
			b, ok2 := is(items[k+1], "(case (+ ?op ADD_NL))")
			_, ok3 := is(items[k+2], "(= _ _)")
			_, ok4 := is(items[k+3], "(goto do_class)")
			if !ok1 || !ok2 || !ok3 || !ok4 {
				break
			}
			if a["op"].IsList() || a["op"].Atom != b["op"].Atom {
				v.Die("regrepeat(): %s is paired with %s + ADD_NL", graph.Lisp(a["op"]), graph.Lisp(b["op"]))
				return
			}
			c, err := clisp.PrintItems([]*clisp.Node{graph.Lisp(items[k+2])})
			if err != nil {
				v.Die("%v", err)
				return
			}
			all = append(all, cls{a["op"].Atom, strings.TrimSpace(string(c))})
		}
		if n := len(all) - 1; n != 17 {
			v.Die("regrepeat(): %d classes jump to do_class right after it, and this phase was written against 17", n)
			return
		}
		if n := v.Count("(goto do_class)"); n != 17 {
			v.Die("regrepeat(): do_class is reached from elsewhere")
			return
		}
		var b strings.Builder
		for _, c := range all {
			fmt.Fprintf(&b, "      case %s:\n      case %s + ADD_NL:\n", c.op, c.op)
		}
		b.WriteString("        switch ( ((int)*(p)) )\n        {\n")
		for _, c := range all {
			fmt.Fprintf(&b, "          case %s:\n          case %s + ADD_NL:\n            %s\n            break;\n", c.op, c.op, c.assign)
		}
		b.WriteString("        }\n")
		classes := [2]*graph.Node{items[first], items[k-1]}
		if err := e.ReplaceRun(classes[0], classes[1]); err != nil {
			v.Die("regrepeat(): %v", err)
			return
		}
		if _, err := e.SpliceC(graph.Frag{At: e.SpotRun(items[at-3], lbl), Src: b.String()}); err != nil {
			v.Die("regrepeat(): %v", err)
			return
		}
		v.Sayf("the %d class opcodes share one case, which sets mask and testval by opcode and then runs the class loop: no goto do_class", len(all))
	})
	return v.Done()
}
