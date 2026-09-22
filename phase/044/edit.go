package p044

// Whim phase 44 -- no filters, sorting or alignment.  See GOAL.md.
//
// Seven rows go to ex_ni: :! (and :{range}!), :sort, :uniq, :retab, :left,
// :center and :right.  :! was kept in Phase 8 on purpose, as the sentence it
// printed; it is dropped here on request.  The `!` operator key built nothing but
// a :{range}! command line, so its row points at nv_error.  :r !cmd and :w !cmd
// reach do_bang() through :read and :write, not through the :! row, and keep
// Phase 8's refusal: with their ! not special, :w !cmd would write a file of
// that name.
//
// THE DELTA: the six rows that succeeded run bare -- :sort, :uniq, :retab, :left,
// :center, :right -- and the behaviour cases that used them: retab, sort_u and
// sort_n.  :! already differed from Phase 8.

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

// bangRow is the nv_cmds[] row for the `!` operator.  A row is POINTED AT
// nv_error and never deleted: nv_cmd_idx[] is a sorted index computed once and
// written into the C, so deleting a row leaves the index its old length and
// every key past the hole resolving to another key's row.
var bangRow = regexp.MustCompile(`(?m)^([ \t]*\{'!', )nv_operator(, 0, 0\} ,)$`)

var retabCompletion = regexp.MustCompile(`(?m)^[ \t]*case CMD_retab:\n[ \t]*xp->xp_context = EXPAND_RETAB;\n[ \t]*xp->xp_pattern = arg;\n[ \t]*break;\n\n`)

// Whim44 takes the filter operator and :retab's completion.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	if n := len(bangRow.FindAll(text, -1)); n != 1 {
		return nil, fmt.Errorf("whim44: the ! operator row -- matched %d times", n)
	}
	text = bangRow.ReplaceAll(text, []byte("${1}nv_error${2}"))
	fmt.Fprintln(w, "  filters      the ! operator's row points at nv_error")

	blanked := cutil.Blank(text)
	a, z, ok := cutil.FindDefinition(text, blanked, "set_context_by_cmdname")
	if !ok {
		return nil, fmt.Errorf("whim44: set_context_by_cmdname is not defined at file scope")
	}
	fn := text[a:z]
	if n := len(retabCompletion.FindAll(fn, -1)); n != 1 {
		return nil, fmt.Errorf("whim44: completion for :retab -- matched %d times", n)
	}
	Out := append([]byte{}, text[:a]...)
	Out = append(Out, retabCompletion.ReplaceAll(fn, nil)...)
	Out = append(Out, text[z:]...)
	fmt.Fprintln(w, "  filters      completion for :retab")
	return Out, nil
}

func init() { edit.Register("whim44", Edit) }
