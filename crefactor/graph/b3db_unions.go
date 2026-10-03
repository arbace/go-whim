package graph

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// THE DEGENERATE UNIONS on the graph: a member whose type is an anonymous
// union of ONE member unions nothing with anything, so the member takes
// that member's type and every `x.m.only` access is `x.m`.  The text step
// (crefactor/xform's Unions, which this replaces) found the unions by
// their keyword and the accesses by `m.only` in the text; here a union is
// a `(union ...)` form, an access is a use of the member by edge whose
// selection is selected from by the union's own member, and the rewrite is
// a Replace of the outer selection by the inner one (its node moved, its
// id kept) and a RETYPE of the member.  The partition the text asserted --
// every mention of the member's name is its declaration or such an access
// -- is asserted both ways: by the edges, and by the text's own count on
// the C view, so that a string, a local or a macro text spelling the name
// refuses as it did.

var unionWord = regexp.MustCompile(`\bunion\b`)

type b3dbUnion struct {
	form    *Node // the (union ...) type form
	member  *Node // the member it is the type of
	name    string
	members int
}

// DegenerateUnions rewrites every one-member union field of the core as
// its member, reported under v's tag as the text step was; minDegenerate
// and minGenuine are the least the scan must find of each, so that a scan
// that stopped matching cannot pass by finding nothing.
func (v *Verbs) DegenerateUnions(minDegenerate, minGenuine int) {
	if v.Err != nil {
		return
	}
	e := v.e
	incs, at, err := e.SystemIncludeRun()
	if err != nil {
		v.Die("%v", err)
		return
	}
	t := v.Text()
	lines := strings.Split(string(t), "\n")
	bound := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "#") {
			bound = i
			break
		}
	}
	v.Sayf("%d directives on consecutive lines from %d, every one an `#include <...>`; the core "+
		"is the %d lines above the first of them", len(incs), bound+1, bound)
	spans, err := edit.LiteralSpans(edit.Ph{Tag: v.Tag}, t)
	if err != nil {
		v.Die("%v", err)
		return
	}
	for _, s := range spans {
		if unionWord.Match(t[s[0]:s[1]]) {
			v.Die("a literal holds the word `union` at line %d, and this step does "+
				"not know it is data", strings.Count(string(t[:s[0]]), "\n")+1)
			return
		}
	}
	v.Sayf("%d string and character literals scanned, so every span below is over code", len(spans))

	var unions []*b3dbUnion
	for i, f := range e.g.Forms {
		Walk(f, func(n *Node) bool {
			if v.Err != nil || !n.Is("union") {
				return v.Err == nil
			}
			if i >= at {
				v.Die("a union is defined below the boundary, in the host, and this step is about the " +
					"CORE")
				return false
			}
			if len(n.Kids) > 1 && !n.Kids[1].list {
				v.Die("the union `%s` is a union type named rather than defined, and the scanner "+
					"cannot classify it", label(n))
				return false
			}
			m := e.Parent(n)
			if m == nil || !isMemberForm(e, m) || len(m.Kids) < 2 || m.Kids[1] != n || m.Kids[0].list {
				v.Die("the union #%d is not declared as one named field, and this step rewrites only "+
					"a union declared as one", n.ID)
				return false
			}
			unions = append(unions, &b3dbUnion{form: n, member: m, name: m.Kids[0].Atom, members: len(n.Kids) - 1})
			return true
		})
		if v.Err != nil {
			return
		}
	}
	var degenerate, genuine []*b3dbUnion
	for _, u := range unions {
		if u.members < 2 {
			degenerate = append(degenerate, u)
		} else {
			genuine = append(genuine, u)
		}
	}
	if len(degenerate) < minDegenerate || len(genuine) < minGenuine {
		v.Die("the scan found %d degenerate unions and %d genuine ones, and it was told to find "+
			"at least %d and %d -- a scanner that stopped matching would otherwise pass by finding nothing",
			len(degenerate), len(genuine), minDegenerate, minGenuine)
		return
	}
	gs := make([]string, len(genuine))
	for i, u := range genuine {
		gs[i] = fmt.Sprintf("%s (%d)", u.name, u.members)
	}
	v.Sayf("%d `union` keywords in the core, every one of them a `union { ... } <name>;` field: "+
		"%d with fewer than two members and %d with two or more.  THOSE THAT STAY ARE "+
		"DOING THE JOB A UNION IS FOR: %s",
		len(unions), len(degenerate), len(genuine), strings.Join(gs, ", "))
	ds := make([]string, len(degenerate))
	for i, u := range degenerate {
		if u.members == 0 {
			v.Die("`%s` is an EMPTY union, and nothing can name a field with no member: it "+
				"is the sweep's, which takes an unused member, and not this edit's", u.name)
			return
		}
		ds[i] = fmt.Sprintf("%s (1 member)", u.name)
	}
	v.Sayf("THE %d THAT GO UNION NOTHING WITH ANYTHING: %s", len(degenerate), strings.Join(ds, ", "))

	// the partition, by edge and by the text's count
	type access struct {
		chain *Node // the selection chain the one member is selected in
		at    int   // its atom's place there
	}
	var report []string
	accesses := map[*b3dbUnion][]access{}
	nAcc := 0
	for _, u := range degenerate {
		only := u.form.Kids[1]
		if len(only.Kids) != 2 || only.Kids[0].list {
			v.Die("the single member of `%s` is not one `<type> <name>;`: %s", u.name, Lisp(only))
			return
		}
		var leftover []string
		for _, use := range e.Uses(u.member) {
			sel := e.Parent(use)
			outer := e.Parent(sel)
			// a selection chain is one form, `a->b->m` or `a.b.m`: the
			// member's own selection drops its one member's atom, in the
			// same chain (`x.m.only`) or the chain around it (`x->m.only`)
			i := -1
			if sel != nil && (sel.Is(".") || sel.Is("->")) {
				for k, x := range sel.Kids {
					if x == use {
						i = k
					}
				}
			}
			switch {
			case i < 2:
				leftover = append(leftover, fmt.Sprintf("#%d %s", use.ID, label(sel)))
			case i+1 < len(sel.Kids):
				if !sel.Is(".") || sel.Kids[i+1].Ref() != only {
					leftover = append(leftover, fmt.Sprintf("#%d %s", sel.ID, label(sel)))
				} else {
					accesses[u] = append(accesses[u], access{sel, i + 1})
				}
			case outer == nil || !outer.Is(".") || len(outer.Kids) < 3 || outer.Kids[1] != sel || outer.Kids[2].Ref() != only:
				leftover = append(leftover, fmt.Sprintf("#%d %s", sel.ID, label(outer)))
			default:
				accesses[u] = append(accesses[u], access{outer, 2})
			}
		}
		if n := len(e.Uses(only)); n != len(accesses[u]) {
			leftover = append(leftover, fmt.Sprintf("%d uses of `.%s` that select from no `%s`", n-len(accesses[u]), only.Kids[0].Atom, u.name))
		}
		if c := edit.MentionCount(t, u.name); c != 1+len(accesses[u]) {
			leftover = append(leftover, fmt.Sprintf("%d mentions of the name in the text", c))
		}
		if len(leftover) > 0 {
			v.Die("`%s` has a mention that is neither its own declaration nor a `.%s` access "+
				"on it, so this step may not rewrite it: %s",
				u.name, only.Kids[0].Atom, strings.Join(edit.First(leftover, 4), "; "))
			return
		}
		if len(accesses[u]) == 0 {
			v.Die("`%s` has no `.%s` access anywhere, so the field this step would promote is "+
				"read by nothing and belongs to the sweep and not to this edit", u.name, only.Kids[0].Atom)
			return
		}
		nAcc += len(accesses[u])
		report = append(report, fmt.Sprintf("%s 1 + %d", u.name, len(accesses[u])))
	}
	v.Sayf("THE PARTITION HOLDS FOR ALL %d: every mention outside a literal is the "+
		"declaration or a `.member` access on it, and there is nothing else -- %s",
		len(degenerate), strings.Join(report, ", "))

	// the member retyped first, so that each selection the rewrite drops
	// has the type of what takes its place, and nothing above it changes
	for _, u := range degenerate {
		if _, err := e.Retype(u.member, Lisp(u.form.Kids[1].Kids[1]).String()); err != nil {
			v.Die("%v", err)
			return
		}
		for _, a := range accesses[u] {
			if err := b3dbDropSelection(e, a.chain, a.at, u.form.Kids[1]); err != nil {
				v.Die("%v", err)
				return
			}
		}
	}
	v.Sayf("%d spans rewritten in ONE pass over the original text: %d declarations and %d "+
		"`.member` accesses", len(degenerate)+nAcc, len(degenerate), nAcc)

	after := v.Text()
	L := strings.Split(string(after), "\n")
	if left := len(unionWord.FindAll(after, -1)); left != len(genuine) {
		v.Die("the file has %d `union` keywords and the %d genuine ones are what must remain",
			left, len(genuine))
		return
	}
	for _, u := range degenerate {
		if n, want := edit.MentionCount(after, u.name), 1+len(accesses[u]); n != want {
			v.Die("`%s` has %d mentions and must have %d -- its own declaration and the %d "+
				"accesses that are now plain field references", u.name, n, want, len(accesses[u]))
			return
		}
		if regexp.MustCompile(`\b` + u.name + `\s*\.\s*` + u.form.Kids[1].Kids[0].Atom + `\b`).Match(after) {
			v.Die("a `%s.%s` access survives", u.name, u.form.Kids[1].Kids[0].Atom)
			return
		}
	}
	nincs, _, err := e.SystemIncludeRun()
	if err != nil || len(nincs) != len(incs) {
		v.Die("the file has %d directives and had %d: this step adds none and removes none (%v)",
			len(nincs), len(incs), err)
		return
	}
	nbound := -1
	for i, l := range L {
		if strings.HasPrefix(l, "#") {
			nbound = i
			break
		}
	}
	if nbound-bound != len(L)-len(lines) {
		v.Die("the boundary moved by %d lines and the file by %d: every line this step touches "+
			"is above the first `#include`", nbound-bound, len(L)-len(lines))
		return
	}
	runs := 0
	for i := 1; i < len(L); i++ {
		if L[i] == "" && L[i-1] == "" {
			runs++
		}
	}
	v.Sayf("`union` %d -> %d, %d -> %d lines, %d directives unmoved relative to the text, and "+
		"the blank-line runs unchanged at %d",
		len(unions), len(genuine), len(lines)-1, len(L)-1, len(nincs), runs)
}

// b3dbDropSelection takes the member atom at chain.Kids[at] out of the
// selection chain: the chain, if that leaves its object alone, else a new
// chain of the rest, its kids moved (their ids kept) and its typed edge the
// one member's type, which the member's now is.  A chain of one operator is
// one form, as the importer makes it: where what is left stands first in a
// chain of its own operator, the two are one chain.
func b3dbDropSelection(e *Editor, chain *Node, at int, only *Node) error {
	var r *Node
	if len(chain.Kids) == 3 {
		r = chain.Kids[1]
	} else {
		r = NewList(append(append([]*Node{}, chain.Kids[:at]...), chain.Kids[at+1:]...)...)
		r.Type = only.Type
	}
	p := e.Parent(chain)
	if p != nil && r.list && (r.Is(".") || r.Is("->")) && p.Head() == r.Head() && len(p.Kids) > 2 && p.Kids[1] == chain {
		m := NewList(append(append([]*Node{}, r.Kids...), p.Kids[2:]...)...)
		m.Type = p.Type
		return e.Replace(p, m)
	}
	return e.Replace(chain, r)
}
