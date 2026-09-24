package p049

// Whim phase 49, the check -- one set of options.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim49", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim49", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  optset       ", `\b%s\b`, check.GERE, check.GERE, true, nil, "do_modelines", "chk_modeline", "p_mls", "p_mle", "p_mlstr", "p_ml", "p_ml_nobin", "b_p_ml", "b_p_ml_nobin",
		"modeline_whitelist", "is_modeline_whitelisted") {
		return harness.ErrReported
	}
	s.Echo("  optset       no modeline, and nothing that set one copy of an option")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if Out, rc := s.OutWork("-e", "-s", "+set ts=3", "+q!"); rc != 0 {
		s.Echo("  optset       the control failed: :set ts=3 exits %d: %s", rc, Out)
		return harness.ErrReported
	}
	if Out, rc := s.OutWork("-e", "-s", "+set ts<", "+q!"); rc != 1 {
		s.Echo("  optset       :set ts< exits %d, expected 1: %s", rc, Out)
		return harness.ErrReported
	}
	d, _ := os.MkdirTemp("", "whimchk")
	defer os.RemoveAll(d)
	m := filepath.Join(d, "m.txt")
	check.Put(m, "one\n# vim: set sw=2:\n")
	// "$OLDPWD/$work/whim-vim": the work tree as seen from where the check runs.
	bin, _ := filepath.Abs(filepath.Join(s.Work, "whim-vim"))
	check.VimRC(d, d, bin, "-e", "-s", "+1normal! >>", "+w", "+q!", "m.txt")
	if first := check.SedN(m, 1); first != "    one" {
		s.Echo("  optset       a modeline set 'shiftwidth': the first line is '%s'", first)
		return harness.ErrReported
	}
	s.Echo("  optset       :set ts< is refused, and a modeline sets nothing")
	return nil
}
