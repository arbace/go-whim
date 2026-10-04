package p062

// Whim phase 62 (formerly 134) -- the empty blocks fold.  See GOAL.md.
//
// Phase 60 left 33 blocks empty, beside those earlier phases left: 49 fold.
// An empty block guarded by a condition that only reads goes, as do an empty
// else and an empty else-if ending its chain; the collection takes what the
// conditions computed and nothing reads any more.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3g): the rule is general, and it is
// crefactor/graph's Editor.EmptyBlocks (B2d's, held to this phase byte for
// byte) -- crefactor/xform's EmptyBlocks, which it replaced -- in the core,
// with the text's own test of a condition's text (crefactor/edit's
// PureCond, which refuses `regname == '='` for its `=`); its floor is an
// argument in the plan.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

func init() { phase.RegisterGraph("whim62", Edit) }

// Edit folds the core's empty blocks, and the locals they leave only given
// values.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	v := graph.NewVerbs("empty", e, w)
	f, err := pipeline.Flags(v.Tag, args, "--at-least")
	if err != nil {
		return err
	}
	if e.FirstInclude() == nil {
		v.Die("the core does not end where this step was told it does")
		return v.Done()
	}
	st, err := e.EmptyBlocks(graph.EmptyOptions{In: whim.GraphCore(e), Cond: edit.PureCond})
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if st.Blocks < f["--at-least"] {
		v.Die("%d empty blocks fold, fewer than the %d this step was told to expect", st.Blocks, f["--at-least"])
		return v.Done()
	}
	v.Sayf("%d empty blocks fold away: an if whose condition only reads, an empty else, an empty else-if that ends its chain", st.Blocks)
	v.Sayf("%d locals only given values once their tests went, and go with their stores: %s", len(st.Locals), strings.Join(st.Locals, " "))
	return v.Done()
}
