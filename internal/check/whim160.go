package check

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim160", Whim160) }

// Whim160 is phase 160's check: no line getter takes a cookie, and every
// void * left in the core is memory.
//
//  1. THE PREMISE, on the input: every call of do_cmdline() passes nullptr as
//     the cookie, so nullptr is the only value any getter was handed; that
//     nothing read it is the output compiling without it.
//  2. THE CUT: cookie and find_func_t are named nowhere; internal/ccx's
//     VoidPtrs finds every declaration that names a void * a function of
//     bytes, an allocator or a growarray's storage, and GrowArrays every
//     growarray of one element type.
//  3. THE GATE, the libc surface unchanged, and the output built.
//  4. THE PROBES: a command typed at ':' (nv_colon()'s getexline()), a <Cmd>
//     mapping (getcmdkeycmd()), :append (its reader) and :@ (ex_at()'s
//     getexline()) do the same on both binaries; each CONTROL moves.
func Whim160(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim160", "cookie")
	if err != nil {
		return err
	}
	r := c.r
	calls := regexp.MustCompile(`do_cmdline\(`).FindAllStringIndex(c.old, -1)
	nulls := regexp.MustCompile(`do_cmdline\([^;]*?, (?:nullptr|getexline|getcmdkeycmd), nullptr, `).FindAllString(c.old, -1)
	if len(calls)-2 != len(nulls) || len(nulls) == 0 {
		r.bad("do_cmdline() is called %d times, %d of them with a nullptr cookie", len(calls)-2, len(nulls))
	}
	for _, name := range []string{"cookie", "find_func_t"} {
		if n := word(c.new, name); n != 0 {
			r.bad("%s is still named %d times", name, n)
		}
	}
	ast, err := parseCore(c.new)
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	nv := 0
	for _, res := range []ccx.Result{ccx.VoidPtrs(ast), ccx.GrowArrays(ast)} {
		for _, f := range res.Left {
			r.bad("%s: %s %s %s", res.Title, f.Fn, f.Where, f.What)
		}
		if res.Title == "void pointers" {
			for _, k := range res.Classes {
				nv += k
			}
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("every one of %d calls of do_cmdline() passed a nullptr cookie; no getter takes one now, and the core compiles without it", len(nulls))
	r.say("each of %d declarations that name a void * is a function of bytes, an allocator or a growarray's storage, and every growarray has one element type", nv)
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	seed := []byte("ione\rtwo\x1b")
	probes := []struct {
		what      string
		keys, ctl [][]byte
	}{
		{"a command typed at ':'", [][]byte{[]byte(":1s/o/0/\r")}, [][]byte{[]byte(":2s/o/0/\r")}},
		{"a <Cmd> mapping", [][]byte{[]byte(":nnoremap x <Cmd>1s/e/E/<CR>\r"), []byte("x")}, [][]byte{[]byte(":nnoremap x <Cmd>2s/o/O/<CR>\r"), []byte("x")}},
		{":append", [][]byte{[]byte(":1append\r"), []byte("three\r"), []byte(".\r")}, [][]byte{[]byte(":2append\r"), []byte("three\r"), []byte(".\r")}},
		{":@", [][]byte{[]byte("ggO2s/w/W/\x1b"), []byte("\"ay$"), []byte(":@a\r")}, [][]byte{[]byte("ggO2s/t/T/\x1b"), []byte("\"ay$"), []byte(":@a\r")}},
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
	r.say("PROBE: a command typed at ':', a <Cmd> mapping, :append and :@ draw the same on both binaries; each CONTROL moves")
	return nil
}
