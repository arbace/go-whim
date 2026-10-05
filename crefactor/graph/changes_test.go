package graph

import (
	"slices"
	"strings"
	"testing"
)

// TestCompare: a constant changed in f moves f and its callers, a member's
// type moves the struct and, by content, its users -- by name, only the
// struct -- and a function taken out (with its call) is removed; and a
// function added is added, though every id after it moved: the forms are
// paired by their labels where their ids do not agree.
func TestCompare(t *testing.T) {
	labels := func(cs []Change) []string {
		var l []string
		for _, c := range cs {
			l = append(l, c.Label)
		}
		slices.Sort(l)
		return l
	}
	_, _, a := importSample(t, hashSample)
	for _, c := range []struct {
		name                    string
		from, to                string
		opt                     HashOptions
		added, removed, changed []string
	}{
		{name: "a constant", from: "int c = 4;", to: "int c = 5;",
			changed: []string{"defn f", "defn g", "defn main"}},
		{name: "a member's type", from: "long b;", to: "short b;",
			changed: []string{"defn k", "defn main", "struct S"}},
		{name: "a member's type, nominal", from: "long b;", to: "short b;", opt: HashOptions{Nominal: true},
			changed: []string{"struct S"}},
		{name: "a function out", from: "static int h(void) { return 7; }", to: "",
			removed: []string{"defn h"}, changed: []string{"defn main"}},
		{name: "a function in", from: "static int k(", to: "static int z(void) { return 0; }\nstatic int k(",
			added: []string{"defn z"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := strings.Replace(hashSample, c.from, c.to, 1)
			if c.name == "a function out" {
				src = strings.Replace(src, " + h()", "", 1)
			}
			_, _, b := importSample(t, src)
			x, err := Compare(a, b, c.opt)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range []struct {
				what      string
				got, want []string
			}{{"added", labels(x.Added), c.added}, {"removed", labels(x.Removed), c.removed}, {"changed", labels(x.Changed), c.changed}} {
				if !slices.Equal(w.got, w.want) {
					t.Errorf("%s %v, want %v", w.what, w.got, w.want)
				}
			}
		})
	}
}
