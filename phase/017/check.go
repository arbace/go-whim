package p017

// Whim phase 17, the check -- the last two per-buffer encoding options.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim17", Check) }

func Check(w io.Writer, args []string) error {
	return check.GlobalsCheck("whim17", check.GBRE, check.GBRE, "  globals      neither option is named or read anywhere any more",
		"p_fenc", "p_bomb", "b_start_fenc", "b_start_bomb", `"fenc"`, `"bomb"`)(w, args)
}
