package steps

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// Phase 101's three steps on the graph (doc/GRAPH-MIGRATION.md, Step6):
// crefactor/graph's PlainIdentity, PlainAsciiClass and PlainConstBranch on
// the core (the forms above the first include), which replaced
// crefactor/xform's plainc.go.  Each reports what the text step reported.
func init() {
	graphOps["identity"] = s6Identity
	graphOps["asciiclass"] = s6AsciiClass
	graphOps["constbranch"] = s6ConstBranch
}

func s6Identity(e *graph.Editor, _ []string, w io.Writer) error {
	n, f, err := e.PlainIdentity(e.Core())
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "  identity: %d calls of %d identity functions are their arguments\n", n, f)
	return nil
}

func s6AsciiClass(e *graph.Editor, _ []string, w io.Writer) error {
	used, err := e.PlainAsciiClass(e.Core())
	if err != nil {
		return err
	}
	n := 0
	for _, k := range used {
		n += k
	}
	fmt.Fprintf(w, "  asciiclass: %d tests named (%v)\n", n, used)
	return nil
}

func s6ConstBranch(e *graph.Editor, _ []string, w io.Writer) error {
	n, err := e.PlainConstBranch(e.Core())
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "  constbranch: %d ifs of a constant condition are the branch they take\n", n)
	return nil
}
