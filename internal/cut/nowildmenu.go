package cut

import (
	"bytes"
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoWildMenu removes 'wildmenu' and the command-line popup it drew.
//
// Each edit must find exactly what it expects, with the report printed at
// the END rather than as it goes.  A miss is fatal rather than silent.  An
// edit that quietly matched nothing leaves the code it was meant to remove
// in place, and the report still says the phase succeeded -- which is how
// an inert option survives three passes.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's brace and
// parenthesis matching and its substitutions are acts on the nodes -- ifs
// dropped and folded, an else-if arm folded away, conditions rewritten from
// their own operands, statements cut -- and showmatches' two parameters are
// dropped with their arguments at every call (PARAM), after the ifs that
// read one of them; the locals initialised from the other dangle for the
// collection, as the text left them to the sweep.  The report is the
// text's, in its order (history keeps the text version).
func NoWildMenu(e *graph.Editor, w io.Writer) error {
	q := graph.NewVerbs("nowildmenu", e, io.Discard)
	type entry struct {
		what string
		n    int
	}
	var log []entry
	act := func(n int, what string, f func()) {
		if q.Failed() {
			return
		}
		f()
		log = append(log, entry{what, n})
	}
	in := func(fn string, f func(*graph.Verbs)) { q.InFunction(fn, f) }

	act(3, "the three `if (p_wmnu)` blocks", func() { q.DropIf("p_wmnu", 3, "the three `if (p_wmnu)` blocks") })
	act(1, "the cmdline_leave guard that only wildmenu needed", func() {
		in("getcmdline_int", func(q *graph.Verbs) {
			q.FoldAlways("(|| (! p_wmnu) _*)", 1, "the cmdline_leave guard that only wildmenu needed")
		})
	})
	// showmatches' second argument was "draw the wildmenu" and its fourth
	// the wildmode flags that only the menu read: the parameters go from
	// its definition, the arguments from its six calls,
	// once the ifs below have taken the second's last readers.  The text
	// counted the seven places one substitution changed.
	showmatches := len(log)
	log = append(log, entry{"showmatches loses its wildmenu and wim_flags arguments", 7})
	in("showmatches", func(q *graph.Verbs) {
		act(1, "the popup form of the wildmenu", func() {
			q.DropIf("(&& display_wildmenu (! display_list) _)", 1, "the popup form of the wildmenu")
		})
		act(1, "the menu drawn beside the list", func() {
			q.DropIf("(&& display_wildmenu display_list)", 1, "the menu drawn beside the list")
		})
		// The middle arm of a three-way chain: `if (got_int) ... else if
		// (menu) ... else if (list) ...`.  Dropping the arm drops its else.
		act(1, "the status-line menu arm of showmatches", func() {
			q.One("(if (&& display_wildmenu (! display_list)) (block (call win_redr_status_matches _*)) _)",
				"the status-line menu arm of showmatches")
			q.FoldNever("(&& display_wildmenu (! display_list))", 1, "the status-line menu arm of showmatches")
		})
	})
	if !q.Failed() {
		f := e.Defn("showmatches")
		var st graph.ParamStats
		var err error
		if f == nil {
			err = fmt.Errorf("showmatches is not defined")
		} else {
			st, err = e.DropParams([]graph.ParamDrop{{Decl: f, I: 1}, {Decl: f, I: 3}}, graph.ParamOptions{Dangle: true})
		}
		if err == nil && (st.Calls != 6 || st.Params != 2) {
			err = fmt.Errorf("%s, expected 2 parameters of its definition and 6 calls", st)
		}
		if err != nil {
			return fmt.Errorf("nowildmenu: %s -- %v", log[showmatches].what, err)
		}
	}
	in("cmdline_wildchar_complete", func(q *graph.Verbs) {
		act(1, "the WILD_NOSELECT condition", func() {
			q.Rewrite("(|| wim_noselect (paren ?x))", "?x", 1, "the WILD_NOSELECT condition")
		})
		act(1, "the WILD_NOINSERT block", func() { q.DropIf("wim_noinsert", 1, "the WILD_NOINSERT block") })
		act(1, "the match-count threshold", func() {
			q.Rewrite("(? (paren (|| wim_noselect wim_noinsert)) 0 1)", "1", 1, "the match-count threshold")
		})
		act(2, "the two `wim_list || show_menu` conditions", func() {
			q.Rewrite("(|| wim_list show_menu)", "wim_list", 2, "the two `wim_list || show_menu` conditions")
		})
		act(1, "the next-wildmode condition", func() {
			q.Rewrite("(|| wim_list_next (paren (&& p_wmnu _)))", "wim_list_next", 1, "the next-wildmode condition")
		})
	})
	act(1, "the CTRL-L listing condition", func() {
		in("getcmdline_int", func(q *graph.Verbs) {
			q.Rewrite("(&& (> (. xpc xp_numfiles) 1) (|| (paren (&& ?b ?c)) p_wmnu))",
				"(&& (> (. xpc xp_numfiles) 1) ?b ?c)", 1, "the CTRL-L listing condition")
		})
	})
	act(4, "the wildmenu_cleanup calls", func() { q.Cut("(call wildmenu_cleanup _)", 4, "the wildmenu_cleanup calls") })
	act(1, "the CTRL-E/CTRL-Y guard", func() {
		in("getcmdline_int", func(q *graph.Verbs) {
			q.Rewrite("(|| (call cmdline_pum_active) wild_menu_showing did_wild_list)", "(paren did_wild_list)", 1,
				"the CTRL-E/CTRL-Y guard")
		})
	})
	act(1, "the redraw guard that asked whether the menu was up", func() {
		in("redraw_after_callback", func(q *graph.Verbs) {
			q.DropOperand("(== wild_menu_showing 0)", 1, "the redraw guard that asked whether the menu was up")
		})
	})
	act(1, "the command-line popup redraw", func() {
		q.Cut("(if (call pum_visible) (block (call cmdline_pum_display)))", 1, "the command-line popup redraw")
	})
	in("getcmdline_int", func(q *graph.Verbs) {
		act(2, "the two popup removals in getcmdline_int", func() {
			q.Cut("(if (call cmdline_pum_active) (block (call cmdline_pum_remove (addr ccline) FALSE)))", 2,
				"the two popup removals in getcmdline_int")
		})
	})
	in("ExpandOne", func(q *graph.Verbs) {
		act(1, "the popup removal in ExpandOne", func() {
			q.Cut("(if (!= cmdline_match_array nullptr) (block (call cmdline_pum_remove (call get_cmdline_info) FALSE)))", 1,
				"the popup removal in ExpandOne")
		})
	})
	in("getcmdline_int", func(q *graph.Verbs) {
		act(1, "the CTRL-A popup cleanup", func() {
			q.Cut("(if (call cmdline_pum_active) (block (call cmdline_pum_cleanup (addr ccline))))", 1, "the CTRL-A popup cleanup")
		})
		act(1, "the popup exception to leaving completion", func() {
			q.Cut("(= end_wildmenu (&& end_wildmenu (|| (! (call cmdline_pum_active)) _)))", 1,
				"the popup exception to leaving completion")
		})
		act(1, "the popup teardown on any other key", func() {
			q.DropIf("(if (call cmdline_pum_active) (block (= skip_pum_redraw _) _*))", 1, "the popup teardown on any other key")
		})
		act(1, "the one place that set it", func() {
			q.DropIf("(&& (== c (paren (- (+ (paren KS_EXTRA) (<< (cast int (paren KE_WILD)) 8))))) (!= firstc '@'))", 1,
				"the one place that set it")
		})
		// if (dead) { A } else { B } is B: the else must be there to keep
		act(1, "the popup page-up/page-down arm", func() {
			q.One("(if (&& (call cmdline_pum_active) _) _ _)", "the popup page-up/page-down arm -- no else branch, so "+
				"there is nothing to keep and deleting the if alone would delete the fallback")
			q.FoldNever("(&& (call cmdline_pum_active) _)", 1, "the popup page-up/page-down arm")
		})
	})
	// 'wildoptions', whose `pum` value selected the menu, is dropped at
	// phase 1 with every option the product has not (optfront, D3).
	if err := q.Done(); err != nil {
		return err
	}

	for _, l := range log {
		fmt.Fprintf(w, "  nowildmenu   %-3d %s\n", l.n, l.what)
	}
	text := q.Text()
	left := 0
	for _, n := range []string{"p_wmnu", "wild_menu_showing", "cmdline_pum_active"} {
		left += bytes.Count(text, []byte(n))
	}
	fmt.Fprintf(w, "  nowildmenu   %d mentions left, all of them definitions for the sweep\n",
		left)
	return nil
}
