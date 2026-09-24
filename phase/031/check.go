package p031

// Whim phase 31, the check -- file-name modifiers.
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

func init() { check.Register("whim31", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim31", args)
	if err != nil {
		return err
	}
	if n := check.GrepC(s.Src(), "modify_fname", check.GBRE); n != 0 {
		s.Echo("  fnamemod     modify_fname still has %d mentions after the sweep", n)
		return harness.ErrReported
	}
	if check.GrepC(s.Src(), `\beval_vars(`, check.GBRE) == 0 {
		s.Echo("  fnamemod     eval_vars went too -- %% and # are the half this keeps")
		return harness.ErrReported
	}
	s.Echo("  fnamemod     no suffix language; %% and # still expand")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d := s.Sub(".fmtest")
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	check.Put(filepath.Join(d, "sub", "f.txt"), "one\n")
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "w! copy.txt", "-c", "qa!", "sub/f.txt")
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "normal Gotwo", "-c", "w! %", "-c", "qa!", "sub/f.txt")
	t := strings.ReplaceAll(check.ReadFile(filepath.Join(d, "sub", "f.txt")), "\n", " ")
	os.RemoveAll(d)
	if t != "one two " {
		s.Echo("  fnamemod     `:w %%` gave '%s', expected 'one two '", t)
		return harness.ErrReported
	}
	s.Echo("  fnamemod     `:w %%` still writes the file being edited")
	return nil
}
