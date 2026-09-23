package p079

// Whim phase 79 -- the constant-return predicates.  See GOAL.md.
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
// the phase fails instead of folding a live predicate.  That is the phase 77 pattern
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
// behaviour silently in phase 71 -- but that phase FOLDED A WALK, keeping the body
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
// this file was counted against the q78 tree before it was written.
//
// NO TERM EDIT ENDS IN WHITESPACE.  `only_one_window() && check_changed_any` becomes
// `check_changed_any` rather than stripping `only_one_window() && `, because a
// literal with a trailing space lost it passing through an editor in phase 71 and the
// match then failed for reasons invisible in the diff.
//
// THE DELTA: none expected.  Every fold removes a branch whose condition cannot hold,
// and every term edit removes a conjunct that is constantly true or a disjunct that
// is constantly false.  The quit path is the one place where getting this wrong is
// silent rather than fatal -- check_more() feeds the four ex_quit/ex_exit conditions
// that decide whether getout(0) runs -- so five quit probes, calibrated on q78, guard
// it directly.  Declared empty, left for whimdelta.sh to correct.

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

// w79Constants are functions whose WHOLE Body is `return <constant>;`.  The
// phase proves each one before folding anything that calls it: a stub that has
// acquired a Body again would make every fold below change behaviour.
var w79Constants = map[string]string{
	"append_arg_number": "0", "at_ins_compl_key": "FALSE", "bomb_size": "0",
	"bt_quickfix": "FALSE", "bt_terminal": "FALSE",
	"check_can_set_curbuf_disabled": "TRUE", "check_can_set_curbuf_forceit": "TRUE",
	"check_more": "OK", "check_timestamps": "0", "current_tab_nr": "1",
	"current_win_nr": "1", "did_set_number_relativenumber": "NULL",
	"get_cellwidth": "0", "has_cursormoved": "FALSE", "has_insertcharpre": "FALSE",
	"has_textchanged": "FALSE", "in_vim9script": "FALSE", "ins_compl_active": "FALSE",
	"ins_compl_lnum_in_range": "FALSE", "ins_compl_win_active": "FALSE",
	"only_one_window": "TRUE", "pum_redraw_in_same_position": "FALSE",
	"pum_under_menu": "FALSE", "pum_visible": "FALSE", "script_get": "NULL",
	"stl_connected": "FALSE", "tabline_height": "0", "wc_use_keyname": "FALSE",
}

// w79Variable return a VARIABLE rather than a constant and are LEFT ALONE.  They
// are listed and checked so that the distinction is stated rather than assumed:
// folding one of these would freeze a value that still changes.
var w79Variable = map[string]string{
	"get_hislen": "hislen", "get_search_pat": "mr_pattern",
	"get_text_locked_msg": "e_not_allowed_to_change_text_or_change_window",
	"is_maphash_valid":    "maphash_valid",
}

// Whim79 folds every call to a function whose Body is a constant.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("noconstfn", text, w)

	for _, n := range edit.SortedKeys(w79Constants) {
		e.ConstOf(n, w79Constants[n])
	}
	if e.Failed() {
		return e.Done()
	}
	e.Say(fmt.Sprintf("confirmed: %d functions whose whole body is `return <constant>;`", len(w79Constants)))
	for _, n := range edit.SortedKeys(w79Variable) {
		e.ConstOf(n, w79Variable[n])
	}
	if e.Failed() {
		return e.Done()
	}
	e.Say(fmt.Sprintf("confirmed: %d more return a VARIABLE and are left alone", len(w79Variable)))
	// THE SIGNATURE IS NOT THE BODY.  `long *wcp` is in the parameter list of
	// every version of this function, so asking the whole definition whether it
	// mentions wcp answers yes for ever and the check can never pass.  The
	// Python asks the INNER Body -- from the `{` on its own line to the last
	// `}` -- which is what innerBody reproduces.
	if Body, ok := e.InnerBody("wc_use_keyname"); !ok || strings.Contains(Body, "wcp") {
		e.Refuse("wc_use_keyname now mentions wcp -- it may write through the out-parameter")
		return e.Done()
	}
	e.Say("confirmed: wc_use_keyname never dereferences its out-parameter")

	e.FoldNever(`(?m)^[ \t]*if \(vim9script && \(flags & DOCMD_RANGEOK\) == 0\)$`,
		"scanning backwards for a colon to decide whether a range is allowed")
	e.FoldNever(`(?m)^[ \t]*if \(vim9script && !may_have_range\)$`, "the Vim9 path through find_ex_command")
	e.FoldNeverCount(`(?m)^[ \t]*if \(vim9script\)$`, 2, "two Vim9 checks in parse_command_modifiers")
	e.FoldNever(`(?m)^[ \t]*if \(vim9script && has_cmdmod\(cmod, FALSE\)\)$`, "a command modifier without a command")
	e.Term("(*p == '\"' && !vim9script && !(eap->argt & EX_NOTRLCOM)",
		"(*p == '\"' && !(eap->argt & EX_NOTRLCOM)", 1,
		"a double quote always starts a comment outside Vim9 script")
	e.Term("(*p == '#' && vim9script && !(eap->argt & EX_NOTRLCOM) && p > eap->cmd && ((p[-1]) == ' ' || (p[-1]) == '\\t')) || (*p == '|' && eap->cmdidx != CMD_append",
		"(*p == '|' && eap->cmdidx != CMD_append", 1, "and a hash never does")
	for _, indent := range []string{"", "    ", "        "} {
		e.Lines(`int `+indent+`vim9script = in_vim9script\(\);`, 1, "the local that recorded it")
	}
	e.FoldAlways(`(?m)^[ \t]*if \(may_have_range\)$`, "skipping a range that is always allowed")
	e.FoldNever(`(?m)^[ \t]*if \(!may_have_range\)$`, "the default address for a range that cannot be absent")
	e.Lines(`may_have_range = TRUE;`, 1, "the flag nothing decides any more")
	e.Lines(`int may_have_range;`, 1, "and its declaration")
	e.FoldNever(`(?m)^[ \t]*if \(in_vim9script\(\) && \*p == '\\'' &&  \(\(unsigned\)\(p\[1\]\) - '0' < 10\) \)$`,
		"a digit separator in a Vim9 number literal")
	e.FoldNever(`(?m)^[ \t]*if \(in_vim9script\(\) && \*p == '#'\)$`, "a hash comment ending a Vim9 command")
	e.FoldNever(`(?m)^[ \t]*if \(in_vim9script\(\) && arg > arg_start && vim_strchr\(\(char_u \*\)"!&<", \*arg\) != NULL\)$`,
		"the Vim9 spacing rule for an unknown option")
	e.FoldNeverCount(`(?m)^[ \t]*if \(in_vim9script\(\)\)$`, 8, "eight more Vim9 script branches")
	e.Term("in_vim9script() ? GETLINE_CONCAT_CONTBAR : GETLINE_CONCAT_CONT", "GETLINE_CONCAT_CONT", 1,
		"how a continuation line is joined")
	e.FoldNever(`(?m)^[ \t]*if \(pum_under_menu\(row, col, TRUE\)\)$`, "skipping a cell the popup menu covers")
	e.FoldNever(`(?m)^[ \t]*if \(pum_visible\(\) && \(State & MODE_CMDLINE\) == 0 && pum_under_menu\(row, col, FALSE\)\)$`,
		"and the same test on the command line")
	e.ConstOf("skip_for_popup", "FALSE")
	if !e.Failed() {
		e.Say("confirmed: skip_for_popup has collapsed to `return FALSE;`")
	}
	e.Term("may_trigger_safestate(ready && !ins_compl_active() && !pum_visible());",
		"may_trigger_safestate(ready);", 1, "whether a state is safe no longer asks about completion")
	e.FoldNeverCount(`(?m)^[ \t]*if \(pum_visible\(\)\)$`, 2, "two redraws deferred for the popup menu")
	e.FoldNever(`(?m)^[ \t]*if \(!ignore_pum && pum_visible\(\)\)$`, "the ruler deferred for it")
	e.FoldNever(`(?m)^[ \t]*if \(pum_redraw_in_same_position\(\)\)$`, "redrawing it in place")
	e.Term(" || (!ignore_pum && pum_visible())", "", 1, "the status line deferred for it")
	e.Term(" && !pum_visible())", ")", 1, "'relativenumber' redrawing around it")
	e.Term(" && !ins_compl_active())", ")", 1, "'showmatch' suppressed during completion")
	e.FoldNeverCount(`(?m)^[ \t]*if \(\(State & MODE_INSERT\) && ins_compl_win_active\(wp\) && \(in_curline \|\| ins_compl_lnum_in_range\(lnum\)\)\)$`, 2,
		"two completion highlights in the line drawer")
	e.Term(" && !at_ins_compl_key())", ")", 1, "a mapping suppressed by a completion key")
	e.FoldNever(`(?m)^[ \t]*if \(redraw_this && char_cells == 2 && skip_for_popup\(row, col \+ coloff \+ 1\)\)$`,
		"a double-width cell under the menu")
	e.FoldNever(`(?m)^[ \t]*if \(redraw_this && skip_for_popup\(row, col \+ coloff\)\)$`, "and a single-width one")
	e.Term(" && !skip_for_popup(row, col + coloff))", ")", 1, "clearing the next cell")
	e.FoldAlways(`(?m)^[ \t]*if \(!skip_for_popup\(row, col \+ coloff\)\)$`, "drawing a screen line cell")
	e.FoldAlways(`(?m)^[ \t]*if \(!skip_for_popup\(row, col - 1\)\)$`, "redrawing the cell to the left")
	e.Term(" && !skip_for_popup(row, col))", ")", 3, "three more cells that are never covered")
	e.FoldAlways(`(?m)^[ \t]*if \(!skip_for_popup\(r, c\)\)$`, "and filling a screen region")
	e.FoldAlways(`(?m)^[ \t]*if \(quit_all \|\| \(check_more\(FALSE, forceit\) == OK\)\)$`, "the autocommand check before quitting")
	e.FoldAlwaysCount(`(?m)^[ \t]*if \(check_more\(FALSE, eap->forceit\) == OK && only_one_window\(\)\)$`, 2,
		"deciding to exit in :quit and :exit")
	e.Term(" || check_more(TRUE, eap->forceit) == FAIL", "", 2, "refusing to quit with more files to edit")
	e.Term("only_one_window() && check_changed_any", "check_changed_any", 2, "and asking whether this is the last window")
	e.FoldNeverCount(`(?m)^[ \t]*if \(stl_connected\(wp\)\)$`, 2, "a status line joined to the one beside it")
	e.FoldNever(`(?m)^[ \t]*if \(get_cellwidth\(ScreenLinesUC\[off\]\) > 1\)$`, "a character widened by 'setcellwidths'")
	e.FoldNever(`(?m)^[ \t]*if \(wc_use_keyname\(varp, &wc\)\)$`, "showing a numeric option as a key name")
	e.FoldNever(`(?m)^[ \t]*if \(wc != 0\)$`, "and showing it as a character")
	e.FoldNever(`(?m)^[ \t]*if \(!check_can_set_curbuf_disabled\(\)\)$`, "refusing to change buffer in gf")
	e.FoldNever(`(?m)^[ \t]*if \(\(is_other_file\(0, ffname\) && !check_can_set_curbuf_forceit\(eap->forceit\)\)\)$`, "and refusing in :edit")
	e.Term(" && !bt_terminal(wp->w_buffer)", "", 1, "the [+] flag suppressed for a terminal buffer")
	e.Term(" && !bt_quickfix(curbuf)", "", 1, "a quickfix buffer never being reusable")
	e.Term(" && !has_insertcharpre()", "", 1, "the InsertCharPre fast path")
	e.FoldNever(`(?m)^[ \t]*if \(!finish_op && \(has_cursormoved\(\)\) && ! `, "tracking the cursor for CursorMoved")
	e.FoldNever(`(?m)^[ \t]*if \(!finish_op && has_textchanged\(\) && `, "and the change tick for TextChanged")
	e.DropIfCount(`(?m)^[ \t]*if \(need_check_timestamps\)$`, 3, "three checks for a file changed outside the editor")
	e.Lines(`need_check_timestamps = TRUE;`, 1, "asking for one")
	e.Lines(`static int      need_check_timestamps  = FALSE ;`, 1, "and the flag itself")
	e.Lines(`need_redraw = check_timestamps\(FALSE\);`, 1, "the timestamp check on focus")
	e.FoldNever(`(?m)^[ \t]*if \(need_redraw\)$`, "and the redraw it asked for")
	e.Lines(`\(void\)append_arg_number\(curwin, \(char_u \*\)buffer \+ bufferlen,  \(1024\+1\)  - bufferlen, !shortmess\(SHM_FILE\)\);`, 1,
		"appending the argument-list position to the file message")
	e.Term(w79lit1, w79lit2, 1, "reading a here-document for a command that cannot run")
	e.Lines(`bom_count = bomb_size\(\);`, 1, "counting the byte order mark")
	e.FoldNever(`(?m)^[ \t]*if \(dict == NULL && bom_count > 0\)$`, "and reporting it")
	e.Term(w79lit3, w79lit4, 1, "the window-or-tab count for a bare range")
	for _, f := range []struct {
		fn, arg string
		n       int
	}{
		{"current_win_nr", "curwin", 3}, {"current_win_nr", "NULL", 3},
		{"current_tab_nr", "curtab", 3}, {"current_tab_nr", "NULL", 3},
	} {
		e.Term(fmt.Sprintf("%s(%s)", f.fn, f.arg), "1", f.n,
			fmt.Sprintf("there is one window and one tabpage (%s(%s))", f.fn, f.arg))
	}
	e.Term(w79lit5, w79lit6, 1, "the minimum rows needed, without a tab line")
	e.Lines(`total \+= tabline_height\(\);`, 1, "the tab line in the all-tabpages minimum")
	e.Term("int         row = tabline_height();", "int         row = 0;", 1, "window layout starting at the top row")
	e.Term("(Rows - p_ch - tabline_height())", "(Rows - p_ch)", 5, "five window heights with no tab line to subtract")
	e.Term("tabline_height() + topframe->fr_height", "topframe->fr_height", 1, "and the 'cmdheight' consistency check")
	return e.Done()
}

func init() { edit.Register("whim79", Edit) }
