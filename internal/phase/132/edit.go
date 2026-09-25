package p132

// Whim phase 132 -- nothing frees.  See GOAL.md.
//
// host_free() has had an empty body since phase 124, so vim_free() -- a NULL
// test around it -- does nothing observable.  All 273 calls to either in the
// core go; the two whose argument decrements a counter keep the decrement.
// vim_free() is then called by nothing and the sweep takes it
// (internal/gen/FINDINGS.md, 9).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/whim"
)

// The rule is general, and it is crefactor/xform's DropCalls, built with vim's
// knobs (internal/whim/xform.go); its counts are arguments in the plan.
func init() { edit.RegisterArgs("whim132", xform.DropCalls(whim.DropCalls).Edit()) }
