package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arbace/go-whim/internal/sweep"
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
	}

	var text []byte
	if o.From > 0 {
		// Starting inside the pipeline: the input is a boundary, not the seed.
		// Nothing here proves it is the boundary it claims to be -- that is for
		// the caller, and only a build from phase 0 answers for the product.
		text = src
	}
	for _, p := range c.Plan {
		if o.To >= 0 && p.N > o.To {
			break
		}
		if p.N < o.From {
			continue
		}
		start := time.Now()
		fmt.Fprintf(o.W, "  phase %-6d %s\n", p.N, p.Name)
		if p.Seed {
			out, err := Seed(src, o.W)
			if err != nil {
				return nil, fmt.Errorf("phase %d: %w", p.N, err)
			}
			text = out
		}
		if text == nil {
			return nil, fmt.Errorf("build: phase %d runs before the input was seeded", p.N)
		}
		if p.NoSource {
			if err := c.keep(o, p.N, text); err != nil {
				return nil, err
			}
			continue
		}
		scratch, err := os.MkdirTemp("", fmt.Sprintf("%s%03d.", c.Name, p.N))
		if err != nil {
			return nil, err
		}
		out, err := c.RunPhase(p, text, scratch, o.W)
		switch {
		case err == nil:
			text = out
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
			fmt.Fprintf(o.W, "  REFUSED %-4d %s\n", p.N, why)
			o.Refused = append(o.Refused, fmt.Sprintf("%d: %s", p.N, why))
		default:
			os.RemoveAll(scratch)
			return nil, fmt.Errorf("phase %d (%s): %w", p.N, p.Name, err)
		}
		// The sweep and the canonical print --
		// also after a refusal, so the text handed on is canonical either way.
		finished, err := c.finish(text, scratch, o.W)
		os.RemoveAll(scratch)
		switch {
		case err == nil:
			text = finished
		case o.KeepGoing:
			fmt.Fprintf(o.W, "  REFUSED %-4d %v\n", p.N, err)
			o.Refused = append(o.Refused, fmt.Sprintf("%d: %v", p.N, err))
		default:
			return nil, fmt.Errorf("phase %d (%s): %w", p.N, p.Name, err)
		}
		if err := c.keep(o, p.N, text); err != nil {
			return nil, err
		}
		if d := time.Since(start); d > time.Second {
			fmt.Fprintf(o.W, "  phase %-6d %ds, %d lines\n", p.N, int(d.Seconds()),
				bytes.Count(text, []byte("\n")))
		}
	}
	// The work tree is left holding the source the pipeline leaves.
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return nil, err
	}
	if o.snap {
		if err := os.WriteFile(filepath.Join(c.SnapDir, "manifest"), []byte(digestOf(src)+"\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return text, nil
}

// RunPhase applies one phase's steps, with scratch as the directory the
// Config's Resolve may hand its arguments.
func (c *Config) RunPhase(p Phase, text []byte, scratch string, w io.Writer) ([]byte, error) {
	for _, s := range p.Steps {
		if s.Op == "sweep" {
			// A phase that sweeps in the middle of its own edit: the same
			// sweep, at the point its program ran one.
			path := filepath.Join(scratch, "inner.c")
			if err := os.WriteFile(path, text, 0o644); err != nil {
				return nil, err
			}
			if _, err := sweep.Sweep(path, w, c.Sweep); err != nil {
				return nil, err
			}
			out, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			text = out
			continue
		}
		op, ok := c.Lookup(s.Op)
		if !ok {
			return nil, fmt.Errorf("no step named %q", s.Op)
		}
		args := s.Args
		if c.Resolve != nil {
			var err error
			if args, err = c.Resolve(p, s.Args, scratch); err != nil {
				return nil, err
			}
		}
		var undo func()
		if s.Declared {
			if c.Declared == nil {
				return nil, fmt.Errorf("phase %d: a step reads what the phase declares, and nothing declares it", p.N)
			}
			var err error
			if undo, err = c.Declared(p.N); err != nil {
				return nil, err
			}
		}
		out, err := op(text, args, w)
		if undo != nil {
			undo()
		}
		if err != nil {
			return nil, err
		}
		text = out
	}
	return text, nil
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
