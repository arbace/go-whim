package p035

// Whim phase 35, the check -- no scripts, no session, no autocommands.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim35", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim35", "  session      ", `\b%s\b`, check.GERE, "  session      no script, session or autocommand machinery is left",
		"ex_source", "ex_redir", "ex_sleep", "ex_smile", "ex_scriptencoding", "ex_scriptversion",
		"ex_vim9script", "ex_autocmd", "ex_doautocmd", "ex_doautoall", "ex_filetype", "ex_setfiletype",
		"do_autocmd", "do_doautocmd", "event_ignored", "check_ei", "did_set_eventignore", "p_ei", "p_lpl",
		"wo_eiw", "check_window_scroll_resize", "au_has_group")(w, args)
}
