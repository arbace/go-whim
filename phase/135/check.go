package p135

// Whim phase 135, the check -- one regexp program type.
// See phase/135/edit.go, and GOALS.md.
//
// phase/135/check.go proves bt_regengine the only engine, requires the
// one definition and no cast left, and requires the input and the output to
// build to the same bytes.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim135", Check) }

// Whim135 is phase 135's check: one regexp program type.
//
//  1. THE PREMISE, on the input: the only regengine_T is bt_regengine -- the
//     one engine whose programs are bt_regprog_T -- so every regprog_T was one.
//  2. THE CUT: regprog_T's definition is W135One exactly; bt_regprog_T has
//     no mention; no cast to regprog_T * or to it is left.
//  3. THE CODE DID NOT MOVE: the input and the output, built with the
//     boundary's flags and SOURCE_DATE_EPOCH=0, are the same bytes.  Every
//     field is where it was and every cast was to itself; a byte-identical
//     binary is the whole of what "nothing changed" can mean, and it is
//     measured, not argued.
//  4. THE GATE, the libc surface unchanged.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim135", "regprog")
	if err != nil {
		return err
	}
	r := c.R
	if n := strings.Count(c.Old, "static regengine_T "); n != 2 || !strings.Contains(c.Old, "static regengine_T bt_regengine =") {
		r.Bad("the input's regengine_T objects are not bt_regengine alone (%d declarations)", n)
	}
	if !strings.Contains(c.New, W135One) {
		r.Bad("regprog_T is not the one type this phase writes")
	}
	if n := check.Word(c.New, "bt_regprog_T"); n != 0 {
		r.Bad("bt_regprog_T has %d mentions left", n)
	}
	// a cast, and not a parameter list: `(*regfree)(regprog_T *)` is the
	// engine table's field, which phase 136 takes
	if n := len(regexp.MustCompile(`[^)]\(regprog_T \*\)`).FindAllStringIndex(c.New, -1)); n != 0 {
		r.Bad("%d casts to regprog_T * are left", n)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("bt_regengine is the only engine, so every program was a backtracking one: regprog_T is that program, bt_regprog_T is gone and so are the five casts")

	same, size, err := c.SameBinary()
	if err != nil {
		r.Bad("the byte comparison did not build: %v", err)
		return r.Done()
	}
	if !same {
		r.Bad("the binary moved: making two types one and dropping casts to themselves must change no code")
		return r.Done()
	}
	r.Say("THE BINARY IS BYTE-IDENTICAL, %d bytes either side: no field moved and no cast did anything", size)
	return c.Gate(true)
}
