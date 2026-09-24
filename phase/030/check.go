package p030

// Whim phase 30, the check -- `K` and the tag jumps, keeping `*` and `#`.
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

func init() { check.Register("whim30", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim30", args)
	if err != nil {
		return err
	}
	if !s.Gone("  ident        ", check.GBRE, check.GBRE, true, nil, "nv_K_getcmd", "do_nv_ident", "g_tag_at_cursor") {
		return harness.ErrReported
	}
	for _, g := range []string{"{'*', nv_ident", "{'#', nv_ident", "{POUND, nv_ident", "{Ctrl_RSB, nv_error", "{'K', nv_error"} {
		if check.GrepC(s.Src(), g, check.GFix) == 0 {
			s.Echo("  ident        %s went -- * and # are the half this phase keeps", g)
			return harness.ErrReported
		}
	}
	s.Echo("  ident        no keywordprg and no tag jump; * # and POUND still dispatch")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	if s.St("starcheck", filepath.Join(s.Work, "whim-vim")) {
		s.Echo("  ident        * still finds the next whole word, and skips foobar")
		return nil
	}
	s.Echo("  ident        * no longer searches -- it is the half this phase keeps")
	return harness.ErrReported
}
