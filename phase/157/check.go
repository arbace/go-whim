package p157

// Whim phase 157, the check -- get_register() and put_register() carry a yankreg_T *, not a void *.
// See phase/157/edit.go, and GOALS.md.
//
// phase/157/check.go requires the typed prototypes, every pointer cast
// in a class, a byte-identical binary, and probes a Visual-mode put.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim157", Check) }

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
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim157", "register")
	if err != nil {
		return err
	}
	r := c.R
	for _, s := range []string{"static yankreg_T *get_register(int name, int copy);", "static void put_register(int name, yankreg_T *reg);", "    *y_current = *reg;\n"} {
		if !strings.Contains(c.New, s) {
			r.Bad("the output does not have %q", s)
		}
	}
	ast, err := check.ParseCore(c.New)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	res := ccx.Casts(ast)
	for _, f := range res.Left {
		r.Bad("a pointer cast no class covers: %s %s", f.Where, f.What)
	}
	if err := r.Done(); err != nil {
		return err
	}
	total := 0
	for _, n := range res.Classes {
		total += n
	}
	r.Say("the register is a yankreg_T * from get_register() to put_register(), and every one of the %d pointer casts in the core is in a class the emitter has a rule for", total)
	same, size, err := c.SameBinary()
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	if !same {
		r.Bad("the binary moved: typing a pointer must change no code")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("THE BINARY IS BYTE-IDENTICAL, %d bytes either side", size)
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	seed := []byte("ione two\x1b0")
	keys := [][]byte{seed, []byte("yiwwviwp"), []byte(":q!\r")}
	ctl := [][]byte{seed, []byte("wyiwbviwp"), []byte(":q!\r")}
	s1, _, e1 := check.Stream(ob, keys, nil)
	s2, _, e2 := check.Stream(nb, keys, nil)
	s3, _, e3 := check.Stream(nb, ctl, nil)
	if e1 != nil || e2 != nil || e3 != nil {
		r.Say("a probe did not run: %v %v %v", e1, e2, e3)
		return harness.ErrReported
	}
	if s1 != s2 {
		r.Bad("a Visual put is drawn differently on the two binaries")
	}
	if s3 == s2 {
		r.Bad("the CONTROL did not move")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: a Visual-mode put draws the same on both binaries; the CONTROL moves")
	return nil
}
