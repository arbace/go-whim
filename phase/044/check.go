package p044

// Whim phase 44, the check -- no filters, sorting or alignment.
// See phase/044/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim44", Check) }

func Check(w io.Writer, args []string) error {
	return check.GoneTools("whim44", "  filters      ", `\b%s\b`, check.GERE, "  filters      no :!, no sorting, no retab, no alignment; the ! key beeps",
		"ex_bang", "ex_sort", "ex_uniq", "ex_retab", "ex_align")(w, args)
}
