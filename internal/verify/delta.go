package verify

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/dead"
	"github.com/arbace/go-whim/internal/harness"
)

// WHAT WHIM-VIM DOES DIFFERENTLY, AS A CHECK RATHER THAN A REPORT.  This was
// tools/whimdelta.sh and tools/coredelta.sh.
//
// This is the rule that separates GOALS.md from arbace/slim-vim's SLIM-GOAL.md.
// There, any behavioural change is a bug and the check is "nothing moved".
// Here a change is the point, so the check is "exactly this moved" -- the phase
// says which behaviour it is removing, in advance, and the harness proves it
// removed that and nothing else.
//
// "Six commands differ" is a check.  "Some commands differ" is not.
//
// FROM build.CoreFrom (83) ON THE DELTA IS MEASURED AGAINST OTHER BASELINES
// AND WITH ANOTHER INSTRUMENT, and Delta hands the phase to CoreDelta: the
// declarations from CoreFrom on, against .reference/core-baselines, which
// phase 83 records from the tree it is handed.  A caller runs Delta for every
// stage and never needs to know.

const (
	slimBaselines = ".reference/baselines"      // phase 0's, from slim-vim.c
	coreBaselines = ".reference/core-baselines" // phase 83's, from q82
)

// Delta checks the delta the phases declare up to phase n against the binary
// this tree builds: every declaration for a phase <= n, folded.  It is run
// once per stage, for the stage's last phase, because the list up to a phase is
// the whole difference from slim at that phase and so contains every earlier
// phase's.
func Delta(bin, src string, n int, w io.Writer) error {
	if n >= build.CoreFrom {
		return CoreDelta(bin, src, n, w)
	}
	text, err := Declarations(0, n)
	if err != nil {
		return err
	}
	d := fold(text, n)
	return DeltaIs(bin, src, d.term, keys(d.cases), keys(d.cmds), w)
}

// DeltaIs is the delta stated outright: exactly these Ex commands moved,
// exactly these behaviour cases moved, and the terminal table moved or did not.
//
// A phase may change an editing BEHAVIOUR as well as an Ex command's exit, and
// until phase 8 none had, so this asserted "behaviour: none" outright.  That is
// the right default -- most of what is removed here is a command, not a
// keystroke -- but a default is not a check, and a phase that genuinely moves a
// case has to be able to say which.  Declared the same way and held to the same
// rule: exactly these, and no others.
func DeltaIs(bin, src string, termMoved bool, cases, cmds []string, w io.Writer) error {
	cases, cmds = uniqSorted(cases), uniqSorted(cmds)

	// BEFORE ANYTHING BEHAVIOURAL: no option global may be left without the
	// row that initialises it.  This is a source question rather than a
	// behavioural one, but it belongs here because it is the whim pipeline
	// that drops rows, and because the thing it catches is invisible to every
	// check that follows -- an orphaned global is *used*, so no warning names
	// it, and it segfaults only on the one command that reaches it.
	//
	// It runs ALONGSIDE the harnesses rather than before them: it reads the
	// source, they run the binary, and neither waits for the other.
	fail := false
	orphans := orphanOpts(src, w, &fail)

	if fi, err := os.Stat(filepath.Join(slimBaselines, "behaviour")); err != nil || !fi.IsDir() {
		orphans()
		fmt.Fprintln(w, "  delta        no slim baselines to compare against")
		if fail {
			return harness.ErrReported
		}
		return nil
	}

	tmp, err := os.MkdirTemp("", "whimdelta")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// THE THREE HARNESSES ARE INDEPENDENT, so they run at once.  Each writes
	// into its own place under tmp and reads nothing the others write.
	// Serially they were 4.4 + 1.7 + 2.2 seconds, which is a third of a phase
	// that does its actual work in two.
	b := filepath.Join(tmp, "b")
	m := filepath.Join(tmp, "m")
	s := filepath.Join(tmp, "s")
	errs := make(chan error, 3)
	go func() { errs <- harness.Behaviour(bin, b, io.Discard) }()
	go func() { errs <- harness.TermCheck(bin, m, io.Discard) }()
	go func() { errs <- harness.ExSweep(bin, src, s, io.Discard) }()
	var herr error
	for i := 0; i < 3; i++ {
		if e := <-errs; e != nil && herr == nil {
			herr = e
		}
	}
	orphans()
	if herr != nil {
		// The shell exited here with the status of the last harness waited
		// for and printed nothing at all.  A refusal with no message says
		// where its message went.
		return fmt.Errorf("a delta harness failed on %s: %w", bin, herr)
	}

	moved := spaced(movedCases(filepath.Join(slimBaselines, "behaviour"), b))
	want := spaced(cases)
	if moved != want {
		fmt.Fprintln(w, "  delta        behaviour cases that moved:")
		fmt.Fprintf(w, "                 got      %s\n", orNone(moved))
		fmt.Fprintf(w, "                 expected %s\n", orNone(want))
		fail = true
	}

	// The terminal table is declared the same way the behaviour cases are.
	// Until phase 19 no phase could move it, so "expected unchanged" was the
	// whole check; a phase that makes every TERM resolve to one entry has to
	// be able to say so.
	same := sameFile(filepath.Join(slimBaselines, "ref-term.txt"), m)
	if termMoved && same {
		fmt.Fprintln(w, "  delta        the terminal table was declared to move and did not")
		fail = true
	}
	if !termMoved && !same {
		fmt.Fprintln(w, "  delta        the terminal table moved, expected unchanged")
		fail = true
	}

	changed := spaced(changedCommands(filepath.Join(slimBaselines, "ref-exsweep.txt"), s))
	expected := spaced(cmds)
	if changed != expected {
		fmt.Fprintln(w, "  delta        Ex commands that moved:")
		fmt.Fprintf(w, "                 got      %s\n", changed)
		fmt.Fprintf(w, "                 expected %s\n", expected)
		fail = true
	}

	if fail {
		fmt.Fprintln(w, "               A phase here may change behaviour, but only the behaviour")
		fmt.Fprintln(w, "               it said it would.  Anything else is a bug, and a delta")
		fmt.Fprintln(w, "               list that is merely widened to fit is not a check.")
		return harness.ErrReported
	}
	if len(cases) > 0 {
		fmt.Fprintf(w, "  delta        exactly as declared: %s; cases: %s\n", expected, want)
	} else {
		fmt.Fprintf(w, "  delta        exactly as declared: %s\n", expected)
	}
	return nil
}

// CoreDelta is Delta's rule against a different instrument: it records the
// binary (harness.CoreRecord) and hands the recording, the baselines and the
// declarations from build.CoreFrom on to harness.CoreCompare, which requires
// EXACTLY the declared difference -- every record that moved is declared, every
// declaration moved something, and nothing else differs at all.
//
// THE BASELINES ARE q82'S: .reference/core-baselines, recorded by phase 83 from
// the tree it is handed, built with the compile line that tree carries.  So the
// delta is the difference from q82, not from slim; it is CUMULATIVE, as phases
// 0-82's are against slim -- the declarations up to phase n are the whole
// difference from q82 at n -- and it starts empty at 83.
//
// THE INSTRUMENT IS THE SCREEN (phase 86, GOALS.md II.2): keystrokes in on
// stdin, escape sequences out on stdout, and a screen per redraw rebuilt from
// them.  The file-based harnesses -- behaviour, exsweep -- are phases 0-82's
// and are untouched; they cannot measure these, because the editor they measure
// is on its way to having no file to write and no stream to print on.
func CoreDelta(bin, src string, n int, w io.Writer) error {
	// The same source check DeltaIs runs beside its harnesses.
	fail := false
	orphans := orphanOpts(src, w, &fail)

	// Unlike the slim delta, an absent baseline is a failure and not a note:
	// phase 83 records them before anything is compared, so a missing set
	// means the pipeline is being run out of order.
	ok := isDir(filepath.Join(coreBaselines, "screen"))
	for _, f := range []string{"ref-excmds.txt", "ref-argv.txt", "ref-term.txt", "ref-pty.txt"} {
		ok = ok && isFile(filepath.Join(coreBaselines, f))
	}
	if !ok {
		orphans()
		fmt.Fprintf(w, "  delta        no core baselines at %s -- phase 83 records them\n", coreBaselines)
		fmt.Fprintln(w, "               (a recording is screen/, ref-excmds.txt, ref-argv.txt,")
		fmt.Fprintln(w, "                ref-pty.txt and ref-term.txt: whimtools zrecord)")
		return harness.ErrReported
	}

	tmp, err := os.MkdirTemp("", "coredelta")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	now := filepath.Join(tmp, "now")
	if err := harness.CoreRecord(bin, src, now, w); err != nil {
		return err // it named the harness that failed
	}
	orphans()

	text, err := Declarations(build.CoreFrom, n)
	if err != nil {
		return err
	}
	declared := filepath.Join(tmp, "declared")
	if err := os.WriteFile(declared, []byte(text), 0o644); err != nil {
		return err
	}
	if err := harness.CoreCompare(coreBaselines, now, declared, n, w); err != nil {
		fail = true
	}
	if fail {
		return harness.ErrReported
	}
	return nil
}

// orphanOpts starts the option-orphan check on the source and returns the
// function that waits for it, prints what it said and records a refusal.  Its
// output goes to the report where the shell put it: after the harnesses, before
// the first comparison.
func orphanOpts(src string, w io.Writer, fail *bool) func() {
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(&out, "whimtools: %v\n", err)
			done <- err
			return
		}
		err = dead.OrphanOpts(data, &out)
		if err != nil && (!strings.HasPrefix(err.Error(), "orphanopts: ") ||
			strings.Contains(err.Error(), "rows parsed") ||
			strings.Contains(err.Error(), "not in this file")) {
			fmt.Fprintln(&out, err)
		}
		done <- err
	}()
	return func() {
		err := <-done
		w.Write(out.Bytes())
		if err != nil {
			*fail = true
		}
	}
}

// movedCases is every behaviour case whose record differs, which is what
// `diff -rq BASE NEW | grep '^Files'` named.  A record only one side holds is
// NOT one of them: a missing case is a harness that died, and the count of
// cases is asserted where the baseline is recorded.
func movedCases(base, cand string) []string {
	var out []string
	for _, rel := range relFiles(base) {
		b, eb := os.ReadFile(filepath.Join(base, rel))
		x, ex := os.ReadFile(filepath.Join(cand, rel))
		if eb != nil || ex != nil {
			continue
		}
		if !bytes.Equal(b, x) {
			out = append(out, rel)
		}
	}
	return out
}

// changedCommands is the second field of every line `diff` prints with a `<`
// or a `>` -- the Ex command whose row moved, whichever side it moved on.
func changedCommands(base, cand string) []string {
	a, err := os.ReadFile(base)
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(cand)
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, l := range diffLines(textLines(string(a)), textLines(string(b))) {
		f := strings.Fields(l)
		if len(f) > 0 {
			set[f[0]] = true
		} else {
			set[""] = true // awk's $2 of a bare `<`, which sort keeps
		}
	}
	return keys(set)
}

// diffLines is every line of either side that is not in their longest common
// subsequence: exactly the lines `diff` prefixes with `<` or `>`.
func diffLines(a, b []string) []string {
	// The longest common subsequence, as a table.  Both sides here are one
	// row per Ex command -- 600 of them -- so the quadratic table is a few
	// megabytes and the clarity is worth more than the Myers algorithm.
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, a[i])
			i++
		default:
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

// spaced is the shell's `sort -u | tr '\n' ' '`: the set in order, each
// followed by a space, and empty when the set is.
func spaced(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return strings.Join(xs, " ") + " "
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// relFiles is every file under a directory, by its path relative to it, sorted.
func relFiles(dir string) []string {
	var out []string
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err == nil {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func sameFile(a, b string) bool {
	x, ea := os.ReadFile(a)
	y, eb := os.ReadFile(b)
	return ea == nil && eb == nil && bytes.Equal(x, y)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// uniqSorted is the shell's `sort -u` on a list stated by hand.
func uniqSorted(xs []string) []string {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return keys(m)
}
