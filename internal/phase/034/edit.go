package p034

// Whim phase 34 (formerly 95) -- the options nothing reads.  See GOAL.md.
//
// Phases 28 to 33 took every way to reach a file and then the refusal that guarded
// the text.  What they left behind is a set of SETTINGS: `options[]` rows whose
// global nothing reads any more, so that `:set fsync?` answers a question about
// machinery that is not there.  An option that cannot do anything is a lie, and the
// same argument that removed `:write` removes `'write'`.
//
// WHICH ROWS GO IS COMPUTED, NOT LISTED.  The edit walks `options[]`, finds each
// row's `(char_u *)&p_xx` and counts readers of that global outside the row, with
// `dropoptions --strict`'s own exclusions -- another row, the row's `var`
// field, the variable's own declaration, and taking the address, which is an
// identity test and not a dereference.  Exactly SEVEN of the 114 rows have no
// reader, and the program requires that set rather than naming six of them:
//
// fsync       p_fs       PV_BOTH   goes, but needs droplocal.py b_p_fs first
// modified    p_mod      PV_BUF    STAYS -- see below
// prompt      p_prompt   PV_NONE   goes
// readonly    p_ro       PV_BUF    goes, by GOALS.md II decision 5
// undoreload  p_ur       PV_NONE   goes
// write       p_write    PV_NONE   goes
// writeany    p_wa       PV_NONE   goes
//
// `'modified'` HAS NO READER OF `p_mod` EITHER AND MUST NOT GO.  GOALS.md II.5
// decision 5 keeps it: the state it reports lives in `b_changed`, not in `p_mod`, so
// `:set modified?` answers correctly and the row is not a lie.  A computation that
// took "no reader" as the criterion would delete it, which is why the seven are
// computed and the six are chosen.  `dropoptions` refuses it anyway, on the
// PV_ guard.
//
// `'paste'` IS EXEMPT FOR EVER, and this is the comment that says so -- GOALS.md II.2d
// and II.5 decision 8, the user's standing promise.  `p_paste` has 12 mentions here and
// has them afterwards, and its five save slots `p_ai_nopaste p_et_nopaste
// p_sts_nopaste p_tw_nopaste p_wm_nopaste` are the non-pointer orphans
// `orphanopts` reports and tolerates, before and after, identically.  THE NEXT
// PERSON TO RUN THE COMPUTATION MUST NOT "FIX" THEM.  `+{command}` is likewise
// untouched, for the same promise.
//
// FOUR PARTS.  A and B are the tools' work; C is the only live code here.
//
// A  the four clean rows, `dropoptions --strict prompt undoreload write
// writeany`.  The sweep then takes the four globals as -Wunused-variable.
// B  `'fsync'`, which --strict alone REFUSES -- not on a reader but on the PV_
// guard, because the row is what initialises the global ('tagcase' taught that
// by segfaulting before the first keystroke).  `droplocal.py b_p_fs` is the
// other half and goes first: six plumbing sites, including get_varp()'s two-line
// "local if set" form.  Then `--strict --local fsync`.
// C  `'readonly'`, which is LIVE CODE and not an inert row.  `p_ro` the global has
// had no reader since whim; what survives is the buffer-local `b_p_ro`, and
// since phase 28 nothing but `:set ro` can set it -- decision 5's premise.  Five
// edits, in this order and for this reason:
// 1  the W10 warning.  `change_warning()` and its six call sites, each one
// statement on a line of its own.  There is NO PROTOTYPE -- it is defined
// above its first call -- so a program that removes one fails loudly.  This
// also takes the `ui_delay(1002L, TRUE)` that phase 4e's GOAL.md named as
// one of the eight other pauses.
// 2  the `[RO]` in `fileinfo()`.  THE FORMAT STRING AND THE ARGUMENT MOVE
// TOGETHER -- `%s%s%s%s%s%s` to `%s%s%s%s%s` -- and nothing in the build
// checks a vim_snprintf_safelen count.
// 3  the `[RO]` on the status line, in `win_redr_status()`: the name-padding
// disjunct and the block that appends it.
// 4  `did_set_readonly()`, BY NAME and with the reason: it is the row's
// callback and the row is its only other reference, but droplocal.py runs
// in the same edit and would otherwise find it still reading `b_p_ro`.
// Measured without it: `droplocal: b_p_ro still has 1 mentions after the
// plumbing went`, which is the tool working.  The alternative is an inner
// sweep; this is cheaper and honest.
// 5  the row, then `droplocal.py b_p_ro` -- three plumbing sites.
//
// WHAT THE SWEEP THEN FINDS: `SHM_RO`, `BV_FS`, `BV_RO`, the static string
// `w_readonly` inside change_warning(), the `b_did_warn` field -- which becomes dead
// only after BOTH change_warning and did_set_readonly have gone, so removing one and
// not the other leaves a field with one reader and one writer that no tool reports --
// and the six globals.
//
// THE FLAG LETTERS ARE NOT TOUCHED, AND THAT IS A DECISION.  `'cpoptions'` and
// `'shortmess'` each have a validity list that is a separate string literal from the
// value, so removing a letter from a list cannot move `:set cpo?` or `:set shm?`.
// But `:set shm=F` is accepted silently and `:set shm=y` answers E539, and dropping a
// letter from the list turns the first into the second -- a behaviour change no
// corpus case, Ex row, argv row or pty scenario can see, which is exactly what
// GOALS.md core rule 2 exists to prevent.  Accepting a letter that does nothing is
// what upstream does for every feature a build lacks.  Measured: 23 of 'cpoptions'
// 60 letters and 14 of 'shortmess' 23 are inert here, and THIS PHASE MAKES EXACTLY
// ONE MORE SO -- `'shortmess'`'s `r`, whose SHM_RO the sweep takes with the `[RO]`
// indicator.  The check asserts both literals character for character.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, as every Part II edit since phase 4e does, and the source goes with it as
// $state/old.c.  The check needs both, and needs them more than any phase so far:
// THIS PHASE DECLARES NOTHING, because `:set` is the one thing the core's instrument
// cannot read, and the probes are the whole evidence.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
// ---- A. the four clean rows ----------------------------------------------------------
// --strict is the guard: it refuses a row while anything still reads its global,
// because the row is what INITIALISES that global.  The sweep takes the four
// variables afterwards as -Wunused-variable.
// ---- B. 'fsync', where --strict alone is NOT the guard --------------------------------
// The row is PV_BOTH + PV_BUF + BV_FS, so dropoptions stops on the PV_ guard before
// the reader test is ever reached, and its message talks about a segfault at startup
// rather than about readers.  droplocal.py is the other half and goes first.
// ---- C5. 'readonly': the row, then the field -------------------------------------------

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  Both programs run on the
// program's graph, with the two droplocal steps between them, so the phase
// is graph from end to end; the reports are the text versions' (history
// keeps them, with editlit.go's literal):
//
//   - the anchors and options[]'s rows are the text's own questions on the
//     C view (TEXTQ), the file's and the table's;
//   - the six calls of change_warning are the six statements calling it,
//     and its definition goes by DeleteDefinition, which refuses while
//     anything still refers to it;
//   - the CTRL-G format is its one string literal respelled, and the
//     [RO]/[readonly] argument the one it fed dropped through `...`
//     (DropArgPure: its `shortmess()` and `_()` named as free of side
//     effects, as the text's deletion of them said);
//   - the two `|| b_p_ro` disjuncts are DropOperand, the [RO] block a Cut.

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() {
	phase.RegisterGraph("whim34", Edit)
	// THE SECOND HEREDOC IS A SECOND REGISTRATION, not a tail of the first, and
	// the position is the reason: two `tools/st.sh droplocal` and two
	// `tools/st.sh dropoptions` calls run BETWEEN them, and the counts this one
	// asserts are the counts after those four have run.  Folding the two into
	// one call would move the assertion to before its subject.
	phase.RegisterGraph("whim34rows", Whim34Rows)
}

var w34OptRow = regexp.MustCompile(`(?m)^[ \t]*\{"([a-z]+)",`)

var w34Before = map[string]int{
	"change_warning": 7, "did_set_readonly": 0,
	"b_p_ro": 9, "b_p_fs": 7, "b_did_warn": 3,
	"p_ro": 0, "p_fs": 0, "p_ur": 0, "p_write": 0, "p_wa": 0, "p_prompt": 0,
	"p_mod": 2, "did_set_modified": 3,
	"SHM_RO": 2, "BV_RO": 2, "BV_FS": 3, "w_readonly": 2,
	"p_paste": 12, "read_cmd_fd": 12,
	"vim_fsync": 3, "scriptin": 8, "redir_fd": 0,
}

var w34After = map[string]int{
	"b_p_ro": 0, "b_p_fs": 0, "change_warning": 0, "did_set_readonly": 0,
	"p_ro": 0, "p_fs": 0, "p_ur": 0, "p_write": 0, "p_wa": 0, "p_prompt": 0,
	"b_did_warn": 1, "p_mod": 2, "did_set_modified": 3, "p_paste": 12,
	"read_cmd_fd": 12, "vim_fsync": 3, "scriptin": 8, "redir_fd": 0,
}

var w34Nopaste = []string{"p_ai_nopaste", "p_et_nopaste", "p_sts_nopaste",
	"p_tw_nopaste", "p_wm_nopaste"}

func init() {
	for _, n := range w34Nopaste {
		w34Before[n] = 4
		w34After[n] = 4
	}
}

// w34Rows are the names of options[]'s rows, as the text read them off the
// table's C view.
func w34Rows(v *graph.Verbs) []string {
	var rows []string
	v.InTable("options", func(v *graph.Verbs) {
		for _, m := range w34OptRow.FindAllStringSubmatch(string(v.Text()), -1) {
			rows = append(rows, m[1])
		}
	})
	return rows
}

// Whim34 removes the options nothing reads.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noopts", e, w)

	t := v.Text()
	for _, name := range edit.SortedKeys(w34Before) {
		if k := edit.MentionCount(t, name); k != w34Before[name] {
			v.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w34Before[name])
			return v.Done()
		}
	}
	v.Say("change_warning 7 (a definition and six calls, and no prototype), " +
		"did_set_readonly 3, b_p_ro 10, b_p_fs 7 -- the file the five edits of part C " +
		"were counted against")

	rows := w34Rows(v)
	if v.Failed() {
		return v.Done()
	}
	for _, gone := range []string{"fsync", "prompt", "readonly", "undoreload", "write", "writeany"} {
		if edit.Contains(rows, gone) {
			v.Die("the row for '%s' is still there, and phase 1 drops it", gone)
			return v.Done()
		}
	}
	if !edit.Contains(rows, "modified") || !edit.Contains(rows, "paste") {
		v.Die("'modified' or 'paste' lost its row, and neither may")
		return v.Done()
	}
	v.Say("the six rows nothing read are gone since phase 1; 'modified' and 'paste' keep theirs")

	// ---- C1. the W10 warning
	const w10 = "the W10 warning: six calls to change_warning() and the definition, which takes " +
		"the static string w_readonly and the ui_delay(1002L, TRUE) with it -- GOALS.md " +
		"phase 4e named that as one of the eight other pauses.  It has NO prototype, so a " +
		"program that removed one would fail here"
	if k := v.Count("(call change_warning _*)"); k != 6 {
		v.Die("change_warning has %d call sites, expected 6", k)
		return v.Done()
	}
	v.Muted(func(v *graph.Verbs) {
		v.CutWhere("(call change_warning _*)", func(x *graph.Node) bool { return e.Item(x) == x }, 6, w10)
		v.DeleteDefinition("change_warning", w10)
	})
	if !v.Failed() {
		if k := edit.MentionCount(v.Text(), "change_warning"); k != 0 {
			v.Die("change_warning still has %d mentions; it has no prototype, so six calls "+
				"and a definition is all of it", k)
		}
	}
	v.Say(w10)

	// ---- C2. fileinfo's [RO], and C3. the [RO] on the status line
	v.InFunction("fileinfo", func(v *graph.Verbs) {
		const what = "fileinfo's CTRL-G line loses one %s, and the argument below goes with " +
			"it in the same step -- nothing in the build checks the count"
		c := v.One(`(call vim_snprintf _ _ "\"%s%s%s%s%s%s" (? (call curbufIsChanged) _ _) _*)`, what)
		if c == nil {
			return
		}
		if err := e.RespellString(c.Kids[4], `"\"%s%s%s%s%s"`); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
		v.Say(what)
		const ro = "the [RO]/[readonly] argument itself, which is SHM_RO's only reader"
		arg := -1
		p := clisp.MustPattern(`(? (-> curbuf b_p_ro) (paren (? (call shortmess SHM_RO) (call _ "[RO]") (call _ "[readonly]"))) "")`)
		for i, a := range c.Kids[2:] {
			if graph.Matches(p, a) {
				arg = i
			}
		}
		if arg < 0 {
			v.Die("%s -- not an argument of the call", ro)
			return
		}
		if err := e.DropArgPure(c, arg, "shortmess", "_"); err != nil {
			v.Die("%s -- %v", ro, err)
			return
		}
		v.Say(ro)
		v.DropOperand("(-> curbuf b_p_ro)", 1, "and the trailing-space test's `|| curbuf->b_p_ro` disjunct")
	})
	v.InFunction("win_redr_status", func(v *graph.Verbs) {
		v.DropOperand("(-> wp w_buffer b_p_ro)", 1, "win_redr_status: the name-padding test's `|| b_p_ro` disjunct")
		v.Cut("(if (-> wp w_buffer b_p_ro) (block (+= plen (call vim_snprintf (+ (cast (ptr char) p) plen) (- PATH_MAX plen) \"%s\" (call _ \"[RO]\")))))", 1,
			"and the block that appended [RO] to it -- nothing else reaches that indicator")
	})
	if v.Failed() {
		return v.Done()
	}
	if k := edit.MentionCount(v.Text(), "b_p_ro"); k != 3 {
		v.Die("b_p_ro has %d mentions, expected 3 -- the field, buf_copy_options' write, "+
			"and get_varp's case", k)
		return v.Done()
	}
	v.Say("b_p_ro 10 -> 3, and the three that are left are plumbing: the field, " +
		"buf_copy_options' write and get_varp's case.  droplocal.py is what takes those")
	return v.Done()
}

// Whim34Rows is the second heredoc: what the collection is handed, as a count
// rather than as trust, taken AFTER the droplocal calls between them.
func Whim34Rows(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noopts", e, w)
	t := v.Text()
	for _, name := range edit.SortedKeys(w34After) {
		if k := edit.MentionCount(t, name); k != w34After[name] {
			v.Die("%s has %d mentions after the cut, expected %d", name, k, w34After[name])
			return v.Done()
		}
	}
	rows := w34Rows(v)
	if v.Failed() {
		return v.Done()
	}
	if len(rows) != 107 {
		v.Die("options[] has %d rows, expected 107", len(rows))
		return v.Done()
	}
	for _, gone := range []string{"fsync", "prompt", "readonly", "undoreload", "write", "writeany"} {
		if edit.Contains(rows, gone) {
			v.Die("the row for '%s' is still there", gone)
			return v.Done()
		}
	}
	if !edit.Contains(rows, "modified") || !edit.Contains(rows, "paste") {
		v.Die("'modified' or 'paste' lost its row, and neither may")
		return v.Done()
	}
	v.Say("the cut is done: options[] has its 107 rows, 'modified' and 'paste' " +
		"among them; b_did_warn is a field nothing names, the collection's now")
	return v.Done()
}
