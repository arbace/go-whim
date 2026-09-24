package p018

// Whim phase 18, the check -- nothing is read at startup, and nothing on the command line decides anything.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim18", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim18", args)
	if err != nil {
		return err
	}
	if !s.Gone("  globals      ", check.GBRE, check.GBRE, true, check.GlobalsExtra,
		"p_exrc", "process_env", "set_init_xdg_rtp", "source_startup_scripts", `"VIMINIT"`, `"EXINIT"`, `"XDG_CONFIG_HOME"`) {
		return harness.ErrReported
	}
	s.Echo("  startup      no config path, option or environment name is left")
	if !s.Gone("  globals      ", check.GBRE, check.GBRE, true, check.GlobalsExtra,
		"evim_mode", "check_restricted", "EX_RESTRICT", "restricted", `"vif"`) {
		return harness.ErrReported
	}
	s.Echo("  options      nothing names the four, or what they set")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	Out, rc := s.OutWork("-e", "-s", "-c", "qa!")
	if rc != 0 {
		s.Echo("  cli          the control failed: -e -s -c qa! exits %d: %s", rc, Out)
		return harness.ErrReported
	}
	Out, rc = s.OutWork("-u", "NONE", "-e", "-s", "-c", "qa!")
	if !strings.Contains(Out, "Unknown option argument") {
		s.Echo("  cli          -u is not refused as unknown (exit %d): %s", rc, Out)
		return harness.ErrReported
	}
	if rc != 1 {
		s.Echo("  cli          -u exits %d, expected 1", rc)
		return harness.ErrReported
	}
	s.Echo("  cli          -u is an unknown option")
	return nil
}
