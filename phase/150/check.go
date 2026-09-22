package p150

// Whim phase 150, the check -- the regexp stack is three typed stacks.
// See phase/150/edit.go, and GOALS.md.
//
// phase/150/check.go requires the same byte adjustments, nothing reading
// the stack as bytes, regexp probes against controls, and E363 at the exact
// 'maxmempattern' the input's binary reaches it.

import (
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim150", Check) }

// Whim150 is phase 150's check: the regexp stack is three typed stacks.
//
//  1. THE ACCOUNTING IS THE SAME: every `regstack.ga_len += / -= sizeof(T)` of
//     the input is a `regstack_bytes += / -= sizeof(T)` of the output, as a
//     multiset of (operation, T); each of the output's sits beside a push or
//     pop of T's own stack (`++`/`--` of it, or the record stack's).
//  2. NOTHING READS THE STACK AS BYTES: no `(char *)regstack.ga_data`, no
//     `((regstar_T *)rp) - 1` or `((regbehind_T *)rp) - 1`; the two accessors
//     are W150Tops.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: stars, a lazy count, positive and negative look-behind and
//     a complex star substitute the same on both binaries, each CONTROL
//     moving.  And 'maxmempattern' is found, by bisection on the input's
//     binary, at the exact value where a complex star over a 600-character
//     line stops failing with E363: the output's binary must fail at one less
//     and not at that value -- the byte accounting measured, not argued.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim150", "regstack")
	if err != nil {
		return err
	}
	r := c.R
	ops := func(text, lhs string) []string {
		var o []string
		for _, m := range regexp.MustCompile(regexp.QuoteMeta(lhs)+` ([+-])= sizeof\((\w+)\);`).FindAllStringSubmatch(text, -1) {
			o = append(o, m[1]+m[2])
		}
		sort.Strings(o)
		return o
	}
	in, Out := ops(c.Old, "regstack.ga_len"), ops(c.New, "regstack_bytes")
	if strings.Join(in, " ") != strings.Join(Out, " ") || len(in) != 9 {
		r.Bad("the byte accounting moved: the input's %v, the output's %v", in, Out)
	}
	stackOf := map[string]string{"regitem_T": "regstack", "regstar_T": "regstack_star", "regbehind_T": "regstack_behind"}
	lines := strings.Split(c.New, "\n")
	for k, l := range lines {
		m := regexp.MustCompile(`regstack_bytes ([+-])= sizeof\((\w+)\);`).FindStringSubmatch(l)
		if m == nil || k == 0 {
			continue
		}
		prev := strings.TrimSpace(lines[k-1])
		want := "++" + stackOf[m[2]] + ".ga_len;"
		if m[1] == "-" {
			want = "--" + stackOf[m[2]] + ".ga_len;"
		}
		if prev != want {
			r.Bad("%s of a %s does not follow %q but %q", m[1], m[2], want, prev)
		}
	}
	for _, bad := range []string{"(char *)regstack.ga_data", "((regstar_T *)rp) - 1", "((regbehind_T *)rp) - 1", "regstack.ga_len -= sizeof", "regstack.ga_len += sizeof"} {
		if strings.Contains(c.New, bad) {
			r.Bad("the stack is still read as bytes: %q", bad)
		}
	}
	if !strings.Contains(c.New, W150Tops) {
		r.Bad("the accessors are not the ones this phase writes")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("each of the 9 byte adjustments of the input is the same adjustment of regstack_bytes, beside a push or pop of its own typed stack; nothing reads the stack as bytes")
	if err := c.Gate(true); err != nil {
		return err
	}

	ob, nb := c.Bins()
	seed := []byte("iababab c\rfoobar xbar\raaaaab\rc" + strings.Repeat("ab", 20) + "c\x1bgg")
	probes := []struct{ What, Keys, ctl string }{
		{"a star", ":1s/\\(ab\\)*/<&>/\r", ":1s/\\(ba\\)*/<&>/\r"},
		{"a lazy count", ":3s/a\\{-2,}/<&>/\r", ":3s/a\\{-3,}/<&>/\r"},
		{"a look-behind", ":2s/\\(foo\\)\\@<=bar/<&>/\r", ":2s/\\(fox\\)\\@<=bar/<&>/\r"},
		{"a negative look-behind", ":2s/\\(foo\\)\\@<!bar/<&>/\r", ":2s/\\(fox\\)\\@<!bar/<&>/\r"},
		{"a complex star", ":4s/a\\(a\\|b\\)*c/<&>/\r", ":4s/a\\(a\\|c\\)*c/<&>/\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr.Keys), []byte(":q!\r")}
		s1, _, e1 := check.Stream(ob, keys, nil)
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
	r.Say("PROBE: a star, a lazy count, a look-behind, a negative one and a complex star substitute the same on both binaries; each CONTROL moves")

	e363 := func(bin string, mmp int) (bool, string) {
		keys := [][]byte{[]byte("ic" + strings.Repeat("ab", 300) + "\x1b0"),
			[]byte(":set mmp=" + strconv.Itoa(mmp) + "\r:s/a\\(a\\|b\\)*c/X/\r"), []byte(":q!\r")}
		_, o, _, _, _ := harness.ZSession(bin, keys, "xterm", nil, 24, 80, 30*time.Second)
		return strings.Contains(string(o), "E363"), string(o)
	}
	lo, hi := 1, 1000
	if a, _ := e363(ob, lo); !a {
		r.Bad("the input's binary does not fail with E363 at 'maxmempattern' 1, so the bisection has no bracket")
	}
	if b, _ := e363(ob, hi); b {
		r.Bad("the input's binary fails with E363 at 'maxmempattern' 1000, so the bisection has no bracket")
	}
	if err := r.Done(); err != nil {
		return err
	}
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if f, _ := e363(ob, mid); f {
			lo = mid
		} else {
			hi = mid
		}
	}
	nlo, olo := e363(nb, lo)
	nhi, ohi := e363(nb, hi)
	_, iolo := e363(ob, lo)
	_, iohi := e363(ob, hi)
	if !nlo || nhi || olo != iolo || ohi != iohi {
		r.Bad("'maxmempattern' %d/%d: the output's binary fails with E363 %v/%v where the input's fails true/false, or draws differently", lo, hi, nlo, nhi)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("'maxmempattern': the input's binary stops failing with E363 at %d KB for a complex star over 600 characters; the output's fails at %d and not at %d, drawing the same bytes at both", hi, lo, hi)
	return nil
}
