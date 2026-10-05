package vmm

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// TestDirStore: blobs by their hash, read back from any offset; a short
// blob (a write cut off) refused; refs set, read and compared-and-set; a
// name the store does not take refused.
func TestDirStore(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenDirStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("(defn f [x] x)")
	h, err := s.Put(data)
	if err != nil || h != sha256.Sum256(data) {
		t.Fatalf("put: %x %v", h, err)
	}
	if h2, err := s.Put(data); err != nil || h2 != h {
		t.Fatalf("put again: %v", err)
	}
	if n, ok := s.Size(h); !ok || n != int64(len(data)) {
		t.Fatalf("size: %d %v", n, ok)
	}
	buf := make([]byte, 4)
	if n, ok := s.Get(h, 6, buf); !ok || string(buf[:n]) != "f [x" {
		t.Fatalf("get at 6: %q %v", buf[:n], ok)
	}
	if n, ok := s.Get(h, 100, buf); !ok || n != 0 {
		t.Fatalf("get past the end: %d %v", n, ok)
	}
	// a blob whose file is short: refused, not served
	short := []byte("a blob cut off")
	hs := sha256.Sum256(short)
	if err := os.WriteFile(s.blob(hs), short[:5], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Size(hs); ok {
		t.Fatal("a short blob served")
	}
	// refs
	if _, ok := s.Ref("graph"); ok {
		t.Fatal("a ref before any is set")
	}
	var none [32]byte
	if err := s.SetRef("graph", h, &none); err != nil {
		t.Fatalf("set ref, none before: %v", err)
	}
	if got, ok := s.Ref("graph"); !ok || got != h {
		t.Fatal("ref read back")
	}
	h3, _ := s.Put([]byte("another"))
	if err := s.SetRef("graph", h3, &none); err != ErrConflict {
		t.Fatalf("set ref over one, none expected: %v", err)
	}
	if err := s.SetRef("graph", h3, &h); err != nil {
		t.Fatalf("compare and set: %v", err)
	}
	for _, bad := range []string{"", ".hidden", "a/b", "../x", string(make([]byte, 65))} {
		if err := s.SetRef(bad, h, nil); err == nil {
			t.Fatalf("ref name %q taken", bad)
		}
	}
	if err := s.SetRef("x", sha256.Sum256([]byte("absent")), nil); err == nil {
		t.Fatal("a ref to a blob the store has not")
	}
	if _, err := os.Stat(filepath.Join(dir, "refs", "graph")); err != nil {
		t.Fatal(err)
	}
}
