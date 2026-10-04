package cut

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

const notHere = "(call emsg (call _ e_sorry_command_is_not_available_in_this_version))"

var shelloutStubs = []struct{ name, body string }{
	{"do_filter", notHere},
	{"do_shell", notHere},
	{"get_cmd_output", "(return nullptr)"},
}

// NoShellOut takes away every way to hand work to a shell.
//
// The three implementations become one line each, and then the TEARDOWN goes:
// vim_deltempdir() outlives the only things that ever made a temp directory,
// and a call left behind would be the phase half-done.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): each body by template (Body),
// its old length the text's count on the function's C view; the teardown's
// calls cut (history keeps the text version).
func NoShellOut(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noshellout", e, w)
	total := 0
	for _, s := range shelloutStubs {
		// one only the commands deleted at phase 1 reached is gone with them
		// (filefront, the reform's D4): absent, and named by nothing
		if e.Defn(s.name) == nil && v.Mentions(s.name) == 0 {
			fmt.Fprintf(w, "  noshellout   %-16s gone already, with the commands that reached it\n", s.name)
			continue
		}
		was := 0
		v.InFunction(s.name, func(v *graph.Verbs) { was = bodyLines(v.Text()) })
		q := graph.NewVerbs("noshellout", e, io.Discard)
		q.Body(s.name, s.body, s.name)
		if err := q.Done(); err != nil {
			return err
		}
		if v.Failed() {
			return v.Done()
		}
		total += was
		fmt.Fprintf(w, "  noshellout   %-16s was %4d lines, is now one\n", s.name, was)
	}

	n := v.Count("(call vim_deltempdir)")
	if n == 0 {
		return fmt.Errorf("noshellout: nothing calls vim_deltempdir any more, so this " +
			"phase has already run or the exit path has moved")
	}
	q := graph.NewVerbs("noshellout", e, io.Discard)
	q.Cut("(call vim_deltempdir)", n, "the temp directory's teardown")
	if err := q.Done(); err != nil {
		return err
	}
	fmt.Fprintf(w, "  noshellout   %d lines of implementation gone; the temp directory "+
		"has nothing left to make it\n", total)
	return v.Done()
}
