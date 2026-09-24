package p019

// Whim phase 19, the check -- the terminal is what the build says.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim19", Check) }

func Check(w io.Writer, args []string) error {
	return check.GlobalsCheck("whim19", check.GBRE, check.GBRE, "  terminal     nothing asks the environment what terminal this is",
		`getenv((char \*)((char_u \*)"TERM")`, `getenv("LINES")`, `getenv("COLUMNS")`, `getenv((char \*)((char_u \*)"COLORS")`)(w, args)
}
