package p102

// Whim phase 102 (formerly 183) -- a file-scope flag is bool.  See GOAL.md.

import (
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// Phase 87a's rule, BoolRet with vim's knobs, now on the file-scope objects
// too (Globals); on the graph (doc/GRAPH-MIGRATION.md, Step6).
func init() {
	k := whim.GraphBoolRet()
	k.Globals = true
	phase.RegisterGraph("whim102", whim.BoolRetStep(k))
}
