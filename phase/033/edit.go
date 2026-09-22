package p033

// Whim phase 33 -- commands whose machinery has already gone.  See GOAL.md.
//
// Every one of these still had a handler, and every one refused or did nothing
// when run with a sensible argument -- measured one by one, in Ex mode, reading
// the message each left:
//
// shell                  E319, no processes since phase 8
// gui gvim               E25, no GUI in this build
// cdo cfdo ldo lfdo      E319, no quickfix lists
// vim9cmd                E319, no eval layer
// endclass endinterface  Vim9 class keywords, invalid without the eval layer
// endenum public static this
// digraphs               E196, no digraphs in this build
// redrawtabpanel         E1547, no tab panel
// colorscheme            E185, no colour scheme to find: nothing is installed
//
// A command that only says no is a row pointing at a handler that exists to say
// no, so the row goes to ex_ni -- GOALS.md rule 3, the table keeps its
// shape -- and the sweep takes the handlers nothing else uses.  `:!` is the one
// refusal kept, on purpose: `:!cmd`, `:r !cmd` and `:w !cmd` are how a user
// reaches for a process, and phase 8's answer to that is the sentence it prints.
//
// ex_listdo also serves :argdo, :bufdo, :windo and :tabdo, so it stays; its two
// tests for the quickfix commands can never be true once those rows point
// elsewhere, and they are folded rather than left asking.
//
// THE DELTA: colorscheme.  Run bare it reported the scheme in slim-vim and
// succeeded; it is not implemented now.  Every other row already failed, or is
// one the sweep skips because it hands over the terminal.
// --- the rows ------------------------------------------------------------------
// --- ex_listdo's questions about commands that no longer reach it ---------------

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// Whim33 takes the quickfix arms of :cdo, :ldo, :cfdo and :lfdo, which answer
// "not implemented".
//
// Its report tag is `listdo` and is written per act rather than per phase,
// which is why this one does not use New()'s tag column the way the others do:
// the heredoc prints "  listdo       <what>: gone".
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("listdo", text, w)
	e.InFunction("ex_listdo", func(e *edit.E) {
		for _, f := range []struct{ What, pattern string }{
			{"the winfixbuf refusal for :ldo and :lfdo",
				`(?m)^[ \t]*if \(\(eap->cmdidx == CMD_ldo \|\| eap->cmdidx == CMD_lfdo\) && !eap->forceit\)$`},
			{"the quickfix commands answering not implemented",
				`(?m)^[ \t]*if \(eap->cmdidx == CMD_cdo \|\| eap->cmdidx == CMD_ldo \|\| eap->cmdidx == CMD_cfdo \|\| eap->cmdidx == CMD_lfdo\)$`},
		} {
			e.FoldNever(f.pattern, f.What+": gone")
		}
	})
	return e.Done()
}

func init() { edit.Register("whim33", Edit) }
