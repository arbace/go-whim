package p070

// Whim phase 70, the check -- :e reloads in place, and there is no swap file.
// See phase/070/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim70", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim70", args)
	if err != nil {
		return err
	}
	if check.GrepQ(s.FnBody(`^fname2fnum\(`), `\bbuflist_new\b`, check.GERE) {
		s.Echo("  onebuffer    fname2fnum can still create a buffer")
		return harness.ErrReported
	}
	if n := check.GrepC(s.Src(), `\bfname2fnum\b`, check.GERE); n != 3 {
		s.Echo("  onebuffer    fname2fnum has %d mentions, expected 3 (prototype, call, definition)", n)
		return harness.ErrReported
	}
	ecmd := s.FnBody(`^do_ecmd\(`)
	if check.GrepQ(ecmd, `\bbuflist_new\b|\bbuflist_findnr\b`, check.GERE) {
		s.Echo("  onebuffer    do_ecmd still reaches for another buffer")
		return harness.ErrReported
	}
	if check.GrepQ(ecmd, `close_buffer\(curwin, curbuf, DOBUF_WIPE`, check.GERE) {
		s.Echo("  onebuffer    do_ecmd still wipes the buffer it leaves")
		return harness.ErrReported
	}
	if !s.KeptE("  onebuffer    ", " went too -- the one buffer still needs it", "buflist_new", "setfname", "open_buffer", "buf_freeall", "readfile", "do_ecmd") {
		return harness.ErrReported
	}
	s.Echo("  onebuffer    :edit reuses the one buffer; nothing creates or wipes another")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	const p = "  onebuffer    "
	if !s.Loads(d, p, "a\nb\nc\n", "a|b|LAST c|") || !s.SwitchesE(d, p, "h1.txt", "h2.txt", "h1\n", "h2\n", "Eh2", "+normal! iE") ||
		!s.Edits(d, p, "e.txt", "one\ntwo\nthree\n", "one|three|") {
		return harness.ErrReported
	}
	r := check.ProbeFile(d, "r.txt", "keep\n", "+normal! iX", "+e!", "+wq")
	if check.CatS(r) != "keep" {
		s.Echo("  onebuffer    :e! did not reload: %s", check.CatS(r))
		return harness.ErrReported
	}
	s.Echo("  onebuffer    the file loads and edits; :e opens another; :e! reloads")
	return nil
}
