package p164

// Whim phase 164 -- no statement follows a jump.  See GOAL.md.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
)

// The rule is general, and it is crefactor/xform's DeadStmt: it takes no knobs.
func init() { phase.RegisterArgs("whim164", xform.DeadStmt().Edit()) }
