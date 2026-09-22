package p162

// Whim phase 162, the check -- no two function pointers are compared.
// See phase/162/edit.go, and GOALS.md.
//
// phase/162/check.go requires internal/ccx's FuncCompares to leave
// nothing, the flag set exactly where getexline is passed, and probes @:.

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim162", Check) }

// Whim162 is phase 162's check: no two function pointers are compared.
//
//  1. THE PARTITION, internal/ccx's FuncCompares: on the input the one
//     comparison of two function pointers is getline_equal()'s; on the
//     output every function pointer is compared only with a null, and
//     getline_equal() is gone.
//  2. THE PREMISE of the flag: every call of do_cmdline() that passes
//     getexline sets DOCMD_GETEXLINE and no other call names it, so the flag
//     is set exactly when the comparison was true; its bit is no other
//     DOCMD flag's; do_one_cmd(), handed the flags, reads only DOCMD_VERBOSE.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: @: after a command typed at ':' (what do_cmdline() keeps as
//     the last command line only for getexline), and after a <Cmd> mapping,
//     which must not replace it, draw the same on both binaries; each
//     CONTROL moves.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim162", "getexline")
	if err != nil {
		return err
	}
	r := c.R
	for _, side := range []struct {
		Name, Text string
		left       int
	}{{"input", c.Old, 1}, {"output", c.New, 0}} {
		ast, err := check.ParseCore(side.Text)
		if err != nil {
			r.Bad("%v", err)
			return r.Done()
		}
		n := 0
		for _, f := range ccx.FuncCompares(ast).Left {
			if side.left == 1 && f.Fn == "getline_equal" {
				n++
				continue
			}
			r.Bad("the %s's %s: %s %s", side.Name, f.Fn, f.Where, f.What)
		}
		if n != side.left {
			r.Bad("the %s compares two function pointers %d times in getline_equal(), where it should %d", side.Name, n, side.left)
		}
	}
	if check.Word(c.New, "getline_equal") != 0 {
		r.Bad("getline_equal() is still named")
	}
	// every call of do_cmdline() in the output: getexline with the flag, or
	// another getter without it
	calls := regexp.MustCompile(`do_cmdline\(([^;]*?), (nullptr|getexline|getcmdkeycmd), ([^;]*)\);`).FindAllStringSubmatch(c.New, -1)
	if len(calls) != len(regexp.MustCompile(`\bdo_cmdline\(`).FindAllString(c.New, -1))-2 {
		r.Bad("a call of do_cmdline() is not one this check reads")
	}
	withFlag := 0
	for _, m := range calls {
		has := strings.Contains(m[3], "DOCMD_GETEXLINE")
		if (m[2] == "getexline") != has {
			r.Bad("do_cmdline() is called with %s and the flag %v: %s", m[2], has, m[0])
		}
		if has {
			withFlag++
		}
	}
	if n := check.Word(c.New, "DOCMD_GETEXLINE"); n != 1+withFlag+4 || withFlag != 2 {
		r.Bad("DOCMD_GETEXLINE is named %d times, %d of them at a call", n, withFlag)
	}
	bits := map[int64]string{}
	for _, m := range regexp.MustCompile(`enum \{ (DOCMD_\w+) = (0x[0-9a-f]+) \};`).FindAllStringSubmatch(c.New, -1) {
		v, _ := strconv.ParseInt(m[2], 0, 64)
		if v&(v-1) != 0 || bits[v] != "" {
			r.Bad("%s is not a bit of its own", m[1])
		}
		bits[v] = m[1]
	}
	if bits[0x40] != "DOCMD_GETEXLINE" {
		r.Bad("DOCMD_GETEXLINE is not 0x40")
	}
	var reads []string
	fn := c.New[strings.Index(c.New, "\ndo_one_cmd("):]
	fn = fn[:strings.Index(fn, "\n}\n")]
	for _, l := range check.LinesWith(fn, "flags") {
		reads = append(reads, l)
	}
	if len(reads) != 2 || !strings.Contains(reads[1], "flags & DOCMD_VERBOSE") {
		r.Bad("do_one_cmd() reads its flags other than for DOCMD_VERBOSE: %q", reads)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the one comparison of two function pointers, getline_equal()'s, is gone: do_cmdline() tests DOCMD_GETEXLINE, which exactly the %d calls passing getexline set, and every other function pointer is compared only with a null", withFlag)
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	seed := []byte("ione\rtwo\rthree\x1bgg")
	probes := []struct {
		What      string
		Keys, ctl [][]byte
	}{
		{"@: after a command typed at ':'", [][]byte{[]byte(":s/e/E/\r"), []byte("j@:")}, [][]byte{[]byte(":s/e/E/\r"), []byte("j")}},
		{"@: after a <Cmd> mapping", [][]byte{[]byte(":nnoremap x <Cmd>s/t/T/<CR>\r"), []byte(":s/e/E/\r"), []byte("jjx"), []byte("k@:")}, [][]byte{[]byte(":nnoremap x <Cmd>s/t/T/<CR>\r"), []byte(":s/e/E/\r"), []byte("jjx"), []byte("k")}},
	}
	for _, pr := range probes {
		keys := append(append([][]byte{seed}, pr.Keys...), []byte(":q!\r"))
		ctl := append(append([][]byte{seed}, pr.ctl...), []byte(":q!\r"))
		s1, _, e1 := check.Stream(ob, keys, nil)
		s2, _, e2 := check.Stream(nb, keys, nil)
		s3, _, e3 := check.Stream(nb, ctl, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.Say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.Bad("%s is drawn differently on the two binaries", pr.What)
		}
		if s3 == s2 {
			r.Bad("the CONTROL for %s did not move", pr.What)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: @: after a command typed at ':' and after a <Cmd> mapping draw the same on both binaries; each CONTROL moves")
	return nil
}
