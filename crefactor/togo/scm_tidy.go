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
type scmTidyStats struct {
	joinsInPlace, invariants int
}

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
	names := map[string]bool{}
	for n := range memNames {
		names[n] = true
	}
	scmLocalNames(fn, names)
	scmJoinsInPlace(fn, names, st)
	scmInvariants(fn, names, st)
	return scmPrint(fn, 0, 0) + "\n", nil
}

// --- scopes ----------------------------------------------------------------

func slist(kids ...*sform) *sform { return &sform{kids: kids} }

// scmIdentAtom says an atom is an identifier, which a binding may name.
func scmIdentAtom(s string) bool {
	if s == "" || s[0] == '"' || s[0] == '#' || s[0] == '\'' {
		return false
	}
	c := s[0]
	if c >= '0' && c <= '9' {
		return false
	}
	if (c == '-' || c == '+') && len(s) > 1 && s[1] >= '0' && s[1] <= '9' {
		return false
	}
	return true
}

// scmBinds are the names a binding form binds for its body, and where its
// body starts; ok false when x binds nothing.
func scmBinds(x *sform) (names []string, from int, ok bool) {
	add := func(f *sform) {
		if !f.isList() {
			names = append(names, f.atom)
			return
		}
		for _, k := range f.kids {
			if !k.isList() {
				names = append(names, k.atom)
			}
		}
	}
	switch h := x.head(); h {
	case "let", "let*", "letrec", "letrec*", "let-values", "let*-values":
		if len(x.kids) < 3 {
			return nil, 0, false
		}
		bs, from := x.kids[1], 2
		if h == "let" && !bs.isList() {
			names = append(names, bs.atom)
			bs, from = x.kids[2], 3
		}
		for _, b := range bs.kids {
			if b.isList() && len(b.kids) >= 1 {
				add(b.kids[0])
			}
		}
		return names, from, true
	case "define", "lambda":
		if len(x.kids) < 3 {
			return nil, 0, false
		}
		sig := x.kids[1]
		if sig.isList() {
			ks := sig.kids
			if h == "define" && len(ks) > 0 {
				ks = ks[1:]
			}
			for _, k := range ks {
				if !k.isList() {
					names = append(names, k.atom)
				}
			}
		}
		return names, 2, true
	}
	return nil, 0, false
}

// scmFree adds the identifiers x names that it does not bind itself to
// out.
func scmFree(x *sform, bound map[string]int, out map[string]bool) {
	if !x.isList() {
		if scmIdentAtom(x.atom) && bound[x.atom] == 0 {
			out[x.atom] = true
		}
		return
	}
	names, from, ok := scmBinds(x)
	if !ok {
		for _, k := range x.kids {
			scmFree(k, bound, out)
		}
		return
	}
	h := x.head()
	// the bindings' values: a let's in the scope outside, a let*'s each
	// after the ones before it
	if h != "define" && h != "lambda" {
		bs := x.kids[from-1]
		seq := h == "let*" || h == "let*-values" || h == "letrec" || h == "letrec*"
		var pushed []string
		if h == "letrec" || h == "letrec*" {
			for _, b := range bs.kids {
				if b.isList() && len(b.kids) == 2 && !b.kids[0].isList() {
					bound[b.kids[0].atom]++
					pushed = append(pushed, b.kids[0].atom)
				}
			}
		}
		for _, b := range bs.kids {
			if !b.isList() || len(b.kids) != 2 {
				continue
			}
			scmFree(b.kids[1], bound, out)
			if seq && h != "letrec" && h != "letrec*" {
				var ns []string
				if b.kids[0].isList() {
					for _, k := range b.kids[0].kids {
						ns = append(ns, k.atom)
					}
				} else {
					ns = []string{b.kids[0].atom}
				}
				for _, n := range ns {
					bound[n]++
					pushed = append(pushed, n)
				}
			}
		}
		for _, n := range pushed {
			bound[n]--
		}
	}
	for _, n := range names {
		bound[n]++
	}
	for _, k := range x.kids[from:] {
		scmFree(k, bound, out)
	}
	for _, n := range names {
		bound[n]--
	}
}

// sref is a place a name is used: the list it is in, at kids[at], the
// names bound between the search's root and it, and the lists from the
// root to it (the root first, in last).
type sref struct {
	in    *sform
	at    int
	bound map[string]bool
	path  []*sform
}

// scmRefs are the uses of name in root's kids from from on, with the names
// bound around each below root.
func scmRefs(root *sform, from int, name string) []sref {
	var out []sref
	var walk func(x *sform, i0 int, bound []string, path []*sform, binds bool)
	walk = func(x *sform, i0 int, bound []string, path []*sform, binds bool) {
		path = append(path, x)
		names, bfrom, ok := scmBinds(x)
		if !binds {
			ok = false
		}
		h := x.head()
		inner := bound
		if ok {
			inner = append(append([]string{}, bound...), names...)
		}
		for i := i0; i < len(x.kids); i++ {
			k := x.kids[i]
			if !k.isList() {
				if k.atom == name {
					b := bound
					if ok && i >= bfrom {
						b = inner
					}
					set := map[string]bool{}
					for _, n := range b {
						set[n] = true
					}
					out = append(out, sref{in: x, at: i, bound: set, path: append([]*sform{}, path...)})
				}
				continue
			}
			switch {
			case ok && i >= bfrom:
				walk(k, 0, inner, path, true)
			case ok && i == bfrom-1 && h != "define" && h != "lambda":
				// the bindings: a let's values in the scope outside, a
				// let*'s each after the bindings before it, a letrec's
				// inside
				seen := bound
				if h == "letrec" || h == "letrec*" {
					seen = inner
				}
				bpath := append(path, k)
				for _, b := range k.kids {
					if b.isList() && len(b.kids) == 2 {
						walk(b, 1, seen, bpath, false)
						if h == "let*" || h == "let*-values" {
							var ns []string
							if b.kids[0].isList() {
								for _, a := range b.kids[0].kids {
									ns = append(ns, a.atom)
								}
							} else {
								ns = []string{b.kids[0].atom}
							}
							seen = append(append([]string{}, seen...), ns...)
						}
					} else {
						walk(b, 0, seen, bpath, true)
					}
				}
			default:
				walk(k, 0, bound, path, true)
			}
		}
	}
	walk(root, from, nil, nil, false)
	return out
}

// scmLocalNames adds the names of the C's locals in the call's frame
// (define-c-local), which read memory, to names.
func scmLocalNames(x *sform, names map[string]bool) {
	if !x.isList() {
		return
	}
	if x.head() == "define-c-local" && len(x.kids) >= 3 {
		names[x.kids[1].atom] = true
		names[x.kids[2].atom] = true
		return
	}
	for _, k := range x.kids {
		scmLocalNames(k, names)
	}
}

// --- joins and loops ---------------------------------------------------------

// scmProcs is the body that holds a function's local procedures: the
// define's, or that of the let that binds mem and fr.
func scmProcs(fn *sform) (*sform, int) {
	body, from := fn, 2
	for {
		hasDefs := false
		for _, k := range body.kids[from:] {
			if k.head() == "define" || k.head() == "define-c-local" {
				hasDefs = true
			}
		}
		if hasDefs || len(body.kids) != from+1 {
			return body, from
		}
		inner := body.kids[from]
		if inner.head() != "let" && inner.head() != "let*" {
			return body, from
		}
		_, f, _ := scmBinds(inner)
		body, from = inner, f
	}
}

// scmJoinsInPlace writes each join that one place calls where it is
// called: its body there, its parameters bound to the call's arguments
// but where the argument is the parameter's own name.  A join whose body
// names what a binding around the call would capture stays, and so does
// one with a parameter of a name that reads memory.
func scmJoinsInPlace(fn *sform, memNames map[string]bool, st *scmTidyStats) {
	procs, from := scmProcs(fn)
	for changed := true; changed; {
		changed = false
		for i := from; i < len(procs.kids); i++ {
			d := procs.kids[i]
			if d.head() != "define" || len(d.kids) < 3 || !d.kids[1].isList() || len(d.kids[1].kids) == 0 {
				continue
			}
			name := d.kids[1].kids[0].atom
			if !strings.HasPrefix(name, "join") || len(scmRefs(d, 2, name)) != 0 {
				continue
			}
			// the calls, the join taken out of the body while they are
			// looked for
			procs.kids = append(procs.kids[:i:i], procs.kids[i+1:]...)
			restore := func() {
				procs.kids = append(procs.kids[:i:i], append([]*sform{d}, procs.kids[i:]...)...)
			}
			refs := scmRefs(procs, from, name)
			if len(refs) != 1 || refs[0].at != 0 || !scmInline(d, refs[0], memNames) {
				restore()
				continue
			}
			st.joinsInPlace++
			changed = true
			break
		}
	}
}

// scmInline writes the join d where r calls it, or says why not.
func scmInline(d *sform, r sref, memNames map[string]bool) bool {
	params := d.kids[1].kids[1:]
	args := r.in.kids[1:]
	if len(args) != len(params) {
		return false
	}
	free := map[string]bool{}
	bound := map[string]int{}
	for _, p := range params {
		if p.isList() || memNames[p.atom] {
			return false
		}
		bound[p.atom]++
	}
	for _, k := range d.kids[2:] {
		scmFree(k, bound, free)
	}
	for n := range free {
		if r.bound[n] {
			return false // a binding around the call would capture it
		}
	}
	var binds []*sform
	for k, p := range params {
		if args[k].isList() || args[k].atom != p.atom {
			binds = append(binds, &sform{brack: true, kids: []*sform{p, args[k]}})
		}
	}
	forms := d.kids[2:]
	if len(binds) > 0 {
		forms = []*sform{slist(append([]*sform{satom("let"), slist(binds...)}, forms...)...)}
	}
	return scmSplice(r, forms)
}

// scmBodyAt says the form at kids[at] of x, which is in parent, is in a
// body: one of several forms evaluated in turn, the last its value.
func scmBodyAt(x *sform, at int, parent *sform) bool {
	if x.brack {
		h := ""
		if parent != nil {
			h = parent.head()
		}
		return at >= 1 && (h == "cond" || h == "case")
	}
	switch h := x.head(); h {
	case "when", "unless", "define", "lambda":
		return at >= 2
	case "begin":
		return at >= 1
	}
	if _, from, ok := scmBinds(x); ok {
		return at >= from
	}
	return false
}

// scmSplice puts forms where r's call is: in a body, as they are; one
// form anywhere; several as an if's arm, the if a cond; else a begin.
func scmSplice(r sref, forms []*sform) bool {
	call := r.in
	if len(r.path) < 2 {
		return false
	}
	parent := r.path[len(r.path)-2]
	var grand *sform
	if len(r.path) >= 3 {
		grand = r.path[len(r.path)-3]
	}
	pos := -1
	for i, k := range parent.kids {
		if k == call {
			pos = i
		}
	}
	if pos < 0 {
		return false
	}
	replace := func(with []*sform) {
		kids := append([]*sform{}, parent.kids[:pos]...)
		kids = append(kids, with...)
		parent.kids = append(kids, parent.kids[pos+1:]...)
	}
	switch {
	case len(forms) == 1 || scmBodyAt(parent, pos, grand) || len(r.path) == 2:
		// the root's own kids are a body too
		replace(forms)
	case parent.head() == "if" && len(parent.kids) == 4 && pos >= 2:
		then, els := []*sform{parent.kids[2]}, []*sform{parent.kids[3]}
		if pos == 2 {
			then = forms
		} else {
			els = forms
		}
		parent.kids = []*sform{satom("cond"),
			{brack: true, kids: append([]*sform{parent.kids[1]}, then...)},
			{brack: true, kids: append([]*sform{satom("else")}, els...)}}
	default:
		replace([]*sform{slist(append([]*sform{satom("begin")}, forms...)...)})
	}
	return true
}

// scmInvariants takes out of each named let the bindings of a name to
// itself that every jump back passes on unchanged: the loop's body sees
// the binding outside, which is the same value.
func scmInvariants(x *sform, memNames map[string]bool, st *scmTidyStats) {
	if !x.isList() {
		return
	}
	for _, k := range x.kids {
		scmInvariants(k, memNames, st)
	}
	if x.head() != "let" || len(x.kids) < 4 || x.kids[1].isList() {
		return
	}
	name, bs := x.kids[1].atom, x.kids[2]
	refs := scmRefs(x, 3, name)
	for _, r := range refs {
		if r.at != 0 || r.bound[name] {
			return // a value, or another binding of the name
		}
	}
	var keep []int
	for k, b := range bs.kids {
		if !b.isList() || len(b.kids) != 2 || b.kids[1].isList() || b.kids[0].atom != b.kids[1].atom || memNames[b.kids[0].atom] {
			keep = append(keep, k)
			continue
		}
		p := b.kids[0].atom
		same := true
		for _, r := range refs {
			args := r.in.kids[1:]
			if len(args) != len(bs.kids) || args[k].isList() || args[k].atom != p || r.bound[p] {
				same = false
			}
		}
		if !same {
			keep = append(keep, k)
		}
	}
	if len(keep) == len(bs.kids) {
		return
	}
	st.invariants += len(bs.kids) - len(keep)
	pick := func(xs []*sform) []*sform {
		var out []*sform
		for _, k := range keep {
			out = append(out, xs[k])
		}
		return out
	}
	for _, r := range refs {
		r.in.kids = append([]*sform{r.in.kids[0]}, pick(r.in.kids[1:])...)
	}
	bs.kids = pick(bs.kids)
}
