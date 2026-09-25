package togo

// clj_shape.go nests a lowered function's blocks as Clojure expressions
// (clj_fn.go's header says what the forms are).  Three things let a block
// with more than one way in be written once and in place:
//
//   - a JOIN of a branch -- the block where an if's or a switch's arms meet
//     again -- when every path from the branch reaches it and nothing else
//     (no return, no loop, no other block's way in) and the arms change at
//     most one variable it reads: (let [x (if test arm arm)] join), each
//     arm's value the x it leaves; with none, (do (if ...) join);
//   - a LOOP, where the function is structured: (loop [x x ...] head) over
//     the variables it changes that its head reads, each back edge a recur;
//     the code after it where it leaves (its exits, when each is the one
//     way into what follows), or, when a loop around it could be reached
//     from there, after the loop as a statement, (do (loop ...) after), if
//     it leaves to one place and changes nothing that place reads;
//   - a small block that returns, written at each way into it.
//
// What none of these nests makes the function a state machine, in which
// every block written by none of them is a state; the joins are still
// written in place there.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// shaper nests one function's blocks.
type shaper struct {
	f      *cfn
	live   map[*lblock]map[*lvar]bool
	header map[*lblock]bool
	edges  map[*lblock]int // incoming edges
	dup    map[*lblock]bool
	idom   map[*lblock]*lblock
	// the joins written in place: join -> its branch, its value; and the
	// branch's join
	owner   map[*lblock]*lblock
	value   map[*lblock]*lvar
	joinOf  map[*lblock]*lblock
	loops   map[*lblock]*sloop
	byName  map[string]*lvar
	machine bool // states: a jump to one is a recur
	states  []*lblock
	state   map[*lblock]int
	vars    []*lvar // what a state machine's recur carries
	// a split machine's
	splitting bool
	slot      map[*lvar]int
	group     []int
	cur       int
	pre       string // the groups' functions
	all       bool   // every block is a state
	why       string // why it is a state machine
}

// sloop is a natural loop: its head, the blocks it holds, and the
// variables it changes that the head reads.
type sloop struct {
	head  *lblock
	body  map[*lblock]bool
	vars  []*lvar
	exits []*lblock
	ok    bool // one way in from outside, and every back edge's source in it
}

// sctx is where a block is written: in a join's region (an edge to stop
// is the region's value) and inside structured loops (innermost last).
type sctx struct {
	stop   *lblock
	val    *lvar
	region bool // stop is a join's: the code is a let's value, where no recur is
	loops  []*sloop
}

// errShape is why a function cannot be structured: it is a state machine.
type errShape struct{ why string }

// structure nests the blocks: the body, how it was written, and the
// functions written before it (a split machine's groups).
func (f *cfn) structure() (string, string, string) {
	lf := f.lf
	s := &shaper{f: f, live: lf.live()}
	s.analyze()
	if body, ok := s.structured(); ok {
		return s.entryLets(body), "structured", ""
	}
	body := s.machineBody()
	if s.cost(body) > f.c.splitAt() {
		b := s.split()
		f.why = s.why
		return b, "split", s.pre
	}
	f.why = s.why
	return s.entryLets(body), "machine", ""
}

func (s *shaper) analyze() {
	f := s.f
	s.header = map[*lblock]bool{}
	s.edges = map[*lblock]int{}
	s.dup = map[*lblock]bool{}
	s.byName = map[string]*lvar{}
	for _, v := range f.lf.vars {
		s.byName[v.name] = v
	}
	for _, b := range f.lf.blocks {
		for _, t := range b.term.to {
			s.edges[t]++
			if t.id <= b.id {
				s.header[t] = true
			}
		}
	}
	for _, b := range f.lf.blocks {
		if s.header[b] || b.id == 0 || s.edges[b] < 2 {
			continue
		}
		// a small block that returns is written at each edge
		if len(b.steps) <= 2 && (b.term.kind == tRet || b.term.kind == tFall) && !s.hasLiteral(b) {
			s.dup[b] = true
		}
	}
	s.dominators()
	s.findLoops()
	s.findJoins()
}

// dominators computes each block's immediate dominator (Cooper, Harvey and
// Kennedy's iteration, on the reverse postorder).
func (s *shaper) dominators() {
	bs := s.f.lf.blocks
	s.idom = map[*lblock]*lblock{bs[0]: bs[0]}
	intersect := func(a, b *lblock) *lblock {
		for a != b {
			for a.id > b.id {
				a = s.idom[a]
			}
			for b.id > a.id {
				b = s.idom[b]
			}
		}
		return a
	}
	for changed := true; changed; {
		changed = false
		for _, b := range bs[1:] {
			var nd *lblock
			for _, p := range b.preds {
				if s.idom[p] == nil {
					continue
				}
				if nd == nil {
					nd = p
				} else {
					nd = intersect(p, nd)
				}
			}
			if nd != nil && s.idom[b] != nd {
				s.idom[b] = nd
				changed = true
			}
		}
	}
}

// dominates says a dominates b.
func (s *shaper) dominates(a, b *lblock) bool {
	for {
		if b == a {
			return true
		}
		d := s.idom[b]
		if d == b || d == nil {
			return false
		}
		b = d
	}
}

// findLoops finds each head's natural loop.
func (s *shaper) findLoops() {
	f := s.f
	s.loops = map[*lblock]*sloop{}
	for _, h := range f.lf.blocks {
		if !s.header[h] {
			continue
		}
		l := &sloop{head: h, body: map[*lblock]bool{h: true}, ok: true}
		var stack []*lblock
		for _, p := range h.preds {
			if p.id >= h.id {
				if !s.dominates(h, p) {
					l.ok = false // a way into the loop past its head: irreducible
				}
				if !l.body[p] {
					l.body[p] = true
					stack = append(stack, p)
				}
			}
		}
		for len(stack) > 0 {
			b := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if b == h {
				continue
			}
			for _, p := range b.preds {
				if !l.body[p] {
					l.body[p] = true
					stack = append(stack, p)
				}
			}
		}
		ins := 0
		for _, p := range h.preds {
			if !l.body[p] {
				ins++
			}
		}
		if ins != 1 && !(ins == 0 && h.id == 0) {
			l.ok = false
		}
		changed := map[*lvar]bool{}
		seen := map[*lblock]bool{}
		for b := range l.body {
			for _, st := range b.steps {
				for _, v := range f.lf.stepWrites(st) {
					if f.rebindable(v) && s.live[h][v] {
						changed[v] = true
					}
				}
			}
			for _, t := range b.term.to {
				if !l.body[t] && !seen[t] {
					seen[t] = true
					l.exits = append(l.exits, t)
				}
			}
		}
		sort.Slice(l.exits, func(a, b int) bool { return l.exits[a].id < l.exits[b].id })
		l.vars = f.lf.sortedVars(changed)
		s.loops[h] = l
	}
}

// findJoins decides, inner joins first, which joins are written in place.
func (s *shaper) findJoins() {
	f := s.f
	s.owner = map[*lblock]*lblock{}
	s.value = map[*lblock]*lvar{}
	s.joinOf = map[*lblock]*lblock{}
	for _, j := range f.lf.blocks {
		if j.id == 0 || s.header[j] || s.dup[j] || s.edges[j] < 2 {
			continue
		}
		d := s.idom[j]
		if d == nil || d == j || s.joinOf[d] != nil || (d.term.kind != tIf && d.term.kind != tSwitch) {
			continue
		}
		// the region: what d's arms reach before j
		region := map[*lblock]bool{}
		var stack []*lblock
		ok := true
		for _, t := range d.term.to {
			if t != j && !region[t] {
				region[t] = true
				stack = append(stack, t)
			}
		}
		for len(stack) > 0 && ok {
			b := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if b == d || s.header[b] || !s.dominates(d, b) || b.term.kind == tRet || b.term.kind == tFall {
				ok = false
				break
			}
			if s.edges[b] > 1 && s.owner[b] == nil {
				ok = false // a join inside not written in place
				break
			}
			for _, t := range b.term.to {
				if t == j {
					continue
				}
				if !region[t] {
					region[t] = true
					stack = append(stack, t)
				}
			}
		}
		if !ok {
			continue
		}
		for _, p := range j.preds {
			if p != d && !region[p] {
				ok = false
			}
		}
		for b := range region {
			if o := s.owner[b]; o != nil && !region[o] && o != d {
				ok = false
			}
		}
		if !ok {
			continue
		}
		changed := map[*lvar]bool{}
		for b := range region {
			for _, st := range b.steps {
				for _, v := range f.lf.stepWrites(st) {
					if f.rebindable(v) && s.live[j][v] {
						changed[v] = true
					}
				}
			}
		}
		if len(changed) > 1 {
			continue
		}
		s.owner[j] = d
		s.joinOf[d] = j
		for v := range changed {
			s.value[j] = v
		}
	}
}

// hasLiteral says a block's text holds a string literal: the suite's
// control needs " INSERT" once, and a copy would make it twice.
func (s *shaper) hasLiteral(b *lblock) bool {
	for _, st := range s.f.steps[b] {
		if strings.Contains(st.form, "(BytePtr/lit ") {
			return true
		}
	}
	return strings.Contains(s.f.terms[b].test, "(BytePtr/lit ")
}

// inline says a block is written where the edge to it is.
func (s *shaper) inline(b *lblock) bool {
	if b.id == 0 || s.header[b] || s.all || s.owner[b] != nil {
		return false
	}
	return s.edges[b] == 1 || s.dup[b]
}

// structured is the body when the blocks nest with no state machine.
func (s *shaper) structured() (body string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			e, is := r.(errShape)
			if !is {
				panic(r)
			}
			s.why = regexp.MustCompile(`block \d+ with \d+`).ReplaceAllString(e.why, "a block with")
			body, ok = "", false
		}
	}()
	for _, l := range s.loops {
		if !l.ok {
			panic(errShape{"an irreducible loop"})
		}
	}
	if l := s.loops[s.f.lf.blocks[0]]; l != nil {
		return s.loopForm(l, &sctx{}), true // a function that starts with a loop
	}
	return s.emit(s.f.lf.blocks[0], &sctx{}), true
}

// machineBody is the body as a state machine.
func (s *shaper) machineBody() string {
	f := s.f
	s.machine = true
	s.state = map[*lblock]int{}
	for _, b := range f.lf.blocks {
		if b.id == 0 || !s.inline(b) && s.owner[b] == nil {
			s.state[b] = len(s.states)
			s.states = append(s.states, b)
		}
	}
	carried := map[*lvar]bool{}
	for _, b := range s.states {
		for v := range s.live[b] {
			if f.rebindable(v) {
				carried[v] = true
			}
		}
	}
	s.vars = f.lf.sortedVars(carried)
	var binds []string
	binds = append(binds, "st 0")
	for _, v := range s.vars {
		binds = append(binds, f.bindName(v)+" "+s.initial(v))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "(loop [%s]\n  (case st\n", strings.Join(binds, "\n       "))
	for i, st := range s.states {
		fmt.Fprintf(&b, "    %d\n%s\n", i, indent(s.emit(st, &sctx{}), 4))
	}
	fmt.Fprintf(&b, "    (throw (IllegalStateException. \"no state\"))))")
	return b.String()
}

// initial is a carried variable's value as the machine starts: a
// parameter's, or its zero.
func (s *shaper) initial(v *lvar) string {
	if v.param {
		return v.name
	}
	if s.live[s.f.lf.blocks[0]][v] {
		return v.name // bound at the start: C may read it before it is set
	}
	return s.f.zeroVal(v)
}

// entryLets binds, before body, the variables C may read before it sets
// them: Java's zero.
func (s *shaper) entryLets(body string) string {
	f := s.f
	var bs []cbind
	for _, v := range f.lf.sortedVars(s.live[f.lf.blocks[0]]) {
		if v.param || !f.rebindable(v) {
			continue
		}
		bs = append(bs, cbind{f.bindName(v), f.zeroVal(v)})
	}
	if len(bs) == 0 {
		return body
	}
	return "(let [" + bindText(bs, "\n      ") + "]\n" + indent(body, 2) + ")"
}

// emit is block b and what it nests, as one expression.
func (s *shaper) emit(b *lblock, ctx *sctx) string {
	if ctx == nil {
		ctx = &sctx{}
	}
	f := s.f
	term := s.emitTerm(b, ctx)
	binds := f.steps[b]
	if len(binds) == 0 {
		return term
	}
	var bs []string
	for _, bd := range binds {
		name := bd.name
		if name != "_" {
			if v := s.byName[name]; v != nil {
				name = f.bindName(v)
			}
		}
		bs = append(bs, name+" "+bd.form)
	}
	return "(let [" + strings.Join(bs, "\n      ") + "]\n" + indent(term, 2) + ")"
}

func (s *shaper) emitTerm(b *lblock, ctx *sctx) string {
	f := s.f
	if j := s.joinOf[b]; j != nil && !s.all {
		// the branch, its arms ending at the join; then the join
		v := s.value[j]
		inner := s.emitBranch(b, &sctx{stop: j, val: v, region: true, loops: ctx.loops})
		rest := s.emit(j, ctx)
		if v == nil {
			return "(do " + indent(inner, 4)[4:] + "\n  " + indent(rest, 2)[2:] + ")"
		}
		return "(let [" + f.bindName(v) + " " + indent(inner, 6+len(f.bindName(v)))[6+len(f.bindName(v)):] + "]\n" + indent(rest, 2) + ")"
	}
	return s.emitBranch(b, ctx)
}

// emitBranch is b's terminator, each way out an edge.
func (s *shaper) emitBranch(b *lblock, ctx *sctx) string {
	f := s.f
	t := f.terms[b]
	switch b.term.kind {
	case tRet:
		if s.splitting {
			if b.term.ret.isZero() {
				return "-1"
			}
			return "(do (aset fo__ 0 " + boxed(t.test, f.ret) + ")\n    -1)"
		}
		if ctx.stop != nil {
			panic(errShape{"a return where the code must reach a join or a loop's end"})
		}
		return t.test
	case tFall:
		return t.test
	case tGoto:
		return s.edge(b, b.term.to[0], ctx)
	case tIf:
		a, e := s.edge(b, b.term.to[0], ctx), s.edge(b, b.term.to[1], ctx)
		if t.swap {
			a, e = e, a
		}
		return "(if " + t.test + "\n  " + indent(a, 2)[2:] + "\n  " + indent(e, 2)[2:] + ")"
	case tSwitch:
		var sb strings.Builder
		fmt.Fprintf(&sb, "(case %s", t.test)
		for i, vs := range b.term.cases {
			var ks []string
			for _, v := range vs {
				ks = append(ks, fmt.Sprint(v))
			}
			sort.Slice(ks, func(a, c int) bool { return ks[a] < ks[c] })
			key := ks[0]
			if len(ks) > 1 {
				key = "(" + strings.Join(ks, " ") + ")"
			}
			fmt.Fprintf(&sb, "\n  %s\n%s", key, indent(s.edge(b, b.term.to[i], ctx), 4))
		}
		fmt.Fprintf(&sb, "\n%s)", indent(s.edge(b, b.term.to[len(b.term.to)-1], ctx), 2))
		return sb.String()
	}
	return "nil"
}

// edge is the jump from b to t.
func (s *shaper) edge(from, t *lblock, ctx *sctx) string {
	f := s.f
	if ctx.stop != nil && t == ctx.stop {
		if ctx.val != nil {
			return ctx.val.name
		}
		return "nil"
	}
	if s.machine {
		if k, ok := s.state[t]; ok {
			if ctx.region {
				f.no(nil, "a jump to a state from inside a join's region")
			}
			parts := []string{"recur", fmt.Sprint(k)}
			for _, v := range s.vars {
				parts = append(parts, v.name)
			}
			return "(" + strings.Join(parts, " ") + ")"
		}
	} else if l := s.loops[t]; l != nil {
		if l.body[from] {
			// a back edge: to the innermost loop around, or no structure
			if len(ctx.loops) == 0 || ctx.loops[len(ctx.loops)-1] != l || ctx.region {
				panic(errShape{"a back edge to a loop that is not the innermost"})
			}
			parts := []string{"recur"}
			for _, v := range l.vars {
				parts = append(parts, v.name)
			}
			return "(" + strings.Join(parts, " ") + ")"
		}
		return s.loopForm(l, ctx)
	}
	if s.inline(t) {
		return s.emit(t, ctx)
	}
	panic(errShape{fmt.Sprintf("a jump to block %d with %d ways in: a join whose arms change more than one variable it reads, or leave it", t.id, s.edges[t])})
}

// loopForm is loop l entered: the code after it where it leaves, or after
// it as a statement.
func (s *shaper) loopForm(l *sloop, ctx *sctx) string {
	f := s.f
	var binds []string
	for _, v := range l.vars {
		binds = append(binds, f.bindName(v)+" "+v.name)
	}
	nested := ctx.region
	for _, o := range ctx.loops {
		if o.body[l.head] {
			nested = true
		}
	}
	tail := !nested
	for _, x := range l.exits {
		if !s.inline(x) {
			tail = false
		}
	}
	inner := &sctx{loops: append(append([]*sloop{}, ctx.loops...), l)}
	if tail {
		return "(loop [" + strings.Join(binds, "\n       ") + "]\n" + indent(s.emit(l.head, inner), 2) + ")"
	}
	// a statement: one place after it, which reads nothing it changes, and
	// no return inside
	if len(l.exits) != 1 {
		panic(errShape{"a loop left to more than one place"})
	}
	x := l.exits[0]
	for v := range s.live[x] {
		for _, lv := range l.vars {
			if lv == v {
				panic(errShape{"a loop whose changes are read after it"})
			}
		}
	}
	for b := range l.body {
		if b.term.kind == tRet || b.term.kind == tFall {
			panic(errShape{"a return inside a loop written as a statement"})
		}
		for _, st := range b.steps {
			for _, v := range f.lf.stepWrites(st) {
				if f.rebindable(v) && s.live[x][v] {
					panic(errShape{"a loop whose changes are read after it"})
				}
			}
		}
	}
	for _, p := range x.preds {
		if !l.body[p] {
			panic(errShape{"a loop's exit with another way in"})
		}
	}
	if s.header[x] || s.owner[x] != nil {
		panic(errShape{"a loop's exit that is a loop or a join"})
	}
	inner.stop, inner.val = x, nil
	loop := "(loop [" + strings.Join(binds, "\n       ") + "]\n" + indent(s.emit(l.head, inner), 2) + ")"
	return "(do " + indent(loop, 4)[4:] + "\n  " + indent(s.emit(x, ctx), 2)[2:] + ")"
}
