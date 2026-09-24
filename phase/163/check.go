package p163

// Whim phase 163, the check -- the product is in the one canonical spelling.
// See phase/163/GOAL.md, and GOALS.md.
//
// phase/163/check.go requires the output to be a fixed point of the canonical
// printer, the input and the output to build to the same bytes, and a
// control -- one live string changed -- to move that binary.

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/cemit"
	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim163", Check) }

// controlOld is a string the product prints (add_time's), so a binary built
// with it changed cannot be the output's.
const controlOld, controlNew = `"%ld seconds ago"`, `"%ld seconds ag0"`

// Whim163 is phase 163's check: the product is canonical and nothing else
// moved.
//
//  1. THE FIXED POINT: the canonical printer, run on the output, gives the
//     output back byte for byte.  The input is required NOT to be one, or the
//     phase did nothing and this check proves nothing about it.
//  2. THE CODE DID NOT MOVE: the input and the output, built with the
//     boundary's flags and SOURCE_DATE_EPOCH=0, are the same bytes.  There is
//     no -g, so a change of layout alone leaves the binary as it was.
//  3. THE CONTROL: the output with one string it prints changed builds to
//     different bytes, so the comparison in 2 can fail.
//  4. THE GATE, the libc surface unchanged.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim163", "canonical")
	if err != nil {
		return err
	}
	r := c.R
	again, err := cemit.Canonical(c.F, []byte(c.New))
	if err != nil {
		r.Bad("the canonical printer refuses the output: %v", err)
		return r.Done()
	}
	if string(again) != c.New {
		r.Bad("the output is not a fixed point of the canonical printer: %d lines in, %d out",
			check.CountLines([]byte(c.New)), check.CountLines(again))
	}
	if canon, err := cemit.Canonical(filepath.Join(c.State, "old.c"), []byte(c.Old)); err == nil && string(canon) == c.Old {
		r.Bad("the input was already canonical, so this phase did nothing")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the output is its own canonical form, %d lines, and the input, %d lines, was not",
		check.CountLines([]byte(c.New)), check.CountLines([]byte(c.Old)))

	same, size, err := c.SameBinary()
	if err != nil {
		r.Bad("the byte comparison did not build: %v", err)
		return r.Done()
	}
	if !same {
		r.Bad("the binary moved: a change of layout alone must change no code")
		return r.Done()
	}
	if strings.Count(c.New, controlOld) != 1 {
		r.Bad("the control's string %s is not in the output exactly once", controlOld)
		return r.Done()
	}
	ctl := filepath.Join(c.State, "control.c")
	if err := os.WriteFile(ctl, []byte(strings.Replace(c.New, controlOld, controlNew, 1)), 0o644); err != nil {
		return err
	}
	a, err := c.BuildAt(c.F, "0")
	if err != nil {
		r.Bad("the output did not build: %v", err)
		return r.Done()
	}
	b, err := c.BuildAt(ctl, "0")
	if err != nil {
		r.Bad("the control did not build: %v", err)
		return r.Done()
	}
	if a == b {
		r.Bad("THE CONTROL DID NOT MOVE: the output with %s made %s builds to the same bytes, so the comparison proves nothing", controlOld, controlNew)
		return r.Done()
	}
	r.Say("THE BINARY IS BYTE-IDENTICAL, %d bytes either side, and the control -- one string changed -- is not", size)
	return c.Gate(true)
}
