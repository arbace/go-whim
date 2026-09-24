package p068

// Whim phase 68, the check -- one window, structurally.
// See phase/068/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim68", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim68", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  onewindow    ", `\b%s\b`, check.GERE, check.GERE, true, nil, "win_split_ins", "win_alloc_popup_win", "win_init_popup_win", "aucmd_win", "AUCMD_WIN_COUNT", "aucmdwin_T",
		"use_aucmd_win_idx", "win_close", "close_windows", "win_close_othertab", "close_last_window_tabpage",
		"close_tabpage", "free_tabpage", "winframe_remove", "win_equal", "win_equal_rec", "frame2win", "win_altframe",
		"make_snapshot", "restore_snapshot", "clear_snapshot", "clear_snapshot_rec") {
		return harness.ErrReported
	}
	if !s.KeptE("  onewindow    ", " went too -- the one window still needs it", "win_comp_pos", "frame_comp_pos", "shell_new_rows", "did_set_laststatus", "last_status", "last_status_rec", "topframe", "curwin", "firstwin", "win_alloc_firstwin") {
		return harness.ErrReported
	}
	s.Echo("  onewindow    nothing can add a window or a tabpage; the one window keeps its geometry")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	t := check.ProbeFile(d, "t.txt", "alpha\nbeta\ngamma\n", "+2", "+normal! dd", "+wq")
	if check.Bar(t) != "alpha|gamma|" {
		s.Echo("  onewindow    editing broke: '%s'", check.Bar(t))
		return harness.ErrReported
	}
	q := check.ProbeFile(d, "q.txt", "one\n", "+normal! ix", "+q", "+wq")
	if !check.GrepQ(check.ReadFile(q), `^xone$`, check.GBRE) {
		s.Echo("  onewindow    :q on a modified buffer did not refuse")
		return harness.ErrReported
	}
	if check.InD(d, "-e", "-s", "+set laststatus=2", "+set lines=30 columns=90", "+wq", "t.txt") != 0 {
		s.Echo("  onewindow    :set laststatus/lines/columns broke")
		return harness.ErrReported
	}
	s.Echo("  onewindow    editing works; :q refuses a modified buffer; laststatus and a resize still compute")
	return nil
}
