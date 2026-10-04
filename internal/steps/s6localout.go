package steps

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/xform"
)

// Phase 100's first step on the graph (doc/GRAPH-MIGRATION.md, Step6):
// crefactor/graph's LocalOut, which replaced crefactor/xform's text step of
// that name -- the core's out-parameters made values in and out.  Its one
// argument is a floor: `--at-least N` refuses when fewer than N parameters
// are taken.  It reports what the text step reported.
func init() {
	graphOps["localout"] = s6LocalOut
}

func s6LocalOut(e *graph.Editor, args []string, w io.Writer) error {
	const tag = "localout"
	f, err := xform.Flags(tag, args, "--at-least")
	if err != nil {
		return err
	}
	st, err := e.LocalOut()
	if err != nil {
		return fmt.Errorf("  %-12s %v", tag, err)
	}
	fmt.Fprintf(w, "  %s: %d out-parameters are values in and out, of %d functions; %d take no value in\n", tag, st.Params, st.Funcs, st.DeadIn)
	if min, ok := f["--at-least"]; ok && st.Params < min {
		return fmt.Errorf("  %-12s %d parameters taken, fewer than the %d asked for", tag, st.Params, min)
	}
	return nil
}
