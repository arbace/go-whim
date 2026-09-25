package p166

// Whim phase 166 -- a question returns bool.  See GOAL.md.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// The rule is general, and it is crefactor/xform's BoolRet, built with vim's
// knobs (internal/whim/xform.go); it takes no counts.
func init() { phase.RegisterArgs("whim166", xform.BoolRet(whim.BoolRet).Edit()) }
