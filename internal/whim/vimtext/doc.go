// Package vimtext is what the phases share that knows vim: the shapes of
// vim's own text that more than one phase anchors on -- the buffer list's
// walks, a special key's encoding, the Ex command table's rows, enum and
// first-two-letters index (phase 80), the swap file's process-id field
// (119), the memline's flags and data blocks (127) -- and the port helpers
// those phases wrote for themselves.
//
// It is internal/edit/shared.go's vim half, and residue.go, split off when
// the rest of internal/edit moved to crefactor/text
// (doc/VIM-VS-GENERIC.md section 4, migration step 4).  internal/edit
// forwards every name, so the phases read as they did.
package vimtext
