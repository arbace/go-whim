package togo

// scm_tidy.go reads a function the Scheme printer wrote (scm_fn.go) back
// as forms, rewrites them by rules each of which keeps what the function
// does, and prints them again in a layout of its own (doc/SCHEME-IDIOMS.md,
// the second pass).  The printer composes its forms as text a block at a
// time, and some of what a Scheme programmer would write differently shows
// only once a function is whole: a line longer than a reader's screen, a
// join that one place calls, a loop's parameter that every jump back
// passes unchanged, a copy bound only to be read.
//
// The rules see the text's scopes -- what each let, named let, let-values,
// define and lambda binds -- and the names that read memory (the C's
// objects and the locals in the call's frame); each refuses where a name
// it would move could mean another binding, or another value, where it
// lands.

import (
	"fmt"
	"strings"
)

// sform is a form: an atom (its text), or a list, written in brackets
// when brack.
type sform struct {
	atom  string
	kids  []*sform
	brack bool
}

func satom(s string) *sform { return &sform{atom: s} }

func (x *sform) isList() bool { return x.atom == "" }

// head is a list's first atom.
func (x *sform) head() string {
	if x.isList() && len(x.kids) > 0 && !x.kids[0].isList() {
		return x.kids[0].atom
	}
	return ""
}

// scmRead reads the forms of text, or says where it cannot.
func scmRead(text string) ([]*sform, error) {
	r := &sreader{src: text}
	var out []*sform
	for {
		r.space()
		if r.i >= len(r.src) {
			return out, nil
		}
		x, err := r.form()
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
}

type sreader struct {
	src string
	i   int
}

func (r *sreader) space() {
	for r.i < len(r.src) {
		switch r.src[r.i] {
		case ' ', '\n', '\t', '\r':
			r.i++
		default:
			return
		}
	}
}

func (r *sreader) form() (*sform, error) {
	r.space()
	if r.i >= len(r.src) {
		return nil, fmt.Errorf("a form ends early")
	}
	switch c := r.src[r.i]; c {
	case '(', '[':
		close := byte(')')
		if c == '[' {
			close = ']'
		}
		r.i++
		x := &sform{brack: c == '[', kids: []*sform{}}
		for {
			r.space()
			if r.i >= len(r.src) {
				return nil, fmt.Errorf("a list ends early")
			}
			if r.src[r.i] == close {
				r.i++
				return x, nil
			}
			if r.src[r.i] == ')' || r.src[r.i] == ']' {
				return nil, fmt.Errorf("a %q closes a %q at %d", r.src[r.i], c, r.i)
			}
			k, err := r.form()
			if err != nil {
				return nil, err
			}
			x.kids = append(x.kids, k)
		}
	case ')', ']':
		return nil, fmt.Errorf("a %q with nothing open at %d", c, r.i)
	case '\'':
		// a quoted datum, '() the one the printer writes: an atom
		r.i++
		q, err := r.form()
		if err != nil {
			return nil, err
		}
		return satom("'" + q.flat()), nil
	case ';', '`', ',':
		return nil, fmt.Errorf("a %q, which the printer does not write, at %d", c, r.i)
	case '"':
		j := r.i + 1
		for j < len(r.src) && r.src[j] != '"' {
			if r.src[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(r.src) {
			return nil, fmt.Errorf("a string ends early")
		}
		s := r.src[r.i : j+1]
		r.i = j + 1
		return satom(s), nil
	}
	j := r.i
	if strings.HasPrefix(r.src[j:], "#\\") && j+2 < len(r.src) {
		j += 3 // a character: its first, whatever it is
	}
	for j < len(r.src) && !strings.ContainsRune(" \n\t\r()[]\";", rune(r.src[j])) {
		j++
	}
	s := r.src[r.i:j]
	r.i = j
	return satom(s), nil
}

// --- printing --------------------------------------------------------------

// scmWidth is the column a line should stay within where its form can
// break.
const scmWidth = 100

// flat is x on one line.
func (x *sform) flat() string {
	if !x.isList() {
		return x.atom
	}
	var b strings.Builder
	x.flatTo(&b)
	return b.String()
}

func (x *sform) flatTo(b *strings.Builder) {
	if !x.isList() {
		b.WriteString(x.atom)
		return
	}
	open, close := "(", ")"
	if x.brack {
		open, close = "[", "]"
	}
	b.WriteString(open)
	for i, k := range x.kids {
		if i > 0 {
			b.WriteByte(' ')
		}
		k.flatTo(b)
	}
	b.WriteString(close)
}

// scmBlockForms are the forms always printed on several lines: what binds
// or defines, and a dispatch.
var scmBlockForms = map[string]bool{"define": true, "let": true, "let*": true, "letrec": true, "letrec*": true,
	"let-values": true, "let*-values": true, "cond": true, "case": true, "lambda": true}

// canFlat says x may be printed on one line.
func (x *sform) canFlat() bool {
	if !x.isList() {
		return true
	}
	if scmBlockForms[x.head()] {
		return false
	}
	for _, k := range x.kids {
		if !k.canFlat() {
			return false
		}
	}
	return true
}

// scmPrint is x laid out from column col, trail closing brackets after
// it on its last line: its lines after the first indented absolutely.
func scmPrint(x *sform, col, trail int) string {
	if !x.isList() {
		return x.atom
	}
	if x.canFlat() {
		// short enough, or too short to be worth breaking
		if f := x.flat(); col+len(f)+trail <= scmWidth || len(f) <= 32 {
			return f
		}
	}
	open, close := "(", ")"
	if x.brack {
		open, close = "[", "]"
	}
	pad := func(n int) string { return "\n" + strings.Repeat(" ", n) }
	// body is forms each on a line at column c, the last before x's close
	body := func(fs []*sform, c int) string {
		var b strings.Builder
		for i, f := range fs {
			t := 0
			if i == len(fs)-1 {
				t = trail + 1
			}
			b.WriteString(pad(c) + scmPrint(f, c, t))
		}
		return b.String()
	}
	h := x.head()
	switch {
	case h == "define" && len(x.kids) >= 3, h == "lambda" && len(x.kids) >= 3:
		sig := x.kids[1].flat()
		if x.kids[1].isList() && len(x.kids[1].kids) > 1 && col+len(h)+2+len(sig) > scmWidth {
			sig = scmCall("("+x.kids[1].kids[0].atom+" ", x.kids[1].kids[1:], ")", col+len(h)+2, 0)
		}
		return "(" + h + " " + sig + body(x.kids[2:], col+2) + ")"
	case (h == "let" || h == "let*" || h == "letrec" || h == "letrec*" || h == "let-values" || h == "let*-values") && len(x.kids) >= 3:
		lead := "(" + h + " "
		binds, rest := x.kids[1], x.kids[2:]
		if h == "let" && !binds.isList() && len(x.kids) >= 4 {
			lead += binds.atom + " "
			binds, rest = x.kids[2], x.kids[3:]
		}
		return lead + scmBindings(binds, col+len(lead)) + body(rest, col+2) + ")"
	case h == "cond" && len(x.kids) > 1:
		return "(cond" + body(x.kids[1:], col+2) + ")"
	case h == "case" && len(x.kids) > 2:
		return "(case " + scmPrint(x.kids[1], col+6, 0) + body(x.kids[2:], col+2) + ")"
	case h == "if" && len(x.kids) == 4:
		return "(if " + scmPrint(x.kids[1], col+4, 0) + pad(col+4) + scmPrint(x.kids[2], col+4, 0) +
			pad(col+4) + scmPrint(x.kids[3], col+4, trail+1) + ")"
	case (h == "when" || h == "unless") && len(x.kids) >= 3:
		return "(" + h + " " + scmPrint(x.kids[1], col+len(h)+2, 0) + body(x.kids[2:], col+2) + ")"
	case h == "begin" && len(x.kids) >= 2:
		return "(begin" + body(x.kids[1:], col+2) + ")"
	case x.brack && len(x.kids) >= 1:
		// a clause: its test, and its forms under it
		t := 0
		if len(x.kids) == 1 {
			t = trail + 1
		}
		return "[" + scmPrint(x.kids[0], col+1, t) + body(x.kids[1:], col+1) + "]"
	case len(x.kids) == 0:
		return open + close
	case h == "":
		// a list whose head is a form: one under the other
		return open + scmPrint(x.kids[0], col+1, 0) + body(x.kids[1:], col+1) + close
	case len(x.kids) == 1:
		return open + h + close
	case h == "and" || h == "or" || h == "not":
		// the operands one under the other
		lead := open + h + " "
		var b strings.Builder
		b.WriteString(lead)
		for i, a := range x.kids[1:] {
			t := 0
			if i == len(x.kids)-2 {
				t = trail + 1
			}
			if i > 0 {
				b.WriteString(pad(col + len(lead)))
			}
			b.WriteString(scmPrint(a, col+len(lead), t))
		}
		return b.String() + close
	}
	return scmCall(open+h+" ", x.kids[1:], close, col, trail)
}

// scmCall is a call's lead, its arguments and its close from column col,
// trail closing brackets after it: the arguments under the first, as many
// to a line as fit when each can be printed flat, else one to a line.
func scmCall(lead string, args []*sform, close string, col, trail int) string {
	c := col + len(lead)
	flat := true
	for _, a := range args {
		if !a.canFlat() {
			flat = false
		}
	}
	var b strings.Builder
	b.WriteString(lead)
	pad := "\n" + strings.Repeat(" ", c)
	for i, a := range args {
		t := 0
		if i == len(args)-1 {
			t = trail + len(close)
		}
		if !flat {
			if i > 0 {
				b.WriteString(pad)
			}
			b.WriteString(scmPrint(a, c, t))
			continue
		}
		// packed: on the line while it fits, else on a line of its own
		f := a.flat()
		cur := b.String()
		at := col + len(cur)
		if nl := strings.LastIndexByte(cur, '\n'); nl >= 0 {
			at = len(cur) - nl - 1
		}
		if i > 0 {
			if at+1+len(f)+t > scmWidth {
				b.WriteString(pad)
				at = c
			} else {
				b.WriteByte(' ')
				at++
			}
		}
		b.WriteString(scmPrint(a, at, t))
	}
	return b.String() + close
}

// scmBindings is a let's bindings from column col: on one line when they
// are short and flat, else one to a line.
func scmBindings(bs *sform, col int) string {
	if !bs.isList() {
		return bs.atom
	}
	if bs.canFlat() {
		if f := bs.flat(); len(f) <= 50 && col+len(f) <= scmWidth {
			return f
		}
	}
	var b strings.Builder
	b.WriteString("(")
	for i, k := range bs.kids {
		t := 1
		if i == len(bs.kids)-1 {
			t = 2
		}
		if i > 0 {
			b.WriteString("\n" + strings.Repeat(" ", col+1))
		}
		if k.isList() && len(k.kids) == 2 {
			lead := "[" + k.kids[0].flat() + " "
			b.WriteString(lead + scmPrint(k.kids[1], col+1+len(lead), t) + "]")
		} else {
			b.WriteString(scmPrint(k, col+1, t-1))
		}
	}
	return b.String() + ")"
}

// scmTidyStats counts what the rules did, over the library.
type scmTidyStats struct{}

// scmTidy is the function text, one define, read back, rewritten and
// printed again; memNames are the file-scope objects' names, which read
// memory, and void says the function's value is not looked at.
func scmTidy(text string, memNames map[string]bool, void bool, st *scmTidyStats) (string, error) {
	forms, err := scmRead(text)
	if err != nil {
		return text, err
	}
	if len(forms) != 1 || forms[0].head() != "define" {
		return text, fmt.Errorf("not one define")
	}
	fn := forms[0]
	return scmPrint(fn, 0, 0) + "\n", nil
}
