package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/steps"
)

// B3a's units, step by step (doc/GRAPH-MIGRATION.md, *B3a as built*): a
// phase's steps run one at a time on q(N-1), each step's output kept
// (B3A_REF, a directory: NNN-KK.c the canonical text after step KK, its
// report beside it) while the steps are the text's, and then each graph
// step held to the text step it replaced -- its input the text the steps
// before it left, imported; its C view, before any sweep, against that
// text step's output printed canonically -- so that a unit in the middle
// of a phase is proved before the phase is.

// b3aPhase is the plan's phase n.
func b3aPhase(t *testing.T, n int) build.Phase {
	for _, p := range build.Plan {
		if p.N == n {
			return p
		}
	}
	t.Fatalf("no phase %d", n)
	return build.Phase{}
}

// TestB3aRecord writes B3A_REF's files for the phases B3A_RECORD names
// (comma-separated): each step of the plan as it stands, one at a time.
func TestB3aRecord(t *testing.T) {
	ref, which := os.Getenv("B3A_REF"), os.Getenv("B3A_RECORD")
	if ref == "" || which == "" {
		t.Skip("B3A_REF and B3A_RECORD are not set")
	}
	dir, _, _, _ := setup(t)
	if err := os.MkdirAll(ref, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range strings.Split(which, ",") {
		n, _ := strconv.Atoi(s)
		p := b3aPhase(t, n)
		text := snapOf(t, dir, n-1)
		for k, st := range p.Steps {
			var rep bytes.Buffer
			out, err := build.RunPhase(build.Phase{N: n, Steps: []build.Step{st}}, text, t.TempDir(), &rep)
			if err != nil {
				t.Fatalf("phase %d step %d (%s %v): %v\n%s", n, k, st.Op, st.Args, err, rep.String())
			}
			canon, err := pipeline.Canonical(out, nil, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(ref, fmt.Sprintf("%03d-%02d", n, k))
			os.WriteFile(base+".c", canon, 0o644)
			os.WriteFile(base+".raw.c", out, 0o644)
			os.WriteFile(base+".txt", rep.Bytes(), 0o644)
			text = out
		}
	}
}

// TestB3aUnit holds the step B3A_UNIT names (`N:K`, phase N's step K as
// the plan has it now, a graph step) to B3A_REF's record of the text step:
// its input the raw text after step K-1 (q(N-1) for K 0), imported.
func TestB3aUnit(t *testing.T) {
	ref, unit := os.Getenv("B3A_REF"), os.Getenv("B3A_UNIT")
	if ref == "" || unit == "" {
		t.Skip("B3A_REF and B3A_UNIT are not set")
	}
	if abs, err := filepath.Abs(ref); err == nil {
		ref = abs
	}
	dir, _, _, _ := setup(t)
	a, b, _ := strings.Cut(unit, ":")
	n, _ := strconv.Atoi(a)
	k, _ := strconv.Atoi(b)
	p := b3aPhase(t, n)
	st := p.Steps[k]
	var in []byte
	if k == 0 {
		in = snapOf(t, dir, n-1)
	} else {
		var err error
		if in, err = os.ReadFile(filepath.Join(ref, fmt.Sprintf("%03d-%02d.raw.c", n, k-1))); err != nil {
			t.Fatal(err)
		}
	}
	base := filepath.Join(ref, fmt.Sprintf("%03d-%02d", n, k))
	want, err := os.ReadFile(base + ".c")
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
	if err != nil {
		t.Fatal(err)
	}
	gop, ok := steps.LookupGraph(st.Op)
	if !ok {
		t.Fatalf("%s is no graph step", st.Op)
	}
	var rep bytes.Buffer
	e := graph.NewEditor(g)
	if err := gop(e, st.Args, &rep); err != nil {
		t.Fatalf("%v\n%s", err, rep.String())
	}
	got, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(base+".graph.c", got, 0o644)
	os.WriteFile(base+".graph.txt", rep.Bytes(), 0o644)
	if len(e.Untyped) > 0 {
		t.Logf("%d expressions left untyped", len(e.Untyped))
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("phase %d step %d (%s %v): the C view differs from the text step's (diff %s.c %s.graph.c)\n%s",
			n, k, st.Op, st.Args, base, base, rep.String())
	}
	t.Logf("byte for byte; report:\n%s", rep.String())
}
