package p119

// Whim phase 119, the check -- the core's libc prototype block empties.
// See phase/119/edit.go, and GOALS.md II.4c.
//
// Runs after phase/119/edit.go and the sweep tools/phaserun.sh runs between them, and
// reads nothing from the edit's shell -- only the work tree and the state directory.
// What the edit left there is `old.c`, the source this phase was HANDED, and `old`, that
// source built with SOURCE_DATE_EPOCH=0 and the boundary's own flags.
//
// WHAT IS CLAIMED, in ten parts:
//
// ARITHMETIC  computed FROM THE INPUT and not written here.  Above the boundary every
// mention of `getpid` and `kill` falls in a class this phase rewrites, and
// both end at 0; `mch_get_pid` and `b0_pid` end at 0, the sweep having
// taken the prototype and the field the edit orphaned; `host_raise` ends
// at three, a prototype, a call and a definition.  The core's block of
// ordinary declarations loses exactly two lines, and what remains is
// PRINTED, because an empty block is this phase's whole point.
// THE CLAIM   and it is measured from the CUT, not from the block, because those are
// two different assertions and only the first is the claim.  A bare
// `extern` declaration is INVISIBLE to `-fsyntax-only` -- gcc warns `used
// but never defined` for a `static` function and says nothing about an
// ordinary one -- so what is measured is `nm -u` of an OBJECT of the cut
// alone: the set of names the core needs from outside itself.  Every name
// in it must be DEFINED BELOW THE BOUNDARY in this same file, computed;
// the identical computation on the INPUT finds exactly two that are not,
// `getpid` and `kill`, and that is the control.  The cut also defines no
// external symbol at all.
// VOCABULARY  `zhostonly`, phase 103's structural check, plus the stronger thing
// this boundary can say: above the first `#include` the ONLY words of that
// tool's host vocabulary left are `SIGHUP` and `SIGTERM`, the two the core
// NAMES because it prints them.
// THE FOLD    NOT TAKEN, and asserted as a byte comparison rather than left to be
// believed: `deathtrap()` is identical in and out, and `vim_handle_signal()`
// differs in exactly one line.  See the edit for the survey.
// THE ORDER   the prototype is above the call, the definition below it, below the
// boundary AND inside the host region.  The control is the output with the
// prototype line deleted, which must not compile and must name it.
// LINKAGE     `nm --extern-only --defined-only` is still exactly `main`, with both
// halves of the `static` trap built.
// SYMBOLS     `nm -u` is THE SAME SET, as a `comm` empty in both directions, and
// `getpid` and `kill` are both still in it.  A phase that takes the last
// core mention of two libc names frees NEITHER, because the host calls
// both and a symbol leaves when its last caller leaves the FILE.
// THE BINARY  the same SIZE or not, stated as a measurement, and NOT the same bytes.
// THE RECORD  two full tools/zrecord.sh recordings, `diff -r` empty over 106 records,
// with one control that MOVES all 106 and one that moves NONE, both on the
// line this phase deletes.
// THE PROBES  THE CORPUS CANNOT SEE EITHER HALF OF THIS PHASE, and that is measured
// rather than assumed: an instrumented input counts the b0_pid write in
// every one of the 102 screen cases and the re-raise in NONE of them.  So
// the phase owes probes, and they are a forced deferral driven identically
// into both binaries, with two controls.
// tools/deadfields.py
// tools/phasecheck.sh
// tools/st.sh
// tools/zrecord.sh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim119", Check) }

var (
	z36Probe  = regexp.MustCompile(`PROBE b0=(\d+) vhs=(\d+) raise=(\d+)`)
	z36DefnHd = regexp.MustCompile(`^([A-Za-z_]\w*)\s*\(`)
	z36Swept1 = regexp.MustCompile(`^static [\w *]+mch_get_pid\(.*\);$`)
	z36Swept2 = regexp.MustCompile(`^\s*char_u\s+b0_pid\[\d+\];$`)
	z36WS     = regexp.MustCompile(`[ \t]+`)
)

const z36Instr = `static long probe_b0, probe_vhs, probe_raise;

    static void
probe_num(long v)
{
    char b[24];
    int i = 24;

    if (v == 0)
    {
        b[--i] = '0';
    }
    while (v > 0)
    {
        b[--i] = (char)('0' + v % 10);
        v /= 10;
    }
    write(2, b + i, (usize)(24 - i));
}

    static void
probe_dump(void)
{
    write(2, "PROBE b0=", 9);
    probe_num(probe_b0);
    write(2, " vhs=", 5);
    probe_num(probe_vhs);
    write(2, " raise=", 7);
    probe_num(probe_raise);
    write(2, "\n", 1);
}

`

// z36Body is the heredoc's Body(): a definition's own lines, from its
// return-type line to its closing brace, in this tree's one shape.
func z36Body(lines []string, name string) []string {
	re := regexp.MustCompile(`^` + name + `\s*\(`)
	var h []int
	for i, l := range lines {
		if re.MatchString(l) && i+1 < len(lines) && lines[i+1] == "{" {
			h = append(h, i)
		}
	}
	if len(h) != 1 || h[0] == 0 {
		return nil
	}
	e := h[0]
	for e < len(lines) && lines[e] != "}" {
		e++
	}
	if e >= len(lines) {
		return nil
	}
	return lines[h[0]-1 : e+1]
}

// Whim119 is phase 119's check: the core names no libc function at all.
func Check(w io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: check whim119 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	f := filepath.Join(work, "whim-vim.c")
	oldC := filepath.Join(state, "old.c")
	const TAG = "noclib"
	r := &check.Rep{Tag: TAG, W: w}
	stop := func(format string, a ...any) error {
		r.Say(format, a...)
		return harness.ErrReported
	}
	raw := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
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
	tmp, err := os.MkdirTemp("", "whim119-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	T := func(n string) string { return filepath.Join(tmp, n) }
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflagsS, ldflagsS := "", ""
	if m := check.Z29CFlags.FindStringSubmatch(mk); m != nil {
		cflagsS = m[1]
	}
	if m := check.Z29LDFlags.FindStringSubmatch(mk); m != nil {
		ldflagsS = m[1]
	}
	link := func(src, Out, logPath string) error {
		a := append(append(strings.Fields(cflagsS), strings.Fields(ldflagsS)...), "-o", Out, src)
		c := exec.Command("gcc", a...)
		c.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
		if logPath != "" {
			lf, _ := os.Create(logPath)
			defer lf.Close()
			c.Stderr = lf
		}
		return c.Run()
	}
	var wgAll sync.WaitGroup
	defer wgAll.Wait()
	type job struct {
		wg  sync.WaitGroup
		Err error
	}
	start := func(fn func() error) *job {
		j := &job{}
		j.wg.Add(1)
		wgAll.Add(1)
		go func() { defer wgAll.Done(); defer j.wg.Done(); j.Err = fn() }()
		return j
	}
	jNew := start(func() error { return link(f, T("new"), "") })

	// --- 0. the eleven variants ------------------------------------------------------
	t := check.ReadFile(f)
	oldT := check.ReadFile(oldC)
	const PROTO = "static void host_raise(int sig);"
	if strings.Count(t, PROTO+"\n") != 1 {
		return stop("the output does not hold `%s` exactly once", PROTO)
	}
	c1 := strings.Replace(t, PROTO+"\n", "", 1)
	c2 := strings.Replace(t, PROTO+"\n", PROTO[len("static "):]+"\n", 1)
	m := regexp.MustCompile(`(?m)^    static (void)\n(host_raise\()`).FindStringSubmatchIndex(c2)
	if m == nil {
		return stop("`host_raise` is not defined in the `    static <type>` shape, so " +
			"c3 would not be a control")
	}
	c3 := c2[:m[0]] + "    " + c2[m[2]:m[3]] + "\n" + c2[m[4]:m[5]] + c2[m[1]:]
	one := func(text, s, what string) (string, error) {
		if strings.Count(text, s) != 1 {
			return "", stop("`%s` is not in that source exactly once, so a control built "+
				"from it would not be one", what)
		}
		return s, nil
	}
	WRITE, e := one(oldT, "        long_to_char(mch_get_pid(), b0p->b0_pid);\n", "ml_open()'s write of b0_pid")
	if e != nil {
		return e
	}
	ctl := map[string]string{
		"cml":  strings.Replace(oldT, WRITE, "        return FAIL;\n", 1),
		"cpid": strings.Replace(oldT, WRITE, "        long_to_char(0x5a5a5a5aL, b0p->b0_pid);\n", 1),
	}
	EXIT, e := one(oldT, "    static void\nhost_exit(int r)\n{\n", "host_exit()")
	if e != nil {
		return e
	}
	RERAISE, e := one(oldT, "kill(getpid(), got_signal);", "vim_handle_signal()'s re-raise")
	if e != nil {
		return e
	}
	VHS, e := one(oldT, "vim_handle_signal(int sig)\n{\n", "vim_handle_signal()'s head")
	if e != nil {
		return e
	}
	const DECLS = "static long probe_b0, probe_vhs, probe_raise;\n\n"
	if strings.Contains(oldT, "probe_b0") {
		return stop("the input already says `probe_b0`")
	}
	const mlHead = "    static int\nml_open(buf_T *buf)\n{\n"
	p := strings.Replace(oldT, mlHead, DECLS+mlHead, 1)
	if p == oldT {
		return stop("ml_open() is not defined in this tree's shape")
	}
	p = strings.Replace(p, WRITE, "        probe_b0++;\n"+WRITE, 1)
	p = strings.Replace(p, VHS, VHS+"    probe_vhs++;\n", 1)
	p = strings.Replace(p, RERAISE, "probe_raise++;\n                                 "+RERAISE, 1)
	p = strings.Replace(p, EXIT, strings.Replace(z36Instr, DECLS, "", 1)+EXIT+"    probe_dump();\n", 1)
	ctl["probe"] = p
	const INIT = "    out_flush();\n\n    musl_host_init();\n\n}\n"
	const FORCE = "    out_flush();\n\n    musl_host_init();\n\n" +
		"    (void)vim_handle_signal(SIGTERM);\n    (void)vim_handle_signal(-2);\n\n}\n"
	forced := map[string]string{}
	for _, x := range []struct{ side, Text string }{{"in", oldT}, {"Out", t}} {
		if _, e := one(x.Text, INIT, "mch_init()'s tail"); e != nil {
			return e
		}
		base := strings.Replace(x.Text, INIT, FORCE, 1)
		call := "host_raise(got_signal);"
		if x.side == "in" {
			call = "kill(getpid(), got_signal);"
		}
		if _, e := one(base, call, "the re-raise in the "+x.side+"put"); e != nil {
			return e
		}
		forced["pf"+x.side] = base
		forced["pf"+x.side+"_drop"] = strings.Replace(base, call, "", 1)
		forced["pf"+x.side+"_hup"] = strings.Replace(base, call, strings.Replace(call, "got_signal", "SIGHUP", 1), 1)
	}
	type variant struct{ Name, Text, base string }
	variants := []variant{{"c1", c1, t}, {"c2", c2, t}, {"c3", c3, t}}
	for _, k := range []string{"cml", "cpid", "probe"} {
		variants = append(variants, variant{k, ctl[k], oldT})
	}
	fk := make([]string, 0, len(forced))
	for k := range forced {
		fk = append(fk, k)
	}
	sort.Strings(fk)
	for _, k := range fk {
		b := t
		if strings.Contains(k, "in") {
			b = oldT
		}
		variants = append(variants, variant{k, forced[k], b})
	}
	for _, v := range variants {
		if v.Text == v.base {
			return stop("the variant %s changed nothing, so it would not be a control", v.Name)
		}
		os.WriteFile(T(v.Name+".c"), []byte(v.Text), 0o644)
	}
	r.Say("eleven variants written: c1 the prototype DELETED, c2 `static` off it, c3 " +
		"`static` off it AND the definition; cml the deleted line replaced by `return " +
		"FAIL;` and cpid the same line writing a constant, both on the INPUT; probe, the " +
		"input with a counter on the write, on vim_handle_signal and on the re-raise; and " +
		"six forced-deferral binaries, the input and the output each plain, with the " +
		"re-raise dropped, and with the re-raise given the wrong signal")

	stderrTo := func(c *exec.Cmd, path string) {
		lf, _ := os.Create(path)
		defer lf.Close()
		c.Stderr = lf
		c.Run()
	}
	jC1 := start(func() error {
		stderrTo(exec.Command("gcc", "-O0", "-fno-stack-protector", "-fsyntax-only", T("c1.c")), T("e.c1"))
		return nil
	})
	jC2 := start(func() error {
		stderrTo(exec.Command("gcc", "-O0", "-fno-stack-protector", "-fsyntax-only", T("c2.c")), T("e.c2"))
		return nil
	})
	jC3 := start(func() error {
		stderrTo(exec.Command("gcc", "-c", "-O0", "-fno-stack-protector", "-o", T("c3.o"), T("c3.c")), T("e.c3"))
		return nil
	})
	jobs := map[string]*job{}
	for _, v := range []string{"cml", "cpid", "probe", "pfin", "pfout", "pfin_drop", "pfout_drop", "pfin_hup", "pfout_hup"} {
		v := v
		jobs[v] = start(func() error { return link(T(v+".c"), T(v), T("e."+v)) })
	}
	os.WriteFile(T("canon.c"), []byte(t), 0o644)
	var canonLog []byte
	jCanon := start(func() error {
		var e error
		canonLog, e = exec.Command("sh", "tools/canon.sh", T("canon.c")).CombinedOutput()
		return e
	})

	// --- 1. the source, as arithmetic on the input ----------------------------------
	beforeLines, _ := strconv.Atoi(strings.TrimSpace(beforeRaw))
	NL, OL := strings.Split(t, "\n"), strings.Split(oldT, "\n")
	GO := []string{"int getpid(void);", "int kill(int pid, int sig);"}
	mentions := func(text, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllStringIndex(text, -1))
	}
	halves := func(lines []string) (string, string, int, bool) {
		var d []int
		for i, l := range lines {
			if check.Z35Dir.MatchString(l) {
				d = append(d, i)
			}
		}
		if len(d) != 11 {
			return "", "", 0, false
		}
		for k, i := range d {
			if i != d[0]+k || !check.Z35Include.MatchString(lines[i]) {
				return "", "", 0, false
			}
		}
		return strings.Join(lines[:d[0]], "\n"), strings.Join(lines[d[0]:], "\n"), d[0], true
	}
	ocore, _, obound, okO := halves(OL)
	ncore, nhost, nbound, okN := halves(NL)
	if !okO || !okN {
		which := "output"
		if !okO {
			which = "input"
		}
		return stop("the eleven `#include` directives are not eleven consecutive lines "+
			"in the %s -- the first `#include` IS the boundary and nothing else marks "+
			"it", which)
	}
	for _, x := range []struct {
		Name  string
		nhost int
		why   string
	}{
		{"getpid", 1, "mch_get_pid()'s body and the re-raise; and NOTHING below the boundary said it before"},
		{"kill", 2, "the re-raise; and musl_suspend()'s `kill(0, SIGTSTP)` below the boundary already did"},
	} {
		occ := mentions(ocore, x.Name)
		paren := check.Z35CallShaped(ocore, x.Name)
		if occ != paren || paren < 2 {
			r.Bad("the INPUT has %d mentions of `%s` above the boundary of which %d "+
				"are call-shaped, and this phase needs every one to be the "+
				"declaration or a call, with at least one call -- %s", occ, x.Name, paren, x.why)
			continue
		}
		if n := mentions(ncore, x.Name); n != 0 {
			r.Bad("`%s` still has %d mentions above the boundary, and the phase takes every one", x.Name, n)
		}
		if n := mentions(nhost, x.Name); n != x.nhost {
			r.Bad("`%s` ends at %d mentions below the boundary, expected %d -- %s", x.Name, n, x.nhost, x.why)
		}
	}
	nGetpid := check.Z35CallShaped(ocore, "getpid")
	nKill := check.Z35CallShaped(ocore, "kill")
	for _, x := range []struct {
		Name string
		o, n int
		why  string
	}{
		{"mch_get_pid", 3, 0, "its definition, its forward declaration and its one call site -- the edit takes " +
			"the first, the sweep the second, and the third is the write"},
		{"b0_pid", 2, 0, "its declaration and the ONE write, with no read anywhere: the edit takes the write " +
			"and tools/deadfields.py the field"},
		{"host_raise", 0, 3, "a prototype, the one call site it takes over from the re-raise, and a definition"},
	} {
		if mentions(oldT, x.Name) != x.o || mentions(t, x.Name) != x.n {
			r.Bad("`%s` goes from %d mentions to %d and this phase was written against "+
				"%d -> %d -- %s", x.Name, mentions(oldT, x.Name), mentions(t, x.Name), x.o, x.n, x.why)
		}
	}
	var blockBefore, blockAfter []string
	var seed []int
	for i, l := range OL[:obound] {
		if l == GO[0] {
			seed = append(seed, i)
		}
	}
	if len(seed) != 1 {
		r.Bad("the input does not declare `%s` exactly once above the boundary", GO[0])
	} else {
		lo, hi := seed[0], seed[0]
		for lo > 0 && check.Z35Decl(OL[lo-1]) {
			lo--
		}
		for hi+1 < obound && check.Z35Decl(OL[hi+1]) {
			hi++
		}
		blockBefore = OL[lo : hi+1]
		for _, l := range blockBefore {
			if !check.Contains(GO, l) {
				blockAfter = append(blockAfter, l)
			}
		}
		if len(blockBefore)-len(blockAfter) != 2 {
			r.Bad("the input's block of ordinary declarations does not hold both of "+
				"this phase's lines: %s", strings.Join(blockBefore, " / "))
		}
		var have []string
		for i, l := range NL[:nbound] {
			prev := NL[len(NL)-1]
			if i > 0 {
				prev = NL[i-1]
			}
			if check.Z35Decl(l) && (prev == "" || check.Z35Decl(prev)) {
				have = append(have, l)
			}
		}
		if strings.Join(have, "\x00") != strings.Join(blockAfter, "\x00") {
			hs, bs := strings.Join(have, " / "), strings.Join(blockAfter, " / ")
			if hs == "" {
				hs = "none"
			}
			if bs == "" {
				bs = "none"
			}
			r.Bad("the ordinary declarations above the boundary are %s and the "+
				"input's block minus the two is %s", hs, bs)
		}
	}
	od, nd := z36Body(OL, "deathtrap"), z36Body(NL, "deathtrap")
	if od == nil || nd == nil {
		r.Bad("deathtrap() is not defined exactly once in one of the two files")
	} else if strings.Join(od, "\n") != strings.Join(nd, "\n") {
		r.Bad("deathtrap() is NOT identical in and out, and this phase surveyed the "+
			"`entered` fold and did not take it: %d lines in, %d out", len(od), len(nd))
	}
	ov, nv := z36Body(OL, "vim_handle_signal"), z36Body(NL, "vim_handle_signal")
	if ov == nil || nv == nil || len(ov) != len(nv) {
		r.Bad("vim_handle_signal() is not the same shape in and out")
	} else {
		var diff []int
		for i := range ov {
			if ov[i] != nv[i] {
				diff = append(diff, i)
			}
		}
		if len(diff) != 1 || !strings.Contains(ov[diff[0]], "kill(getpid(), ") || !strings.Contains(nv[diff[0]], "host_raise(") {
			var ds []string
			for k, i := range diff {
				if k >= 3 {
					break
				}
				ds = append(ds, check.PyRepr26(ov[i])+" -> "+check.PyRepr26(nv[i]))
			}
			r.Bad("vim_handle_signal() differs in %d lines and this phase changes "+
				"exactly one, the re-raise: %s", len(diff), strings.Join(ds, " / "))
		}
	}
	// tools/create_cmdidxs.py -- named as a PATH so tools/implhash.sh hashes
	// it into this phase's key.  Do not delete it.
	nOld := len(check.Z35CmdRow.FindAllString(oldT, -1))
	nNew := len(check.Z35CmdRow.FindAllString(t, -1))
	cn, _ := harness.CommandNames(f)
	if nNew != nOld || len(cn) != nOld {
		r.Bad("cmdnames[] is %d rows and the input had %d -- this phase touches no Ex "+
			"command", nNew, nOld)
	}
	if check.Z29RowCount(t) != check.Z29RowCount(oldT) {
		r.Bad("options[] has %d rows and the input had %d -- this phase removes no "+
			"option", check.Z29RowCount(t), check.Z29RowCount(oldT))
	}
	if check.Z27Runs(NL) > 0 {
		r.Bad("there is a run of two blank lines, which canon.sh should have taken")
	}
	gp, hr := z36Body(OL, "mch_get_pid"), z36Body(NL, "host_raise")
	var swept []int
	for i, l := range OL {
		if z36Swept1.MatchString(l) || z36Swept2.MatchString(l) {
			swept = append(swept, i)
		}
	}
	if gp == nil || hr == nil || len(swept) != 2 {
		pb := func(b bool) string {
			if b {
				return "True"
			}
			return "False"
		}
		r.Bad("mch_get_pid() is not defined exactly once in the input (%s), or "+
			"host_raise() in the output (%s), or the two lines the sweep takes are "+
			"not exactly two in the input (%d)", pb(gp != nil), pb(hr != nil), len(swept))
	} else {
		aboveIn := 1
		blk := len(blockBefore) + 1
		if len(blockAfter) > 0 {
			blk = 2
		}
		aboveOut := 1 + (len(gp) + 1) + blk + len(swept)
		belowIn := len(hr) + 1
		want := beforeLines + aboveIn + belowIn - aboveOut
		if len(NL)-1 != want || len(OL)-1 != beforeLines {
			r.Bad("the file is %d lines and the input was %d (%d recorded) -- expected "+
				"%d: %d in above the boundary and %d below, %d out above it",
				len(NL)-1, len(OL)-1, beforeLines, want, aboveIn, belowIn, aboveOut)
		}
		if nbound != obound-(aboveOut-aboveIn) {
			r.Bad("the boundary moved from line %d to line %d and every one of the %d "+
				"lines that go and the %d that arrives above it says it should move "+
				"up by %d", obound+1, nbound+1, aboveOut, aboveIn, aboveOut-aboveIn)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("ABOVE THE BOUNDARY both names go: `getpid` %d -> 0 (its declaration and "+
		"%d calls, one in mch_get_pid() and one in the re-raise) and `kill` %d -> 0 (its "+
		"declaration and %d call, the re-raise).  BELOW IT `getpid` 0 -> 1 and `kill` 1 -> "+
		"2, both in host_raise(); so ONE of the two is MOVED and the other is AVOIDED "+
		"outright -- `mch_get_pid` %d -> 0 and `b0_pid` %d -> 0, a write-only field and the "+
		"function that fed it.  `host_raise` 0 -> 3",
		nGetpid, nGetpid-1, nKill, nKill-1, mentions(oldT, "mch_get_pid"), mentions(oldT, "b0_pid"))
	var tail string
	if len(blockAfter) > 0 {
		var fn []string
		for _, l := range blockAfter {
			fn = append(fn, check.Z35FnName.ReplaceAllString(l, "$1"))
		}
		tail = strings.Join(fn, " ") + " remain, and each belongs to a phase of its own"
	} else {
		tail = "IT IS EMPTY.  The core names no libc function at all"
	}
	r.Cont("THE CORE'S BLOCK OF ORDINARY DECLARATIONS -- the libc it names, the one "+
		"run above the boundary that is not `static` -- is %d lines and was %d: %s",
		len(blockAfter), len(blockBefore), tail)
	r.Cont("THE FOLD WAS SURVEYED AND NOT TAKEN: deathtrap() is byte-identical in and "+
		"out, %d lines either side, and vim_handle_signal() differs in exactly one line.  "+
		"`entered` has three reachable values since phase 100 and every one of them is read "+
		"-- 0 by the entry guard, 1 and 2 by BOTH the double-signal arm and `v_dying = "+
		"entered`, which getout() tests -- so there is no fold to take and the phase says "+
		"so here rather than leaving it to be believed", len(od))
	r.Cont("%d -> %d lines, %d fewer; cmdnames[] %d and options[] %d unmoved; the "+
		"eleven #includes still eleven consecutive lines, at %d where they were %d",
		beforeLines, len(NL)-1, beforeLines-(len(NL)-1), nNew, check.Z29RowCount(t), nbound+1, obound+1)

	// --- 2. THE CLAIM: the cut, and what the core needs from outside itself ------------
	type cut struct {
		lines, errs, warns int
		names              []string
		log                string
		objOK              bool
	}
	editorcut := func(src, dst string) (cut, error) {
		lines := check.Z28Cut(check.ReadFile(src))
		text := ""
		for _, l := range lines {
			text += l + "\n"
		}
		os.WriteFile(dst, []byte(text), 0o644)
		for _, l := range lines {
			if check.Z30Dir.MatchString(l) {
				return cut{}, stop("the cut of %s holds a directive, so it found the wrong line", src)
			}
		}
		if len(lines) <= 70000 {
			return cut{}, stop("the cut of %s is %d lines, and whim.mk's floor is 70,000 -- a cut that found "+
				"line 1 would be empty and every check below would pass on nothing", src, len(lines))
		}
		os.Remove(dst + ".o")
		c := exec.Command("gcc", "-c", "-O0", "-fno-stack-protector", "-o", dst+".o", dst)
		var eb strings.Builder
		c.Stderr = &eb
		c.Run()
		var cu cut
		cu.lines, cu.log = len(lines), eb.String()
		for _, l := range strings.Split(cu.log, "\n") {
			if strings.Contains(l, "error:") {
				cu.errs++
			}
			if strings.Contains(l, "warning:") {
				cu.warns++
			}
			if m := check.Z35Warn.FindStringSubmatch(l); m != nil {
				cu.names = append(cu.names, m[1])
			}
		}
		sort.Strings(cu.names)
		_, e := os.Stat(dst + ".o")
		cu.objOK = e == nil
		return cu, nil
	}
	co, e := editorcut(oldC, T("cut-old.c"))
	if e != nil {
		return e
	}
	cnw, e := editorcut(f, T("cut-new.c"))
	if e != nil {
		return e
	}
	for _, x := range []struct {
		W, path string
		c       cut
	}{{"old", T("cut-old.c"), co}, {"new", T("cut-new.c"), cnw}} {
		if x.c.errs != 0 || !x.c.objOK {
			r.Say("the %s cut does not compile: %d errors", x.W, x.c.errs)
			var el []string
			for _, l := range strings.Split(x.c.log, "\n") {
				if strings.Contains(l, "error:") {
					el = append(el, l)
				}
			}
			head(el, 3, "               ")
			return harness.ErrReported
		}
		if x.c.warns != len(x.c.names) {
			r.Say("the %s cut has a warning that is not a 'used but never defined':", x.W)
			var wl []string
			for _, l := range strings.Split(x.c.log, "\n") {
				if strings.Contains(l, "warning:") && !strings.Contains(l, "used but never defined") {
					wl = append(wl, l)
				}
			}
			head(wl, 3, "               ")
			return harness.ErrReported
		}
		if Out, _ := exec.Command("nm", "--extern-only", "--defined-only", x.path+".o").Output(); len(Out) > 0 {
			r.Say("the %s cut DEFINES an external symbol, and the core defines none -- main is the host's:", x.W)
			head(strings.Split(strings.TrimRight(string(Out), "\n"), "\n"), 3, "               ")
			return harness.ErrReported
		}
	}
	cutBytes, _ := os.ReadFile(T("cut-new.c"))
	fb, _ := os.ReadFile(f)
	if len(fb) < len(cutBytes) || string(fb[:len(cutBytes)]) != string(cutBytes) {
		return stop("the cut is not a byte prefix of whim-vim.c")
	}
	arrived := check.Z31Words(check.Comm23(cnw.names, co.names))
	left := check.Z31Words(check.Comm23(co.names, cnw.names))
	if arrived != "host_raise " || left != "" {
		a, l := arrived, left
		if a == "" {
			a = "nothing"
		}
		if l == "" {
			l = "nothing"
		}
		r.Say("THE CUT'S BOUNDARY SET DID NOT MOVE AS THIS PHASE CLAIMS.")
		raw("               arrived: %s", a)
		raw("               left:    %s", l)
		return harness.ErrReported
	}
	r.Say("THE CUT -- `make editor.c`'s own rule, and a byte prefix of the file -- is %d lines in and %d out, 0 "+
		"errors either side, and its warning set, which is the core -> host interface, goes from %d names to %d: "+
		"host_raise ARRIVES and nothing leaves.  Neither cut defines an external symbol -- `main` is below the "+
		"boundary and is the host's", co.lines, cnw.lines, len(co.names), len(cnw.names))
	below := func(path string) map[string]bool {
		L := strings.Split(check.ReadFile(path), "\n")
		b := -1
		for i, l := range L {
			if check.Z35Dir.MatchString(l) {
				b = i
				break
			}
		}
		Out := map[string]bool{}
		if b < 0 {
			return Out
		}
		for i := b; i < len(L)-1; i++ {
			if m := z36DefnHd.FindStringSubmatch(L[i]); m != nil && L[i+1] == "{" {
				Out[m[1]] = true
			}
		}
		return Out
	}
	needs := func(obj string) ([]string, error) {
		Out, err := exec.Command("nm", "-u", obj).Output()
		if err != nil {
			return nil, stop("nm -u failed on %s", obj)
		}
		var u []string
		for _, l := range strings.Split(string(Out), "\n") {
			if fs := strings.Fields(l); len(fs) > 0 {
				u = append(u, fs[len(fs)-1])
			}
		}
		sort.Strings(u)
		return u, nil
	}
	type needRes struct{ u, outside []string }
	res := map[string]needRes{}
	for _, x := range []struct{ side, src, obj string }{
		{"new", f, T("cut-new.c.o")}, {"old", oldC, T("cut-old.c.o")},
	} {
		u, e := needs(x.obj)
		if e != nil {
			return e
		}
		if len(u) < 5 {
			return stop("the %s cut needs %d names from outside itself, and an editor "+
				"core that asked its host for almost nothing would mean nm read the "+
				"wrong object", x.side, len(u))
		}
		d := below(x.src)
		var outside []string
		for _, n := range u {
			if !d[n] {
				outside = append(outside, n)
			}
		}
		res[x.side] = needRes{u, outside}
	}
	if strings.Join(res["old"].outside, " ") != "getpid kill" {
		got := "nothing"
		if len(res["old"].outside) > 0 {
			var q []string
			for _, x := range res["old"].outside {
				q = append(q, "'"+x+"'")
			}
			got = "[" + strings.Join(q, ", ") + "]"
		}
		return stop("THE CONTROL DID NOT SHOW.  The identical computation on the INPUT "+
			"must find exactly `getpid` and `kill` outside this file; it finds %s -- so "+
			"the emptiness measured on the output would be two numbers agreeing rather "+
			"than a phase", got)
	}
	if len(res["new"].outside) > 0 {
		return stop("THE CORE STILL NEEDS %s FROM OUTSIDE THIS FILE, and the claim is "+
			"not writable: every name the cut needs must be DEFINED below the boundary",
			strings.Join(res["new"].outside, " "))
	}
	r.Say("THE CLAIM, AND IT IS MEASURED FROM THE CUT AND NOT FROM THE BLOCK.  `nm "+
		"-u` on an object of the cut ALONE -- the core, and nothing of the host -- is the "+
		"set of names the core needs from outside itself: %d names in and %d out.  EVERY "+
		"ONE of the %d is defined BELOW the boundary in this same file, computed from the "+
		"text and not listed here; the input's %d are not, and they are `%s` -- so the "+
		"emptiness is a phase and not two numbers agreeing.",
		len(res["old"].u), len(res["new"].u), len(res["new"].u), len(res["old"].outside),
		strings.Join(res["old"].outside, "` and `"))
	r.Cont("IT IS EMPTY.  The core names no libc function at all: above the first "+
		"`#include` there is not one declaration that is not `static`, and every outward "+
		"call the editor makes is a `musl_` or a `host_` defined below that line.  %s",
		strings.Join(res["new"].u, " "))

	// --- 3. the vocabulary the core has left -------------------------------------------
	zh := exec.Command("sh", "tools/st.sh", "zhostonly", f)
	zh.Stdout, zh.Stderr = w, w
	if err := zh.Run(); err != nil {
		return harness.ErrReported
	}
	// tools/zhostonly.py -- named as a PATH so tools/implhash.sh hashes it
	// into this phase's key.  Do not delete it.
	vocab := map[string]map[string][]int{}
	for _, x := range []struct{ side, path string }{{"new", f}, {"old", oldC}} {
		L := strings.Split(check.ReadFile(x.path), "\n")
		b := -1
		for i, l := range L {
			if check.Z35Dir.MatchString(l) {
				b = i
				break
			}
		}
		hits := map[string][]int{}
		for i := 0; i < b; i++ {
			if strings.HasPrefix(L[i], "#") {
				continue
			}
			for _, mm := range harness.HostVocab().FindAllString(harness.StripStrings(L[i]), -1) {
				k := z36WS.ReplaceAllString(mm, " ")
				hits[k] = append(hits[k], i+1)
			}
		}
		vocab[x.side] = hits
	}
	keys := func(m map[string][]int) []string {
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return ks
	}
	if ko := keys(vocab["old"]); strings.Join(ko, " ") != "SIGHUP SIGTERM getpid kill" {
		s := strings.Join(ko, " ")
		if s == "" {
			s = "nothing"
		}
		return stop("the INPUT's core says %s of zhostonly's vocabulary, and "+
			"this phase was written against SIGHUP SIGTERM getpid kill", s)
	}
	if kn := keys(vocab["new"]); strings.Join(kn, " ") != "SIGHUP SIGTERM" {
		return stop("the core still says %s of the host's vocabulary above the "+
			"boundary", strings.Join(kn, " "))
	}
	r.Say("AND THE CORE'S WHOLE REMAINING VOCABULARY OF THE HOST IS TWO WORDS.  "+
		"Above the first `#include`, zhostonly's host vocabulary now matches only "+
		"`SIGHUP` (%d times) and `SIGTERM` (%d) -- the two deadly signals the editor NAMES "+
		"because it PRINTS them, in signal_info[], in deathtrap's own test and in the "+
		"core's `enum`.  The input said `getpid` %d times and `kill` %d as well, and those "+
		"were the only mentions of any host word in the core that were not a message",
		len(vocab["new"]["SIGHUP"]), len(vocab["new"]["SIGTERM"]),
		len(vocab["old"]["getpid"]), len(vocab["old"]["kill"]))

	// --- 4. declaration before use --------------------------------------------------------
	jC1.wg.Wait()
	errC1 := check.ReadFile(T("e.c1"))
	bound := -1
	for i, l := range NL {
		if check.Z35Dir.MatchString(l) {
			bound = i
			break
		}
	}
	var hb, he []int
	for i, l := range NL {
		if strings.HasPrefix(l, "static volatile sig_atomic_t host_winch_pending") {
			hb = append(hb, i)
		}
		if strings.HasPrefix(l, "musl_suspend(") {
			he = append(he, i)
		}
	}
	if len(hb) != 1 || len(he) != 1 {
		return stop("the host region does not begin and end exactly once")
	}
	end := he[0]
	for end < len(NL) && NL[end] != "}" {
		end++
	}
	name := "host_raise"
	reP := regexp.MustCompile(`^static [\w *]*` + name + `\(.*\);$`)
	var proto, defn, uses []int
	for i, l := range NL {
		if reP.MatchString(l) {
			proto = append(proto, i)
		}
	}
	for i, l := range NL {
		if strings.HasPrefix(l, name+"(") && i+1 < len(NL) && NL[i+1] == "{" {
			defn = append(defn, i)
		}
	}
	for i, l := range NL {
		if check.Z35CallShaped(l, name) > 0 && !check.ContainsInt(proto, i) && !check.ContainsInt(defn, i) {
			uses = append(uses, i)
		}
	}
	if len(proto) != 1 || len(defn) != 1 || len(uses) != 1 {
		return stop("`%s` has %d prototypes, %d definitions and %d call sites in the "+
			"output", name, len(proto), len(defn), len(uses))
	}
	if !(proto[0] < uses[0] && uses[0] < defn[0] && bound < defn[0] && hb[0] <= defn[0] && defn[0] <= end) {
		return stop("`%s`: prototype at %d, call at %d, definition at %d, boundary at "+
			"%d, host region %d-%d -- the prototype must be ABOVE the call, the "+
			"definition BELOW it, below the boundary AND inside the host region, "+
			"because its body says `kill` and `getpid` and zhostonly reads "+
			"that region and no other", name, proto[0]+1, uses[0]+1, defn[0]+1, bound+1, hb[0]+1, end+1)
	}
	if !regexp.MustCompile(`\berror\b`).MatchString(errC1) || !strings.Contains(errC1, name) {
		return stop("THE CONTROL c1 DID NOT SHOW: with the prototype line DELETED the "+
			"file must fail to compile and must name `%s`.  Without that the line "+
			"numbers above are a statement about a file and not about a program", name)
	}
	r.Say("`%s`: prototype line %d, one call site at line %d, definition line %d, "+
		"which is below the boundary at %d and inside the %d-line host region.  AND THE "+
		"DECLARATION IS LOAD-BEARING: the same output with that one line deleted gives %d "+
		"errors naming it", name, proto[0]+1, uses[0]+1, defn[0]+1, bound+1, end+1-hb[0],
		strings.Count(errC1, "error:"))

	// --- 5. linkage -------------------------------------------------------------------------
	jC2.wg.Wait()
	jC3.wg.Wait()
	if !strings.Contains(check.ReadFile(T("e.c2")), "static declaration of 'host_raise' follows non-static declaration") {
		r.Say("THE CONTROL c2 DID NOT SHOW.  With `static` off the PROTOTYPE and")
		raw("               left on the definition, gcc must refuse:")
		head(fileLines(T("e.c2")), 4, "               ")
		return harness.ErrReported
	}
	if _, e := os.Stat(T("c3.o")); check.SizeOf(T("e.c3")) > 0 || e != nil {
		r.Say("the control c3 did not build, and the point of it is that it DOES:")
		head(fileLines(T("e.c3")), 4, "               ")
		return harness.ErrReported
	}
	extOut, _ := exec.Command("nm", "--extern-only", "--defined-only", T("c3.o")).Output()
	var c3ext []string
	for _, l := range strings.Split(strings.TrimRight(string(extOut), "\n"), "\n") {
		fs := strings.Fields(l)
		if len(fs) > 0 {
			c3ext = append(c3ext, fs[len(fs)-1])
		} else {
			c3ext = append(c3ext, "")
		}
	}
	sort.Strings(c3ext)
	c3s := check.Z31Words(c3ext)
	if c3s != "host_raise main " {
		r.Say("THE CONTROL c3 DID NOT SHOW.  With `static` off the prototype AND")
		raw("               the definition the object must define host_raise and main; it")
		raw("               defines: %s", c3s)
		return harness.ErrReported
	}
	r.Say("THE `static` TRAP, BOTH HALVES, MEASURED ON THIS PHASE'S OWN OUTPUT: with the keyword off the PROTOTYPE "+
		"gcc REFUSES -- \"static declaration of 'host_raise' follows non-static declaration\" -- and with it off "+
		"the prototype AND the definition the build is SILENT and the object defines %s.  The second is the "+
		"mistake this phase could have made without anything else noticing, and tools/phasecheck.sh below is "+
		"what catches it", c3s)

	// --- 6. the libc surface ---------------------------------------------------------------
	os.WriteFile(T("before.u"), []byte(check.ReadFile(filepath.Join(state, "symbols", "undefined"))), 0o644)
	pc := exec.Command("sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols"))
	pc.Stdout, pc.Stderr = w, w
	if err := pc.Run(); err != nil {
		return harness.ErrReported
	}
	lastU := ".cache/symbols/last/undefined"
	if !check.Z30Same(T("before.u"), lastU) {
		bu, lu := fileLines(T("before.u")), fileLines(lastU)
		r.Say("the libc surface moved, and NEITHER HALF OF THIS PHASE CAN MOVE IT --")
		raw("               host_raise() calls kill() and getpid() where vim_handle_signal() did,")
		raw("               and mch_get_pid()'s getpid() was one of two:")
		raw("               gone: %s", check.Z31Words(check.Comm23(bu, lu)))
		raw("               came: %s", check.Z31Words(check.Comm23(lu, bu)))
		return harness.ErrReported
	}
	for _, s := range []string{"getpid", "kill"} {
		if !check.Contains(fileLines(lastU), s) {
			r.Say("`%s` is no longer an undefined symbol, and it must still be one:", s)
			raw("               host_raise() calls both, and musl_suspend() calls kill as well.")
			return harness.ErrReported
		}
	}
	r.Say("symbols %s -> %s, and the set is IDENTICAL as a cmp -- nothing left and nothing arrived.  `getpid` and "+
		"`kill` are both still there, which is the point: one of them MOVED into the host and the other was "+
		"AVOIDED, and the avoided one had a second call site -- the re-raise -- that moved rather than going.  "+
		"An undefined symbol leaves only when its last caller leaves the FILE.  main is still the only external "+
		"symbol", strings.TrimRight(check.ReadFile(".cache/symbols/last/before"), "\n"),
		strings.TrimRight(check.ReadFile(".cache/symbols/last/after"), "\n"))

	// --- 7. the binary, and canon ------------------------------------------------------------
	jCanon.wg.Wait()
	if !check.Z30Same(T("canon.c"), f) {
		r.Say("tools/canon.sh CHANGED THE OUTPUT, and it must be a no-op:")
		head(check.Z30Diff(f, T("canon.c")), 6, "               ")
		return harness.ErrReported
	}
	canonWord := ""
	for _, l := range strings.Split(string(canonLog), "\n") {
		if check.Z35CanonLn.MatchString(l) {
			canonWord = check.Z35CanonLn.ReplaceAllString(l, "")
			break
		}
	}
	r.Say("tools/canon.sh is a NO-OP on the output (%s)", canonWord)
	_ = exec.Command("make", "-C", work, "clean").Run()
	bin := filepath.Join(work, "whim-vim")
	if _, e := os.Stat(bin); e == nil {
		(&check.Rep{Tag: "build", W: w}).Say("the clean did not remove whim-vim, so a 'rebuild' below could be no rebuild at all")
		return harness.ErrReported
	}
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes", beforeRaw, check.CountLines(fb), check.SizeOf(bin))
	jNew.wg.Wait()
	if jNew.Err != nil {
		return stop("the reproducible build of the output failed")
	}
	oldBinP := filepath.Join(state, "old")
	oldSize, newSize := check.SizeOf(oldBinP), check.SizeOf(T("new"))
	if newSize < 500000 || oldSize < 500000 {
		return stop("one of the two binaries is %d / %d bytes, which is not an editor", oldSize, newSize)
	}
	if newSize != check.SizeOf(bin) {
		return stop("the reproducible build is %d bytes and make produced %d: the two differ by more than a "+
			"timestamp, so nothing below would be about this boundary", newSize, check.SizeOf(bin))
	}
	if check.Z30Same(oldBinP, T("new")) {
		r.Say("THE BINARY IS BYTE-IDENTICAL, and it must not be: a statement goes,")
		raw("               a function goes, a call site becomes a call into another function")
		raw("               of this file and a definition arrives.")
		return harness.ErrReported
	}
	ob, _ := os.ReadFile(oldBinP)
	nbb, _ := os.ReadFile(T("new"))
	nDiff := 0
	for k := 0; k < len(ob) && k < len(nbb); k++ {
		if ob[k] != nbb[k] {
			nDiff++
		}
	}
	r.Say("the binary is %d bytes in and %d out, %d of them differing.  Both are MEASUREMENTS and neither is "+
		"aimed for: a function leaves, a definition arrives, and at -O0 a call to a static function in this file "+
		"is not the instruction stream of a call to a libc symbol.  So this phase cannot use tier 1 of "+
		"CLAUDE.md's verification table and does not pretend to; the evidence is the recording and the probes "+
		"below", oldSize, newSize, nDiff)

	// --- 8. THE EVIDENCE: two recordings, and two controls on the deleted line -------------
	jobs["cml"].wg.Wait()
	if jobs["cml"].Err != nil {
		r.Say("the control cml did not build:")
		head(fileLines(T("e.cml")), 5, "               ")
		return harness.ErrReported
	}
	jobs["cpid"].wg.Wait()
	if jobs["cpid"].Err != nil {
		r.Say("the control cpid did not build:")
		head(fileLines(T("e.cpid")), 5, "               ")
		return harness.ErrReported
	}
	oldBin, _ := filepath.Abs(oldBinP)
	var wgR sync.WaitGroup
	recErr := make([]error, 4)
	for k, x := range []struct{ Name, bin, src string }{
		{"old", oldBin, oldC}, {"new", T("new"), f}, {"cml", T("cml"), T("cml.c")}, {"cpid", T("cpid"), T("cpid.c")},
	} {
		k, x := k, x
		wgR.Add(1)
		go func() {
			defer wgR.Done()
			recErr[k] = check.RecCmd("sh", "tools/zrecord.sh", x.bin, x.src, T("REC-"+x.Name))
		}()
	}
	wgR.Wait()
	if check.RecReport(w, recErr...) {
		return harness.ErrReported
	}
	base := check.WalkFiles(T("REC-old"))
	if len(base) < 100 {
		return stop("a recording holds %d records, and a zero recording is 106 -- 102 "+
			"screen cases and four sweeps.  A comparison of two things nothing wrote "+
			"passes", len(base))
	}
	movedRec := func(which string) ([]string, error) {
		d := T("REC-" + which)
		if strings.Join(check.WalkFiles(d), "\x00") != strings.Join(base, "\x00") {
			return nil, stop("the recording of %s holds different records from the input's", which)
		}
		var mv []string
		for _, n := range base {
			if !check.Z30Same(filepath.Join(T("REC-old"), n), filepath.Join(d, n)) {
				mv = append(mv, n)
			}
		}
		return mv, nil
	}
	same, e := movedRec("new")
	if e != nil {
		return e
	}
	if len(same) > 0 {
		show := same
		if len(show) > 8 {
			show = show[:8]
		}
		r.Say("THE RECORDING MOVED, in %d of %d records: %s", len(same), len(base), strings.Join(show, " "))
		r.Cont("This phase declares NOTHING.  A write nothing reads goes, and one " +
			"libc call becomes a call into the host that makes the same libc call with " +
			"the same signal.")
		return harness.ErrReported
	}
	mCml, e := movedRec("cml")
	if e != nil {
		return e
	}
	mCpid, e := movedRec("cpid")
	if e != nil {
		return e
	}
	if len(mCml) != len(base) {
		return stop("THE CONTROL cml DID NOT SHOW: with the deleted line REPLACED by "+
			"`return FAIL;` -- the same line, in the same place -- %d of %d records "+
			"move and every one must.  If the corpus never executed that line, nothing "+
			"below would be evidence", len(mCml), len(base))
	}
	if len(mCpid) > 0 {
		return stop("a control that was MEASURED to move nothing moved something: cpid %d "+
			"of %d.  It is reported here as a finding and not hidden", len(mCpid), len(base))
	}
	r.Say("THE RECORDING IS BYTE-IDENTICAL, all %d records -- 102 screen cases, "+
		"every Ex command typed at `:`, every command line the parser may see, the four "+
		"pty scenarios and the terminal table", len(base))
	r.Cont("AND THE TWO CONTROLS ARE THE PHASE ITSELF, on the ONE line it deletes:")
	r.Cont("  the same line replaced by `return FAIL;` moves %d of %d records.  So "+
		"ml_open() runs and that statement is executed, and the empty diff above is not "+
		"the corpus missing the code", len(mCml), len(base))
	r.Cont("  the same line writing a CONSTANT instead of the pid moves %d of %d.  "+
		"That is `b0_pid is write-only` measured rather than argued: nothing in any build "+
		"of whim-vim reads block zero back, because the swap file it belonged to is a disk "+
		"format this editor has not had since the filesystem phases.  A control that moves "+
		"nothing is REPORTED here and not quietly dropped", len(mCpid), len(base))

	// --- 9. THE PROBES --------------------------------------------------------------------
	jobs["probe"].wg.Wait()
	if jobs["probe"].Err != nil {
		r.Say("the instrumented build failed:")
		head(fileLines(T("e.probe")), 5, "               ")
		return harness.ErrReported
	}
	pf := []string{"pfin", "pfout", "pfin_drop", "pfout_drop", "pfin_hup", "pfout_hup"}
	for _, v := range pf {
		jobs[v].wg.Wait()
		if jobs[v].Err != nil {
			r.Say("the forced binary %s did not build:", v)
			head(fileLines(T("e."+v)), 5, "               ")
			return harness.ErrReported
		}
	}
	var wgS sync.WaitGroup
	var scErr error
	wgS.Add(1)
	go func() {
		defer wgS.Done()
		scErr = exec.Command("sh", "tools/st.sh", "zcases", T("probe"), T("SC-probe")).Run()
	}()
	os.WriteFile(T("keys"), []byte(":q!\r"), 0o644)
	type sess struct {
		Out, Err []byte
		Rc       int
	}
	sessions := map[string]sess{}
	for _, v := range pf {
		in, _ := os.Open(T("keys"))
		c := exec.Command("./" + v)
		c.Dir = tmp
		var env []string
		for _, kv := range os.Environ() {
			k := kv[:strings.IndexByte(kv+"=", '=')]
			switch k {
			case "VIMINIT", "EXINIT", "HOME", "VIM", "VIMRUNTIME", "XDG_CONFIG_HOME", "TERM":
				continue
			}
			env = append(env, kv)
		}
		c.Env = append(env, "HOME=", "VIM=", "VIMRUNTIME=", "XDG_CONFIG_HOME=", "TERM=xterm")
		var ob, eb strings.Builder
		c.Stdin, c.Stdout, c.Stderr = in, &ob, &eb
		err := c.Run()
		in.Close()
		sessions[v] = sess{[]byte(ob.String()), []byte(eb.String()), check.Z36ShellRC(err, c.ProcessState)}
	}
	wgS.Wait()
	if scErr != nil {
		return stop("the screen corpus failed on the instrumented build")
	}
	ents, _ := os.ReadDir(T("SC-probe"))
	var sbase []string
	for _, en := range ents {
		sbase = append(sbase, en.Name())
	}
	sort.Strings(sbase)
	if len(sbase) != 102 {
		return stop("the screen corpus is %d cases and it is 102", len(sbase))
	}
	tot := map[string]int{}
	cases := map[string]int{}
	seen := 0
	fields := []string{"b0", "vhs", "raise"}
	for _, n := range sbase {
		mm := z36Probe.FindStringSubmatch(check.ReadFile(filepath.Join(T("SC-probe"), n)))
		if mm == nil {
			continue
		}
		seen++
		for k, fld := range fields {
			v, _ := strconv.Atoi(mm[k+1])
			tot[fld] += v
			if v != 0 {
				cases[fld]++
			}
		}
	}
	if seen != 102 {
		return stop("the instrumented build marked %d of 102 cases, and it must mark "+
			"every one: the counter is printed from host_exit(), which every case "+
			"reaches", seen)
	}
	if cases["b0"] != 102 || tot["b0"] < 102 {
		return stop("the write this phase deletes runs in %d of 102 cases (%d times in "+
			"all), and ml_open() is on the path of every buffer the editor opens -- a "+
			"counter that small is not on the path", cases["b0"], tot["b0"])
	}
	if tot["raise"] > 0 {
		return stop("the re-raise fired %d times in the corpus, and this phase's "+
			"probes were written because it fires NONE: no recorded case sends the "+
			"editor a deadly signal, so `got_signal` is never set and the deferral "+
			"never has anything to re-raise", tot["raise"])
	}
	r.Say("THE CORPUS CANNOT SEE EITHER HALF OF THIS PHASE, AND THAT IS MEASURED, "+
		"and the two halves are invisible for OPPOSITE reasons.  An instrumented build of "+
		"the INPUT, over all 102 screen cases and marking every one of them: the b0_pid "+
		"write runs in %d of 102 cases, %d times in all -- it is on the path of every "+
		"buffer the editor opens, and the recording still does not move, because nothing "+
		"reads the field.  The RE-RAISE fires %d times, in %d cases: nothing in the corpus "+
		"sends the editor a deadly signal, so `got_signal` is never set and the deferral "+
		"has nothing to re-raise.  vim_handle_signal() itself is entered only %d times in "+
		"%d cases -- REPORTED and not pinned, because this phase cannot move it: "+
		"ui_inchar() calls it only around a wait longer than 100 ms, and a corpus whose "+
		"stdin is a file of keystrokes almost never waits",
		cases["b0"], tot["b0"], tot["raise"], cases["raise"], tot["vhs"], cases["vhs"])
	for _, x := range []struct{ kind, What string }{
		{"", "the forced deferral"}, {"_drop", "the re-raise DELETED"}, {"_hup", "the re-raise given SIGHUP"},
	} {
		i, o := sessions["pfin"+x.kind], sessions["pfout"+x.kind]
		if string(i.Out) != string(o.Out) || string(i.Err) != string(o.Err) || i.Rc != o.Rc {
			return stop("%s does not behave the same on the two binaries: in %d bytes "+
				"out / %d err / rc %d, out %d / %d / %d",
				x.What, len(i.Out), len(i.Err), i.Rc, len(o.Out), len(o.Err), o.Rc)
		}
	}
	probe, drop, hup := sessions["pfin"], sessions["pfin_drop"], sessions["pfin_hup"]
	if !strings.Contains(string(probe.Out), "Vim: Caught deadly signal TERM") || !strings.Contains(string(probe.Out), "Vim: Finished.") {
		tail := probe.Out
		if len(tail) > 90 {
			tail = tail[len(tail)-90:]
		}
		return stop("the forced deferral did not reach deathtrap(): the screen does not "+
			"carry `Vim: Caught deadly signal TERM`.  %s", check.Z30BytesRepr(tail))
	}
	if probe.Rc != 1 {
		return stop("the forced deferral exits %d and preserve_exit() ends in getout(1)", probe.Rc)
	}
	if strings.Contains(string(drop.Out), "Caught deadly signal") || string(drop.Out) == string(probe.Out) {
		return stop("THE CONTROL `drop` DID NOT SHOW: with the re-raise deleted the " +
			"deferred signal must simply be lost and the editor must NOT die of it -- " +
			"so the probe above would not be reaching the re-raise at all")
	}
	if !strings.Contains(string(hup.Out), "Vim: Caught deadly signal HUP") || string(hup.Out) == string(probe.Out) {
		return stop("THE CONTROL `hup` DID NOT SHOW: with the re-raise given SIGHUP " +
			"instead of the signal that was deferred the editor must report HUP -- so " +
			"the probe above would not be carrying the ARGUMENT across")
	}
	r.Say("THE PROBE THIS PHASE THEREFORE OWES, and it is driven identically into "+
		"BOTH binaries: two statements appended to mch_init(), `vim_handle_signal(SIGTERM)` "+
		"with `blocked` still TRUE and then `vim_handle_signal(-2)`, which is exactly the "+
		"deferral the corpus never reaches.  The input and the output agree in EVERY byte "+
		"of stdout (%d), stderr (%d) and status (%d), and the screen carries `Vim: Caught "+
		"deadly signal TERM` and `Vim: Finished.` -- so `host_raise(got_signal)` raises "+
		"what `kill(getpid(), got_signal)` raised, on the same process, at the same moment",
		len(probe.Out), len(probe.Err), probe.Rc)
	r.Cont("AND IT CAN FAIL, TWICE, identically on both binaries:")
	r.Cont("  the re-raise DELETED -- the deferred signal is lost, the editor does "+
		"NOT die of it and runs on to end of input: %d bytes against %d.  So the probe "+
		"really does go through the line this phase rewrites", len(drop.Out), len(probe.Out))
	r.Cont("  the re-raise given SIGHUP instead of the signal that was deferred: the "+
		"screen says `Caught deadly signal HUP`, %d bytes against %d.  So the ARGUMENT "+
		"crosses the boundary and not merely the call", len(hup.Out), len(probe.Out))
	return nil
}
