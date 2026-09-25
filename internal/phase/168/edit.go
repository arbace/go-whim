package p168

// Whim phase 168 -- a goto whose label returns is that return.  See GOAL.md.

import (
	"github.com/arbace/go-whim/internal/crefactor/xform"
	"github.com/arbace/go-whim/internal/edit"
)

// The rule is general, and it is crefactor/xform's GotoReturn: it takes no knobs.
func init() { edit.RegisterArgs("whim168", xform.GotoReturn().Edit()) }
