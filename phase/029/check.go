package p029

// Whim phase 29, the check -- `:command`, user-defined commands.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim29", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim29", "  ucmd         ", "%s", check.GBRE, "  ucmd         no user command table, and no dispatch into one",
		"do_ucmd", "uc_check_code", "uc_add_command", "b_ucmds", "ex_delcommand")(w, args)
}
