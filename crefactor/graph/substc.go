package graph

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// SUBSTITUTE: a text program's literal substitution -- `sub(old, new, n)`,
// the old C found in the file and replaced by the new -- made on the
// graph, where the old C is not whole items.  LiteralC (fragverbs.go) takes
// a literal that is a run of whole items; a text program's literals often
// begin or end inside one (an `else if` condition, one declaration of a
// run, a statement and the brace after it).  SubstituteC finds, for each
// place the old C occurs (spaces aside, outside literals: normC), the
// INNERMOST run of items whose C holds it, and then narrows: of the nodes
// inside that run, the smallest whose own C holds every byte the
// substitution changes -- an item, a body, an expression -- is the node
// replaced, by its C with the substitution made, through FRAG.  So what
// the text changed is new nodes, with fresh ids, and everything around it
// keeps its ids and edges.
//
// The result is held to the text twice.  Before anything changes, the node
// narrowed to is shown to be where the old C was: the run printed with that
// node replaced by a marker must be the run's C with the marker standing
// for the node's C, spaces aside -- so the new C made there is the text's
// substitution of the run, byte for byte once printed.  And FRAG refuses a
// fragment that does not stand where it is written (an operator's
// precedence across the node's edge), so nothing parses otherwise.

// A Subst is one literal substitution: Old's C becomes New's, at N places.
type Subst struct {
	In       string // the function it is made in; "" is the verbs' scope
	Near     string // at file scope: among the top-level forms around those declaring Near
	Old, New string
	N        int
	What     string
}

// substPlace is one place a substitution is made.
type substPlace struct {
	sub   int     // the Subst
	items []*Node // the innermost run of items holding the old C
	at    *Node   // the node replaced: an item of the run, or a node inside one; nil is the run
	front *Node   // or the item of the run the new items go before, the run's own kept
	back  *Node   // or the run's last item, when they go after it
	src   string  // at's new C (the run's, for nil; the new items, for front)
	list  *Node   // the run's container
	o     int     // where the old C stands in the run's, normalized
	klen  int     // and its length
	made  []*Node // what replaced it
}

// substFinder caches the normalized C of the items it prints, for one
// act: nothing is edited while it finds.
type substFinder struct {
	e     *Editor
	text  map[*Node]string // the items', for one finding
	forms map[*Node]string // the top-level forms', kept across a Substitutor's flushes
}

func newFinder(e *Editor, forms map[*Node]string) *substFinder {
	if forms == nil {
		forms = map[*Node]string{}
	}
	return &substFinder{e: e, text: map[*Node]string{}, forms: forms}
}

// itemC is an item's C, normalized; a top-level form's printed as the C
// view prints it.
func (f *substFinder) itemC(n *Node) (string, error) {
	top := n.up == f.e.top[0]
	if s, ok := f.text[n]; ok {
		return s, nil
	}
	if s, ok := f.forms[n]; ok && top {
		return s, nil
	}
	var c string
	var err error
	if top {
		var b []byte
		b, err = FormsC([]*Node{n})
		c = string(b)
	} else {
		c, err = clisp.PrintItems([]*clisp.Node{Lisp(n)})
	}
	if err != nil {
		return "", err
	}
	s := normC(c)
	if top {
		f.forms[n] = s
	} else {
		f.text[n] = s
	}
	return s, nil
}

// SubstituteC makes the substitutions, every one's places found on the
// graph as it stands and all made in ONE synthesized import; each
// refuses unless it is found at exactly N places, and each is reported,
// in order, once all are made.  Two substitutions may not touch one run
// of items, or one inside another's: make them one after the other
// (SubstituteSeq).
func (v *Verbs) SubstituteC(subs ...Subst) {
	if v.Err != nil {
		return
	}
	f := newFinder(v.e, nil)
	var places []*substPlace
	for k, s := range subs {
		ps, err := f.find(v, k, s)
		if err != nil {
			v.Die("%s -- %v", s.What, err)
			return
		}
		if len(ps) != s.N {
			v.Die("%s -- occurs %d times, expected %d", s.What, len(ps), s.N)
			return
		}
		if q := overlapping(v.e, places, ps); q != nil {
			v.Die("%s -- its run is %s's too, or inside it: make them one after the other", s.What, subs[q.sub].What)
			return
		}
		places = append(places, ps...)
	}
	if !v.substApply(subs, places) {
		return
	}
	for _, s := range subs {
		v.Say(s.What)
	}
}

// SubstituteSeq makes the substitutions one after the other, as a text
// program's literals are made: each found on the graph the ones before it
// left.  Consecutive ones that are independent -- each found at its count
// on the graph before them, none in a run another touches or inside one,
// none whose old C another's new C holds or whose new C names a name
// another's new C brings -- are made in one synthesized import.  Each is
// reported, in order, once all are made.
func (v *Verbs) SubstituteSeq(subs ...Subst) {
	b := v.Substitutions()
	for _, s := range subs {
		b.Add(s)
	}
	b.Flush()
	for _, s := range subs {
		v.Say(s.What)
	}
}

// A Substitutor is SubstituteSeq taken apart, for a program that makes
// other acts between its substitutions: Add finds a substitution on the
// graph as the ones pending leave it, and makes those first where it must;
// Flush makes what is pending.  A caller's own act on the graph is made
// after a Flush, or, where it touches nothing pending (Pending), at once;
// it may delete or add top-level forms, and a form it changes inside is
// said by Touched, since the Substitutor keeps each form's C between its
// flushes.  Nothing is reported: the program says what it made.
type Substitutor struct {
	v      *Verbs
	f      *substFinder
	subs   []Subst
	places []*substPlace
}

// Substitutions starts a Substitutor on v's scope.
func (v *Verbs) Substitutions() *Substitutor {
	return &Substitutor{v: v, f: newFinder(v.e, nil)}
}

// Add finds s, after the substitutions pending, and holds it pending.
func (b *Substitutor) Add(s Subst) {
	v := b.v
	if v.Err != nil {
		return
	}
	k := len(b.subs)
	ps, err := b.f.find(v, k, s)
	free := err == nil && len(ps) == s.N && overlapping(v.e, b.places, ps) == nil
	for k, q := range b.subs {
		if free && (strings.Contains(normC(q.New), normC(s.Old)) || usesNew(v.e, q, b.of(k), s, ps)) {
			free = false
		}
	}
	if !free && len(b.subs) > 0 {
		b.Flush()
		if v.Err != nil {
			return
		}
		k = 0
		ps, err = b.f.find(v, k, s)
	}
	if err != nil {
		v.Die("%s -- %v", s.What, err)
		return
	}
	if len(ps) != s.N {
		v.Die("%s -- occurs %d times, expected %d", s.What, len(ps), s.N)
		return
	}
	b.subs = append(b.subs, s)
	b.places = append(b.places, ps...)
}

// usesNew says s's new C names a name q's new C brings that does not
// resolve where s is made: a fragment's names resolve to what the graph
// holds, so s waits for q.  What q brings is what its new C declares where
// s can see it: in the same top-level form, any name; from another, only
// the names a top-level fragment writes outside braces.
func usesNew(e *Editor, q Subst, qps []*substPlace, s Subst, ps []*substPlace) bool {
	old := map[string]bool{}
	for _, x := range identsIn(q.Old) {
		old[x] = true
	}
	for _, x := range identsIn(s.Old) {
		old[x] = true
	}
	tops := map[*Node]bool{}
	top := false
	for _, p := range qps {
		tops[e.TopForm(p.items[0])] = true
		top = top || p.list == e.top[0]
	}
	same := false
	for _, p := range ps {
		same = same || tops[e.TopForm(p.items[0])]
	}
	var names []string
	switch {
	case same:
		names = identsIn(q.New)
	case top:
		names = identsIn(outsideBraces(q.New))
	}
	brought := map[string]bool{}
	for _, x := range names {
		if !old[x] && !keywords[x] && !c23Keywords[x] {
			brought[x] = true
		}
	}
	for _, x := range identsIn(s.New) {
		if !brought[x] {
			continue
		}
		for _, p := range ps {
			if e.Resolve(p.items[0], x) == nil {
				return true
			}
		}
	}
	return false
}

// outsideBraces is C with what is inside braces taken out, literals kept.
func outsideBraces(c string) string {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(c); i++ {
		switch ch := c[i]; {
		case ch == '"' || ch == '\'':
			k := skipLiteral(c, i)
			if depth == 0 {
				b.WriteString(c[i:k])
			}
			i = k - 1
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			b.WriteByte(' ')
		case depth == 0:
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// of is substitution k's places.
func (b *Substitutor) of(k int) []*substPlace {
	var out []*substPlace
	for _, p := range b.places {
		if p.sub == k {
			out = append(out, p)
		}
	}
	return out
}

// Touched says the caller changed what is inside the top-level form of n.
func (b *Substitutor) Touched(n *Node) { delete(b.f.forms, b.v.e.TopForm(n)) }

// Pending says whether a pending substitution touches n or what is in it.
func (b *Substitutor) Pending(n *Node) bool {
	return overlapping(b.v.e, b.places, []*substPlace{{items: []*Node{n}}}) != nil
}

// Flush makes the pending substitutions.
func (b *Substitutor) Flush() {
	if b.v.Err == nil && len(b.subs) > 0 {
		// the top-level forms the substitutions are made in are printed
		// again when next asked; the others' C is kept
		for _, p := range b.places {
			for _, it := range p.items {
				delete(b.f.forms, b.v.e.TopForm(it))
			}
		}
		b.v.substApply(b.subs, b.places)
	}
	b.subs, b.places = nil, nil
	b.f = newFinder(b.v.e, b.f.forms)
}

// overlapping is a place of ps that a place of qs conflicts with, or nil:
// one whose old C, where it stands in the other's run, meets the other's,
// or whose node replaced is the other's or holds it.  Two places in one
// run, or one in a run inside the other's, whose old C are apart are made
// in one import as they would be one after the other.
func overlapping(e *Editor, ps, qs []*substPlace) *substPlace {
	for _, p := range ps {
		for _, q := range qs {
			if conflict(e, p, q) || conflict(e, q, p) {
				return p
			}
		}
	}
	return nil
}

// replaced is the node a place's fragment takes the place of (or goes
// before): its narrowed node, its insertion's item, or its run's first.
func (p *substPlace) replaced() []*Node {
	switch {
	case p.front != nil:
		return []*Node{p.front}
	case p.back != nil:
		return []*Node{p.back}
	case p.at != nil:
		return []*Node{p.at}
	}
	return p.items
}

// conflict says q's run is p's or inside it and the two conflict.
func conflict(e *Editor, p, q *substPlace) bool {
	in := map[*Node]bool{}
	for _, it := range p.items {
		in[it] = true
	}
	held := false
	for x := q.items[0]; x != nil && !held; x = e.Parent(x) {
		held = in[x]
	}
	if !held {
		return false
	}
	// the nodes replaced: one the other's or inside it
	for _, a := range p.replaced() {
		for _, b := range q.replaced() {
			for x := b; x != nil; x = e.Parent(x) {
				if x == a {
					return true
				}
			}
			for x := a; x != nil; x = e.Parent(x) {
				if x == b {
					return true
				}
			}
		}
	}
	// where q's run stands in p's run's C: p's run printed with q's first
	// item a marker
	const mark = "__whim_subst_mark"
	m := clisp.L(clisp.A("call"), clisp.A(mark))
	if p.list == e.top[0] && q.list == e.top[0] {
		m = clisp.L(clisp.A("def"), clisp.A(mark), clisp.A("int"))
	}
	ls := make([]*clisp.Node, len(p.items))
	for i, it := range p.items {
		ls[i] = lispWith(it, q.items[0], m)
	}
	var c string
	var err error
	if p.list == e.top[0] {
		b, perr := clisp.Print(ls)
		c, err = string(b), perr
	} else {
		c, err = clisp.PrintItems(ls)
	}
	if err != nil {
		return true
	}
	at := strings.Index(normC(c), mark)
	if at < 0 {
		return true
	}
	lo, hi := at+q.o, at+q.o+q.klen
	return lo < p.o+p.klen && p.o < hi
}

// substApply makes the places found: the deletions, then every fragment in
// one unit.
func (v *Verbs) substApply(subs []Subst, places []*substPlace) bool {
	var frags []Frag
	var fragOf []*substPlace
	for _, p := range places {
		if p.src != "" {
			continue
		}
		first, last := p.items[0], p.items[len(p.items)-1]
		if p.at != nil {
			first, last = p.at, p.at
		}
		if err := v.e.ReplaceRun(first, last); err != nil {
			v.Die("%s -- %v", subs[p.sub].What, err)
			return false
		}
	}
	for _, p := range places {
		if p.src == "" {
			continue
		}
		at := v.e.SpotRun(p.items[0], p.items[len(p.items)-1])
		switch {
		case p.front != nil:
			at = v.e.SpotBefore(p.front)
		case p.back != nil:
			at = v.e.SpotAfter(p.back)
		case p.at != nil:
			at = v.e.SpotOf(p.at)
		}
		if at.err != nil {
			v.Die("%s -- %v", subs[p.sub].What, at.err)
			return false
		}
		frags = append(frags, Frag{At: at, Src: p.src})
		fragOf = append(fragOf, p)
	}
	if len(frags) > 0 {
		made, err := v.e.SpliceC(frags...)
		if err != nil {
			what := "the substitutions"
			if len(fragOf) == 1 {
				what = subs[fragOf[0].sub].What
			}
			if m := fragNumRE.FindStringSubmatch(err.Error()); m != nil {
				var k int
				fmt.Sscan(m[1], &k)
				if k >= 1 && k <= len(fragOf) {
					what = subs[fragOf[k-1].sub].What
				}
			}
			v.Die("%s -- %v", what, err)
			return false
		}
		for i, p := range fragOf {
			p.made = made[i]
		}
	}
	return true
}

// find is the places substitution k is made: the innermost runs holding
// its old C, each narrowed.
func (f *substFinder) find(v *Verbs, k int, s Subst) ([]*substPlace, error) {
	key, rep := normC(s.Old), normC(s.New)
	if key == "" {
		return nil, fmt.Errorf("the old C is empty")
	}
	var roots []*Node
	var forms []*Node // the file's forms searched as a run
	switch {
	case s.In != "":
		d := f.e.Defn(s.In)
		if d == nil {
			return nil, fmt.Errorf("%s is not defined at file scope", s.In)
		}
		roots = []*Node{d}
	case s.Near != "":
		at := map[int]bool{}
		for i, x := range f.e.g.Forms {
			if declares(x, s.Near) {
				for j := max(0, i-16); j <= min(len(f.e.g.Forms)-1, i+16); j++ {
					at[j] = true
				}
			}
		}
		if len(at) == 0 {
			return nil, fmt.Errorf("no top-level form declares %s", s.Near)
		}
		for j, x := range f.e.g.Forms {
			if at[j] {
				forms = append(forms, x)
				roots = append(roots, x)
			}
		}
	case v.scope != nil:
		roots = v.roots()
	default:
		// the file: its forms as a run, and inside them only the forms
		// whose C holds the key
		forms = f.e.g.Forms
		for _, x := range forms {
			c, err := f.itemC(x)
			if err != nil {
				return nil, err
			}
			if strings.Contains(c, key) {
				roots = append(roots, x)
			}
		}
	}
	type run struct {
		items []*Node
		text  string
	}
	var runs []run
	scan := func(items []*Node) error {
		ts := make([]string, len(items))
		for i := range items {
			// a quick look first: the run must begin with the key's start
			// inside item i, so it is no longer than t[i] and the key
			var err error
			if ts[i], err = f.itemC(items[i]); err != nil {
				return err
			}
		}
		for i := 0; i < len(items); i++ {
			acc := ts[i]
			for j := i; j < len(items); j++ {
				if j > i {
					acc += " " + ts[j]
				}
				if p := strings.Index(acc, key); p >= 0 {
					if p >= len(ts[i]) {
						break // it begins in a later item: that item's run
					}
					if strings.Count(acc, key) > 1 {
						return fmt.Errorf("occurs more than once in one run of items")
					}
					runs = append(runs, run{append([]*Node(nil), items[i:j+1]...), acc})
					break
				}
				if len(acc) > len(ts[i])+1+len(key) {
					break
				}
			}
		}
		return nil
	}
	if forms != nil {
		if err := scan(forms); err != nil {
			return nil, err
		}
	}
	var err error
	for _, r := range roots {
		Walk(r, func(l *Node) bool {
			if err != nil {
				return false
			}
			if lo := itemsFrom(l); lo >= 0 && lo <= len(l.Kids) {
				// a quick look first: nothing to find where the list's C
				// cannot hold the key
				err = scan(l.Kids[lo:])
			}
			return err == nil
		})
		if err != nil {
			return nil, err
		}
	}
	// the innermost: a run of one item that holds another run goes
	inner := map[*Node]bool{}
	for _, r := range runs {
		for p := f.e.Parent(r.items[0]); p != nil; p = f.e.Parent(p) {
			inner[p] = true
		}
	}
	var out []*substPlace
	for _, r := range runs {
		if len(r.items) == 1 && inner[r.items[0]] {
			continue
		}
		list, _ := f.e.index(r.items[0])
		p := &substPlace{sub: k, items: r.items, list: list, o: strings.Index(r.text, key), klen: len(key)}
		if err := f.narrow(p, r.text, key, rep); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// narrow is the smallest node of the place's run whose C holds every byte
// the substitution changes, and its new C.
func (f *substFinder) narrow(p *substPlace, text, key, rep string) error {
	o := strings.Index(text, key)
	pre := 0
	for pre < len(key) && pre < len(rep) && key[pre] == rep[pre] {
		pre++
	}
	suf := 0
	for suf < len(key)-pre && suf < len(rep)-pre && key[len(key)-1-suf] == rep[len(rep)-1-suf] {
		suf++
	}
	a, b := o+pre, o+len(key)-suf // the bytes changed, in text
	y := rep[pre : len(rep)-suf]  // what they become
	newOf := func(t string, a, b int) string { return normC(t[:a] + y + t[b:]) }
	// the run's items first: new items between two -- an insertion slid
	// back to where an item begins, if the text allows -- or the change
	// inside one of them
	starts := map[int]*Node{}
	off := 0
	for _, it := range p.items {
		starts[off] = it
		off += len(f.cached(it)) + 1
	}
	if a == b && a == len(text) && strings.HasPrefix(y, " ") {
		p.back, p.src = p.items[len(p.items)-1], normC(y)
		return nil
	}
	if a == b {
		for k := 0; k <= a && k <= len(y); k++ {
			if text[a-k:a] != y[len(y)-k:] {
				break
			}
			ys := text[a-k:a] + y[:len(y)-k]
			if it := starts[a-k]; it != nil && strings.HasSuffix(ys, " ") {
				p.front, p.src = it, normC(ys)
				return nil
			}
		}
	}
	var d *Node
	t := text
	base := 0 // where t stands in text
	off = 0
	for _, it := range p.items {
		ti := f.cached(it)
		if off <= a && b <= off+len(ti) && !(a == b && (a == off || b == off+len(ti))) {
			d, t, a, b, base = it, ti, a-off, b-off, off
			break
		}
		off += len(ti) + 1
	}
	if d == nil {
		p.src = newOf(text, a, b)
		return nil
	}
	for {
		moved := false
		for i, c := range d.Kids {
			if i == 0 && d.list {
				continue // a form's head is its token
			}
			item := f.e.Item(c) == c
			body := false
			if q, j := f.e.index(c); j >= 0 {
				pl := f.e.place(q, j)
				body = pl == placeBody || pl == placeOptional
			}
			var tc string
			var err error
			switch {
			case item || body:
				tc, err = clisp.PrintItems([]*clisp.Node{Lisp(c)})
			case f.isExpr(c, d):
				tc, err = clisp.PrintExpr(Lisp(c))
			default:
				continue
			}
			if err != nil {
				continue
			}
			tc = normC(tc)
			q := strings.Index(t, tc)
			if tc == "" || q < 0 || strings.Contains(t[q+1:], tc) {
				continue
			}
			if a < q || b > q+len(tc) || a == b && (a == q || b == q+len(tc)) {
				continue
			}
			nc := newOf(tc, a-q, b-q)
			if nc == "" && !item {
				continue
			}
			d, t, a, b, base = c, tc, a-q, b-q, base+q
			moved = true
			break
		}
		if !moved {
			break
		}
	}
	p.at = d
	p.src = newOf(t, a, b)
	if d == p.items[0] && len(p.items) == 1 {
		return nil
	}
	return f.placed(p, text, base, len(t))
}

// placed shows that the node narrowed to stands at text[at:at+n]: the run
// printed with it replaced by a marker is the run's C with the marker for
// those bytes.
func (f *substFinder) placed(p *substPlace, text string, at, n int) error {
	const mark = "__whim_subst_mark"
	var m *clisp.Node
	q, j := f.e.index(p.at)
	switch pl := f.e.place(q, j); {
	case q == f.e.top[0]:
		m = clisp.L(clisp.A("def"), clisp.A(mark), clisp.A("int"))
	case f.e.Item(p.at) == p.at:
		m = clisp.L(clisp.A("call"), clisp.A(mark))
	case pl == placeBody || pl == placeOptional:
		m = clisp.L(clisp.A("block"), clisp.L(clisp.A("call"), clisp.A(mark)))
	default:
		m = clisp.A(mark)
	}
	var mc string
	var err error
	if m.Atom != "" {
		mc, err = clisp.PrintExpr(m)
	} else {
		mc, err = clisp.PrintItems([]*clisp.Node{m})
	}
	if err != nil {
		return err
	}
	ls := make([]*clisp.Node, len(p.items))
	for i, it := range p.items {
		ls[i] = lispWith(it, p.at, m)
	}
	var c string
	if p.list == f.e.top[0] {
		b, perr := clisp.Print(ls)
		c, err = string(b), perr
	} else {
		c, err = clisp.PrintItems(ls)
	}
	if err != nil {
		return err
	}
	if squashC(normC(c)) != squashC(normC(text[:at]+" "+normC(mc)+" "+text[at+n:])) {
		return fmt.Errorf("the node #%d (%s) its change was narrowed to is not where the old C stands", p.at.ID, label(p.at))
	}
	return nil
}

// lispWith is n as C-lisp, with the node at as m.
func lispWith(n, at *Node, m *clisp.Node) *clisp.Node {
	if n == at {
		return m
	}
	if !n.list {
		return clisp.A(n.Atom)
	}
	kids := make([]*clisp.Node, len(n.Kids))
	for i, k := range n.Kids {
		kids[i] = lispWith(k, at, m)
	}
	return clisp.L(kids...)
}

// isExpr says c, a kid of d, is an expression in an expression's place:
// typed, or an operand atom of a typed expression.
func (f *substFinder) isExpr(c, d *Node) bool {
	if IsStatement(c) {
		return false
	}
	if c.Type != nil {
		return true
	}
	return !c.list && d.list && !IsStatement(d) && d.Type != nil
}

// declares says the top-level form x declares name.
func declares(x *Node, name string) bool {
	return DeclName(x) == name
}

// squashC is normalized C with the spaces gone, but between two words: two
// spellings of one token sequence (`f(a) , b` and `f(a), b`) are one.
func squashC(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' && i > 0 && i+1 < len(s) && identChar(s[i-1]) && identChar(s[i+1]) {
			b.WriteByte(c)
			continue
		}
		if c == ' ' {
			continue
		}
		if c == '"' || c == '\'' {
			k := skipLiteral(s, i)
			b.WriteString(s[i:k])
			i = k - 1
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// cached is an item's C as itemC has it already.
func (f *substFinder) cached(n *Node) string {
	if s, ok := f.text[n]; ok {
		return s
	}
	return f.forms[n]
}
