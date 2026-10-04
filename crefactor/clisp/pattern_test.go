package clisp

import (
	"sort"
	"strings"
	"testing"
)

// TestShapedBinding: `?name:P` binds a node only where P matches it -- P a
// list or an atom, nested, beside `_*`, a second ?name equal to the first --
// and Subst puts the bound node back; a `?name:` with nothing after it, or
// with `_*`, is refused.
func TestShapedBinding(t *testing.T) {
	read := func(src string) *Node {
		fs, err := Read([]byte(src))
		if err != nil || len(fs) != 1 {
			t.Fatalf("%q: %v", src, err)
		}
		return fs[0]
	}
	cases := []struct {
		pat, form string
		want      string // the bindings, name=form, sorted; "-" no match
	}{
		{"(call ?f:(paren _) _*)", "(call (paren (* p)) 1 2)", "f=(paren (* p))"},
		{"(call ?f:(paren _) _*)", "(call g 1 2)", "-"},
		{"(if ?c:(== _ nullptr) _*)", "(if (== p nullptr) (block))", "c=(== p nullptr)"},
		{"(if ?c:(== _ nullptr) _*)", "(if (!= p nullptr) (block))", "-"},
		{"(if ?c:(== _ nullptr) _*)", "(if (== p 0) (block))", "-"},
		// an atom as the shape
		{"(= ?x:errno 0)", "(= errno 0)", "x=errno"},
		{"(= ?x:errno 0)", "(= err 0)", "-"},
		// nested: a shaped binding inside a shape, with its own binding
		{"(&& ?a:(== ?v:_ 0) ?b:(!= ?v _))", "(&& (== x 0) (!= x 1))", "a=(== x 0) b=(!= x 1) v=x"},
		{"(&& ?a:(== ?v:_ 0) ?b:(!= ?v _))", "(&& (== x 0) (!= y 1))", "-"},
		// beside _*, and the whole pattern a shaped binding
		{"?s:(call free _*)", "(call free p)", "s=(call free p)"},
		{"?s:(call free _*)", "(call vim_free p)", "-"},
		// a second ?name with a shape must equal the first
		{"(+ ?x:(* _ 2) ?x:(* _ 2))", "(+ (* a 2) (* a 2))", "x=(* a 2)"},
		{"(+ ?x:(* _ 2) ?x:(* _ 2))", "(+ (* a 2) (* b 2))", "-"},
		// an atom where the shape wants a list
		{"(return ?r:(cast _ _))", "(return r)", "-"},
	}
	for _, c := range cases {
		p, err := Pattern(c.pat)
		if err != nil {
			t.Fatalf("%s: %v", c.pat, err)
		}
		b, ok := Match(p, read(c.form))
		got := "-"
		if ok {
			var kv []string
			for k, v := range b {
				kv = append(kv, k+"="+v.String())
			}
			sort.Strings(kv)
			got = strings.Join(kv, " ")
		}
		if got != c.want {
			t.Errorf("%s on %s: %s, want %s", c.pat, c.form, got, c.want)
		}
		if Matches(p, read(c.form)) != (c.want != "-") {
			t.Errorf("%s on %s: Matches disagrees with Match", c.pat, c.form)
		}
	}
	// Subst: the shaped binding is its node
	p := MustPattern("(call g ?f:(paren _))")
	b, _ := Match(p, read("(call h (paren (+ a 1)))"))
	if b != nil {
		t.Fatalf("matched a call of h: %v", b)
	}
	b, ok := Match(p, read("(call g (paren (+ a 1)))"))
	if !ok {
		t.Fatal("no match")
	}
	if s := Subst(MustPattern("(return ?f:(paren _))"), b).String(); s != "(return (paren (+ a 1)))" {
		t.Errorf("Subst: %s", s)
	}
	for _, bad := range []string{"(call ?f:)", "(call ?f: _*)", "(call ?f:_*)"} {
		if _, err := Pattern(bad); err == nil {
			t.Errorf("%s: read", bad)
		}
	}
}
