package p005d

// Whim phase 5d (formerly 75) -- no autocommands.  See GOAL.md.
//
// PROVED BY ABSENCE, not inferred from the command table.  `first_autopat[NUM_EVENTS]
// = { NULL }` is the ONLY write to that array in the whole file -- every other mention
// reads it.  No autocommand pattern can ever be registered, so:
//
// * apply_autocmds_group() is already `return FALSE;`
// * apply_autocmds(), apply_autocmds_exarg() and apply_autocmds_retval() are
// one-line wrappers onto it, so ALL ~74 dispatch sites are no-ops;
// * has_cursormovedI/has_textchangedI/has_textchangedP each return
// `first_autopat[...] != NULL`, i.e. always FALSE;
// * au_cleanup, au_remove_pat, au_del_cmd and aubuflocal_remove walk a permanently
// empty list.
//
// :autocmd, :augroup, :doautocmd, :doautoall and :noautocmd were already ex_ni, but
// that is the weaker argument; the array being write-once-to-NULL is the strong one.
//
// TWO SITES ARE REWRITTEN, NOT FOLDED, and both would have been silent damage:
//
// * close_buffer() -- the label `aucmd_abort:` sits INSIDE the block guarded by
// apply_autocmds(EVENT_BUFWINLEAVE, ...), and THREE gotos target it, two of them
// from outside that block under `if (abort_if_last)`.  fold_never would delete
// the label and orphan them.  This is the phase-71 break-rebinding hazard wearing
// a label instead of a loop.  The abort arm is hoisted out and kept reachable.
// * open_buffer() -- the aco block mixes the autocommand call with REAL work:
// `curbuf->b_flags &= ~(BF_CHECK_RO | BF_NEVERLOADED)`.  Deleting it wholesale
// would change behaviour.
//
// THE CLASSIFICATION WAS DONE BY HAND because a scan got two sites BACKWARDS.
// 7712 and 7741 read `if (!(did_cmd = apply_autocmds_exarg(...)))` -- negated with an
// embedded assignment -- so they are ALWAYS TRUE (fold_always), not always false.
// A `startswith("if (!apply_autocmds")` test misses the `!(var = ...)` shape, and
// folding them the other way would have deleted the branch that actually runs.
//
// fold_never (condition always FALSE)   6327 6344 6416 6423 6768 27758 27768
// fold_always (condition always TRUE)   6531 7712 7741
// dies with its guard                   15497 15512 (has_textchanged*), 21638
// (has_cmdundefined)
// value consumed                        7725 (did_cmd), 18098 (ins_apply_autocmds
// -> return FALSE), 86559 (drop the |= term)
// bare statements                       60 lines, deleted
//
// WHAT GOES BY CASCADE: the EVENT_ enum (123 enumerators, 127 lines), event_tab
// (127 rows), event_nr2name, auto_next_pat, AutoPat, AutoCmd, AutoPatCmd_T,
// active_apc_list, first_autopat, last_autopat, au_need_clean, autocmd_blocked,
// aucmd_prepbuf, aucmd_restbuf and aco_save_T.  Nothing here deletes those by name.
//
// SCOPE NOTE: trigger_cmd_autocmd() comes out HERE rather than with the other empty
// functions, because its call sites pass EVENT_* constants that this phase removes.
// may_trigger_modechanged() takes no argument and waits for the combined phase.
//
// THE DELTA: none expected.  Nothing could fire an autocommand, so removing the
// dispatch cannot change what the editor does.  Declared empty, left for the delta check.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the proof is asked of the
// edges -- first_autopat's uses, none of them a store -- and the text
// version's literals, heads and lines are acts on the nodes: two literals
// made nodes (FRAG), folds, a body, runs and statements cut, the bare
// dispatches found by their callee; each counted, its report the text's
// (history keeps the text version).

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

var (
	applyAutocmds = regexp.MustCompile(`^apply_autocmds\w*$`)
	insEvent      = regexp.MustCompile(`^EVENT_[A-Z]+$`)
)

// isStore says the use u is written: the left side of an assignment, of
// itself or of an element of it.
func isStore(e *graph.Editor, u *graph.Node) bool {
	at := u
	if p := e.Parent(at); p != nil && p.Is("index") && p.Kids[1] == at {
		at = p
	}
	p := e.Parent(at)
	if p == nil || len(p.Kids) < 2 || p.Kids[1] != at {
		return false
	}
	switch p.Head() {
	case "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=":
		return true
	}
	return false
}

// cutStatements deletes the n statements -- items -- that are a call of a
// function whose name re matches (cast to void where void says so), whose
// arguments ok accepts, as one act.
func cutStatements(v *graph.Verbs, re *regexp.Regexp, void bool, ok func([]*graph.Node) bool, n int, what string) {
	if v.Failed() {
		return
	}
	e := v.Editor()
	var ms []*graph.Node
	for _, c := range v.Find("(call _*)") {
		f := c.Kids[1]
		if f.IsList() || !re.MatchString(f.Atom) || !ok(c.Args()[1:]) {
			continue
		}
		at := c
		if p := e.Parent(c); void && p != nil && p.Is("cast") && len(p.Kids) == 3 && !p.Kids[1].IsList() && p.Kids[1].Atom == "void" {
			at = p
		}
		if e.Item(at) == at {
			ms = append(ms, at)
		}
	}
	if len(ms) != n {
		v.Die("%s -- matched %d times, expected %d", what, len(ms), n)
		return
	}
	for _, m := range ms {
		if err := e.Delete(m); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
	}
	v.Say(what)
}

func anyArgs([]*graph.Node) bool { return true }

// Whim5d removes the autocommand dispatch, having first PROVED that nothing can
// register one.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noautocmd", e, w)

	d := v.One("(def static first_autopat (array NUM_EVENTS (ptr AutoPat)) (init nullptr))",
		"the first_autopat declaration is where this phase expects it")
	if d != nil {
		ws := 0
		for _, u := range e.Uses(d) {
			if isStore(e, u) {
				ws++
			}
		}
		v.Expect(ws == 0, "first_autopat is assigned in %d place(s), not just its all-nullptr initialiser -- an autocommand CAN be registered and this whole phase is wrong", ws)
	}
	if v.Failed() {
		return v.Done()
	}
	v.Say("confirmed: first_autopat is only ever the all-nullptr initialiser")

	v.InFunction("close_buffer", func(v *graph.Verbs) {
		v.LiteralC(w5dOldCb, w5dNewCb, 1, "close_buffer, whose abort label three gotos still target")
	})
	v.InFunction("buf_freeall", func(v *graph.Verbs) {
		v.FoldNever("(&& (call apply_autocmds EVENT_BUFUNLOAD _*) _*)", 1, "unloading a buffer asking the autocommands first")
		v.FoldNever("(&& (call apply_autocmds EVENT_BUFWIPEOUT _*) _*)", 1, "wiping a buffer asking them")
	})
	v.InFunction("buflist_new", func(v *graph.Verbs) {
		v.FoldNever("(&& (call apply_autocmds EVENT_BUFNEW _*) _*)", 1, "a new buffer announcing itself")
	})
	// readfile went at phase 1 (readfront, phase 31's move)
	// set_curbuf went with ex_quit's refusal at phase 1 (quitfront, phase 33's move)
	v.InFunction("ins_redraw", func(v *graph.Verbs) {
		v.FoldNever("(&& ready (call has_textchangedI) _*)", 1, "insert mode reporting a change")
		v.FoldNever("(&& ready (call has_textchangedP) _*)", 1, "and the popup-menu variant")
	})
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.FoldNever("(&& (!= p nullptr) (== (. ea cmdidx) CMD_SIZE) (! (. ea skip)) _ (call has_cmdundefined))", 1,
			"an unknown command being defined by an autocommand")
	})
	v.Body("ins_apply_autocmds", "(return FALSE)", "ins_apply_autocmds, which dispatched and watched the tick")
	v.InFunction("ui_focus_change", func(v *graph.Verbs) {
		v.Cut("(|= need_redraw (call apply_autocmds (? in_focus EVENT_FOCUSGAINED EVENT_FOCUSLOST) nullptr nullptr FALSE curbuf))", 1,
			"a focus change telling the autocommands")
	})
	// the modelines are still applied there: phase 16 (oneoptset), which
	// takes them, runs after this program now (whim5d runs at phase 5)
	v.InFunction("open_buffer", func(v *graph.Verbs) {
		v.LiteralC(w5dlit6, w5dlit7, 1, "open_buffer, keeping the flag clearing the autocmd call was wrapped around")
	})
	// buf_write, with its autocommands, went with :write at phase 1
	// (filefront, the reform's D4)
	v.InFunction("set_termname", func(v *graph.Verbs) {
		v.Cut("(if (!= (. (-> curbuf b_ml) ml_mfp) nullptr) _*)", 1, "a new terminal telling every buffer")
	})
	cutStatements(v, regexp.MustCompile(`^ins_apply_autocmds$`), false, func(a []*graph.Node) bool {
		return len(a) == 1 && !a[0].IsList() && insEvent.MatchString(a[0].Atom)
	}, 6, "the insert-mode dispatches")
	v.InFunction("ins_redraw", func(v *graph.Verbs) {
		v.FoldNever("(&& ready (paren (call has_cursormovedI)) _*)", 1, "insert mode reporting the cursor moved")
	})
	v.InFunction("free_buffer", func(v *graph.Verbs) {
		v.Cut("(call aubuflocal_remove buf)", 1, "a freed buffer detaching its buffer-local patterns")
	})
	v.InFunction("getout", func(v *graph.Verbs) {
		for _, ev := range []string{"VIMLEAVEPRE", "VIMLEAVE"} {
			v.CutRun(fmt.Sprintf("quitting unblocking autocommands to announce EVENT_%s", ev),
				"(if (call is_autocmd_blocked) (block (call unblock_autocmds) (pre++ unblock)))",
				"(call apply_autocmds EVENT_"+ev+" nullptr nullptr FALSE curbuf)",
				"(if unblock (block (call block_autocmds)))")
		}
	})
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.DropOperand("(! (call getline_equal fgetline cookie getnextac))", 1, "asking whether the command came from an autocommand")
	})
	// the command-line type: its one write goes, and the collection takes
	// the declaration
	v.InFunction("getcmdline_int", func(v *graph.Verbs) {
		v.Cut("(= cmdline_type (? (== firstc NUL) '-' firstc))", 1, "the line that set the command-line type")
	})
	// 43: six more were in the write and read paths, gone at phase 1 (D4),
	// and seven in the buffer and window switching :q's refusal reached
	// (quitfront, phase 33's move), and eleven in the read path (readfront, phase
	// 31's move).  Run at phase 5, it finds 18 that the phases between took
	// first when it ran as phase 75 of the old numbering: buf_write()'s 8, set_rw_fname()'s 4, set_buflisted()'s
	// and enter_buffer()'s 2 each, do_ecmd()'s third and do_filetype_autocmd()'s
	cutStatements(v, applyAutocmds, true, anyArgs, 43, "every remaining bare dispatch (43)")
	cutStatements(v, regexp.MustCompile(`^trigger_cmd_autocmd$`), false, anyArgs, 7, "the command-line triggers (7)")
	v.InFunction("set_termname", func(v *graph.Verbs) {
		v.DropBareBlock("(= buf curbuf)", "the husk the terminal notification left behind")
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim5d", Edit) }
