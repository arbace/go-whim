package cut

import (
	"bytes"

	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// nfaNewCompile is vim_regcomp from the engine choice on: the backtracking
// engine's compile, and nothing else.
const nfaNewCompile = "(= (. rex reg_buf) curbuf) " +
	"(= prog (call (. bt_regengine regcomp) expr re_flags)) " +
	"(if (!= prog nullptr) (block (= (-> prog re_engine) BACKTRACKING_ENGINE) (= (-> prog re_flags) re_flags))) " +
	"(return prog)"

// NoNfa leaves the backtracking engine as the only one.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): vim_regcomp's items from the
// engine choice through its return are spliced by template, prog_magic_wrong's
// test and the island's two halves cut by form; the mentions left are the
// text's count on the C view (history keeps the text version).
func NoNfa(e *graph.Editor, w io.Writer) error {
	q := graph.NewVerbs("nonfa", e, io.Discard)
	// The whole body of vim_regcomp from the engine choice to its closing brace.
	q.InFunction("vim_regcomp", func(q *graph.Verbs) {
		q.Splice("(= regexp_engine p_re)", "(return prog)", nfaNewCompile,
			"vim_regcomp does not begin the way this expects -- it has moved, and "+
				"replacing a body by guesswork is how an editor ends up with no regexp engine at all")
	})
	q.InFunction("prog_magic_wrong", func(q *graph.Verbs) {
		q.Cut("(if (== (-> prog engine) (addr nfa_regengine)) (block (return FALSE)))", 1,
			"prog_magic_wrong no longer tests for the NFA engine")
	})
	// The island: BOTH HALVES OR NEITHER.  Removing the struct alone leaves
	// nfa_regcomp assigning the address of something that no longer exists.
	q.InFunction("nfa_regcomp", func(q *graph.Verbs) {
		q.Cut("(= (-> prog engine) (addr nfa_regengine))", 1,
			"cannot find nfa_regcomp's back-reference to it, and cutting one half without the other does not compile")
	})
	q.Cut("(def static nfa_regengine regengine_T (init _ _ _ _))", 1,
		"cannot find the nfa_regengine definition, and cutting one half without the other does not compile")
	if err := q.Done(); err != nil {
		return err
	}
	v := graph.NewVerbs("nonfa", e, w)
	v.Sayf("vim_regcomp compiles with bt only; prog_magic_wrong "+
		"stops asking; %d nfa_regengine mentions left for the sweep",
		bytes.Count(v.Text(), []byte("nfa_regengine")))
	return v.Done()
}
