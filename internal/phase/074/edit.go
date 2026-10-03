package p074

// Whim phase 74 (formerly 149) -- the allocation-failure branches fold.  See GOAL.md.
//
// Every NULL test of a never-NULL allocation's result that follows it folds,
// the never-NULL functions found to a fixpoint from host_alloc(); labels no
// goto reaches any more go (internal/gen/FINDINGS.md, 9).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3g): the rule is general, and it is
// crefactor/graph's Editor.NeverNull -- crefactor/xform's NeverNull, which
// it replaced, asked of the nodes and the edges -- built with vim's knobs
// (internal/whim/xform.go); its floor is an argument in the plan.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

func init() { phase.RegisterGraph("whim74", Edit) }

// Edit folds the core's tests of a never-NULL function's result.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	v := graph.NewVerbs("allocnull", e, w)
	f, err := xform.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	if e.FirstInclude() == nil {
		v.Die("the core does not end where this step was told it does")
		return v.Done()
	}
	o := whim.NeverNull
	o.In = whim.GraphCore(e)
	r, err := e.NeverNull(o)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if r.Folded < f["--at-least"] {
		v.Die("%d NULL tests fold, fewer than the %d this step was told to expect", r.Folded, f["--at-least"])
		return v.Done()
	}
	for _, l := range r.Lines() {
		v.Say(l)
	}
	return v.Done()
}
