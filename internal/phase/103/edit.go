package p103

// Whim phase 103 (formerly 184) -- more flags are bool.  See GOAL.md.

import (
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// Phase 102's rule, BoolRet on the file-scope objects, with what it left
// int taken too (Relax); on the graph (doc/GRAPH-MIGRATION.md, Step6).
func init() {
	k := whim.GraphBoolRet()
	k.Globals = true
	k.Relax = true
	phase.RegisterGraph("whim103", whim.BoolRetStep(k))
}
