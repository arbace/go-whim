package p034

// Whim phase 34, the check -- no abbreviations.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim34", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim34", "  abbr         ", `\b%s\b`, check.GERE, "  abbr         nothing defines, lists or expands an abbreviation",
		"check_abbr", "echeck_abbr", "ccheck_abbr", "ex_abbreviate", "ex_abclear")(w, args)
}
