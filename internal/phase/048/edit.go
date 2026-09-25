package p048

// Whim phase 48 -- no :noswapfile.  See GOAL.md.
//
// There has been no swap file since Phase 21: the memfile is memory.  The
// modifier set CMOD_NOSWAPFILE, and its two readers, in ml_open() and
// buf_copy_options(), were already empty blocks.  The modifier is matched by name
// before the table, so its branch goes as well as its row.
//
// THE DELTA: the row, which succeeded run bare.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

var noswapTest = edit.Head("if (cmdmod.cmod_flags & CMOD_NOSWAPFILE)")

// Whim48 takes the :noswapfile modifier and everything that reads it.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("noswapfile", text, w)
	e.InFunction("parse_command_modifiers", func(e *edit.E) {
		e.Cut(edit.Line("case 'n':", `if (!checkforcmd_noparen(&eap->cmd, "noswapfile", 3))`, "{", "break;", "}", "cmod->cmod_flags |= CMOD_NOSWAPFILE;", "continue;"), 1,
			"the :noswapfile modifier")
	})
	e.InFunction("set_context_by_cmdname", func(e *edit.E) {
		e.Cut(edit.Line("case CMD_noswapfile:"), 1, "completion for :noswapfile")
	})
	e.InFunction("ml_open", func(e *edit.E) {
		e.DropIf(noswapTest, 1, "ml_open asking for it")
	})
	e.InFunction("buf_copy_options", func(e *edit.E) {
		e.FoldNever(noswapTest, 1, "buf_copy_options asking for it")
	})

	// The enumerator is the only mention that may survive: anything else is a
	// reader this phase did not find, and shipping it would be the phase
	// half-done.
	n := e.Mentions("CMOD_NOSWAPFILE")
	e.Expect(n == 1, "CMOD_NOSWAPFILE outside its enumerator -- %d mentions, expected 1", n)
	return e.Done()
}

func init() { edit.Register("whim48", Edit) }
