package p087a

// Whim phase 87a (formerly 166) -- a question returns bool.  See GOAL.md.

import (
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// The rule is general, and it is crefactor/graph's BoolRet, built with vim's
// knobs (internal/whim/s6boolret.go); it takes no counts.  ON THE GRAPH
// (doc/GRAPH-MIGRATION.md, Step6): crefactor/xform's text version asked of
// the forms and their edges, history keeps it.
func init() { phase.RegisterGraph("whim87a", whim.BoolRetStep(whim.GraphBoolRet())) }
