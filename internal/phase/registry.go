// Package phase is the registry of the pipeline's phase programs, and its
// directory holds the phases, one package each (NNN/).
//
// A phase is a directory: GOAL.md says what it removes and why, and edit.go,
// where the phase has one, is its program, written in crefactor/edit's verb
// set and internal/whim/vimtext's shared shapes.  The program registers itself
// here in an init(), so SOMETHING HAS TO IMPORT IT: cmd/whim/phases.go names
// every phase directory that holds Go.  internal/build looks a phase up here
// by its number.
//
// The programs were `python3 - "$f" <<'PY'` blocks inside the phase programs:
// 225 of them across whim and zero, 34,284 lines, the larger half of the port,
// and they do not collapse.  Measured over all 225: 94 were bespoke drivers
// over cutil, 60 bespoke regex programs, and exactly ONE the simple "assert a
// literal occurs once and replace it" shape.
//
// THE SHAPE IS THE CUTTERS', deliberately: func([]byte, io.Writer) ([]byte,
// error), rewrite in place, report each act on stdout as it succeeds, refuse
// with an error rather than writing a half-done tree.  ORDER IS OUTPUT -- a
// phase program's log is read by a human comparing two phase commits.
//
// They are reached as `whim edit <phase> <file>` through this one registry
// rather than as subcommands of their own.
package phase

import (
	"io"
	"sort"
)

// A Func is one phase's edit: it takes the tree and returns it rewritten.
type Func func(text []byte, w io.Writer) ([]byte, error)

// An ArgFunc is a Func that is handed the phase program's remaining arguments.
// ONE phase needs it -- whim80 writes the prefix table its check dispatches to
// a second path -- and it is a separate registration rather than a wider Func
// so that the other 33 ports keep a signature with nothing in it to ignore.
type ArgFunc func(text []byte, w io.Writer, args []string) ([]byte, error)

// phases is populated by each phase file's init(), NOT by a literal here, and
// that is a working arrangement rather than a style: two sessions port whim and
// zero in parallel, and a shared map literal is the one file they would both
// have to edit for every phase.  register() makes each phase's registration
// live beside its code, so the packages never collide.
var phases = map[string]ArgFunc{}

func Register(name string, f Func) {
	RegisterArgs(name, func(t []byte, w io.Writer, _ []string) ([]byte, error) { return f(t, w) })
}

func RegisterArgs(name string, f ArgFunc) {
	if _, dup := phases[name]; dup {
		panic("edit: " + name + " registered twice")
	}
	phases[name] = f
}

// Lookup returns the edit for a phase, and whether there is one.
func Lookup(phase string) (ArgFunc, bool) {
	f, ok := phases[phase]
	return f, ok
}

// Names returns every phase that has an edit here, sorted, for the usage
// message -- ranging a Go map yields a different order every run, and a usage
// message that reorders itself is a diff nobody wanted.
func Names() []string {
	Out := make([]string, 0, len(phases))
	for k := range phases {
		Out = append(Out, k)
	}
	sort.Strings(Out)
	return Out
}
