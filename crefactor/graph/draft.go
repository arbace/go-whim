package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// DRAFT: a phase's acts made as text on the graph's C view, and committed
// to the graph as FRAG of the smallest runs of whole items that hold them.
//
// Some phases are mostly literal C -- a struct written anew, a function's
// head and body, a statement swapped for another -- and their programs
// find each literal by its text and assert what they assert by counting
// lines.  A Draft keeps those acts as they are: the program edits the C
// view as a string, as its text version edited the file, and Commit makes
// the difference on the graph.  The text is diffed against the view line by
// line, each change is widened to the run of whole items -- top-level forms,
// or a block's or a body's items, the innermost list whose items hold it --
// and every run is replaced in ONE synthesized import (SpliceC), or deleted
// where its new text is empty.  Everything outside the runs keeps its ids
// and its edges.
//
// What FRAG retargets by itself is what cc resolves in the forms it prints
// whole.  A top-level form written anew can also be what other forms name
// through a tag, a member or an enumerator -- `struct memfile` rewritten
// with one member fewer, while a hundred functions select the members it
// keeps -- and those uses are carried: every live use of a node the commit
// removed is pointed at the new node of the same kind and name (a tag's
// definition, a member of that tag, an enumerator, a top-level
// declaration), its typed edges with it.  A use with no such node is a
// refusal: the text would not have compiled.

// A Draft is the C view of an editor's graph, with where each top-level
// form and item was printed.
type Draft struct {
	e     *Editor
	text  string
	spans map[*Node][2]int
	owner map[*Node]*Node // a list of items -> the item (or nil: the file) that holds it
	lists map[*Node][]*Node
}

// DraftStats say what a commit made.
type DraftStats struct {
	Hunks    int // the line changes the diff found
	Replaced int // runs replaced by FRAG
	Inserted int // runs inserted by FRAG
	Deleted  int // runs deleted
	Items    int // the old items the runs held
	Carried  int // uses carried to a new declaration of their name
}

func (s DraftStats) String() string {
	return fmt.Sprintf("%d changes: %d runs replaced, %d inserted, %d deleted (%d items), %d uses carried",
		s.Hunks, s.Replaced, s.Inserted, s.Deleted, s.Items, s.Carried)
}

// Draft prints the graph's C view, keeping where each form and item is.
func (e *Editor) Draft() (*Draft, error) {
	d := &Draft{e: e, spans: map[*Node][2]int{}, owner: map[*Node]*Node{}, lists: map[*Node][]*Node{}}
	of := map[*clisp.Node]*Node{}
	var lisp func(n *Node) *clisp.Node
	lisp = func(n *Node) *clisp.Node {
		var c *clisp.Node
		if !n.list {
			c = clisp.A(n.Atom)
		} else {
			kids := make([]*clisp.Node, len(n.Kids))
			for i, k := range n.Kids {
				kids[i] = lisp(k)
			}
			c = clisp.L(kids...)
		}
		of[c] = n
		return c
	}
	forms := make([]*clisp.Node, len(e.g.Forms))
	for i, f := range e.g.Forms {
		forms[i] = lisp(f)
	}
	text, err := clisp.PrintSpans(forms, func(n *clisp.Node, s, z int) {
		if g := of[n]; g != nil {
			d.spans[g] = [2]int{s, z}
		}
	})
	if err != nil {
		return nil, err
	}
	d.text = string(text)
	// the lists of items, each with the item that holds it
	var walk func(n, item *Node)
	walk = func(n, item *Node) {
		if !n.list {
			return
		}
		if lo := itemsFrom(n); lo >= 0 {
			d.owner[n] = item
			d.lists[item] = append(d.lists[item], n)
			for _, k := range n.Kids[lo:] {
				walk(k, k)
			}
			return
		}
		for _, k := range n.Kids {
			walk(k, item)
		}
	}
	for _, f := range e.g.Forms {
		walk(f, f)
	}
	return d, nil
}

// Text is the C view the draft was made from.
func (d *Draft) Text() string { return d.text }

// a draftRun is where one group of changes goes: the items [lo, hi) of
// list p (an insertion where lo == hi), the old bytes [s, z) it stands for.
type draftRun struct {
	p      *Node // the list: e.top[0] for the file's forms
	lo, hi int
	s, z   int
}

func (d *Draft) items(p *Node) []*Node {
	if p == d.e.top[0] {
		return d.e.g.Forms
	}
	return p.Kids[itemsFrom(p):]
}

func (d *Draft) base(p *Node) int {
	if p == d.e.top[0] {
		return 0
	}
	return itemsFrom(p)
}

// locate is the innermost run of items holding the old bytes [s, z), or the
// place an insertion at s goes.
func (d *Draft) locate(s, z int) draftRun {
	p := d.e.top[0]
	for {
		its := d.items(p)
		first, last := -1, -1
		for i, it := range its {
			sp := d.spans[it]
			in := sp[0] < z && sp[1] > s
			if s == z {
				in = sp[0] < s && s < sp[1]
			}
			if in {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		b := d.base(p)
		if first < 0 { // between items: an insertion before the first that starts at z or later
			k := len(its)
			for i, it := range its {
				if d.spans[it][0] >= z {
					k = i
					break
				}
			}
			return draftRun{p: p, lo: b + k, hi: b + k, s: s, z: z}
		}
		if first == last {
			x := its[first]
			sp := d.spans[x]
			if s >= sp[0] && z <= sp[1] {
				var into *Node
				for _, l := range d.lists[x] {
					li := d.items(l)
					if len(li) == 0 {
						continue
					}
					ls, lz := d.spans[li[0]][0], d.spans[li[len(li)-1]][1]
					if s >= ls && z <= lz {
						into = l
						break
					}
				}
				if into != nil {
					p = into
					continue
				}
			}
		}
		rs, rz := d.spans[its[first]][0], d.spans[its[last]][1]
		return draftRun{p: p, lo: b + first, hi: b + last + 1, s: min(s, rs), z: max(z, rz)}
	}
}

// within says list p is inside item x, or is x's own.
func (d *Draft) within(p, x *Node) bool {
	for l := p; l != nil && l != d.e.top[0]; {
		it := d.owner[l]
		if it == x {
			return true
		}
		if it == nil {
			return false
		}
		l = it.up
	}
	return false
}

// conflict says two runs cannot be made side by side: they share or touch
// items of one list, one is inside an item the other replaces, or their
// old bytes overlap.
func (d *Draft) conflict(a, b draftRun) bool {
	if a.s < b.z && b.s < a.z || a.s == a.z && b.s < a.s && a.s < b.z || b.s == b.z && a.s < b.s && b.s < a.z {
		return true
	}
	if a.p == b.p {
		if a.lo == b.lo {
			return true
		}
		return a.lo < b.hi && b.lo < a.hi
	}
	ks := func(r draftRun) []*Node { return d.e.kids(r.p)[r.lo:r.hi] }
	for _, x := range ks(a) {
		if d.within(b.p, x) {
			return true
		}
	}
	for _, x := range ks(b) {
		if d.within(a.p, x) {
			return true
		}
	}
	return false
}

// Commit makes the graph's C view the text: the changes from the draft's
// text, each the run of whole items around it written anew by FRAG (in one
// synthesized import), or deleted, and the uses outside the runs of what the
// runs declared carried to the new declarations.
func (d *Draft) Commit(text string) (DraftStats, error) {
	var st DraftStats
	if text == d.text {
		return st, nil
	}
	e := d.e
	ol, nl := splitLines(d.text), splitLines(text)
	off := make([]int, len(ol)+1)
	for i, l := range ol {
		off[i+1] = off[i] + len(l)
	}
	hunks := slideHunks(diffLines(ol, nl), ol, nl)
	st.Hunks = len(hunks)
	// groups of hunks, merged until their runs can be made side by side,
	// and widened to the item around them until each run's new text closes
	// every bracket it opens
	type group struct {
		hs  []lineHunk
		run draftRun
		src string
	}
	var gs []*group
	for _, h := range hunks {
		gs = append(gs, &group{hs: []lineHunk{h}, run: d.locate(off[h.OA], off[h.OB])})
	}
	line := func(at int) int { return sort.SearchInts(off, at) }
	text1 := func(g *group) string {
		sort.Slice(g.hs, func(a, b int) bool { return g.hs[a].OA < g.hs[b].OA })
		la, lb := line(g.run.s), line(g.run.z)
		var b strings.Builder
		k := la
		for _, h := range g.hs {
			for ; k < h.OA; k++ {
				b.WriteString(ol[k])
			}
			for _, l := range nl[h.NA:h.NB] {
				b.WriteString(l)
			}
			k = h.OB
		}
		for ; k < lb; k++ {
			b.WriteString(ol[k])
		}
		return b.String()
	}
	for again := true; again; {
		again = false
		for merged := true; merged; {
			merged = false
		scan:
			for i := 0; i < len(gs); i++ {
				for j := i + 1; j < len(gs); j++ {
					if d.conflict(gs[i].run, gs[j].run) {
						a, b := gs[i], gs[j]
						a.hs = append(a.hs, b.hs...)
						a.run = d.locate(min(a.run.s, b.run.s), max(a.run.z, b.run.z))
						gs = append(gs[:j], gs[j+1:]...)
						merged = true
						break scan
					}
				}
			}
		}
		for _, g := range gs {
			g.src = text1(g)
			if balanced(g.src) {
				continue
			}
			x := d.owner[g.run.p]
			if g.run.p == e.top[0] || x == nil {
				return st, fmt.Errorf("draft: the change at bytes %d-%d does not close its brackets within the file's forms", g.run.s, g.run.z)
			}
			sp := d.spans[x]
			g.run = d.locate(min(g.run.s, sp[0]), max(g.run.z, sp[1]))
			again = true
		}
	}
	type made struct {
		run   draftRun
		first *Node // the run's first old item, or for an insertion the item it goes before (nil: the end)
		last  *Node
		src   string
	}
	var ms []made
	for _, g := range gs {
		m := made{run: g.run, src: g.src}
		ks := e.kids(g.run.p)
		if g.run.lo < g.run.hi {
			m.first, m.last = ks[g.run.lo], ks[g.run.hi-1]
		} else if g.run.lo < len(ks) {
			m.first = ks[g.run.lo]
		}
		ms = append(ms, m)
	}
	// what the runs held, and what other forms name in it, by key
	keyOf := map[*Node]string{}
	var old []*Node
	for _, m := range ms {
		for _, x := range e.kids(m.run.p)[m.run.lo:m.run.hi] {
			st.Items++
			draftKeys(x, m.run.p == e.top[0], "", func(n *Node, k string) {
				keyOf[n] = k
			})
			Walk(x, func(n *Node) bool { old = append(old, n); return true })
		}
	}
	// the deletions first, so that nothing the new text no longer declares
	// stands beside it in the synthesized unit
	var fs []Frag
	var placed []made
	for _, m := range ms {
		if strings.TrimSpace(m.src) != "" {
			placed = append(placed, m)
			continue
		}
		if m.run.lo == m.run.hi {
			continue
		}
		p, lo := e.index(m.first)
		_, hi := e.index(m.last)
		if lo < 0 || hi < 0 {
			return st, fmt.Errorf("draft: the run #%d..#%d is not in the graph", m.first.ID, m.last.ID)
		}
		if err := e.splice("delete", p, lo, hi+1, nil); err != nil {
			return st, fmt.Errorf("draft: %w", err)
		}
		st.Deleted++
	}
	for _, m := range placed {
		var at Spot
		switch {
		case m.run.lo < m.run.hi:
			at = e.SpotRun(m.first, m.last)
			st.Replaced++
		case m.first != nil:
			at = e.SpotBefore(m.first)
			st.Inserted++
		default:
			p := m.run.p
			if p == e.top[0] {
				p = nil
			}
			at = e.SpotEnd(p)
			st.Inserted++
		}
		fs = append(fs, Frag{At: at, Src: m.src})
	}
	var nodes [][]*Node
	if len(fs) > 0 {
		var err error
		if nodes, err = e.spliceOrdered(fs); err != nil {
			return st, fmt.Errorf("draft: %w", err)
		}
	}
	// the new declarations by key
	byKey := map[string]*Node{}
	for i, ns := range nodes {
		top := fs[i].At.p == e.top[0]
		for _, n := range ns {
			draftKeys(n, top, "", func(x *Node, k string) {
				if _, dup := byKey[k]; !dup {
					byKey[k] = x
				}
			})
		}
	}
	// carry what still names what the runs held
	var lost []string
	for _, x := range old {
		uses := e.Uses(x)
		var typed []*Node
		for _, q := range e.typedBy[x] {
			if q.Type == x && e.Live(q) {
				typed = append(typed, q)
			}
		}
		if len(uses) == 0 && len(typed) == 0 {
			continue
		}
		k := keyOf[x]
		to := byKey[k]
		if to == nil && k != "" {
			to = e.draftExternTag(k)
		}
		if k == "" || to == nil {
			at := ""
			if len(uses) > 0 {
				at = ", first in " + label(e.topOf(uses[0]))
				if n := topName(e.topOf(uses[0])); n != "" {
					at += " " + n
				}
			}
			lost = append(lost, fmt.Sprintf("#%d (%s) %q, %d uses%s", x.ID, label(x), k, len(uses)+len(typed), at))
			continue
		}
		for _, u := range uses {
			for i, r := range u.Refs {
				if r == x {
					e.retargetTo(u, i, to)
					st.Carried++
				}
			}
		}
		for _, q := range typed {
			q.Type = to
			e.typedBy[to] = append(e.typedBy[to], q)
			st.Carried++
		}
	}
	if len(lost) > 0 {
		sort.Strings(lost)
		if len(lost) > 8 {
			lost = append(lost[:8], fmt.Sprintf("and %d more", len(lost)-8))
		}
		return st, fmt.Errorf("draft: still named, and the new text declares nothing of its kind and name: %s",
			strings.Join(lost, "; "))
	}
	return st, nil
}

// draftKeys gives each declaration in n that other forms can name a key:
// a top-level declaration by its head and name, a tag's definition by its
// keyword and tag, a member by its struct's key and its name, an
// enumerator by its name.  ctx is the key of the declaration n is in.
func draftKeys(n *Node, top bool, ctx string, f func(*Node, string)) {
	if !n.list {
		return
	}
	if top {
		if name := topName(n); name != "" {
			ctx = n.Head() + " " + name
			f(n, ctx)
		}
	}
	if isDefForm(n) {
		k := ctx + "/" + n.Head()
		if a := n.Args(); len(a) > 0 && !a[0].list && a[0].Atom != "{}" {
			k = n.Head() + " " + a[0].Atom
		}
		f(n, k)
		for _, m := range body(n) {
			if !m.list || len(m.Kids) == 0 {
				continue
			}
			if n.Is("enum") {
				if !m.Kids[0].list {
					f(m, "enumerator "+m.Kids[0].Atom)
				}
				continue
			}
			if !m.Kids[0].list {
				f(m, k+"."+m.Kids[0].Atom)
			}
			for _, x := range m.Kids[1:] {
				draftKeys(x, false, k+"."+m.Kids[0].Atom, f)
			}
		}
		return
	}
	for _, x := range n.Kids {
		draftKeys(x, false, ctx, f)
	}
}

// spliceOrdered is SpliceC for fragments that name one another: a function
// written anew calls another written anew, a struct written anew points at
// another.  The splice takes a fragment's nodes only where every node they
// refer to is in the graph or in the same replacement, so each edge from
// one fragment's nodes into another's is held at a node the graph has
// while the fragments go in -- their spots kept as nodes, since one splice
// moves another's indexes -- and put back, indexed, once all are in.
func (e *Editor) spliceOrdered(fs []Frag) ([][]*Node, error) {
	s, err := e.frag(fs)
	if err != nil {
		return nil, err
	}
	type place struct {
		p           *Node
		first, last *Node // the run replaced, or first the item an insertion goes before
	}
	owner := map[*Node]*fragJob{}
	places := make([]place, len(s.jobs))
	for i, j := range s.jobs {
		at := j.f.At
		ks := e.kids(at.p)
		pl := place{p: at.p}
		switch {
		case at.lo < at.hi:
			pl.first, pl.last = ks[at.lo], ks[at.hi-1]
		case at.lo < len(ks):
			pl.first = ks[at.lo]
		}
		places[i] = pl
		for _, n := range j.nodes {
			Walk(n, func(x *Node) bool { owner[x] = j; return true })
		}
	}
	// the edges across fragments, held at a node the graph has meanwhile
	hold := e.g.Forms[0]
	type held struct {
		x *Node
		i int // a refers edge's index; -1 the typed edge
		r *Node
	}
	var hs []held
	for _, j := range s.jobs {
		for _, n := range j.nodes {
			Walk(n, func(x *Node) bool {
				for i, r := range x.Refs {
					if o := owner[r]; o != nil && o != j {
						hs = append(hs, held{x, i, r})
						x.Refs[i] = hold
					}
				}
				if o := owner[x.Type]; x.Type != nil && o != nil && o != j {
					hs = append(hs, held{x, -1, x.Type})
					x.Type = nil
				}
				return true
			})
		}
	}
	for i, j := range s.jobs {
		pl := places[i]
		var lo, hi int
		switch {
		case pl.last != nil:
			_, lo = e.index(pl.first)
			_, hi = e.index(pl.last)
			hi++
		case pl.first != nil:
			_, lo = e.index(pl.first)
			hi = lo
		default:
			lo = len(e.kids(pl.p))
			hi = lo
		}
		if lo < 0 || hi < lo {
			return nil, fmt.Errorf("frag %d: its spot left the graph", j.k)
		}
		op := "replace"
		if lo == hi {
			op = "insert"
		}
		if err := e.splice(op, pl.p, lo, hi, j.nodes); err != nil {
			return nil, fmt.Errorf("frag %d: %w", j.k, err)
		}
	}
	for _, h := range hs {
		if h.i < 0 {
			h.x.Type = h.r
			if e.inFile(h.r) {
				e.typedBy[h.r] = append(e.typedBy[h.r], h.x)
			}
			continue
		}
		h.x.Refs[h.i] = h.r
		e.extra[h.r] = append(e.extra[h.r], h.x)
	}
	if err := s.retarget(); err != nil {
		return nil, err
	}
	out := make([][]*Node, len(s.jobs))
	for i, j := range s.jobs {
		out[i] = j.nodes
	}
	return out, nil
}

// draftExternTag is the external node a tag nothing defines any more
// resolves to, as the importer resolves one (`(extern-struct T)`), found or
// added; nil for a key that is not a tag's.
func (e *Editor) draftExternTag(key string) *Node {
	kw, tag, ok := strings.Cut(key, " ")
	if !ok || kw != "struct" && kw != "union" && kw != "enum" || strings.ContainsAny(tag, " ./") {
		return nil
	}
	for _, x := range e.g.Externs {
		if x.Is("extern-"+kw) && len(x.Kids) > 1 && x.Kids[1].Atom == tag {
			return x
		}
	}
	n := NewList(NewAtom("extern-"+kw), NewAtom(tag))
	e.addTo(2, n)
	return n
}

// slideHunks moves each hunk that only inserts or only deletes along the
// lines that repeat around it -- a diff may put `}` on either side of a
// function it adds -- to where the lines it moves close every bracket
// they open, the lowest such place, and leaves it where it is when none
// does.  A hunk does not slide past its neighbours.
func slideHunks(hs []lineHunk, a, b []string) []lineHunk {
	for i := range hs {
		h := hs[i]
		lo, hi := 0, len(a) // the old lines it may move within
		if i > 0 {
			lo = hs[i-1].OB
		}
		if i+1 < len(hs) {
			hi = hs[i+1].OA
		}
		moved := func(h lineHunk) string {
			if h.OA == h.OB {
				return strings.Join(b[h.NA:h.NB], "")
			}
			return strings.Join(a[h.OA:h.OB], "")
		}
		var at []lineHunk
		switch {
		case h.OA == h.OB && h.NA < h.NB: // an insertion
			x := h
			for x.OA > lo && x.NA > 0 && b[x.NA-1] == b[x.NB-1] {
				x = lineHunk{x.OA - 1, x.OB - 1, x.NA - 1, x.NB - 1}
			}
			for ; ; x = (lineHunk{x.OA + 1, x.OB + 1, x.NA + 1, x.NB + 1}) {
				at = append(at, x)
				if !(x.OB < hi && x.NB < len(b) && b[x.NA] == b[x.NB]) {
					break
				}
			}
		case h.NA == h.NB && h.OA < h.OB: // a deletion
			x := h
			for x.OA > lo && a[x.OA-1] == a[x.OB-1] {
				x = lineHunk{x.OA - 1, x.OB - 1, x.NA - 1, x.NB - 1}
			}
			for ; ; x = (lineHunk{x.OA + 1, x.OB + 1, x.NA + 1, x.NB + 1}) {
				at = append(at, x)
				if !(x.OB < hi && a[x.OA] == a[x.OB]) {
					break
				}
			}
		default:
			continue
		}
		for k := len(at) - 1; k >= 0; k-- {
			if balanced(moved(at[k])) {
				hs[i] = at[k]
				break
			}
		}
	}
	return hs
}

// balanced says s closes every brace, bracket and parenthesis it opens, in
// order, outside its literals.
func balanced(s string) bool {
	var open []byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			i = skipLiteral(s, i) - 1
		case '{', '[', '(':
			open = append(open, c)
		case '}', ']', ')':
			want := map[byte]byte{'}': '{', ']': '[', ')': '('}[c]
			if len(open) == 0 || open[len(open)-1] != want {
				return false
			}
			open = open[:len(open)-1]
		}
	}
	return len(open) == 0
}
