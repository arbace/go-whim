package p060

// Whim phase 60, the check -- no suffix, case, delay, verbose-file, debug or filter-program options.
// See phase/060/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim60", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim60", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  nosixopts    ", `\b%s\b`, check.GERE, check.GERE, true, nil, "p_su", "match_suffix", "p_fic", "p_acl", "delay_pending", "acl_elapsed", "p_vfile", "verbose_fd", "verbose_open", "verbose_stop", "verbose_enter", "verbose_leave",
		"p_debug", "p_fp", "b_p_fp", "p_ep", "b_p_ep", "get_equalprg", "did_set_verbosefile", "did_set_debug") {
		return harness.ErrReported
	}
	s.Echo("  nosixopts    none of the seven options or their readers is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  nosixopts    ", "suffixes", "fileignorecase", "autocompletedelay", "verbosefile", "debug", "formatprg", "equalprg") {
		return harness.ErrReported
	}
	g := filepath.Join(d, "g.txt")
	check.Put(g, "aaa bbb\n")
	check.InD(d, "-e", "-s", "+set tw=4", "+1normal! gqq", "+wq", "g.txt")
	if check.Bar(g) != "aaa|bbb|" {
		s.Echo("  nosixopts    gqq with tw=4 left '%s'", check.Bar(g))
		return harness.ErrReported
	}
	s.Echo("  nosixopts    :set sw works; the seven are unknown; gq still formats internally")
	return nil
}
