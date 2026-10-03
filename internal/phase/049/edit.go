package p049

// Whim phase 49 (formerly 119) -- the core's libc prototype block empties.  GOALS.md II.4c, GOALS.md.
//
// TWO LINES ARE LEFT IN THE CORE'S BLOCK OF ORDINARY DECLARATIONS AND THIS PHASE TAKES
// BOTH.  Phase 49b took `malloc`, `free` and `write`, the three the editor uses so
// constantly that nobody had looked at them, and wrote its own programs so that the phase
// which empties the block would need no further edit to the machinery:
//
// int getpid(void);
// int kill(int pid, int sig);
//
// THEY ARE REACHED TWO DIFFERENT WAYS AND ONLY ONE OF THEM NEEDS A HOST CALL.
//
// getpid   IS AVOIDABLE OUTRIGHT, not moved.  `mch_get_pid()` is `return (long)getpid();`
// and has exactly ONE caller, `long_to_char(mch_get_pid(), b0p->b0_pid)` in
// ml_open().  `b0_pid` is the process id written into block zero of a swap
// file, and it has exactly TWO mentions in the whole file -- its own
// declaration and that write.  IT IS WRITE-ONLY: nothing in any build of
// whim-vim reads it back, because the swap file it belonged to is a disk
// format this editor has not had since the filesystem phases took every way
// to name a file.  So the write goes, mch_get_pid() goes with it, and `getpid`
// leaves the core without anybody calling a host.  Phase 39 saw this
// coming and said so -- "`b0_pid` is written and never read, so one line frees
// it whenever block zero is somebody's phase".  This is that phase.
// kill     NEEDS A HOST CALL.  Its one core site is in vim_handle_signal():
// `kill(getpid(), got_signal);`, re-raising a deadly signal that arrived while
// the editor was not reading.  It becomes `host_raise(got_signal);`, and the
// host defines
//
// static void host_raise(int sig);
//
// beside host_exit, host_message, host_time, host_alloc, host_free and
// host_write.  IT TAKES NO PID, and that is the whole shape of it: a core that
// can no longer ask for its own process id must not be handed one.  What the
// core says is "raise this signal on me"; WHICH process that is, is the host's
// idea, exactly as fd 1 is the host's idea of where the screen is
// (host_write, phase 49b) and fd 0 the host's idea of where the keyboard is
// (musl_read_input, phase 39).
//
// THE DEFINITION GOES INSIDE THE HOST BLOCK AND NOT BELOW IT, which is phase 42's lesson
// about musl_gettimeofday and phase 44's about musl_now_ms.  `zhostonly` reads the
// host region as the lines from `host_winch_pending` to musl_suspend's last brace, and
// host_raise's body says `kill` and `getpid`; a definition below musl_suspend would put
// two host words outside the region and the tool would refuse.  So it is written
// immediately above musl_suspend, which is the last function of that region.
//
// THE NAME PASSES `zhostonly`'s PATTERN FOR THE REASON `musl_gettimeofday` DOES.
// `raise` is in that tool's vocabulary and `\braise\b` cannot match inside `host_raise`,
// because `_` is a word character -- the same trick that lets `musl_gettimeofday` sit in
// the host block while the bare `gettimeofday` is a word the core may not say.  And
// host_raise's body calls `kill(getpid(), sig)` rather than libc's `raise()`: `raise` has
// not been an undefined symbol of this file since phase 39, and a wrapper that reached
// for it would ADD a libc symbol in a phase whose whole subject is the core's last two.
//
// WHAT THIS PHASE IS FOR.  When the block is empty the core names no libc function at
// all.  The edit does not assert that -- it finds the block, takes the two lines it owns
// out of it, and prints what is left, exactly as phase 49b's does; the check states the
// claim as a measurement of the OUTPUT, and computes it from `make editor.c`'s cut rather
// than from the block, because those are two different assertions and only the first is
// the claim.  A bare declaration is invisible to the cut -- gcc warns `used but never
// defined` for a `static` function and says nothing about an `extern` one -- so what the
// check measures is `nm -u` of an object of the CUT ALONE, which is the set of names the
// core needs from outside itself.
//
// THE FOLD THAT IS NOT THERE, SURVEYED AND NOT TAKEN.  Phase 38a removed deathtrap()'s
// `entered >= 3` ladder as code no build of whim-vim could reach, which leaves `entered`
// able to reach 2 and no further, and the question was put whether that makes anything
// around the `if (entered == 2)` arm foldable.  Measured, it does not: `entered` has
// exactly three reachable values and EVERY ONE OF THEM IS READ.  0 is read by the guard
// `if (entered == 0 && ...)`, which is what distinguishes the first entry from a nested
// one; 1 and 2 are told apart TWICE -- by `if (entered == 2)`, the double-signal arm
// which calls getout(1) and never returns, and by `v_dying = entered;`, whose value
// reaches getout()'s two `if (v_dying <= 1)` tests and selects the buffer cleanup there.
// So the counter is genuinely three-valued, no two of its states are interchangeable, and
// there is no fold to take.  This edit therefore leaves deathtrap() alone, and the check
// asserts that as a byte comparison of the function in and out rather than leaving it to
// be believed.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check records from it.
//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// cut is on the program's graph, its report the text version's, which the
// plan ran until then (history keeps it); the @state argument and the two
// files it wrote there are gone, since nothing read them once the checks
// went (448e9a8):
//
//   - the include checks are the include forms, the block of ordinary
//     declarations the core forms around `int getpid(void);`
//     (vimtext.OrdinaryBlock);
//   - the classes every mention of getpid and kill above the boundary must
//     fall in are the edges: getpid's prototype used in mch_get_pid() and in
//     the re-raise, kill's in the re-raise alone, the re-raise the one
//     statement `kill(getpid(), SIG);` of the core; the mention counts on the
//     C view are the text's own, beside them;
//   - "b0_pid is write-only" is the member's one use, the selection inside
//     the one call of mch_get_pid(), `long_to_char(mch_get_pid(), p->b0_pid);`,
//     a statement of its own, which is cut;
//   - the re-raise is made again by FRAG, `host_raise(SIG);`, after
//     host_raise's prototype goes in after host_time's, and its definition
//     above musl_suspend()'s, FRAG's both;
//   - the two prototypes go by DeleteForHeader: musl_suspend()'s kill becomes
//     <signal.h>'s, and mch_get_pid()'s getpid, which the text left
//     undeclared for the sweep, is left dangling for the collection, which
//     takes mch_get_pid() with it, as it took it from the text;
//   - the host region is asked of the forms (host_raise's definition between
//     host_winch_pending's and musl_suspend()'s); the report's line numbers
//     are the C view's.

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim49", Edit) }

// w49Lines are the 1-based numbers of the lines of L in [lo, hi) that
// mention the word.
func w49Lines(L []string, lo, hi int, word string) []string {
	var out []string
	for i := lo; i < hi && i < len(L); i++ {
		if edit.MentionCount([]byte(L[i]), word) > 0 {
			out = append(out, fmt.Sprint(i+1))
		}
	}
	return out
}

// Edit is phase 49 on the graph: getpid and kill leave the core, the
// deferred deadly signal re-raised through host_raise().
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noclib", e, w)
	incs, err := vimtext.IncludeRun(e)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	text := v.Text()
	core, host, bound := vimtext.SplitCore(text)
	L := strings.Split(string(text), "\n")
	linesBefore := len(L) - 1
	v.Sayf("the boundary is line %d, the first of the eleven `#include`s, and there is not a "+
		"directive above it", bound+1)

	block, blockBefore, err := vimtext.OrdinaryBlock(e, w49Go[0])
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	for _, line := range w49Go {
		if !edit.Contains(blockBefore, line) {
			v.Die("`%s` is not in the core's block of ordinary declarations, which is %s",
				line, strings.Join(blockBefore, " / "))
			return v.Done()
		}
	}
	var blockAfter []string
	for _, l := range blockBefore {
		if !edit.Contains(w49Go, l) {
			blockAfter = append(blockAfter, l)
		}
	}
	names := make([]string, len(block))
	decl := map[string]*graph.Node{}
	declLine := map[string]int{}
	for i, f := range block {
		names[i] = graph.DeclName(f)
		decl[names[i]] = f
		for j, l := range L[:bound] {
			if l == blockBefore[i] {
				declLine[names[i]] = j
				break
			}
		}
	}
	v.Sayf("the core's block of ordinary declarations is lines %d-%d, %d of them: %s",
		declLine[names[0]]+1, declLine[names[len(names)-1]]+1, len(blockBefore), strings.Join(names, " "))

	// the classes, by edge
	gp := e.Defn("mch_get_pid")
	if gp == nil || !e.InCore(gp) {
		v.Die("mch_get_pid() is not defined above the boundary, and this phase is about what the CORE says")
		return v.Done()
	}
	var raise *graph.Node
	for _, u := range e.Uses(decl["kill"]) {
		if !e.InCore(u) {
			continue
		}
		call := e.Parent(u)
		if raise != nil || call == nil || !call.Is("call") || len(call.Kids) != 4 || e.Item(call) != call ||
			!call.Kids[2].Is("call") || len(call.Kids[2].Kids) != 2 || call.Kids[3].IsList() {
			v.Die("`kill` is used above the boundary other than once, as `kill(getpid(), <name>);`, the "+
				"deferred deadly signal vim_handle_signal() re-raises: %s", graph.Lisp(call))
			return v.Done()
		}
		raise = call
	}
	if raise == nil || e.Function(raise) == nil || graph.DeclName(e.Function(raise)) != "vim_handle_signal" {
		v.Die("`kill(getpid(), <name>);` is not one statement of vim_handle_signal() above the boundary")
		return v.Done()
	}
	inner := raise.Kids[2].Kids[1]
	deferred := raise.Kids[3]
	for _, u := range e.Uses(decl["getpid"]) {
		if u != inner && e.Function(u) != gp {
			v.Die("`getpid` is used at %s, which is in none of the classes this phase rewrites "+
				"(declaration, mch_get_pid(), the re-raise) -- and this phase will not delete a "+
				"declaration whose every use it cannot account for", graph.Lisp(e.Parent(u)))
			return v.Done()
		}
	}
	gpLo, gpHi := -1, -1
	for i, l := range L[:bound] {
		if strings.HasPrefix(l, "mch_get_pid(") && i+1 < len(L) && L[i+1] == "{" {
			gpLo = i - 1
			for gpHi = i; gpHi < len(L) && L[gpHi] != "}"; gpHi++ {
			}
			gpHi++
		}
	}
	rl := -1
	for i, l := range L[:bound] {
		if strings.TrimSpace(l) == fmt.Sprintf("kill(getpid(), %s);", deferred.Atom) {
			rl = i
		}
	}
	for _, c := range []struct {
		Name    string
		mention int
		cls     [][2]string
	}{
		{"getpid", 3, [][2]string{{"declaration", fmt.Sprint(declLine["getpid"] + 1)},
			{"mch_get_pid()", strings.Join(w49Lines(L, gpLo, gpHi, "getpid"), " ")},
			{"the re-raise", fmt.Sprint(rl + 1)}}},
		{"kill", 2, [][2]string{{"declaration", fmt.Sprint(declLine["kill"] + 1)},
			{"the re-raise", fmt.Sprint(rl + 1)}}},
	} {
		if n := edit.MentionCount(core, c.Name); n != c.mention {
			v.Die("`%s` is said %d times above the boundary, where its declaration and the uses its "+
				"classes hold are %d", c.Name, n, c.mention)
			return v.Done()
		}
		var parts []string
		for _, cl := range c.cls {
			parts = append(parts, cl[0]+" at "+cl[1])
		}
		v.Sayf("`%s` above the boundary: %d mentions, and every one falls in a class this "+
			"phase rewrites -- %s", c.Name, c.mention, strings.Join(parts, ", "))
	}
	for _, nh := range []struct {
		Name  string
		nhost int
	}{{"getpid", 0}, {"kill", 1}} {
		if k := edit.MentionCount(host, nh.Name); k != nh.nhost {
			v.Die("`%s` has %d mentions below the boundary and this phase was written against %d -- "+
				"musl_suspend() stops the process group with `kill(0, SIGTSTP)` and nothing below "+
				"the boundary asks for a pid", nh.Name, k, nh.nhost)
			return v.Done()
		}
	}
	if edit.MentionCount(text, "host_raise") > 0 {
		v.Die("`host_raise` is already a name in this file")
		return v.Done()
	}

	// mch_get_pid()'s one call, and b0_pid
	var protoGP []*graph.Node
	for _, d := range e.Decls("mch_get_pid") {
		if d != gp {
			protoGP = append(protoGP, d)
		}
	}
	var callers []*graph.Node
	for _, d := range append(protoGP, gp) {
		for _, u := range e.Uses(d) {
			if e.Function(u) != gp {
				callers = append(callers, u)
			}
		}
	}
	if len(protoGP) != 1 || len(callers) != 1 {
		v.Die("mch_get_pid() has %d forward declarations and %d call sites outside its own definition, "+
			"and this phase needs one of each -- the prototype the sweep takes, and ml_open()'s write "+
			"of b0_pid", len(protoGP), len(callers))
		return v.Done()
	}
	write := e.Parent(e.Parent(callers[0]))
	if write == nil || !write.Is("call") || len(write.Kids) != 4 || write.Kids[1].Atom != "long_to_char" ||
		write.Kids[2] != e.Parent(callers[0]) || !write.Kids[3].Is("->") || e.Item(write) != write ||
		graph.DeclName(e.Function(write)) != "ml_open" {
		v.Die("mch_get_pid()'s one call site is %s, and this phase was written against ml_open()'s "+
			"`long_to_char(mch_get_pid(), b0p->b0_pid);`", graph.Lisp(write))
		return v.Done()
	}
	field := write.Kids[3].Kids[2].Ref()
	if field == nil || len(e.Uses(field)) != 1 || edit.MentionCount(text, "b0_pid") != 2 {
		v.Die("`b0_pid` has %d mentions and this phase needs exactly two, its own declaration and the "+
			"one write", edit.MentionCount(text, "b0_pid"))
		return v.Done()
	}
	fieldLine, writeLine := 0, 0
	for i, l := range L {
		if strings.Contains(l, "b0_pid") {
			if strings.Contains(l, "long_to_char") {
				writeLine = i + 1
			} else {
				fieldLine = i + 1
			}
		}
	}
	v.Sayf("`b0_pid` is WRITE-ONLY: 2 mentions in the whole file, its declaration at line %d "+
		"and ml_open()'s write at line %d, and not one read.  So the write is the entry "+
		"point, mch_get_pid() (lines %d-%d) is its only feeder, and `getpid` leaves the core "+
		"without a host call", fieldLine, writeLine, gpLo+1, gpHi)
	if err := e.Delete(write); err != nil {
		v.Die("ml_open()'s write of b0_pid: %v", err)
		return v.Done()
	}

	// host_raise: its prototype, the re-raise, the old prototypes, the definition
	var ht []*graph.Node
	for _, f := range e.Core() {
		if graph.DeclName(f) == "host_time" && f.Is("def") {
			if c, err := vimtext.FormC(f); err == nil && c == "static long host_time(void);" {
				ht = append(ht, f)
			}
		}
	}
	if len(ht) != 1 {
		v.Die("the last of the core -> host prototypes, `static long host_time(void);`, occurs %d times, "+
			"expected 1 -- the boundary is ONE block, and this belongs at the end of it", len(ht))
		return v.Done()
	}
	made, err := e.SpliceC(graph.Frag{At: e.SpotAfter(ht[0]), Src: w49Proto},
		graph.Frag{At: e.SpotOf(raise), Src: "host_raise($sig);", Holes: graph.Bindings{"sig": deferred}})
	if err != nil {
		v.Die("host_raise's prototype and vim_handle_signal()'s re-raise of a deferred deadly signal: %v", err)
		return v.Done()
	}
	proto := made[0][0]
	suspend := e.Defn("musl_suspend")
	if suspend == nil || !e.InHost(suspend) {
		v.Die("musl_suspend() is not defined below the boundary, the last function of the host region")
		return v.Done()
	}
	// kill's every use is the header's now, its definition's too; getpid's one
	// left in the core is mch_get_pid()'s, for the collection
	if _, dangling, err := e.DeleteForHeader([]*graph.Node{decl["getpid"], decl["kill"]}, true,
		func() []graph.Frag { return []graph.Frag{{At: e.SpotBefore(suspend), Src: w49Def}} }); err != nil {
		v.Die("%v", err)
		return v.Done()
	} else if len(dangling) != 1 || e.Function(dangling[0]) != gp || dangling[0].Atom != "getpid" {
		v.Die("`getpid` and `kill` are left used %d times, where mch_get_pid()'s one getpid, for the "+
			"collection, was expected", len(dangling))
		return v.Done()
	}

	text = v.Text()
	core, host, bound = vimtext.SplitCore(text)
	L = strings.Split(string(text), "\n")
	for _, r := range []struct {
		Name         string
		ncore, nhost int
	}{{"getpid", 1, 1}, {"kill", 0, 2}, {"mch_get_pid", 2, 0}} {
		if c, h := edit.MentionCount(core, r.Name), edit.MentionCount(host, r.Name); c != r.ncore || h != r.nhost {
			v.Die("`%s` ends at %d mentions above the boundary and %d below, expected %d and %d",
				r.Name, c, h, r.ncore, r.nhost)
			return v.Done()
		}
	}
	if k := edit.MentionCount(text, "host_raise"); k != 3 {
		v.Die("`host_raise` has %d mentions and it must have three -- its prototype, the one call site "+
			"it took over from `kill` and its definition", k)
		return v.Done()
	}
	if k := edit.MentionCount(text, "b0_pid"); k != 1 || len(e.Uses(field)) != 0 {
		v.Die("`b0_pid` has %d mentions and the edit leaves exactly one, its own declaration, for the "+
			"sweep to take", k)
		return v.Done()
	}
	var have []string
	for _, f := range e.Core() {
		if vimtext.IsOrdinaryDecl(f) {
			c, _ := vimtext.FormC(f)
			have = append(have, c)
		}
	}
	if strings.Join(have, "\x00") != strings.Join(blockAfter, "\x00") {
		v.Die("the ordinary declarations left above the boundary are %s and the input's block minus the "+
			"two is %s", vimtext.JoinOrNone(have), vimtext.JoinOrNone(blockAfter))
		return v.Done()
	}
	if len(blockAfter) > 0 {
		var left []string
		for _, l := range blockAfter {
			left = append(left, vimtext.DeclNameRe.ReplaceAllString(l, "$1"))
		}
		v.Sayf("the core's block of ordinary declarations is %d lines and was %d: %s remain, "+
			"and each is a libc function some LATER phase owns",
			len(blockAfter), len(blockBefore), strings.Join(left, " "))
	} else {
		v.Sayf("THE CORE'S BLOCK OF ORDINARY DECLARATIONS IS EMPTY: it was %d lines and it is "+
			"now none.  Above the first `#include` there is no declaration that is not "+
			"`static`, and the check states what that means as a measurement of the cut "+
			"rather than as a sentence written here", len(blockBefore))
	}

	pos := map[*graph.Node]int{}
	for i, f := range e.Graph().Forms {
		pos[f] = i
	}
	df := e.Defn("host_raise")
	uses := e.Uses(proto)
	if df == nil || len(uses) != 1 || len(e.Decls("host_raise")) != 2 {
		v.Die("`host_raise` has %d declarations, a definition %v and %d call sites",
			len(e.Decls("host_raise")), df != nil, len(uses))
		return v.Done()
	}
	if !(pos[proto] < pos[e.TopForm(uses[0])] && pos[e.TopForm(uses[0])] < pos[df]) {
		v.Die("`host_raise`: the prototype must be above the call and the definition below it")
		return v.Done()
	}
	var hb *graph.Node
	for _, d := range e.Decls("host_winch_pending") {
		if d.Is("def") && e.InHost(d) {
			hb = d
		}
	}
	if hb == nil || !(pos[e.FirstInclude()] < pos[df] && pos[hb] <= pos[df] && pos[df] < pos[suspend]) {
		v.Die("`host_raise` must be defined below the boundary and INSIDE the host region, from " +
			"host_winch_pending to musl_suspend(): its body says `kill` and `getpid`, and every " +
			"mention of a host word lives in that region")
		return v.Done()
	}
	prLine, callLine, defLine, hbLine, end := 0, 0, 0, 0, 0
	for i, l := range L {
		switch {
		case l == w49Proto:
			prLine = i + 1
		case strings.HasPrefix(l, "host_raise("):
			defLine = i + 1
		case strings.Contains(l, "host_raise("):
			callLine = i + 1
		case strings.HasPrefix(l, "static volatile sig_atomic_t host_winch_pending"):
			hbLine = i + 1
		case strings.HasPrefix(l, "musl_suspend("):
			for end = i; end < len(L) && L[end] != "}"; end++ {
			}
		}
	}
	v.Sayf("`host_raise`: prototype line %d, one call site at line %d, definition line %d, "+
		"which is below the boundary at %d and inside the %d-line host region "+
		"zhostonly reads", prLine, callLine, defLine, bound+1, end+2-hbLine)

	if _, err := vimtext.IncludeRun(e); err != nil || len(e.Includes()) != len(incs) || e.FirstInclude() != incs[0] {
		v.Die("the output does not have the same %d contiguous include forms -- this phase adds a "+
			"DECLARATION and a DEFINITION, never a directive", len(incs))
		return v.Done()
	}
	v.Sayf("%d -> %d lines before the collection, the eleven #includes untouched at line %d.  "+
		"The collection is left mch_get_pid(), its prototype and `b0_pid` to find",
		linesBefore, len(L)-1, bound+1)
	return v.Done()
}
