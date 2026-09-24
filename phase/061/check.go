package p061

// Whim phase 61, the check -- no window title.
// See phase/061/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim61", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim61", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  notitle      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "p_title", "p_titlelen", "p_titleold", "p_titlestring", "p_icon", "p_iconstring", "maketitle", "resettitle", "mch_settitle", "mch_restore_title",
		"set_title_defaults", "need_maketitle", "lasttitle", "lasticon", "oldtitle", "oldicon", "term_settitle", "term_push_title", "term_pop_title") {
		return harness.ErrReported
	}
	s.Echo("  notitle      nothing sets, restores, pushes or pops the terminal's title")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  notitle      ", "title", "titlelen", "titleold", "titlestring", "icon", "iconstring") {
		return harness.ErrReported
	}
	s.Echo("  notitle      :set sw works; the six title and icon options are unknown")
	return nil
}
