package p058

// Whim phase 58, the check -- no language mappings.
// See phase/058/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim58", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim58", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  nolangmap    ", `\b%s\b`, check.GERE, check.GERE, true, nil, "b_p_iminsert", "b_p_imsearch", "p_iminsert", "p_imsearch", "B_IMODE_NONE", "B_IMODE_LMAP", "B_IMODE_LAST", "MODE_LANGMAP",
		"ins_ctrl_hat", "cmdline_toggle_langmap", "set_iminsert_global", "set_imsearch_global", "did_set_iminsert", "did_set_imsearch",
		"get_keymap_str", "b_im_ptr", "langmap_active") {
		return harness.ErrReported
	}
	s.Echo("  nolangmap    no language-mapping mode, toggle or option is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  nolangmap    ", "iminsert", "imsearch") {
		return harness.ErrReported
	}
	if check.InD(d, "-e", "-s", "+lmap a b", "+q!") == 0 {
		s.Echo("  nolangmap    :lmap was accepted")
		return harness.ErrReported
	}
	h := filepath.Join(d, "h.txt")
	check.Put(h, "x\n")
	check.InD(d, "-e", "-s", "+1normal! Ia\036b", "+wq", "h.txt")
	if check.CatS(h) != "abx" {
		s.Echo("  nolangmap    CTRL-^ in Insert mode left '%s'", check.CatS(h))
		return harness.ErrReported
	}
	s.Echo("  nolangmap    :set sw works; iminsert, imsearch and :lmap are unknown; CTRL-^ inserts nothing")
	return nil
}
