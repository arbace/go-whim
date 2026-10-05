package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// Run applies the plan to the input and returns the source it leaves.
//
// The text lives in memory between steps.  It reaches the disk only where the
// sweep needs a path: the sweep compiles the tree speculatively to answer the
// enumerator question, which is the one thing in the pipeline that is not a
// function of the bytes alone.
func (c *Config) Run(o *Options) ([]byte, error) {
	if o.W == nil {
		o.W = io.Discard
	}
	defer inOrderGC()()
	src, err := os.ReadFile(o.Src)
	if err != nil {
		return nil, err
	}
	work := o.Work
	if work == "" {
		work, err = os.MkdirTemp("", c.Name+"-build")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(work)
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(work, c.WorkName)
	// Only a whole, ordinary run from phase 0 writes the snapshots, and it
	// unseals the set first and seals it last, so a run that stops part way
	// leaves no set that claims to be whole.
	o.snap = c.SnapDir != "" && o.From == 0 && o.To < 0 && !o.KeepGoing
	if o.snap {
		if err := os.MkdirAll(c.SnapDir, 0o755); err != nil {
			return nil, err
		}
		os.Remove(filepath.Join(c.SnapDir, "manifest"))
		// and the graph snapshots: this run writes those its plan reads,
		// into a store begun anew (and the .g files of before the store)
		if old, err := filepath.Glob(filepath.Join(c.SnapDir, "q*.g")); err == nil {
			for _, f := range old {
				os.Remove(f)
			}
		}
		st, err := graph.CreateStore(c.SnapDir)
		if err != nil {
			return nil, err
		}
		o.store = st
		defer func() {
			if o.store != nil {
				o.store.Discard()
				o.store = nil
			}
		}()
	}

	// pr is the program between phases: the text, always, and the graph
	// when the phase about to run begins on it (hybrid.go).
	var pr *prog
	if o.From > 0 {
		// Starting inside the pipeline: the input is a boundary, not the seed.
		// Nothing here proves it is the boundary it claims to be -- that is for
		// the caller, and only a build from phase 0 answers for the product.
		pr = &prog{text: src}
	}
	last := -1 // the boundary before the phase about to run
	for _, p := range c.Plan {
		if o.To >= 0 && p.N > o.To {
			break
		}
		if p.N < o.From {
			continue
		}
		if p.Block != "" {
			fmt.Fprintf(o.W, "  block  %s\n", p.Block)
		}
		start := time.Now()
		// rep is the phase's own report: o.W itself when verbose, and
		// otherwise held back, and written only if the phase refuses.
		rep, held := o.W, (*bytes.Buffer)(nil)
		if o.Verbose {
			fmt.Fprintf(o.W, "  phase %-6d %s\n", p.N, p.Name)
		} else {
			held = &bytes.Buffer{}
			rep = held
		}
		refused := func() {
			if held != nil {
				fmt.Fprintf(o.W, "  phase %-6d %s\n", p.N, p.Name)
				o.W.Write(held.Bytes())
				held.Reset()
			}
		}
		var before int
		if pr != nil {
			before = bytes.Count(pr.text, []byte("\n"))
		}
		// The seed: a phase 0 that begins on the graph imports the input
		// itself, and is handed that graph; otherwise the canonical print.
		var seedG *graph.Graph
		var seedImport time.Duration
		if p.Seed {
			var out []byte
			var err error
			if BeginsOnGraph(p) && !p.NoSource {
				seedG, out, seedImport, err = SeedGraph(src, filepath.Join(os.TempDir(), c.WorkName), rep)
			} else {
				out, err = Seed(src, rep)
			}
			if err != nil {
				refused()
				return nil, fmt.Errorf("phase %d: %w", p.N, err)
			}
			pr = &prog{text: out}
			before = bytes.Count(src, []byte("\n"))
		}
		if pr == nil {
			return nil, fmt.Errorf("build: phase %d runs before the input was seeded", p.N)
		}
		if p.NoSource {
			if held != nil {
				summary := "no edit"
				if p.Seed {
					summary = fmt.Sprintf("%d lines from %d", bytes.Count(pr.text, []byte("\n")), before)
				}
				fmt.Fprintf(o.W, "  phase %-6d %s: %s\n", p.N, p.Name, summary)
			}
			if err := c.keep(o, p.N, pr.text); err != nil {
				return nil, err
			}
			last = p.N
			continue
		}
		scratch, err := os.MkdirTemp("", fmt.Sprintf("%s%03d.", c.Name, p.N))
		if err != nil {
			return nil, err
		}
		in := pr.text
		pr.conv = Conv{}
		// The boundary: the graph goes on only to a phase that begins on
		// it, with an editor of its own, imported here if the phase before
		// ended on text; and a whole run keeps it beside the boundary.
		if BeginsOnGraph(p) {
			if seedG != nil {
				pr.ed = graph.NewEditor(seedG)
				pr.conv.Graph = true
				pr.conv.Imports++
				pr.conv.Import += seedImport
			} else if pr.ed != nil {
				pr.ed = graph.NewEditor(pr.ed.Graph())
				pr.conv.Graph = true
			} else if _, err := c.editor(pr, scratch); err != nil {
				os.RemoveAll(scratch)
				refused()
				return nil, fmt.Errorf("phase %d (%s): %w", p.N, p.Name, err)
			}
			if o.snap && last >= 0 {
				if err := o.writeGraphSnap(last, in, pr.ed.Graph()); err != nil {
					return nil, err
				}
			}
		} else {
			pr.ed = nil
		}
		err = c.steps(p, pr, scratch, rep)
		acts := 0
		if held != nil {
			acts = bytes.Count(held.Bytes(), []byte("\n"))
		}
		switch {
		case err == nil:
		case o.KeepGoing:
			// A PHASE THAT HAS ALREADY SAID WHY RETURNS AN EMPTY ERROR.  Several
			// edits print their refusal to the report and return `fmt.Errorf("")`,
			// so repeating `%v` here wrote `REFUSED 127` with nothing after it,
			// and a reader of the log could not tell a silent refusal from a
			// missing message.
			why := err.Error()
			if strings.TrimSpace(why) == "" {
				why = "(it printed its reason above)"
			}
			refused()
			fmt.Fprintf(o.W, "  REFUSED %-4d %s\n", p.N, why)
			o.Refused = append(o.Refused, fmt.Sprintf("%d: %s", p.N, why))
			pr = &prog{text: in}
		default:
			os.RemoveAll(scratch)
			refused()
			return nil, fmt.Errorf("phase %d (%s): %w", p.N, p.Name, err)
		}
		// The lines the edits took: on text, before the sweep; a phase that
		// ends on the graph is not printed until it is collected.
		edited := -1
		if pr.ed == nil {
			edited = bytes.Count(pr.text, []byte("\n"))
		}
		// The sweep and the canonical print, or the collection and the C
		// view -- also after a refusal, so the text handed on is canonical
		// either way.
		err = c.finish(pr, scratch, rep)
		os.RemoveAll(scratch)
		switch {
		case err == nil:
		case o.KeepGoing:
			refused()
			fmt.Fprintf(o.W, "  REFUSED %-4d %v\n", p.N, err)
			o.Refused = append(o.Refused, fmt.Sprintf("%d: %v", p.N, err))
			pr = &prog{text: in}
		default:
			refused()
			return nil, fmt.Errorf("phase %d (%s): %w", p.N, p.Name, err)
		}
		if err := c.keep(o, p.N, pr.text); err != nil {
			return nil, err
		}
		last = p.N
		d := time.Since(start)
		after := bytes.Count(pr.text, []byte("\n"))
		if held != nil {
			fmt.Fprintf(o.W, "  phase %-6d %s: %s\n", p.N, p.Name, summary(acts, before, edited, after, d, pr.conv))
		} else if d > time.Second {
			fmt.Fprintf(o.W, "  phase %-6d %ds, %d lines\n", p.N, int(d.Seconds()), after)
		}
	}
	if pr == nil {
		return nil, fmt.Errorf("build: no phase ran")
	}
	// The work tree is left holding the source the pipeline leaves.
	if err := os.WriteFile(path, pr.text, 0o644); err != nil {
		return nil, err
	}
	if o.snap {
		st := o.store
		o.store = nil
		if err := st.Close(); err != nil {
			return nil, err
		}
		if st.Units > 0 {
			fmt.Fprintf(o.W, "  graphs       %d kept in the store, %d units of %d distinct: %d bytes on disk for %d of Lisp\n",
				st.Graphs, st.Units, st.New, st.Size(), st.Bytes)
		}
		if err := os.WriteFile(filepath.Join(c.SnapDir, "manifest"), []byte(digestOf(src)+"\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return pr.text, nil
}

// RunPhase applies one phase's steps, with scratch as the directory the
// Config's Resolve may hand its arguments: the text in, the text out, the
// graph steps on a graph imported where they begin and printed where they
// end (hybrid.go).
func (c *Config) RunPhase(p Phase, text []byte, scratch string, w io.Writer) ([]byte, error) {
	pr := &prog{text: text}
	if err := c.steps(p, pr, scratch, w); err != nil {
		return nil, err
	}
	return pr.textOf()
}

// steps applies one phase's steps to the program as it holds it, each on
// its own kind, converting where the kind changes.
func (c *Config) steps(p Phase, pr *prog, scratch string, w io.Writer) error {
	for _, s := range p.Steps {
		if s.Op == "sweep" {
			// A phase that sweeps in the middle of its own edit: the same
			// sweep, at the point its program ran one -- on the graph, its
			// collection.
			if pr.ed != nil {
				if err := c.collect(pr, w); err != nil {
					return err
				}
				continue
			}
			path := filepath.Join(scratch, "inner.c")
			if err := os.WriteFile(path, pr.text, 0o644); err != nil {
				return err
			}
			if _, err := sweep.Sweep(path, w, c.Sweep); err != nil {
				return err
			}
			out, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			pr.text = out
			continue
		}
		var op Op
		var gop GraphOp
		ok := false
		switch {
		case s.Graph && c.GraphLookup == nil:
			return fmt.Errorf("no graph step named %q: the pipeline has no graph steps", s.Op)
		case s.Graph:
			if gop, ok = c.GraphLookup(s.Op); !ok {
				return fmt.Errorf("no graph step named %q", s.Op)
			}
		default:
			if op, ok = c.Lookup(s.Op); !ok {
				return fmt.Errorf("no step named %q", s.Op)
			}
		}
		args := s.Args
		if c.Resolve != nil {
			var err error
			if args, err = c.Resolve(p, s.Args, scratch); err != nil {
				return err
			}
		}
		var undo func()
		if s.Declared {
			if c.Declared == nil {
				return fmt.Errorf("phase %d: a step reads what the phase declares, and nothing declares it", p.N)
			}
			var err error
			if undo, err = c.Declared(p.N); err != nil {
				return err
			}
		}
		var err error
		if s.Graph {
			var e *graph.Editor
			if e, err = c.editor(pr, scratch); err == nil {
				err = gop(e, args, w)
				pr.text = nil
			}
		} else {
			var t []byte
			if t, err = pr.textOf(); err == nil {
				var out []byte
				if out, err = op(t, args, w); err == nil {
					pr.tally()
					pr.ed = nil
					pr.text = out
				}
			}
		}
		if undo != nil {
			undo()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// keep writes the boundary after phase n into the snapshots, on a whole run,
// and into o.Keep, when it is set.
func (c *Config) keep(o *Options, n int, text []byte) error {
	if o.snap {
		if err := os.WriteFile(c.snapPath(n), text, 0o644); err != nil {
			return err
		}
	}
	if o.Keep == "" {
		return nil
	}
	if err := os.MkdirAll(o.Keep, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(o.Keep, fmt.Sprintf("q%03d.c", n)), text, 0o644)
}

// summary is a phase in one line: how many acts its steps reported, the lines
// its edits and then the sweep and canonical print took (a negative count is
// lines added), the lines left, and the time when it is a second or more;
// and, for a phase that touched the graph, what it spent converting and
// what its editor's log says (Conv).
//
//	54 acts, -1203 edited, -91 swept; 84059 lines, 9s
//	2 acts, -40 edited and collected; 84019 lines; graph: 1 collection 140ms, 9 acts, 312 ids superseded, 0 given
func summary(acts, before, edited, after int, d time.Duration, conv Conv) string {
	noun := "acts"
	if acts == 1 {
		noun = "act"
	}
	var s string
	if edited < 0 {
		// it ended on the graph: the edits and the collection, together
		s = fmt.Sprintf("%d %s, %s edited and collected; %d lines", acts, noun, took(before-after), after)
	} else {
		s = fmt.Sprintf("%d %s, %s edited, %s swept; %d lines", acts, noun, took(before-edited), took(edited-after), after)
	}
	if d >= time.Second {
		s += fmt.Sprintf(", %ds", int(d.Seconds()))
	}
	if g := conv.String(); g != "" {
		s += "; " + g
	}
	return s
}

// took is a count of lines removed, signed: -91 for 91 removed, +3 for 3 added.
func took(n int) string {
	if n == 0 {
		return "0"
	}
	if n > 0 {
		return fmt.Sprintf("-%d", n)
	}
	return fmt.Sprintf("+%d", -n)
}

// inOrderGC sets the collector for a run in order, and returns what puts it
// back.  A run holds one text and its parses at a time, so it can trade
// memory for the collector's time: GOGC 400 and a soft limit of 3 GiB, where
// the default is 100 and none.  Measured on phases 1-3 from q000, A/B/A/B:
// CPU 201 and 209 s at 100, 131 and 131 s at 400; peak resident 0.9 GB and
// 2.0-2.2 GB.  The parallel check (Check) runs a phase per core and is left
// as it is.  GOGC or GOMEMLIMIT set in the environment is left as set.
func inOrderGC() (restore func()) {
	if os.Getenv("GOGC") != "" || os.Getenv("GOMEMLIMIT") != "" {
		return func() {}
	}
	pct := debug.SetGCPercent(400)
	lim := debug.SetMemoryLimit(3 << 30)
	return func() {
		debug.SetGCPercent(pct)
		debug.SetMemoryLimit(lim)
	}
}
