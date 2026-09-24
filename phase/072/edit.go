package p072

// Whim phase 72 -- one window, one tabpage, structurally.  See GOAL.md.
//
// THE INVARIANT IS PROVABLE, not imposed -- the phase 68 shape rather than the
// phase 70 one.  Windows are created in exactly one place: win_alloc(NULL, FALSE)
// from win_alloc_firstwin(), whose only caller is win_alloc_first() at startup.
// alloc_tabpage() is called exactly once, from the same function, and curtab is set
// to it there.  :tabnew, :tabedit, :split, :new and the rest are already ex_ni, and
// aucmd_win went in phase 68.  So firstwin == lastwin == curwin and
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
// depth has nothing to do with it.  In phase 71 getout() failed to compile that way,
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

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/edit"
)

// THE ONE PATTERN BACKREFERENCE IN ANY EDIT PART, and RE2 has none.
//
// The Python matches a walk with `(?P<v>\w+)` and then requires the SAME name
// at every later position with `(?P=v)`, so a `for` whose variables disagree
// simply does not match.  Go cannot say that, so each position is captured
// SEPARATELY and the equality is asserted afterwards -- the route the Go session
// measured on q71, where NESTED, TABS and WINS give 15, 17 and 23 either way.
//
// The measurement that makes that mean something is the CONTROL, not the
// agreement: on the real tree the equality rejects NOTHING, because every raw
// match already has equal groups, so agreeing shows only that the rewrite loses
// no match.  A synthetic header with mismatched variables is what shows it
// discriminates -- backref finds 0, raw RE2 finds 1, capture-and-compare finds 0.
const (
	nestedWalk = `for \(\((\w+)\) = \(\((\w+)\) == (?:NULL \|\| \((\w+)\) == )?curtab\) *\? firstwin : \((\w+)\)->tp_firstwin; \((\w+)\); \((\w+)\) = \((\w+)\)->w_next\)`
	tabsWalk   = `for \(\((\w+)\) = first_tabpage; \((\w+)\) != NULL; \((\w+)\) = \((\w+)\)->tp_next\)`
	winsWalk   = `for \(\((\w+)\) = firstwin; \((\w+)\) != NULL; \((\w+)\) = \((\w+)\)->w_next\)`
)

// allEqual says whether every named index holds the same non-empty text.  An
// EMPTY capture is skipped rather than failed: NESTED's third group is inside
// `(?:...)?` and is absent when that alternative did not run.
func allEqual(g []string, idx ...int) bool {
	var want string
	for _, i := range idx {
		if i >= len(g) || g[i] == "" {
			continue
		}
		if want == "" {
			want = g[i]
		} else if g[i] != want {
			return false
		}
	}
	return want != ""
}

// Whim72 makes one window and one tabpage the layout: every walk over the window
// or tabpage list folds to curwin or curtab, and the list heads themselves go.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("onewin", text, w)

	// 1. the constant tests, before anything renames what they compare.
	// ONE_WINDOW, expanded at three sites and missed by phase 68, which only
	// folded the one_window()/last_window()/only_one_window() functions.
	e.LiteralN("(firstwin == lastwin)", "TRUE", 3, "ONE_WINDOW, expanded in place")
	e.LiteralN("wp == firstwin", "TRUE", 2, "win_update asking whether this is the top window")
	e.Literal("wp == lastwin", "wp == curwin", "win_redr_ruler asking for the bottom window")

	e.DropWalkIn("aucmd_prepbuf", `for \(\(win\) = firstwin; \(win\) != NULL; \(win\) = \(win\)->w_next\)`,
		w72lit3, "aucmd_prepbuf searching for the window showing a buffer", 1)
	e.DropWalkIn("can_unload_buffer", `for \(\(wp\) = firstwin; \(wp\) != NULL; \(wp\) = \(wp\)->w_next\)`,
		w72lit4, "can_unload_buffer asking whether the buffer is on screen", 1)
	e.Lines(`borrow_stl_vsep_hl\(\);`, 2, "the two calls to the separator-highlight pass")
	e.DeleteDefinition("borrow_stl_vsep_hl", "borrow_stl_vsep_hl, which had no window to borrow from")
	e.Body("current_win_nr", w72lit5, "current_win_nr, which counted to the window")
	e.Body("current_tab_nr", w72lit5, "current_tab_nr, which counted to the tabpage")
	e.ReplaceBlock("getout", `(?m)^[ \t]*for \(tp = first_tabpage; tp != NULL; tp = next_tp\)$`,
		w72lit6, "quitting walking every window of every tabpage")
	e.Body("create_windows", w72lit7, "the startup scan rewinding over the window list")
	e.Body("win_valid", w72lit8, "win_valid, which walked the list for the window it was given")
	e.Body("win_valid_any_tab", w72lit8, "win_valid_any_tab, which walked every tabpage for it")
	e.Body("win_find_by_id", w72lit9, "win_find_by_id, which walked the list by id")
	e.Body("valid_tabpage", w72lit10, "valid_tabpage, which walked the tabpage list")
	e.FoldNeverIn("goto_tabpage_tp", `(?m)^[ \t]*if \(tp != curtab && leave_tabpage\(`, "switching to another tabpage", 1)
	e.FoldNeverIn("close_buffer", `(?m)^[ \t]*if \(is_curwin && curwin != win && win_valid\)$`, "closing a buffer from another window", 1)
	e.FoldNeverIn("buf_freeall", `(?m)^[ \t]*if \(is_curwin && curwin != the_curwin && win_valid_any_tab\(the_curwin\)\)$`,
		"freeing a buffer from another window", 1)
	e.Body("win_alloc_firstwin", w72lit11, "win_alloc_firstwin cloning an existing window")
	e.Body("win_alloc_first", w72lit12, "the first tabpage being the head of a list")
	e.FoldNeverIn("win_alloc", `(?m)^[ \t]*if \(!hidden\)$`, "win_alloc appending to the window list", 1)
	e.Body("unuse_tabpage", w72lit13, "a tabpage remembering the ends of its window list")
	e.Body("win_rest_invalid", w72lit14, "win_rest_invalid invalidating every window after one")

	e.FoldWalks(nestedWalk,
		func(g []string) bool { return allEqual(g, 2, 6, 7, 8) && allEqual(g, 3, 4, 5) },
		func(g []string) string { return g[2] + " = curwin;" },
		"the window walk inside every tabpage walk")
	e.FoldWalks(tabsWalk,
		func(g []string) bool { return allEqual(g, 2, 3, 4, 5) },
		func(g []string) string { return g[2] + " = curtab;" },
		"every walk over the tabpage list")
	e.FoldWalks(winsWalk,
		func(g []string) bool { return allEqual(g, 2, 3, 4, 5) },
		func(g []string) string { return g[2] + " = curwin;" },
		"every walk over the window list")

	e.InFunction("win_ins_lines", func(e *edit.E) {
		e.Literal(w72lit16, w72lit17, "scrolling asking whether a window is below")
	})
	e.InFunction("win_ins_lines", func(e *edit.E) {
		e.Literal(w72lit18, "", "scrolling refusing when a window is below")
	})
	e.InFunction("win_ins_lines", func(e *edit.E) {
		e.Literal(w72lit19, w72lit20, "scrolling invalidating the window below")
	})
	e.InFunction("win_del_lines", func(e *edit.E) {
		e.Literal(w72lit21, w72lit22, "deleting lines asking whether a window is below")
	})
	e.InFunction("win_del_lines", func(e *edit.E) {
		e.Literal(w72lit23, w72lit24, "deleting lines invalidating the window below")
	})
	e.InFunction("win_do_lines", func(e *edit.E) {
		e.Literal(w72lit25, "", "'termfastscroll' refusing to scroll a window that has one below")
	})
	e.Lines(`win_T[ \t]+\*w_next;`, 1, "the window list pointer in win_T")

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
		f := f
		e.InFunction(f.fn, func(e *edit.E) {
			e.Lines(f.v+` = curtab;`, f.n, fmt.Sprintf("the tabpage %s no longer walks", f.fn))
		})
		e.InFunction(f.fn, func(e *edit.E) {
			e.Lines(`tabpage_T[ \t]+\*`+f.v+`;`, 1, "and the variable that held it")
		})
	}

	// 7. what is left of the two lists
	e.Lines(`static win_T[ \t]+\*firstwin;`, 1, "firstwin")
	e.Lines(`static win_T[ \t]+\*lastwin;`, 1, "lastwin")
	e.Lines(`static tabpage_T[ \t]+\*first_tabpage;`, 1, "first_tabpage")
	if !e.Failed() {
		n := e.Mentions("firstwin") + e.Mentions("lastwin")
		t := regexp.MustCompile(`\bfirstwin\b`).ReplaceAll(e.Text(), []byte("curwin"))
		t = regexp.MustCompile(`\blastwin\b`).ReplaceAll(t, []byte("curwin"))
		e.Set(t)
		e.Say(fmt.Sprintf("the list heads read as layout state (%d mentions -> curwin)", n))
	}
	return e.Done()
}

func init() { edit.Register("whim72", Edit) }
