package p086a

// Whim phase 86a (formerly 164) -- no statement follows a jump.  See GOAL.md.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1c): the rule is general, and it is
// crefactor/graph's Editor.DeadStmt -- crefactor/xform's DeadStmt, which it
// replaced, asked of the nodes: it takes no knobs, and reports what the text
// reported.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim86a", Edit) }

// Edit deletes every run of statements after a jump, up to the next label.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	v := graph.NewVerbs("nodeadstmt", e, w)
	if len(args) > 0 {
		return fmt.Errorf("nodeadstmt: unexpected argument %q (it takes none)", args[0])
	}
	cut, held, err := e.DeadStmt()
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	v.Sayf("%d runs of statements after a jump go; %d held for a declaration", cut, held)
	return v.Done()
}
