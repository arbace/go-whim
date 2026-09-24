package p047

// Whim phase 47, the check -- no `:startinsert`, `:startreplace`, `:startgreplace` or `:stopinsert`.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim47", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim47", "  insertcmds   ", `\b%s\b`, check.GERE, "  insertcmds   no command enters or leaves Insert mode",
		"ex_startinsert", "ex_stopinsert")(w, args)
}
