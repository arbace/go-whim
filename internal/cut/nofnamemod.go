package cut

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

var modifyFname = regexp.MustCompile(`\bmodify_fname\b`)

// NoFnameMod makes % a file name and nothing more.
//
// eval_vars' modifier arm goes, then the stores to the two locals that only
// fed it.  The collection takes a local nothing names but not a store to
// one, so the stores go here and the declarations, like modify_fname
// itself, are the collection's.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the arm a DropIf, the stores
// Cuts by form, in eval_vars (history keeps the text version).
func NoFnameMod(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nofnamemod", e, w)
	q := graph.NewVerbs("nofnamemod", e, io.Discard)
	q.InFunction("eval_vars", func(q *graph.Verbs) {
		q.DropIf("(! skip_mod)", 1, "eval_vars' modifier arm")
	})
	if q.Failed() {
		return q.Done()
	}
	v.Say("%% is a file name and nothing more")
	q.InFunction("eval_vars", func(q *graph.Verbs) {
		q.Cut(`(= tilde_file (== (call strcmp (cast (ptr char) (paren result)) (cast (ptr char) (paren "~"))) 0))`, 2,
			"a tilde_file assignment")
		q.Cut("(= skip_mod TRUE)", 1, "skip_mod's assignment")
	})
	if q.Failed() {
		return q.Done()
	}
	v.Say("tilde_file and skip_mod, which only fed it")
	v.Sayf("%d modify_fname mentions left for the sweep", len(modifyFname.FindAll(v.Text(), -1)))
	return v.Done()
}
