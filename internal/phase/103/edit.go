package p103

// Whim phase 103 (formerly 184) -- more flags are bool.  See GOAL.md.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// Phase 102's rule, BoolRet on the file-scope objects, with what it left
// int taken too (Relax).
func init() {
	k := whim.BoolRet
	k.Globals = true
	k.Relax = true
	phase.RegisterArgs("whim103", xform.BoolRet(k).Edit())
}
