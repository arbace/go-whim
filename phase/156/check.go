package p156

// Whim phase 156, the check -- the regex size pass's node is a static byte, not (char_u *) -1.
// See phase/156/edit.go, and GOALS.md.
//
// phase/156/check.go requires every use of the sentinel to be a
// comparison or the one assignment, before and after, no integer cast to a
// pointer left, and probes every node-making path of the regex compiler.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim156", Check) }

// Whim156 is phase 156's check: the size pass's node is a real byte.
//
//  1. THE PREMISE, on the input: the sentinel is written 14 times, and every
//     one is a comparison (== or !=) or the one assignment to regcode in
//     bt_regcomp(): it is never dereferenced, only compared.
//  2. THE CUT: no (char_u *) -1 is left; reg_calc_size_node is its declaration
//     and those 14 uses, each still a comparison or that assignment; and
//     internal/ccx's Casts finds no integer cast to a pointer.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: patterns that exercise every node-making path of the
//     compiler -- branches, counted and lazy repeats, a look-behind, a
//     collection, back-references -- substitute the same on both binaries,
//     each CONTROL moving.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim156", "calcsize")
	if err != nil {
		return err
	}
	r := c.R
	use := regexp.MustCompile(`(==|!=)\s*` + regexp.QuoteMeta(W156Sentinel) + `|regcode =\s*` + regexp.QuoteMeta(W156Sentinel) + `\s*;`)
	if n, k := strings.Count(c.Old, W156Sentinel), len(use.FindAllString(c.Old, -1)); n != 14 || k != 14 {
		r.Bad("the input's sentinel is written %d times, %d of them compared or assigned to regcode", n, k)
	}
	if strings.Contains(c.New, W156Sentinel) {
		r.Bad("the sentinel is still written")
	}
	nuse := regexp.MustCompile(`(==|!=)\s*reg_calc_size_node\b|regcode =\s*reg_calc_size_node\s*;`)
	if n, k := check.Word(c.New, "reg_calc_size_node"), len(nuse.FindAllString(c.New, -1)); n != 15 || k != 14 {
		r.Bad("reg_calc_size_node is named %d times and compared or assigned %d times, where its declaration and 14 uses are 15 and 14", n, k)
	}
	ast, err := check.ParseCore(c.New)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	for _, f := range ccx.Casts(ast).Left {
		if strings.Contains(f.What, "of a int") {
			r.Bad("an integer is cast to a pointer: %s %s", f.Where, f.What)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the sentinel, only ever compared or assigned, is the address of a static byte at all 14 uses; no integer is cast to a pointer")
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	seed := []byte("iabab c foobar ab1ab1 [x] aaa\x1b0")
	probes := []struct{ What, Keys, ctl string }{
		{"a branch", ":s/b\\|c/<&>/g\r", ":s/b\\|d/<&>/g\r"},
		{"a counted repeat", ":s/\\(ab\\)\\{2}/<&>/\r", ":s/\\(ab\\)\\{3}/<&>/\r"},
		{"a lazy repeat", ":s/a\\{-1,}/<&>/g\r", ":s/b\\{-1,}/<&>/g\r"},
		{"a look-behind", ":s/\\(foo\\)\\@<=bar/<&>/\r", ":s/\\(fox\\)\\@<=bar/<&>/\r"},
		{"a collection", ":s/[a-c]\\+/<&>/g\r", ":s/[x-z]\\+/<&>/g\r"},
		{"a back-reference", ":s/\\(ab1\\)\\1/<&>/\r", ":s/\\(ab\\)\\1/<&>/\r"},
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
			r.Bad("%s is compiled and matched differently on the two binaries", pr.What)
		}
		if s3 == s2 {
			r.Bad("the CONTROL for %s did not move", pr.What)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: a branch, counted and lazy repeats, a look-behind, a collection and a back-reference substitute the same on both binaries; each CONTROL moves")
	return nil
}
