package p057

// Whim phase 57 -- no lisp.  See GOAL.md.
//
// 'lisp' and 'lispwords' go, and with them everything they switched on:
// get_lisp_indent() for autoindent, =, gq and new lines; lisp_match() over
// 'lispwords'; '-' as a keyword character; ';' line comments in check_linecomment();
// and findmatchlimit()'s lisp mode, which stopped % at a ';' comment and skipped
// #\( and #\[ character literals.  'lispoptions' went in Phase 55.
//
// b_p_lisp is folded as FALSE at every reader rather than stubbed, so each branch
// it guarded is either gone or taken unconditionally.
//
// THE DELTA: none the harnesses record -- no case sets 'lisp'.  The probes check
// the two options are unknown and that % still matches across a ';'.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// Whim57 takes lisp mode: the indenting, the ';' comment leader, and the ten
// places `%` knew about a lisp comment.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nolisp", text, w)

	e.InFunction("open_line", func(e *edit.E) {
		e.DropIf(edit.Head("if (leader == NULL && !use_indentexpr_for_lisp() && curbuf->b_p_lisp && curbuf->b_p_ai)"), 1,
			"a new line taking its indent from get_lisp_indent()")
		e.Cut(edit.Line("if (!p_paste)", "{", "}"), 1,
			"open_line's now-empty 'paste' test")
	})
	e.InFunction("buf_init_chartab", func(e *edit.E) {
		e.DropIf(edit.Head("if (buf->b_p_lisp)"), 1, "'-' as a keyword character")
	})
	e.InFunction("check_linecomment", func(e *edit.E) {
		e.FoldNever(edit.Head("if (curbuf->b_p_lisp)"), 1, "a ';' starting a line comment")
	})
	e.InFunction("op_reindent", func(e *edit.E) {
		e.FoldAlways(edit.Head("if (i != oap->line_count - 1 || oap->line_count == 1 || how != get_lisp_indent)"), 1,
			"= skipping the last line only for lisp")
	})
	e.InFunction("fix_indent", func(e *edit.E) {
		e.DropIf(edit.Head("if (curbuf->b_p_lisp && curbuf->b_p_ai)"), 1, "fix_indent re-indenting lisp")
	})
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.DropIf(edit.Head("if (curbuf->b_p_lisp)"), 1, "= indenting lisp")
	})
	e.InFunction("format_lines", func(e *edit.E) {
		e.FoldNever(edit.Head("else if (curbuf->b_p_lisp)"), 1, "gq indenting lisp")
	})

	// findmatchlimit carried a lisp comment state through the whole scan, so
	// this is eight acts rather than one: six conditions that named its two
	// locals and two that named b_p_lisp directly.  The locals themselves,
	// named by nothing after that, go to the sweep.
	e.InFunction("findmatchlimit", func(e *edit.E) {
		e.Literal("if ((backwards && comment_dir) || lisp || skip_comments)",
			"if ((backwards && comment_dir) || skip_comments)", 1,
			"% looking for a comment only for a comment direction or FM_SKIPCOMM")
		e.DropIf(edit.Head("if (lisp && comment_col != MAXCOL && pos.col > (colnr_T)comment_col)"), 1,
			"% starting inside a lisp comment")
		e.DropIf(edit.Head("if (lispcomm && pos.col < (colnr_T)comment_col)"), 1,
			"% stopping at a lisp comment backwards")
		e.Literal("if (comment_dir || lisp || skip_comments)", "if (comment_dir || skip_comments)", 1,
			"% rescanning a line for lisp")
		e.FoldNever(edit.Head("if (lisp && comment_col != MAXCOL)"), 1,
			"% jumping to a lisp comment backwards")
		e.Literal("if (linep[pos.col] == NUL || (lisp && comment_col != MAXCOL && pos.col == (colnr_T)comment_col))",
			"if (linep[pos.col] == NUL)", 1, "% ending a line at a lisp comment")
		e.Literal("if (pos.lnum == curbuf->b_ml.ml_line_count || lispcomm)",
			"if (pos.lnum == curbuf->b_ml.ml_line_count)", 1, "% stopping at a lisp comment forwards")
		e.Literal("if (lisp || skip_comments)", "if (skip_comments)", 1, "% scanning the next line for lisp")
		e.DropIf(`(?m)^[ \t]*if \(curbuf->b_p_lisp && vim_strchr\(\(char_u \*\)"\{\}\(\)\[\]", c\) != NULL`, 1,
			`% skipping #\( character literals`)
	})
	return e.Done()
}

func init() { edit.Register("whim57", Edit) }
