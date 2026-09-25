package p078

// Whim phase 78 -- empty functions, write-only counters, and the window id.
// See GOAL.md.
//
// Three unrelated kinds of leftover, all of them invisible to the compiler and so to
// every sweep this pipeline runs.  A fourth kind -- the constant-return predicates --
// was split out into its own phase after the survey showed it is not one shape but
// several: only about twenty of the twenty-nine sit in a foldable `if`, the rest
// needing term-level or expression edits, and one of them is a function pointer in an
// option table row that must not be touched at all.  Bundling them here would have
// repeated the shape that cost phase 75 eight iterations.
//
// (1) FIFTEEN FUNCTIONS WITH EMPTY BODIES, 49 call sites.  Each was emptied by an
// earlier phase and left with its callers in place; the call is a no-op that the
// compiler still emits.  EVERY ONE OF THE 49 IS A BARE STATEMENT -- checked, not
// assumed: none appears in an if, an assignment or any larger expression, so a
// line removal cannot corrupt a condition.  That was the trap in phase 75, where
// ins_apply_autocmds calls were invisible to a regex anchored on `apply_autocmds`.
//
// nv_nop IS NOT AMONG THEM.  It is empty by design -- the nv_cmds row for KE_NOP
// -- and nvidxcheck.py requires nv_cmd_idx[] to stay a permutation of the rows.
//
// (2) SIX WRITE-ONLY STATICS.  gcc never warns: assigning to a static counts as using
// it, which is the can_cindent shape.  Each is incremented and decremented and
// never read:
//
// autocmd_blocked         its reader is_autocmd_blocked went in phase 75
// autocmd_no_enter        ++/-- in create_windows
// autocmd_no_leave        ++/-- in create_windows
// redrawing_for_callback  ++/-- in redraw_after_callback
// prevwin                 written once in win_enter_ext, read nowhere since 75
// last_win_id             only `w_id = ++last_win_id`, and w_id goes below
//
// TWO OTHERS ARE FLAGGED BY THE SAME SCAN AND MUST NOT BE TOUCHED.
// breakcheck_count is READ by `if (++breakcheck_count >= BREAKCHECK_SKIP)`, and
// vim_ignored is the deliberate sink for discarded return values, kept on purpose
// in phase 67.  A scanner that counts `++x` as a write and cannot see the read in
// `x = call()` reports both as write-only.  They are not.
//
// block_autocmds() and unblock_autocmds() become EMPTY once the counter goes, and
// they stay that way: they have eight live call sites, one of them deliberately
// unpaired in deathtrap() -- the process is dying and never unblocks -- so
// removing calls would touch a signal handler for no gain.
//
// (3) THE WINDOW ID.  With one window, curwin->w_id is a constant, so both
// `if (is_state.winid != curwin->w_id)` guards in getcmdline_int can never fire.
// Folding them makes incsearch_state_T.winid write-only, which makes w_id
// write-only, which makes last_win_id and LOWEST_WIN_ID unread.  One chain.
// init_incsearch_state keeps a live caller at the top of getcmdline_int, so the
// function stays; only the two re-initialising guards go.
//
// The two guards are spelled at DIFFERENT INDENTS -- one at eight spaces, one at
// twelve -- so they are matched by a regex, not by a literal with a count of two.
//
// (4) ONE DEAD FIELD the field sweep cannot see: cmdarg_T.prechar.  deadfields.py
// exempts every field of a type that has a positional initialiser anywhere, and
// cmdarg_T has `cmdarg_T ca = { 0 };` -- which supplies one value and zero-fills
// the rest, so removing prechar cannot overflow it.
//
// termrequest_T.tr_start WAS on this list and is NOT removed.  It has a single
// identifier mention, its declaration, which is what made an audit call it dead --
// but termrequest_T is positionally initialised three times as {STATUS_GET, -1},
// and that -1 IS tr_start.  A positional initialiser names nothing, so counting
// identifiers cannot see the use.  That is the very reason deadfields exempts such
// types, and the exemption was recorded during the audit and then ignored.
//
// THE DELTA: none expected.  An empty function called or not called does the same
// nothing; a counter nobody reads has no effect; and the two winid guards can never
// fire.  Declared empty, left for the delta check to correct.

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

// emptyFns do nothing at all.  The phase PROVES that before removing a single
// call: if one has acquired a Body again, deleting its calls would change
// behaviour.
var emptyFns = []string{
	"clear_chartabsize_arg", "may_trigger_modechanged",
	"may_trigger_win_scrolled_resized", "out_flush_check", "add_b0_fenc",
	"set_b0_dir_flag", "pum_may_redraw", "ml_setname", "ml_preserve",
	"trigger_undo_ftplugin", "set_init_lang_env", "set_init_default_printencoding",
	"set_init_3", "mch_new_shellsize", "mch_early_init",
}

// Whim78 removes every call to fifteen functions that do nothing, and the
// write-only state five more kept.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nostubs", text, w)

	for _, fn := range emptyFns {
		body, found := e.InnerBody(fn)
		e.Expect(found && strings.TrimSpace(body) == "", "%s is no longer empty -- removing its calls would change behaviour", fn)
	}
	body, found := e.InnerBody("nv_nop")
	e.Expect(found && strings.TrimSpace(body) == "", "nv_nop is no longer empty; it is the nv_cmds KE_NOP row and must stay empty")

	for _, fn := range emptyFns {
		q := regexp.QuoteMeta(fn)
		bare := `(?m)^[ \t]*(?:\(void\))?` + q + `\([^;\n]*\);\n`
		allref := len(regexp.MustCompile(`\b`+q+`\b`).FindAll(cutil.Blank(e.Text()), -1))
		nBare := len(e.Query(bare, 0))
		// every mention that is not the prototype, the definition or a bare
		// call is a use this phase cannot simply delete
		nProto := len(e.Query(`(?m)^static [^\n]*\b`+q+`\(`, 0))
		nDefn := len(e.Query(`(?m)^`+q+`\(`, 0))
		e.Expect(allref == nBare+nProto+nDefn, "%s has %d mentions but only %d bare calls (+%d proto +%d defn) -- one is inside an expression and a line removal would corrupt it",
			fn, allref, nBare, nProto, nDefn)
		e.Cut(bare, nBare, fmt.Sprintf("the %d calls to %s, which does nothing", nBare, fn))
	}

	e.InFunction("getcmdline_int", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(is_state\.winid != curwin->w_id\)$`, 2, "the command line re-initialising incremental search for another window")
	})
	e.InFunction("init_incsearch_state", func(e *edit.E) {
		e.Lines(`is_state->winid = curwin->w_id;`, 1, "recording which window the search started in")
	})
	e.InFunction("win_alloc", func(e *edit.E) {
		e.Lines(`new_wp->w_id = \+\+last_win_id;`, 1, "numbering the one window")
	})
	e.InFunction("block_autocmds", func(e *edit.E) { e.Lines(`\+\+autocmd_blocked;`, 1, "blocking autocommands") })
	e.InFunction("unblock_autocmds", func(e *edit.E) { e.Lines(`--autocmd_blocked;`, 1, "and unblocking them") })
	for _, v := range []string{"autocmd_no_enter", "autocmd_no_leave"} {
		v := v
		e.InFunction("create_windows", func(e *edit.E) {
			e.Lines(`\+\+`+v+`;`, 1, fmt.Sprintf("startup suppressing %s", v))
		})
		e.InFunction("create_windows", func(e *edit.E) {
			e.Lines(`--`+v+`;`, 1, "and restoring it")
		})
	}
	e.InFunction("redraw_after_callback", func(e *edit.E) {
		e.Lines(`\+\+redrawing_for_callback;`, 1, "marking a callback redraw")
	})
	e.InFunction("redraw_after_callback", func(e *edit.E) {
		e.Lines(`--redrawing_for_callback;`, 1, "and unmarking it")
	})
	e.InFunction("win_enter_ext", func(e *edit.E) {
		e.Lines(`prevwin = curwin;`, 1, "remembering the previous window")
	})
	// The fields and statics those writes were the last mention of --
	// incsearch_state_T.winid, w_id, last_win_id, LOWEST_WIN_ID,
	// autocmd_blocked, autocmd_no_enter, autocmd_no_leave,
	// redrawing_for_callback and prevwin -- are named by nothing now; the
	// sweep takes them.
	// A PARTITION, not a count: the member is here and this phase removes it,
	// or it is already gone -- which is accepted only when nothing at all is
	// left that says `prechar`, the one way the sweep's closure
	// (internal/sweep) leaves it.  A stray mention
	// is neither, and Lines refuses it with the message it always had.
	if e.Mentions("prechar") == 0 {
		e.Say("cmdarg_T.prechar: already gone -- nothing says prechar, so the sweep's closure took it")
	} else {
		e.Lines(`int[ \t]+prechar;`, 1, "cmdarg_T.prechar")
	}
	return e.Done()
}

func init() { edit.Register("whim78", Edit) }
