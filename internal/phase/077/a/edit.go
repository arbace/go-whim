package p077a

// Whim phase 77a (formerly 153) -- free_one_termoption() compares without a cast.  See GOAL.md.
//
// It compared an option variable's address, cast to char_u *, with a string
// value; the two are equal exactly when both are NULL, and the comparison
// now says so, with no pointer cast to another type (internal/gen/FINDINGS.md).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim77a", Edit) }

// Edit says what free_one_termoption() compares.
//
// free_one_termoption(var) looks for the option whose variable is var: it
// compared each row's variable ADDRESS, cast to char_u *, with the string
// VALUE its one caller hands it, term_strings[KS_CCO] -- as vim does
// upstream.  An option's non-NULL ov_str is the address of a char_u *
// variable (a p_* global or a term_strings slot), and no code stores the
// address of such a variable as a string, so the two are equal exactly when
// both are NULL: a row with no variable, and a NULL terminal string -- where
// the C then writes through the NULL.  The comparison is written as that, so
// no pointer is cast to a pointer of another type; what it does is unchanged,
// the latent NULL write included (internal/gen/FINDINGS.md).  The Go transpilation of
// phase 76 already wrote it this way, since Go cannot compare the two types.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the comparison, found by its
// form, written anew by FRAG; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("termopt", e, w)
	v.InFunction("free_one_termoption", func(v *graph.Verbs) {
		v.ReplaceC("(== (cast (ptr char_u) (. (-> p var) ov_str)) var)", "p->var.ov_str == nullptr && var == nullptr", 1,
			"free_one_termoption() compares the only way the two can be equal: both NULL")
	})
	return v.Done()
}
