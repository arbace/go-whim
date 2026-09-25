package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/text"
)

// ed is the shape most cutters share: a tool name, a writer, and the counted
// edits, which refuse in that tool's words.
//
// THE EDITS ARE THE VERB SET'S.  This was a third copy of it, after edit.E and
// edit.Ph; what it does now is the part that is the cutters' own -- the report
// column and the refusal's wording -- around crefactor/text's counted
// acts (counted.go) and folds, the ones E and Ph are written on.  A refusal's
// reason is theirs, word for word, and the line in front of it is the tool's.
//
// The report column is the same in every tool -- two spaces, the name padded
// to thirteen -- so it is written once here rather than per message.
//
// ORDER IS OUTPUT.  Every one of these prints a line as it succeeds, so the
// sequence of calls IS the sequence of lines, and a Go port that groups edits
// of the same shape into a loop -- which they invite, being four one-line
// literal replacements in four different functions -- emits the same lines in
// a different order and the comparison differs on every input that cuts.  It
// happened twice: oneoptset's sixteen edits and onebuffer's four.  Follow the
// Python's order even where it looks arbitrary.
type ed struct {
	tool string
	w    io.Writer
}

func (e ed) say(what string) { fmt.Fprintf(e.w, "  %-13s%s\n", e.tool, what) }

// refuse is the tool's refusal of an act, with the verb's reason.
func (e ed) refuse(what string, err error) error {
	return fmt.Errorf("%s: %s -- %v", e.tool, what, err)
}

// done reports an act that succeeded, or refuses one that did not.
func (e ed) done(out []byte, err error, what string) ([]byte, error) {
	if err != nil {
		return nil, e.refuse(what, err)
	}
	e.say(what)
	return out, nil
}

// literal replaces exact text, counted.
func (e ed) literal(seg []byte, old, new, what string, count int) ([]byte, error) {
	out, err := text.ReplaceLiteral(seg, old, new, count)
	return e.done(out, err, what)
}

// subOnce deletes a pattern that must match exactly once.
func (e ed) subOnce(seg []byte, pattern, what string) ([]byte, error) {
	return e.subCount(seg, pattern, what, 1)
}

// foldNever folds a condition that is now always false.
func (e ed) foldNever(seg []byte, pattern, what string) ([]byte, error) {
	out, err := text.FoldNever(seg, "(?m)"+pattern, 1)
	return e.done(out, err, what)
}

// foldAlways folds a condition that is now always true.
func (e ed) foldAlways(seg []byte, pattern, what string) ([]byte, error) {
	out, err := text.FoldAlways(seg, "(?m)"+pattern, 1)
	return e.done(out, err, what)
}

// dropIf deletes an `if` and the block it guards, after COUNTING it, in the
// tool's own words before the fold's.
func (e ed) dropIf(seg []byte, pattern, what string) ([]byte, error) {
	n := len(regexp.MustCompile("(?m)"+pattern).FindAll(seg, -1))
	if n != 1 {
		return nil, fmt.Errorf("%s: %s -- the condition occurs %d times, expected 1",
			e.tool, what, n)
	}
	return e.dropIfUncounted(seg, pattern, what)
}

// subCount deletes a pattern that must match exactly `count` times.
func (e ed) subCount(seg []byte, pattern, what string, count int) ([]byte, error) {
	out, err := text.ReplacePattern(seg, "(?m)"+pattern, "", count)
	return e.done(out, err, what)
}

// subCountRepl replaces a pattern `count` times with a replacement that may
// expand $1.
func (e ed) subCountRepl(seg []byte, pattern, repl, what string, count int) ([]byte, error) {
	out, err := text.ReplacePattern(seg, pattern, repl, count)
	return e.done(out, err, what)
}

// dropIfUncounted is dropIf without the tool's count, for a caller whose
// Python passes straight to cutil.drop_if and reports its ValueError: the
// fold counts for itself, in its own words.
func (e ed) dropIfUncounted(seg []byte, pattern, what string) ([]byte, error) {
	out, err := text.DropIf(seg, "(?m)"+pattern, 1)
	return e.done(out, err, what)
}

// inFunction applies an edit to ONE function's text and splices it back, so a
// pattern that would match elsewhere in the file cannot.
func (e ed) inFunction(t []byte, name string, edit func([]byte) ([]byte, error)) ([]byte, error) {
	out, found, err := text.InDefinition(t, name, edit)
	if !found {
		return nil, fmt.Errorf("%s: %s is not defined at file scope", e.tool, name)
	}
	return out, err
}

// linesMatchingUnless is RE2's answer to a negative lookahead at line start.
//
// Go's regexp has no lookahead of either sign, by design, so a Python pattern
// like `^(?![ \t]*\[?CMD_)[^\n]*\bCMD_x\b` cannot be transcribed.  What it
// means is "a line that names CMD_x and is not itself a table row", and that
// is two tests over the lines rather than one pattern.
func linesMatchingUnless(text []byte, want, unless *regexp.Regexp) []string {
	var out []string
	for _, line := range bytes.Split(text, []byte{'\n'}) {
		if want.Match(line) && !unless.Match(line) {
			out = append(out, string(line))
		}
	}
	return out
}
