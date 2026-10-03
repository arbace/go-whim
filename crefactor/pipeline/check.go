package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// Advance is ONE PHASE, as a function of the text it is handed: its steps, the
// sweep, and the canonical print of what is left.
// Every boundary is canonical -- the one C23 spelling phase 0 seeds with -- so
// a phase reads the form its anchors were written against whatever the phases
// before it wrote, and the text it hands on is a fixed point of the printer.
//
// It touches no shared state: its scratch and its sweep's file are temporary
// directories of its own, which is what lets Check run phases side by side.
func (c *Config) Advance(p Phase, text []byte, w io.Writer) ([]byte, error) {
	pr := &prog{text: text}
	if err := c.advance(p, pr, w); err != nil {
		return nil, err
	}
	return pr.text, nil
}

// AdvanceFrom is Advance handed the graph g of text as well, as a run in
// order hands a phase that begins on the graph the graph the phase before
// it left, and the parallel check the graph snapshot it reads (g nil: as
// Advance, the graph imported where the graph steps begin).  It returns
// what the phase spent converting and what its editor did.
func (c *Config) AdvanceFrom(p Phase, text []byte, g *graph.Graph, w io.Writer) ([]byte, Conv, error) {
	pr := &prog{text: text}
	if g != nil && BeginsOnGraph(p) {
		pr.ed = graph.NewEditor(g)
		pr.conv.Graph = true
	}
	err := c.advance(p, pr, w)
	return pr.text, pr.conv, err
}

// GraphSnapshot is the graph a whole run kept beside boundary n, read back
// from its Lisp, when there is one and it is text's graph; nil otherwise.
func (c *Config) GraphSnapshot(n int, text []byte) *graph.Graph { return c.readGraphSnap(n, text) }

// advance is Advance on the program as it is held: a phase that begins on
// the graph may be handed it (pr.ed).  It leaves the text, and the graph
// when the phase ended on it.
func (c *Config) advance(p Phase, pr *prog, w io.Writer) error {
	if p.NoSource {
		return nil
	}
	scratch, err := os.MkdirTemp("", fmt.Sprintf("%s%03d.", c.Name, p.N))
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	if err := c.steps(p, pr, scratch, w); err != nil {
		return err
	}
	return c.finish(pr, scratch, w)
}

// finish is what follows every phase's steps: the sweep, and the canonical
// print -- or, for a phase that ended on the graph, the collection and the
// C view, which are the same text (doc/GRAPH.md, step 3).  There are no
// stages -- no phase hands on a text the sweep has not seen, and every text
// a phase hands on is C.
func (c *Config) finish(pr *prog, scratch string, w io.Writer) error {
	if pr.ed != nil {
		if err := c.collect(pr, w); err != nil {
			return err
		}
		pr.tally()
		_, err := pr.textOf()
		return err
	}
	// In memory: the sweep is a function of the bytes, and the path is only
	// the name it parses them under.  When it cuts nothing in its last round
	// it hands on that round's parse, which is the parse the canonical
	// print would make of the same text.
	path := filepath.Join(scratch, c.WorkName)
	start := time.Now()
	swept, st, ast, err := sweep.PruneParsed(pr.text, path, c.Sweep)
	if err != nil {
		return fmt.Errorf("sweep: %w", err)
	}
	fmt.Fprintf(w, "  sweep        %s; %dms\n", st, time.Since(start).Milliseconds())
	canon, err := cemit.CanonicalParsed(path, swept, ast)
	if err != nil {
		return fmt.Errorf("canonical print: cemit: %w", err)
	}
	pr.text = canon
	return nil
}

// SNAPSHOTS.  A complete run from phase 0 keeps every boundary it produced in
// SnapDir -- qNNN.c, the canonical text after phase NNN -- and then writes
// `manifest`, the digest of the input they came from.  A manifest that names
// the input on disk means the set is whole and is that input's.

func (c *Config) snapPath(n int) string {
	return filepath.Join(c.SnapDir, fmt.Sprintf("q%03d.c", n))
}

func digestOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// snapshotsFor says whether SnapDir holds a whole set for src.
func (c *Config) snapshotsFor(src []byte) bool {
	if c.SnapDir == "" {
		return false
	}
	m, err := os.ReadFile(filepath.Join(c.SnapDir, "manifest"))
	if err != nil || strings.TrimSpace(string(m)) != digestOf(src) {
		return false
	}
	for _, p := range c.Plan {
		if _, err := os.Stat(c.snapPath(p.N)); err != nil {
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
// breaks its own link and is named.  jobs phases run at a time; 0 means
// runtime.NumCPU.
//
// Without snapshots it runs the pipeline in order, which writes them.
func (c *Config) Check(o *Options, jobs int) ([]byte, error) {
	if o.W == nil {
		o.W = io.Discard
	}
	src, err := os.ReadFile(o.Src)
	if err != nil {
		return nil, err
	}
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	if !c.snapshotsFor(src) {
		fmt.Fprintf(o.W, "  check        no snapshots of this input in %s -- running the pipeline in order, which writes them\n", c.SnapDir)
		text, err := c.Run(o)
		if err != nil {
			return nil, err
		}
		if err := c.compileAll(o.W, jobs); err != nil {
			return nil, err
		}
		return text, nil
	}
	start := time.Now()
	// Phase 0 as Run does it: the seed, and then -- unless the phase is only
	// the seed (NoSource) -- its steps, the sweep and the canonical print.
	seed, err := Seed(src, io.Discard)
	if err == nil {
		seed, err = c.Advance(c.Plan[0], seed, io.Discard)
	}
	if err != nil {
		return nil, fmt.Errorf("phase 0: %w", err)
	}
	if q0, _ := os.ReadFile(c.snapPath(0)); !bytes.Equal(seed, q0) {
		return nil, fmt.Errorf("phase 0: the seed of %s is not %s", o.Src, c.snapPath(0))
	}

	type result struct {
		n       int
		err     error
		log     []byte
		refused bool // the phase stopped, as against ran and gave other bytes
	}
	var (
		mu      sync.Mutex
		results []result
		wg      sync.WaitGroup
		sem     = make(chan struct{}, jobs)
		// the links begun on the graph: read from the graph snapshot, or
		// imported from the text where there is none of that text
		read, imported int
		links          []string // each link's time and conversions, for -v
	)
	for i := 1; i < len(c.Plan); i++ {
		p, prev := c.Plan[i], c.Plan[i-1]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var log bytes.Buffer
			in, err := os.ReadFile(c.snapPath(prev.N))
			var out, want []byte
			refused := false
			if err == nil {
				// A phase that begins on the graph begins on q(N-1)'s, read
				// back from its Lisp -- the graph the run handed it -- or,
				// where there is none, imported from q(N-1).c.
				t0 := time.Now()
				var g *graph.Graph
				if BeginsOnGraph(p) && !p.NoSource {
					g = c.readGraphSnap(prev.N, in)
					mu.Lock()
					if g != nil {
						read++
					} else {
						imported++
					}
					mu.Unlock()
				}
				var conv Conv
				out, conv, err = c.AdvanceFrom(p, in, g, &log)
				conv.FromSnapshot = g != nil
				refused = err != nil
				if o.Verbose {
					mu.Lock()
					links = append(links, fmt.Sprintf("  link  %-6d %dms %s", p.N, time.Since(t0).Milliseconds(), conv))
					mu.Unlock()
				}
			}
			if err == nil {
				want, err = os.ReadFile(c.snapPath(p.N))
			}
			if err == nil && !bytes.Equal(out, want) {
				err = fmt.Errorf("gives %d lines where %s holds %d -- the phase no longer does what it did when the snapshots were made",
					bytes.Count(out, []byte("\n")), c.snapPath(p.N), bytes.Count(want, []byte("\n")))
			}
			if err != nil {
				mu.Lock()
				results = append(results, result{p.N, err, log.Bytes(), refused})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(results) > 0 {
		// In phase order, whatever order they finished in; a phase that
		// refused shows the report its steps wrote first, as a run does.
		sort.Slice(results, func(i, j int) bool { return results[i].n < results[j].n })
		for _, r := range results {
			if r.refused || o.Verbose {
				o.W.Write(r.log)
			}
			fmt.Fprintf(o.W, "  phase %-6d %v\n", r.n, r.err)
		}
		return nil, fmt.Errorf("check: %d of %d phases do not reproduce their snapshot", len(results), len(c.Plan)-1)
	}
	last, err := os.ReadFile(c.snapPath(c.Plan[len(c.Plan)-1].N))
	if err != nil {
		return nil, err
	}
	if o.Verbose {
		sort.Strings(links)
		for _, l := range links {
			fmt.Fprintln(o.W, l)
		}
	}
	fmt.Fprintf(o.W, "  check        %d phases, each from its snapshot, %d at a time: every one reproduces the next, %ds\n",
		len(c.Plan)-1, jobs, int(time.Since(start).Seconds()))
	if read+imported > 0 {
		fmt.Fprintf(o.W, "  check        %d of them began on the graph: %d read from its graph snapshot, %d imported from the text\n",
			read+imported, read, imported)
	}
	if err := c.compileAll(o.W, jobs); err != nil {
		return nil, err
	}
	return last, nil
}

// compileAll holds EVERY BOUNDARY TO BEING C THAT COMPILES, and not only to
// being the bytes it was: each snapshot, q000 to the last, is handed to
// Config.Compile, jobs at a time.  Reproducing the snapshots proves that the
// plan is what it was; it says nothing of whether what it was is a program,
// and snapshots q004-q029 were not one for as long as nobody compiled them
// (a literal calling a function the sweep had taken, a walk over a member a
// cut had deleted).  The product compiled throughout, which is why only this
// sees it.  A boundary that does not compile is named with the compiler's
// first errors, every one of them in phase order.
func (c *Config) compileAll(w io.Writer, jobs int) error {
	if c.Compile == nil {
		return nil
	}
	start := time.Now()
	type failure struct {
		n   int
		err error
	}
	var (
		mu    sync.Mutex
		fails []failure
		wg    sync.WaitGroup
		sem   = make(chan struct{}, jobs)
	)
	for _, p := range c.Plan {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			scratch, err := os.MkdirTemp("", fmt.Sprintf("%scc%03d.", c.Name, p.N))
			if err == nil {
				err = c.Compile(c.snapPath(p.N), scratch)
				os.RemoveAll(scratch)
			}
			if err != nil {
				mu.Lock()
				fails = append(fails, failure{p.N, err})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(fails) > 0 {
		sort.Slice(fails, func(i, j int) bool { return fails[i].n < fails[j].n })
		for _, f := range fails {
			fmt.Fprintf(w, "  compile %-4d %v\n", f.n, f.err)
		}
		return fmt.Errorf("check: %d of %d boundaries do not compile", len(fails), len(c.Plan))
	}
	fmt.Fprintf(w, "  check        %d boundaries, %d at a time: every one compiles, %ds\n",
		len(c.Plan), jobs, int(time.Since(start).Seconds()))
	return nil
}
