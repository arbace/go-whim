package p062

// Whim phase 62 -- no buffer-type, file-type, listing, jump, update-time or autowrite options.  See GOAL.md.
//
// 'buflisted'     every reader chose which autocommand event to fire, and
// apply_autocmds_group() is `return FALSE` -- or searched a
// buffer list of one
// 'filetype'      every reader fed the FileType event, which cannot fire, or
// fix_help_buffer(), and :help is ex_ni
// 'buftype'       live only through :set bt=: nofile, nowrite, acwrite and prompt
// refused :w and skipped reading; help set b_help.  Nothing
// inside the editor ever set it.  bt_dontwrite(),
// bt_nofilename(), bt_nofileread() and bt_prompt() fold as false
// at every caller
// 'jumpoptions'   empty: the "stack" behaviour of the jump list folds away
// 'updatetime'    dead, though it looked live: after that long idle,
// inchar_loop() asked trigger_cursorhold(), which is `return
// FALSE`, and called before_blocking(), whose swap sync reaches
// an empty ml_sync_all() and whose terminal flush only acts
// inside a screen redraw, never at idle.  So the idle wait goes:
// a wait with no timeout blocks at once, and before_blocking(),
// updatescript() and ml_sync_all() go with the CursorHold probe
// 'autowrite'     off: autowrite() always failed and autowrite_all() returned,
// 'autowriteall'  so their callers and the CCGD_AW flag fold
//
// THE DELTA: none the harnesses record.  The probes check the seven are unknown
// and that :w still writes.
// 'buflisted' leaves too little plumbing for droplocal.py: buflist_new() was its
// initialiser, and the folds above took that.  What is left is the field and its
// get_varp() case, and they go by hand.

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim62 takes five buffer options: 'autowrite' and 'autowriteall', 'buftype',
// 'jumpoptions', 'updatetime' and 'buflisted', and 'filetype' with them.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nobufopts", text, w)

	// 'autowrite' and 'autowriteall' -- first, since autowrite() reads bt_dontwrite()
	e.InFunction("getfile", func(e *edit.E) {
		e.Literal(" && autowrite(curbuf, forceit) == FAIL)", ")", 1, "switching files trying autowrite first")
	})
	e.InFunction("check_changed", func(e *edit.E) {
		e.Literal(" && (!(flags & CCGD_AW) || autowrite(buf, forceit) == FAIL))", ")", 1, "a changed buffer trying autowrite first")
	})
	e.InFunction("nv_gotofile", func(e *edit.E) {
		e.DropIf(edit.Head("if (curbufIsChanged() && curbuf->b_nwindows <= 1)"), 1, "gf writing the buffer first")
	})
	e.InFunction("do_bang", func(e *edit.E) {
		e.Cut(edit.Line("if (addr_count == 0)", "{", "msg_scroll = FALSE;", "autowrite_all();", "msg_scroll = scroll_save;", "}"), 1, ":! writing all buffers first")
	})
	e.InFunction("ex_stop", func(e *edit.E) {
		e.Cut(edit.Line("if (!eap->forceit)", "{", "autowrite_all();", "}"), 1, ":stop writing all buffers first")
	})
	e.Literal("(p_awa ? CCGD_AW : 0) | ", "", 3, "'autowriteall' asking check_changed to write")
	e.Literal("CCGD_AW | ", "", 2, ":next and the argument list asking check_changed to autowrite")

	// 'buftype'
	e.InFunction("fileinfo", func(e *edit.E) {
		e.Literal("(curbuf->b_flags & BF_NOTEDITED) && !bt_dontwrite(curbuf) ?", "(curbuf->b_flags & BF_NOTEDITED) ?", 1, "[Not edited] shown only for a writable 'buftype'")
		e.Literal("(curbuf->b_flags & BF_NEW) && !bt_dontwrite(curbuf) ?", "(curbuf->b_flags & BF_NEW) ?", 1, "[New] shown only for a writable 'buftype'")
	})
	e.InFunction("buf_write", func(e *edit.E) {
		e.Literal(" && !bt_nofilename(buf)", "", 1, "a first :w naming the buffer unless 'buftype' forbids it")
		e.FoldNever(edit.Head("if (overwriting && bt_nofilename(curbuf))"), 3, "writing a no-file buffer refused")
		e.FoldNever(edit.Head("if (nofile_err)"), 2, "the no-file refusal reported")
		e.Literal(" || did_cmd || nofile_err)", " || did_cmd)", 1, "an autocommand check waiting on the no-file refusal")
	})
	e.InFunction("buf_copy_options", func(e *edit.E) {
		e.DropIf(edit.Head("if (buf->b_p_bt[0] == 'h')"), 1, "a help buffer's 'buftype' cleared on copy")
	})
	e.InFunction("changed", func(e *edit.E) {
		e.Literal("if (curbuf->b_may_swap && !bt_dontwrite(curbuf))", "if (curbuf->b_may_swap)", 1, "the swap file opened only for a writable 'buftype'")
	})
	e.InFunction("readfile", func(e *edit.E) {
		e.FoldAlways(edit.Head("if (!bt_dontwrite(curbuf))"), 2, "reading checking for a swap file only for a writable 'buftype'")
	})
	e.InFunction("open_buffer", func(e *edit.E) {
		e.DropIf(edit.Head("if (bt_nofileread(curbuf))"), 1, "a no-file 'buftype' skipping the read")
	})
	e.InFunction("do_write", func(e *edit.E) {
		e.Literal("(bt_dontwrite_msg(curbuf) || check_fname() == FAIL", "(check_fname() == FAIL", 1, ":w refused for 'buftype'")
	})
	e.InFunction("check_overwrite", func(e *edit.E) {
		e.Literal("!bt_nofilename(buf) && ", "", 1, "the overwrite check skipping a no-file buffer")
	})
	e.InFunction("shorten_buf_fname", func(e *edit.E) {
		e.Literal(" && !bt_nofilename(buf)", "", 1, "a no-file buffer's name not shortened")
	})
	e.InFunction("buf_spname", func(e *edit.E) {
		e.DropIf(edit.Head("if (bt_nofilename(buf))"), 1, "[Scratch] for a no-file buffer")
	})
	e.InFunction("edit", func(e *edit.E) {
		e.Literal("if (!bt_prompt(curwin->w_buffer) && stop_insert_mode)", "if (stop_insert_mode)", 1, "leaving Insert mode differently in a prompt buffer")
	})
	e.InFunction("bufIsChangedNotTerm", func(e *edit.E) {
		e.Sub(`return \(!bt_dontwrite\(buf\) \|\| bt_prompt\(buf\)\)\s*&& \(buf->b_changed\);`, "return buf->b_changed;", 1, "a changed buffer judged by 'buftype'")
	})

	// 'jumpoptions'
	e.InFunction("setpcmark", func(e *edit.E) {
		e.DropIf(edit.Head("if (jop_flags & JOP_STACK)"), 1, "the jump list as a stack")
	})
	e.InFunction("cleanup_jumplist", func(e *edit.E) {
		e.Literal("mustfree = !(jop_flags & JOP_STACK);", "mustfree = TRUE;", 1, "duplicate jumps kept for a stack")
	})
	e.InFunction("didset_string_options", func(e *edit.E) {
		e.Cut(edit.Line("(void)opt_strings_flags(p_jop, p_jop_values, &jop_flags, TRUE);"), 1, "startup parsing 'jumpoptions'")
	})

	// 'updatetime': the idle wait did nothing, so it goes
	e.InFunction("inchar_loop", func(e *edit.E) {
		e.Literal("if (wtime < 0 && did_start_blocking)", "if (wtime < 0)", 1, "a wait with no timeout blocking at once")
		e.Sub(`(?m)^([ \t]*)if \(wtime >= 0\)\n[ \t]*\{\n[ \t]*wait_time = wtime - elapsed_time;\n[ \t]*\}\n[ \t]*else\n[ \t]*\{\n[ \t]*wait_time = p_ut - elapsed_time;\n[ \t]*\}\n`,
			"${1}wait_time = wtime - elapsed_time;\n", 1, "the 'updatetime' idle timeout")
		// The expired-wait block is replaced by its own head plus `return 0;`.
		// It is matched by its ENDS rather than by one pattern: the Body is
		// CursorHold handling that has nothing in common with the test above it.
		e.Splice("            if (wait_time <= 0 && did_call_wait_func)\n",
			"                before_blocking();\n                continue;\n            }\n",
			"            if (wait_time <= 0 && did_call_wait_func)\n            {\n                return 0;\n            }\n",
			"CursorHold and before_blocking() after the idle wait")
		// Blocking now starts on the first wait with no timeout, so by the time
		// the loop's exit test runs for one, it has blocked: did_start_blocking
		// was TRUE there, and an interrupted indefinite wait must still return 0
		// rather than block again.
		e.Literal(" || (wtime < 0 && !did_start_blocking))", ")", 1, "an interrupted indefinite wait returning instead of blocking again")
	})
	e.InFunction("gotchars", func(e *edit.E) {
		e.Cut(edit.Line("for (i = 0; i < state.buflen; ++i)", "{", "updatescript(state.buf[i]);", "}"), 1, "typed characters passed to a script file and a swap sync that are both gone")
	})
	e.InFunction("wait_return", func(e *edit.E) {
		for _, line := range []string{"save_scriptout = scriptout;", "scriptout = NULL;", "scriptout = save_scriptout;"} {
			e.Cut(`(?m)^[ \t]*`+regexp.QuoteMeta(line)+`\n`, 1, "wait_return saving and restoring a script file that is never open")
		}
	})
	e.InFunction("check_num_option_bounds", func(e *edit.E) {
		e.DropIf(edit.Head("if (p_ut < 0)"), 1, "'updatetime' kept non-negative")
	})

	// 'buflisted'
	e.InFunction("buf_freeall", func(e *edit.E) {
		e.DropIf(edit.Head("if ((flags & BFA_DEL) && buf->b_p_bl)"), 1, "BufDelete for a listed buffer")
	})
	e.InFunction("buflist_findpat", func(e *edit.E) {
		e.Literal("buf->b_p_bl == find_listed && ", "find_listed && ", 1, "a buffer search telling listed from unlisted")
	})
	e.InFunction("buflist_new", func(e *edit.E) {
		e.DropIf(edit.Head("if ((flags & BLN_LISTED) && !buf->b_p_bl)"), 1, "an existing buffer becoming listed")
		e.Cut(edit.Line("buf->b_p_bl = (flags & BLN_LISTED) ? TRUE : FALSE;"), 1, "a new buffer recording whether it is listed")
		e.DropIf(edit.Head("if (flags & BLN_LISTED)"), 1, "BufAdd for a new listed buffer")
	})
	e.InFunction("close_buffer", func(e *edit.E) {
		e.Cut(edit.Line("if (del_buf)", "{", "buf->b_p_bl = FALSE;", "}"), 1, "a deleted buffer becoming unlisted")
	})
	e.InFunction("set_rw_fname", func(e *edit.E) {
		e.FoldNever(edit.Head("if (curbuf->b_p_bl)"), 2, "BufDelete and BufAdd around a renamed listed buffer")
	})
	e.InFunction("do_ecmd", func(e *edit.E) {
		e.Cut(edit.Line("else", "{", "if (!curbuf->b_help)", "{", "set_buflisted(TRUE);", "}", "}"), 1, "an edited buffer becoming listed")
	})
	e.Cut(`(?m)^[ \t]*set_buflisted\((?:TRUE|FALSE)\);\n`, 3, "stdin, startup and help buffers setting whether they are listed")

	// 'filetype'
	e.InFunction("enter_buffer", func(e *edit.E) {
		e.DropIf(edit.Head("if (*curbuf->b_p_ft == NUL)"), 1, "entering a buffer with no 'filetype' forgetting FileType")
	})
	e.InFunction("do_ecmd", func(e *edit.E) {
		e.Cut(edit.Line("curbuf->b_did_filetype = false;"), 1, ":edit forgetting FileType")
	})
	e.InFunction("readfile", func(e *edit.E) {
		e.Cut(edit.Line("curbuf->b_au_did_filetype = false;"), 1, "reading forgetting FileType")
		e.DropIf(edit.Head("if (!curbuf->b_au_did_filetype && *curbuf->b_p_ft != NUL)"), 1, "reading firing FileType")
	})
	e.InFunction("did_set_string_option", func(e *edit.E) {
		e.FoldNever(edit.Head("else if (varp == &(curbuf->b_p_ft))"), 1, ":set ft= firing FileType")
	})
	e.InFunction("fix_help_buffer", func(e *edit.E) {
		e.DropIf(edit.Head(`if (strcmp((char *)(curbuf->b_p_ft), (char *)("help")) != 0)`), 1, "a help buffer setting 'filetype' to help")
	})
	return e.Done()
}

// Whim62BL takes 'buflisted”s get_varp case; the field, named by nothing
// after it, goes to the sweep.  A second entry, standing where its heredoc
// stood.
func Whim62BL(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nobufopts", text, w)
	e.Cut(`(?m)^[ \t]*case[^\n]*\bBV_BL\b[^\n]*\n[ \t]*return \(char_u \*\)&\(curbuf->b_p_bl\);\n`, 1, "'buflisted''s get_varp case removed")
	return e.Done()
}

func init() {
	phase.Register("whim62", Edit)
	phase.Register("whim62bl", Whim62BL)
}
