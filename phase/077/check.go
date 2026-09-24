package p077

// Whim phase 77, the check -- no buffer-name argument matching.
// See phase/077/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim77", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim77", args)
	if err != nil {
		return err
	}
	const p = "  nobufpat     "
	if !s.Cnt0(p, "buflist_findpat", "file_pat_to_reg_pat", "buflist_match") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bEX_BUFNAME\b`, check.GERE) {
		s.Echo("  nobufpat     EX_BUFNAME went -- the rows and the count check still name it")
		return harness.ErrReported
	}
	// `ni = (!((int)(ea.cmdidx) < 0) && …` -- the printer closed up the space
	// the residue had after the `!`.  Measured: the residue spelling occurs
	// once in slim-vim.c and this one occurs once here, at the same site.
	if !check.GrepQ(s.Src(), `^[ \t]*ni = \(!\(\(int\)\(ea\.cmdidx\) < 0\)`, check.GERE) {
		s.Echo("  nobufpat     do_one_cmd no longer computes ni")
		return harness.ErrReported
	}
	if n := check.GrepC(s.Src(), `!ni\b`, check.GERE); n < 7 {
		s.Echo("  nobufpat     only %d checks still consult ni, expected at least 7", n)
		return harness.ErrReported
	}
	Body := s.FnBody(`^do_one_cmd\(`)
	if !check.GrepQ(Body, `^doend:$`, check.GERE) {
		s.Echo("  nobufpat     do_one_cmd lost its shared exit label")
		return harness.ErrReported
	}
	if g := check.GrepC(Body, `goto doend;`, check.GERE); g < 5 {
		s.Echo("  nobufpat     only %d gotos target doend, expected many -- the label may have been orphaned", g)
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- the Ex dispatcher still needs it", "do_one_cmd", "find_ex_command", "ex_ni", "cmdnames") {
		return harness.ErrReported
	}
	s.Echo("  nobufpat     no buffer-name matching; ni, EX_BUFNAME and the doend label intact")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.Loads(d, p, "a\nb\nc\n", "a|b|LAST c|") {
		return harness.ErrReported
	}
	wt := check.ProbeFile(d, "w.txt", "p1\np2\n", "+1", "+normal! A-w", "+wq")
	if check.Bar(wt) != "p1-w|p2|" {
		s.Echo("  nobufpat     writing broke: '%s'", check.Bar(wt))
		return harness.ErrReported
	}
	if !s.SwitchesE(d, p, "e1.txt", "e2.txt", "e1\n", "e2\n", "e2-E", "+normal! A-E") {
		return harness.ErrReported
	}
	g := check.ProbeFile(d, "g.txt", "k1\ndrop\nk2\n", "+g/drop/d", "+wq")
	if check.Bar(g) != "k1|k2|" {
		s.Echo("  nobufpat     :g broke: '%s'", check.Bar(g))
		return harness.ErrReported
	}
	z := check.ProbeFile(d, "z.txt", "z\n", "+buffer nosuchname", "+normal! A-ok", "+wq")
	if check.CatS(z) != "z-ok" {
		s.Echo("  nobufpat     :buffer with a name broke the session: %s", check.CatS(z))
		return harness.ErrReported
	}
	s.Echo("  nobufpat     loads, writes, :e names a file, :g takes a pattern, :buffer still refused")
	return nil
}
