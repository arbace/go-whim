package p057

// Whim phase 57, the check -- no lisp.
// See phase/057/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim57", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim57", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  nolisp       ", `\b%s\b`, check.GERE, check.GERE, true, nil, "b_p_lisp", "p_lisp", "b_p_lw", "p_lispwords", "get_lisp_indent", "lisp_match", "use_indentexpr_for_lisp", "did_set_lisp", "lispcomm", "CPO_LISP", "BV_LISP", "BV_LW") {
		return harness.ErrReported
	}
	s.Echo("  nolisp       no lisp option, indenter, word list or match mode is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  nolisp       ", "lisp", "lispwords") {
		return harness.ErrReported
	}
	m := filepath.Join(d, "m.txt")
	check.Put(m, "(a ; b)\n")
	check.InD(d, "-e", "-s", "+1normal! 0%x", "+wq", "m.txt")
	if check.CatS(m) != "(a ; b" {
		s.Echo("  nolisp       %% across ';' left '%s'", check.CatS(m))
		return harness.ErrReported
	}
	s.Echo("  nolisp       :set sw works; lisp and lispwords are unknown; %% matches across ';'")
	return nil
}
