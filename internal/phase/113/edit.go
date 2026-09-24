package p113

// Whim phase 113 -- the message fold: msg_puts_printf() and the branch that reaches it.
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
// WHICH KIND OF DEAD THIS IS, AND IT IS NOT PHASE 92'S.  Phase 92 removed code that
// COULD NOT RUN; this removes code that CAN run and never does.  `msg_use_printf()`
// returns TRUE 23 times in a single recording -- once per `mainerr` row of
// ref-argv.txt -- so the predicate is alive; it is never TRUE at THIS call site.  That
// is phase 95's kind, and phase 95's evidence is what is owed: an instrument at the
// site, a control that proves the instrument works, and probes that try hard to make
// it fire.  internal/phase/113/check.go has all three -- the input built twice with
// `write(2, "PP-ENTERED\n", 11)`, first in `msg_puts_printf()` (0 of 106 records) and
// then in `msg_puts_display()` (103 of 106, the identical instrument), plus 32 stream
// probes and 4 deadly-signal probes.
//
// WHY THE MESSAGE IS KEPT RATHER THAN DROPPED.  Deleting the arm's body outright is
// five lines smaller and records identically, and it was REJECTED: a phase about
// removing dead CODE must not quietly remove a CAPABILITY.  `host_message()` is
// already the core's declared way to speak when there is no screen (phase 104 wrote it,
// phase 108 made it a direct call), and `host_message(msg, len, err)` treats `len < 0`
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
// 2. `exit_scroll()`'s printf arm IS ALIVE, and phase 104 was wrong to name it a
// follow-up beside `msg_puts_printf()`.  internal/phase/104/check.go says the two "fire in
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

import (
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim113", Edit) }

var whim113Directive = regexp.MustCompile(`^ *#`)
var whim113Include = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)

// whim113Old is the four-line block at msg_puts_attr_len().
//
// THE ONE-LINE FORM IS NOT AN ANCHOR: `    if (msg_use_printf())` at four
// spaces is a substring of the same line at eight, so it counts 3 where the
// block counts 1.
const whim113Old = `    if (msg_use_printf())
    {
        msg_puts_printf((char_u *)str, maxlen);
    }
`

const whim113New = `    if (msg_use_printf())
    {
        host_message((char *)str, maxlen, !info_message);
        msg_didout = TRUE;
    }
`

// Whim113 folds msg_puts_attr_len()'s never-taken arm into one host_message()
// call.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 167 drops the unused)
	p := edit.Ph{Tag: "msgfold", W: w}
	linesBefore := p.Lines(text)

	// ---- 0. the file this edit was written against -----------------------
	// ELEVEN DIRECTIVES AND NONE ABOVE THE BOUNDARY.  Phase 110 made the first
	// `#include` the core -> host boundary; this phase adds no directive,
	// removes none and moves none.
	lines := bytes.Split(text, []byte{'\n'})
	var directives []int
	for i, l := range lines {
		if whim113Directive.Match(l) {
			directives = append(directives, i)
		}
	}
	consecutive := len(directives) > 0
	for k, i := range directives {
		if i != directives[0]+k {
			consecutive = false
		}
	}
	if len(directives) != nInc || !consecutive {
		var first []string
		for _, i := range directives {
			if len(first) < 4 {
				first = append(first, strconv.Itoa(i+1))
			}
		}
		return nil, p.Die("the file does not have exactly eleven preprocessor directives on eleven "+
			"consecutive lines: %d at %s", len(directives), strings.Join(first, " "))
	}
	for _, i := range directives {
		if !whim113Include.Match(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	p.Sayf("eleven directives, every one an `#include <...>`, on lines %d-%d -- the first of "+
		"them is the boundary and this phase writes nothing above it but C",
		directives[0]+1, directives[len(directives)-1]+1)

	// ---- 1. the inventory, counted here rather than remembered -----------
	for _, inv := range []struct {
		Name string
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
	} {
		if got := p.Mentions(text, inv.Name); got != inv.want {
			return nil, p.Die("`%s` has %d mentions and this phase was written against %d -- %s",
				inv.Name, got, inv.want, inv.why)
		}
	}
	p.Say("the inventory, and the FOUR call sites are four: msg_use_printf 6, " +
		"msg_puts_printf 3, vim_strlen_maxlen 3, msg_puts_display 4, host_message 10, " +
		"info_message 9, msg_didout 29")

	// ---- 2. the three sites this phase does NOT touch, asserted VERBATIM -
	// Two of them are folds that were measured to be WRONG and one is a live
	// guard.  They are named so a later edit cannot quietly widen this phase
	// into them.
	keep := []struct {
		who, s string
		want   int
		why    string
	}{
		{"hit_return_msg", "    if (!msg_use_printf())\n", 1,
			"the live guard -- and it is written with a `!`, which is why an edit that " +
				"greps for the positive spelling finds three sites and not four"},
		{"msg_clr_eos_force", "msg_clr_eos_force(void)\n{\n    if (msg_use_printf())\n", 1,
			"CANNOT BE FOLDED SAFELY: the false arm calls screen_fill() with no valid " +
				"screen, which the corpus cannot see and two probes can"},
		{"exit_scroll", "        if (msg_use_printf())\n", 1,
			"ALIVE: its printf arm fires in three of the 32 stream probes and three of " +
				"the four signal probes, with no signal needed for the first three"},
	}
	for _, k := range keep {
		if got := bytes.Count(text, []byte(k.s)); got != k.want {
			return nil, p.Die("the %s site is %d occurrences of %s and must be %d -- %s",
				k.who, got, cutil.PyRepr(k.s), k.want, k.why)
		}
	}
	p.Say("and the THREE sites this phase leaves alone, each asserted verbatim: " +
		"hit_return_msg's `!msg_use_printf()` guard, msg_clr_eos_force's test (folding " +
		"it moves t_ti_stopterm 2,266 -> 2,280 and hup_clean 2,124 -> 2,142) and " +
		"exit_scroll's (its printf arm is ALIVE, and phase 104 named it dead)")

	// ---- 3. the fold, at the one anchor whose count is 1 -----------------
	if got := bytes.Count(text, []byte(whim113Old)); got != 1 {
		return nil, p.Die("the four-line block at msg_puts_attr_len() occurs %d times and must occur "+
			"exactly once.  THE ONE-LINE FORM IS NOT AN ANCHOR: `    if "+
			"(msg_use_printf())` at four spaces is a substring of the same line at eight, "+
			"so it counts 3 where the block counts 1", got)
	}
	text = bytes.Replace(text, []byte(whim113Old), []byte(whim113New), 1)
	p.Say("the fold: msg_puts_attr_len()'s true arm becomes `host_message((char *)str, " +
		"maxlen, !info_message); msg_didout = TRUE;`.  host_message() takes len < 0 as " +
		"strlen and len >= 0 as an exact count, which IS msg_puts_printf's maxlen " +
		"contract -- and the arm is never executed, so equivalence is not claimed: what " +
		"the two lines do not reproduce is the CR-before-NL insertion and the msg_col " +
		"bookkeeping, which no recording or probe can reach")

	// ---- 4. what the file is now -----------------------------------------
	if n := p.Lines(text); n != linesBefore+1 {
		return nil, p.Die("the file is %d lines and the input was %d -- this edit adds exactly one",
			n-1, linesBefore-1)
	}
	if got := p.Mentions(text, "msg_use_printf"); got != 6 {
		return nil, p.Die("`msg_use_printf` is %d mentions after the edit and must still be 6: the test "+
			"stays, and only the arm behind it goes", got)
	}
	if got := p.Mentions(text, "msg_puts_printf"); got != 2 {
		return nil, p.Die("`msg_puts_printf` is %d mentions after the edit and must be 2 -- the "+
			"prototype and the definition, which the SWEEP removes and this edit does not", got)
	}
	for _, k := range keep {
		if bytes.Count(text, []byte(k.s)) != k.want {
			return nil, p.Die("the %s site moved, and this edit touches exactly one site", k.who)
		}
	}
	n := 0
	for _, l := range bytes.Split(text, []byte{'\n'}) {
		if whim113Directive.Match(l) {
			n++
		}
	}
	if n != nInc {
		return nil, p.Die("the file no longer has exactly eleven directives")
	}
	p.Sayf("one line added, msg_use_printf still 6, msg_puts_printf down to 2 -- the "+
		"prototype and the definition, which are the SWEEP's to take along with "+
		"vim_strlen_maxlen and its prototype; eleven directives unmoved and the "+
		"blank-line runs at %d", p.BlankRuns(text))
	return text, nil
}
