package p048

// Whim phase 48 (formerly 115) -- the clock crosses the boundary.
// See GOALS.md II.4c, GOALS.md, internal/phase/042/edit.go (which wrote the core's libc
// prototypes) and internal/phase/044/edit.go (the scalar clock, whose musl_now_ms is this
// phase's sibling).
//
// THE CORE READS TWO CLOCKS AND ONLY ONE OF THEM HAS CROSSED.  Phase 44 gave the
// elapsed-milliseconds clock to the host as `long musl_now_ms(void)`.  The other one --
// the wall clock, `time(2)`, which the editor stamps a history entry and an undo header
// with -- is still the core's: `time_T vim_time(void) { return time(nullptr); }` at
// five call sites, `long time(long *tp);` in the core's own libc prototype block, and
// TWO MORE CALLS THAT BYPASS THE WRAPPER ALTOGETHER, inside `ui_focus_change()`.  This
// phase is the user's two steps, in order.
//
// STEP ONE   the two standalone `time(nullptr)` in ui_focus_change become `vim_time()`.
// After it, `time(` has exactly ONE call site above the boundary: the one
// inside the wrapper.
// STEP TWO   the wrapper moves below the boundary as `host_time()`, declared in the
// core's host block beside host_exit, host_message and the musl_ set, and
// `long time(long *tp);` leaves the core -- the host takes `time` from
// <time.h>, which is one of the eleven includes.
//
// THE PROTOTYPE IS LOAD-BEARING AND REMOVING IT WOULD SILENTLY REGRESS.  `typedef long
// time_T;` is phase 42's, and it is correct ONLY because `long time(long *tp);` sits
// above <time.h>'s own declaration of the same function: gcc compares the two and says
// `conflicting types for 'time'` if they disagree -- MEASURED by phase 42's `m1`
// control, and measured again here.  Phase 42's sixteen static_asserts were a control in
// ITS check and are NOT in the product (the twelve that survive below the includes are
// phase 43's constants), so once the prototype goes there is nothing left comparing the
// core's width to the host's, and `host_time()` returning a narrower type than `time_t`
// would truncate in silence on a target where the two differ.  So the prototype is
// REPLACED, not merely deleted:
//
// static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");
//
// below the includes, beside the twelve.  internal/phase/048/check.go measures that it holds
// AND that it fails when `time_T` is perturbed to `int` -- a guarantee you cannot break
// is not one.
//
// `host_time()` RETURNS `long`, NOT `time_T`, AND THAT IS A DECISION.  Its DEFINITION is
// below the boundary, and `time_T` is a core typedef declared above it: when the file is
// finally cut at the first `#include` the host half cannot name it.  `musl_now_ms()`
// returns `long` for exactly that reason and this is its sibling, so the two halves of
// the clock cross in the same shape -- and the boundary's stated property, that every
// core -> host signature takes scalars and byte buffers only (internal/phase/044/check.go),
// survives a fourteenth name.  Nothing is converted at any call site: `time_T` IS `long`
// in this file, the check asserts the typedef line itself, and the static_assert above
// pins that `long` to `time_t`.
//
// WHAT DOES NOT CHANGE, AND THE CHECK MEASURES IT RATHER THAN ARGUING IT.
// `ui_focus_change()` reads the clock TWICE, in two separate statements --
// `if (in_focus && last_time + 2 < ...)` and then `last_time = ...` -- and it still
// does: two reads, the same two statements, the same order.  The pair could always
// straddle a second boundary between the test and the store, and it can still, neither
// more nor less often: what changes is the spelling of the read, not how many there are
// or when.  internal/phase/048/check.go instruments every clock read on BOTH binaries and
// requires the same count in the same records, with a control that collapses the two
// reads into one and moves it.
//
// THE ENGLISH WORD `time` IS NOT A CALL.  `op_shift()`'s NGETTEXT strings say "%ld line
// %sed %d time" / "times", and ``zhostonly`` learned the same lesson about
// `"close buffer"`: every count here is taken with string and character literals blanked
// out, and the substitutions are exact multi-line blocks, never a bare `time` -> anything.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// phase's measurements are the text's own, asked of the C view (the counts
// above and below the first `#include`, its literals, the libc prototype
// block), so its report is the text version's; its acts are the graph's,
// and keep their ids where the text MOVED something: ui_focus_change's two
// `time(nullptr)` are FRAG's `vim_time()`; `vim_time` is RENAMEd
// `host_time` by edge (declarations and the five calls, the two new ones
// with them); its prototype MOVEs to the core's host block and its
// definition to the host, between musl_now_ms and musl_delay, and both say
// `long` (RetypeResult); `long time(long *tp);` is deleted, and the one
// call it covered, host_time's, is written again by FRAG below <time.h>, so
// that it refers to the header's `time` as an import of the text after
// does; and the static_assert joins the twelve, FRAG after SIGTERM's.

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim48", Edit) }

var (
	w48Proto    = regexp.MustCompile(`^[A-Za-z_].*\);$`)
	w48Names    = regexp.MustCompile(`\b(?:vim_time|host_time|time_T|time_t)\b`)
	w48Word     = regexp.MustCompile(`\btime\b`)
	w48TimeCall = regexp.MustCompile(`\btime\s*\(`)
	w48VimTime  = regexp.MustCompile(`\bvim_time\b`)
	w48HostTime = regexp.MustCompile(`\bhost_time\b`)
	w48TimeT    = regexp.MustCompile(`\btime_t\b`)
	w48TimeTT   = regexp.MustCompile(`\btime_T\b`)
)

// w48Strip blanks every string and character literal on a line, so that a
// count over what it returns is a count over code.
func w48Strip(line []byte) []byte {
	Out := make([]byte, 0, len(line))
	i, n := 0, len(line)
	for i < n {
		c := line[i]
		if c == '"' || c == '\'' {
			q := c
			Out = append(Out, ' ')
			i++
			for i < n {
				if line[i] == '\\' {
					i += 2
					continue
				}
				if line[i] == q {
					break
				}
				i++
			}
			i++
			continue
		}
		Out = append(Out, c)
		i++
	}
	return Out
}

func w48Code(lines [][]byte) []byte {
	Out := make([][]byte, len(lines))
	for i, l := range lines {
		Out[i] = w48Strip(l)
	}
	return bytes.Join(Out, []byte{'\n'})
}

// w48Cut is the C view's lines, and the index of the first include line:
// the boundary.
func w48Cut(t []byte) ([][]byte, int) {
	lines := bytes.Split(t, []byte{'\n'})
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte("#include ")) {
			return lines, i
		}
	}
	return lines, -1
}

// Edit is the phase on the graph.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("wallclock", e, w)
	q := graph.NewVerbs("wallclock", e, io.Discard) // the acts; the text said its own lines

	// ---- 0. the file: the includes one run of system headers, <time.h>
	// among them, nothing above the first but the core
	incs, _, err := e.SystemIncludeRun()
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	nInc := len(incs)
	hasTime := false
	for _, inc := range incs {
		hasTime = hasTime || graph.IncludeSpec(inc) == "<time.h>"
	}
	if !hasTime {
		v.Die("<time.h> is not among the eleven includes.  It is what will declare " +
			"`time()` for the host once the core stops declaring it, and what the " +
			"static_assert this phase adds compares `time_T` against")
		return v.Done()
	}
	t := v.Text()
	lines, cut := w48Cut(t)
	core := w48Code(lines[:cut])
	below := w48Code(lines[cut:])
	v.Sayf("eleven `#include`s, contiguous, at lines %d-%d, <time.h> among them, and "+
		"NOTHING above the first of them -- so the core is the %d lines above the "+
		"boundary and the host is the %d below it",
		cut+1, cut+nInc, cut, len(lines)-cut-1)

	// ---- 1. the inventory, the text's counts on the C view
	for _, x := range []struct {
		Name          string
		nCore, nBelow int
	}{
		{"time", 4, 0}, {"vim_time", 7, 0}, {"host_time", 0, 0},
		{"time_T", 10, 0}, {"time_t", 0, 0}, {"musl_now_ms", 9, 1},
	} {
		re := regexp.MustCompile(`\b` + x.Name + `\b`)
		gc := len(re.FindAll(edit.WithoutIncludes(core), -1))
		gb := len(re.FindAll(edit.WithoutIncludes(below), -1))
		if gc != x.nCore || gb != x.nBelow {
			v.Die("`%s` occurs %d times above the boundary and %d below it, where this "+
				"phase was written against %d and %d", x.Name, gc, gb, x.nCore, x.nBelow)
			return v.Done()
		}
	}
	v.Say("the core says `time` FOUR times -- the libc prototype, vim_time's own call, " +
		"and ui_focus_change's TWO, which bypass the wrapper -- and `vim_time` seven: a " +
		"prototype, a definition and five call sites.  Below the boundary `time` is only " +
		"in `#include <time.h>`, which names a header, and musl_now_ms, the clock that has already crossed, " +
		"is 9 above and 1 below")

	// ---- 2. the prototype block: `long time(long *tp);`, phase 42's
	const tp = "long time(long *tp);"
	var at []int
	for i, l := range lines {
		if string(l) == tp {
			at = append(at, i)
		}
	}
	timeDecls := e.FileDecls("time")
	if len(at) != 1 || len(timeDecls) != 1 || !e.InCore(timeDecls[0]) {
		v.Die("`%s` is not on a line of its own exactly once above the boundary, the one "+
			"declaration of `time` -- it is phase 42's, and it is what this phase removes", tp)
		return v.Done()
	}
	a, b := at[0], at[0]
	for len(bytes.TrimSpace(lines[a-1])) > 0 {
		a--
	}
	for len(bytes.TrimSpace(lines[b+1])) > 0 {
		b++
	}
	if b >= cut {
		v.Die("the prototype block runs past the boundary, so it is not the block " +
			"phase 42 wrote")
		return v.Done()
	}
	var notProto, protoNames []string
	for _, l := range lines[a : b+1] {
		if !w48Proto.Match(l) {
			notProto = append(notProto, string(l))
		}
		if bytes.HasPrefix(l, []byte("static ")) {
			v.Die("a prototype in the block is `static`, which phase 42 forbade: it " +
				"would give the core an internal function that is never defined")
			return v.Done()
		}
		f := strings.Fields(strings.SplitN(string(l), "(", 2)[0])
		protoNames = append(protoNames, strings.TrimLeft(f[len(f)-1], "*"))
	}
	if len(notProto) > 0 {
		v.Die("the block around `%s` is not all prototypes: %s", tp, strings.Join(notProto, " / "))
		return v.Done()
	}
	v.Sayf("the core's libc prototype block is %d lines (%d-%d), every one a plain "+
		"non-`static` prototype: %s.  This phase takes `time` out of it and leaves %d",
		b-a+1, a+1, b+1, strings.Join(protoNames, " "), b-a)

	// ---- 3. the literals
	spans, err := edit.LiteralSpans(edit.Ph{Tag: v.Tag}, t)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	var bad, english []string
	for _, s := range spans {
		lit := t[s[0]:s[1]]
		if w48Names.Match(lit) {
			bad = append(bad, string(lit))
		}
		if w48Word.Match(lit) {
			english = append(english, string(lit))
		}
	}
	if len(bad) > 0 {
		v.Die("a literal holds a name this phase substitutes: %s", strings.Join(bad, " / "))
		return v.Done()
	}
	var shortened []string
	for _, x := range english {
		if len(x) > 34 {
			x = x[:34]
		}
		shortened = append(shortened, x)
	}
	v.Sayf("%d string and character literals, NONE holding `vim_time`, `host_time`, "+
		"`time_T` or `time_t` -- and %d holding the English word `time` (%s), which is "+
		"why no substitution below is a bare name",
		len(spans), len(english), strings.Join(shortened, " / "))

	// ---- 4. STEP ONE: ui_focus_change's two direct reads are vim_time()
	q.InFunction("ui_focus_change", func(q *graph.Verbs) {
		q.ReplaceC("(call time nullptr)", "vim_time()", 2,
			"ui_focus_change's two standalone clock reads, which are the whole of step one")
	})
	if q.Err != nil {
		return q.Err
	}
	mlines, mcut := w48Cut(v.Text())
	if k := len(w48TimeCall.FindAll(w48Code(mlines[:mcut]), -1)); k != 2 {
		v.Die("`time(` occurs %d times above the boundary after step one, where 2 were "+
			"expected -- the prototype and the one call inside the wrapper", k)
		return v.Done()
	}
	v.Say("STEP ONE: ui_focus_change's two direct `time(nullptr)` are `vim_time()`.  " +
		"STILL TWO READS, in the same two statements, in the same order -- and `time(` " +
		"above the boundary is now the prototype and ONE call site, the wrapper's own")

	// ---- 5. STEP TWO: vim_time is host_time, its prototype in the core's
	// host block and its definition in the host, `long` both
	nRen := len(w48VimTime.FindAll(v.Text(), -1))
	if nRen != 9 {
		v.Die("%d `vim_time` were renamed where 9 were counted -- the prototype, the "+
			"definition, five call sites and the two step one just made", nRen)
		return v.Done()
	}
	decls := e.FileDecls("vim_time")
	if len(decls) != 2 || !decls[0].Is("def") || !decls[1].Is("defn") {
		v.Die("`vim_time` is not one prototype and one definition: %d declarations", len(decls))
		return v.Done()
	}
	proto, def := decls[0], decls[1]
	if _, err := e.Rename(proto, "host_time"); err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	msg := e.FileDecls("host_message")
	musl := e.Defn("musl_delay")
	if len(msg) == 0 || !msg[0].Is("def") || !e.InCore(msg[0]) || musl == nil || !e.InHost(musl) {
		v.Die("the core's host block (`host_message`'s prototype) or the host's musl_delay is not where " +
			"this phase moves host_time's declarations")
		return v.Done()
	}
	if err := e.MoveFormsAfter(msg[0], proto); err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if err := e.MoveFormsBefore(musl, def); err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if _, err := e.RetypeResult("host_time", "long"); err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	v.Sayf("STEP TWO: `vim_time` is `host_time` at all %d mentions; its declaration moves "+
		"from the forward-declaration block to the host block, as `static long "+
		"host_time(void);` -- `long` and not `time_T`, because the DEFINITION is below "+
		"the boundary and the host cannot name a core typedef once the file is cut; and "+
		"the definition lands between musl_now_ms and musl_delay", nRen)

	// ---- 6. `time` leaves the core; host_time's call is <time.h>'s
	if u := e.Uses(timeDecls[0]); len(u) != 1 || e.Function(u[0]) != def {
		v.Die("`%s` is used %d times, where host_time's one call was expected", tp, len(u))
		return v.Done()
	}
	if err := e.Delete(timeDecls[0]); err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	q.InFunction("host_time", func(q *graph.Verbs) {
		q.ReplaceC("(call time nullptr)", "time(nullptr)", 1,
			"host_time's clock read, now <time.h>'s `time`")
	})
	sigterm := q.One(`(static_assert (== 15 SIGTERM) "SIGTERM")`,
		"the twelve constants phase 43 put below the includes, which is the only place "+
			"in the file where a core name and a header name are both in scope")
	if q.Err != nil {
		return q.Err
	}
	if !e.InHost(sigterm) {
		v.Die("SIGTERM's static_assert is not below the boundary")
		return v.Done()
	}
	if _, err := e.SpliceC(graph.Frag{At: e.SpotAfter(sigterm),
		Src: `static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");`}); err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	v.Sayf("`%s` leaves the core -- the block goes %d lines to %d -- and "+
		"`static_assert(_Generic((time_T)0, time_t: 1, default: 0), \"time_T is "+
		"time_t\");` joins the twelve below the includes.  THE PROTOTYPE WAS THE "+
		"GUARANTEE: gcc compared it with <time.h>'s and would have said `conflicting "+
		"types for 'time'`.  The assertion says the same thing about the same two "+
		"types, and the check proves it can fail", tp, b-a+1, b-a)

	// ---- 7. what the file is now, on the C view
	T := v.Text()
	L, ncut := w48Cut(T)
	// the C view writes a form and the blank line after it: the core loses
	// the prototype and the definition (2 + 6 lines, the host_time
	// prototype only moving), the host gains the definition and the assert
	const coreDelta = -2 - 6
	const belowDelta = 6 + 2
	if len(L)-len(lines) != coreDelta+belowDelta {
		v.Die("the file moved by %d lines where %d was expected", len(L)-len(lines), coreDelta+belowDelta)
		return v.Done()
	}
	if _, at, err := e.SystemIncludeRun(); err != nil || len(e.Includes()) != nInc || ncut != cut+coreDelta || at < 0 {
		v.Die("the eleven directives are not the eleven contiguous lines at %d: the first is at %d (%v).  "+
			"The first one IS the boundary, and it moves up by exactly what the core lost",
			cut+coreDelta+1, ncut+1, err)
		return v.Done()
	}
	ncore := w48Code(L[:ncut])
	nbelow := w48Code(L[ncut:])
	if w48Word.Match(ncore) {
		v.Die("`time` is still named %d times above the boundary, and the whole "+
			"product of this phase is that the core does not name it at all",
			len(w48Word.FindAll(ncore, -1)))
		return v.Done()
	}
	if len(w48VimTime.FindAll(T, -1)) > 0 {
		v.Die("`vim_time` survives somewhere in the file")
		return v.Done()
	}
	if k := len(w48HostTime.FindAll(ncore, -1)); k != 8 {
		v.Die("`host_time` occurs %d times above the boundary where 8 were expected "+
			"-- the declaration and seven call sites", k)
		return v.Done()
	}
	if k := len(w48HostTime.FindAll(nbelow, -1)); k != 1 {
		v.Die("`host_time` occurs %d times below the boundary where 1 was expected "+
			"-- its definition", k)
		return v.Done()
	}
	if k := len(w48Word.FindAll(edit.WithoutIncludes(nbelow), -1)); k != 1 {
		v.Die("`time` occurs %d times below the boundary where 1 was expected -- "+
			"host_time's call (an #include line names a header).  `time_t` is not one of them: `_` "+
			"is a word character, so `\\btime\\b` does not match inside it", k)
		return v.Done()
	}
	if k := len(w48TimeT.FindAll(nbelow, -1)); k != 1 {
		v.Die("`time_t` occurs %d times below the boundary where 1 was expected -- "+
			"the static_assert", k)
		return v.Done()
	}
	if k := len(w48TimeTT.FindAll(ncore, -1)); k != 8 {
		v.Die("`time_T` occurs %d times above the boundary where 8 were expected -- "+
			"the input had 10 and the two that go are the prototype's and the "+
			"definition's", k)
		return v.Done()
	}
	if !bytes.Contains(ncore, []byte("typedef long time_T;")) {
		v.Die("`typedef long time_T;` is not in the core.  host_time returns " +
			"`long`, so the core's clock type being `long` is what makes every call site " +
			"an assignment and not a conversion")
		return v.Done()
	}
	// host_time's call is <time.h>'s, by edge, as an import says it
	call := q.One("(call time nullptr)", "host_time's clock read")
	if q.Err != nil {
		return q.Err
	}
	if r := call.Kids[1].Ref(); r == nil || e.InCore(r) || e.InHost(r) {
		v.Die("host_time's `time` does not refer to the header's declaration")
		return v.Done()
	}
	v.Sayf("THE CORE DOES NOT NAME `time` AT ALL -- four mentions to none -- `host_time` "+
		"is 8 above the boundary and 1 below, `time_t` is named once in the whole file "+
		"and it is the static_assert, and the file is %d lines against %d",
		len(L)-1, len(lines)-1)
	return v.Done()
}
