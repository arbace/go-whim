package whim

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// GraphBoolRet is phase 87a's knobs on the graph (crefactor/graph's
// Editor.BoolRet; BoolRet's, xform's, before): vim's truth constants, TRUE
// and OK beside true, FALSE and FAIL beside false; main, whose int is the
// process's; and the collection's layout guard, which says which members a
// positional initialiser fills.  Phases 102 and 103 add Globals and Relax.
func GraphBoolRet() graph.BoolRetOptions {
	return graph.BoolRetOptions{
		True:   []string{"TRUE", "OK"},
		False:  []string{"FALSE", "FAIL"},
		Keep:   []string{"main"},
		Layout: GraphCollect(),
	}
}

// BoolRetStep is the graph program of 87a, 102 and 103: BoolRet with opt,
// its report the text step's, under its tag.
func BoolRetStep(opt graph.BoolRetOptions) func(*graph.Editor, io.Writer, []string) error {
	return func(e *graph.Editor, w io.Writer, args []string) error {
		v := graph.NewVerbs("boolret", e, w)
		if len(args) > 0 {
			return fmt.Errorf("boolret: takes no arguments, given %q", args)
		}
		r, err := e.BoolRet(opt)
		if err != nil {
			v.Die("%v", err)
			return v.Done()
		}
		for _, l := range r.Lines(opt) {
			v.Say(l)
		}
		return v.Done()
	}
}
