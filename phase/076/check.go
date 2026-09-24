package p076

// Whim phase 76, the check -- one regexp engine, so no retry.
// See phase/076/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim76", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim76", args)
	if err != nil {
		return err
	}
	const p = "  oneengine    "
	if !s.Cnt0(p, "AUTOMATIC_ENGINE", "p_re", "nfa_regprog_T", "nfa_state_T", "nfa_regengine", "regexp_engine") {
		return harness.ErrReported
	}
	if !s.KeptE(p, " went -- matching still needs it", "BACKTRACKING_ENGINE", "re_engine", "bt_regengine", "bt_regprog_T", "regprog_T", "regengine_T",
		"vim_regcomp", "vim_regfree", "vim_regexec_string", "vim_regexec_multi", "re_in_use", "re_flags") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `prog->re_engine = BACKTRACKING_ENGINE;`, check.GERE) {
		s.Echo("  oneengine    the engine is no longer recorded on the program")
		return harness.ErrReported
	}
	s.Echo("  oneengine    one engine, no retry; the matcher and its program types intact")
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
	for _, c := range []struct {
		file, content, want, What string
		Args                      []string
	}{
		{"s.txt", "alpha\nbeta\ngamma\n", "XlphX|betX|gXmmX|", "a quantified match broke", []string{`+%s/a\+/X/g`, "+wq"}},
		{"d.txt", "one1\ntwo2\nthree3\n", "oneN|twoN|threeN|", ":g over a pattern broke", []string{"+g/[0-9]$/s/[0-9]$/N/", "+wq"}},
		{"r.txt", "foo\nbar\nfoobar\n", "foo|bar|barfoo|", "back-references broke", []string{`+%s/\(foo\)\(bar\)/\2\1/`, "+wq"}},
		{"c.txt", "aaa\nbbb\n", "Za|Zb|", "a counted group broke", []string{`+%s/\%(a\|b\)\{2}/Z/`, "+wq"}},
		{"n.txt", "x\ny\n", "x|y-found|", "search broke", []string{"+/y", "+normal! A-found", "+wq"}},
	} {
		f := check.ProbeFile(d, c.file, c.content, c.Args...)
		if check.Bar(f) != c.want {
			s.Echo("  oneengine    %s: '%s'", c.What, check.Bar(f))
			return harness.ErrReported
		}
	}
	s.Echo("  oneengine    quantifiers, :g, back-references, counted groups and search all match")
	return nil
}
