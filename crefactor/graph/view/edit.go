package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
)

// EDITABLE VIEWS (doc/GRAPH.md, *Editable views, the first gate*).  A view
// is printed with its span table (spans.go); the text is edited -- with the
// ids shown or not: the spans, not the marks, say which node a form is --
// and Edit makes the edit a graph edit:
//
//   - THE TWO TEXTS ARE READ as forms with their places, the printed one
//     and the edited one, and aligned form by form: the elements of two
//     lists by their longest common subsequence of equal forms, a run of
//     the same length on both sides pair by pair, so that an unchanged form
//     keeps its node and its id.
//   - A RENAMING is found first: atoms changed from one name to one other
//     name, which are exactly a declaration's name and every use of its
//     entity the graph holds, are the editor's Rename (ids kept).
//   - THE REST IS MADE EDITS at the least node that can take it: a changed
//     form is replaced (FRAG: its C, printed from the edited Lisp, made
//     nodes in context, its names resolved as the file resolves them) where
//     it is an item, a body or an expression -- a changed token climbs to
//     the form that is one; a run of items replaced, inserted or deleted
//     where the lengths differ; a top-level form deleted with every
//     declaration of its entity (a function's prototypes with it).
//   - REFUSED: a change to what the view elides (`...`) or to the view's
//     own structure (an entry, its name, a label), a name the context
//     does not declare and a C that cc's check refuses (FRAG's), a
//     deletion that leaves a use dangling unless the fall-out closure is
//     asked for, and an edit after which the view printed again does not
//     say what was written (the alignment's own check).  The graph handed
//     in is never touched: the edit is made on a copy, returned when it
//     stands.

// A Render prints a view with its span table from an index of the graph,
// the same view each time: Edit prints it before the edit, and again after
// it on a fresh index.
type Render func(ix *Index) (text string, spans []Span, err error)

// EditOptions are how an edit is made.
type EditOptions struct {
	// IDs: the view was printed with the ids, and the edited text may keep
	// its marks (`#ID`, `@ID`), which are read past.
	IDs bool
	// FallOut, when set, closes over the uses a deletion leaves dangling
	// (graph.Editor.FallOut); nil refuses such a deletion.
	FallOut *graph.FallOutOptions
	// Printed, when set, is the text the edit was made on: refused unless
	// the view prints the same now.
	Printed *string
	// InPlace edits the graph handed in, under a journal (graph.Begin),
	// instead of a copy: a refusal undoes what was made, and the Result's
	// Journal undoes the edit made.  It spares the copy, the graph's Lisp
	// printed and read (130 ms on whim-vim.c's).
	InPlace bool
	// Index, in place, is a current index of the graph: the edit begins on
	// it rather than indexing the graph again (33 ms on whim-vim.c's).
	Index *Index
	// Editor, in place, is a current editor of the graph, kept by a
	// session from one edit to the next (18 ms on whim-vim.c's to make).
	Editor *graph.Editor
	// stale, set when an edit in place was undone after it changed the
	// graph: the Editor handed in is not the graph's any more.
	stale *bool
	// within, a typed change's: the alignment begins at that form of the
	// printed text (buffer.go).
	within *within
}

// An Op is one edit an aligned text makes.
type Op struct {
	Kind  string      // rename, delete, replace, insert
	Node  *graph.Node // rename: a declaration; delete, replace: the node, a run's first
	Last  *graph.Node // replace: a run's last item, or nil
	Where string      // insert: before, after (Node), or end (of Node)
	To    string      // rename: the new name
	From  string      // rename: the old
	Src   string      // replace, insert: the C; member: the member's C-lisp, (NAME TYPE)
	As    string      // replace, insert: what the C is: items, top, expr, body
	Made  []*graph.Node // what the op made, once applied
}

func (o Op) String() string {
	switch o.Kind {
	case "rename":
		return fmt.Sprintf("rename #%d %s to %s", o.Node.ID, o.From, o.To)
	case "member":
		return fmt.Sprintf("member %s #%d: %s", o.Where, o.Node.ID, oneLine(o.Src))
	case "delete":
		return fmt.Sprintf("delete #%d (%s)", o.Node.ID, o.Node.Head())
	case "insert":
		return fmt.Sprintf("insert %s #%d: %s", o.Where, o.Node.ID, oneLine(o.Src))
	}
	what := fmt.Sprintf("#%d", o.Node.ID)
	if o.Node.ID == 0 {
		what = o.Node.Atom
	}
	if o.Last != nil {
		what += fmt.Sprintf("..#%d", o.Last.ID)
	}
	return fmt.Sprintf("replace %s with %s", what, oneLine(o.Src))
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// A Refusal is why an edit was not made.
type Refusal struct{ Reason string }

func (r *Refusal) Error() string { return "refused: " + r.Reason }

func refuse(format string, a ...any) error { return &Refusal{Reason: fmt.Sprintf(format, a...)} }

// A Result is an edit made.
type Result struct {
	Graph   *graph.Graph // the edited copy
	Editor  *graph.Editor
	Ops     []Op
	Closure []graph.Act // what the fall-out closure did
	FallOut graph.FallOutStats
	Recheck graph.RecheckStats
	Text    string // the view printed again
	Spans   []Span // its span table
	// Journal, for an edit made in place, undoes it (Journal.Undo; Session
	// keeps them in order).
	Journal *graph.Journal
	// Index is the graph's after the edit.
	Index *Index
	// Added are the top-level forms the edit put beside the view's own,
	// which it does not show.
	Added []*graph.Node
}

// tamper, when a test sets it, changes the alignment's ops before they are
// applied: the control that a wrong alignment is caught.
var tamper func(*graph.Editor, []Op)

// Edit aligns edited with the view render prints of g and makes the edit
// on a copy of g, which it returns; g is not touched -- or, InPlace, on g
// itself, which a refusal leaves as it was.
func Edit(g *graph.Graph, render Render, edited string, opt EditOptions) (*Result, error) {
	if !opt.InPlace {
		w, err := graph.Read(g.Lisp())
		if err != nil {
			return nil, err
		}
		return edit(w, render, edited, opt)
	}
	j := g.Begin()
	r, err := edit(g, render, edited, opt)
	j.End()
	if err != nil {
		changed := j.Changed() > 0
		j.Undo()
		if changed {
			if opt.Index != nil {
				opt.Index.Update(j.Saved())
				opt.Index.verify("a refusal undone")
			}
			if opt.stale != nil {
				*opt.stale = true
			}
		}
		return nil, err
	}
	r.Journal = j
	return r, nil
}

func edit(w *graph.Graph, render Render, edited string, opt EditOptions) (*Result, error) {
	ix := opt.Index
	if ix == nil || !opt.InPlace || ix.G != w {
		ix = NewIndex(w)
	}
	text, spans, err := render(ix)
	if err != nil {
		return nil, err
	}
	if opt.Printed != nil && *opt.Printed != text {
		return nil, refuse("the view prints otherwise than the text the edit was made on")
	}
	e := opt.Editor
	if e == nil || !opt.InPlace || e.Graph() != w {
		e = graph.NewEditor(w)
	}
	a := &aligner{e: e, ix: ix, ids: opt.IDs, info: map[*sx]sinfo{}, renamed: map[*sx]string{}}
	var ops []Op
	if opt.within != nil {
		ops, err = a.planWithin(text, spans, edited, *opt.within)
	} else {
		ops, err = a.plan(text, spans, edited)
	}
	if err != nil {
		return nil, err
	}
	if tamper != nil {
		tamper(e, ops)
	}
	r := &Result{Graph: w, Editor: e, Ops: ops}
	made, err := applyMade(e, a.ix, ops)
	if err != nil {
		return nil, err
	}
	if err := agrees(e, made); err != nil {
		return nil, err
	}
	if len(a.beside) > 0 {
		top := map[*graph.Node]bool{}
		for _, f := range w.Forms {
			top[f] = true
		}
		for _, m := range made {
			for _, n := range m {
				if top[n] {
					r.Added = append(r.Added, n)
				}
			}
		}
	}
	// the alignment's check: the view printed again says what was written
	reindex := func() *Index {
		if j := w.Recording(); j != nil && ix == opt.Index {
			ix.Update(j.Saved()) // the forms the edit changed, walked again
			ix.verify("an edit")
			return ix
		}
		return NewIndex(w)
	}
	r.Index = reindex()
	again, againSpans, rerr := render(r.Index)
	if rerr != nil {
		again = "" // the view's root went: an empty view
	}
	if err := sameText(again, without(edited, a.beside), opt.IDs, a.entryHeads); err != nil {
		// the view may no longer show what the edit wrote -- a use made
		// another entity's leaves a uses or member view -- so the text as
		// printed before, each op's node replaced in place by what it made,
		// printed, says what was written as well (and a node misplaced still
		// puts it where it was not)
		if len(a.renamedOps(ops)) > 0 || sameText(inPlace(text, spans, ops, made, Printer{IDs: opt.IDs}), without(edited, a.beside), opt.IDs, a.entryHeads) != nil {
			return nil, refuse("the edit was made, and the view printed again does not say what was written (%v): the alignment is wrong", err)
		}
	}
	r.Text, r.Spans = again, againSpans
	if d := e.Dangling(); len(d) > 0 {
		if opt.FallOut == nil {
			u := d[0]
			in := ""
			if f := e.Function(u.Use); f != nil {
				in = " in " + graph.DeclName(f)
			}
			return nil, refuse("#%d (%s)%s refers to #%d (%s), which the edit deleted (%s so): --fallout closes over them",
				u.Use.ID, label(u.Use), in, u.Target.ID, label(u.Target), plural(len(d), "use"))
		}
		k := len(e.Log)
		st, err := e.FallOut(*opt.FallOut)
		if err != nil {
			return nil, refuse("%v", err)
		}
		r.FallOut, r.Closure = st, append([]graph.Act(nil), e.Log[k:]...)
	}
	r.Recheck = e.Recheck()
	if err := callersAgree(e, made); err != nil {
		return nil, err
	}
	if len(r.Closure) > 0 || r.Recheck != (graph.RecheckStats{}) {
		r.Index = reindex() // the closure's and the re-check's changes indexed too
		if t, sp, err := render(r.Index); err == nil {
			r.Text, r.Spans = t, sp
		}
	}
	if r.Recheck.Left > 0 {
		return nil, refuse("%d expressions the edit wrote have no type the re-check derives", r.Recheck.Left)
	}
	if err := check(e, w, made); err != nil {
		return nil, refuse("the graph's invariants: %v", err)
	}
	return r, nil
}

func label(n *graph.Node) string {
	if !n.IsList() {
		return n.Atom
	}
	if name := graph.DeclName(n); name != "" {
		return name
	}
	return n.Head()
}

// applyMade makes the ops -- the renamings, the deletions, then every FRAG
// in one unit -- and is the nodes the fragments made.
func applyMade(e *graph.Editor, ix *Index, ops []Op) ([][]*graph.Node, error) {
	for _, o := range ops {
		if o.Kind == "rename" {
			if _, err := e.Rename(o.Node, o.To); err != nil {
				return nil, refuse("%v", err)
			}
		}
	}
	for _, o := range ops {
		if o.Kind != "delete" {
			continue
		}
		ds := []*graph.Node{o.Node}
		if ix.IsTop(o.Node) {
			ds = ix.Decls(o.Node) // an entity: a function's prototypes with it
		}
		for _, d := range ds {
			if !e.Live(d) {
				continue
			}
			if err := e.Delete(d); err != nil {
				return nil, refuse("%v", err)
			}
		}
	}
	for i, o := range ops {
		if o.Kind != "member" {
			continue
		}
		m, err := e.InsertMember(o.Node, o.Where == "after", o.Src)
		if err != nil {
			return nil, refuse("%v", err)
		}
		ops[i].Made = []*graph.Node{m}
	}
	var fs []graph.Frag
	var spliced []int // the ops the frags are, in order
	for i, o := range ops {
		var at graph.Spot
		switch o.Kind {
		case "replace":
			if o.Last != nil {
				at = e.SpotRun(o.Node, o.Last)
			} else {
				at = e.SpotOf(o.Node)
			}
		case "insert":
			switch o.Where {
			case "before":
				at = e.SpotBefore(o.Node)
			case "after":
				at = e.SpotAfter(o.Node)
			default:
				at = e.SpotEnd(o.Node)
			}
		default:
			continue
		}
		if err := at.Err(); err != nil {
			return nil, refuse("%v", err)
		}
		fs = append(fs, graph.Frag{At: at, Src: o.Src})
		spliced = append(spliced, i)
	}
	var made [][]*graph.Node
	if len(fs) > 0 {
		var err error
		if made, err = e.SpliceC(fs...); err != nil {
			return nil, refuse("%v", err)
		}
		for k, i := range spliced {
			if k < len(made) {
				ops[i].Made = made[k]
			}
		}
	}
	for _, o := range ops {
		if o.Kind == "member" {
			made = append(made, o.Made)
		}
	}
	return made, nil
}

// AN SX is a form read from a view's text, where it is, its marks read
// past.
type sx struct {
	atom       string
	list       bool
	kids       []*sx
	start, end int
	up         *sx
}

func (x *sx) String() string {
	if !x.list {
		return x.atom
	}
	var b strings.Builder
	b.WriteByte('(')
	for i, k := range x.kids {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(k.String())
	}
	b.WriteByte(')')
	return b.String()
}

// readSx reads text's forms, under a root list of them.  With ids, a
// list's `#ID` before it and `@ID`s after it, an atom's `#ID:` and `@ID`s,
// and an entry's `#ID` or `@ID` before its name, are marks and not forms.
func readSx(text string, ids bool) (*sx, error) {
	r := &sxReader{s: text, ids: ids}
	root := &sx{list: true, start: 0, end: len(text)}
	for {
		r.space()
		if r.i >= len(r.s) {
			return root, nil
		}
		x, err := r.node()
		if err != nil {
			return nil, err
		}
		if x != nil {
			x.up = root
			root.kids = append(root.kids, x)
		}
	}
}

type sxReader struct {
	s   string
	i   int
	ids bool
}

func (r *sxReader) errf(format string, a ...any) error {
	line := 1 + strings.Count(r.s[:min(r.i, len(r.s))], "\n")
	return refuse("the edited text, line %d: %s", line, fmt.Sprintf(format, a...))
}

func (r *sxReader) space() {
	for r.i < len(r.s) {
		switch r.s[r.i] {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			r.i++
		case ';':
			for r.i < len(r.s) && r.s[r.i] != '\n' {
				r.i++
			}
		default:
			return
		}
	}
}

// node is the form at r.i; nil for a mark standing alone.
func (r *sxReader) node() (*sx, error) {
	start := r.i
	switch r.s[r.i] {
	case ')':
		return nil, r.errf("unexpected )")
	case '(':
		return r.list(start)
	}
	a, err := r.atom()
	if err != nil {
		return nil, err
	}
	if r.ids && isMark(a, '#') {
		if r.i < len(r.s) && r.s[r.i] == '(' {
			return r.list(start) // `#ID(...)`
		}
		return nil, nil // an entry's `#ID NAME`
	}
	if r.ids && isMark(a, '@') {
		return nil, nil // a link's `@ID NAME`
	}
	if r.ids {
		a = stripAtom(a)
	}
	return &sx{atom: a, start: start, end: r.i}, nil
}

func (r *sxReader) list(start int) (*sx, error) {
	r.i++ // (
	x := &sx{list: true, start: start}
	for {
		r.space()
		if r.i >= len(r.s) {
			return nil, r.errf("unclosed list")
		}
		if r.s[r.i] == ')' {
			r.i++
			for r.ids && r.i+1 < len(r.s) && r.s[r.i] == '@' && isDigit(r.s[r.i+1]) {
				r.i++
				for r.i < len(r.s) && isDigit(r.s[r.i]) {
					r.i++
				}
			}
			x.end = r.i
			return x, nil
		}
		k, err := r.node()
		if err != nil {
			return nil, err
		}
		if k != nil {
			k.up = x
			x.kids = append(x.kids, k)
		}
	}
}

// atom reads up to a space or a parenthesis, a quoted literal whole
// (crefactor/clisp's reader's rule).
func (r *sxReader) atom() (string, error) {
	start := r.i
	for r.i < len(r.s) {
		c := r.s[r.i]
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v', '(', ')', ';':
			return r.s[start:r.i], nil
		case '"', '\'':
			if c == '\'' && r.i > start && (isDigit(r.s[start]) || r.s[start] == '.') {
				r.i++
				continue
			}
			r.i++
			for r.i < len(r.s) && r.s[r.i] != c {
				if r.s[r.i] == '\n' {
					return "", r.errf("a newline in a literal")
				}
				if r.s[r.i] == '\\' {
					r.i++
				}
				r.i++
			}
			if r.i >= len(r.s) {
				return "", r.errf("unclosed literal")
			}
			r.i++
		default:
			r.i++
		}
	}
	return r.s[start:r.i], nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isMark says a is `#ID` or `@ID`, c its first byte.
func isMark(a string, c byte) bool {
	if len(a) < 2 || a[0] != c {
		return false
	}
	for i := 1; i < len(a); i++ {
		if !isDigit(a[i]) {
			return false
		}
	}
	return true
}

// stripAtom takes an atom's marks off: `#ID:text@ID@ID` is text.
func stripAtom(a string) string {
	if len(a) > 1 && a[0] == '#' && isDigit(a[1]) {
		if i := strings.IndexByte(a, ':'); i > 0 && isMark(a[:i], '#') {
			a = a[i+1:]
		}
	}
	for {
		i := strings.LastIndexByte(a, '@')
		if i <= 0 || !isMark(a[i:], '@') || a[0] == '"' && !strings.HasSuffix(a[:i], "\"") {
			return a
		}
		a = a[:i]
	}
}

func eqSx(a, b *sx) bool {
	if a.list != b.list || a.atom != b.atom || len(a.kids) != len(b.kids) {
		return false
	}
	for i := range a.kids {
		if !eqSx(a.kids[i], b.kids[i]) {
			return false
		}
	}
	return true
}

// sinfo is what the span table says an old form is.
type sinfo struct {
	form  *graph.Node // the node whose form it is
	whole bool        // the form, nothing elided below it
	ctx   *graph.Node // a context around uses: its form
	entry *graph.Node // a view's entry
	isCtx bool
	isEnt bool
}

// THE ALIGNER.
type aligner struct {
	e          *graph.Editor
	ix         *Index
	ids        bool
	info       map[*sx]sinfo
	renamed    map[*sx]string // old atoms a renaming respells, to their new text
	entryHeads map[string]bool
	beside     [][2]int // the edited text's top-level forms made beside the view's, which it does not show
}

// plan reads the two texts, maps the printed one's forms to their spans,
// and aligns them: the renamings, then the edits.
func (a *aligner) plan(text string, spans []Span, edited string) ([]Op, error) {
	old, err := readSx(text, a.ids)
	if err != nil {
		return nil, fmt.Errorf("the view as printed: %w", err)
	}
	neu, err := readSx(edited, a.ids)
	if err != nil {
		return nil, err
	}
	a.mapSpans(old, spans)
	var pairs [][2]*sx
	a.atoms(old, neu, &pairs)
	ops := a.renames(pairs)
	more, esc, err := a.align(old, neu)
	if err != nil {
		return nil, err
	}
	if esc {
		return nil, refuse("the change is to the view's own structure, not to a form of the graph")
	}
	return append(ops, more...), nil
}

// mapSpans gives every form read from the printed text what the span table
// says it is.
func (a *aligner) mapSpans(old *sx, spans []Span) {
	type key struct{ s, e int }
	at := map[key][]int{}
	for i, s := range spans {
		at[key{s.Start, s.End}] = append(at[key{s.Start, s.End}], i)
	}
	a.entryHeads = map[string]bool{}
	var mapAll func(x *sx)
	mapAll = func(x *sx) {
		var in sinfo
		for _, i := range at[key{x.start, x.end}] {
			s := spans[i]
			switch s.Kind {
			case SpanForm:
				in.form, in.whole = s.Node, s.Whole
			case SpanContext:
				in.ctx, in.isCtx = s.Node, true
			case SpanEntry:
				in.entry, in.isEnt = s.Node, true
				if x.list && len(x.kids) > 0 && x.up != nil && x.up.up != nil {
					a.entryHeads[x.kids[0].atom] = true
				}
			}
		}
		a.info[x] = in
		for _, k := range x.kids {
			mapAll(k)
		}
	}
	mapAll(old)
}

// gap is a run of elements one list has and the other has not, between
// two that both have.
type gap struct{ o0, o1, n0, n1 int }

// gaps aligns two lists' elements by their longest common subsequence of
// equal forms (eq), and gives the runs between.
func gaps(o, n []*sx, eq func(a, b *sx) bool) []gap {
	l := make([][]int, len(o)+1)
	for i := range l {
		l[i] = make([]int, len(n)+1)
	}
	for i := len(o) - 1; i >= 0; i-- {
		for j := len(n) - 1; j >= 0; j-- {
			if eq(o[i], n[j]) {
				l[i][j] = l[i+1][j+1] + 1
			} else {
				l[i][j] = max(l[i+1][j], l[i][j+1])
			}
		}
	}
	var out []gap
	i, j, gi, gj := 0, 0, 0, 0
	flush := func() {
		if gi < i || gj < j {
			out = append(out, gap{gi, i, gj, j})
		}
	}
	for i < len(o) && j < len(n) {
		switch {
		case eq(o[i], n[j]):
			flush()
			i++
			j++
			gi, gj = i, j
		case l[i+1][j] >= l[i][j+1]:
			i++
		default:
			j++
		}
	}
	i, j = len(o), len(n)
	flush()
	return out
}

// atoms collects the atoms changed one for one: the candidates for a
// renaming.
func (a *aligner) atoms(o, n *sx, out *[][2]*sx) {
	if eqSx(o, n) {
		return
	}
	if !o.list || !n.list {
		if !o.list && !n.list {
			*out = append(*out, [2]*sx{o, n})
		}
		return
	}
	for _, g := range gaps(o.kids, n.kids, eqSx) {
		if g.o1-g.o0 == g.n1-g.n0 {
			for k := 0; k < g.o1-g.o0; k++ {
				a.atoms(o.kids[g.o0+k], n.kids[g.n0+k], out)
			}
		}
	}
}

// renames finds the renamings among the atoms changed: a declaration's
// name and every use of its entity, each changed from one name to one
// other, and nothing of it left unchanged.
func (a *aligner) renames(pairs [][2]*sx) []Op {
	type group struct {
		decl  *graph.Node
		to    string
		atoms []*sx
		bad   bool
	}
	groups := map[*graph.Node]*group{}
	var order []*graph.Node
	for _, p := range pairs {
		n := a.info[p[0]].form
		if n == nil || p[0].up == nil {
			continue
		}
		var d *graph.Node
		if r := n.Ref(); r != nil {
			d = r
		} else if up := a.info[p[0].up].form; up != nil && graph.DeclAtom(up) == n && a.declares(up) {
			d = up
		}
		if d == nil {
			continue
		}
		ent := a.ix.Rep(d)
		g := groups[ent]
		if g == nil {
			g = &group{decl: d, to: p[1].atom}
			groups[ent] = g
			order = append(order, ent)
		}
		g.bad = g.bad || g.to != p[1].atom
		g.atoms = append(g.atoms, p[0])
	}
	var ops []Op
	for _, ent := range order {
		g := groups[ent]
		if g.bad {
			continue
		}
		changed := map[*graph.Node]bool{}
		for _, x := range g.atoms {
			changed[a.info[x].form] = true
		}
		all := true
		for _, d := range a.ix.Decls(ent) {
			if at := graph.DeclAtom(d); at == nil || !changed[at] {
				all = false
			}
			for _, u := range a.e.Uses(d) {
				all = all && changed[u]
			}
		}
		if !all {
			continue
		}
		for _, x := range g.atoms {
			a.renamed[x] = g.to
		}
		ops = append(ops, Op{Kind: "rename", Node: g.decl, To: g.to, From: g.atoms[0].atom})
	}
	return ops
}

// declares says d is a declaration a renaming can take: a def, a
// function, a typedef, a label, or a parameter, member or enumerator --
// whose name is its first atom, as an operator's is -- that something
// refers to.
func (a *aligner) declares(d *graph.Node) bool {
	switch d.Head() {
	case "def", "defn", "typedef", "label":
		return true
	}
	return len(a.e.Uses(d)) > 0
}

// same is eqSx with the renamings made: an old atom a renaming respells
// is the same as its new text.
func (a *aligner) same(o, n *sx) bool {
	if !o.list && !n.list {
		if to, ok := a.renamed[o]; ok {
			return to == n.atom
		}
		return o.atom == n.atom
	}
	if o.list != n.list || len(o.kids) != len(n.kids) {
		return false
	}
	for i := range o.kids {
		if !a.same(o.kids[i], n.kids[i]) {
			return false
		}
	}
	return true
}

// align is the edits that make o say n; esc says o cannot take the change
// itself, and the list holding it must.
func (a *aligner) align(o, n *sx) ([]Op, bool, error) {
	if a.same(o, n) {
		return nil, false, nil
	}
	if !o.list || !n.list {
		return a.replace(o, n)
	}
	var ops []Op
	for _, g := range gaps(o.kids, n.kids, a.same) {
		var sub []Op
		var esc bool
		var err error
		if g.o1-g.o0 == g.n1-g.n0 {
			for k := 0; k < g.o1-g.o0 && !esc && err == nil; k++ {
				var s []Op
				s, esc, err = a.align(o.kids[g.o0+k], n.kids[g.n0+k])
				sub = append(sub, s...)
			}
		} else {
			sub, esc, err = a.run(o, n, g)
		}
		if err != nil {
			return nil, false, err
		}
		if esc {
			return a.replace(o, n)
		}
		ops = append(ops, sub...)
	}
	return ops, false, nil
}

// replace is o's node replaced by n's C, where o is a form that can take
// it; else the change climbs.
func (a *aligner) replace(o, n *sx) ([]Op, bool, error) {
	in := a.info[o]
	switch {
	case in.form != nil:
		x := in.form
		as := a.unit(x)
		if as == "" {
			return nil, true, nil
		}
		if !in.whole {
			return nil, false, refuse("the change to #%d (%s) reaches into what the view elides (`%s`)", x.ID, label(x), Elided)
		}
		src, err := a.cText(as, []*sx{n})
		if err != nil {
			return nil, false, err
		}
		return []Op{{Kind: "replace", Node: x, Src: src, As: as}}, false, nil
	case in.isCtx:
		return nil, false, refuse("a context's label (`%s`) is the view's, not the graph's: edit the form inside it", o.String())
	case in.isEnt:
		return nil, false, refuse("an entry of the view (`(%s ...)`) is not a form of the graph: edit the forms inside it", headOf(o))
	case o.up == nil:
		return nil, true, nil
	case a.info[o.up].isEnt:
		return nil, false, refuse("`%s` is an entry's name or head, the view's, not a form of the graph", o.String())
	case o.atom == Elided:
		return nil, false, refuse("`%s` stands for what the view elides: it is not edited", Elided)
	}
	return nil, true, nil
}

func headOf(x *sx) string {
	if x.list && len(x.kids) > 0 {
		return x.kids[0].String()
	}
	return x.String()
}

// unit is what x's place takes as a fragment: "top" (a top-level form),
// "items" (an item of a block or a body), "body" (a block in a statement's
// body), "expr" (an expression), or "" for a place that is not one --
// a token, a type, a declarator.
func (a *aligner) unit(x *graph.Node) string {
	sp := a.e.SpotOf(x)
	switch {
	case sp.Err() != nil:
		return ""
	case sp.Items() && a.ix.IsTop(x):
		return "top"
	case sp.Items():
		return "items"
	case x.Is("block") || graph.IsStatement(x):
		return "body"
	case x.Type != nil:
		return "expr"
	}
	return ""
}

// run is a run of o's elements replaced by a run of n's of another length:
// items replaced, inserted or deleted in a form that holds items; deleted
// from an entry or a context of the view.
func (a *aligner) run(o, n *sx, g gap) ([]Op, bool, error) {
	olds, news := o.kids[g.o0:g.o1], n.kids[g.n0:g.n1]
	in := a.info[o]
	if in.form == nil {
		// the view's own structure: only deletions, of whole forms
		if len(news) > 0 {
			if o.up == nil && len(olds) == 0 {
				// beside a top-level form the view shows (a def view's):
				// top-level forms of their own, after it or before it
				if ops, ok, err := a.besideTop(o, news, g); ok || err != nil {
					return ops, false, err
				}
				return nil, false, refuse("a form added beside the view's own: insert it inside a form")
			}
			return nil, false, refuse("`%s` added in the view's own structure (an entry, a label): a form goes inside a form", oneLine(news[0].String()))
		}
		var ops []Op
		for _, x := range olds {
			ds, err := a.deletes(x)
			if err != nil {
				return nil, false, err
			}
			for _, d := range ds {
				ops = append(ops, Op{Kind: "delete", Node: d})
			}
		}
		return ops, false, nil
	}
	p := in.form
	if (p.Is("struct") || p.Is("union")) && graph.IsTypeDef(p) {
		return a.members(o, p, olds, news, g)
	}
	var nodes []*graph.Node
	for _, x := range olds {
		f := a.info[x].form
		if f == nil {
			if x.atom == Elided {
				return nil, false, refuse("the change in #%d (%s) reaches into what the view elides (`%s`)", p.ID, label(p), Elided)
			}
			return nil, true, nil
		}
		nodes = append(nodes, f)
	}
	for i := 1; i < len(nodes); i++ {
		if a.e.Sibling(nodes[i-1], 1) != nodes[i] {
			return nil, false, refuse("the change in #%d (%s) reaches into what the view elides (`%s`)", p.ID, label(p), Elided)
		}
	}
	as := "items"
	if len(nodes) > 0 {
		if sp := a.e.SpotRun(nodes[0], nodes[len(nodes)-1]); sp.Err() != nil || !sp.Items() {
			return nil, true, nil
		}
		if len(news) == 0 {
			var ops []Op
			for _, d := range nodes {
				ops = append(ops, Op{Kind: "delete", Node: d})
			}
			return ops, false, nil
		}
		src, err := a.cText(as, news)
		if err != nil {
			return nil, false, err
		}
		op := Op{Kind: "replace", Node: nodes[0], Src: src, As: as}
		if len(nodes) > 1 {
			op.Last = nodes[len(nodes)-1]
		} else {
			op.Last = nodes[0]
		}
		return []Op{op}, false, nil
	}
	// an insertion: before the next form, after the one before, or at the
	// end of the form -- found before the C is printed, so that a list
	// holding no items (a function's parameters) climbs to the form around
	// it, which takes the change as a replacement
	var op *Op
	if g.o1 < len(o.kids) {
		if next := a.info[o.kids[g.o1]].form; next != nil && a.e.SpotBefore(next).Err() == nil {
			op = &Op{Kind: "insert", Node: next, Where: "before", As: as}
		}
	}
	if op == nil && g.o0 > 0 {
		if prev := a.info[o.kids[g.o0-1]].form; prev != nil && a.e.SpotAfter(prev).Err() == nil {
			op = &Op{Kind: "insert", Node: prev, Where: "after", As: as}
		}
	}
	if op == nil && g.o1 == len(o.kids) && a.e.SpotEnd(p).Err() == nil && in.whole {
		op = &Op{Kind: "insert", Node: p, Where: "end", As: as}
	}
	if op != nil {
		src, err := a.cText(as, news)
		if err != nil {
			return nil, false, err
		}
		op.Src = src
		return []Op{*op}, false, nil
	}
	if !in.whole {
		return nil, false, refuse("the insertion in #%d (%s) is beside what the view elides (`%s`)", p.ID, label(p), Elided)
	}
	return nil, true, nil
}

// deletes are the nodes deleting x deletes: a form, a context's, an
// entry's contexts'.
func (a *aligner) deletes(x *sx) ([]*graph.Node, error) {
	in := a.info[x]
	switch {
	case in.form != nil:
		return []*graph.Node{in.form}, nil
	case in.isCtx:
		return []*graph.Node{in.ctx}, nil
	case in.isEnt:
		var out []*graph.Node
		for _, k := range x.kids {
			kin := a.info[k]
			switch {
			case kin.isEnt:
				return nil, refuse("the entry `(%s ...)` holds entries of its own: delete their forms, not the entry", headOf(x))
			case kin.isCtx:
				out = append(out, kin.ctx)
			}
		}
		return out, nil
	}
	return nil, refuse("`%s` is the view's, not a form of the graph: it is not deleted", oneLine(x.String()))
}

// cText is the edited forms as C, of the kind the place takes.
func (a *aligner) cText(as string, xs []*sx) (string, error) {
	var forms []*clisp.Node
	for _, x := range xs {
		forms = append(forms, toClisp(x))
	}
	var s string
	var err error
	switch as {
	case "expr":
		if len(forms) != 1 {
			return "", refuse("an expression's place takes one form")
		}
		s, err = clisp.PrintExpr(forms[0])
	case "top":
		var b []byte
		b, err = clisp.Print(forms)
		s = string(b)
	default:
		s, err = clisp.PrintItems(forms)
	}
	if err != nil {
		return "", refuse("the edited form does not print as C: %v", err)
	}
	return s, nil
}

func toClisp(x *sx) *clisp.Node {
	if !x.list {
		return clisp.A(x.atom)
	}
	kids := make([]*clisp.Node, len(x.kids))
	for i, k := range x.kids {
		kids[i] = toClisp(k)
	}
	return clisp.L(kids...)
}

// sameText says two views' texts are the same forms, layout, comments and
// marks aside, an entry left with nothing in it absent.
func sameText(a, b string, ids bool, heads map[string]bool) error {
	x, err := readSx(a, ids)
	if err != nil {
		return err
	}
	y, err := readSx(b, ids)
	if err != nil {
		return err
	}
	x, y = dropEmpty(x, heads), dropEmpty(y, heads)
	if eqSx(x, y) {
		return nil
	}
	// the first form that differs, for the message
	for i := 0; i < min(len(x.kids), len(y.kids)); i++ {
		if !eqSx(x.kids[i], y.kids[i]) {
			return fmt.Errorf("`%s` against `%s`", clip(x.kids[i].String()), clip(y.kids[i].String()))
		}
	}
	return fmt.Errorf("%d forms against %d", len(x.kids), len(y.kids))
}

func clip(s string) string {
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

func dropEmpty(x *sx, heads map[string]bool) *sx {
	if !x.list {
		return x
	}
	y := &sx{list: true, start: x.start, end: x.end}
	for _, k := range x.kids {
		k = dropEmpty(k, heads)
		if k.list && len(k.kids) > 0 && len(k.kids) <= 2 && !k.kids[0].list && heads[k.kids[0].atom] {
			continue
		}
		y.kids = append(y.kids, k)
	}
	return y
}

// besideTop is forms added at the view's top level beside a top-level form
// it shows, made top-level forms after it (or before the next); ok false
// when no neighbour is one.
func (a *aligner) besideTop(o *sx, news []*sx, g gap) ([]Op, bool, error) {
	top := func(k int) *graph.Node {
		if k < 0 || k >= len(o.kids) {
			return nil
		}
		if f := a.info[o.kids[k]].form; f != nil && a.ix.IsTop(f) && a.info[o.kids[k]].whole {
			return f
		}
		return nil
	}
	where, at := "after", top(g.o0-1)
	if at == nil {
		where, at = "before", top(g.o1)
	}
	if at == nil {
		return nil, false, nil
	}
	src, err := a.cText("top", news)
	if err != nil {
		return nil, true, err
	}
	for _, x := range news {
		a.beside = append(a.beside, [2]int{x.start, x.end})
	}
	return []Op{{Kind: "insert", Node: at, Where: where, Src: src, As: "top"}}, true, nil
}

// without is text with the byte ranges cut out, in order.
func without(text string, cut [][2]int) string {
	if len(cut) == 0 {
		return text
	}
	var b strings.Builder
	at := 0
	for _, c := range cut {
		b.WriteString(text[at:c[0]])
		at = c[1]
	}
	b.WriteString(text[at:])
	return b.String()
}

// checkWhole, which the package's tests set, holds the local check of an
// edit made in place to the whole graph's: both or neither refuse.
var checkWhole bool

// check is the graph's invariants after an edit: in place, on the forms
// holding what the journal saved and the edit made (Editor.CheckForms, a
// few forms where Check walks every node); on a copy, the whole graph.
func check(e *graph.Editor, w *graph.Graph, made [][]*graph.Node) error {
	j := w.Recording()
	if j == nil {
		return e.Check()
	}
	ns := j.Saved()
	for _, m := range made {
		ns = append(ns, m...)
	}
	err := e.CheckForms(e.TopForms(ns))
	if checkWhole {
		if whole := e.Check(); (whole == nil) != (err == nil) {
			return fmt.Errorf("the local check (%v) and the whole graph's (%v) disagree", err, whole)
		}
	}
	return err
}

// renamedOps are ops' renamings: the text printed before says them only
// respelled, which inPlace does not.
func (a *aligner) renamedOps(ops []Op) []Op {
	var out []Op
	for _, o := range ops {
		if o.Kind == "rename" {
			out = append(out, o)
		}
	}
	return out
}

// inPlace is text, the view printed before an edit, with each op's span
// replaced by what it made, printed by p -- a deleted node's span emptied,
// an insertion's put before, after or at the end of its node's -- applied
// from the last place back.  made is applyMade's: the splices' nodes, in
// the order of the ops that splice.
func inPlace(text string, spans []Span, ops []Op, made [][]*graph.Node, p Printer) string {
	spanOf := func(n *graph.Node) (Span, bool) {
		best, ok := Span{}, false
		for _, s := range spans {
			if s.Node == n && (s.Kind == SpanForm || !ok) {
				best, ok = s, true
			}
		}
		return best, ok
	}
	type cut struct {
		lo, hi int
		with   string
	}
	var cuts []cut
	for _, o := range ops {
		switch o.Kind {
		case "delete":
			if sp, ok := spanOf(o.Node); ok {
				cuts = append(cuts, cut{sp.Start, sp.End, ""})
			}
			continue
		case "replace", "insert", "member":
		default:
			continue
		}
		var printed []string
		for _, n := range o.Made {
			t, _ := p.RenderForm(n)
			printed = append(printed, t)
		}
		with := " " + strings.Join(printed, " ") + " "
		sp, ok := spanOf(o.Node)
		if !ok {
			continue
		}
		switch {
		case o.Kind == "replace":
			hi := sp.End
			if o.Last != nil && o.Last != o.Node {
				if l, ok := spanOf(o.Last); ok {
					hi = l.End
				}
			}
			cuts = append(cuts, cut{sp.Start, hi, with})
		case o.Where == "before":
			cuts = append(cuts, cut{sp.Start, sp.Start, with})
		case o.Where == "after":
			cuts = append(cuts, cut{sp.End, sp.End, with})
		default: // at the end of the node: before its closing parenthesis
			cuts = append(cuts, cut{sp.End - 1, sp.End - 1, with})
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].lo > cuts[j].lo })
	for _, c := range cuts {
		if c.lo < 0 || c.hi > len(text) || c.lo > c.hi {
			continue
		}
		text = text[:c.lo] + c.with + text[c.hi:]
	}
	return text
}

// members is a run of a struct's (or union's) definition changed: members
// inserted beside one it has (InsertMember: the struct stays the node its
// uses name, where a replacement of the definition left them dangling), or
// deleted; a member replaced climbs, as the definition's own change.
func (a *aligner) members(o *sx, p *graph.Node, olds, news []*sx, g gap) ([]Op, bool, error) {
	switch {
	case len(olds) > 0 && len(news) == 0:
		var ops []Op
		for _, x := range olds {
			f := a.info[x].form
			if f == nil {
				return nil, true, nil
			}
			ops = append(ops, Op{Kind: "delete", Node: f})
		}
		return ops, false, nil
	case len(olds) == 0 && len(news) > 0:
		var at *graph.Node
		where := "after"
		if g.o0 > 0 {
			at = a.info[o.kids[g.o0-1]].form
		}
		if (at == nil || !a.isMember(p, at)) && g.o1 < len(o.kids) {
			at, where = a.info[o.kids[g.o1]].form, "before"
		}
		if at == nil || !a.isMember(p, at) {
			return nil, true, nil
		}
		var ops []Op
		for i := range news {
			x := news[i]
			if where == "after" {
				x = news[len(news)-1-i] // each after the same member: the last first
			}
			if !x.list {
				return nil, true, nil
			}
			ops = append(ops, Op{Kind: "member", Node: at, Where: where, Src: x.String()})
		}
		return ops, false, nil
	}
	return nil, true, nil
}

// isMember says m is one of struct p's members.
func (a *aligner) isMember(p, m *graph.Node) bool { return m != nil && a.ix.Parent(m) == p && m.IsList() }
