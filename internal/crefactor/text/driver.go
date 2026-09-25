package text

import (
	"fmt"
	"io"
	"regexp"
)

// An E is one phase's edit in progress: the tree, the tag its report lines
// carry, and the first error that stopped it.
//
// THE TRANSFORMATIONS DO NOT COLLAPSE AND THE SCAFFOLDING DOES.  Measured over
// all 225 heredocs, 94 are bespoke drivers over cutil and exactly one is a
// shape another shares -- so there is no cut to factor Out.  But those 94
// drivers each open with the same twenty lines: a die() that prefixes the
// phase's tag, an in_function() that splices one definition back, a count check
// that refuses on anything but the expected number, and fold_never/drop_if/sub
// wrappers that print the act after it succeeds.  That is what this is.  A port
// then reads as the phase's argument and nothing else, which is the only way
// 33,000 lines of these are reviewable.
//
// ERRORS ACCUMULATE rather than returning at every step, so a port reads like
// the Python it replaces -- a sequence of acts, not a sequence of `if err !=
// nil`.  The first failure stops the rest: every later act would be operating
// on a tree the previous one did not produce, and its complaint would describe
// a state that never existed.
//
// ORDER IS OUTPUT.  Each act prints when it succeeds and not before, in the
// order it was asked for, because a phase program's log is what a human reads
// when comparing two phase commits.
//
// THE VERB SET, and its one rule: an act that could match more than once
// takes the number of times it applies, just before the `what` it reports,
// and refuses on any other count; an act on one thing -- a span, a
// definition, a block -- refuses unless there is exactly one.
//
//	text        Literal(old, new, n, what)
//	regex       Sub(re, repl, n, what)         Cut(re, n, what)
//	            Lines(re, n, what)
//	if          FoldNever(re, n, what)         FoldAlways(re, n, what)
//	            DropIf(re, n, what)            FoldAlwaysElse(re, n, what)
//	span        Splice(from, through, with, what)
//	definition  Body(fn, body, what)           DeleteDefinition(fn, what)
//	block       DropBlocks(fn, re, n, what)    ReplaceBlock(fn, re, repl, what)
//	            DropBareBlock(fn, stmt, what)
//	walk        FoldWalk(fn, v, to, head, n, what)
//	            DropWalk(fn, head, repl, n, what)
//	            FoldWalks(re, ok, subst, what)
//
// A count is written, never inferred: these run on a tree every earlier phase
// has touched, and "however many there are" carries a moved anchor silently
// into the boundary.  The ASSERTIONS change nothing and report nothing:
// CountIs(re, n, what), Expect(ok, format, ...) and ConstOf(fn, value).  The
// QUERIES answer from the tree as it stands: Mentions(name), Query(re, group),
// BodyOf(fn) and InnerBody(fn).  The SCOPES are InFunction(fn, acts) and
// InTable(head, acts).  Text and Set are the escape hatch, for a phase whose
// cut is a computation no verb says.
type E struct {
	Tag string
	buf []byte
	W   io.Writer
	Err error
}

// New starts an edit that reports under tag.
func New(tag string, text []byte, w io.Writer) *E {
	return &E{Tag: tag, buf: text, W: w}
}

// Done returns the rewritten tree, or the first error.
func (e *E) Done() ([]byte, error) {
	if e.Err != nil {
		return nil, e.Err
	}
	return e.buf, nil
}

// Text is the tree as it now stands, for an act this file does not cover.
func (e *E) Text() []byte { return e.buf }

// Set replaces the tree, for the same reason.
func (e *E) Set(text []byte) { e.buf = text }

// Failed says whether an act has already refused.
func (e *E) Failed() bool { return e.Err != nil }

func (e *E) Die(format string, a ...any) {
	if e.Err == nil {
		e.Err = fmt.Errorf("  %-12s %s", e.Tag, fmt.Sprintf(format, a...))
	}
}

// Say reports an act, and says nothing once the edit has failed: a line after
// the refusal would describe a tree that is never written.
func (e *E) Say(what string) {
	if e.Err == nil {
		fmt.Fprintf(e.W, "  %-12s %s\n", e.Tag, what)
	}
}

// Refuse stops the edit with a message of the phase's own wording, for the
// assertions that are not a count of a pattern -- "do_exedit mentions n 4 times
// after the title went, expected 3 (declaration, readonlymode save and
// restore)".  CountIs would say the right thing about the wrong subject.
func (e *E) Refuse(format string, a ...any) { e.Die(format, a...) }

// Refused records the message and returns it, so a raw check reads as one line.
func (e *E) Refused(format string, a ...any) error {
	e.Die(format, a...)
	_, err := e.Done()
	return err
}

// Expect refuses, in the phase's own words, unless ok: the one-line form of
// `if got != want { refuse }`, for an invariant that is not the count of a
// pattern -- a name's mentions, the shape of the file's end.  It is skipped once
// the edit has failed, like every act, so the first refusal is the one reported.
func (e *E) Expect(ok bool, format string, a ...any) {
	if e.Err == nil && !ok {
		e.Die(format, a...)
	}
}

// CountIs refuses unless the pattern matches exactly n times.  It reports
// nothing: it is an assertion about the tree and not an act upon it.
func (e *E) CountIs(pattern string, n int, what string) {
	if e.Err != nil {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.Die("%s -- %v", what, err)
		return
	}
	if k := len(re.FindAll(e.buf, -1)); k != n {
		e.Die("%s -- matched %d times, expected %d", what, k, n)
	}
}

// Mentions counts whole-word occurrences of a name in the tree as it stands.
func (e *E) Mentions(name string) int { return MentionCount(e.buf, name) }

// Query returns the given capture group of every match of the pattern, in
// order: the names a phase computes from the text rather than lists, and
// reports as it cuts them.
func (e *E) Query(pattern string, group int) []string {
	var out []string
	for _, m := range regexp.MustCompile(pattern).FindAllSubmatch(e.buf, -1) {
		out = append(out, string(m[group]))
	}
	return out
}

// Sub rewrites the pattern's n matches, refusing on any other count.
func (e *E) Sub(pattern, repl string, n int, what string) {
	if e.Err != nil {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.Die("%s -- %v", what, err)
		return
	}
	if k := len(re.FindAll(e.buf, -1)); k != n {
		e.Die("%s -- matched %d times, expected %d", what, k, n)
		return
	}
	e.buf = re.ReplaceAll(e.buf, []byte(repl))
	e.Say(what)
}

// Cut deletes the pattern's n matches, refusing on any other count.
func (e *E) Cut(pattern string, n int, what string) { e.Sub(pattern, "", n, what) }

// Lines deletes n whole lines matching the pattern, which is Cut with the
// indentation and the newline supplied -- `^[ \t]*<pattern>\n`.  Most of what
// these phases remove is a statement on a line of its own, and writing that
// wrapper at every call site is where a missing `^` or a missing `\n` turns a
// line deletion into a text deletion that leaves a blank behind.
func (e *E) Lines(pattern string, n int, what string) {
	e.Cut(`(?m)^[ \t]*`+pattern+`\n`, n, what)
}

// Literal replaces the n occurrences of old with new, refusing on any other
// count.  It is Sub without a regular expression, for the many places where the
// C being matched is full of parentheses and stars and the pattern would be
// mostly backslashes.  The count is stated because these edits run on a tree
// every earlier phase has touched: a phrase that has gone from two occurrences
// to one has had a reader removed somewhere else, and rewriting "however many
// there are" would carry that silently into the boundary.
func (e *E) Literal(old, new string, n int, what string) {
	if e.Err != nil {
		return
	}
	k, norm := matchCount(e.buf, old)
	if k != n {
		e.Die("%s -- occurs %d times, expected %d", what, k, n)
		return
	}
	e.buf = replaceMatched(e.buf, old, new, n, norm)
	e.Say(what)
}

// FoldNever takes the n `if`s the pattern matches whose condition can no longer
// be true, keeping any else arm; DropIf takes ones whose condition is now
// always true and that have no else, keeping nothing; FoldAlways keeps the
// then arm of those.  Each refuses unless the pattern matches exactly n times,
// because these run on a tree every earlier phase has touched and an anchor
// that has stopped matching means the phase is about to fold something else.
//
// With n above one, the first match is folded n times over (cutil's own
// count).  The drivers used to keep a second shape, folding the LAST match each
// time, because the two differed by blank lines before the canonical print;
// every boundary is printed canonically now, and the two give the same phase
// output at every site that used either (measured on phases 60, 62, 64 and 65).
func (e *E) FoldNever(pattern string, n int, what string) {
	e.fold(pattern, n, what, FoldNever)
}

// FoldAlways is FoldNever's twin for the then arm; see there.
func (e *E) FoldAlways(pattern string, n int, what string) {
	e.fold(pattern, n, what, FoldAlways)
}

// DropIf is FoldNever's twin for a test that is now always true and guards
// nothing the tree still needs; see there.
func (e *E) DropIf(pattern string, n int, what string) {
	e.fold(pattern, n, what, DropIf)
}

func (e *E) fold(pattern string, n int, what string, f func([]byte, string, int) ([]byte, error)) {
	if e.Err != nil {
		return
	}
	e.CountIs(pattern, n, what)
	if e.Err != nil {
		return
	}
	out, err := f(e.buf, pattern, n)
	if err != nil {
		e.Die("%s -- %v", what, err)
		return
	}
	e.buf = out
	e.Say(what)
}

// FoldAlwaysElse turns `if (TRUE) { A } else { B }` into A, n times.
// FoldAlways refuses an else arm, because a condition that has become
// always-true makes its else unreachable and deciding that is this verb's
// business, not a fold's.  A block with no else refuses, and so does an else
// followed by another else.
func (e *E) FoldAlwaysElse(pattern string, n int, what string) {
	if e.Err != nil {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.Die("%s -- %v", what, err)
		return
	}
	if ms := re.FindAllIndex(e.buf, -1); len(ms) != n {
		e.Die("%s -- %d matches, expected %d", what, len(ms), n)
		return
	}
	for i := 0; i < n; i++ {
		t := e.buf
		b := Blank(t)
		k, o, c, head, err := Guarded(t, b, re.FindIndex(t))
		if err != nil {
			e.Die("%s -- %v", what, err)
			return
		}
		if head != "if" {
			e.Die("%s -- not a plain if: %s", what, PyRepr(head))
			return
		}
		end := IndexFrom(t, []byte("\n"), c) + 1
		m := elseLine.FindIndex(t[end:])
		if m == nil {
			e.Die("%s -- expected an else", what)
			return
		}
		o2 := IndexFrom(b, []byte("{"), end+m[1])
		if o2 < 0 {
			e.Die("%s -- expected an else", what)
			return
		}
		c2 := Match(b, o2)
		after := IndexFrom(t, []byte("\n"), c2) + 1
		if elseWord.Match(t[after:]) {
			e.Die("%s -- the else is followed by another else", what)
			return
		}
		body := t[IndexFrom(t, []byte("\n"), o)+1 : LastNewlineBefore(t, c)+1]
		out := append([]byte{}, t[:k]...)
		out = append(out, body...)
		e.buf = append(out, t[after:]...)
	}
	e.Say(what)
}

var (
	elseLine = regexp.MustCompile(`^[ \t]*else[ \t]*\n`)
	elseWord = regexp.MustCompile(`^[ \t]*else\b`)
)

// InFunction runs the acts against ONE file-scope definition's Body and splices
// it back, which is what every one of these heredocs spells in_function().
// Scoping matters: a pattern that is unique inside one function is very often
// not unique in a 180,000-line file, and a count that passes for the wrong
// reason is the failure this whole construct exists to prevent.
func (e *E) InFunction(name string, acts func(*E)) {
	if e.Err != nil {
		return
	}
	a, z, ok := FindDefinition(e.buf, Blank(e.buf), name)
	if !ok {
		e.Die("%s is not defined at file scope", name)
		return
	}
	inner := &E{Tag: e.Tag, buf: e.buf[a:z], W: e.W}
	acts(inner)
	if inner.Err != nil {
		e.Err = inner.Err
		return
	}
	out := append([]byte{}, e.buf[:a]...)
	out = append(out, inner.buf...)
	e.buf = append(out, e.buf[z:]...)
}

// InTable is InFunction for a file-scope table: the acts run against one
// initialised object, from its head -- `static struct vimoption options[] =`,
// literal, which must occur exactly once -- through the brace that closes its
// initialiser, found by matching braces outside strings, and are spliced back.
// A pattern over a table's rows is as easily matched outside it, by a call or
// a prototype, as a pattern in a function's body is.
func (e *E) InTable(head string, acts func(*E)) {
	if e.Err != nil {
		return
	}
	if k := countBytes(e.buf, head); k != 1 {
		e.Die("%s occurs %d times, expected 1", PyRepr(head), k)
		return
	}
	a := IndexFrom(e.buf, []byte(head), 0)
	b := Blank(e.buf)
	o := IndexFrom(b, []byte("{"), a+len(head))
	c := -1
	if o >= 0 {
		c = Match(b, o)
	}
	if c < 0 {
		e.Die("%s -- no initialiser, or an unbalanced one", PyRepr(head))
		return
	}
	z := IndexFrom(e.buf, []byte("\n"), c) + 1
	inner := &E{Tag: e.Tag, buf: e.buf[a:z], W: e.W}
	acts(inner)
	if inner.Err != nil {
		e.Err = inner.Err
		return
	}
	out := append([]byte{}, e.buf[:a]...)
	out = append(out, inner.buf...)
	e.buf = append(out, e.buf[z:]...)
}

// Splice replaces a span matched by its two ENDS -- from the start of from
// through the end of through, both literal C -- with `with`.  Each end must
// occur exactly once, from first; anything else refuses.
//
// It is for a span whose middle shares no shape with its ends: a test and the
// last line of the body it guards, hundreds of lines of unrelated cases.  One
// regular expression over the whole span would match anything; two anchors,
// each counted, match what the phase was written against or refuse.  The ends
// are literal because on canonical text a line is spelled one way, its
// indentation included.
//
// TWO NARROW SPLICES ARE OFTEN RIGHT WHERE ONE WIDE ONE IS WRONG, which is
// whim68's lesson: a single cut from aucmd_win[]'s search through `curbuf =
// buf;` also swallows aco->save_curwin_id and aco->save_prevwin_id, which the
// surviving else branch reads back through win_find_by_id() -- and it would
// have COMPILED, restoring from uninitialised stack.
func (e *E) Splice(from, through, with, what string) {
	if e.Err != nil {
		return
	}
	for _, end := range []struct{ s, which string }{{from, "its start"}, {through, "its end"}} {
		if k := countBytes(e.buf, end.s); k != 1 {
			e.Die("%s -- %s occurs %d times, expected 1", what, end.which, k)
			return
		}
	}
	a := IndexFrom(e.buf, []byte(from), 0)
	z := IndexFrom(e.buf, []byte(through), 0) + len(through)
	if z-len(through) < a+len(from) {
		e.Die("%s -- its end comes before its start", what)
		return
	}
	out := append([]byte{}, e.buf[:a]...)
	out = append(out, with...)
	e.buf = append(out, e.buf[z:]...)
	e.Say(what)
}

// Body replaces a function's whole Body with the given text.
//
// The whole Body and not an early return: leaving the old Body behind leaves
// unreachable code that NO WARNING NAMES.  gcc reports an unused local and says
// nothing about a loop that can never run, so the old statements would stay --
// dead code that looks deliberate.
func (e *E) Body(name, newBody, what string) {
	e.InFunction(name, func(e *E) {
		if e.Failed() {
			return
		}
		i := IndexFrom(e.buf, []byte("{\n"), 0)
		if i < 0 {
			e.Die("%s -- no body", name)
			return
		}
		head := append([]byte{}, e.buf[:i+2]...)
		head = append(head, newBody...)
		e.buf = append(head, []byte("}\n")...)
		e.Say(what)
	})
}

// DeleteDefinition removes a file-scope definition and says so, refusing if it
// is not there.  The refusal names the function, because "not defined" about a
// function the phase is removing on purpose is the case where an earlier phase
// has already taken it and this one is about to claim work it did not do.
func (e *E) DeleteDefinition(name, what string) {
	if e.Failed() {
		return
	}
	out, gone := DeleteDefinition(e.buf, name)
	if !gone {
		e.Refuse("%s is not defined", name)
		return
	}
	e.buf = out
	e.Say(what)
}

// DropBlocks deletes a brace-matched block n times, anchored on the line that
// opens it.
//
// Brace matching rather than a line pattern, because these bodies are
// macro-expanded one-liners hundreds of characters wide -- transcribing them is
// exactly what killed whim74's first attempt, and what genlits.py exists to
// stop.
func (e *E) DropBlocks(fn, anchorRe string, n int, what string) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		rx := regexp.MustCompile(anchorRe)
		if k := len(rx.FindAll(e.buf, -1)); k != n {
			e.Die("%s -- the anchor matches %d times, expected %d", what, k, n)
			return
		}
		for i := 0; i < n; i++ {
			m := rx.FindIndex(e.buf)
			b := Blank(e.buf)
			k0 := LastNewlineBefore(e.buf, m[0]) + 1
			o := IndexFrom(e.buf, []byte("{"), m[0])
			c := Match(b, o)
			if c < 0 {
				e.Die("%s -- unbalanced block", what)
				return
			}
			out := append([]byte{}, e.buf[:k0]...)
			e.buf = append(out, e.buf[IndexFrom(e.buf, []byte("\n"), c)+1:]...)
		}
	})
	if !e.Failed() {
		e.Say(what)
	}
}

// BodyOf returns the text of a file-scope definition, or "" and false.
func (e *E) BodyOf(name string) ([]byte, bool) {
	a, z, ok := FindDefinition(e.buf, Blank(e.buf), name)
	if !ok {
		return nil, false
	}
	return e.buf[a:z], true
}

func LastNewlineBefore(text []byte, i int) int {
	for j := i - 1; j >= 0; j-- {
		if text[j] == '\n' {
			return j
		}
	}
	return -1
}
