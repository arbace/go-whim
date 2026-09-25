package p066

// Whim phase 66 -- no sentences, paragraphs, sections, methods, #if blocks or
// comment blocks.  See GOAL.md.
//
// One idea, cut at all three places it is reachable from:
//
// THE MOTIONS  ( and ) by sentence, { and } by paragraph, [[ ]] [] ][ by
// section, [m ]m [M ]M to a method's braces, [# ]# to the enclosing
// #if/#endif, and [/ ]/ [* ]* to the enclosing C comment.  The first four
// rows point at nv_error; the bracket ones go from nv_brackets() and
// nv_bracket_block().
// THE TEXT OBJECTS  is, as, ip and ap -- current_sent() and current_par().
// A sentence you cannot move over is not one you can select either.
// THE EX ADDRESSES  '{ '} '( ') as line addresses, which get_address() answered
// with findpar() and findsent().
//
// After which findsent(), findpar() and startPS() have no callers at all, and the
// concept is gone from the editor rather than merely unbound.
//
// WHAT STAYS, and is checked: % and the enclosing-bracket motions [{ ]} [( ]),
// which are findmatchlimit() rather than paragraphs; the ( ) { } [ ] TEXT OBJECTS
// i( a{ i[ and so on, which are current_block(); iw/aw; and the '[ '] '< '> marks,
// which get_address() answers from stored positions.
//
// THE DELTA: none the harnesses record -- no behaviour case moves over a sentence
// or a paragraph, and no Ex command changes.  The probes check each cut key does
// nothing, that [{ and % still move, and that i{ still selects.

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

// methodTest is `if (cap->nchar == 'm' || cap->nchar == 'M')`, which appears
// TWICE in nv_bracket_block: the head that picks the character to match, and
// the half that walks Out to the method.
const methodTest = `(?m)^[ \t]*if \(cap->nchar == 'm' \|\| cap->nchar == 'M'\)$`

// Whim66 takes the sentence, paragraph and section motions, the bracket
// commands that found a comment or a method, and the text objects for them.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nopara", text, w)

	for _, m := range []struct{ key, handler, What string }{
		{`\(`, "nv_brace", "( by sentence"},
		{`\)`, "nv_brace", ") by sentence"},
		{`\{`, "nv_findpar", "{ by paragraph"},
		{`\}`, "nv_findpar", "} by paragraph"},
	} {
		e.Sub(fmt.Sprintf(`(?m)^([ \t]*\{'%s', )%s(, 0, [^}]*\},)$`, m.key, m.handler),
			"${1}nv_error${2}", 1, fmt.Sprintf("%s points at nv_error", m.What))
	}
	e.InFunction("nv_brackets", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*else if \(cap->nchar == '\[' \|\| cap->nchar == '\]'\)$`, 1, "[[ ]] [] ][ by section")
	})
	e.Literal(`vim_strchr((char_u *)"{(*/#mM", cap->nchar)`, `vim_strchr((char_u *)"{(", cap->nchar)`, 1,
		"[ no longer taking a comment, #if or method")
	e.Literal(`vim_strchr((char_u *)"})*/#mM", cap->nchar)`, `vim_strchr((char_u *)"})", cap->nchar)`, 1,
		"] no longer taking a comment, #if or method")

	e.InFunction("nv_bracket_block", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(cap->nchar == '\*'\)$`, 1, "[* and ]* spelled as [/ and ]/")
		e.FoldAlways(`(?m)^[ \t]*if \(cap->nchar != 'm' && cap->nchar != 'M'\)$`, 1,
			"a miss beeping, which only a method did not")
		// The counted helpers cannot express "the second of two" -- they refuse
		// on any count but the one given -- so the walk-Out is cut from a slice
		// that STARTS at it, and only then is the head the single match the
		// counted fold wants.
		if !e.Failed() {
			re := regexp.MustCompile(methodTest)
			hits := re.FindAllIndex(e.Text(), -1)
			if len(hits) != 2 {
				e.Refuse("nv_bracket_block -- the method test matched %d times, expected 2", len(hits))
				return
			}
			t := e.Text()
			cut := edit.LastNewlineBefore(t, hits[1][0]) + 1
			tail, err := cutil.DropIf(t[cut:], methodTest, 1)
			if err != nil {
				e.Refuse("walking out to a method start or end -- %v", err)
				return
			}
			e.Set(append(append([]byte{}, t[:cut]...), tail...))
			e.Say("walking out to a method start or end")
		}
		e.FoldNever(methodTest, 1, "a method's braces choosing the character to match")
		e.Lines(`prev_pos\.lnum = 0;`, 1, "the previous match, which only a method walk-out read")
		e.Lines(`prev_pos = new_pos;`, 1, "remembering the previous match")
	})

	e.InFunction("nv_object", func(e *edit.E) {
		e.Cut(`(?m)^[ \t]*case 'p':\n[ \t]*flag = current_par\(cap->oap, cap->count1, include, 'p'\);\n[ \t]*break;\n`, 1,
			"ip and ap, the paragraph objects")
		e.Cut(`(?m)^[ \t]*case 's':\n[ \t]*flag = current_sent\(cap->oap, cap->count1, include\);\n[ \t]*break;\n`, 1,
			"is and as, the sentence objects")
	})

	e.FoldNever(`(?m)^[ \t]*else if \(c == '\{' \|\| c == '\}'\)$`, 1, "'{ and '} as line addresses")
	e.FoldNever(`(?m)^[ \t]*else if \(c == '\(' \|\| c == '\)'\)$`, 1, "'( and ') as line addresses")
	return e.Done()
}

func init() { edit.Register("whim66", Edit) }
