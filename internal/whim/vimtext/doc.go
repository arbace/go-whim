// Package vimtext is what more than one phase shares that knows vim: the
// shapes of vim's own text that several phases anchor on -- the buffer list's
// walks, a special key's encoding, the Ex command table's residue check, the
// include forms and the core's block of ordinary declarations as part 49a, part
// 49b and phase 49 read them on the graph (graph.go) -- and the
// port helpers phases 54-56 share.  A shape one phase alone uses is in that
// phase's own shapes.go: phase 26's command table, 55's
// memline flags and data blocks, 56's local declarations.
//
// It is the old internal/edit/shared.go's vim half, and residue.go, split off
// when the rest moved to crefactor/edit.  The phases import it by its own name.
package vimtext
