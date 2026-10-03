package p044

// Whim phase 44 (formerly 111) -- the scalar clock.
// See GOALS.md II.4c, GOALS.md, and internal/phase/042/edit.go, which created the thing
// this phase retires.
//
// WHAT THE CORE DOES WITH TIME, read out of the input rather than remembered: it STAMPS
// NOW and later ASKS HOW MANY MILLISECONDS HAVE PASSED.  That is the whole of it, at
// four places -- do_sleep's `done < msec` loop, vim_beep's 500 ms rate limit,
// handle_osc's `>= p_ost` timeout and inchar_loop's `wtime - elapsed_time` deadline --
// and NOT ONE of the four ever reads a field, prints a reading or compares two stamps
// for equality.  So the core never needs the LAYOUT of a clock, only a scalar.
//
// WHAT GOES, all of it phase 42's:
//
// elapsed_T       8 mentions.  A core-owned TAGLESS `struct { long tv_sec; long
// tv_usec; }` -- phase 42's mirror of <sys/time.h>'s struct timeval,
// whose layout that phase had to static_assert equal.  The core
// stops modelling a host structure: the four objects become `long`.
// elapsed()       6 mentions.  Its entire body is one clock read and a subtraction to
// milliseconds; with a scalar clock the subtraction is the caller's
// one operator and the function has nothing left to do.
// musl_gettimeofday(long *, long *)
// 7 mentions.  The host call phase 42 introduced, an out-parameter
// pair because a struct could not cross.  It becomes
// `long musl_now_ms(void)`: a value, not two stores.
//
// WHAT IT EARNS.  Read all thirteen core -> host signatures and the sentence is true:
// every one takes scalars and byte buffers only.  IT WAS ALREADY TRUE AT q043 -- phase 42
// chose `long *, long *` precisely so that `struct timeval` would not cross -- so this
// phase does not earn THAT sentence and the check does not claim it.  What it earns is
// the narrower one that was false until now: the core no longer DECLARES a type shaped
// like a libc struct, and its whole notion of time is one `long`.  `struct timeval`,
// `elapsed_T`, `elapsed` and `musl_gettimeofday` are all at 0 above the boundary
// afterwards (the last three once the sweep has taken the uncalled elapsed()); the host keeps its own `struct timeval`, for `select` and for the one
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

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// text program's literals are graph acts, and its report is the text
// version's, which the plan ran until then (history keeps it):
//
//   - the four `start_tv` declarations -- three locals and oscstate_T's
//     member -- are RETYPED `long`, keeping their ids, where the text
//     rewrote their lines;
//   - the stamps and the readings are expressions replaced by C (FRAG),
//     each found by its form: `musl_gettimeofday(&X.tv_sec, &X.tv_usec)`
//     is `X = musl_now_ms()`, `elapsed(&(X))` is `musl_now_ms() - X`;
//   - musl_now_ms's prototype, the host's two objects and its definition
//     are external declarations put beside what they replace or follow,
//     all of it ONE synthesized unit (Together); musl_gettimeofday's
//     prototype and definition are then deleted;
//   - the eleven directives are the include forms, contiguous where the
//     host begins, the core the forms above the first and the host the
//     rest, and the counts above and below the boundary the text's
//     `\bname\b` counts on each side's C view; the literal scan is the
//     text's own, on the file's C view;
//   - the line counts are the text's own layout and are dropped.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim44", Edit) }

var (
	whim44Timev = regexp.MustCompile(`struct timeval\b`)
	whim44Names = regexp.MustCompile(`\b(?:elapsed_T|elapsed|now_tv|start_tv|musl_gettimeofday` +
		`|gettimeofday|musl_now_ms)\b|struct timeval\b`)
)

const whim44Def = `static long
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
}`

// whim44Sides is the core's C view and the host's, refused unless the
// include forms are contiguous where the host begins.
func whim44Sides(v *graph.Verbs) (core, host []byte, nInc int) {
	e := v.Editor()
	incs, hs := e.Includes(), e.Host()
	ok := len(incs) > 0 && len(hs) >= len(incs)
	for i := 0; ok && i < len(incs); i++ {
		ok = hs[i] == incs[i]
	}
	if !ok {
		v.Die("the file's %d include forms are not contiguous where the host begins", len(incs))
		return nil, nil, 0
	}
	c, err := graph.FormsC(e.Core())
	if err != nil {
		v.Die("the core's C view: %v", err)
		return nil, nil, 0
	}
	h, err := graph.FormsC(hs)
	if err != nil {
		v.Die("the host's C view: %v", err)
		return nil, nil, 0
	}
	return c, h, len(incs)
}

// Edit is the scalar clock: musl_now_ms() for musl_gettimeofday() and
// elapsed().
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("clock", e, w)
	core, below, nInc := whim44Sides(v)
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d `#include` forms, contiguous where the host begins, and NOTHING above the first of "+
		"them -- so the core is the %d forms above the boundary and the host the %d from it on",
		nInc, len(e.Core()), len(e.Host()))

	hb := e.Decls("host_winch_pending")
	v.Expect(len(hb) == 1 && e.InHost(hb[0]), "the host block does not begin exactly once with "+
		"host_winch_pending, declared below the boundary -- found %d", len(hb))

	for _, inv := range []struct {
		name          string
		ncore, nbelow int
	}{
		{"elapsed_T", 8, 0}, {"elapsed", 6, 0}, {"elapsed_time", 3, 0},
		{"now_tv", 5, 0}, {"start_tv", 20, 0}, {"musl_gettimeofday", 6, 1},
		{"gettimeofday", 0, 1}, {"musl_now_ms", 0, 0},
	} {
		gc, gb := edit.MentionCount(core, inv.name), edit.MentionCount(below, inv.name)
		v.Expect(gc == inv.ncore && gb == inv.nbelow, "`%s` occurs %d times above the boundary and %d "+
			"below it, where this phase was written against %d and %d", inv.name, gc, gb, inv.ncore, inv.nbelow)
	}
	tvCore, tvBelow := len(whim44Timev.FindAll(core, -1)), len(whim44Timev.FindAll(below, -1))
	v.Expect(tvCore == 0 && tvBelow == 3, "`struct timeval` occurs %d times above the boundary and %d "+
		"below, where this phase was written against 0 and 3", tvCore, tvBelow)
	if v.Failed() {
		return v.Done()
	}
	v.Say("the core's clock is elapsed_T 8, elapsed 6, musl_gettimeofday 6 and `struct " +
		"timeval` 0 -- phase 42 took the last of those; the host has musl_gettimeofday's " +
		"definition, its one real `gettimeofday` call and three `struct timeval`")

	text := v.Text()
	spans, err := edit.LiteralSpans(edit.Ph{Tag: "clock", W: io.Discard}, text)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	var bad []string
	for _, s := range spans {
		if whim44Names.Match(text[s[0]:s[1]]) {
			bad = append(bad, string(text[s[0]:s[1]]))
		}
	}
	v.Expect(len(bad) == 0, "a literal holds a name this phase substitutes: %s", strings.Join(bad, " / "))
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d string and character literals, NONE holding any of the seven names -- so every "+
		"substitution below is over code", len(spans))

	// The acts, quiet: the text reported them in two lines.  The four
	// clock objects are `long` first, so that the C put in below type-checks
	// where it lands.
	q := graph.NewVerbs("clock", e, io.Discard)
	for _, fn := range []string{"do_sleep", "inchar_loop"} {
		q.InFunction(fn, func(q *graph.Verbs) {
			q.Retype("(def start_tv elapsed_T)", "long", "the clock object in "+fn)
		})
	}
	q.InFunction("vim_beep", func(q *graph.Verbs) {
		q.Retype("(def static start_tv elapsed_T)", "long", "the clock object in vim_beep")
	})
	q.Retype("(start_tv elapsed_T)", "long", "the clock object in oscstate_T")
	// Two units: a fragment's names resolve to the graph's declarations,
	// so the declarations go in before what names them.
	q.Together(func(q *graph.Verbs) {
		q.TopBeforeC("musl_gettimeofday", "static long musl_now_ms(void);", "the host call's prototype")
		q.TopAfterC("host_tty_raw", "static long host_now_base = 0;\nstatic int host_now_based = FALSE;",
			"the host's own state, beside the terminal's")
	})
	q.Together(func(q *graph.Verbs) {
		for _, fn := range []string{"do_sleep", "vim_beep", "inchar_loop"} {
			q.InFunction(fn, func(q *graph.Verbs) {
				q.ReplaceC("(call musl_gettimeofday (addr (. ?x tv_sec)) (addr (. ?x tv_usec)))",
					"$x = musl_now_ms();", 1, fn+"'s stamp")
			})
		}
		q.InFunction("handle_osc", func(q *graph.Verbs) {
			q.ReplaceC("(call musl_gettimeofday (addr (. osc_state start_tv tv_sec)) (addr (. osc_state start_tv tv_usec)))",
				"osc_state.start_tv = musl_now_ms();", 1, "handle_osc's stamp")
		})
		for _, fn := range []string{"do_sleep", "vim_beep", "handle_osc", "inchar_loop"} {
			q.InFunction(fn, func(q *graph.Verbs) {
				q.ReplaceC("(call elapsed (addr (paren ?x)))", "musl_now_ms() - $x", 1, fn+"'s reading")
			})
		}
		q.TopAfterC("musl_gettimeofday", whim44Def,
			"the host call's definition, above musl_delay and inside zhostonly's region")
	})
	q.Cut("(def static musl_gettimeofday _*)", 1, "musl_gettimeofday's prototype")
	q.Cut("(defn static musl_gettimeofday _*)", 1, "musl_gettimeofday's definition")
	if q.Failed() {
		return q.Done()
	}
	v.Say("four stamps -- do_sleep, vim_beep, handle_osc, inchar_loop -- are `X = " +
		"musl_now_ms();`, and the four readings are `musl_now_ms() - X`.  `-` binds tighter " +
		"than `>` and `>=`, so the two comparisons need no parenthesis they did not have")
	v.Say("`long musl_now_ms(void)` replaces `void musl_gettimeofday(long *, long *)`: " +
		"milliseconds since the WHOLE SECOND of its first call, which makes a difference of " +
		"two readings identical to what an epoch-millisecond clock would give and keeps " +
		"(tv_sec - base) * 1000 inside a 32-bit `long` for 24.86 days")

	ncore, nbelow, n := whim44Sides(v)
	if v.Failed() {
		return v.Done()
	}
	v.Expect(n == nInc, "the %d include forms are %d now", nInc, n)
	text = v.Text()
	for _, r := range []struct {
		name string
		want int
	}{{"elapsed_T", 4}, {"elapsed", 2}, {"now_tv", 5}, {"musl_gettimeofday", 1}} {
		got := edit.MentionCount(text, r.name)
		v.Expect(got == r.want, "`%s` occurs %d times in the file, where %d were expected -- the uncalled "+
			"elapsed() and its type, which the collection takes", r.name, got, r.want)
	}
	v.Expect(len(v.UsesOf("elapsed")) == 0, "elapsed() is still called")
	v.Expect(!whim44Timev.Match(ncore), "`struct timeval` is back above the boundary")
	k := len(whim44Timev.FindAll(nbelow, -1))
	v.Expect(k == 3, "the host should still have its three `struct timeval` and has %d", k)
	k = edit.MentionCount(text, "gettimeofday")
	v.Expect(k == 1, "the bare name `gettimeofday` occurs %d times and must occur exactly once -- "+
		"the one call musl_now_ms makes", k)
	v.Expect(edit.MentionCount(ncore, "gettimeofday") == 0, "the core calls `gettimeofday` directly")
	k = edit.MentionCount(ncore, "musl_now_ms")
	v.Expect(k == 9, "`musl_now_ms` occurs %d times above the boundary where 9 were expected -- the "+
		"prototype, four stamps and four readings", k)
	k = edit.MentionCount(nbelow, "musl_now_ms")
	v.Expect(k == 1, "`musl_now_ms` occurs %d times below the boundary where 1 was expected -- its "+
		"definition", k)
	k = edit.MentionCount(text, "elapsed_time")
	v.Expect(k == 3, "`elapsed_time`, which is inchar_loop's own `long` and not this phase's, is at "+
		"%d and was at 3", k)
	if v.Failed() {
		return v.Done()
	}
	v.Say("the core has NO clock type and NO clock call of its own once the collection has " +
		"taken the uncalled elapsed(): `struct timeval` is at 0 above the boundary, " +
		"musl_now_ms is 9 there and 1 below")
	return v.Done()
}
