package view

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
)

// EDITABLE VIEWS, the first gate (doc/GRAPH.md).

// defView is `def NAME` as a Render; usesView `uses NAME`.
func defView(name string, ids bool) Render {
	return func(ix *Index) (string, []Span, error) {
		ns, err := ix.Find(name)
		if err != nil {
			return "", nil, err
		}
		s, sp := Printer{IDs: ids}.RenderForm(Def(ix, ns[0]))
		return s, sp, nil
	}
}

func usesView(name string, ids bool) Render {
	return func(ix *Index) (string, []Span, error) {
		ns, err := ix.Find(name)
		if err != nil {
			return "", nil, err
		}
		s, sp := Printer{IDs: ids}.Render(ix, Uses(ix, ns[0], Options{}))
		return s, sp, nil
	}
}

// copyOf is g read back from its Lisp: what Edit edits, the same ids.
func copyOf(t *testing.T, g *graph.Graph) *graph.Graph {
	t.Helper()
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// edited is the view's text with old replaced by new, once.
func edited(t *testing.T, r Render, g *graph.Graph, old, new string) string {
	t.Helper()
	text, _, err := r(NewIndex(g))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(text, old) != 1 {
		t.Fatalf("%q is in the view %d times:\n%s", old, strings.Count(text, old), text)
	}
	return strings.Replace(text, old, new, 1)
}

// checkSpans holds a text's span table to it: every span inside its
// parent, every form's span exactly its form -- read back, it is the
// node's Lisp, its marks when the ids are shown -- and the nodes in the
// order the text has them.
func checkSpans(t *testing.T, what, text string, spans []Span, ids bool) {
	t.Helper()
	for i, s := range spans {
		if s.Start < 0 || s.End > len(text) || s.Start >= s.End {
			t.Fatalf("%s: span %d [%d,%d) outside the text", what, i, s.Start, s.End)
		}
		if s.Parent >= 0 {
			p := spans[s.Parent]
			if s.Parent >= i || s.Start < p.Start || s.End > p.End {
				t.Fatalf("%s: span %d [%d,%d) not inside its parent %d [%d,%d)", what, i, s.Start, s.End, s.Parent, p.Start, p.End)
			}
		}
		if i > 0 && s.Start < spans[i-1].Start {
			t.Fatalf("%s: span %d starts before the one before it", what, i)
		}
		if s.Kind != SpanForm || !s.Whole {
			continue
		}
		x, err := readSx(text[s.Start:s.End], ids)
		if err != nil || len(x.kids) != 1 {
			t.Fatalf("%s: span %d %q is not one form (%v)", what, i, text[s.Start:s.End], err)
		}
		if got, want := toClisp(x.kids[0]).String(), graph.Lisp(s.Node).String(); got != want {
			t.Fatalf("%s: span %d is %s, its node #%d %s", what, i, got, s.Node.ID, want)
		}
		if ids && s.Node.ID != 0 && !strings.HasPrefix(text[s.Start:], "#"+idText(s.Node.ID)) {
			t.Fatalf("%s: span %d of #%d does not begin with its id: %.40q", what, i, s.Node.ID, text[s.Start:])
		}
		if !ids && s.Node.IsList() && text[s.Start] != '(' {
			t.Fatalf("%s: span %d of a list begins %.20q", what, i, text[s.Start:])
		}
	}
}

// sameSpans holds the tables of one view printed with and without the ids
// to each other: the same nodes, kinds and nesting, in the same order.
func sameSpans(t *testing.T, what string, a, b []Span) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("%s: %d spans with the ids, %d without", what, len(a), len(b))
	}
	for i := range a {
		if a[i].Node != b[i].Node || a[i].Kind != b[i].Kind || a[i].Parent != b[i].Parent || a[i].Whole != b[i].Whole {
			t.Fatalf("%s: span %d differs with the ids and without", what, i)
		}
	}
}

func TestSpansSample(t *testing.T) {
	ix := sample(t)
	for _, name := range []string{"get", "main", "fact", "table"} {
		d := Def(ix, root(t, ix, name))
		a, as := Printer{IDs: true}.RenderForm(d)
		b, bs := Printer{}.RenderForm(d)
		if a != (Printer{IDs: true}).Form(d) || b != (Printer{}).Form(d) {
			t.Fatalf("def %s: Render prints otherwise than Form", name)
		}
		checkSpans(t, "def "+name+" --ids", a, as, true)
		checkSpans(t, "def "+name, b, bs, false)
		sameSpans(t, "def "+name, as, bs)
	}
	for _, v := range []struct {
		name string
		tree func() *Tree
	}{
		{"uses opt", func() *Tree { return Uses(ix, root(t, ix, "opt"), Options{}) }},
		{"callers even", func() *Tree { return Callers(ix, root(t, ix, "even"), Options{Depth: -1}) }},
		{"member b_ml", func() *Tree { return Member(ix, root(t, ix, "buf_T.b_ml"), Options{}) }},
	} {
		a, as := Printer{IDs: true}.Render(ix, v.tree())
		b, bs := Printer{}.Render(ix, v.tree())
		if a != (Printer{IDs: true}).Tree(ix, v.tree()) || b != (Printer{}).Tree(ix, v.tree()) {
			t.Fatalf("%s: Render prints otherwise than Tree", v.name)
		}
		checkSpans(t, v.name+" --ids", a, as, true)
		checkSpans(t, v.name, b, bs, false)
		sameSpans(t, v.name, as, bs)
	}
}

// opText is the ops an edit made, one a line.
func opText(ops []Op) string {
	var s []string
	for _, o := range ops {
		s = append(s, o.String())
	}
	return strings.Join(s, "\n")
}

// direct is a change made on a copy of g with the editor's operations:
// what an edit through a view must equal.
func direct(t *testing.T, g *graph.Graph, f func(e *graph.Editor, ix *Index) error) *graph.Graph {
	t.Helper()
	h := copyOf(t, g)
	e := graph.NewEditor(h)
	if err := f(e, NewIndex(h)); err != nil {
		t.Fatal(err)
	}
	e.Recheck()
	return h
}

// imported holds g to the import of its C view, ids aside, and gives the C.
func imported(t *testing.T, g *graph.Graph) []byte {
	t.Helper()
	c, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	h, _, err := graph.Import(filepath.Join(t.TempDir(), "edited.c"), c)
	if err != nil {
		t.Fatalf("the edited C does not import: %v", err)
	}
	if err := graph.SameGraph(g, h); err != nil {
		t.Fatalf("the edited graph is not the import of its C view: %v", err)
	}
	return c
}

func TestEditSample(t *testing.T) {
	g := sample(t).G
	find := func(ix *Index, _ *graph.Graph, id graph.ID) *graph.Node { return ix.Node(id) }
	cases := []struct {
		name     string
		view     Render
		old, new string
		ops      string
		direct   func(e *graph.Editor, ix *Index) error
	}{
		{"a constant", defView("get", false), "(+= (-> b b_ml) 2)", "(+= (-> b b_ml) 3)",
			"replace #83 with b->b_ml += 3;",
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotOf(find(ix, nil, 83)), Src: "b->b_ml += 3;"})
				return err
			}},
		{"a parameter renamed", defView("get", false), "(n int)) int)\n  (= (-> b b_ml) n)", "(m int)) int)\n  (= (-> b b_ml) m)",
			"rename #73 n to m",
			func(e *graph.Editor, ix *Index) error { _, err := e.Rename(find(ix, nil, 73), "m"); return err }},
		{"a statement deleted", defView("get", false), "  (post++ (-> b b_ml))\n", "",
			"delete #79 (post++)",
			func(e *graph.Editor, ix *Index) error { return e.Delete(find(ix, nil, 79)) }},
		{"items inserted", defView("get", false), "  (post++ (-> b b_ml))\n", "  (post++ (-> b b_ml))\n  (def k int 3)\n  (+= opt k)\n",
			"insert before #83: int k = 3; opt += k;",
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotBefore(find(ix, nil, 83)), Src: "int k = 3;\nopt += k;\n"})
				return err
			}},
		{"an else added", defView("get", false), "(return 0)))", "(return 0))\n    (block (return 1)))",
			"replace #87 with if (b->next) { opt = 0; return 0; } else { return 1; }",
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotOf(find(ix, nil, 87)), Src: "if (b->next)\n{\n    opt = 0;\n    return 0;\n}\nelse\n{\n    return 1;\n}\n"})
				return err
			}},
		{"an expression", defView("main", false), "(call fact 3)", "(call fact (+ 3 1))",
			"replace #143 with fact(3 + 1)",
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotOf(find(ix, nil, 143)), Src: "fact(3 + 1)"})
				return err
			}},
		{"a use's statement deleted", usesView("opt", false), "\n    (:write (= opt 1))", "",
			"delete #137 (=)",
			func(e *graph.Editor, ix *Index) error { return e.Delete(find(ix, nil, 137)) }},
		{"an entry deleted", usesView("opt", false), "\n  (in main\n    (:write (= opt 1)))", "",
			"delete #137 (=)",
			func(e *graph.Editor, ix *Index) error { return e.Delete(find(ix, nil, 137)) }},
		{"a context's form", usesView("opt", false), "(= opt 0)", "(= opt 2)",
			"replace #92 with opt = 2;",
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotOf(find(ix, nil, 92)), Src: "opt = 2;"})
				return err
			}},
		{"with the ids shown", defView("get", true), "#83(+= #84(-> #85:b@70 #86:b_ml@3) 2)", "#83(+= #84(-> #85:b@70 #86:b_ml@3) 3)",
			"replace #83 with b->b_ml += 3;",
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotOf(find(ix, nil, 83)), Src: "b->b_ml += 3;"})
				return err
			}},
		{"with the ids shown, a form written without", defView("get", true), "#83(+= #84(-> #85:b@70 #86:b_ml@3) 2)", "(+= (-> b b_ml) 3)",
			"replace #83 with b->b_ml += 3;",
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotOf(find(ix, nil, 83)), Src: "b->b_ml += 3;"})
				return err
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := g.Lisp()
			r, err := Edit(g, c.view, edited(t, c.view, g, c.old, c.new), EditOptions{IDs: strings.Contains(c.name, "ids")})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, g.Lisp()) {
				t.Fatal("the graph handed in was changed")
			}
			if got := opText(r.Ops); got != c.ops {
				t.Fatalf("ops:\n%s\nwant:\n%s", got, c.ops)
			}
			if err := graph.Equal(r.Graph, direct(t, g, c.direct)); err != nil {
				t.Fatalf("the edit through the view is not the editor's: %v", err)
			}
			imported(t, r.Graph)
		})
	}
}

func TestEditRefusals(t *testing.T) {
	g := sample(t).G
	cases := []struct {
		name     string
		view     Render
		old, new string
		fallout  bool
		why      string
	}{
		{"an undeclared name", defView("get", false), "(= (-> b b_ml) n)", "(= (-> b b_ml) nope)", false, "undefined: nope"},
		{"a member the type has not", defView("get", false), "(-> b next)", "(-> b nexx)", false, "no member named nexx"},
		{"a pointer in an int", defView("get", false), "(= opt 0)", "(= opt (-> b next))", false, "a type that does not agree: `opt = b->next` stores a value of pointer in arith"},
		{"an int in a pointer", defView("main", false), "(def p (ptr int) (addr (. b b_ml)))", "(def p (ptr int) (. b b_ml))", false, "a type that does not agree: `int *p = b.b_ml;`"},
		{"a struct in an int", defView("main", false), "(= (. o b_ml) 1)", "(= (. o b_ml) b)", false, "stores a value of struct buf in arith"},
		{"a function still called", defView("fact", false), "", "", false, "which the edit deleted (2 uses so): --fallout"},
		{"a function still called, its value used", defView("fact", false), "", "", true, "fall-out: "},
		{"a label", usesView("opt", false), "(:write (= opt 0))", "(:read (= opt 0))", false, "a context's label"},
		{"an entry's name", usesView("opt", false), "(in main", "(in mainx", false, "an entry's name or head"},
		{"a form added beside the view's", usesView("opt", false), "\n    (:write (= opt 1))", "\n    (:write (= opt 1))\n    (= opt 3)", false, "added in the view's own structure"},
		{"what the view elides", nil, "", "", false, "elides"},
		{"a form that does not read", defView("get", false), "(= opt 0)", "(= opt 0", false, "the edited text, line"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ed string
			switch {
			case c.name == "what the view elides":
				// get's b's uses: the if's block holds none, and is elided
				r := usesView("get/b", false)
				c.view = r
				text, _, _ := r(NewIndex(g))
				if !strings.Contains(text, "(if (-> b next) ...)") {
					t.Fatalf("no elided block in:\n%s", text)
				}
				ed = strings.Replace(text, "(if (-> b next) ...)", "(if (-> b next) (block))", 1)
			case c.old == "":
				ed = "" // the whole view deleted
			default:
				ed = edited(t, c.view, g, c.old, c.new)
			}
			before := g.Lisp()
			opt := EditOptions{}
			if c.fallout {
				opt.FallOut = &graph.FallOutOptions{}
			}
			_, err := Edit(g, c.view, ed, opt)
			var ref *Refusal
			if !errors.As(err, &ref) || !strings.Contains(err.Error(), c.why) {
				t.Fatalf("got %v, want a refusal saying %q", err, c.why)
			}
			t.Log(err)
			if !bytes.Equal(before, g.Lisp()) {
				t.Fatal("a refused edit changed the graph")
			}
		})
	}
}

// TestEditFallOut: a deletion's dangling uses closed over, when asked.
func TestEditFallOut(t *testing.T) {
	src := []byte("static int n;\nstatic void bump(void) { n++; }\nint main(void) { bump(); bump(); return n; }\n")
	g0, _, err := graph.Import(filepath.Join(t.TempDir(), "f.c"), src)
	if err != nil {
		t.Fatal(err)
	}
	g := copyOf(t, g0)
	r, err := Edit(g, defView("bump", false), "", EditOptions{FallOut: &graph.FallOutOptions{}})
	if err != nil {
		t.Fatal(err)
	}
	c := imported(t, r.Graph)
	w, _, err := graph.Import(filepath.Join(t.TempDir(), "w.c"), []byte("static int n;\nint main(void) { return n; }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := w.C(); !bytes.Equal(c, want) {
		t.Fatalf("got\n%s\nwant\n%s", c, want)
	}
	if len(r.FallOut.Removed) != 2 {
		t.Fatalf("the closure removed %d calls, not 2", len(r.FallOut.Removed))
	}
}

// TestEditControl: an alignment made wrong on purpose -- the change put
// on the statement after the one changed -- is caught by the view printed
// again, and refused.
func TestEditControl(t *testing.T) {
	g := sample(t).G
	v := defView("get", false)
	ed := edited(t, v, g, "(+= (-> b b_ml) 2)", "(+= (-> b b_ml) 3)")
	tamper = func(e *graph.Editor, ops []Op) { ops[0].Node = e.Sibling(ops[0].Node, 1) }
	defer func() { tamper = nil }()
	_, err := Edit(g, v, ed, EditOptions{})
	if err == nil || !strings.Contains(err.Error(), "the alignment is wrong") {
		t.Fatalf("a wrong alignment was not caught: %v", err)
	}
}

// gcc, when there is one: the C compiles with the pipeline's flags.
func syntaxOK(t *testing.T, c []byte) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Log("no gcc: the C not compiled")
		return
	}
	f := filepath.Join(t.TempDir(), "edited.c")
	if err := os.WriteFile(f, c, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("gcc", "-O0", "-fno-stack-protector", "-fsyntax-only", f).CombinedOutput()
	if err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}
}

// THE PRODUCT: on whim-vim.c when it is there (GRAPH_VIEW_FILE names
// another).

// TestSpansProduct: def F of every function, printed with the ids and
// without, its span table held to the text and the two tables to each
// other.
func TestSpansProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	ix, _ := product(t)
	n, bytesOut := 0, 0
	var with, without time.Duration
	for _, f := range ix.G.Forms {
		if !f.Is("defn") {
			continue
		}
		start := time.Now()
		a, as := Printer{IDs: true}.RenderForm(f)
		with += time.Since(start)
		start = time.Now()
		b, bs := Printer{}.RenderForm(f)
		without += time.Since(start)
		name := "def " + graph.DeclName(f)
		if b != (Printer{}).Form(f) {
			t.Fatalf("%s: Render prints otherwise than Form", name)
		}
		checkSpans(t, name+" --ids", a, as, true)
		checkSpans(t, name, b, bs, false)
		sameSpans(t, name, as, bs)
		n++
		bytesOut += len(b)
	}
	if n < 1000 {
		t.Fatalf("only %d definitions", n)
	}
	start := time.Now()
	for _, f := range ix.G.Forms {
		if f.Is("defn") {
			_ = Printer{}.Form(f)
		}
	}
	plain := time.Since(start)
	start = time.Now()
	_ = copyOf(t, ix.G)
	t.Logf("%d definitions, %d bytes: rendered with their spans in %v (ids) and %v, without them %v; the graph copied (written, read) in %v",
		n, bytesOut, with, without, plain, time.Since(start))
}

// productEdit is an edit through a view of whim-vim.c's graph, held to
// the same change made with the editor's operations (Equal: the same ids
// too), its C to the import of it (SameGraph) and to gcc.
func productEdit(t *testing.T, g *graph.Graph, r Render, ed string, opt EditOptions, f func(e *graph.Editor, ix *Index) error) *Result {
	t.Helper()
	before := g.Lisp()
	start := time.Now()
	res, err := Edit(g, r, ed, opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %v", opText(res.Ops), time.Since(start))
	if !bytes.Equal(before, g.Lisp()) {
		t.Fatal("the graph handed in was changed")
	}
	if f != nil {
		if err := graph.Equal(res.Graph, direct(t, g, f)); err != nil {
			t.Fatalf("the edit through the view is not the editor's: %v", err)
		}
	}
	syntaxOK(t, imported(t, res.Graph))
	return res
}

func TestEditProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	ix, _ := product(t)
	g := ix.G
	const fn = "ml_clearmarked"
	def := defView(fn, false)
	last := func(ix *Index) *graph.Node {
		d := Def(ix, root(t, ix, fn))
		return d.Kids[len(d.Kids)-1]
	}

	t.Run("a constant", func(t *testing.T) {
		productEdit(t, g, def, edited(t, def, g, "(= lowest_marked 0))", "(= lowest_marked 1))"), EditOptions{},
			func(e *graph.Editor, ix *Index) error {
				_, err := e.SpliceC(graph.Frag{At: e.SpotOf(last(ix)), Src: "lowest_marked = 1;"})
				return err
			})
	})
	t.Run("a local renamed", func(t *testing.T) {
		text, _, _ := def(NewIndex(g))
		ed := strings.NewReplacer("(def i int)", "(def k int)", "(= i ", "(= k ", "(pre++ i)", "(pre++ k)", "db_line) i)", "db_line) k)").Replace(text)
		r := productEdit(t, g, def, ed, EditOptions{}, func(e *graph.Editor, ix *Index) error {
			_, err := e.Rename(root(t, ix, fn+"/i"), "k")
			return err
		})
		if len(r.Ops) != 1 || r.Ops[0].Kind != "rename" {
			t.Fatalf("not a renaming: %s", opText(r.Ops))
		}
	})
	t.Run("an expression, with the ids shown", func(t *testing.T) {
		v := defView(fn, true)
		text, spans, _ := v(NewIndex(g))
		// the step's ++lnum, found by its span: the second operand of the comma
		var at Span
		for _, s := range spans {
			if s.Kind == SpanForm && s.Node.Is("comma") {
				at = spans[indexOf(spans, s.Node.Kids[2])]
			}
		}
		ed := text[:at.Start] + "(+= lnum 1)" + text[at.End:]
		var id graph.ID
		productEdit(t, g, v, ed, EditOptions{IDs: true}, func(e *graph.Editor, ix *Index) error {
			n := ix.Node(at.Node.ID)
			id = n.ID
			_, err := e.SpliceC(graph.Frag{At: e.SpotOf(n), Src: "lnum += 1"})
			return err
		})
		if id == 0 {
			t.Fatal("no step found")
		}
	})
	t.Run("a statement deleted in a uses view", func(t *testing.T) {
		v := usesView("p_wiv", false)
		text, spans, _ := v(NewIndex(g))
		var at Span
		for _, s := range spans {
			if s.Kind == SpanContext && ix.Holder(s.Node) == root(t, ix, "ttest") {
				at = s
			}
		}
		if at.Node == nil {
			t.Fatal("no context in ttest")
		}
		start := strings.LastIndexByte(text[:at.Start], '\n')
		productEdit(t, g, v, text[:start]+text[at.End:], EditOptions{}, func(e *graph.Editor, ix *Index) error {
			return e.Delete(ix.Node(at.Node.ID))
		})
	})
	t.Run("a function deleted, its calls closed over", func(t *testing.T) {
		fo := graph.FallOutOptions{KeepEmpty: true}
		r := productEdit(t, g, def, "", EditOptions{FallOut: &fo}, nil)
		if len(r.FallOut.Removed) == 0 {
			t.Fatal("no call removed")
		}
		for _, rm := range r.FallOut.Removed {
			t.Logf("fall-out: %s in %s", rm.Rule, rm.Fn)
		}
	})

	// refused, the graph untouched
	for _, c := range []struct {
		name, old, new, why string
	}{
		{"an undeclared name", "(= lowest_marked 0))", "(= lowest_marked no_such_name))", "undefined: no_such_name"},
		{"a type that does not agree", "(= lowest_marked 0))", "(= lowest_marked curbuf))", "a type that does not agree: `lowest_marked = curbuf` stores a value of pointer in arith"},
		{"a function still called", "", "", "which the edit deleted"},
		{"a member the type has not", "(= dp (-> hp bh_data))", "(= dp (-> hp bh_datum))", "no member named bh_datum"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ed := ""
			if c.old != "" {
				ed = edited(t, def, g, c.old, c.new)
			}
			before := g.Lisp()
			_, err := Edit(g, def, ed, EditOptions{})
			var ref *Refusal
			if !errors.As(err, &ref) || !strings.Contains(err.Error(), c.why) {
				t.Fatalf("got %v, want a refusal saying %q", err, c.why)
			}
			t.Log(err)
			if !bytes.Equal(before, g.Lisp()) {
				t.Fatal("a refused edit changed the graph")
			}
		})
	}

	t.Run("the control", func(t *testing.T) {
		ed := edited(t, def, g, "(= lowest_marked 0))", "(= lowest_marked 1))")
		tamper = func(e *graph.Editor, ops []Op) { ops[0].Node = e.Sibling(ops[0].Node, -1) }
		defer func() { tamper = nil }()
		_, err := Edit(g, def, ed, EditOptions{})
		if err == nil || !strings.Contains(err.Error(), "the alignment is wrong") {
			t.Fatalf("a wrong alignment was not caught: %v", err)
		}
	})
}

func indexOf(spans []Span, n *graph.Node) int {
	for i, s := range spans {
		if s.Kind == SpanForm && s.Node == n {
			return i
		}
	}
	return -1
}
