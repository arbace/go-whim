package xform

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

// EmptyBlocks is the step that folds the empty blocks of the core: an empty
// block guarded by a condition that only reads goes, as do an empty else and
// an empty else-if ending its chain; a local then only ever given a value
// goes with its stores (edit.DeadStores), the two alternating to a fixpoint.
// The host, past core, is not touched.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N
// blocks fold.  Without it there is none.
func EmptyBlocks(core Core) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "empty", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		i := core(text)
		if i < 0 {
			return nil, p.Die("the core does not end where this step was told it does")
		}
		out, n, took := EmptyBlocksRule(text[:i+1])
		if n < f["--at-least"] {
			return nil, p.Die("%d empty blocks fold, fewer than the %d this step was told to expect", n, f["--at-least"])
		}
		p.Say(fmt.Sprintf("%d empty blocks fold away: an if whose condition only reads, an empty else, an empty else-if that ends its chain", n))
		p.Say(fmt.Sprintf("%d locals only given values once their tests went, and go with their stores: %s", len(took), strings.Join(took, " ")))
		return append(out, text[i+1:]...), nil
	}
}

// EmptyBlocksFold is the fold, applied once over text to its fixpoint.  It
// returns the text and how many blocks went.
//
//	if (C) {}      no else after, C pure        -> nothing
//	else {}                                     -> nothing
//	else if (C) {} last of its chain, C pure    -> nothing
//
// Nothing else: an empty block whose condition does something, an empty if
// with an else after it, and every empty loop stay.
func EmptyBlocksFold(core []byte) ([]byte, int) {
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

// EmptyBlocksRule is EmptyBlocksFold and edit.DeadStores, each to its
// fixpoint, in turn until neither changes anything -- a flag tested only by
// an empty if is only stored once the if goes, and a store that goes can
// leave a block empty.  It returns the text, the blocks that went and the
// locals that did.
func EmptyBlocksRule(core []byte) ([]byte, int, []string) {
	n := 0
	var took []string
	for {
		var k int
		var t []string
		core, k = EmptyBlocksFold(core)
		core, t = edit.DeadStores(core)
		n += k
		took = append(took, t...)
		if k == 0 && len(t) == 0 {
			return core, n, took
		}
	}
}
