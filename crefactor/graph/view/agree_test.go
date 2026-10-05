package view

import (
	"errors"
	"strings"
	"testing"
)

// CALLS AND RETURNS under agreement, and top-level forms beside a def
// view's (doc/GRAPH.md, *Calls, returns and forms beside*).
func TestAgreeCallsReturns(t *testing.T) {
	g := sample(t).G
	for _, c := range []struct {
		name, view, old, new, why string
	}{
		{"too many arguments", "main", "(call fact 3)", "(call fact 3 4)", "passes 2 arguments where the prototype takes 1"},
		{"too few", "main", "(call fact 3)", "(call fact)", "passes 0 arguments where the prototype takes 1"},
		{"a struct for an int", "main", "(call fact 3)", "(call fact b)", "argument 1 stores a value of struct buf in arith"},
		{"an int for a pointer", "main", "(call get (addr b) (call fact 3))", "(call get 7 (call fact 3))", "argument 1 stores a value of arith in pointer"},
		{"through a pointer to a function", "main", "(call (index table 0) 2)", "(call (index table 0) 2 2)", "passes 2 arguments"},
		{"a return without a value", "get", "(return 0)", "(return)", "returns no value from a function returning arith"},
		{"a return of a pointer from an int function", "get", "(return 0)", "(return b)", "stores a value of pointer in arith"},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := defView(c.view, false)
			_, err := Edit(g, v, edited(t, v, g, c.old, c.new), EditOptions{})
			var ref *Refusal
			if !errors.As(err, &ref) || !strings.Contains(err.Error(), c.why) {
				t.Fatalf("got %v, want a refusal saying %q", err, c.why)
			}
		})
	}
	// what agrees still stands
	for _, c := range []struct{ view, old, new string }{
		{"main", "(call fact 3)", "(call fact (call odd 3))"},
		{"main", "(call get (addr b) (call fact 3))", "(call get nullptr (call fact 3))"},
		{"get", "(return 0)", "(return (-> b b_ml))"},
	} {
		v := defView(c.view, false)
		if _, err := Edit(g, v, edited(t, v, g, c.old, c.new), EditOptions{}); err != nil {
			t.Fatalf("%s -> %s: %v", c.old, c.new, err)
		}
	}
}

// TestTopBeside: a function typed after the one a def view shows is a
// top-level form of its own, inserted after it; the view printed again
// shows the definition alone, so the edit is held to the C.
func TestTopBeside(t *testing.T) {
	g := sample(t).G
	s := NewSession(g)
	v := defView("fact", false)
	b, err := s.Open(v, EditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	add := "\n(defn static twice (fn ((n int)) int) (return (* 2 (call fact n))))"
	st, r := typeInto(t, b, len(strings.TrimRight(b.Text, "\n")), add)
	if r.Status != "applied" && !strings.Contains(strings.Join(st, " "), "applied") {
		t.Fatalf("statuses %v (%s)", st, b.Reason)
	}
	c, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	fact := strings.Index(string(c), "\nfact(int n)")
	twice := strings.Index(string(c), "\ntwice(int n)")
	if fact < 0 || twice < fact {
		t.Fatalf("twice not after fact:\n%s", c)
	}
	imported(t, g)
	// beside a uses view's entries it is still refused (TestEditRefusals)
}
