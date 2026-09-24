package p051

// Whim phase 51, the check -- a byte that is not UTF-8 is kept as it is.
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

func init() { check.Register("whim51", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim51", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  keepbytes    ", `\b%s\b`, check.GERE, check.GERE, true, nil, "get_bad_opt", "bad_char_behavior", "b_bad_char", "BAD_REPLACE") {
		return harness.ErrReported
	}
	s.Echo("  keepbytes    nothing replaces, drops or chooses what to do with an invalid byte")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	u := filepath.Join(d, "u.txt")
	check.Put(u, "caf\303\251\n")
	check.InD(d, "-e", "-s", `+s/\%u00e9/E/`, "+wq", "u.txt")
	if check.CatS(u) != "cafE" {
		s.Echo("  keepbytes    the control failed: a UTF-8 edit gave '%s'", check.CatS(u))
		return harness.ErrReported
	}
	ill := filepath.Join(d, "ill.txt")
	check.Put(ill, "ok\n\377 bad\n")
	check.InD(d, "-e", "-s", "+1s/ok/OK/", "+wq", "ill.txt")
	if check.OdC(ill) != `OK\n377bad\n` {
		s.Echo("  keepbytes    an invalid byte was not written back unchanged: %s", check.OdCs(ill))
		return harness.ErrReported
	}
	Out, _ := check.VimOut(d, d, "./vim", true, "-e", "-s", "+set ro?", "+q!", "ill.txt")
	ro := strings.NewReplacer(" ", "", "\n", "").Replace(Out)
	if ro != "noreadonly" {
		s.Echo("  keepbytes    reading an invalid byte made the buffer '%s'", ro)
		return harness.ErrReported
	}
	if check.InD(d, "-e", "-s", "+e ++bad=keep ill.txt", "+q!") == 0 {
		s.Echo("  keepbytes    ++bad=keep was accepted")
		return harness.ErrReported
	}
	s.Echo("  keepbytes    an invalid byte is written back unchanged, the buffer stays writable, ++bad is refused")
	return nil
}
