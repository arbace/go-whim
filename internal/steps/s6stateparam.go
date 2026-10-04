package steps

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/whim"
)

// Phase 95's step on the graph (doc/GRAPH-MIGRATION.md, Step6):
// crefactor/graph's StateParam, which replaced crefactor/xform's text step
// of the name.  `--at-least N` refuses when fewer than N functions take the
// state; the report is the text step's line.
func init() { graphOps["stateparam"] = s6StateParam }

func s6StateParam(e *graph.Editor, args []string, w io.Writer) error {
	v := graph.NewVerbs("stateparam", e, w)
	f, err := xform.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	r, err := e.StateParam(whim.StateParam)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if min, ok := f["--at-least"]; ok && len(r.Takers) < min {
		v.Die("%d functions take the state, fewer than the %d asked for", len(r.Takers), min)
		return v.Done()
	}
	v.Say(r.Line(whim.StateParam.Type))
	return v.Done()
}
