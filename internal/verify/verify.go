// Package verify runs the pipeline's evidence: every phase's check, on exactly
// the tree and the state it was written against, and the declared delta of
// every stage.
//
// IT IS THE OTHER HALF OF internal/build.  A build applies the plan and stops;
// this applies the same plan and, at each phase, writes what that phase's
// program wrote for its check -- the source it was handed, that source compiled,
// its enumerator values -- then runs the check and the delta.  The two read one
// plan, so a phase cannot be built one way and checked another.
//
// THE STAGE IS PRESERVED BECAUSE IT IS SEMANTICS.  A shared stage runs every
// edit, ONE sweep, then every check on that one swept text, with the symbol
// snapshot of the text the FIRST edit was handed; an each stage sweeps after
// every edit and gives every check its own tree.  A check in the middle of a
// shared stage was written against what that arrangement hands it, so verifying
// it phase by phase would be asking it a different question.  What is gone is
// the memoize around this, not the arrangement.
package verify

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/sweep"
)

// Options is what a verification needs.
type Options struct {
	Src  string    // the pipeline's input, or a boundary when From is set
	From int       // the first phase to verify; phases before it are only built
	To   int       // the last phase to verify
	W    io.Writer // the report
	Root string    // where the work tree and the state directories go
}

// Run builds the pipeline and verifies the stages between From and To.
func Run(o Options) error {
	if o.W == nil {
		o.W = io.Discard
	}
	if o.To <= 0 {
		o.To = build.Plan[len(build.Plan)-1].N
	}
	root := o.Root
	if root == "" {
		var err error
		if root, err = os.MkdirTemp("", "whim-verify"); err != nil {
			return err
		}
		defer os.RemoveAll(root)
	} else if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	src := filepath.Join(work, "whim-vim.c")
	mk := filepath.Join(work, "Makefile")

	text, err := os.ReadFile(o.Src)
	if err != nil {
		return err
	}
	if o.From > 0 {
		// A boundary was handed in; the makefile it was produced with has to
		// be handed in with it, or every binary the checks build is wrong.
		if err := build.MakefileFor(o.From, mk); err != nil {
			return err
		}
	}

	var failed []int
	for i := 0; i < len(build.Plan); i++ {
		p := build.Plan[i]
		if p.N > o.To {
			break
		}
		if p.N < o.From {
			continue // Src is the boundary before From; see --from
		}
		// The stage this phase opens, and every phase in it.
		stage := []build.Phase{p}
		for j := i + 1; j < len(build.Plan) && build.Plan[j].Stage == p.Stage; j++ {
			stage = append(stage, build.Plan[j])
		}
		i += len(stage) - 1

		start := time.Now()
		fmt.Fprintf(o.W, "  stage %-6s %s\n", p.Stage, p.Name)
		out, err := runStage(stage, text, work, src, mk, root, o.Src, o.W)
		if err != nil {
			fmt.Fprintf(o.W, "  stage %-6s FAILED: %v\n", p.Stage, err)
			failed = append(failed, p.N)
		} else {
			fmt.Fprintf(o.W, "  stage %-6s ok, %ds\n", p.Stage, int(time.Since(start).Seconds()))
		}
		// A STAGE THAT REFUSED MAY STILL HAVE A TREE.  A check does not produce
		// the text; an edit does.  So runStage gives its text back whenever the
		// edits and sweeps ran, and only an edit, a sweep or the harness itself
		// leaves nothing -- and then there is no input for the stage after this
		// one and the run stops.  Assigning `out` unconditionally is what turned
		// one refusal at phase 79 into three more at 80, 81 and 82, each of them
		// `the input binary did not build: undefined reference to main` from a
		// state file that was the empty string.
		if out == nil {
			return fmt.Errorf("verify: stage %s left no tree, so the stages after it have no input: %w", p.Stage, err)
		}
		text = out
	}
	if len(failed) > 0 {
		return fmt.Errorf("verify: stage(s) starting at phase %v failed", failed)
	}
	return nil
}

// runStage runs one stage: its edits, its sweep or sweeps, its checks and its
// delta, and returns the text it leaves.
//
// IT RETURNS THE TEXT EVEN WHEN IT REFUSES, unless there is none.  A check and a
// delta read the tree and do not make it, so one that refuses is a fact about
// the stage and not a reason for the stage after it to be verified against
// nothing; they are collected here and reported together, which is the same
// habit a check has about its own assertions.  An edit, a sweep or the harness
// failing is the other case: there is no tree, nil comes back, and Run stops.
func runStage(stage []build.Phase, text []byte, work, src, mk, root, input string, w io.Writer) ([]byte, error) {
	each := stage[0].Each
	states := make([]string, len(stage))
	var stageSymbols string
	var refused []string

	for k, p := range stage {
		state := filepath.Join(root, "state", fmt.Sprintf("q%d", p.N))
		states[k] = state
		os.RemoveAll(state)
		if err := os.MkdirAll(state, 0o755); err != nil {
			return nil, err
		}
		if p.Seed {
			// build.Seed, and not a read of the file: phase 0 canonicalises,
			// and a verification that skipped that would run every phase after
			// it on a spelling the pipeline never produces.
			out, err := build.Seed(mustRead(input), w)
			if err != nil {
				return nil, fmt.Errorf("phase %d: %w", p.N, err)
			}
			text = out
		}
		if p.Makefile != "" {
			if err := build.ApplyMakefile(p.Makefile, mk); err != nil {
				return nil, err
			}
		}
		// What every check is handed beside its tree.
		if err := os.WriteFile(filepath.Join(state, "input-lines"),
			[]byte(fmt.Sprintf("%d\n", countLines(text))), 0o644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(src, text, 0o644); err != nil {
			return nil, err
		}
		// THE SYMBOL SNAPSHOT IS THE STAGE'S START, which for an each stage is
		// every phase's own input and for a shared stage is the text its first
		// edit was handed -- the one text in a stage that is certain to compile.
		if each || k == 0 {
			if err := Symbols(src, filepath.Join(state, "symbols")); err != nil {
				return nil, fmt.Errorf("phase %d: symbols: %w", p.N, err)
			}
			stageSymbols = filepath.Join(state, "symbols")
		} else if err := copyDir(stageSymbols, filepath.Join(state, "symbols")); err != nil {
			return nil, err
		}
		if err := writeState(p, text, state, mk, w); err != nil {
			return nil, fmt.Errorf("phase %d: state: %w", p.N, err)
		}

		scratch := state // the edits write their check's files straight into it
		out, err := build.RunPhase(p, text, scratch, w)
		if err != nil {
			return nil, err
		}
		text = out
		if p.Sweep {
			if err := os.WriteFile(src, text, 0o644); err != nil {
				return nil, err
			}
			if _, err := sweep.Sweep(src, w); err != nil {
				return nil, fmt.Errorf("phase %d: sweep: %w", p.N, err)
			}
			text = mustRead(src)
		}
		if each {
			if err := os.WriteFile(src, text, 0o644); err != nil {
				return nil, err
			}
			if err := runCheck(p, work, state, w); err != nil {
				refused = append(refused, err.Error())
			}
			if err := delta(p.N, work, src, mk, w); err != nil {
				refused = append(refused, err.Error())
			}
		}
	}
	if each {
		return text, joined(refused)
	}
	// A shared stage: one swept text, every check on it, one delta at the end.
	if err := os.WriteFile(src, text, 0o644); err != nil {
		return nil, err
	}
	for k, p := range stage {
		if err := runCheck(p, work, states[k], w); err != nil {
			refused = append(refused, err.Error())
		}
	}
	if err := delta(stage[len(stage)-1].N, work, src, mk, w); err != nil {
		refused = append(refused, err.Error())
	}
	return text, joined(refused)
}

// joined is the stage's refusals as one error, or nil when there were none.
func joined(refused []string) error {
	if len(refused) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(refused, "; "))
}

// runCheck runs the phase's check, which is internal/check's -- the same
// function the phase's check.sh dispatched to.
func runCheck(p build.Phase, work, state string, w io.Writer) error {
	name := fmt.Sprintf("whim%d", p.N)
	f, ok := check.Lookup(name)
	if !ok {
		fmt.Fprintf(w, "  check %-6d %s -- no check of its own\n", p.N, p.Name)
		return nil
	}
	fmt.Fprintf(w, "  check %-6d %s\n", p.N, p.Name)
	os.Remove(filepath.Join(work, "whim-vim")) // the delta builds it, not a check
	if err := f(w, []string{work, state}); err != nil {
		return fmt.Errorf("phase %d check: %w", p.N, err)
	}
	return nil
}

// delta is the declared delta at a phase, measured on the binary its tree
// builds -- Delta, which hands a phase from build.CoreFrom on to the core's
// checker.
func delta(n int, work, src, mk string, w io.Writer) error {
	bin := filepath.Join(work, "whim-vim")
	if _, err := os.Stat(bin); err != nil {
		if err := buildBinary(src, mk, bin, w); err != nil {
			return err
		}
	}
	if err := Delta(bin, src, n, w); err != nil {
		return fmt.Errorf("phase %d delta: %w", n, err)
	}
	return nil
}

// writeState writes what the phase's program built for its check.
func writeState(p build.Phase, text []byte, state, mk string, w io.Writer) error {
	if p.OldSource {
		if err := os.WriteFile(filepath.Join(state, "old.c"), text, 0o644); err != nil {
			return err
		}
	}
	if p.OldDir {
		d := filepath.Join(state, "old")
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(d, "whim-vim.c"), text, 0o644); err != nil {
			return err
		}
	}
	if p.EnumVals {
		f := filepath.Join(state, "old.c")
		cmd := exec.Command("sh", "tools/enumvals.sh", f, filepath.Join(state, "enums-before"))
		cmd.Stdout, cmd.Stderr = w, w
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	if p.OldBinary != "" {
		oldC := filepath.Join(state, "old.c")
		if _, err := os.Stat(oldC); err != nil {
			if err := os.WriteFile(oldC, text, 0o644); err != nil {
				return err
			}
		}
		return buildOld(p, oldC, filepath.Join(state, "old"), mk)
	}
	return nil
}

// buildOld builds the source the phase was handed, with the flags its program
// used: the boundary's, or the fixed line two phases wrote out.
func buildOld(p build.Phase, src, out, mk string) error {
	var args []string
	switch p.OldBinary {
	case "fixed":
		args = []string{"-O0", "-static", "-s", "-w"}
	default:
		c, l, err := build.Flags(mk)
		if err != nil {
			return err
		}
		args = append(append([]string{}, c...), l...)
	}
	cmd := exec.Command("gcc", append(args, "-o", out, src)...)
	if p.OldBinary == "epoch" {
		cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("the input binary did not build: %v: %s", err, out)
	}
	return nil
}

// buildBinary builds the tree's own binary, with the tree's own makefile.
func buildBinary(src, mk, out string, w io.Writer) error {
	c, l, err := build.Flags(mk)
	if err != nil {
		return err
	}
	cmd := exec.Command("gcc", append(append(append([]string{}, c...), l...), "-o", out, src)...)
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("the binary the delta measures did not build: %v: %s", err, o)
	}
	return nil
}

// copyDir copies a directory of small files, which is what a symbol snapshot is.
func copyDir(from, to string) error {
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	es, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, e := range es {
		b, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

func countLines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}
