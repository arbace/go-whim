package check

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim157", Whim157) }

// Whim157 is phase 157's check: the register is a yankreg_T * throughout.
//
//  1. THE CUT: get_register() returns and put_register() takes yankreg_T *,
//     and internal/ccx's Casts finds no pointer cast left that is not an
//     allocation, a growarray, a function of bytes, bytes or a null -- the
//     partition is complete.
//  2. THE CODE DID NOT MOVE: input and output built with the boundary's flags
//     and SOURCE_DATE_EPOCH=0 are the same bytes.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE: a Visual-mode put, the path that saves and restores the
//     register, draws the same on both binaries; the CONTROL moves.
func Whim157(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim157", "register")
	if err != nil {
		return err
	}
	r := c.r
	for _, s := range []string{"static yankreg_T *get_register(int name, int copy);", "static void put_register(int name, yankreg_T *reg);", "    *y_current = *reg;\n"} {
		if !strings.Contains(c.new, s) {
			r.bad("the output does not have %q", s)
		}
	}
	ast, err := parseCore(c.new)
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	res := ccx.Casts(ast)
	for _, f := range res.Left {
		r.bad("a pointer cast no class covers: %s %s", f.Where, f.What)
	}
	if err := r.done(); err != nil {
		return err
	}
	total := 0
	for _, n := range res.Classes {
		total += n
	}
	r.say("the register is a yankreg_T * from get_register() to put_register(), and every one of the %d pointer casts in the core is in a class the emitter has a rule for", total)
	same, size, err := c.sameBinary()
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	if !same {
		r.bad("the binary moved: typing a pointer must change no code")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("THE BINARY IS BYTE-IDENTICAL, %d bytes either side", size)
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	seed := []byte("ione two\x1b0")
	keys := [][]byte{seed, []byte("yiwwviwp"), []byte(":q!\r")}
	ctl := [][]byte{seed, []byte("wyiwbviwp"), []byte(":q!\r")}
	s1, _, e1 := stream(ob, keys, nil)
	s2, _, e2 := stream(nb, keys, nil)
	s3, _, e3 := stream(nb, ctl, nil)
	if e1 != nil || e2 != nil || e3 != nil {
		r.say("a probe did not run: %v %v %v", e1, e2, e3)
		return harness.ErrReported
	}
	if s1 != s2 {
		r.bad("a Visual put is drawn differently on the two binaries")
	}
	if s3 == s2 {
		r.bad("the CONTROL did not move")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: a Visual-mode put draws the same on both binaries; the CONTROL moves")
	return nil
}
