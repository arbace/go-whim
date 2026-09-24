package p032

// Whim phase 32, the check -- insert completion, the popup menu, and the keys that reached them.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim32", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim32", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  compl        ", `\b%s\b`, check.GERE, check.GERE, true, nil, "ins_compl_get_exp", "pum_redraw", "ins_compl_next", "b_p_cpt", "b_p_dict", "p_pumheight",
		"docomplete", "ctrl_x_mode", "ins_complete", "ins_compl_addleader", "ins_compl_bs",
		"compl_busy", "compl_match_array", "ins_compl_show_pum", "ins_compl_build_pum",
		"ins_compl_has_autocomplete") {
		return harness.ErrReported
	}
	s.Echo("  compl        no sources, no match list, no menu, no CTRL-X mode, no docomplete")
	if check.GrepC(s.Src(), `^\s*lastc = c;`, check.GERE) != 1 {
		s.Echo("  compl        the lastc save is gone -- the disarm cut took the wrong block")
		return harness.ErrReported
	}
	s.Echo("  compl        the lastc save is untouched")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if s.St("complcheck", filepath.Join(s.Work, "whim-vim")) {
		s.Echo("  compl        insert mode still inserts; CTRL-X CTRL-N completes nothing")
		return nil
	}
	s.Echo("  compl        insert mode or CTRL-X CTRL-N is not behaving as declared")
	return harness.ErrReported
}
