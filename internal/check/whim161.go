package check

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim161", Whim161) }

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
func Whim161(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim161", "errorret")
	if err != nil {
		return err
	}
	r := c.r
	for _, side := range []struct {
		name, text string
		into       int
	}{{"input", c.old, 1}, {"output", c.new, 0}} {
		ast, err := parseCore(side.text)
		if err != nil {
			r.bad("%v", err)
			return r.done()
		}
		res := ccx.Gotos(ast)
		n := 0
		for _, f := range res.Left {
			if side.into == 1 && f.Fn == "ml_get_buf" && strings.Contains(f.What, "goto errorret jumps into a block") {
				n++
				continue
			}
			r.bad("the %s's %s: %s %s", side.name, f.Fn, f.Where, f.What)
		}
		if n != side.into {
			r.bad("the %s has %d gotos into ml_get_buf()'s error block, where it should have %d", side.name, n, side.into)
		}
	}
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	tail := strings.TrimPrefix(edit.W161Tail, "errorret:\n")
	body := edit.W161Func[strings.Index(edit.W161Func, "questions[4];\n")+len("questions[4];\n") : strings.LastIndex(edit.W161Func, "}")]
	if norm(tail) != norm(body) || !strings.Contains(c.new, edit.W161Func) {
		r.bad("ml_get_invalid() is not the old tail, statement for statement")
	}
	if strings.Contains(c.new, "errorret") || !strings.Contains(c.old, "    static char_u questions[4];\n\n    if (lnum > buf->b_ml.ml_line_count)\n") {
		r.bad("the error path or its buffer is not where this check expects it")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("the one goto that jumped into a block, ml_get_buf()'s errorret, is gone: its tail is ml_get_invalid(), statement for statement, and no goto in the core jumps into a block")
	r.say("the error path is vim's internal error for a line the memline cannot give; no key reaches it, so no probe can")
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	seed := []byte("ione\rtwo\rthree\rfour\x1bgg")
	keys := [][]byte{seed, []byte("jJGkdd"), []byte(":q!\r")}
	ctl := [][]byte{seed, []byte("jJGkyy"), []byte(":q!\r")}
	s1, _, e1 := stream(ob, keys, nil)
	s2, _, e2 := stream(nb, keys, nil)
	s3, _, e3 := stream(nb, ctl, nil)
	if e1 != nil || e2 != nil || e3 != nil {
		r.say("a probe did not run: %v %v %v", e1, e2, e3)
		return harness.ErrReported
	}
	if s1 != s2 {
		r.bad("lines are drawn differently on the two binaries")
	}
	if s3 == s2 {
		r.bad("the CONTROL did not move")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: lines drawn, joined, moved through and deleted are the same on both binaries; the CONTROL moves")
	return nil
}
