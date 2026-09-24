package p067

// Whim phase 67, the check -- no mouse, no spell plumbing, no write-only flags.
// See phase/067/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim67", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim67", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  nomouse      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "is_mouse_key", "dragwin", "held_button", "mouse_row", "mouse_col", "old_mouse_row", "old_mouse_col",
		"reset_dragwin", "reset_held_button", "spellvars_T", "spv_has_spell",
		"did_check_timestamps", "did_emsg_syntax", "typebuf_was_empty", "in_mch_delay", "frame_locked",
		"swap_exists_did_quit", "did_swapwrite_msg", "autocmd_nested", "oldtitle_outdated", "deadly_signal",
		"mr_patternlen", "was_safe", "state_no_longer_safe", "mouse_index_found", "looks_like_mouse_start") {
		return harness.ErrReported
	}
	const keys = `\(char_u \*\)\("\w*(Mouse|Drag|Release|Wheel)\w*"\)`
	if n := check.GrepC(s.Src(), keys, check.GERE); n != 0 {
		s.Echo("  nomouse      %d mouse key names left in key_names_table", n)
		nums, ls := check.GrepLines(s.Src(), keys, check.GERE)
		for i := 0; i < len(nums) && i < 3; i++ {
			s.Echo("%s", check.CutC(fmt.Sprintf("%d:%s", nums[i], ls[i]), 110))
		}
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bvim_ignored\b`, check.GBRE) {
		s.Echo("  nomouse      vim_ignored went -- warn_unused_result has nothing to assign to")
		return harness.ErrReported
	}
	if n := check.GrepC(check.AwkRanges(s.Src(), `nv_cmds\[\] =`, `^\};`), `KE_(MOUSE|LEFT|MIDDLE|RIGHT|X1|X2)|SCROLLBAR|TABLINE|TABMENU`, check.GERE); n != 26 {
		s.Echo("  nomouse      %d mouse rows left in nv_cmds, expected 26 kept at nv_error", n)
		return harness.ErrReported
	}
	s.Echo("  nomouse      no mouse, spell plumbing or write-only flag is left; the nv_cmds rows are")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	t := check.ProbeFile(d, "t.txt", "alpha\nbeta\ngamma\n", "+2", "+normal! dd", "+wq")
	if check.Bar(t) != "alpha|gamma|" {
		s.Echo("  nomouse      editing broke: '%s'", check.Bar(t))
		return harness.ErrReported
	}
	if check.InD(d, "-e", "-s", "+map <Home> x", "+q!") != 0 {
		s.Echo("  nomouse      mapping a real key name broke")
		return harness.ErrReported
	}
	s.Echo("  nomouse      editing works; a real key name still maps")
	return nil
}
