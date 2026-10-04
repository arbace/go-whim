package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoBufList leaves only :bnext and :bprevious walking the buffer list.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's folds by line head
// are folds by condition, its literals a Rewrite and a DropOperand, its
// line cuts case labels and a case's run cut (history keeps the text
// version).
func NoBufList(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nobuflist", e, w)
	const added = "(& flags (| ECMD_ADDBUF ECMD_ALTBUF))"

	// ex_edit and do_exedit went with :edit at phase 1 (filefront, D4)
	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.FoldNever("(&& (paren "+added+") (|| (== ffname nullptr) (== (deref ffname) NUL)))", 1,
			"do_ecmd adding a buffer with no name")
		v.Rewrite("(| ECMD_HIDE ECMD_ADDBUF ECMD_ALTBUF)", "(paren ECMD_HIDE)", 1,
			"do_ecmd sparing an added buffer the changed check")
		v.FoldAlways("(! "+added+")", 1, "do_ecmd keeping the alternate file")
		v.FoldNever(added, 1, "do_ecmd adding a buffer without editing it")
		v.DropOperand("(paren "+added+")", 1, "do_ecmd stopping after an added buffer")
	})
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.FoldNever("(|| (== (. ea cmdidx) CMD_bdelete) (== (. ea cmdidx) CMD_bwipeout) (== (. ea cmdidx) CMD_bunload))", 1,
			"do_one_cmd reading a buffer list argument")
	})

	// do_buffer_ext's unloading and goto_buffer's :bNext died with the buffer
	// commands, retired at phase 1 (exfront, the reform's D2)
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		v.Cut("(case CMD_bufdo)", 1, "completion for :bufdo")
		v.CutRun("completion for :bdelete, :bwipeout and :bunload",
			"(case CMD_bdelete)", "(case CMD_bwipeout)", "(case CMD_bunload)",
			"(while (!= (= (-> xp xp_pattern) (call vim_strchr arg ' ')) nullptr) (block (= arg (+ (-> xp xp_pattern) 1))))",
			"(attributed (std-attr fallthrough))")
		v.Cut("(case CMD_buffer)", 1, "completion for :buffer")
	})

	// Nothing is counted here: this runs at phase 3 (the reform's D9),
	// where ex_listdo() and ex_bunload() and the code that set ECMD_ADDBUF and
	// ECMD_ALTBUF are still in the text, for the sweep.
	if v.Failed() {
		return v.Done()
	}
	v.Say("only :bnext and :bprevious walk the buffer list")
	return v.Done()
}
