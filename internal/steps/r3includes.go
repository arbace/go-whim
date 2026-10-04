package steps

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// includesGraph is phase 88 (R3, doc/GRAPH-MIGRATION.md): every system
// header the file does not need, deleted -- asked of the graph's include
// rule (crefactor/graph's Spares, B2e's extern rule) where the text step
// asked gcc whether the file still compiled silently and preprocessed its
// own lines to the same tokens (crefactor/xform's Includes with Silent's
// Same, deleted with it).  A header is spare when deleting it leaves every
// name the file takes from the headers provided by an include above its
// first use; the rule never lets a token mean something else, which is
// what Same asked of the preprocessor.  In the text step's order: each
// include alone, then those together, else a fold from the bottom.
//
// Its refusals are the text step's: a directive other than an include,
// no include at all, and an input the rule does not accept as it stands --
// a name the headers provide nowhere above its use, or a header's macro
// over the file's own name (the text step's "does not compile silently").
// internal/graphcheck's TestIncludesPhase88 holds the answer to gcc's.
func includesGraph(e *graph.Editor, args []string, w io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("  includes     takes no argument (%s)", strings.Join(args, " "))
	}
	for _, f := range e.Graph().Forms {
		if f.Is("directive") {
			return fmt.Errorf("  includes     a directive other than #include is in the file")
		}
	}
	r, err := e.Spares()
	if err != nil {
		return err
	}
	if len(r.All) == 0 {
		return fmt.Errorf("  includes     no #include lines found")
	}
	if len(r.Unprovided) > 0 || len(r.Collisions) > 0 {
		var why []string
		for _, u := range r.Unprovided {
			why = append(why, u.String()+", which no include above it provides")
		}
		for _, c := range r.Collisions {
			why = append(why, c.String())
		}
		return fmt.Errorf("  includes     the input does not stand under the rule -- nothing to measure against: %s", strings.Join(why, "; "))
	}
	fmt.Fprintf(w, "  includes     %d of %d can go on their own\n", len(r.Alone), len(r.All))
	if r.Together {
		fmt.Fprintf(w, "  includes     and all of them together\n")
	} else {
		fmt.Fprintf(w, "  includes     together they do not build; %d removed one by one, from the bottom\n", len(r.Spare))
	}
	if len(r.Spare) == 0 {
		fmt.Fprintf(w, "  includes     every header is needed\n")
		return nil
	}
	if err := e.DeleteIncludes(r.Spare...); err != nil {
		return err
	}
	var removed []string
	for _, inc := range r.Spare {
		removed = append(removed, strings.TrimSuffix(strings.TrimPrefix(graph.IncludeSpec(inc), "<"), ">"))
	}
	fmt.Fprintf(w, "  includes     removed: %s \n", strings.Join(removed, " "))
	return nil
}
