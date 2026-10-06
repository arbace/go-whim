// Package vimgraph is what crefactor/graph's fall-out closure is told about
// vim, apart from internal/whim, so that a program that needs only it --
// the Joker guest's editor (guest/joker/ed) -- links crefactor/graph and
// nothing of the translators internal/whim brings.
package vimgraph

import "github.com/arbace/go-whim/crefactor/graph"

// FallOut is what crefactor/graph's fall-out closure is told about vim:
// vim_strsave only allocates, so a store of its result to a field a cut
// deletes goes with the call; check_string_option and clear_string_option
// act on the option field they are handed and nothing else, so a call
// handed a deleted field's address goes with it.  And the pipeline's text
// cutters leave a block they empty -- `if (ready) {}` is in q024 -- so the
// closure is told to leave one too: its own rule takes it (doc/GRAPH.md,
// step 4).
var FallOut = graph.FallOutOptions{
	Pure:      []string{"vim_strsave"},
	Through:   []string{"check_string_option", "clear_string_option"},
	KeepEmpty: true,
}
