package p111

// Whim phase 111, the check -- the scalar clock.
// See phase/111/edit.go, and GOALS.md II.4c.
//
// Runs after phase/111/edit.go and the sweep internal/verify runs between them, and
// reads nothing from the edit's shell -- only the work tree and the state directory.
// What the edit left there is `old.c`, the source this phase was HANDED, and `old`, that
// source built with SOURCE_DATE_EPOCH=0 and the boundary's own flags.
//
// WHAT IS CLAIMED, in eight parts:
//
// ARITHMETIC  computed FROM THE INPUT: elapsed_T, elapsed, now_tv and
// musl_gettimeofday are at 0, `struct timeval` is 0 above the boundary
// and 3 below it, musl_now_ms is 9 above and 1 below, and the line count
// moved by exactly -15 in the core and +6 in the host.
// THE BOUNDARY
// `make editor.c`'s cut, computed here by the same awk clause: 0
// directives, `-fsyntax-only` with no error and no warning that is not a
// boundary name, and the warning set STATED AS A SET -- twelve
// `musl_`/`host_` names and `vim_snprintf`, with musl_gettimeofday
// REPLACED by musl_now_ms and not added to.
// THE SHAPE   every one of the thirteen signatures takes scalars and byte buffers
// only.  IT IS COMPUTED ON THE INPUT TOO, and the input satisfies it:
// phase 109 chose `musl_gettimeofday(long *, long *)` precisely so that
// `struct timeval` would not cross, so this phase does NOT earn that
// sentence and must not claim it.  What it earns is stated below.
// THE ROUNDING
// MEASURED, not argued.  elapsed() subtracted and then divided;
// musl_now_ms divides at each reading and the caller subtracts.  A probe
// compiled and run here compares the two over 20,000,000 random pairs and
// must find the difference bounded by EXACTLY 1 ms, in both directions,
// with the two formulas equally far from the true elapsed time.
// CANON       tools/canon.sh is a NO-OP on the output: the four `long` declarations,
// the eight new statements and musl_now_ms are written the way this file
// writes everything else.
// HOST        `zhostonly`, whose vocabulary this phase EXTENDS: `gettimeofday`
// is a host word from here, with the five core call sites phase 109 moved
// named as exceptions at the counts they had at q103, q104 and q108.
// SYMBOLS     `nm -u` is THE SAME SET -- 17 names, `comm` empty in both directions --
// and `main` is still the only external symbol.  **`gettimeofday` DOES
// NOT LEAVE**, and the check says so as an equality rather than letting a
// reader expect a clock phase to free a clock symbol: the host still calls
// it to implement musl_now_ms, and a symbol leaves when its last caller
// leaves the FILE, which is the split and not this phase.
// BEHAVIOUR   the declared delta is NOTHING AT ALL, and the 102 screen cases CANNOT
// SEE THIS PHASE -- which is measured rather than assumed, and is why the
// phase owes probes.  See below.
//
// THE SCREEN CORPUS IS BLIND TO THE CLOCK, AND THAT IS A MEASUREMENT.  Two full
// recordings being byte-identical would mean very little on its own here: of the 102
// cases, 95 ring the bell once and 7 not at all, and NOT ONE rings it twice -- so
// vim_beep's 500 ms rate limit is never exercised a second time and the only reading the
// corpus could see never happens.  Measured: a control whose clock NEVER ADVANCES, one
// that RUNS BACKWARDS and one that runs 1000x FAST all move 0 of the 102 cases.  Phase
// 109's control moved six, and it was not a clock control: swapping musl_gettimeofday's
// two output fields makes each reading an independent random number rather than a
// consistently wrong one, which is a different thing from a clock.
//
// SO THE PHASE OWES PROBES, and `gs` is what they are built on: `nv_g_cmd`'s `s` arm is
// `do_sleep(count * 1000)`, which is the ONE call site a keystroke file can drive and
// the one that makes real time pass inside the editor.  Four probes on the binary this
// phase was handed and on its own, required to agree:
//
// 1gs            ~1,010 ms   do_sleep's loop, measured against the wall clock
// 2gs            ~2,009 ms   and again at twice the length: the loop caps a wait at
// 1,000 ms, so 2gs is TWO iterations and 1gs is one
// hgshh          2 bells     vim_beep's threshold in BOTH directions in one probe:
// h at column 0 rings, gs lets a second pass, the next h
// rings because more than 500 ms have gone, and the third
// is suppressed because less than 500 have
//
// AND EACH HALF IS PROVEN ABLE TO FAIL, by a control aimed at THAT half:
//
// fast    the clock runs 1000x fast.  `2gs` returns after ONE 1,000 ms wait instead
// of two -- measured 1,008 ms against 2,009 -- so the difference `2gs - 1gs`
// collapses from ~1,000 ms to ~0.  It terminates, which is why it is the
// control the timing assertion uses.
// froze   the clock never advances.  `do_sleep`'s `done` stays 0 and `1gs` NEVER
// RETURNS -- the probe hits its timeout.
// nobell  vim_beep's `> 500` written `> 500000`, so the second h cannot ring: 1 bell.
// allbell     ... written `> -1`, so the third one can: 3 bells.
// THE BELL CONTROLS MOVE THE THRESHOLD AND NOT THE CLOCK, and that is a
// measurement about the probe rather than a preference.  A clock control that
// breaks the rate limit breaks do_sleep FIRST -- `froze` and a backwards clock
// were both tried, and `hgshh` blocks on each, so neither can say anything
// about bells at all.
// ceil    every reading rounded UP instead of down: `(tv_usec + 999) / 1000`.  This is
// the ROUNDING question made into a control, and it perturbs every reading by
// up to a full millisecond -- twice what the change from elapsed() can.  Its
// probes are the product's and its FULL RECORDING is byte-identical, which is
// the answer to "can any caller see one millisecond": no.
//
// AND ONE MORE THAT IS NOT A CONTROL BUT A DECISION.  `epoch` is musl_now_ms written the
// other way -- milliseconds since 1970 rather than since the whole second of its first
// call.  Its probes and its full recording are the product's, which is what makes the
// origin a free choice and not a behaviour change; the reason the product takes the
// monotonic one is 32-bit arithmetic, and phase/111/edit.go states it.

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim111", Check) }

var z28Declared = strings.Fields("host_exit host_message musl_delay musl_get_winsize " +
	"musl_host_init musl_now_ms musl_read_input musl_suspend musl_term_start " +
	"musl_term_stop musl_tty_keys musl_wait_for_input vim_snprintf")

const z28Gone = "musl_gettimeofday"

// z28RoundC is the rounding probe, written and run here so that the claim in
// the edit's header is a measurement at every boundary and not a memory.
// `srandom(12345)`, so its output is the same every run and the report line it
// feeds is stable where the timing probes are not.
const z28RoundC = `#include <stdio.h>
#include <stdlib.h>
static long old_f(long s1, long u1, long s2, long u2)
{ return (s2 - s1) * 1000L + (u2 - u1) / 1000L; }
static long now_ms(long s, long u, long b)
{ return (s - b) * 1000L + u / 1000L; }
int main(void)
{
    long b = 1000000, n = 20000000, i, worst = 0, d[3] = {0, 0, 0};
    double miss_old = 0, miss_new = 0;
    srandom(12345);
    for (i = 0; i < n; i++) {
        long s1 = b + random() % 100, u1 = random() % 1000000;
        long gap = random() % 5000000;
        long t1 = s1 * 1000000L + u1, t2 = t1 + gap;
        long s2 = t2 / 1000000L, u2 = t2 % 1000000L;
        long o = old_f(s1, u1, s2, u2);
        long w = now_ms(s2, u2, b) - now_ms(s1, u1, b);
        long delta = w - o;
        if (delta < -1 || delta > 1) { printf("RANGE %ld\n", delta); return 1; }
        d[delta + 1]++;
        if (labs(delta) > worst) worst = labs(delta);
        miss_old += (double)(o - gap / 1000);
        miss_new += (double)(w - gap / 1000);
    }
    printf("pairs %ld  minus1 %ld  same %ld  plus1 %ld  worst %ld  mean_old %+.4f  mean_new %+.4f\n",
           n, d[0], d[1], d[2], worst, miss_old / n, miss_new / n);
    return 0;
}
`

// z28Args is the heredoc's `argsof`: the parameter list by BRACE MATCHING and
// not by a split on the first `(`.  `__attribute__((format(printf, 3, 4)))`
// carries parentheses and commas of its own, and a naive split reads them as
// parameters -- measured, three of them.
func z28Args(decl, name string) (string, []string) {
	i := strings.Index(decl, name) + len(name)
	for i < len(decl) && (decl[i] == ' ' || decl[i] == '\t') {
		i++
	}
	d, j := 0, i
	for j < len(decl) {
		if decl[j] == '(' {
			d++
		} else if decl[j] == ')' {
			d--
			if d == 0 {
				break
			}
		}
		j++
	}
	Inner := decl[i+1 : j]
	var Out []string
	d, last := 0, 0
	for x := 0; x < len(Inner); x++ {
		switch Inner[x] {
		case '(':
			d++
		case ')':
			d--
		case ',':
			if d == 0 {
				Out = append(Out, Inner[last:x])
				last = x + 1
			}
		}
	}
	Out = append(Out, Inner[last:])
	for k := range Out {
		Out[k] = strings.TrimSpace(Out[k])
	}
	return decl[len("static"):strings.Index(decl, name)], Out
}

type z28Probe struct {
	Name  string
	Keys  []byte
	bells int
}

// `gs` IS nv_g_cmd's `s` arm and it is do_sleep(count * 1000): the one call
// site a keystroke file can drive, and the only way real time passes inside
// the editor.
var z28Probes = []z28Probe{
	{"1gs", []byte("gs:q!\r"), 0},
	{"2gs", []byte("2gs:q!\r"), 0},
	{"hgshh", []byte("hgshh:q!\r"), 2},
}

type z28Res struct {
	ms, bells, Rc int
	ok            bool // false is Python's (None, None, None): the probe blocked
}

// Whim111 is phase 111's check: the scalar clock.
func Check(w io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: check whim111 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	f := filepath.Join(work, "whim-vim.c")
	r := &check.Rep{Tag: "clock", W: w}
	stop := func(format string, a ...any) error {
		r.Say(format, a...)
		return harness.ErrReported
	}
	fatal := func(head, logPath string, n int) error {
		r.Say("%s", head)
		for i, l := range strings.Split(check.ReadFile(logPath), "\n") {
			if i >= n {
				break
			}
			fmt.Fprintln(w, l)
		}
		return harness.ErrReported
	}

	beforeLines, err := strconv.Atoi(strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines"))))
	if err != nil {
		return stop("the state directory holds no usable input-lines")
	}
	tmp, err := os.MkdirTemp("", "whim111-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflags := strings.Fields(check.Z28CFlags.FindStringSubmatch(mk)[1])
	ldflags := strings.Fields(check.Z28LDFlags.FindStringSubmatch(mk)[1])
	link := func(src, Out string) *exec.Cmd {
		a := append(append([]string{}, cflags...), ldflags...)
		a = append(a, "-o", Out, src)
		c := exec.Command("gcc", a...)
		c.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
		return c
	}

	t := check.ReadFile(f)

	// ---- the six variants ------------------------------------------------
	const ret = "    return (tv.tv_sec - host_now_base) * 1000L + tv.tv_usec / 1000L;"
	const beep = "        if (!did_init || musl_now_ms() - start_tv > 500)"
	if strings.Count(t, ret) != 1 {
		return stop("musl_now_ms does not compute its answer in exactly one line, so not " +
			"one of the variants below could be built from it")
	}
	variants := []struct{ Name, Old, line string }{
		{"ceil", ret, "    return (tv.tv_sec - host_now_base) * 1000L + (tv.tv_usec + 999) / 1000L;"},
		{"epoch", ret, "    return tv.tv_sec * 1000L + tv.tv_usec / 1000L;"},
		{"fast", ret, "    return (tv.tv_sec - host_now_base) * 1000000L + tv.tv_usec;"},
		{"froze", ret, "    return (tv.tv_sec - host_now_base) * 0L + tv.tv_usec * 0L;"},
		// THE BELL PROBE NEEDS CONTROLS OF ITS OWN, and they move vim_beep's
		// THRESHOLD rather than the clock: a clock control that breaks the rate
		// limit breaks do_sleep first, and `hgshh` then never returns at all.
		{"nobell", beep, strings.Replace(beep, "> 500", "> 500000", 1)},
		{"allbell", beep, strings.Replace(beep, "> 500", "> -1", 1)},
	}
	for _, v := range variants {
		if strings.Count(t, v.Old) != 1 {
			return stop("the line the %s variant rewrites is not in the output exactly "+
				"once, so it could not be a control", v.Name)
		}
		nv := strings.Replace(t, v.Old, v.line, 1)
		if nv == t {
			return stop("the %s variant changed nothing, so it is not a control", v.Name)
		}
		if err := os.WriteFile(filepath.Join(tmp, v.Name+".c"), []byte(nv), 0o644); err != nil {
			return err
		}
	}
	r.Say("six variants written: ceil (every reading rounded UP), epoch (milliseconds " +
		"since 1970), fast (microseconds, so 1000x), froze (a clock that never " +
		"advances), and nobell/allbell, which move vim_beep's 500 to 500000 and to -1")

	var wg sync.WaitGroup
	var errNew error
	wg.Add(1)
	go func() { defer wg.Done(); errNew = link(f, filepath.Join(tmp, "new")).Run() }()
	for _, v := range variants {
		name := v.Name
		wg.Add(1)
		go func() {
			defer wg.Done()
			link(filepath.Join(tmp, name+".c"), filepath.Join(tmp, name)).Run()
		}()
	}
	canonC := filepath.Join(tmp, "canon.c")
	os.WriteFile(canonC, []byte(t), 0o644)
	var errCanon error
	wg.Add(1)
	go func() {
		defer wg.Done()
		b, e := exec.Command("sh", "tools/canon.sh", canonC).CombinedOutput()
		errCanon = e
		os.WriteFile(filepath.Join(tmp, "canon.log"), b, 0o644)
	}()
	var errRound error
	wg.Add(1)
	go func() {
		defer wg.Done()
		rc := filepath.Join(tmp, "round.c")
		os.WriteFile(rc, []byte(z28RoundC), 0o644)
		if e := exec.Command("gcc", "-O2", "-o", filepath.Join(tmp, "round"), rc).Run(); e != nil {
			errRound = e
			return
		}
		Out, e := exec.Command(filepath.Join(tmp, "round")).Output()
		errRound = e
		os.WriteFile(filepath.Join(tmp, "round.txt"), Out, 0o644)
	}()

	// ---- 1. the source, as arithmetic on the input -----------------------
	old := check.ReadFile(filepath.Join(state, "old.c"))
	split := func(text, which string) (string, string, []string, int, error) {
		lines := strings.Split(text, "\n")
		var d []int
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimLeft(l, " \t\n\v\f\r"), "#") {
				d = append(d, i)
			}
		}
		ok := len(d) == 11
		for k, i := range d {
			if ok && i != d[0]+k {
				ok = false
			}
		}
		if !ok {
			var at []string
			for i, x := range d {
				if i >= 3 {
					break
				}
				at = append(at, strconv.Itoa(x+1))
			}
			return "", "", nil, 0, stop("%s does not have eleven contiguous directives: %d "+
				"at %s.  The first one IS the boundary and every count below "+
				"distinguishes the core from the host", which, len(d), strings.Join(at, " "))
		}
		return strings.Join(lines[:d[0]], "\n"), strings.Join(lines[d[0]:], "\n"), lines, d[0], nil
	}
	_, obelow, olines, ocut, e := split(old, "old.c")
	if e != nil {
		return e
	}
	ncore, nbelow, nlines, ncut, e := split(t, "the output")
	if e != nil {
		return e
	}
	count := func(s, pat string) int {
		return len(regexp.MustCompile(pat).FindAllString(s, -1))
	}

	if len(olines)-1 != beforeLines {
		r.Bad("the state directory says the edit was handed %d lines and old.c has %d",
			beforeLines, len(olines)-1)
	}
	for _, name := range []string{"elapsed_T", "elapsed", "now_tv", "musl_gettimeofday"} {
		was, now := count(old, `\b`+name+`\b`), count(t, `\b`+name+`\b`)
		if now != 0 {
			r.Bad("`%s` was %d in the input and is still %d in the output -- this phase's "+
				"whole product is that it is 0 everywhere", name, was, now)
		}
	}
	if n := len(check.Z28TV.FindAllString(ncore, -1)); n > 0 {
		r.Bad("`struct timeval` is back above the boundary, %d times", n)
	}
	tvh := len(check.Z28TV.FindAllString(nbelow, -1))
	if oh := len(check.Z28TV.FindAllString(obelow, -1)); tvh != oh {
		r.Bad("`struct timeval` is %d below the boundary and the input had %d there: "+
			"musl_now_ms keeps musl_gettimeofday's one and adds none", tvh, oh)
	}
	if bare := count(t, `\bgettimeofday\b`); bare != 1 {
		r.Bad("the bare name `gettimeofday` occurs %d times in the output and must occur "+
			"exactly once -- the call inside musl_now_ms", bare)
	}
	if count(ncore, `\bgettimeofday\b`) > 0 {
		r.Bad("the core calls `gettimeofday` directly, which is the whole thing the host " +
			"call exists to prevent")
	}
	for _, x := range []struct {
		Where, Text string
		want        int
		why         string
	}{
		{"above the boundary", ncore, 9, "the prototype, four stamps and four readings"},
		{"below the boundary", nbelow, 1, "its definition"},
	} {
		if n := count(x.Text, `\bmusl_now_ms\b`); n != x.want {
			r.Bad("`musl_now_ms` occurs %d times %s where %d were expected -- %s",
				n, x.Where, x.want, x.why)
		}
	}
	stamps := len(check.Z28Stamp.FindAllString(ncore, -1))
	reads := len(check.Z28Read.FindAllString(ncore, -1))
	if stamps != 4 || reads != 4 {
		r.Bad("the core has %d stamps of the shape `X = musl_now_ms();` and %d readings "+
			"of the shape `musl_now_ms() - X`, where 4 and 4 were expected", stamps, reads)
	}
	if on, nn := count(old, `\belapsed_time\b`), count(t, `\belapsed_time\b`); on != nn {
		r.Bad("`elapsed_time` is inchar_loop's own `long` and not this phase's, and it "+
			"moved from %d to %d", on, nn)
	}
	// -15, and it was -16.  The two removals re-counted on the canonical text:
	// the tagless struct, its prototype, the blank line between them and the
	// blank line below is SEVEN, and elapsed()'s definition with the blank line
	// below it is EIGHT -- the canonical text writes the return in one line
	// where the residue wrapped it, and writes no blank line after the opening
	// brace.  Measured: the core 76,473 -> 76,458.
	if ncut-ocut != -15 {
		r.Bad("the core is %d lines and was %d, a difference of %d where -15 was "+
			"expected: -7 for the typedef and the prototype with their blank lines and "+
			"-8 for elapsed() with a blank", ncut, ocut, ncut-ocut)
	}
	hostN, hostO := len(nlines)-ncut, len(olines)-ocut
	if hostN-hostO != 6 {
		r.Bad("the host is %d lines and was %d, a difference of %d where +6 was "+
			"expected: +4 for musl_now_ms's longer body and +2 for its two statics",
			hostN, hostO, hostN-hostO)
	}
	for _, line := range []string{"static long host_now_base = 0;", "static int host_now_based = FALSE;"} {
		if strings.Count(t, "\n"+line+"\n") != 1 {
			r.Bad("`%s` is not on a line of its own exactly once below the boundary", line)
		}
	}
	if !strings.Contains(nbelow, "if (!host_now_based)") {
		r.Bad("musl_now_ms does not take its base LAZILY.  Setting it in " +
			"musl_host_init() instead would be an ordering dependency a host rewrite " +
			"breaks silently, and the symptom would be base 0, epoch milliseconds and a " +
			"32-bit overflow on every call")
	}
	if rn, ro := check.Z27Runs(nlines), check.Z27Runs(olines); rn != ro {
		r.Bad("the edit left %d runs of two blank lines where there were %d", rn, ro)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the core has NO clock of its own: elapsed_T (%d in the input), elapsed (%d), "+
		"now_tv (%d) and musl_gettimeofday (%d) are all 0 in the whole file, `struct "+
		"timeval` is 0 above the boundary and %d below, and the one bare `gettimeofday` "+
		"left is the call musl_now_ms makes",
		count(old, `\belapsed_T\b`), count(old, `\belapsed\b`), count(old, `\bnow_tv\b`),
		count(old, `\bmusl_gettimeofday\b`), tvh)
	r.Say("four stamps `X = musl_now_ms();` and four readings `musl_now_ms() - X`, the "+
		"prototype above and the definition below: the core is %d lines against %d "+
		"(-15) and the host %d against %d (+6)", ncut, ocut, hostN, hostO)

	// ---- 2 and 3. the boundary: the cut, and the shape of what crosses it --
	r2 := &check.Rep{Tag: "clock", W: w}
	for _, x := range []struct{ which, path string }{
		{"output", f}, {"input", filepath.Join(state, "old.c")},
	} {
		lines := check.Z28Cut(check.ReadFile(x.path))
		text := strings.Join(lines, "\n") + "\n"
		cp := filepath.Join(tmp, "cut."+x.which+".c")
		os.WriteFile(cp, []byte(text), 0o644)
		for _, l := range lines {
			if check.Z28Hash.MatchString(l) {
				var dcount int
				for _, l2 := range lines {
					if check.Z28Hash.MatchString(l2) {
						dcount++
					}
				}
				r2.Bad("the %s cut holds %d directive(s), so it found the wrong line: %s",
					x.which, dcount, l)
				break
			}
		}
		wb, _ := exec.Command("gcc", "-O0", "-fno-stack-protector", "-Wall", "-Wextra",
			"-Wno-unused-parameter", "-fsyntax-only", cp).CombinedOutput()
		wtxt := string(wb)
		if errs := check.Z28ErrLine.FindAllString(wtxt, -1); len(errs) > 0 {
			if len(errs) > 3 {
				errs = errs[:3]
			}
			r2.Bad("the %s cut does not compile on its own:\n    %s",
				x.which, strings.Join(errs, "\n    "))
		}
		for _, l := range strings.Split(wtxt, "\n") {
			if strings.Contains(l, ": warning: ") && !strings.Contains(l, "used but never defined") {
				r2.Bad("the %s cut has a warning that is not a boundary name: %s", x.which, l)
				break
			}
		}
		seenSet := map[string]bool{}
		for _, m := range check.Z28Undef.FindAllStringSubmatch(wtxt, -1) {
			seenSet[m[1]] = true
		}
		seen := check.Z27Keys(seenSet)
		var want []string
		if x.which == "output" {
			want = append(want, z28Declared...)
		} else {
			for _, n := range z28Declared {
				if n != "musl_now_ms" {
					want = append(want, n)
				}
			}
			want = append(want, z28Gone)
		}
		sort.Strings(want)
		if strings.Join(seen, "\x00") != strings.Join(want, "\x00") {
			r2.Bad("the %s boundary is %s and this phase declares %s.  A phase that "+
				"widens the core -> host interface changes this set and nothing else in "+
				"the pipeline would say so", x.which, strings.Join(seen, " "), strings.Join(want, " "))
		}
		for _, name := range seen {
			reD := regexp.MustCompile(`^static\s.*\b` + regexp.QuoteMeta(name) + `\s*\(`)
			var decl []string
			for _, l := range lines {
				if reD.MatchString(l) && strings.HasSuffix(strings.TrimRight(l, " \t"), ";") {
					decl = append(decl, l)
				}
			}
			if len(decl) != 1 {
				r2.Bad("the %s boundary name `%s` has %d declarations above the cut and "+
					"must have exactly one", x.which, name, len(decl))
				continue
			}
			retT, params := z28Args(decl[0], name)
			for _, a := range append([]string{retT}, params...) {
				a = strings.TrimSpace(check.Z28WS.ReplaceAllString(a, " "))
				if a == "..." {
					if !strings.Contains(decl[0], "format(printf") {
						r2.Bad("the %s boundary name `%s` is VARIADIC and carries no "+
							"`format(printf, ...)` attribute, so nothing constrains what "+
							"crosses in its argument list", x.which, name)
					}
					continue
				}
				bare := strings.TrimSpace(check.Z28ParmNm.ReplaceAllString(a, ""))
				if !(check.Z28Scalar.MatchString(a) || check.Z28Scalar.MatchString(bare)) {
					r2.Bad("the %s boundary name `%s` takes or returns `%s`, which is not "+
						"a scalar or a byte buffer", x.which, name, a)
				}
			}
		}
	}
	if err := r2.Done(); err != nil {
		return err
	}
	decl := append([]string{}, z28Declared...)
	sort.Strings(decl)
	r.Say("THE BOUNDARY IS THIRTEEN NAMES, stated as a SET: %s.  `musl_gettimeofday` is "+
		"REPLACED by `musl_now_ms` and not added to, and the cut is %d lines with 0 "+
		"directives, no error and no warning that is not one of the thirteen",
		strings.Join(decl, " "), len(check.Z28Cut(t)))
	r.Say("and EVERY ONE OF THE THIRTEEN TAKES SCALARS AND BYTE BUFFERS ONLY -- void, " +
		"int, long, usize, char * and int *, with vim_snprintf's `...` held to printf " +
		"arguments by `format(printf, 3, 4)` on a build that is -Wall -Wextra clean.  IT " +
		"IS TRUE OF THE INPUT TOO, and this phase does not claim to have made it so: " +
		"phase 109 wrote `musl_gettimeofday(long *, long *)` precisely so that `struct " +
		"timeval` would not cross.  WHAT THIS PHASE EARNS is that the workaround is gone " +
		"-- no host call's shape is decided any more by a type the core cannot name, and " +
		"the core declares nothing shaped like a libc struct")

	// ---- 4. the rounding, measured ---------------------------------------
	wg.Wait()
	if errRound != nil {
		return stop("the rounding probe did not build or did not run")
	}
	fields := strings.Fields(check.ReadFile(filepath.Join(tmp, "round.txt")))
	v := map[string]string{}
	for k := 0; k+1 < len(fields); k += 2 {
		v[fields[k]] = fields[k+1]
	}
	worst, _ := strconv.Atoi(v["worst"])
	if worst != 1 {
		return stop("the rounding probe found a worst-case disagreement of %d ms, where "+
			"the whole claim of this phase is that it is exactly 1", worst)
	}
	m1, _ := strconv.Atoi(v["minus1"])
	p1, _ := strconv.Atoi(v["plus1"])
	if m1 == 0 || p1 == 0 {
		return stop("the rounding probe found the disagreement in only one direction (%s "+
			"below, %s above), which would make the new formula systematically biased "+
			"rather than differently rounded", v["minus1"], v["plus1"])
	}
	mo, _ := strconv.ParseFloat(v["mean_old"], 64)
	mn, _ := strconv.ParseFloat(v["mean_new"], 64)
	if math.Abs(mo-mn) > 0.01 {
		return stop("the two formulas are %s and %s ms from the true elapsed time on "+
			"average, and the claim is that neither is better", v["mean_old"], v["mean_new"])
	}
	r.Say("PRECISION IS NOT LOST AND THE ROUNDING POINT MOVES, measured over %s random "+
		"pairs: elapsed() subtracted and THEN divided, musl_now_ms divides at each "+
		"reading and the caller subtracts, and the two differ by EXACTLY +-1 ms and "+
		"never more -- %s pairs 1 ms lower, %s the same, %s 1 ms higher.  Microseconds "+
		"were already discarded either way, and neither formula is closer to the truth: "+
		"%s ms against %s ms on average.  Whether a CALLER can see 1 ms is section 8's "+
		"`ceil` control", v["pairs"], v["minus1"], v["same"], v["plus1"],
		v["mean_old"], v["mean_new"])

	// ---- 5. canon.sh ------------------------------------------------------
	if errCanon != nil {
		return fatal("tools/canon.sh failed on the output:", filepath.Join(tmp, "canon.log"), 10)
	}
	if check.ReadFile(canonC) != t {
		r.Say("tools/canon.sh is not a no-op on the output -- the new text is not written " +
			"the way this file writes everything else:")
		return harness.ErrReported
	}
	r.Say("tools/canon.sh is a NO-OP on the output: the four `long` declarations, the " +
		"four stamps, the four readings and musl_now_ms are written the way this file " +
		"writes everything else")

	// ---- 6. the host's vocabulary, which this phase extends --------------
	zh := exec.Command("sh", "tools/st.sh", "zhostonly", f)
	zh.Stdout, zh.Stderr = w, w
	if err := zh.Run(); err != nil {
		return harness.ErrReported
	}

	// ---- 7. the symbols ---------------------------------------------------
	if errNew != nil {
		return stop("the output did not build with '%s' '%s'",
			strings.Join(cflags, " "), strings.Join(ldflags, " "))
	}
	obj := func(src, Out string) error {
		return exec.Command("gcc", "-c", "-O0", "-fno-stack-protector", "-o", Out, src).Run()
	}
	if err := obj(filepath.Join(state, "old.c"), filepath.Join(tmp, "old.o")); err != nil {
		return err
	}
	if err := obj(f, filepath.Join(tmp, "new.o")); err != nil {
		return err
	}
	uOld := check.NmField26(filepath.Join(tmp, "old.o"), []string{"-u"}, 1)
	uNew := check.NmField26(filepath.Join(tmp, "new.o"), []string{"-u"}, 1)
	if g, c := check.Minus26(uOld, uNew), check.Minus26(uNew, uOld); len(g)+len(c) > 0 {
		return stop("`nm -u` moved: gone [%s] arrived [%s].  THIS PHASE FREES NOTHING AND "+
			"NEEDS NOTHING", strings.Join(g, " ")+" ", strings.Join(c, " ")+" ")
	}
	hasGTOD := false
	for _, s := range uNew {
		if s == "gettimeofday" {
			hasGTOD = true
		}
	}
	if !hasGTOD {
		return stop("`gettimeofday` is NOT in the undefined set, and it must be: the host " +
			"still calls it to implement musl_now_ms")
	}
	ext := check.NmField26(filepath.Join(tmp, "new.o"), []string{"--extern-only", "--defined-only"}, 2)
	if s := strings.Join(ext, " ") + " "; s != "main " {
		return stop("the output defines external symbols other than main: %s", s)
	}
	r.Say("`nm -u` is THE SAME SET, %d names, as a `comm` empty in BOTH directions, and "+
		"`main` is still the only external symbol.  `gettimeofday` IS STILL THERE and "+
		"this phase says so as an equality: a reader expects a clock phase to free a "+
		"clock symbol, and it cannot -- the host calls it to implement musl_now_ms, and "+
		"a symbol leaves when its last CALLER leaves the FILE, which is the split and not "+
		"this phase.  The output is %d bytes against %d", len(uNew),
		check.SizeOf(filepath.Join(tmp, "new")), check.SizeOf(filepath.Join(state, "old")))

	// ---- 8. the probes, and the recordings -------------------------------
	for _, vv := range variants {
		if check.SizeOf(filepath.Join(tmp, vv.Name)) < 0 {
			return stop("the %s control did not build", vv.Name)
		}
	}
	var wgR sync.WaitGroup
	recErr := make([]error, 4)
	for k, x := range []struct{ bin, src, Out string }{
		{filepath.Join(state, "old"), filepath.Join(state, "old.c"), "REC.old"},
		{filepath.Join(tmp, "new"), f, "REC.new"},
		{filepath.Join(tmp, "ceil"), filepath.Join(tmp, "ceil.c"), "REC.ceil"},
		{filepath.Join(tmp, "epoch"), filepath.Join(tmp, "epoch.c"), "REC.epoch"},
	} {
		k, x := k, x
		wgR.Add(1)
		go func() {
			defer wgR.Done()
			recErr[k] = check.RecZ(x.bin, x.src, filepath.Join(tmp, x.Out))
		}()
	}

	run := func(bin string, keys []byte) (z28Res, error) {
		t0 := time.Now()
		_, stdout, _, rc, err := harness.ZSession(bin, [][]byte{keys}, "xterm", nil, 24, 80, 8*time.Second)
		if err == harness.ErrBlocked {
			return z28Res{}, nil
		}
		if err != nil {
			return z28Res{}, err
		}
		return z28Res{int(time.Since(t0) / time.Millisecond), bytes.Count(stdout, []byte{7}), rc, true}, nil
	}
	// SEQUENTIAL and in the Python's order, because the measurement is wall
	// time and running them at once would make each one's load the others'.
	type who struct{ Name, path, which string }
	whos := []who{
		{"old", filepath.Join(state, "old"), "all"}, {"new", filepath.Join(tmp, "new"), "all"},
		{"ceil", filepath.Join(tmp, "ceil"), "all"}, {"epoch", filepath.Join(tmp, "epoch"), "all"},
		{"fast", filepath.Join(tmp, "fast"), "sleep"}, {"froze", filepath.Join(tmp, "froze"), "one"},
		{"nobell", filepath.Join(tmp, "nobell"), "bell"}, {"allbell", filepath.Join(tmp, "allbell"), "bell"},
	}
	wantBy := map[string][]string{
		"all": {"1gs", "2gs", "hgshh"}, "sleep": {"1gs", "2gs"},
		"one": {"1gs"}, "bell": {"hgshh"},
	}
	res := map[string]map[string]z28Res{}
	for _, x := range whos {
		res[x.Name] = map[string]z28Res{}
		for _, p := range z28Probes {
			if !containsStr28(wantBy[x.which], p.Name) {
				continue
			}
			got, err := run(x.path, p.Keys)
			if err != nil {
				return err
			}
			res[x.Name][p.Name] = got
		}
	}

	r3 := &check.Rep{Tag: "clock", W: w}
	for _, who := range []string{"old", "new", "ceil", "epoch"} {
		for _, p := range z28Probes {
			g := res[who][p.Name]
			if !g.ok {
				r3.Bad("%s: the %s probe never returned", who, p.Name)
				continue
			}
			if g.Rc != 0 {
				r3.Bad("%s: the %s probe exited %d", who, p.Name, g.Rc)
			}
			if g.bells != p.bells {
				r3.Bad("%s: the %s probe rang %d bell(s) and %d were expected",
					who, p.Name, g.bells, p.bells)
			}
		}
		one, two := res[who]["1gs"], res[who]["2gs"]
		if !one.ok || !two.ok {
			continue
		}
		if one.ms < 950 {
			r3.Bad("%s: `1gs` returned after %d ms and do_sleep(1000) cannot be shorter "+
				"than 1,000", who, one.ms)
		}
		// A LOWER BOUND ON A SLEEP, NEVER A DIFFERENCE BETWEEN TWO RUNS: each run
		// carries its own startup jitter, and a sleep takes at least as long as it
		// asks for while load can only make it longer.
		if two.ms < 1900 {
			r3.Bad("%s: `2gs` took %d ms, where two capped 1,000 ms waits were expected "+
				"-- do_sleep's loop is not measuring time", who, two.ms)
		}
	}
	fone, ftwo := res["fast"]["1gs"], res["fast"]["2gs"]
	if !fone.ok || !ftwo.ok {
		r3.Bad("the `fast` control did not return, and it was chosen because it does")
	} else if ftwo.ms >= 1900 {
		r3.Bad("the `fast` control -- a clock running 1000x -- gave `2gs` %d ms, which is "+
			"over the bound the assertion above uses.  It should return after ONE wait, "+
			"and if it does not that assertion proves nothing", ftwo.ms)
	}
	if fr := res["froze"]["1gs"]; fr.ok {
		r3.Bad("the `froze` control -- a clock that never advances -- returned from `1gs` "+
			"after %d ms.  do_sleep's `done` can never reach 1,000 with a clock that does "+
			"not move, so the probe is not measuring do_sleep", fr.ms)
	}
	nob, allb := res["nobell"]["hgshh"], res["allbell"]["hgshh"]
	if nob.bells != 1 {
		r3.Bad("the `nobell` control -- vim_beep's 500 written 500000 -- rang %s bells on "+
			"`hgshh` and 1 was expected.  The second h rings BECAUSE more than 500 ms have "+
			"passed, and if moving the threshold does not stop it the probe is measuring "+
			"something else", z28Opt(nob))
	}
	if allb.bells != 3 {
		r3.Bad("the `allbell` control -- vim_beep's 500 written -1 -- rang %s bells on "+
			"`hgshh` and 3 were expected.  The THIRD h is suppressed because fewer than "+
			"500 ms have passed, and if making the test always true does not let it ring, "+
			"that half of the probe measures nothing", z28Opt(allb))
	}
	if err := r3.Done(); err != nil {
		return err
	}
	r.Say("THE PROBES: `1gs` %d ms and `2gs` %d on the binary this phase was HANDED, %d "+
		"and %d on its own -- do_sleep's loop, at one wait and at two.  `hgshh` rings 2 "+
		"bells on both, which is vim_beep's threshold in BOTH directions in one probe: h "+
		"at column 0 rings, gs lets a second pass, the next h rings because more than 500 "+
		"ms have gone and the third is suppressed because fewer than 500 have",
		res["old"]["1gs"].ms, res["old"]["2gs"].ms, res["new"]["1gs"].ms, res["new"]["2gs"].ms)
	r.Say("AND EACH HALF FAILS ON A CONTROL AIMED AT IT.  The timing: with the clock "+
		"running 1000x fast `2gs` is %d ms against `1gs` %d -- one wait instead of two, "+
		"the difference gone -- and with a clock that never advances `1gs` NEVER "+
		"RETURNS.  The bells: a clock control cannot answer for those, because one that "+
		"breaks the rate limit breaks do_sleep first and `hgshh` then blocks, so the two "+
		"bell controls move vim_beep's 500 instead -- to 500000, and `hgshh` rings %d; "+
		"to -1, and it rings %d", ftwo.ms, fone.ms, nob.bells, allb.bells)
	r.Say("`ceil` -- every reading rounded UP instead of down, which perturbs each one "+
		"by up to a full millisecond, TWICE what section 4 measured -- gives `1gs` %d "+
		"ms, `2gs` %d and 2 bells, exactly the product.  `epoch` -- milliseconds since "+
		"1970 rather than since the whole second of the first call -- gives %d, %d and "+
		"2.  So no caller can see one millisecond, and the origin is a free choice",
		res["ceil"]["1gs"].ms, res["ceil"]["2gs"].ms,
		res["epoch"]["1gs"].ms, res["epoch"]["2gs"].ms)

	wgR.Wait()
	if check.RecReport(w, recErr...) {
		return harness.ErrReported
	}
	for _, vv := range []string{"old", "ceil", "epoch"} {
		dq, _ := exec.Command("diff", "-rq", filepath.Join(tmp, "REC."+vv),
			filepath.Join(tmp, "REC.new")).CombinedOutput()
		if len(dq) > 0 {
			r.Say("the declared delta is NOTHING AT ALL and the recording of `%s` differs "+
				"from the output's:", vv)
			return harness.ErrReported
		}
	}
	// THE CORPUS IS BLIND TO THE CLOCK, and this is that measurement rather than
	// a claim: no case rings the bell twice, so vim_beep's rate limit is never
	// asked to suppress anything.
	scr := filepath.Join(tmp, "REC.new", "screen")
	ents, _ := os.ReadDir(scr)
	var names []string
	for _, en := range ents {
		names = append(names, en.Name())
	}
	sort.Strings(names)
	var bells []int
	for _, n := range names {
		for _, line := range strings.Split(check.ReadFile(filepath.Join(scr, n)), "\n") {
			if strings.HasPrefix(line, "--- bells ") {
				if fs := strings.Fields(line); len(fs) > 2 {
					b, _ := strconv.Atoi(fs[2])
					bells = append(bells, b)
				}
				break
			}
		}
	}
	one, zero, maxb := 0, 0, 0
	for _, b := range bells {
		if b > maxb {
			maxb = b
		}
		if b == 1 {
			one++
		}
		if b == 0 {
			zero++
		}
	}
	if maxb > 1 {
		many := 0
		for _, b := range bells {
			if b > 1 {
				many++
			}
		}
		return stop("%d of the %d screen cases now ring the bell more than once, so the "+
			"corpus CAN see vim_beep's rate limit and this phase's empty declaration needs "+
			"re-arguing rather than asserting", many, len(bells))
	}
	r.Say("the declared delta is NOTHING AT ALL and FOUR FULL RECORDINGS ARE "+
		"BYTE-IDENTICAL -- the binary this phase was handed, its own, `ceil` and `epoch` "+
		"-- across 102 screen cases, every Ex command, every command line, the pty "+
		"scenarios and the terminal table.  AND THE CORPUS IS BLIND TO THE CLOCK, "+
		"measured: %d of the %d cases ring the bell once and %d not at all, and NOT ONE "+
		"rings it twice, so vim_beep's 500 ms limit -- the only reading a screen case "+
		"could see -- is never asked to suppress anything.  That is why this phase owes "+
		"the probes above, and they are what answer for it", one, len(bells), zero)
	return nil
}

// z28Opt is Python's `%s` of a value that may be None: a probe that blocked
// printed `None` there, and the Go must too or the refusal reads differently.
func z28Opt(x z28Res) string {
	if !x.ok {
		return "None"
	}
	return strconv.Itoa(x.bells)
}

func containsStr28(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
