package p024

// Whim phase 24, the check -- there is no mouse.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim24", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim24", args)
	if err != nil {
		return err
	}
	if !s.Gone("  mouse        ", check.GBRE, check.GBRE, true, nil, "do_mouse", "jump_to_mouse", "setmouse", "mouse_has", "nv_mouse",
		"check_termcode_mouse", "p_mouse", "ttymouse", `"LeftMouse"`, "ScrollWheelUp", "WaitForCharOrMouse") {
		return harness.ErrReported
	}
	s.Echo("  mouse        no handler, no option, no key name, no protocol")
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
	if s.ProbeSet("mouse=a") == 0 {
		s.Echo("  options      :set mouse=a was accepted, so the option is still there")
		return harness.ErrReported
	}
	s.Echo("  options      :set mouse=a is refused, :set ignorecase still taken")
	return nil
}
