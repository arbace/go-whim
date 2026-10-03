package cut

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

var identityLeft = regexp.MustCompile(`\bgetuid\b|\bgetgid\b|\bget_user_name\b`)

// NoOwner stops the editor asking who owns anything.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the text version's literal
// substitutions and brace-matched block surgery are acts on the nodes --
// an operand dropped, a guard folded always true, an if dropped, two
// statements cut, an if with an else kept to its then, an assignment made
// a void call -- each counted, its report the text's (history keeps it).
// The identity mentions left are counted on the C view, the text's number.
func NoOwner(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noowner", e, w)
	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.DropOperand("(== (. st_old st_uid) (call getuid))", 1,
			":w! clears the read-only bit without asking whose it is")
		// The mode masking stays; only the test that guarded it goes.  No
		// indentation in the text's anchor: nobackup, which cut the block
		// around it, runs at phase 5 now.
		masking := "(|| (!= (. st_old st_uid) (call getuid)) (!= (. st_old st_gid) (call getgid)))"
		v.One("(if "+masking+" (block (&= perm 0777)))", "the mode masking is not where this expects")
		v.FoldAlways(masking, 1, "a written file never carries a setuid bit, whoever wrote it")
	})
	v.DropIf("(&& (== (. (index options opt_idx) indir) _) (== (call getuid) ROOT_UID))", 1,
		"'modeline' stops asking whether this is root")

	// Who wrote the swap file: block zero's user name, and the other caller's
	// `if`, which was already always true -- get_user_name() has returned
	// FAIL since Phase 9 -- so its `else` has been dead that long.  The text
	// reported the three acts as one line, after them.
	quiet := graph.NewVerbs("noowner", e, io.Discard)
	quiet.InFunction("ml_open", func(q *graph.Verbs) {
		q.Cut("(cast void (call get_user_name (-> b0p b0_uname) B0_UNAME_SIZE))", 1, "block zero's user name")
		q.Cut("(= (index (-> b0p b0_uname) (- B0_UNAME_SIZE 1)) NUL)", 1, "block zero's user name")
	})
	quiet.InFunction("set_b0_fname", func(q *graph.Verbs) {
		q.FoldAlwaysElse("(|| (== (call get_user_name uname B0_UNAME_SIZE) FAIL) _)", 1, "the get_user_name arm")
		q.Rewrite("(= flen ?c)", "(cast void ?c)", 1, "set_b0_fname's home_replace")
	})
	if err := quiet.Done(); err != nil {
		return err
	}
	v.Say("who wrote the swap file, a stub since Phase 9")

	v.Sayf("%d identity mentions left for the sweep", v.TextCount(identityLeft.String()))
	return v.Done()
}
