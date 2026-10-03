package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/sweep"
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/cut"
	"github.com/arbace/go-whim/internal/steps"
	"github.com/arbace/go-whim/internal/whim"
)

// FOLDX on the front (doc/GRAPH-MIGRATION.md, B2d): each front phase's cuts
// run as text on q(N-1), and then its closure two ways -- the text's
// xform.FallOutOf, and FoldX on the cut's graph read back from its Lisp,
// seeded with what q(N-1)'s graph had unwritten -- each swept and printed:
// the same C byte for byte, and the same report line for line.  For phase
// 2, whose other step only asks, that is q002.c too.

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

// orderFree is a closure's report with the two numbers the order of its
// parameter rule moves left out: the text takes the first parameter of Go's
// map order each round (19 to 23 in phase 3, run after run, the bytes the
// same), the graph the first in the file's.
func orderFree(rep []string) string {
	params := regexp.MustCompile(`\d+ (parameters every call passes one constant)`)
	rounds := regexp.MustCompile(`; \d+ rounds$`)
	var out []string
	for _, l := range rep {
		out = append(out, rounds.ReplaceAllString(params.ReplaceAllString(l, "N $1"), "; N rounds"))
	}
	return strings.Join(out, "\n")
}

func TestFoldXFront(t *testing.T) {
	dir, _, _, _ := setup(t)
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("phase%d", n), func(t *testing.T) {
			if n == 1 {
				t.Setenv("REMOVED", declaredRows(t))
			}
			in := snapOf(t, dir, n-1)
			cut, hold := steps.FrontCut(n)
			out, err := cut(in, nil, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			// the text's closure, swept and printed
			var tlog bytes.Buffer
			t0 := time.Now()
			text, err := xform.FallOutOf(func(b []byte, _ []string, _ io.Writer) ([]byte, error) { return out, nil }, hold...)(in, nil, &tlog)
			if err != nil {
				t.Fatal(err)
			}
			tText := time.Since(t0)
			path := filepath.Join(t.TempDir(), "whim-vim.c")
			swept, _, err := sweep.Prune(text, path, whim.Profile.Sweep)
			if err != nil {
				t.Fatal(err)
			}
			want, err := cemit.Canonical(path, swept)
			if err != nil {
				t.Fatal(err)
			}
			// the graph's: what q(N-1) had unwritten, and the closure on
			// the cut's graph read back
			g0, _, err := graph.Import(path, in)
			if err != nil {
				t.Fatal(err)
			}
			before := graph.NewEditor(g0).Unwritten(nil)
			g1, _, err := graph.Import(path, out)
			if err != nil {
				t.Fatal(err)
			}
			h, err := graph.Read(g1.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			e := graph.NewEditor(h)
			t1 := time.Now()
			st, err := e.FoldX(graph.FoldX{Before: before, Hold: hold})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("phase %d: the text's closure %v (its two analyses' parses included), FoldX %v", n, tText.Round(time.Millisecond), time.Since(t1).Round(time.Millisecond))
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
				t.Errorf("phase %d: FoldX is not the text's closure: %s", n, firstDiff(got, want))
			}
			var wantRep []string
			for _, l := range strings.Split(strings.TrimSpace(tlog.String()), "\n") {
				wantRep = append(wantRep, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "fallout")))
			}
			if g, w := orderFree(st.Report), orderFree(wantRep); g != w {
				t.Errorf("phase %d: the report\n%s\nthe text's\n%s", n, g, w)
			}
			if n != 2 {
				return
			}
			if !bytes.Equal(got, snapOf(t, dir, 2)) {
				t.Errorf("phase 2: not q002.c")
			}
			// the control: the && and || rule left out moves the bytes
			h, err = graph.Read(g1.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			e = graph.NewEditor(h)
			if _, err := e.FoldX(graph.FoldX{Before: before, Hold: hold, Off: []string{"logic"}}); err != nil {
				t.Fatal(err)
			}
			if _, err := e.Collect(whim.GraphCollect()); err != nil {
				t.Fatal(err)
			}
			if c, _ := h.C(); bytes.Equal(c, want) {
				t.Error("the control: FoldX without its && and || rule gives the same bytes")
			}
		})
	}
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
	// the text's own numbers, on the same core
	core := in[:bytes.Index(in, []byte("\n#include"))+1]
	_, n, took := xform.EmptyBlocksRule(core)
	if st.Blocks != n || strings.Join(st.Locals, " ") != strings.Join(took, " ") {
		t.Errorf("graph: %v; text: %d blocks, locals %q", st, n, took)
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
	text, err := cut.NoTags(in, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want, err := cemit.Canonical(path, text)
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

// FoldX with every unwritten object and member a seed, xform.FallOut's own
// step, on snapshots where the text closure runs (before q040 it refuses
// itself: a declaration it cuts is used after), each swept: the same C.
func TestFoldXEverySeed(t *testing.T) {
	dir, _, _, _ := setup(t)
	for _, n := range []int{40, 70, 103} {
		in := snapOf(t, dir, n)
		path := filepath.Join(t.TempDir(), "whim-vim.c")
		t0 := time.Now()
		text, err := xform.FallOut()(in, nil, io.Discard)
		if err != nil {
			t.Fatalf("q%03d: the text closure: %v", n, err)
		}
		tText := time.Since(t0)
		swept, _, err := sweep.Prune(text, path, whim.Profile.Sweep)
		if err != nil {
			t.Fatal(err)
		}
		want, err := cemit.Canonical(path, swept)
		if err != nil {
			t.Fatal(err)
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
		if !bytes.Equal(got, want) {
			t.Errorf("q%03d: %s", n, firstDiff(got, want))
		}
		t.Logf("q%03d: the text closure %v, FoldX %v", n, tText.Round(time.Millisecond), tGraph.Round(time.Millisecond))
	}
}
