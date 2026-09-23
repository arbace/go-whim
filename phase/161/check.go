package p161

// Whim phase 161, the check -- no goto jumps into a block.
// See phase/161/edit.go, and GOALS.md.
//
// phase/161/check.go requires internal/ccx's Gotos to leave nothing,
// the new function to be the old tail, and probes ml_get_buf().

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim161", Check) }

// Whim161 is phase 161's check: no goto jumps into a block.
//
//  1. THE PARTITION, internal/ccx's Gotos: on the input exactly one goto
//     jumps into a block, ml_get_buf()'s goto errorret; on the output none.
//  2. THE CUT: ml_get_invalid()'s statements are the old tail's, statement
//     for statement, and its static buffer is the one ml_get_buf() had.  The
//     error path is vim's internal error for a line the memline cannot give;
//     no key reaches it, so no probe can, and this is the evidence for it.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE: lines drawn, joined and moved through ml_get_buf() on both
//     binaries are the same; the CONTROL moves.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim161", "errorret")
	if err != nil {
		return err
	}
	r := c.R
	for _, side := range []struct {
		Name, Text string
		into       int
	}{{"input", c.Old, 1}, {"output", c.New, 0}} {
		ast, err := check.ParseCore(side.Text)
		if err != nil {
			r.Bad("%v", err)
			return r.Done()
		}
		res := ccx.Gotos(ast)
		n := 0
		for _, f := range res.Left {
			if side.into == 1 && f.Fn == "ml_get_buf" && strings.Contains(f.What, "goto errorret jumps into a block") {
				n++
				continue
			}
			r.Bad("the %s's %s: %s %s", side.Name, f.Fn, f.Where, f.What)
		}
		if n != side.into {
			r.Bad("the %s has %d gotos into ml_get_buf()'s error block, where it should have %d", side.Name, n, side.into)
		}
	}
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	tail := strings.TrimPrefix(W161Tail, "errorret:\n")
	Body := W161Func[strings.Index(W161Func, "questions[4];\n")+len("questions[4];\n") : strings.LastIndex(W161Func, "}")]
	if norm(tail) != norm(Body) || !strings.Contains(c.New, W161Func) {
		r.Bad("ml_get_invalid() is not the old tail, statement for statement")
	}
	if strings.Contains(c.New, "errorret") || !strings.Contains(c.Old, "    static char_u questions[4];\n    if (lnum > buf->b_ml.ml_line_count)\n") {
		r.Bad("the error path or its buffer is not where this check expects it")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the one goto that jumped into a block, ml_get_buf()'s errorret, is gone: its tail is ml_get_invalid(), statement for statement, and no goto in the core jumps into a block")
	r.Say("the error path is vim's internal error for a line the memline cannot give; no key reaches it, so no probe can")
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	seed := []byte("ione\rtwo\rthree\rfour\x1bgg")
	keys := [][]byte{seed, []byte("jJGkdd"), []byte(":q!\r")}
	ctl := [][]byte{seed, []byte("jJGkyy"), []byte(":q!\r")}
	s1, _, e1 := check.Stream(ob, keys, nil)
	s2, _, e2 := check.Stream(nb, keys, nil)
	s3, _, e3 := check.Stream(nb, ctl, nil)
	if e1 != nil || e2 != nil || e3 != nil {
		r.Say("a probe did not run: %v %v %v", e1, e2, e3)
		return harness.ErrReported
	}
	if s1 != s2 {
		r.Bad("lines are drawn differently on the two binaries")
	}
	if s3 == s2 {
		r.Bad("the CONTROL did not move")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: lines drawn, joined, moved through and deleted are the same on both binaries; the CONTROL moves")
	return nil
}
