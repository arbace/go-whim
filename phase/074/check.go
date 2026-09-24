package p074

// Whim phase 74, the check -- no file marks.
// See phase/074/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim74", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim74", args)
	if err != nil {
		return err
	}
	const p = "  nofmark      "
	if !s.Cnt0(p, "namedfm", "xfmark_T", "EXTRA_MARKS", "fname2fnum", "fmarks_check_names", "fmarks_check_one", "fm_getname", "buflist_getfile") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bfmark_T\b`, check.GERE) {
		s.Echo("  nofmark      fmark_T went -- the tag stack embeds it")
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `fmark_T[ \t]+fmark;`, check.GERE) {
		s.Echo("  nofmark      struct taggy lost its fmark")
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- that was never a file mark", "b_namedm", "b_last_cursor", "b_last_insert", "b_last_change", "b_op_start", "b_op_end",
		"w_pcmark", "w_prev_pcmark", "setpcmark", "getmark", "setmark", "setmark_pos", "check_mark",
		"clrallmarks", "mark_adjust_internal", "mark_col_adjust", "ex_marks", "ex_delmarks") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `do_join\(long[^)]*int[ \t]+setmark\)`, check.GERE) {
		s.Echo("  nofmark      do_join lost its setmark parameter")
		return harness.ErrReported
	}
	s.Echo("  nofmark      no file marks; lowercase, the special marks and the tag stack intact")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.Loads(d, p, "a\nb\nc\n", "a|b|LAST c|") || !s.Edits(d, p, "e.txt", "one\ntwo\nthree\n", "one|three|") {
		return harness.ErrReported
	}
	lo := check.ProbeFile(d, "lo.txt", "K1\nK2\nK3\n", "+normal! 2Gmb", "+normal! 3G", "+normal! 'bA-kept", "+wq")
	if check.Bar(lo) != "K1|K2-kept|K3|" {
		s.Echo("  nofmark      a lowercase mark stopped working: '%s'", check.Bar(lo))
		return harness.ErrReported
	}
	up := check.ProbeFile(d, "up.txt", "U1\nU2\nU3\n", "+normal! 1GmA", "+normal! 3G", "+normal! 'AA-up", "+wq")
	if check.Bar(up) != "U1|U2|U3|" {
		s.Echo("  nofmark      an uppercase mark still works: '%s'", check.Bar(up))
		return harness.ErrReported
	}
	bt := check.ProbeFile(d, "bt.txt", "B1\nB2\nB3\n", "+normal! 2Gmc", "+normal! 3G", "+normal! `cA-bt", "+wq")
	if check.Bar(bt) != "B1|B2-bt|B3|" {
		s.Echo("  nofmark      a backtick mark stopped working: '%s'", check.Bar(bt))
		return harness.ErrReported
	}
	mk := check.ProbeFile(d, "mk.txt", "M1\n", "+normal! 1Gma", "+marks", "+normal! A-mk", "+wq")
	if check.CatS(mk) != "M1-mk" {
		s.Echo("  nofmark      :marks broke the session: %s", check.CatS(mk))
		return harness.ErrReported
	}
	dm := check.ProbeFile(d, "dm.txt", "D1\n", "+normal! 1Gma", "+delmarks a", "+normal! A-dm", "+wq")
	if check.CatS(dm) != "D1-dm" {
		s.Echo("  nofmark      :delmarks broke the session: %s", check.CatS(dm))
		return harness.ErrReported
	}
	s.Echo("  nofmark      lowercase and backtick marks work; uppercase is unset; :marks and :delmarks run")
	return nil
}
