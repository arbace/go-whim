package p027

// Whim phase 27, the check -- `[[=a=]]` stops meaning "a with any accent".
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

func init() { check.Register("whim27", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim27", args)
	if err != nil {
		return err
	}
	if !s.Gone("  equiclass    ", check.GBRE, check.GBRE, false, nil, "reg_equi_class", "get_equi_class") {
		return harness.ErrReported
	}
	if !s.KeptCall("  equiclass    ", " went too -- [[:alpha:]] and [[.x.]] are different", "get_char_class", "get_coll_element") {
		return harness.ErrReported
	}
	s.Echo("  equiclass    [[=a=]] is gone; [[:alpha:]] and [[.x.]] are not")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d := s.Sub(".eqtest")
	check.Put(filepath.Join(d, "f.txt"), "xax\nx\303\241x\nx=x\n")
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "s/[[=a=]]/#/g", "-c", "wq", "f.txt")
	t := strings.ReplaceAll(check.ReadFile(filepath.Join(d, "f.txt")), "\n", " ")
	check.Put(filepath.Join(d, "g.txt"), "a1b\n")
	check.VimRC(d, "", "../whim-vim", "-e", "-s", "-c", "s/[[:alpha:]]/#/g", "-c", "wq", "g.txt")
	alpha := strings.ReplaceAll(check.ReadFile(filepath.Join(d, "g.txt")), "\n", " ")
	os.RemoveAll(d)
	if strings.Contains(t, "x#x") {
		s.Echo("  equiclass    [[=a=]] still matched an accented a")
		return harness.ErrReported
	}
	if alpha != "#1# " {
		s.Echo("  equiclass    [[:alpha:]] gave '%s', expected '#1# '", alpha)
		return harness.ErrReported
	}
	s.Echo("  equiclass    [[=a=]] is literal now, and [[:alpha:]] still classifies")
	return nil
}
