package p071

// Whim phase 71 (formerly 145) -- check_termcode() has no goto.  See GOAL.md.
//
// While an OSC response arrived over several reads, the loop jumped into the
// OSC branch of a later if-chain (internal/gen/FINDINGS.md, 11).  The jump's if handles
// the response itself and everything the jump skipped becomes its else.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim71", Edit) }

// Edit takes the jump into an if body out of check_termcode().
//
// While an OSC response was arriving over several reads, check_termcode()
// jumped from the top of its loop to handle_osc, a label inside the OSC branch
// of the if-chain in `if (key_name[0] == NUL)`, skipping everything between
// (internal/gen/FINDINGS.md, 11).  Nothing follows that chain inside its block, so the
// jump did exactly this: the OSC handling, then the code after the block.  So
// the jump's if gets the handling as its body, and everything it skipped --
// from the key's first byte through the end of the block -- becomes its else.
// A continue or break in that code binds to the loop it bound to: an if does
// not catch either.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the jump replaced by the
// handling (FRAG), the jump's if given an else whose items are the ones it
// skipped, moved there with their nodes (MOVE: a declaration a later use
// needs, or a jump that would bind elsewhere, is refused), and the label
// deleted.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("oscgoto", e, w)
	is := func(n *graph.Node, pat string) bool {
		p, err := clisp.Pattern(pat)
		return err == nil && n != nil && graph.Matches(p, n)
	}
	v.InFunction("check_termcode", func(v *graph.Verbs) {
		gs, ls := v.Find("(goto handle_osc)"), v.Find("(label handle_osc)")
		if len(gs) != 1 || len(ls) != 1 {
			v.Die("the jump to handle_osc or its label is not where this phase expects it")
			return
		}
		g, lbl := gs[0], ls[0]
		then := e.Parent(g)
		jump := e.Parent(then)
		if !is(jump, "(if (. osc_state processing) (block (= (index tp len) NUL) (= (index key_name 0) NUL) (= (index key_name 1) NUL) (= modifiers 0) (goto handle_osc)))") {
			v.Die("the jump to handle_osc or its label is not where this phase expects it")
			return
		}
		// the item of the jump's list that holds the label: the block of
		// `if (key_name[0] == NUL)`, which ends the code the jump skipped
		list := e.Parent(jump)
		var key *graph.Node
		for p := lbl; p != nil; p = e.Parent(p) {
			if e.Parent(p) == list {
				key = p
				break
			}
		}
		if !is(key, "(if (== (index key_name 0) NUL) _)") {
			v.Die("the label is not inside `if (key_name[0] == NUL)`")
			return
		}
		var first *graph.Node
		for i, x := range list.Kids {
			if x == jump {
				first = list.Kids[i+1]
			}
		}
		if _, err := e.SpliceC(graph.Frag{At: e.SpotOf(g), Src: "if (handle_osc(tp, len, key_name, &slen) == FAIL)\n{\n    return -1;\n}\n"}); err != nil {
			v.Die("check_termcode(): the handling -- %v", err)
			return
		}
		// the else: a placeholder, the skipped items after it, the placeholder gone
		ni, err := e.Build(jump, "(if ?c ?t (block (empty)))", graph.Bindings{"c": jump.Kids[1], "t": then})
		if err == nil {
			err = e.Replace(jump, ni...)
		}
		if err != nil {
			v.Die("check_termcode(): the else -- %v", err)
			return
		}
		hold := ni[0].Kids[3].Kids[1]
		if err := e.MoveRun(first, key, hold, true); err != nil {
			v.Die("check_termcode(): the skipped code -- %v", err)
			return
		}
		if err := e.Delete(hold); err != nil {
			v.Die("%v", err)
			return
		}
		if err := e.Delete(lbl); err != nil {
			v.Die("%v", err)
			return
		}
		if v.Count("(goto _)") != 0 || v.Count("(label handle_osc)") != 0 {
			v.Die("check_termcode() still jumps")
			return
		}
		v.Say("while an OSC response is arriving, the loop handles it in the jump's own if, and everything the jump skipped is its else: no goto left")
	})
	return v.Done()
}
