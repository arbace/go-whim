package p068

// Whim phase 68 -- one window, structurally.  See GOAL.md.
//
// THE INVARIANT THIS ESTABLISHES, and then spends:
//
// A window is created in exactly two places -- win_alloc_firstwin(), once at
// startup, and win_split_ins(), whose ONLY caller is aucmd_prepbuf().  win_split(),
// make_windows() and win_new_tabpage() have no mentions at all.  A tabpage is
// created once, by alloc_tabpage() in win_alloc_first().  So remove the
// autocommand window and nothing can add a window or a tabpage ever again:
//
// firstwin == lastwin  and  first_tabpage->tp_next == NULL
//
// one_window(), last_window() and only_one_window() are then constant TRUE, and
// every caller folds.  That is not a guess about the harness -- it is what the
// two creation sites allow.
//
// WHY THE AUTOCOMMAND WINDOW CAN GO.  aucmd_prepbuf() splits one open only when no
// window shows the buffer, in order to run autocommands there -- and
// apply_autocmds_group() has been `return FALSE` since the phase that removed
// autocommands.  The window would be built to run nothing.
//
// WHAT WAS ALREADY A NO-OP, which is why this removes capability from the source
// and none from the editor:
//
// win_close() tests last_window() first and answers "cannot close last window",
// so the calls in ex_quit(), ex_exit() and do_exedit() could never close
// anything.  ex_quit() and ex_exit() reach getout(0) before them anyway.
// do_exedit()'s call is guarded by old_curwin != NULL, and its one caller passes
// NULL.
// close_windows() loops `wp != NULL && !(firstwin == lastwin)`, false at once,
// and then over tabpages other than curtab, of which there are none.
//
// WHAT STAYS, because it is not about having two windows: win_comp_pos() and
// last_status() are reached from shell_new_rows() and did_set_laststatus(), so a
// terminal resize and :set laststatus still compute the one window's geometry.
// The frame code does not vanish wholesale, and the probes check that.
//
// THE DELTA: none.  No key, command or option changes.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim68 makes one window and one tabpage an invariant: the autocommand window
// goes, one_window(), last_window() and only_one_window() become constant TRUE,
// and everything those tests guarded goes with them.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("onewindow", text, w)

	// 1. the autocommand window, the last thing that could add a window
	e.InFunction("aucmd_prepbuf", func(e *edit.E) {
		e.Splice("    win_T *auc_win = NULL;\n", "        if (auc_win == NULL)\n        {\n            return;\n        }\n    }\n",
			"    if (win == NULL)\n    {\n        return;\n    }\n",
			"aucmd_prepbuf building a window to run autocommands in")
		e.Splice("    if (win != NULL)\n", "        curwin = auc_win;\n    }\n", "    curwin = win;\n",
			"aucmd_prepbuf choosing between that window and the real one")
	})
	// fold_never, not drop_if: this `if` HAS an else -- the same-window restore
	// -- and DropIf refuses that shape on purpose, since deleting the if alone
	// would orphan the else.  FoldNever keeps the else Body, which is what is
	// left when the index can never be >= 0.
	e.InFunction("aucmd_restbuf", func(e *edit.E) {
		e.FoldNever(edit.Head("if (aco->use_aucmd_win_idx >= 0)"), 1, "aucmd_restbuf taking that window down again")
	})
	// Both writes to use_aucmd_win_idx went with the branch above, and its only
	// reader went with aucmd_restbuf's folded test, so the sweep takes the
	// field, as it takes win_alloc_popup_win() and win_init_popup_win(),
	// which only the autocommand window used.
	//
	// Three places MANAGE the aucmd_win[] table without ever reading it -- the
	// can_cindent shape again.  Each is guarded by auc_win != NULL, which
	// nothing can make true now.  With them gone the sweep takes the table and
	// autocmd_init(), whose body was the memset that zeroed it.
	e.Cut(edit.Line("autocmd_init();"), 1, "the call that zeroed the table at startup")
	e.Cut(edit.Line("for (int i = 0; i < AUCMD_WIN_COUNT; ++i)", "{", "if (aucmd_win[i].auc_win != NULL)", "{", "win_free_lsize(aucmd_win[i].auc_win);", "}", "}"), 1,
		"screenalloc freeing the line sizes of windows that do not exist")
	e.Cut(edit.Line("for (int i = 0; i < AUCMD_WIN_COUNT; ++i)", "{", "if (aucmd_win[i].auc_win != NULL && aucmd_win[i].auc_win->w_lines == NULL && win_alloc_lines(aucmd_win[i].auc_win) == FAIL)", "{", "outofmem = TRUE;", "break;", "}", "}"), 1,
		"screenalloc allocating lines for them")

	// 2. the invariant: one window, one tabpage
	e.Body("one_window", "    return TRUE;\n", "one_window() is constant TRUE")
	e.Body("last_window", "    return TRUE;\n", "last_window() is constant TRUE")
	e.Body("only_one_window", "    return TRUE;\n", "only_one_window() is constant TRUE")

	// 3. what those tests guarded
	e.InFunction("ex_quit", func(e *edit.E) {
		e.Sub(`(?m)^[ \t]*if \(eap->addr_count > 0\)\n[ \t]*\{\n(?s:.*?)[ \t]*\}\n[ \t]*else\n[ \t]*\{\n[ \t]*wp = curwin;\n[ \t]*\}\n`,
			"    wp = curwin;\n", 1, ":quit with a window count, of which there is one")
	})
	e.InFunction("ex_quit", func(e *edit.E) {
		e.FoldAlways(edit.Head("if (only_one_window() && ((firstwin == lastwin) || eap->addr_count == 0))"), 1,
			":quit leaving the editor")
	})
	e.InFunction("ex_quit", func(e *edit.E) {
		e.Cut(edit.Line("win_close(wp, TRUE);"), 1, ":quit closing a window it can never reach")
	})
	e.InFunction("ex_exit", func(e *edit.E) {
		e.FoldAlways(edit.Head("if (only_one_window())"), 1, ":xit leaving the editor")
	})
	e.InFunction("ex_exit", func(e *edit.E) {
		e.Cut(edit.Line("win_close(curwin, TRUE);"), 1, ":xit closing a window it can never reach")
	})
	e.InFunction("do_exedit", func(e *edit.E) {
		e.DropIf(edit.Head("if (old_curwin != NULL)"), 1, ":edit closing the window it came from, which is never given one")
	})
	e.InFunction("set_curbuf", func(e *edit.E) {
		e.DropIf(edit.Head("if (unload)"), 1, "unloading a buffer closing the windows that show it")
	})
	// only_one_window() is TRUE, so these terms go rather than the tests.
	e.InFunction("check_more", func(e *edit.E) {
		e.Literal("only_one_window() && ", "", 1, "check_more asking how many windows there are")
	})
	e.InFunction("before_quit_autocmds", func(e *edit.E) {
		e.Literal(" && only_one_window()", "", 1, "the quit autocommands asking how many windows there are")
	})
	e.InFunction("create_windows", func(e *edit.E) {
		e.Literal("got_int || only_one_window()", "TRUE", 1, "the swap-file quit asking how many windows there are")
	})
	e.InFunction("close_buffer", func(e *edit.E) {
		e.Literal("abort_if_last && one_window()", "abort_if_last", 2,
			"closing a buffer asking whether its window is the last")
	})
	return e.Done()
}

func init() { phase.Register("whim68", Edit) }
