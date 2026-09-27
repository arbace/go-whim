package togo

// clj_shape.go nests a lowered function's blocks as Clojure expressions
// (clj_fn.go's header says what the forms are).  Three things let a block
// with more than one way in be written once and in place:
//
//   - a JOIN of a branch -- the block where an if's or a switch's arms meet
//     again -- when every path from the branch reaches it or returns, and
//     no other block's way in is inside: (let [x (if test arm arm)] join),
//     each arm's value the x it leaves; with none, (do (if ...) join); and
//     when the arms change several variables the join reads, or return, the
//     value is a tuple -- [0 x y] where the join is reached, [-1 v] where
//     the function returns v -- taken apart after the branch.  A loop inside
//     the region is written as a value (below);
//   - a LOOP, where the function is structured: (loop [x x ...] head) over
//     the variables it changes that its head reads, each back edge a recur;
//     the code after it where it leaves (its exits, when each is the one
//     way into what follows), or, when a loop around it could be reached
//     from there or it is inside a join's region, after the loop: (do (loop
//     ...) after) when it leaves to one place and changes nothing that place
//     reads, and otherwise the loop's value is a tuple -- [k x y] at its
//     k-th exit, [-1 v] where the function returns v -- and a case on k
//     goes on at that exit;
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
	value   map[*lblock][]*lvar // what a join's arms change that it reads
	rets    map[*lblock]bool    // a join's region returns
	looped  map[*lblock]bool    // a join's region holds a loop
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

// sctx is where a block is written: in a join's region or a loop written
// as a value (an edge to one of stops is that text, the region's value),
// and inside structured loops (innermost last).
type sctx struct {
	stops  map[*lblock]string
	retTag bool // a return is the region's value [-1 v]
	region bool // the code is a let's value, where no recur is
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
	// a state machine: a jump to a state -- a loop's head is one -- cannot
	// be inside a join's region, so the joins are found again without them
	why := s.why
	s.findJoins(false)
	s.why = why
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
	s.findJoins(true)
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

// findJoins decides, inner joins first, which joins are written in place;
// loops says a loop may be inside a region (not in a state machine, where
// its head is a state).
func (s *shaper) findJoins(loops bool) {
	f := s.f
	s.owner = map[*lblock]*lblock{}
	s.value = map[*lblock][]*lvar{}
	s.rets = map[*lblock]bool{}
	s.looped = map[*lblock]bool{}
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
		ok, rets, looped := true, false, false
		for _, t := range d.term.to {
			if t != j && !region[t] {
				region[t] = true
				stack = append(stack, t)
			}
		}
		for len(stack) > 0 && ok {
			b := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if b == d || !s.dominates(d, b) {
				ok = false
				break
			}
			if s.header[b] {
				// a loop inside: written as a value, whole, one way into it
				if l := s.loops[b]; !loops || l == nil || !l.ok {
					ok = false
					break
				}
				looped = true
			} else if s.edges[b] > 1 && s.owner[b] == nil && !s.dup[b] {
				ok = false // a join inside not written in place
				break
			}
			if b.term.kind == tRet || b.term.kind == tFall {
				rets = true // written as the region's value [-1 v]
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
		s.owner[j] = d
		s.joinOf[d] = j
		s.value[j] = f.lf.sortedVars(changed)
		s.rets[j] = rets
		s.looped[j] = looped
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
		vs := s.value[j]
		if !s.rets[j] && len(vs) <= 1 {
			val := "nil"
			if len(vs) == 1 {
				val = vs[0].name
			}
			inner := s.emitBranch(b, &sctx{stops: map[*lblock]string{j: val}, region: true, loops: ctx.loops})
			rest := s.emit(j, ctx)
			if len(vs) == 0 {
				return "(do " + indent(inner, 4)[4:] + "\n  " + indent(rest, 2)[2:] + ")"
			}
			v := vs[0]
			if s.looped[j] {
				// a loop's value is an Object: the variable's primitive again
				inner = s.unboxed(inner, f.vars[v].jt)
			}
			return "(let [" + f.bindName(v) + " " + indent(inner, 6+len(f.bindName(v)))[6+len(f.bindName(v)):] + "]\n" + indent(rest, 2) + ")"
		}
		// a tuple: [0 x y] at the join, [-1 v] where the region returns
		tag := ""
		if s.rets[j] {
			tag = "0"
		}
		inner := s.emitBranch(b, &sctx{stops: map[*lblock]string{j: tuple(tag, vs)}, retTag: s.rets[j], region: true, loops: ctx.loops})
		r := f.tmpName("j")
		return s.unpack(r, inner, []*lblock{j}, [][]*lvar{vs}, s.rets[j], ctx)
	}
	return s.emitBranch(b, ctx)
}

// primBool is v as a loop's binding or a recur's argument: a boolean as a
// primitive, since a step may bind the literal true, which Clojure holds
// as a Boolean, and a recur may not pass one to a primitive local.
func (s *shaper) primBool(v *lvar) string {
	if s.f.vars[v].jt == "boolean" {
		return "(boolean " + v.name + ")"
	}
	return v.name
}

// tuple is a region's value: a tag, when there is a choice to make after
// it, then variables' values.
func tuple(tag string, vs []*lvar) string {
	var parts []string
	if tag != "" {
		parts = append(parts, tag)
	}
	for _, v := range vs {
		parts = append(parts, v.name)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// unpack is the code after a region whose value, value, is a tuple bound
// to r: at tag k (none when there is one way on and no return), the
// variables vs[k] taken out of it and the code at conts[k] -- or, at tag -1, the function's return of what it holds, as a
// return is where the region is (ret).
func (s *shaper) unpack(r, value string, conts []*lblock, vs [][]*lvar, ret bool, ctx *sctx) string {
	f := s.f
	first := 1 // past the tag
	if len(conts) == 1 && !ret {
		first = 0 // no tag: nothing to choose
	}
	at := func(k int) string {
		x := conts[k]
		var binds []string
		for i, v := range vs[k] {
			binds = append(binds, f.bindName(v)+" "+s.unboxed(fmt.Sprintf("(nth %s %d)", r, i+first), f.vars[v].jt))
		}
		body, ok := ctx.stops[x] // the end of the region around: its value
		if !ok {
			body = s.emit(x, ctx)
		}
		if len(binds) == 0 {
			return body
		}
		return "(let [" + strings.Join(binds, "\n      ") + "]\n" + indent(body, 2) + ")"
	}
	var b strings.Builder
	b.WriteString("(let [" + r + " " + indent(value, 6+len(r))[6+len(r):] + "]\n")
	var tail string
	switch {
	case len(conts) == 1 && !ret:
		tail = at(0)
	case len(conts) == 1:
		tail = "(if (== (long (nth " + r + " 0)) -1)\n  " + indent(s.retOf(ctx, s.unboxed("(nth "+r+" 1)", f.ret)), 2)[2:] +
			"\n  " + indent(at(0), 2)[2:] + ")"
	default:
		var cb strings.Builder
		fmt.Fprintf(&cb, "(case (long (nth %s 0))", r)
		for k := range conts {
			fmt.Fprintf(&cb, "\n  %d\n%s", k, indent(at(k), 4))
		}
		if ret {
			fmt.Fprintf(&cb, "\n  %s", indent(s.retOf(ctx, s.unboxed("(nth "+r+" 1)", f.ret)), 2)[2:])
		}
		cb.WriteString(")")
		tail = cb.String()
	}
	b.WriteString(indent(tail, 2) + ")")
	return b.String()
}

// unboxed is x, an element of a tuple, as a value of Java type jt.
func (s *shaper) unboxed(x, jt string) string {
	switch {
	case isIntJ(jt):
		return "(long " + x + ")"
	case jt == "boolean":
		return "(boolean " + x + ")"
	}
	return x
}

// retOf is the function's return of v where ctx is: v itself, or inside a
// region that can hand a return out, the region's value [-1 v].
func (s *shaper) retOf(ctx *sctx, v string) string {
	if ctx.retTag {
		return "[-1 " + v + "]"
	}
	if len(ctx.stops) > 0 || ctx.region {
		panic(errShape{"a return where the code must reach a join or a loop's end"})
	}
	return v
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
		return s.retOf(ctx, t.test)
	case tFall:
		if ctx.retTag || len(ctx.stops) > 0 || ctx.region {
			return s.retOf(ctx, t.test)
		}
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
	if v, ok := ctx.stops[t]; ok {
		return v
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
				parts = append(parts, s.primBool(v))
			}
			return "(" + strings.Join(parts, " ") + ")"
		}
		return s.loopForm(l, ctx)
	}
	if s.inline(t) {
		return s.emit(t, ctx)
	}
	panic(errShape{fmt.Sprintf("a jump to block %d with %d ways in: a join whose region is entered from outside it, or left elsewhere", t.id, s.edges[t])})
}

// loopForm is loop l entered: the code after it where it leaves, or after
// it as a statement.
func (s *shaper) loopForm(l *sloop, ctx *sctx) string {
	f := s.f
	var binds []string
	for _, v := range l.vars {
		binds = append(binds, f.bindName(v)+" "+s.primBool(v))
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
	// after the loop: each exit the one way into what follows it, and
	// neither a loop nor a join -- or the end of the region the loop is in
	for _, x := range l.exits {
		if _, ok := ctx.stops[x]; ok {
			continue
		}
		for _, p := range x.preds {
			if !l.body[p] {
				panic(errShape{"a loop's exit with another way in"})
			}
		}
		if s.header[x] || s.owner[x] != nil {
			panic(errShape{"a loop's exit that is a loop or a join"})
		}
	}
	// what it changes that an exit reads, and whether it returns
	out := map[*lvar]bool{}
	rets := false
	for b := range l.body {
		if b.term.kind == tRet || b.term.kind == tFall {
			rets = true
		}
		for _, st := range b.steps {
			for _, v := range f.lf.stepWrites(st) {
				if !f.rebindable(v) {
					continue
				}
				for _, x := range l.exits {
					if s.live[x][v] {
						out[v] = true
					}
				}
			}
		}
	}
	for _, v := range l.vars {
		for _, x := range l.exits {
			if s.live[x][v] {
				out[v] = true
			}
		}
	}
	vs := f.lf.sortedVars(out)
	// each exit's own: what the loop changes that it reads
	per := make([][]*lvar, len(l.exits))
	for k, x := range l.exits {
		for _, v := range vs {
			if s.live[x][v] {
				per[k] = append(per[k], v)
			}
		}
	}
	if _, ok := ctx.stops[l.exits[0]]; ok && len(l.exits) == 1 {
		// it leaves to the end of the region it is in: its value is the
		// region's, as that end's edge writes it
		inner.stops = map[*lblock]string{l.exits[0]: ctx.stops[l.exits[0]]}
		inner.retTag = ctx.retTag
		return "(loop [" + strings.Join(binds, "\n       ") + "]\n" + indent(s.emit(l.head, inner), 2) + ")"
	}
	if len(l.exits) == 1 && len(vs) == 0 && !rets {
		// a statement: (do (loop ...) after)
		inner.stops = map[*lblock]string{l.exits[0]: "nil"}
		loop := "(loop [" + strings.Join(binds, "\n       ") + "]\n" + indent(s.emit(l.head, inner), 2) + ")"
		return "(do " + indent(loop, 4)[4:] + "\n  " + indent(s.emit(l.exits[0], ctx), 2)[2:] + ")"
	}
	// a value: [k x y] at its k-th exit, [-1 v] where the function returns
	inner.stops = map[*lblock]string{}
	for k, x := range l.exits {
		tag := fmt.Sprint(k)
		if len(l.exits) == 1 && !rets {
			tag = ""
		}
		inner.stops[x] = tuple(tag, per[k])
	}
	inner.retTag = rets
	loop := "(loop [" + strings.Join(binds, "\n       ") + "]\n" + indent(s.emit(l.head, inner), 2) + ")"
	return s.unpack(f.tmpName("l"), loop, l.exits, per, rets, ctx)
}
