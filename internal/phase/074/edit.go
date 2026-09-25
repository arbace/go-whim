package p074

// Whim phase 74 -- no file marks.  See GOAL.md.
//
// THIS IS THE PHASE THAT WAS ABANDONED AS 70 AND IS NOW REDONE PROPERLY.  The first
// attempt died three times on patterns transcribed from truncated views -- the
// `|| to < from` tail, clrallmarks' `static int i = -1` guard over 26 + 1, and four
// unenumerated adjust sites -- and, worse, its probes MEASURED NOTHING, because a
// mark name like 'A carries a quote that broke the shell quoting so the key was never
// pressed.  Every site below was read verbatim with cat -A first, and every probe was
// calibrated against q73 before being trusted.
//
// WHAT GOES: namedfm[26 + EXTRA_MARKS], which is BOTH the uppercase A-Z file marks
// and the numbered 0-9 marks -- one array, one set of code paths, so they cannot be
// separated.  With one buffer the numbered marks could never be set anyway: viminfo
// is long gone, so they were a store nothing could write.
//
// mA .. mZ   'A .. 'Z   `A .. `Z   '0 .. '9
// the cross-file jump: getmark_buf_fnum's arm was the ONLY caller of
// buflist_getfile(), and of fname2fnum() -- itself already an empty body from
// phase 70, folded there precisely because the file marks were a separate cut.
//
// WHAT STAYS: the lowercase marks a-z in buf->b_namedm[], and every special mark --
// ' ` " ^ . [ ] < > -- none of which touch namedfm.  :marks still lists what is left
// and :delmarks still clears it; uppercase and digits become "invalid argument".
// fmark_T STAYS: struct taggy embeds it, so the tag stack depends on it.  Only
// xfmark_T, which exists to bolt a filename onto a mark, goes.
//
// SCOPE EVERY EDIT BY FUNCTION.  do_join() has a PARAMETER named `setmark`, so a
// global rename or an unscoped pattern would corrupt it -- the b_next lesson from
// phase 71 in a new costume.
//
// THE DELTA: none expected.  :marks and :delmarks keep their exit status and write
// nothing to stderr for the arguments exsweep uses, and an exsweep row is
// `exit= left= err=`.  Declared empty and left for the delta check to correct.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// markArm is the macro-expanded test for an uppercase letter or a digit, which
// is how a file mark is spelled.
const markArm = `\(\(\(unsigned\)\(c\) - 'A' < 26\) \|\| \(\(unsigned\)\(c\) - '0' < 10\)\)$`

// Whim74 takes file marks: the uppercase and numbered marks, the table that
// held them, and the type that carried a filename with each.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nofmark", text, w)

	e.InFunction("getmark_buf_fnum", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*else if `+markArm, 1, "reading an uppercase or numbered mark")
	})
	e.InFunction("setmark_pos", func(e *edit.E) { e.DropIf(`(?m)^[ \t]*if `+markArm, 1, "setting an uppercase or numbered mark") })
	e.Body("clrallmarks", w74lit2, "clrallmarks initialising the file marks once")
	e.DropBlocks("ex_marks", edit.Head("for (i = 0; i < ('z' - 'a' + 1) + EXTRA_MARKS; ++i)"), 1,
		":marks listing the file marks")
	e.Body("ex_delmarks", w74lit3, ":delmarks clearing an uppercase or numbered mark")
	for _, fn := range []string{"mark_adjust_internal", "mark_col_adjust"} {
		e.DropBlocks(fn, edit.Head("if (namedfm[i].fmark.fnum == fnum)"), 2,
			fmt.Sprintf("adjusting the file marks in %s", fn))
		e.DropBlocks(fn, edit.Head("for (i = ('z' - 'a' + 1); i < ('z' - 'a' + 1) + EXTRA_MARKS; i++)"), 1,
			fmt.Sprintf("and the loop over the numbered marks in %s", fn))
	}
	// fmarks_check_names existed to reattach a file mark to a buffer by name;
	// with no file marks there is nothing to reattach.  With its two calls
	// gone the sweep takes it, fname2fnum (folded empty in phase 70), namedfm
	// and EXTRA_MARKS.
	e.Cut(edit.Line("fmarks_check_names(buf);"), 2, "the two calls that rematched file marks")
	return e.Done()
}

func init() { edit.Register("whim74", Edit) }
