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
// fire.  Declared empty, left for whimdelta.sh to correct.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

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

// bodyIsEmpty says whether a definition's Body holds nothing but whitespace.
func bodyIsEmpty(text []byte, name string) (bool, bool) {
	a, z, ok := cutil.FindDefinition(text, cutil.Blank(text), name)
	if !ok {
		return false, false
	}
	Inner := text[a:z]
	i := bytes.Index(Inner, []byte("{"))
	j := bytes.LastIndex(Inner, []byte("}"))
	if i < 0 || j <= i {
		return false, false
	}
	return len(bytes.TrimSpace(Inner[i+1:j])) == 0, true
}

// Whim78 removes every call to fifteen functions that do nothing, and the
// write-only state five more kept.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nostubs", text, w)

	for _, fn := range emptyFns {
		empty, found := bodyIsEmpty(text, fn)
		if !found || !empty {
			e.Refuse("%s is no longer empty -- removing its calls would change behaviour", fn)
			return e.Done()
		}
	}
	if empty, found := bodyIsEmpty(text, "nv_nop"); !found || !empty {
		e.Refuse("nv_nop is no longer empty; it is the nv_cmds KE_NOP row and must stay empty")
		return e.Done()
	}

	total := 0
	for _, fn := range emptyFns {
		q := regexp.QuoteMeta(fn)
		bare := regexp.MustCompile(`(?m)^[ \t]*(?:\(void\))?` + q + `\([^;\n]*\);[ \t]*\n`)
		allref := len(regexp.MustCompile(`\b`+q+`\b`).FindAll(cutil.Blank(e.Text()), -1))
		nBare := len(bare.FindAll(e.Text(), -1))
		// every mention that is not the prototype, the definition or a bare
		// call is a use this phase cannot simply delete
		nProto := len(regexp.MustCompile(`(?m)^static [^\n]*\b`+q+`\(`).FindAll(e.Text(), -1))
		nDefn := len(regexp.MustCompile(`(?m)^`+q+`\(`).FindAll(e.Text(), -1))
		if allref != nBare+nProto+nDefn {
			e.Refuse("%s has %d mentions but only %d bare calls (+%d proto +%d defn) -- one is inside an expression and a line removal would corrupt it",
				fn, allref, nBare, nProto, nDefn)
			return e.Done()
		}
		e.Set(bare.ReplaceAll(e.Text(), nil))
		total += nBare
	}
	e.Say(fmt.Sprintf("every call to the %d functions that do nothing (%d sites)", len(emptyFns), total))

	e.FoldNeverIn2("getcmdline_int", `(?m)^[ \t]*if \(is_state\.winid != curwin->w_id\)$`,
		"the command line re-initialising incremental search for another window", 2)
	e.InFunction("init_incsearch_state", func(e *edit.E) {
		e.Lines(`is_state->winid = curwin->w_id;`, 1, "recording which window the search started in")
	})
	e.Lines(`int[ \t]+winid;`, 1, "the field that recorded it")
	e.InFunction("win_alloc", func(e *edit.E) {
		e.Lines(`new_wp->w_id = \+\+last_win_id;`, 1, "numbering the one window")
	})
	e.Lines(`int[ \t]+w_id;`, 1, "the number it was given")
	e.Lines(`static int last_win_id = LOWEST_WIN_ID - 1;`, 1, "the counter behind it")
	e.Lines(`enum \{ LOWEST_WIN_ID = 1000 \};`, 1, "and where the numbering started")
	e.InFunction("block_autocmds", func(e *edit.E) { e.Lines(`\+\+autocmd_blocked;`, 1, "blocking autocommands") })
	e.InFunction("unblock_autocmds", func(e *edit.E) { e.Lines(`--autocmd_blocked;`, 1, "and unblocking them") })
	e.Lines(`static int[ \t]+autocmd_blocked = 0;`, 1, "the count nothing reads")
	for _, v := range []string{"autocmd_no_enter", "autocmd_no_leave"} {
		v := v
		e.InFunction("create_windows", func(e *edit.E) {
			e.Lines(`\+\+`+v+`;`, 1, fmt.Sprintf("startup suppressing %s", v))
		})
		e.InFunction("create_windows", func(e *edit.E) {
			e.Lines(`--`+v+`;`, 1, "and restoring it")
		})
	}
	e.Lines(`static int[ \t]+autocmd_no_enter  = FALSE ;`, 1, "the enter flag")
	e.Lines(`static int[ \t]+autocmd_no_leave  = FALSE ;`, 1, "the leave flag")
	e.InFunction("redraw_after_callback", func(e *edit.E) {
		e.Lines(`\+\+redrawing_for_callback;`, 1, "marking a callback redraw")
	})
	e.InFunction("redraw_after_callback", func(e *edit.E) {
		e.Lines(`--redrawing_for_callback;`, 1, "and unmarking it")
	})
	e.Lines(`static int redrawing_for_callback = 0;`, 1, "the mark nothing reads")
	e.InFunction("win_enter_ext", func(e *edit.E) {
		e.Lines(`prevwin = curwin;`, 1, "remembering the previous window")
	})
	e.Lines(`static win_T[ \t]+\*prevwin  = NULL ;`, 1, "the window nothing looks back at")
	e.Lines(`int[ \t]+prechar;`, 1, "cmdarg_T.prechar")
	return e.Done()
}

func init() { edit.Register("whim78", Edit) }
