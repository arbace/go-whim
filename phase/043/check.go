package p043

// Whim phase 43, the check -- no -c, --cmd, -R, -m, -M or -w.
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

func init() { check.Register("whim43", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim43", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  cmdargs      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "exe_pre_commands", "pre_commands", "n_pre_commands", "reset_modifiable") {
		return harness.ErrReported
	}
	s.Echo("  cmdargs      nothing collects or runs --cmd commands")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if _, rc := s.OutWork("-e", "-s", "+q!"); rc != 0 {
		s.Echo("  cli          the control failed: +q! exits %d", rc)
		return harness.ErrReported
	}
	for _, o := range []string{"-c qa!", "-cqa!", "--cmd qa!", "-R", "-m", "-M", "-w7"} {
		Out, rc := s.OutWork(append(strings.Fields(o), "-e", "-s", "+q!")...)
		if !strings.Contains(Out, "Unknown option argument") {
			s.Echo("  cli          %s is not refused as unknown (exit %d): %s", o, rc, Out)
			return harness.ErrReported
		}
		if rc != 1 {
			s.Echo("  cli          %s exits %d, expected 1", o, rc)
			return harness.ErrReported
		}
	}
	s.Echo("  cli          -c, --cmd, -R, -m, -M and -w are unknown; +{command} still runs")
	return nil
}
