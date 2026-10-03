package p060

// Whim phase 60 (formerly 132) -- nothing frees.  See GOAL.md.
//
// host_free() has had an empty body since phase 52, so vim_free() -- a NULL
// test around it -- does nothing observable.  All 273 calls to either in the
// core go; the two whose argument decrements a counter keep the decrement.
// vim_free() is then called by nothing and the collection takes it
// (internal/gen/FINDINGS.md, 9).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3g): the rule is general, and it is
// crefactor/graph's Editor.DropCalls -- crefactor/xform's DropCalls, which it
// replaced, asked of the nodes -- built with vim's knobs
// (internal/whim/xform.go); its counts are arguments in the plan.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

func init() { phase.RegisterGraph("whim60", Edit) }

// Edit drops the calls of the functions that do nothing from the core, and
// points the host's at its own.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	v := graph.NewVerbs("free", e, w)
	f, err := xform.Flags(v.Tag, args, "--calls", "--redirected")
	if err != nil {
		return err
	}
	k := whim.DropCalls
	k.In = whim.GraphCore(e)
	r, err := e.DropCalls(k)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if want, ok := f["--redirected"]; ok && r.Redirected != want {
		v.Die("the host makes %d calls to redirect, and this step was told %d", r.Redirected, want)
	}
	if want, ok := f["--calls"]; ok && r.Calls != want {
		v.Die("%d calls to %s() in the core, and this step was told %d", r.Calls, strings.Join(k.Funcs, "(), "), want)
	}
	for _, l := range r.Lines(k.Funcs) {
		v.Say(l)
	}
	return v.Done()
}
