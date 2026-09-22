package p151

// Whim phase 151, the check -- the option table's defaults are typed.
// See phase/151/edit.go, and GOALS.md.
//
// phase/151/check.go computes every row's two pairs from the input's,
// requires no def_val field left, and probes every option's default shown.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim151", Check) }

// Whim151 is phase 151's check: the option table's defaults are typed.
//
//  1. EVERY ROW, COMPUTED: each input row's default pair, split by the row's
//     kind with W151Pair, is the output row's two pairs, in order, row
//     for row -- so every number default is the number it was, every string
//     the string, and a string row's NULL is nullptr where it was 0L.
//  2. NO def_val FIELD IS LEFT (the local variable of that name in
//     set_option_default() is a local), and no number is cast to a pointer
//     to be stored.
//  3. THE GATE, the libc surface unchanged: the silent compile is itself the
//     proof that no read took a string where a number is, or the reverse.
//  4. THE PROBES: every option's default, set and shown -- `:set all&` then
//     `:set all` paged through -- and the terminal options' defaults
//     (`:set termcap`), the same on both binaries; the CONTROL, one option
//     changed before they are shown, moves.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim151", "defaults")
	if err != nil {
		return err
	}
	r := c.R
	in, _, err := W151Rows(c.Old)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	// the output's rows: the same walk, where each row now ends in two pairs
	head := "static struct vimoption options[] =\n{\n"
	i := strings.Index(c.New, head)
	b := cutil.Blank([]byte(c.New))
	open := i + len(head) - 2
	end := cutil.Match(b, open)
	var outPairs []string
	for k := open + 1; k < end; k++ {
		if b[k] != '{' {
			continue
		}
		re := cutil.Match(b, k)
		row := c.New[k:re]
		br := string(b[k:re])
		n := strings.LastIndex(br, "{")
		m := strings.LastIndex(br[:n], "{")
		outPairs = append(outPairs, strings.Join(strings.Fields(row[m:]), " "))
		k = re
	}
	if len(outPairs) != len(in) {
		r.Bad("options[] has %d rows on the input and %d on the output", len(in), len(outPairs))
	} else {
		for k, row := range in {
			str, num := W151Pair(row[0], row[1], row[2])
			want := strings.Join(strings.Fields(str+", "+num), " ")
			if outPairs[k] != want {
				r.Bad("row %d (%s): %q where the rule gives %q", k, row[0], outPairs[k], want)
				break
			}
		}
	}
	if n := len(regexp.MustCompile(`(\.|->)def_val\b`).FindAllString(c.New, -1)); n != 0 {
		r.Bad("def_val is still a field, %d times", n)
	}
	if strings.Contains(c.New, "(char_u *)(long_i)") {
		r.Bad("a number is still cast to a pointer to be stored")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("each of the %d rows of options[] has the input's defaults split by its kind -- computed row for row; no def_val field and no number stored as a pointer is left", len(in))
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	more := strings.Repeat(" ", 40)
	probes := []struct{ What, Keys, ctl string }{
		{"every option's default", ":set all&\r:set all\r" + more, ":set all&\r:set ts=3\r:set all\r" + more},
		{"the terminal options' defaults", ":set termcap\r" + more, ":set t_ZH=x\r:set termcap\r" + more},
		{"one option reset to its default", ":set sw=7\r:set sw&\r:set sw?\r", ":set sw=7\r:set sw?\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{[]byte(pr.Keys), []byte(":q!\r")}
		s1, _, e1 := check.Stream(ob, keys, nil)
		s2, _, e2 := check.Stream(nb, keys, nil)
		s3, _, e3 := check.Stream(nb, [][]byte{[]byte(pr.ctl), []byte(":q!\r")}, nil)
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
	r.Say("PROBE: every option set to its default and listed, the terminal options' defaults, and one option reset, are drawn the same by both binaries; each CONTROL moves")
	return nil
}
