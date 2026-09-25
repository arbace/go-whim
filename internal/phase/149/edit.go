package p149

// Whim phase 149 -- the allocation-failure branches fold.  See GOAL.md.
//
// Every NULL test of a never-NULL allocation's result that follows it folds,
// the never-NULL functions found to a fixpoint from host_alloc(); labels only
// the folded branches jumped to go (internal/gen/FINDINGS.md, 9).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/whim"
)

// The rule is general, and it is crefactor/xform's NeverNull, built with vim's
// knobs (internal/whim/xform.go); its counts are arguments in the plan.
func init() { edit.RegisterArgs("whim149", xform.NeverNull(whim.NeverNull).Edit()) }
