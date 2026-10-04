package p020

// Whim phase 20 (formerly 62) -- no buffer-type, file-type, listing, jump, update-time or autowrite options.  See GOAL.md.
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

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim20 takes five buffer options: 'autowrite' and 'autowriteall', 'buftype',
// 'jumpoptions', 'updatetime' and 'buflisted', and 'filetype' with them.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the same acts in the same
// order, each found by its C-lisp form and counted, its report the text
// version's (history keeps it).  A substring cut from a condition is
// DropOperand; the expired-wait block, which the text matched by its ends,
// is the if's then-block rewritten.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nobufopts", e, w)

	// 'autowrite' and 'autowriteall' -- first, since autowrite() reads bt_dontwrite()
	// getfile went with its last caller, the file-mark jump, at phase 3
	// (whim3f, on the front)
	v.InFunction("check_changed", func(v *graph.Verbs) {
		v.DropOperand("(|| (! (& flags CCGD_AW)) (== (call autowrite buf forceit) FAIL))", 1,
			"a changed buffer trying autowrite first")
	})
	v.InFunction("nv_gotofile", func(v *graph.Verbs) {
		v.DropIf("(&& (call curbufIsChanged) (<= (-> curbuf b_nwindows) 1))", 1, "gf writing the buffer first")
	})
	// check_overwrite, do_bang and do_write went with :write and :! at phase 1
	// (D2, D4)
	v.InFunction("ex_stop", func(v *graph.Verbs) {
		v.Cut("(if (! (-> eap forceit)) (block (call autowrite_all)))", 1, ":stop writing all buffers first")
	})
	// one: ex_quit's and check_changed_any's went with :q's refusal at phase 1
	// (quitfront, phase 33's move)
	v.DropOperand("(? p_awa CCGD_AW 0)", 1, "'autowriteall' asking check_changed to write")
	// :next and the argument list asked check_changed to autowrite; they
	// died with the commands, retired at phase 1 (exfront, the reform's D2)

	// 'buftype'
	v.InFunction("fileinfo", func(v *graph.Verbs) {
		for _, f := range []struct{ flag, what string }{
			{"BF_NOTEDITED", "[Not edited] shown only for a writable 'buftype'"},
			{"BF_NEW", "[New] shown only for a writable 'buftype'"},
		} {
			v.Rewrite("(&& ?k:(paren (& (-> curbuf b_flags) "+f.flag+")) (! (call bt_dontwrite curbuf)))", "?k", 1, f.what)
		}
	})
	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.DropOperand("(! (call bt_nofilename buf))", 1, "a first :w naming the buffer unless 'buftype' forbids it")
		v.FoldNever("(&& overwriting (call bt_nofilename curbuf))", 3, "writing a no-file buffer refused")
		v.FoldNever("nofile_err", 2, "the no-file refusal reported")
		v.DropOperand("nofile_err", 1, "an autocommand check waiting on the no-file refusal")
	})
	v.InFunction("buf_copy_options", func(v *graph.Verbs) {
		v.DropIf("(== (index (-> buf b_p_bt) 0) 'h')", 1, "a help buffer's 'buftype' cleared on copy")
	})
	v.InFunction("changed", func(v *graph.Verbs) {
		v.DropOperand("(! (call bt_dontwrite curbuf))", 1, "the swap file opened only for a writable 'buftype'")
	})
	// readfile went at phase 1 (readfront, phase 31's move)
	v.InFunction("open_buffer", func(v *graph.Verbs) {
		v.DropIf("(call bt_nofileread curbuf)", 1, "a no-file 'buftype' skipping the read")
	})
	v.InFunction("shorten_buf_fname", func(v *graph.Verbs) {
		v.DropOperand("(! (call bt_nofilename buf))", 1, "a no-file buffer's name not shortened")
	})
	v.InFunction("buf_spname", func(v *graph.Verbs) {
		v.DropIf("(call bt_nofilename buf)", 1, "[Scratch] for a no-file buffer")
	})
	v.InFunction("edit", func(v *graph.Verbs) {
		v.DropOperand("(! (call bt_prompt (-> curwin w_buffer)))", 1, "leaving Insert mode differently in a prompt buffer")
	})
	v.InFunction("bufIsChangedNotTerm", func(v *graph.Verbs) {
		v.Rewrite("(return (&& (|| (! (call bt_dontwrite buf)) (call bt_prompt buf)) (paren ?changed)))",
			"(return ?changed)", 1, "a changed buffer judged by 'buftype'")
	})

	// 'jumpoptions'
	v.InFunction("setpcmark", func(v *graph.Verbs) {
		v.DropIf("(& jop_flags JOP_STACK)", 1, "the jump list as a stack")
	})
	// cleanup_jumplist went with :jumps and CTRL-O at phase 3 (whim3c, on
	// the front)
	v.InFunction("didset_string_options", func(v *graph.Verbs) {
		v.Cut("(cast void (call opt_strings_flags p_jop p_jop_values (addr jop_flags) TRUE))", 1, "startup parsing 'jumpoptions'")
	})

	// 'updatetime': the idle wait did nothing, so it goes
	v.InFunction("inchar_loop", func(v *graph.Verbs) {
		v.DropOperand("did_start_blocking", 1, "a wait with no timeout blocking at once")
		v.FoldAlwaysElse("(if (>= wtime 0) _ (block (= wait_time (- p_ut elapsed_time))))", 1, "the 'updatetime' idle timeout")
		// The expired-wait block is replaced by its own head plus `return 0;`:
		// the CursorHold handling it held has nothing to do once the idle
		// wait is gone.
		v.RewriteAt("(if (&& (<= wait_time 0) did_call_wait_func) ?then)", "then", "(block (return 0))", 1,
			"CursorHold and before_blocking() after the idle wait")
		// Blocking now starts on the first wait with no timeout, so by the time
		// the loop's exit test runs for one, it has blocked: did_start_blocking
		// was TRUE there, and an interrupted indefinite wait must still return 0
		// rather than block again.
		v.DropOperand("(paren (&& (< wtime 0) (! did_start_blocking)))", 1, "an interrupted indefinite wait returning instead of blocking again")
	})
	v.InFunction("gotchars", func(v *graph.Verbs) {
		v.Cut("(for (= i 0) (< i (. state buflen)) (pre++ i) (block (call updatescript _)))", 1,
			"typed characters passed to a script file and a swap sync that are both gone")
	})
	v.InFunction("wait_return", func(v *graph.Verbs) {
		for _, s := range []string{"(= save_scriptout scriptout)", "(= scriptout nullptr)", "(= scriptout save_scriptout)"} {
			v.Cut(s, 1, "wait_return saving and restoring a script file that is never open")
		}
	})
	v.InFunction("check_num_option_bounds", func(v *graph.Verbs) {
		v.DropIf("(< p_ut 0)", 1, "'updatetime' kept non-negative")
	})

	// 'buflisted'
	v.InFunction("buf_freeall", func(v *graph.Verbs) {
		v.DropIf("(&& (paren (& flags BFA_DEL)) (-> buf b_p_bl))", 1, "BufDelete for a listed buffer")
	})
	// buflist_findpat's body is phase 4d's one-buffer search from phase 4
	// (whim4d, which runs before this phase now)
	v.InFunction("buflist_new", func(v *graph.Verbs) {
		v.DropIf("(&& (paren (& flags BLN_LISTED)) (! (-> buf b_p_bl)))", 1, "an existing buffer becoming listed")
		v.Cut("(= (-> buf b_p_bl) (? (paren (& flags BLN_LISTED)) TRUE FALSE))", 1, "a new buffer recording whether it is listed")
		v.DropIf("(& flags BLN_LISTED)", 1, "BufAdd for a new listed buffer")
	})
	v.InFunction("close_buffer", func(v *graph.Verbs) {
		v.Cut("(if del_buf (block (= (-> buf b_p_bl) FALSE)))", 1, "a deleted buffer becoming unlisted")
	})
	v.InFunction("set_rw_fname", func(v *graph.Verbs) {
		v.FoldNever("(-> curbuf b_p_bl)", 2, "BufDelete and BufAdd around a renamed listed buffer")
	})
	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.Cut("(block (if (! (-> curbuf b_help)) (block (call set_buflisted TRUE))))", 1, "an edited buffer becoming listed")
	})
	// read_stdin()'s went with it at phase 1: nothing wrote the edit type
	// that called it once the command line was cut (argvfront, D1), and
	// create_windows()'s with its body at phase 5 (whim5b, phase 5b's
	// program, which runs before this phase now): the help buffer's is left
	v.Cut("(call set_buflisted _)", 1, "the help buffer setting whether it is listed")

	// 'filetype'
	v.InFunction("enter_buffer", func(v *graph.Verbs) {
		v.DropIf("(== (deref (-> curbuf b_p_ft)) NUL)", 1, "entering a buffer with no 'filetype' forgetting FileType")
	})
	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.Cut("(= (-> curbuf b_did_filetype) false)", 1, ":edit forgetting FileType")
	})
	// readfile and fix_help_buffer went at phase 1 (readfront, phase 31's move)
	v.InFunction("did_set_string_option", func(v *graph.Verbs) {
		v.FoldNever("(== varp (addr (paren (-> curbuf b_p_ft))))", 1, ":set ft= firing FileType")
	})
	return v.Done()
}

// Whim20BL takes the get_varp case of 'buflisted'; the field, named by
// nothing after it, goes to the collection.  A second entry, standing where
// its heredoc stood.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the case label, heading a run
// of its own, goes with its `return` (DropCase).
func Whim20BL(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nobufopts", e, w)
	v.InFunction("get_varp", func(v *graph.Verbs) {
		v.DropCase("(case (cast idopt_T (+ PV_BUF (cast int (paren BV_BL)))))", 1, "'buflisted''s get_varp case removed")
	})
	return v.Done()
}

func init() {
	phase.RegisterGraph("whim20", Edit)
	phase.RegisterGraph("whim20bl", Whim20BL)
}
