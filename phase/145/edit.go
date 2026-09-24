package p145

// Whim phase 145 -- check_termcode() has no goto.  See GOAL.md.
//
// While an OSC response arrived over several reads, the loop jumped into the
// OSC branch of a later if-chain (tx/FINDINGS.md, 11).  The jump's if handles
// the response itself and everything the jump skipped becomes its else.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

// w145LabelLine is the label with the whole of its line: the indentation before
// it and the newline after it.
var w145LabelLine = regexp.MustCompile(`(?m)^[ \t]*handle_osc:\n`)

func init() { edit.Register("whim145", Edit) }

const (
	w145Jump = `        if (osc_state.processing)
        {
            tp[len] = NUL;
            key_name[0] = NUL;
            key_name[1] = NUL;
            modifiers = 0;
            goto handle_osc;
        }
`
	w145Label = "handle_osc:\n"
	w145Block = "        if (key_name[0] == NUL)\n        {\n"
)

// Whim145 takes the jump into an if Body Out of check_termcode().
//
// While an OSC response was arriving over several reads, check_termcode()
// jumped from the top of its loop to handle_osc, a label inside the OSC branch
// of the if-chain in `if (key_name[0] == NUL)`, skipping everything between
// (tx/FINDINGS.md, 11).  Nothing follows that chain inside its block, so the
// jump did exactly this: the OSC handling, then the code after the block.  So
// the jump's if gets the handling as its Body, and everything it skipped --
// from the key's first byte through the end of the block -- becomes its else.
// A continue or break in that code binds to the loop it bound to: an if does
// not catch either.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "oscgoto", W: w}
	return p.InFunction(text, "check_termcode", func(seg []byte) ([]byte, error) {
		s := string(seg)
		j := strings.Index(s, w145Jump)
		l := strings.Index(s, w145Label)
		if j < 0 || strings.Count(s, w145Jump) != 1 || l < 0 || strings.Count(s, "handle_osc:") != 1 {
			return nil, p.Die("the jump to handle_osc or its label is not where this phase expects it")
		}
		// the block holding the label: the last `if (key_name[0] == NUL)` before it
		bi := strings.LastIndex(s[:l], w145Block)
		if bi < 0 {
			return nil, p.Die("the label is not inside `if (key_name[0] == NUL)`")
		}
		b := cutil.Blank([]byte(s))
		open := bi + len(w145Block) - 2
		cl := cutil.Match(b, open)
		if cl < 0 || l > cl {
			return nil, p.Die("the label is not inside that block")
		}
		// the chain ends where the block does: nothing but the closing brace after
		// the last branch
		end := cl + 1 + strings.Index(s[cl:], "\n")
		mid := s[j+len(w145Jump) : end]
		// THE LABEL'S WHOLE LINE, indentation included.  The canonical text
		// writes `            handle_osc:` twelve spaces in; replacing the bare
		// name left those twelve spaces standing and glued them to the line
		// below, which the `    ` of the re-indentation below then made
		// thirty-two.  Measured on q144: one line, and it is the label's.
		if n := len(w145LabelLine.FindAllString(mid, -1)); n != 1 {
			return nil, p.Die("the label's line is in the skipped code %d times, expected 1", n)
		}
		mid = w145LabelLine.ReplaceAllString(mid, "")
		var ib strings.Builder
		for _, ln := range strings.SplitAfter(mid, "\n") {
			if strings.TrimSpace(ln) == "" {
				ib.WriteString(ln)
			} else {
				ib.WriteString("    " + ln)
			}
		}
		Body := strings.Replace(w145Jump, "            goto handle_osc;\n",
			"            if (handle_osc(tp, len, key_name, &slen) == FAIL)\n            {\n                return -1;\n            }\n", 1)
		// w145Jump ends at the if's own closing brace and its newline, so the
		// else follows it directly.  It used to end at the BLANK LINE the
		// residue wrote after that brace, and the TrimSuffix here was what took
		// it back off; the canonical text writes no blank line there, and with
		// the literal ending in a newline a TrimSuffix would run the brace and
		// the `else` together.
		Body = Body + "        else\n        {\n" + ib.String() + "        }\n"
		s = s[:j] + Body + s[end:]
		if strings.Contains(s, "handle_osc:") || strings.Contains(s, "goto ") {
			return nil, p.Die("check_termcode() still jumps")
		}
		p.Say("while an OSC response is arriving, the loop handles it in the jump's own if, and everything the jump skipped is its else: no goto left")
		return []byte(s), nil
	})
}
