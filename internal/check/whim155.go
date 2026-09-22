package check

import (
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim155", Whim155) }

var w155Pair = regexp.MustCompile(`calls on both sides, \[([^\]]*)\] and \[([^\]]*)\]`)

// Whim155 is phase 155's check: arguments with effects are evaluated in the
// order gcc evaluated them.
//
//  1. GCC'S ORDER, MEASURED on the input's own code: at each rewritten call
//     the argument this phase hoists is the one gcc's code calls first -- the
//     length before the line, linetabsize_str() before ml_get_curline_len(),
//     new_file_message() before shortmess() -- read from the input compiled
//     with -g and disassembled line by line.
//  2. NO ARGUMENT PAIR IS LEFT: internal/ccx's Order finds no two arguments of
//     one call that both call a function with an effect; what it finds is
//     binary operands only, and each of them, in the OUTPUT's code, calls its
//     left operand's functions first -- the order Go evaluates them in.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: :copy, :move, opening a line, CTRL-G, g CTRL-G and a Tab,
//     the paths through the rewritten calls, draw the same on both binaries.
func Whim155(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim155", "argorder")
	if err != nil {
		return err
	}
	r := c.r
	in, err := asmCalls(filepath.Join(c.state, "old.c"))
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	oldLines := strings.Split(c.old, "\n")
	saves, cols, files := 0, 0, 0
	for k, l := range oldLines {
		ln := k + 1
		switch {
		case regexp.MustCompile(`= vim_strnsave\(ml_get\w*\(.*\), ml_get\w*_len\(`).MatchString(l):
			m := regexp.MustCompile(`(ml_get\w*)\(`).FindStringSubmatch(l)
			get := m[1]
			if a, b := firstOf(in[ln], []string{get + "_len"}), firstOf(in[ln], []string{get}); a < 0 || b < 0 || a > b {
				r.bad("line %d: gcc does not call %s_len() before %s(): %v", ln, get, get, in[ln])
			}
			saves++
		case strings.Contains(l, "col_print(buf2, sizeof(buf2), ml_get_curline_len(), linetabsize_str(p));"):
			if a, b := firstOf(in[ln], []string{"linetabsize_str"}), firstOf(in[ln], []string{"ml_get_curline_len"}); a < 0 || b < 0 || a > b {
				r.bad("line %d: gcc does not call linetabsize_str() first: %v", ln, in[ln])
			}
			cols++
		case strings.Contains(l, `(curbuf->b_flags & BF_NEW) ? new_file_message() : "", `):
			if a, b := firstOf(in[ln], []string{"new_file_message"}), firstOf(in[ln], []string{"shortmess"}); a < 0 || b < 0 || a > b {
				r.bad("line %d: gcc does not call new_file_message() before shortmess(): %v", ln, in[ln])
			}
			files++
		}
	}
	if saves != edit.W155Saves || cols != 1 || files != 1 {
		r.bad("the input's sites are %d, %d and %d, where this phase rewrites %d, 1 and 1", saves, cols, files, edit.W155Saves)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("measured on the input's code: at every one of the %d rewritten calls gcc evaluates the hoisted argument first", saves+cols+files)

	ast, err := parseCore(c.new)
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	res := ccx.Order(ast)
	out, err := asmCalls(c.f)
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	for _, f := range res.Left {
		if !strings.Contains(f.What, "Expression: calls on both sides") || strings.HasPrefix(f.What, "call arguments") {
			r.bad("an unsequenced pair is left that is not binary operands: %s %s", f.Where, f.What)
			continue
		}
		m := w155Pair.FindStringSubmatch(f.What)
		ln, _ := strconv.Atoi(strings.Split(f.Where, ":")[1])
		var calls []string
		for d := 0; d < 3; d++ {
			calls = append(calls, out[ln+d]...)
		}
		a, b := firstOf(calls, strings.Fields(m[1])), firstOf(calls, strings.Fields(m[2]))
		if a < 0 || b < 0 || a > b {
			r.bad("%s: gcc does not call the left operand first: %v", f.Where, calls)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("no two arguments of a call both have effects; the %d binary operand pairs that do are called left to right by gcc, as Go evaluates them", len(res.Left))
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	seed := []byte("ione\rtwo\rthree\x1bgg")
	probes := []struct{ what, keys, ctl string }{
		{":copy", ":1copy 2\r", ":1copy 3\r"},
		{":move", ":1move 3\r", ":1move 2\r"},
		{"opening a line", "jox\x1b", "jOx\x1b"},
		{"CTRL-G", "\x07", "j\x07"},
		{"g CTRL-G", "jlg\x07", "jg\x07"},
		{"a Tab", "A\tx\x1b", "A\ty\x1b"},
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
			r.bad("%s is written differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: :copy, :move, opening a line, CTRL-G, g CTRL-G and a Tab draw the same on both binaries; each CONTROL moves")
	return nil
}
