package whim

import "github.com/arbace/go-whim/crefactor/graph"

// GraphFallOut is what crefactor/graph's fall-out closure is told about
// vim: vim_strsave only allocates, so a store of its result to a field a
// cut deletes goes with the call; check_string_option and
// clear_string_option act on the option field they are handed and nothing
// else, so a call handed a deleted field's address goes with it.  And the
// pipeline's text cutters leave a block they empty -- `if (ready) {}` is in
// q024 -- so the closure is told to leave one too: its own rule takes it
// (doc/GRAPH.md, step 4).
var GraphFallOut = graph.FallOutOptions{
	Pure:      []string{"vim_strsave"},
	Through:   []string{"check_string_option", "clear_string_option"},
	KeepEmpty: true,
}

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
