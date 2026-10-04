package suite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/internal/procattr"
)

// runPipe is how Run fed the keys before: through a pipe.  The stress test runs
// it beside Run as its control -- under load it is the way that flakes.  It is
// Run in all else: the clock held (pinnedTime), the child in a group of its own
// and reaped by reap, the output a file; so the one thing that differs between
// the modes is how the keys arrive.  With pieces > 1 the keys go in that many
// pieces, pause apart: the "trickle" control, for an editor that starts more
// slowly than one write of the keys takes to land (whimsical: about 35 ms).
func runPipe(bin string, args []string, keys []byte, pieces int, pause, limit time.Duration) ([]byte, int, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, -1, err
	}
	out, err := os.CreateTemp("", "out.")
	if err != nil {
		r.Close()
		w.Close()
		return nil, -1, err
	}
	defer os.Remove(out.Name())
	defer out.Close()
	attr := &os.ProcAttr{
		Env:   append(os.Environ(), pinnedTime),
		Files: []*os.File{r, out, out},
		Sys:   procattr.Group(),
	}
	p, err := os.StartProcess(bin, append([]string{bin}, args...), attr)
	r.Close()
	if err != nil {
		w.Close()
		return nil, -1, err
	}
	// The keys go in as the feeding goroutine of exec.Cmd wrote them: one
	// write, which an editor that reads first sees in part or not at all.
	go func() {
		defer w.Close()
		for i := 0; i < pieces; i++ {
			if i > 0 {
				time.Sleep(pause)
			}
			if _, err := w.Write(keys[len(keys)*i/pieces : len(keys)*(i+1)/pieces]); err != nil {
				return
			}
		}
	}()
	code, err := reap(p.Pid, limit)
	b, rerr := os.ReadFile(out.Name())
	if err != nil {
		return b, -1, err
	}
	return b, code, rerr
}

// once runs c on bin in mode: "file" as the suite runs it, "pipe" in one
// write, "trickle" in 16 pieces 10 ms apart, or "slow" in 16 pieces 100 ms
// apart -- the control for the editors on the JVM, which start after the
// trickle's 150 ms have passed and so find all its keys waiting.
func once(bin, mode string, c WideCase, limit time.Duration) ([]byte, int, error) {
	switch mode {
	case "file":
		return runWide(bin, c, limit)
	case "pipe":
		return runPipe(bin, c.Args, c.Keys, 1, 0, limit)
	case "trickle":
		return runPipe(bin, c.Args, c.Keys, 16, 10*time.Millisecond, limit)
	case "slow":
		return runPipe(bin, c.Args, c.Keys, 16, 100*time.Millisecond, limit)
	}
	return nil, -1, fmt.Errorf("stress: no mode %q", mode)
}

// stress runs every case reps times on each of bins, in each of modes, and
// logs, by group, how many runs answered other than the case's first -- its
// output or its exit status -- and which cases they were.  A terminal case is
// run only from the file: through a pipe it would not be one.
func stress(t *testing.T, bins []string, reps int, cases []WideCase, modes []string, limit time.Duration) {
	for _, b := range bins {
		for _, mode := range modes {
			diff, runs := map[string]int{}, map[string]int{}
			var which []string
			for _, c := range cases {
				if mode != "file" && c.pty != nil {
					continue
				}
				first, fs, ferr := once(b, mode, c, limit)
				n := 0
				for i := 1; i < reps; i++ {
					out, s, err := once(b, mode, c, limit)
					runs[c.Group]++
					if s != fs || !bytes.Equal(out, first) {
						diff[c.Group]++
						n++
						if n <= 3 {
							keep(t, b, mode, c, "first", first, fs, ferr)
							keep(t, b, mode, c, fmt.Sprint("run", i), out, s, err)
						}
					}
				}
				if n > 0 {
					which = append(which, fmt.Sprintf("%s/%s x%d", c.Group, c.Name, n))
				}
			}
			var groups []string
			total, totalRuns := 0, 0
			for g := range runs {
				groups = append(groups, g)
			}
			sort.Strings(groups)
			var by []string
			for _, g := range groups {
				by = append(by, fmt.Sprintf("%s %d of %d", g, diff[g], runs[g]))
				total += diff[g]
				totalRuns += runs[g]
			}
			t.Logf("%s %s: %d of %d runs differ from the case's first (%s)", b, mode, total, totalRuns, strings.Join(by, ", "))
			if len(which) > 0 {
				t.Logf("%s %s: differing: %s", b, mode, strings.Join(which, ", "))
			}
		}
	}
}

// keep writes a run's output, status and error into $DIFFS, when it is set,
// as <editor>-<mode>-<group>-<case>-<which>: the first run of a case and its
// first three that differ, to be compared by hand.
func keep(t *testing.T, bin, mode string, c WideCase, which string, out []byte, status int, err error) {
	dir := os.Getenv("DIFFS")
	if dir == "" {
		return
	}
	name := strings.Join([]string{filepath.Base(bin), mode, c.Group, c.Name, which}, "-")
	note := fmt.Sprintf("status %d, error %v\n", status, err)
	if werr := os.WriteFile(filepath.Join(dir, strings.ReplaceAll(name, "/", "_")), append([]byte(note), out...), 0o644); werr != nil {
		t.Log(werr)
	}
}

// stressArgs reads BINS (space-separated absolute paths of editors: the C
// binary, the Go one, bin/braaam, bin/vijure, bin/caprice, bin/whimsy,
// bin/whimsical, bin/whimsical-debug, bin/whiml or bin/whim++ -- anything run as the suite runs an
// editor), REPS (the runs of each case, default 40), MODES (default "file
// pipe"; "trickle" and "slow" too), LIMIT (a run's limit, a Go duration,
// default DefaultLimit), or skips.  DIFFS, a directory, keeps what differed
// (keep).
func stressArgs(t *testing.T) ([]string, int, []string, time.Duration) {
	bins := strings.Fields(os.Getenv("BINS"))
	if len(bins) == 0 {
		t.Skip("BINS is not set")
	}
	reps, _ := strconv.Atoi(os.Getenv("REPS"))
	if reps < 2 {
		reps = 40
	}
	modes := strings.Fields(os.Getenv("MODES"))
	if len(modes) == 0 {
		modes = []string{"file", "pipe"}
	}
	limit := DefaultLimit
	if s := os.Getenv("LIMIT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			t.Fatalf("LIMIT=%s is not a duration", s)
		}
		limit = d
	}
	return bins, reps, modes, limit
}

// TestStress runs every quick case REPS times on each of BINS, fed from a
// file (Run) and through a pipe (the control), and counts the runs whose
// output or status is not the case's first.  Run it under load, from the
// package's directory (the test binary's), the editors named absolutely:
//
//	BINS="$PWD/bin/whimsy $PWD/bin/whimsical" REPS=40 go test -run 'TestStress$' -timeout 0 -v ./internal/suite
//
// Measured on 2026-09-25 (45 cases then) under 48 busy loops: from a file 0
// of 1,755 runs differ for the C and the Go editors; through a pipe the Go
// editor 18, the C 0.  On 2026-10-02, 80 cases, 40 runs each, 48 busy loops
// on 64 cores: doc/RUST.md, doc/SCHEME.md, doc/JAVA.md, doc/CLOJURE.md and
// doc/HASKELL.md, *Under load* -- 0 runs from a file differ on any of them.
func TestStress(t *testing.T) {
	bins, reps, modes, limit := stressArgs(t)
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	var cs []WideCase
	for _, c := range cases {
		cs = append(cs, WideCase{Group: "quick", Name: c.Name, Keys: c.Keys})
	}
	stress(t, bins, reps, cs, modes, limit)
}

// TestWideStress is TestStress for the wide suite: every wide case REPS times
// on each of BINS, counting runs whose output or status is not the case's
// first; the terminal cases from the file alone.  The ex group is made from
// src/whim-vim.c.  Measured on 2026-09-25 under 48 busy loops, 15 runs a case
// from a file: 0 of 6,720 runs differ (the C and the Go editors).
func TestWideStress(t *testing.T) {
	bins, reps, modes, limit := stressArgs(t)
	cases, err := WideCases()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("../../src/whim-vim.c")
	if err != nil {
		t.Fatal(err)
	}
	ex, err := exCases(src)
	if err != nil {
		t.Fatal(err)
	}
	stress(t, bins, reps, append(cases, ex...), modes, limit)
}
