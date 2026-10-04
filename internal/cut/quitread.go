package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// quitCond is ex_quit's refusal on the seed: a changed buffer, more files to
// edit, or another changed buffer kept `:q` from quitting.
const quitCond = "(|| (paren (&& (! (call buf_hide _)) (call check_changed _ _))) " +
	"(== (call check_more TRUE _) FAIL) (paren (&& (call only_one_window) (call check_changed_any _ TRUE))))"

// QuitFront makes `:q` quit (the reform's front cut for phase 33's change):
// with nothing that can be written, the refusal to quit a changed buffer is a
// door onto nothing.  It folds never, the else arm -- quit -- stays, and
// check_changed_any's tail, the last caller of the editor's buffer- and
// window-switching code, goes with it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): one FoldNever by the
// condition's form, in ex_quit (history keeps the text version).
func QuitFront(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("quitfront", e, w)
	if e.Defn("ex_quit") == nil {
		v.Die("ex_quit is not defined")
		return v.Done()
	}
	q := graph.NewVerbs("quitfront", e, io.Discard)
	q.InFunction("ex_quit", func(q *graph.Verbs) { q.FoldNever(quitCond, 1, "ex_quit's refusal") })
	if err := q.Done(); err != nil {
		return err
	}
	v.Say(":q quits: ex_quit's refusal for a changed buffer folds never")
	return v.Done()
}

// ReadFront takes every read of a file (the reform's front cut for phase 31's
// change): open_buffer's two arms -- the named file, and stdin -- fold never,
// and readfile(), read_buffer() and everything only they reached go with
// them.  A buffer opens empty.  On the graph (B4): two FoldNever by form.
func ReadFront(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("readfront", e, w)
	if e.Defn("open_buffer") == nil {
		v.Die("open_buffer is not defined")
		return v.Done()
	}
	q := graph.NewVerbs("readfront", e, io.Discard)
	q.InFunction("open_buffer", func(q *graph.Verbs) {
		q.FoldNever("(!= (-> curbuf b_ffname) nullptr)", 1, "open_buffer's file arm")
		q.FoldNever("read_stdin", 1, "open_buffer's stdin arm")
	})
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("nothing reads a byte: open_buffer's file and stdin arms fold never")
	return v.Done()
}
