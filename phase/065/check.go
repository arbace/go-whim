package p065

// Whim phase 65, the check -- no rot13, no operator function, no empty key handler.
// See phase/065/edit.go, and GOALS.md.
//
// A transcription of the shell check it replaced, written against internal/check's
// Wsh (shell.go) and what more than one Part I check uses (partone.go).

import (
	"io"
	"path/filepath"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim65", Check) }

func Check(w io.Writer, args []string) error {
	s, err := check.NewWsh(w, "whim65", args)
	if err != nil {
		return err
	}
	if !s.GoneW("  norot13      ", `\b%s\b`, check.GERE, check.GERE, true, nil, "OP_ROT13", "op_function", "OP_FUNCTION", "ins_ctrl_x", "e_eval_feature_not_available") {
		return harness.ErrReported
	}
	if !s.KeptE("  norot13      ", " went too -- it was not rot13's", "swapchar", "op_tilde", "OP_TILDE", "OP_UPPER", "OP_LOWER", "nv_search", "get_op_type") {
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `^[ \t]*case Ctrl_P:`, check.GERE) {
		s.Echo("  norot13      Insert-mode CTRL-P lost its case and would insert a control character")
		return harness.ErrReported
	}
	if !check.GrepQ(s.Src(), `\bcase 'y':`, check.GERE) {
		s.Echo("  norot13      zy went with the operators")
		return harness.ErrReported
	}
	s.Echo("  norot13      no rot13, operator function or empty key handler is left")
	if err := s.Phasecheck(); err != nil {
		return err
	}
	if err := s.Phasebuild(); err != nil {
		return err
	}
	d, done := s.Scratch()
	defer done()
	t := filepath.Join(d, "t.txt")
	for _, p := range [][2]string{{"g?g?", "abc def|ghi|"}, {"g??", "abc def|ghi|"}, {"g@g@", "abc def|ghi|"},
		{"gUU", "ABC DEF|ghi|"}, {"guu", "abc def|ghi|"}, {"g~~", "ABC DEF|ghi|"}, {"zyy", "abc def|ghi|"}} {
		check.Put(t, "abc def\nghi\n")
		check.InD(d, "-e", "-s", "+1", "+normal! "+p[0], "+wq", "t.txt")
		if got := check.Bar(t); got != p[1] {
			s.Echo("  norot13      %s gave '%s', expected '%s'", p[0], got, p[1])
			return harness.ErrReported
		}
	}
	s.Echo("  norot13      g? and g@ do nothing; gU, gu and g~ still change case; zy still yanks")
	return nil
}
