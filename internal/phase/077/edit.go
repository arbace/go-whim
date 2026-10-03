package p077

// Whim phase 77 (formerly 154) -- the NULL write in free_one_termoption() is gone.  See GOAL.md.
//
// ttest() called free_one_termoption() with t_Co's value where its address was
// meant; the call never cleared t_Co, and its one effect was a write through
// NULL when both were NULL (phase 77a).  The call goes; the collection takes
// the function.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1c): the text program's one
// literal, the if and its call, is one node found by its form and cut;
// history keeps the text version.  Part 77a, which runs before it in the
// phase, is still text.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// W77Call is the if this phase cuts, with the call in it, as C-lisp.
const W77Call = "(if (&& (== (deref (paren (index term_strings (cast int (paren KS_CSB))))) NUL)" +
	" (== (deref (paren (index term_strings (cast int (paren KS_CAB))))) NUL))" +
	" (block (call free_one_termoption (paren (index term_strings (cast int (paren KS_CCO)))))))"

func init() { phase.RegisterGraph("whim77", Edit) }

// Edit fixes vim's NULL write in free_one_termoption().
//
// ttest() called free_one_termoption(t_Co) when the terminal had neither
// t_Sb nor t_AB, meaning to clear 't_Co'.  But it passed the string value,
// and the function looks for the option whose variable ADDRESS is its
// argument: phase 77a showed the two are equal only when both are NULL, and
// then the function wrote empty_option through the NULL variable of the first
// option that has none.  So the call never cleared 't_Co' -- the one thing it
// ever did was that write.  It goes, with the if around it, whose condition
// only reads the two strings the lines above it already read; the collection
// takes free_one_termoption(), which nothing else calls.  What the editor
// does is unchanged, but for the crash (internal/gen/FINDINGS.md).
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nullwrite", e, w)
	v.Cut(W77Call, 1, "ttest() no longer calls free_one_termoption(), whose one effect was a write through NULL")
	return v.Done()
}
