package togo

// ml_doc.go is the layout the OCaml backend prints with (ml.go): Wadler's
// pretty printer -- a document of text, breaks that are a space where
// their group fits on the line and a newline where it does not, nesting,
// and groups -- so that a function of the core reads as OCaml is laid out
// by hand, within mlWidth columns where its forms allow.

import "strings"

// mlWidth is the column a line should stay within.
const mlWidth = 100

type mdocKind int

const (
	mdText  mdocKind = iota
	mdLine           // a space, or a newline where the group breaks
	mdSoft           // nothing, or a newline where the group breaks
	mdHard           // always a newline
	mdNest           // kids indented by n more where they break
	mdGroup          // kids on one line if they fit
	mdCat
)

// mdoc is a document.
type mdoc struct {
	kind mdocKind
	s    string
	n    int
	kids []*mdoc
}

func mtext(s string) *mdoc { return &mdoc{kind: mdText, s: s} }

var (
	mline = &mdoc{kind: mdLine}
	mhard = &mdoc{kind: mdHard}
)

func mcat(ds ...*mdoc) *mdoc         { return &mdoc{kind: mdCat, kids: ds} }
func mnest(n int, ds ...*mdoc) *mdoc { return &mdoc{kind: mdNest, n: n, kids: ds} }
func mgroup(ds ...*mdoc) *mdoc       { return &mdoc{kind: mdGroup, kids: ds} }

// mjoin is ds with sep between them.
func mjoin(sep *mdoc, ds []*mdoc) *mdoc {
	var out []*mdoc
	for i, d := range ds {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, d)
	}
	return mcat(out...)
}

// flat says d holds no hard break.
func (d *mdoc) flat() bool {
	if d.kind == mdHard {
		return false
	}
	if d.kind == mdText {
		return !strings.Contains(d.s, "\n")
	}
	for _, k := range d.kids {
		if !k.flat() {
			return false
		}
	}
	return true
}

type mframe struct {
	ind int
	brk bool
	d   *mdoc
}

// mrender is d laid out from column 0.
func mrender(d *mdoc) string {
	var b strings.Builder
	col := 0
	stack := []mframe{{0, true, d}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch f.d.kind {
		case mdText:
			b.WriteString(f.d.s)
			if i := strings.LastIndexByte(f.d.s, '\n'); i >= 0 {
				col = len(f.d.s) - i - 1
			} else {
				col += len(f.d.s)
			}
		case mdLine, mdSoft:
			if f.brk {
				mnewline(&b, f.ind)
				col = f.ind
			} else if f.d.kind == mdLine {
				b.WriteByte(' ')
				col++
			}
		case mdHard:
			mnewline(&b, f.ind)
			col = f.ind
		case mdNest:
			for i := len(f.d.kids) - 1; i >= 0; i-- {
				stack = append(stack, mframe{f.ind + f.d.n, f.brk, f.d.kids[i]})
			}
		case mdCat:
			for i := len(f.d.kids) - 1; i >= 0; i-- {
				stack = append(stack, mframe{f.ind, f.brk, f.d.kids[i]})
			}
		case mdGroup:
			brk := !f.d.flat() || !mfits(mlWidth-col, f.d, stack)
			for i := len(f.d.kids) - 1; i >= 0; i-- {
				stack = append(stack, mframe{f.ind, brk, f.d.kids[i]})
			}
		}
	}
	return b.String()
}

// mnewline ends a line, its trailing spaces taken, and indents the next.
func mnewline(b *strings.Builder, ind int) {
	s := b.String()
	if t := strings.TrimRight(s, " "); len(t) != len(s) {
		b.Reset()
		b.WriteString(t)
	}
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(" ", ind))
}

// mfits says d, flat, and what follows it up to the next break fit in w
// columns.
func mfits(w int, d *mdoc, rest []mframe) bool {
	w -= mflatWidth(d, w)
	if w < 0 {
		return false
	}
	// what follows on the same line: up to the first break that breaks
	for i := len(rest) - 1; i >= 0 && w >= 0; i-- {
		n, stop := mtrail(rest[i].d, rest[i].brk, w)
		w -= n
		if stop {
			break
		}
	}
	return w >= 0
}

// mflatWidth is d's width on one line, or more than max.
func mflatWidth(d *mdoc, max int) int {
	switch d.kind {
	case mdText:
		return len(d.s)
	case mdLine:
		return 1
	case mdSoft:
		return 0
	case mdHard:
		return max + 1
	}
	n := 0
	for _, k := range d.kids {
		n += mflatWidth(k, max-n)
		if n > max {
			return n
		}
	}
	return n
}

// mtrail is the width d adds to the line before its first break, laid out
// with its breaks broken when brk, and whether it has such a break.
func mtrail(d *mdoc, brk bool, max int) (int, bool) {
	switch d.kind {
	case mdText:
		if i := strings.IndexByte(d.s, '\n'); i >= 0 {
			return i, true
		}
		return len(d.s), false
	case mdLine:
		if brk {
			return 0, true
		}
		return 1, false
	case mdSoft:
		return 0, brk
	case mdHard:
		return 0, true
	case mdGroup:
		// a group that follows lays itself out: count it flat when it
		// fits, else up to its first break
		if w := mflatWidth(d, max); w <= max && d.flat() {
			return w, false
		}
		brk = true
	}
	n := 0
	for _, k := range d.kids {
		m, stop := mtrail(k, brk, max-n)
		n += m
		if stop || n > max {
			return n, true
		}
	}
	return n, false
}
