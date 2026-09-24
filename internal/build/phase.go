package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/sweep"
)

// Advance is ONE PHASE, as a function of the text it is handed: its steps, the
// sweep, and the canonical print of what is left.
// Every boundary is canonical -- the one C23 spelling phase 0 seeds with -- so
// a phase reads the form its anchors were written against whatever the phases
// before it wrote, and the text it hands on is a fixed point of the printer.
//
// It touches no shared state: its scratch and its sweep's file are temporary
// directories of its own, which is what lets Check run phases side by side.
func Advance(p Phase, text []byte, w io.Writer) ([]byte, error) {
	if p.NoSource {
		return text, nil
	}
	scratch, err := os.MkdirTemp("", fmt.Sprintf("whim%03d.", p.N))
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	out, err := RunPhase(p, text, scratch, w)
	if err != nil {
		return nil, err
	}
	return finish(p, out, scratch, w)
}

// finish is what follows every phase's steps: the sweep, and the canonical
// print.  There are no stages -- no phase hands on a text the sweep has not
// seen, and every text a phase hands on is C: phases 69 and 74, which removed
// a typedef while prototypes naming it waited for a later sweep, leave the
// typedef to the sweep now, which takes it with them.
func finish(p Phase, text []byte, scratch string, w io.Writer) ([]byte, error) {
	path := filepath.Join(scratch, "whim-vim.c")
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return nil, err
	}
	if _, err := sweep.Sweep(path, w); err != nil {
		return nil, err
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	canon, err := Seed(text, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("canonical print: %w", err)
	}
	return canon, nil
}

// SNAPSHOTS.  A complete build from phase 0 keeps every boundary it produced
// in .cache/boundaries/ -- qNNN.c, the canonical text after phase NNN -- and
// then writes `manifest`, the digest of the input they came from.  A manifest
// that names the input on disk means the set is whole and is that input's.
const SnapDir = ".cache/boundaries"

func snapPath(n int) string { return filepath.Join(SnapDir, fmt.Sprintf("q%03d.c", n)) }

func digestOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// snapshotsFor says whether .cache/boundaries holds a whole set for src.
func snapshotsFor(src []byte) bool {
	m, err := os.ReadFile(filepath.Join(SnapDir, "manifest"))
	if err != nil || strings.TrimSpace(string(m)) != digestOf(src) {
		return false
	}
	for _, p := range Plan {
		if _, err := os.Stat(snapPath(p.N)); err != nil {
			return false
		}
	}
	return true
}

// Check answers whether the plan, run on the input o.Src names, gives back
// the product -- and returns that product for the caller to compare.
//
// With a whole set of snapshots for this input it does not run the pipeline
// end to end.  It proves it LINK BY LINK, every phase at once: phase 0's seed
// of the input is q000, and for every phase N, Advance on q(N-1) is qN.  Those
// together are the induction the sequential run would have walked, so the
// last snapshot is what the pipeline gives; a phase whose program changed
// breaks its own link and is named.  jobs phases run at a time; 0 means as
// many as fit (runtime.NumCPU; measured, 64 at once peak at 14 GB and finish in 77 s).
//
// Without snapshots it runs the pipeline in order, which writes them.
func Check(o *Options, jobs int) ([]byte, error) {
	src, err := os.ReadFile(o.Src)
	if err != nil {
		return nil, err
	}
	if !snapshotsFor(src) {
		fmt.Fprintf(o.W, "  check        no snapshots of this input in %s -- running the pipeline in order, which writes them\n", SnapDir)
		return Run(o)
	}
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	start := time.Now()
	seed, err := Seed(src, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("phase 0: %w", err)
	}
	if q0, _ := os.ReadFile(snapPath(0)); !bytes.Equal(seed, q0) {
		return nil, fmt.Errorf("phase 0: the seed of %s is not %s", o.Src, snapPath(0))
	}

	type result struct {
		n   int
		err error
		log []byte
	}
	var (
		mu      sync.Mutex
		results []result
		wg      sync.WaitGroup
		sem     = make(chan struct{}, jobs)
	)
	for i := 1; i < len(Plan); i++ {
		p, prev := Plan[i], Plan[i-1]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var log bytes.Buffer
			in, err := os.ReadFile(snapPath(prev.N))
			var out, want []byte
			if err == nil {
				out, err = Advance(p, in, &log)
			}
			if err == nil {
				want, err = os.ReadFile(snapPath(p.N))
			}
			if err == nil && !bytes.Equal(out, want) {
				err = fmt.Errorf("gives %d lines where %s holds %d -- the phase no longer does what it did when the snapshots were made",
					bytes.Count(out, []byte("\n")), snapPath(p.N), bytes.Count(want, []byte("\n")))
			}
			if err != nil {
				mu.Lock()
				results = append(results, result{p.N, err, log.Bytes()})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(results) > 0 {
		for _, r := range results {
			fmt.Fprintf(o.W, "  phase %-6d %v\n", r.n, r.err)
		}
		return nil, fmt.Errorf("check: %d of %d phases do not reproduce their snapshot", len(results), len(Plan)-1)
	}
	last, err := os.ReadFile(snapPath(Plan[len(Plan)-1].N))
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(o.W, "  check        %d phases, each from its snapshot, %d at a time: every one reproduces the next, %ds\n",
		len(Plan)-1, jobs, int(time.Since(start).Seconds()))
	return last, nil
}
