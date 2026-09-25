package p154

// Whim phase 154 -- the NULL write in free_one_termoption() is gone.  See GOAL.md.
//
// ttest() called free_one_termoption() with t_Co's value where its address was
// meant; the call never cleared t_Co, and its one effect was a write through
// NULL when both were NULL (phase 153).  The call goes; the sweep takes the
// function.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// W154Call is the call this phase takes Out, with the if around it.
const W154Call = "\n        if (*(term_strings[(int)(KS_CSB)]) == NUL && *(term_strings[(int)(KS_CAB)]) == NUL)\n        {\n            free_one_termoption((term_strings[(int)(KS_CCO)]));\n        }\n"

func init() { phase.Register("whim154", Edit) }

// Whim154 fixes vim's NULL write in free_one_termoption().
//
// ttest() called free_one_termoption(t_Co) when the terminal had neither
// t_Sb nor t_AB, meaning to clear 't_Co'.  But it passed the string value,
// and the function looks for the option whose variable ADDRESS is its
// argument: phase 153 showed the two are equal only when both are NULL, and
// then the function wrote empty_option through the NULL variable of the first
// option that has none.  So the call never cleared 't_Co' -- the one thing it
// ever did was that write.  It goes, with the if around it, whose condition
// only reads the two strings the lines above it already read; the sweep takes
// free_one_termoption(), which nothing else calls.  What the editor does is
// unchanged, but for the crash (internal/gen/FINDINGS.md).
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nullwrite", text, w)
	e.Literal(W154Call, "\n", 1, "ttest() no longer calls free_one_termoption(), whose one effect was a write through NULL")
	return e.Done()
}
