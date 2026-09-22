package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { register("whim148", Whim148) }

// Whim148 is phase 148's check: allocation cannot fail.
//
//  1. WHY host_alloc() NEVER RETURNS NULL, on the input: its body has one
//     return, of a pointer into the arena, and its one other way out is
//     host_arena_exhausted(), which ends in host_exit(), which ends in
//     __builtin_longjmp -- it does not return.
//  2. THE CUT: lalloc() is edit.W148LallocBody: E341 for zero bytes, as
//     before, and then host_alloc().  Nothing that could only follow a NULL is
//     left in it.
//  3. THE GATE, the libc surface unchanged; the recording, which never asks
//     for zero bytes and never runs out, is the stage's delta check.  A zero
//     request is an internal error no input reaches; it now gets a pointer to
//     no bytes where it got NULL, and says so as before.
func Whim148(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim148", "nofail")
	if err != nil {
		return err
	}
	r := c.r
	body := func(text, name string) string {
		o, cl, f, _ := cutil.Body([]byte(text), name)
		if !f {
			return ""
		}
		return text[o : cl+1]
	}
	ha := body(c.old, "host_alloc")
	rets := regexp.MustCompile(`\breturn\b[^;]*;`).FindAllString(ha, -1)
	if len(rets) != 1 || rets[0] != "return p;" || !strings.Contains(ha, "p = (char *)host_arena + host_arena_used;") || !strings.Contains(ha, "host_arena_exhausted(n);") {
		r.bad("host_alloc() is not one return of a pointer into the arena and a call to host_arena_exhausted(): %v", rets)
	}
	hx := body(c.old, "host_arena_exhausted")
	if !strings.HasSuffix(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(hx), "}")), "host_exit(1);") || strings.Contains(hx, "return") {
		r.bad("host_arena_exhausted() does not end in host_exit()")
	}
	he := body(c.old, "host_exit")
	if !strings.HasSuffix(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(he), "}")), "__builtin_longjmp(host_jump, 1);") || strings.Contains(he, "return") {
		r.bad("host_exit() does not end in __builtin_longjmp")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("on the input host_alloc() returns a pointer into the arena or ends the process through host_exit()'s longjmp: it never returns NULL")
	if body(c.new, "lalloc") != "{\n"+edit.W148LallocBody+"\n}" {
		r.bad("lalloc()'s body is not the one this phase writes")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("lalloc() reports a request for zero bytes as before and returns host_alloc()'s pointer: no allocation can fail (the file lost %d lines)",
		countLines([]byte(c.old))-countLines([]byte(c.new)))
	return c.gate(true)
}
