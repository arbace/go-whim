package p028

// Whim phase 28, the check -- C indenting.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim28", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim28", args)
	if err != nil {
		return err
	}
	if !s.Gone("  cindent      ", check.GBRE, check.GBRE, true, nil, "get_c_indent", "in_cinkeys", "do_c_expr_indent", "b_p_cin", "b_p_cino", "p_cinw") {
		return harness.ErrReported
	}
	if !s.KeptCall("  cindent      ", " went too -- 'lisp' and 'autoindent' are not this", "get_lisp_indent", "get_indent") {
		return harness.ErrReported
	}
	s.Echo("  cindent      no C syntax model; 'autoindent' and 'lisp' untouched")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d := s.Sub(".citest")
	check.Put(filepath.Join(d, "f.txt"), "    one\n")
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "set autoindent", "-c", "normal Gotwo", "-c", "wq", "f.txt")
	ai := ""
	if l := check.SedN(filepath.Join(d, "f.txt"), 2); l != "" || len(check.Lines(check.ReadFile(filepath.Join(d, "f.txt")))) >= 2 {
		check.Put(filepath.Join(d, ".l2"), l+"\n")
		ai = strings.TrimSuffix(check.CatA(filepath.Join(d, ".l2")), "\n")
	}
	os.RemoveAll(d)
	if ai != "    two$" {
		s.Echo("  autoindent   a new line gave '%s', expected four spaces then two", ai)
		return harness.ErrReported
	}
	if s.ProbeSet("autoindent") != 0 {
		s.Echo("  options      the control failed")
		return harness.ErrReported
	}
	if s.ProbeSet("cindent") == 0 {
		s.Echo("  options      :set cindent was accepted")
		return harness.ErrReported
	}
	s.Echo("  autoindent   still indents; :set cindent is refused")
	return nil
}
