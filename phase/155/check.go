package p155

// Whim phase 155, the check -- call arguments with effects are evaluated in gcc's order.
// See phase/155/edit.go, and GOALS.md.
//
// phase/155/check.go measures gcc's order on the input's code, requires
// no argument pair left and the binary pairs left to right in the output's
// code, and probes the rewritten calls.

import (
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim155", Check) }

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
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim155", "argorder")
	if err != nil {
		return err
	}
	r := c.R
	in, err := check.AsmCalls(filepath.Join(c.State, "old.c"))
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	oldLines := strings.Split(c.Old, "\n")
	saves, cols, files := 0, 0, 0
	for k, l := range oldLines {
		ln := k + 1
		switch {
		case regexp.MustCompile(`= vim_strnsave\(ml_get\w*\(.*\), ml_get\w*_len\(`).MatchString(l):
			m := regexp.MustCompile(`(ml_get\w*)\(`).FindStringSubmatch(l)
			get := m[1]
			if a, b := check.FirstOf(in[ln], []string{get + "_len"}), check.FirstOf(in[ln], []string{get}); a < 0 || b < 0 || a > b {
				r.Bad("line %d: gcc does not call %s_len() before %s(): %v", ln, get, get, in[ln])
			}
			saves++
		case strings.Contains(l, "col_print(buf2, sizeof(buf2), ml_get_curline_len(), linetabsize_str(p));"):
			if a, b := check.FirstOf(in[ln], []string{"linetabsize_str"}), check.FirstOf(in[ln], []string{"ml_get_curline_len"}); a < 0 || b < 0 || a > b {
				r.Bad("line %d: gcc does not call linetabsize_str() first: %v", ln, in[ln])
			}
			cols++
		case strings.Contains(l, `(curbuf->b_flags & BF_NEW) ? new_file_message() : "", `):
			if a, b := check.FirstOf(in[ln], []string{"new_file_message"}), check.FirstOf(in[ln], []string{"shortmess"}); a < 0 || b < 0 || a > b {
				r.Bad("line %d: gcc does not call new_file_message() before shortmess(): %v", ln, in[ln])
			}
			files++
		}
	}
	if saves != edit.W155Saves || cols != 1 || files != 1 {
		r.Bad("the input's sites are %d, %d and %d, where this phase rewrites %d, 1 and 1", saves, cols, files, edit.W155Saves)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("measured on the input's code: at every one of the %d rewritten calls gcc evaluates the hoisted argument first", saves+cols+files)

	ast, err := check.ParseCore(c.New)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	res := ccx.Order(ast)
	Out, err := check.AsmCalls(c.F)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	for _, f := range res.Left {
		if !strings.Contains(f.What, "Expression: calls on both sides") || strings.HasPrefix(f.What, "call arguments") {
			r.Bad("an unsequenced pair is left that is not binary operands: %s %s", f.Where, f.What)
			continue
		}
		m := w155Pair.FindStringSubmatch(f.What)
		ln, _ := strconv.Atoi(strings.Split(f.Where, ":")[1])
		var calls []string
		for d := 0; d < 3; d++ {
			calls = append(calls, Out[ln+d]...)
		}
		a, b := check.FirstOf(calls, strings.Fields(m[1])), check.FirstOf(calls, strings.Fields(m[2]))
		if a < 0 || b < 0 || a > b {
			r.Bad("%s: gcc does not call the left operand first: %v", f.Where, calls)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("no two arguments of a call both have effects; the %d binary operand pairs that do are called left to right by gcc, as Go evaluates them", len(res.Left))
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	seed := []byte("ione\rtwo\rthree\x1bgg")
	probes := []struct{ What, Keys, ctl string }{
		{":copy", ":1copy 2\r", ":1copy 3\r"},
		{":move", ":1move 3\r", ":1move 2\r"},
		{"opening a line", "jox\x1b", "jOx\x1b"},
		{"CTRL-G", "\x07", "j\x07"},
		{"g CTRL-G", "jlg\x07", "jg\x07"},
		{"a Tab", "A\tx\x1b", "A\ty\x1b"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr.Keys), []byte(":q!\r")}
		s1, _, e1 := check.Stream(ob, keys, nil)
		s2, _, e2 := check.Stream(nb, keys, nil)
		s3, _, e3 := check.Stream(nb, [][]byte{seed, []byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.Say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.Bad("%s is written differently on the two binaries", pr.What)
		}
		if s3 == s2 {
			r.Bad("the CONTROL for %s did not move", pr.What)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: :copy, :move, opening a line, CTRL-G, g CTRL-G and a Tab draw the same on both binaries; each CONTROL moves")
	return nil
}
