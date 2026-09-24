package p037

// Whim phase 37, the check -- no command that does nothing.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim37", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim37", "  inert        ", `\b%s\b`, check.GERE, "  inert        no handler left for a command that did nothing",
		"ex_behave", "ex_mode", "ex_open", "ex_winpos", "get_behave_arg",
		"e_winpos_requires_two_number_arguments", "e_screen_mode_setting_not_supported")(w, args)
}
