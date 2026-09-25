// Package whim is what the pipeline knows about vim that the generic C
// machinery must be told rather than know (doc/VIM-VS-GENERIC.md): one value,
// Profile, that the vim side hands to the library.  It grows as the library
// is separated, one knob at a time; each knob names the file that used to
// hard-code it.
package whim

import "github.com/arbace/go-whim/internal/sweep"

// Profile is vim's.
var Profile = struct {
	// Sweep is what the sweep is told: vim's one entry point, and ml_recover,
	// whose presence makes a struct layout a swap-file format.  It was
	// hard-coded in internal/sweep/prune.go.
	Sweep sweep.Options
}{
	Sweep: sweep.Options{
		Roots:          []string{"main"},
		FreezeLayoutIf: []string{"ml_recover"},
	},
}
