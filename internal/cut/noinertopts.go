package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoInertOpts removes the last readers of six globals whose option rows go
// with them.
//
// THE THIRD ONE SEGFAULTED.  Every other reader of these six is an ADDRESS
// comparison -- `var == &p_path` -- which survives the global going away
// without noticing.  That one DEREFERENCES p_tc, at startup, in a function
// that runs before anything else, so the editor died before its first
// keystroke and the harness reported it as all 67 behaviour cases, the
// terminal table and every Ex command moving at once.
//
// The address comparisons are harmless at run time -- a test against a
// variable nobody can name is simply false -- but they keep the globals alive,
// and a global that is alive is one the phase's own check cannot prove unread.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): three acts on the program's
// graph, each counted, as the text version's three counted substitutions
// were (history keeps it).
//
//   - ex_drop's save and restore of 'autoread' died with :drop, retired at
//     phase 1 (exfront, the reform's D2).
//   - The directory-completion block, with its backslash rule for 'path',
//     folds at phase 2 (whim18, the reform's D6).  The file-completion
//     block's rule for 'tags' went with set_context_in_set_cmd(), :set's
//     completion, at phase 4 (whim4f, phase 4f's program, which runs before
//     this phase now).
//   - ml_open's swap-file test read p_uc, which nothing writes once
//     the row of 'updatecount' is dropped: the fall-out closure took it at phase
//     1 (optfront, the reform's D3).
func NoInertOpts(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noinertopts", e, w)
	v.FoldNever("(&& (== (cast (ptr int) varp) (addr (-> curbuf b_p_ar))) (== opt_flags OPT_LOCAL))", 1,
		"`:setlocal autoread` meaning \"follow the global\"")
	v.RewriteAt("(def esc int ?v)", "v", "FALSE", 1, "option_expand escaping for 'path' and 'tags'")
	v.Cut("(cast void (call opt_strings_flags p_tc p_tc_values (addr tc_flags) FALSE))", 1,
		"didset_string_options reading 'tagcase' at startup")
	return v.Done()
}
