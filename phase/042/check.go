package p042

// Whim phase 42, the check -- one buffer, always.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim42", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim42", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  onebuffer    ", `\b%s\b`, check.GERE, check.GERE, true, nil, "buf_hide", "p_hid", "b_p_bh", "setaltfname", "buflist_altfpos", "w_alt_fnum", "goto_buffer",
		"do_buffer_ext", "ex_bnext", "ex_bprevious", "nv_hat", "did_set_bufhidden") {
		return harness.ErrReported
	}
	const cm = `cmod_flags (&|\|=) CMOD_(KEEPALT|HIDE)\b`
	if check.GrepQ(s.Src(), cm, check.GERE) {
		s.Echo("  onebuffer    :keepalt or :hide is still set or tested after the sweep")
		nums, ls := check.GrepLines(s.Src(), cm, check.GERE)
		for i := 0; i < len(nums) && i < 3; i++ {
			s.Echo("               %s", check.CutC(fmt.Sprintf("%d:%s", nums[i], ls[i]), 100))
		}
		return harness.ErrReported
	}
	calls := check.GrepC(s.Src(), `\bbuflist_new\(`, check.GERE)
	s.Echo("  onebuffer    buflist_new is named %d times: %d calls", calls, check.GrepC(s.Src(), `^[ \t]+[^ \t].*\bbuflist_new\(`, check.GERE))
	s.Echo("  onebuffer    nothing hides, nothing is the alternate, and a buffer left is wiped")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	a := filepath.Join(d, "a")
	check.Put(a, "one\ntwo\nthree\n")
	check.Put(filepath.Join(d, "b"), "other\n")
	check.VimRC(d, "", "./vim", "-e", "-s", "+2", "+mark a", "+e b", "+e a", "+'ad", "+w", "+q!", "a")
	if check.CatS(a) != "one\ntwo\nthree" {
		s.Echo("  onebuffer    a mark survived leaving its file -- the buffer was kept, not wiped")
		for _, l := range check.Lines(check.ReadFile(a)) {
			s.Echo("               %s", l)
		}
		return harness.ErrReported
	}
	if check.VimRC(d, "", "./vim", "-e", "-s", "+e b", "+e #", "+q!", "a") == 0 {
		s.Echo("  onebuffer    :e # succeeded, so there is still an alternate file")
		return harness.ErrReported
	}
	check.Put(a, "one\n")
	check.VimRC(d, "", "./vim", "-e", "-s", "+saveas c", "+s/$/X/", "+w", "+q!", "a")
	if check.CatS(a) != "one" || check.CatS(filepath.Join(d, "c")) != "oneX" {
		s.Echo("  onebuffer    :saveas did not rename the buffer: a=%s c=%s", check.CatS(a), check.CatS(filepath.Join(d, "c")))
		return harness.ErrReported
	}
	s.Echo("  onebuffer    a mark goes with its file, :e # is refused, :saveas renames")
	return nil
}
