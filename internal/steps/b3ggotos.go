package steps

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/whim"
)

// The goto steps, phases 89-92, on the graph (doc/GRAPH-MIGRATION.md, B3g):
// crefactor/graph's GotoTail, GotoBreak, GotoLoop and GotoBlock, which
// replaced crefactor/xform's text steps of those names.  Each takes one
// argument, a floor -- `--at-least N` refuses when fewer than N gotos go --
// and reports what the text step reported.
func init() {
	graphOps["gototail"] = b3gGotoTail
	graphOps["gotobreak"] = b3gGotoBreak
	graphOps["gotoloop"] = b3gGotoLoop
	graphOps["gotoblock"] = b3gGotoBlock
}

func b3gGotoTail(e *graph.Editor, args []string, w io.Writer) error {
	v := graph.NewVerbs("gototail", e, w)
	f, err := pipeline.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	r, err := e.GotoTail(whim.GotoTail)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if r.Taken < f["--at-least"] {
		v.Die("%d gotos take their tail, fewer than the %d this step was told to expect", r.Taken, f["--at-least"])
		return v.Done()
	}
	for _, l := range r.Lines() {
		v.Say(l)
	}
	return v.Done()
}

func b3gGotoBreak(e *graph.Editor, args []string, w io.Writer) error {
	v := graph.NewVerbs("gotobreak", e, w)
	f, err := pipeline.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	r, err := e.GotoBreak()
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	v.Say(r.Line())
	if r.Breaks+r.Falls < f["--at-least"] {
		v.Die("%d gotos go, fewer than the %d this step was told to expect", r.Breaks+r.Falls, f["--at-least"])
	}
	return v.Done()
}

func b3gGotoLoop(e *graph.Editor, args []string, w io.Writer) error {
	v := graph.NewVerbs("gotoloop", e, w)
	f, err := pipeline.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	r, err := e.GotoLoop()
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	v.Say(r.Line())
	if r.Gotos < f["--at-least"] {
		v.Die("%d gotos become loops, fewer than the %d this step was told to expect", r.Gotos, f["--at-least"])
	}
	return v.Done()
}

func b3gGotoBlock(e *graph.Editor, args []string, w io.Writer) error {
	v := graph.NewVerbs("gotoblock", e, w)
	f, err := pipeline.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	r, err := e.GotoBlock()
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	v.Say(r.Line())
	if r.Gotos < f["--at-least"] {
		v.Die("%d gotos become a break, fewer than the %d this step was told to expect", r.Gotos, f["--at-least"])
	}
	return v.Done()
}
