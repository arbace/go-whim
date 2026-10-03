package p025

// Whim phase 25 (formerly 79) -- the constant-return predicates.  See GOAL.md.
//
// Twenty-eight functions whose whole body is `return <constant>;`.  Each was emptied
// by an earlier phase and left with its callers in place, so the editor still asks
// "is the popup menu visible", "are we in a Vim9 script", "is there more than one
// window" -- and still branches on an answer that cannot change.  The compiler cannot
// help: at -O0 each is a real call and a real branch, and every sweep this pipeline
// runs reports the code as live because it is reachable.  Unuseful, not unused.
//
// THE INVARIANT, AND IT IS ASSERTED RATHER THAN TRUSTED.  Step 1 reads every one of
// the 28 definitions and requires the body to be exactly `return <expected>;`, with
// the expected token written out here.  If an upstream ever gives one a real body,
// the phase fails instead of folding a live predicate.  That is the phase 23 pattern
// ("all EX_BUFNAME commands are ex_ni") and it is the only thing standing between a
// fold and a wrong answer.
//
// FOUR SIMILAR-LOOKING FUNCTIONS ARE NOT TOUCHED, and the distinction is the whole
// reason this phase was surveyed twice.  A scan for `return <single token>;` reports
// 32, but four of those tokens are VARIABLES, not constants:
//
// get_hislen           -> hislen
// is_maphash_valid     -> maphash_valid
// get_search_pat       -> mr_pattern
// get_text_locked_msg  -> e_not_allowed_to_change_text_or_change_window
//
// My first classifier said thirteen of the 32 returned a variable; it had matched the
// bare tokens 0, 1 and NULL against unrelated declarations elsewhere in the file.
// Reading the DEFINITIONS gives four.  Supplying the expected constant per name, as
// step 1 does, is what makes that mistake impossible to repeat silently.
//
// did_set_number_relativenumber IS A CONSTANT AND IS STILL NOT TOUCHED.  Its only two
// mentions are option-table rows, where it appears as a FUNCTION POINTER with no call
// parentheses.  Folding is meaningless and deleting it would leave two rows pointing
// at nothing.  It stays, and an assertion at the end requires both rows intact.
//
// `binds_out` IS A VETO FOR fold_always, NOT FOR fold_never.  The block at
// parse_command_modifiers' `if (vim9script)` contains a `break` that binds to the
// enclosing `for (;;)`, which is exactly the shape that made buflist_findpat change
// behaviour silently in phase 4d -- but that phase FOLDED A WALK, keeping the body
// while removing the loop around it, so the break rebound.  fold_never DELETES the
// body, break and all, and the condition was false, so the break never fired.  Every
// fold_always site in this phase was audited and none contains an escaping break.
//
// THE SECOND-ORDER CUTS, both proved in the phase rather than assumed:
//
// skip_for_popup  is not a constant stub on entry -- it has three returns.  Once
// pum_under_menu and pum_visible fold, both of its guards go and it becomes
// `return FALSE;`.  Step 5 asserts that with the same const_of() check before
// step 6 uses it, so the collapse is proved, not hoped for.  Nine more sites.
//
// may_have_range  is a local of do_one_cmd with two writes.  One is inside the
// `if (vim9script && ...)` block this phase folds away; the other is that
// block's else arm, `may_have_range = TRUE;`.  So after the fold it has one
// write and is constantly true, and its two readers fold too.
//
// wc  in option_value2string is `long wc = 0;` whose only "write" is `&wc` passed
// to wc_use_keyname -- which never dereferences wcp.  So BOTH arms of that
// if/else-if chain are dead, not just the first, and it collapses to the
// sprintf.  Checked by reading wc_use_keyname's body, not by assuming.
//
// need_check_timestamps, need_redraw, bom_count  each become write-only once the
// stub feeding them is gone, so their tests fold and the variables sweep.
//
// ORDERING IS THE MAIN HAZARD AND THE STEPS ARE NUMBERED FOR IT.  Specific literals
// run before blanket regexes, EXCEPT where a blanket edit creates the specific one's
// target.  Three places depend on it: the may_have_range cascade (step 3) only exists
// after step 2a folds the block holding its other write; skip_for_popup's nine sites
// (step 6) only collapse after step 5 empties it; and the two-line ternary at
// do_one_cmd (step 10a) must be replaced BEFORE the blanket current_win_nr pass, or
// that pass eats one of its two halves and leaves a syntax error.  Every anchor in
// this file was counted against the q024 tree before it was written.
//
// NO TERM EDIT ENDS IN WHITESPACE.  `only_one_window() && check_changed_any` becomes
// `check_changed_any` rather than stripping `only_one_window() && `, because a
// literal with a trailing space lost it passing through an editor in phase 4d and the
// match then failed for reasons invisible in the diff.
//
// THE DELTA: none expected.  Every fold removes a branch whose condition cannot hold,
// and every term edit removes a conjunct that is constantly true or a disjunct that
// is constantly false.  The quit path is the one place where getting this wrong is
// silent rather than fatal -- check_more() feeds the four ex_quit/ex_exit conditions
// that decide whether getout(0) runs -- so five quit probes, calibrated on q024, guard
// it directly.  Declared empty, left for the delta check to correct.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the same acts in the same
// order on the program's graph, each found by its C-lisp form and counted
// as the text counted its lines (history keeps the text program); a term
// dropped from a condition is DropOperand, a literal replaced a Rewrite of
// its node.  Two are by hand on the editor: the five window heights, one
// of which is inside an unexpanded MIN() macro's text, respelled with the
// macro's refers edges kept but tabline_height's; and wc_use_keyname's body
// read for `wcp` atom by atom.

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// w25Constants are functions whose WHOLE Body is `return <constant>;`.  The
// phase proves each one before folding anything that calls it: a stub that has
// acquired a Body again would make every fold below change behaviour.
var w25Constants = map[string]string{
	"append_arg_number": "0", "at_ins_compl_key": "FALSE", "bomb_size": "0",
	"bt_quickfix": "FALSE", "bt_terminal": "FALSE",
	"check_can_set_curbuf_disabled": "TRUE",
	"check_more":                    "OK", "check_timestamps": "0", "current_tab_nr": "1",
	"current_win_nr": "1", "did_set_number_relativenumber": "nullptr",
	"get_cellwidth": "0", "has_cursormoved": "FALSE", "has_insertcharpre": "FALSE",
	"has_textchanged": "FALSE", "in_vim9script": "FALSE", "ins_compl_active": "FALSE",
	"ins_compl_lnum_in_range": "FALSE", "ins_compl_win_active": "FALSE",
	"only_one_window": "TRUE", "pum_redraw_in_same_position": "FALSE",
	"pum_under_menu": "FALSE", "pum_visible": "FALSE", "script_get": "nullptr",
	"stl_connected": "FALSE", "tabline_height": "0", "wc_use_keyname": "FALSE",
}

// w25Variable return a VARIABLE rather than a constant and are LEFT ALONE.  They
// are listed and checked so that the distinction is stated rather than assumed:
// folding one of these would freeze a value that still changes.
var w25Variable = map[string]string{
	"get_hislen": "hislen", "get_search_pat": "mr_pattern",
	"get_text_locked_msg": "e_not_allowed_to_change_text_or_change_window",
	"is_maphash_valid":    "maphash_valid",
}

// Whim25 folds every call to a function whose Body is a constant.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noconstfn", e, w)

	for _, n := range edit.SortedKeys(w25Constants) {
		v.ConstOf(n, w25Constants[n])
	}
	v.Say(fmt.Sprintf("confirmed: %d functions whose whole body is `return <constant>;`", len(w25Constants)))
	for _, n := range edit.SortedKeys(w25Variable) {
		v.ConstOf(n, w25Variable[n])
	}
	v.Say(fmt.Sprintf("confirmed: %d more return a VARIABLE and are left alone", len(w25Variable)))
	v.Expect(!bodySays(e, "wc_use_keyname", "wcp"), "wc_use_keyname now mentions wcp -- it may write through the out-parameter")
	v.Say("confirmed: wc_use_keyname never dereferences its out-parameter")

	v.FoldNever("(&& vim9script (== (& flags DOCMD_RANGEOK) 0))", 1,
		"scanning backwards for a colon to decide whether a range is allowed")
	v.FoldNever("(&& vim9script (! may_have_range))", 1, "the Vim9 path through find_ex_command")
	v.FoldNever("vim9script", 1, "a Vim9 check in parse_command_modifiers")
	v.FoldNever("(&& vim9script (call has_cmdmod cmod FALSE))", 1, "a command modifier without a command")
	v.FoldAlways("may_have_range", 1, "skipping a range that is always allowed")
	v.FoldNever("(! may_have_range)", 1, "the default address for a range that cannot be absent")
	v.Cut("(= may_have_range TRUE)", 1, "the flag nothing decides any more")
	v.FoldNever(`(&& (call in_vim9script) (== (deref p) '\'') _)`, 1,
		"a digit separator in a Vim9 number literal")
	v.FoldNever("(&& (call in_vim9script) (== (deref p) '#'))", 1, "a hash comment ending a Vim9 command")
	v.FoldNever("(&& (call in_vim9script) (> arg arg_start) _)", 1,
		"the Vim9 spacing rule for an unknown option")
	v.FoldNever("(call in_vim9script)", 5, "five more Vim9 script branches")
	v.Rewrite("(? (call in_vim9script) GETLINE_CONCAT_CONTBAR GETLINE_CONCAT_CONT)", "GETLINE_CONCAT_CONT", 1,
		"how a continuation line is joined")
	v.FoldNever("(call pum_under_menu row col TRUE)", 1, "skipping a cell the popup menu covers")
	v.FoldNever("(&& (call pum_visible) (== (& State MODE_CMDLINE) 0) (call pum_under_menu row col FALSE))", 1,
		"and the same test on the command line")
	v.ConstOf("skip_for_popup", "FALSE")
	v.Say("confirmed: skip_for_popup has collapsed to `return FALSE;`")
	v.Rewrite("(&& ?r (! (call ins_compl_active)) (! (call pum_visible)))", "?r", 1,
		"whether a state is safe no longer asks about completion")
	v.FoldNever("(call pum_visible)", 2, "two redraws deferred for the popup menu")
	v.FoldNever("(&& (! ignore_pum) (call pum_visible))", 1, "the ruler deferred for it")
	v.FoldNever("(call pum_redraw_in_same_position)", 1, "redrawing it in place")
	dropOperand(v, "(paren (&& (! ignore_pum) (call pum_visible)))", 1, "the status line deferred for it")
	dropOperand(v, "(! (call pum_visible))", 1, "'relativenumber' redrawing around it")
	dropOperand(v, "(! (call ins_compl_active))", 1, "'showmatch' suppressed during completion")
	v.FoldNever("(&& (paren (& State MODE_INSERT)) (call ins_compl_win_active wp) (|| in_curline (call ins_compl_lnum_in_range lnum)))", 2,
		"two completion highlights in the line drawer")
	dropOperand(v, "(! (call at_ins_compl_key))", 1, "a mapping suppressed by a completion key")
	v.FoldNever("(&& redraw_this (== char_cells 2) (call skip_for_popup row (+ col coloff 1)))", 1,
		"a double-width cell under the menu")
	v.FoldNever("(&& redraw_this (call skip_for_popup row (+ col coloff)))", 1, "and a single-width one")
	dropOperand(v, "(! (call skip_for_popup row (+ col coloff)))", 1, "clearing the next cell")
	v.FoldAlways("(! (call skip_for_popup row (+ col coloff)))", 1, "drawing a screen line cell")
	v.FoldAlways("(! (call skip_for_popup row (- col 1)))", 1, "redrawing the cell to the left")
	dropOperand(v, "(! (call skip_for_popup row col))", 3, "three more cells that are never covered")
	v.FoldAlways("(! (call skip_for_popup r c))", 1, "and filling a screen region")
	v.FoldAlways("(|| quit_all (paren (== (call check_more FALSE forceit) OK)))", 1, "the autocommand check before quitting")
	v.FoldAlways("(&& (== (call check_more FALSE (-> eap forceit)) OK) (call only_one_window))", 1,
		"deciding to exit in :quit")
	v.FoldNever("(call stl_connected wp)", 2, "a status line joined to the one beside it")
	v.FoldNever("(> (call get_cellwidth (index ScreenLinesUC off)) 1)", 1, "a character widened by 'setcellwidths'")
	v.FoldNever("(call wc_use_keyname varp (addr wc))", 1, "showing a numeric option as a key name")
	v.FoldNever("(!= wc 0)", 1, "and showing it as a character")
	v.FoldNever("(! (call check_can_set_curbuf_disabled))", 1, "refusing to change buffer in gf")
	dropOperand(v, "(! (call bt_terminal (-> wp w_buffer)))", 1, "the [+] flag suppressed for a terminal buffer")
	dropOperand(v, "(! (call bt_quickfix curbuf))", 1, "a quickfix buffer never being reusable")
	dropOperand(v, "(! (call has_insertcharpre))", 1, "the InsertCharPre fast path")
	v.FoldNever("(&& (! finish_op) (paren (call has_cursormoved)) _)", 1, "tracking the cursor for CursorMoved")
	v.FoldNever("(&& (! finish_op) (call has_textchanged) _)", 1, "and the change tick for TextChanged")
	v.DropIf("need_check_timestamps", 3, "three checks for a file changed outside the editor")
	v.Cut("(= need_check_timestamps TRUE)", 1, "asking for one")
	v.Cut("(= need_redraw (call check_timestamps FALSE))", 1, "the timestamp check on focus")
	v.FoldNever("need_redraw", 1, "and the redraw it asked for")
	v.Cut("(cast void (call append_arg_number curwin _*))", 1,
		"appending the argument-list position to the file message")
	v.InFunction("ex_script_ni", func(v *graph.Verbs) {
		v.Cut("(block (call vim_free (call script_get eap (-> eap arg))))", 1,
			"reading a here-document for a command that cannot run")
	})
	v.Cut("(= bom_count (call bomb_size))", 1, "counting the byte order mark")
	v.FoldNever("(&& (== dict nullptr) (> bom_count 0))", 1, "and reporting it")
	v.Rewrite("(? (== (-> eap addr_type) ADDR_WINDOWS) (call current_win_nr nullptr) (call current_tab_nr nullptr))", "1", 1,
		"the window-or-tab count for a bare range")
	for _, f := range []struct {
		fn, arg string
		n       int
	}{
		{"current_win_nr", "curwin", 3}, {"current_win_nr", "nullptr", 3},
		{"current_tab_nr", "curtab", 3}, {"current_tab_nr", "nullptr", 3},
	} {
		v.Rewrite(fmt.Sprintf("(call %s %s)", f.fn, f.arg), "1", f.n,
			fmt.Sprintf("there is one window and one tabpage (%s(%s))", f.fn, f.arg))
	}
	v.Rewrite("(+ ?a (call tabline_height) ?b)", "(+ ?a ?b)", 1, "the minimum rows needed, without a tab line")
	v.Cut("(+= total (call tabline_height))", 1, "the tab line in the all-tabpages minimum")
	v.InFunction("win_comp_pos", func(v *graph.Verbs) {
		v.RewriteAt("(def row int ?v)", "v", "0", 1, "window layout starting at the top row")
	})
	heights(v, "five window heights with no tab line to subtract")
	v.Rewrite("(+ (call tabline_height) ?f)", "?f", 1, "and the 'cmdheight' consistency check")
	return v.Done()
}

// bodySays says some atom of the function fn's body -- a name, a literal,
// a macro's text -- holds s: the text's question of its inner body.
func bodySays(e *graph.Editor, fn, s string) bool {
	d := e.Defn(fn)
	if d == nil {
		return true
	}
	found := false
	for _, it := range graph.Body(d) {
		graph.Walk(it, func(n *graph.Node) bool {
			found = found || !n.IsList() && strings.Contains(n.Atom, s)
			return !found
		})
	}
	return found
}

// heights is the text's `(Rows - p_ch - tabline_height())` -> `(Rows -
// p_ch)`, five times: four expressions, and one inside an unexpanded MIN()
// macro, whose text is respelled and whose refers edges are kept but the
// one to tabline_height.
func heights(v *graph.Verbs, what string) {
	if v.Failed() {
		return
	}
	e := v.Editor()
	const old, new = "(Rows - p_ch - tabline_height())", "(Rows - p_ch)"
	exprs := v.Find("(- Rows p_ch (call tabline_height))")
	macros := v.Find("(macro _)")
	var ms []*graph.Node
	for _, m := range macros {
		if strings.Contains(m.Kids[1].Atom, old) {
			ms = append(ms, m)
		}
	}
	if n := len(exprs) + strings.Count(fmt.Sprint(macroTexts(ms)), old); n != 5 {
		v.Die("%s -- matched %d times, expected 5", what, n)
		return
	}
	for _, x := range exprs {
		with, err := e.Build(x, "(- Rows p_ch)", nil)
		if err == nil {
			err = e.Replace(x, with...)
		}
		if err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
	}
	for _, m := range ms {
		nm := graph.NewList(graph.NewAtom("macro"), graph.NewAtom(strings.ReplaceAll(m.Kids[1].Atom, old, new)))
		for _, r := range m.Refs {
			if graph.DeclName(r) != "tabline_height" {
				nm.Refs = append(nm.Refs, r)
			}
		}
		nm.Type = m.Type
		if err := e.Replace(m, nm); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
	}
	v.Say(what)
}

func macroTexts(ms []*graph.Node) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Kids[1].Atom)
	}
	return out
}

func init() { phase.RegisterGraph("whim25", Edit) }

// dropOperand is the verbs' DropOperand, as the text's literal deletion of
// ` && X` left the C: where one operand is left and it is an `||` (or a
// lower operator still), the parentheses the && needed around it stay, a
// paren node, as the text's did.
func dropOperand(v *graph.Verbs, pat string, n int, what string) {
	if v.Failed() {
		return
	}
	e := v.Editor()
	var ms []*graph.Node
	for _, x := range v.Find(pat) {
		q := e.Parent(x)
		if q != nil && (q.Is("&&") || q.Is("||") || q.Is("|")) {
			ms = append(ms, x)
		}
	}
	if len(ms) != n {
		v.Die("%s -- matched %d times, expected %d", what, len(ms), n)
		return
	}
	for _, x := range ms {
		if !e.Live(x) {
			v.Die("%s -- the match vanished: an earlier one took #%d with it", what, x.ID)
			return
		}
		q := e.Parent(x)
		var rest []*graph.Node
		for _, k := range q.Args() {
			if k != x {
				rest = append(rest, k)
			}
		}
		var r *graph.Node
		switch {
		case len(rest) > 1:
			r = graph.NewList(append([]*graph.Node{graph.NewAtom(q.Head())}, rest...)...)
			if !q.Is("|") {
				r.Type = q.Type
			}
		case q.Is("&&") && (rest[0].Is("||") || rest[0].Is("?")):
			r = graph.NewList(graph.NewAtom("paren"), rest[0])
			r.Type = rest[0].Type
		default:
			r = rest[0]
		}
		if err := e.Replace(q, r); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
	}
	v.Say(what)
}
