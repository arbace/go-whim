package p005b

// Whim phase 5b (formerly 72) -- one window, one tabpage, structurally.  See GOAL.md.
//
// THE INVARIANT IS PROVABLE, not imposed -- the phase 4c shape rather than the
// phase 22 one.  Windows are created in exactly one place: win_alloc(NULL, FALSE)
// from win_alloc_firstwin(), whose only caller is win_alloc_first() at startup.
// alloc_tabpage() is called exactly once, from the same function, and curtab is set
// to it there.  :tabnew, :tabedit, :split, :new and the rest are already ex_ni, and
// aucmd_win went in phase 4c.  So firstwin == lastwin == curwin and
// first_tabpage == curtab, always, and this phase deletes the traversal that
// pretended otherwise.
//
// TWO LAYERS, FOLDED AS A PAIR.  Nearly every tabpage walk immediately contains
//
// for ((wp) = ((tp) == curtab) ? firstwin : (tp)->tp_firstwin; (wp); ...)
//
// so folding the outer walk to `tp = curtab` makes that ternary constant-fold to
// firstwin, and folding the inner one then gives curwin.  Cutting one layer without
// the other would leave half a traversal at every one of these sites.
//
// SEVEN BODIES ARE REWRITTEN, NOT FOLDED, and they were found BEFORE writing a line
// of this file rather than after three dry runs.  Folding a walk deletes its `for`
// header, and C binds break/continue to the nearest enclosing loop or switch -- brace
// depth has nothing to do with it.  In phase 4d getout() failed to compile that way,
// which is the cheap outcome, and buflist_findpat() COMPILED FINE AND CHANGED
// BEHAVIOUR.  So fold_walks() below audits every body it is about to fold and
// REFUSES if any break/continue would rebind:
//
// aucmd_prepbuf        break   -- the walk searched for the window showing a buffer
// can_unload_buffer    break   -- same search, for "is it on screen"
// borrow_stl_vsep_hl   both    -- two walks; the whole function collapses
// current_win_nr       break   -- counts to the window, so: 1
// current_tab_nr       break   -- counts to the tabpage, so: 1
// getout               both    -- a tabpage walk that re-seeds next_tp and breaks
// create_windows       break   -- a rewind loop over w_next, twice
//
// THE TABPAGE-SWITCHING GROUP IS DEAD BEHIND ONE GATE.  goto_tabpage_tp()'s whole
// body is `if (tp != curtab && leave_tabpage(...) == OK)`, which with one tabpage is
// never true.  Folding that gate away orphans leave_tabpage, enter_tabpage,
// valid_tabpage and use_tabpage, and the sweep then removes them -- taking the last
// readers of tp_firstwin, tp_lastwin and tp_prevwin with them.  Nothing here deletes
// those functions by name; removing the one gate is what kills them.
//
// WHAT STAYS, deliberately:
// * the FRAME layer -- topframe, frame_T, fr_next, fr_child, fr_parent.  One window
// still has one frame, and new_frame()/topframe are load-bearing for sizing.
// Cutting frames is its own phase.
// * b_nwindows.  Tracing every write: = 1 at window creation, balanced ++/-- pairs
// in enter_buffer and aucmd_restbuf, and -- in close_buffer when the window drops
// the buffer.  It is genuinely 0 after that, so `<= 0` and `== 0` are live
// "no longer displayed" tests.  An earlier plan folded all 20 sites to a constant
// 1; that would have broken buffer release silently.
// * prevwin and w_id.  aucmd_prepbuf/aucmd_restbuf save and restore the window by
// id, and the incsearch state compares curwin->w_id, so neither is list state.
// * the `curwin == NULL` guards that were `firstwin == NULL` in shell_new_rows,
// shell_new_columns, min_rows and min_rows_for_all_tabpages.  They exist because
// a resize can arrive before win_alloc_first(), and proving that it cannot is not
// this phase's job.
//
// THE DELTA: none expected.  Every window and tabpage Ex command is already ex_ni, so
// no exsweep row can move; declared empty and left for the delta check to correct.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's literals,
// walks and bodies are acts on the nodes -- the walks folded by their
// shape, the variables the text's backreferences asked to agree bound once
// in the pattern (a second ?name must be the same form), the one-statement
// bodies built from templates, the long ones spliced as C in three units
// (FRAG, Together), the list heads' last uses pointed at curwin by edge
// (RetargetUses) -- each counted, its report the text's (history keeps the
// text version).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// The walks, as forms: a variable bound once and named again at every
// place the text's backreference asked for it.
const (
	nestedWalk = "(for (= (paren ?w) (? (paren ?c) firstwin (-> (paren ?t) tp_firstwin))) (paren ?w) (= (paren ?w) (-> (paren ?w) w_next)) _)"
	tabsWalk   = "(for (= (paren ?v) first_tabpage) (!= (paren ?v) nullptr) (= (paren ?v) (-> (paren ?v) tp_next)) _)"
	winsWalk   = "(for (= (paren ?v) firstwin) (!= (paren ?v) nullptr) (= (paren ?v) (-> (paren ?v) w_next)) _)"
)

// nestedTab is the tabpage test of a nested walk: `(tp) == curtab`, or
// `(tp) == nullptr || (tp) == curtab`, of the walk's own tabpage.
var nestedTab = []*clisp.Node{
	clisp.MustPattern("(== (paren ?t) curtab)"),
	clisp.MustPattern("(|| (== (paren ?t) nullptr) (== (paren ?t) curtab))"),
}

func named(n *graph.Node) bool { return n != nil && !n.IsList() && n.Atom != "" }

// Whim5b makes one window and one tabpage the layout: every walk over the window
// or tabpage list folds to curwin or curtab, and the list heads themselves go.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("onewin", e, w)

	// 1. the constant tests, before anything renames what they compare.
	// ONE_WINDOW, expanded at three sites and missed by phase 4c, which only
	// folded the one_window()/last_window()/only_one_window() functions.  The
	// text's literal took the parentheses: the expansion's own where they are
	// a node, the C view's under a `!`.
	if ms := v.Find("(== firstwin lastwin)"); len(ms) != 3 {
		v.Die("ONE_WINDOW, expanded in place -- occurs %d times, expected 3", len(ms))
	} else {
		for _, m := range ms {
			at := m
			if p := e.Parent(m); p != nil && p.Is("paren") {
				at = p
			}
			// TRUE by BUILD: the use, its edge to the enumerator
			ns, err := e.Build(at, "TRUE", nil)
			if err == nil {
				err = e.Replace(at, ns...)
			}
			if err != nil {
				v.Die("ONE_WINDOW, expanded in place -- %v", err)
				break
			}
		}
		if !v.Failed() {
			v.Say("ONE_WINDOW, expanded in place")
		}
	}
	v.Rewrite("(== wp firstwin)", "TRUE", 2, "win_update asking whether this is the top window")
	v.Rewrite("(== wp lastwin)", "(== wp curwin)", 1, "win_redr_ruler asking for the bottom window")

	v.InFunction("aucmd_prepbuf", func(v *graph.Verbs) {
		v.Rewrite(winsWalk, "(= win (? (paren (== (-> curwin w_buffer) buf)) curwin nullptr))", 1,
			"aucmd_prepbuf searching for the window showing a buffer")
	})
	v.InFunction("can_unload_buffer", func(v *graph.Verbs) {
		v.Rewrite(winsWalk, "(if (== (-> curwin w_buffer) buf) (block (= can_unload FALSE)))", 1,
			"can_unload_buffer asking whether the buffer is on screen")
	})
	v.Cut("(call borrow_stl_vsep_hl)", 2, "the two calls to the separator-highlight pass")
	v.DeleteDefinition("borrow_stl_vsep_hl", "borrow_stl_vsep_hl, which had no window to borrow from")
	v.Body("current_win_nr", "(return 1)", "current_win_nr, which counted to the window")
	v.Body("current_tab_nr", "(return 1)", "current_tab_nr, which counted to the tabpage")
	v.Together(func(v *graph.Verbs) {
		v.InFunction("getout", func(v *graph.Verbs) {
			v.ReplaceC("(for (= tp first_tabpage) (!= tp nullptr) (= tp next_tp) _)", w5blit6, 1,
				"quitting walking every window of every tabpage")
		})
		v.BodyC("create_windows", w5blit7, "the startup scan rewinding over the window list")
		v.BodyC("win_valid", w5blit8, "win_valid, which walked the list for the window it was given")
		v.BodyC("win_valid_any_tab", w5blit8, "win_valid_any_tab, which walked every tabpage for it")
		v.BodyC("win_find_by_id", w5blit9, "win_find_by_id, which walked the list by id")
		v.BodyC("valid_tabpage", w5blit10, "valid_tabpage, which walked the tabpage list")
	})
	v.InFunction("goto_tabpage_tp", func(v *graph.Verbs) {
		v.FoldNever("(&& (!= tp curtab) (== (call leave_tabpage _*) OK))", 1, "switching to another tabpage")
	})
	v.InFunction("close_buffer", func(v *graph.Verbs) {
		v.FoldNever("(&& is_curwin (!= curwin win) win_valid)", 1, "closing a buffer from another window")
	})
	v.InFunction("buf_freeall", func(v *graph.Verbs) {
		v.FoldNever("(&& is_curwin (!= curwin the_curwin) (call win_valid_any_tab the_curwin))", 1, "freeing a buffer from another window")
	})
	v.Together(func(v *graph.Verbs) {
		v.BodyC("win_alloc_firstwin", w5blit11, "win_alloc_firstwin cloning an existing window")
		v.BodyC("win_alloc_first", w5blit12, "the first tabpage being the head of a list")
	})
	v.InFunction("win_alloc", func(v *graph.Verbs) {
		v.FoldNever("(! hidden)", 1, "win_alloc appending to the window list")
	})
	v.Together(func(v *graph.Verbs) {
		v.BodyC("unuse_tabpage", w5blit13, "a tabpage remembering the ends of its window list")
		v.BodyC("win_rest_invalid", w5blit14, "win_rest_invalid invalidating every window after one")
	})

	v.FoldWalks(nestedWalk,
		func(b graph.Bindings) bool {
			if !named(b["w"]) || !named(b["t"]) {
				return false
			}
			for _, p := range nestedTab {
				if m, ok := graph.Match(p, b["c"]); ok && graph.SameForm(m["t"], b["t"]) {
					return true
				}
			}
			return false
		},
		func(b graph.Bindings) string { return "(= " + b["w"].Atom + " curwin)" },
		"the window walk inside every tabpage walk")
	v.FoldWalks(tabsWalk, func(b graph.Bindings) bool { return named(b["v"]) },
		func(b graph.Bindings) string { return "(= " + b["v"].Atom + " curtab)" },
		"every walk over the tabpage list")
	v.FoldWalks(winsWalk, func(b graph.Bindings) bool { return named(b["v"]) },
		func(b graph.Bindings) string { return "(= " + b["v"].Atom + " curwin)" },
		"every walk over the window list")

	v.InFunction("win_ins_lines", func(v *graph.Verbs) {
		v.DropOperand("(!= (-> wp w_next) nullptr)", 1, "scrolling asking whether a window is below")
		v.FoldNever("(-> wp w_next)", 1, "scrolling refusing when a window is below")
		v.Cut("(call win_rest_invalid (paren (-> (paren wp) w_next)))", 1, "scrolling invalidating the window below")
	})
	v.InFunction("win_del_lines", func(v *graph.Verbs) {
		v.DropOperand("(-> wp w_next)", 1, "deleting lines asking whether a window is below")
		v.Cut("(call win_rest_invalid (-> wp w_next))", 1, "deleting lines invalidating the window below")
	})
	v.InFunction("win_do_lines", func(v *graph.Verbs) {
		v.Cut("(if (&& (!= (-> wp w_next) nullptr) p_tf) (block (return FAIL)))", 1,
			"'termfastscroll' refusing to scroll a window that has one below")
	})

	for _, f := range []struct {
		fn, v string
		n     int
	}{
		{"setfname", "tab", 1}, {"changed_common", "tp", 3},
		{"mark_adjust_internal", "tab", 1}, {"set_options_default", "tp", 1},
		{"did_set_global_listfillchars", "tp", 1},
		{"check_chars_options", "tp", 1}, {"check_lnums_both", "tp", 1},
		{"screenalloc", "tp", 2},
	} {
		v.InFunction(f.fn, func(v *graph.Verbs) {
			v.Cut("(= "+f.v+" curtab)", f.n, fmt.Sprintf("the tabpage %s no longer walks", f.fn))
		})
	}
	// The variables those writes held, win_T's w_next and first_tabpage are
	// named by nothing now; the collection takes them.

	// 7. what is left of the two lists: the heads' declarations go, and
	// every use left of either is curwin's, by edge
	if v.Failed() {
		return v.Done()
	}
	cw := e.FileDecls("curwin")
	if len(cw) == 0 {
		return fmt.Errorf("onewin: curwin is not declared")
	}
	uses := 0
	for _, h := range []string{"firstwin", "lastwin"} {
		ds := e.FileDecls(h)
		if len(ds) != 1 {
			return fmt.Errorf("onewin: %s -- %d declarations, expected 1", h, len(ds))
		}
		moved, err := e.RetargetUses(ds[0], cw[0])
		if err != nil {
			return fmt.Errorf("onewin: %s -- %v", h, err)
		}
		uses += len(moved)
	}
	for _, h := range []string{"firstwin", "lastwin"} {
		v.Cut("(def static "+h+" (ptr win_T))", 1, h)
	}
	v.Expect(uses == 35, "the list heads read as layout state -- %d uses, expected 35", uses)
	if !v.Failed() {
		v.Sayf("the list heads read as layout state (%d mentions -> curwin)", uses)
	}
	return v.Done()
}

func init() { phase.RegisterGraph("whim5b", Edit) }
