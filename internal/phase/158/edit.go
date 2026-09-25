package p158

// Whim phase 158 -- a highlight's terminal font is read only from a colour entry.  See GOAL.md.
//
// screen_start_highlight() tested a colour entry's font before t_colors, so
// on a terminal without colours it read it out of a term entry's start
// pointer.  The test asks t_colors first.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim158", Edit) }

// W158Font is the test phase 158 finds and the one it writes.
const (
	W158Font  = "        if (aep->ae_u.cterm.font > 0 && aep->ae_u.cterm.font < 12)\n"
	W158Guard = "        if (t_colors > 1 && aep->ae_u.cterm.font > 0 && aep->ae_u.cterm.font < 12)\n"
)

// Whim158 reads a highlight's terminal font only from a colour entry.
//
// attrentry_T holds either a term entry (the start and stop strings) or a
// cterm entry (the colours and a font) in one union, and which one is
// t_colors > 1.  screen_start_highlight() tests cterm.font before it looks
// at t_colors, so on a terminal without colours it reads the font Out of a
// term entry: the top two bytes of term.start, which no x86-64 user-space
// pointer has set.  The one union member read where its discriminant does not
// say it holds (internal/ccx's Unions).  The test now asks t_colors first.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("font", text, w)
	e.Literal(W158Font, W158Guard, 1, "screen_start_highlight() reads cterm.font only when t_colors > 1 says the entry is a cterm entry")
	return e.Done()
}
