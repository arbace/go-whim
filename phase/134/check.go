package p134

// Whim phase 134, the check -- the empty blocks fold.
// See phase/134/edit.go, and GOALS.md.
//
// phase/134/check.go COMPUTES the whole output: edit.W134Fold applied to
// the input's core, run through tools/sweep.sh, is the output byte for byte; and
// every empty block left is one the rule must keep.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim134", Check) }

// Whim134 is phase 134's check: the empty blocks fold.
//
//  1. THE WHOLE OUTPUT IS COMPUTED: the input's core with W134Rule applied
//     -- the same function -- and the host unchanged, run through the real
//     sweep, is the output byte for byte.  What the sweep took beyond the
//     blocks is named.
//  2. WHAT IS LEFT is the rule's complement: every empty block remaining above
//     the boundary is a loop, a function Body, an if with an else after it, or
//     guarded by a condition that does something.
//  3. THE GATE, the libc surface unchanged.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim134", "empty")
	if err != nil {
		return err
	}
	r := c.R
	i := strings.Index(c.Old, "\n#include")
	core, n, _ := W134Rule([]byte(c.Old[:i+1]))
	pre := string(core) + c.Old[i+1:]
	comp, err := check.Swept(pre)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	if comp != c.New {
		l, a, b := check.FirstDiff(comp, c.New)
		r.Bad("the output is not the input with the fold applied and swept: they part at line %d\n    computed: %q\n    output:   %q", l, a, b)
	}
	if err := r.Done(); err != nil {
		return err
	}
	took := check.SweptLines(pre, c.New)
	r.Say("the output IS the input with %d empty blocks folded and the locals left only given values taken by the rule, then swept: computed, byte for byte", n)
	r.Say("the sweep took, beyond the blocks, %d lines: %s", len(took), strings.Join(took, " | "))

	j := strings.Index(c.New, "\n#include")
	left := w134Left(c.New[:j+1])
	r.Say("%d empty blocks are left, every one a loop, a function body, an if with an else after it, or a condition that does something: %s",
		len(left), strings.Join(left, " | "))
	return c.Gate(true)
}

// w134Left names each empty block above the boundary by its head, and refuses
// (by returning a marked name) one the rule should have folded.
func w134Left(core string) []string {
	var r []string
	again, _, _ := W134Rule([]byte(core))
	if string(again) != core {
		r = append(r, "!! THE RULE STILL FOLDS SOMETHING HERE")
	}
	lines := strings.Split(core, "\n")
	for k := 0; k+2 < len(lines); k++ {
		if strings.TrimSpace(lines[k+1]) == "{" && strings.TrimSpace(lines[k+2]) == "}" {
			h := strings.TrimSpace(lines[k])
			if len(h) > 40 {
				h = h[:40] + "..."
			}
			r = append(r, h)
		}
	}
	return r
}
