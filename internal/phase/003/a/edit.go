package p003a

// Whim phase 3a (formerly 57) -- no lisp.  See GOAL.md.
//
// 'lisp' and 'lispwords' go, and with them everything they switched on:
// get_lisp_indent() for autoindent, =, gq and new lines; lisp_match() over
// 'lispwords'; '-' as a keyword character; ';' line comments in check_linecomment();
// and findmatchlimit()'s lisp mode, which stopped % at a ';' comment and skipped
// #\( and #\[ character literals.  'lispoptions' went in Phase 17.
//
// b_p_lisp is folded as FALSE at every reader rather than stubbed, so each branch
// it guarded is either gone or taken unconditionally.
//
// THE DELTA: none the harnesses record -- no case sets 'lisp'.  The probes check
// the two options are unknown and that % still matches across a ';'.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the folds by form, each scoped
// to its function as the text scoped it; the literals that took one operand
// out of a condition are DropOperand inside that condition (history keeps
// the text version).

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// dropIn drops the operand op from the one condition cond matches.
func dropIn(v *graph.Verbs, cond, op, what string) {
	if c := v.One(cond, what); c != nil {
		v.In(c, func(v *graph.Verbs) { v.DropOperand(op, 1, what) })
	}
}

// Edit takes lisp mode: the indenting, the ';' comment leader, and the ten
// places `%` knew about a lisp comment.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nolisp", e, w)

	v.InFunction("open_line", func(v *graph.Verbs) {
		v.DropIf("(&& (== leader nullptr) (! (call use_indentexpr_for_lisp)) (-> curbuf b_p_lisp) (-> curbuf b_p_ai))", 1,
			"a new line taking its indent from get_lisp_indent()")
		v.Cut("(if (! p_paste) (block))", 1, "open_line's now-empty 'paste' test")
	})
	v.InFunction("buf_init_chartab", func(v *graph.Verbs) {
		v.DropIf("(-> buf b_p_lisp)", 1, "'-' as a keyword character")
	})
	v.InFunction("check_linecomment", func(v *graph.Verbs) {
		v.FoldNever("(-> curbuf b_p_lisp)", 1, "a ';' starting a line comment")
	})
	v.InFunction("op_reindent", func(v *graph.Verbs) {
		v.FoldAlways("(|| (!= i (- (-> oap line_count) 1)) (== (-> oap line_count) 1) (!= how get_lisp_indent))", 1,
			"= skipping the last line only for lisp")
	})
	v.InFunction("fix_indent", func(v *graph.Verbs) {
		v.DropIf("(&& (-> curbuf b_p_lisp) (-> curbuf b_p_ai))", 1, "fix_indent re-indenting lisp")
	})
	v.InFunction("do_pending_operator", func(v *graph.Verbs) {
		v.DropIf("(-> curbuf b_p_lisp)", 1, "= indenting lisp")
	})
	v.InFunction("format_lines", func(v *graph.Verbs) {
		v.FoldNever("(-> curbuf b_p_lisp)", 1, "gq indenting lisp")
	})

	// findmatchlimit carried a lisp comment state through the whole scan, so
	// this is eight acts rather than one: six conditions that named its two
	// locals and two that named b_p_lisp directly.  The locals themselves,
	// named by nothing after that, go to the collection.
	v.InFunction("findmatchlimit", func(v *graph.Verbs) {
		dropIn(v, "(|| (paren (&& backwards comment_dir)) lisp skip_comments)", "lisp",
			"% looking for a comment only for a comment direction or FM_SKIPCOMM")
		v.DropIf("(&& lisp (!= comment_col MAXCOL) (> (. pos col) (cast colnr_T comment_col)))", 1,
			"% starting inside a lisp comment")
		v.DropIf("(&& lispcomm (< (. pos col) (cast colnr_T comment_col)))", 1,
			"% stopping at a lisp comment backwards")
		dropIn(v, "(|| comment_dir lisp skip_comments)", "lisp", "% rescanning a line for lisp")
		v.FoldNever("(&& lisp (!= comment_col MAXCOL))", 1, "% jumping to a lisp comment backwards")
		dropIn(v, "(|| (== (index linep (. pos col)) NUL) (paren (&& lisp _ _)))", "(paren (&& lisp _ _))",
			"% ending a line at a lisp comment")
		dropIn(v, "(|| (== (. pos lnum) (. (-> curbuf b_ml) ml_line_count)) lispcomm)", "lispcomm",
			"% stopping at a lisp comment forwards")
		dropIn(v, "(|| lisp skip_comments)", "lisp", "% scanning the next line for lisp")
		v.DropIf("(&& (-> curbuf b_p_lisp) (!= (call vim_strchr (cast (ptr char_u) \"{}()[]\") c) nullptr) _ _ _)", 1,
			`% skipping #\( character literals`)
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim3a", Edit) }
