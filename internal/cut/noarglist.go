package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoArgList takes the argument-list commands out of completion: the commands
// themselves are retired at phase 1 (exfront, the reform's D2).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): a case label and a case's run
// cut, and a row of ExpandOther's table deleted (history keeps the text
// version).
func NoArgList(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noarglist", e, w)

	// :next asking whether it was :snext, do_argfile sparing :argdo, and
	// :argdo's walk in ex_listdo died with those commands, retired at phase 1
	// (exfront, the reform's D2): what is left is completion.
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		v.Cut("(case CMD_argdo)", 1, "completion for :argdo")
		v.CutRun("completion for :argdelete", "(case CMD_argdelete)",
			"(while (!= (= (-> xp xp_pattern) (call vim_strchr arg ' ')) nullptr) (block (= arg (+ (-> xp xp_pattern) 1))))",
			"(= (-> xp xp_context) EXPAND_ARGLIST)", "(= (-> xp xp_pattern) arg)", "(break)")
	})
	expandRow(v, "(init EXPAND_ARGLIST get_arglist_name TRUE FALSE)", "the argument-list expansion")

	// Not asserted that nothing outside the table names an argument-list
	// command: this runs at phase 3 (the reform's D9), where the retired
	// commands' handlers and what the phases after it take still do.
	if v.Failed() {
		return v.Done()
	}
	v.Say("no argument-list command is completed")
	return v.Done()
}
