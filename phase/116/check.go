package p116

// Whim phase 116 -- the terminal table is asked with `+set term={name}`.
// See GOAL.md, GOALS.md II.4c.
//
// NO SOURCE CHANGE AT ALL: q116's whim-vim.c is its input's, byte for byte, and this
// phase asserts it first and last.  What changes is one fifth of how every later
// phase is measured.  This is phase 86's shape exactly -- the other phase that
// changes no source and replaces an instrument -- and it is here for the same reason:
// a harness that cannot see a phase must be fixed BEFORE the phase, never after.
//
// WHAT WAS WRONG WITH THE OLD QUESTION.  ``ztermcheck`` put the name it was
// asking about in `$TERM`, which is `termcheck`'s question and whim's.  But
// phase 19 removed the `getenv("TERM")` from `termcapinit()` -- "the terminal is
// what the build says" -- and left a compiled `"xterm-256color"` in its place.  So
// every row of `.reference/core-baselines/ref-term.txt` read
//
// TERM='vt100'              -> term=xterm-256color t_Co=256
//
// and the table was as many ways of recording that the environment does nothing.
// MEASURED, and this is the finding that makes the phase rather than the argument:
// a prototype that DELETED eight of the ten built-in terminal names and three of the
// nine capability tables -- 118 lines of terminal description -- passed
// `tools/zcompare.py` against the real baselines DECLARING NOTHING AT ALL.  A phase
// is allowed to declare nothing only when the instrument could have seen it; here it
// could not.
//
// WHAT THE NEW QUESTION IS.  `+set term={name}`, which reaches `did_set_term()`
// rather than `termcapinit()`'s compiled default, and which is `+{command}` -- the
// one facility GOALS.md II decision 8 promises to survive every phase.  It is NOT
// `-T {term}`: measured, a `-T` harness records nothing but `(none)` against a binary
// with no `-T`, which is precisely the failure the tool's own docstring exists to
// prevent, and `-T` is being abandoned.  `termcheck` is imported and
// untouched -- it is named by tools/whimdelta.sh and tools/verify.sh and its bytes
// are in every whim stage's key (GOALS.md core rule 9).
//
// WHY RE-RECORDING THE BASELINES IS LEGITIMATE, which is the delicate part.
// CLAUDE.md's rule is "never regenerate it from the current binary, which would make
// the comparison self-fulfilling".  The mistake it names is a pipeline re-recording
// from its OWN OUTPUT.  Phase 83 does the opposite and phase/083/check.go enforces
// it: the baselines come from `whim-vim.c`, the pipeline's immutable input, built
// with WHIM's compile line, recorded three times and required identical.  Nothing
// the core produces is on the recording side.  The same input, the same compile line,
// the same five harnesses; one of the five now asks its question a different way.
// Section 4 below is the independent check that the answer is the same one
// everywhere, which is the property a baseline must have and a self-fulfilling one
// cannot be tested for.  `phase/083/check.go` REFUSES a differing baseline set rather
// than overwriting it, so the incantation is
//
// rm -rf .reference/core-baselines .cache/q83 && make whim-phase-83
//
// and `rm -rf .cache/r0` alone is not enough.  Measured: without the first path it
// exits 1 naming ref-term.txt; with it, 31 s.
//
// NOTHING HERE IS A NUMBER THAT WAS OBSERVED.  The table has as many rows as
// `termcheck` has names; which of them resolve is read out of
// `builtin_terminals[]` in the source the phase was handed; and what a REFUSED name
// leaves the terminal as is measured from the binary, by asking it with no
// `+set term=` at all.  So the rules below stay true of the phase that deletes eight
// of those names, and of anything else that changes the table -- they say what the
// table MEANS, and the recording itself is what says what it currently is.
//
// WHAT THIS PHASE PROVES, in order, each depending on the one before:
//
// 1. the tree is untouched: whim-vim.c is what the phase was handed;
// 2. it builds with the boundary's flags, is still absolutely static, and `main` is
// still the only external symbol.  Every SOURCE fact is the input's by the sha
// in 1; these are facts about a binary that was rebuilt;
// 3. THE NEW TABLE MEANS WHAT IT CLAIMS: one row per name asked, every name in
// `builtin_terminals[]` resolving TO ITSELF, and every other name refused with
// an `E5NN` AND the terminal left at the compiled default;
// 4. IT IS THE SAME TABLE EVERYWHERE.  `whim-vim.c`, built with whim's own line,
// records exactly those rows -- and so does every recorded boundary binary, one
// digest across all of them.  THAT is what makes the re-record safe: the
// baseline and every phase's recording move together, so no earlier phase's
// declared delta changes;
// 5. THE INSTRUMENT IS DETERMINISTIC: three whole recordings of that binary, byte
// for byte identical, stream digests included;
// 6. THE INSTRUMENT CAN FAIL, AND THE ONE IT REPLACES CANNOT.  A scratch copy of
// the source with ONE row deleted from `builtin_terminals[]` must move EXACTLY
// that name's row, from resolving to refused -- and must move NOTHING AT ALL
// when the same names are asked the old way.  A corpus that cannot fail is not
// evidence, and that pair is the whole of this phase in one measurement;
// 7. the declared delta holds -- NOTHING, and nothing new: tools/coredelta.sh
// --phase 116 against the re-recorded .reference/core-baselines.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
	"github.com/arbace/go-whim/internal/verify"
)

func init() { check.Register("whim116", Check) }

var (
	z33Named = regexp.MustCompile(`\{\s*"([^"]*)"`)
	z33Err   = regexp.MustCompile(`\bE\d+:`)
	z33Code  = regexp.MustCompile(`^E\d+$`)
	z33ErrW  = regexp.MustCompile(`\bE\d+\b`)
)

// z33Ask is one pty session with no file argument, and the `term=` and
// `t_Co=` answers scraped from every line of it.
func z33Ask(bin string, env []string, term string) []string {
	d, err := os.MkdirTemp("", "ztermcheck-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(d)
	text, _, err := harness.Session(bin, nil, [][]byte{[]byte(":set term? t_Co?\r"), []byte(":q!\r")},
		term, 20*time.Second, time.Second, d, env, 0, 0)
	if err != nil {
		return nil
	}
	var got []string
	for _, line := range strings.Split(strings.ToValidUTF8(string(text), "�"), "\n") {
		for _, kw := range []string{"term=", "t_Co="} {
			if i := strings.Index(line, kw); i >= 0 {
				if fs := strings.Fields(line[i:]); len(fs) > 0 {
					got = append(got, fs[0])
				}
			}
		}
	}
	return got
}

func z33Lines(p string) []string {
	s := check.ReadFile(p)
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func z33Asked(row string) string {
	i, j := strings.Index(row, "'"), strings.LastIndex(row, "'")
	if i < 0 || j <= i {
		return ""
	}
	return row[i+1 : j]
}

func printPrefixed(w io.Writer, prefix, text string) {
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		fmt.Fprintln(w, prefix+l)
	}
}

func unifiedHead(w io.Writer, a, b string, n int) {
	o, _ := exec.Command("diff", a, b).Output()
	ls := strings.Split(strings.TrimRight(string(o), "\n"), "\n")
	for _, l := range check.Head(ls, n) {
		fmt.Fprintln(w, "               "+l)
	}
}

// Whim116 is phase 116, whole: the terminal table is asked with
// `+set term={name}`.
func Check(w io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: check whim116 <work-dir>")
	}
	work := args[0]
	const self = 116
	f := filepath.Join(work, "whim-vim.c")
	base := ".reference/core-baselines"
	tmp, err := os.MkdirTemp("", "whim116")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	T := func(n string) string { return filepath.Join(tmp, n) }
	home, _ := os.MkdirTemp(tmp, "home-")
	env := harness.Env(home)

	// --- 0. the baselines must already be the new shape ----------------------
	if _, e := os.Stat(base + "/ref-term.txt"); e == nil {
		rows := z33Lines(base + "/ref-term.txt")
		ok := len(rows) == len(harness.Terms)
		for _, r := range rows {
			if !strings.HasPrefix(r, ":set term='") {
				ok = false
			}
		}
		if !ok {
			b := &check.Rep{Tag: "baselines", W: w}
			b.Say("%s/ref-term.txt is not the table this phase asks for:", base)
			for _, l := range check.Head(rows, 2) {
				fmt.Fprintln(w, "                 "+l)
			}
			b.Cont("Phase 83 records it, from whim-vim.c -- the pipeline's")
			b.Cont("immutable input -- and REFUSES to overwrite a set that")
			b.Cont("differs, so a changed harness needs both paths removed:")
			fmt.Fprintln(w, "                 rm -rf .reference/core-baselines .cache/q83 && make whim-phase-83")
			return harness.ErrReported
		}
	}

	// --- 1. the tree is untouched --------------------------------------------
	before := check.Sha256File(f)

	// --- 2. the build ---------------------------------------------------------
	// The input the core baselines were recorded from is q82's whim-vim.c, the
	// tree phase 83 was handed, and it is read Out of that boundary's tar.
	var whim chan error
	if in, e := exec.Command("tar", "-xOf", ".build/q82.tar", "./whim-vim.c").Output(); e == nil && len(in) > 0 {
		os.WriteFile(T("whim-vim.c"), in, 0o644)
		whim = make(chan error, 1)
		go func() {
			whim <- exec.Command("gcc", "-O0", "-static", "-s", "-o", T("whim-vim"), T("whim-vim.c")).Run()
		}()
		defer func() {
			if whim != nil {
				<-whim
			}
		}()
	}
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin := filepath.Join(work, "whim-vim")
	if !check.StaticFacts(w, bin) {
		return harness.ErrReported
	}
	if err := exec.Command("gcc", "-c", "-O0", "-fno-stack-protector", "-o", T("new.o"), f).Run(); err != nil {
		return harness.ErrReported
	}
	u, _ := exec.Command("nm", "-u", T("new.o")).Output()
	nu := 0
	for _, l := range strings.Split(string(u), "\n") {
		if strings.TrimSpace(l) != "" {
			nu++
		}
	}
	x, _ := exec.Command("nm", "--extern-only", "--defined-only", T("new.o")).Output()
	var ext []string
	for _, l := range strings.Split(string(x), "\n") {
		if fs := strings.Fields(l); len(fs) > 0 {
			ext = append(ext, fs[len(fs)-1])
		}
	}
	sort.Strings(ext)
	if es := check.TrSpace(ext); es != "main " {
		(&check.Rep{Tag: "symbols", W: w}).Say("the output defines external symbols other than main: %s", es)
		return harness.ErrReported
	}
	(&check.Rep{Tag: "symbols", W: w}).Say("`main` is still the only external symbol, over %d undefined", nu)

	// --- 3. the new table means what it claims ------------------------------
	// A table that did not finish says why (recjob.go); the boundary loop
	// below collects its errors and reports them together, after the wait.
	ztcQuiet := func(b, Out string) error { return check.RecCmd("tools/st.sh", "ztermcheck", b, Out) }
	ztc := func(b, Out string) error {
		e := ztcQuiet(b, Out)
		check.RecReport(w, e)
		return e
	}
	if ztc(bin, T("term")) != nil {
		return harness.ErrReported
	}
	tb := "  table        "
	rule, ok := z33Rule(f, T("term"), bin, env)
	printPrefixed(w, tb, rule)
	if !ok {
		return harness.ErrReported
	}

	// --- 4. it is the same table everywhere ----------------------------------
	sm := &check.Rep{Tag: "same", W: w}
	if whim != nil && <-whim == nil {
		whim = nil
		if ztc(T("whim-vim"), T("term-whim")) != nil {
			return harness.ErrReported
		}
		if check.ReadFile(T("term")) != check.ReadFile(T("term-whim")) {
			sm.Say("whim-vim.c -- the pipeline's immutable input, and where the baselines come from -- records a DIFFERENT table:")
			unifiedHead(w, T("term-whim"), T("term"), 20)
			return harness.ErrReported
		}
		sm.Say("whim-vim, built -O0 -static -s, records the identical table: the baseline is this table and not a third thing")
	} else {
		// Nothing produces .build's tars any more (.gitignore says so), and
		// without q82's the comparison has no left-hand side.  This arm used to
		// say that and pass, which made it a sentence rather than evidence -- a
		// check that cannot fail is not one.  It refuses instead, and names
		// which of the two reasons applies.
		if whim == nil {
			sm.Say("no .build/q82.tar here -- q82's whim-vim.c is what the core baselines were recorded from, so without it this table cannot be held against the input's own recording")
		} else {
			sm.Say("q82's whim-vim.c did not build -- this table cannot be held against the input's own recording")
		}
		whim = nil
		return harness.ErrReported
	}
	bd := &check.Rep{Tag: "boundaries", W: w}
	if fi, e := os.Stat(".build"); e == nil && fi.IsDir() {
		os.MkdirAll(T("bins"), 0o755)
		os.MkdirAll(T("rows"), 0o755)
		var names []string
		for i := 83; i <= self; i++ {
			t := fmt.Sprintf(".build/q%d.tar", i)
			if _, e := os.Stat(t); e != nil {
				continue
			}
			r := fmt.Sprintf("q%d", i)
			b, e := exec.Command("tar", "-xOf", t, "./whim-vim").Output()
			if e != nil {
				if b, e = exec.Command("tar", "-xOf", t, "whim-vim").Output(); e != nil {
					continue
				}
			}
			if len(b) == 0 {
				continue
			}
			os.WriteFile(T("bins/"+r), b, 0o755)
			names = append(names, r)
		}
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		rowErr := make([]error, len(names))
		for i, r := range names {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, r string) {
				defer wg.Done()
				rowErr[i] = ztcQuiet(T("bins/"+r), T("rows/"+r))
				<-sem
			}(i, r)
		}
		wg.Wait()
		if check.RecReport(w, rowErr...) {
			return harness.ErrReported
		}
		if len(names) == 0 {
			bd.Say(".build holds no boundary binary from q83 on -- the table cannot be rechecked across the pipeline")
			return harness.ErrReported
		} else {
			ents, _ := os.ReadDir(T("rows"))
			var rs []string
			for _, e := range ents {
				rs = append(rs, e.Name())
			}
			sort.Strings(rs)
			bad := false
			for _, r := range rs {
				if check.ReadFile(T("term")) != check.ReadFile(T("rows/"+r)) {
					bd.Say("%s records a different table:", r)
					unifiedHead(w, T("rows/"+r), T("term"), 6)
					bad = true
				}
			}
			if bad {
				bd.Cont("The baseline and the recordings do NOT move together, and re-recording would change an earlier phase's declared delta.")
				return harness.ErrReported
			}
			bd.Say("all %d recorded boundary binaries up to q%d record the SAME table as whim-vim and as this one, one digest across every one of them", len(names), self)
		}
	} else {
		bd.Say("no .build here -- this arm needs the boundary tars, and nothing produces them any more; keep the set this pass was run with")
		return harness.ErrReported
	}

	// --- 5. the instrument is deterministic ------------------------------------
	if !check.RecordThrice(w, bin, f, tmp) {
		return harness.ErrReported
	}
	run1 := T("run1")
	(&check.Rep{Tag: "instrument", W: w}).Say("%d cases, %d commands, %d command lines, %d pty scenarios, %d terminals: 3 identical runs, digests included",
		check.DirCount(filepath.Join(run1, "screen")), check.CountHeaders(filepath.Join(run1, "ref-excmds.txt")),
		check.CountHeaders(filepath.Join(run1, "ref-argv.txt")), check.CountHeaders(filepath.Join(run1, "ref-pty.txt")),
		check.CountLines([]byte(check.ReadFile(filepath.Join(run1, "ref-term.txt")))))

	// --- 6. the instrument can fail, and the one it replaces cannot ---------
	af := &check.Rep{Tag: "ablefail", W: w}
	t := check.ReadFile(f)
	i := strings.Index(t, "builtin_terminals[] = {")
	if i < 0 {
		return fmt.Errorf("builtin_terminals[] is not in %s", f)
	}
	j := strings.Index(t[i:], "\n};") + i
	var trows []string
	named := regexp.MustCompile(`^\s*\{\s*"`)
	for _, r := range strings.Split(t[i:j], "\n") {
		if named.MatchString(r) {
			trows = append(trows, r)
		}
	}
	if len(trows) == 0 {
		fmt.Fprintln(w, "builtin_terminals[] has no named row to delete: the break cannot be made")
		return harness.ErrReported
	}
	row := trows[len(trows)-1]
	if strings.Count(t, row+"\n") != 1 {
		fmt.Fprintf(w, "%s is not one line of the file, so deleting it would delete more\n", check.PyRepr(row))
		return harness.ErrReported
	}
	gone := regexp.MustCompile(`^\s*\{\s*"([^"]*)"`).FindStringSubmatch(row)[1]
	broken := T("broken")
	os.MkdirAll(broken, 0o755)
	os.WriteFile(filepath.Join(broken, "whim-vim.c"), []byte(strings.Replace(t, row+"\n", "", 1)), 0o644)
	if err := check.CopyExec(filepath.Join(work, "Makefile"), filepath.Join(broken, "Makefile")); err != nil {
		return err
	}
	if exec.Command("make", "-C", broken).Run() != nil {
		af.Say("the patched copy did not build -- the break is wrong, not the corpus")
		return harness.ErrReported
	}
	if ztc(filepath.Join(broken, "whim-vim"), T("broken-term")) != nil {
		return harness.ErrReported
	}
	moved, mok := z33Moved(T("term"), T("broken-term"), gone)
	if !mok {
		printPrefixed(w, "  ablefail     ", moved)
		unifiedHead(w, T("term"), T("broken-term"), 10)
		return harness.ErrReported
	}
	oldQ := func(b, Out string) {
		rows := make([]string, len(harness.Terms))
		var wg sync.WaitGroup
		for k, term := range harness.Terms {
			wg.Add(1)
			go func(k int, term string) {
				defer wg.Done()
				got := strings.Join(z33Ask(b, env, term), " ")
				if got == "" {
					got = "(none)"
				}
				rows[k] = fmt.Sprintf("TERM=%-20s -> %s", check.PyRepr(term), got)
			}(k, term)
		}
		wg.Wait()
		os.WriteFile(Out, []byte(strings.Join(rows, "\n")+"\n"), 0o644)
	}
	oldQ(bin, T("old-in"))
	oldQ(filepath.Join(broken, "whim-vim"), T("old-broken"))
	oin := check.ReadFile(T("old-in"))
	if strings.Contains(oin, "(none)") {
		af.Say("the old question recorded (none) on an unbroken binary -- the control is broken, not the editor")
		return harness.ErrReported
	}
	if oin != check.ReadFile(T("old-broken")) {
		af.Say("the OLD question saw the deleted row, which contradicts the reason for this phase:")
		unifiedHead(w, T("old-in"), T("old-broken"), 10)
		return harness.ErrReported
	}
	ans := map[string]bool{}
	oldRows := z33Lines(T("old-in"))
	for _, l := range oldRows {
		if k := strings.LastIndex(l, "-> "); k >= 0 {
			ans[l[k+3:]] = true
		} else {
			ans[l] = true
		}
	}
	af.Say("%s, and 0 of %d under the question this replaces -- whose %d rows carry %d distinct answer between them", moved, len(oldRows), len(oldRows), len(ans))

	// --- 7. the declared delta ------------------------------------------------
	if err := verify.CoreDelta(bin, f, 116, w); err != nil {
		return harness.ErrReported
	}

	// --- 1, concluded ---------------------------------------------------------
	if check.Sha256File(f) != before {
		(&check.Rep{Tag: "source", W: w}).Say("whim-vim.c was modified by a phase that must not modify it")
		return harness.ErrReported
	}
	(&check.Rep{Tag: "source", W: w}).Say("whim-vim.c unchanged, %d lines: r33 is its input's tree, and only the instrument moved", check.CountLines([]byte(check.ReadFile(f))))
	return nil
}

// z33Rule is section 3's partition: the recording against builtin_terminals[]
// and the default measured from the binary.  It returns the text the heredoc
// printed, or the message it exited with, and whether it passed.
func z33Rule(src, rec, bin string, env []string) (string, bool) {
	text := check.ReadFile(src)
	i := strings.Index(text, "builtin_terminals[] = {")
	if i < 0 {
		return "builtin_terminals[] is not in the source: nothing to check the table against", false
	}
	j := strings.Index(text[i:], "\n};")
	if j < 0 {
		return "builtin_terminals[] is not in the source: nothing to check the table against", false
	}
	var resolves []string
	for _, m := range z33Named.FindAllStringSubmatch(text[i:i+j], -1) {
		resolves = append(resolves, m[1])
	}
	if len(resolves) == 0 {
		return "builtin_terminals[] holds no named row: the rule below would be vacuous", false
	}
	d, _ := os.MkdirTemp("", "ztermcheck-")
	defer os.RemoveAll(d)
	Out, _, _ := harness.Session(bin, nil, [][]byte{[]byte(":set term? t_Co?\r"), []byte(":q!\r")},
		"xterm", 20*time.Second, time.Second, d, env, 0, 0)
	def := ""
	for _, line := range strings.Split(strings.ToValidUTF8(string(Out), "�"), "\n") {
		if k := strings.Index(line, "term="); k >= 0 && !z33Err.MatchString(line) {
			def = strings.Fields(line[k:])[0]
			break
		}
	}
	if def == "" {
		return "the binary answered nothing with no +set term= at all: the default is unmeasurable", false
	}
	rows := z33Lines(rec)
	asked := make([]string, len(rows))
	for k, r := range rows {
		asked[k] = z33Asked(r)
	}
	if strings.Join(asked, "\x00") != strings.Join(harness.Terms, "\x00") || len(asked) != len(harness.Terms) {
		q := func(s []string) string {
			o := make([]string, len(s))
			for k, v := range s {
				o[k] = check.PyRepr(v)
			}
			return "[" + strings.Join(o, ", ") + "]"
		}
		return fmt.Sprintf("the table asks about %s, and termcheck names %s", q(asked), q(harness.Terms)), false
	}
	var bad []string
	for k, row := range rows {
		name := asked[k]
		_, answer, _ := strings.Cut(row, " -> ")
		fields := strings.Fields(answer)
		got := ""
		var errs []string
		for _, x := range fields {
			if got == "" && strings.HasPrefix(x, "term=") {
				got = x
			}
			if z33Code.MatchString(x) {
				errs = append(errs, x)
			}
		}
		if check.Contains(resolves, name) {
			if len(errs) > 0 || got != "term="+name {
				bad = append(bad, fmt.Sprintf("%-22s resolves in builtin_terminals[] and answered %s", check.PyRepr(name), answer))
			}
		} else if len(errs) == 0 || got != def {
			bad = append(bad, fmt.Sprintf("%-22s is in no row of builtin_terminals[] and answered %s, where a refusal and %s were due", check.PyRepr(name), answer, def))
		}
	}
	if len(bad) > 0 {
		return "the table does not mean what builtin_terminals[] says:\n  " + strings.Join(bad, "\n  "), false
	}
	nRef := 0
	for _, a := range asked {
		if !check.Contains(resolves, a) {
			nRef++
		}
	}
	return fmt.Sprintf("%d rows: %d names builtin_terminals[] carries, each resolving to itself, and %d refused with an E5NN and left at %s", len(rows), len(rows)-nRef, nRef, def), true
}

// z33Moved is section 6's heredoc: exactly the deleted name's row moved, and
// from resolving to refused.
func z33Moved(was, now, gone string) (string, bool) {
	a, b := z33Lines(was), z33Lines(now)
	if len(a) != len(b) {
		return fmt.Sprintf("the broken build recorded %d rows where the input recorded %d", len(b), len(a)), false
	}
	var mv [][2]string
	for k := range a {
		if a[k] != b[k] {
			mv = append(mv, [2]string{a[k], b[k]})
		}
	}
	if len(mv) != 1 {
		return fmt.Sprintf("deleting the %s row moved %d rows, and exactly 1 was due", check.PyRepr(gone), len(mv)), false
	}
	name := z33Asked(mv[0][0])
	if name != gone {
		return fmt.Sprintf("deleting the %s row moved the %s row instead", check.PyRepr(gone), check.PyRepr(name)), false
	}
	_, before, _ := strings.Cut(mv[0][0], " -> ")
	_, after, _ := strings.Cut(mv[0][1], " -> ")
	if !check.Contains(strings.Fields(before), "term="+gone) || !z33ErrW.MatchString(after) {
		return fmt.Sprintf("the %s row went %s -> %s, where resolving -> refused was due", check.PyRepr(gone), before, after), false
	}
	return fmt.Sprintf("deleting the %s row from builtin_terminals[] moves EXACTLY 1 of %d rows here, %s -> %s", check.PyRepr(gone), len(a), before, after), true
}
