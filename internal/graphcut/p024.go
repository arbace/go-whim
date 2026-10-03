package graphcut

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
)

// emptyFns are phase 24's twelve functions that do nothing
// (internal/phase/024's list, the same names).
var emptyFns = []string{
	"clear_chartabsize_arg", "may_trigger_modechanged",
	"may_trigger_win_scrolled_resized", "out_flush_check", "add_b0_fenc",
	"pum_may_redraw",
	"trigger_undo_ftplugin", "set_init_lang_env", "set_init_default_printencoding",
	"set_init_3", "mch_new_shellsize", "mch_early_init",
}

// writeOnly are the locations phase 24's statements were the last writes
// of: deleted, their stores are the closure's.  `struct.member` names a
// member.
var writeOnly = []string{
	"incsearch_state_T.winid", "win_T.w_id", "last_win_id",
	"autocmd_blocked", "autocmd_no_enter", "autocmd_no_leave", "redrawing_for_callback",
}

// p24acts are the text version's acts on those stores, in its order: in the
// function fn, n items of the pattern's shape go, and what says so.  The
// closure must have removed exactly these.
var p24acts = []struct {
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

// controlAlways is the control: set, the two window guards are taken as
// always true instead of never -- the guarded call stays, and the byte
// comparison must catch it.
var controlAlways bool

// P24 is phase 24 (internal/phase/024, whim24) on the graph, as deletions
// and their fall-out:
//
//   - the twelve empty functions are DELETED -- each still empty, checked
//     first -- and the closure's call rule takes every call: it refuses a
//     use that is not a call standing as a statement, which is the text's
//     "every mention is a bare call, its prototype or its definition",
//     asked of the edges instead of counted with \bname\b;
//   - the two window guards' conditions are REPLACED by 0 -- one window:
//     never another's -- and the closure folds the ifs;
//   - the seven write-only locations are DELETED, and the closure's store
//     rule takes their stores, which must be exactly the text's ten acts;
//   - cmdarg_T.prechar is deleted, or is gone already.
//
// What nothing names afterwards -- LOWEST_WIN_ID, the empty functions'
// types -- is the collection's.  It reports what the text version reports,
// line for line.
func P24(e *graph.Editor, opt graph.FallOutOptions, w io.Writer) error {
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
	never := "0"
	if controlAlways {
		never = "1"
	}
	for _, s := range ifs {
		if err := e.Replace(s.Kids[1], graph.NewAtom(never)); err != nil {
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
	for _, k := range p24acts {
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

	// cmdarg_T.prechar: the member is here and goes, or nothing names it
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
