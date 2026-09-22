package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { register("whim135", Whim135) }

// Whim135 is phase 135's check: one regexp program type.
//
//  1. THE PREMISE, on the input: the only regengine_T is bt_regengine -- the
//     one engine whose programs are bt_regprog_T -- so every regprog_T was one.
//  2. THE CUT: regprog_T's definition is edit.W135One exactly; bt_regprog_T has
//     no mention; no cast to regprog_T * or to it is left.
//  3. THE CODE DID NOT MOVE: the input and the output, built with the
//     boundary's flags and SOURCE_DATE_EPOCH=0, are the same bytes.  Every
//     field is where it was and every cast was to itself; a byte-identical
//     binary is the whole of what "nothing changed" can mean, and it is
//     measured, not argued.
//  4. THE GATE, the libc surface unchanged.
func Whim135(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim135", "regprog")
	if err != nil {
		return err
	}
	r := c.r
	if n := strings.Count(c.old, "static regengine_T "); n != 2 || !strings.Contains(c.old, "static regengine_T bt_regengine =") {
		r.bad("the input's regengine_T objects are not bt_regengine alone (%d declarations)", n)
	}
	if !strings.Contains(c.new, edit.W135One) {
		r.bad("regprog_T is not the one type this phase writes")
	}
	if n := word(c.new, "bt_regprog_T"); n != 0 {
		r.bad("bt_regprog_T has %d mentions left", n)
	}
	// a cast, and not a parameter list: `(*regfree)(regprog_T *)` is the
	// engine table's field, which phase 136 takes
	if n := len(regexp.MustCompile(`[^)]\(regprog_T \*\)`).FindAllStringIndex(c.new, -1)); n != 0 {
		r.bad("%d casts to regprog_T * are left", n)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("bt_regengine is the only engine, so every program was a backtracking one: regprog_T is that program, bt_regprog_T is gone and so are the five casts")

	same, size, err := c.sameBinary()
	if err != nil {
		r.bad("the byte comparison did not build: %v", err)
		return r.done()
	}
	if !same {
		r.bad("the binary moved: making two types one and dropping casts to themselves must change no code")
		return r.done()
	}
	r.say("THE BINARY IS BYTE-IDENTICAL, %d bytes either side: no field moved and no cast did anything", size)
	return c.gate(true)
}
