// Package pipeline is the generic driver of a C refactoring pipeline: a plan
// of phases, each a sequence of named steps followed by the sweep and the
// canonical print, applied to one translation unit in memory.
//
// It knows no code base.  What it is told -- the plan, the op table, the name
// the text is swept under, where the snapshots go, how a non-literal argument
// is resolved and what a step marked Declared reads -- is a Config, built by
// the caller (for whim, internal/build).
//
// A phase is a function of the text it is handed, p_N = f_N(p_{N-1}), so a
// whole run from phase 0 can keep every boundary (the snapshots), and Check
// can prove the plan link by link, every phase at once.
package pipeline

import (
	"io"

	"github.com/arbace/go-whim/crefactor/sweep"
)

// An Op is what a step calls: the text in, the text out, a report to w.
type Op func(text []byte, args []string, w io.Writer) ([]byte, error)

// A Step is one call: an op in the Config's table and its arguments.  The op
// "sweep" is not in any table: it is the sweep, run where the step stands.
type Step struct {
	Op   string
	Args []string
	// Declared says the step reads what its phase declares: Config.Declared
	// prepares it before the call and undoes it after.
	Declared bool
}

// A Phase is what one phase of the pipeline does to the source.
type Phase struct {
	N        int
	Name     string
	Seed     bool // the tree is the input, printed canonically
	NoSource bool // the phase changes no source at all
	Steps    []Step
}

// A Plan is the pipeline, phase by phase, in order.
type Plan []Phase

// Config is everything the driver is told about the code base it runs on.
type Config struct {
	Plan Plan

	// Lookup is the op table: the op of that name, and whether there is one.
	Lookup func(name string) (Op, bool)

	// Name prefixes the temporary directories a run makes (Name-build for the
	// work tree, NameNNN. for a phase's scratch).
	Name string

	// WorkName is the file name the text is written under to be swept, and
	// the file a run leaves in its work tree.
	WorkName string

	// SnapDir is where a whole run from phase 0 keeps every boundary, as
	// qNNN.c, sealed by `manifest`: the digest of the input.  Empty: no
	// snapshots, and Check runs the pipeline in order.
	SnapDir string

	// Sweep is what every sweep is told: the roots, and what freezes layout.
	Sweep sweep.Options

	// Resolve turns a step's arguments into the ones its op is called with,
	// given the phase's scratch directory.  Nil: every argument is literal.
	Resolve func(p Phase, args []string, scratch string) ([]string, error)

	// Declared prepares what a step marked Declared reads, for phase n, and
	// returns what undoes it.  Nil: a Declared step is refused.
	Declared func(n int) (undo func(), err error)
}

// Options is what one run needs: where the input is, how far to go, and where
// the report goes.
type Options struct {
	Src  string // the pipeline's input -- or, with From, a boundary
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

	snap bool // a whole ordinary run from phase 0: write the snapshots
}
