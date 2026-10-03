package cut

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/whim"
)

// A reader left is refused, and named: b_p_ro on q016, which phase 34's own
// edit has not yet freed of its readers -- 8 of them, as the text version
// this cut replaced counted.  GRAPH_SNAPS names the snapshots, or the test
// is skipped; the cut's every success is the build check's (phases 4, 5, 7,
// 8, 12-14, 16-20 and 34, byte for byte).
func TestDropLocalRefuses(t *testing.T) {
	dir := os.Getenv("GRAPH_SNAPS")
	if dir == "" {
		t.Skip("GRAPH_SNAPS is not set")
	}
	text, err := os.ReadFile(filepath.Join(dir, "q016.c"))
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), text)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DropLocal(graph.NewEditor(g), "b_p_ro", whim.GraphFallOut)
	if err == nil || !strings.HasPrefix(err.Error(), "droplocal: b_p_ro still has 8 mentions after the plumbing went") {
		t.Fatalf("a field with readers: %v", err)
	}
}

// No field, and too few sites, are refused.
func TestDropLocalShapes(t *testing.T) {
	src := "struct buf { int b_p_x; int b_p_y; };\nstruct buf *curbuf;\n" +
		"void f(struct buf *buf) { buf->b_p_y = 1; }\nint main(void) { f(curbuf); return 0; }\n"
	path := filepath.Join(t.TempDir(), "a.c")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ field, want string }{
		{"b_p_z", "droplocal: there is no b_p_z here"},
		{"b_p_y", "droplocal: b_p_y: only 2 plumbing sites, expected at least the field, an initialiser and a get_varp case -- the shape has moved"},
	} {
		g, _, err := graph.Import(path, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DropLocal(graph.NewEditor(g), c.field, whim.GraphFallOut); err == nil || err.Error() != c.want {
			t.Errorf("%s: %v", c.field, err)
		}
	}
}
