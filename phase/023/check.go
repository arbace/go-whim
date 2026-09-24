package p023

// Whim phase 23, the check -- no floating-point library.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim23", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim23", args)
	if err != nil {
		return err
	}
	if !s.Gone("  libm         ", check.GBRE, check.GBRE, true, nil, "ceil(", "floor(", "log10(", "infinity_str", "TYPE_FLOAT", "typename_float") {
		return harness.ErrReported
	}
	s.Echo("  libm         nothing calls a floating-point function")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if !s.SymsGone("ceil", "floor", "log10") {
		return harness.ErrReported
	}
	s.Echo("  symbols      ceil, floor and log10 are gone from nm -u")
	return s.Phasebuild()
}
