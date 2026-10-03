package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// OptReaders removes what read the options a previous phase dropped.
//
// -nb's early scan and --clean's pre-scan of argv went with the command
// line, and what --clean, --not-a-term and -n set is never written after it:
// the fall-out closure folds its readers (argvfront, the reform's D1).  -p's
// test of the window layout went with the window layouts at phase 1
// (nowindows, the reform's D9), and -p's tab pages, and 'shortmess' restored
// after filling them, with them at phase 3.  What is left is -h's pointer,
// and what must be LEFT is counted after it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the statement is found by its
// string and deleted, counted; the text version removed its line by its
// exact text (history keeps it).  What is left is counted on the C view, the
// text's own question.
func OptReaders(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("optreaders", e, w)
	v.Cut(`(call fprintf stderr "%s" (paren (call _ "\nMore info with: \"vim -h\"\n")))`, 1,
		"-h: the pointer to it at the end of every usage error")
	if !v.Failed() {
		if n := v.TextCount(`\bearly_arg_scan\(paramp\)`); n != 0 {
			v.Die("early_arg_scan outside its definition and prototype -- %d left, expected 0", n)
		}
	}
	v.Say("nothing reads what a dropped option set")
	return v.Done()
}
