package whim

import (
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/whim/vimgraph"
)

// GraphFallOut is what crefactor/graph's fall-out closure is told about
// vim (internal/whim/vimgraph's FallOut, which says why).
var GraphFallOut = vimgraph.FallOut

// GraphCollect is what crefactor/graph's collection is told: the sweep's
// roots and its guard (Profile.Sweep).
func GraphCollect() graph.CollectOptions {
	return graph.CollectOptions{Roots: Profile.Sweep.Roots, FreezeLayoutIf: Profile.Sweep.FreezeLayoutIf}
}

// GraphCore is Core on the graph: the forms above the first include form,
// as e holds them now (every form when there is none).
func GraphCore(e *graph.Editor) func(*graph.Node) bool {
	core := map[*graph.Node]bool{}
	for _, f := range e.Core() {
		core[f] = true
	}
	return func(f *graph.Node) bool { return core[f] }
}
