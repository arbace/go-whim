package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoFencs leaves p_fencs and p_tenc as a declaration and a row, so the rows
// can go next.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the else-if arm goes from its
// chain by FoldNever (history keeps the text version).
func NoFencs(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nofencs", e, w)
	v.FoldNever("(&& (== (cast (ptr (ptr char_u)) varp) (addr p_fencs)) enc_utf8)", 1,
		"the reset that restored a unicode 'fileencodings'")
	// readfile's choice between an empty list and a list went with readfile,
	// which dies at record 13 since ml_recover went at phase 1 (norecover, the
	// reform's D5)

	// did_set_encoding's conversion between 'termencoding' and 'encoding'
	// went with the two rows, dropped at phase 1 (optfront, the reform's D3).

	// Not asserted that p_fencs and p_tenc are read by nothing: this runs at
	// phase 2 (the reform's D7), where their rows are dropped already
	// (optfront) and readfile, still unswept, names p_fencs.
	return v.Done()
}
