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
	Src  string // the pipeline's input, slim-vim.c -- or, with From, a boundary
	From int    // the first phase to run; its input is Src as it stands
	// To is the last phase to run, and a NEGATIVE value means all of them.
	// It was `0 or less`, which made `--to 0` -- seed and stop -- run the whole
	// pipeline instead, and a caller measuring the seed got the product back
	// with no sign anything was wrong.
	To   int
	W    io.Writer // the report
	Work string    // a directory to sweep in; a temporary one when empty

	// KeepGoing is for measuring, never for producing: a phase that refuses is
	// recorded, its text change dropped, and the run carries on with the text
	// as it was.  Everything after a dropped phase is answering a different
	// question, so what this gives is a LIST and not a product.
	KeepGoing bool

	// Keep, when set, is a directory the text after every phase is written into
	// as qNNN.c -- after the phase's sweep and canonical print, so each
	// file is the boundary that phase hands on.  For measuring every boundary
	// from one run (whim measure); it writes nothing else.
	Keep    string
	Refused []string

	snap bool // a whole ordinary run from phase 0: write .cache/boundaries
}

// Seed is what phase 0 hands the pipeline: the input IN CANONICAL FORM.  Every
// later phase reads that form -- one C23 spelling per construct, so an anchor
// matches what it means rather than what the input happened to write.  It was a
// --canonical flag while the 163 phases' anchors were migrated to it; it is the
// pipeline now, and there is no second spelling to fall back to.
//
// IT IS A FUNCTION SO THAT THERE IS ONE OF IT: when a second caller seeded by
// reading the file, it ran every phase after 0 on the residue spelling, a
// pipeline the build does not run.
func Seed(src []byte, w io.Writer) ([]byte, error) {
	if w == nil {
		w = io.Discard
	}
	out, err := steps.Lookup2("cemit")(src, nil, w)
	if err != nil {
		return nil, fmt.Errorf("cemit: %w", err)
	}
	return out, nil
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
	// Only a whole, ordinary run from phase 0 writes the snapshots, and it
	// unseals the set first and seals it last, so a run that stops part way
	// leaves no set that claims to be whole.
	o.snap = o.From == 0 && o.To < 0 && !o.KeepGoing
	if o.snap {
		if err := os.MkdirAll(SnapDir, 0o755); err != nil {
			return nil, err
		}
		os.Remove(filepath.Join(SnapDir, "manifest"))
	}

	var text []byte
	if o.From > 0 {
		// Starting inside the pipeline: the input is a boundary, not the seed.
		// Nothing here proves it is the boundary it claims to be -- that is for
		// the caller, and only a build from phase 0 answers for the product.
		text = src
	}
	for _, p := range Plan {
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
			if err := o.keep(p.N, text); err != nil {
				return nil, err
			}
			continue
		}
		scratch, err := os.MkdirTemp("", fmt.Sprintf("whim%03d.", p.N))
		if err != nil {
			return nil, err
		}
		out, err := RunPhase(p, text, scratch, o.W)
		switch {
		case err == nil:
			text = out
		case o.KeepGoing:
			// A PHASE THAT HAS ALREADY SAID WHY RETURNS AN EMPTY ERROR.  Several
			// edits print their refusal to the report and return `fmt.Errorf("")`,
			// the same arrangement as harness.ErrReported -- so repeating `%v`
			// here wrote `REFUSED 127` with nothing after it, and a reader of the
			// log could not tell a silent refusal from a missing message.
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
		finished, err := finish(p, text, scratch, o.W)
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
		if err := o.keep(p.N, text); err != nil {
			return nil, err
		}
		if d := time.Since(start); d > time.Second {
			fmt.Fprintf(o.W, "  phase %-6d %ds, %d lines\n", p.N, int(d.Seconds()),
				bytes.Count(text, []byte("\n")))
		}
	}
	// The work tree is left holding the source the pipeline leaves; its compile
	// line is FlagsFor the last phase run.
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return nil, err
	}
	if o.snap {
		if err := os.WriteFile(filepath.Join(SnapDir, "manifest"), []byte(digestOf(src)+"\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return text, nil
}

// RunPhase applies one phase's steps, with scratch as the directory its @state
// arguments name.  A few edits still write files there that their checks read
// when the pipeline had checks (448e9a8 and before); the build discards them.
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

// declared is the tokens inside internal/phase/NNN/delta.md's FENCED BLOCK, with the prose
// around it left out.  Only phase 80 has one now: its edit cuts exactly the
// command rows it lists, so the file is that edit's input and not a test's.
func declared(n int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("internal/phase/%03d/delta.md", n))
	if err != nil {
		return "", err
	}
	var toks []string
	fence := false
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			fence = !fence
			continue
		}
		if !fence {
			continue
		}
		toks = append(toks, strings.Fields(ln)...)
	}
	return strings.Join(toks, " "), nil
}

// keep writes the boundary after phase n into o.Keep, when it is set.
func (o *Options) keep(n int, text []byte) error {
	if o.snap {
		if err := os.WriteFile(snapPath(n), text, 0o644); err != nil {
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
