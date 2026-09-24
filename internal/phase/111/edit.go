package p111

// Whim phase 111 -- the scalar clock.
// See GOALS.md II.4c, GOALS.md, and internal/phase/109/edit.go, which created the thing
// this phase retires.
//
// WHAT THE CORE DOES WITH TIME, read out of the input rather than remembered: it STAMPS
// NOW and later ASKS HOW MANY MILLISECONDS HAVE PASSED.  That is the whole of it, at
// four places -- do_sleep's `done < msec` loop, vim_beep's 500 ms rate limit,
// handle_osc's `>= p_ost` timeout and inchar_loop's `wtime - elapsed_time` deadline --
// and NOT ONE of the four ever reads a field, prints a reading or compares two stamps
// for equality.  So the core never needs the LAYOUT of a clock, only a scalar.
//
// WHAT GOES, all of it phase 109's:
//
// elapsed_T       8 mentions.  A core-owned TAGLESS `struct { long tv_sec; long
// tv_usec; }` -- phase 109's mirror of <sys/time.h>'s struct timeval,
// whose layout that phase had to static_assert equal.  The core
// stops modelling a host structure: the four objects become `long`.
// elapsed()       6 mentions.  Its entire body is one clock read and a subtraction to
// milliseconds; with a scalar clock the subtraction is the caller's
// one operator and the function has nothing left to do.
// musl_gettimeofday(long *, long *)
// 7 mentions.  The host call phase 109 introduced, an out-parameter
// pair because a struct could not cross.  It becomes
// `long musl_now_ms(void)`: a value, not two stores.
//
// WHAT IT EARNS.  Read all thirteen core -> host signatures and the sentence is true:
// every one takes scalars and byte buffers only.  IT WAS ALREADY TRUE AT q110 -- phase 109
// chose `long *, long *` precisely so that `struct timeval` would not cross -- so this
// phase does not earn THAT sentence and the check does not claim it.  What it earns is
// the narrower one that was false until now: the core no longer DECLARES a type shaped
// like a libc struct, and its whole notion of time is one `long`.  `struct timeval`,
// `elapsed_T`, `elapsed` and `musl_gettimeofday` are all at 0 above the boundary
// afterwards; the host keeps its own `struct timeval`, for `select` and for the one
// `gettimeofday` call that is left in the file.
//
// WHAT `musl_now_ms()` RETURNS, AND IT IS A DECISION, NOT A DETAIL.  Milliseconds since
// a MONOTONIC ORIGIN the host chooses -- the whole second of its first call -- and not
// milliseconds since the epoch.  Every core use is a DIFFERENCE, so the origin is free,
// and the two choices are measured against each other in the check:
//
// epoch      tv_sec * 1000 is ~1.79e12 today.  On a target where `long` is 64 bits
// that is nothing; on one where it is 32 bits it overflows ON THE FIRST
// CALL, every call, for ever -- signed overflow, so the standard gives no
// value at all and -fwrapv gives a wrong one.  The clock is broken from
// the moment the editor starts.
// monotonic  (tv_sec - base) * 1000 overflows a 32-bit `long` after 2**31 ms, which
// is 24.86 DAYS of editor uptime.  Until then every reading is exact.
//
// So the origin moves the 32-bit failure from "immediately" to "after 24.86 days", and
// it costs one branch and two statics IN THE HOST -- which is the part a new target
// rewrites anyway.  The base is LAZY rather than set in musl_host_init(), because an
// ordering dependency on another host function is exactly the kind of thing a host
// rewrite breaks silently, and the symptom would be base 0, epoch milliseconds and the
// overflow above.
//
// THE ORIGIN IS A WHOLE SECOND ON PURPOSE.  With `base` a second and no microsecond
// part, musl_now_ms() is epoch-milliseconds less the constant base*1000, so a
// DIFFERENCE of two readings is IDENTICAL under the two choices.  The origin is
// therefore behaviourally invisible, which is what lets the check measure the rounding
// ONCE instead of twice, and what lets it require the epoch variant's recording to be
// byte-identical to the product's.
//
// PRECISION IS NOT LOST AND THE ROUNDING POINT MOVES.  elapsed() subtracted and THEN
// divided: `(now.tv_sec - start.tv_sec) * 1000 + (now.tv_usec - start.tv_usec) / 1000`.
// musl_now_ms() divides at each reading and the caller subtracts.  Microseconds were
// already discarded either way -- the phase loses no precision the input had -- but the
// two roundings are not the same function, and MEASURED over 20,000,000 random pairs
// the difference is EXACTLY +-1 ms, both ways, never more, with the two formulas equally
// far from the true elapsed time (+0.4995 ms and +0.4996 ms mean).  The check runs that
// measurement rather than repeating it.  Whether a caller can SEE 1 ms is a separate
// question and the check answers it at each of the four.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"bytes"
	"github.com/arbace/go-whim/internal/cutil"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim111", Edit) }

var (
	whim111Inc   = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
	whim111Timev = regexp.MustCompile(`struct timeval\b`)
	whim111Names = regexp.MustCompile(`\b(?:elapsed_T|elapsed|now_tv|start_tv|musl_gettimeofday` +
		`|gettimeofday|musl_now_ms)\b|struct timeval\b`)
)

const whim111HostMark = "static volatile sig_atomic_t host_winch_pending"

// whim111Once replaces text that must occur exactly once, with this phase's own
// refusal: the anchor truncated to seventy characters with its newlines shown.
func whim111Once(p edit.Ph, text []byte, old, new, why string) ([]byte, error) {
	if k := cutil.CountAnchorB(text, old); k != 1 {
		shown := strings.ReplaceAll(old, "\n", `\n`)
		if len(shown) > 70 {
			shown = shown[:70]
		}
		return nil, p.Die("`%s` is not in the file exactly once (%s)", shown, why)
	}
	return cutil.ReplaceAnchorB(text, old, []byte(new), 1), nil
}

// Whim111 makes the clock a scalar: `long musl_now_ms(void)` replaces
// `void musl_gettimeofday(long *, long *)` and takes elapsed_T, elapsed() and
// the Out-parameter pair with it.
//
// THE POINT IS THE SIGNATURE AND NOT THE SAVING: after this phase no core ->
// host call's shape is decided by a type the core cannot name.  Phase 109 had
// to invent a tagless `struct timeval` mirror for the core to hold; this
// deletes it.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "clock", W: w}
	lines := bytes.Split(text, []byte{'\n'})
	var err error

	// ---- 0. the file this edit was written against -----------------------
	// ELEVEN DIRECTIVES, contiguous, and NOTHING ABOVE THEM.  Phase 110 made
	// the first of them the boundary; this phase edits both sides of that
	// line and must know exactly where it is.
	var directives []int
	for i, l := range lines {
		if bytes.HasPrefix(bytes.TrimLeft(l, " \t"), []byte("#")) {
			directives = append(directives, i)
		}
	}
	if len(directives) != 11 {
		return nil, p.Die("the file has %d preprocessor directives and this phase was written against 11",
			len(directives))
	}
	for k, i := range directives {
		if i != directives[0]+k {
			var at []string
			for _, d := range directives {
				at = append(at, strconv.Itoa(d+1))
			}
			return nil, p.Die("the eleven directives are not contiguous: %s", strings.Join(at, " "))
		}
	}
	for _, i := range directives {
		if !whim111Inc.Match(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	cut := directives[0]
	core := bytes.Join(lines[:cut], []byte{'\n'})
	below := bytes.Join(lines[cut:], []byte{'\n'})
	p.Sayf("eleven `#include`s, contiguous, at lines %d-%d, and NOTHING above the first of "+
		"them -- so the core is the %d lines above the boundary and the host is the %d "+
		"below it", cut+1, cut+11, cut, len(lines)-cut-1)

	// ---- 1. the host region, which is where the new definition has to land
	// `zhostonly` reads the host as the lines from `host_winch_pending` to
	// musl_suspend()'s last brace, and requires every mention of its
	// vocabulary -- `struct timeval` among them -- to be inside it.
	hb := 0
	for _, l := range lines {
		if bytes.HasPrefix(l, []byte(whim111HostMark)) {
			hb++
		}
	}
	if hb != 1 {
		return nil, p.Die("the host block does not begin exactly once with %s -- found %d",
			"'"+whim111HostMark+"'", hb)
	}

	// ---- 2. the inventory, counted here rather than remembered -----------
	for _, inv := range []struct {
		Name          string
		ncore, nbelow int
	}{
		{"elapsed_T", 8, 0}, {"elapsed", 6, 0}, {"elapsed_time", 3, 0},
		{"now_tv", 5, 0}, {"start_tv", 20, 0}, {"musl_gettimeofday", 6, 1},
		{"gettimeofday", 0, 1}, {"musl_now_ms", 0, 0},
	} {
		gc := p.Mentions(core, inv.Name)
		gb := p.Mentions(below, inv.Name)
		if gc != inv.ncore || gb != inv.nbelow {
			return nil, p.Die("`%s` occurs %d times above the boundary and %d below it, where this phase "+
				"was written against %d and %d", inv.Name, gc, gb, inv.ncore, inv.nbelow)
		}
	}
	tvCore := len(whim111Timev.FindAll(core, -1))
	tvBelow := len(whim111Timev.FindAll(below, -1))
	if tvCore != 0 || tvBelow != 3 {
		return nil, p.Die("`struct timeval` occurs %d times above the boundary and %d below, where this "+
			"phase was written against 0 and 3", tvCore, tvBelow)
	}
	p.Say("the core's clock is elapsed_T 8, elapsed 6, musl_gettimeofday 6 and `struct " +
		"timeval` 0 -- phase 109 took the last of those; the host has musl_gettimeofday's " +
		"definition, its one real `gettimeofday` call and three `struct timeval`")

	// ---- 3. the literals -------------------------------------------------
	// Phase 106 was caught Out by three string literals holding `NULL`, and
	// phase 109 applied the lesson rather than assuming it.  So does this one.
	spans, err := edit.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, s := range spans {
		if whim111Names.Match(text[s[0]:s[1]]) {
			bad = append(bad, string(text[s[0]:s[1]]))
		}
	}
	if len(bad) > 0 {
		return nil, p.Die("a literal holds a name this phase substitutes: %s", strings.Join(bad, " / "))
	}
	p.Sayf("%d string and character literals, NONE holding any of the seven names -- so every "+
		"substitution below is over code", len(spans))

	// ---- 4. the type and the function leave the core ---------------------
	// The typedef, the prototype and ONE of the two blank lines around them.
	// Deleting the five lines alone would leave a run of two blank lines,
	// which no verification tier can see and which the arithmetic at the end
	// refuses.
	text, err = whim111Once(p, text, `
typedef struct {
    long        tv_sec;
    long        tv_usec;
} elapsed_T;

static long elapsed(elapsed_T *start_tv);
`, "", "the tagless struct and the prototype, which phase 109 wrote")
	if err != nil {
		return nil, err
	}
	text, err = whim111Once(p, text, `
    static long
elapsed(elapsed_T *start_tv)
{
    elapsed_T       now_tv;
    musl_gettimeofday(&now_tv.tv_sec, &now_tv.tv_usec);
    return (now_tv.tv_sec - start_tv->tv_sec) * 1000L + (now_tv.tv_usec - start_tv->tv_usec) / 1000L;
}
`, "", "elapsed(), whose entire body is one clock read and one subtraction")
	if err != nil {
		return nil, err
	}
	p.Say("`elapsed_T` and `elapsed()` leave the core: 6 lines of declaration and 7 of " +
		"definition, with one blank line of each pair, so no run of two blank lines is left " +
		"behind")

	// ---- 5. the four objects become `long` -------------------------------
	// The declarator column is kept: this file aligns a declaration block and
	// `long` is five characters shorter than `elapsed_T`.
	for _, s := range []struct{ Old, New, who string }{
		{"    elapsed_T start_tv;\n    musl_gettimeofday(&start_tv.tv_sec, " +
			"&start_tv.tv_usec);\n    if (hide_cursor)",
			"    long start_tv;\n    start_tv = musl_now_ms();\n" +
				"    if (hide_cursor)", "do_sleep"},
		{"        static elapsed_T start_tv;",
			"        static long             start_tv;", "vim_beep"},
		{"    elapsed_T start_tv;\n} oscstate_T;",
			"    long            start_tv;\n} oscstate_T;", "oscstate_T"},
		{"    elapsed_T start_tv;\n    musl_gettimeofday(&start_tv.tv_sec, " +
			"&start_tv.tv_usec);\n    for (;;)",
			"    long start_tv;\n    start_tv = musl_now_ms();\n" +
				"    for (;;)", "inchar_loop"},
	} {
		text, err = whim111Once(p, text, s.Old, s.New, "the clock object in "+s.who)
		if err != nil {
			return nil, err
		}
	}

	// ---- 6. the remaining stamps and every difference --------------------
	// A stamp is `X = musl_now_ms();` and a reading is `musl_now_ms() - X`.
	// The leading space and the space before the semicolon at these sites are
	// what macro expansion left behind, three pipelines ago.
	for _, s := range []struct{ Old, New, who string }{
		{"            musl_gettimeofday(&start_tv.tv_sec, &start_tv.tv_usec);",
			"            start_tv = musl_now_ms();", "vim_beep's stamp"},
		{"        musl_gettimeofday(&osc_state.start_tv.tv_sec, " +
			"&osc_state.start_tv.tv_usec);",
			"        osc_state.start_tv = musl_now_ms();", "handle_osc's stamp"},
		{"        done = elapsed(&(start_tv));",
			"        done = musl_now_ms() - start_tv;", "do_sleep's reading"},
		{"        if (!did_init || elapsed(&(start_tv)) > 500)",
			"        if (!did_init || musl_now_ms() - start_tv > 500)",
			"vim_beep's 500 ms rate limit"},
		{"    if (elapsed(&(osc_state.start_tv)) >= p_ost)",
			"    if (musl_now_ms() - osc_state.start_tv >= p_ost)",
			"handle_osc's p_ost timeout"},
		{"            elapsed_time = elapsed(&(start_tv));",
			"            elapsed_time = musl_now_ms() - start_tv;",
			"inchar_loop's deadline"},
	} {
		text, err = whim111Once(p, text, s.Old, s.New, s.who)
		if err != nil {
			return nil, err
		}
	}
	p.Say("four stamps -- do_sleep, vim_beep, handle_osc, inchar_loop -- are `X = " +
		"musl_now_ms();`, and the four readings are `musl_now_ms() - X`.  `-` binds tighter " +
		"than `>` and `>=`, so the two comparisons need no parenthesis they did not have")

	// ---- 7. the host call ------------------------------------------------
	text, err = whim111Once(p, text, "static void musl_gettimeofday(long *sec, long *usec);\n",
		"static long musl_now_ms(void);\n", "the host call's prototype")
	if err != nil {
		return nil, err
	}
	text, err = whim111Once(p, text, "static int host_tty_raw = FALSE;\n",
		"static int host_tty_raw = FALSE;\n"+
			"static long host_now_base = 0;\n"+
			"static int host_now_based = FALSE;\n",
		"the host's own state, beside the terminal's")
	if err != nil {
		return nil, err
	}
	text, err = whim111Once(p, text, `    static void
musl_gettimeofday(long *sec, long *usec)
{
    struct timeval tv;

    gettimeofday(&tv, nullptr);
    *sec = tv.tv_sec;
    *usec = tv.tv_usec;
}
`, `    static long
musl_now_ms(void)
{
    struct timeval tv;

    gettimeofday(&tv, nullptr);
    if (!host_now_based)
    {
        host_now_based = TRUE;
        host_now_base = tv.tv_sec;
    }
    return (tv.tv_sec - host_now_base) * 1000L + tv.tv_usec / 1000L;
}
`, "the host call's definition, above musl_delay and inside zhostonly's region")
	if err != nil {
		return nil, err
	}
	p.Say("`long musl_now_ms(void)` replaces `void musl_gettimeofday(long *, long *)`: " +
		"milliseconds since the WHOLE SECOND of its first call, which makes a difference of " +
		"two readings identical to what an epoch-millisecond clock would give and keeps " +
		"(tv_sec - base) * 1000 inside a 32-bit `long` for 24.86 days")

	// ---- 8. what the file is now -----------------------------------------
	// It counted lines -- -7 for the typedef and its prototype and their
	// blanks, -8 for elapsed() -- which is layout, and layout is the
	// canonical print's now.  What the phase removed is asserted by content
	// below: musl_now_ms above and below the boundary.
	L := bytes.Split(text, []byte{'\n'})
	var ndir []int
	for i, l := range L {
		if bytes.HasPrefix(bytes.TrimLeft(l, " \t"), []byte("#")) {
			ndir = append(ndir, i)
		}
	}
	// THE ELEVEN DIRECTIVES ARE STILL ONE BLOCK.  How far it moved up was a
	// count of lines, which is layout; what the core and the host each lost is
	// asserted by content below.
	okDir := len(ndir) == 11
	for k, i := range ndir {
		if i != ndir[0]+k {
			okDir = false
		}
	}
	if !okDir {
		var at []string
		for _, i := range ndir {
			if len(at) < 3 {
				at = append(at, strconv.Itoa(i+1))
			}
		}
		return nil, p.Die("the eleven directives are not eleven contiguous lines: they are at %s",
			strings.Join(at, " "))
	}
	for _, i := range ndir {
		if !whim111Inc.Match(L[i]) {
			return nil, p.Die("a directive is no longer an `#include <...>` of a system header")
		}
	}
	ncore := bytes.Join(L[:ndir[0]], []byte{'\n'})
	nbelow := bytes.Join(L[ndir[0]:], []byte{'\n'})
	for _, name := range []string{"elapsed_T", "elapsed", "now_tv", "musl_gettimeofday"} {
		if n := p.Mentions(text, name); n != 0 {
			return nil, p.Die("`%s` still occurs %d times in the file", name, n)
		}
	}
	if whim111Timev.Match(ncore) {
		return nil, p.Die("`struct timeval` is back above the boundary")
	}
	if n := len(whim111Timev.FindAll(nbelow, -1)); n != 3 {
		return nil, p.Die("the host should still have its three `struct timeval` and has %d", n)
	}
	if n := p.Mentions(text, "gettimeofday"); n != 1 {
		return nil, p.Die("the bare name `gettimeofday` occurs %d times and must occur exactly once -- "+
			"the one call musl_now_ms makes", n)
	}
	if p.Mentions(ncore, "gettimeofday") > 0 {
		return nil, p.Die("the core calls `gettimeofday` directly")
	}
	if n := p.Mentions(ncore, "musl_now_ms"); n != 9 {
		return nil, p.Die("`musl_now_ms` occurs %d times above the boundary where 9 were expected -- the "+
			"prototype, four stamps and four readings", n)
	}
	if n := p.Mentions(nbelow, "musl_now_ms"); n != 1 {
		return nil, p.Die("`musl_now_ms` occurs %d times below the boundary where 1 was expected -- its "+
			"definition", n)
	}
	if n := p.Mentions(text, "elapsed_time"); n != 3 {
		return nil, p.Die("`elapsed_time`, which is inchar_loop's own `long` and not this phase's, is at "+
			"%d and was at 3", n)
	}
	p.Sayf("the core has NO clock type and NO clock call of its own: elapsed_T, elapsed, "+
		"now_tv, musl_gettimeofday and `struct timeval` are all at 0 above the boundary, "+
		"musl_now_ms is 9 there and 1 below, and the file is %d lines against %d",
		len(L)-1, len(lines)-1)
	return text, nil
}
