package p097

// Whim phase 97 (formerly 178) -- :g marks the lines match_lines finds.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim97", Edit) }

// Edit has ex_global()'s marking pass -- a search of each line of the range
// from column 0, the lines unchanged while it runs -- ask match_range()
// first, as ex_substitute() does (phase 96): one search a line, made on
// each line alone, and taken from the record by the pass.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the marking loop found by its
// form, the two locals put before it and its first statement written anew,
// by FRAG in one unit; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("globalfound", e, w)
	v.InFunction("ex_global", func(v *graph.Verbs) {
		loops := v.Find("(for (= lnum (-> eap line1)) (&& (<= lnum (-> eap line2)) (! got_int)) (pre++ lnum) (block (= match (call vim_regexec_multi (addr regmatch) curwin curbuf lnum (cast colnr_T 0) nullptr)) _*))")
		if len(loops) != 1 {
			v.Die("the marking pass matches the range's lines each alone first -- %d loops, expected 1", len(loops))
			return
		}
		v.Together(func(v *graph.Verbs) {
			v.BeforeEachC(loops, "linefound_T *found = eap->line2 > eap->line1 ? match_range(&regmatch, false, eap->line1, eap->line2) : nullptr;\nlinenr_T found_count = curbuf->b_ml.ml_line_count;\n", 1,
				"the marking pass matches the range's lines each alone first")
			v.ReplaceEachC([]*graph.Node{loops[0].Kids[4].Kids[1]}, "match = search_found(found, eap->line1, found_count, &regmatch, lnum, (colnr_T)0);\n", 1,
				"and takes each line's search from what that found")
		})
	})
	return v.Done()
}
