package p050

// Whim phase 50, the check -- only LF text files.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim50", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim50", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  lfonly       ", `\b%s\b`, check.GERE, check.GERE, true, nil, "get_fileformat", "get_fileformat_force", "set_fileformat", "default_fileformat", "file_ff_differs",
		"save_file_ff", "set_file_options", "set_options_bin", "msg_add_fileformat", "check_ff_value",
		"force_ff", "force_bin", "p_ffs", "p_bin", "b_p_bin", "b_p_ff", "b_p_eol", "b_p_fixeol", "b_p_eof", "b_p_tx",
		"b_start_ffc", "b_start_eol", "b_start_eof", "b_no_eol_lnum", "try_mac", "try_dos", "write_bin") {
		return harness.ErrReported
	}
	s.Echo("  lfonly       no format, no binary mode, no end-of-line option")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	Out, rc := check.VimOut(d, "", "./vim", true, "-b", "-e", "-s", "+q!")
	if !strings.Contains(Out, "Unknown option argument") {
		s.Echo("  lfonly       -b is not refused as unknown (exit %d): %s", rc, Out)
		return harness.ErrReported
	}
	if check.VimRC(d, "", "./vim", "-e", "-s", "+set ts=3", "+q!") != 0 {
		s.Echo("  lfonly       the control :set ts=3 failed")
		return harness.ErrReported
	}
	if check.VimRC(d, "", "./vim", "-e", "-s", "+set ff=dos", "+q!") == 0 {
		s.Echo("  lfonly       :set ff=dos was accepted")
		return harness.ErrReported
	}
	c := filepath.Join(d, "crlf.txt")
	check.Put(c, "one\r\ntwo\r\n")
	check.VimRC(d, "", "./vim", "-e", "-s", "+%s/$/X/", "+wq", "crlf.txt")
	if check.OdC(c) != `one\rX\ntwo\rX\n` {
		s.Echo("  lfonly       a CR LF file was not edited as LF text: %s", check.OdCs(c))
		return harness.ErrReported
	}
	n := filepath.Join(d, "noeol.txt")
	check.Put(n, "one\ntwo")
	check.VimRC(d, "", "./vim", "-e", "-s", "+w", "+q!", "noeol.txt")
	if check.OdC(n) != `one\ntwo\n` {
		s.Echo("  lfonly       a last line was written without LF: %s", check.OdCs(n))
		return harness.ErrReported
	}
	s.Echo("  lfonly       -b unknown, ff refused, CR is text, and every line ends with LF")
	return nil
}
