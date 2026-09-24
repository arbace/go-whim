package p115

// Whim phase 115, the check -- the clock crosses the boundary.
// See phase/115/edit.go, and GOALS.md II.4c.
//
// Runs after phase/115/edit.go and the sweep internal/verify runs between them, and
// reads nothing from the edit's shell -- only the work tree and the state directory.
// What the edit left there is `old.c`, the source this phase was HANDED, and `old`, that
// source built with SOURCE_DATE_EPOCH=0 and the boundary's own flags.
//
// WHAT IS CLAIMED, in eight parts:
//
// ARITHMETIC  computed FROM THE INPUT: the core does not name `time` at all, the four
// mentions it had having become none; `host_time` is 8 above the boundary
// and 1 below; the libc prototype block loses exactly one line and keeps
// every other entry; the core loses 7 lines and the host gains 7.
// THE GUARANTEE
// THE PART OF THIS PHASE THAT COULD SILENTLY REGRESS, as FOUR compiles.
// `long time(long *tp);` was what pinned `time_T`'s width, and it is
// replaced by a static_assert rather than deleted.  m1 shows the prototype
// really was a check (on the INPUT, written wrong: `conflicting types for
// 'time'`); m2 shows what it did NOT check (on the INPUT, `time_T`
// perturbed to `int` with the prototype untouched: SILENT); p1 shows the
// hole this phase would leave without its replacement (the OUTPUT with the
// assert deleted and `time_T` perturbed: SILENT); and p2/p3 show the
// replacement closing it (`static assertion failed: "time_T is time_t"`).
// The assert is therefore STRICTLY STRONGER than the prototype, and the
// check says so with m2 rather than claiming an equivalence.
// THE BOUNDARY
// `make editor.c`'s cut, computed here by the same awk clause on BOTH
// sides: 0 directives, `-fsyntax-only` with no error and no warning that
// is not a boundary name, and the warning set compared AT RUN TIME with
// the input's -- exactly `host_time` arriving, nothing gone, 13 -> 14.
// The thirteen are never written out: phase 111 renamed one of them and a
// list here would already be stale.
// CANON       tools/canon.sh is a NO-OP on the output.
// HOST        `zhostonly`, unchanged: `time` is not in its vocabulary and this
// phase deliberately does not add it -- see THE TOOL THIS PHASE DOES NOT
// EDIT below.
// SYMBOLS     `nm -u` is THE SAME SET -- 17 names, `comm` empty in both directions --
// and `main` is still the only external symbol.  **`time` DOES NOT LEAVE**,
// and this is said as an equality rather than left for a reader to expect
// otherwise: the host still calls it to implement host_time(), and a symbol
// leaves when its last caller leaves the FILE, which is the split and not
// this phase.  That is phase 111's sentence about `gettimeofday`, and the
// two clocks are now in exactly the same position.
// THE READS   THE INSTRUMENTED PAIR, and it is the evidence the recording cannot give.
// `write(2, "TICK\n", 5)` at EVERY clock read on both sides -- on the input
// inside vim_time() and, as a comma expression, at each of
// ui_focus_change's two direct reads; on the output inside host_time(),
// which is now every read there is.  The two instrumented 102-case
// recordings must be BYTE-IDENTICAL, which is a statement about the screens
// AND about the number and the order of the clock reads in every case.
// Plus focus probes, because no corpus case reaches ui_focus_change at all.
// BEHAVIOUR   the declared delta is NOTHING AT ALL: two full recordings, of the binary
// this phase was handed and of its own, byte-identical across all 106
// records.  tools/st.sh delta is run by internal/verify after this check
// and is the second opinion.
//
// THE CORPUS CANNOT REACH ui_focus_change AND THE PHASE SAYS SO RATHER THAN HOPING.
// `ui_focus_change()` is called from `handle_key_without_mapping`'s KE_FOCUSGAINED and
// KE_FOCUSLOST arms, and those key codes arrive as `\033[I` and `\033[O`, which
// `set_termname()` registers unconditionally.  So a keystroke file CAN drive it -- which
// GOALS.md II.2l calls a hazard (a typed Escape followed by `[` is read as a key code)
// and which is exactly what is wanted here.  The probes are:
//
// focus        \033[O \033[I         3 TICKs on both binaries
// focus_twice  \033[O \033[I x2      4 TICKs on both binaries
//
// and the arithmetic is worth writing down because the control turns on it.  `focus_state`
// starts MAYBE, so the first `\033[O` calls ui_focus_change(FALSE) -- which reads NOTHING,
// `in_focus &&` short-circuiting -- and the first `\033[I` calls it with TRUE, where
// `last_time` is 0, the test is true and BOTH reads happen.  On the second round the test
// is FALSE, so only the condition reads.  0 + 2 + 0 + 1, and one more for the `:q!` that
// reaches add_to_history: four.
//
// AND THE CONTROL IS THE QUESTION THE USER ASKED, MADE INTO A PROGRAM.  `hoist` is the
// output with ui_focus_change's two reads collapsed into one local -- which is what
// "could the two reads now straddle a second boundary differently" would mean if it were
// true -- and it reads the clock ONCE PER CALL, unconditionally: 1 + 1 + 1 + 1 + 1 = 5
// where the product gives 4.  So the instrument can see a collapsed read, the product
// does not collapse one, and the answer to the question is measured rather than argued:
// two reads before, two reads after, in the same two statements and the same order.
//
// A HAZARD THIS PHASE FOUND IN THE SHARED RECORDING, NOT WORKED AROUND HERE.  Comparing
// two FULL recordings is load-sensitive in exactly three records, and this phase is the
// one that would notice.  `tools/zrec.py` scrubs the undo message's elapsed time to
// `<ago>` PADDED TO THE WIDTH IT REPLACES, so the SCREEN is protected -- but the record
// also carries `--- stream <len> sha=<...>`, and that digest is taken over the RAW byte
// stream, where "0 seconds ago" and "1 second ago" are 13 bytes and 12.  MEASURED with a
// control built for it, `add_time()` reporting one second more: exactly three records
// move -- undo_after_ins, undo_block, undo_redo, which are exactly the three whose screen
// carries `<ago>` -- and in each of them exactly ONE line moves, the `--- stream` line,
// with the 24 screen lines byte-identical.  Under a 33-way concurrent `make whim-verify`
// the gap between `u_savecommon()`'s stamp and `undo_time()`'s read can straddle a second
// tick, and one such run failed here on `undo_after_ins` alone.
//
// IT IS NOT THIS PHASE'S TO FIX AND NOT THIS PHASE'S TO PAPER OVER.  Every Part II phase
// since 3 compares two full recordings and every one of them is exposed; so is
// tools/st.sh delta, which compares against baselines recorded the same way.  Hashing
// the SCRUBBED stream in tools/zrec.py would close it, and would re-key all 33 Part II
// phases and require .reference/core-baselines to be recorded again.  A private exclusion
// HERE would be a check narrowed to fit what it saw, would leave coredelta failing on the
// same load, and is refused: the comparison below stays an exact `diff -rq`.  The reading
// that matters for THIS phase is that the exposure is unchanged by it -- the undo path
// reads the clock the same number of times before and after, which is what the
// instrumented pair measures.
//
// THE TOOL THIS PHASE DOES NOT EDIT, and it is a decision rather than an oversight.
// ``zhostonly`` asserts that the core names none of the host's vocabulary, and
// `gettimeofday` joined that vocabulary at phase 111 for exactly this shape of reason.
// `time` is NOT added here.  Adding it would re-key phases 103, 104, 108, 109 and 110, whose
// checks run the tool on their own output, and every one of those boundaries has the core
// calling `time()` -- so each would need a named exception with a count, and four
// boundaries would have to be re-verified to buy a fact this check already asserts
// directly and more strongly: `\btime\b` is at ZERO above the boundary, computed on the
// literal-stripped text, and the `make editor.c` cut compiles with `host_time` as a
// boundary name.  The tool is still RUN, on this phase's output, so that nothing else in
// its vocabulary moved.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim115", Check) }

var (
	w115Time     = regexp.MustCompile(`\btime\b`)
	w115VimTimeC = regexp.MustCompile(`\bvim_time\b`)
	w115Host     = regexp.MustCompile(`\bhost_time\b`)
	w115HostC    = regexp.MustCompile(`\bhost_time\(\)`)
	w115VimC     = regexp.MustCompile(`\bvim_time\(\)`)
	w115Null     = regexp.MustCompile(`\btime\(nullptr\)`)
	w115TimeTC   = regexp.MustCompile(`\btime_t\b`)
	w115TimeTTC  = regexp.MustCompile(`\btime_T\b`)
)

// w115Strip is zhostonly's strip_strings: every string and character literal
// becomes one space, so English in an NGETTEXT is not counted as code.
func w115StripC(line string) string {
	var Out strings.Builder
	i, n := 0, len(line)
	for i < n {
		c := line[i]
		if c == '"' || c == '\'' {
			q := c
			Out.WriteByte(' ')
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
		Out.WriteByte(c)
		i++
	}
	return Out.String()
}

type w115Res struct {
	ok      bool
	got, Rc int
}

func (r w115Res) Show() string {
	if !r.ok {
		return "None"
	}
	return strconv.Itoa(r.got)
}

// Whim115 is phase 115's check: the clock crosses the boundary.
func Check(w io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: check whim115 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	f := filepath.Join(work, "whim-vim.c")
	oldC := filepath.Join(state, "old.c")
	r := &check.Rep{Tag: "wallclock", W: w}
	stop := func(format string, a ...any) error {
		r.Say(format, a...)
		return harness.ErrReported
	}
	head := func(ls []string, n int, prefix string) {
		for i, l := range ls {
			if i >= n {
				break
			}
			fmt.Fprintf(w, "%s%s\n", prefix, l)
		}
	}
	fileLines := func(p string) []string {
		s := strings.TrimRight(check.ReadFile(p), "\n")
		if s == "" {
			return nil
		}
		return strings.Split(s, "\n")
	}
	beforeRaw := strings.TrimRight(check.ReadFile(filepath.Join(state, "input-lines")), "\n")
	tmp, err := os.MkdirTemp("", "whim115-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	T := func(n string) string { return filepath.Join(tmp, n) }
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflagsS, ldflagsS := "", ""
	if m := check.W112CFlags.FindStringSubmatch(mk); m != nil {
		cflagsS = m[1]
	}
	if m := check.W112LDFlags.FindStringSubmatch(mk); m != nil {
		ldflagsS = m[1]
	}
	link := func(src, Out string) error {
		a := append(append(strings.Fields(cflagsS), strings.Fields(ldflagsS)...), "-o", Out, src)
		c := exec.Command("gcc", a...)
		c.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
		return c.Run()
	}

	// --- 0. the controls -------------------------------------------------------
	newT, oldT := check.ReadFile(f), check.ReadFile(oldC)
	const TD = "typedef long        time_T;"
	const AS = "static_assert(_Generic((time_T)0, time_t: 1, default: 0), \"time_T is time_t\");\n"
	const PR = "long time(long *tp);"
	for _, x := range []struct{ Text, What, Where string }{
		{newT, TD, "the output"}, {newT, AS, "the output"}, {oldT, TD, "the input"}, {oldT, PR, "the input"},
	} {
		if strings.Count(x.Text, x.What) != 1 {
			return stop("`%s` is not in %s exactly once, so the controls below would not be controls",
				strings.TrimSpace(x.What), x.Where)
		}
	}
	const INT = "typedef int         time_T;"
	const LL = "typedef long long   time_T;"
	type ctl struct{ Name, Text string }
	files := []ctl{
		{"m1", strings.Replace(oldT, PR, "int time(int *tp);", 1)},
		{"m2", strings.Replace(oldT, TD, INT, 1)},
		{"p1", strings.Replace(strings.Replace(newT, TD, INT, 1), AS, "", 1)},
		{"p2", strings.Replace(newT, TD, INT, 1)},
		{"p3", strings.Replace(newT, TD, LL, 1)},
	}
	const TICK = "    write(2, \"TICK\\n\", 5);\n"
	const OW = "vim_time(void)\n{\n    return time(nullptr);\n}"
	const OF = `    if (in_focus && last_time + 2 < time(nullptr))
    {
        last_time = time(nullptr);
    }
`
	const NW = "host_time(void)\n{\n    return time(nullptr);\n}"
	const NF = `    if (in_focus && last_time + 2 < host_time())
    {
        last_time = host_time();
    }
`
	for _, x := range []struct{ Text, What, Where string }{
		{oldT, OW, "the input"}, {oldT, OF, "the input"}, {newT, NW, "the output"}, {newT, NF, "the output"},
	} {
		if strings.Count(x.Text, x.What) != 1 {
			s := strings.ReplaceAll(x.What, "\n", "\\n")
			if len(s) > 60 {
				s = s[:60]
			}
			return stop("`%s` is not in %s exactly once, so the instrument would not be at every "+
				"clock read", s, x.Where)
		}
	}
	tOld := strings.Replace(strings.Replace(oldT, OW, "vim_time(void)\n{\n"+TICK+"    return time(nullptr);\n}", 1),
		OF, `    if (in_focus && last_time + 2 < (write(2, "TICK\n", 5), time(nullptr)))
    {
        last_time = (write(2, "TICK\n", 5), time(nullptr));
    }
`, 1)
	tNew := strings.Replace(newT, NW, "host_time(void)\n{\n"+TICK+"    return time(nullptr);\n}", 1)
	hoist := strings.Replace(tNew, NF, `    long        now = host_time();

    if (in_focus && last_time + 2 < now)
    {
        last_time = now;
    }
`, 1)
	files = append(files, ctl{"t_old", tOld}, ctl{"t_new", tNew}, ctl{"hoist", hoist})
	for _, c := range files {
		if c.Text == newT || c.Text == oldT {
			return stop("%s changed nothing, so it is not a control", c.Name)
		}
		if err := os.WriteFile(T(c.Name+".c"), []byte(c.Text), 0o644); err != nil {
			return err
		}
	}
	r.Say("eight controls written: m1 the input's `time` prototype written wrong, " +
		"m2 the input's time_T perturbed with the prototype LEFT ALONE, p1 the output " +
		"with the static_assert deleted and time_T perturbed, p2 and p3 the same " +
		"perturbations WITH the assert, t_old and t_new the instrumented pair -- five " +
		"bytes at every clock read on each side -- and hoist, which collapses " +
		"ui_focus_change's two reads into one")

	var wgNew, wgT, wgSyn, wgCanon sync.WaitGroup
	var errNew, errCanon error
	var canonLog []byte
	wgNew.Add(1)
	go func() { defer wgNew.Done(); errNew = link(f, T("new")) }()
	for _, v := range []string{"t_old", "t_new", "hoist"} {
		v := v
		wgT.Add(1)
		go func() { defer wgT.Done(); link(T(v+".c"), T(v)) }()
	}
	for _, v := range []string{"m1", "m2", "p1", "p2", "p3"} {
		v := v
		wgSyn.Add(1)
		go func() {
			defer wgSyn.Done()
			c := exec.Command("gcc", "-O0", "-fno-stack-protector", "-fsyntax-only", T(v+".c"))
			lf, _ := os.Create(T("w." + v))
			c.Stderr = lf
			c.Run()
			lf.Close()
		}()
	}
	os.WriteFile(T("canon.c"), []byte(newT), 0o644)
	wgCanon.Add(1)
	go func() {
		defer wgCanon.Done()
		canonLog, errCanon = exec.Command("sh", "tools/canon.sh", T("canon.c")).CombinedOutput()
	}()
	defer func() { wgNew.Wait(); wgT.Wait(); wgSyn.Wait(); wgCanon.Wait() }()

	// --- 1. the source, as arithmetic on the input ------------------------------
	beforeLines, _ := strconv.Atoi(strings.TrimSpace(beforeRaw))
	split := func(text, which string) (string, string, []string, int, error) {
		lines := strings.Split(text, "\n")
		var d []int
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimLeft(l, " \t\n\v\f\r"), "#") {
				d = append(d, i)
			}
		}
		ok := len(d) == 11
		for k := range d {
			if ok && d[k] != d[0]+k {
				ok = false
			}
		}
		if !ok {
			var at []string
			for k, x := range d {
				if k >= 3 {
					break
				}
				at = append(at, strconv.Itoa(x+1))
			}
			return "", "", nil, 0, stop("%s does not have eleven contiguous directives: %d at %s.  The "+
				"first one IS the boundary and every count below distinguishes the core "+
				"from the host", which, len(d), strings.Join(at, " "))
		}
		var c, b []string
		for _, l := range lines[:d[0]] {
			c = append(c, w115StripC(l))
		}
		for _, l := range lines[d[0]:] {
			b = append(b, w115StripC(l))
		}
		return strings.Join(c, "\n"), strings.Join(b, "\n"), lines, d[0], nil
	}
	ocore, obelow, olines, ocut, e := split(oldT, "old.c")
	if e != nil {
		return e
	}
	ncore, nbelow, nlines, ncut, e := split(newT, "the output")
	if e != nil {
		return e
	}
	cnt := func(re *regexp.Regexp, s string) int { return len(re.FindAllStringIndex(s, -1)) }
	if len(olines)-1 != beforeLines {
		r.Bad("the state directory says the edit was handed %d lines and old.c has %d", beforeLines, len(olines)-1)
	}
	was, now := cnt(w115Time, ocore), cnt(w115Time, ncore)
	if was != 4 {
		r.Bad("the INPUT's core names `time` %d times and this phase was written "+
			"against 4 -- the libc prototype, the wrapper's call and "+
			"ui_focus_change's two", was)
	}
	if now > 0 {
		r.Bad("the output's core still names `time` %d times, and the whole product "+
			"of this phase is that it names it none", now)
	}
	if n := cnt(w115VimTimeC, newT); n > 0 {
		r.Bad("`vim_time` survives in the output, %d times", n)
	}
	for _, x := range []struct {
		Where, Text string
		want        int
		why         string
	}{
		{"above the boundary", ncore, 8, "the declaration and seven call sites"},
		{"below the boundary", nbelow, 1, "its definition"},
	} {
		if n := cnt(w115Host, x.Text); n != x.want {
			r.Bad("`host_time` occurs %d times %s where %d were expected -- %s", n, x.Where, x.want, x.why)
		}
	}
	calls := cnt(w115HostC, ncore)
	owrap := cnt(w115VimC, ocore)
	obypass := cnt(w115Null, ocore) - 1
	if calls != owrap+obypass {
		r.Bad("the core makes %d calls to host_time() and the input made %d to "+
			"vim_time() and %d directly to time(nullptr) outside the wrapper", calls, owrap, obypass)
	}
	if owrap != 5 || obypass != 2 {
		r.Bad("the input had %d wrapper calls and %d bypassing ones, where 5 and 2 "+
			"were counted", owrap, obypass)
	}
	if cnt(w115Time, nbelow) != cnt(w115Time, obelow)+1 {
		r.Bad("`time` is %d below the boundary and the input had %d there: host_time "+
			"brings exactly one call with it, beside the `#include <time.h>`",
			cnt(w115Time, nbelow), cnt(w115Time, obelow))
	}
	if n := cnt(w115TimeTC, ncore+nbelow); n != 1 {
		r.Bad("`time_t` is named %d times in the file's CODE and must be named once "+
			"-- the static_assert, which is the only place a core name and a header "+
			"name are both in scope.  The assert's own message says the name again "+
			"and is a literal", n)
	}
	if cnt(w115TimeTTC, ncore) != cnt(w115TimeTTC, ocore)-2 {
		r.Bad("`time_T` is %d in the core and the input had %d: the two that go are "+
			"the wrapper's prototype and its definition head", cnt(w115TimeTTC, ncore), cnt(w115TimeTTC, ocore))
	}
	const DECL = "static void host_message(const char *msg, int len, int err);\nstatic long host_time(void);\n"
	if !strings.Contains(newT, DECL) {
		r.Bad("`static long host_time(void);` is not on the line after " +
			"host_message's declaration: the host block is ONE block and this is " +
			"the fourteenth name in it")
	}
	if !strings.Contains(ncore, "typedef long        time_T;") {
		r.Bad("`typedef long        time_T;` is not in the core, so host_time " +
			"returning `long` would be a conversion at seven call sites rather than " +
			"an assignment")
	}
	const DEF = "    static long\nhost_time(void)\n{\n    return time(nullptr);\n}\n"
	if strings.Count(newT, DEF) != 1 {
		r.Bad("host_time's definition is not below the boundary exactly once in the " +
			"shape the edit wrote")
	}
	if !strings.Contains(nbelow+"\n", DEF) {
		r.Bad("host_time is defined above the boundary")
	}
	block := func(lines []string, which string) ([]string, error) {
		var at []int
		for i, l := range lines {
			if l == "void *malloc(usize n);" {
				at = append(at, i)
			}
		}
		if len(at) != 1 {
			return nil, stop("`void *malloc(usize n);` is not on a line of its own exactly "+
				"once in %s, so the prototype block cannot be found", which)
		}
		a, b := at[0], at[0]
		for a > 0 && strings.TrimSpace(lines[a-1]) != "" {
			a--
		}
		for b+1 < len(lines) && strings.TrimSpace(lines[b+1]) != "" {
			b++
		}
		return lines[a : b+1], nil
	}
	ob, e := block(olines, "old.c")
	if e != nil {
		return e
	}
	nb, e := block(nlines, "the output")
	if e != nil {
		return e
	}
	notIn := func(a, b []string) []string {
		var Out []string
		for _, x := range a {
			if !check.Contains(b, x) {
				Out = append(Out, x)
			}
		}
		return Out
	}
	if lost := notIn(ob, nb); strings.Join(lost, "\x00") != "long time(long *tp);" {
		r.Bad("the prototype block lost %s and this phase removes exactly "+
			"`long time(long *tp);`", strings.Join(lost, " / "))
	}
	if gained := notIn(nb, ob); len(gained) > 0 {
		r.Bad("the prototype block gained %s", strings.Join(gained, " / "))
	}
	if len(nb) != len(ob)-1 {
		r.Bad("the prototype block is %d lines and was %d", len(nb), len(ob))
	}
	// -8 in the core, and it was -7.  The forward declaration costs TWO lines
	// and not one: the canonical text writes a blank line between forward
	// declarations, so `static time_T vim_time(void);` takes its blank with it,
	// where in the residue the block was consecutive lines.  Measured: the core
	// 76,017 -> 76,009 and the host 1,686 -> 1,693.
	if ncut-ocut != -8 {
		r.Bad("the core is %d lines and was %d, a difference of %d where -8 was "+
			"expected: -2 for the forward declaration with its blank line, +1 for the "+
			"host-block one, -6 for the definition with its blank and -1 for the libc "+
			"prototype", ncut, ocut, ncut-ocut)
	}
	if hd := (len(nlines) - ncut) - (len(olines) - ocut); hd != 7 {
		r.Bad("the host is %d lines and was %d, a difference of %d where +7 was "+
			"expected: +6 for the definition with its blank and +1 for the "+
			"static_assert", len(nlines)-ncut, len(olines)-ocut, hd)
	}
	// The two halves DO NOT cancel any more, and the one line is named: the core
	// loses 8 and the host gains 7, because the blank line that separated the
	// forward declaration from its neighbours goes with it and the host block
	// the declaration joins already had its own separators.  So the file is one
	// line shorter, and the assertion states that one line rather than zero.
	if len(nlines) != len(olines)-1 {
		r.Bad("the file is %d lines and was %d, and exactly one line was expected to "+
			"go -- the blank line above the forward declaration, which the core loses "+
			"and the host does not gain", len(nlines)-1, len(olines)-1)
	}
	if check.W110Runs(nlines) != check.W110Runs(olines) {
		r.Bad("the edit left %d runs of two blank lines where there were %d", check.W110Runs(nlines), check.W110Runs(olines))
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("THE CORE DOES NOT NAME `time` AT ALL: %d mentions in the input's core -- "+
		"the libc prototype, the wrapper's call and ui_focus_change's TWO -- and 0 here, "+
		"counted on the literal-stripped text because two NGETTEXT strings say the English "+
		"word.  `host_time` is 8 above the boundary and 1 below, and its %d call sites are "+
		"the input's %d wrapper calls plus the %d that bypassed it", was, calls, owrap, obypass)
	r.Say("the boundary crosses as `static long host_time(void);`, on the line below " +
		"host_message's, and NOT as `time_T`: the definition is below the boundary and the " +
		"host cannot name a core typedef once the file is cut, which is musl_now_ms's own " +
		"reason.  `typedef long        time_T;` is what makes every call site an assignment " +
		"and not a conversion")
	var bnames []string
	for _, l := range nb {
		fs := strings.Fields(strings.SplitN(l, "(", 2)[0])
		name := ""
		if len(fs) > 0 {
			name = strings.TrimLeft(fs[len(fs)-1], "*")
		}
		bnames = append(bnames, name)
	}
	r.Say("the libc prototype block is %d lines and was %d, losing `long time(long "+
		"*tp);` and nothing else: %s", len(nb), len(ob), strings.Join(bnames, " "))
	r.Say("the core is %d lines against %d (-8) and the host %d against %d (+7), so "+
		"the file is one line shorter -- %d against %d, the blank line the forward "+
		"declaration took with it -- and `time_t` is named ONCE in the whole file: "+
		"the static_assert", ncut, ocut, len(nlines)-ncut, len(olines)-ocut,
		len(nlines)-1, len(olines)-1)

	// --- 2. THE GUARANTEE, as four compiles -------------------------------------
	wgSyn.Wait()
	wl := func(v string) string { return check.ReadFile(T("w." + v)) }
	if !strings.Contains(wl("m1"), "conflicting types for 'time'") {
		r.Say("m1 -- the INPUT's `long time(long *tp);` written `int time(int *tp);` -- did not give " +
			"`conflicting types for 'time'`, so the prototype this phase removes was not a check after all and " +
			"there is nothing to replace:")
		head(fileLines(T("w.m1")), 6, "")
		return harness.ErrReported
	}
	if check.SizeOf(T("w.m2")) > 0 {
		r.Say("m2 -- the INPUT's time_T perturbed to `int` with the prototype LEFT ALONE -- was expected to " +
			"compile SILENTLY, and did not:")
		head(fileLines(T("w.m2")), 6, "")
		return harness.ErrReported
	}
	if check.SizeOf(T("w.p1")) > 0 {
		r.Say("p1 -- the OUTPUT with the static_assert deleted and time_T perturbed to `int` -- was expected " +
			"to compile SILENTLY, which is the hole this phase would leave, and did not:")
		head(fileLines(T("w.p1")), 6, "")
		return harness.ErrReported
	}
	for _, v := range []string{"p2", "p3"} {
		if !strings.Contains(wl(v), `static assertion failed: "time_T is time_t"`) {
			r.Say("%s -- the OUTPUT with time_T perturbed and the static_assert PRESENT -- did not fail the "+
				"assertion, so the guarantee that replaces the prototype cannot be broken and is therefore not "+
				"a guarantee:", v)
			head(fileLines(T("w."+v)), 6, "")
			return harness.ErrReported
		}
	}
	r.Say("THE PROTOTYPE WAS LOAD-BEARING AND WHAT REPLACES IT IS STRONGER, in four compiles.  m1: the input's " +
		"`long time(long *tp);` written `int time(int *tp);` is `conflicting types for 'time'` from <time.h> " +
		"below it -- that WAS the guarantee, and it is what phase 109 relied on when it wrote `typedef long " +
		"time_T;`.  m2: the same input with `time_T` perturbed to `int` and the prototype LEFT ALONE compiles " +
		"in SILENCE -- so the prototype pinned `long == time_t` and never `time_T == long`.  p1: the output " +
		"with the static_assert deleted and time_T perturbed compiles in silence too -- that is the regression " +
		"this phase would have shipped.  p2 and p3: with the assert present, `int` and `long long` both give " +
		"`static assertion failed: \"time_T is time_t\"`.  The one line names time_T ITSELF, which the " +
		"prototype could not")

	// --- 3. the editor.c cut, and its warning set compared WITH THE INPUT'S ------
	bset := map[string][]string{}
	cutN := map[string]int{}
	for _, x := range []struct{ side, src string }{{"old", oldC}, {"new", f}} {
		lines := check.W111Cut(check.ReadFile(x.src))
		var dl []string
		for i, l := range lines {
			if check.W113Dir.MatchString(l) {
				dl = append(dl, fmt.Sprintf("%d:%s", i+1, l))
			}
		}
		if len(dl) > 0 {
			r.Say("the %s cut holds a directive, so it found the wrong line:", x.side)
			head(dl, 3, "")
			return harness.ErrReported
		}
		text := ""
		for _, l := range lines {
			text += l + "\n"
		}
		cp := T("ed." + x.side + ".c")
		os.WriteFile(cp, []byte(text), 0o644)
		c := exec.Command("gcc", "-O0", "-fno-stack-protector", "-Wall", "-Wextra", "-Wno-unused-parameter",
			"-fsyntax-only", cp)
		var eb strings.Builder
		c.Stderr = &eb
		c.Run()
		var errs, warns []string
		for _, l := range strings.Split(eb.String(), "\n") {
			if strings.Contains(l, ": error:") {
				errs = append(errs, l)
			}
			if strings.Contains(l, ": warning: ") && !strings.Contains(l, "used but never defined") {
				warns = append(warns, l)
			}
		}
		if len(errs) > 0 {
			r.Say("the %s cut does not parse on its own:", x.side)
			head(errs, 4, "")
			return harness.ErrReported
		}
		set := map[string]bool{}
		for _, m := range check.W113Undef.FindAllStringSubmatch(eb.String(), -1) {
			set[m[1]] = true
		}
		bset[x.side] = check.W110Keys(set)
		if len(warns) > 0 {
			r.Say("the %s cut has a warning that is not a boundary name:", x.side)
			head(warns, 3, "")
			return harness.ErrReported
		}
		cutN[x.side] = len(lines)
	}
	gone := check.W114Words(check.Minus26(bset["old"], bset["new"]))
	came := check.W114Words(check.Minus26(bset["new"], bset["old"]))
	if gone != "" || came != "host_time " {
		return stop("the core -> host boundary moved by something other than host_time arriving: gone [%s] "+
			"arrived [%s].  This phase adds exactly one name to it and takes none away", gone, came)
	}
	r.Say("the `make editor.c` cut: %d lines -> %d, 0 directives, 0 errors under `-fsyntax-only`, and the "+
		"WHOLE warning set is the core -> host boundary -- %d names -> %d, with `host_time` ARRIVING and "+
		"NOTHING gone, compared name by name at run time and never written out here (phase 111 renamed one of "+
		"them and phase 114 renames none)", cutN["old"], cutN["new"], len(bset["old"]), len(bset["new"]))

	// --- 4. canon.sh ----------------------------------------------------------------
	wgCanon.Wait()
	if errCanon != nil {
		r.Say("tools/canon.sh failed on the output:")
		head(strings.Split(string(canonLog), "\n"), 10, "")
		return harness.ErrReported
	}
	if !check.W113Same(f, T("canon.c")) {
		r.Say("tools/canon.sh is not a no-op on the output -- the new text is not written the way this file " +
			"writes everything else:")
		head(check.W113Diff(f, T("canon.c")), 12, "")
		return harness.ErrReported
	}
	r.Say("tools/canon.sh is a NO-OP on the output: host_time's declaration, its definition and the " +
		"static_assert are written the way this file writes everything else")

	// --- 5. the host's vocabulary is still the host's ------------------------------
	hostOnly := exec.Command("sh", "tools/st.sh", "zhostonly", f)
	hostOnly.Stdout, hostOnly.Stderr = w, w
	if err := hostOnly.Run(); err != nil {
		return harness.ErrReported
	}

	// --- 6. the symbols, and the binary ----------------------------------------------
	wgNew.Wait()
	if errNew != nil {
		return stop("the output did not build with '%s' '%s'", cflagsS, ldflagsS)
	}
	for _, x := range []struct{ src, obj string }{{oldC, "old.o"}, {f, "new.o"}} {
		if Out, err := exec.Command("gcc", "-c", "-O0", "-fno-stack-protector", "-o", T(x.obj), x.src).CombinedOutput(); err != nil {
			w.Write(Out)
			return harness.ErrReported
		}
	}
	uOld := check.NmField26(T("old.o"), []string{"-u"}, 1)
	uNew := check.NmField26(T("new.o"), []string{"-u"}, 1)
	if g, c := check.Minus26(uOld, uNew), check.Minus26(uNew, uOld); len(g)+len(c) > 0 {
		return stop("`nm -u` moved: gone [%s] arrived [%s].  MOVING A CALL FROM THE CORE INTO THE HOST INSIDE "+
			"ONE TRANSLATION UNIT FREES NOTHING AND NEEDS NOTHING", check.W114Words(g), check.W114Words(c))
	}
	if !check.Contains(uNew, "time") {
		return stop("`time` is NOT in the undefined set, and it must be: the host still calls it to implement " +
			"host_time()")
	}
	ext := check.NmField26(T("new.o"), []string{"--extern-only", "--defined-only"}, 2)
	if s := check.W114Words(ext); s != "main " {
		return stop("the output defines external symbols other than main: %s", s)
	}
	if check.W113Same(T("new"), filepath.Join(state, "old")) {
		return stop("the output binary is byte-identical to the input's, which cannot be: a call replaces an " +
			"inlined read at two sites and five lines of definition move past two thousand")
	}
	r.Say("`nm -u` is THE SAME SET, %d names, as a `comm` empty in BOTH directions, and `main` is still the "+
		"only external symbol.  `time` IS STILL THERE and this phase says so as an equality, exactly as phase "+
		"111 did for `gettimeofday`: the host calls it to implement host_time(), and a symbol leaves when its "+
		"last CALLER leaves the FILE, which is the split and not this phase.  The binary is %d bytes against %d "+
		"and they are NOT the same bytes", len(uNew), check.SizeOf(T("new")), check.SizeOf(filepath.Join(state, "old")))
	if err := check.PhaseCheck(w, work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}

	// --- 7. the reads, instrumented, and the recordings ------------------------------
	wgT.Wait()
	for _, v := range []string{"t_old", "t_new", "hoist"} {
		if _, e := os.Stat(T(v)); e != nil {
			return stop("the %s build did not finish", v)
		}
	}
	oldBin, _ := filepath.Abs(filepath.Join(state, "old"))
	var wgR, wgC sync.WaitGroup
	recErr := make([]error, 2)
	casErr := make([]error, 2)
	for k, x := range []struct{ bin, src, Out string }{{oldBin, oldC, "REC.old"}, {T("new"), f, "REC.new"}} {
		k, x := k, x
		wgR.Add(1)
		go func() {
			defer wgR.Done()
			recErr[k] = check.RecCore(x.bin, x.src, T(x.Out))
		}()
	}
	for k, x := range []struct{ bin, Out string }{{T("t_old"), "SC.told"}, {T("t_new"), "SC.tnew"}} {
		k, x := k, x
		wgC.Add(1)
		go func() {
			defer wgC.Done()
			casErr[k] = check.RecCmd("sh", "tools/st.sh", "zcases", x.bin, T(x.Out))
		}()
	}

	// `\033[I` and `\033[O` are KE_FOCUSGAINED and KE_FOCUSLOST, registered
	// unconditionally by set_termname(), so a keystroke file reaches
	// ui_focus_change() -- the one clock reader no screen case touches.
	probes := []struct {
		Name string
		Keys string
		want int
	}{
		{"focus", "\x1b[O\x1b[I:q!\r", 3},
		{"focus_twice", "\x1b[O\x1b[I\x1b[O\x1b[I:q!\r", 4},
		{"edit", "ihi\x1b:q!\r", 2},
		{"undo", "ihi\x1bu:q!\r", 3},
		{"plainq", ":q!\r", 1},
	}
	res := map[string]map[string]w115Res{}
	for _, who := range []string{"t_old", "t_new", "hoist"} {
		res[who] = map[string]w115Res{}
		for _, p := range probes {
			_, _, se, rc, err := harness.CoreSession(T(who), [][]byte{[]byte(p.Keys)}, "xterm", nil, 24, 80, 8*time.Second)
			if err != nil {
				res[who][p.Name] = w115Res{}
				continue
			}
			res[who][p.Name] = w115Res{true, bytes.Count(se, []byte("TICK")), rc}
		}
	}
	r7 := &check.Rep{Tag: "wallclock", W: w}
	for _, p := range probes {
		for _, who := range []string{"t_old", "t_new"} {
			g := res[who][p.Name]
			if !g.ok {
				r7.Bad("%s: the %s probe never returned", who, p.Name)
				continue
			}
			if g.Rc != 0 {
				r7.Bad("%s: the %s probe exited %d", who, p.Name, g.Rc)
			}
			if g.got != p.want {
				r7.Bad("%s: the %s probe recorded %d clock reads and %d were expected", who, p.Name, g.got, p.want)
			}
		}
		o, n := res["t_old"][p.Name], res["t_new"][p.Name]
		if o.ok != n.ok || (o.ok && o.got != n.got) {
			r7.Bad("the %s probe reads the clock %s times on the binary this phase was "+
				"handed and %s times on its own.  The phase renames a read and moves "+
				"it; it does not add or remove one", p.Name, o.Show(), n.Show())
		}
	}
	if h := res["hoist"]["focus_twice"]; !h.ok || h.got != 5 {
		r7.Bad("the `hoist` control -- ui_focus_change reading the clock ONCE into a "+
			"local -- recorded %s clock reads on `focus_twice` and 5 were expected: "+
			"one per call plus the :q!.  If collapsing the two reads does not move "+
			"the probe, the probe cannot see whether they were collapsed", h.Show())
	}
	if h, n := res["hoist"]["focus"], res["t_new"]["focus"]; h.ok != n.ok || (h.ok && h.got != n.got) {
		r7.Bad("the `hoist` control moved the `focus` probe, where the arithmetic says " +
			"it must not: 1 + 1 against 0 + 2, plus the :q!, is 3 either way.  If " +
			"that has changed, the reasoning behind `focus_twice` needs re-doing")
	}
	if err := r7.Done(); err != nil {
		return err
	}
	tn := res["t_new"]
	r.Say("THE READS ARE THE SAME READS, at every probe: `focus` %s, `focus_twice` "+
		"%s, `edit` %s, `undo` %s, `plainq` %s -- identical on the instrumented input and "+
		"the instrumented output.  ui_focus_change still reads the clock TWICE, in two "+
		"statements, in the same order: at `focus_twice` the four calls read 0, 2, 0 and 1 "+
		"times, because in_focus FALSE short-circuits and the second FocusGained finds "+
		"last_time fresh", tn["focus"].Show(), tn["focus_twice"].Show(), tn["edit"].Show(),
		tn["undo"].Show(), tn["plainq"].Show())
	r.Say("AND THE INSTRUMENT CAN SEE A COLLAPSED READ: `hoist`, the output with "+
		"ui_focus_change's two reads hoisted into one local, reads ONCE PER CALL and "+
		"gives `focus_twice` %s against the product's %s.  So \"can the two reads straddle "+
		"a second differently now\" is answered by measurement -- the program that would "+
		"make it true is a DIFFERENT program and the probe says so",
		res["hoist"]["focus_twice"].Show(), tn["focus_twice"].Show())

	// A recording that fails says why, and ends the check: see recjob.go.
	wgC.Wait()
	if check.RecReport(w, casErr...) {
		return harness.ErrReported
	}
	if dl := check.DiffRQ(T("SC.told"), T("SC.tnew")); len(dl) > 0 {
		r.Say("the two INSTRUMENTED 102-case recordings differ, so the clock is read a different number of " +
			"times or in a different order somewhere in the corpus:")
		head(dl, 12, "")
		return harness.ErrReported
	}
	marked, total := 0, 0
	ents, _ := os.ReadDir(T("SC.tnew"))
	for _, en := range ents {
		d := check.ReadFile(filepath.Join(T("SC.tnew"), en.Name()))
		n := 0
		for _, l := range strings.Split(d, "\n") {
			if strings.Contains(l, "TICK") {
				n++
			}
		}
		if n > 0 {
			marked++
		}
		total += n
	}
	if marked < 1 {
		return stop("the instrument marks NO screen case, so the byte-identical instrumented recording above " +
			"is two empty files agreeing")
	}
	r.Say("THE INSTRUMENTED PAIR: the same five bytes at EVERY clock read on each side -- three sites on the "+
		"input (the wrapper and ui_focus_change's two, which bypass it) and one on the output, because after "+
		"this phase there is only one -- and the two 102-case recordings are BYTE-IDENTICAL.  The instrument is "+
		"not silent: it marks %d of 102 cases with %d reads in all, the two it does not being ctrl_c_clean and "+
		"ctrl_c_changed, which exit before a key is looked up", marked, total)
	wgR.Wait()
	if check.RecReport(w, recErr...) {
		return harness.ErrReported
	}
	if dl := check.DiffRQ(T("REC.old"), T("REC.new")); len(dl) > 0 {
		r.Say("the declared delta is NOTHING AT ALL and the two recordings differ:")
		head(dl, 12, "")
		return harness.ErrReported
	}
	r.Say("the declared delta is NOTHING AT ALL and TWO FULL RECORDINGS ARE BYTE-IDENTICAL -- 102 screen " +
		"cases, every Ex command, every command line, the pty scenarios and the terminal table.  On its own " +
		"that would say little, the corpus never reaching ui_focus_change at all; what answers for this phase " +
		"is the instrumented pair and the focus probes above")
	return nil
}
