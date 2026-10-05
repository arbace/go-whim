package graph

import (
	"fmt"
	"sort"
	"strings"
)

// FOLDX: xform.FallOut's closure on the graph (doc/GRAPH-MIGRATION.md,
// FOLDX; crefactor/xform, the text closure, is deleted since Fin, and what
// it gave on foldx_test.go's samples and three snapshots is recorded as
// that test's and graphcheck's golden files).  What the closure's base rules (fallout.go) take is what a cut
// left dangling; what FoldX takes is what a cut left UNWRITTEN, and what
// that makes constant, in the text closure's rules and shapes, so that a
// front phase can end on the graph with the bytes its text closure gives:
//
//   - SEEDS.  Each read of an object or a member nothing writes (Unwritten)
//     is its value: an object's initialiser, zero for a member; a pointer's
//     `((void *)0)`, written only where it is tested.  A local initialised
//     with a value the closure wrote, and never written again, is that
//     value at each read.
//   - CONSTANTS.  An integer expression holding a value the closure wrote,
//     and constant with it, is its value (cc's evaluation, foldx_eval.go);
//     `a && K` and `a || K` lose the K where only the truth of the result is
//     read, and are K's value where K decides and a has no side effect
//     (Logic).
//   - BRANCHES.  An if, a while or a `?:` of such a constant is the branch
//     it takes: a braced branch standing alone in a block spliced into it
//     (kept a block where it declares), an if taking none gone, an else-if
//     taking none gone from its chain; a `?:` its branch in parentheses.
//   - JUMPS.  The statements after a jump the closure revealed, up to the
//     next label, go (Jumps).
//   - CALLS.  A static function only called, whose body is then `return K;`,
//     is K at each call whose arguments have no side effect (Returns); one
//     left with nothing to do is not called (Empty); a parameter every call
//     passes one value the closure wrote is that value in the body, and
//     goes from the definition, its prototypes and the calls (Params).
//
// The rules after the seeds fire only on what holds a value the closure
// wrote or a branch it took -- the text's enumerator marks and comments,
// here the values' nodes and MARK items, which the closure removes when it
// is done -- so code the cut did not touch is left as it is.  A round finds
// every rewrite first, outermost first, none inside another, and then makes
// them: the text's rounds, so that the same rewrites happen in the same
// round and the bytes are the text's.

// FoldX is what FoldX is told.
type FoldX struct {
	// Roots are where the live code starts (main when empty): only a
	// write it can reach keeps an object written.
	Roots []string
	// Before is what was unwritten before the cut (Editor.Unwritten, asked
	// first).  With it, the seeds are what is unwritten now and was not
	// then, but for Hold, to a fixed point over what the closure's folds
	// leave unwritten in turn: xform.FallOutOf.  Without it every
	// unwritten object and member is a seed: xform.FallOut.
	Before *Unwritten
	// Hold are names left to the phases that fold them by hand.
	Hold []string
	// Off turns rules off by name: "logic", "returns", "params", "empty",
	// "jumps", "locals" (each the rule above).  Every rule is on by
	// default, as the text closure has them.
	Off []string
	// Marks are values a cut wrote (Literal's nodes) that count as the
	// closure's own: its rules fire on them as on what it writes -- a
	// predicate the cut made `return 0;` is 0 at its calls, an `if (x &&
	// 0)` it wrote loses its branch.  Any other node marked is a branch
	// taken, as a cut's fold is.
	Marks []*Node
	// NoSeeds seeds nothing: the closure starts from the Marks alone.
	NoSeeds bool
}

// FoldXStats are what FoldX did, by the text closure's rule names.
type FoldXStats struct {
	Rules  map[string]int
	Rounds int
	// Report is the text closure's report, line for line.
	Report []string
}

// the text closure's rule names
const (
	ruleObjects    = "reads of objects nothing writes"
	ruleMembers    = "reads of members nothing writes"
	ruleLocals     = "reads of locals so initialised"
	ruleConst      = "constant expressions"
	ruleLogic      = "constant operands of && and ||"
	ruleIfs        = "ifs of a constant condition"
	ruleWhiles     = "whiles of a false condition"
	ruleCond       = "?: of a constant condition"
	ruleJumps      = "statements after a revealed jump"
	ruleEmpty      = "calls of functions left empty"
	ruleReturns    = "calls of functions returning a constant"
	ruleParams     = "parameters every call passes one constant"
	xMarkText      = "\x00fallout"
	xMaxRounds     = 200
	xMaxSeedPasses = 64
)

// xfold is one FoldX: the marks it has written, and a round's indexes.
type xfold struct {
	e   *Editor
	x   FoldX
	off map[string]bool

	val      map[*Node]int64 // the values the closure wrote, by their atom
	marks    []*Node         // the MARK items it put in blocks
	trail    map[*Node]bool  // an if a mark ends: its else, taking nothing, went
	inner    map[*Node]bool  // a node a mark is inside of, not as an item
	enumVals map[*Node]int64
	skeys    map[*Node]string

	// the round's
	fns      map[string]*Node
	fnList   []*Node
	mentions map[string]int
	calls    map[string][]*Node
	caller   map[*Node]*Node
	live     map[*Node]bool
	marked   map[*Node]bool // a node a mark or a value is inside of
	seedFn   map[*Node]bool
	seedObj  map[*Node]int64 // the seeded objects' declarations
	seedMem  map[*Node]bool  // the seeded members
	only     map[string]bool // the seeds by name; nil: every unwritten one
	vals     map[string]int64
	claimed  map[*Node]bool
	below    map[*Node]bool
	rws      []xrw
	stats    *FoldXStats
	dropped  map[string]int // a function a round took a parameter of, and its count left
}

// xrw is one rewrite a round found.
type xrw struct {
	rule  string
	apply func() error
}

func newXFold(e *Editor, x FoldX) *xfold {
	f := &xfold{e: e, x: x, off: set(x.Off), val: map[*Node]int64{}, trail: map[*Node]bool{},
		inner: map[*Node]bool{}, enumVals: map[*Node]int64{}, skeys: map[*Node]string{}}
	return f
}

// FoldX runs xform.FallOut's closure on the graph: the seeds, and what
// they make constant, to the fixed point.
func (e *Editor) FoldX(x FoldX) (FoldXStats, error) {
	st := FoldXStats{Rules: map[string]int{}}
	f := newXFold(e, x)
	f.stats = &st
	for _, m := range x.Marks {
		if v, ok := constant(m); ok && (!m.list || m.Is("-")) {
			f.val[m] = v
		} else {
			f.inner[m] = true
		}
	}
	if x.NoSeeds {
		f.only = map[string]bool{}
		if err := f.closure(); err != nil {
			return st, err
		}
		return st, f.unmark()
	}
	if x.Before == nil {
		f.only = nil
		if err := f.closure(); err != nil {
			return st, err
		}
		return st, f.unmark()
	}
	held := set(x.Hold)
	seeds := map[string]bool{}
	f.index()
	after := f.unwritten()
	for pass := 0; ; pass++ {
		var names []string
		for _, k := range after.Names() {
			if !x.Before.Has(k) && !held[k] && !seeds[k] {
				seeds[k] = true
				names = append(names, k)
			}
		}
		sort.Strings(names)
		if len(names) == 0 {
			break
		}
		if pass == 0 {
			st.Report = append(st.Report, fmt.Sprintf("%d left unwritten by the cut: %s", len(names), strings.Join(names, " ")))
		} else {
			st.Report = append(st.Report, fmt.Sprintf("%d more left unwritten by what fell out: %s", len(names), strings.Join(names, " ")))
		}
		if pass > xMaxSeedPasses {
			return st, fmt.Errorf("foldx: no fixed point over the seeds after %d passes", pass)
		}
		f.only = seeds
		if err := f.closure(); err != nil {
			return st, err
		}
		f.index()
		after = f.unwritten()
	}
	return st, f.unmark()
}

// closure is one pass: rounds until none rewrites anything.
func (f *xfold) closure() error {
	f.vals = nil
	counts := map[string]int{}
	rounds := 0
	for round := 0; ; round++ {
		rounds = round
		f.collect()
		if len(f.rws) == 0 {
			break
		}
		for _, r := range f.rws {
			counts[r.rule]++
			f.stats.Rules[r.rule]++
		}
		for _, r := range f.rws {
			if err := r.apply(); err != nil {
				return fmt.Errorf("foldx: round %d, %s: %w", round, r.rule, err)
			}
		}
		if err := f.checkDropped(); err != nil {
			return fmt.Errorf("foldx: round %d: %w", round, err)
		}
		if round > xMaxRounds {
			return fmt.Errorf("foldx: no fixed point after %d rounds", round)
		}
	}
	f.stats.Rounds += rounds
	var rules []string
	for r := range counts {
		rules = append(rules, r)
	}
	sort.Strings(rules)
	var parts []string
	for _, r := range rules {
		parts = append(parts, fmt.Sprintf("%d %s", counts[r], r))
	}
	if len(parts) == 0 {
		parts = append(parts, "nothing falls out")
	}
	f.stats.Report = append(f.stats.Report, fmt.Sprintf("%s; %d rounds", strings.Join(parts, ", "), rounds))
	return nil
}

// unmark takes the marks out: the values stay, plain constants.  The
// editor's records of what was written and emptied go with them: the
// closure has folded what they held.
func (f *xfold) unmark() error {
	for _, m := range f.marks {
		if f.e.Live(m) {
			if err := f.e.Delete(m); err != nil {
				return err
			}
		}
	}
	f.marks = nil
	f.trail = map[*Node]bool{}
	f.inner = map[*Node]bool{}
	f.e.Written = nil
	f.e.emptied = nil
	return nil
}

// ---- the marks

func isXMark(n *Node) bool { return n != nil && !n.list && n.Atom == xMarkText }

func (f *xfold) mark() *Node {
	m := NewAtom(xMarkText)
	f.marks = append(f.marks, m)
	return m
}

// lit is the value v written by the closure.
func (f *xfold) lit(v int64) *Node {
	n := Literal(v)
	f.val[n] = v
	return n
}

// isMarker says n is exactly a value the closure wrote.
func (f *xfold) isMarker(n *Node) bool {
	_, ok := f.val[n]
	return ok && f.e.Live(n)
}

// indexMarks finds every node a mark or a written value is inside of.
func (f *xfold) indexMarks() {
	f.marked = map[*Node]bool{}
	up := func(n *Node, self bool) {
		if !f.e.Live(n) {
			return
		}
		if !self {
			n = f.e.Parent(n)
		}
		for ; n != nil && !f.marked[n]; n = f.e.Parent(n) {
			f.marked[n] = true
		}
	}
	for n := range f.val {
		up(n, true)
	}
	for _, m := range f.marks {
		up(m, false)
	}
	for n := range f.trail {
		up(n, true)
	}
	for n := range f.inner {
		up(n, true)
	}
}

// markedBefore says a mark stands right before the item: an item before
// it is a mark, or ends with one.
func (f *xfold) markedBefore(first *Node) bool {
	prev := f.e.Sibling(first, -1)
	if prev == nil {
		return false
	}
	if isXMark(prev) {
		return true
	}
	return f.endsWithMark(prev)
}

func (f *xfold) endsWithMark(n *Node) bool {
	for n.Is("if") {
		if f.trail[n] {
			return true
		}
		if len(n.Kids) < 4 {
			return false
		}
		n = n.Kids[3]
	}
	return false
}

// ---- a round

// collect finds the round's rewrites, outermost first.
func (f *xfold) collect() {
	f.index()
	f.indexMarks()
	f.prepSeeds()
	f.claimed = map[*Node]bool{}
	f.below = map[*Node]bool{}
	f.rws = nil
	f.dropped = map[string]int{}
	f.branches()
	if !f.off["jumps"] {
		f.deadAfterJump()
	}
	if !f.off["empty"] {
		f.emptyFunctions()
	}
	if !f.off["returns"] {
		f.constReturns()
	}
	if !f.off["params"] {
		f.constParams()
	}
	if !f.off["logic"] {
		f.logic()
	}
	f.constExprs()
	f.seeds()
	if !f.off["locals"] {
		f.locals()
	}
}

// add records a rewrite of the nodes roots unless one of them is inside, or
// holds, what a rewrite this round took already.
func (f *xfold) add(rule string, roots []*Node, apply func() error) bool {
	for _, r := range roots {
		if f.below[r] {
			return false
		}
		for n := r; n != nil; n = f.e.Parent(n) {
			if f.claimed[n] {
				return false
			}
		}
	}
	for _, r := range roots {
		f.claimed[r] = true
		for n := f.e.Parent(r); n != nil && !f.below[n]; n = f.e.Parent(n) {
			f.below[n] = true
		}
	}
	f.rws = append(f.rws, xrw{rule: rule, apply: apply})
	return true
}

// markedFns are the functions holding a mark or a value the closure wrote,
// or reading a seed: the only ones a rule after the seeds fires in.
func (f *xfold) markedFns() []*Node {
	var out []*Node
	for _, fn := range f.fnList {
		if f.marked[fn] || f.seedFn[fn] {
			out = append(out, fn)
		}
	}
	return out
}

// walkBody walks a function's items, a node before what it holds, not
// below a node fn returns false for.
func walkBody(fn *Node, visit func(*Node) bool) {
	for _, it := range Body(fn) {
		Walk(it, visit)
	}
}

// ---- seeds

// prepSeeds finds the seeded objects and members of this round: the
// values are the pass's first round's, since the rounds only take code
// out, so what was unwritten stays so, at the same value.
func (f *xfold) prepSeeds() {
	if f.vals == nil {
		f.vals = f.unwrittenObjects()
	}
	f.seedObj = map[*Node]int64{}
	for _, d := range f.e.g.Forms {
		if !d.Is("def") || hasPrefix(d, "typedef") {
			continue
		}
		nm := declName(d)
		if v, ok := f.vals[nm]; ok && (f.only == nil || f.only[nm]) {
			f.seedObj[d] = v
		}
	}
	mems := f.only
	if mems == nil {
		mems = f.unwrittenMembers()
	}
	f.seedMem = map[*Node]bool{}
	fields := map[string]bool{}
	for k := range mems {
		if i := strings.LastIndexByte(k, '.'); i >= 0 {
			fields[k[i+1:]] = true
		}
	}
	f.seedFn = map[*Node]bool{}
	for _, fn := range f.fnList {
		Walk(fn, func(n *Node) bool {
			if !n.list {
				if f.only != nil && f.only[f.identUse(n)] {
					f.seedFn[fn] = true
				}
				return true
			}
			for _, a := range selected(n) {
				if r := a.Ref(); r != nil {
					if k := f.memberKey(r); k != "" && mems[k] {
						f.seedMem[r] = true
					}
				}
				if f.only != nil && fields[memberAtomName(a)] {
					f.seedFn[fn] = true
				}
			}
			return true
		})
	}
}

// lvalue says n is written where it stands: assigned, stepped, or its
// address taken, or indexed where the element is.
func (f *xfold) lvalue(n *Node) bool {
	for {
		p := f.e.Parent(n)
		if p == nil {
			return false
		}
		switch h := p.Head(); {
		case h == "paren":
			n = p
			continue
		case assignOps[h]:
			return p.Kids[1] == n
		case incDec[h], h == "addr":
			return true
		case h == "index" && p.Kids[1] == n:
			n = p
			continue
		}
		return false
	}
}

// underSizeof says n is the operand of a sizeof, an alignof or a typeof:
// not a read.
func (f *xfold) underSizeof(n *Node) bool {
	for p := f.e.Parent(n); p != nil; p = f.e.Parent(p) {
		if IsStatement(p) && !p.Is("def") {
			return false
		}
		switch h := p.Head(); {
		case strings.HasPrefix(h, "sizeof"), strings.HasPrefix(h, "alignof"), strings.HasPrefix(h, "typeof"),
			strings.HasPrefix(h, "__typeof"), strings.HasPrefix(h, "typeof_unqual"):
			return true
		case h == "def", h == "defn":
			return false
		}
	}
	return false
}

// boolContext says only the truth of n's value is read.
func (f *xfold) boolContext(n *Node) bool {
	for {
		p := f.e.Parent(n)
		if p == nil {
			return false
		}
		switch h := p.Head(); h {
		case "paren":
			n = p
			continue
		case "if", "while":
			return p.Kids[1] == n
		case "do":
			return p.Kids[2] == n
		case "for":
			return p.Kids[2] == n
		case "?":
			return p.Kids[1] == n
		case "!":
			return true
		case "&&", "||":
			return true
		}
		return false
	}
}

// tested says a pointer's value is only compared or tested for null.
func (f *xfold) tested(n *Node) bool {
	if f.boolContext(n) {
		return true
	}
	for {
		p := f.e.Parent(n)
		if p == nil {
			return false
		}
		switch p.Head() {
		case "paren":
			n = p
			continue
		case "==", "!=":
			return true
		}
		return false
	}
}

// seedLit is the literal a seed read becomes: a null pointer constant for
// a pointer.
func (f *xfold) seedLit(ptr bool, v int64) *Node {
	l := f.lit(v)
	if ptr {
		return NewList(NewAtom("paren"), NewList(NewAtom("cast"), NewList(NewAtom("ptr"), NewAtom("void")), l))
	}
	return l
}

// seeds writes each read of an object or a member nothing writes as its
// value.
func (f *xfold) seeds() {
	for _, fn := range f.fnList {
		if !f.live[fn] {
			continue
		}
		walkBody(fn, func(n *Node) bool {
			if f.lvalue(n) {
				return true
			}
			if !n.list {
				d := n.Ref()
				if d == nil || f.identUse(n) == "" {
					return true
				}
				v, ok := f.seedObj[d]
				if !ok || f.underSizeof(n) {
					return true
				}
				ptr := d.Type.Is("pointer")
				if ptr && !f.tested(n) {
					return true
				}
				f.replace(ruleObjects, n, func() *Node { return f.seedLit(ptr, v) })
				return true
			}
			ms := selected(n)
			if len(ms) == 0 {
				return true
			}
			m := ms[len(ms)-1].Ref()
			if m == nil || !f.seedMem[m] || f.underSizeof(n) {
				return true
			}
			target := n
			t := m.Type
			if t.Is("array") {
				ix := f.e.Parent(n)
				if ix == nil || !ix.Is("index") || ix.Kids[1] != n || len(ix.Kids) != 3 || !f.pure(ix.Kids[2]) {
					return true
				}
				target = ix
				t = pointee(t)
			} else if !isScalarType(t) {
				return true
			}
			ptr := t.Is("pointer")
			if ptr && !f.tested(target) {
				return true
			}
			f.replace(ruleMembers, target, func() *Node { return f.seedLit(ptr, 0) })
			return false
		})
	}
}

// replace adds the rewrite of n by what mk makes, in the parentheses n
// stood in: the text rewrites the expression inside them.
func (f *xfold) replace(rule string, n *Node, mk func() *Node) bool {
	paren := f.e.parenthesised(n)
	return f.add(rule, []*Node{n}, func() error { return f.e.Replace(n, carried(paren, mk())) })
}

// replaceAll adds the rewrite of n by what mk makes, the parentheses n
// stood in with it: cc's node for a parenthesised expression is the
// parentheses', and the constant rule takes the outermost.
func (f *xfold) replaceAll(rule string, n *Node, mk func() *Node) bool {
	return f.add(rule, []*Node{n}, func() error { return f.e.Replace(n, mk()) })
}

// ---- values

// pure is cc's purity: no store, no call, nothing the forms keep as text.
func (f *xfold) pure(n *Node) bool {
	ok := true
	Walk(n, func(x *Node) bool {
		if !ok || !x.list {
			return ok
		}
		switch h := x.Head(); {
		case h == "macro":
			// an invocation is opaque, but offsetof's
			_, ok = f.offsetofValue(x)
		case assignOps[h], incDec[h], h == "call", h == "stmt-expr", h == "verbatim":
			ok = false
		}
		return ok
	})
	return ok
}

// constOf is a marked expression's value, or one a seed's read decides.
func (f *xfold) constOf(n *Node) (int64, bool) {
	if f.marked[n] {
		v, t, ok := f.xconst(n)
		if !ok || t.size == 0 {
			return 0, false
		}
		return v.v, true
	}
	if v, ok, seedy := f.seedEval(n); ok && seedy {
		return v, true
	}
	return 0, false
}

// seedEval is an expression's value with each seed's read counted as the
// seed's value, and whether a seed was read: only where C's own answer
// cannot differ (xform's eval).
func (f *xfold) seedEval(n *Node) (int64, bool, bool) {
	small := func(v int64) bool { return v >= -1<<31 && v < 1<<31 }
	if !n.list {
		if d := n.Ref(); d != nil && f.identUse(n) != "" {
			if v, ok := f.seedObj[d]; ok && !f.underSizeof(n) && !f.lvalue(n) {
				return v, true, true
			}
		}
	} else {
		args := n.Args()
		switch h := n.Head(); {
		case h == "paren" && len(args) == 1:
			return f.seedEval(args[0])
		case (h == "->" || h == ".") && len(args) >= 2:
			if m := args[len(args)-1].Ref(); m != nil && f.seedMem[m] && isScalarType(m.Type) && !f.underSizeof(n) && !f.lvalue(n) {
				return 0, true, true
			}
		case h == "cast" && len(args) == 2:
			if t := xtypeOfForm(args[0]); t.integer() {
				if v, ok, sy := f.seedEval(args[1]); ok && sy && small(v) && v >= 0 {
					return v, true, true
				}
			}
		case h == "!" && len(args) == 1:
			if v, ok, sy := f.seedEval(args[0]); ok && sy {
				return b2i(v == 0), true, true
			}
		case (h == "==" || h == "!=") && len(args) == 2:
			l, lok, ls := f.seedEval(args[0])
			r, rok, rs := f.seedEval(args[1])
			if lok && rok && (ls || rs) && small(l) && small(r) {
				return b2i((l == r) == (h == "==")), true, true
			}
		case (h == "<" || h == ">" || h == "<=" || h == ">=") && len(args) == 2:
			l, lok, ls := f.seedEval(args[0])
			r, rok, rs := f.seedEval(args[1])
			if lok && rok && (ls || rs) && small(l) && small(r) && l >= 0 && r >= 0 {
				res := h == "<" && l < r || h == ">" && l > r || h == "<=" && l <= r || h == ">=" && l >= r
				return b2i(res), true, true
			}
		case (h == "&&" || h == "||") && len(args) >= 2:
			// left-nested: ((a && b) && c)
			return f.seedLogic(h, args, len(args))
		case h == "?" && len(args) == 3:
			if c, ok, sy := f.seedEval(args[0]); ok && sy {
				b := args[2]
				if c != 0 {
					b = args[1]
				}
				if v, ok, _ := f.seedEval(b); ok {
					return v, true, true
				}
			}
		}
	}
	// a leaf the check knows
	if v, t, ok := f.xconst(n); ok && t.size > 0 {
		return v.v, true, false
	}
	return 0, false, false
}

// seedLogic is seedEval of the first k operands of an && or || run.
func (f *xfold) seedLogic(h string, args []*Node, k int) (int64, bool, bool) {
	var l int64
	var lok, ls bool
	if k == 2 {
		l, lok, ls = f.seedEval(args[0])
	} else {
		l, lok, ls = f.seedLogic(h, args, k-1)
	}
	r, rok, rs := f.seedEval(args[k-1])
	pureLeft := func() bool {
		for _, a := range args[:k-1] {
			if !f.pure(a) {
				return false
			}
		}
		return true
	}
	if h == "&&" {
		switch {
		case lok && ls && l == 0:
			return 0, true, true
		case lok && rok && (ls || rs):
			return b2i(l != 0 && r != 0), true, true
		case rok && rs && r == 0 && pureLeft():
			return 0, true, true
		}
	} else {
		switch {
		case lok && ls && l != 0:
			return 1, true, true
		case lok && rok && (ls || rs):
			return b2i(l != 0 || r != 0), true, true
		case rok && rs && r != 0 && pureLeft():
			return 1, true, true
		}
	}
	if k == len(args) {
		if v, t, ok := f.xconst(&Node{list: true, Kids: append([]*Node{NewAtom(h)}, args[:k]...)}); ok && t.size > 0 {
			return v.v, true, false
		}
	}
	return 0, false, false
}

// exprType is n's type, as far as an integer test needs it.
func (f *xfold) exprXType(n *Node) xtype {
	if _, t, ok := f.xconst(n); ok {
		return t
	}
	if !n.list {
		if d := n.Ref(); d != nil {
			return xtypeOfType(d.Type)
		}
		return xtype{}
	}
	switch h := n.Head(); {
	case h == "paren":
		return f.exprXType(n.Kids[1])
	case h == "==", h == "!=", h == "<", h == ">", h == "<=", h == ">=", h == "&&", h == "||", h == "!":
		return xInt
	case h == "cast":
		return xtypeOfForm(n.Kids[1])
	}
	if t := f.typeNode(n); t != nil {
		return xtypeOfType(t)
	}
	return xtype{}
}

// ---- constants

// isExpr says n is an expression's form.
func isExpr(n *Node) bool {
	if !n.list {
		return true
	}
	return xExprHeads[n.Head()] || assignOps[n.Head()] || incDec[n.Head()] || binaryOps[n.Head()]
}

var xExprHeads = map[string]bool{
	"paren": true, "cast": true, "call": true, "index": true, "->": true, ".": true,
	"!": true, "~": true, "addr": true, "deref": true, "?": true, "comma": true,
	"sizeof": true, "sizeof-bare": true, "sizeof-type": true, "alignof": true,
	"alignof-bare": true, "alignof-type": true, "literal": true, "generic": true,
	"stmt-expr": true, "label-addr": true, "macro": true,
}

// chainOps are the operators C-lisp writes as one form for a left-nested
// run: `(+ a b c)` is `(a + b) + c`, two of cc's nodes.
func chainOp(n *Node) bool {
	return n.list && len(n.Kids) > 3 && (binaryOps[n.Head()] || n.Is("comma"))
}

// runPrefix is the virtual node of the first k operands of a run: a new list
// over the same operands, never in the graph.
func runPrefix(n *Node, k int) *Node {
	return &Node{list: true, Kids: append([]*Node{n.Kids[0]}, n.Kids[1:k+1]...)}
}

// constExprs writes an integer expression holding a value the closure
// wrote, and constant with it, as its value: the outermost such.
func (f *xfold) constExprs() {
	for _, fn := range f.markedFns() {
		walkBody(fn, func(n *Node) bool { return f.constExpr(n, n, 0) })
	}
}

// constExpr is the rule at n, or, for k > 0, at the virtual node of n's
// first k operands.
func (f *xfold) constExpr(n, real *Node, k int) bool {
	if !isExpr(n) || isXMark(n) {
		return true
	}
	if f.isMarker(n) {
		return false
	}
	x := n
	if k > 0 {
		x = runPrefix(n, k)
		f.marked[x] = f.anyMarked(n.Kids[1 : k+1])
	}
	if v, ok := f.constOf(x); ok && f.pure(x) && f.exprXType(x).integer() && !f.underSizeof(real) {
		if k == 0 {
			f.replaceAll(ruleConst, n, func() *Node { return f.lit(v) })
		} else {
			f.replaceRun(ruleConst, n, k, func() *Node { return f.lit(v) })
		}
		return false
	}
	if k == 0 && chainOp(n) {
		// cc's next nodes: the runs inside, then the operands
		for j := len(n.Kids) - 2; j >= 2; j-- {
			if !f.constExpr(n, n, j) {
				for _, o := range n.Kids[j+1:] {
					Walk(o, func(m *Node) bool { return f.constExpr(m, m, 0) })
				}
				return false
			}
		}
	}
	return true
}

func (f *xfold) anyMarked(ns []*Node) bool {
	for _, n := range ns {
		if f.marked[n] {
			return true
		}
	}
	return false
}

// replaceRun adds the rewrite of the first k operands of the run n by
// what mk makes: one operand, or the whole when k is all of them.
func (f *xfold) replaceRun(rule string, n *Node, k int, mk func() *Node) bool {
	if k == len(n.Kids)-1 {
		return f.replaceAll(rule, n, mk)
	}
	return f.add(rule, n.Kids[1:k+1], func() error {
		return f.e.Replace(n, NewList(append([]*Node{NewAtom(n.Head()), mk()}, n.Kids[k+1:]...)...))
	})
}

// isTruth says n's value is 0 or 1 already: cc's node for it is one of
// the operators that say so, not the parentheses it stands in.
func (f *xfold) isTruth(n *Node) bool {
	if f.e.parenthesised(n) {
		return false
	}
	switch n.Head() {
	case "&&", "||", "==", "!=", "<", ">", "<=", ">=", "!":
		return true
	}
	return false
}

// logic takes the constant operand out of && and ||.
func (f *xfold) logic() {
	for _, fn := range f.markedFns() {
		walkBody(fn, func(n *Node) bool {
			if !(n.Is("&&") || n.Is("||")) || len(n.Kids) < 3 {
				return true
			}
			// cc's nodes, the outermost first: the first k operands
			for k := len(n.Kids) - 1; k >= 2; k-- {
				if f.logicAt(n, k) {
					// what follows the run's rewritten prefix
					for _, o := range n.Kids[k+1:] {
						Walk(o, func(m *Node) bool { return f.logicVisit(m) })
					}
					return false
				}
			}
			return true
		})
	}
}

func (f *xfold) logicVisit(n *Node) bool {
	if !(n.Is("&&") || n.Is("||")) || len(n.Kids) < 3 {
		return true
	}
	for k := len(n.Kids) - 1; k >= 2; k-- {
		if f.logicAt(n, k) {
			for _, o := range n.Kids[k+1:] {
				Walk(o, func(m *Node) bool { return f.logicVisit(m) })
			}
			return false
		}
	}
	return true
}

// logicAt tries the rule at the virtual node of n's first k operands: its
// left operand the first k-1, its right the k-th.
func (f *xfold) logicAt(n *Node, k int) bool {
	or := n.Is("||")
	var l *Node
	if k == 2 {
		l = n.Kids[1]
	} else {
		l = runPrefix(n, k-1)
		f.marked[l] = f.anyMarked(n.Kids[1:k])
	}
	r := n.Kids[k]
	bctx := k < len(n.Kids)-1 || f.boolContext(n)
	// the identity: K true in &&, K false in ||, keeps the other operand
	keep := func(kk, other *Node, otherLeft bool) bool {
		v, ok := f.constOf(kk)
		if !ok || (v != 0) == or {
			return false
		}
		if !bctx && !f.isTruth(other) {
			return false
		}
		if otherLeft && k == 2 {
			in := f.e.parenthesised(l)
			return f.logicRewrite(n, k, func() []*Node { return []*Node{carried(in, l)} })
		}
		in := f.e.parenthesised(r)
		return f.logicRewrite(n, k, func() []*Node {
			if otherLeft {
				return n.Kids[1:k]
			}
			return []*Node{carried(in, r)}
		})
	}
	// the absorbing value: K false in &&, K true in ||, is the result
	absorb := func(kk, other *Node) bool {
		v, ok := f.constOf(kk)
		if !ok || (v != 0) != or || !f.pure(other) {
			return false
		}
		res := int64(0)
		if or {
			res = 1
		}
		return f.logicRewrite(n, k, func() []*Node { return []*Node{f.lit(res)} })
	}
	return keep(l, r, false) || keep(r, l, true) || absorb(l, r) || absorb(r, l)
}

// logicRewrite adds the rewrite of the run n's first k operands by the
// operands mk makes.
func (f *xfold) logicRewrite(n *Node, k int, mk func() []*Node) bool {
	roots := []*Node{n}
	paren := false
	if k < len(n.Kids)-1 {
		roots = n.Kids[1 : k+1]
	} else {
		paren = f.e.parenthesised(n)
	}
	return f.add(ruleLogic, roots, func() error {
		ops := mk()
		rest := n.Kids[k+1:]
		all := append(append([]*Node(nil), ops...), rest...)
		if len(all) == 1 {
			return f.e.Replace(n, carried(paren, all[0]))
		}
		return f.e.Replace(n, carried(paren, NewList(append([]*Node{NewAtom(n.Head())}, all...)...)))
	})
}

// ---- statements

// hasLabel says a statement holds a label or a case.
func hasLabel(s *Node) bool {
	found := false
	Walk(s, func(n *Node) bool {
		found = found || n.Is("label") || isCaseLabel(n)
		return !found
	})
	return found
}

func isLabelOrAttr(n *Node) bool { return n.Is("label") || isCaseLabel(n) || n.Is("stmt-attr") }

// alone says s is an item of a block, not the statement of a label.
func (f *xfold) alone(s *Node) bool {
	p, i := f.e.index(s)
	if p == nil || f.e.place(p, i) != placeItem {
		return false
	}
	for prev := f.e.Sibling(s, -1); prev != nil; prev = f.e.Sibling(prev, -1) {
		if isXMark(prev) {
			continue
		}
		return !isLabelOrAttr(prev)
	}
	return true
}

// spliced is a taken branch's items, as the text splices them: a block
// standing alone gives its items from its first to its last (the marks
// before the first and after the last left out), unless one declares.
func spliced(take *Node, alone bool) []*Node {
	if alone && take.Is("block") && !(len(take.Kids) > 1 && take.Kids[1].Is("@")) {
		its := blockItems(take)
		decl := false
		lo, hi := -1, -1
		for i, it := range its {
			if isXMark(it) {
				continue
			}
			decl = decl || isDeclItem(it)
			if lo < 0 {
				lo = i
			}
			hi = i
		}
		if !decl {
			if lo < 0 {
				return nil
			}
			return its[lo : hi+1]
		}
	}
	return []*Node{take}
}

// branches takes the branch an if, a while or a ?: of a constant condition
// takes.
func (f *xfold) branches() {
	for _, fn := range f.markedFns() {
		walkBody(fn, func(n *Node) bool {
			switch {
			case n.Is("if") && len(n.Kids) >= 3:
				v, ok := f.constOf(n.Kids[1])
				if !ok || hasLabel(n.Kids[2]) || len(n.Kids) > 3 && hasLabel(n.Kids[3]) {
					return true
				}
				f.ifRule(n, v)
				return false
			case n.Is("while") && len(n.Kids) == 3:
				v, ok := f.constOf(n.Kids[1])
				if !ok || v != 0 || hasLabel(n.Kids[2]) {
					return true
				}
				f.gone(ruleWhiles, n)
				return false
			case n.Is("?") && len(n.Kids) == 4:
				v, ok := f.constOf(n.Kids[1])
				if !ok {
					return true
				}
				take := n.Kids[3]
				if v != 0 {
					take = n.Kids[2]
				}
				in := f.e.parenthesised(take)
				f.replace(ruleCond, n, func() *Node { return NewList(NewAtom("paren"), carried(in, take)) })
				return false
			}
			return true
		})
	}
}

// gone adds the rewrite of the statement s by a mark: and an empty
// statement where a label holds it.
func (f *xfold) gone(rule string, s *Node) bool {
	alone := f.alone(s)
	return f.add(rule, []*Node{s}, func() error {
		if alone {
			return f.e.Replace(s, f.mark())
		}
		return f.e.Replace(s, f.mark(), NewList(NewAtom("empty")))
	})
}

func (f *xfold) ifRule(s *Node, v int64) {
	alone := f.alone(s)
	var take *Node
	switch {
	case v != 0:
		take = s.Kids[2]
	case len(s.Kids) > 3:
		take = s.Kids[3]
	}
	p, i := f.e.index(s)
	optional := p != nil && f.e.place(p, i) == placeOptional
	if take != nil {
		f.add(ruleIfs, []*Node{s}, func() error {
			if optional {
				// an else-if: `else /*mark*/ ...` -- the mark inside the
				// chain, not an item of its own
				if err := f.e.Replace(s, take); err != nil {
					return err
				}
				f.inner[p] = true
				return nil
			}
			its := spliced(take, alone)
			return f.e.Replace(s, append([]*Node{f.mark()}, its...)...)
		})
		return
	}
	// no branch taken: an else arm goes from the if that holds it
	if optional {
		f.add(ruleIfs, []*Node{s}, func() error {
			if err := f.e.Delete(s); err != nil {
				return err
			}
			f.trail[p] = true
			return nil
		})
		return
	}
	f.gone(ruleIfs, s)
}

// An xitem is one of cc's block items: labels, and the statement they
// label.
type xitem struct {
	nodes   []*Node // the graph's items, from the first label to the statement
	stmt    *Node
	labeled bool
}

// xitems are a block's items as cc has them, the marks left out.
func xitems(items []*Node) []xitem {
	var out []xitem
	for i := 0; i < len(items); i++ {
		if isXMark(items[i]) {
			continue
		}
		start := i
		labeled := false
		for i < len(items) && (isLabelOrAttr(items[i]) || isXMark(items[i]) && i > start) {
			labeled = labeled || !items[i].Is("stmt-attr")
			i++
		}
		if i == len(items) {
			out = append(out, xitem{nodes: items[start:], labeled: labeled})
			break
		}
		out = append(out, xitem{nodes: items[start : i+1], stmt: items[i], labeled: labeled})
	}
	return out
}

// terminates says control never runs off the end of the item.
func (it xitem) terminates() bool {
	return !it.labeled && it.stmt != nil && stmtTerminates(it.stmt)
}

// stmtTerminates is xform.StmtTerminates on the forms.
func stmtTerminates(s *Node) bool {
	switch s.Head() {
	case "return", "break", "continue", "goto", "goto*":
		return true
	case "block":
		its := xitems(blockItems(s))
		return len(its) > 0 && its[len(its)-1].terminates()
	case "if":
		return len(s.Kids) == 4 && stmtTerminates(s.Kids[2]) && stmtTerminates(s.Kids[3])
	}
	return false
}

// deadAfterJump takes what follows a jump a taken branch revealed, up to
// the next label.
func (f *xfold) deadAfterJump() {
	for _, fn := range f.markedFns() {
		f.jumpsIn(Body(fn))
		walkBody(fn, func(n *Node) bool {
			switch {
			case n.Is("block"):
				f.jumpsIn(blockItems(n))
			case n.Is("stmt-expr"):
				f.jumpsIn(n.Kids[1:])
			}
			return true
		})
	}
}

func (f *xfold) jumpsIn(items []*Node) {
	its := xitems(items)
	for i := 0; i < len(its); i++ {
		it := its[i]
		if !it.terminates() || !(f.markedBefore(it.nodes[0]) || f.anyMarked(it.nodes)) {
			continue
		}
		j := i + 1
		decl := false
		for j < len(its) && !its[j].labeled {
			decl = decl || its[j].stmt != nil && isDeclItem(its[j].stmt)
			j++
		}
		if j == i+1 || decl {
			continue
		}
		first := its[i+1].nodes[0]
		last := its[j-1].nodes[len(its[j-1].nodes)-1]
		var region []*Node
		in := false
		for _, n := range items {
			if n == first {
				in = true
			}
			if in {
				region = append(region, n)
			}
			if n == last {
				break
			}
		}
		f.add(ruleJumps, region, func() error { return f.e.ReplaceRun(first, last, f.mark()) })
		i = j - 1
	}
}

// ---- calls

// onlyCalled says every mention of the function is the callee of a call.
func (f *xfold) onlyCalled(nm string) bool {
	fn := f.fns[nm]
	return fn != nil && hasPrefix(fn, "static") && len(f.calls[nm]) == f.mentions[nm] && len(f.calls[nm]) > 0
}

// realBody is a function's items, the marks left out.
func realBody(fn *Node) []*Node {
	var out []*Node
	for _, it := range Body(fn) {
		if !isXMark(it) {
			out = append(out, it)
		}
	}
	return out
}

// emptyFunctions removes the calls of a function the closure left with
// nothing to do.
func (f *xfold) emptyFunctions() {
	for _, fn := range f.fnList {
		nm := declName(fn)
		if !f.marked[fn] || !f.onlyCalled(nm) {
			continue
		}
		if r := resultType(fn.Type); r == nil || !(r.Is("basic") && len(r.Kids) == 2 && r.Kids[1].Atom == "void") {
			continue
		}
		empty := true
		for _, it := range realBody(fn) {
			switch {
			case it.Is("def"):
				if v := defValue(it); v != nil && (v.Is("init") || !f.pure(v)) {
					empty = false
				}
			case it.Is("return") && len(it.Kids) == 1, it.Is("empty"):
			case isDeclItem(it):
			default:
				empty = false
			}
		}
		if !empty {
			continue
		}
		var sites []*Node
		for _, call := range f.calls[nm] {
			ok := true
			for _, a := range call.Kids[2:] {
				ok = ok && f.pure(a)
			}
			p, i := f.e.index(call)
			if !ok || p == nil || f.e.place(p, i) != placeItem {
				sites = nil
				break
			}
			sites = append(sites, call)
		}
		for _, s := range sites {
			s := s
			f.add(ruleEmpty, []*Node{s}, func() error { return f.e.Replace(s, f.mark()) })
		}
	}
}

// constReturns writes each call of a function whose body is `return K;`, K
// a value the closure wrote, as K.
func (f *xfold) constReturns() {
	for _, fn := range f.fnList {
		nm := declName(fn)
		if !f.marked[fn] || !f.onlyCalled(nm) {
			continue
		}
		its := realBody(fn)
		if len(its) != 1 || !its[0].Is("return") || len(its[0].Kids) != 2 {
			continue
		}
		v, ok := f.constOf(its[0].Kids[1])
		if !ok {
			continue
		}
		for _, call := range f.calls[nm] {
			pure := true
			for _, a := range call.Kids[2:] {
				pure = pure && f.pure(a)
			}
			if pure {
				f.replace(ruleReturns, call, func() *Node { return f.lit(v) })
			}
		}
	}
}

// writeCount is cc's count of a declaration's writes: its initialiser, and
// each store, step or member store through it.
func (f *xfold) writeCount(d *Node) int {
	n := 0
	if d.Is("def") && defValue(d) != nil {
		n++
	}
	for _, u := range f.e.Uses(d) {
		x := u
		for {
			p := f.e.Parent(x)
			if p == nil {
				break
			}
			if p.Is("paren") || p.Is(".") && p.Kids[1] == x {
				x = p
				continue
			}
			if assignOps[p.Head()] && p.Kids[1] == x || incDec[p.Head()] {
				n++
			}
			break
		}
	}
	return n
}

// constParams takes out a parameter every call passes the same value the
// closure wrote: the value in the body, the parameter from the definition
// and its prototypes, the argument from the calls.  One a round.
func (f *xfold) constParams() {
	for _, fn := range f.fnList {
		nm := declName(fn)
		if !f.onlyCalled(nm) {
			continue
		}
		callersMarked := true
		for _, call := range f.calls[nm] {
			callersMarked = callersMarked && f.marked[f.caller[call]]
		}
		if !callersMarked {
			continue
		}
		ps := params(fn)
		for i, p := range ps {
			if !p.list || len(p.Kids) < 2 || p.Kids[0].list || p.Head() == "" {
				continue
			}
			var val int64
			same := true
			for k, call := range f.calls[nm] {
				as := call.Kids[2:]
				if i >= len(as) {
					same = false
					break
				}
				v, ok := f.constOf(as[i])
				if !ok || !f.isMarker(as[i]) || k > 0 && v != val {
					same = false
					break
				}
				val = v
			}
			if !same || f.addrTakenAny(p) || f.writeCount(p) != 0 {
				continue
			}
			f.dropParam(nm, fn, i, len(ps), val)
			return
		}
	}
}

func (f *xfold) addrTakenAny(d *Node) bool {
	for _, u := range f.e.Uses(d) {
		if f.addrTaken(u) {
			return true
		}
	}
	return false
}

// params is a function definition's or prototype's parameters.
func params(d *Node) []*Node {
	t := defType(d)
	if t == nil || !t.Is("fn") || len(t.Kids) < 3 || !t.Kids[1].list {
		return nil
	}
	ps := t.Kids[1].Kids
	if len(ps) == 1 && !ps[0].list && ps[0].Atom == "void" {
		return nil
	}
	return ps
}

// checkDropped refuses a round that left a call passing a parameter it
// took, or a use of one: where the text's next round would not type-check
// (a rewrite of the drop refused for one taken before it), and refuse.
func (f *xfold) checkDropped() error {
	for nm, n := range f.dropped {
		for _, c := range f.calls[nm] {
			if f.e.Live(c) && len(c.Kids)-2 != n {
				return fmt.Errorf("a call of %s passes %d arguments to %d parameters, after one went", nm, len(c.Kids)-2, n)
			}
		}
	}
	if d := f.e.Dangling(); len(d) > 0 {
		return &Unhandled{D: d[0], Fn: declName(f.e.Function(d[0].Use)), Left: len(d)}
	}
	return nil
}

// dropParam writes the rewrites constParams decided on.
func (f *xfold) dropParam(nm string, fn *Node, i, n int, val int64) {
	f.dropped[nm] = n - 1
	p := params(fn)[i]
	for _, u := range f.e.Uses(p) {
		u := u
		f.replace(ruleParams, u, func() *Node { return f.lit(val) })
	}
	for _, d := range f.e.FileDecls(nm) {
		ps := params(d)
		if len(ps) != n {
			continue
		}
		t := defType(d)
		list := t.Kids[1]
		f.add(ruleParams, []*Node{ps[i]}, func() error {
			kids := append([]*Node(nil), list.Kids[:i]...)
			kids = append(kids, list.Kids[i+1:]...)
			if len(kids) == 0 {
				kids = []*Node{NewAtom("void")}
			}
			nl := NewList(kids...)
			nt := NewList(append([]*Node{t.Kids[0], nl}, t.Kids[2:]...)...)
			return f.e.Replace(t, nt)
		})
	}
	for _, call := range f.calls[nm] {
		call := call
		as := call.Kids[2:]
		if i >= len(as) {
			continue
		}
		f.add(ruleParams, []*Node{as[i]}, func() error {
			kids := append([]*Node(nil), call.Kids[:2+i]...)
			kids = append(kids, call.Kids[3+i:]...)
			nc := NewList(kids...)
			f.e.g.save(nc)
			nc.Type = call.Type
			return f.e.Replace(call, nc)
		})
	}
}

// ---- locals

// locals writes each read of a local initialised with a value the closure
// wrote, and never written again, as that value.
func (f *xfold) locals() {
	for _, fn := range f.markedFns() {
		walkBody(fn, func(n *Node) bool {
			if n.list || f.identUse(n) == "" {
				return true
			}
			d := n.Ref()
			if !d.Is("def") || f.e.Function(d) == nil || hasPrefix(d, "static") || f.addrTakenAny(d) || f.writeCount(d) != 1 {
				return true
			}
			init := defValue(d)
			if init == nil || !f.isMarker(init) || f.underSizeof(n) {
				return true
			}
			v := f.val[init]
			f.replace(ruleLocals, n, func() *Node { return f.lit(v) })
			return true
		})
	}
}
