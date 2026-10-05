package graph

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStore: a graph put in the store reads back as its Lisp byte for byte,
// so as the same graph; a second graph sharing its forms adds only what it
// does not share; and a manifest cut short is refused.
func TestStore(t *testing.T) {
	_, _, g := importSample(t, hashSample)
	g, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.Join(g.LispUnits(), nil), g.Lisp()) {
		t.Fatal("the units are not the Lisp")
	}
	// h: g read back, under another head
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w, err := CreateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := []byte(";; g\n")
	if err := w.Put("g", head, g); err != nil {
		t.Fatal(err)
	}
	first := w.New
	if err := w.Put("h", []byte(";; h\n"), h); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// h's units beyond g's: its head, and nothing else
	if added := w.New - first; added != 1 {
		t.Errorf("h added %d units to the store, want 1", added)
	}
	for _, c := range []struct {
		name string
		g    *Graph
		lisp []byte
	}{{"g", g, append(bytes.Clone(head), g.Lisp()...)}, {"h", h, append([]byte(";; h\n"), h.Lisp()...)}} {
		b, err := ReadStoreLisp(dir, c.name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, c.lisp) {
			t.Fatalf("%s read back is not its Lisp", c.name)
		}
		r, err := Read(b)
		if err != nil {
			t.Fatal(err)
		}
		if err := Equal(r, c.g); err != nil {
			t.Fatalf("%s read back: %v", c.name, err)
		}
	}
	m := filepath.Join(dir, "h.gm")
	b, _ := os.ReadFile(m)
	os.WriteFile(m, b[:len(b)-1], 0o644)
	if _, err := ReadStoreLisp(dir, "h"); err == nil || !strings.Contains(err.Error(), "not a whole manifest") {
		t.Errorf("a manifest cut short: %v", err)
	}
	if err := RemoveStore(dir); err != nil || StoreHas(dir, "g") {
		t.Errorf("the store outlived its removal: %v", err)
	}
}
