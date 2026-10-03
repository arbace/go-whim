package cut

import (
	"bytes"
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// nostatCalls are the two direct buf_check_timestamp() sites, each with the
// function it sits in, so a refusal names WHERE the shape moved.
var nostatCalls = []struct{ pat, where string }{
	{"(cast void (call buf_check_timestamp curbuf FALSE))", "do_ecmd"},
	{"(cast void (call buf_check_timestamp buf FALSE))", "enter_buffer"},
	// ex_drop's went with :drop, retired at phase 1 (exfront, the reform's D2)
}

// NoStat stops the editor re-reading a file it has already read.
//
// check_timestamps() becomes `return 0` -- its four callers each already handle
// that answer, so stubbing is deliberate where unpicking four different control
// structures is not.  The two direct buf_check_timestamp() calls then go,
// because with the poll gone they are the only thing keeping 339 lines alive.
//
// check_mtime() is NOT touched: it runs only when the user asks to write, and
// it is what stops a write silently clobbering someone else's edit.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the body is replaced by its
// one statement (Body), and the two calls are deleted as items, each
// counted; the text version rewrote the lines (history keeps it).  The old
// body's length and the mentions left are the text's numbers, on the C view.
func NoStat(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nostat", e, w)
	was := 0
	v.InFunction("check_timestamps", func(v *graph.Verbs) { was = bodyLines(v.Text()) })
	v.Body("check_timestamps", "(return 0)",
		fmt.Sprintf("check_timestamps was %d lines, and now looks at nothing", was))
	for _, c := range nostatCalls {
		if v.Failed() {
			break
		}
		if n := v.Count(c.pat); n != 1 {
			return fmt.Errorf("nostat: expected one buf_check_timestamp call in %s, "+
				"matched %d", c.where, n)
		}
		v.Cut(c.pat, 1, c.where+" stops checking on the way in")
	}
	if !v.Failed() {
		v.Sayf("%d buf_check_timestamp mentions left for the sweep",
			bytes.Count(v.Text(), []byte("buf_check_timestamp")))
	}
	return v.Done()
}

// bodyLines is the text's measure of a function's body, on the function's C
// view: the line ends from its opening brace, at the start of a line, to its
// closing one -- what edit.ReplaceBody reported as the body it replaced.
func bodyLines(fn []byte) int {
	o := bytes.Index(fn, []byte("\n{"))
	c := bytes.LastIndexByte(fn, '}')
	if o < 0 || c < o {
		return 0
	}
	return bytes.Count(fn[o+1:c], []byte{'\n'})
}
