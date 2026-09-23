package build

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arbace/go-whim/internal/steps"
	"github.com/arbace/go-whim/internal/sweep"
)

// Options is what a build needs: where the input is, how far to go, and where
// the report goes.
type Options struct {
	Src  string    // the pipeline's input, slim-vim.c -- or, with From, a boundary
	From int       // the first phase to run; its input is Src as it stands
	To   int       // the last phase to run; 0 or less means all of them
	W    io.Writer // the report
	Work string    // a directory to sweep in; a temporary one when empty

	// KeepGoing is for measuring, never for producing: a phase that refuses is
	// recorded, its text change dropped, and the run carries on with the text
	// as it was.  Everything after a dropped phase is answering a different
	// question, so what this gives is a LIST and not a product.
	KeepGoing bool
	Refused   []string
}

// Run applies the plan to the input and returns the source it leaves.
//
// The text lives in memory between steps.  It reaches the disk only where the
// sweep needs a path: the sweep compiles the tree speculatively to answer the
// enumerator question, which is the one thing in the pipeline that is not a
// function of the bytes alone.
func Run(o *Options) ([]byte, error) {
	if o.W == nil {
		o.W = io.Discard
	}
	src, err := os.ReadFile(o.Src)
	if err != nil {
		return nil, err
	}
	work := o.Work
	if work == "" {
		work, err = os.MkdirTemp("", "whim-build")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(work)
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(work, "whim-vim.c")
	mk := filepath.Join(work, "Makefile")
	if o.From > 0 {
		if err := MakefileFor(o.From, mk); err != nil {
			return nil, err
		}
	}

	var text []byte
	if o.From > 0 {
		// Starting inside the pipeline: the input is a boundary, not the seed.
		// Nothing here proves it is the boundary it claims to be -- that is for
		// the caller, and only a build from phase 0 answers for the product.
		text = src
	}
	for _, p := range Plan {
		if o.To > 0 && p.N > o.To {
			break
		}
		if p.N < o.From {
			continue
		}
		start := time.Now()
		fmt.Fprintf(o.W, "  phase %-6d %s\n", p.N, p.Name)
		if p.Seed {
			// Phase 0 seeds the input IN CANONICAL FORM, and every later phase
			// reads that form: one C23 spelling per construct, so an anchor
			// matches what it means rather than what the input happened to
			// write.  It was a --canonical flag while the 163 phases' anchors
			// were migrated to it; it is the pipeline now, and there is no
			// second spelling to fall back to.
			text = src
			out, err := steps.Lookup2("cemit")(text, nil, o.W)
			if err != nil {
				return nil, fmt.Errorf("phase %d: cemit: %w", p.N, err)
			}
			text = out
		}
		if text == nil {
			return nil, fmt.Errorf("build: phase %d runs before the input was seeded", p.N)
		}
		if p.Makefile != "" {
			if err := ApplyMakefile(p.Makefile, mk); err != nil {
				return nil, fmt.Errorf("phase %d: makefile: %w", p.N, err)
			}
		}
		if p.NoSource {
			continue
		}
		scratch, err := os.MkdirTemp("", fmt.Sprintf("whim%03d.", p.N))
		if err != nil {
			return nil, err
		}
		out, err := RunPhase(p, text, scratch, o.W)
		os.RemoveAll(scratch)
		switch {
		case err == nil:
			text = out
		case o.KeepGoing:
			fmt.Fprintf(o.W, "  REFUSED %-4d %v\n", p.N, err)
			o.Refused = append(o.Refused, fmt.Sprintf("%d: %v", p.N, err))
		default:
			return nil, fmt.Errorf("phase %d (%s): %w", p.N, p.Name, err)
		}
		if p.Sweep {
			if err := os.WriteFile(path, text, 0o644); err != nil {
				return nil, err
			}
			if _, err := sweep.Sweep(path, o.W); err != nil {
				return nil, fmt.Errorf("phase %d (%s): sweep: %w", p.N, p.Name, err)
			}
			if text, err = os.ReadFile(path); err != nil {
				return nil, err
			}
		}
		if d := time.Since(start); d > time.Second {
			fmt.Fprintf(o.W, "  phase %-6d %ds, %d lines\n", p.N, int(d.Seconds()),
				bytes.Count(text, []byte("\n")))
		}
	}
	// The work tree is left as the pipeline leaves it: the source and the
	// makefile it was produced with, which is what a phase program run by hand
	// (the baseline recorders, phase 0 and phase 83) is handed.
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return nil, err
	}
	return text, nil
}

// RunPhase applies one phase's steps, with scratch as the directory its @state
// arguments name.  internal/verify hands it the phase's real state directory,
// which is how a check gets the files the phase's edit writes for it.
func RunPhase(p Phase, text []byte, scratch string, w io.Writer) ([]byte, error) {
	for _, s := range p.Steps {
		if s.Op == "sweep" {
			// A phase that sweeps in the middle of its own edit: the same
			// sweep, at the point its program ran one.
			path := filepath.Join(scratch, "inner.c")
			if err := os.WriteFile(path, text, 0o644); err != nil {
				return nil, err
			}
			if _, err := sweep.Sweep(path, w); err != nil {
				return nil, err
			}
			out, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			text = out
			continue
		}
		op, ok := steps.Lookup(s.Op)
		if !ok {
			return nil, fmt.Errorf("no step named %q", s.Op)
		}
		args, err := resolve(p, s.Args, scratch)
		if err != nil {
			return nil, err
		}
		if s.Declared {
			removed, err := declared(p.N)
			if err != nil {
				return nil, err
			}
			if err := os.Setenv("REMOVED", removed); err != nil {
				return nil, err
			}
		}
		out, err := op(text, args, w)
		if s.Declared {
			os.Unsetenv("REMOVED")
		}
		if err != nil {
			return nil, err
		}
		text = out
	}
	return text, nil
}

// resolve turns the three non-literal arguments into paths.
func resolve(p Phase, args []string, scratch string) ([]string, error) {
	out := make([]string, 0, len(args))
	for _, a := range args {
		switch {
		case a == "@state":
			out = append(out, scratch)
		case strings.HasPrefix(a, "@state/"):
			out = append(out, filepath.Join(scratch, strings.TrimPrefix(a, "@state/")))
		case a == "@minmax":
			probe, err := steps.MinMax()
			if err != nil {
				return nil, err
			}
			path := filepath.Join(scratch, "minmax.txt")
			if err := os.WriteFile(path, probe, 0o644); err != nil {
				return nil, err
			}
			out = append(out, path)
		default:
			out = append(out, a)
		}
	}
	return out, nil
}

// declared is the phase's own declaration, as tools/declared.sh reads it: the
// tokens of phase/NNN/delta with its notes left out.
func declared(n int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("phase/%03d/delta", n))
	if err != nil {
		return "", err
	}
	var toks []string
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		toks = append(toks, strings.Fields(ln)...)
	}
	return strings.Join(toks, " "), nil
}
