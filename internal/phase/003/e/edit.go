package p003e

// Whim phase 3e (formerly 66) -- no sentences, paragraphs, sections, methods, #if blocks or
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
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the four rows' handlers by
// RewriteAt, the bracket strings by RespellString, the folds and runs by form
// (history keeps the text version, and TestRowsSame, which held its row
// regexp on line windows to the whole text's).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// rows are the four keys of nv_cmds[] that go to nv_error.
var rows = []struct{ key, handler, What string }{
	{`'('`, "nv_brace", "( by sentence"},
	{`')'`, "nv_brace", ") by sentence"},
	{`'{'`, "nv_findpar", "{ by paragraph"},
	{`'}'`, "nv_findpar", "} by paragraph"},
}

// Edit takes the sentence, paragraph and section motions, the bracket
// commands that found a comment or a method, and the text objects for them.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nopara", e, w)

	v.InTable("nv_cmds", func(v *graph.Verbs) {
		for _, m := range rows {
			what := fmt.Sprintf("%s points at nv_error", m.What)
			if n := v.Count("(init " + m.key + " " + m.handler + " 0 _)"); n != 1 {
				v.Die("%s -- %d rows of %s with %s, expected 1", what, n, m.key, m.handler)
				return
			}
			v.RewriteAt("(init "+m.key+" ?h 0 _)", "h", "nv_error", 1, what)
		}
	})
	v.InFunction("nv_brackets", func(v *graph.Verbs) {
		v.FoldNever("(|| (== (-> cap nchar) '[') (== (-> cap nchar) ']'))", 1, "[[ ]] [] ][ by section")
	})
	v.RespellString(`"{(*/#mM"`, `"{("`, 1, "[ no longer taking a comment, #if or method")
	v.RespellString(`"})*/#mM"`, `"})"`, 1, "] no longer taking a comment, #if or method")

	v.InFunction("nv_bracket_block", func(v *graph.Verbs) {
		v.DropIf("(== (-> cap nchar) '*')", 1, "[* and ]* spelled as [/ and ]/")
		v.FoldAlways("(&& (!= (-> cap nchar) 'm') (!= (-> cap nchar) 'M'))", 1,
			"a miss beeping, which only a method did not")
		// Both tests are never true now, and the second has no else: one
		// counted fold takes the pair, keeping the first's else arm.
		v.FoldNever("(|| (== (-> cap nchar) 'm') (== (-> cap nchar) 'M'))", 2,
			"a method's braces choosing the character to match, and walking out to a method start or end")
		v.Cut("(= (. prev_pos lnum) 0)", 1, "the previous match, which only a method walk-out read")
		v.Cut("(= prev_pos new_pos)", 1, "remembering the previous match")
	})

	v.InFunction("nv_object", func(v *graph.Verbs) {
		v.CutRun("ip and ap, the paragraph objects", "(case 'p')", "(= flag (call current_par _ _ _ _))", "(break)")
		v.CutRun("is and as, the sentence objects", "(case 's')", "(= flag (call current_sent _ _ _))", "(break)")
	})

	v.FoldNever("(|| (== c '{') (== c '}'))", 1, "'{ and '} as line addresses")
	v.FoldNever("(|| (== c '(') (== c ')'))", 1, "'( and ') as line addresses")
	return v.Done()
}

func init() { phase.RegisterGraph("whim3e", Edit) }
