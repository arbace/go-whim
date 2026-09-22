package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim154", Whim154) }

// Whim154 is phase 154's check: the NULL write is gone.
//
//  1. WHY THE CALL ONLY EVER WROTE THROUGH NULL, on the input: its one call
//     is edit.W154Call, and free_one_termoption() matches a row only when
//     `p->var.ov_str == nullptr && var == nullptr` (phase 153) -- so when it
//     matched, `*p->var.ov_str = empty_option` wrote through NULL, and when it
//     did not, it did nothing.  The if around the call only reads: its
//     condition dereferences t_Sb and t_AB, which the lines above it already
//     dereference.
//  2. THE CUT: the call and its if are gone, and free_one_termoption() with
//     them -- the sweep took it; no NULL test of an option's ov_str is
//     followed by a write through it anywhere.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: the paths into ttest() -- a terminal set, the colour
//     options cleared, t_Co set -- draw the same on both binaries, each
//     CONTROL moving.
func Whim154(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim154", "nullwrite")
	if err != nil {
		return err
	}
	r := c.r
	if strings.Count(c.old, edit.W154Call) != 1 {
		r.bad("the call is not the one this phase takes out")
	}
	b := []byte(c.old)
	a, z, ok := cutil.FindDefinition(b, cutil.Blank(b), "free_one_termoption")
	if !ok || !strings.Contains(c.old[a:z], "if (p->var.ov_str == nullptr && var == nullptr)") ||
		!strings.Contains(c.old[a:z], "*p->var.ov_str = empty_option;") {
		r.bad("free_one_termoption() does not match only both-NULL and then write through the match")
	}
	if n := len(regexp.MustCompile(`\bfree_one_termoption\(`).FindAllString(c.old, -1)); n != 2 {
		r.bad("free_one_termoption is named %d times on the input, where its definition and the one call are 2", n)
	}
	before := c.old[:strings.Index(c.old, edit.W154Call)]
	tail := before[strings.LastIndex(before, "\n    static void\n"):]
	for _, k := range []string{"KS_CSB", "KS_CAB"} {
		if !strings.Contains(tail, "* ( term_strings[(int)("+k+")] )  == NUL") {
			r.bad("%s is not already dereferenced above the call", k)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("on the input the call's one effect is a write through NULL, and the if around it reads only what the lines above already read")
	if strings.Contains(c.new, "free_one_termoption") || strings.Contains(c.new, edit.W154Call) {
		r.bad("free_one_termoption() or its call is still there")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("the call and free_one_termoption() are gone (the file lost %d lines)", countLines([]byte(c.old))-countLines([]byte(c.new)))
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	probes := []struct{ what, keys, ctl string }{
		{"the colour options cleared", ":set t_AB=\r:set t_AF=\r:set t_Sb=\r:set t_Co?\r", ":set t_AB=\r:set t_Co=16\r:set t_Co?\r"},
		{"t_Co set", ":set t_Co=256\r:set t_Co?\r", ":set t_Co=8\r:set t_Co?\r"},
		{"a terminal set", ":set term=xterm\r:set t_Co?\r", ":set term=xterm\r:set t_Co=2\r:set t_Co?\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{[]byte(pr.keys), []byte(":q!\r")}
		s1, _, e1 := stream(ob, keys, nil)
		s2, _, e2 := stream(nb, keys, nil)
		s3, _, e3 := stream(nb, [][]byte{[]byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.bad("%s is written differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: clearing the colour options, setting t_Co and setting the terminal draw the same on both binaries; each CONTROL moves")
	return nil
}
