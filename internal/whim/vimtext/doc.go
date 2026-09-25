// Package vimtext is what the phases share that knows vim: the shapes of
// vim's own text that more than one phase anchors on -- the buffer list's
// walks, a special key's encoding, the Ex command table's rows, enum and
// first-two-letters index (phase 80), the swap file's process-id field
// (119), the memline's flags and data blocks (127) -- and the port helpers
// those phases wrote for themselves.
//
// It is the old internal/edit/shared.go's vim half, and residue.go, split off
// when the rest moved to crefactor/edit (doc/VIM-VS-GENERIC.md section 4,
// migration step 4).  The phases import it by its own name.
package vimtext
