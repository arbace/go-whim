package p041

// Whim phase 41, the check -- the buffer list is walked by `:bnext` and `:bprevious` alone.
// See GOAL.md, its steps in internal/build's Plan, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim41", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim41", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  buflist      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "ex_buffer", "do_exbuffer", "buflist_list", "ex_bmodified", "ex_brewind", "ex_blast",
		"ex_bunload", "do_bufdel", "do_buffer", "ex_listdo") {
		return harness.ErrReported
	}
	calls := check.GrepC(s.Src(), `\bdo_buffer_ext\(`, check.GERE)
	gotos := check.GrepC(s.Src(), "do_buffer_ext(DOBUF_GOTO, ", check.GBRE)
	if calls != 3 || gotos != 1 {
		s.Echo("  buflist      do_buffer_ext has %d mentions and %d DOBUF_GOTO calls, expected 3 and 1", calls, gotos)
		return harness.ErrReported
	}
	nums, ls := check.GrepLines(s.Src(), `\bCMD_(buffer|bNext|badd|balt|bdelete|bfirst|blast|bmodified|brewind|buffers|files|ls|bufdo|bunload|bwipeout)\b`, check.GERE)
	tbl := check.Gre(`^[0-9]+:[ \t]*(\[CMD_|CMD_)`, check.GERE)
	var left []string
	for i := range nums {
		if l := fmt.Sprintf("%d:%s", nums[i], ls[i]); !tbl.MatchString(l) {
			left = append(left, l)
		}
	}
	if len(left) > 0 {
		s.Echo("  buflist      a retired buffer command is still named outside the table:")
		for _, l := range check.Head(left, 5) {
			s.Echo("               %s", check.CutC(l, 120))
		}
		return harness.ErrReported
	}
	if !s.KeptE("  buflist      ", " is gone, and :bnext or :bprevious needed it", "ex_bnext", "ex_bprevious", "goto_buffer") {
		return harness.ErrReported
	}
	s.Echo("  buflist      :bnext and :bprevious walk the list; nothing else does")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	return s.Phasebuild()
}
