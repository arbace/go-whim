package steps

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
)

// Phase 94's step on the graph (doc/GRAPH-MIGRATION.md, Step6):
// crefactor/graph's MemberOut, which replaced crefactor/xform's text step
// of the name -- the same sites taken and held, asked of the typed edges.
// Its one argument is a floor: `--at-least N` refuses when fewer than N
// arguments are taken.
func init() { graphOps["memberout"] = s6MemberOut }

func s6MemberOut(e *graph.Editor, args []string, w io.Writer) error {
	v := graph.NewVerbs("memberout", e, w)
	f, err := pipeline.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	r, err := e.MemberOut()
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	for _, l := range r.Lines() {
		fmt.Fprintf(w, "  %s: %s\n", v.Tag, l) // the text step's report, line for line
	}
	if min, ok := f["--at-least"]; ok && r.Taken < min {
		v.Die("%d sites taken, fewer than the %d asked for", r.Taken, min)
	}
	return v.Done()
}
