package steps

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/whim"
)

// structscalar, phase 100's second step, on the graph (doc/GRAPH-MIGRATION.md,
// Step6): crefactor/graph's StructScalars on the core's functions, which
// replaced crefactor/xform's text step of that name.  Its one argument is a
// floor -- `--at-least N` refuses when fewer than N structs are taken -- and
// it reports what the text step reported.
func init() { graphOps["structscalar"] = s6StructScalar }

func s6StructScalar(e *graph.Editor, args []string, w io.Writer) error {
	v := graph.NewVerbs("structscalar", e, w)
	f, err := pipeline.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	n, err := e.StructScalars(whim.GraphCore(e))
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	fmt.Fprintf(w, "  %s: %d struct locals are their members' locals\n", v.Tag, n)
	if min, ok := f["--at-least"]; ok && n < min {
		v.Die("%d structs taken, fewer than the %d asked for", n, min)
	}
	return v.Done()
}
