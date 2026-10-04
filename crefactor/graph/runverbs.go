package graph

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// Runs of items as verbs (B3a, doc/GRAPH-MIGRATION.md): what a text
// program said by a literal of several lines, or by a splice from one line
// to the first line after it of a shape, and a case's lines cut where the
// statement before them cannot fall into them.

// Run is the run of consecutive items the patterns say -- an item of the
// scope the first matches, and each next pattern its next sibling -- which
// must occur exactly once in the scope, as a text literal of several lines
// is found once.  It refuses (nil) otherwise.
func (v *Verbs) Run(what string, pats ...string) []*Node {
	if v.Err != nil || len(pats) == 0 {
		return nil
	}
	ps := make([]*clisp.Node, len(pats))
	for i, pat := range pats {
		if ps[i] = v.pattern(pat, what); ps[i] == nil {
			return nil
		}
	}
	var runs [][]*Node
	for _, first := range v.find(ps[0], func(x *Node) bool { return v.e.Item(x) == x }) {
		run := []*Node{first}
		for _, p := range ps[1:] {
			next := v.e.Sibling(run[len(run)-1], 1)
			if next == nil || !Matches(p, next) {
				run = nil
				break
			}
			run = append(run, next)
		}
		if run != nil {
			runs = append(runs, run)
		}
	}
	if len(runs) != 1 {
		v.Die("%s -- the run of %d items occurs %d times, expected 1", what, len(pats), len(runs))
		return nil
	}
	return runs[0]
}

// oneItem is the one item of the scope pat matches.
func (v *Verbs) oneItem(pat, what string) *Node {
	p := v.pattern(pat, what)
	if p == nil {
		return nil
	}
	ms := v.find(p, func(x *Node) bool { return v.e.Item(x) == x })
	if len(ms) != 1 {
		v.Die("%s -- %d items match %s, expected 1", what, len(ms), pat)
		return nil
	}
	return ms[0]
}

// SpliceFirst replaces the run of items from the one item of the scope from
// matches through the first item at or after it, in its list, that through
// matches, by the template -- the text's splice from a line to the first
// line of a shape after it.
func (v *Verbs) SpliceFirst(from, through, tmpl, what string) {
	if v.Err != nil {
		return
	}
	first := v.oneItem(from, what)
	if first == nil {
		return
	}
	p := v.pattern(through, what)
	if p == nil {
		return
	}
	last := first
	for !Matches(p, last) {
		if last = v.e.Sibling(last, 1); last == nil {
			v.Die("%s -- no item after %s matches %s", what, label(first), through)
			return
		}
	}
	fb, _ := Match(v.pattern(from, what), first)
	lb, _ := Match(p, last)
	for k, x := range lb {
		fb[k] = x
	}
	with, err := v.e.Build(first, tmpl, fb)
	if err == nil {
		err = v.e.ReplaceRun(first, last, with...)
	}
	if err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}

// DropCaseRun is DropCase judging the statement before a label by whether
// control can run off its end (StmtTerminates: a block ending in a jump
// cannot), where DropCase asks only whether it is a jump itself.
func (v *Verbs) DropCaseRun(pat string, n int, what string) {
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
		if i > 0 && v.e.place(p, i-1) == placeItem && !StmtTerminates(ks[i-1]) {
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

// EndLabels gives every label that a cut left last in its block, in the
// tree under root, the null statement the text's canonical print puts there
// (`theend:` then `;`), as the import of that text has it: `(label x)
// (empty)`.  It says how many it gave.
func (e *Editor) EndLabels(root *Node) (int, error) {
	var lasts []*Node
	Walk(root, func(l *Node) bool {
		if lo := itemsFrom(l); lo >= 0 && len(l.Kids) > lo {
			if last := l.Kids[len(l.Kids)-1]; last.Is("label") && e.Live(last) {
				lasts = append(lasts, last)
			}
		}
		return true
	})
	for _, x := range lasts {
		if err := e.InsertAfter(x, NewList(NewAtom("empty"))); err != nil {
			return 0, err
		}
	}
	return len(lasts), nil
}

// DropOperandAsText is DropOperand as a text cut makes it: where one operand
// is left, it keeps the parentheses the C view wrote around it in the
// operator it stood in (`(a, b) && c` less `c` is `(a, b)`), which a text
// cut of ` && c` leaves and DropOperand does not.
func (v *Verbs) DropOperandAsText(pat string, n int, what string) {
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
			return v.e.Replace(q, carried(v.e.parenthesised(rest[0]), rest[0]))
		}
		r := NewList(append([]*Node{NewAtom(q.Head())}, rest...)...)
		if !q.Is("|") {
			r.Type = q.Type // && and || are int whatever their operands
		}
		return v.e.Replace(q, r)
	}) {
		v.Say(what)
	}
}

// HeadFold is a text fold of a line head, `if (C)` or `else if (C)`: the
// ifs whose condition cond matches that are an else's arm (arm) or are not
// (!arm), as the text's head told them apart, folded ONE AT A TIME FROM
// THE LAST in the scope -- the last has no copy inside it, so a condition
// that repeats inside its own block is folded inner first, as the text did.
// kind is "never" (FoldNever), "always" (FoldAlways) or "drop" (DropIf).
// n < 0 folds as many as there are, at least one, and reports how many
// where that is not 1: `what (3)`.
func (v *Verbs) HeadFold(kind string, arm bool, cond string, n int, what string) {
	if v.Err != nil {
		return
	}
	p := v.pattern(cond, what)
	if p == nil {
		return
	}
	find := func() []*Node {
		return v.find(p, func(x *Node) bool {
			q := v.e.Parent(x)
			s := q
			if s == nil || !s.Is("if") || s.Kids[1] != x {
				return false
			}
			return v.arm(s) == arm
		})
	}
	count := n
	if count < 0 {
		count = len(find())
		if count == 0 {
			v.Die("%s -- no occurrence", what)
			return
		}
	} else if k := len(find()); k != count {
		v.Die("%s -- matched %d times, expected %d -- a fold that is not counted is a guess", what, k, count)
		return
	}
	q := &Verbs{Tag: v.Tag, W: io.Discard, e: v.e, batch: v.batch}
	for i := 0; i < count; i++ {
		ms := find()
		if len(ms) == 0 {
			v.Die("%s -- list index out of range", what)
			return
		}
		s := v.e.Parent(ms[len(ms)-1])
		q.scope = s
		switch kind {
		case "never":
			q.FoldNever(cond, 1, what)
		case "always":
			q.FoldAlways(cond, 1, what)
		case "drop":
			q.DropIf(cond, 1, what)
		default:
			q.Die("%s -- no fold %q", what, kind)
		}
		if q.Err != nil {
			v.Err = q.Err
			return
		}
	}
	if n < 0 && count != 1 {
		v.Sayf("%s (%d)", what, count)
	} else {
		v.Say(what)
	}
}
