package graphcheck

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/cut"
	"github.com/arbace/go-whim/internal/whim"
)

// FOLDX on the front was held here to xform.FallOutOf on the text cutters'
// output (B2d); since B4 the front runs on the graph, FoldX its closure,
// and the phases are held to their snapshots (TestPhasesOnGraph).

// declaredRows are phase 1's rows (internal/build's declared).
func declaredRows(t *testing.T) string {
	b, err := os.ReadFile("internal/phase/001/delta.md")
	if err != nil {
		t.Fatal(err)
	}
	var toks []string
	fence := false
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			fence = !fence
			continue
		}
		if fence {
			toks = append(toks, strings.Fields(ln)...)
		}
	}
	return strings.Join(toks, " ")
}

// coreForms says a form is above the first #include: the core
// (internal/whim's Cut, as the graph sees it).
func coreForms(g *graph.Graph) func(*graph.Node) bool {
	core := map[*graph.Node]bool{}
	for _, f := range g.Forms {
		if f.Is("include") {
			break
		}
		core[f] = true
	}
	return func(f *graph.Node) bool { return core[f] }
}

// Phase 62 on the graph: EmptyBlocks on the core of q061's graph read back,
// with the text's own test of a condition's text, collected, is q062.c
// byte for byte, with the text's report numbers.
func TestEmptyBlocksPhase62(t *testing.T) {
	dir, _, _, _ := setup(t)
	in, want := snapOf(t, dir, 61), snapOf(t, dir, 62)
	path := filepath.Join(t.TempDir(), "whim-vim.c")
	g, _, err := graph.Import(path, in)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := graph.NewEditor(h)
	st, err := e.EmptyBlocks(graph.EmptyOptions{In: coreForms(h), Cond: edit.PureCond})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Collect(whim.GraphCollect()); err != nil {
		t.Fatal(err)
	}
	got, err := h.C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("phase 62 on the graph is not q062.c: %s", firstDiff(got, want))
	}
	// the text's own numbers, as its report said them on q061 (B3g deleted
	// it: phase 62 runs this on the graph now)
	if st.Blocks != 49 || strings.Join(st.Locals, " ") != "did_intro event_cmdlineleavepre_triggered mustfree pp free_str" {
		t.Errorf("graph: %v; the text said 49 blocks, 5 locals", st)
	}
	t.Logf("%v", st)
	// the control: without the text's test of a condition's text, `if
	// (regname == '=') {}` goes too, which the text's PureCond keeps for
	// its `=`
	h, err = graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e = graph.NewEditor(h)
	if _, err := e.EmptyBlocks(graph.EmptyOptions{In: coreForms(h)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Collect(whim.GraphCollect()); err != nil {
		t.Fatal(err)
	}
	if c, _ := h.C(); bytes.Equal(c, want) {
		t.Error("the control: EmptyBlocks without Cond gives q062.c too")
	}
}

// notags' two conditions kept for their effect (phase 3's front): on
// q002's graph read back, the call in each if deleted and the closure run
// with KeepCondition, nv_help and nv_tagpop are the text cutter's, byte
// for byte; without it the ifs stay, emptied.
func TestKeepConditionNoTags(t *testing.T) {
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 2)
	path := filepath.Join(t.TempDir(), "whim-vim.c")
	// the cutter itself, on the graph (B4)
	gc, _, err := graph.Import(path, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := cut.NoTags(graph.NewEditor(gc), io.Discard); err != nil {
		t.Fatal(err)
	}
	want, err := gc.C()
	if err != nil {
		t.Fatal(err)
	}
	fn := func(src []byte, name string) string {
		s := string(src)
		a := strings.Index(s, "\n"+name+"(")
		if a < 0 {
			return ""
		}
		z := strings.Index(s[a:], "\n}\n")
		return s[a : a+z+3]
	}
	g, _, err := graph.Import(path, in)
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []bool{true, false} {
		h, err := graph.Read(g.Lisp())
		if err != nil {
			t.Fatal(err)
		}
		e := graph.NewEditor(h)
		v := graph.NewVerbs("notags", e, io.Discard)
		v.InFunction("nv_help", func(v *graph.Verbs) { v.Cut("(call ex_help nullptr)", 1, "the <Help> key") })
		v.InFunction("nv_tagpop", func(v *graph.Verbs) { v.Cut("(call do_tag _*)", 1, "CTRL-T") })
		if err := v.Done(); err != nil {
			t.Fatal(err)
		}
		opt := whim.GraphFallOut
		opt.KeepCondition = keep
		if _, err := e.FallOut(opt); err != nil {
			t.Fatal(err)
		}
		got, err := h.C()
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"nv_help", "nv_tagpop"} {
			same := fn(got, name) == fn(want, name) && fn(want, name) != ""
			if same != keep {
				t.Errorf("KeepCondition %v: %s\n%s\nthe text's\n%s", keep, name, fn(got, name), fn(want, name))
			}
		}
	}
}

// FoldX with every unwritten object and member a seed, the text closure's
// own step, on snapshots where the text closure ran (before q040 it refused
// itself: a declaration it cuts is used after), collected: what the text
// closure gave there, swept, by its digest (testdata/foldx_every_seed.md,
// recorded before crefactor/xform was deleted).
func TestFoldXEverySeed(t *testing.T) {
	dir, _, _, _ := setup(t)
	rows := everySeedRows(t)
	for _, n := range []int{40, 70, 103} {
		in := snapOf(t, dir, n)
		path := filepath.Join(t.TempDir(), "whim-vim.c")
		row, ok := rows[n]
		if !ok || row.in != fmt.Sprintf("%x", sha256.Sum256(in)) {
			t.Logf("q%03d: not the snapshot testdata/foldx_every_seed.md records; skipped", n)
			continue
		}
		g, _, err := graph.Import(path, in)
		if err != nil {
			t.Fatal(err)
		}
		h, err := graph.Read(g.Lisp())
		if err != nil {
			t.Fatal(err)
		}
		e := graph.NewEditor(h)
		t1 := time.Now()
		if _, err := e.FoldX(graph.FoldX{}); err != nil {
			t.Fatal(err)
		}
		tGraph := time.Since(t1)
		if _, err := e.Collect(whim.GraphCollect()); err != nil {
			t.Fatal(err)
		}
		got, err := h.C()
		if err != nil {
			t.Fatal(err)
		}
		if d := fmt.Sprintf("%x", sha256.Sum256(got)); d != row.want || len(got) != row.n {
			t.Errorf("q%03d: FoldX gives %d bytes, %s; the text closure gave %d, %s", n, len(got), d, row.n, row.want)
		}
		if bytes.Equal(got, in) {
			t.Errorf("q%03d: FoldX changed nothing", n)
		}
		t.Logf("q%03d: FoldX %v", n, tGraph.Round(time.Millisecond))
	}
}

type everySeedRow struct {
	in, want string
	n        int
}

// everySeedRows reads testdata/foldx_every_seed.md's fenced block.
func everySeedRows(t *testing.T) map[int]everySeedRow {
	b, err := os.ReadFile("internal/graphcheck/testdata/foldx_every_seed.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[int]everySeedRow{}
	fence := false
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(ln, "```") {
			fence = !fence
			continue
		}
		var n int
		var r everySeedRow
		if fence {
			if _, err := fmt.Sscan(ln, &n, &r.in, &r.want, &r.n); err != nil {
				t.Fatalf("foldx_every_seed.md: %q: %v", ln, err)
			}
			rows[n] = r
		}
	}
	return rows
}
