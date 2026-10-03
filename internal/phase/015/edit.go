package p015

// Whim phase 15 (formerly 48) -- no :noswapfile.  See GOAL.md.
//
// There has been no swap file since record 21: the memfile is memory.  The
// modifier set CMOD_NOSWAPFILE, and its two readers, in ml_open() and
// buf_copy_options(), were already empty blocks.  The modifier is matched by name
// before the table, so its branch goes as well as its row.
//
// THE DELTA: the row, which succeeded run bare.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim15 takes the :noswapfile modifier and everything that reads it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the modifier's case, a label
// heading a run of its own, goes with its run (DropCase); the two readers
// are folded as the text version folded them (history keeps it).  The
// mentions are the text's count, on the C view; and no use of the
// enumerator is left, by edge.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noswapfile", e, w)
	noswap := "(& (. cmdmod cmod_flags) CMOD_NOSWAPFILE)"
	v.InFunction("parse_command_modifiers", func(v *graph.Verbs) {
		v.DropCase("(case 'n')", 1, "the :noswapfile modifier")
	})
	// :noswapfile's completion went with set_context_by_cmdname() at phase 4
	// (whim4f, phase 4f's program, which runs before this phase now)
	v.InFunction("ml_open", func(v *graph.Verbs) { v.DropIf(noswap, 1, "ml_open asking for it") })
	v.InFunction("buf_copy_options", func(v *graph.Verbs) { v.FoldNever(noswap, 1, "buf_copy_options asking for it") })
	n := v.Mentions("CMOD_NOSWAPFILE")
	v.Expect(n == 1, "CMOD_NOSWAPFILE outside its enumerator -- %d mentions, expected 1", n)
	v.Expect(len(v.UsesOf("CMOD_NOSWAPFILE")) == 0, "CMOD_NOSWAPFILE still used")
	return v.Done()
}

func init() { phase.RegisterGraph("whim15", Edit) }
