package view

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
)

// EDITS AS THEY ARE TYPED (doc/GRAPH.md).

// typeInto types s into b at byte at, one byte a change, the cursor
// following each change (Typed.Cursor), and returns the statuses and the
// last change's Typed.
func typeInto(t *testing.T, b *Buffer, at int, s string) ([]string, *Typed) {
	t.Helper()
	var st []string
	var last *Typed
	for i := 0; i < len(s); i++ {
		r, err := b.Change(at, at, s[i:i+1])
		if err != nil {
			t.Fatal(err)
		}
		st = append(st, r.Status)
		last, at = r, r.Cursor
	}
	return st, last
}

// erase deletes the n bytes before at, one a change, as a backspace does.
func erase(t *testing.T, b *Buffer, at, n int) []string {
	t.Helper()
	var st []string
	for range n {
		r, err := b.Change(at-1, at, "")
		if err != nil {
			t.Fatal(err)
		}
		st = append(st, r.Status)
		at = r.Cursor
	}
	return st
}

func TestBufferSample(t *testing.T) {
	g := sample(t).G
	s := NewSession(g)
	b, err := s.Open(defView("get", false), EditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	start := b.Text

	t.Run("a constant retyped", func(t *testing.T) {
		at := strings.Index(b.Text, "(+= (-> b b_ml) 2)") + len("(+= (-> b b_ml) 2")
		if st := erase(t, b, at, 1); st[0] != "pending" {
			t.Fatalf("an operand erased: %v (%s)", st, b.Reason)
		}
		st, r := typeInto(t, b, at-1, "3")
		if st[0] != "applied" {
			t.Fatalf("typed: %v (%s)", st, b.Reason)
		}
		if got := opText(r.Result.Ops); got != "replace #83 with b->b_ml += 3;" {
			t.Fatalf("ops %s", got)
		}
		if !strings.Contains(b.Text, "(+= (-> b b_ml) 3)") || b.Text != b.Printed {
			t.Fatalf("the buffer after:\n%s", b.Text)
		}
	})
	t.Run("a statement typed in", func(t *testing.T) {
		at := strings.Index(b.Text, "  (+= (-> b b_ml) 3)")
		st, r := typeInto(t, b, at, "  (+= opt 1)\n")
		// pending until the form is whole, then applied once; the newline
		// after it is the view's own
		applied := strings.Count(strings.Join(st, " "), "applied")
		if applied != 1 || r.Status != "layout" && r.Status != "same" && r.Status != "applied" {
			t.Fatalf("statuses %v (%s)", st, b.Reason)
		}
		if !strings.Contains(b.Printed, "(+= opt 1)") {
			t.Fatalf("not inserted:\n%s", b.Printed)
		}
	})
	t.Run("undone", func(t *testing.T) {
		for b.S.Edits() > 0 {
			if ok, err := b.Undo(); !ok || err != nil {
				t.Fatal(ok, err)
			}
		}
		if b.Text != start {
			t.Fatalf("the buffer undone:\n%s\nwas:\n%s", b.Text, start)
		}
	})
}

// TestBufferTyping: typing through the states between two edits, on a
// call in main -- an argument made an expression, the callee renamed to
// another function -- each byte a change, the graph untouched while the
// text is pending, and each applied edit the one Edit makes of the whole
// text on a copy.
func TestBufferTyping(t *testing.T) {
	g := sample(t).G
	s := NewSession(g)
	b, err := s.Open(defView("main", false), EditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, what string, st []string, r *Typed, wantOps string) {
		t.Helper()
		if r.Status != "applied" {
			t.Fatalf("%s: the last change %s (%s); all %v", what, r.Status, r.Reason, st)
		}
		if got := opText(r.Result.Ops); got != wantOps {
			t.Fatalf("%s: ops %q, want %q", what, got, wantOps)
		}
		imported(t, b.S.G)
		t.Logf("%s: %v", what, st)
	}

	// 3 -> (+ 3 1), typed: pending (not yet forms) until the parenthesis closes
	at := strings.Index(b.Text, "(call fact 3)") + len("(call fact 3")
	if st := erase(t, b, at, 1); st[0] != "pending" {
		t.Fatalf("fact() with no argument: %v", st)
	}
	at--
	lisp := g.Lisp()
	st, r := typeInto(t, b, at, "(+ 3 1)")
	for _, x := range st[:len(st)-1] {
		if x != "pending" {
			t.Fatalf("a state before the last applied: %v", st)
		}
	}
	check(t, "an argument", st, r, "replace #143 with fact(3 + 1)")
	_ = lisp

	// fact -> odd, the name erased and typed: od is no name, odd is
	at = strings.Index(b.Text, "(call fact (+ 3 1))") + len("(call fact")
	before := g.Lisp()
	est := erase(t, b, at, len("fact"))
	at -= len("fact")
	if !bytes.Equal(before, g.Lisp()) {
		t.Fatal("a pending text changed the graph")
	}
	st, r = typeInto(t, b, at, "odd")
	check(t, "a callee", append(est, st...), r, "replace #184 with odd(3 + 1)")

	// against Edit on a copy, the whole texts aligned: the same graph
	h := sample(t).G
	r1, err := Edit(h, defView("main", false), edited(t, defView("main", false), h, "(call fact 3)", "(call fact (+ 3 1))"), EditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Edit(r1.Graph, defView("main", false), edited(t, defView("main", false), r1.Graph, "(call fact (+ 3 1))", "(call odd (+ 3 1))"), EditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Equal(g, r2.Graph); err != nil {
		t.Fatalf("typed, not the whole texts' edits: %v", err)
	}
}

// TestBufferProduct: the same on whim-vim.c's graph, timed.
func TestBufferProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	ix, _ := product(t)
	g := ix.G
	s := NewSession(g)
	b, err := s.Open(defView("ml_clearmarked", false), EditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	before := g.Lisp()
	at := strings.Index(b.Text, "(= lowest_marked 0))") + len("(= lowest_marked ")
	var times []string
	for _, c := range []struct {
		s, e int
		text string
	}{{at, at + 1, ""}, {at, at, "1"}, {at, at + 1, "("}, {at + 1, at + 1, "+ 1 1)"}} {
		t0 := time.Now()
		r, err := b.Change(c.s, c.e, c.text)
		if err != nil {
			t.Fatal(err)
		}
		times = append(times, r.Status+" "+time.Since(t0).Round(time.Millisecond).String())
		if r.Status == "pending" {
			t.Logf("pending: %s", r.Reason)
		}
	}
	t.Logf("changes: %s", strings.Join(times, ", "))
	if !strings.Contains(b.Printed, "(= lowest_marked (+ 1 1)))") {
		t.Fatalf("the buffer:\n%s", b.Printed)
	}
	for b.S.Edits() > 0 {
		b.Undo()
	}
	if !bytes.Equal(before, g.Lisp()) {
		t.Fatal("undone, the graph is not what it was")
	}
}

// TestBufferRename: a renaming at a cursor -- fact, from its call in main
// -- every declaration and use respelled in one edit, undone; a name that
// is taken refused, the graph untouched.
func TestBufferRename(t *testing.T) {
	g := sample(t).G
	s := NewSession(g)
	b, err := s.Open(defView("main", false), EditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	before, opened := g.Lisp(), b.Text
	at := strings.Index(b.Text, "(call fact 3)") + len("(call ")
	if _, _, err := b.Rename(at, "odd"); err == nil {
		t.Fatal("a rename to a name the file declares was not refused")
	}
	if !bytes.Equal(before, g.Lisp()) || s.Edits() != 0 {
		t.Fatal("a refused rename changed the graph")
	}
	rn, d, err := b.Rename(at, "factorial")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := g.C()
	if strings.Contains(string(c), "fact(") || strings.Count(string(c), "factorial") < 4 || d.Head() != "defn" {
		t.Fatalf("renamed %d declarations, %d uses:\n%s", len(rn.Decls), len(rn.Uses), c)
	}
	if !strings.Contains(b.Text, "(call factorial 3)") {
		t.Fatalf("the buffer:\n%s", b.Text)
	}
	imported(t, g)
	if ok, err := b.Undo(); !ok || err != nil || b.Text != opened || !bytes.Equal(before, g.Lisp()) {
		t.Fatal("the rename undone is not the graph before it")
	}
}
