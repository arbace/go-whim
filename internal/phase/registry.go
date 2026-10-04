// Package phase is the registry of the pipeline's phase programs, and its
// directory holds the phases, one package each (NNN/, N the phase's place in
// the plan), and their parts (NNN/x/: a program the phase runs that had a
// number of its own; doc/PHASES.md).
//
// A phase is a directory: GOAL.md says what it removes and why, and edit.go,
// where the phase has one, is its program, a GraphFunc: it edits
// crefactor/graph's graph through the editor it is handed (doc/GRAPH.md), and
// the plan runs it as a graph step.  The program registers itself here in an
// init() (RegisterGraph), so SOMETHING HAS TO IMPORT IT: cmd/whim/phases.go
// names every phase directory that holds Go.  internal/build looks a program
// up here by its name: whimN for phase N's, whimNx for its part x.
//
// The programs were `python3 - "$f" <<'PY'` blocks inside the phase programs:
// 225 of them across whim and zero, 34,284 lines, the larger half of the port,
// and they do not collapse.  Measured over all 225: 94 were bespoke drivers
// over cutil, 60 bespoke regex programs, and exactly ONE the simple "assert a
// literal occurs once and replace it" shape.  They became programs on the
// text first (Register, crefactor/edit's text verb set, which 864655e
// still has), then programs on the graph.
//
// A program reports each act on w as it succeeds and refuses with an error
// rather than leaving a half-done edit.  ORDER IS OUTPUT -- a phase program's
// log is read by a human comparing two phase commits.
//
// They are reached as `whim edit <phase> <file>` through this one registry
// rather than as subcommands of their own.
package phase

import (
	"io"
	"sort"

	"github.com/arbace/go-whim/crefactor/graph"
)

// Names returns every phase that has an edit here, sorted, for the usage
// message -- ranging a Go map yields a different order every run, and a usage
// message that reorders itself is a diff nobody wanted.
func Names() []string {
	Out := make([]string, 0, len(graphPhases))
	for k := range graphPhases {
		Out = append(Out, k)
	}
	sort.Strings(Out)
	return Out
}

// A GraphFunc is a phase's edit on the graph (crefactor/graph, doc/GRAPH.md):
// it edits the program through the editor it is handed -- deletions and
// replacements, their fall-out the editor's closure -- and reports to w as
// it goes.  The plan runs it as a graph step (`edit whimN` with Graph set),
// on the graph the pipeline holds.
type GraphFunc func(e *graph.Editor, w io.Writer, args []string) error

var graphPhases = map[string]GraphFunc{}

// RegisterGraph registers a phase's edit on the graph.
func RegisterGraph(name string, f GraphFunc) {
	if _, dup := graphPhases[name]; dup {
		panic("edit: " + name + " registered twice")
	}
	graphPhases[name] = f
}

// LookupGraph returns the graph edit for a phase, and whether there is one.
func LookupGraph(phase string) (GraphFunc, bool) {
	f, ok := graphPhases[phase]
	return f, ok
}
