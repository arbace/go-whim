package p039

// Whim phase 39, the check -- one window, always.
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

func init() { check.Register("whim39", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim39", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  windows      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "ex_splitview", "ex_close", "ex_only", "ex_resize", "ex_wincmd", "ex_syncbind", "ex_buffer_all",
		"do_window", "nv_window", "win_split", "make_windows", "edit_buffers", "open_cmdwin",
		"cmdwin_type", "cmdwin_win", "cmdwin_buf", "cmdwin_result", "cedit_key", "p_cedit", "p_cwh",
		"p_sbo", "p_swb", "swb_flags", "swbuf_goto_win_with_buf", "tabpage_new",
		"do_check_scrollbind", "do_check_cursorbind", "check_scrollbind",
		"wo_scb", "wo_crb", "wo_wfb", "cmod_split", "postponed_split", "window_count", "window_layout") {
		return harness.ErrReported
	}
	s.Echo("  windows      nothing makes, reaches, closes or binds a second window")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	for _, o := range []string{"-o", "-O", "-o2"} {
		Out, rc := s.OutWork(o, "-e", "-s", "-c", "qa!")
		if !strings.Contains(Out, "Unknown option argument") {
			s.Echo("  cli          %s is not refused as unknown (exit %d): %s", o, rc, Out)
			return harness.ErrReported
		}
		if rc != 1 {
			s.Echo("  cli          %s exits %d, expected 1", o, rc)
			return harness.ErrReported
		}
	}
	s.Echo("  cli          -o and -O are unknown options")
	return nil
}
