package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim130", Whim130) }

var w130Sentinel = regexp.MustCompile(`\(pos_T \*\)\s*-\s*1`)

// Whim130 is phase 130's check: the (pos_T *)-1 tests are gone, and they could
// never have been taken.
//
//  1. WHY THEY WERE DEAD, on the input: every mention of (pos_T *)-1 is one of
//     the three `== (pos_T *)-1` tests -- none is returned, assigned or
//     passed -- and every `return` of getmark(), getmark_buf(),
//     getmark_buf_fnum() and movechangelist() returns NULL or an address.
//  2. THE CUT: no mention is left, and the file lost exactly the three tests,
//     their bodies and the else lines -- counted from the input, not written
//     down.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE: marks used every way the three sites are reached -- `'a` and
//     “ `a “ in Normal mode, `:'a` as an Ex address, `g;` on the change
//     list -- write the same bytes on both binaries; and the CONTROL, the same
//     keys naming a mark that is not set, writes different ones.
func Whim130(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim130", "sentinel")
	if err != nil {
		return err
	}
	r := c.r

	all := w130Sentinel.FindAllStringIndex(c.old, -1)
	tests := regexp.MustCompile(`if \([a-z]+ == \(pos_T \*\)-1\)`).FindAllStringIndex(c.old, -1)
	if len(all) != 3 || len(tests) != 3 {
		r.bad("the input has %d mentions of (pos_T *)-1 and %d are `if (x == (pos_T *)-1)` tests; this phase was written against 3 and 3", len(all), len(tests))
	}
	for _, fn := range []string{"getmark", "getmark_buf", "getmark_buf_fnum", "movechangelist"} {
		o, cl, found, _ := cutil.Body([]byte(c.old), fn)
		if !found {
			r.bad("%s() is not defined in the input", fn)
			continue
		}
		for _, l := range strings.Split(c.old[o:cl], "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "return") && strings.Contains(l, "-1") {
				r.bad("%s() returns something with -1 in it: %s", fn, l)
			}
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("on the input (pos_T *)-1 is exactly the three tests, and no return of getmark(), getmark_buf(), getmark_buf_fnum() or movechangelist() can produce it: each test was never taken")

	if n := len(w130Sentinel.FindAllStringIndex(c.new, -1)); n != 0 {
		r.bad("(pos_T *)-1 still has %d mentions", n)
	}
	// what FoldNever must have removed, measured on the input: each test's
	// line, its then-block, and the `else` line with its braces or the `else `
	// of an `else if`
	want := 0
	ob := []byte(c.old)
	bl := cutil.Blank(ob)
	for _, t := range tests {
		ls := strings.LastIndex(c.old[:t[0]], "\n") + 1
		o := strings.IndexByte(c.old[t[1]:], '{') + t[1]
		cl := cutil.Match(bl, o)
		end := cl + strings.IndexByte(c.old[cl:], '\n') + 1
		want += strings.Count(c.old[ls:end], "\n")
		rest := c.old[end:]
		if strings.HasPrefix(strings.TrimLeft(rest, " "), "else if") {
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(rest, " "), "else") {
			want += 3 // `else`, its `{` and its `}`
		}
	}
	got := countLines(ob) - countLines([]byte(c.new))
	if got != want {
		r.bad("the file lost %d lines; the three tests, their bodies and their else lines are %d", got, want)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("no (pos_T *)-1 is left; the file lost %d lines, exactly the three tests with their bodies and their else lines", got)

	if err := c.gate(true); err != nil {
		return err
	}

	old, nw := c.bins()
	seed := []byte("ione\rtwo\rthree\x1bggmajma")
	uses := [][]byte{[]byte("G'a"), []byte("G`a"), []byte(":'a\r"), []byte("g;g;")}
	for _, u := range uses {
		keys := [][]byte{seed, u, []byte(":q!\r")}
		so, _, e1 := stream(old, keys, nil)
		sn, _, e2 := stream(nw, keys, nil)
		ctl := [][]byte{seed, []byte(strings.ReplaceAll(string(u), "a", "z")), []byte(":q!\r")}
		sc, _, e3 := stream(nw, ctl, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if so != sn {
			r.bad("%q is written differently: %s on the input's binary, %s on this one", u, so, sn)
		}
		if sc == sn && !strings.HasPrefix(string(u), "g;") {
			r.bad("the CONTROL for %q did not move: an unset mark writes the same bytes", u)
		}
	}
	// g; has no mark name to unset: its control is the change list being empty
	ctl := [][]byte{[]byte("g;g;"), []byte(":q!\r")}
	sc, _, _ := stream(nw, ctl, nil)
	sg, _, _ := stream(nw, [][]byte{seed, []byte("g;g;"), []byte(":q!\r")}, nil)
	if sc == sg {
		r.bad("the CONTROL for g; did not move: an empty change list writes the same bytes")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: 'a, `a, :'a and g; write the same bytes on both binaries, and each CONTROL -- an unset mark, an empty change list -- writes different ones")
	return nil
}
