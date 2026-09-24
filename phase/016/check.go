package p016

// Whim phase 16, the check -- six options that no longer decide anything.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim16", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim16", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  globals      ", `\b%s\b`, check.GBRE, check.GBRE, true, check.GlobalsExtra, "p_path", "p_sua", "p_tags", "p_tc", "p_ar", "p_swf") {
		return harness.ErrReported
	}
	s.Echo("  globals      none of the six is mentioned anywhere any more")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	return s.Phasebuild()
}
