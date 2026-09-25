package p115

// Whim phase 115 -- the clock crosses the boundary.
// See GOALS.md II.4c, GOALS.md, internal/phase/109/edit.go (which wrote the core's libc
// prototypes) and internal/phase/111/edit.go (the scalar clock, whose musl_now_ms is this
// phase's sibling).
//
// THE CORE READS TWO CLOCKS AND ONLY ONE OF THEM HAS CROSSED.  Phase 111 gave the
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
// time_T;` is phase 109's, and it is correct ONLY because `long time(long *tp);` sits
// above <time.h>'s own declaration of the same function: gcc compares the two and says
// `conflicting types for 'time'` if they disagree -- MEASURED by phase 109's `m1`
// control, and measured again here.  Phase 109's sixteen static_asserts were a control in
// ITS check and are NOT in the product (the twelve that survive below the includes are
// phase 110's constants), so once the prototype goes there is nothing left comparing the
// core's width to the host's, and `host_time()` returning a narrower type than `time_t`
// would truncate in silence on a target where the two differ.  So the prototype is
// REPLACED, not merely deleted:
//
// static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");
//
// below the includes, beside the twelve.  internal/phase/115/check.go measures that it holds
// AND that it fails when `time_T` is perturbed to `int` -- a guarantee you cannot break
// is not one.
//
// `host_time()` RETURNS `long`, NOT `time_T`, AND THAT IS A DECISION.  Its DEFINITION is
// below the boundary, and `time_T` is a core typedef declared above it: when the file is
// finally cut at the first `#include` the host half cannot name it.  `musl_now_ms()`
// returns `long` for exactly that reason and this is its sibling, so the two halves of
// the clock cross in the same shape -- and the boundary's stated property, that every
// core -> host signature takes scalars and byte buffers only (internal/phase/111/check.go),
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
// or when.  internal/phase/115/check.go instruments every clock read on BOTH binaries and
// requires the same count in the same records, with a control that collapses the two
// reads into one and moves it.
//
// THE ENGLISH WORD `time` IS NOT A CALL.  `op_shift()`'s NGETTEXT strings say "%ld line
// %sed %d time" / "times", and ``zhostonly`` learned the same lesson about
// `"close buffer"`: every count here is taken with string and character literals blanked
// out, and the substitutions are exact multi-line blocks, never a bare `time` -> anything.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim115", Edit) }

var (
	w115Inc      = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
	w115Proto    = regexp.MustCompile(`^[A-Za-z_].*\);$`)
	w115Names    = regexp.MustCompile(`\b(?:vim_time|host_time|time_T|time_t)\b`)
	w115Word     = regexp.MustCompile(`\btime\b`)
	w115TimeCall = regexp.MustCompile(`\btime\s*\(`)
	w115VimTime  = regexp.MustCompile(`\bvim_time\b`)
	w115HostTime = regexp.MustCompile(`\bhost_time\b`)
	w115TimeT    = regexp.MustCompile(`\btime_t\b`)
	w115TimeTT   = regexp.MustCompile(`\btime_T\b`)
)

// w115Strip blanks string and character literals in ONE line.  It is
// zhostonly's, and for the same reason: this file says "%ld line %sed %d time"
// in two NGETTEXT strings, and a count that read those as calls would be
// counting English.
//
// It is NOT edit.Blank -- it collapses a literal to a single space rather
// than preserving its offsets, because nothing here indexes back into it.
func w115Strip(line []byte) []byte {
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

func w115Code(lines [][]byte) []byte {
	Out := make([][]byte, len(lines))
	for i, l := range lines {
		Out[i] = w115Strip(l)
	}
	return bytes.Join(Out, []byte{'\n'})
}

// Whim115 sends the wall clock across the boundary: vim_time() becomes
// host_time() below the line, `long time(long *tp);` leaves the core's
// prototype block and a static_assert stronger than it replaces it.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 169 drops the unused)
	p := edit.Ph{Tag: "wallclock", W: w}
	t := text

	once := func(text []byte, old, new, why string) ([]byte, error) {
		if k := edit.CountAnchorB(text, old); k != 1 {
			shown := strings.ReplaceAll(old, "\n", "\\n")
			if len(shown) > 70 {
				shown = shown[:70]
			}
			return nil, p.Die("`%s` is not in the file exactly once (%s)", shown, why)
		}
		return edit.ReplaceAnchorB(text, old, []byte(new), 1), nil
	}

	lines := bytes.Split(t, []byte{'\n'})

	// ---- 0. the file this edit was written against ----------------------
	// ELEVEN DIRECTIVES, every one an `#include <...>`, CONTIGUOUS, and
	// NOTHING ABOVE THEM.  Phase 110 made the first of them the boundary
	// between the core and the host (GOALS.md II.4c); this phase edits both
	// sides of that line and must know where it is.
	var directives []int
	for i, l := range lines {
		if bytes.HasPrefix(bytes.TrimLeft(l, " \t\n\v\f\r"), []byte("#")) {
			directives = append(directives, i)
		}
	}
	if len(directives) != nInc {
		return nil, p.Die("the file has %d preprocessor directives and this phase was written "+
			"against 11", len(directives))
	}
	for k, i := range directives {
		if i != directives[0]+k {
			var at []string
			for _, j := range directives {
				at = append(at, fmt.Sprint(j+1))
			}
			return nil, p.Die("the eleven directives are not contiguous: %s", strings.Join(at, " "))
		}
	}
	var incNames []string
	for _, i := range directives {
		if !w115Inc.Match(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no " +
				"phase may add one")
		}
		incNames = append(incNames, string(lines[i]))
	}
	if !edit.ContainsStr(incNames, "#include <time.h>") {
		return nil, p.Die("<time.h> is not among the eleven includes.  It is what will declare " +
			"`time()` for the host once the core stops declaring it, and what the " +
			"static_assert this phase adds compares `time_T` against")
	}
	cut := directives[0]
	core := w115Code(lines[:cut])
	below := w115Code(lines[cut:])
	p.Sayf("eleven `#include`s, contiguous, at lines %d-%d, <time.h> among them, and "+
		"NOTHING above the first of them -- so the core is the %d lines above the "+
		"boundary and the host is the %d below it",
		cut+1, cut+nInc, cut, len(lines)-cut-1)

	// ---- 1. the inventory, counted here rather than remembered -----------
	// Every number is what this edit is about to act on, taken on the
	// literal-stripped text.
	want := []struct {
		Name          string
		nCore, nBelow int
	}{
		{"time", 4, 0}, {"vim_time", 7, 0}, {"host_time", 0, 0},
		{"time_T", 10, 0}, {"time_t", 0, 0}, {"musl_now_ms", 9, 1},
	}
	for _, x := range want {
		re := regexp.MustCompile(`\b` + x.Name + `\b`)
		gc := len(re.FindAll(edit.WithoutIncludes(core), -1))
		gb := len(re.FindAll(edit.WithoutIncludes(below), -1))
		if gc != x.nCore || gb != x.nBelow {
			return nil, p.Die("`%s` occurs %d times above the boundary and %d below it, where this "+
				"phase was written against %d and %d", x.Name, gc, gb, x.nCore, x.nBelow)
		}
	}
	p.Say("the core says `time` FOUR times -- the libc prototype, vim_time's own call, " +
		"and ui_focus_change's TWO, which bypass the wrapper -- and `vim_time` seven: a " +
		"prototype, a definition and five call sites.  Below the boundary `time` is only " +
		"in `#include <time.h>`, which names a header, and musl_now_ms, the clock that has already crossed, " +
		"is 9 above and 1 below")

	// ---- 2. the core's own libc prototype block, computed ----------------
	// The block is the run of non-blank lines around `long time(long *tp);`,
	// not a line number and not a count this file states: another phase in
	// flight removes two of its entries, and a written size would already be
	// stale.
	const tp = "long time(long *tp);"
	var at []int
	for i, l := range lines {
		if string(l) == tp {
			at = append(at, i)
		}
	}
	if len(at) != 1 {
		return nil, p.Die("`%s` is not on a line of its own exactly once above the boundary -- it "+
			"is phase 109's, and it is what this phase removes", tp)
	}
	a, b := at[0], at[0]
	for len(bytes.TrimSpace(lines[a-1])) > 0 {
		a--
	}
	for len(bytes.TrimSpace(lines[b+1])) > 0 {
		b++
	}
	if b >= cut {
		return nil, p.Die("the prototype block runs past the boundary, so it is not the block " +
			"phase 109 wrote")
	}
	var notProto []string
	for _, l := range lines[a : b+1] {
		if !w115Proto.Match(l) {
			notProto = append(notProto, string(l))
		}
	}
	if len(notProto) > 0 {
		return nil, p.Die("the block around `%s` is not all prototypes: %s",
			tp, strings.Join(notProto, " / "))
	}
	for _, l := range lines[a : b+1] {
		if bytes.HasPrefix(l, []byte("static ")) {
			return nil, p.Die("a prototype in the block is `static`, which phase 109 forbade: it " +
				"would give the core an internal function that is never defined")
		}
	}
	var protoNames []string
	for _, l := range lines[a : b+1] {
		f := strings.Fields(strings.SplitN(string(l), "(", 2)[0])
		protoNames = append(protoNames, strings.TrimLeft(f[len(f)-1], "*"))
	}
	p.Sayf("the core's libc prototype block is %d lines (%d-%d), every one a plain "+
		"non-`static` prototype: %s.  This phase takes `time` out of it and leaves %d",
		b-a+1, a+1, b+1, strings.Join(protoNames, " "), b-a)

	// ---- 3. the literals -------------------------------------------------
	// Phase 106 was caught Out by three string literals holding `NULL`; 26 and
	// 28 applied the lesson rather than assuming it.  So does this one.
	spans, err := edit.LiteralSpans(p, t)
	if err != nil {
		return nil, err
	}
	var bad, english []string
	for _, s := range spans {
		lit := t[s[0]:s[1]]
		if w115Names.Match(lit) {
			bad = append(bad, string(lit))
		}
		if w115Word.Match(lit) {
			english = append(english, string(lit))
		}
	}
	if len(bad) > 0 {
		return nil, p.Die("a literal holds a name this phase substitutes: %s", strings.Join(bad, " / "))
	}
	var shortened []string
	for _, x := range english {
		if len(x) > 34 {
			x = x[:34]
		}
		shortened = append(shortened, x)
	}
	p.Sayf("%d string and character literals, NONE holding `vim_time`, `host_time`, "+
		"`time_T` or `time_t` -- and %d holding the English word `time` (%s), which is "+
		"why no substitution below is a bare name",
		len(spans), len(english), strings.Join(shortened, " / "))

	// ---- 4. STEP ONE: ui_focus_change stops bypassing the wrapper --------
	// TWO READS BEFORE AND TWO READS AFTER.  The condition reads the clock
	// and the Body reads it again; nothing is hoisted into a local, because
	// that would be a different program -- the two reads could always fall
	// either side of a second tick and they still can, exactly as often.  The
	// check instruments both binaries and requires the same count, with a
	// control that DOES hoist and moves it.
	t, err = once(t, `    if (in_focus && last_time + 2 < time(nullptr))
    {
        last_time = time(nullptr);
    }
`, `    if (in_focus && last_time + 2 < vim_time())
    {
        last_time = vim_time();
    }
`, "ui_focus_change's two standalone clock reads, which are the whole of step one")
	if err != nil {
		return nil, err
	}
	mid := w115Code(bytes.Split(t, []byte{'\n'})[:cut])
	if k := len(w115TimeCall.FindAll(mid, -1)); k != 2 {
		return nil, p.Die("`time(` occurs %d times above the boundary after step one, where 2 were "+
			"expected -- the prototype and the one call inside the wrapper", k)
	}
	p.Say("STEP ONE: ui_focus_change's two direct `time(nullptr)` are `vim_time()`.  " +
		"STILL TWO READS, in the same two statements, in the same order -- and `time(` " +
		"above the boundary is now the prototype and ONE call site, the wrapper's own")

	// ---- 5. STEP TWO: the wrapper becomes the host's ---------------------
	nRen := len(w115VimTime.FindAll(t, -1))
	t = w115VimTime.ReplaceAll(t, []byte("host_time"))
	if nRen != 9 {
		return nil, p.Die("%d `vim_time` were renamed where 9 were counted -- the prototype, the "+
			"definition, five call sites and the two step one just made", nRen)
	}
	// WITH THE BLANK LINE UNDER IT.  The canonical text puts one between two
	// file-scope declarations, so taking the line alone would leave a run of
	// two -- which no verification tier can see and which the arithmetic at the
	// end refuses.
	t, err = once(t, "static time_T host_time(void);\n\n", "",
		"the core's own forward declaration, in the block of them")
	if err != nil {
		return nil, err
	}
	t, err = once(t, "static void host_message(const char *msg, int len, int err);\n",
		"static void host_message(const char *msg, int len, int err);\n"+
			"static long host_time(void);\n",
		"the core's HOST BLOCK, where host_exit, host_message and the nine musl_ "+
			"names are declared")
	if err != nil {
		return nil, err
	}
	// The definition leaves with one of the two blank lines around it, so no
	// run of two is left behind -- which no verification tier can see
	// (CLAUDE.md).
	t, err = once(t, `
    static time_T
host_time(void)
{
    return time(nullptr);
}
`, "", "the definition, which is one clock read and nothing else")
	if err != nil {
		return nil, err
	}
	// It lands between musl_now_ms and musl_delay: beside the clock that
	// crossed at phase 111, and INSIDE the region `zhostonly` reads as the
	// host.
	t, err = once(t, `    static void
musl_delay(long ms, int interruptible)
`, `    static long
host_time(void)
{
    return time(nullptr);
}

    static void
musl_delay(long ms, int interruptible)
`, "the host block, immediately below musl_now_ms -- the clock's other half")
	if err != nil {
		return nil, err
	}
	p.Sayf("STEP TWO: `vim_time` is `host_time` at all %d mentions; its declaration moves "+
		"from the forward-declaration block to the host block, as `static long "+
		"host_time(void);` -- `long` and not `time_T`, because the DEFINITION is below "+
		"the boundary and the host cannot name a core typedef once the file is cut; and "+
		"the definition lands between musl_now_ms and musl_delay", nRen)

	// ---- 6. the prototype the core no longer needs, and what replaces it --
	t, err = once(t, tp+"\n", "",
		"phase 109's prototype for time(): nothing above the boundary calls it now")
	if err != nil {
		return nil, err
	}
	t, err = once(t, "static_assert(15 == SIGTERM, \"SIGTERM\");\n",
		"static_assert(15 == SIGTERM, \"SIGTERM\");\n"+
			"static_assert(_Generic((time_T)0, time_t: 1, default: 0), \"time_T is time_t\");\n",
		"the twelve constants phase 110 put below the includes, which is the only place "+
			"in the file where a core name and a header name are both in scope")
	if err != nil {
		return nil, err
	}
	p.Sayf("`%s` leaves the core -- the block goes %d lines to %d -- and "+
		"`static_assert(_Generic((time_T)0, time_t: 1, default: 0), \"time_T is "+
		"time_t\");` joins the twelve below the includes.  THE PROTOTYPE WAS THE "+
		"GUARANTEE: gcc compared it with <time.h>'s and would have said `conflicting "+
		"types for 'time'`.  The assertion says the same thing about the same two "+
		"types, and the check proves it can fail", tp, b-a+1, b-a)

	// ---- 7. what the file is now -----------------------------------------
	L := bytes.Split(t, []byte{'\n'})
	const coreDelta = -2 + 1 - 6 - 1
	const belowDelta = 6 + 1
	if len(L)-len(lines) != coreDelta+belowDelta {
		return nil, p.Die("the file moved by %d lines where %d was expected",
			len(L)-len(lines), coreDelta+belowDelta)
	}
	var ndir []int
	for i, l := range L {
		if bytes.HasPrefix(bytes.TrimLeft(l, " \t\n\v\f\r"), []byte("#")) {
			ndir = append(ndir, i)
		}
	}
	// THE BOUNDARY MOVES UP BY WHAT THE CORE LOST, and by exactly that: the
	// core is the lines above the first `#include`, so the directive's index
	// IS the core's line count.
	okDir := len(ndir) == nInc
	for k, i := range ndir {
		if okDir && i != cut+coreDelta+k {
			okDir = false
		}
	}
	if !okDir {
		var at3 []string
		for _, i := range ndir {
			if len(at3) < 3 {
				at3 = append(at3, fmt.Sprint(i+1))
			}
		}
		return nil, p.Die("the eleven directives are not the eleven contiguous lines at %d: they "+
			"are at %s.  The first one IS the boundary, and it moves up by exactly what "+
			"the core lost", cut+coreDelta+1, strings.Join(at3, " "))
	}
	for _, i := range ndir {
		if !w115Inc.Match(L[i]) {
			return nil, p.Die("a directive is no longer an `#include <...>` of a system header")
		}
	}
	ncore := w115Code(L[:ndir[0]])
	nbelow := w115Code(L[ndir[0]:])
	if w115Word.Match(ncore) {
		return nil, p.Die("`time` is still named %d times above the boundary, and the whole "+
			"product of this phase is that the core does not name it at all",
			len(w115Word.FindAll(ncore, -1)))
	}
	if len(w115VimTime.FindAll(t, -1)) > 0 {
		return nil, p.Die("`vim_time` survives somewhere in the file")
	}
	if k := len(w115HostTime.FindAll(ncore, -1)); k != 8 {
		return nil, p.Die("`host_time` occurs %d times above the boundary where 8 were expected "+
			"-- the declaration and seven call sites", k)
	}
	if k := len(w115HostTime.FindAll(nbelow, -1)); k != 1 {
		return nil, p.Die("`host_time` occurs %d times below the boundary where 1 was expected "+
			"-- its definition", k)
	}
	if k := len(w115Word.FindAll(edit.WithoutIncludes(nbelow), -1)); k != 1 {
		return nil, p.Die("`time` occurs %d times below the boundary where 1 was expected -- "+
			"host_time's call (an #include line names a header).  `time_t` is not one of them: `_` "+
			"is a word character, so `\\btime\\b` does not match inside it", k)
	}
	if k := len(w115TimeT.FindAll(nbelow, -1)); k != 1 {
		return nil, p.Die("`time_t` occurs %d times below the boundary where 1 was expected -- "+
			"the static_assert", k)
	}
	if k := len(w115TimeTT.FindAll(ncore, -1)); k != 8 {
		return nil, p.Die("`time_T` occurs %d times above the boundary where 8 were expected -- "+
			"the input had 10 and the two that go are the prototype's and the "+
			"definition's", k)
	}
	if !bytes.Contains(ncore, []byte("typedef long time_T;")) {
		return nil, p.Die("`typedef long time_T;` is not in the core.  host_time returns " +
			"`long`, so the core's clock type being `long` is what makes every call site " +
			"an assignment and not a conversion")
	}
	p.Sayf("THE CORE DOES NOT NAME `time` AT ALL -- four mentions to none -- `host_time` "+
		"is 8 above the boundary and 1 below, `time_t` is named once in the whole file "+
		"and it is the static_assert, and the file is %d lines against %d",
		len(L)-1, len(lines)-1)
	return t, nil
}
