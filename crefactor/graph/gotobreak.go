package graph

import "fmt"

// GOTOBREAK (doc/GRAPH-MIGRATION.md, B3g): crefactor/xform's GotoBreak on
// the nodes.  The gotos a structured statement already says:
//
//   - a goto to the label control reaches next anyway -- it would fall
//     through to it, leaving only blocks, ifs, labels and the end of a
//     switch -- goes (after a label, `;` stands in its place);
//   - a goto whose label is where control goes when the innermost loop or
//     switch around it ends is `break;`.
//
// A label no goto reaches any more goes.  A goto that leaves more than one
// loop or switch to reach its label is held: C has no break for it.  A
// function with a computed goto, a label's address, a local label, a nested
// function or a jump inside an expression is left as it is.

// BreakReport is what one run of GotoBreak did.
type BreakReport struct {
	Breaks, Falls, Held, Labels, Odd int
}

// Line is the report as the text step wrote it.
func (r BreakReport) Line() string {
	return fmt.Sprintf("%d gotos are break; %d gotos to where control falls anyway go; %d held, leaving more than one loop or switch; %d labels go; %d functions left, doing what the walk does not model",
		r.Breaks, r.Falls, r.Held, r.Labels, r.Odd)
}

// GotoBreak takes the gotos that are a break, or nothing.
func (e *Editor) GotoBreak() (BreakReport, error) {
	var r BreakReport
	type act struct {
		jump *Node
		with *Node // nil: the goto goes
	}
	var acts []act
	var labels []*Node
	for _, fd := range e.gfFunctions() {
		fl := gfFlowOf(fd)
		if fl.odd {
			r.Odd++
			continue
		}
		took := map[string]int{}
		var order []string
		for _, j := range fl.jumps {
			if !j.n.Is("goto") {
				continue
			}
			name := labelName(j.n)
			if f, ok := gfNext(j.stack); ok && gfMarks(f, name) {
				// control falls to the label: the goto goes, and after a
				// label `;` keeps the label's statement
				var with *Node
				if last := j.stack[len(j.stack)-1]; labeledAt(last.items, last.idx) {
					with = NewList(NewAtom("empty"))
				}
				acts = append(acts, act{j.n, with})
				if took[name] == 0 {
					order = append(order, name)
				}
				took[name]++
				r.Falls++
				continue
			}
			b := gfBreakable(j.stack)
			if b < 0 {
				continue
			}
			if f, ok := gfNext(j.stack[:b]); ok && gfMarks(f, name) {
				acts = append(acts, act{j.n, Break()})
				if took[name] == 0 {
					order = append(order, name)
				}
				took[name]++
				r.Breaks++
				continue
			}
			for o := gfBreakable(j.stack[:b]); o >= 0; o = gfBreakable(j.stack[:o]) {
				if f, ok := gfNext(j.stack[:o]); ok && gfMarks(f, name) {
					r.Held++
					break
				}
			}
		}
		for _, name := range order {
			if took[name] == fl.gotos[name] {
				if l, ok := fl.labels[name]; ok {
					labels = append(labels, l.n)
					r.Labels++
				}
			}
		}
	}
	for _, a := range acts {
		var err error
		if a.with == nil {
			err = e.Delete(a.jump)
		} else {
			err = e.Replace(a.jump, a.with)
		}
		if err != nil {
			return r, err
		}
	}
	for _, l := range labels {
		if err := e.Delete(l); err != nil {
			return r, err
		}
	}
	return r, nil
}
