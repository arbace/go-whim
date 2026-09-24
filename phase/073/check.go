package p073

// Whim phase 73, the check -- one frame.
// See phase/073/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim73", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim73", args)
	if err != nil {
		return err
	}
	const p = "  oneframe     "
	if !s.Cnt0(p, "fr_parent", "fr_next", "fr_prev", "fr_child", "frame_fixed_height", "frame_fixed_width", "FR_ROW", "FR_COL") {
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- the one frame still needs it", "frame_T", "topframe", "fr_width", "fr_height", "fr_win", "fr_layout", "FR_LEAF", "new_frame",
		"win_comp_pos", "frame_comp_pos", "frame_minheight", "win_new_height") {
		return harness.ErrReported
	}
	mh := s.FnBody(`^frame_minheight\(`)
	if !check.GrepQ(mh, `\bp_wmh\b`, check.GERE) || !check.GrepQ(mh, `\bp_wh\b`, check.GERE) {
		s.Echo("  oneframe     frame_minheight lost its arithmetic")
		return harness.ErrReported
	}
	s.Echo("  oneframe     one frame; its width, height and the minima kept")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.Loads(d, p, "a\nb\nc\n", "a|b|LAST c|") || !s.Edits(d, p, "e.txt", "one\ntwo\nthree\n", "one|three|") ||
		!s.SwitchesE(d, p, "h1.txt", "h2.txt", "h1\n", "h2\n", "Eh2", "+normal! iE") || !s.MapsLocal(d, p, "x\n", "x!") {
		return harness.ErrReported
	}
	sc := check.ProbeFile(d, "s.txt", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", "+5", "+normal! O-ins", "+wq")
	if check.SedN(sc, 5) != "-ins" {
		s.Echo("  oneframe     inserting a line broke the scroll path: %s", check.SedN(sc, 5))
		return harness.ErrReported
	}
	c := check.ProbeFile(d, "c.txt", "c1\nc2\n", "+set cmdheight=2", "+1", "+normal! A-ch", "+wq")
	if check.Bar(c) != "c1-ch|c2|" {
		s.Echo("  oneframe     :set cmdheight broke the layout: '%s'", check.Bar(c))
		return harness.ErrReported
	}
	l := check.ProbeFile(d, "l.txt", "l1\nl2\n", "+set laststatus=2", "+1", "+normal! A-ls", "+wq")
	if check.Bar(l) != "l1-ls|l2|" {
		s.Echo("  oneframe     'laststatus' broke the layout: '%s'", check.Bar(l))
		return harness.ErrReported
	}
	s.Echo("  oneframe     loads, edits, :e switches, mappings fire, cmdheight and laststatus resize")
	return nil
}
