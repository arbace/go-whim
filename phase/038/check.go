package p038

// Whim phase 38, the check -- the argument list is walked by `:next` and `:previous` alone.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim38", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim38", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  arglist      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "ex_args", "ex_argadd", "ex_argdelete", "ex_argdedupe", "ex_argedit", "ex_argument",
		"ex_last", "ex_wnext", "ex_all", "get_arglist_name") {
		return harness.ErrReported
	}
	if !s.KeptE("  arglist      ", " is gone, and :next, :previous or :drop needed it", "ex_next", "ex_previous", "do_argfile", "ex_rewind") {
		return harness.ErrReported
	}
	s.Echo("  arglist      :next, :previous and :drop walk the list; nothing else does")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	return s.Phasebuild()
}
