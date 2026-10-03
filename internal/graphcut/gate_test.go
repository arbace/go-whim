package graphcut

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
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

// FirstOnGraph are the gate's phases whose first step is a graph step: the
// graph of q(N-1) can be handed in, read from its Lisp.
var FirstOnGraph = []int{8, 17, 24}

// snaps is GRAPHCUT_SNAPS, the directory of the boundaries (qNNN.c), or the
// test is skipped.
func snaps(t *testing.T) string {
	d := os.Getenv("GRAPHCUT_SNAPS")
	if d == "" {
		t.Skip("GRAPHCUT_SNAPS names no directory of boundaries")
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

// readGraph is q(N-1)'s graph as the pipeline would hand it on: imported
// once, written as Lisp, read back -- no cc node behind it.
func readGraph(t *testing.T, dir string, n int) *graph.Graph {
	t.Helper()
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), snap(t, dir, n))
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// THE GATE.  Every phase with a graph step, its graph steps on the graph:
// qN byte for byte from q(N-1); and, where the phase starts on the graph,
// from q(N-1)'s graph read from its Lisp too.
func TestGraphBytes(t *testing.T) {
	dir := snaps(t)
	for _, n := range append(append([]int{}, DropLocalPhases...), 24) {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			p, _ := PhaseOf(n)
			var log bytes.Buffer
			got, _, err := RunGraph(p, snap(t, dir, n-1), nil, &log)
			if err != nil {
				t.Fatalf("phase %d on the graph: %v\n%s", n, err, log.String())
			}
			want := snap(t, dir, n)
			if !bytes.Equal(got, want) {
				t.Fatalf("phase %d on the graph is not q%03d: %s", n, n, firstDiff(got, want))
			}
			for _, m := range FirstOnGraph {
				if m != n {
					continue
				}
				got, _, err := RunGraph(p, nil, readGraph(t, dir, n-1), io.Discard)
				if err != nil {
					t.Fatalf("phase %d on q%03d's graph read back: %v", n, n-1, err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("phase %d on q%03d's graph read back is not q%03d: %s", n, n-1, n, firstDiff(got, want))
				}
				log.WriteString("  (and from the graph read back: the same bytes)\n")
			}
			t.Logf("phase %d: q%03d byte for byte\n%s", n, n, log.String())
		})
	}
}

// Field by field, on the text the plan's steps before it leave: the text's
// DropLocal and the graph's -- the same count of plumbing sites, and the
// same C (the text's printed canonically, the graph's C view before any
// collection).
func TestDropLocalSteps(t *testing.T) {
	dir := snaps(t)
	for _, n := range DropLocalPhases {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			p, _ := PhaseOf(n)
			scratch := t.TempDir()
			path := filepath.Join(scratch, "whim-vim.c")
			text := snap(t, dir, n-1)
			var rows []string
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
					want, n1, err := cut.DropLocal(text, f)
					if err != nil {
						t.Fatal(err)
					}
					g, _, err := graph.Import(path, text)
					if err != nil {
						t.Fatal(err)
					}
					e := graph.NewEditor(g)
					n2, err := DropLocal(e, f, whim.GraphFallOut)
					if err != nil {
						t.Fatal(err)
					}
					if err := e.Check(); err != nil {
						t.Fatalf("%s: the invariants: %v", f, err)
					}
					got, err := g.C()
					if err != nil {
						t.Fatal(err)
					}
					canon, err := cemit.Canonical(path, want)
					if err != nil {
						t.Fatal(err)
					}
					if n1 != n2 || !bytes.Equal(got, canon) {
						t.Fatalf("%s: text %d sites, graph %d; the C: %s", f, n1, n2, firstDiff(got, canon))
					}
					rows = append(rows, fmt.Sprintf("%s %d", f, n2))
					text = want
				}
			}
			t.Logf("phase %d: %s", n, strings.Join(rows, ", "))
		})
	}
}

// Phase 24's report on the graph is the text version's, line for line: the
// same acts, with the same counts.
func TestP24Report(t *testing.T) {
	dir := snaps(t)
	p, _ := PhaseOf(24)
	var text, gr bytes.Buffer
	if _, err := build.RunPhase(p, snap(t, dir, 23), t.TempDir(), &text); err != nil {
		t.Fatal(err)
	}
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), snap(t, dir, 23))
	if err != nil {
		t.Fatal(err)
	}
	e := graph.NewEditor(g)
	if err := P24(e, whim.GraphFallOut, &gr); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatalf("the invariants: %v", err)
	}
	if text.String() != gr.String() {
		t.Fatalf("the reports differ\ntext:\n%s\ngraph:\n%s", text.String(), gr.String())
	}
	t.Logf("the edit log: %d acts, %d expressions left untyped", len(e.Log), len(e.Untyped))
}

// The controls: a deliberate fault in each rewrite, which the gate must
// catch; and the closure's own empty-block rule, which the text cutters do
// not have.
func TestGraphControl(t *testing.T) {
	dir := snaps(t)
	for _, c := range []struct {
		n    int
		what string
		knob *bool
	}{
		{17, "the get_varp rule leaves the case label", &controlKeepCase},
		{24, "the window guards taken as always", &controlAlways},
		{24, "the closure's empty-block rule on", &emptyGoes},
	} {
		*c.knob = true
		p, _ := PhaseOf(c.n)
		got, _, err := RunGraph(p, snap(t, dir, c.n-1), nil, io.Discard)
		*c.knob = false
		if err != nil {
			t.Logf("phase %d, %s: refused: %v", c.n, c.what, err)
			continue
		}
		want := snap(t, dir, c.n)
		if bytes.Equal(got, want) {
			t.Fatalf("phase %d, %s: the bytes did not move", c.n, c.what)
		}
		t.Logf("phase %d, %s: the bytes move, %d against %d: %s", c.n, c.what, len(got), len(want), firstDiff(got, want))
	}
}

func firstDiff(a, b []byte) string {
	line := 1
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			s := bytes.LastIndexByte(a[:i], '\n') + 1
			ea := bytes.IndexByte(a[i:], '\n')
			eb := bytes.IndexByte(b[i:], '\n')
			if ea < 0 {
				ea = len(a) - i
			}
			if eb < 0 {
				eb = len(b) - i
			}
			return fmt.Sprintf("line %d: got `%s`, want `%s`", line, a[s:i+ea], b[s:i+eb])
		}
		if a[i] == '\n' {
			line++
		}
	}
	return fmt.Sprintf("lengths %d and %d", len(a), len(b))
}

func TestMain(m *testing.M) {
	if os.Getenv("TMPDIR") == "" {
		os.Setenv("TMPDIR", os.TempDir())
	}
	os.Exit(m.Run())
}

// A reader left is refused by both: b_p_ro on q016, which phase 34's own
// edit has not yet freed of its readers.
func TestDropLocalRefuses(t *testing.T) {
	dir := snaps(t)
	text := snap(t, dir, 16)
	_, _, terr := cut.DropLocal(text, "b_p_ro")
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), text)
	if err != nil {
		t.Fatal(err)
	}
	_, gerr := DropLocal(graph.NewEditor(g), "b_p_ro", whim.GraphFallOut)
	if terr == nil || gerr == nil {
		t.Fatalf("a field with readers: the text %v, the graph %v", terr, gerr)
	}
	t.Logf("text: %v\ngraph: %v", terr, gerr)
}
