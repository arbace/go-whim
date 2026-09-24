package p059

// Whim phase 59, the check -- no command-line completion.
// See phase/059/edit.go, and GOALS.md.
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

func init() { check.Register("whim59", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim59", args)
	if err != nil {
		return err
	}
	if callers := check.GrepC(s.Src(), `\bExpandOne\(`, check.GERE); callers != 3 {
		s.Echo("  nocompletion ExpandOne is named %d times, expected 3 (prototype, definition, expand_filename)", callers)
		nums, ls := check.GrepLines(s.Src(), `\bExpandOne\(`, check.GERE)
		for i := range nums {
			s.Echo("%s", check.CutC(fmt.Sprintf("%d:%s", nums[i], ls[i]), 120))
		}
		return harness.ErrReported
	}
	if !s.GoneW("  nocompletion ", `\b%s\b`, check.GERE, check.GERE, true, nil, "p_wc", "p_wcm", "p_wim", "p_wop", "p_wig", "p_wic", "wim_flags", "nextwild", "showmatches", "cmdline_wildchar_complete", "set_expand_context",
		"set_one_cmd_context", "ExpandSettings", "ExpandMappings", "ExpandBufnames", "expand_argopt", "get_next_or_prev_match",
		"find_longest_match", "did_wild_list", "check_opt_wim") {
		return harness.ErrReported
	}
	s.Echo("  nocompletion no completion key, context, match list or wild* option is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	if !s.UnknownOpts(d, "  nocompletion ", "wildchar", "wildcharm", "wildmode", "wildoptions", "wildignore", "wildignorecase") {
		return harness.ErrReported
	}
	o := filepath.Join(d, "onlyone.txt")
	check.Put(o, "x\n")
	check.InD(d, "-e", "-s", "+e onlyone.txt", "+1s/x/y/", "+w", "+q!")
	if check.CatS(o) != "y" {
		s.Echo("  nocompletion :e onlyone.txt did not edit it: '%s'", check.CatS(o))
		return harness.ErrReported
	}
	s.Echo("  nocompletion :set sw works; the wild* options are unknown; :e still edits a named file")
	return nil
}
