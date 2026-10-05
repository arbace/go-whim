package view

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// THE SPAN TABLE (doc/GRAPH.md, *Editable views, the first gate*).  A
// printed view is text; its span table says, for every node the printer
// wrote, the bytes of the text that are that node's form -- `#ID` and
// `@ID` marks included when the ids are shown -- so that a position in the
// text names a node with or without the ids in sight, and an edited text
// can be aligned with the nodes it was printed from (edit.go).  The spans
// nest as the forms do: each records the innermost span around it.

// A SpanKind says what a span is the printing of.
type SpanKind uint8

const (
	// SpanForm is a node's form: a list, `(` to `)`, or an atom.
	SpanForm SpanKind = iota
	// SpanContext is a context shown around uses: its label's list,
	// `(:read FORM)`, or the form alone when it has no label.  Its Node
	// is the context's form.
	SpanContext
	// SpanEntry is a view's entry, `(in NAME contexts... entries...)`:
	// the view's structure, not a form of the graph.  Its Node is the node
	// the entry names (nil for none).
	SpanEntry
	// SpanName is the name an entry gives its node, `NAME`, `@NAME`,
	// `#ID NAME`.
	SpanName
)

func (k SpanKind) String() string {
	return [...]string{"form", "context", "entry", "name"}[k]
}

// A Span is where one printed thing is in a view's text: bytes Start to
// End.
type Span struct {
	Node       *graph.Node
	Kind       SpanKind
	Start, End int
	Parent     int  // the innermost span around it, -1 for none
	Whole      bool // nothing below it was elided (`...`)
}

// A spanner records spans while the printer writes.  Its methods are
// no-ops on nil: the printer without a table.
type spanner struct {
	spans []Span
	open_ []int // the spans being written
}

func (sp *spanner) open(b *strings.Builder, n *graph.Node, k SpanKind, whole bool) int {
	if sp == nil {
		return -1
	}
	parent := -1
	if len(sp.open_) > 0 {
		parent = sp.open_[len(sp.open_)-1]
	}
	sp.spans = append(sp.spans, Span{Node: n, Kind: k, Start: b.Len(), Parent: parent, Whole: whole})
	i := len(sp.spans) - 1
	sp.open_ = append(sp.open_, i)
	return i
}

func (sp *spanner) close(b *strings.Builder, i int) {
	if sp == nil || i < 0 {
		return
	}
	sp.spans[i].End = b.Len()
	sp.open_ = sp.open_[:len(sp.open_)-1]
}

// At is the innermost span of kind k holding byte pos of the text, or -1.
func At(spans []Span, pos int, k SpanKind) int {
	best := -1
	for i, s := range spans {
		if s.Kind == k && s.Start <= pos && pos < s.End && (best < 0 || s.Start >= spans[best].Start && s.End <= spans[best].End) {
			best = i
		}
	}
	return best
}
