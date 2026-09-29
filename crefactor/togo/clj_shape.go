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
	"slices"
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
	owner  map[*lblock]*lblock
	value  map[*lblock][]*lvar   // what a join's arms change that it reads
	rets   map[*lblock]bool      // a join's region returns
	looped map[*lblock]bool      // a join's region holds a loop
	outs   map[*lblock][]*lblock // a join's region's other ways out: a break, a continue
	noOuts bool                  // no join with other ways out: the second try
	// the tuples' frame: a slot per variable a region hands out, in a long
	// array or an Object array, and the return value's
	tslot    map[*lvar]int
	ntl, nto int
	retSlot  int
	frame    bool // the frame is used
	region   map[*lblock]map[*lblock]bool
	joinOf   map[*lblock]*lblock
	loops    map[*lblock]*sloop
	byName   map[string]*lvar
	machine  bool // states: a jump to one is a recur
	states   []*lblock
	state    map[*lblock]int
	vars     []*lvar // what a state machine's recur carries
	// a split machine's
	splitting bool
	stores    []string // a split machine's state: what it writes back to the frame before a jump
	slot      map[*lvar]int
	group     []int
	cur       int
	pre       string // the groups' functions
	all       bool   // every block is a state
	why       string // why it is a state machine
	// outlined pieces: a region's code or a loop past olimit, written as a
	// function of its own that takes its variables through the frame
	outlining bool
	olimit    int
	opre      []string      // the pieces' functions
	fslot     map[*lvar]int // a fixed local's slot in to__, passed to a piece
}

// sloop is a natural loop: its head, the blocks it holds, and the
// variables it changes that the head reads.
type sloop struct {
	head  *lblock
	body  map[*lblock]bool
	vars  []*lvar
	exits []*lblock
	tail  map[*lblock]bool // blocks on a way out, written in place in the loop
	ok    bool             // one way in from outside, and every back edge's source in it
}

// sctx is where a block is written: in a join's region or a loop written
// as a value (an edge to one of stops is that text, the region's value),
// and inside structured loops (innermost last).
type sctx struct {
	stops  map[*lblock]string
	retTag bool // a return is the region's value [-1 v]
	region bool // the code is a let's value, where no recur is
	loops  []*sloop
	vt     string // the Java type of the code's value ("" at the top: the function's)
}

// errShape is why a function cannot be structured: it is a state machine.
type errShape struct{ why string }

// structure nests the blocks: the body, how it was written, and the
// functions written before it (a split machine's groups).
func (f *cfn) structure() (string, string, string) {
	lf := f.lf
	s := &shaper{f: f, live: lf.live()}
	s.analyze()
	n0 := f.ntmp // an attempt that fails leaves no names behind
	// structured, with a region's other ways out and then without: a join
	// taken with them can leave a way out no place can take, where without
	// it the function nests; and, when what nests is too large for one
	// method, again with its large pieces as functions of their own
	large := -1 // the try whose body nested but was too large
	first := "" // why the first try did not nest
	for try, noOuts := range []bool{false, true} {
		if noOuts {
			s.noOuts = true
			s.findJoins(true)
		}
		f.ntmp = n0
		s.resetFrame()
		body, ok := s.structured()
		if ok && s.cost(body) <= f.c.splitAt() {
			return s.entryLets(s.withFrame(body)), "structured", ""
		}
		if ok && large < 0 {
			large = try
		}
		if !ok && try == 0 {
			first = s.why
		}
	}
	why := "structured, too large for one method"
	if large < 0 {
		why = first
	} else {
		s.noOuts = large == 1
		s.findJoins(true)
		f.ntmp = n0
		s.resetFrame()
		s.outlining, s.olimit = true, f.c.outlineAt()
		body, ok := s.structured()
		if ok && s.cost(body) <= f.c.splitAt() {
			return s.entryLets(s.withFrame(body)), "structured", strings.Join(s.opre, "")
		}
		s.outlining, s.opre = false, nil
	}
	s.noOuts = false
	f.ntmp = n0
	// a state machine: a jump to a state -- a loop's head is one -- cannot
	// be inside a join's region, so the joins are found again without them
	s.findJoins(false)
	s.why = why
	s.resetFrame()
	body := s.machineBody()
	if s.cost(body) > f.c.splitAt() {
		b := s.split()
		f.why = s.why
		return b, "split", s.pre
	}
	f.why = s.why
	return s.entryLets(s.withFrame(body)), "machine", ""
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
		mergeCases(b)
	}
	for _, b := range f.lf.blocks {
		b.preds = nil
	}
	for _, b := range f.lf.blocks {
		for _, t := range b.term.to {
			t.preds = append(t.preds, b)
		}
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

// mergeCases makes a switch's cases one per block: values the lowering's
// later steps sent to one block (the empty blocks of `case 180: case 181:`
// retargeted to the one after them) are one case, (case x (180 181) ...),
// and values that go where the default goes are the default's -- so a block
// a switch reaches is one way in, written once.
func mergeCases(b *lblock) {
	if b.term.kind != tSwitch || len(b.term.to) == 0 {
		return
	}
	def := b.term.to[len(b.term.to)-1]
	var to []*lblock
	var cases [][]int64
	idx := map[*lblock]int{}
	for i, t := range b.term.to[:len(b.term.to)-1] {
		if t == def {
			continue
		}
		k, ok := idx[t]
		if !ok {
			k = len(to)
			idx[t] = k
			to = append(to, t)
			cases = append(cases, nil)
		}
		cases[k] = append(cases[k], b.term.cases[i]...)
	}
	b.term.to = append(to, def)
	b.term.cases = cases
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
		// an exit that is only the way out to another exit -- one way in,
		// every successor an exit (a break's `status = FAIL; break;`) -- is
		// written in place inside the loop, on its way out: the loop's exit is
		// where the ways out meet
		l.tail = map[*lblock]bool{}
		for changed := true; changed; {
			changed = false
			isExit := map[*lblock]bool{}
			for _, x := range l.exits {
				isExit[x] = true
			}
			for i, x := range l.exits {
				if s.edges[x] != 1 || s.header[x] || x.id == 0 || len(x.term.to) == 0 {
					continue
				}
				all := true
				for _, t := range x.term.to {
					if !isExit[t] || t == x {
						all = false
					}
				}
				if !all {
					continue
				}
				l.tail[x] = true
				l.exits = append(l.exits[:i:i], l.exits[i+1:]...)
				changed = true
				break
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
	s.outs = map[*lblock][]*lblock{}
	s.region = map[*lblock]map[*lblock]bool{}
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
		// a way out that is not the join -- to a block the branch does not
		// dominate: a break out of a switch, a continue of a loop around --
		// is the region's value too, tagged, and taken after it
		outs := map[*lblock]bool{}
		// the loops the branch is in: leaving one is a way out, a break
		var around []*sloop
		for _, l := range s.loops {
			if l.body[d] || l.tail[d] {
				around = append(around, l)
			}
		}
		leaves := func(t *lblock) bool {
			for _, l := range around {
				if !l.body[t] && !l.tail[t] {
					return true
				}
			}
			return false
		}
		enter := func(t *lblock) {
			switch {
			case t == j || region[t] || outs[t]:
			case t == d:
				ok = false // back to the branch: a loop, not a join
			case !s.dominates(d, t) || leaves(t):
				outs[t] = true
			default:
				region[t] = true
				stack = append(stack, t)
			}
		}
		for _, t := range d.term.to {
			enter(t)
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
			} else if s.edges[b] > 1 && s.owner[b] == nil && !s.dup[b] && !(loops && s.loopExit(b) != nil) {
				ok = false // a join inside not written in place
				break
			}
			if b.term.kind == tRet || b.term.kind == tFall {
				rets = true // written as the region's value [-1 v]
			}
			for _, t := range b.term.to {
				enter(t)
			}
		}
		if !ok {
			continue
		}
		// a loop's exit is in the region only with its loop: written after
		// the loop, which the region holds
		for b := range region {
			if s.edges[b] > 1 && s.owner[b] == nil && !s.dup[b] && !s.header[b] {
				if l := s.loopExit(b); l == nil || !region[l.head] {
					ok = false
				}
			}
		}
		if (!loops || s.noOuts) && len(outs) > 0 {
			ok = false // a state machine's: its states take the jumps as they are
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
		var os []*lblock
		for o := range outs {
			os = append(os, o)
		}
		sort.Slice(os, func(a, b int) bool { return os[a].id < os[b].id })
		s.outs[j] = os
		s.region[j] = region
	}
}

// writtenIn is the rebindable variables the blocks write.
func (s *shaper) writtenIn(blocks map[*lblock]bool) map[*lvar]bool {
	out := map[*lvar]bool{}
	for b := range blocks {
		for _, st := range b.steps {
			for _, v := range s.f.lf.stepWrites(st) {
				if s.f.rebindable(v) {
					out[v] = true
				}
			}
		}
	}
	return out
}

// loopExit is the loop b is where a loop that nests is left: an exit of it
// whose every way in is from the loop -- written after the loop, as its
// value's continuation (loopForm), not a join -- or nil.  Of two loops b
// leaves, the inner one: a block is the exit of each loop it ends.
func (s *shaper) loopExit(b *lblock) *sloop {
	var in *sloop
	for _, l := range s.loops {
		if !l.ok || !slices.Contains(l.exits, b) {
			continue
		}
		whole := true
		for _, p := range b.preds {
			if !l.body[p] && !l.tail[p] {
				whole = false
			}
		}
		if !whole {
			return nil
		}
		if in == nil || len(l.body) < len(in.body) || len(l.body) == len(in.body) && l.head.id < in.head.id {
			in = l
		}
	}
	return in
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
		outs := s.outs[j]
		if !s.rets[j] && len(vs) <= 1 && len(outs) == 0 {
			val := "nil"
			if len(vs) == 1 {
				val = vs[0].name
			}
			vt := "void"
			if len(vs) == 1 {
				vt = f.vars[vs[0]].jt
			}
			inner := s.emitBranch(b, &sctx{stops: map[*lblock]string{j: val}, region: true, loops: ctx.loops, vt: vt})
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
		// a tuple: [0 x y] at the join, [k ...] at its k-th other way out,
		// [-1 v] where the region returns
		tag := ""
		if s.rets[j] || len(outs) > 0 {
			tag = "0"
		}
		stops := map[*lblock]string{j: s.tuple(tag, vs)}
		conts := []*lblock{j}
		per := [][]*lvar{vs}
		for k, o := range outs {
			var ovs []*lvar
			for _, v := range f.lf.sortedVars(s.writtenIn(s.region[j])) {
				if s.live[o][v] {
					ovs = append(ovs, v)
				}
			}
			stops[o] = s.tuple(fmt.Sprint(k+1), ovs)
			conts = append(conts, o)
			per = append(per, ovs)
		}
		inner := s.emitBranch(b, &sctx{stops: stops, retTag: s.rets[j], region: true, loops: ctx.loops, vt: "long"})
		r := f.tmpName("j")
		return s.unpackFrom(r, inner, b, conts, per, s.rets[j], ctx)
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

// tuple is a region's value: the variables it hands out written to the
// frame, then its tag -- a primitive long, 0 where there is no choice to
// make after it.  (A vector of them was made and taken apart at every
// crossing, and boxed its numbers: in regmatch's opcode loop, every regex
// operation.)
func (s *shaper) tuple(tag string, vs []*lvar) string {
	if tag == "" {
		tag = "0"
	}
	var ws []string
	for _, v := range vs {
		ws = append(ws, s.frameStore(v))
	}
	if len(ws) == 0 {
		return tag
	}
	return "(do " + strings.Join(ws, "\n    ") + "\n    " + tag + ")"
}

// frameSlot is v's slot in the frame, given the first time it is asked.
func (s *shaper) frameSlot(v *lvar) int {
	s.frame = true
	if k, ok := s.tslot[v]; ok {
		return k
	}
	var k int
	if isIntJ(s.f.vars[v].jt) {
		k = s.ntl
		s.ntl++
	} else {
		k = s.nto
		s.nto++
	}
	s.tslot[v] = k
	return k
}

// frameStore writes v to its slot; frameLoad reads it back as v's type.
func (s *shaper) frameStore(v *lvar) string {
	k := s.frameSlot(v)
	if isIntJ(s.f.vars[v].jt) {
		return fmt.Sprintf("(aset tl__ %d %s)", k, v.name)
	}
	return fmt.Sprintf("(aset to__ %d %s)", k, boxed(v.name, s.f.vars[v].jt))
}

func (s *shaper) frameLoad(v *lvar) string {
	k := s.frameSlot(v)
	switch jt := s.f.vars[v].jt; {
	case isIntJ(jt):
		return fmt.Sprintf("(aget tl__ %d)", k)
	case jt == "boolean":
		return fmt.Sprintf("(boolean (aget to__ %d))", k)
	}
	return fmt.Sprintf("(aget to__ %d)", k)
}

// frameRet is the return value's slot, written and read.
func (s *shaper) frameRet(v string) (store, load string) {
	f := s.f
	if s.retSlot < 0 {
		s.frame = true
		if isIntJ(f.ret) {
			s.retSlot = s.ntl
			s.ntl++
		} else if f.ret != "void" {
			s.retSlot = s.nto
			s.nto++
		} else {
			s.retSlot = 0
		}
	}
	switch {
	case f.ret == "void":
		return "", "nil"
	case isIntJ(f.ret):
		return fmt.Sprintf("(aset tl__ %d %s)", s.retSlot, v), fmt.Sprintf("(aget tl__ %d)", s.retSlot)
	case f.ret == "boolean":
		return fmt.Sprintf("(aset to__ %d %s)", s.retSlot, boxed(v, f.ret)), fmt.Sprintf("(boolean (aget to__ %d))", s.retSlot)
	}
	return fmt.Sprintf("(aset to__ %d %s)", s.retSlot, v), fmt.Sprintf("(aget to__ %d)", s.retSlot)
}

// resetFrame starts a function's frame, or a group's, empty.
func (s *shaper) resetFrame() {
	s.tslot = map[*lvar]int{}
	s.fslot = map[*lvar]int{}
	s.ntl, s.nto, s.retSlot, s.frame = 0, 0, -1, false
}

// withFrame binds the frame around body when body uses it.
func (s *shaper) withFrame(body string) string {
	if !s.frame {
		return body
	}
	var arrays []string
	if len(s.opre) > 0 {
		// the pieces take both
		s.ntl, s.nto = max(s.ntl, 1), max(s.nto, 1)
	}
	if s.ntl > 0 {
		arrays = append(arrays, fmt.Sprintf("^longs tl__ (long-array %d)", s.ntl))
	}
	if s.nto > 0 {
		arrays = append(arrays, fmt.Sprintf("^objects to__ (object-array %d)", s.nto))
	}
	return "(let [" + strings.Join(arrays, "\n      ") + "]\n" + indent(body, 2) + ")"
}

// unpack is the code after a region whose value, value, is a tuple bound
// to r: at tag k (none when there is one way on and no return), the
// variables vs[k] taken out of it and the code at conts[k] -- or, at tag -1, the function's return of what it holds, as a
// return is where the region is (ret).
func (s *shaper) unpack(r, value string, conts []*lblock, vs [][]*lvar, ret bool, ctx *sctx) string {
	return s.unpackFrom(r, value, nil, conts, vs, ret, ctx)
}

// unpackFrom is unpack for a join's region: conts[0] is the join, written
// in place, and the others its region's other ways out, each taken as a jump
// from the branch, from, is taken where the join is (edge).
func (s *shaper) unpackFrom(r, value string, from *lblock, conts []*lblock, vs [][]*lvar, ret bool, ctx *sctx) string {
	f := s.f
	_, retLoad := s.frameRet("")
	at := func(k int) string {
		x := conts[k]
		var binds []string
		for _, v := range vs[k] {
			binds = append(binds, f.bindName(v)+" "+s.frameLoad(v))
		}
		body, ok := ctx.stops[x] // the end of the region around: its value
		switch {
		case ok:
		case from != nil && k > 0:
			body = s.edge(from, x, ctx) // a way out of the region, taken
		default:
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
		tail = "(if (== (long " + r + ") -1)\n  " + indent(s.retOf(ctx, retLoad), 2)[2:] +
			"\n  " + indent(at(0), 2)[2:] + ")"
	default:
		var cb strings.Builder
		fmt.Fprintf(&cb, "(case (long %s)", r)
		for k := range conts {
			fmt.Fprintf(&cb, "\n  %d\n%s", k, indent(at(k), 4))
		}
		if ret {
			fmt.Fprintf(&cb, "\n  %s", indent(s.retOf(ctx, retLoad), 2)[2:])
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
		store, _ := s.frameRet(v)
		if store == "" {
			if v == "nil" || v == "" {
				return "-1"
			}
			return "(do " + v + "\n    -1)" // a void function's: what it does, then the tag
		}
		return "(do " + store + "\n    -1)"
	}
	if len(ctx.stops) > 0 || ctx.region {
		panic(errShape{"a return where the code must reach a join or a loop's end"})
	}
	if s.splitting {
		// a split machine's group returns the next state, -1 when the
		// function returns: its value in the frame (as emitBranch writes a
		// return), not the value itself -- a void function's nil would be
		// taken for a state
		if v == "nil" || v == "" || s.f.ret == "void" {
			if v == "nil" || v == "" {
				return "-1"
			}
			return "(do " + v + "\n    -1)"
		}
		return "(do (aset fo__ 0 " + boxed(v, s.f.ret) + ")\n    -1)"
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
		// Clojure's case writes an arm's body once per key: an arm of several
		// keys is chosen by an index, (case (case x (1 2) 0 (3 4) 1 -1) 0 ...),
		// so each body is written once
		grouped := false
		for _, vs := range b.term.cases {
			grouped = grouped || len(vs) > 1
		}
		// the arms; when together they are past olimit, each large one a
		// function of its own (a switch of many small arms is past it too)
		arms := make([]string, len(b.term.to))
		total := 0
		for i, t := range b.term.to {
			arms[i] = s.edge(b, t, ctx)
			total += len(arms[i])
		}
		if s.outlining && total > s.olimit && (ctx.region || len(ctx.loops) == 0 && len(ctx.stops) == 0) {
			total = 0
			for i, t := range b.term.to {
				if _, stop := ctx.stops[t]; !stop {
					arms[i] = s.outlineAt(arms[i], s.live[t], nil, s.valueType(ctx), s.olimit/2)
				}
				total += len(arms[i])
			}
			if total > s.olimit {
				return s.switchGroups(b, arms, ctx)
			}
		}
		var sb strings.Builder
		keyOf := func(vs []int64) string {
			var ks []string
			for _, v := range vs {
				ks = append(ks, fmt.Sprint(v))
			}
			sort.Slice(ks, func(a, c int) bool { return ks[a] < ks[c] })
			if len(ks) > 1 {
				return "(" + strings.Join(ks, " ") + ")"
			}
			return ks[0]
		}
		if grouped {
			var ib strings.Builder
			fmt.Fprintf(&ib, "(case %s", t.test)
			for i, vs := range b.term.cases {
				fmt.Fprintf(&ib, " %s %d", keyOf(vs), i)
			}
			ib.WriteString(" -1)")
			fmt.Fprintf(&sb, "(case %s", ib.String())
			for i := range b.term.cases {
				fmt.Fprintf(&sb, "\n  %d\n%s", i, indent(arms[i], 4))
			}
		} else {
			fmt.Fprintf(&sb, "(case %s", t.test)
			for i, vs := range b.term.cases {
				fmt.Fprintf(&sb, "\n  %s\n%s", keyOf(vs), indent(arms[i], 4))
			}
		}
		fmt.Fprintf(&sb, "\n%s)", indent(arms[len(arms)-1], 2))
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
			if s.splitting {
				// the frame holds the variables: the state's writes, then the jump
				if len(s.stores) == 0 {
					return fmt.Sprintf("(recur %d)", k)
				}
				return "(do " + strings.Join(s.stores, "\n    ") + fmt.Sprintf("\n    (recur %d))", k)
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
		if ctx.region || len(ctx.loops) == 0 && len(ctx.stops) == 0 {
			// no recur in it to a loop around: it may be a function of its own
			return s.outline(s.emit(t, ctx), s.live[t], nil, s.valueType(ctx))
		}
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
	// the loop, as a function of its own when it is large: it recurs to
	// itself alone, and its variables' first values are what its head reads
	loopOf := func() string {
		return s.outline("(loop ["+strings.Join(binds, "\n       ")+"]\n"+indent(s.emit(l.head, inner), 2)+")", s.live[l.head], l.vars, inner.vt)
	}
	if tail {
		inner.vt = s.valueType(ctx)
		return loopOf()
	}
	// after the loop: each exit the one way into what follows it, and
	// neither a loop nor a join -- or the end of the region the loop is in
	for _, x := range l.exits {
		if _, ok := ctx.stops[x]; ok {
			continue
		}
		for _, p := range x.preds {
			if !l.body[p] && !l.tail[p] {
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
	region := map[*lblock]bool{}
	for b := range l.body {
		region[b] = true
	}
	for b := range l.tail {
		region[b] = true // a way out, written in place in the loop
	}
	for b := range region {
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
		inner.vt = s.valueType(ctx)
		return loopOf()
	}
	if len(l.exits) == 1 && len(vs) == 0 && !rets {
		// a statement: (do (loop ...) after)
		inner.stops = map[*lblock]string{l.exits[0]: "nil"}
		inner.vt = "void"
		loop := loopOf()
		return "(do " + indent(loop, 4)[4:] + "\n  " + indent(s.emit(l.exits[0], ctx), 2)[2:] + ")"
	}
	// a value: [k x y] at its k-th exit, [-1 v] where the function returns
	inner.stops = map[*lblock]string{}
	for k, x := range l.exits {
		tag := fmt.Sprint(k)
		if len(l.exits) == 1 && !rets {
			tag = ""
		}
		inner.stops[x] = s.tuple(tag, per[k])
	}
	inner.retTag = rets
	inner.vt = "long"
	return s.unpack(f.tmpName("l"), loopOf(), l.exits, per, rets, ctx)
}

// valueType is the Java type of the value code in ctx yields: the region's
// or the loop's, or at the top the function's.
func (s *shaper) valueType(ctx *sctx) string {
	if ctx.vt != "" {
		return ctx.vt
	}
	return s.f.ret
}

// symbolRe is a Clojure symbol in the printed text.
var symbolRe = regexp.MustCompile(`[^\s()\[\]{}"^;,@~'` + "`" + `]+`)

// outline is text -- a region's code, or a loop, whose value is of Java
// type vt -- as a call of a function of its own when it is past olimit, so
// that no method is larger than the first JIT tier compiles.  The piece
// takes the editor and the frame; its variables go through the frame: each
// the text names that is live where it starts (live, and extra, the loop's
// first values) is stored before the call and read at the piece's start,
// and each fixed local it names -- a box, an object made at the start --
// likewise.  The frame's slots are the function's, so the tuples the piece
// hands out are where its caller reads them.
func (s *shaper) outline(text string, live map[*lvar]bool, extra []*lvar, vt string) string {
	return s.outlineAt(text, live, extra, vt, s.olimit)
}

// outlineAt is outline with a limit of its own: a switch's arm, when the
// switch is past olimit, is outlined past a smaller one.
func (s *shaper) outlineAt(text string, live map[*lvar]bool, extra []*lvar, vt string, limit int) string {
	if !s.outlining || s.cost(text) < limit {
		return text
	}
	return s.piece(text, live, extra, vt, nil, nil)
}

// piece is text as a call of a function of its own (outline): stores are
// the caller's writes to the frame before the call beside the variables',
// and loads the piece's reads of it after them.
func (s *shaper) piece(text string, live map[*lvar]bool, extra []*lvar, vt string, stores, loads []string) string {
	f := s.f
	named := map[string]bool{}
	for _, m := range symbolRe.FindAllString(text, -1) {
		named[m] = true
	}
	takes := map[*lvar]bool{}
	for _, v := range extra {
		takes[v] = true
	}
	var vstores, vloads []string
	for _, v := range f.lf.vars {
		if !named[v.name] {
			continue
		}
		cvr := f.vars[v]
		if !f.rebindable(v) {
			k, ok := s.fslot[v]
			if !ok {
				k = s.nto
				s.nto++
				s.fslot[v] = k
			}
			h := cljHint(cvr.jt)
			if cvr.boxed {
				h = cljHint(cvr.jt + "[]")
			}
			vstores = append(vstores, fmt.Sprintf("(aset to__ %d %s)", k, v.name))
			if h != "" {
				vloads = append(vloads, fmt.Sprintf("^%s %s (aget to__ %d)", h, v.name, k))
			} else {
				vloads = append(vloads, fmt.Sprintf("%s (aget to__ %d)", v.name, k))
			}
			continue
		}
		if !live[v] && !takes[v] {
			continue // bound in the text
		}
		vstores = append(vstores, s.frameStore(v))
		vloads = append(vloads, f.bindName(v)+" "+s.frameLoad(v))
	}
	stores = append(vstores, stores...)
	loads = append(vloads, loads...)
	s.frame = true
	name := fmt.Sprintf("%s__r%d", f.c.fnName(f.name), len(s.opre))
	ret, call := "", "("+name+" ed tl__ to__)"
	switch h := cljHint(vt); {
	case isIntJ(vt):
		ret = "^long "
	case vt == "boolean":
		call = "(boolean " + call + ")"
	case h != "":
		ret = "^" + h + " "
	}
	body := text
	if len(loads) > 0 {
		body = "(let [" + strings.Join(loads, "\n      ") + "]\n" + indent(text, 2) + ")"
	}
	s.opre = append(s.opre, fmt.Sprintf("(defn- %s %s[^Editor ed ^longs tl__ ^objects to__]\n%s)\n\n", name, ret, indent(body, 2)))
	if len(stores) == 0 {
		return call
	}
	return "(do " + strings.Join(stores, "\n    ") + "\n    " + call + ")"
}

// switchGroups is switch b, its arms (arms, in b's order, the default last)
// in groups of functions of their own: the arm's index computed, and each
// group a case on it, called for the indices it holds.  A switch of many
// small arms -- regmatch's over its opcodes -- is past what one method can
// hold as a whole.
func (s *shaper) switchGroups(b *lblock, arms []string, ctx *sctx) string {
	f := s.f
	t := f.terms[b]
	var ib strings.Builder
	fmt.Fprintf(&ib, "(case %s", t.test)
	for i, vs := range b.term.cases {
		var ks []string
		for _, v := range vs {
			ks = append(ks, fmt.Sprint(v))
		}
		sort.Strings(ks)
		key := ks[0]
		if len(ks) > 1 {
			key = "(" + strings.Join(ks, " ") + ")"
		}
		fmt.Fprintf(&ib, " %s %d", key, i)
	}
	fmt.Fprintf(&ib, " %d)", len(arms)-1)
	k := f.tmpName("k")
	s.frame = true
	slot := s.ntl
	s.ntl++
	type group struct{ from, to int }
	var gs []group
	size := 0
	for i := range arms {
		if len(gs) == 0 || size > 0 && size+len(arms[i]) > s.olimit/2 {
			gs = append(gs, group{i, i})
			size = 0
		}
		gs[len(gs)-1].to = i
		size += len(arms[i])
	}
	vt := s.valueType(ctx)
	calls := make([]string, len(gs))
	for g, gr := range gs {
		var cb strings.Builder
		live := map[*lvar]bool{}
		fmt.Fprintf(&cb, "(case %s", k)
		for i := gr.from; i <= gr.to; i++ {
			fmt.Fprintf(&cb, "\n  %d\n%s", i, indent(arms[i], 4))
			for v := range s.live[b.term.to[i]] {
				live[v] = true
			}
		}
		cb.WriteString(")")
		calls[g] = s.piece(cb.String(), live, nil, vt,
			[]string{fmt.Sprintf("(aset tl__ %d %s)", slot, k)}, []string{fmt.Sprintf("%s (aget tl__ %d)", k, slot)})
	}
	dispatch := calls[len(calls)-1]
	for g := len(gs) - 2; g >= 0; g-- {
		dispatch = fmt.Sprintf("(if (<= %s %d)\n  %s\n  %s)", k, gs[g].to, indent(calls[g], 2)[2:], indent(dispatch, 2)[2:])
	}
	return "(let [" + k + " " + ib.String() + "]\n" + indent(dispatch, 2) + ")"
}
