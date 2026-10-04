package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoTabs removes tab pages: nothing makes or reaches a second one.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's line folds are
// folds by condition, its literals Rewrites, operands dropped, bodies and
// runs by template, CTRL-W T's case run cut, each counted and reported as
// the text did (history keeps the text version).
func NoTabs(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("notabs", e, w)
	q := graph.NewVerbs("notabs", e, io.Discard)
	quiet := func(acts func(*graph.Verbs), what string) {
		if v.Failed() {
			return
		}
		acts(q)
		if q.Err != nil {
			v.Err = q.Err
			return
		}
		v.Say(what)
	}

	v.DropIf(`(call checkforcmd_noparen (addr p) "tab" 3)`, 1, "the :tab modifier")
	quiet(func(q *graph.Verbs) {
		q.Body("tabline_height", "(return 0)", "tabline_height")
		q.Body("draw_tabline", "(= redraw_tabline FALSE)", "draw_tabline")
	}, "the tab line is 0 lines and draws nothing")

	// A stub answers the question; it does not remove the caller.  win_split()
	// asked may_open_tabpage() whether a :tab-modified split had become a tab
	// page, and nothing can modify one now -- so the question goes, and the
	// sweep takes the function.
	v.InFunction("win_split", func(v *graph.Verbs) {
		v.FoldNever("(== (call may_open_tabpage) OK)", 1, "win_split opening a tab page for :tab")
	})
	v.DropOperand("(!= (-> cmod cmod_tab) 0)", 1, "has_cmdmod counting :tab")
	v.DropOperand("(== (. cmdmod cmod_tab) 0)", 1, "CTRL-W's 'switchbuf' test for :tab")
	v.InFunction("open_cmdwin", func(v *graph.Verbs) {
		v.Cut("(= (. cmdmod cmod_tab) 0)", 1, "the command-line window clearing :tab")
	})
	// :argedit's, :drop's and :wincmd's tests for :tab, and :ball's, :all's
	// and :tabdo's tab-page loops, died with those commands, retired at
	// phase 1 (exfront, the reform's D2)

	v.InFunction("ex_splitview", func(v *graph.Verbs) {
		v.FoldNever("use_tab", 1, "ex_splitview opening a tab page")
	})
	v.InFunction("do_exedit", func(v *graph.Verbs) {
		quiet(func(q *graph.Verbs) {
			q.In(v.Scope(), func(q *graph.Verbs) {
				q.DropOperand("(== (-> eap cmdidx) CMD_tabnew)", 1, "")
				q.DropOperand("(== (-> eap cmdidx) CMD_tabedit)", 1, "")
			})
		}, ":tabnew and :tabedit with no file in do_exedit")
	})
	// close_disallowed falls out in phase 2's closure (window_layout_lock()'s
	// callers, autocommand triggers, went with the front's cuts), and its
	// term with it
	v.FoldNever("(== cmd CMD_tabnew)", 1, "window_layout_locked naming :tabnew")

	v.InFunction("nv_g_cmd", func(v *graph.Verbs) {
		v.Rewrite("(if (! (call checkclearop oap)) (block (call goto_tabpage (cast int (-> cap count0)))))",
			"(if (&& (! (call checkclearop oap)) (> (-> cap count0) 1)) (block (call beep_flush)))", 1,
			"gt: a count above 1 beeps, as with one tab page")
		v.Rewrite("(if (! (call checkclearop oap)) (block (call goto_tabpage (- (cast int (-> cap count1))))))",
			"(cast void (call checkclearop oap))", 1, "gT: nothing, as with one tab page")
		v.Rewrite("(&& ?a (== (call goto_tabpage_lastused) FAIL))", "?a", 1,
			"g<Tab>: beeps, there being no last-used tab page")
	})
	v.InFunction("nv_pcmark", func(v *graph.Verbs) {
		v.FoldAlways("(== (call goto_tabpage_lastused) FAIL)", 1, "CTRL-Tab: beeps")
	})
	v.InFunction("nv_page", func(v *graph.Verbs) {
		v.Rewrite("(if (== (-> cap arg) ?m) (block (call goto_tabpage _)) (block (call goto_tabpage _)))",
			"(if (&& (!= (-> cap arg) ?m) (> (-> cap count0) 1)) (block (call beep_flush)))", 1,
			"CTRL-PageUp and CTRL-PageDown: the one-tab-page answer")
	})
	for _, fn := range []string{"ins_pageup", "ins_pagedown"} {
		v.InFunction(fn, func(v *graph.Verbs) {
			v.FoldNever("(!= (-> first_tabpage tp_next) nullptr)", 1, fn+": no second tab page to reach")
		})
	}

	v.InFunction("do_window", func(v *graph.Verbs) {
		run := v.Run("CTRL-W T is not where this expects", "(case 'T')", "(if (!= cmdwin_type 0) _)",
			"(if (call one_window) _ _)", "(break)", "(case 't')")
		if run == nil {
			return
		}
		ok := false
		v.In(run[2], func(v *graph.Verbs) { ok = v.Count("(call win_new_tabpage _)") == 1 })
		v.Expect(ok, "the CTRL-W T block does not open a tab page")
		quiet(func(q *graph.Verbs) {
			if err := e.ReplaceRun(run[0], run[3]); err != nil {
				q.Die("CTRL-W T -- %v", err)
			}
		}, "CTRL-W T: moving a window to a new tab page")
		v.Splice("(= (. cmdmod cmod_tab) (+ (call tabpage_index curtab) 1))", "(goto wingotofile)",
			"(call beep_flush) (break)", "CTRL-W gf and gF: editing a file in a new tab page")
		// CTRL-W gf was the only goto to it; CTRL-W f falls into the code
		// directly.
		v.Cut("(label wingotofile)", 1, "the label only CTRL-W gf jumped to")
		v.Rewrite("(call goto_tabpage (cast int Prenum))", "(if (> Prenum 1) (block (call beep_flush)))", 1,
			"CTRL-W gt: the one-tab-page answer")
		v.Cut("(call goto_tabpage (- (cast int Prenum1)))", 1, "CTRL-W gT: the one-tab-page answer")
		v.FoldAlways("(== (call goto_tabpage_lastused) FAIL)", 1, "CTRL-W g<Tab>: beeps")
	})

	// With no row nothing sets tcl_flags, so it is 0 for ever: "use the
	// last-used tab page" is never asked for, and "go left" never is either.
	v.InFunction("alt_tabpage", func(v *graph.Verbs) {
		v.FoldNever("(&& (paren (& tcl_flags TCL_USELAST)) (call valid_tabpage lastused_tabpage))", 1,
			"alt_tabpage: 'tabclose' asking for the last-used tab page")
		v.Rewrite("(&& ?a (|| (== (& tcl_flags TCL_LEFT) 0) (== curtab first_tabpage)))", "?a", 1,
			"alt_tabpage: 'tabclose' asking to go left")
	})
	v.Cut("(cast void (call opt_strings_flags p_tcl p_tcl_values (addr tcl_flags) TRUE))", 1,
		"didset_string_options reading 'tabclose'")

	// cmod_tab is not counted here: this runs at phase 3 (the reform's D9),
	// where the retired commands' handlers and what the phases after it take
	// still name it.
	if v.Failed() {
		return v.Done()
	}
	v.Say("nothing makes or reaches a second tab page")
	return v.Done()
}
