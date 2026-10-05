package view

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
)

// THE VIEW AT THE CURSOR, AND UNDO (doc/GRAPH.md).

// posOf is the byte where the nth (from 0) occurrence of s begins in text,
// plus off.
func posOf(t *testing.T, text, s string, nth, off int) int {
	t.Helper()
	at := -1
	for i := 0; i <= nth; i++ {
		k := strings.Index(text[at+1:], s)
		if k < 0 {
			t.Fatalf("no %q (%d) in:\n%s", s, nth, text)
		}
		at += 1 + k
	}
	return at + off
}

func TestCursorSample(t *testing.T) {
	ix := sample(t)
	text, spans, err := defView("main", false)(ix)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name   string
		at     string
		nth    int
		off    int
		entity string // what the view at the cursor is rooted at
	}{
		{"a call's callee", "(call fact 3)", 0, len("(call "), "fact"},
		{"a call's head", "(call fact 3)", 0, 1, "fact"},
		{"a call's argument, no entity of its own", "(call fact 3)", 0, len("(call fact "), "fact"},
		{"a global", "(= opt 1)", 0, len("(= "), "opt"},
		{"a member", "(. o b_ml)", 0, len("(. o "), "struct other.b_ml"},
		{"a local", "(= opt 1)", 0, 0, "main"}, // an assignment's head: the statement's function
		{"a typedef in a type", "buf_T", 0, 0, "buf_T"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, on := EntityAt(ix, spans, posOf(t, text, c.at, c.nth, c.off))
			if e == nil {
				t.Fatalf("no entity at %q+%d", c.at, c.off)
			}
			want := root(t, ix, strings.TrimPrefix(c.entity, "struct "))
			if e != want {
				t.Fatalf("on #%d (%s): entity #%d %s, want #%d %s", on.ID, label(on), e.ID, ix.Name(e), want.ID, ix.Name(want))
			}
		})
	}
	// nothing at a position outside every span
	if e, _ := EntityAt(ix, spans, len(text)+1); e != nil {
		t.Fatalf("an entity past the text: #%d", e.ID)
	}
	// the view re-rooted there: the callers of fact, from a cursor in main
	e, _ := EntityAt(ix, spans, posOf(t, text, "(call fact 3)", 0, len("(call ")))
	got := Printer{}.Tree(ix, Callers(ix, e, Options{}))
	want := Printer{}.Tree(ix, Callers(ix, root(t, ix, "fact"), Options{}))
	if got != want {
		t.Fatalf("re-rooted:\n%s\nwant:\n%s", got, want)
	}
}

// TestSessionSample: edits made in place, each the edit Edit makes on a
// copy, then undone the other way round, the graph each time what it was.
func TestSessionSample(t *testing.T) {
	g := sample(t).G
	s := NewSession(g)
	steps := []struct {
		view     Render
		old, new string
	}{
		{defView("get", false), "(+= (-> b b_ml) 2)", "(+= (-> b b_ml) 3)"},
		{defView("get", false), "(n int)) int)\n  (= (-> b b_ml) n)", "(m int)) int)\n  (= (-> b b_ml) m)"},
		{defView("main", false), "(call fact 3)", "(call fact (+ 3 1))"},
		{usesView("opt", false), "\n    (:write (= opt 1))", ""},
		{defView("get", false), "  (post++ (-> b b_ml))\n", "  (post++ (-> b b_ml))\n  (def k int 3)\n  (+= opt k)\n"},
	}
	var was [][]byte
	for i, c := range steps {
		ed := edited(t, c.view, g, c.old, c.new)
		want, err := Edit(g, c.view, ed, EditOptions{}) // on a copy
		if err != nil {
			t.Fatalf("step %d on a copy: %v", i, err)
		}
		was = append(was, g.Lisp())
		r, err := s.Edit(c.view, ed, EditOptions{})
		if err != nil {
			t.Fatalf("step %d in place: %v", i, err)
		}
		if r.Graph != g {
			t.Fatalf("step %d: not edited in place", i)
		}
		if err := graph.Equal(g, want.Graph); err != nil {
			t.Fatalf("step %d: in place, not the copy's edit: %v", i, err)
		}
		if opText(r.Ops) != opText(want.Ops) {
			t.Fatalf("step %d: ops %s, on the copy %s", i, opText(r.Ops), opText(want.Ops))
		}
		imported(t, g)
	}
	// a refusal in place: the graph as it was, nothing to undo
	before, n := g.Lisp(), s.Edits()
	if _, err := s.Edit(defView("get", false), edited(t, defView("get", false), g, "(+= (-> b b_ml) 3)", "(+= (-> b b_ml) nope)"), EditOptions{}); err == nil {
		t.Fatal("an undeclared name was not refused")
	}
	if !bytes.Equal(before, g.Lisp()) || s.Edits() != n {
		t.Fatal("a refused edit in place changed the graph")
	}
	for i := len(steps) - 1; i >= 0; i-- {
		if !s.Undo() {
			t.Fatalf("no edit to undo at step %d", i)
		}
		if !bytes.Equal(g.Lisp(), was[i]) {
			t.Fatalf("step %d undone: the graph is not what it was", i)
		}
	}
	if s.Undo() {
		t.Fatal("an undo past the first edit")
	}
	// and the graph edits again after the undos, as it did the first time
	if _, err := s.Edit(steps[0].view, edited(t, steps[0].view, g, steps[0].old, steps[0].new), EditOptions{}); err != nil {
		t.Fatalf("an edit after the undos: %v", err)
	}
}

// TestSessionProduct: the same on whim-vim.c's graph, timed against the
// edits on a copy, and the view at a cursor in it.
func TestSessionProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	ix, _ := product(t)
	g := ix.G
	const fn = "ml_clearmarked"
	def := defView(fn, false)

	text, spans, _ := def(NewIndex(g))
	start := time.Now()
	e, on := EntityAt(ix, spans, posOf(t, text, "lowest_marked", 0, 0))
	at := Printer{}.Tree(ix, Uses(ix, e, Options{}))
	t.Logf("at the cursor: #%d (%s) -> %s, its uses view %d bytes: %v", on.ID, label(on), ix.Name(e), len(at), time.Since(start))
	if e != root(t, ix, "lowest_marked") {
		t.Fatalf("the cursor's entity is %s", ix.Name(e))
	}

	s := NewSession(g)
	fo := graph.FallOutOptions{KeepEmpty: true}
	steps := []struct {
		name string
		ed   func() string
		opt  EditOptions
	}{
		{"a constant", func() string { return edited(t, def, g, "(= lowest_marked 0))", "(= lowest_marked 1))") }, EditOptions{}},
		{"a local renamed", func() string {
			text, _, _ := def(NewIndex(g))
			return strings.NewReplacer("(def i int)", "(def k int)", "(= i ", "(= k ", "(pre++ i)", "(pre++ k)", "db_line) i)", "db_line) k)").Replace(text)
		}, EditOptions{}},
		{"the function deleted, its calls closed over", func() string { return "" }, EditOptions{FallOut: &fo}},
	}
	var was [][]byte
	for _, c := range steps {
		ed := c.ed()
		t0 := time.Now()
		want, err := Edit(g, def, ed, c.opt)
		onCopy := time.Since(t0)
		if err != nil {
			t.Fatalf("%s, on a copy: %v", c.name, err)
		}
		was = append(was, g.Lisp())
		t0 = time.Now()
		r, err := s.Edit(def, ed, c.opt)
		inPlace := time.Since(t0)
		if err != nil {
			t.Fatalf("%s, in place: %v", c.name, err)
		}
		if err := graph.Equal(g, want.Graph); err != nil {
			t.Fatalf("%s: in place, not the copy's edit: %v", c.name, err)
		}
		t.Logf("%s: %s, %d nodes saved; in place %v, on a copy %v", c.name, opText(r.Ops), r.Journal.Changed(), inPlace, onCopy)
	}
	// a refusal in place
	before := g.Lisp()
	_, err := s.Edit(defView("ml_append_int", false), "", EditOptions{})
	var ref *Refusal
	if !errors.As(err, &ref) {
		t.Fatalf("deleting a function still called: %v, want a refusal", err)
	}
	if !bytes.Equal(before, g.Lisp()) {
		t.Fatal("a refused edit in place changed the graph")
	}
	for i := len(steps) - 1; i >= 0; i-- {
		t0 := time.Now()
		s.Undo()
		d := time.Since(t0)
		if !bytes.Equal(g.Lisp(), was[i]) {
			t.Fatalf("%s undone: the graph is not what it was", steps[i].name)
		}
		t.Logf("%s undone: %v", steps[i].name, d)
	}
	syntaxOK(t, imported(t, g))
}
