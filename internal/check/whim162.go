package check

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim162", Whim162) }

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
func Whim162(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim162", "getexline")
	if err != nil {
		return err
	}
	r := c.r
	for _, side := range []struct {
		name, text string
		left       int
	}{{"input", c.old, 1}, {"output", c.new, 0}} {
		ast, err := parseCore(side.text)
		if err != nil {
			r.bad("%v", err)
			return r.done()
		}
		n := 0
		for _, f := range ccx.FuncCompares(ast).Left {
			if side.left == 1 && f.Fn == "getline_equal" {
				n++
				continue
			}
			r.bad("the %s's %s: %s %s", side.name, f.Fn, f.Where, f.What)
		}
		if n != side.left {
			r.bad("the %s compares two function pointers %d times in getline_equal(), where it should %d", side.name, n, side.left)
		}
	}
	if word(c.new, "getline_equal") != 0 {
		r.bad("getline_equal() is still named")
	}
	// every call of do_cmdline() in the output: getexline with the flag, or
	// another getter without it
	calls := regexp.MustCompile(`do_cmdline\(([^;]*?), (nullptr|getexline|getcmdkeycmd), ([^;]*)\);`).FindAllStringSubmatch(c.new, -1)
	if len(calls) != len(regexp.MustCompile(`\bdo_cmdline\(`).FindAllString(c.new, -1))-2 {
		r.bad("a call of do_cmdline() is not one this check reads")
	}
	withFlag := 0
	for _, m := range calls {
		has := strings.Contains(m[3], "DOCMD_GETEXLINE")
		if (m[2] == "getexline") != has {
			r.bad("do_cmdline() is called with %s and the flag %v: %s", m[2], has, m[0])
		}
		if has {
			withFlag++
		}
	}
	if n := word(c.new, "DOCMD_GETEXLINE"); n != 1+withFlag+4 || withFlag != 2 {
		r.bad("DOCMD_GETEXLINE is named %d times, %d of them at a call", n, withFlag)
	}
	bits := map[int64]string{}
	for _, m := range regexp.MustCompile(`enum \{ (DOCMD_\w+) = (0x[0-9a-f]+) \};`).FindAllStringSubmatch(c.new, -1) {
		v, _ := strconv.ParseInt(m[2], 0, 64)
		if v&(v-1) != 0 || bits[v] != "" {
			r.bad("%s is not a bit of its own", m[1])
		}
		bits[v] = m[1]
	}
	if bits[0x40] != "DOCMD_GETEXLINE" {
		r.bad("DOCMD_GETEXLINE is not 0x40")
	}
	var reads []string
	fn := c.new[strings.Index(c.new, "\ndo_one_cmd("):]
	fn = fn[:strings.Index(fn, "\n}\n")]
	for _, l := range linesWith(fn, "flags") {
		reads = append(reads, l)
	}
	if len(reads) != 2 || !strings.Contains(reads[1], "flags & DOCMD_VERBOSE") {
		r.bad("do_one_cmd() reads its flags other than for DOCMD_VERBOSE: %q", reads)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("the one comparison of two function pointers, getline_equal()'s, is gone: do_cmdline() tests DOCMD_GETEXLINE, which exactly the %d calls passing getexline set, and every other function pointer is compared only with a null", withFlag)
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	seed := []byte("ione\rtwo\rthree\x1bgg")
	probes := []struct {
		what      string
		keys, ctl [][]byte
	}{
		{"@: after a command typed at ':'", [][]byte{[]byte(":s/e/E/\r"), []byte("j@:")}, [][]byte{[]byte(":s/e/E/\r"), []byte("j")}},
		{"@: after a <Cmd> mapping", [][]byte{[]byte(":nnoremap x <Cmd>s/t/T/<CR>\r"), []byte(":s/e/E/\r"), []byte("jjx"), []byte("k@:")}, [][]byte{[]byte(":nnoremap x <Cmd>s/t/T/<CR>\r"), []byte(":s/e/E/\r"), []byte("jjx"), []byte("k")}},
	}
	for _, pr := range probes {
		keys := append(append([][]byte{seed}, pr.keys...), []byte(":q!\r"))
		ctl := append(append([][]byte{seed}, pr.ctl...), []byte(":q!\r"))
		s1, _, e1 := stream(ob, keys, nil)
		s2, _, e2 := stream(nb, keys, nil)
		s3, _, e3 := stream(nb, ctl, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.bad("%s is drawn differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: @: after a command typed at ':' and after a <Cmd> mapping draw the same on both binaries; each CONTROL moves")
	return nil
}
