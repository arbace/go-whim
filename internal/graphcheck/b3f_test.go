package graphcheck

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arbace/go-whim/internal/build"
)

// TestB3fDiff runs GRAPH_PHASES' phases from their snapshot, as a run in
// order would (the graph imported where its graph steps begin), and where
// one does not give qN.c writes what it gave beside the snapshot's and
// shows the diff: the converter's view of what TestPhasesOnGraph only
// counts.
func TestB3fDiff(t *testing.T) {
	if os.Getenv("GRAPH_PHASES") == "" {
		t.Skip("GRAPH_PHASES is not set")
	}
	dir, only, _, _ := setup(t)
	out := filepath.Join(os.TempDir(), "b3f-diff")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range only {
		p := build.Plan[n]
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			in, want := snapOf(t, dir, n-1), snapOf(t, dir, n)
			var log bytes.Buffer
			got, conv, err := build.AdvanceFrom(p, in, nil, &log)
			t.Logf("%s\n%s", log.String(), conv)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(got, want) {
				return
			}
			g := filepath.Join(out, fmt.Sprintf("got%03d.c", n))
			os.WriteFile(g, got, 0o644)
			d, _ := exec.Command("diff", "-u", filepath.Join(dir, fmt.Sprintf("q%03d.c", n)), g).CombinedOutput()
			if len(d) > 6000 {
				d = d[:6000]
			}
			t.Fatalf("phase %d differs:\n%s", n, d)
		})
	}
}
