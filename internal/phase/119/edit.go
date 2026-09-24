package p119

// Whim phase 119 -- the core's libc prototype block empties.  GOALS.md II.4c, GOALS.md.
//
// TWO LINES ARE LEFT IN THE CORE'S BLOCK OF ORDINARY DECLARATIONS AND THIS PHASE TAKES
// BOTH.  Phase 118 took `malloc`, `free` and `write`, the three the editor uses so
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
// leaves the core without anybody calling a host.  Phase 103 saw this
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
// (host_write, phase 118) and fd 0 the host's idea of where the keyboard is
// (musl_read_input, phase 103).
//
// THE DEFINITION GOES INSIDE THE HOST BLOCK AND NOT BELOW IT, which is phase 109's lesson
// about musl_gettimeofday and phase 111's about musl_now_ms.  `zhostonly` reads the
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
// not been an undefined symbol of this file since phase 103, and a wrapper that reached
// for it would ADD a libc symbol in a phase whose whole subject is the core's last two.
//
// WHAT THIS PHASE IS FOR.  When the block is empty the core names no libc function at
// all.  The edit does not assert that -- it finds the block, takes the two lines it owns
// out of it, and prints what is left, exactly as phase 118's does; the check states the
// claim as a measurement of the OUTPUT, and computes it from `make editor.c`'s cut rather
// than from the block, because those are two different assertions and only the first is
// the claim.  A bare declaration is invisible to the cut -- gcc warns `used but never
// defined` for a `static` function and says nothing about an `extern` one -- so what the
// check measures is `nm -u` of an object of the CUT ALONE, which is the set of names the
// core needs from outside itself.
//
// THE FOLD THAT IS NOT THERE, SURVEYED AND NOT TAKEN.  Phase 100 removed deathtrap()'s
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
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.RegisterArgs("whim119", Edit) }

// Whim119 leaves the core naming no libc function at all.  The last two go by
// DIFFERENT routes: `getpid` is avoidable outright, its one caller feeding a
// `b0_pid` that nothing reads; `kill` is moved, becoming host_raise(), which
// takes no pid because a core that cannot ask for its own process id must not be
// handed one.
func Edit(text []byte, w io.Writer, args []string) ([]byte, error) {
	p := edit.Ph{Tag: "noclib", W: w}
	if len(args) != 1 {
		return nil, p.Die("usage: edit whim119 <file> <state-dir>")
	}
	state := args[0]
	t := string(text)

	mentions := func(s, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(s, -1))
	}
	blankRuns := func(s string) int {
		L := strings.Split(s, "\n")
		n := 0
		for i := 1; i < len(L); i++ {
			if L[i] == "" && L[i-1] == "" {
				n++
			}
		}
		return n
	}
	swap := func(old, new, what, why string) error {
		if c := strings.Count(t, old); c != 1 {
			return p.Die("%s occurs %d times, expected 1 -- %s", what, c, why)
		}
		t = strings.Replace(t, old, new, 1)
		return nil
	}
	// defn is the half-open line range of a definition in this tree's ONE shape,
	// the same shape zhostonly reads.
	defn := func(L []string, name string) (int, int, error) {
		head := regexp.MustCompile(`^` + name + `\s*\(`)
		var heads []int
		for i, l := range L {
			if head.MatchString(l) && i+1 < len(L) && L[i+1] == "{" {
				heads = append(heads, i)
			}
		}
		if len(heads) != 1 {
			return 0, 0, p.Die("`%s` is defined %d times at column 0, and this phase needs exactly one",
				name, len(heads))
		}
		end := heads[0]
		for end < len(L) && L[end] != "}" {
			end++
		}
		if end >= len(L) {
			return 0, 0, p.Die("`%s` does not close at column 0", name)
		}
		if !edit.W119RetType.MatchString(L[heads[0]-1]) {
			return 0, 0, p.Die("the line above `%s`'s head is %s and every definition in this tree carries "+
				"its return type there, indented", name, cutil.PyRepr(L[heads[0]-1]))
		}
		return heads[0] - 1, end + 1, nil
	}
	shortName := func(l string) string { return edit.W119Name.ReplaceAllString(l, "$1") }

	L := strings.Split(t, "\n")
	linesBefore := len(L) - 1
	runsBefore := blankRuns(t)

	// ---- 0. the boundary, and the file this edit was written against ---------
	var directives []int
	for i, l := range L {
		if edit.W119Dir.MatchString(l) {
			directives = append(directives, i)
		}
	}
	if len(directives) != 11 {
		return nil, p.Die("the file holds %d preprocessor directives and this phase was written against "+
			"the eleven `#include`s phase 104 left", len(directives))
	}
	for i := range directives {
		if directives[i] != directives[0]+i {
			return nil, p.Die("the eleven directives are not eleven consecutive lines")
		}
	}
	for _, i := range directives {
		if !edit.W119Inc.MatchString(L[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	boundary := directives[0]
	p.Sayf("the boundary is line %d, the first of the eleven `#include`s, and there is not a "+
		"directive above it", boundary+1)

	// ---- 1. the core's block of ordinary declarations, FOUND rather than assumed
	var seed []int
	for i, l := range L[:boundary] {
		if l == w119Go[0] {
			seed = append(seed, i)
		}
	}
	if len(seed) != 1 {
		return nil, p.Die("the core does not declare `%s` exactly once, so this phase has not been handed "+
			"the file it was written for", w119Go[0])
	}
	lo, hi := seed[0], seed[0]
	for lo > 0 && edit.W119IsDecl(L[lo-1]) {
		lo--
	}
	for hi+1 < boundary && edit.W119IsDecl(L[hi+1]) {
		hi++
	}
	blockBefore := append([]string{}, L[lo:hi+1]...)
	for _, line := range w119Go {
		if !edit.Contains(blockBefore, line) {
			return nil, p.Die("`%s` is not in the core's block of ordinary declarations, which is %s",
				line, strings.Join(blockBefore, " / "))
		}
	}
	if L[lo-1] != "" || L[hi+1] != "" {
		return nil, p.Die("the block is not a paragraph of its own -- line %d is %s and line %d is %s",
			lo, cutil.PyRepr(L[lo-1]), hi+2, cutil.PyRepr(L[hi+1]))
	}
	var blockAfter []string
	for _, l := range blockBefore {
		if !edit.Contains(w119Go, l) {
			blockAfter = append(blockAfter, l)
		}
	}
	names := make([]string, len(blockBefore))
	for i, l := range blockBefore {
		names[i] = shortName(l)
	}
	p.Sayf("the core's block of ordinary declarations is lines %d-%d, %d of them: %s",
		lo+1, hi+1, len(blockBefore), strings.Join(names, " "))

	// ---- 2. the two names above the boundary, AS A PARTITION AND NOT A COUNT -
	core := strings.Join(L[:boundary], "\n")
	host := strings.Join(L[boundary:], "\n")
	gpLo, gpHi, err := defn(L, "mch_get_pid")
	if err != nil {
		return nil, err
	}
	if gpHi > boundary {
		return nil, p.Die("mch_get_pid() is defined below the boundary, and this phase is about what the " +
			"CORE says")
	}
	var raiseLines []int
	for i := 0; i < boundary; i++ {
		if edit.W119Reraise.MatchString(L[i]) {
			raiseLines = append(raiseLines, i)
		}
	}
	if len(raiseLines) != 1 {
		return nil, p.Die("`kill(getpid(), <name>);` is %d lines above the boundary and this phase needs "+
			"exactly one, the deferred deadly signal vim_handle_signal() re-raises", len(raiseLines))
	}
	rl := raiseLines[0]
	m := edit.W119Reraise.FindStringSubmatch(L[rl])
	indent, deferred := m[1], m[2]
	gpRange := make([]int, 0, gpHi-gpLo)
	for i := gpLo; i < gpHi; i++ {
		gpRange = append(gpRange, i)
	}
	type class struct {
		What  string
		lines []int
	}
	classes := []struct {
		Name string
		cls  []class
	}{
		{"getpid", []class{
			{"declaration", []int{lo + edit.IndexOf(blockBefore, w119Go[0])}},
			{"mch_get_pid()", gpRange},
			{"the re-raise", []int{rl}},
		}},
		{"kill", []class{
			{"declaration", []int{lo + edit.IndexOf(blockBefore, w119Go[1])}},
			{"the re-raise", []int{rl}},
		}},
	}
	// The Python iterates a dict, whose order is the insertion order; the report
	// is what a port must reproduce, so the classes are a SLICE here and not a
	// map -- ranging a Go map would reorder the line every run.
	for _, c := range classes {
		word := regexp.MustCompile(`\b` + c.Name + `\b`)
		var seen []int
		for i := 0; i < boundary; i++ {
			if word.MatchString(L[i]) {
				seen = append(seen, i)
			}
		}
		owned := map[int]bool{}
		for _, cl := range c.cls {
			for _, i := range cl.lines {
				owned[i] = true
			}
		}
		var stray []string
		for _, i := range seen {
			if !owned[i] {
				stray = append(stray, fmt.Sprintf("%d:%s", i+1, strings.TrimSpace(L[i])))
			}
		}
		if len(stray) > 0 {
			var labels []string
			for _, cl := range c.cls {
				labels = append(labels, cl.What)
			}
			return nil, p.Die("`%s` is said above the boundary at %s, which is in none of the classes this "+
				"phase rewrites (%s) -- and this phase will not delete a declaration whose "+
				"every use it cannot account for",
				c.Name, strings.Join(edit.First(stray, 4), " "), strings.Join(labels, ", "))
		}
		var parts []string
		for _, cl := range c.cls {
			any := false
			var where []string
			for _, i := range cl.lines {
				if word.MatchString(L[i]) {
					any = true
					where = append(where, fmt.Sprintf("%d", i+1))
				}
			}
			if !any {
				return nil, p.Die("the class `%s` of `%s` holds no mention of it", cl.What, c.Name)
			}
			parts = append(parts, fmt.Sprintf("%s at %s", cl.What, strings.Join(where, " ")))
		}
		s := "s"
		if len(seen) == 1 {
			s = ""
		}
		p.Sayf("`%s` above the boundary: %d mention%s, and every one falls in a class this "+
			"phase rewrites -- %s", c.Name, len(seen), s, strings.Join(parts, ", "))
	}
	for _, nh := range []struct {
		Name  string
		nhost int
	}{{"getpid", 0}, {"kill", 1}} {
		if k := mentions(host, nh.Name); k != nh.nhost {
			return nil, p.Die("`%s` has %d mentions below the boundary and this phase was written against "+
				"%d -- musl_suspend() stops the process group with `kill(0, SIGTSTP)` and "+
				"nothing below the boundary asks for a pid", nh.Name, k, nh.nhost)
		}
	}
	_ = core
	if mentions(t, "host_raise") > 0 {
		return nil, p.Die("`host_raise` is already a name in this file")
	}

	// ---- 3. getpid is AVOIDED -------------------------------------------------
	var protoGP, callers []int
	for i, l := range L {
		if edit.W119ProtoGP.MatchString(l) {
			protoGP = append(protoGP, i)
		}
	}
	for i, l := range L {
		if len(edit.CallsNotAfterWord([]byte(l), "mch_get_pid")) > 0 &&
			!(gpLo <= i && i < gpHi) && !edit.Contains(protoGP, i) {
			callers = append(callers, i)
		}
	}
	if len(protoGP) != 1 || len(callers) != 1 {
		return nil, p.Die("mch_get_pid() has %d forward declarations and %d call sites outside its own "+
			"definition, and this phase needs one of each -- the prototype tools/deadprotos.py "+
			"takes, and ml_open()'s write of b0_pid", len(protoGP), len(callers))
	}
	if !edit.W119Write.MatchString(L[callers[0]]) {
		return nil, p.Die("mch_get_pid()'s one call site is %s, and this phase was written against "+
			"ml_open()'s `long_to_char(mch_get_pid(), b0p->b0_pid);`", cutil.PyRepr(L[callers[0]]))
	}
	var b0, field []int
	b0Re := regexp.MustCompile(`\bb0_pid\b`)
	for i, l := range L {
		if b0Re.MatchString(l) {
			b0 = append(b0, i)
			if edit.W119Field.MatchString(l) {
				field = append(field, i)
			}
		}
	}
	want := append(append([]int{}, field...), callers[0])
	sort.Ints(want)
	if len(b0) != 2 || len(field) != 1 || !sameInts(b0, want) {
		var shown []string
		for _, i := range b0 {
			shown = append(shown, fmt.Sprintf("%d:%s", i+1, strings.TrimSpace(L[i])))
		}
		return nil, p.Die("`b0_pid` has %d mentions and this phase needs exactly two, its own declaration "+
			"and the one write: %s", len(b0), strings.Join(shown, " "))
	}
	p.Sayf("`b0_pid` is WRITE-ONLY: %d mentions in the whole file, its declaration at line %d "+
		"and ml_open()'s write at line %d, and not one read.  So the write is the entry "+
		"point, mch_get_pid() (lines %d-%d) is its only feeder, and `getpid` leaves the core "+
		"without a host call", len(b0), field[0]+1, callers[0]+1, gpLo+1, gpHi)
	if err := swap(L[callers[0]]+"\n", "", "ml_open()'s write of b0_pid",
		"it is the only mention of the field that is not its declaration, and the only "+
			"caller of mch_get_pid()"); err != nil {
		return nil, err
	}
	if err := swap(strings.Join(L[gpLo:gpHi], "\n")+"\n\n", "", "mch_get_pid()'s definition",
		"its one caller has just gone, and its body is the only other place the core says "+
			"`getpid`"); err != nil {
		return nil, err
	}

	// ---- 4. kill is MOVED -----------------------------------------------------
	if err := swap(fmt.Sprintf("%skill(getpid(), %s);\n", indent, deferred),
		fmt.Sprintf("%shost_raise(%s);\n", indent, deferred),
		"vim_handle_signal()'s re-raise of a deferred deadly signal",
		"the core asks the host to raise the signal on this process; WHICH process that is "+
			"is the host's idea, exactly as fd 1 is under host_write"); err != nil {
		return nil, err
	}

	// ---- 5. the two declarations leave the core's block -----------------------
	// AS ONE REPLACEMENT OF THE WHOLE BLOCK, because of what happens when it
	// empties: the block is a paragraph, and taking its last line away leaves two
	// blank lines in a row.
	oldBlock := strings.Join(blockBefore, "\n") + "\n"
	dropped := 0
	if len(blockAfter) > 0 {
		if err := swap(oldBlock, strings.Join(blockAfter, "\n")+"\n",
			"the core's block of declarations",
			"the two this phase owns come out of it and the rest stay where they are"); err != nil {
			return nil, err
		}
		dropped = len(w119Go)
	} else {
		if err := swap(oldBlock+"\n", "", "the core's block of declarations AND its trailing "+
			"blank line",
			"the block is empty now, and a paragraph separator with nothing to separate "+
				"is the run of two blank lines this file does not have"); err != nil {
			return nil, err
		}
		dropped = len(blockBefore) + 1
	}

	// ---- 6. one prototype at the end of the core -> host block ---------------
	if err := swap("static long host_time(void);\n",
		"static long host_time(void);\n"+w119Proto+"\n",
		"the last of the core -> host prototypes",
		"the boundary is ONE block, and this belongs at the end of it rather than wherever "+
			"a declaration happened to fit"); err != nil {
		return nil, err
	}

	// ---- 7. and the host defines it, INSIDE the host block -------------------
	if err := swap("    static void\nmusl_suspend(void)\n{\n",
		w119Def+"    static void\nmusl_suspend(void)\n{\n",
		"musl_suspend()'s head, the last function of the host region",
		"the definition goes immediately above it, so that it is INSIDE the region "+
			"zhostonly reads and its two host words are where every other one is"); err != nil {
		return nil, err
	}

	// ---- 8. what the file is now ----------------------------------------------
	L = strings.Split(t, "\n")
	boundary = -1
	for i, l := range L {
		if edit.W119Dir.MatchString(l) {
			boundary = i
			break
		}
	}
	core, host = strings.Join(L[:boundary], "\n"), strings.Join(L[boundary:], "\n")
	for _, r := range []struct {
		Name         string
		ncore, nhost int
	}{{"getpid", 0, 1}, {"kill", 0, 2}, {"mch_get_pid", 1, 0}} {
		if mentions(core, r.Name) != r.ncore || mentions(host, r.Name) != r.nhost {
			return nil, p.Die("`%s` ends at %d mentions above the boundary and %d below, expected %d and "+
				"%d", r.Name, mentions(core, r.Name), mentions(host, r.Name), r.ncore, r.nhost)
		}
	}
	if k := mentions(t, "host_raise"); k != 3 {
		return nil, p.Die("`host_raise` has %d mentions and it must have three -- its prototype, the one "+
			"call site it took over from `kill` and its definition", k)
	}
	if k := mentions(t, "b0_pid"); k != 1 {
		return nil, p.Die("`b0_pid` has %d mentions and the edit leaves exactly one, its own declaration, "+
			"for tools/deadfields.py to take", k)
	}
	have := append([]string{}, L[lo:lo+len(blockAfter)]...)
	if strings.Join(have, "\x00") != strings.Join(blockAfter, "\x00") {
		return nil, p.Die("the ordinary declarations left above the boundary are %s and the input's "+
			"block minus the two is %s", edit.W119Or(have), edit.W119Or(blockAfter))
	}
	if len(blockAfter) > 0 && (L[lo-1] != "" || L[lo+len(blockAfter)] != "") {
		return nil, p.Die("what is left of the block is not a paragraph of its own")
	}
	var stray []string
	for i := 0; i < boundary; i++ {
		if edit.W119IsDecl(L[i]) && (L[i-1] == "" || edit.W119IsDecl(L[i-1])) &&
			!(lo <= i && i < lo+len(blockAfter)) {
			stray = append(stray, fmt.Sprintf("%d:%s", i+1, L[i]))
		}
	}
	if len(stray) > 0 {
		return nil, p.Die("an ordinary declaration is above the boundary and outside the block: %s",
			strings.Join(edit.First(stray, 4), " / "))
	}
	if err := os.WriteFile(state+"/block-before",
		[]byte(strings.Join(blockBefore, "\n")+"\n"), 0o644); err != nil {
		return nil, p.Die("%v", err)
	}
	var after strings.Builder
	for _, l := range blockAfter {
		after.WriteString(l + "\n")
	}
	if err := os.WriteFile(state+"/block-after", []byte(after.String()), 0o644); err != nil {
		return nil, p.Die("%v", err)
	}
	if len(blockAfter) > 0 {
		var left []string
		for _, l := range blockAfter {
			left = append(left, shortName(l))
		}
		p.Sayf("the core's block of ordinary declarations is %d lines and was %d: %s remain, "+
			"and each is a libc function some LATER phase owns",
			len(blockAfter), len(blockBefore), strings.Join(left, " "))
	} else {
		p.Sayf("THE CORE'S BLOCK OF ORDINARY DECLARATIONS IS EMPTY: it was %d lines and it is "+
			"now none.  Above the first `#include` there is no declaration that is not "+
			"`static`, and the check states what that means as a measurement of the cut "+
			"rather than as a sentence written here", len(blockBefore))
	}

	// DECLARATION BEFORE USE, COMPUTED, and the definition INSIDE the host region.
	var pr, df, uses []int
	for i, l := range L {
		if l == w119Proto {
			pr = append(pr, i)
		}
		if strings.HasPrefix(l, "host_raise(") && i+1 < len(L) && L[i+1] == "{" {
			df = append(df, i)
		}
	}
	for i, l := range L {
		if len(edit.CallsNotAfterWord([]byte(l), "host_raise")) > 0 &&
			!edit.Contains(pr, i) && !edit.Contains(df, i) {
			uses = append(uses, i)
		}
	}
	if len(pr) != 1 || len(df) != 1 || len(uses) != 1 {
		return nil, p.Die("`host_raise` has %d prototypes, %d definitions and %d call sites",
			len(pr), len(df), len(uses))
	}
	var hb, he []int
	for i, l := range L {
		if strings.HasPrefix(l, "static volatile sig_atomic_t host_winch_pending") {
			hb = append(hb, i)
		}
		if strings.HasPrefix(l, "musl_suspend(") {
			he = append(he, i)
		}
	}
	if len(hb) != 1 || len(he) != 1 {
		return nil, p.Die("the host region does not begin and end exactly once -- zhostonly reads " +
			"it from `host_winch_pending` to musl_suspend's last brace")
	}
	end := he[0]
	for end < len(L) && L[end] != "}" {
		end++
	}
	if !(pr[0] < uses[0] && uses[0] < df[0]) {
		return nil, p.Die("`host_raise`: prototype at %d, call at %d, definition at %d -- the prototype "+
			"must be above the call and the definition below it", pr[0]+1, uses[0]+1, df[0]+1)
	}
	if !(boundary < df[0] && hb[0] <= df[0] && df[0] <= end) {
		return nil, p.Die("`host_raise` is defined at line %d, and it must be below the boundary (%d) and "+
			"INSIDE the host region (%d-%d): its body says `kill` and `getpid`, and every "+
			"mention of a host word lives in that region",
			df[0]+1, boundary+1, hb[0]+1, end+1)
	}
	p.Sayf("`host_raise`: prototype line %d, one call site at line %d, definition line %d, "+
		"which is below the boundary at %d and inside the %d-line host region "+
		"zhostonly reads", pr[0]+1, uses[0]+1, df[0]+1, boundary+1, end+1-hb[0])

	// ---- 9. the arithmetic, every term computed from what was found ----------
	added := 1 + len(strings.Split(w119Def, "\n")) - 1
	removed := 1 + (gpHi - gpLo) + 1 + dropped
	if len(L)-1 != linesBefore+added-removed {
		return nil, p.Die("the file is %d lines and the input was %d -- expected %d: one prototype and a "+
			"%d-line definition in, and ml_open()'s write, mch_get_pid()'s %d lines with "+
			"the blank after it and %d out of the core's block",
			len(L)-1, linesBefore, linesBefore+added-removed, added-1, gpHi-gpLo, dropped)
	}
	if r := blankRuns(t); r != runsBefore {
		return nil, p.Die("the edit left %d runs of two blank lines where there were %d", r, runsBefore)
	}
	var d2 []int
	for i, l := range L {
		if edit.W119Dir.MatchString(l) {
			d2 = append(d2, i)
		}
	}
	okd := len(d2) == 11 && d2[0] == boundary
	for i := range d2 {
		if d2[i] != d2[0]+i {
			okd = false
		}
	}
	if !okd {
		return nil, p.Die("the output does not have the same eleven contiguous `#include` directives -- " +
			"this phase adds a DECLARATION and a DEFINITION, never a directive")
	}
	p.Sayf("%d -> %d lines before the sweep, the eleven #includes untouched at line %d, and no "+
		"run of two blank lines.  tools/deadprotos.py and tools/deadfields.py are left "+
		"`static long mch_get_pid(void);` and `b0_pid` to find",
		linesBefore, len(L)-1, boundary+1)
	return []byte(t), nil
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
