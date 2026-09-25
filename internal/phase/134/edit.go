package p134

// Whim phase 134 -- the empty blocks fold.  See GOAL.md.
//
// Phase 132 left 33 blocks empty, beside those earlier phases left: 49 fold.
// An empty block guarded by a condition that only reads goes, as do an empty
// else and an empty else-if ending its chain; the sweep takes what the
// conditions computed and nothing reads any more.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// The rule is general, and it is crefactor/xform's EmptyBlocks, built with vim's
// knobs (internal/whim/xform.go); its counts are arguments in the plan.
func init() { phase.RegisterArgs("whim134", xform.EmptyBlocks(whim.Core).Edit()) }
