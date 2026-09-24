package p011

// Whim phase 11, the check -- nothing is written that was not asked for.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim11", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim11", args)
	if err != nil {
		return err
	}
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d := s.Sub(".swtest")
	check.Put(filepath.Join(d, "f.txt"), "a\nb\n")
	check.VimRC(d, "", "../whim-vim", "-u", "NONE", "-i", "NONE", "-e", "-s", "-c", "normal ohello", "-c", "wq", "f.txt")
	sw := check.LsA(d)
	os.RemoveAll(d)
	if sw != "f.txt " {
		s.Echo("  swapfile     editing left: %s", sw)
		s.Echo("               expected f.txt alone -- something still writes beside the file")
		return harness.ErrReported
	}
	s.Echo("  swapfile     editing a file leaves the file, and nothing else")
	return nil
}
