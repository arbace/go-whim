package p137

// Whim phase 137 -- the changedtick is a number.  See GOAL.md.
//
// b:changedtick was a dictionary item inside buf_T, read as
// ((buf)->b_ct_di.di_tv.vval); with no buffer variables left it is a number
// in a typval in a struct, and the one field keeping typval_T alive in buf_T.
// The field becomes a varnumber_T and its 18 uses name it.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim137", Edit) }

var w137Tick = regexp.MustCompile(`( *)\(\((\w+)\)->b_ct_di\.di_tv\.vval\)( *)`)

// W137InitBody is init_changedtick()'s Body after this phase, inside its braces.
const W137InitBody = `    buf->b_changedtick = 0;`

// W137Tick is the rule for one read or write of the changedtick, exported so
// the check applies the identical one: `((X)->b_ct_di.di_tv.vval)`, the
// expansion of CHANGEDTICK(X), becomes `X->b_changedtick`, and the spaces the
// expansion left around it become what the code around it would have had --
// none after `(` or `++` or before `;` or `)`, one elsewhere, and an
// indentation at the start of a line.
func W137Tick(text string) (string, int) {
	n := 0
	var b strings.Builder
	last := 0
	for _, m := range w137Tick.FindAllStringSubmatchIndex(text, -1) {
		before := text[last:m[0]]
		b.WriteString(before)
		n++
		lead := text[m[2]:m[3]]
		x := text[m[4]:m[5]]
		prev := strings.TrimRight(text[:m[0]], " ")
		switch {
		case m[0] == 0 || text[m[0]-1] == '\n':
			b.WriteString(lead[:len(lead)-len(lead)%4])
		case strings.HasSuffix(prev, "(") || strings.HasSuffix(prev, "++"):
		default:
			b.WriteString(" ")
		}
		b.WriteString(x + "->b_changedtick")
		next := text[m[1]:]
		if !strings.HasPrefix(next, ";") && !strings.HasPrefix(next, ")") {
			b.WriteString(" ")
		}
		last = m[1]
	}
	b.WriteString(text[last:])
	return b.String(), n
}

// Whim137 makes the changedtick a number.
//
// vim kept b:changedtick as a dictionary item inside buf_T, so the buffer's
// own variables could hold it without a copy: CHANGEDTICK(buf) read the number
// Out of a dictitem16_T's typval.  Whim has no buffer variables since the eval
// layer went, so the dictionary item is a number in a typval in a struct, and
// it is the one field that keeps typval_T -- and through it the list, dict,
// type and class structures -- alive in buf_T.  The field becomes the number;
// init_changedtick() stops setting a type, a lock and flags nothing reads.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "tick", W: w}
	Out, err := p.Literal(text, "    dictitem16_T b_ct_di;\n", "    varnumber_T b_changedtick;\n",
		"buf_T's changedtick is a varnumber_T", 1)
	if err != nil {
		return nil, err
	}
	Out, _, err = cutil.ReplaceBody(Out, "init_changedtick", W137InitBody)
	if err != nil {
		return nil, p.Die("init_changedtick: %v", err)
	}
	p.Say("init_changedtick() sets it to 0, and no type, lock or flags")
	s, n := W137Tick(string(Out))
	if n != 18 {
		return nil, p.Die("the changedtick is read or written %d times, and this phase was written against 18", n)
	}
	p.Say(fmt.Sprintf("its %d reads and writes name the field", n))
	return []byte(s), nil
}
