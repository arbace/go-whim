package p005a

// Whim phase 5a (formerly 64) -- no formatting, comment or nroff-macro options.  See GOAL.md.
//
// Five options, and the machinery that only they gave a meaning to:
//
// 'comments'       no comment leader is recognised any more.  get_leader_len()
// and get_last_leader_offset() would return 0 and -1, so everything built
// on a leader goes: open_line() copying, replacing and aligning one,
// insertchar() completing a comment's end, J removing leaders, the leader
// a formatted or wrapped line keeps, same_leader(), skip_comment(), the
// declaration search skipping comment lines, and % skipping a // comment
// in a buffer whose 'comments' looked like C.
// 'formatoptions'  fixed at its default, "tcq".  With no leader, 'c' and 'q'
// have nothing to act on, so what is left is 't': text still wraps at
// 'textwidth' while typing, and gq still formats.  Every other flag was
// off, so its code goes: 'a' (auto_format(), check_auto_format() and the
// 18 calls), 'w', 'n', '2', 'b', 'l', 'v', 'm', 'M', 'B', '1', 'p', ']',
// 'j', 'r', 'o' and '/'.  'paste' still stops the wrapping, as it did.
// 'formatlistpat'  only 'n' read it, through get_number_indent().
// 'paragraphs', 'sections'  no nroff macro starts a paragraph or section: {, },
// [[, ]], ( and ) and the ip/ap text objects stop at blank lines, form
// feeds and braces, and inmacro() goes.
//
// And the two mechanisms that were left reading what those options described:
//
// THE FORMAT OPERATOR  gq and gw, their doubled gqq/gqgq/gww/gwgw, op_format(),
// format_lines() and fmt_check_par().  A paragraph is only a paragraph to
// decide where a format stops, and nothing formats now.  What stays is the
// wrap while typing: 'textwidth' and 'wrapmargin' still break a line
// through insertchar() and internal_format(), and 'paste' still stops it.
// With no gq, INSCHAR_FORMAT is never set and comp_textwidth() loses the
// flag that chose the screen width for it.
// GO TO LOCAL DECLARATION  gd and gD, nv_gd() and find_decl(), which searched
// from the start of the block the cursor was in.  gd was the only caller.
// THE = OPERATOR  ==, =G and the rest.  op_reindent() re-applied get_indent(),
// which is the indent the line already has: 'equalprg' went in phase 19 and
// C-indenting is off, so = could not compute an indent to apply.
// THE ! OPERATOR  !{motion}, which was ALREADY dead -- its nv_cmds row has been
// nv_error for phases, and get_op_type() is reached only from nv_operator()
// -- so OP_FILTER could no longer be set at all.  What goes is the dispatch
// nothing reached: the OP_FILTER case, the `!` op_colon() typed after a
// range, and do_bang()'s bangredo block, which only that case set.
// :w !cmd and :r !cmd still reach do_bang(), and do_filter() still says the
// command is not available in this version.
// WHAT C-INDENTING LEFT BEHIND  the engine went phases ago -- no get_c_indent(),
// no cin_* anything, no 'cindent', 'cinoptions', 'cinkeys', 'cinwords',
// 'indentexpr' or 'indentkeys'.  What stayed was a switch wired to FALSE and
// its plumbing: cindent_on(), which is `return FALSE`, and can_cindent,
// WRITTEN IN TEN PLACES AND READ IN NONE -- gcc does not warn, because a
// static that is assigned counts as used.  set_can_cindent() goes with it.
// 'smartindent' STAYS: may_do_si(), did_si/can_si/can_si_back/no_si and
// open_line()'s {, }, # and ) rules are a different mechanism, and this
// build switches it on by default.
//
// THE DELTA: three behaviour cases -- format_gq (gqq now beeps and changes
// nothing) and the two that set the options, format_comment and open_comment.
// The probes check the five options are unknown, that typing still wraps at
// 'textwidth', that gqq and gd do nothing, and that } no longer stops at .PP.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's lines,
// heads and literals are acts on the nodes -- ifs dropped and folded,
// statements and runs of items cut, operands dropped, conditions rewritten
// from their own operands, comp_textwidth's flag dropped with the argument
// at its calls (PARAM), the `=` row pointed at nv_error -- each counted, its
// report the text's in its order (history keeps the text version).

import (
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// cutEach cuts the n items pat matches whose atoms bound to binds are each
// one of vals -- `x = TRUE;` and `x = FALSE;`, the text's `x = (?:TRUE|FALSE);`
// -- as one act.
func cutEach(v *graph.Verbs, pat string, binds []string, vals map[string]bool, n int, what string) {
	if v.Failed() {
		return
	}
	var got []*graph.Node
	p := clisp.MustPattern(pat)
	for _, m := range v.Find(pat) {
		b, _ := graph.Match(p, m)
		ok := v.Editor().Item(m) == m
		for _, k := range binds {
			if x := b[k]; x == nil || x.IsList() || !vals[x.Atom] {
				ok = false
			}
		}
		if ok {
			got = append(got, m)
		}
	}
	if len(got) != n {
		v.Die("%s -- %d matches, expected %d", what, len(got), n)
		return
	}
	for _, m := range got {
		if err := v.Editor().Delete(m); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
	}
	v.Say(what)
}

var answers = map[string]bool{"TRUE": true, "FALSE": true}

// Whim5a takes 'formatoptions' and everything only it reached: the comment
// leader in open_line, auto-formatting, the gq operator, and the C-indenting
// flag that ten writers set and nothing read.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noformatopts", e, w)

	// open_line: no leader to find, copy or align
	v.InFunction("open_line", func(v *graph.Verbs) {
		v.Cut("(if (& flags OPENLINE_DO_COM) (block (= lead_len (call get_leader_len ptr nullptr FALSE TRUE))) (block (= lead_len 0)))",
			2, "smartindent looking for a comment leader")
		v.Rewrite("(&& (== lead_len 0) (== (index ptr 0) '#'))", "(== (index ptr 0) '#')", 2,
			"smartindent after a # line not asking about a leader")
		// The leader block first: it holds lead_len = 0 statements and an
		// if (lead_len > 0) of its own, which would throw every count after it.
		v.DropIf("(if (> lead_len 0) (block (def lead_repl (ptr char_u) nullptr) _*))", 1,
			"copying, replacing and aligning a comment leader")
		v.FoldNever("(& flags OPENLINE_DO_COM)", 1, "a new line finding the leader to repeat")
		v.Cut("(= lead_len 0)", 1, "a new line with no leader")
		v.FoldNever("(> lead_len 0)", 1, "smartindent treating a comment line specially")
		v.FoldNever("lead_len", 1, "the new line starting with its leader")
		v.Cut("(= end_comment_pending NUL)", 2, "a new line clearing the pending comment end")
		v.DropOperand("(! (& flags OPENLINE_KEEPTRAIL))", 1, "a broken line always losing its trailing blanks ('w')")
		v.DropIf("(if _ (block (while (> (post-- lead_len) 0) (block (call replace_push NUL)))))", 1,
			"Replace mode pushing a NUL per leader byte")
		v.DropOperand("(! (& flags OPENLINE_COM_LIST))", 1, "the second-line indent no longer for a comment list")
		v.Cut("(call vim_free allocated)", 1, "freeing the leader")
		// extra_len sized the leader's allocation and nothing else
		v.Cut("(= extra_len (cast int (call strlen (cast (ptr char) (paren p_extra)))))", 1,
			"measuring the text after the cursor for the leader")
	})
	v.Rewrite("(? (call has_format_option FO_RET_COMS) OPENLINE_DO_COM 0)", "0", 1, "Enter in Insert mode repeating a leader ('r')")
	v.Rewrite("(? (call has_format_option FO_OPEN_COMS) OPENLINE_DO_COM 0)", "0", 1, "o and O repeating a leader ('o')")

	// insertchar: 'b' and 'l' off, no comment end to complete
	v.InFunction("insertchar", func(v *graph.Verbs) {
		v.Cut("(= fo_ins_blank (call has_format_option FO_INS_BLANK))", 1, "insertchar asking for 'b'")
		v.DropOperand("(|| (!= (. (-> curwin w_cursor) lnum) (. Insstart lnum)) _)", 1,
			"wrapping a line that was already long when Insert began ('l', 'b')")
		v.DropIf("(&& did_ai (== c end_comment_pending))", 1, "typing the last character of a comment end")
		v.Cut("(= end_comment_pending NUL)", 1, "insertchar clearing the pending comment end")
	})
	v.Cut("(= Insstart_textlen (cast colnr_T (call linetabsize_str (call ml_get_curline))))", 3, "measuring the line Insert began on")
	v.Cut("(= Insstart_blank_vcol MAXCOL)", 1, "resetting the first blank typed")
	v.DropIf("(&& (== Insstart_blank_vcol MAXCOL) (== (. (-> curwin w_cursor) lnum) (. Insstart lnum)))", 2,
		"remembering the first blank typed")
	v.Cut("(= end_comment_pending NUL)", 1, "ins_bs clearing the pending comment end")

	// 'a' and 'w': no auto-formatting
	v.InFunction("stop_insert", func(v *graph.Verbs) {
		v.DropIf("(&& (! ins_need_undo) (call has_format_option FO_AUTO))", 1, "leaving Insert mode auto-formatting")
		v.Cut("(call check_auto_format TRUE)", 1, "leaving Insert mode removing an auto-format space")
	})
	v.InFunction("ins_bs", func(v *graph.Verbs) {
		v.DropIf("(&& (call has_format_option FO_AUTO) (call has_format_option FO_WHITE_PAR))", 1,
			"backspacing over a line break dropping a trailing space")
	})
	v.InFunction("do_pending_operator", func(v *graph.Verbs) {
		v.DropIf("(&& (== (-> oap motion_type) MLINE) (call has_format_option FO_AUTO) (== (call u_save_cursor) OK))", 1,
			"a linewise delete auto-formatting")
	})
	v.InFunction("op_delete", func(v *graph.Verbs) {
		v.Cut("(if (== (-> oap op_type) OP_DELETE) (block (call auto_format FALSE TRUE)))", 1,
			"a characterwise delete auto-formatting")
	})
	cutEach(v, "(call auto_format ?a ?b)", []string{"a", "b"}, answers, 15, "the other calls to auto_format")

	// J: 'j', 'M' and 'B' off
	v.InFunction("do_join", func(v *graph.Verbs) {
		v.DropIf("remove_comments", 4, "J removing comment leaders")
		q := graph.NewVerbs(v.Tag, e, io.Discard)
		q.In(v.Scope(), func(q *graph.Verbs) {
			q.DropOperand("(|| (! (call has_format_option FO_MBYTE_JOIN)) _)", 1, "'M'")
			q.DropOperand("(|| (! (call has_format_option FO_MBYTE_JOIN2)) _*)", 1, "'B'")
		})
		if q.Err != nil {
			v.Err = q.Err
			return
		}
		v.Say("J inserting no space between multibyte characters ('M', 'B')")
	})

	// internal_format: 't' alone, no leader
	v.InFunction("internal_format", func(v *graph.Verbs) {
		v.Cut("(= skip_pos 0)", 1, "internal_format: skip_pos = 0;")
		v.Cut("(= wcc 0)", 1, "internal_format: wcc = 0;")
		v.Splice("(if no_leader _*)", "(if (== leader_len 0) (block (= no_leader TRUE)))", "",
			"wrapping a line looking up its leader ('c')")
		v.Rewrite("(&& ?f (== leader_len 0) (! (call has_format_option FO_WRAP)))", "(&& ?f p_paste)", 1,
			"wrapping only with 't', which 'paste' turns off")
		v.Rewrite("(while (|| (paren (&& (! fo_ins_blank) (! (call has_format_option FO_INS_VI)))) _*) ?b)", "(for () () () ?b)", 1,
			"breaking only at blanks typed in this Insert ('v', 'b')")
		v.DropIf("(< wcc 2)", 1, "counting the blanks before a break")
		v.DropIf("(&& (call has_format_option FO_PERIOD_ABBR) (== cc '.') (< wcc 2))", 1, "not breaking after a period ('p')")
		v.FoldNever("(&& (|| (>= cc 0x100) (! (call utf_allow_break_before cc))) fo_multibyte)", 1,
			"breaking between multibyte characters ('m', ']')")
		v.DropIf("(call has_format_option FO_ONE_LETTER)", 1, "not breaking after a one-letter word ('1')")
		v.Cut("(if (< (. (-> curwin w_cursor) col) leader_len) (block (break)))", 1, "not breaking inside the leader")
		v.DropOperandAsText("(|| (! fo_white_par) (< (. (-> curwin w_cursor) col) startcol))", 1, "keeping a trailing blank ('w')")
		v.FoldAlways("(! fo_white_par)", 2, "removing the blanks at the break ('w')")
		v.Rewrite("(call open_line FORWARD (+ OPENLINE_DELSPACES OPENLINE_MARKFIX _*) _ (addr did_do_comment))",
			"(call open_line FORWARD (+ OPENLINE_DELSPACES OPENLINE_MARKFIX) old_indent nullptr)", 1,
			"the break opening a line with no leader")
		v.DropIf("did_do_comment", 1, "a leader found by the new line")
		// second_indent is -1: ins_char() is the only caller left once the operator goes
		v.DropIf("first_line", 1, "the first broken line's second-line indent ('2', 'n')")
		v.FoldAlways("(! (& flags INSCHAR_COM_LIST))", 1, "a comment list keeping its indent")
	})

	// format_lines() and fmt_check_par() are NOT folded for the options: once
	// the operator section below takes gq, nothing reaches them and the
	// collection takes both, so folding them is work this phase would throw away.

	// the rest of 'comments'
	v.InFunction("find_decl", func(v *graph.Verbs) {
		v.DropIf("(> (call get_leader_len (call ml_get_curline) nullptr FALSE TRUE) 0)", 1, "gd skipping comment lines")
	})
	v.InFunction("nv_percent", func(v *graph.Verbs) {
		v.FoldNever("(&& (== (call vim_strchr p_cpo CPO_MATCH) nullptr) (call buf_has_cstyle_comments))", 1, "% skipping a // comment")
	})

	// 'paragraphs' and 'sections'
	v.InFunction("startPS", func(v *graph.Verbs) {
		v.DropIf("(&& (== (deref s) '.') (|| (call inmacro p_sections (+ s 1)) _))", 1,
			"an nroff macro starting a paragraph or section")
	})

	// the format operator: gq, gw, and gqq/gwgw
	v.InFunction("nv_g_cmd", func(v *graph.Verbs) {
		v.CutRun("gq and gw as operators", "(case 'q')", "(case 'w')",
			"(= (-> oap cursor_start) (-> curwin w_cursor))", "(attributed (std-attr fallthrough))")
	})
	v.InFunction("nv_record", func(v *graph.Verbs) {
		v.DropIf("(== (-> cap oap op_type) OP_FORMAT)", 1, "gqq and gqgq doubling the operator")
	})
	v.InFunction("do_pending_operator", func(v *graph.Verbs) {
		// with 'formatprg''s test still in it: phase 19, which folded it, runs
		// after this program now (whim5a runs at phase 5)
		v.CutRun("the operator reaching the formatter", "(case OP_FORMAT)",
			"(block (if (|| (!= (deref p_fp) NUL) (!= (deref (-> curbuf b_p_fp)) NUL)) (block (call op_colon oap)) (block (call op_format oap FALSE))))",
			"(break)", "(case OP_FORMAT2)", "(call op_format oap TRUE)", "(break)")
	})

	// gd and gD
	v.InFunction("nv_g_cmd", func(v *graph.Verbs) {
		v.CutRun("gd and gD", "(case 'd')", "(case 'D')", "(call nv_gd oap (-> cap nchar) (cast int (-> cap count0)))", "(break)")
	})

	// what only the formatter set: INSCHAR_FORMAT
	v.InFunction("insertchar", func(v *graph.Verbs) {
		// the width to wrap at: comp_textwidth's flag goes with its
		// parameter, below, from every call at once
		v.One("(= textwidth (call comp_textwidth force_format))", "the width to wrap at")
		if !v.Failed() {
			v.Say("the width to wrap at")
		}
		v.Rewrite("(&& (> textwidth 0) (|| force_format (paren (&& ?a ?b))))", "(&& (> textwidth 0) ?a ?b)", 1,
			"wrapping only a character that was typed")
		v.RewriteAt("(call internal_format textwidth second_indent flags ?x c)", "x", "FALSE", 1,
			"the wrap never being a whole-line format")
		v.DropIf("(== c NUL)", 1, "insertchar called with no character to insert")
	})
	v.InFunction("comp_textwidth", func(v *graph.Verbs) {
		v.DropIf("(&& ff (== textwidth 0))", 1, "the width gq used when 'textwidth' is 0")
	})
	if !v.Failed() {
		protos := 0
		for _, d := range e.FileDecls("comp_textwidth") {
			if d.Is("def") {
				protos++
			}
		}
		v.Expect(protos == 1, "comp_textwidth's prototype -- %d, expected 1", protos)
		v.One("(= cols (call comp_textwidth FALSE))", "the change list asking for the width")
	}
	v.DropParam("comp_textwidth", "ff", "comp_textwidth without its gq flag")
	if !v.Failed() {
		v.Say("comp_textwidth's prototype")
		v.Say("the change list asking for the width")
	}
	v.InFunction("internal_format", func(v *graph.Verbs) {
		v.Rewrite("(&& (! (& flags INSCHAR_FORMAT)) p_paste)", "p_paste", 1, "wrapping stopped only by 'paste'")
		v.DropOperand("(! format_only)", 1, "the wrap always redrawing")
	})

	// oparg_T's cursor_start was gq's alone, and goes by hand.  deadfields.py
	// keeps every field of a type that is ever initialised WITHOUT designators,
	// because a positional initialiser names no field and removing one silently
	// shifts what the rest fill -- and `oparg_T oa = { 0 };` in pagescroll() is
	// exactly that.  `{ 0 }` fills only the first field, so removing a later one
	// is safe here.
	v.Cut("(cursor_start pos_T)", 1, "the cursor gw returned to")

	// the = operator, and what only the unreachable ! operator left behind
	v.InTable("nv_cmds", func(v *graph.Verbs) {
		v.RewriteAt("(init '=' ?h 0 0)", "h", "nv_error", 1, "= in Normal and Visual mode points at nv_error")
	})
	v.InFunction("do_pending_operator", func(v *graph.Verbs) {
		// case OP_FILTER and case OP_INDENT go with what they ran, and
		// OP_COLON's run is op_colon(oap) alone (the text's literal: the old
		// dispatch, oldDispatch, made newDispatch)
		r := v.Run("the filter and indent operators being dispatched",
			"(case OP_FILTER)",
			`(if (!= (call vim_strchr p_cpo CPO_FILTER) nullptr) (block (call AppendToRedobuff (cast (ptr char_u) "!\r"))) (block (= bangredo TRUE)))`,
			"(attributed (std-attr fallthrough))", "(case OP_INDENT)", "(case OP_COLON)",
			"(if (&& (== (-> oap op_type) OP_INDENT) (== (deref (call get_equalprg)) NUL)) (block (call op_reindent oap get_indent) (break)))",
			"(call op_colon oap)", "(break)")
		if r == nil {
			return
		}
		err := e.ReplaceRun(r[0], r[3])
		if err == nil {
			err = e.Delete(r[5])
		}
		if err != nil {
			v.Die("the filter and indent operators being dispatched -- %v", err)
			return
		}
		v.Say("the filter and indent operators being dispatched")
		v.DropOperand("(== (-> oap op_type) OP_FILTER)", 1, "a filter deciding whether the motion is inclusive")
	})
	v.InFunction("op_colon", func(v *graph.Verbs) {
		v.DropIf("(!= (-> oap op_type) OP_COLON)", 1, "the ! typed after an operator range")
	})
	// do_bang, its bangredo block and its label went with :! and the filters
	// at phase 1 (D2, D4)

	// what C-indenting left behind.  Three of the ten writes are the whole Body
	// of an `if`, so the test goes with them rather than leaving an empty block.
	// Each is scoped to its function: `if (inindent(0))` also guards
	// do_pending_operator's `oap->motion_type = MLINE`, which stays, and an
	// unscoped drop would have had two matches to choose between.
	v.InFunction("edit", func(v *graph.Verbs) {
		v.DropIf("(call inindent 0)", 1, "a space typed in the indent forbidding a reindent")
	})
	v.InFunction("ins_bs", func(v *graph.Verbs) {
		v.DropIf("in_indent", 1, "a backspace in the indent forbidding a reindent")
	})
	v.InFunction("ins_tab", func(v *graph.Verbs) {
		v.DropIf("ind", 1, "a Tab in the indent forbidding a reindent")
	})
	cutEach(v, "(= can_cindent ?a)", []string{"a"}, answers, 7, "the other places that armed or disarmed a reindent")
	v.InFunction("internal_format", func(v *graph.Verbs) {
		v.Cut("(call set_can_cindent TRUE)", 1, "a wrapped line arming a reindent")
	})

	// cindent_on() is `return FALSE`; its two callers fold and the
	// collection takes it.
	v.InFunction("ins_bs", func(v *graph.Verbs) {
		v.Rewrite("(|| ?a (call cindent_on))", "?a", 1, "CTRL-U keeping the indent for 'autoindent' alone")
	})
	v.InFunction("insertchar", func(v *graph.Verbs) {
		v.DropOperand("(! (call cindent_on))", 1, "the multi-character insert asking whether C-indenting is on")
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim5a", Edit) }
