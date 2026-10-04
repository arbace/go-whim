package graph

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// TestEDNSamples: every sample's graph written as EDN and read back is the
// same graph, ids and edges, and its C view the text.
func TestEDNSamples(t *testing.T) {
	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			_, canon, g := importSample(t, src)
			e, err := g.EDN()
			if err != nil {
				t.Fatal(err)
			}
			h, err := ReadEDN(e)
			if err != nil {
				t.Fatalf("%v\n%s", err, e)
			}
			if err := Equal(g, h); err != nil {
				t.Fatalf("read back: %v\n%s", err, e)
			}
			out, err := h.C()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, canon) {
				t.Fatalf("the C view of the EDN read back: %s", firstDiff(out, canon))
			}
			if e2, _ := h.EDN(); !bytes.Equal(e, e2) {
				t.Fatal("written again, other bytes")
			}
		})
	}
}

// TestEDNAtoms: each kind of atom, written and read back.
func TestEDNAtoms(t *testing.T) {
	for _, c := range []struct{ atom, edn string }{
		{"ascii_isupper", "ascii_isupper"},
		{"->", "->"},
		{"<<=", "<<="},
		{".", "."},
		{"...", "..."},
		{"/", "/"},
		{"true", "true"},
		{"26", "26"},
		{"0", "0"},
		{"007", `#c/num "007"`},
		{"0x7fUL", `#c/num "0x7fUL"`},
		{"1.5e3f", `#c/num "1.5e3f"`},
		{".5", `#c/num ".5"`},
		{"1'000", `#c/num "1'000"`},
		{"'A'", `#c/char "'A'"`},
		{`'\''`, `#c/char "'\\''"`},
		{`L'x'`, `#c/char "L'x'"`},
		{`"a\"b\\n"`, `"a\\\"b\\\\n"`},
		{`"<limits.h>"`, `"<limits.h>"`},
		{`L"wide"`, `#c/tok "L\"wide\""`},
		{"||", `#c/tok "||"`},
		{"^=", `#c/tok "^="`},
		{"/=", `#c/tok "/="`},
		{"~", `#c/tok "~"`},
		{"-1", `#c/tok "-1"`},
		{"nil", `#c/tok "nil"`},
		{"a:", `#c/tok "a:"`},
	} {
		var b bytes.Buffer
		if err := ednAtom(&b, c.atom); err != nil {
			t.Fatal(err)
		}
		if b.String() != c.edn {
			t.Errorf("%s: written %s, want %s", c.atom, b.String(), c.edn)
		}
		if ednAtomLen(c.atom) != b.Len() {
			t.Errorf("%s: its length %d, written %d", c.atom, ednAtomLen(c.atom), b.Len())
		}
		g := &Graph{Forms: []*Node{NewList(NewAtom("x"), NewAtom(c.atom))}, ids: &Sequential{}}
		g.Number()
		e, err := g.EDN()
		if err != nil {
			t.Fatal(err)
		}
		h, err := ReadEDN(e)
		if err != nil {
			t.Fatalf("%s: %v", c.atom, err)
		}
		if got := h.Forms[0].Kids[1].Atom; got != c.atom {
			t.Errorf("%s: read back %s", c.atom, got)
		}
	}
}

// TestCorpusEDN (GRAPH_BOUNDARIES): every boundary's graph -- read from its
// snapshot, or imported -- written as EDN and read back is the same graph,
// ids and edges (Equal); its C view is the boundary's text byte for byte;
// and it is the import of that text, ids aside (SameGraph).  It times the
// read of the EDN against the read of the Lisp, the best of three, and
// with GRAPH_EDN_OUT=D writes each EDN to D/qNNN.edn, for a reader of EDN
// to read (doc/GRAPH.md, *EDN*).
func TestCorpusEDN(t *testing.T) {
	dir, ns, jobs := boundaries(t)
	out := os.Getenv("GRAPH_EDN_OUT")
	type row struct {
		n                 int
		lisp, edn         int
		readLisp, readEDN time.Duration
		write             time.Duration
	}
	rows := make([]*row, len(ns))
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	for i, n := range ns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			src, g, _, _, err := boundary(dir, n)
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			var e []byte
			start := time.Now()
			e, err = g.EDN()
			write := time.Since(start)
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			h, err := ReadEDN(e)
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			if err := Equal(g, h); err != nil {
				t.Errorf("q%03d: the EDN read back is not the graph: %v", n, err)
				return
			}
			c, err := h.C()
			if err != nil || !bytes.Equal(c, src) {
				t.Errorf("q%03d: the C view of the EDN read back is not the text: %v", n, err)
				return
			}
			ig, _, err := Import(fmt.Sprintf("q%03d.c", n), src)
			if err == nil {
				err = SameGraph(h, ig)
			}
			if err != nil {
				t.Errorf("q%03d: the EDN read back is not the import of its text: %v", n, err)
				return
			}
			lisp := g.Lisp()
			r := &row{n: n, lisp: len(lisp), edn: len(e), write: write}
			r.readLisp = best(3, func() { _, _ = Read(lisp) })
			r.readEDN = best(3, func() { _, _ = ReadEDN(e) })
			if out != "" {
				if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("q%03d.edn", n)), e, 0o644); err != nil {
					t.Error(err)
				}
			}
			rows[i] = r
		}()
	}
	wg.Wait()
	var lisp, edn int
	var rl, re []float64
	for _, r := range rows {
		if r == nil {
			continue
		}
		lisp += r.lisp
		edn += r.edn
		rl = append(rl, float64(r.readLisp.Microseconds())/1000)
		re = append(re, float64(r.readEDN.Microseconds())/1000)
		t.Logf("q%03d: Lisp %d bytes, read %v; EDN %d bytes (%.2f), read %v (%.2f), written %v",
			r.n, r.lisp, ms(r.readLisp), r.edn, float64(r.edn)/float64(r.lisp), ms(r.readEDN),
			float64(r.readEDN)/float64(r.readLisp), ms(r.write))
	}
	slices.Sort(rl)
	slices.Sort(re)
	if len(rl) > 0 {
		t.Logf("%d boundaries: Lisp %d bytes, EDN %d (%.2f); the read, median, Lisp %.1f ms, EDN %.1f ms (%.1f-%.1f, %.1f-%.1f)",
			len(rl), lisp, edn, float64(edn)/float64(lisp), rl[len(rl)/2], re[len(re)/2], rl[0], rl[len(rl)-1], re[0], re[len(re)-1])
	}
}

// TestCorpusEDNClojure holds the EDN to a reader of EDN that is not ours:
// Clojure's (GRAPH_CLOJURE_CP, the classpath of clojure.jar and the two
// spec jars it needs).  testdata/edn2lisp.clj reads each GRAPH_EDN_OUT/qNNN.edn
// TestCorpusEDN wrote with clojure.edn/read and writes it back as the
// graph's Lisp; read by Read, that is the boundary's graph, ids and edges.
func TestCorpusEDNClojure(t *testing.T) {
	cp, out := os.Getenv("GRAPH_CLOJURE_CP"), os.Getenv("GRAPH_EDN_OUT")
	if cp == "" || out == "" {
		t.Skip("GRAPH_CLOJURE_CP or GRAPH_EDN_OUT is not set")
	}
	dir, ns, jobs := boundaries(t)
	args := []string{"-Djava.io.tmpdir=" + os.TempDir(), "-Dclojure.main.report=stderr", "-cp", cp, "clojure.main", "testdata/edn2lisp.clj"}
	for _, n := range ns {
		f := filepath.Join(out, fmt.Sprintf("q%03d", n))
		args = append(args, f+".edn", f+".clj.lisp")
	}
	start := time.Now()
	if b, err := exec.Command("java", args...).CombinedOutput(); err != nil {
		t.Fatalf("clojure: %v\n%s", err, b)
	}
	t.Logf("clojure.edn read the %d EDN files and wrote them as Lisp in %v", len(ns), ms(time.Since(start)))
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	for _, n := range ns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			_, g, _, _, err := boundary(dir, n)
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			b, err := os.ReadFile(filepath.Join(out, fmt.Sprintf("q%03d.clj.lisp", n)))
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			h, err := Read(b)
			if err == nil {
				err = Equal(g, h)
			}
			if err != nil {
				t.Errorf("q%03d: through clojure.edn: %v", n, err)
			}
		}()
	}
	wg.Wait()
}

// TestEDNControl: one edge retargeted in the EDN, `#g/r N` made another
// node's, is caught by Equal; a token changed, by the C view.
func TestEDNControl(t *testing.T) {
	_, canon, g := importSample(t, samples["functions and objects"])
	e, err := g.EDN()
	if err != nil {
		t.Fatal(err)
	}
	var from, to ID
	g.Walk(func(n *Node) bool {
		if from == 0 && len(n.Refs) == 1 && n.Refs[0].ID > 1 {
			from, to = n.Refs[0].ID, n.Refs[0].ID-1
		}
		return true
	})
	bad := bytes.Replace(e, fmt.Appendf(nil, "#g/r %d]", from), fmt.Appendf(nil, "#g/r %d]", to), 1)
	h, err := ReadEDN(bad)
	if err == nil && Equal(g, h) == nil {
		t.Error("an edge retargeted in the EDN, and the graph read back is the same")
	}
	bad = bytes.Replace(e, []byte(" 1)"), []byte(" 2)"), 1)
	if h, err = ReadEDN(bad); err != nil {
		t.Fatal(err)
	}
	if c, _ := h.C(); bytes.Equal(c, canon) {
		t.Error("a token changed in the EDN, and the C view is the same")
	}
}
