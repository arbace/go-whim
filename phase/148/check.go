package p148

// Whim phase 148, the check -- allocation cannot fail.
// See phase/148/edit.go, and GOALS.md.
//
// phase/148/check.go proves host_alloc() never returns NULL and requires
// lalloc() to be the phase's body.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
)

func init() { check.Register("whim148", Check) }

// Whim148 is phase 148's check: allocation cannot fail.
//
//  1. WHY host_alloc() NEVER RETURNS NULL, on the input: its Body has one
//     return, of a pointer into the arena, and its one other way Out is
//     host_arena_exhausted(), which ends in host_exit(), which ends in
//     __builtin_longjmp -- it does not return.
//  2. THE CUT: lalloc() is W148LallocBody: E341 for zero bytes, as
//     before, and then host_alloc().  Nothing that could only follow a NULL is
//     left in it.
//  3. THE GATE, the libc surface unchanged; the recording, which never asks
//     for zero bytes and never runs Out, is the stage's delta check.  A zero
//     request is an internal error no input reaches; it now gets a pointer to
//     no bytes where it got NULL, and says so as before.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim148", "nofail")
	if err != nil {
		return err
	}
	r := c.R
	Body := func(text, name string) string {
		o, cl, f, _ := cutil.Body([]byte(text), name)
		if !f {
			return ""
		}
		return text[o : cl+1]
	}
	ha := Body(c.Old, "host_alloc")
	rets := regexp.MustCompile(`\breturn\b[^;]*;`).FindAllString(ha, -1)
	if len(rets) != 1 || rets[0] != "return p;" || !strings.Contains(ha, "p = (char *)host_arena + host_arena_used;") || !strings.Contains(ha, "host_arena_exhausted(n);") {
		r.Bad("host_alloc() is not one return of a pointer into the arena and a call to host_arena_exhausted(): %v", rets)
	}
	hx := Body(c.Old, "host_arena_exhausted")
	if !strings.HasSuffix(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(hx), "}")), "host_exit(1);") || strings.Contains(hx, "return") {
		r.Bad("host_arena_exhausted() does not end in host_exit()")
	}
	he := Body(c.Old, "host_exit")
	if !strings.HasSuffix(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(he), "}")), "__builtin_longjmp(host_jump, 1);") || strings.Contains(he, "return") {
		r.Bad("host_exit() does not end in __builtin_longjmp")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("on the input host_alloc() returns a pointer into the arena or ends the process through host_exit()'s longjmp: it never returns NULL")
	if Body(c.New, "lalloc") != "{\n"+W148LallocBody+"\n}" {
		r.Bad("lalloc()'s body is not the one this phase writes")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("lalloc() reports a request for zero bytes as before and returns host_alloc()'s pointer: no allocation can fail (the file lost %d lines)",
		check.CountLines([]byte(c.Old))-check.CountLines([]byte(c.New)))
	return c.Gate(true)
}
