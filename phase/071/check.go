package p071

// Whim phase 71, the check -- one buffer, structurally.
// See phase/071/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim71", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim71", args)
	if err != nil {
		return err
	}
	const p = "  onebuf       "
	if !check.GrepQ(s.Src(), `^[ \t]*buffblock_T \*b_next;`, check.GERE) {
		s.Echo("  onebuf       buffblock_T lost its b_next -- the redo buffer is gone")
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- that was never the buffer list", "bh_first", "redobuff", "readbuf1", "readbuf2") {
		return harness.ErrReported
	}
	if !s.Cnt0(p, "firstbuf", "lastbuf", "buf_reuse", "BLN_REUSE", "au_pending_free_buf") {
		return harness.ErrReported
	}
	if nums, ls := check.GrepLines(s.Src(), `\b(close_buffer|set_curbuf)\([^;]*DOBUF_WIPE_REUSE`, check.GERE); len(nums) > 0 {
		for i := range nums {
			s.Echo("%d:%s", nums[i], ls[i])
		}
		s.Echo("  onebuf       something still asks for a reusable wipe")
		return harness.ErrReported
	}
	if check.GrepQ(check.AwkRanges(s.Src(), `^struct file_buffer$`, `^\};$`), `buf_T[ \t]+\*b_(next|prev);`, check.GERE) {
		s.Echo("  onebuf       buf_T still links to another buffer")
		return harness.ErrReported
	}
	if !s.KeptE(p, " went too -- the one buffer still needs it", "buflist_new", "buflist_add", "buflist_findnr", "buf_hashtab", "win_alloc_firstwin") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `for \(bp = curbuf; ; bp = NULL\)`, check.GERE) {
		s.Echo("  onebuf       the mapping scan no longer runs its global pass")
		return harness.ErrReported
	}
	s.Echo("  onebuf       one buffer, structurally; the redo chain and the free list untouched")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.Loads(d, p, "a\nb\nc\n", "a|b|LAST c|") || !s.SwitchesE(d, p, "h1.txt", "h2.txt", "h1\n", "h2\n", "Eh2", "+normal! iE") ||
		!s.Edits(d, p, "e.txt", "one\ntwo\nthree\n", "one|three|") || !s.MapsLocal(d, p, "x\n", "x!") || !s.MapsGlobal(d, p) {
		return harness.ErrReported
	}
	alt := check.ProbeFile(d, "alt.txt", "z\n", "+normal! A1", "+wq")
	if check.CatS(alt) != "z1" {
		s.Echo("  onebuf       editing after the alternate-file cut broke: %s", check.CatS(alt))
		return harness.ErrReported
	}
	s.Echo("  onebuf       loads, edits, :e switches, and both mapping passes still fire")
	return nil
}
