package p008

// Whim phase 8, the check -- `:!` keeps its name and loses its process.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim8", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim8", args)
	if err != nil {
		return err
	}
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if check.SymNum("after") >= check.SymNum("before") {
		s.Echo("  symbols      this phase must lower the count")
		return harness.ErrReported
	}
	return s.Phasebuild()
}
