package cut

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

var cindentLeft = regexp.MustCompile(`\b(?:get_c_indent|in_cinkeys)\b`)

// NoCindent removes 'cindent': the editor cannot read C any more, and says so
// by indenting the way 'autoindent' does.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's folds and line cuts
// as DropIf, FoldNever, DropOperand and Cut by form, each counted and scoped
// where the text anchored on its function; the literals that rewrote a
// return as Rewrite, and cindent_on's body by Body (history keeps the text
// version).
func NoCindent(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nocindent", e, w)
	q := graph.NewVerbs("nocindent", e, io.Discard)
	say := func(what string) bool {
		if q.Failed() {
			return false
		}
		v.Say(what)
		return true
	}
	q.DropIf("(&& (call cindent_on) (call ctrl_x_mode_none))", 1, "insert mode's re-indent")
	q.DropIf("(&& can_cindent (call cindent_on) (call ctrl_x_mode_normal))", 1, "its second test")
	if !say("insert mode stops re-indenting, and the goto between its two tests goes with them") {
		return q.Done()
	}

	q.InFunction("open_line", func(q *graph.Verbs) {
		q.Cut("(= do_cindent (&& (! p_paste) (paren (-> curbuf b_p_cin)) (call in_cinkeys _ _ _) "+
			"(! (& flags OPENLINE_FORCE_INDENT))))", 1, "open_line's do_cindent")
		if c := q.One("(&& (== lead_len 0) (-> curbuf b_p_cin) do_cindent (== dir FORWARD) _)", "open_line's comment test"); c != nil {
			q.In(c, func(q *graph.Verbs) {
				q.DropOperand("(-> curbuf b_p_cin)", 1, "open_line's comment test")
				q.DropOperand("do_cindent", 1, "open_line's comment test")
			})
		}
		q.DropIf("(|| do_cindent (paren (&& (-> curbuf b_p_ai) (call use_indentexpr_for_lisp))))", 1,
			"open_line's C indent")
	})
	if !say("open_line stops asking whether to indent as C") {
		return q.Done()
	}

	q.FoldNever("(if (call cindent_on) (block (= indent (call get_c_indent))) _)", 1, "the `=` operator's C arm")
	if !say("`=` indents by the line above, which is what 'autoindent' does") {
		return q.Done()
	}

	q.InFunction("preprocs_left", func(q *graph.Verbs) {
		q.Rewrite("(return (|| (paren (&& ?si (! (-> curbuf b_p_cin)))) "+
			"(paren (&& (-> curbuf b_p_cin) (call in_cinkeys '#' ' ' TRUE) (== (-> curbuf b_ind_hash_comment) 0)))))",
			"(return ?si)", 1, "preprocs_left")
	})
	q.InFunction("fix_indent", func(q *graph.Verbs) {
		q.FoldNever("(if (call use_indentexpr_for_lisp) (block (call do_c_expr_indent)) _)", 1, "fix_indent's lisp arm")
		q.DropIf("(if (call cindent_on) (block (call do_c_expr_indent)))", 1, "fix_indent's C arm")
	})
	// want_cindent is (get_can_cindent() && cindent_on()), so it is FALSE.  Its
	// declaration, like do_cindent's, is the collection's; so are parse_cino and
	// do_c_expr_indent once their calls are gone.
	q.Cut("(= want_cindent (paren (&& (call get_can_cindent) (call cindent_on))))", 1, "ins_compl_stop's want_cindent")
	q.Cut("(if want_cindent (block (call do_c_expr_indent) (= want_cindent FALSE)))", 1, "its first use")
	q.Cut("(if (&& want_cindent (call in_cinkeys KEY_COMPLETE ' ' (call inindent 0))) (block (call do_c_expr_indent)))", 1,
		"its second use")
	// op_reindent() takes the indenter as a FUNCTION POINTER, which is why a
	// grep for `get_c_indent(` does not find this one.  Without a C indenter,
	// `=` sets each line's indent to the indent it already has: a no-op, which
	// is the honest answer for a buffer whose language the editor cannot read.
	q.Rewrite("(call op_reindent oap get_c_indent)", "(call op_reindent oap get_indent)", 1, "`=`'s indenter")
	q.DropIf("(&& (== leader_len 0) (-> curbuf b_p_cin))", 1, "internal_format's comment hunt")
	if !say("preprocs_left, fix_indent, completion, `=` and the comment hunt in internal_format") {
		return q.Done()
	}

	q.InFunction("may_do_si", func(q *graph.Verbs) {
		q.DropOperand("(! (-> curbuf b_p_cin))", 1, "may_do_si")
	})
	if !say("'smartindent' stops deferring to an option that is gone") {
		return q.Done()
	}

	q.DropIf("(&& (!= last_char ';') (!= last_char '}') (call cin_is_cinword ptr))", 1,
		"'smartindent' consulting cin_is_cinword")
	// FOUR callers, not two: 'shiftwidth' re-parses 'cinoptions' because some
	// of them are expressed in shiftwidths, and check_buf_options() re-parses
	// on every option check.  Neither is about indenting; both just keep the
	// b_ind_* fields in step with a string that no longer exists.
	q.Cut("(call parse_cino curbuf)", 3, "a parse_cino call")
	q.Cut("(call parse_cino buf)", 1, "check_buf_options' parse_cino")
	if !say("'cinwords' for 'smartindent', and 'cinoptions' parsing") {
		return q.Done()
	}

	// cindent_on() stays and answers no: five of its seven callers only ask in
	// order to do something else instead.
	q.Body("cindent_on", "(return FALSE)", "cindent_on")
	if !say("cindent_on() answers no, which is now true") {
		return q.Done()
	}
	v.Sayf("%d get_c_indent/in_cinkeys mentions left for the sweep", len(cindentLeft.FindAll(v.Text(), -1)))
	return v.Done()
}
