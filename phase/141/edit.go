package p141

// Whim phase 141 -- regrepeat() does not jump into a case.  See GOAL.md.
//
// Seventeen character classes set their mask and jumped to do_class, a label
// inside the \s case (tx/FINDINGS.md, 11).  All eighteen now share one case
// that sets mask and testval in a switch on the same opcode, then runs the
// unchanged loop.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim141", Edit) }

var (
	w141Head  = "      case RE_WHITE:\n      case RE_WHITE + ADD_NL:\n        testval = mask = RI_WHITE;\ndo_class:\n"
	w141Class = regexp.MustCompile(`      case ([A-Z_]+):\n      case ([A-Z_]+) \+ ADD_NL:\n        ((?:testval = )?mask = RI_[A-Z]+;)\n        goto do_class;\n`)
)

// Whim141 takes the jumps into a case Out of regrepeat().
//
// The character classes \s \S \d \D ... \u \U were eighteen pairs of case
// labels: \s set its mask and testval and ran into the class loop, labelled
// do_class, and each of the other seventeen set its own and jumped to the
// label from further down the switch.  Go cannot jump into a case, and the
// transpilation restructured it by hand (tx/FINDINGS.md, 11).  Here all
// thirty-six labels lead to one case that sets the two variables in a switch
// on the same opcode, then runs the loop: the same assignments for each
// opcode, the loop unchanged, and no goto.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "doclass", W: w}
	return p.InFunction(text, "regrepeat", func(seg []byte) ([]byte, error) {
		s := string(seg)
		h := strings.Index(s, w141Head)
		if h < 0 || strings.Count(s, w141Head) != 1 {
			return nil, p.Die("regrepeat(): the \\s case and its do_class label are not where this phase expects them")
		}
		loopStart := h + len(w141Head)
		loopEnd := strings.Index(s[loopStart:], "\n        }\n        break;\n")
		if loopEnd < 0 {
			return nil, p.Die("regrepeat(): the class loop has no end")
		}
		loopEnd += loopStart + len("\n        }\n        break;\n")
		loop := s[loopStart:loopEnd]
		rest := s[loopEnd:]
		rest = strings.TrimPrefix(rest, "\n")
		ms := w141Class.FindAllStringSubmatchIndex(rest, -1)
		if len(ms) != 17 || ms[0][0] != 0 {
			return nil, p.Die("regrepeat(): %d classes jump to do_class right after it, and this phase was written against 17", len(ms))
		}
		for k := 1; k < len(ms); k++ {
			if ms[k][0] != ms[k-1][1] {
				return nil, p.Die("regrepeat(): the classes that jump to do_class are not one run")
			}
		}
		type cls struct{ op, assign string }
		all := []cls{{"RE_WHITE", "testval = mask = RI_WHITE;"}}
		for _, m := range ms {
			op, op2, as := rest[m[2]:m[3]], rest[m[4]:m[5]], rest[m[6]:m[7]]
			if op != op2 {
				return nil, p.Die("regrepeat(): %s is paired with %s + ADD_NL", op, op2)
			}
			all = append(all, cls{op, as})
		}
		after := rest[ms[len(ms)-1][1]:]
		if strings.Contains(after, "do_class") || strings.Contains(s[:h], "do_class") {
			return nil, p.Die("regrepeat(): do_class is reached from elsewhere")
		}
		var b strings.Builder
		for _, c := range all {
			fmt.Fprintf(&b, "      case %s:\n      case %s + ADD_NL:\n", c.op, c.op)
		}
		b.WriteString("        switch ( ((int)*(p)) )\n        {\n")
		for _, c := range all {
			fmt.Fprintf(&b, "          case %s:\n          case %s + ADD_NL:\n            %s\n            break;\n", c.op, c.op, c.assign)
		}
		b.WriteString("        }\n")
		b.WriteString(loop)
		p.Say(fmt.Sprintf("the %d class opcodes share one case, which sets mask and testval by opcode and then runs the class loop: no goto do_class", len(all)))
		return []byte(s[:h] + b.String() + after), nil
	})
}
