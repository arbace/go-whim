package treepilot

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/cut"
	_ "github.com/arbace/go-whim/internal/phase/004/a"
	_ "github.com/arbace/go-whim/internal/phase/004/b"
	_ "github.com/arbace/go-whim/internal/phase/004/c"
	_ "github.com/arbace/go-whim/internal/phase/004/d"
	_ "github.com/arbace/go-whim/internal/phase/004/e"
	_ "github.com/arbace/go-whim/internal/phase/004/f"
	_ "github.com/arbace/go-whim/internal/phase/005/a"
	_ "github.com/arbace/go-whim/internal/phase/005/b"
	_ "github.com/arbace/go-whim/internal/phase/005/c"
	_ "github.com/arbace/go-whim/internal/phase/005/d"
	_ "github.com/arbace/go-whim/internal/phase/018"
	_ "github.com/arbace/go-whim/internal/phase/019"
	_ "github.com/arbace/go-whim/internal/phase/020"
	_ "github.com/arbace/go-whim/internal/phase/024"
	_ "github.com/arbace/go-whim/internal/phase/034"
	"github.com/arbace/go-whim/internal/whim"
)

// DropLocalPhases are the plan's phases with a droplocal step.
var DropLocalPhases = []int{4, 5, 7, 8, 12, 13, 14, 16, 17, 18, 19, 20, 34}

// snaps is the directory of the boundaries, TREEPILOT_SNAPS (qNNN.c), or the
// test is skipped: they are a whole build's (`go tool whim build --keep D`,
// or .cache/boundaries when its manifest names the input).
func snaps(t *testing.T) string {
	d := os.Getenv("TREEPILOT_SNAPS")
	if d == "" {
		t.Skip("TREEPILOT_SNAPS names no directory of boundaries")
	}
	return d
}

func snap(t *testing.T, dir string, n int) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("q%03d.c", n)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Every phase with a tree step, its tree steps on the tree: qN byte for
// byte from q(N-1), as the plan's text steps give it.
func TestTreeBytes(t *testing.T) {
	dir := snaps(t)
	for _, n := range append(append([]int{}, DropLocalPhases...), 24) {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			p, _ := PhaseOf(n)
			var log bytes.Buffer
			got, _, err := RunTree(p, snap(t, dir, n-1), &log)
			if err != nil {
				t.Fatalf("phase %d on the tree: %v", n, err)
			}
			if !bytes.Equal(got, snap(t, dir, n)) {
				t.Fatalf("phase %d on the tree is not q%03d", n, n)
			}
			t.Logf("phase %d: q%03d byte for byte\n%s", n, n, log.String())
		})
	}
}

// The control: a deliberate change to each tree version, which the byte
// comparison must catch.
func TestTreeControl(t *testing.T) {
	dir := snaps(t)
	for _, c := range []struct {
		n    int
		knob *bool
	}{{17, &controlKeepCase}, {24, &controlKeepCall}} {
		*c.knob = true
		p, _ := PhaseOf(c.n)
		got, _, err := RunTree(p, snap(t, dir, c.n-1), io.Discard)
		*c.knob = false
		if err != nil {
			t.Logf("phase %d's control refused: %v", c.n, err)
			continue
		}
		if bytes.Equal(got, snap(t, dir, c.n)) {
			t.Fatalf("phase %d's control did not move the bytes", c.n)
		}
		t.Logf("phase %d's control moves the bytes: %d against %d", c.n, len(got), len(snap(t, dir, c.n)))
	}
}

// graphDropLocal is internal/cut's DropLocal on text: since doc/GRAPH.md's
// step 5 it is a cut on the graph, so the text is imported and the graph's
// C view returned.
func graphDropLocal(scratch string, text []byte, field string) ([]byte, int, error) {
	g, _, err := graph.Import(filepath.Join(scratch, "whim-vim.c"), text)
	if err != nil {
		return nil, 0, err
	}
	n, err := cut.DropLocal(graph.NewEditor(g), field, whim.GraphFallOut)
	if err != nil {
		return nil, 0, err
	}
	out, err := g.C()
	return out, n, err
}

// Step by step: each droplocal step's fields, on the text the plan's steps
// before it leave, by internal/cut's DropLocal (on the graph since
// doc/GRAPH.md's step 5, the text version the pilot was held to before) and
// by the tree's -- the same count of plumbing sites, and the same C,
// canonically.
func TestDropLocalSteps(t *testing.T) {
	dir := snaps(t)
	for _, n := range DropLocalPhases {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			p, _ := PhaseOf(n)
			scratch := t.TempDir()
			text := snap(t, dir, n-1)
			for _, s := range p.Steps {
				if s.Op != "droplocal" {
					one := p
					one.Steps = []build.Step{s}
					var err error
					if text, err = build.RunPhase(one, text, scratch, io.Discard); err != nil {
						t.Fatal(err)
					}
					continue
				}
				for _, f := range s.Args {
					want, n1, err := graphDropLocal(scratch, text, f)
					if err != nil {
						t.Fatal(err)
					}
					fs, err := clisp.Forms(filepath.Join(scratch, "whim-vim.c"), text)
					if err != nil {
						t.Fatal(err)
					}
					tr := &Tree{Root: clisp.Root(fs)}
					n2, err := DropLocal(tr, f)
					if err != nil {
						t.Fatal(err)
					}
					got, err := clisp.Print(tr.Root.List)
					if err != nil {
						t.Fatal(err)
					}
					canon, err := cemit.Canonical(filepath.Join(scratch, "whim-vim.c"), want)
					if err != nil {
						t.Fatal(err)
					}
					if n1 != n2 || !bytes.Equal(got, canon) {
						t.Fatalf("%s: text %d sites, tree %d; the same C: %v", f, n1, n2, bytes.Equal(got, canon))
					}
					text = want
				}
			}
		})
	}
}

// Phase 24's report on the tree is the text version's, line for line: the
// same acts, with the same counts.
func TestP24Report(t *testing.T) {
	dir := snaps(t)
	p, _ := PhaseOf(24)
	var text, tree bytes.Buffer
	if _, err := build.RunPhase(p, snap(t, dir, 23), t.TempDir(), &text); err != nil {
		t.Fatal(err)
	}
	fs, err := clisp.Forms("whim-vim.c", snap(t, dir, 23))
	if err != nil {
		t.Fatal(err)
	}
	if err := P24(&Tree{Root: clisp.Root(fs)}, &tree); err != nil {
		t.Fatal(err)
	}
	if text.String() != tree.String() {
		t.Fatalf("the reports differ\ntext:\n%s\ntree:\n%s", text.String(), tree.String())
	}
}
