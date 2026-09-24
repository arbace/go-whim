package p063

// Whim phase 63, the check -- no jump list.
// See phase/063/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim63", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim63", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  nojumplist   ", `\b%s\b`, check.GERE, check.GERE, true, nil, "w_jumplist", "w_jumplistlen", "w_jumplistidx", "movemark", "cleanup_jumplist", "copy_jumplist", "free_jumplist", "ex_jumps", "ex_clearjumps") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), "w_pcmark", check.GBRE) {
		s.Echo("  nojumplist   w_pcmark went too -- the '' mark was not the jump list's")
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), "movechangelist", check.GBRE) {
		s.Echo("  nojumplist   movechangelist went too -- g; and g, were not the jump list's")
		return harness.ErrReported
	}
	s.Echo("  nojumplist   no jump list is left; the '' mark and the change list are")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if check.InD(d, "-e", "-s", "+jumps", "+q!") == 0 {
		s.Echo("  nojumplist   :jumps was accepted")
		return harness.ErrReported
	}
	j := filepath.Join(d, "j.txt")
	check.Put(j, "a\nb\nc\n")
	check.InD(d, "-e", "-s", "+1", "+normal! 3G", "+normal! \017", "+s/^/X/", "+wq", "j.txt")
	if check.Bar(j) != "a|b|Xc|" {
		s.Echo("  nojumplist   CTRL-O moved the cursor: '%s'", check.Bar(j))
		return harness.ErrReported
	}
	k := filepath.Join(d, "k.txt")
	check.Put(k, "a\nb\nc\n")
	check.InD(d, "-e", "-s", "+1", "+normal! 3G''", "+s/^/Y/", "+wq", "k.txt")
	if check.Bar(k) != "Ya|b|c|" {
		s.Echo("  nojumplist   '' did not return to line 1: '%s'", check.Bar(k))
		return harness.ErrReported
	}
	s.Echo("  nojumplist   :jumps is refused; CTRL-O stays put; '' still jumps back")
	return nil
}
