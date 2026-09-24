package p062

// Whim phase 62, the check -- no buffer-type, file-type, listing, jump, update-time or autowrite options.
// See phase/062/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim62", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim62", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  nobufopts    ", `\b%s\b`, check.GERE, check.GERE, true, nil, "b_p_bl", "b_p_bt", "b_p_ft", "p_bl", "p_bt", "p_ft", "p_jop", "jop_flags", "p_ut", "p_aw", "p_awa", "autowrite", "autowrite_all", "bt_dontwrite", "bt_dontwrite_msg",
		"bt_nofilename", "bt_nofileread", "bt_prompt", "set_buflisted", "did_set_buftype", "did_set_buflisted", "did_set_filetype_or_syntax",
		"do_filetype_autocmd", "b_did_filetype", "b_au_did_filetype", "CCGD_AW", "nofile_err",
		"before_blocking", "trigger_cursorhold", "updatescript", "ml_sync_all", "scriptout", "did_start_blocking") {
		return harness.ErrReported
	}
	s.Echo("  nobufopts    none of the seven options, the bt_ helpers or the autowrite path is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  nobufopts    ", "buflisted", "buftype", "filetype", "jumpoptions", "updatetime", "autowrite", "autowriteall") {
		return harness.ErrReported
	}
	f := filepath.Join(d, "w.txt")
	check.Put(f, "x\n")
	check.InD(d, "-e", "-s", "+1s/x/y/", "+w", "+q!", "w.txt")
	if check.CatS(f) != "y" {
		s.Echo("  nobufopts    :w did not write: '%s'", check.CatS(f))
		return harness.ErrReported
	}
	s.Echo("  nobufopts    :set sw works; the seven are unknown; :w still writes")
	return nil
}
