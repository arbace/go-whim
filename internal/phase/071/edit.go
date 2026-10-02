package p071

// Whim phase 71 (formerly 145) -- check_termcode() has no goto.  See GOAL.md.
//
// While an OSC response arrived over several reads, the loop jumped into the
// OSC branch of a later if-chain (internal/gen/FINDINGS.md, 11).  The jump's if handles
// the response itself and everything the jump skipped becomes its else.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// w71LabelLine is the label with the whole of its line: the indentation before
// it and the newline after it.
var w71LabelLine = regexp.MustCompile(edit.Line("handle_osc:"))

func init() { phase.Register("whim71", Edit) }

const (
	w71Jump = `        if (osc_state.processing)
        {
            tp[len] = NUL;
            key_name[0] = NUL;
            key_name[1] = NUL;
            modifiers = 0;
            goto handle_osc;
        }
`
	w71Label = "handle_osc:\n"
	w71Block = "        if (key_name[0] == NUL)\n        {\n"
)

// Whim71 takes the jump into an if Body Out of check_termcode().
//
// While an OSC response was arriving over several reads, check_termcode()
// jumped from the top of its loop to handle_osc, a label inside the OSC branch
// of the if-chain in `if (key_name[0] == NUL)`, skipping everything between
// (internal/gen/FINDINGS.md, 11).  Nothing follows that chain inside its block, so the
// jump did exactly this: the OSC handling, then the code after the block.  So
// the jump's if gets the handling as its Body, and everything it skipped --
// from the key's first byte through the end of the block -- becomes its else.
// A continue or break in that code binds to the loop it bound to: an if does
// not catch either.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "oscgoto", W: w}
	return p.InFunction(text, "check_termcode", func(seg []byte) ([]byte, error) {
		s := string(seg)
		j := strings.Index(s, w71Jump)
		l := strings.Index(s, w71Label)
		if j < 0 || strings.Count(s, w71Jump) != 1 || l < 0 || strings.Count(s, "handle_osc:") != 1 {
			return nil, p.Die("the jump to handle_osc or its label is not where this phase expects it")
		}
		// the block holding the label: the last `if (key_name[0] == NUL)` before it
		bi := strings.LastIndex(s[:l], w71Block)
		if bi < 0 {
			return nil, p.Die("the label is not inside `if (key_name[0] == NUL)`")
		}
		b := edit.Blank([]byte(s))
		open := bi + len(w71Block) - 2
		cl := edit.Match(b, open)
		if cl < 0 || l > cl {
			return nil, p.Die("the label is not inside that block")
		}
		// the chain ends where the block does: nothing but the closing brace after
		// the last branch
		end := cl + 1 + strings.Index(s[cl:], "\n")
		mid := s[j+len(w71Jump) : end]
		// THE LABEL'S WHOLE LINE, indentation included: the canonical text
		// writes `            handle_osc:` twelve spaces in.  Measured on
		// q144 of the old numbering: one line, and it is the label's.  The skipped code keeps its
		// indentation as the else's body; the canonical print re-lays it.
		if n := len(w71LabelLine.FindAllString(mid, -1)); n != 1 {
			return nil, p.Die("the label's line is in the skipped code %d times, expected 1", n)
		}
		mid = w71LabelLine.ReplaceAllString(mid, "")
		Body := strings.Replace(w71Jump, "            goto handle_osc;\n",
			"            if (handle_osc(tp, len, key_name, &slen) == FAIL)\n            {\n                return -1;\n            }\n", 1)
		// w71Jump ends at the if's own closing brace and its newline, so the
		// else follows it directly.  It used to end at the BLANK LINE the
		// residue wrote after that brace, and the TrimSuffix here was what took
		// it back off; the canonical text writes no blank line there, and with
		// the literal ending in a newline a TrimSuffix would run the brace and
		// the `else` together.
		Body = Body + "        else\n        {\n" + mid + "        }\n"
		s = s[:j] + Body + s[end:]
		if strings.Contains(s, "handle_osc:") || strings.Contains(s, "goto ") {
			return nil, p.Die("check_termcode() still jumps")
		}
		p.Say("while an OSC response is arriving, the loop handles it in the jump's own if, and everything the jump skipped is its else: no goto left")
		return []byte(s), nil
	})
}
