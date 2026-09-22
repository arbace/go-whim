package p134

// Whim phase 134 -- the empty blocks fold.  See GOAL.md.
//
// Phase 132 left 33 blocks empty, beside those earlier phases left: 49 fold.
// An empty block guarded by a condition that only reads goes, as do an empty
// else and an empty else-if ending its chain; the sweep takes what the
// conditions computed and nothing reads any more.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim134", Edit) }

// W134Fold is the rule, applied once over the whole core, exported so the check
// applies the identical one.  It returns the text and how many blocks went.
//
//	if (C) {}      no else after, C pure        -> nothing
//	else {}                                     -> nothing
//	else if (C) {} last of its chain, C pure    -> nothing
//
// Nothing else: an empty block whose condition does something, an empty if
// with an else after it, and every empty loop stay.
func W134Fold(core []byte) ([]byte, int) {
	n := 0
	for {
		changed := false
		for _, m := range edit.W134Empty.FindAllSubmatchIndex(core, -1) {
			head := string(core[m[4]:m[5]])
			after := core[m[1]:]
			var cond string
			switch {
			case head == "else":
			case bytes.HasPrefix([]byte(head), []byte("if (")):
				cond = head[4 : len(head)-1]
			default: // else if
				cond = head[9 : len(head)-1]
			}
			if head != "else" && (!edit.W134Pure(cond) || edit.W134Else.Match(after)) {
				continue
			}
			core = append(append([]byte{}, core[:m[0]]...), core[m[1]:]...)
			n++
			changed = true
			break
		}
		if !changed {
			return core, n
		}
	}
}

// W134Rule is the phase: W134Fold and DeadStores, each to its fixpoint, in turn
// until neither changes anything -- a flag tested only by an empty if is only
// stored once the if goes, and a store that goes can leave a block empty.
func W134Rule(core []byte) ([]byte, int, []string) {
	n := 0
	var took []string
	for {
		var k int
		var t []string
		core, k = W134Fold(core)
		core, t = edit.DeadStores(core)
		n += k
		took = append(took, t...)
		if k == 0 && len(t) == 0 {
			return core, n, took
		}
	}
}

// Whim134 folds the blocks phase 132 left empty.
//
// Taking Out 273 frees left 33 blocks that had held nothing else (`if
// (allocated) { vim_free(p); }` is `if (allocated) { }`), beside the empty
// blocks the pipeline had already left: 49 in all.  An empty block
// guarded by a condition that only reads does nothing, and goes; so does an
// empty else, and an empty else-if that ends its chain.  What the condition
// computed is then read by nothing: a local only ever given a value goes with
// its stores (DeadStores), and the sweep takes the rest.
// The Go transpilation never had these blocks (tx/FINDINGS.md, 9).
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "empty", W: w}
	i := bytes.Index(text, []byte("\n#include"))
	if i < 0 {
		return nil, p.Die("no #include: the boundary is not where this phase expects it")
	}
	core, n, took := W134Rule(text[:i+1])
	if n < 20 {
		return nil, p.Die("%d empty blocks fold; this phase was written against the 30 or so phase 132 leaves", n)
	}
	p.Say(fmt.Sprintf("%d empty blocks fold away: an if whose condition only reads, an empty else, an empty else-if that ends its chain", n))
	p.Say(fmt.Sprintf("%d locals only given values once their tests went, and go with their stores: %s", len(took), strings.Join(took, " ")))
	return append(core, text[i+1:]...), nil
}
