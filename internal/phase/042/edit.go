package p042

// Whim phase 42 (formerly 109) -- the header types and macros the core can own.
// See GOALS.md II.4c, GOALS.md, and .claude/briefs/zero-reorg.md 1 and 7.
//
// GOALS.md II.4c's design is that the FIRST `#include` becomes the boundary: the core
// is the prefix above it and has no preprocessor syntax at all.  That is phase 43's
// move.  This phase is the part of it that can be done BEFORE the move, and doing it
// before is the whole point -- see THE ORDERING below.
//
// WHAT THE CORE STILL TAKES FROM A HEADER, and what it gets instead.  Every count is
// `grep -ow` above the host block and every one is re-measured from the input here:
//
// time_t          6   ->  `typedef long time_T;` and `time_T` at the other five.
// The core chooses the width, as phase 0a made it choose
// usize's.  What PINS the choice is the `time` prototype
// below: `long time(long *)` is accepted against <time.h>
// only where `time_t` IS `long`.
// sig_atomic_t    2   ->  spelled `volatile int`, and the name disappears.  The
// THREE in the host block keep it: they are below the
// boundary and <signal.h> is theirs.
// uintptr_t       1   ->  `usize`.  One cast, `(unsigned long long)(uintptr_t)p`.
// struct timeval  4   ->  a core-owned TAGLESS struct with two `long` fields, and
// `musl_gettimeofday(long *sec, long *usec)` in the host
// block for the five calls.  This is the ONE item that
// changes code; see THE CLOCK.
// MIN 7 / MAX 16  ->      expanded at 19 lines to the text <sys/param.h> gives,
// READ FROM THE HEADER rather than written here.
// offsetof        9   ->  `__builtin_offsetof`, which GOALS.md II.4c settled.
// ten libc calls  ->      plain prototypes: malloc realloc free time getpid kill
// write labs abs -- nine, `gettimeofday` being the tenth and
// the one that cannot stay, its argument being a struct.
//
// THE DERIVED CONSTANTS ARE NOT THIS PHASE'S.  `enum : int { INT_MAX = ... };` placed
// after `#include <limits.h>` is `enum : int { 0x7fffffff = ... };`, a syntax error, so
// the twelve constants LONG_MAX INT_MAX PATH_MAX ULLONG_MAX LLONG_MAX INT_MIN SIGHUP
// SIGTERM LONG_MIN LLONG_MIN SIZE_MAX EXIT_FAILURE can only be written once the
// includes have moved.  They belong to phase 43 with the move, and they are the only
// header-supplied names this phase leaves in the core.
//
// THE ORDERING -- WHY THIS IS A PHASE OF ITS OWN AND WHY IT COMES FIRST.  Every
// declaration here REPLACES something a header above it still supplies, so the ordinary
// build cross-checks every one of them for free, and after the move there is nothing
// left to check against.  MEASURED, four ways, each a control in the check:
//
// `int time(int *tp);`            error: conflicting types for 'time'
// `void *malloc(int n);`          error: conflicting types for 'malloc'
// `long getpid(void);`            error: conflicting types for 'getpid'
// `static void *malloc(usize n);` error: static declaration of 'malloc' follows
// non-static declaration
//
// THE LAST IS THE TRAP THE BRIEF NAMES, AND THE ORDERING CHANGES ITS SHAPE.  After the
// move a `static` prototype is a LINK failure -- gcc says `'malloc' used but never
// defined` -- because there is no other declaration to conflict with.  Here it is a
// hard error at the declaration itself, which is cheaper and points at the line.
//
// AND THE POSITIVE FORM IS WHAT THE MOVE DESTROYS.  Fifteen `static_assert`s compare
// every core-owned spelling against the header type it replaces -- `_Generic((time_T)0,
// time_t: ...)`, `_Generic((usize)0, uintptr_t: ...)`, `sizeof(elapsed_T) ==
// sizeof(struct timeval)`, `__builtin_offsetof(T, m) == offsetof(T, m)` at all six
// types -- and every one of them names a header type, so not one can be written once
// the headers are below.  They are a control in the check, not in the product.
//
// THE TAG IS STILL FORBIDDEN, AND THE REASON IN THE BRIEF IS NO LONGER TRUE.  The brief
// says a core-defined `struct timeval` tag is a hard `error: redefinition`.  MEASURED
// here: it is NOT, under the dialect this file compiles in.  C23 permits a struct to be
// redeclared with the same members, and gcc 15 takes it in silence both ways round --
// `-std=c11` and `-std=c17` give `error: redefinition of 'struct timeval'` and `-std=c23`
// gives nothing.  So the tagless struct is mandatory for a better reason than a
// diagnostic: the core must not define a libc TAG at all, and the layout equality has to
// be ASSERTED rather than left to an error C23 has removed.  The check does both --
// the tag control under c11 and under the file's own dialect, and the three
// `static_assert`s on the layout.
//
// THE CLOCK IS THE ONLY THING THAT CHANGES CODE, and the check proves that as an
// equality rather than claiming it.  Everything above is a rename or a macro expansion
// that the preprocessor was already performing, so it cannot generate a different
// instruction; `gettimeofday(&start_tv, nullptr)` becoming
// `musl_gettimeofday(&start_tv.tv_sec, &start_tv.tv_usec)` at five sites can and does.
// MEASURED: with the clock alone reverted, the binary is `cmp`-IDENTICAL to the one
// this phase was handed -- 788,488 bytes -- so the other six items and the nine
// prototypes are tier 1 of CLAUDE.md's verification table, and the clock is the only
// thing a recording has to answer for.  It does: MEASURED, the two recordings are
// BYTE-IDENTICAL, and the recording is NOT blind to the clock -- `musl_gettimeofday`
// writing the two fields the wrong way round moves SEVEN records.
//
// WHERE `musl_gettimeofday` GOES, AND IT IS NOT A FREE CHOICE.  It is defined inside
// the host block, immediately above `musl_delay`, because ``zhostonly`` reads
// the host region as the lines from `host_winch_pending` to the last brace of
// `musl_suspend()` and requires every mention of `struct timeval` to be inside it.  A
// definition below `musl_suspend` would be outside the region and the tool would refuse.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).
// MIN and MAX ARE READ FROM THE HEADER, not written here.  Two macro calls with
// distinctive arguments go through the preprocessor and come back as the expansion the
// header actually defines; the edit turns that into its template by replacing the two
// arguments.  If <sys/param.h> ever spelled MIN differently this phase would expand it
// differently, which is the only honest meaning of "the exact text the header gives".
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3c).  Every count and partition is
// the text's, on the C view; the edits are the graph's.  The nine libc
// prototypes are a fragment after the usize typedef.  time_T is RETYPE'd to
// long and the five other time_t uses retargeted to it; full_screen and
// got_int RETYPE'd `volatile int`; the one uintptr_t cast's type name
// retargeted to usize.  elapsed_T is a fragment in its typedef's place -- the
// tagless struct, cc's own members for the uses that select from it --
// and elapsed()'s prototype and definition the text's substitutions; each
// `gettimeofday(&X, nullptr)` a call node rebuilt, X's C read from the node.
// offsetof, MIN and MAX are MACROX's: each invocation of the macro, its
// arguments read from its text, made the C the text made of it.  The
// fragments are made in three imports.  What has no counterpart: the
// count of lines the phase adds, which counted the text's own output
// before its canonical print.

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim42", Edit) }

var (
	w42Inc   = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
	w42Names = regexp.MustCompile(`\b(?:time_t|sig_atomic_t|uintptr_t|MIN|MAX|offsetof|gettimeofday)\b|struct timeval\b`)
	w42TV    = regexp.MustCompile(`struct timeval\b`)
	w42MM    = regexp.MustCompile(`\b(MIN|MAX)\(`)
)

// Whim42 gives the core the header types and macros it can own: time_t,
// sig_atomic_t, uintptr_t, struct timeval, MIN, MAX and offsetof become the
// core's own and nine libc prototypes are written Out, WHILE THE HEADERS ARE
// STILL ABOVE THEM to be cross-checked against.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	p := edit.Ph{Tag: "headers", W: w}
	if len(args) != 1 {
		return p.Die("usage: edit whim42 <file> <minmax.txt>")
	}
	t, err := e.Graph().C()
	if err != nil {
		return err
	}
	nInc := edit.IncludeCount(t) // the headers it was handed (phase 88 drops the unused)

	lines := bytes.Split(t, []byte{'\n'})

	// ---- 0. the file this edit was written against ----------------------
	// ELEVEN DIRECTIVES, every one an `#include` of a system header, on the
	// first eleven lines -- GOALS.md's charter.  This phase adds no
	// directive and moves none: the move is phase 43's, and a phase that
	// quietly did it here would make every cross-check below impossible
	// rather than merely wrong.
	var dIdx []int
	var dLine [][]byte
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte("#")) {
			dIdx = append(dIdx, i)
			dLine = append(dLine, l)
		}
	}
	okFirst := len(dIdx) == nInc
	for k, i := range dIdx {
		if okFirst && i != k {
			okFirst = false
		}
	}
	if !okFirst {
		var at []string
		for _, i := range dIdx {
			at = append(at, strconv.Itoa(i))
		}
		return p.Die("the file does not have exactly eleven preprocessor directives on its "+
			"first eleven lines: %d directives at lines %s", len(dIdx), strings.Join(at, " "))
	}
	var headers []string
	for _, l := range dLine {
		m := w42Inc.FindSubmatch(l)
		if m == nil {
			return p.Die("a directive is not an `#include <...>` of a system header, and no " +
				"phase may add one")
		}
		headers = append(headers, string(m[1]))
	}
	for _, want := range []string{"stdlib.h", "unistd.h", "sys/param.h", "time.h",
		"signal.h", "stdint.h", "stddef.h"} {
		if !edit.ContainsStr(headers, want) {
			return p.Die("<%s> is not among the eleven includes, and it is one of the headers "+
				"this phase replaces: the cross-check it depends on would not happen", want)
		}
	}
	var shown []string
	for _, h := range headers {
		shown = append(shown, "<"+h+">")
	}
	p.Sayf("eleven directives, every one an `#include <...>` on the first eleven lines, "+
		"and the seven headers this phase takes from are all among them: %s",
		strings.Join(shown, " "))

	// ---- 1. the host block, which is the other side of the boundary ------
	// The core is everything above it.  Phase 43 puts the eleven includes
	// here; today the line is already exactly where the host block begins,
	// and the counts below are the counts that matter -- a `sig_atomic_t` in
	// the host is not this phase's business and a `sig_atomic_t` in the core
	// is.
	const host = "static volatile sig_atomic_t host_winch_pending"
	var hb []int
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte(host)) {
			hb = append(hb, i)
		}
	}
	if len(hb) != 1 {
		return p.Die("the host block does not begin exactly once with %s -- found %d.  "+
			"Without it this phase cannot tell a core mention from a host one",
			edit.PyRepr(host), len(hb))
	}
	hostAt := hb[0]
	coreT := bytes.Join(lines[11:hostAt], []byte{'\n'})
	hostT := bytes.Join(lines[hostAt:], []byte{'\n'})

	// ---- 2. the inventory, as a partition and not a list -----------------
	// What the core still takes from a header, counted here rather than
	// remembered.  Each number is checked against what the substitution below
	// actually does, so a count that has moved under this phase stops it
	// instead of letting it cut something else.
	for _, x := range []struct {
		Name         string
		nCore, nHost int
	}{
		{"time_t", 6, 0}, {"sig_atomic_t", 2, 3}, {"uintptr_t", 1, 0},
		{"size_t", 0, 0}, {"MIN", 7, 0}, {"MAX", 16, 0}, {"offsetof", 9, 0},
	} {
		re := regexp.MustCompile(`\b` + x.Name + `\b`)
		gc := len(re.FindAll(coreT, -1))
		gh := len(re.FindAll(hostT, -1))
		if gc != x.nCore || gh != x.nHost {
			return p.Die("`%s` occurs %d times in the core and %d in the host block, where "+
				"this phase was written against %d and %d", x.Name, gc, gh, x.nCore, x.nHost)
		}
	}
	tvCore := len(w42TV.FindAll(coreT, -1))
	tvHost := len(w42TV.FindAll(hostT, -1))
	if tvCore != 4 || tvHost != 2 {
		return p.Die("`struct timeval` occurs %d times in the core and %d in the host block, "+
			"where this phase was written against 4 and 2", tvCore, tvHost)
	}
	p.Say("the core takes eight things from a header: time_t 6, sig_atomic_t 2, " +
		"uintptr_t 1, struct timeval 4, MIN 7, MAX 16, offsetof 9 -- and size_t 0, " +
		"phase 0a having taken it.  The host block keeps its own sig_atomic_t 3 and " +
		"struct timeval 2")

	// ---- 3. the literals -------------------------------------------------
	// Phase 0a was caught Out by three string literals holding `NULL`.  The
	// lesson is applied rather than assumed: no literal may hold any name
	// this phase substitutes.
	spans, err := edit.LiteralSpans(p, t)
	if err != nil {
		return err
	}
	var bad []string
	for _, s := range spans {
		if w42Names.Match(t[s[0]:s[1]]) {
			bad = append(bad, string(t[s[0]:s[1]]))
		}
	}
	if len(bad) > 0 {
		return p.Die("a literal holds a name this phase substitutes, and no substitution "+
			"below may reach inside a string: %s", strings.Join(bad, " / "))
	}
	p.Sayf("%d string and character literals, NONE holding any of the eight names or "+
		"`gettimeofday` -- so every substitution below is over code", len(spans))

	// ---- 4. the nine libc prototypes -------------------------------------
	// PLAIN, NEVER `static`.  A `static` prototype gives the core an internal
	// function that is never defined; here that is `error: static declaration
	// of 'malloc' follows non-static declaration` and after the move it is a
	// link failure.
	//
	// Their types are the core's statement of the ABI, and <stdlib.h>,
	// <unistd.h> and <time.h> above them are what makes it a statement that
	// can be wrong Out loud.
	const anchor = "typedef typeof(sizeof(0)) usize;\n"
	const block = `
void *malloc(usize n);
void *realloc(void *p, usize n);
void free(void *p);
long time(long *tp);
int getpid(void);
int kill(int pid, int sig);
long write(int fd, const void *buf, usize n);
long labs(long n);
int abs(int n);
`
	if edit.CountAnchorB(t, anchor) != 1 {
		return p.Die("`%s` is not in the file exactly once -- phase 0a put it directly below "+
			"the last `#include` and this phase declares the libc calls beneath it",
			strings.TrimSpace(anchor))
	}
	var usize *graph.Node
	for _, d := range e.Decls("usize") {
		if d.Is("typedef") {
			usize = d
		}
	}
	// ---- 8a. struct timeval -> a TAGLESS core struct ---------------------
	// TAGLESS IS MANDATORY.  A tag would be the core defining a libc name,
	// and C23 would not even complain about it (measured; see the header of
	// this file), so the layout equality has to be asserted instead -- which
	// the check did, against the header that is still above.  The typedef is
	// made first, with the prototypes, so that what selects from an
	// elapsed_T below selects its own members.
	const tv = "typedef struct timeval elapsed_T;"
	if bytes.Count(t, []byte(tv)) != 1 {
		return p.Die("`%s` is not in the file exactly once", tv)
	}
	var elapsedT *graph.Node
	for _, d := range e.Decls("elapsed_T") {
		if d.Is("typedef") {
			elapsedT = d
		}
	}
	if usize == nil || elapsedT == nil {
		return p.Die("the usize or elapsed_T typedef is not a declaration of the file")
	}
	const td = "typedef time_t time_T;"
	if bytes.Count(t, []byte(td)) != 1 {
		return p.Die("`%s` is not in the file exactly once", td)
	}
	var timeT, timet *graph.Node
	for _, d := range e.Decls("time_T") {
		if d.Is("typedef") {
			timeT = d
		}
	}
	for _, d := range e.Decls("time_t") {
		if d.Is("extern-typedef") {
			timet = d
		}
	}
	if timeT == nil || timet == nil {
		return p.Die("time_T or <time.h>'s time_t is not in the file")
	}
	// musl_gettimeofday's prototype and definition go in with them: the
	// five calls below name it.
	const proto = "static void musl_delay(long ms, int interruptible);\n"
	if bytes.Count(t, []byte(proto)) != 1 {
		return p.Die("musl_delay's prototype is not in the file exactly once, so the new one " +
			"has nowhere it belongs")
	}
	const def = "    static void\nmusl_delay(long ms, int interruptible)\n"
	if bytes.Count(t, []byte(def)) != 1 {
		return p.Die("musl_delay's definition is not in the file exactly once")
	}
	var delayProto *graph.Node
	for _, d := range e.FileDecls("musl_delay") {
		if d.Is("def") && delayProto == nil {
			delayProto = d
		}
	}
	delay := e.Defn("musl_delay")
	if delayProto == nil || delay == nil {
		return p.Die("musl_delay is not declared and defined in the file")
	}
	gtFrags := []graph.Frag{
		{At: e.SpotBefore(delayProto), Src: "static void musl_gettimeofday(long *sec, long *usec);"},
		{At: e.SpotBefore(delay), Src: `    static void
musl_gettimeofday(long *sec, long *usec)
{
    struct timeval tv;

    gettimeofday(&tv, nullptr);
    *sec = tv.tv_sec;
    *usec = tv.tv_usec;
}
`}}

	if _, err := e.SpliceC(append([]graph.Frag{{At: e.SpotAfter(usize), Src: block},
		{At: e.SpotOf(elapsedT), Src: "typedef struct {\n    long        tv_sec;\n    long        tv_usec;\n} elapsed_T;"},
		{At: e.SpotOf(timeT), Src: "typedef long        time_T;"}}, gtFrags...)...); err != nil {
		return p.Die("the libc prototypes and elapsed_T: %v", err)
	}
	// what is declared an elapsed_T is typed again, the tagless struct now:
	// each local, static and member whose type form names it
	elapsedT = nil
	for _, d := range e.Decls("elapsed_T") {
		if d.Is("typedef") {
			elapsedT = d
		}
	}
	for _, u := range e.Uses(elapsedT) {
		var d, form *graph.Node
		for x := u; e.Parent(x) != nil && d == nil; x = e.Parent(x) {
			q := e.Parent(x)
			switch {
			case q.Is("def") && graph.DeclType(q) == x:
				d, form = q, x
			case q.Is("struct") || q.Is("union"):
				d, form = x, x.Kids[1] // a member, (NAME TYPE ...)
			case q.Is("defn") || q.Is("fn"):
				x = nil // a function's parameter or result: the substitutions below make them
			}
			if x == nil {
				break
			}
		}
		if d == nil {
			continue
		}
		if _, err := e.Retype(d, graph.Lisp(form).String()); err != nil {
			return p.Die("%s: %v", graph.Lisp(d), err)
		}
	}
	// each call of the nine now names the core's prototype, as cc resolves
	// it: the headers' declarations are no longer what the file's uses mean
	for _, name := range []string{"malloc", "realloc", "free", "time", "getpid", "kill", "write", "labs", "abs"} {
		var from, to *graph.Node
		for _, d := range e.Decls(name) {
			switch {
			case d.Is("extern"):
				from = d
			case d.Is("def") && to == nil:
				to = d
			}
		}
		if to == nil {
			return p.Die("%s's prototype is not in the file", name)
		}
		if from == nil {
			continue
		}
		if _, err := e.RetargetUses(from, to); err != nil {
			return p.Die("%s: %v", name, err)
		}
	}
	p.Say("nine plain prototypes below the usize typedef -- malloc realloc free time " +
		"getpid kill write labs abs -- and NOT ONE of them `static`.  gettimeofday is " +
		"the tenth and is the one that cannot stay: its argument is a struct")

	// ---- 5. time_t -> time_T, and the core owns the width ----------------
	// the typedef was made in its own place with the prototypes (the forms
	// that name time_T typed again as cc types them, which RETYPE would
	// clear where it cannot say them, a function's result among them)
	timeT = nil
	for _, d := range e.Decls("time_T") {
		if d.Is("typedef") {
			timeT = d
		}
	}
	nTime := 0
	for _, u := range e.Uses(timet) {
		if err := e.RetargetAs(u, 0, timeT); err != nil {
			return p.Die("time_t -> time_T: %v", err)
		}
		nTime++
	}
	if nTime != 5 {
		return p.Die("%d further `time_t` were rewritten where 5 were counted", nTime)
	}
	p.Sayf("`typedef time_t time_T;` -> `typedef long time_T;` and %d further `time_t` "+
		"-> `time_T`.  `long time(long *)` above is what pins the width against "+
		"<time.h>", nTime)

	// ---- 6. sig_atomic_t -> volatile int, in the core only ---------------
	for _, x := range []struct{ old, name string }{
		{"static volatile sig_atomic_t full_screen ", "full_screen"},
		{"static volatile sig_atomic_t got_int ", "got_int"},
	} {
		if bytes.Count(t, []byte(x.old)) != 1 {
			return p.Die("`%s` is not in the file exactly once", strings.TrimSpace(x.old))
		}
		var d *graph.Node
		for _, f := range e.FileDecls(x.name) {
			if f.Is("def") && graph.Matches(clisp.MustPattern("(volatile sig_atomic_t)"), graph.DeclType(f)) {
				d = f
			}
		}
		if d == nil {
			return p.Die("`%s` is not a `volatile sig_atomic_t` of the file", x.name)
		}
		if _, err := e.Retype(d, "(volatile int)"); err != nil {
			return p.Die("%s: %v", x.name, err)
		}
	}
	p.Say("the core's two `volatile sig_atomic_t` objects -- full_screen and got_int " +
		"-- are spelled `volatile int`, and the host block's three are left alone")

	// ---- 7. uintptr_t -> usize -------------------------------------------
	const up = "(unsigned long long)(uintptr_t)p"
	if bytes.Count(t, []byte(up)) != 1 {
		return p.Die("`%s` is not in the file exactly once -- it is the one cast that names "+
			"uintptr_t", up)
	}
	var ups []*graph.Node
	for _, d := range e.Decls("uintptr_t") {
		if d.Is("extern-typedef") {
			ups = append(ups, e.Uses(d)...)
		}
	}
	if len(ups) != 1 || e.Function(ups[0]) != e.Defn("musl_fmtptr") {
		return p.Die("uintptr_t is named %d times, where the one cast in musl_fmtptr is its one use", len(ups))
	}
	if err := e.RetargetAs(ups[0], 0, usize); err != nil {
		return p.Die("uintptr_t -> usize: %v", err)
	}
	e.Rederive(e.Parent(ups[0]))
	p.Say("the one `(uintptr_t)` cast, in musl_fmtptr, is `(usize)` -- the same type, " +
		"and the check asserts that with _Generic against <stdint.h>")

	// ---- 8b. elapsed() takes an elapsed_T --------------------------------
	for _, x := range []string{"static long elapsed(struct timeval *start_tv);",
		"elapsed(struct timeval *start_tv)\n{", "    struct timeval now_tv;"} {
		if bytes.Count(t, []byte(x)) != 1 {
			return p.Die("`%s` is not in the file exactly once",
				strings.ReplaceAll(x, "\n", "\\n"))
		}
	}
	quiet := graph.NewVerbs("headers", e, io.Discard)
	quiet.SubstituteC(
		graph.Subst{Old: "static long elapsed(struct timeval *start_tv);",
			New: "static long elapsed(elapsed_T *start_tv);", N: 1, What: "elapsed()'s prototype"},
		graph.Subst{Old: "elapsed(struct timeval *start_tv)\n{\n    struct timeval now_tv;",
			New: "elapsed(elapsed_T *start_tv)\n{\n    elapsed_T       now_tv;", N: 1, What: "elapsed()'s definition"})
	if quiet.Err != nil {
		return quiet.Err
	}

	// ---- 8c. the five gettimeofday calls, and musl_gettimeofday ----------
	// One import for the rest: the five calls, musl_gettimeofday's prototype
	// and definition, the nine offsetof and the twenty-three MIN and MAX.
	var frags []graph.Frag
	var gt *graph.Node
	for _, d := range e.Decls("gettimeofday") {
		if d.Is("extern") {
			gt = d
		}
	}
	nGt := 0
	if gt != nil {
		own := e.Defn("musl_gettimeofday") // the host's own call, which is what the five go through
		for _, u := range e.Uses(gt) {
			if e.Function(u) == own {
				continue
			}
			c := e.Parent(u)
			if c == nil || !c.Is("call") || c.Kids[1] != u || len(c.Kids) != 4 || !c.Kids[3].Is("nullptr") && c.Kids[3].Atom != "nullptr" ||
				!c.Kids[2].Is("addr") {
				return p.Die("a use of gettimeofday is not `gettimeofday(&X, nullptr)`")
			}
			x := c.Kids[2].Kids[1]
			if x.Is("paren") {
				x = x.Kids[1]
			}
			xc, err := clisp.PrintExpr(graph.Lisp(x))
			if err != nil {
				return err
			}
			src := "musl_gettimeofday(&" + xc + ".tv_sec, &" + xc + ".tv_usec)"
			if e.Item(c) == c {
				src += ";" // a statement of its own
			}
			frags = append(frags, graph.Frag{At: e.SpotOf(c), Src: src})
			nGt++
		}
	}
	if nGt != 5 {
		return p.Die("%d `gettimeofday(&X, nullptr)` calls were rewritten where 5 were "+
			"counted", nGt)
	}
	// ---- 9. offsetof -> __builtin_offsetof -------------------------------
	// GOALS.md II.4c settled this.  The plain-C alternative
	// `(usize)&(((T *)0)->m)` was measured to compile, to run, and to
	// static_assert equal to libc's offsetof -- but `-Wpedantic` says it is
	// not an integer constant expression, so it could never be an enumerator.
	// None of these nine needs to be one, so it stays a real option and not a
	// reason to change: one gcc extension in one construct is cheaper to
	// explain than a UB-by-the-letter idiom in nine places.
	//
	// ---- 10. MIN and MAX, expanded to the text the header gives ----------
	probe, err := os.ReadFile(args[0])
	if err != nil {
		return p.Die("the preprocessed probe could not be read: %v", err)
	}
	tmpl := map[string]string{}
	for _, raw := range strings.Split(string(probe), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "ZZA") || !strings.Contains(line, "ZZB") {
			return p.Die("the preprocessed probe line %s does not mention both arguments -- "+
				"the expansion could not be turned into a template", edit.PyRepr(line))
		}
		if strings.Contains(line, "<") {
			tmpl["MIN"] = line
		} else {
			tmpl["MAX"] = line
		}
	}
	if len(tmpl) != 2 || tmpl["MIN"] == "" || tmpl["MAX"] == "" {
		return p.Die("the preprocessed probe gave %d templates and not one for MIN and one "+
			"for MAX", len(tmpl))
	}
	// minmax_lines is counted on the view before the expansion
	seen := map[int]bool{}
	for _, m := range w42MM.FindAllIndex(t, -1) {
		seen[bytes.Count(t[:m[0]], []byte{'\n'})] = true
	}
	minmaxLines := len(seen)
	nOff, nMin, nMax := 0, 0, 0
	for _, f := range e.Graph().Forms {
		var bad error
		graph.Walk(f, func(x *graph.Node) bool {
			name, margs, ok := graph.MacroCall(x)
			if !ok || bad != nil {
				return bad == nil
			}
			switch name {
			case "offsetof":
				frags = append(frags, graph.Frag{At: e.SpotOf(x), Src: "__builtin_offsetof(" + strings.Join(margs, ", ") + ")"})
				nOff++
			case "MIN", "MAX":
				if len(margs) != 2 {
					bad = p.Die("`%s` with %d arguments, so it is not the two-argument macro this phase expands", name, len(margs))
					return false
				}
				frags = append(frags, graph.Frag{At: e.SpotOf(x),
					Src: strings.ReplaceAll(strings.ReplaceAll(tmpl[name], "ZZA", strings.TrimSpace(margs[0])), "ZZB", strings.TrimSpace(margs[1]))})
				if name == "MIN" {
					nMin++
				} else {
					nMax++
				}
			}
			return true
		})
		if bad != nil {
			return bad
		}
	}
	if nOff != 9 {
		return p.Die("%d `offsetof` were rewritten where 9 were counted", nOff)
	}
	if nMin != 7 || nMax != 16 {
		return p.Die("%d MIN and %d MAX were expanded where 7 and 16 were counted", nMin, nMax)
	}
	if _, err := e.SpliceC(frags...); err != nil {
		return p.Die("the gettimeofday calls, musl_gettimeofday, offsetof, MIN and MAX: %v", err)
	}
	p.Sayf("`elapsed_T` is the core's own TAGLESS `struct { long tv_sec; long tv_usec; "+
		"}`, the three other `struct timeval` in the core are it, and the %d "+
		"`gettimeofday(&X, nullptr)` calls go through `musl_gettimeofday(long *, long "+
		"*)` -- defined in the host block above musl_delay, which is inside the region "+
		"zhostonly reads", nGt)
	p.Sayf("%d `offsetof` -> `__builtin_offsetof`, which is what <stddef.h> expands it "+
		"to here.  The check asserts the two are equal at all six types", nOff)
	p.Sayf("%d MIN and %d MAX on %d lines expanded to the header's own text -- %s and "+
		"%s, read back through the preprocessor and not written into this program",
		nMin, nMax, minmaxLines, tmpl["MIN"], tmpl["MAX"])
	if t, err = e.Graph().C(); err != nil {
		return err
	}

	// ---- 11. what the file is now ----------------------------------------
	L := bytes.Split(t, []byte{'\n'})
	var host2 []int
	for i, l := range L {
		if bytes.HasPrefix(l, []byte(host)) {
			host2 = append(host2, i)
		}
	}
	if len(host2) != 1 {
		return p.Die("the host block no longer begins exactly once with %s", edit.PyRepr(host))
	}
	ncore := bytes.Join(L[11:host2[0]], []byte{'\n'})
	nhost := bytes.Join(L[host2[0]:], []byte{'\n'})
	for _, name := range []string{"time_t", "sig_atomic_t", "uintptr_t", "size_t",
		"MIN", "MAX", "offsetof"} {
		n := len(regexp.MustCompile(`\b`+name+`\b`).FindAll(ncore, -1))
		if n > 0 {
			return p.Die("`%s` still occurs %d times in the core", name, n)
		}
	}
	if w42TV.Match(ncore) {
		return p.Die("`struct timeval` still occurs in the core")
	}
	if len(regexp.MustCompile(`\bsig_atomic_t\b`).FindAll(nhost, -1)) != 3 {
		return p.Die("the host block no longer has its three `sig_atomic_t`")
	}
	if k := len(w42TV.FindAll(nhost, -1)); k != 3 {
		return p.Die("the host block should have three `struct timeval` -- its two and "+
			"musl_gettimeofday's -- and has %d", k)
	}
	nd := 0
	for _, l := range L {
		if bytes.HasPrefix(l, []byte("#")) {
			nd++
		}
	}
	if nd != nInc {
		return p.Die("the file no longer has exactly eleven directives")
	}
	p.Say("the core is clean: size_t, time_t, sig_atomic_t, uintptr_t, struct timeval, " +
		"MIN, MAX and offsetof are ALL at 0 above the host block, the eleven directives " +
		"are where they were, and the only header-supplied names left are the twelve " +
		"constants phase 43 takes with the move")
	return nil
}
