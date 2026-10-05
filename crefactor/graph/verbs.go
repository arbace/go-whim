package graph

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// THE VERBS (doc/GRAPH-MIGRATION.md, B0): crefactor/edit's verb set on the
// graph, so that a phase converted from its text program reads like it --
// the same acts in the same order, each COUNTED and reported as it
// succeeds, the first refusal stopping the rest -- but each act finds what it
// acts on by a C-lisp pattern on the nodes (`(if (== x 0) _*)`, `(call f
// _*)`) instead of a regular expression on the lines, and makes its change
// through the Editor, under its checks: places, containment, the id rule,
// the dangling records the fall-out closure discharges.
//
//	edit.E                               Verbs
//	InFunction(fn, acts)                 InFunction(fn, acts)
//	InTable(head, acts)                  InTable(name, acts)
//	Literal, Sub(re, repl, n, what)      Rewrite(pat, tmpl, n, what), RewriteAt
//	Cut, Lines, DropBlocks               Cut(pat, n, what)
//	FoldNever, FoldAlways, DropIf        FoldNever, FoldAlways, DropIf
//	FoldAlwaysElse, keepThen(Chain)      FoldAlwaysElse, KeepThen
//	Splice(from, through, with, what)    Splice(from, through, tmpl, what)
//	Body(fn, body, what)                 Body(fn, tmpl, what)
//	DeleteDefinition(fn, what)           DeleteDefinition(fn, what)
//	ReplaceBlock, DropWalk               Rewrite
//	DropBareBlock(fn, stmt, what)        DropBareBlock(pat, what)
//	FoldWalk, FoldWalks                  FoldWalk(pat, set, n, what), FoldWalks
//	CountIs, Expect, ConstOf             CountIs, Expect, ConstOf
//	Mentions, Query, BodyOf, InnerBody   Mentions, TextQuery, Query, Editor().Defn, Body
//	(by hand in the cutters)             DropOperand, DropCase
//
// A PATTERN is clisp's (crefactor/clisp/pattern.go): `_` any node, `_*` the
// rest of a list, `?x` a binding.  It matches nodes in the scope, the scope's
// root included, in the file's order -- nested ones too, as a line pattern
// matches a nested if's line.  The fold verbs match the `if` itself, or,
// when the pattern is not headed `if`, an if's CONDITION.  A TEMPLATE is
// what Build reads (build.go): the new forms, with `?x` for a node the
// pattern bound.
//
// THE COUNT IS THE ASSERTION, as in crefactor/edit: an act that could match
// more than once takes the number of times it applies, just before its
// `what`, and refuses on any other count; an act on one thing refuses unless
// there is exactly one.  The matches are found, counted, and then acted on
// in order; one that an earlier act of the same verb took with it is a
// refusal (*the match vanished*), never a smaller cut.
//
// WHAT IS NOT THE TEXT'S: an act refuses what the text would have made
// wrong -- a branch spliced whose declarations clash (Unwrap), a walk's
// break that would rebind, a definition deleted while something uses it --
// and the text's line arithmetic (a body's old length, blank lines) has no
// counterpart: a report that printed it says something else.
type Verbs struct {
	Tag   string
	W     io.Writer
	Err   error
	e     *Editor
	scope *Node      // the scope's root; nil is the file
	batch *fragBatch // FRAG's acts deferred to one import (Together, fragverbs.go)
}

// NewVerbs starts a phase's acts on e, reported under tag.
func NewVerbs(tag string, e *Editor, w io.Writer) *Verbs {
	return &Verbs{Tag: tag, W: w, e: e}
}

// Editor is the editor the acts are made through, for an act no verb says.
func (v *Verbs) Editor() *Editor { return v.e }

// Scope is the node the acts are scoped to, nil for the file.
func (v *Verbs) Scope() *Node { return v.scope }

// Done is the first refusal, or nil.
func (v *Verbs) Done() error { return v.Err }

// Failed says whether an act has already refused.
func (v *Verbs) Failed() bool { return v.Err != nil }

// Die records a refusal, the first one only.
func (v *Verbs) Die(format string, a ...any) {
	if v.Err == nil {
		v.Err = fmt.Errorf("  %-12s %s", v.Tag, fmt.Sprintf(format, a...))
	}
}

// Refuse is Die, in the phase's own words.
func (v *Verbs) Refuse(format string, a ...any) { v.Die(format, a...) }

// Say reports an act, and nothing once the acts have failed.
func (v *Verbs) Say(what string) {
	if v.Err == nil {
		fmt.Fprintf(v.W, "  %-12s %s\n", v.Tag, what)
	}
}

// Sayf is Say with a format.
func (v *Verbs) Sayf(format string, a ...any) { v.Say(fmt.Sprintf(format, a...)) }

// Expect refuses, in the phase's own words, unless ok.
func (v *Verbs) Expect(ok bool, format string, a ...any) {
	if v.Err == nil && !ok {
		v.Die(format, a...)
	}
}

// within runs acts scoped to root, the refusal carried back.
func (v *Verbs) within(root *Node, acts func(*Verbs)) {
	inner := &Verbs{Tag: v.Tag, W: v.W, e: v.e, scope: root, batch: v.batch}
	acts(inner)
	if inner.Err != nil {
		v.Err = inner.Err
	}
}

// InFunction runs the acts scoped to the definition of the function name.
func (v *Verbs) InFunction(name string, acts func(*Verbs)) {
	if v.Err != nil {
		return
	}
	d := v.e.Defn(name)
	if d == nil {
		v.Die("%s is not defined at file scope", name)
		return
	}
	v.within(d, acts)
}

// InTable runs the acts scoped to the file-scope object name's definition,
// its initialiser and all: a table's rows.
func (v *Verbs) InTable(name string, acts func(*Verbs)) {
	if v.Err != nil {
		return
	}
	var t *Node
	for _, d := range v.e.FileDecls(name) {
		if d.Is("def") && defValue(d) != nil {
			if t != nil {
				v.Die("%s is initialised twice", name)
				return
			}
			t = d
		}
	}
	if t == nil {
		v.Die("%s -- no initialised definition at file scope", name)
		return
	}
	v.within(t, acts)
}

// In runs the acts scoped to the node root.
func (v *Verbs) In(root *Node, acts func(*Verbs)) {
	if v.Err != nil {
		return
	}
	if !v.e.Live(root) {
		v.Die("#%d (%s) is not in the graph", root.ID, label(root))
		return
	}
	v.within(root, acts)
}

// roots are the scope's nodes: its root, or the file's forms.
func (v *Verbs) roots() []*Node {
	if v.scope != nil {
		return []*Node{v.scope}
	}
	return append([]*Node(nil), v.e.g.Forms...)
}

// pattern reads a pattern, refusing one that does not read.
func (v *Verbs) pattern(src, what string) *clisp.Node {
	p, err := clisp.Pattern(src)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	return p
}

// Find is every node of the scope the pattern matches, in order.
func (v *Verbs) Find(pat string) []*Node {
	p := v.pattern(pat, pat)
	if p == nil {
		return nil
	}
	return v.find(p, nil)
}

// find is every node of the scope p matches that ok, when given, accepts.
func (v *Verbs) find(p *clisp.Node, ok func(*Node) bool) []*Node {
	var out []*Node
	for _, r := range v.roots() {
		Walk(r, func(n *Node) bool {
			if Matches(p, n) && (ok == nil || ok(n)) {
				out = append(out, n)
			}
			return true
		})
	}
	return out
}

// Count is how many nodes of the scope the pattern matches.
func (v *Verbs) Count(pat string) int { return len(v.Find(pat)) }

// CountIs refuses unless the pattern matches exactly n times; it reports
// nothing.
func (v *Verbs) CountIs(pat string, n int, what string) {
	if v.Err != nil {
		return
	}
	if k := v.Count(pat); v.Err == nil && k != n {
		v.Die("%s -- matched %d times, expected %d", what, k, n)
	}
}

// One is the one node of the scope the pattern matches, refusing on any
// other count.
func (v *Verbs) One(pat, what string) *Node {
	if v.Err != nil {
		return nil
	}
	ms := v.Find(pat)
	if v.Err == nil && len(ms) != 1 {
		v.Die("%s -- matched %d times, expected 1", what, len(ms))
		return nil
	}
	if v.Err != nil {
		return nil
	}
	return ms[0]
}

// Query is the node each match bound to name, in order: what a phase
// computes from the graph rather than lists.
func (v *Verbs) Query(pat, name string) []*Node {
	p := v.pattern(pat, pat)
	if p == nil {
		return nil
	}
	var out []*Node
	for _, n := range v.find(p, nil) {
		b, _ := Match(p, n)
		out = append(out, b[name])
	}
	return out
}

// counted is the matches of pat that ok accepts, refused unless there are n.
func (v *Verbs) counted(pat string, n int, what string, ok func(*Node) bool) ([]*Node, *clisp.Node) {
	if v.Err != nil {
		return nil, nil
	}
	p := v.pattern(pat, what)
	if p == nil {
		return nil, nil
	}
	ms := v.find(p, ok)
	if len(ms) != n {
		v.Die("%s -- matched %d times, expected %d", what, len(ms), n)
		return nil, nil
	}
	return ms, p
}

// each acts on every match in order, refusing one an earlier act took.
func (v *Verbs) each(ms []*Node, what string, act func(*Node) error) bool {
	for _, m := range ms {
		if !v.e.Live(m) {
			v.Die("%s -- the match vanished: an earlier one took #%d with it", what, m.ID)
			return false
		}
		if err := act(m); err != nil {
			v.Die("%s -- %v", what, err)
			return false
		}
	}
	return true
}

// Cut deletes the pattern's n matches: items, members, enumerators, top-level
// forms, an if's else.
func (v *Verbs) Cut(pat string, n int, what string) {
	ms, p := v.counted(pat, n, what, nil)
	if p == nil {
		return
	}
	if v.each(ms, what, v.e.Delete) {
		v.Say(what)
	}
}

// Rewrite replaces each of the pattern's n matches by the template, its
// holes the match's bindings; an empty template deletes it.
func (v *Verbs) Rewrite(pat, tmpl string, n int, what string) {
	v.rewrite(pat, "", tmpl, n, what)
}

// RewriteAt replaces, in each of the pattern's n matches, the node bound
// to at by the template: the text's `${1}new${2}`, the rest of the match
// kept as it is.
func (v *Verbs) RewriteAt(pat, at, tmpl string, n int, what string) {
	v.rewrite(pat, at, tmpl, n, what)
}

func (v *Verbs) rewrite(pat, at, tmpl string, n int, what string) {
	ms, p := v.counted(pat, n, what, nil)
	if p == nil {
		return
	}
	if v.each(ms, what, func(m *Node) error {
		b, _ := Match(p, m)
		target := m
		if at != "" {
			if target = b[at]; target == nil {
				return fmt.Errorf("the pattern binds no ?%s", at)
			}
			delete(b, at)
		}
		with, err := v.e.Build(target, tmpl, b)
		if err != nil {
			return err
		}
		return v.e.Replace(target, with...)
	}) {
		v.Say(what)
	}
}

// RewriteFunc replaces each of the pattern's n matches by what f makes of
// it and its bindings.
func (v *Verbs) RewriteFunc(pat string, n int, f func(m *Node, b Bindings) ([]*Node, error), what string) {
	ms, p := v.counted(pat, n, what, nil)
	if p == nil {
		return
	}
	if v.each(ms, what, func(m *Node) error {
		b, _ := Match(p, m)
		with, err := f(m, b)
		if err != nil {
			return err
		}
		return v.e.Replace(m, with...)
	}) {
		v.Say(what)
	}
}

// ifs are the n ifs the pattern matches: the if itself when the pattern is
// headed `if`, else by its condition.
func (v *Verbs) ifs(pat string, n int, what string) ([]*Node, bool) {
	if v.Err != nil {
		return nil, false
	}
	p := v.pattern(pat, what)
	if p == nil {
		return nil, false
	}
	var ms []*Node
	for _, r := range v.roots() {
		Walk(r, func(x *Node) bool {
			if x.Is("if") && (p.Is("if") && Matches(p, x) || !p.Is("if") && Matches(p, x.Kids[1])) {
				ms = append(ms, x)
			}
			return true
		})
	}
	if len(ms) != n {
		v.Die("%s -- matched %d times, expected %d -- a fold that is not counted is a guess", what, len(ms), n)
		return nil, false
	}
	return ms, true
}

// arm says the if s is an else's `if`: `else if (...)`.
func (v *Verbs) arm(s *Node) bool {
	p, i := v.e.index(s)
	return p != nil && p.Is("if") && i == 3
}

// FoldNever takes the n ifs whose condition can no longer be true, keeping
// what they chose between: `if (F) A` goes, `if (F) A else C` is C (a
// block's items spliced, Unwrap), `if (F) A else if (X) B` is `if (X) B`,
// and an else-if arm goes from its chain.
func (v *Verbs) FoldNever(pat string, n int, what string) {
	ms, ok := v.ifs(pat, n, what)
	if !ok {
		return
	}
	if v.each(ms, what, func(s *Node) error {
		if len(s.Kids) < 4 {
			return v.e.Delete(s)
		}
		return v.e.Unwrap(s, s.Kids[3])
	}) {
		v.Say(what)
	}
}

// FoldAlways takes the n ifs whose condition is now always true and that
// have no else: the then arm's items in the if's place.
func (v *Verbs) FoldAlways(pat string, n int, what string) {
	ms, ok := v.ifs(pat, n, what)
	if !ok {
		return
	}
	if v.each(ms, what, func(s *Node) error {
		switch {
		case v.arm(s):
			return fmt.Errorf("fold_always: only a plain if, not an else-if arm")
		case len(s.Kids) > 3:
			return fmt.Errorf("fold_always: the block has an else")
		}
		return v.e.Unwrap(s, s.Kids[2])
	}) {
		v.Say(what)
	}
}

// DropIf deletes the n ifs whose condition is now always true and that
// guard nothing the program still needs, refusing one with an else, which
// deleting the if alone would orphan.
func (v *Verbs) DropIf(pat string, n int, what string) {
	ms, ok := v.ifs(pat, n, what)
	if !ok {
		return
	}
	if v.each(ms, what, func(s *Node) error {
		if len(s.Kids) > 3 {
			return fmt.Errorf("drop_if: block has an else; deleting the if alone would orphan it")
		}
		return v.e.Delete(s)
	}) {
		v.Say(what)
	}
}

// FoldAlwaysElse turns `if (T) A else B` into A, n times: an if with one
// plain else, never an else-if.
func (v *Verbs) FoldAlwaysElse(pat string, n int, what string) {
	v.keepThen(pat, n, what, true)
}

// KeepThen turns `if (T) A else ...` into A, n times, whatever chain of
// else-ifs follows.
func (v *Verbs) KeepThen(pat string, n int, what string) {
	v.keepThen(pat, n, what, false)
}

func (v *Verbs) keepThen(pat string, n int, what string, plain bool) {
	ms, ok := v.ifs(pat, n, what)
	if !ok {
		return
	}
	if v.each(ms, what, func(s *Node) error {
		switch {
		case v.arm(s):
			return fmt.Errorf("not a plain if: an else-if arm")
		case len(s.Kids) < 4 || plain && !s.Kids[3].Is("block"):
			return fmt.Errorf("expected an else")
		}
		return v.e.Unwrap(s, s.Kids[2])
	}) {
		v.Say(what)
	}
}

// DropOperand takes, from each of the n `&&`, `||` or `|` it is an operand
// of, the operand the pattern matches: `a && B && c` is `a && c`, `a ||
// B` is `a`.  A match that is no such operand is not counted.
func (v *Verbs) DropOperand(pat string, n int, what string) {
	ms, p := v.counted(pat, n, what, func(x *Node) bool {
		q, i := v.e.index(x)
		return i >= 1 && (q.Is("&&") || q.Is("||") || q.Is("|"))
	})
	if p == nil {
		return
	}
	if v.each(ms, what, func(x *Node) error {
		q := v.e.Parent(x)
		var rest []*Node
		for _, k := range q.Args() {
			if k != x {
				rest = append(rest, k)
			}
		}
		if len(rest) == 1 {
			return v.e.Replace(q, rest[0])
		}
		r := NewList(append([]*Node{NewAtom(q.Head())}, rest...)...)
		if !q.Is("|") {
			v.e.g.save(r)
			r.Type = q.Type // && and || are int whatever their operands
		}
		return v.e.Replace(q, r)
	}) {
		v.Say(what)
	}
}

// isCaseLabel says x is a switch's label: case, case-range, default.
func isCaseLabel(x *Node) bool { return x.Is("case") || x.Is("case-range") || x.Is("default") }

// jumps says the item x never falls through to the next.
func jumps(x *Node) bool {
	return x.Is("break") || x.Is("return") || x.Is("continue") || x.Is("goto") || x.Is("goto*")
}

// DropCase takes the n case labels the pattern matches (`(case 'n')`,
// `(default)`): a label that shares its statements with another goes alone;
// one that heads a run of its own goes with the run, up to the next label --
// refused where the case before it falls into it.
func (v *Verbs) DropCase(pat string, n int, what string) {
	ms, p := v.counted(pat, n, what, isCaseLabel)
	if p == nil {
		return
	}
	if v.each(ms, what, func(x *Node) error {
		p, i := v.e.index(x)
		ks := v.e.kids(p)
		if v.e.place(p, i) != placeItem {
			return fmt.Errorf("#%d (%s) is not an item", x.ID, label(x))
		}
		if i+1 < len(ks) && isCaseLabel(ks[i+1]) || i > 0 && isCaseLabel(ks[i-1]) {
			return v.e.Delete(x)
		}
		if i > 0 && v.e.place(p, i-1) == placeItem && !jumps(ks[i-1]) {
			return fmt.Errorf("the statement before %s (%s) falls into it", label(x), label(ks[i-1]))
		}
		j := i + 1
		for j < len(ks) && !isCaseLabel(ks[j]) {
			j++
		}
		return v.e.ReplaceRun(x, ks[j-1])
	}) {
		v.Say(what)
	}
}

// Splice replaces the run of items from the one from matches through the
// one through matches -- each matching exactly one item of the scope, in
// one list, in that order -- by the template.
func (v *Verbs) Splice(from, through, tmpl, what string) {
	if v.Err != nil {
		return
	}
	var ends [2]*Node
	var binds [2]Bindings
	for k, end := range []struct{ pat, which string }{{from, "its start"}, {through, "its end"}} {
		p := v.pattern(end.pat, what)
		if p == nil {
			return
		}
		ms := v.find(p, func(x *Node) bool { return v.e.Item(x) == x })
		if len(ms) != 1 {
			v.Die("%s -- %s matches %d items, expected 1", what, end.which, len(ms))
			return
		}
		ends[k] = ms[0]
		binds[k], _ = Match(p, ms[0])
	}
	holes := Bindings{}
	for _, b := range binds {
		for k, x := range b {
			holes[k] = x
		}
	}
	with, err := v.e.Build(ends[0], tmpl, holes)
	if err == nil {
		err = v.e.ReplaceRun(ends[0], ends[1], with...)
	}
	if err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// Body replaces the function name's whole body by the template's items.
func (v *Verbs) Body(name, tmpl, what string) {
	if v.Err != nil {
		return
	}
	d := v.e.Defn(name)
	if d == nil {
		v.Die("%s is not defined at file scope", name)
		return
	}
	at := defnItemsAt(d)
	with, err := v.e.buildAt(d, at, tmpl, nil)
	if err == nil {
		err = v.e.splice("replace", d, at, len(d.Kids), with)
	}
	if err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// DeleteDefinition deletes the definition of the function name, refusing
// one that is not there, or that something outside it still refers to (a
// use of its prototype is not one: the prototype stays).
func (v *Verbs) DeleteDefinition(name, what string) {
	if v.Err != nil {
		return
	}
	d := v.e.Defn(name)
	if d == nil {
		v.Refuse("%s is not defined", name)
		return
	}
	for _, u := range v.e.Uses(d) {
		if v.e.Function(u) != d {
			v.Refuse("%s -- %s still refers to the definition", what, label(u))
			return
		}
	}
	if err := v.e.Delete(d); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// DropBareBlock deletes the innermost block around the one statement the
// pattern matches, refusing unless the block is an item and holds nothing
// but that statement and declarations without a value: the husk a removed
// call leaves behind.
func (v *Verbs) DropBareBlock(pat, what string) {
	s := v.One(pat, what)
	if s == nil {
		return
	}
	it := v.e.Item(s)
	b := v.e.Parent(it)
	if b == nil || !b.Is("block") {
		v.Die("%s -- no enclosing block", what)
		return
	}
	for _, x := range blockItems(b) {
		if x == it || x.Is("def") && defValue(x) == nil {
			continue
		}
		v.Die("%s -- the block still does real work: %s", what, label(x))
		return
	}
	if q, i := v.e.index(b); v.e.place(q, i) != placeItem {
		v.Die("%s -- the block is %s's, not an item", what, label(q))
		return
	}
	if err := v.e.Delete(b); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// FoldWalk turns each of the n loops the pattern matches -- a walk over a
// list that has one element now -- into the template set (`(= buf
// curbuf)`), built at the loop, followed by the loop's body's items,
// refusing a body whose break or continue binds to the loop.
func (v *Verbs) FoldWalk(pat, set string, n int, what string) {
	ms, p := v.counted(pat, n, what, isLoop)
	if p == nil {
		return
	}
	if v.each(ms, what, func(s *Node) error {
		b, _ := Match(p, s)
		return v.foldWalk(s, set, b)
	}) {
		v.Say(what)
	}
}

// FoldWalks folds every loop the pattern matches that ok accepts, each into
// the template set makes of its bindings, refusing every one whose break
// or continue would rebind; it reports how many it folded.
func (v *Verbs) FoldWalks(pat string, ok func(Bindings) bool, set func(Bindings) string, what string) {
	if v.Err != nil {
		return
	}
	p := v.pattern(pat, what)
	if p == nil {
		return
	}
	ms := v.find(p, func(x *Node) bool {
		b, _ := Match(p, x)
		return isLoop(x) && ok(b)
	})
	if len(ms) == 0 {
		v.Die("%s -- no walk of this shape is left to fold", what)
		return
	}
	var unsafe []string
	for _, s := range ms {
		if k := bindsTo(s); k != "" {
			unsafe = append(unsafe, fmt.Sprintf("#%d (%s in %s)", s.ID, k, topName(v.e.Function(s))))
		}
	}
	if len(unsafe) > 0 {
		v.Die("%s -- %d walk(s) still carry an escaping break/continue and need an explicit rewrite: %s",
			what, len(unsafe), strings.Join(unsafe, " "))
		return
	}
	if v.each(ms, what, func(s *Node) error {
		b, _ := Match(p, s)
		return v.foldWalk(s, set(b), b)
	}) {
		v.Sayf("%s (%d)", what, len(ms))
	}
}

func isLoop(x *Node) bool { return x.Is("for") || x.Is("while") || x.Is("do") }

func (v *Verbs) foldWalk(s *Node, set string, b Bindings) error {
	if k := bindsTo(s); k != "" {
		return fmt.Errorf("the body has a `%s;` that binds to the walk being removed, not to anything inside it", k)
	}
	if s.Is("for") && s.Kids[1].Is("def") {
		return fmt.Errorf("the walk declares its variable: nothing would declare it after")
	}
	pre, err := v.e.Build(s, set, b)
	if err != nil {
		return err
	}
	body := s.Kids[len(s.Kids)-1]
	if s.Is("do") {
		body = s.Kids[1]
	}
	return v.e.unwrap(s, pre, body)
}

// bindsTo names a break or continue in the loop s's body that binds to s,
// or "".
func bindsTo(s *Node) string {
	body := s.Kids[len(s.Kids)-1]
	if s.Is("do") {
		body = s.Kids[1]
	}
	found := ""
	var walk func(n *Node, inLoop, inSwitch bool)
	walk = func(n *Node, inLoop, inSwitch bool) {
		if found != "" || !n.list {
			return
		}
		switch {
		case n.Is("break") && !inLoop && !inSwitch:
			found = "break"
			return
		case n.Is("continue") && !inLoop:
			found = "continue"
			return
		}
		l, sw := inLoop || isLoop(n), inSwitch || n.Is("switch")
		for _, k := range n.Kids {
			walk(k, l, sw)
		}
	}
	walk(body, false, false)
	return found
}

// ConstOf requires the function name's whole body to be `return K;`, K
// the form expect (C-lisp: `0`, `FALSE`, `(- 1)`).
func (v *Verbs) ConstOf(name, expect string) {
	if v.Err != nil {
		return
	}
	d := v.e.Defn(name)
	if d == nil {
		v.Refuse("%s is not defined", name)
		return
	}
	items := Body(d)
	if len(items) != 1 || !items[0].Is("return") || len(items[0].Kids) != 2 {
		got := make([]string, len(items))
		for i, it := range items {
			got[i] = Lisp(it).String()
		}
		s := strings.Join(got, " ")
		if len(s) > 70 {
			s = s[:70]
		}
		v.Refuse("%s is no longer a one-line stub: %s", name, s)
		return
	}
	p := v.pattern(expect, name)
	if p == nil {
		return
	}
	if k := items[0].Kids[1]; !Matches(p, k) {
		v.Refuse("%s returns %s, not %s -- folding it would change behaviour", name, Lisp(k), expect)
	}
}

// FallOut closes over what the acts so far left dangling (Editor.FallOut),
// refusing, in the closure's words, a use no rule takes.  The verbs do not
// call it: a phase says where its closure runs, as the text phases left
// their uses to the sweep or to an act of their own.
func (v *Verbs) FallOut(opt FallOutOptions) FallOutStats {
	if v.Err != nil {
		return FallOutStats{}
	}
	st, err := v.e.FallOut(opt)
	if err != nil {
		v.Die("%v", err)
	}
	return st
}
