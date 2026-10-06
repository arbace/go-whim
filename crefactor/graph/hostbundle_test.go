package graph

import (
	"os"
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

// TestHostBundle: whim-vim.c imported on a bundle read back -- the
// compiler's answers and the headers recorded -- is the graph the host's
// compiler gives.
func TestHostBundle(t *testing.T) {
	const path = "../../src/whim-vim.c" // its ten #includes
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := Import(path, src)
	if err != nil {
		t.Fatal(err)
	}
	b, err := HostBundle(path, src)
	if err != nil {
		t.Fatal(err)
	}
	h, err := cc.ReadBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.FS.(cc.MemFS)) == 0 {
		t.Fatal("no headers recorded")
	}
	cc.SetHost(h)
	defer cc.SetHost(nil)
	got, _, err := Import(path, src)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(want, got); err != nil {
		t.Fatalf("on the bundle: %v", err)
	}
	t.Logf("%d bytes, %d headers", len(b), len(h.FS.(cc.MemFS)))
}
