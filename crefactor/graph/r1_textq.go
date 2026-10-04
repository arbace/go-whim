package graph

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// TEXTQ, a name's lines (doc/GRAPH-MIGRATION.md, *R1 as built*): a text
// program that partitioned the lines saying a name -- each line in exactly
// one class of its regular expressions, the classes counted -- asks the
// same of the lines of the forms that say it, found by edge: the name's
// declarations and the nodes that refer to them, and the top-level forms
// holding those, printed as the C view prints them.  A form prints in well
// under a millisecond, so a partition costs what its few forms cost, not
// the file's 60-90 ms; what it cannot see is a mention no edge makes (a
// string's or a macro's text), which these programs assert there is none
// of.

// A FormLine is a line of a top-level form's C view, and the function it
// is in: "" for the file scope -- a declaration, or a definition's lines
// above its name's, as a text program reading heads at column 0 saw them.
type FormLine struct {
	Text string
	Func string
}

// FormLines are the lines of the top-level forms holding any of ns, in the
// file's order, each form once.
func (e *Editor) FormLines(ns ...*Node) ([]FormLine, error) {
	want := map[*Node]bool{}
	for _, n := range ns {
		if n == nil || !e.Live(n) {
			continue
		}
		if t := e.TopForm(n); t != nil {
			want[t] = true
		}
	}
	var out []FormLine
	for _, f := range e.g.Forms {
		if !want[f] {
			continue
		}
		c, err := FormsC([]*Node{f})
		if err != nil {
			return nil, fmt.Errorf("form lines: %v", err)
		}
		ls := strings.Split(strings.TrimRight(string(c), "\n"), "\n")
		fn := ""
		name := ""
		if f.Is("defn") {
			name = topName(f)
		}
		for _, l := range ls {
			if name != "" && fn == "" && strings.HasPrefix(l, name+"(") {
				fn = name
			}
			out = append(out, FormLine{Text: l, Func: fn})
		}
	}
	return out, nil
}

// MemberDecls are the members named name of the structs and unions the
// file defines.
func (e *Editor) MemberDecls(name string) []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		if f.Is("defn") {
			continue
		}
		Walk(f, func(x *Node) bool {
			if x.Is("struct") || x.Is("union") {
				for _, m := range members(x) {
					if m.Head() == name {
						out = append(out, m)
					}
				}
			}
			return true
		})
	}
	return out
}

// LocalDecls are the declarations named name in the function fn: its
// parameters and its locals.
func (e *Editor) LocalDecls(fn *Node, name string) []*Node {
	var out []*Node
	if t := defType(fn); t != nil && t.Is("fn") && len(t.Kids) > 1 {
		for _, p := range t.Kids[1].Kids {
			if p.list && len(p.Kids) > 0 && !p.Kids[0].list && p.Kids[0].Atom == name {
				out = append(out, p)
			}
		}
	}
	for _, it := range Body(fn) {
		Walk(it, func(x *Node) bool {
			if x.Is("def") && topName(x) == name {
				out = append(out, x)
			}
			return true
		})
	}
	return out
}

// AndUses are ds and the nodes that refer to one of them, in no order.
func (e *Editor) AndUses(ds ...*Node) []*Node {
	out := append([]*Node(nil), ds...)
	for _, d := range ds {
		out = append(out, e.Uses(d)...)
	}
	return out
}

// ItemsC is the C of a run of items, as the C view prints them in a body.
func ItemsC(items []*Node) (string, error) {
	ls := make([]*clisp.Node, len(items))
	for i, it := range items {
		ls[i] = Lisp(it)
	}
	return clisp.PrintItems(ls)
}
