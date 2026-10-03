package p046

// Whim phase 46 (formerly 113) -- the message fold: msg_puts_printf() and the branch that reaches it.
// See GOALS.md II.4c, GOALS.md, and .claude/briefs/zero-last-two.md PART ONE.
//
// ONE FOLD, AND ONLY ONE.  `msg_puts_attr_len()` ends in
//
// if (msg_use_printf())  { msg_puts_printf((char_u *)str, maxlen); }
// else                   { msg_puts_display((char_u *)str, maxlen, attr, FALSE); }
//
// and the true arm is never taken.  The test STAYS -- `msg_use_printf()` is a live
// predicate and this phase leaves it at six mentions -- and the arm becomes two lines
// that speak to the host directly:
//
// host_message((char *)str, maxlen, !info_message);
// msg_didout = TRUE;
//
// With it go `msg_puts_printf()` (75 lines), its prototype, and `vim_strlen_maxlen()`
// and its prototype, which the sweep finds because that function held its only call.
//
// WHICH KIND OF DEAD THIS IS, AND IT IS NOT PHASE 31'S.  Phase 31 removed code that
// COULD NOT RUN; this removes code that CAN run and never does.  `msg_use_printf()`
// returns TRUE 23 times in a single recording -- once per `mainerr` row of
// ref-argv.txt -- so the predicate is alive; it is never TRUE at THIS call site.  That
// is phase 34's kind, and phase 34's evidence is what is owed: an instrument at the
// site, a control that proves the instrument works, and probes that try hard to make
// it fire.  internal/phase/046/check.go has all three -- the input built twice with
// `write(2, "PP-ENTERED\n", 11)`, first in `msg_puts_printf()` (0 of 106 records) and
// then in `msg_puts_display()` (103 of 106, the identical instrument), plus 32 stream
// probes and 4 deadly-signal probes.
//
// WHY THE MESSAGE IS KEPT RATHER THAN DROPPED.  Deleting the arm's body outright is
// five lines smaller and records identically, and it was REJECTED: a phase about
// removing dead CODE must not quietly remove a CAPABILITY.  `host_message()` is
// already the core's declared way to speak when there is no screen (phase 40 wrote it,
// phase 41 made it a direct call), and `host_message(msg, len, err)` treats `len < 0`
// as `strlen` and `len >= 0` as an exact count -- which is `msg_puts_printf`'s own
// `maxlen` contract, MEASURED by reading both.  The call is never executed, so
// "exactly equivalent" is not claimed: what the two lines do not reproduce is the
// CR-before-NL insertion and the `msg_col` bookkeeping, neither of which any recording
// or probe can reach.
//
// ------------------------------------------------------------------------------------
// TWO FOLDS THAT LOOK LIKE THIS ONE AND ARE NOT DONE.  Each is a RESULT of this phase
// and not an omission, and each was measured on a binary built for the purpose.
//
// 1. `msg_clr_eos_force()`'s test CANNOT BE FOLDED SAFELY, and this is where a
// corpus-only check would ship a bug.  Its true arm writes `t_CD`/`t_CE`; its false
// arm calls `screen_fill()` twice.  Folding to the false arm leaves the RECORDING
// BYTE-IDENTICAL -- `screen_fill()` returns early on `ScreenLines == nullptr`, and
// `ScreenLines` is NULL in all 23 `mainerr` cases, which are the only 23 places the
// predicate is TRUE in a recording.  Two probes see it and nothing else does:
// `t_ti_stopterm` (`:set t_ti=X`, `:set t_te=Y`, `ZQ`) goes 2,266 -> 2,280 bytes,
// and `hup_clean` (`kill -HUP` on a clean pty) goes 2,124 -> 2,142, the extra
// eighteen being `\x1b[24;63H\x1b[K\x1b[24;1H` AFTER `Vim: Finished.` -- the editor
// erasing the last line of a screen it has just declared unusable, on its way out.
// GUARDING WITH `msg_check_screen()` INSTEAD IS NOT EQUIVALENT: that drops the
// `swapping_screen() && !termcap_active` disjunct, and that disjunct is exactly what
// `t_ti_stopterm` reaches -- measured, it moves the same two probes, to the same two
// numbers.  The check builds both of those and requires them to move.
//
// 2. `exit_scroll()`'s printf arm IS ALIVE, and phase 40 was wrong to name it a
// follow-up beside `msg_puts_printf()`.  internal/phase/040/check.go says the two "fire in
// ZERO of 106 records"; that is true of the CORPUS and true of the editor only for
// the first.  MEASURED: the arm fires with no signal at all in three of this phase's
// 32 stream probes -- `t_ti_more`, `debug_more` and `term_ti_then_ti`, each `:set
// t_ti=X` (or `-T debug`) plus a paged `:set all` plus exit -- and in three of the
// four deadly-signal probes.  Folding it to `out_char('\n')` is not a crash risk:
// `out_char('\n')` emits `\r` first, so the bytes are the same two.  It moves them
// from FD 2 TO FD 1, which on a pty where both are the same device is invisible and
// therefore undeclarable.  It belongs to whichever phase decides the core writes
// nothing to fd 2 at all -- GOALS.md II.4c's host-boundary question, not a tidy-up.
// The check builds that fold too and requires it to move three stream probes and
// three signal probes, which is the evidence that THIS phase did not disturb it.
//
// ------------------------------------------------------------------------------------
// THE COUNTING TRAP, MEASURED.  `    if (msg_use_printf())` at four spaces is a
// SUBSTRING of the same line at eight: `str.count()` says 3 where
// `grep -c '^    if (msg_use_printf())$'` says 2, the third match being
// `exit_scroll`'s, indented eight.  So the anchor is the whole four-line block, whose
// count is 1.  And there are FOUR call sites, not three: the fourth is written
// `if (!msg_use_printf())` in `hit_return_msg()` and an edit that greps for the
// positive spelling misses it.  All three sites this phase does not touch are asserted
// present VERBATIM below, before and after.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).
//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// fold is one LiteralC on the program's graph -- the call item in
// msg_puts_attr_len()'s true arm made two items, written at their place
// and resolved as the importer resolves them -- and its report is the text
// version's, which the plan ran until then (history keeps it):
//
//   - the eleven directives are the include forms, contiguous where the
//     host begins, nothing above the first but C;
//   - the inventory is the text's `\bname\b` counts on the C view (TEXTQ's
//     Mentions), the file's view taken once;
//   - the FOUR call sites, and the three this phase leaves alone, are the
//     uses of msg_use_printf by edge, each told by its function and its
//     shape -- hit_return_msg's under a `!`, msg_clr_eos_force's its body's
//     first item, exit_scroll's an if -- where the text counted indented
//     lines (the counting trap above is a text's, and does not arise);
//   - the line count and the blank-line runs are the text's own layout and
//     are dropped.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim46", Edit) }

// w46Includes is the include forms, refused unless they are contiguous
// where the host begins: the first is the boundary and nothing above it is
// a directive.
func w46Includes(v *graph.Verbs) []*graph.Node {
	e := v.Editor()
	incs, host := e.Includes(), e.Host()
	ok := len(incs) > 0 && len(host) >= len(incs)
	for i := 0; ok && i < len(incs); i++ {
		ok = host[i] == incs[i]
	}
	v.Expect(ok, "the file's %d include forms are not contiguous where the host begins", len(incs))
	return incs
}

// w46Site is the use of msg_use_printf in fn, refused unless there is
// exactly one and shape says yes to it.
func w46Site(v *graph.Verbs, uses []*graph.Node, fn, why string, shape func(call, use *graph.Node) bool) {
	e := v.Editor()
	var in []*graph.Node
	for _, u := range uses {
		if f := e.Function(u); f != nil && graph.DeclName(f) == fn {
			in = append(in, u)
		}
	}
	v.Expect(len(in) == 1 && shape(e.Parent(in[0]), in[0]),
		"the %s site is %d uses of msg_use_printf and must be 1, in its shape -- %s", fn, len(in), why)
}

// Edit is the fold: msg_puts_attr_len()'s true arm speaks to the host.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("msgfold", e, w)
	incs := w46Includes(v)
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d directives, every one an `#include <...>` form, contiguous where the host "+
		"begins -- the first of them is the boundary and this phase writes nothing above it but C",
		len(incs))

	inventory := []struct {
		name string
		want int
		why  string
	}{
		{"msg_use_printf", 6, "a prototype, a definition and FOUR call sites -- this " +
			"phase leaves all six"},
		{"msg_puts_printf", 3, "a prototype, a definition and the one call this phase " +
			"folds away"},
		{"vim_strlen_maxlen", 3, "a prototype, a definition and its ONLY call, which " +
			"is inside msg_puts_printf"},
		{"msg_puts_display", 4, "a prototype, a definition and two calls -- the false " +
			"arm this phase keeps, and one recursive"},
		{"host_message", 10, "a prototype, a definition and eight calls, four of them " +
			"inside msg_puts_printf"},
		{"info_message", 9, "a declaration, its setters and the four reads inside " +
			"msg_puts_printf"},
		{"msg_didout", 29, "one of them msg_puts_printf's last statement, which the " +
			"replacement keeps"},
	}
	text := v.Text()
	for _, inv := range inventory {
		got := edit.MentionCount(text, inv.name)
		v.Expect(got == inv.want, "`%s` has %d mentions and this phase was written against %d -- %s",
			inv.name, got, inv.want, inv.why)
	}
	if v.Failed() {
		return v.Done()
	}
	v.Say("the inventory, and the FOUR call sites are four: msg_use_printf 6, " +
		"msg_puts_printf 3, vim_strlen_maxlen 3, msg_puts_display 4, host_message 10, " +
		"info_message 9, msg_didout 29")

	keep := func() {
		uses := v.UsesOf("msg_use_printf")
		v.Expect(len(uses) == 4, "msg_use_printf has %d uses, where the four call sites were counted", len(uses))
		ifTest := func(call, _ *graph.Node) bool {
			p := e.Parent(call)
			return p != nil && p.Head() == "if" && len(p.Kids) > 1 && p.Kids[1] == call
		}
		w46Site(v, uses, "hit_return_msg", "the live guard, written with a `!`",
			func(call, _ *graph.Node) bool {
				p := e.Parent(call)
				return p != nil && p.Head() == "!" && ifTest(p, nil)
			})
		w46Site(v, uses, "msg_clr_eos_force", "CANNOT BE FOLDED SAFELY: the false arm calls "+
			"screen_fill() with no valid screen, which the corpus cannot see and two probes can",
			func(call, u *graph.Node) bool {
				if !ifTest(call, u) {
					return false
				}
				f := e.Function(u)
				b := graph.Body(f)
				return len(b) > 0 && b[0] == e.Parent(call)
			})
		w46Site(v, uses, "exit_scroll", "ALIVE: its printf arm fires in three of the 32 stream "+
			"probes and three of the four signal probes, with no signal needed for the first three", ifTest)
		w46Site(v, uses, "msg_puts_attr_len", "the arm this phase folds", ifTest)
	}
	keep()
	if v.Failed() {
		return v.Done()
	}
	v.Say("and the THREE sites this phase leaves alone, each asserted by its use and shape: " +
		"hit_return_msg's `!msg_use_printf()` guard, msg_clr_eos_force's test (folding " +
		"it moves t_ti_stopterm 2,266 -> 2,280 and hup_clean 2,124 -> 2,142) and " +
		"exit_scroll's (its printf arm is ALIVE, and phase 40 named it dead)")

	v.InFunction("msg_puts_attr_len", func(v *graph.Verbs) {
		v.CountIs("(if (call msg_use_printf) (block (call msg_puts_printf (cast (ptr char_u) str) maxlen)) _)", 1,
			"the if at msg_puts_attr_len() whose true arm is the one call")
		v.LiteralC("msg_puts_printf((char_u *)str, maxlen);",
			"host_message((char *)str, maxlen, !info_message);\nmsg_didout = TRUE;", 1,
			"the fold: msg_puts_attr_len()'s true arm becomes `host_message((char *)str, "+
				"maxlen, !info_message); msg_didout = TRUE;`.  host_message() takes len < 0 as "+
				"strlen and len >= 0 as an exact count, which IS msg_puts_printf's maxlen "+
				"contract -- and the arm is never executed, so equivalence is not claimed: what "+
				"the two lines do not reproduce is the CR-before-NL insertion and the msg_col "+
				"bookkeeping, which no recording or probe can reach")
	})
	if v.Failed() {
		return v.Done()
	}

	text = v.Text()
	got := edit.MentionCount(text, "msg_use_printf")
	v.Expect(got == 6, "`msg_use_printf` is %d mentions after the edit and must still be 6: the test "+
		"stays, and only the arm behind it goes", got)
	got = edit.MentionCount(text, "msg_puts_printf")
	v.Expect(got == 2, "`msg_puts_printf` is %d mentions after the edit and must be 2 -- the "+
		"prototype and the definition, which the SWEEP removes and this edit does not", got)
	v.Expect(len(v.UsesOf("msg_puts_printf")) == 0, "msg_puts_printf is still called")
	keep()
	after := w46Includes(v)
	v.Expect(len(after) == len(incs), "the file no longer has its %d include forms", len(incs))
	if v.Failed() {
		return v.Done()
	}
	v.Say("msg_use_printf still 6, msg_puts_printf down to 2 -- the prototype and the " +
		"definition, which are the collection's to take along with vim_strlen_maxlen and " +
		"its prototype; the include forms unmoved, and every site but the one folded as it was")
	return v.Done()
}
