package p143

// Whim phase 143, the check -- regatom() has no goto.
// See phase/143/edit.go, and GOALS.md.
//
// phase/143/check.go requires the helper to be the input's block, the
// line diff of regatom() to be exactly the accounted changes, and probes every
// path the rewrite touched.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim143", Check) }

// Whim143 is phase 143's check: regatom() has no goto.
//
//  1. THE HELPER IS THE BLOCK: regatom_delim()'s Body, between its `ret`
//     declaration and its `return ret;`, is the input's delimiter_atom block,
//     line for line; it is called three times, from the block's case and the
//     two jumps.
//  2. NOTHING ELSE MOVED: a line diff of regatom() from the input to the
//     output is a partition -- every line it lost is a label, a jump, the
//     block that moved, a brace or the old `switch (c)`, and every line it
//     gained is the dispatch loop, a call to the helper with its NULL check,
//     the multibyte node written at the jump, or a brace.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES, one per path the rewrite touched: \%) \%t) \%f) \%> and
//     \_%) (the delimiter atom, from its case and both jumps), \_[ (the
//     collection, dispatched again), and `.` before a composing character (the
//     multibyte node).  Each substitutes the same on both binaries, and each
//     CONTROL, the same pattern made not to match, moves.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim143", "regatom")
	if err != nil {
		return err
	}
	r := c.R
	fn := func(text, name string) string {
		b := []byte(text)
		a, z, ok := cutil.FindDefinition(b, cutil.Blank(b), name)
		if !ok {
			return ""
		}
		return text[a:z]
	}
	in, Out, helper := fn(c.Old, "regatom"), fn(c.New, "regatom"), fn(c.New, "regatom_delim")
	if in == "" || Out == "" || helper == "" {
		r.Bad("regatom() or regatom_delim() is not defined")
		return r.Done()
	}
	// 1. the block
	bi := strings.Index(in, "delimiter_atom:\n")
	if bi < 0 {
		r.Bad("the input has no delimiter_atom block")
		return r.Done()
	}
	ob := bi + strings.Index(in[bi:], "{")
	cb := cutil.Match(cutil.Blank([]byte(in)), ob)
	block := check.W143Lines(in[ob+1 : cb])
	hb := check.W143Lines(helper)
	// head (two lines), `{`, `char_u *ret;`, the block, `return ret;`, `}`
	if len(hb) < 5 || strings.Join(hb[4:len(hb)-2], "\n") != strings.Join(block, "\n") || hb[len(hb)-2] != "return ret;" {
		r.Bad("regatom_delim()'s body is not the input's delimiter block")
	}
	calls := regexp.MustCompile(`\bregatom_delim\(c, delim_nl, flagp\)`)
	if n := len(calls.FindAllString(Out, -1)); n != 3 {
		r.Bad("regatom() calls regatom_delim() %d times, where the block's case and the two jumps are 3", n)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("regatom_delim() is the input's delimiter block, line for line, and regatom() calls it from the block's case and both jumps")

	// 2. the partition
	gone, added := check.W143Diff(check.W143Lines(in), check.W143Lines(Out))
	// a line the diff sees leave and come back is the alignment's choice
	// around the moved block, not a change: the pair cancels
	for l, k := range gone {
		if a := added[l]; a > 0 {
			m := check.Min(k, a)
			gone[l] -= m
			added[l] -= m
		}
	}
	blockSet := map[string]int{}
	for _, l := range block {
		blockSet[l]++
	}
	okGone := map[string]bool{"delimiter_atom:": true, "collection:": true, "do_multibyte:": true,
		"goto delimiter_atom;": true, "goto collection;": true, "goto do_multibyte;": true,
		"switch (c)": true, "{": true, "}": true}
	okAdded := map[string]bool{"int             sw;": true, "sw = c;": true, "for (;;)": true, "switch (sw)": true,
		"{": true, "}": true, "break;": true, "continue;": true, "sw = ((int)('[') - 256);": true,
		"ret = regatom_delim(c, delim_nl, flagp);": true, "if (ret == nullptr)": true, "return nullptr;": true,
		"ret = regnode(MULTIBYTECODE);": true, "regmbc(c);": true, "*flagp |= HASWIDTH | SIMPLE;": true}
	for l, k := range gone {
		if k == 0 || okGone[l] {
			continue
		}
		if blockSet[l] >= k {
			continue
		}
		r.Bad("regatom() lost a line this phase does not account for: %q (x%d)", l, k)
	}
	for l, k := range added {
		if k > 0 && !okAdded[l] {
			r.Bad("regatom() gained a line this phase does not account for: %q", l)
		}
	}
	if strings.Contains(Out, "goto ") {
		r.Bad("regatom() still jumps")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("regatom()'s line diff is a partition: it lost labels, jumps, braces, `switch (c)` and the moved block, and gained the dispatch loop, the helper's calls and the multibyte node; no goto left")
	if err := c.Gate(true); err != nil {
		return err
	}

	ob2, nb := c.Bins()
	seed := []byte("if(a(b)c)d[e]f{g}h<i>j\rsecond(x\ry)z\ré\xcc\x81x\x1bgg")
	probes := []struct{ What, Keys, ctl string }{
		{"\\%)", ":1s/(\\%)/<&>/\r", ":1s/Q\\%)/<&>/\r"},
		{"\\%t)", ":1s/(\\%t)/<&>/\r", ":1s/Q\\%t)/<&>/\r"},
		{"\\%f]", ":1s/\\[\\%f]/<&>/\r", ":1s/Q\\%f]/<&>/\r"},
		{"\\%>", ":1s/<\\%>/<&>/\r", ":1s/Q\\%>/<&>/\r"},
		{"\\_%)", ":2s/(\\_%)/<&>/\r", ":2s/Q\\_%)/<&>/\r"},
		{"\\_[", ":1s/\\_[a-c]\\+/<&>/g\r", ":1s/\\_[Q]\\+/<&>/g\r"},
		{". before a composing character", ":4s/.\xcc\x81/<&>/\r", ":4s/Q\xcc\x81/<&>/\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr.Keys), []byte(":q!\r")}
		s1, _, e1 := check.Stream(ob2, keys, nil)
		s2, _, e2 := check.Stream(nb, keys, nil)
		s3, _, e3 := check.Stream(nb, [][]byte{seed, []byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.Say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.Bad("%s is written differently on the two binaries", pr.What)
		}
		if s3 == s2 {
			r.Bad("the CONTROL for %s did not move", pr.What)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("%s", "PROBE: \\%) \\%t) \\%f] \\%> \\_%) \\_[ and `.` before a composing character substitute the same on both binaries; each CONTROL moves")
	return nil
}
