package p064

// Whim phase 64 -- no formatting, comment or nroff-macro options.  See GOAL.md.
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
// which is the indent the line already has: 'equalprg' went in phase 60 and
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

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// internalFormatDecls are the writes internal_format loses, paired with the
// report line the heredoc printed for each.  The declarations they wrote,
// and open_line's, are named by nothing after this phase and go to its sweep.
var internalFormatDecls = []struct{ pattern, What string }{
	{`skip_pos = 0;`, "internal_format: skip_pos = 0;"},
	{`wcc = 0;`, "internal_format: wcc = 0;"},
}

const oldDispatch = `        case OP_FILTER:
            if (vim_strchr(p_cpo, CPO_FILTER) != NULL)
            {
                AppendToRedobuff((char_u *)"!\r");
            }
            else
            {
                bangredo = TRUE;
            }
            __attribute__((fallthrough));
        case OP_INDENT:
        case OP_COLON:
            if (oap->op_type == OP_INDENT)
            {
                op_reindent(oap, get_indent);
                break;
            }
            op_colon(oap);
            break;
`

const newDispatch = `        case OP_COLON:
            op_colon(oap);
            break;
`

// Whim64 takes 'formatoptions' and everything only it reached: the comment
// leader in open_line, auto-formatting, the gq operator, and the C-indenting
// flag that ten writers set and nothing read.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("noformatopts", text, w)

	// open_line: no leader to find, copy or align
	e.InFunction("open_line", func(e *edit.E) {
		e.Cut(edit.Line("if (flags & OPENLINE_DO_COM)", "{", "lead_len = get_leader_len(ptr, NULL, FALSE, TRUE);", "}", "else", "{", "lead_len = 0;", "}"),
			2, "smartindent looking for a comment leader")
		e.Sub(`\( ?lead_len == 0 && ptr\[0\] == '#'\)`, "(ptr[0] == '#')", 2,
			"smartindent after a # line not asking about a leader")
		// The leader block first: it holds lead_len = 0 statements and an
		// if (lead_len > 0) of its own, which would throw every count after it.
		e.DropIf(`(?m)^[ \t]*if \(lead_len > 0\)\n[ \t]*\{\n[ \t]*char_u[ \t]+\*lead_repl = NULL;$`, 1,
			"copying, replacing and aligning a comment leader")
		e.FoldNever(`(?m)^[ \t]*if \(flags & OPENLINE_DO_COM\)$`, 1, "a new line finding the leader to repeat")
		e.Lines(`lead_len = 0;`, 1, "a new line with no leader")
		e.FoldNever(`(?m)^[ \t]*if \(lead_len > 0\)$`, 1, "smartindent treating a comment line specially")
		e.FoldNever(`(?m)^[ \t]*if \(lead_len\)$`, 1, "the new line starting with its leader")
		e.Lines(`end_comment_pending = NUL;`, 2, "a new line clearing the pending comment end")
		e.Literal("if (trunc_line && !(flags & OPENLINE_KEEPTRAIL))", "if (trunc_line)", 1,
			"a broken line always losing its trailing blanks ('w')")
		e.DropIf(`(?m)^[ \t]*if \(\(\(\(State\) & REPLACE_FLAG\) && !\(\(State\) & VREPLACE_FLAG\)\)\)$\n[ \t]*\{\n[ \t]*while \(lead_len-- > 0\)`, 1,
			"Replace mode pushing a NUL per leader byte")
		e.Literal("if (newindent == 0 && !(flags & OPENLINE_COM_LIST))", "if (newindent == 0)", 1,
			"the second-line indent no longer for a comment list")
		e.Lines(`vim_free\(allocated\);`, 1, "freeing the leader")
		// extra_len sized the leader's allocation and nothing else
		e.Lines(`extra_len = \(int\)strlen\(\(char \*\)\(p_extra\)\);`, 1,
			"measuring the text after the cursor for the leader")
	})
	e.Literal("has_format_option(FO_RET_COMS) ? OPENLINE_DO_COM : 0", "0", 1, "Enter in Insert mode repeating a leader ('r')")
	e.Literal("has_format_option(FO_OPEN_COMS) ? OPENLINE_DO_COM : 0", "0", 1, "o and O repeating a leader ('o')")

	// insertchar: 'b' and 'l' off, no comment end to complete
	e.InFunction("insertchar", func(e *edit.E) {
		e.Lines(`fo_ins_blank = has_format_option\(FO_INS_BLANK\);`, 1, "insertchar asking for 'b'")
		e.Literal(" && (curwin->w_cursor.lnum != Insstart.lnum || ((!has_format_option(FO_INS_LONG) || Insstart_textlen <= (colnr_T)textwidth) && (!fo_ins_blank || Insstart_blank_vcol <= (colnr_T)textwidth)))",
			"", 1, "wrapping a line that was already long when Insert began ('l', 'b')")
		e.DropIf(`(?m)^[ \t]*if \(did_ai && c == end_comment_pending\)$`, 1, "typing the last character of a comment end")
		e.Lines(`end_comment_pending = NUL;`, 1, "insertchar clearing the pending comment end")
	})
	e.Lines(`Insstart_textlen = \(colnr_T\)linetabsize_str\(ml_get_curline\(\)\);`, 3, "measuring the line Insert began on")
	e.Lines(`Insstart_blank_vcol = MAXCOL;`, 1, "resetting the first blank typed")
	e.DropIf(`(?m)^[ \t]*if \(Insstart_blank_vcol == MAXCOL && curwin->w_cursor\.lnum == Insstart\.lnum\)$`, 2,
		"remembering the first blank typed")
	e.Lines(`end_comment_pending = NUL;`, 1, "ins_bs clearing the pending comment end")

	// 'a' and 'w': no auto-formatting
	e.InFunction("stop_insert", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(!ins_need_undo && has_format_option\(FO_AUTO\)\)$`, 1, "leaving Insert mode auto-formatting")
	})
	e.InFunction("stop_insert", func(e *edit.E) {
		e.Lines(`check_auto_format\(TRUE\);`, 1, "leaving Insert mode removing an auto-format space")
	})
	e.InFunction("ins_bs", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(has_format_option\(FO_AUTO\) && has_format_option\(FO_WHITE_PAR\)\)$`, 1,
			"backspacing over a line break dropping a trailing space")
	})
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(oap->motion_type == MLINE && has_format_option\(FO_AUTO\) && u_save_cursor\(\) == OK\)$`, 1,
			"a linewise delete auto-formatting")
	})
	e.InFunction("op_delete", func(e *edit.E) {
		e.Cut(edit.Line("if (oap->op_type == OP_DELETE)", "{", "auto_format(FALSE, TRUE);", "}"), 1,
			"a characterwise delete auto-formatting")
	})
	e.Lines(`auto_format\((?:FALSE|TRUE), (?:FALSE|TRUE)\);`, 15, "the other calls to auto_format")

	// J: 'j', 'M' and 'B' off
	e.InFunction("do_join", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(remove_comments\)$`, 4, "J removing comment leaders")
		e.Literal(" && (!has_format_option(FO_MBYTE_JOIN) || (utf_ptr2char(curr) < 0x100 && endcurr1 < 0x100)) && (!has_format_option(FO_MBYTE_JOIN2) || (utf_ptr2char(curr) < 0x100 && !(utf_eat_space(endcurr1))) || (endcurr1 < 0x100 && !(utf_eat_space(utf_ptr2char(curr)))))",
			"", 1, "J inserting no space between multibyte characters ('M', 'B')")
	})

	// internal_format: 't' alone, no leader
	e.InFunction("internal_format", func(e *edit.E) {
		for _, d := range internalFormatDecls {
			e.Lines(d.pattern, 1, d.What)
		}
		e.Splice("        if (no_leader)\n", "        if (leader_len == 0)\n        {\n            no_leader = TRUE;\n        }\n", "",
			"wrapping a line looking up its leader ('c')")
		e.Literal("if (!(flags & INSCHAR_FORMAT) && leader_len == 0 && !has_format_option(FO_WRAP))",
			"if (!(flags & INSCHAR_FORMAT) && p_paste)", 1, "wrapping only with 't', which 'paste' turns off")
		e.Literal("while ((!fo_ins_blank && !has_format_option(FO_INS_VI)) || (flags & INSCHAR_FORMAT) || curwin->w_cursor.lnum != Insstart.lnum || curwin->w_cursor.col >= Insstart.col)",
			"for (;;)", 1, "breaking only at blanks typed in this Insert ('v', 'b')")
		e.DropIf(`(?m)^[ \t]*if \(wcc < 2\)$`, 1, "counting the blanks before a break")
		e.DropIf(`(?m)^[ \t]*if \(has_format_option\(FO_PERIOD_ABBR\) && cc == '\.' && wcc < 2\)$`, 1, "not breaking after a period ('p')")
		e.FoldNever(`(?m)^[ \t]*else if \(\(cc >= 0x100 \|\| !utf_allow_break_before\(cc\)\) && fo_multibyte\)$`, 1,
			"breaking between multibyte characters ('m', ']')")
		e.DropIf(`(?m)^[ \t]*if \(has_format_option\(FO_ONE_LETTER\)\)$`, 1, "not breaking after a one-letter word ('1')")
		e.Cut(edit.Line("if (curwin->w_cursor.col < leader_len)", "{", "break;", "}"), 1, "not breaking inside the leader")
		e.Literal(" && (!fo_white_par || curwin->w_cursor.col < startcol)", "", 1, "keeping a trailing blank ('w')")
		e.FoldAlways(`(?m)^[ \t]*if \(!fo_white_par\)$`, 2, "removing the blanks at the break ('w')")
		e.Literal("open_line(FORWARD, OPENLINE_DELSPACES + OPENLINE_MARKFIX + (fo_white_par ? OPENLINE_KEEPTRAIL : 0) + (do_comments ? OPENLINE_DO_COM : 0) + OPENLINE_FORMAT + ((flags & INSCHAR_COM_LIST) ? OPENLINE_COM_LIST : 0), ((flags & INSCHAR_COM_LIST) ? second_indent : old_indent), &did_do_comment);",
			"open_line(FORWARD, OPENLINE_DELSPACES + OPENLINE_MARKFIX, old_indent, NULL);", 1, "the break opening a line with no leader")
		e.DropIf(`(?m)^[ \t]*if \(did_do_comment\)$`, 1, "a leader found by the new line")
		// second_indent is -1: ins_char() is the only caller left once the operator goes
		e.DropIf(`(?m)^[ \t]*if \(first_line\)$`, 1, "the first broken line's second-line indent ('2', 'n')")
		e.FoldAlways(`(?m)^[ \t]*if \(!\(flags & INSCHAR_COM_LIST\)\)$`, 1, "a comment list keeping its indent")
	})

	// format_lines() and fmt_check_par() are NOT folded for the options: once
	// the operator section below takes gq, nothing reaches them and the sweep
	// takes both, so folding them is work this phase would throw away.

	// the rest of 'comments'
	e.InFunction("find_decl", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(get_leader_len\(ml_get_curline\(\), NULL, FALSE, TRUE\) > 0\)$`, 1, "gd skipping comment lines")
	})
	e.InFunction("nv_percent", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(vim_strchr\(p_cpo, CPO_MATCH\) == NULL && buf_has_cstyle_comments\(\)\)$`, 1, "% skipping a // comment")
	})

	// 'paragraphs' and 'sections'
	e.InFunction("startPS", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(\*s == '\.' && \(inmacro\(p_sections, s \+ 1\) \|\| \(!para && inmacro\(p_para, s \+ 1\)\)\)\)$`, 1,
			"an nroff macro starting a paragraph or section")
	})

	// the format operator: gq, gw, and gqq/gwgw
	e.InFunction("nv_g_cmd", func(e *edit.E) {
		e.Cut(edit.Line("case 'q':", "case 'w':", "oap->cursor_start = curwin->w_cursor;", "__attribute__((fallthrough));"), 1,
			"gq and gw as operators")
	})
	e.InFunction("nv_record", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(cap->oap->op_type == OP_FORMAT\)$`, 1, "gqq and gqgq doubling the operator")
	})
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.Cut(edit.Line("case OP_FORMAT:", "{", "op_format(oap, FALSE);", "}", "break;", "case OP_FORMAT2:", "op_format(oap, TRUE);", "break;"), 1,
			"the operator reaching the formatter")
	})

	// gd and gD
	e.InFunction("nv_g_cmd", func(e *edit.E) {
		e.Cut(edit.Line("case 'd':", "case 'D':", "nv_gd(oap, cap->nchar, (int)cap->count0);", "break;"), 1, "gd and gD")
	})

	// what only the formatter set: INSCHAR_FORMAT
	e.InFunction("insertchar", func(e *edit.E) {
		e.Literal("textwidth = comp_textwidth(force_format);", "textwidth = comp_textwidth();", 1, "the width to wrap at")
		e.Literal("if (textwidth > 0 && (force_format || (!((c) == ' ' || (c) == '\\t') && !((State & REPLACE_FLAG) && !(State & VREPLACE_FLAG) && *ml_get_cursor() != NUL))))",
			"if (textwidth > 0 && !((c) == ' ' || (c) == '\\t') && !((State & REPLACE_FLAG) && !(State & VREPLACE_FLAG) && *ml_get_cursor() != NUL))", 1,
			"wrapping only a character that was typed")
		e.Literal("internal_format(textwidth, second_indent, flags, c == NUL, c);",
			"internal_format(textwidth, second_indent, flags, FALSE, c);", 1, "the wrap never being a whole-line format")
		e.DropIf(`(?m)^[ \t]*if \(c == NUL\)$`, 1, "insertchar called with no character to insert")
	})
	e.InFunction("comp_textwidth", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(ff && textwidth == 0\)$`, 1, "the width gq used when 'textwidth' is 0")
		e.Literal("comp_textwidth(int ff)", "comp_textwidth(void)", 1, "comp_textwidth without its gq flag")
	})
	e.Literal("static int comp_textwidth(int ff);", "static int comp_textwidth(void);", 1, "comp_textwidth's prototype")
	e.Literal("cols = comp_textwidth(FALSE);", "cols = comp_textwidth();", 1, "the change list asking for the width")
	e.InFunction("internal_format", func(e *edit.E) {
		e.Literal("if (!(flags & INSCHAR_FORMAT) && p_paste)", "if (p_paste)", 1, "wrapping stopped only by 'paste'")
	})
	e.InFunction("internal_format", func(e *edit.E) {
		e.Literal("if (!format_only && haveto_redraw)", "if (haveto_redraw)", 1, "the wrap always redrawing")
	})

	// oparg_T's cursor_start was gq's alone, and goes by hand.  deadfields.py
	// keeps every field of a type that is ever initialised WITHOUT designators,
	// because a positional initialiser names no field and removing one silently
	// shifts what the rest fill -- and `oparg_T oa = { 0 };` in pagescroll() is
	// exactly that.  `{ 0 }` fills only the first field, so removing a later one
	// is safe here.
	e.Lines(`pos_T[ \t]+cursor_start;`, 1, "the cursor gw returned to")

	// the = operator, and what only the unreachable ! operator left behind
	e.Sub(`(?m)^([ \t]*\{'=', )nv_operator(, 0, 0\},)$`, "${1}nv_error${2}", 1,
		"= in Normal and Visual mode points at nv_error")
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.Literal(oldDispatch, newDispatch, 1, "the filter and indent operators being dispatched")
	})
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.Literal(" || oap->op_type == OP_FILTER", "", 1, "a filter deciding whether the motion is inclusive")
	})
	e.InFunction("op_colon", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(oap->op_type != OP_COLON\)$`, 1, "the ! typed after an operator range")
	})
	e.InFunction("do_bang", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(bangredo\)$`, 1, "the ! operator putting its command in the redo buffer")
	})
	// That block held the only `goto theend`, and a label with nothing jumping
	// to it is a warning.  The free below it runs either way, so only the marker
	// goes.
	e.InFunction("do_bang", func(e *edit.E) {
		e.Lines(`theend:`, 1, "do_bang's label, which only the redo block jumped to")
	})

	// what C-indenting left behind.  Three of the ten writes are the whole Body
	// of an `if`, so the test goes with them rather than leaving an empty block.
	// Each is scoped to its function: `if (inindent(0))` also guards
	// do_pending_operator's `oap->motion_type = MLINE`, which stays, and an
	// unscoped drop would have had two matches to choose between.
	e.InFunction("edit", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(inindent\(0\)\)$`, 1, "a space typed in the indent forbidding a reindent")
	})
	e.InFunction("ins_bs", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(in_indent\)$`, 1, "a backspace in the indent forbidding a reindent")
	})
	e.InFunction("ins_tab", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(ind\)$`, 1, "a Tab in the indent forbidding a reindent")
	})
	e.Lines(`can_cindent = (?:TRUE|FALSE);`, 7, "the other places that armed or disarmed a reindent")
	e.InFunction("internal_format", func(e *edit.E) {
		e.Lines(`set_can_cindent\(TRUE\);`, 1, "a wrapped line arming a reindent")
	})

	// cindent_on() is `return FALSE`; its two callers fold and the sweep takes it.
	e.InFunction("ins_bs", func(e *edit.E) {
		e.Literal("if (mode == BACKSPACE_LINE && (curbuf->b_p_ai || cindent_on()))",
			"if (mode == BACKSPACE_LINE && curbuf->b_p_ai)", 1, "CTRL-U keeping the indent for 'autoindent' alone")
	})
	e.InFunction("insertchar", func(e *edit.E) {
		e.Literal(" && !cindent_on()", "", 1, "the multi-character insert asking whether C-indenting is on")
	})
	return e.Done()
}

func init() { edit.Register("whim64", Edit) }
