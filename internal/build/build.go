package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/steps"
	"github.com/arbace/go-whim/internal/whim"
)

// Options is what a build needs: where the input is, how far to go, and where
// the report goes (pipeline.Options).
type Options = pipeline.Options

// SnapDir is where a whole build from phase 0 keeps every boundary, qNNN.c,
// sealed by `manifest`, the digest of the input they came from.
const SnapDir = ".cache/boundaries"

// config is whim's pipeline, as the generic driver is told it: this plan, the
// op table, the work file whim-vim.c, the snapshots, vim's sweep, and the
// three arguments that are not literal.
var config = &pipeline.Config{
	Plan: Plan,
	Lookup: func(name string) (pipeline.Op, bool) {
		s, ok := steps.Lookup(name)
		return pipeline.Op(s), ok
	},
	Name:     "whim",
	WorkName: "whim-vim.c",
	SnapDir:  SnapDir,
	Sweep:    whim.Profile.Sweep,
	Resolve:  resolve,
	Declared: func(n int) (func(), error) {
		removed, err := declared(n)
		if err != nil {
			return nil, err
		}
		if err := os.Setenv("REMOVED", removed); err != nil {
			return nil, err
		}
		return func() { os.Unsetenv("REMOVED") }, nil
	},
}

// Seed is what phase 0 hands the pipeline: the input in canonical form
// (pipeline.Seed).
func Seed(src []byte, w io.Writer) ([]byte, error) { return pipeline.Seed(src, w) }

// Run applies the plan to the input and returns the source it leaves; the work
// tree is left holding it as whim-vim.c, whose compile line is FlagsFor the
// last phase run.
func Run(o *Options) ([]byte, error) { return config.Run(o) }

// Check proves the plan link by link from .cache/boundaries, jobs phases at a
// time, or runs it in order when there are no snapshots of this input.
func Check(o *Options, jobs int) ([]byte, error) { return config.Check(o, jobs) }

// Advance is one phase, as a function of the text it is handed.
func Advance(p Phase, text []byte, w io.Writer) ([]byte, error) {
	return config.Advance(p, text, w)
}

// RunPhase applies one phase's steps, with scratch as the directory its @state
// arguments name.  A few edits still write files there that their checks read
// when the pipeline had checks (448e9a8 and before); the build discards them.
func RunPhase(p Phase, text []byte, scratch string, w io.Writer) ([]byte, error) {
	return config.RunPhase(p, text, scratch, w)
}

// resolve turns the non-literal arguments into paths.
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
