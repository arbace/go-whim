// Package sweep deletes what a phase left unreachable: Prune (prune.go), one
// reachability closure over the parsed text, and the text cut to what it
// reached.  It replaced six deleters -- deadsweep (gcc's unused warnings),
// deadprotos, typereach, funcreach, deadfields and deadenums -- that looped to
// a fixpoint, each seeing one kind of thing, because what they found between
// them is one closure.
package sweep

import (
	"fmt"
	"io"
	"os"
	"time"
)

// MaxRounds is how many rounds Prune takes before it calls the text
// non-converging.  One round finds everything the closure can; a second is
// needed only when deleting an unused local orphans what its initialiser
// named, and the second finding nothing is the fixpoint.
const MaxRounds = 15

// Sweep prunes the file at path in place and reports one line to w.
func Sweep(path string, w io.Writer) (Stats, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Stats{}, err
	}
	start := time.Now()
	out, st, err := Prune(src, path)
	if err != nil {
		return st, fmt.Errorf("sweep: %w", err)
	}
	fmt.Fprintf(w, "  sweep        %s; %dms\n", st, time.Since(start).Milliseconds())
	if string(out) == string(src) {
		return st, nil
	}
	return st, os.WriteFile(path, out, 0o644)
}
