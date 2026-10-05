package view

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// EDITS AS THEY ARE TYPED (doc/GRAPH.md, *Edits as they are typed*).  A
// Buffer is a view's text as an editor holds it: typed into change by
// change, each a byte range of its text replaced.  A buffer knows where it
// changed, so the alignment does not search: the text before the change
// and after it are the printed view's, and the edit is aligned from the
// least form around the change (the forms read from both texts at the
// same place, the one after shifted by what the change added), climbing
// only when that form cannot take it.  What is typed is half-made most of
// the time -- an open parenthesis, a name not yet whole -- and is not an
// error: the buffer keeps it PENDING, the graph untouched, the reason
// said, until a change makes the text an edit that stands; then the edit
// is made in place, in its session, and the buffer is the view printed
// again.

// A Buffer is a view of a session's graph, being typed into.
type Buffer struct {
	S      *Session
	Render Render
	Opt    EditOptions // IDs, FallOut: how its edits are made

	Text    string // what the buffer says
	Printed string // the view as the graph prints it
	Spans   []Span // Printed's span table
	Ix      *Index // the graph's, current
	Reason  string // why Text is pending, "" when it is Printed
}

// Open is a buffer on render's view of the session's graph.
func (s *Session) Open(render Render, opt EditOptions) (*Buffer, error) {
	b := &Buffer{S: s, Render: render, Opt: opt, Ix: NewIndex(s.G)}
	if err := b.reprint(); err != nil {
		return nil, err
	}
	return b, nil
}

// reprint prints the view again from the buffer's index; what was typed
// and pending is dropped.
func (b *Buffer) reprint() error {
	text, spans, err := b.Render(b.Ix)
	if err != nil {
		return err
	}
	b.Text, b.Printed, b.Spans, b.Reason = text, text, spans, ""
	return nil
}

// A Typed is what a change made of the buffer: Status "applied" (an edit
// made, Result its), "pending" (the text kept, the graph untouched,
// Reason why), "layout" (the text says the printed forms, spaced
// otherwise: kept, nothing to edit), or "same" (the text is the view as
// printed again).  Cursor is where the change's end is in the buffer's
// text after it: past what was typed, or, the view printed again, at the
// same place in the new print (MapPos).
type Typed struct {
	Status string
	Reason string
	Result *Result
	Cursor int
}

// Change replaces bytes start to end of the buffer's text by text, and
// makes the edit the buffer then says, when it stands.  Its error is a
// range that is not in the text; every other failure is a pending text.
func (b *Buffer) Change(start, end int, text string) (*Typed, error) {
	if start < 0 || end < start || end > len(b.Text) {
		return nil, fmt.Errorf("a change at %d-%d of a text of %d bytes", start, end, len(b.Text))
	}
	b.Text = b.Text[:start] + text + b.Text[end:]
	cur := start + len(text)
	if b.Text == b.Printed {
		b.Reason = ""
		return &Typed{Status: "same", Cursor: cur}, nil
	}
	// where the buffer differs from the view as printed: the same before
	// lo, the same after the last q bytes of each
	p, t := b.Printed, b.Text
	lo := 0
	for lo < len(p) && lo < len(t) && p[lo] == t[lo] {
		lo++
	}
	q := 0
	for q < len(p)-lo && q < len(t)-lo && p[len(p)-1-q] == t[len(t)-1-q] {
		q++
	}
	neu, err := readSx(t, b.Opt.IDs)
	if err != nil {
		return b.pending(cur, "not yet forms: %v", err), nil
	}
	if old, err := readSx(p, b.Opt.IDs); err == nil && eqSx(old, neu) {
		b.Reason = ""
		return &Typed{Status: "layout", Cursor: cur}, nil
	}
	opt := b.Opt
	opt.Index = b.Ix
	opt.within = &within{lo: lo, oldHi: len(p) - q, newHi: len(t) - q}
	r, err := b.S.Edit(b.Render, t, opt)
	if err != nil {
		var ref *Refusal
		if errors.As(err, &ref) {
			return b.pending(cur, "%s", ref.Reason), nil
		}
		return b.pending(cur, "%v", err), nil
	}
	b.Ix = r.Index
	cur = MapPos(t, r.Text, cur)
	b.Text, b.Printed, b.Spans, b.Reason = r.Text, r.Text, r.Spans, ""
	return &Typed{Status: "applied", Result: r, Cursor: cur}, nil
}

func (b *Buffer) pending(cur int, format string, a ...any) *Typed {
	b.Reason = Plain(fmt.Sprintf(format, a...))
	return &Typed{Status: "pending", Reason: b.Reason, Cursor: cur}
}

var (
	plainWhere  = regexp.MustCompile(`(frag: )?cc's check: |frag \d+ line \d+:\d+: | \(\w+\.go:\d+:\w+:\)|refused: `)
	plainStruct = regexp.MustCompile(`type ((struct|union) \w+) \{[^}]*\}`)
	plainRepeat = regexp.MustCompile(`; (.*)$`)
	plainParse  = regexp.MustCompile(`^frag: (.*?) the context, line .*$`)
)

// Plain is a reason an editor shows while its text is typed: what cc's
// check said without where in the fragment it said it (the fragment is the
// form being typed) or where in cc's source, a struct by its tag and not
// its members, and the first of several.
func Plain(reason string) string {
	r := plainParse.ReplaceAllString(reason, "$1") // a parse's first error
	r = plainWhere.ReplaceAllString(r, "")
	r = plainStruct.ReplaceAllString(r, "$1")
	if strings.HasPrefix(reason, "frag") || strings.Contains(reason, "cc's check") {
		r = plainRepeat.ReplaceAllString(r, "")
	}
	return strings.TrimSpace(r)
}

// MapPos is where byte pos of old is in new, the same text printed again:
// the same byte before where they differ, shifted by the difference
// after, and within what differs, the same count of non-blank bytes from
// its start (a print lays forms out again, it does not change their
// tokens), else its end.
func MapPos(old, new string, pos int) int {
	lo := 0
	for lo < len(old) && lo < len(new) && old[lo] == new[lo] {
		lo++
	}
	if pos <= lo {
		return pos
	}
	q := 0
	for q < len(old)-lo && q < len(new)-lo && old[len(old)-1-q] == new[len(new)-1-q] {
		q++
	}
	if pos >= len(old)-q {
		return pos + len(new) - len(old)
	}
	n := 0
	for i := lo; i < pos; i++ {
		if !isBlank(old[i]) {
			n++
		}
	}
	j := lo
	for ; j < len(new)-q && n > 0; j++ {
		if !isBlank(new[j]) {
			n--
		}
	}
	return j
}

func isBlank(c byte) bool { return c == ' ' || c == '\n' || c == '\t' }

// Undo undoes the session's last edit and prints the view again, what was
// pending dropped; false when there is nothing to undo.
func (b *Buffer) Undo() (bool, error) {
	if !b.S.Undo() {
		return false, nil
	}
	b.Ix.Update(b.S.undone.Saved())
	b.Ix.verify("an undo")
	return true, b.reprint()
}

// Rename renames the entity at byte pos of the buffer -- its every
// declaration and use, wherever they are (Editor.Rename) -- as one edit of
// the session, and prints the view again; what was pending is dropped.
// Typing a name anew renames nothing: a buffer sees one form change at a
// time, and a renaming changes every use at once.
func (b *Buffer) Rename(pos int, to string) (*graph.Renamed, *graph.Node, error) {
	if b.Reason != "" {
		b.Revert()
	}
	d, _ := EntityAt(b.Ix, b.Spans, pos)
	if d == nil {
		return nil, nil, fmt.Errorf("rename: no entity at byte %d", pos)
	}
	var rn *graph.Renamed
	if _, err := b.S.Do(func(e *graph.Editor) error {
		var err error
		rn, err = e.Rename(d, to)
		return err
	}); err != nil {
		return nil, d, err
	}
	b.Ix.Update(b.S.done[len(b.S.done)-1].Saved())
	b.Ix.verify("a rename")
	return rn, d, b.reprint()
}

// Revert drops what is pending: the text is the view as printed.
func (b *Buffer) Revert() { b.Text, b.Reason = b.Printed, "" }

// Reopen is the buffer on another view of the same graph: render's.
func (b *Buffer) Reopen(render Render) error {
	old := b.Render
	b.Render = render
	if err := b.reprint(); err != nil {
		b.Render = old
		return err
	}
	return nil
}

// within is a typed change: the printed text's bytes lo to oldHi are the
// edited text's lo to newHi, the rest the same.
type within struct{ lo, oldHi, newHi int }

// planWithin aligns from the least form around the change: the form read
// from the printed text that holds the changed bytes, paired with the form
// read from the edited text at the same place -- the same start, its end
// shifted by what the change added -- and climbing to the form around it
// while there is no such form in the edited text or the pair cannot take
// the change.  At the top it is plan's alignment of the whole texts.
func (a *aligner) planWithin(text string, spans []Span, edited string, w within) ([]Op, error) {
	old, err := readSx(text, a.ids)
	if err != nil {
		return nil, fmt.Errorf("the view as printed: %w", err)
	}
	neu, err := readSx(edited, a.ids)
	if err != nil {
		return nil, err
	}
	a.mapSpans(old, spans)
	delta := w.newHi - w.oldHi
	at := map[[2]int]*sx{}
	var index func(y *sx)
	index = func(y *sx) {
		if y.up != nil { // not the root, which is paired below
			at[[2]int{y.start, y.end}] = y
		}
		for _, k := range y.kids {
			index(k)
		}
	}
	index(neu)
	x := innermost(old, w.lo, w.oldHi)
	for ; x != nil && x.up != nil; x = x.up {
		y := at[[2]int{x.start, x.end + delta}]
		if y == nil || y.list != x.list {
			continue
		}
		var pairs [][2]*sx
		a.atoms(x, y, &pairs)
		ops := a.renames(pairs)
		more, esc, err := a.align(x, y)
		if err != nil {
			return nil, err
		}
		if !esc {
			return append(ops, more...), nil
		}
		a.renamed = map[*sx]string{}
	}
	return a.plan(text, spans, edited)
}

// innermost is the least form of x's holding bytes lo to hi; one that
// only touches them is paired, and found not to be the change's, above.
func innermost(x *sx, lo, hi int) *sx {
	for {
		var in *sx
		for _, k := range x.kids {
			if k.start <= lo && hi <= k.end {
				in = k
				break
			}
		}
		if in == nil {
			return x
		}
		x = in
	}
}

// the session's graph, for a server that writes it
func (b *Buffer) Graph() *graph.Graph { return b.S.G }
