package p003f

// Whim phase 3f (formerly 74) -- no file marks.  See GOAL.md.
//
// THIS IS THE PHASE THAT WAS ABANDONED AS 70 AND IS NOW REDONE PROPERLY.  The first
// attempt died three times on patterns transcribed from truncated views -- the
// `|| to < from` tail, clrallmarks' `static int i = -1` guard over 26 + 1, and four
// unenumerated adjust sites -- and, worse, its probes MEASURED NOTHING, because a
// mark name like 'A carries a quote that broke the shell quoting so the key was never
// pressed.  Every site below was read verbatim with cat -A first, and every probe was
// calibrated against q73 of the old numbering before being trusted.
//
// WHAT GOES: namedfm[26 + EXTRA_MARKS], which is BOTH the uppercase A-Z file marks
// and the numbered 0-9 marks -- one array, one set of code paths, so they cannot be
// separated.  With one buffer the numbered marks could never be set anyway: viminfo
// is long gone, so they were a store nothing could write.
//
// mA .. mZ   'A .. 'Z   `A .. `Z   '0 .. '9
// the cross-file jump: getmark_buf_fnum's arm was the ONLY caller of
// buflist_getfile(), and of fname2fnum() -- itself already an empty body from
// phase 22, folded there precisely because the file marks were a separate cut.
//
// WHAT STAYS: the lowercase marks a-z in buf->b_namedm[], and every special mark --
// ' ` " ^ . [ ] < > -- none of which touch namedfm.  :marks still lists what is left
// and :delmarks still clears it; uppercase and digits become "invalid argument".
// fmark_T STAYS: struct taggy embeds it, so the tag stack depends on it.  Only
// xfmark_T, which exists to bolt a filename onto a mark, goes.
//
// SCOPE EVERY EDIT BY FUNCTION.  do_join() has a PARAMETER named `setmark`, so a
// global rename or an unscoped pattern would corrupt it -- the b_next lesson from
// phase 4d in a new costume.
//
// THE DELTA: none expected.  :marks and :delmarks keep their exit status and write
// nothing to stderr for the arguments exsweep uses, and an exsweep row is
// `exit= left= err=`.  Declared empty and left for the delta check to correct.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the folds and cuts by form,
// each scoped to its function, the two bodies FRAG in one unit (Together);
// the acts run quietly and the report is written after them, in the text's
// order, since Together reports its FRAG acts last (history keeps the text
// version).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// markArm is the test for an uppercase letter or a digit, which is how a
// file mark is spelled.
const markArm = "(|| (paren (< (- (cast unsigned (paren c)) 'A') 26)) (paren (< (- (cast unsigned (paren c)) '0') 10)))"

// Edit takes file marks: the uppercase and numbered marks, the table that
// held them, and the type that carried a filename with each.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nofmark", e, w)
	q := graph.NewVerbs("nofmark", e, io.Discard)
	var said []string
	say := func(what string) string { said = append(said, what); return what }

	q.Together(func(q *graph.Verbs) {
		q.InFunction("getmark_buf_fnum", func(q *graph.Verbs) {
			q.FoldNever(markArm, 1, say("reading an uppercase or numbered mark"))
		})
		q.InFunction("setmark_pos", func(q *graph.Verbs) {
			q.DropIf(markArm, 1, say("setting an uppercase or numbered mark"))
		})
		q.BodyC("clrallmarks", w3flit2, say("clrallmarks initialising the file marks once"))
		q.InFunction("ex_marks", func(q *graph.Verbs) {
			q.Cut("(for (= i 0) (< i (+ (paren (+ (- 'z' 'a') 1)) EXTRA_MARKS)) (pre++ i) _)", 1,
				say(":marks listing the file marks"))
		})
		q.BodyC("ex_delmarks", w3flit3, say(":delmarks clearing an uppercase or numbered mark"))
		for _, fn := range []string{"mark_adjust_internal", "mark_col_adjust"} {
			q.InFunction(fn, func(q *graph.Verbs) {
				q.Cut("(if (== (. (index namedfm i) fmark fnum) fnum) _)", 2,
					say(fmt.Sprintf("adjusting the file marks in %s", fn)))
				q.Cut("(for (= i (paren (+ (- 'z' 'a') 1))) _ _ _)", 1,
					say(fmt.Sprintf("and the loop over the numbered marks in %s", fn)))
			})
		}
		// fmarks_check_names existed to reattach a file mark to a buffer by
		// name; with no file marks there is nothing to reattach.  With its two
		// calls gone the collection takes it, fname2fnum (folded empty in
		// phase 22), namedfm and EXTRA_MARKS.
		q.Cut("(call fmarks_check_names buf)", 2, say("the two calls that rematched file marks"))
	})
	if err := q.Done(); err != nil {
		return err
	}
	for _, s := range said {
		v.Say(s)
	}
	return v.Done()
}

func init() { phase.RegisterGraph("whim3f", Edit) }
