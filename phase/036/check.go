package p036

// Whim phase 36, the check -- one tab page, always.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim36", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim36", "  tabs         ", `\b%s\b`, check.GERE, "  tabs         nothing makes, reaches, moves, lists or draws a second tab page",
		"ex_tabclose", "ex_tabnext", "ex_tabmove", "ex_tabonly", "ex_tabs", "ex_redrawtabline",
		"goto_tabpage", "goto_tabpage_lastused", "win_new_tabpage", "tabpage_close", "tabpage_move",
		"may_open_tabpage", "p_stal", "p_tpm", "p_tcl", "tcl_flags", "postponed_split_tab")(w, args)
}
