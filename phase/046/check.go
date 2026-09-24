package p046

// Whim phase 46, the check -- no `:wall`, `:qall`, `:quitall`, `:wqall` or `:xall`.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim46", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim46", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  allcmds      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "do_wqall", "ex_quit_all") {
		return harness.ErrReported
	}
	s.Echo("  allcmds      nothing is left of the -all commands")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if Out, rc := s.OutWork("-e", "-s", "+q!"); rc != 0 {
		s.Echo("  quit         +q! exits %d: %s", rc, Out)
		return harness.ErrReported
	}
	if Out, rc := s.OutWork("-e", "-s", "+qa!"); rc != 1 {
		s.Echo("  quit         +qa! exits %d, expected 1: %s", rc, Out)
		return harness.ErrReported
	}
	s.Echo("  quit         :q! quits, :qa! is not a command")
	return nil
}
