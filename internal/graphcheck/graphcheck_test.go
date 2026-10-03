package graphcheck

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/build"
)

// GRAPH_SNAPS is a directory of the current input's snapshots (q000-q103,
// .cache/boundaries); GRAPH_PHASES the phases to hold, `0-103` by default
// (phase 0 only with GRAPH_INPUT, slim-vim.c); GRAPH_JOBS how many at once.
func setup(t *testing.T) (dir string, phases []int, src []byte, jobs int) {
	dir = os.Getenv("GRAPH_SNAPS")
	if dir == "" {
		t.Skip("GRAPH_SNAPS is not set")
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if f := os.Getenv("GRAPH_INPUT"); f != "" {
		var err error
		if src, err = os.ReadFile(f); err != nil {
			t.Fatal(err)
		}
	}
	// the plan's steps read the repository's files by their paths from its root
	t.Chdir("../..")
	lo, hi := 0, len(build.Plan)-1
	if s := os.Getenv("GRAPH_PHASES"); s != "" {
		a, b, _ := strings.Cut(s, "-")
		lo, _ = strconv.Atoi(a)
		hi = lo
		if b != "" {
			hi, _ = strconv.Atoi(b)
		}
	}
	for n := lo; n <= hi; n++ {
		if n == 0 && src == nil {
			continue
		}
		phases = append(phases, n)
	}
	jobs = 8
	if s := os.Getenv("GRAPH_JOBS"); s != "" {
		jobs, _ = strconv.Atoi(s)
	}
	return dir, phases, src, jobs
}

// run checks each phase, jobs at a time, and returns the results by phase.
func run(t *testing.T, dir string, phases []int, src []byte, jobs int, opt graph.CollectOptions, control func(*graph.Graph)) []*Result {
	results := make([]*Result, len(build.Plan))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	for _, n := range phases {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			pre, ok, err := PreSweep(dir, n, src)
			if err != nil {
				t.Errorf("phase %d: %v", n, err)
				return
			}
			if !ok {
				return
			}
			want, err := Snap(dir, n)
			if err != nil {
				t.Errorf("phase %d: %v", n, err)
				return
			}
			r, err := Check(n, pre, want, opt, control)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
			mu.Lock()
			results[n] = &r
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}

// TestCollect is step 3's gate: every phase's collected graph prints as
// its snapshot, and took what the sweep took, kind by kind.
func TestCollect(t *testing.T) {
	dir, phases, src, jobs := setup(t)
	same := 0
	for _, r := range run(t, dir, phases, src, jobs, Options(), nil) {
		if r == nil {
			continue
		}
		if !r.Same {
			t.Errorf("phase %d: the collected graph's C view is not q%03d: %s", r.N, r.N, r.Diff)
			continue
		}
		if d := r.StatsDiffer(); d != "" {
			t.Errorf("phase %d: the same text, other counts: %s", r.N, d)
		}
		if r.Counts.Dangling != 0 {
			t.Errorf("phase %d: %d edges dangle after the collection", r.N, r.Counts.Dangling)
		}
		if rep := r.Report; rep.Check != nil || len(rep.Unresolved) > 0 {
			t.Logf("phase %3d: before its sweep, the type check: %v; unresolved: %v", r.N, firstLine(rep.Check), rep.Unresolved)
		}
		same++
		t.Logf("phase %3d: byte for byte; prune %v, import %v, collect %v, print %v; %s",
			r.N, r.Prune.Wall, r.Import.Wall, r.Collect.Wall, r.Print.Wall, r.Stats)
	}
	t.Logf("%d of %d phases byte for byte", same, len(phases))
}

// TestCollectControl is the gate's control, GRAPH_PHASES' phases (1-5 by
// default): a fault in the collected graph -- its last top-level form
// dropped -- is caught on every phase; and the graph's own member rule,
// members by type, is not the sweep's, wherever a phase cuts a member by
// type that a name keeps (a measurement: how many).
func TestCollectControl(t *testing.T) {
	if os.Getenv("GRAPH_PHASES") == "" {
		os.Setenv("GRAPH_PHASES", "1-5")
	}
	dir, phases, src, jobs := setup(t)
	for _, r := range run(t, dir, phases, src, jobs, Options(), func(g *graph.Graph) { g.Forms = g.Forms[:len(g.Forms)-1] }) {
		if r != nil && r.Same {
			t.Errorf("phase %d: the control (the last form dropped) was not caught", r.N)
		}
	}
	opt := Options()
	opt.MembersByType = true
	moved := 0
	for _, r := range run(t, dir, phases, src, jobs, opt, nil) {
		if r == nil {
			continue
		}
		if !r.Same {
			moved++
		}
		t.Logf("phase %3d: members by type: %d members against the sweep's %d; the text %s", r.N, r.Stats.Members, r.Sweep.Members,
			map[bool]string{true: "the same", false: "moved"}[r.Same])
	}
	if moved == 0 {
		t.Errorf("members by type moved no phase")
	}
}

func firstLine(err error) string {
	if err == nil {
		return "none"
	}
	s, _, _ := strings.Cut(err.Error(), "\n")
	return s
}
