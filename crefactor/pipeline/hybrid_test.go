package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
)

// The hybrid driver on the toy: the same product as the text plan, with
// phase 2's literal made on the graph, a phase that turns from the graph to
// text, and one that begins with a sweep on the graph.

// gcall is `gcall F V`: every call of F becomes the literal V, on the graph.
func gcall(e *graph.Editor, args []string, w io.Writer) error {
	n := 0
	for _, d := range e.FileDecls(args[0]) {
		for _, u := range e.Uses(d) {
			call := e.Parent(u)
			if call == nil || !call.Is("call") {
				return fmt.Errorf("gcall: a use of %s that is not a call", args[0])
			}
			if err := e.Replace(call, graph.NewAtom(args[1])); err != nil {
				return err
			}
			n++
		}
	}
	if n == 0 {
		return fmt.Errorf("gcall: no call of %s", args[0])
	}
	fmt.Fprintf(w, "  gcall        %s -> %s, %d calls\n", args[0], args[1], n)
	return nil
}

// gnone is `gnone F`: it asserts nothing declares F, and changes nothing.
func gnone(e *graph.Editor, args []string, w io.Writer) error {
	if len(e.FileDecls(args[0])) > 0 {
		return fmt.Errorf("gnone: %s is declared", args[0])
	}
	return nil
}

var graphOps = map[string]GraphOp{"gcall": gcall, "gnone": gnone}

func hybridPlan() Plan {
	return Plan{
		{N: 0, Name: "seed", Seed: true, NoSource: true},
		{N: 1, Name: "square is sq", Steps: []Step{{Op: "rename", Args: []string{"square", "sq"}}}},
		{N: 2, Name: "cube(2) is 8, on the graph", Steps: []Step{{Op: "gcall", Graph: true, Args: []string{"cube", "8"}}}},
		{N: 3, Name: "from the graph to text", Steps: []Step{
			{Op: "gnone", Graph: true, Args: []string{"cube"}},
			{Op: "rename", Args: []string{"x", "v"}},
		}},
		{N: 4, Name: "a sweep on the graph", Steps: []Step{{Op: "sweep"}, {Op: "gnone", Graph: true, Args: []string{"cube"}}}},
	}
}

func hybridConfig(t *testing.T) (*Config, string) {
	c, dir := config(t)
	c.Plan = hybridPlan()
	c.GraphLookup = func(name string) (GraphOp, bool) { op, ok := graphOps[name]; return op, ok }
	c.Collect = graph.CollectOptions{Roots: []string{"main"}}
	return c, dir
}

// In order: the text plan's product, the graph kept beside the boundary
// before each phase that begins on it, and the summary saying what each
// phase on the graph spent.
func TestHybridRun(t *testing.T) {
	c, dir := hybridConfig(t)
	o := options(dir)
	var log bytes.Buffer
	o.W = &log
	out, err := c.Run(o)
	if err != nil {
		t.Fatalf("run: %v\n%s", err, log.String())
	}
	if string(out) != product {
		t.Fatalf("the product is\n%s\nwant\n%s", out, product)
	}
	for n, want := range map[int]bool{0: false, 1: true, 2: true, 3: true, 4: false} {
		if has := graph.StoreHas(c.SnapDir, graphSnapName(n)); has != want {
			t.Errorf("q%03d's graph: in the store %v, want %v", n, has, want)
		}
		if !want {
			continue
		}
		text, _ := os.ReadFile(c.snapPath(n))
		g, err := c.readGraphSnap(n, text)
		if g == nil {
			t.Fatalf("q%03d's graph is not q%03d.c's: %v", n, n, err)
		}
		if v, _ := g.C(); !bytes.Equal(v, text) {
			t.Errorf("q%03d's graph's C view is not q%03d.c", n, n)
		}
	}
	for _, want := range []string{
		"phase 2      cube(2) is 8, on the graph: 1 act, -6 edited and collected; 15 lines; graph: 1 import",
		"phase 3      from the graph to text: 1 act, 0 edited, 0 swept; 15 lines; graph: 1 C view",
		"phase 4      a sweep on the graph: 1 act, 0 edited and collected; 15 lines; graph: 1 import",
		// what each phase changed in forms, from the graphs' hashes: phase
		// 1 began on text, so it is not compared
		"graph: 1 import", "; forms: 1 removed, 1 changed\n",
		"from the graph to text: 1 act, 0 edited, 0 swept; 15 lines; graph: 1 C view", "; forms: 2 changed\n",
		"; forms: none changed\n",
		"forms        4 boundaries read back and hashed beside the run",
	} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("no %q in the log:\n%s", want, log.String())
		}
	}
	if !strings.Contains(log.String(), "graphs       3 kept in the store") {
		t.Errorf("no account of the store in the log:\n%s", log.String())
	}
	if strings.Count(log.String(), "forms:") != 3 {
		t.Errorf("not 3 phases compared:\n%s", log.String())
	}
	// --changes lists the forms; --no-changes hashes nothing
	for _, x := range []struct {
		forms, none bool
		want, not   string
	}{
		{forms: true, want: "; forms: 1 removed, 1 changed\n      removed  defn cube\n      changed  defn main\n"},
		{none: true, not: "forms"},
	} {
		o := options(dir)
		var log bytes.Buffer
		o.W, o.Forms, o.NoForms = &log, x.forms, x.none
		if _, err := c.Run(o); err != nil {
			t.Fatal(err)
		}
		if x.want != "" && !strings.Contains(log.String(), x.want) {
			t.Errorf("no %q in the log:\n%s", x.want, log.String())
		}
		if x.not != "" && strings.Contains(log.String(), x.not) {
			t.Errorf("%q in the log:\n%s", x.not, log.String())
		}
	}
	// a second run takes away the graph snapshots no phase reads any more
	c.Plan[2].Steps[0].Graph, c.Plan[2].Steps[0].Op = false, "replace"
	c.Plan[2].Steps[0].Args = []string{"cube(2)", "8"}
	if _, err := c.Run(options(dir)); err != nil {
		t.Fatal(err)
	}
	if graph.StoreHas(c.SnapDir, graphSnapName(1)) {
		t.Error("q001's graph outlived the plan that read it")
	}
}

// The check begins each phase that begins on the graph on its graph
// snapshot; without one, or with one of another text, on the text imported;
// and a graph step changed is named, as a text step is.
func TestHybridCheck(t *testing.T) {
	c, dir := hybridConfig(t)
	if _, err := c.Run(options(dir)); err != nil {
		t.Fatal(err)
	}
	check := func() (string, error) {
		var log bytes.Buffer
		o := options(dir)
		o.W = &log
		out, err := c.Check(o, 4)
		if err == nil && string(out) != product {
			t.Fatalf("the check returned\n%s", out)
		}
		return log.String(), err
	}
	log, err := check()
	if err != nil || !strings.Contains(log, "3 of them began on the graph: 3 read from the store of graphs, 0 imported from the text") {
		t.Fatalf("%v\n%s", err, log)
	}
	// none for q002, and q003's of another text: q001's manifest
	m := func(n int) string { return filepath.Join(dir, "snap", graphSnapName(n)+".gm") }
	m1, _ := os.ReadFile(m(1))
	m2, _ := os.ReadFile(m(2))
	m3, _ := os.ReadFile(m(3))
	os.Remove(m(2))
	os.WriteFile(m(3), m1, 0o644)
	log, err = check()
	if err != nil || !strings.Contains(log, "3 of them began on the graph: 1 read from the store of graphs, 2 imported from the text") ||
		!strings.Contains(log, "phase 4 imports q003.c: q003's graph is not q003.c's") ||
		strings.Contains(log, "phase 3 imports") {
		t.Fatalf("%v\n%s", err, log)
	}
	// a store that cannot give a graph whole is refused, never read: the
	// phases it was for import, and the check says why
	pack := filepath.Join(dir, "snap", "graphs.pack")
	idx := filepath.Join(dir, "snap", "graphs.idx")
	p, _ := os.ReadFile(pack)
	x, _ := os.ReadFile(idx)
	for _, bad := range []struct {
		name, why string
		spoil     func()
	}{
		{"a byte of the pack changed", "its digest differs", func() {
			q := bytes.Clone(p)
			q[len(q)/2] ^= 1
			os.WriteFile(pack, q, 0o644)
		}},
		{"the pack cut short", "graphs.pack: unexpected EOF", func() { os.WriteFile(pack, p[:len(p)-1], 0o644) }},
		{"the index cut short", "graphs.idx: not a whole index", func() { os.WriteFile(idx, x[:len(x)-1], 0o644) }},
		{"no index (a run stopped part way)", "graphs.idx: no such file", func() { os.Remove(idx) }},
		{"a key the index does not hold", "is not in the store", func() {
			y := bytes.Clone(x)
			y[len("whim graph store 1\n")] ^= 1
			os.WriteFile(idx, y, 0o644)
		}},
	} {
		os.WriteFile(m(2), m2, 0o644)
		os.WriteFile(m(3), m3, 0o644)
		os.WriteFile(pack, p, 0o644)
		os.WriteFile(idx, x, 0o644)
		bad.spoil()
		log, err = check()
		if err != nil || strings.Contains(log, "3 read from the store") || !strings.Contains(log, bad.why) {
			t.Fatalf("%s: %v\n%s", bad.name, err, log)
		}
	}
	os.WriteFile(pack, p, 0o644)
	os.WriteFile(idx, x, 0o644)
	// the control: phase 2's graph step writes another literal
	c.Plan[2].Steps[0].Args = []string{"cube", "9"}
	log, err = check()
	if err == nil || !strings.Contains(log, "phase 2      gives") {
		t.Fatalf("a changed graph step was not named: %v\n%s", err, log)
	}
}

// A graph step with no graph table, or not in it, is refused by name.
func TestHybridRefusals(t *testing.T) {
	c, _ := config(t)
	text, _ := os.ReadFile("testdata/toy.c")
	scratch := t.TempDir()
	p := Phase{N: 9, Steps: []Step{{Op: "gcall", Graph: true, Args: []string{"cube", "8"}}}}
	if _, err := c.RunPhase(p, text, scratch, io.Discard); err == nil || !strings.Contains(err.Error(), "the pipeline has no graph steps") {
		t.Errorf("no graph table: %v", err)
	}
	c.GraphLookup = func(string) (GraphOp, bool) { return nil, false }
	if _, err := c.RunPhase(p, text, scratch, io.Discard); err == nil || err.Error() != `no graph step named "gcall"` {
		t.Errorf("no such graph step: %v", err)
	}
}

// A phase that begins on the graph, handed it or not, gives the same text;
// and an in-phase sweep on the graph is the collection, through the editor,
// so that the step after it edits on the same index.
func TestAdvanceFrom(t *testing.T) {
	c, _ := hybridConfig(t)
	text, _ := os.ReadFile("testdata/toy.c")
	seed, err := Seed(text, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := Phase{N: 9, Steps: []Step{{Op: "sweep"}, {Op: "gcall", Graph: true, Args: []string{"cube", "8"}}}}
	a, conv, err := c.AdvanceFrom(p, seed, nil, io.Discard)
	if err != nil || conv.Imports != 1 || conv.Collections != 1 { // the sweep on text: no graph is held yet
		t.Fatalf("%v: %s", err, conv)
	}
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "toy.c"), seed)
	if err != nil {
		t.Fatal(err)
	}
	b, conv, err := c.AdvanceFrom(p, seed, g, io.Discard)
	if err != nil || conv.Imports != 0 || conv.Collections != 2 || !bytes.Equal(a, b) {
		t.Fatalf("%v: %s\n%s\n%s", err, conv, a, b)
	}
	if strings.Contains(string(a), "cube") || strings.Contains(string(a), "unused") {
		t.Errorf("not collected:\n%s", a)
	}
}
