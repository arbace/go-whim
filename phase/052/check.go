package p052

// Whim phase 52, the check -- UTF-8 is not a question.
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

func init() { check.Register("whim52", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim52", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  utf8only     ", `\b%s\b`, check.GERE, check.GERE, true, nil, "enc_utf8", "has_mbyte", "enc_dbcs", "enc_unicode", "enc_latin1like", "__T__", "__F__", "__Z__") {
		return harness.ErrReported
	}
	s.Echo("  utf8only     no encoding flag is asked, and %d DBCS call sites are left", check.GrepC(s.Src(), `\bdbcs_[a-z_0-9]+\(`, check.GERE))
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	u := filepath.Join(d, "u.txt")
	check.Put(u, "\303\240\303\251\n")
	check.InD(d, "-e", "-s", "+1normal! gUU", "+wq", "u.txt")
	if check.OdX(u) != "c380c3890a" {
		s.Echo("  utf8only     gUU over a-grave e-acute gave %s", check.OdXs(u))
		return harness.ErrReported
	}
	x := filepath.Join(d, "x.txt")
	check.Put(x, "a\346\227\245b\n")
	check.InD(d, "-e", "-s", "+1normal! 0lx", "+wq", "x.txt")
	if check.CatS(x) != "ab" {
		s.Echo("  utf8only     x on a three-byte character left '%s'", check.CatS(x))
		return harness.ErrReported
	}
	s.Echo("  utf8only     gUU and x work on multibyte characters")
	return nil
}
