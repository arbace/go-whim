package p004c

// Whim phase 4c (formerly 68) -- one window, structurally.  See GOAL.md.
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

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's splices,
// lines and literals are acts on the nodes -- a run of items replaced, an
// if-else rewritten, a fold, loops cut, bodies, operands dropped -- each
// counted, its report the text's (history keeps it).

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim4c makes one window and one tabpage an invariant: the autocommand window
// goes, one_window(), last_window() and only_one_window() become constant TRUE,
// and everything those tests guarded goes with them.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("onewindow", e, w)

	// 1. the autocommand window, the last thing that could add a window
	v.InFunction("aucmd_prepbuf", func(v *graph.Verbs) {
		v.Splice("(def auc_win (ptr win_T) nullptr)",
			"(if (== win nullptr) (block (for _*) (if (== auc_win nullptr) (block (return)))))",
			"(if (== win nullptr) (block (return)))",
			"aucmd_prepbuf building a window to run autocommands in")
		v.Rewrite("(if (!= win nullptr) (block (= (-> aco use_aucmd_win_idx) (- 1)) (= curwin win)) (block (= (-> aco use_aucmd_win_idx) auc_idx) _*))", "(= curwin win)", 1,
			"aucmd_prepbuf choosing between that window and the real one")
	})
	// fold_never, not drop_if: this `if` HAS an else -- the same-window restore
	// -- and DropIf refuses that shape on purpose, since deleting the if alone
	// would orphan the else.  FoldNever keeps the else Body, which is what is
	// left when the index can never be >= 0.
	v.InFunction("aucmd_restbuf", func(v *graph.Verbs) {
		v.FoldNever("(>= (-> aco use_aucmd_win_idx) 0)", 1, "aucmd_restbuf taking that window down again")
	})
	// Both writes to use_aucmd_win_idx went with the branch above, and its only
	// reader went with aucmd_restbuf's folded test, so the collection takes
	// the field, as it takes win_alloc_popup_win() and win_init_popup_win(),
	// which only the autocommand window used.
	//
	// Three places MANAGE the aucmd_win[] table without ever reading it -- the
	// can_cindent shape again.  Each is guarded by auc_win != NULL, which
	// nothing can make true now.  With them gone the collection takes the
	// table and autocmd_init(), whose body was the memset that zeroed it.
	v.Cut("(call autocmd_init)", 1, "the call that zeroed the table at startup")
	aucwin := "(. (index aucmd_win i) auc_win)"
	v.Cut("(for (def i int 0) (< i AUCMD_WIN_COUNT) (pre++ i) (block (if (!= "+aucwin+" nullptr) (block (call win_free_lsize "+aucwin+")))))", 1,
		"screenalloc freeing the line sizes of windows that do not exist")
	v.Cut("(for (def i int 0) (< i AUCMD_WIN_COUNT) (pre++ i) (block (if (&& (!= "+aucwin+" nullptr) (== (-> "+aucwin+" w_lines) nullptr) (== (call win_alloc_lines "+aucwin+") FAIL)) (block (= outofmem TRUE) (break)))))", 1,
		"screenalloc allocating lines for them")

	// 2. the invariant: one window, one tabpage
	v.Body("one_window", "(return TRUE)", "one_window() is constant TRUE")
	v.Body("last_window", "(return TRUE)", "last_window() is constant TRUE")
	v.Body("only_one_window", "(return TRUE)", "only_one_window() is constant TRUE")

	// 3. what those tests guarded
	v.InFunction("ex_quit", func(v *graph.Verbs) {
		v.Rewrite("(if (> (-> eap addr_count) 0) (block _*) (block (= wp curwin)))", "(= wp curwin)", 1,
			":quit with a window count, of which there is one")
		v.FoldAlways("(&& (call only_one_window) (|| (paren (== firstwin lastwin)) (== (-> eap addr_count) 0)))", 1,
			":quit leaving the editor")
		v.Cut("(call win_close wp TRUE)", 1, ":quit closing a window it can never reach")
	})
	// do_exedit and ex_exit went with :edit and :exit at phase 1 (filefront, the
	// reform's D4)
	// set_curbuf went with ex_quit's refusal at phase 1 (quitfront, phase 33's move)
	// only_one_window() is TRUE, so these terms go rather than the tests.
	v.InFunction("check_more", func(v *graph.Verbs) {
		v.DropOperand("(call only_one_window)", 1, "check_more asking how many windows there are")
	})
	v.InFunction("before_quit_autocmds", func(v *graph.Verbs) {
		v.DropOperand("(call only_one_window)", 1, "the quit autocommands asking how many windows there are")
	})
	v.InFunction("create_windows", func(v *graph.Verbs) {
		v.Rewrite("(|| got_int (call only_one_window))", "TRUE", 1, "the swap-file quit asking how many windows there are")
	})
	v.InFunction("close_buffer", func(v *graph.Verbs) {
		v.Rewrite("(&& abort_if_last (call one_window))", "abort_if_last", 2,
			"closing a buffer asking whether its window is the last")
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim4c", Edit) }
