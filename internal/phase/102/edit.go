package p102

// Whim phase 102 (formerly 183) -- a file-scope flag is bool.  See GOAL.md.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// Phase 87a's rule, BoolRet with vim's knobs, now on the file-scope objects
// too (Globals).
func init() {
	k := whim.BoolRet
	k.Globals = true
	phase.RegisterArgs("whim102", xform.BoolRet(k).Edit())
}
