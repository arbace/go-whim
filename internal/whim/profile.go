// Package whim is what the pipeline knows about vim that the generic C
// machinery, crefactor, must be told rather than know: Profile and the
// values beside it, which the vim side hands to the library.  Each knob names
// the file that used to hard-code it.
package whim

import "github.com/arbace/go-whim/crefactor/sweep"

// Profile is vim's.
var Profile = struct {
	// Sweep is what the sweep is told: vim's one entry point, and ml_recover,
	// whose presence makes a struct layout a swap-file format.  It was
	// hard-coded in crefactor/sweep/prune.go.
	Sweep sweep.Options
}{
	Sweep: sweep.Options{
		Roots:          []string{"main"},
		FreezeLayoutIf: []string{"ml_recover"},
	},
}
