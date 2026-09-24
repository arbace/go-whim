package p066

// Whim phase 66, the check -- no sentences, paragraphs, sections, methods, #if blocks or comment blocks.
// See phase/066/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim66", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim66", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  nopara       ", `\b%s\b`, check.GERE, check.GERE, true, nil, "findsent", "findpar", "startPS", "current_sent", "current_par", "nv_brace", "nv_findpar") {
		return harness.ErrReported
	}
	if !s.KeptE("  nopara       ", " went too -- it was not the paragraph's", "findmatchlimit", "nv_bracket_block", "current_block", "current_word", "current_quote", "nv_percent", "getnextmark", "nv_brackets") {
		return harness.ErrReported
	}
	s.Echo("  nopara       no sentence, paragraph or section is left; brackets and words are")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	const sample = "One two. Three four.\n\nvoid f(void)\n{\n    if (x)\n    {\n        y;\n    }\n}\n#if A\n#endif\n"
	t := filepath.Join(d, "t.txt")
	for _, k := range []string{")ix", "(ix", "}ix", "{ix", "]]ix", "[[ix", "[mix", "]mix", "[#ix", "[/ix", "dapix", "disix"} {
		check.Put(t, sample)
		check.InD(d, "-e", "-s", "+5", "+normal! "+k, "+wq", "t.txt")
		if check.ReadFile(t) != sample {
			s.Echo("  nopara       %s changed the file:", k)
			for _, l := range check.Head(check.Lines(check.ReadFile(t)), 3) {
				s.Echo("               %s", l)
			}
			return harness.ErrReported
		}
	}
	check.Put(t, sample)
	check.InD(d, "-e", "-s", "+5", "+normal! ix", "+wq", "t.txt")
	if !check.GrepQ(check.ReadFile(t), `^    xif (x)$`, check.GBRE) {
		s.Echo("  nopara       the control insert failed")
		return harness.ErrReported
	}
	firstX := func() string { return strings.SplitN(check.GrepNum(t, "X", check.GBRE), "\n", 2)[0] }
	check.Put(t, sample)
	check.InD(d, "-e", "-s", "+7", "+normal! [{", "+s/^/X/", "+wq", "t.txt")
	if !check.GrepQ(check.ReadFile(t), `^X    {$`, check.GBRE) {
		s.Echo("  nopara       [{ no longer walks out: %s", firstX())
		return harness.ErrReported
	}
	check.Put(t, sample)
	check.InD(d, "-e", "-s", "+4", "+normal! %", "+s/^/X/", "+wq", "t.txt")
	if !check.GrepQ(check.ReadFile(t), `^X}$`, check.GBRE) {
		s.Echo("  nopara       %% no longer matches: %s", firstX())
		return harness.ErrReported
	}
	check.Put(t, sample)
	check.InD(d, "-e", "-s", "+7", "+normal! di{", "+wq", "t.txt")
	if !check.GrepQ(check.ReadFile(t), `^    {$`, check.GBRE) {
		s.Echo("  nopara       i{ took the enclosing braces too")
		return harness.ErrReported
	}
	if check.GrepQ(check.ReadFile(t), "y;", check.GBRE) {
		s.Echo("  nopara       i{ no longer selects a block -- y; survived")
		return harness.ErrReported
	}
	check.Put(t, sample)
	check.InD(d, "-e", "-s", "+'{,'}d", "+wq", "t.txt")
	if !check.GrepQ(check.ReadFile(t), `^One two\. Three four\.$`, check.GBRE) {
		s.Echo("  nopara       '{,'} still addressed a paragraph")
		return harness.ErrReported
	}
	s.Echo("  nopara       the cut keys do nothing; [{ and %% still move; i{ still selects; '{ is refused")
	return nil
}
