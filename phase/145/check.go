package p145

// Whim phase 145, the check -- check_termcode() has no goto.
// See phase/145/edit.go, and GOALS.md.
//
// phase/145/check.go proves from the input that the jump's path is the
// if/else, requires the else to be the skipped code byte for byte, and probes an
// OSC response split across two writes.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim145", Check) }

// Whim145 is phase 145's check: check_termcode() has no goto.
//
//  1. WHY THE IF/ELSE IS THE JUMP, on the input: handle_osc labels the OSC
//     branch of the if-chain inside `if (key_name[0] == NUL)`, and after that
//     chain the block holds nothing but its closing brace -- so the jump ran
//     the OSC handling and then what follows the block, which is what the
//     else leaves for the if's own Body.
//  2. THE CUT: the jump's if now calls handle_osc() and returns -1 on FAIL,
//     as the label's code did; its else, with the four spaces taken off, is
//     the input's code from after the jump through the end of the block,
//     byte for byte, less the label.  No goto is left.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE: an OSC response split across two writes, then text typed, is
//     drawn the same by both binaries; the CONTROL types other text and
//     moves, and without the response the output is another again -- so the
//     probe sees the response handled.  Whether the pty hands the two writes
//     to the editor as two reads, which is when osc_state.processing is set,
//     is the kernel's; the byte-for-byte else is the evidence for that path.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim145", "oscgoto")
	if err != nil {
		return err
	}
	r := c.R
	fn := func(text string) string {
		b := []byte(text)
		a, z, ok := cutil.FindDefinition(b, cutil.Blank(b), "check_termcode")
		if !ok {
			return ""
		}
		return text[a:z]
	}
	in, Out := fn(c.Old), fn(c.New)
	jump := "            modifiers = 0;\n            goto handle_osc;\n        }\n\n"
	ji, li := strings.Index(in, jump), strings.Index(in, "handle_osc:\n")
	bi := strings.LastIndex(in[:max(li, 0)], "        if (key_name[0] == NUL)\n        {\n")
	if ji < 0 || li < 0 || bi < 0 {
		r.Bad("the input's jump, label or block is not where this phase expects")
		return r.Done()
	}
	b := cutil.Blank([]byte(in))
	cl := cutil.Match(b, bi+strings.Index(in[bi:], "{"))
	chainEnd := strings.LastIndex(in[:cl], "}")
	if strings.TrimSpace(in[chainEnd+1:cl]) != "" {
		r.Bad("the block holding the label has code after its if-chain")
	}
	end := cl + 1 + strings.Index(in[cl:], "\n")
	skipped := strings.Replace(in[ji+len(jump):end], "handle_osc:\n", "", 1)
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("on the input the label's chain is the last thing in its block: the jump ran the OSC handling, then what follows the block")

	want := "            modifiers = 0;\n            if (handle_osc(tp, len, key_name, &slen) == FAIL)\n            {\n                return -1;\n            }\n        }\n        else\n        {\n"
	oi := strings.Index(Out, want)
	if oi < 0 {
		r.Bad("the jump's if does not handle the OSC response itself")
		return r.Done()
	}
	rest := Out[oi+len(want):]
	var dd strings.Builder
	for _, ln := range strings.SplitAfter(skipped, "\n") {
		if strings.TrimSpace(ln) == "" {
			dd.WriteString(ln)
		} else {
			dd.WriteString("    " + ln)
		}
	}
	if !strings.HasPrefix(rest, dd.String()+"        }\n") {
		r.Bad("the else is not the code the jump skipped, byte for byte")
	}
	if strings.Contains(Out, "goto ") {
		r.Bad("check_termcode() still jumps")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the if handles the response and the else is the skipped code, byte for byte, four spaces in; no goto left")
	if err := c.Gate(true); err != nil {
		return err
	}
	ob, nb := c.Bins()
	osc1 := []byte("\x1b]11;rgb:1234/5678/9abc")
	keys := [][]byte{osc1, []byte("\x07ihello\x1b"), []byte(":q!\r")}
	ctl := [][]byte{osc1, []byte("\x07iHello\x1b"), []byte(":q!\r")}
	plain := [][]byte{[]byte("ihello\x1b"), []byte(":q!\r")}
	s1, _, e1 := check.Stream(ob, keys, nil)
	s2, _, e2 := check.Stream(nb, keys, nil)
	s3, _, e3 := check.Stream(nb, ctl, nil)
	s4, _, e4 := check.Stream(nb, plain, nil)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		r.Say("a probe did not run: %v %v %v %v", e1, e2, e3, e4)
		return harness.ErrReported
	}
	if s1 != s2 {
		r.Bad("the split OSC response is handled differently on the two binaries")
	}
	if s3 == s2 || s4 == s2 {
		r.Bad("a CONTROL did not move: other text %v, no response %v", s3 != s2, s4 != s2)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: an OSC response in two writes, then typing, is drawn the same by both binaries; other text, and no response, each move")
	return nil
}
