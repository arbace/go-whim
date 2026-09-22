package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim144", Whim144) }

// Whim144 is phase 144's check: edit() has no goto.
//
//  1. THE HELPERS: edit_esc() and edit_normalchar() are edit.W144Helpers,
//     once; the input's doESCkey and normalchar blocks are what they were
//     made from, line for line but for the locals passed by pointer.
//  2. EVERY JUMP STILL GOES WHERE IT WENT: each `continue;` after an
//     edit_esc() call has the main for (;;) as its innermost loop -- the next
//     key, as the label's own continue meant -- and each `break;` after an
//     edit_normalchar() call leaves the main switch (c), as the label's did.
//     The do-while's esc_now `break;` has the do-while as its innermost loop,
//     and the flag is tested right after it.
//  3. NOTHING ELSE MOVED: the line diff of edit() is a partition into what
//     the phase accounts for (a moved line's leave-and-return cancels).
//  4. THE GATE, the libc surface unchanged.
//  5. THE PROBES: every key whose case jumped that a terminal can deliver --
//     Esc, CTRL-O, Tab, CTRL-K, CTRL-], CTRL-F, CTRL-S, CTRL-L, CTRL-Z, CTRL-A
//     and Enter -- in Insert mode, the same on both binaries, each CONTROL
//     moving.  CTRL-C cannot be probed: on the harness's pty it is SIGINT, the
//     read fails and both binaries exit before drawing (measured).  The
//     do-while's site needs stop_insert_mode, which no key sets, and do_intr's
//     an interrupt character other than CTRL-C: nothing here reaches those
//     three, and point 2 is their evidence.
func Whim144(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim144", "editgoto")
	if err != nil {
		return err
	}
	r := c.r
	fn := func(text, name string) string {
		b := []byte(text)
		a, z, ok := cutil.FindDefinition(b, cutil.Blank(b), name)
		if !ok {
			return ""
		}
		return text[a:z]
	}
	in, out := fn(c.old, "edit"), fn(c.new, "edit")
	if in == "" || out == "" || strings.Count(c.new, edit.W144Helpers) != 1 {
		r.bad("edit() or the helpers are not as this phase writes them")
		return r.done()
	}
	// 1. the helpers are the blocks
	esc := w143Lines(fn(c.new, "edit_esc"))
	blockOf := func(label, end string) []string {
		i := strings.Index(in, label+":\n")
		if i < 0 {
			return nil
		}
		j := strings.Index(in[i:], end)
		return w143Lines(in[i+len(label)+2 : i+j])
	}
	eb := blockOf("doESCkey", "            continue;\n")
	norm := w143Lines(fn(c.new, "edit_normalchar"))
	nb := blockOf("normalchar", "            break;\n")
	un := strings.NewReplacer("*o_lnum = ", "o_lnum = ", "ins_esc(count,", "ins_esc(&count,", "return TRUE;", "return (c == Ctrl_O);", "*inserted_space = ", "inserted_space = ")
	strip := func(ls []string) string {
		// head, `{` ... `}`; edit_esc's trailing `return FALSE;`
		body := ls[3 : len(ls)-1]
		if body[len(body)-1] == "return FALSE;" {
			body = body[:len(body)-1]
		}
		return un.Replace(strings.Join(body, "\n"))
	}
	if eb == nil || strip(esc) != strings.Join(eb, "\n") {
		r.bad("edit_esc() is not the input's doESCkey block with the locals passed by pointer")
	}
	if nb == nil || strip(norm) != strings.Join(nb, "\n") {
		r.bad("edit_normalchar() is not the input's normalchar block with inserted_space passed by pointer")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("edit_esc() and edit_normalchar() are the input's doESCkey and normalchar blocks, with the locals they wrote passed by pointer")

	// 2. where each continue and break goes
	escs, norms := 0, 0
	for _, m := range regexp.MustCompile(`if \(edit_esc\(&count, cmdchar, nomove, &o_lnum\)\)\n *\{\n *return \(c == Ctrl_O\);\n *\}\n *continue;\n`).FindAllStringIndex(out, -1) {
		loop, _ := edit.W144Inner(edit.W144Enclosing(out, m[1]-len("continue;\n")))
		if loop != "for (;;)" {
			r.bad("a continue after edit_esc() has %q as its innermost loop", loop)
		}
		escs++
	}
	for _, m := range regexp.MustCompile(`edit_normalchar\(c, &inserted_space\);\n *break;\n`).FindAllStringIndex(out, -1) {
		_, brk := edit.W144Inner(edit.W144Enclosing(out, m[1]-len("break;\n")))
		if brk != "switch (c)" {
			r.bad("a break after edit_normalchar() leaves %q", brk)
		}
		norms++
	}
	dm := regexp.MustCompile(`esc_now = TRUE;\n *break;\n`).FindAllStringIndex(out, -1)
	if len(dm) != 1 {
		r.bad("esc_now is set %d times", len(dm))
	} else {
		loop, brk := edit.W144Inner(edit.W144Enclosing(out, dm[0][1]-len("break;\n")))
		after := out[dm[0][1]:]
		wi := regexp.MustCompile(`\n *\} while \(.*\);\n *if \(esc_now\)\n`).FindStringIndex(after)
		if loop != "do" || brk != "do" || wi == nil {
			r.bad("the esc_now break does not leave the do-while straight to the flag's test")
		}
	}
	// the jumps were 9 and 7 and the ESC case's own block is one more edit_esc
	// call; do_intr's copy another
	if escs != 11 || norms != 8 {
		r.bad("edit_esc() is reached %d times and edit_normalchar() %d, where 9 jumps less the do-while's, the Esc case, do_intr's copy and the flag's test are 11, and 7 jumps and the default case are 8", escs, norms)
	}
	if strings.Contains(out, "goto ") {
		r.bad("edit() still jumps")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("every continue after edit_esc() is the main loop's and every break after edit_normalchar() leaves the main switch, as the labels' own did; the do-while's jump leaves it straight to the flag")

	// 3. the partition
	gone, added := w143Diff(w143Lines(in), w143Lines(out))
	for l, k := range gone {
		if a := added[l]; a > 0 {
			m := min(k, a)
			gone[l] -= m
			added[l] -= m
		}
	}
	okGone := map[string]bool{"do_intr:": true, "doESCkey:": true, "normalchar:": true,
		"goto doESCkey;": true, "goto normalchar;": true, "goto do_intr;": true, "{": true, "}": true}
	for _, l := range append(eb, nb...) {
		okGone[l] = true
	}
	okAdded := map[string]bool{"int         esc_now = FALSE;": true, "esc_now = TRUE;": true, "esc_now = FALSE;": true,
		"if (esc_now)": true, "if (edit_esc(&count, cmdchar, nomove, &o_lnum))": true, "return (c == Ctrl_O);": true,
		"continue;": true, "break;": true, "edit_normalchar(c, &inserted_space);": true, "{": true, "}": true}
	for _, l := range w143Lines(strings.SplitN(in[strings.Index(in, "do_intr:\n"):], "doESCkey:", 2)[0]) {
		okAdded[l] = true
	}
	for l, k := range gone {
		if k > 0 && !okGone[l] {
			r.bad("edit() lost a line this phase does not account for: %q", l)
		}
	}
	for l, k := range added {
		if k > 0 && !okAdded[l] {
			r.bad("edit() gained a line this phase does not account for: %q", l)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("edit()'s line diff is a partition: labels, jumps and the two moved blocks out; the calls, their continue and break, the flag and do_intr's copy in")
	if err := c.gate(true); err != nil {
		return err
	}

	ob, nbin := c.bins()
	probes := []struct{ what, keys, ctl string }{
		{"Esc", "ione two\x1b", "ione Two\x1b"},
		{"CTRL-O", "iab\x0f0X\x1b", "iab\x0f0Y\x1b"},
		{"Tab", "ia\tb\x1b", "ia\tc\x1b"},
		{"CTRL-K", "ia\x0bb\x1b", "ia\x0bc\x1b"},
		{"CTRL-]", "ia\x1db\x1b", "ia\x1dc\x1b"},
		{"CTRL-F", "ia\x06b\x1b", "ia\x06c\x1b"},
		{"CTRL-S", "ia\x13b\x1b", "ia\x13c\x1b"},
		{"CTRL-L", "ia\x0cb\x1b", "ia\x0cc\x1b"},
		{"CTRL-Z", "ia\x1ab\x1b", "ia\x1ac\x1b"},
		{"CTRL-A", "iab\x1bo\x01\x1b", "iac\x1bo\x01\x1b"},
		{"Enter", "ia\rb\x1b", "ia\rc\x1b"},
	}
	chunks := func(s string) [][]byte { return [][]byte{[]byte(s), []byte(":q!\r")} }
	for _, pr := range probes {
		keys := chunks(pr.keys)
		s1, _, e1 := stream(ob, keys, nil)
		s2, _, e2 := stream(nbin, keys, nil)
		s3, _, e3 := stream(nbin, chunks(pr.ctl), nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.bad("%s in Insert mode is written differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: Esc, CTRL-O, Tab, CTRL-K, CTRL-], CTRL-F, CTRL-S, CTRL-L, CTRL-Z, CTRL-A and Enter in Insert mode write the same bytes on both binaries; each CONTROL moves")
	return nil
}
