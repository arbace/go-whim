package p045

// Whim phase 45, the check -- no `:drop`.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim45", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim45", "  drop         ", `\b%s\b`, check.GERE, "  drop         nothing is left of :drop",
		"ex_drop", "set_arglist", "ex_rewind")(w, args)
}
