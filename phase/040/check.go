package p040

// Whim phase 40, the check -- no window sizes to set.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim40", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim40", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  winsizes     ", `\b%s\b`, check.GERE, check.GERE, true, nil, "p_hh", "wo_wfh", "wo_wfw", "did_set_winheight_helpheight", "did_set_winminheight",
		"did_set_winwidth", "did_set_winminwidth", "did_set_equalalways", "did_set_eadirection",
		"did_set_splitkeep", "expand_set_eadirection", "expand_set_splitkeep") {
		return harness.ErrReported
	}
	s.Echo("  winsizes     no row, callback or field is left for a window size")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if s.ProbeSet("ignorecase") != 0 {
		s.Echo("  options      the control failed: :set ignorecase exits non-zero too")
		return harness.ErrReported
	}
	for _, o := range []string{"winheight", "winminheight", "winwidth", "winminwidth", "helpheight", "splitbelow", "splitright",
		"splitkeep", "equalalways", "eadirection", "winfixheight", "winfixwidth"} {
		if s.ProbeSet(o+"?") == 0 {
			s.Echo("  options      :set %s? was accepted, so the option is still there", o)
			return harness.ErrReported
		}
	}
	s.Echo("  options      the twelve sizing options are refused, :set ignorecase still taken")
	return nil
}
