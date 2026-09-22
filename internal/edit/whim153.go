package edit

import (
	"io"
)

func init() { register("whim153", Whim153) }

// Whim153 says what free_one_termoption() compares.
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
// the latent NULL write included (tx/FINDINGS.md).  The Go transpilation of
// phase 152 already wrote it this way, since Go cannot compare the two types.
func Whim153(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "termopt", w: w}
	return p.literal(text, "        if ((char_u *)p->var.ov_str == var)\n", "        if (p->var.ov_str == nullptr && var == nullptr)\n",
		"free_one_termoption() compares the only way the two can be equal: both NULL", 1)
}
