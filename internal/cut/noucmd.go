package cut

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

var ucmdLeft = regexp.MustCompile(`\b(?:do_ucmd|ucmds)\b`)

// ucmdComplRows are the six rows of ExpandOther's completion table that keep
// six get_user_cmd_* functions alive.
var ucmdComplRows = []string{
	"(init EXPAND_USER_COMMANDS get_user_commands FALSE TRUE)",
	"(init EXPAND_USER_ADDR_TYPE get_user_cmd_addr_type FALSE TRUE)",
	"(init EXPAND_USER_CMD_FLAGS get_user_cmd_flags FALSE TRUE)",
	"(init EXPAND_USER_NARGS get_user_cmd_nargs FALSE TRUE)",
	"(init EXPAND_USER_COMPLETE get_user_cmd_complete FALSE TRUE)",
	"(init EXPAND_USER_COMPLETEOPT get_user_cmd_completeopt FALSE TRUE)",
}

// NoUcmd removes user-defined commands.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the dispatch folded never (its
// else stays: an unknown name has already been rejected upstream), the six
// rows of ExpandOther's table deleted (INITROW, the local table its scope),
// the calls and the completion contexts cut by form (history keeps the text
// version).
func NoUcmd(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noucmd", e, w)
	q := graph.NewVerbs("noucmd", e, io.Discard)
	q.FoldNever("(if (paren (< (cast int (paren (. ea cmdidx))) 0)) (block (call do_ucmd (addr ea))) _)", 1,
		"the user-command dispatch")
	if q.Failed() {
		return q.Done()
	}
	v.Say("the dispatch for a name that is not in cmdnames[]")

	// Six rows of the completion table keep six get_user_cmd_* functions
	// alive, and expand_user_command_name() is how `:`-completion walks past
	// the end of cmdnames[] into the user table.  None of them was found by
	// grepping for do_ucmd -- A TABLE ROW IS A REFERENCE THE SAME AS A CALL.
	q.InFunction("ExpandOther", func(q *graph.Verbs) {
		if tab := q.One("(def static tab _ _)", "ExpandOther's table"); tab != nil {
			q.In(tab, func(q *graph.Verbs) {
				deleteRowsTyped(q, ucmdComplRows, "6 completion rows")
			})
		}
	})
	q.Rewrite("(return (call get_user_commands nullptr (- idx (cast int CMD_SIZE))))", "(return nullptr)", 1,
		"the name walk past cmdnames[]")
	// find_ucmd() looks a name up in the user table; two callers, one in the
	// completion path and one in do_one_cmd's name scan.
	q.Cut("(= p (call find_ucmd eap p nullptr xp complp))", 1, "find_ucmd's completion caller")
	q.Cut("(= p (call find_ucmd eap p full nullptr nullptr))", 1, "find_ucmd's name-scan caller")
	q.CutRun("the :command completion contexts", "(case CMD_command)",
		"(return (call set_context_in_user_cmd xp arg))", "(case CMD_delcommand)",
		"(= (-> xp xp_context) EXPAND_USER_COMMANDS)", "(= (-> xp xp_pattern) arg)", "(break)")
	if q.Failed() {
		return q.Done()
	}
	v.Say("six completion rows, the name walk past cmdnames[], and two completion contexts")

	q.Cut("(call uc_clear (addr (-> buf b_ucmds)))", 1, "the buffer's table")
	if q.Failed() {
		return q.Done()
	}
	// The b_ucmds member itself is the collection's once nothing names it.
	v.Say("the per-buffer command table")
	v.Sayf("%d do_ucmd/ucmds mentions left for the sweep", len(ucmdLeft.FindAll(v.Text(), -1)))
	return v.Done()
}
