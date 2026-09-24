package p012

// Whim phase 12, the check -- UTF-8, and no other encoding, ever.
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

func init() { check.Register("whim12", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim12", args)
	if err != nil {
		return err
	}
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if check.SymNum("after") >= check.SymNum("before") {
		s.Echo("  symbols      this phase must lower the count")
		return harness.ErrReported
	}
	var left strings.Builder
	for _, l := range check.Lines(check.ReadFile(".cache/symbols/last/undefined")) {
		if strings.HasPrefix(l, "iconv") {
			left.WriteString(l + " ")
		}
	}
	if left.Len() > 0 {
		s.Echo("  iconv        still linked: %s", left.String())
		s.Echo("               a dependency that is never reached is still a dependency")
		return harness.ErrReported
	}
	s.Echo("  iconv        no longer linked at all")
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if enc := s.EncodingIn(".e.txt", ".e.out"); enc != "encoding=utf-8" {
		s.Echo("  encoding     got '%s', expected encoding=utf-8", enc)
		return harness.ErrReported
	}
	u := filepath.Join(s.Work, ".u.txt")
	check.Put(u, "\303\240\303\251\n")
	s.InWork("-u", "NONE", "-i", "NONE", "-e", "-s", "-c", "normal gUU", "-c", "wq", ".u.txt")
	ok := check.OdX(u)
	os.Remove(u)
	if ok != "c380c3890a" {
		s.Echo("  utf-8        gUU over 'a-grave e-acute' gave %s, expected c380c3890a", ok)
		s.Echo("               the editor is no longer handling UTF-8 as UTF-8")
		return harness.ErrReported
	}
	s.Echo("  utf-8        multibyte case conversion still works, and 'encoding' is utf-8")
	return nil
}
