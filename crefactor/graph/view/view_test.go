package view

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func TestMain(m *testing.M) {
	if os.Getenv("TMPDIR") == "" {
		os.Setenv("TMPDIR", os.TempDir())
	}
	os.Exit(m.Run())
}

// sample is testdata/sample.c's graph, imported and then written and read
// back, so that the views are shown to need no cc node behind the graph.
func sample(t *testing.T) *Index {
	t.Helper()
	src, err := os.ReadFile("testdata/sample.c")
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "sample.c"), src)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return NewIndex(h)
}

func root(t *testing.T, ix *Index, what string) *graph.Node {
	t.Helper()
	ns, err := ix.Find(what)
	if err != nil {
		t.Fatal(err)
	}
	return ns[0]
}

// views are the golden cases: each named view on the sample, its output
// in testdata/NAME.lisp (go test -update writes them).
var views = []struct {
	file string
	make func(t *testing.T, ix *Index) string
}{
	{"callers-fact", func(t *testing.T, ix *Index) string {
		// a recursive function: its caller is itself, a link
		return Printer{}.Tree(ix, Callers(ix, root(t, ix, "fact"), Options{}))
	}},
	{"callers-even", func(t *testing.T, ix *Index) string {
		// mutual recursion, no depth limit: the cycle closes on a link
		return Printer{}.Tree(ix, Callers(ix, root(t, ix, "even"), Options{Depth: -1}))
	}},
	{"callers-even-ids", func(t *testing.T, ix *Index) string {
		return Printer{IDs: true}.Tree(ix, Callers(ix, root(t, ix, "even"), Options{Depth: -1}))
	}},
	{"callees-main", func(t *testing.T, ix *Index) string {
		return Printer{}.Tree(ix, Callees(ix, root(t, ix, "main"), Options{}))
	}},
	{"uses-opt", func(t *testing.T, ix *Index) string {
		return Printer{}.Tree(ix, Uses(ix, root(t, ix, "opt"), Options{}))
	}},
	{"uses-fact-fn", func(t *testing.T, ix *Index) string {
		// a function's uses: its calls, and the table that takes it
		return Printer{}.Tree(ix, Uses(ix, root(t, ix, "fact"), Options{Show: ShowFn}))
	}},
	{"uses-buf_T-node", func(t *testing.T, ix *Index) string {
		return Printer{}.Tree(ix, Uses(ix, root(t, ix, "buf_T"), Options{Show: ShowNode}))
	}},
	{"member-b_ml", func(t *testing.T, ix *Index) string {
		// buf_T's b_ml, not struct other's of the same name
		return Printer{}.Tree(ix, Member(ix, root(t, ix, "buf_T.b_ml"), Options{}))
	}},
	{"type-buf_T", func(t *testing.T, ix *Index) string {
		s, err := ix.Aggregate("buf_T")
		if err != nil {
			t.Fatal(err)
		}
		return Printer{}.Tree(ix, Type(ix, s, Options{}))
	}},
	{"def-get-ids", func(t *testing.T, ix *Index) string {
		return Printer{IDs: true}.Form(Def(ix, root(t, ix, "get")))
	}},
	{"follow-typed", func(t *testing.T, ix *Index) string {
		// ad hoc: every node of the type of get's parameter b, by function
		st, err := ParseSteps("typed typed< ^fn")
		if err != nil {
			t.Fatal(err)
		}
		return Printer{}.Tree(ix, Build(ix, root(t, ix, "get/b"), Spec{Name: "follow", Child: "to", Rel: Steps(st), Depth: 1, Show: ShowNode}))
	}},
}

func TestGolden(t *testing.T) {
	ix := sample(t)
	for _, v := range views {
		t.Run(v.file, func(t *testing.T) {
			got := v.make(t, ix)
			path := filepath.Join("testdata", v.file+".lisp")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestKnown: the sample's answers, stated here rather than only in the
// golden files.
func TestKnown(t *testing.T) {
	ix := sample(t)
	callers := func(f string, o Options) []string {
		var out []string
		for _, k := range Callers(ix, root(t, ix, f), o).Kids {
			s := ix.Name(k.Node)
			if k.Link {
				s = "@" + s
			}
			out = append(out, s)
		}
		return out
	}
	if got := strings.Join(callers("fact", Options{Depth: 1}), " "); got != "@fact main" {
		t.Errorf("callers of fact: %s", got)
	}
	if got := strings.Join(callers("odd", Options{Depth: 1}), " "); got != "even main" {
		t.Errorf("callers of odd: %s", got)
	}
	// the member by type: 5 uses of buf_T's b_ml in get, 4 in main
	// (o.b_ml is struct other's), and what each does
	m := root(t, ix, "buf_T.b_ml")
	acc := map[string]int{}
	for _, u := range ix.Uses(m) {
		acc[ix.Access(u)]++
	}
	if got := tallyText(acc); got != "3 read, 1 write, 2 update, 1 addr, 1 unevaluated" {
		t.Errorf("b_ml's accesses: %s", got)
	}
	o := root(t, ix, "other.b_ml")
	if n := len(ix.Uses(o)); n != 2 {
		t.Errorf("struct other's b_ml: %d uses", n)
	}
	// a typedef's members are its struct's
	if root(t, ix, "struct buf.b_ml") != m || root(t, ix, "buf.b_ml") != m {
		t.Error("buf.b_ml is not buf_T.b_ml")
	}
}

// TestDeterministic: a view is the same text however often it is made,
// and on the graph imported or read back.
func TestDeterministic(t *testing.T) {
	src, _ := os.ReadFile("testdata/sample.c")
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "sample.c"), src)
	if err != nil {
		t.Fatal(err)
	}
	a, b := NewIndex(g), sample(t)
	for i := 0; i < 3; i++ {
		for _, f := range []string{"fact", "even", "main", "get"} {
			x := Printer{IDs: true}.Tree(a, Callers(a, root(t, a, f), Options{Depth: -1}))
			y := Printer{IDs: true}.Tree(b, Callers(b, root(t, b, f), Options{Depth: -1}))
			if x != y {
				t.Errorf("callers %s: imported\n%s\nread back\n%s", f, x, y)
			}
		}
	}
}

// TestSteps: the spec's words, and what it refuses.
func TestSteps(t *testing.T) {
	st, err := ParseSteps(CallersSteps)
	if err != nil || len(st) != 3 || st[0].String() != "refers<" || st[2].String() != "^fn" {
		t.Errorf("%v %v", st, err)
	}
	for _, bad := range []string{"", "refers> x", "^fn<"} {
		if _, err := ParseSteps(bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
}

// THE PRODUCT.  On whim-vim.c when it is there (GRAPH_VIEW_FILE names
// another): the invariants a view must keep on a real program.
func product(t *testing.T) (*Index, []byte) {
	t.Helper()
	f := os.Getenv("GRAPH_VIEW_FILE")
	if f == "" {
		f = "../../../src/whim-vim.c"
	}
	src, err := os.ReadFile(f)
	if err != nil {
		t.Skipf("no %s", f)
	}
	g, _, err := graph.Import(f, src)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return NewIndex(h), src
}

func TestProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	ix, src := product(t)

	// uses X: exactly the refers edges into X's declarations, each once,
	// counted by walking the graph rather than the index
	for _, x := range []string{"p_wiv", "ml_get", "buf_T", "curbuf", "buf_T.b_ml", "pos_T.lnum", "NUL", "ex_substitute/lnum"} {
		ns, err := ix.Find(x)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range ns {
			decls := map[*graph.Node]bool{}
			for _, d := range ix.Decls(n) {
				decls[d] = true
			}
			want := 0
			ix.G.Walk(func(k *graph.Node) bool {
				for _, r := range k.Refs {
					if decls[r] {
						want++
					}
				}
				return true
			})
			tree := Uses(ix, n, Options{})
			got, seen := 0, map[*graph.Node]bool{}
			for _, k := range tree.Kids {
				for _, c := range k.Contexts {
					for _, u := range c.Uses {
						if seen[u] {
							t.Errorf("uses %s: %d shown twice", x, u.ID)
						}
						seen[u] = true
						got++
					}
				}
			}
			if got != want || want == 0 {
				t.Errorf("uses %s: %d uses shown, %d refers edges into it", x, got, want)
			}
		}
	}

	// def F printed through the C view is F's text in the file: every
	// function the file defines
	n := 0
	for _, f := range ix.G.Forms {
		if !f.Is("defn") {
			continue
		}
		d := Def(ix, root(t, ix, graph.DeclName(f)))
		if d != f {
			t.Errorf("def %s is not its definition", graph.DeclName(f))
			continue
		}
		c, err := clisp.Print([]*clisp.Node{graph.Lisp(d)})
		if err != nil {
			t.Fatal(err)
		}
		i := bytes.Index(src, c)
		if i < 0 || i > 0 && !bytes.HasSuffix(src[:i], []byte("\n\n")) {
			t.Errorf("def %s: its C is not a definition of the file:\n%s", graph.DeclName(f), c)
		}
		n++
	}
	if n < 1000 {
		t.Errorf("only %d definitions", n)
	}

	// callers to depth 1: the distinct functions holding a call
	for _, f := range []string{"ml_get", "update_topline", "ml_get_buf"} {
		r := root(t, ix, f)
		want := map[*graph.Node]bool{}
		for _, d := range ix.Decls(r) {
			for _, u := range ix.Uses(d) {
				if ix.InCall(u) {
					want[ix.Rep(ix.Holder(u))] = true
				}
			}
		}
		tree := Callers(ix, r, Options{Depth: 1})
		if len(tree.Kids) != len(want) {
			t.Errorf("callers %s: %d, %d functions hold a call", f, len(tree.Kids), len(want))
		}
		for _, k := range tree.Kids {
			if !want[k.Node] {
				t.Errorf("callers %s: %s holds no call", f, ix.Name(k.Node))
			}
		}
	}

	// every named view, twice: the same text
	for _, v := range []func() string{
		func() string {
			return Printer{IDs: true}.Tree(ix, Callers(ix, root(t, ix, "ml_get"), Options{Depth: 3}))
		},
		func() string { return Printer{}.Tree(ix, Callees(ix, root(t, ix, "main"), Options{Depth: 3})) },
		func() string { return Printer{}.Tree(ix, Member(ix, root(t, ix, "buf_T.b_ml"), Options{})) },
	} {
		if a, b := v(), v(); a != b {
			t.Error("a view is not deterministic")
		}
	}
}
