package p024

// Whim phase 24 (formerly 78) -- empty functions, write-only counters, and the window id.
// See GOAL.md.
//
// Three unrelated kinds of leftover, all of them invisible to the compiler and so to
// every sweep this pipeline runs.  A fourth kind -- the constant-return predicates --
// was split out into its own phase after the survey showed it is not one shape but
// several: only about twenty of the twenty-nine sit in a foldable `if`, the rest
// needing term-level or expression edits, and one of them is a function pointer in an
// option table row that must not be touched at all.  Bundling them here would have
// repeated the shape that cost phase 5d eight iterations.
//
// (1) FIFTEEN FUNCTIONS WITH EMPTY BODIES, 49 call sites.  Each was emptied by an
// earlier phase and left with its callers in place; the call is a no-op that the
// compiler still emits.  EVERY ONE OF THE 49 IS A BARE STATEMENT -- checked, not
// assumed: none appears in an if, an assignment or any larger expression, so a
// line removal cannot corrupt a condition.  That was the trap in phase 5d, where
// ins_apply_autocmds calls were invisible to a regex anchored on `apply_autocmds`.
//
// nv_nop IS NOT AMONG THEM.  It is empty by design -- the nv_cmds row for KE_NOP
// -- and nvidxcheck.py requires nv_cmd_idx[] to stay a permutation of the rows.
//
// (2) SIX WRITE-ONLY STATICS.  gcc never warns: assigning to a static counts as using
// it, which is the can_cindent shape.  Each is incremented and decremented and
// never read:
//
// autocmd_blocked         its reader is_autocmd_blocked went in phase 5d
// autocmd_no_enter        ++/-- in create_windows
// autocmd_no_leave        ++/-- in create_windows
// redrawing_for_callback  ++/-- in redraw_after_callback
// prevwin                 written once in win_enter_ext, read nowhere since 75
// last_win_id             only `w_id = ++last_win_id`, and w_id goes below
//
// TWO OTHERS ARE FLAGGED BY THE SAME SCAN AND MUST NOT BE TOUCHED.
// breakcheck_count is READ by `if (++breakcheck_count >= BREAKCHECK_SKIP)`, and
// vim_ignored is the deliberate sink for discarded return values, kept on purpose
// in phase 4b.  A scanner that counts `++x` as a write and cannot see the read in
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
// twelve -- so the text version matched them by a regex, not by a literal with a
// count of two; on the graph they are one form, matched twice.
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

//
// ON THE GRAPH (doc/GRAPH.md, step 5).  The cut is deletions on the program's
// graph and their fall-out, the editor's closure (crefactor/graph), not
// lines matched and removed; its report is the text version's, line for
// line, which the plan ran until step 5 (history keeps it):
//
//   - the twelve empty functions are DELETED -- each still empty, checked
//     first -- and the closure's call rule takes every call: it refuses a
//     use that is not a call standing as a statement, which is the text's
//     "every mention is a bare call, its prototype or its definition",
//     asked of the edges instead of counted with \bname\b;
//   - the two window guards' conditions are REPLACED by 0 -- one window:
//     never another's -- and the closure folds the ifs;
//   - the seven write-only locations are DELETED, and the closure's store
//     rule takes their stores, which must be exactly the text's ten acts,
//     function by function and shape by shape;
//   - cmdarg_T.prechar is deleted, or is gone already.
//
// What nothing names afterwards -- LOWEST_WIN_ID, the empty functions'
// types -- is the collection's.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// emptyFns do nothing at all.  The phase PROVES that before removing a single
// call: if one has acquired a body again, deleting its calls would change
// behaviour.
var emptyFns = []string{
	"clear_chartabsize_arg", "may_trigger_modechanged",
	"may_trigger_win_scrolled_resized", "out_flush_check", "add_b0_fenc",
	"pum_may_redraw",
	"trigger_undo_ftplugin", "set_init_lang_env", "set_init_default_printencoding",
	"set_init_3", "mch_new_shellsize", "mch_early_init",
}

// ml_preserve, the fifteenth, went with :write at phase 1 (filefront, D4).
// ml_setname and set_b0_dir_flag, the thirteenth and fourteenth, went with
// buf_name_changed(), their last caller, once setfname() lost its last
// callers: phase 20's set_rw_fname() fold and, from phase 5, phase 5b's body for
// create_windows() (whim5b, which runs before this phase now).

// writeOnly are the locations the phase's statements were the last writes
// of: deleted, their stores are the closure's.  `type.member` names a
// member of the struct a typedef names.  (prevwin's one write, in
// win_enter_ext, went with :q's refusal at phase 1: quitfront.)
var writeOnly = []string{
	"incsearch_state_T.winid", "win_T.w_id", "last_win_id",
	"autocmd_blocked", "autocmd_no_enter", "autocmd_no_leave", "redrawing_for_callback",
}

// acts are the statements those deletions take, in the order the phase
// reports them: in the function fn, n items of the pattern's shape go, and
// what says so.  The closure must have removed exactly these.
var acts = []struct {
	fn, pat string
	n       int
	what    string
}{
	{"init_incsearch_state", "(= (-> is_state winid) (-> curwin w_id))", 1, "recording which window the search started in"},
	{"win_alloc", "(= (-> new_wp w_id) (pre++ last_win_id))", 1, "numbering the one window"},
	{"block_autocmds", "(pre++ autocmd_blocked)", 1, "blocking autocommands"},
	{"unblock_autocmds", "(pre-- autocmd_blocked)", 1, "and unblocking them"},
	{"create_windows", "(pre++ autocmd_no_enter)", 1, "startup suppressing autocmd_no_enter"},
	{"create_windows", "(pre-- autocmd_no_enter)", 1, "and restoring it"},
	{"create_windows", "(pre++ autocmd_no_leave)", 1, "startup suppressing autocmd_no_leave"},
	{"create_windows", "(pre-- autocmd_no_leave)", 1, "and restoring it"},
	{"redraw_after_callback", "(pre++ redrawing_for_callback)", 1, "marking a callback redraw"},
	{"redraw_after_callback", "(pre-- redrawing_for_callback)", 1, "and unmarking it"},
}

// Edit removes every call to twelve functions that do nothing, and the
// write-only state five more kept, on the graph.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	opt := whim.GraphFallOut
	say := func(what string) { fmt.Fprintf(w, "  %-12s %s\n", "nostubs", what) }
	for _, fn := range append(append([]string{}, emptyFns...), "nv_nop") {
		if d := e.Defn(fn); d == nil || len(graph.Body(d)) != 0 {
			if fn == "nv_nop" {
				return fmt.Errorf("nv_nop is no longer empty; it is the nv_cmds KE_NOP row and must stay empty")
			}
			return fmt.Errorf("%s is no longer empty -- removing its calls would change behaviour", fn)
		}
	}

	for _, fn := range emptyFns {
		for _, d := range e.FileDecls(fn) {
			if err := e.Delete(d); err != nil {
				return err
			}
		}
		st, err := e.FallOut(opt)
		if u, ok := err.(*graph.Unhandled); ok {
			return fmt.Errorf("%s has a mention that is not a bare call, its prototype or its definition -- one is inside an expression (%v)", fn, u)
		}
		if err != nil {
			return err
		}
		calls := 0
		for _, r := range st.Removed {
			switch r.Rule {
			case "call":
				calls++
			case "empty": // a block the call was alone in, when the closure is told to take it
			default:
				return fmt.Errorf("%s: the closure removed a %s, not a call", fn, r.Rule)
			}
		}
		say(fmt.Sprintf("the %d calls to %s, which does nothing", calls, fn))
	}

	// the two guards re-initialising incremental search for another window
	guard := clisp.MustPattern("(if (!= (. is_state winid) (-> curwin w_id)) _*)")
	d := e.Defn("getcmdline_int")
	if d == nil {
		return fmt.Errorf("getcmdline_int is not defined at file scope")
	}
	ifs := graph.Find(d, func(n *graph.Node) bool { return graph.Matches(guard, n) })
	if len(ifs) != 2 {
		return fmt.Errorf("the command line re-initialising incremental search for another window -- matched %d times, expected 2", len(ifs))
	}
	for _, s := range ifs {
		if err := e.Replace(s.Kids[1], graph.NewAtom("0")); err != nil {
			return err
		}
	}
	st, err := e.FallOut(opt)
	if err != nil {
		return err
	}
	if st.Branches != 2 {
		return fmt.Errorf("the window guards: %d folded, expected 2", st.Branches)
	}
	say("the command line re-initialising incremental search for another window")

	// the write-only state
	for _, loc := range writeOnly {
		n, err := location(e, loc)
		if err != nil {
			return err
		}
		if err := e.Delete(n); err != nil {
			return err
		}
	}
	st, err = e.FallOut(opt)
	if err != nil {
		return fmt.Errorf("the write-only state: %w", err)
	}
	claimed := 0
	for _, k := range acts {
		p := clisp.MustPattern(k.pat)
		n := 0
		for _, r := range st.Removed {
			if r.Fn == k.fn && graph.Matches(p, r.Item) {
				n++
			}
		}
		if n != k.n {
			return fmt.Errorf("%s -- %d matches, expected %d", k.what, n, k.n)
		}
		claimed += n
		say(k.what)
	}
	if claimed != len(st.Removed) {
		return fmt.Errorf("the write-only state: the closure removed %d statements, the acts say %d", len(st.Removed), claimed)
	}

	// A PARTITION, not a count: cmdarg_T.prechar is here and this phase
	// removes it, or it is already gone -- which is accepted only when
	// nothing at all says `prechar`, the one way the collection leaves it.
	m, err := location(e, "cmdarg_T.prechar")
	if m == nil && !says(e, "prechar") {
		say("cmdarg_T.prechar: already gone -- nothing says prechar, so the sweep's closure took it")
		return nil
	}
	if err != nil {
		return err
	}
	if !graph.Matches(clisp.MustPattern("(prechar int)"), m) {
		return fmt.Errorf("cmdarg_T.prechar -- not `int prechar;`")
	}
	if err := e.Delete(m); err != nil {
		return err
	}
	if _, err := e.FallOut(opt); err != nil {
		return fmt.Errorf("cmdarg_T.prechar: %w", err)
	}
	say("cmdarg_T.prechar")
	return nil
}

// location is the file-scope object, or the member `type.member` of the
// struct a typedef of that name defines.
func location(e *graph.Editor, loc string) (*graph.Node, error) {
	for i := 0; i < len(loc); i++ {
		if loc[i] != '.' {
			continue
		}
		typ, mem := loc[:i], loc[i+1:]
		for _, f := range e.Graph().Forms {
			if !f.Is("typedef") || graph.DeclName(f) != typ {
				continue
			}
			s := f.Kids[len(f.Kids)-1]
			if len(graph.Members(s)) == 0 && s.Ref() != nil {
				s = s.Ref() // `typedef struct tag T;`: the tag's definition
			}
			if !s.Is("struct") && !s.Is("union") {
				return nil, fmt.Errorf("%s: not a struct's typedef", typ)
			}
			for _, m := range graph.Members(s) {
				if m.IsList() && len(m.Kids) > 0 && m.Kids[0].Atom == mem {
					return m, nil
				}
			}
			return nil, fmt.Errorf("%s has no member %s", typ, mem)
		}
		return nil, fmt.Errorf("no typedef %s", typ)
	}
	ds := e.FileDecls(loc)
	if len(ds) != 1 || !ds[0].Is("def") {
		return nil, fmt.Errorf("%s: %d file-scope declarations, expected its one definition", loc, len(ds))
	}
	return ds[0], nil
}

// says is whether any node of the file is the atom name.
func says(e *graph.Editor, name string) bool {
	found := false
	for _, f := range e.Graph().Forms {
		graph.Walk(f, func(n *graph.Node) bool {
			found = found || !n.IsList() && n.Atom == name
			return !found
		})
	}
	return found
}

func init() { phase.RegisterGraph("whim24", Edit) }
