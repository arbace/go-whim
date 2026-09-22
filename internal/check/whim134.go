package check

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { register("whim134", Whim134) }

// Whim134 is phase 134's check: the empty blocks fold.
//
//  1. THE WHOLE OUTPUT IS COMPUTED: the input's core with edit.W134Rule applied
//     -- the same function -- and the host unchanged, run through the real
//     sweep, is the output byte for byte.  What the sweep took beyond the
//     blocks is named.
//  2. WHAT IS LEFT is the rule's complement: every empty block remaining above
//     the boundary is a loop, a function body, an if with an else after it, or
//     guarded by a condition that does something.
//  3. THE GATE, the libc surface unchanged.
func Whim134(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim134", "empty")
	if err != nil {
		return err
	}
	r := c.r
	i := strings.Index(c.old, "\n#include")
	core, n, _ := edit.W134Rule([]byte(c.old[:i+1]))
	pre := string(core) + c.old[i+1:]
	comp, err := swept(pre)
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	if comp != c.new {
		l, a, b := firstDiff(comp, c.new)
		r.bad("the output is not the input with the fold applied and swept: they part at line %d\n    computed: %q\n    output:   %q", l, a, b)
	}
	if err := r.done(); err != nil {
		return err
	}
	took := sweptLines(pre, c.new)
	r.say("the output IS the input with %d empty blocks folded and the locals left only given values taken by the rule, then swept: computed, byte for byte", n)
	r.say("the sweep took, beyond the blocks, %d lines: %s", len(took), strings.Join(took, " | "))

	j := strings.Index(c.new, "\n#include")
	left := w134Left(c.new[:j+1])
	r.say("%d empty blocks are left, every one a loop, a function body, an if with an else after it, or a condition that does something: %s",
		len(left), strings.Join(left, " | "))
	return c.gate(true)
}

// w134Left names each empty block above the boundary by its head, and refuses
// (by returning a marked name) one the rule should have folded.
func w134Left(core string) []string {
	var r []string
	again, _, _ := edit.W134Rule([]byte(core))
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
