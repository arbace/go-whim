package p168

// Whim phase 168 -- a goto whose label returns is that return.  See GOAL.md.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
)

// The rule is general, and it is crefactor/xform's GotoReturn: it takes no knobs.
func init() { phase.RegisterArgs("whim168", xform.GotoReturn().Edit()) }
