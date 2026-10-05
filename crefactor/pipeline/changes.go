package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
)

// WHAT A PHASE CHANGED, IN FORMS, in its log line: the top-level forms and
// external nodes of the graph it was handed against those of the graph it
// hands on, paired and compared as graph.Compare pairs and compares them,
// by their nominal hashes (doc/GRAPH.md, *What a phase changed*) -- what
// `whim graph --changes N --nominal` says of the two boundaries' graphs.
//
// BESIDE THE RUN.  The graph a phase is handed is edited in place, so what
// it was must be taken before the phase's first step, and hashing a graph
// takes 85-210 ms: 13 s of a 134 s run in order, measured, when the run
// waited for it.  So the run takes the graph's Lisp instead -- the units a
// whole run makes anyway for its store of graphs, 40 ms where it does not
// -- and a goroutine reads it back (the same graph, ids and all) and
// hashes it while the next phase runs.  Each boundary is hashed once: the
// graph a phase hands on is the next phase's before.  A phase's line waits
// for its comparison, and what the run writes after it waits behind it
// (formsWriter), so the log is in order; the run waits only at its end.
//
// A phase that ends on text has its C view imported at its end rather than
// at the next phase's start -- the import the next phase would make, and
// counted in its line as before -- and the last phase's, which nothing
// would import, for this alone.  A phase that begins on text is not
// compared: there is no graph of what it was handed.

// formTracker follows the graphs from phase to phase.
type formTracker struct {
	opt     graph.HashOptions
	before  *formFuture // the graph handed to the phase now running; nil for none
	pending []formLine  // the lines waiting for their comparison, in order
	mu      sync.Mutex
	hashed  int           // boundaries read back and hashed
	cpu     time.Duration // in doing it
	waited  time.Duration // the run's, for the comparisons
	extra   time.Duration // in the last phase's import, made for this alone
}

// A formFuture is a boundary's UnitHashes, being made.
type formFuture struct {
	done  chan struct{}
	units []graph.UnitHash
	err   error
}

func (x *formFuture) wait() ([]graph.UnitHash, error) {
	<-x.done
	return x.units, x.err
}

// launch reads the Lisp of a graph back and hashes it, beside the run.
func (f *formTracker) launch(lisp []byte) *formFuture {
	x := &formFuture{done: make(chan struct{})}
	go func() {
		defer close(x.done)
		start := time.Now()
		g, err := graph.Read(lisp)
		if err == nil {
			x.units, x.err = g.UnitHashes(f.opt)
		} else {
			x.err = err
		}
		f.mu.Lock()
		f.hashed++
		f.cpu += time.Since(start)
		f.mu.Unlock()
	}()
	return x
}

// A formLine is what waits to be written: text, or a phase's line --
// head, the comparison, and with list each form under it.
type formLine struct {
	text          []byte
	n             int
	head          string
	before, after *formFuture
	list          bool
}

func (l formLine) ready() bool {
	if l.after == nil {
		return true
	}
	select {
	case <-l.before.done:
	default:
		return false
	}
	select {
	case <-l.after.done:
		return true
	default:
		return false
	}
}

// begin takes what the graph handed to a phase that begins on it is, when
// the phase before did not hand it on.
func (f *formTracker) begin(g *graph.Graph) {
	if f.before == nil {
		f.before = f.launch(g.Lisp())
	}
}

// end takes what the graph a phase hands on is, from its Lisp, and queues
// the phase's line.
func (f *formTracker) end(n int, units [][]byte, head string, list bool) {
	after := f.launch(bytes.Join(units, nil))
	f.pending = append(f.pending, formLine{n: n, head: head, before: f.before, after: after, list: list})
	f.before = after
}

// A formsWriter is the run's report while lines wait: what is written
// after a line that waits waits behind it, so the log is in order.
type formsWriter struct {
	w io.Writer
	f *formTracker
}

func (x *formsWriter) Write(p []byte) (int, error) {
	if len(x.f.pending) == 0 {
		return x.w.Write(p)
	}
	x.f.pending = append(x.f.pending, formLine{text: bytes.Clone(p)})
	x.f.drain(x.w, false)
	return len(p), nil
}

// drain writes the lines queued that are ready, in order -- with wait,
// every one, waiting for its comparison.
func (f *formTracker) drain(w io.Writer, wait bool) {
	start := time.Now()
	for len(f.pending) > 0 {
		l := f.pending[0]
		if !wait && !l.ready() {
			break
		}
		f.pending = f.pending[1:]
		if l.after == nil {
			w.Write(l.text)
			continue
		}
		b, err := l.before.wait()
		if err == nil {
			_, err = l.after.wait()
		}
		if err != nil {
			fmt.Fprintf(w, "%sforms not compared: %v\n", l.head, err)
			continue
		}
		a, _ := l.after.wait()
		u := graph.CompareUnits(b, a)
		fmt.Fprintf(w, "%s%s\n", l.head, formsLine(u))
		if l.list {
			listForms(w, u, b, a)
		}
	}
	if wait {
		f.waited += time.Since(start)
	}
}

// report is the account of the hashing, for the run's end.
func (f *formTracker) report(w io.Writer) {
	f.mu.Lock()
	hashed, cpu := f.hashed, f.cpu
	f.mu.Unlock()
	if hashed == 0 {
		return
	}
	fmt.Fprintf(w, "  forms        %d boundaries read back and hashed beside the run, %dms of CPU; the run waited %dms for them",
		hashed, cpu.Milliseconds(), f.waited.Milliseconds())
	if f.extra > 0 {
		fmt.Fprintf(w, ", and imported the last for them in %dms", f.extra.Milliseconds())
	}
	fmt.Fprintln(w)
}

// importAhead imports the text a phase ended on, for the phase after it
// (or, after the last, for its comparison alone).
func (c *Config) importAhead(pr *prog) (Conv, error) {
	start := time.Now()
	g, _, err := graph.Import(filepath.Join(os.TempDir(), c.WorkName), pr.text)
	if err != nil {
		return Conv{}, fmt.Errorf("import: %w", err)
	}
	pr.ed = graph.NewEditor(g)
	return Conv{Imports: 1, Import: time.Since(start)}, nil
}

// formsLine is UnitChanges in a few words: `forms: 24 removed, 72
// changed`, the lists that are empty left out.
func formsLine(u *graph.UnitChanges) string {
	var parts []string
	for _, x := range []struct {
		n    int
		what string
	}{{len(u.Added), "added"}, {len(u.Removed), "removed"}, {len(u.Changed), "changed"}} {
		if x.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", x.n, x.what))
		}
	}
	if len(parts) == 0 {
		return "forms: none changed"
	}
	return "forms: " + strings.Join(parts, ", ")
}

// listForms writes each form a phase added, removed and changed, a line
// each, as `whim graph --changes N --nominal` does.
func listForms(w io.Writer, u *graph.UnitChanges, before, after []graph.UnitHash) {
	for _, i := range u.Added {
		fmt.Fprintf(w, "      added    %s\n", after[i].Label)
	}
	for _, i := range u.Removed {
		fmt.Fprintf(w, "      removed  %s\n", before[i].Label)
	}
	for _, p := range u.Changed {
		fmt.Fprintf(w, "      changed  %s\n", after[p[1]].Label)
	}
}
