package text

import (
	"bytes"
	"fmt"
	"io"
)

// ph is the driver every Part II edit heredoc writes for itself at the top.
//
// IT IS THE SECOND DRIVER IN THIS PACKAGE, and that is a merge and not a plan.
// driver.go's `E` grew alongside the whim ports in one session while this grew
// alongside the zero ports in another; they collide in no name and differ in
// shape -- `E` accumulates its error and halts, `ph` returns one -- so both are
// here rather than one of the two being rewritten against the other's 40 call
// sites.  Since then E has become THE verb set (driver.go), and every phase
// that is a sequence of acts is written against it; Ph is left to the phases
// whose cut is a computation -- a partition derived from the text, a fixpoint,
// a rewrite of lines -- which thread the text through their own code and want
// an early return.  Its Literal takes its count before its `what`, as E's does.
//
// Each of those blocks opens with the same six helpers -- die, say, and some
// subset of in_function, fold_never, fold_always and literal -- around a TAG
// that is the phase's word in the report column.  That is not an idiom worth
// factoring Out of the PYTHON, where each copy is eight lines and lives beside
// the edit it serves; it is worth writing once here, because a Go file per
// phase would otherwise repeat it forty-two times.
//
// It is deliberately a near-copy of internal/cut's `ed` rather than a shared
// version of it: `ed` is reached by all 57 cutters, and hoisting it into a
// third package to save this file would edit every one of them to save
// forty lines.  If the two ever need to agree on something, agreeing by
// copy is cheaper here than agreeing by dependency.
//
// THE REPORT COLUMN IS THE PYTHON'S, exactly: `'  %-12s %s'`.  That is two
// spaces, the tag padded to twelve, one space, the message -- which is the
// same fifteen columns `ed` writes as `"  %-13s%s"`, and it is written the
// Python's way here so the two can be compared by eye against the heredoc
// they replace.
//
// ORDER IS OUTPUT.  Every helper prints as it succeeds, so the sequence of
// calls IS the sequence of lines.  A phase log is read by a human comparing
// two phase commits, and grouping edits of the same shape into a loop gives
// byte-identical trees and a differently-ordered report -- measured twice in
// the cutters, on oneoptset and onebuffer.  Follow the heredoc's order even
// where it looks arbitrary.
type Ph struct {
	Tag string
	W   io.Writer
}

func (p Ph) Say(what string) { fmt.Fprintf(p.W, "  %-12s %s\n", p.Tag, what) }

// die is the heredoc's `die()`: the phase refuses and writes nothing.
func (p Ph) Die(format string, a ...any) error {
	return fmt.Errorf("  %-12s %s", p.Tag, fmt.Sprintf(format, a...))
}

// literal replaces exact text, counted, and refuses on any other count.
//
// The count is the assertion.  These edits run on a tree every earlier phase
// has touched, so an anchor that has stopped matching means the phase is about
// to cut something other than what it was written to cut -- which is worth a
// refusal rather than a silent smaller cut.
func (p Ph) Literal(text []byte, old, new string, n int, what string) ([]byte, error) {
	k, norm := matchCount(text, old)
	if k != n {
		return nil, p.Die("%s -- occurs %d times, expected %d", what, k, n)
	}
	p.Say(what)
	return replaceMatched(text, old, new, k, norm), nil
}

// inFunction applies an edit to ONE function's text and splices it back, so a
// pattern that would match elsewhere in the file cannot.
func (p Ph) InFunction(text []byte, name string, edit func([]byte) ([]byte, error)) ([]byte, error) {
	a, z, ok := FindDefinition(text, Blank(text), name)
	if !ok {
		return nil, p.Die("%s is not defined", name)
	}
	seg, err := edit(text[a:z])
	if err != nil {
		return nil, err
	}
	Out := make([]byte, 0, len(text)-(z-a)+len(seg))
	Out = append(Out, text[:a]...)
	Out = append(Out, seg...)
	return append(Out, text[z:]...), nil
}

// mentions counts a name as a WHOLE WORD, which is how these blocks state
// every invariant about an identifier.  `main_loop`, `main_errors`, `vim_main2`
// and `domain` are different words from `main`, and \b does not match inside
// them -- which is the whole reason the counts in these phases mean anything.
func (p Ph) Mentions(text []byte, name string) int { return MentionCount(text, name) }

// blankRuns counts runs of two blank lines.
//
// CLAUDE.md: no verification tier can see a blank line, so a phase that could
// leave one states the count before and after and compares.  This is the
// Python's exact rule -- an empty line whose predecessor is also empty -- and
// not "paragraphs", so three blank lines count as two runs.
func (p Ph) BlankRuns(text []byte) int {
	lines := bytes.Split(text, []byte{'\n'})
	n := 0
	for i := 1; i < len(lines); i++ {
		if len(lines[i]) == 0 && len(lines[i-1]) == 0 {
			n++
		}
	}
	return n
}

// assertOnce requires a literal to occur exactly once and changes nothing.
// The heredocs use it to pin an anchor before cutting and again afterwards, so
// it carries the phase's reason as well as its count.
func (p Ph) AssertOnce(text []byte, s, what, why string) error {
	k, _ := matchCount(text, s)
	if k != 1 {
		return p.Die("%s occurs %d times, expected 1 -- %s", what, k, why)
	}
	return nil
}

// lines is len(text.split('\n')), which is one more than the number of
// newlines -- the Python's count, not "lines of content".
func (p Ph) Lines(text []byte) int {
	return bytes.Count(text, []byte{'\n'}) + 1
}

// sayf is say with a format, which several phases need for a count they have
// just computed.
func (p Ph) Sayf(format string, a ...any) { p.Say(fmt.Sprintf(format, a...)) }

// swapOnce is assertOnce followed by one replacement, and it reports NOTHING.
// Several heredocs define exactly this and then say one summarising line at
// the end rather than one per edit -- so a driver that printed here would add
// lines the Python does not have, and ORDER IS OUTPUT covers absence as well
// as order.
func (p Ph) SwapOnce(text []byte, old, new, what, why string) ([]byte, error) {
	if err := p.AssertOnce(text, old, what, why); err != nil {
		return nil, err
	}
	return bytes.Replace(text, []byte(old), []byte(new), 1), nil
}
