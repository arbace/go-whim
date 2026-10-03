package treepilot

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
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

// controlKeepCall is the pilot's control: set, P24 leaves the last call to
// each empty function -- C that does the same nothing, and a change the byte
// comparison must catch.
var controlKeepCall bool

// An act is one of the phase's counted acts: in the function fn, the n items
// the pattern matches go, and what says so.
type act struct {
	fn   string
	pat  string
	n    int
	what string
}

var p24acts = []act{
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

// P24 is phase 24 (internal/phase/024, whim24) on the tree: every call to
// twelve functions that do nothing, and the write-only state five more kept.
// It asserts what the text version asserts, with the same counts -- each
// function still empty, every mention of one a bare call, its prototype or
// its definition -- but asks the resolver rather than counting words: a
// mention is a use of the file-scope function, which a local or a member of
// the same name is not.
func P24(t *Tree, w io.Writer) error {
	root, ix := t.Root, t.Index()
	say := func(what string) { fmt.Fprintf(w, "  %-12s %s\n", "nostubs", what) }
	for _, fn := range append(append([]string{}, emptyFns...), "nv_nop") {
		d := clisp.Definition(root, fn)
		if d == nil || len(clisp.Body(d.Node())) != 0 {
			if fn == "nv_nop" {
				return fmt.Errorf("nv_nop is no longer empty; it is the nv_cmds KE_NOP row and must stay empty")
			}
			return fmt.Errorf("%s is no longer empty -- removing its calls would change behaviour", fn)
		}
	}

	sc := clisp.Resolve(root.List)
	bare := clisp.MustPattern("(call ?fn _*)")
	voided := clisp.MustPattern("(cast void (call ?fn _*))")
	for _, fn := range emptyFns {
		d := sc.File[fn]
		if d == nil {
			return fmt.Errorf("%s is not declared at file scope", fn)
		}
		// every use is the callee of a call that is a statement of its own
		var calls []*clisp.Cursor
		for _, a := range ix.Atoms(fn) {
			if sc.DeclOf(a.Node()) != d {
				continue
			}
			it := a.Item()
			if it == nil {
				continue
			}
			b, ok := clisp.Match(bare, it.Node())
			if !ok {
				b, ok = clisp.Match(voided, it.Node())
			}
			if ok && b["fn"] == a.Node() {
				calls = append(calls, it)
			}
		}
		proto, defn := 0, 0
		for _, x := range d.Nodes {
			if x.Is("defn") {
				defn++
			} else if clisp.HasPrefix(x, "static") {
				proto++
			}
		}
		uses := len(sc.Uses(d))
		if uses != len(calls) || len(d.Nodes) != proto+defn {
			return fmt.Errorf("%s has %d mentions but only %d bare calls (+%d proto +%d defn) -- one is inside an expression and a line removal would corrupt it",
				fn, uses+len(d.Nodes), len(calls), proto, defn)
		}
		if controlKeepCall && len(calls) > 0 {
			calls = calls[:len(calls)-1]
		}
		for _, c := range calls {
			c.Delete()
		}
		say(fmt.Sprintf("the %d calls to %s, which does nothing", len(calls), fn))
	}

	// the two guards re-initialising incremental search for another window
	guard := clisp.MustPattern("(if (!= (. is_state winid) (-> curwin w_id)) _*)")
	ifs, ok := clisp.FindIn(root, "getcmdline_int", func(n *clisp.Node) bool { return clisp.Matches(guard, n) })
	if !ok {
		return fmt.Errorf("getcmdline_int is not defined at file scope")
	}
	if len(ifs) != 2 {
		return fmt.Errorf("the command line re-initialising incremental search for another window -- matched %d times, expected 2", len(ifs))
	}
	for _, c := range ifs {
		c.FoldNever()
	}
	say("the command line re-initialising incremental search for another window")

	for _, k := range p24acts {
		p := clisp.MustPattern(k.pat)
		cs, ok := clisp.FindIn(root, k.fn, func(n *clisp.Node) bool { return clisp.Matches(p, n) })
		if !ok {
			return fmt.Errorf("%s is not defined at file scope", k.fn)
		}
		items := cs[:0]
		for _, c := range cs {
			if c.IsItem() {
				items = append(items, c)
			}
		}
		if len(items) != k.n {
			return fmt.Errorf("%s -- %d matches, expected %d", k.what, len(items), k.n)
		}
		for _, c := range items {
			c.Delete()
		}
		say(k.what)
	}

	// cmdarg_T.prechar: the member is here and goes, or nothing names it
	if ix.Mentions("prechar", true) == 0 {
		say("cmdarg_T.prechar: already gone -- nothing says prechar, so the sweep's closure took it")
		return nil
	}
	var members []*clisp.Cursor
	for _, a := range ix.Atoms("prechar") {
		if m := a.Up(); m != nil && m.Parent().Is("struct") && clisp.Equal(m.Node(), clisp.MustPattern("(prechar int)")) {
			members = append(members, m)
		}
	}
	if len(members) != 1 {
		return fmt.Errorf("cmdarg_T.prechar -- %d matches, expected 1", len(members))
	}
	members[0].Delete()
	say("cmdarg_T.prechar")
	return nil
}
