package p053

// Whim phase 53, the check -- no conversion layer, no 'encoding'.
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

func init() { check.Register("whim53", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim53", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  noconv       ", `\b%s\b`, check.GERE, check.GERE, true, nil, "need_conversion", "get_fio_flags", "check_for_bom", "next_fenc", "set_forced_fenc", "enc_canonize",
		"force_enc", "p_enc", "iconv_fd", "ucs2bytes", "mb_ptr2len", "mb_head_off", "mb_ptr2char", "latin_ptr2len",
		"dbcs_head_off", "dbcs_ptr2len", "get_encoding_name", "get_bad_name", "get_fileformat_name",
		"input_conv", "output_conv", "convert_setup", "string_convert", "convert_input_safe", "p_menc", "b_p_menc", "did_set_encoding") {
		return harness.ErrReported
	}
	s.Echo("  noconv       no conversion, no 'encoding', no mb_* pointer, no DBCS path")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if check.InD(d, "-e", "-s", "+set ts=3", "+q!") != 0 {
		s.Echo("  noconv       the control :set ts=3 failed")
		return harness.ErrReported
	}
	if check.InD(d, "-e", "-s", "+set enc?", "+q!") == 0 {
		s.Echo("  noconv       :set enc? was accepted")
		return harness.ErrReported
	}
	if check.InD(d, "-e", "-s", "+set menc?", "+q!") == 0 {
		s.Echo("  noconv       :set menc? was accepted")
		return harness.ErrReported
	}
	check.Put(filepath.Join(d, "e.txt"), "x\n")
	if check.InD(d, "-e", "-s", "+e ++enc=latin1 e.txt", "+q!") == 0 {
		s.Echo("  noconv       ++enc was accepted")
		return harness.ErrReported
	}
	u := filepath.Join(d, "u.txt")
	check.Put(u, "\303\240\303\251\n")
	check.InD(d, "-e", "-s", "+1normal! gUU", "+wq", "u.txt")
	if check.OdX(u) != "c380c3890a" {
		s.Echo("  noconv       gUU over a-grave e-acute gave %s", check.OdXs(u))
		return harness.ErrReported
	}
	ill := filepath.Join(d, "ill.txt")
	check.Put(ill, "ok\n\377 bad\n")
	check.InD(d, "-e", "-s", "+1s/ok/OK/", "+wq", "ill.txt")
	if check.OdC(ill) != `OK\n377bad\n` {
		s.Echo("  noconv       an invalid byte was not kept: %s", check.OdCs(ill))
		return harness.ErrReported
	}
	s.Echo("  noconv       :set enc, :set menc and ++enc refused; UTF-8 edits and a kept invalid byte unchanged")
	return nil
}
