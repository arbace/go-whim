package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim143", Whim143) }

// w143Lines is a function's lines, trimmed, without the blank ones.
func w143Lines(s string) []string {
	var r []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			r = append(r, t)
		}
	}
	return r
}

// w143Diff is a line diff by longest common subsequence: what a lost and what
// b gained, as multisets.
func w143Diff(a, b []string) (gone, added map[string]int) {
	n, m := len(a), len(b)
	l := make([][]int32, n+1)
	for i := range l {
		l[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				l[i][j] = l[i+1][j+1] + 1
			} else if l[i+1][j] >= l[i][j+1] {
				l[i][j] = l[i+1][j]
			} else {
				l[i][j] = l[i][j+1]
			}
		}
	}
	gone, added = map[string]int{}, map[string]int{}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			i++
			j++
		case l[i+1][j] >= l[i][j+1]:
			gone[a[i]]++
			i++
		default:
			added[b[j]]++
			j++
		}
	}
	for ; i < n; i++ {
		gone[a[i]]++
	}
	for ; j < m; j++ {
		added[b[j]]++
	}
	return gone, added
}

// Whim143 is phase 143's check: regatom() has no goto.
//
//  1. THE HELPER IS THE BLOCK: regatom_delim()'s body, between its `ret`
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
func Whim143(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim143", "regatom")
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
	in, out, helper := fn(c.old, "regatom"), fn(c.new, "regatom"), fn(c.new, "regatom_delim")
	if in == "" || out == "" || helper == "" {
		r.bad("regatom() or regatom_delim() is not defined")
		return r.done()
	}
	// 1. the block
	bi := strings.Index(in, "delimiter_atom:\n")
	if bi < 0 {
		r.bad("the input has no delimiter_atom block")
		return r.done()
	}
	ob := bi + strings.Index(in[bi:], "{")
	cb := cutil.Match(cutil.Blank([]byte(in)), ob)
	block := w143Lines(in[ob+1 : cb])
	hb := w143Lines(helper)
	// head (two lines), `{`, `char_u *ret;`, the block, `return ret;`, `}`
	if len(hb) < 5 || strings.Join(hb[4:len(hb)-2], "\n") != strings.Join(block, "\n") || hb[len(hb)-2] != "return ret;" {
		r.bad("regatom_delim()'s body is not the input's delimiter block")
	}
	calls := regexp.MustCompile(`\bregatom_delim\(c, delim_nl, flagp\)`)
	if n := len(calls.FindAllString(out, -1)); n != 3 {
		r.bad("regatom() calls regatom_delim() %d times, where the block's case and the two jumps are 3", n)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("regatom_delim() is the input's delimiter block, line for line, and regatom() calls it from the block's case and both jumps")

	// 2. the partition
	gone, added := w143Diff(w143Lines(in), w143Lines(out))
	// a line the diff sees leave and come back is the alignment's choice
	// around the moved block, not a change: the pair cancels
	for l, k := range gone {
		if a := added[l]; a > 0 {
			m := min(k, a)
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
		r.bad("regatom() lost a line this phase does not account for: %q (x%d)", l, k)
	}
	for l, k := range added {
		if k > 0 && !okAdded[l] {
			r.bad("regatom() gained a line this phase does not account for: %q", l)
		}
	}
	if strings.Contains(out, "goto ") {
		r.bad("regatom() still jumps")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("regatom()'s line diff is a partition: it lost labels, jumps, braces, `switch (c)` and the moved block, and gained the dispatch loop, the helper's calls and the multibyte node; no goto left")
	if err := c.gate(true); err != nil {
		return err
	}

	ob2, nb := c.bins()
	seed := []byte("if(a(b)c)d[e]f{g}h<i>j\rsecond(x\ry)z\ré\xcc\x81x\x1bgg")
	probes := []struct{ what, keys, ctl string }{
		{"\\%)", ":1s/(\\%)/<&>/\r", ":1s/Q\\%)/<&>/\r"},
		{"\\%t)", ":1s/(\\%t)/<&>/\r", ":1s/Q\\%t)/<&>/\r"},
		{"\\%f]", ":1s/\\[\\%f]/<&>/\r", ":1s/Q\\%f]/<&>/\r"},
		{"\\%>", ":1s/<\\%>/<&>/\r", ":1s/Q\\%>/<&>/\r"},
		{"\\_%)", ":2s/(\\_%)/<&>/\r", ":2s/Q\\_%)/<&>/\r"},
		{"\\_[", ":1s/\\_[a-c]\\+/<&>/g\r", ":1s/\\_[Q]\\+/<&>/g\r"},
		{". before a composing character", ":4s/.\xcc\x81/<&>/\r", ":4s/Q\xcc\x81/<&>/\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr.keys), []byte(":q!\r")}
		s1, _, e1 := stream(ob2, keys, nil)
		s2, _, e2 := stream(nb, keys, nil)
		s3, _, e3 := stream(nb, [][]byte{seed, []byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.bad("%s is written differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("%s", "PROBE: \\%) \\%t) \\%f] \\%> \\_%) \\_[ and `.` before a composing character substitute the same on both binaries; each CONTROL moves")
	return nil
}
