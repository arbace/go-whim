package check

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim141", Whim141) }

var (
	w141Pair = regexp.MustCompile(`case ([A-Z_]+):\n\s*case ([A-Z_]+) \+ ADD_NL:\n\s*((?:testval = )?mask = RI_[A-Z]+;)\n\s*(goto do_class;|break;|do_class:)`)
)

// w141Table is opcode -> the assignment its class makes before the loop, read
// from one of the two shapes: `case X: case X + ADD_NL: <assign>; goto
// do_class;` (or the \s case, whose assignment is followed by the label), and
// the output's inner `case X: case X + ADD_NL: <assign>; break;`.
func w141Table(fn string) map[string]string {
	t := map[string]string{}
	for _, m := range w141Pair.FindAllStringSubmatch(fn, -1) {
		if m[1] == m[2] {
			t[m[1]] = m[3]
		}
	}
	return t
}

// w141Loop is the class loop: from its `while (count < maxcount)` to the
// `break;` that ends the case.
func w141Loop(fn string) string {
	a := strings.Index(fn, "        while (count < maxcount)\n        {\n            int         l;\n")
	if a < 0 {
		return ""
	}
	z := strings.Index(fn[a:], "\n        }\n        break;\n")
	if z < 0 {
		return ""
	}
	return fn[a : a+z]
}

// Whim141 is phase 141's check: regrepeat() does not jump into a case.
//
//  1. THE SAME ASSIGNMENTS: read from the input, each of the 18 class opcodes
//     sets mask (and testval, for the classes that match rather than exclude)
//     and reaches the class loop; read from the output, each is a case of the
//     inner switch making the same assignment.  The two tables are equal, and
//     have 18 entries.
//  2. THE SAME LOOP, byte for byte, and no do_class, goto or label, left.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE: every one of the 18 classes with a multi, \s\+ to \U\+, each
//     substituting on its own line, writes the same bytes on both binaries;
//     the CONTROL swaps one class for its complement and moves.
func Whim141(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim141", "doclass")
	if err != nil {
		return err
	}
	r := c.r
	fnOf := func(text string) string {
		b := []byte(text)
		a, z, ok := cutil.FindDefinition(b, cutil.Blank(b), "regrepeat")
		if !ok {
			return ""
		}
		return text[a:z]
	}
	in, out := fnOf(c.old), fnOf(c.new)
	if in == "" || out == "" {
		r.bad("regrepeat() is not defined on both sides")
		return r.done()
	}
	ti, to := w141Table(in), w141Table(out)
	if len(ti) != 18 || fmt.Sprint(ti) != fmt.Sprint(to) {
		var d []string
		for k, v := range ti {
			if to[k] != v {
				d = append(d, fmt.Sprintf("%s: %q -> %q", k, v, to[k]))
			}
		}
		sort.Strings(d)
		r.bad("the class assignments differ (%d on the input, %d on the output): %v", len(ti), len(to), d)
	}
	if gi := strings.Count(in, "goto do_class;"); gi != 17 {
		r.bad("the input jumps to do_class %d times, and this phase was written against 17", gi)
	}
	li, lo := w141Loop(in), w141Loop(out)
	if li == "" || li != lo {
		r.bad("the class loop is not the input's, byte for byte")
	}
	if strings.Contains(out, "do_class") || strings.Contains(out, "goto ") {
		r.bad("regrepeat() still names do_class or jumps")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("each of the 18 class opcodes makes the assignment it made, in a switch rather than before a jump; the loop is the input's; regrepeat() has no goto left")
	if err := c.gate(true); err != nil {
		return err
	}

	ob, nb := c.bins()
	var seed strings.Builder
	seed.WriteString("i")
	for k := 0; k < 18; k++ {
		seed.WriteString("ab 12 Xy_z 0x7f\r")
	}
	seed.WriteString("\x1b")
	classes := "sSdDxXoOwWhHaAlLuU"
	sub := func(cls string) []byte {
		var b strings.Builder
		for k, ch := range cls {
			fmt.Fprintf(&b, ":%ds/\\%c\\+/<&>/g\r", k+1, ch)
		}
		return []byte(b.String())
	}
	keys := [][]byte{[]byte(seed.String()), sub(classes), []byte("gg")}
	ctl := [][]byte{[]byte(seed.String()), sub("S" + classes[1:]), []byte("gg")}
	s1, _, e1 := stream(ob, append(keys, []byte(":q!\r")), nil)
	s2, _, e2 := stream(nb, append(keys, []byte(":q!\r")), nil)
	s3, _, e3 := stream(nb, append(ctl, []byte(":q!\r")), nil)
	if e1 != nil || e2 != nil || e3 != nil {
		r.say("a probe did not run: %v %v %v", e1, e2, e3)
		return harness.ErrReported
	}
	if s1 != s2 {
		r.bad("the 18 class substitutions are written differently on the two binaries")
	}
	if s3 == s2 {
		r.bad("the CONTROL, \\S for \\s on the first line, did not move")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: \\s\\+ through \\U\\+, one per line, substitute the same on both binaries (%s); the CONTROL, \\S for \\s, moves", s2)
	return nil
}
