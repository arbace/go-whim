package check

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim158", Whim158) }

// Whim158 is phase 158's check: no union member is read where its
// discriminant does not say it holds.
//
//  1. THE PREMISE, measured by gcc on the input's own types: cterm.font is the
//     two bytes six past term.start, an eight-byte pointer -- its top two
//     bytes, which are 0 in every x86-64 user-space address and in NULL.  So
//     on the input the test read 0 and failed on every term entry, as it now
//     does without reading.  CONTROL: the same assertion at offset 4 fails.
//  2. THE PARTITION, internal/ccx's Unions: on the input it leaves exactly
//     the three reads of that test and nothing else; on the output nothing.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: a highlight drawn on a terminal without colours with a
//     start string (a term entry whose term.start is a pointer, the path that
//     read it), and one with a terminal font on a colour terminal (the path
//     the test now guards) are drawn the same on both binaries; each CONTROL
//     moves.
func Whim158(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim158", "font")
	if err != nil {
		return err
	}
	r := c.r
	if strings.Count(c.old, edit.W158Font) != 1 || strings.Count(c.new, edit.W158Guard) != 1 || strings.Contains(c.new, edit.W158Font) {
		r.bad("the font test is not the one this phase rewrites, once")
	}
	core := c.old[:strings.Index(c.old, "\n#include")+1]
	for _, off := range []string{"6", "4"} {
		ok, err := layout(core, off)
		if err != nil {
			r.bad("%v", err)
			return r.done()
		}
		if ok != (off == "6") {
			r.bad("gcc says cterm.font is %s bytes past term.start: %v", off, ok)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("measured by gcc on the input's types: cterm.font is the top two bytes of term.start, an 8-byte pointer; the CONTROL at offset 4 is refused")
	for _, side := range []struct {
		name, text string
		left       int
	}{{"input", c.old, 3}, {"output", c.new, 0}} {
		ast, err := parseCore(side.text)
		if err != nil {
			r.bad("%v", err)
			return r.done()
		}
		res := ccx.Unions(ast)
		n := 0
		for _, f := range res.Left {
			if side.left > 0 && f.Fn == "screen_start_highlight" && strings.HasPrefix(f.What, "ae_u.cterm of aep->") {
				n++
				continue
			}
			r.bad("the %s's %s: %s %s", side.name, f.Fn, f.Where, f.What)
		}
		if n != side.left {
			r.bad("the %s has %d reads of cterm.font where t_colors does not say it holds, where it had %d", side.name, n, side.left)
		}
		if side.left == 0 {
			total := 0
			for _, k := range res.Classes {
				total += k
			}
			r.say("every one of %d union accesses, discriminant calls and state moves is where its discriminant holds, and no discriminant is written under a read of it", total)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("the input's three reads of cterm.font in screen_start_highlight() were the only accesses of a member its discriminant did not say held")
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	seed := []byte("ifoo bar\x1b0")
	probes := []struct{ what, keys, ctl string }{
		{"a term entry with a start string",
			":set t_Co=0 hls\r:hi Search term=NONE start=<Esc>[4m stop=<Esc>[24m\r/foo\r",
			":set t_Co=0 hls\r:hi Search term=NONE start=<Esc>[4m stop=<Esc>[24m\r/bar\r"},
		{"a cterm entry with a font",
			":set t_CF=\x16\x1b[%dm hls\r:hi Search ctermfg=1 ctermfont=3\r/foo\r",
			":set t_CF=\x16\x1b[%dm hls\r:hi Search ctermfg=1 ctermfont=4\r/foo\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr.keys), []byte(":q!\r")}
		s1, _, e1 := stream(ob, keys, nil)
		s2, _, e2 := stream(nb, keys, nil)
		s3, _, e3 := stream(nb, [][]byte{seed, []byte(pr.ctl), []byte(":q!\r")}, nil)
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
	r.say("PROBE: a term entry with a start string and a cterm entry with a font are drawn the same on both binaries; each CONTROL moves")
	return nil
}

// layout asks gcc, on the core's own types, whether cterm.font is off bytes
// past term.start, two bytes wide, in an 8-byte pointer.
func layout(core, off string) (bool, error) {
	tmp, err := os.MkdirTemp("", "layout-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)
	f := filepath.Join(tmp, "l.c")
	src := core + "_Static_assert(__builtin_offsetof(attrentry_T, ae_u.cterm.font) - __builtin_offsetof(attrentry_T, ae_u.term.start) == " + off +
		" && sizeof(((attrentry_T *)0)->ae_u.cterm.font) == 2 && sizeof(((attrentry_T *)0)->ae_u.term.start) == 8, \"layout\");\n"
	if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
		return false, err
	}
	out, err := exec.Command("gcc", "-std=gnu2x", "-fsyntax-only", "-w", f).CombinedOutput()
	if err == nil {
		return true, nil
	}
	if strings.Contains(string(out), "static assertion failed") {
		return false, nil
	}
	return false, &asmErr{string(out)}
}
