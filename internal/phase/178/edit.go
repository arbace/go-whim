package p178

// Whim phase 178 -- :g marks the lines match_lines finds.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim178", Edit) }

// Edit has ex_global()'s marking pass -- a search of each line of the range
// from column 0, the lines unchanged while it runs -- ask match_range()
// first, as ex_substitute() does (phase 177): one search a line, made on
// each line alone, and taken from the record by the pass.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("globalfound", text, w)
	e.InFunction("ex_global", func(e *edit.E) {
		e.Literal("        for (lnum = eap->line1; lnum <= eap->line2 && !got_int; ++lnum)\n        {\n            match = vim_regexec_multi(&regmatch, curwin, curbuf, lnum, (colnr_T)0, nullptr);\n",
			"        linefound_T *found = eap->line2 > eap->line1 ? match_range(&regmatch, false, eap->line1, eap->line2) : nullptr;\n"+
				"        linenr_T found_count = curbuf->b_ml.ml_line_count;\n"+
				"        for (lnum = eap->line1; lnum <= eap->line2 && !got_int; ++lnum)\n        {\n"+
				"            match = search_found(found, eap->line1, found_count, &regmatch, lnum, (colnr_T)0);\n", 1,
			"the marking pass matches the range's lines each alone first, and takes each line's search from what that found")
	})
	return e.Done()
}
