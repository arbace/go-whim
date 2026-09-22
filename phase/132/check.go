package p132

// Whim phase 132, the check -- nothing frees.
// See phase/132/edit.go, and GOALS.md.
//
// phase/132/check.go proves the premise from the input (host_free() is
// empty, vim_free() only calls it) and COMPUTES the whole output: the input with
// every call replaced by edit.W132Rule, and vim_free() swept, is the output byte
// for byte.  It reports the blocks the calls leave empty.

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { check.Register("whim132", Check) }

var (
	w132CallC = regexp.MustCompile(`^([ \t]*)(vim_free|host_free)\((.*)\);[ \t]*$`)
	w132Empty = regexp.MustCompile(`(?m)^([ \t]*)(.*)\n([ \t]*)\{\n[ \t]*\}\n`)
)

func w132Core(text string) string {
	if i := strings.Index(text, "\n#include"); i >= 0 {
		return text[:i]
	}
	return text
}

// Whim132 is phase 132's check: nothing in the core frees.
//
//  1. WHY IT IS SAFE, on the input: host_free()'s definition, below the
//     boundary, has an empty Body; vim_free() is a NULL test around it.
//  2. THE WHOLE OUTPUT IS COMPUTED, not sampled: the input with every call
//     replaced by the rule the edit applies (W132Rule, the same function),
//     and then vim_free()'s definition and prototype removed, is the output
//     byte for byte.  So the sweep took vim_free() and nothing else, and no
//     call was dropped that had an effect.
//  3. NOTHING FREES: vim_free has no mention left, and host_free none above the
//     boundary but its prototype, which the host's formatter still needs.
//  4. THE BLOCKS THE CALLS LEAVE EMPTY are counted and reported: a later
//     phase's to fold, once each condition is shown to have no side effect.
//  5. THE GATE, the libc surface unchanged.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim132", "free")
	if err != nil {
		return err
	}
	r := c.R

	hostPart := c.Old[len(w132Core(c.Old)):]
	if !regexp.MustCompile(`(?s)\nhost_free\(void \*p\)\n\{\n\s*\(void\)p;\n\}\n`).MatchString(hostPart) {
		r.Bad("host_free()'s definition is not the empty body this phase depends on")
	}
	if !strings.Contains(c.Old, "vim_free(void *x)\n{\n    if (x != nullptr && !really_exiting)\n    {\n        host_free(x);\n    }\n}\n") {
		r.Bad("vim_free() is not the NULL test around host_free() this phase depends on")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("host_free() has an empty body and vim_free() only calls it: no call to either does anything")

	// 2. the output, computed from the input
	var want bytes.Buffer
	core := w132Core(c.Old)
	for _, line := range strings.SplitAfter(core, "\n") {
		m := w132CallC.FindStringSubmatch(strings.TrimRight(line, "\n"))
		if m == nil {
			want.WriteString(line)
			continue
		}
		repl, ok := W132Rule(m[1], m[3])
		if !ok {
			r.Bad("the input has a call the rule refuses: %s", strings.TrimSpace(line))
			continue
		}
		want.WriteString(repl)
	}
	// the rule applied to the input, the locals left only given values taken
	// with their stores (edit.DeadStores, the same function), and the host's
	// calls rerouted, is the edit; what the sweep then takes is the sweep's own
	// business -- vim_free(), and every local that existed only to be freed --
	// so the output is computed through the sweep itself, on a copy
	stored, _ := edit.DeadStores(want.Bytes())
	pre := string(stored) + strings.ReplaceAll(c.Old[len(core):], "vim_free(", "host_free(")
	comp, err := check.Swept(pre)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	if comp != c.New {
		l, a, b := check.FirstDiff(comp, c.New)
		r.Bad("the output is not the input with the rule applied and swept: they part at line %d\n    computed: %q\n    output:   %q", l, a, b)
	}
	took := check.SweptLines(pre, c.New)
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the output IS the input with every call to vim_free() and host_free() in the core replaced by the rule, the locals only given values taken with their stores, and the host's three vim_free() calls made host_free() calls, then swept: computed, byte for byte")
	r.Say("the sweep took, beyond the calls, %d lines: %s", len(took), strings.Join(took, " | "))

	newCore_ := w132Core(c.New)
	if n := check.Word(c.New, "vim_free"); n != 0 {
		r.Bad("vim_free has %d mentions left", n)
	}
	if ls := check.LinesWith(newCore_, "host_free"); len(ls) != 1 || ls[0] != "static void host_free(void *p);" {
		r.Bad("host_free above the boundary is not its prototype alone: %q", ls)
	}
	oldEmpty := len(w132Empty.FindAllStringIndex(core, -1))
	newEmpty := len(w132Empty.FindAllStringIndex(newCore_, -1))
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("nothing above the boundary frees: vim_free is gone and host_free is the prototype the host's formatter needs; %d blocks are left empty that were not (%d -> %d) -- the next phase's to fold", newEmpty-oldEmpty, oldEmpty, newEmpty)
	return c.Gate(true)
}
