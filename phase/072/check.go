package p072

// Whim phase 72, the check -- one window, one tabpage, structurally.
// See phase/072/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim72", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim72", args)
	if err != nil {
		return err
	}
	const p = "  onewin       "
	if !s.Cnt0(p, "firstwin", "lastwin", "first_tabpage", "w_next", "w_prev", "tp_next", "tp_firstwin", "tp_lastwin",
		"leave_tabpage", "enter_tabpage", "use_tabpage", "valid_tabpage", "win_append", "win_init",
		"borrow_stl_vsep_hl") {
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- the frame layer is not this phase", "topframe", "frame_T", "fr_next", "fr_child", "fr_parent", "new_frame", "win_comp_pos") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bb_nwindows\b`, check.GERE) {
		s.Echo("  onewin       b_nwindows went -- buffer release depends on it")
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `buf->b_nwindows <= 0`, check.GERE) {
		s.Echo("  onewin       the 'no longer displayed' test went")
		return harness.ErrReported
	}
	if !s.KeptE(p, " went too -- the one window still needs it", "win_alloc", "win_alloc_firstwin", "alloc_tabpage", "curwin", "curtab") {
		return harness.ErrReported
	}
	s.Echo("  onewin       one window and one tabpage; the frame layer and b_nwindows intact")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.Loads(d, p, "a\nb\nc\n", "a|b|LAST c|") || !s.Edits(d, p, "e.txt", "one\ntwo\nthree\n", "one|three|") ||
		!s.SwitchesE(d, p, "h1.txt", "h2.txt", "h1\n", "h2\n", "Eh2", "+normal! iE") || !s.MapsLocal(d, p, "x\n", "x!") || !s.MapsGlobal(d, p) {
		return harness.ErrReported
	}
	au := check.ProbeFile(d, "au.txt", "p1\np2\n", "+1", "+normal! A-au", "+wq")
	if check.Bar(au) != "p1-au|p2|" {
		s.Echo("  onewin       quitting through the rewritten BUFWINLEAVE block broke: '%s'", check.Bar(au))
		return harness.ErrReported
	}
	sc := check.ProbeFile(d, "s.txt", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", "+5", "+normal! O-ins", "+wq")
	if check.SedN(sc, 5) != "-ins" {
		s.Echo("  onewin       inserting a line broke the scroll path: %s", check.SedN(sc, 5))
		return harness.ErrReported
	}
	r := check.ProbeFile(d, "r.txt", "r1\nr2\nr3\n", "+%s/^r/R/", "+wq")
	if check.Bar(r) != "R1|R2|R3|" {
		s.Echo("  onewin       a whole-buffer range broke: '%s'", check.Bar(r))
		return harness.ErrReported
	}
	s.Echo("  onewin       loads, edits, :e switches, mappings fire, autocommands reach the buffer")
	return nil
}
