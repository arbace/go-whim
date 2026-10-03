package p015a

// Whim phase 15a (formerly 44) -- no filters, sorting or alignment.  See GOAL.md.
//
// Seven rows go to ex_ni: :! (and :{range}!), :sort, :uniq, :retab, :left,
// :center and :right.  :! was kept in record 8 on purpose, as the sentence it
// printed; it is dropped here on request.  The `!` operator key built nothing but
// a :{range}! command line, so its row points at nv_error.  :r !cmd and :w !cmd
// reach do_bang() through :read and :write, not through the :! row, and keep
// record 8's refusal: with their ! not special, :w !cmd would write a file of
// that name.
//
// THE DELTA: the six rows that succeeded run bare -- :sort, :uniq, :retab, :left,
// :center, :right -- and the behaviour cases that used them: retab, sort_u and
// sort_n.  :! already differed from record 8.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim15a takes the filter operator and :retab's completion.
//
// The `!` operator's nv_cmds[] row is POINTED AT nv_error and never deleted:
// nv_cmd_idx[] is a sorted index computed once and written into the C, so
// deleting a row leaves the index its old length and every key past the hole
// resolving to another key's row.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the row's handler, one node
// of the table's initialiser, replaced in place (RewriteAt); the text
// version substituted the line (history keeps it).
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("filters", e, w)
	v.InTable("nv_cmds", func(v *graph.Verbs) {
		v.RewriteAt("(init '!' ?h 0 0)", "h", "nv_error", 1, "the ! operator's row points at nv_error")
	})
	// :retab's completion went with set_context_by_cmdname() at phase 4
	// (whim4f, phase 4f's program, which runs before this phase now)
	return v.Done()
}

func init() { phase.RegisterGraph("whim15a", Edit) }
