package graph

import (
	"os"
	"testing"
	"time"
)

// TestFragPared: the pared unit (fragpare.go) against the whole file's,
// FRAG's unit before it: on whim-vim.c's graph, splices of each kind -- an
// expression, items with a local, a function's body, a top-level form that
// names a typedef, a tag, an enumerator and a prototype -- made once each
// way on a graph of their own, the graphs Equal.  The pipeline's own
// FRAGs are held by whim-build-check, byte for byte.
func TestFragPared(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	src, err := os.ReadFile("../../src/whim-vim.c")
	if err != nil {
		t.Skip("no src/whim-vim.c")
	}
	g0, _, err := Import("whim-vim.c", src)
	if err != nil {
		t.Fatal(err)
	}
	lisp := g0.Lisp()
	cases := []struct {
		name string
		make func(e *Editor) []Frag
	}{
		{"an expression", func(e *Editor) []Frag {
			d := e.Defn("ml_clearmarked")
			var at *Node
			Walk(d, func(x *Node) bool {
				if at == nil && x.Is("=") && len(x.Kids) == 3 && x.Kids[1].Atom == "lowest_marked" {
					at = x.Kids[2]
				}
				return at == nil
			})
			return []Frag{{At: e.SpotOf(at), Src: "curbuf->b_ml.ml_line_count + ML_FIND - ML_FIND"}}
		}},
		{"items with a local", func(e *Editor) []Frag {
			var at *Node
			Walk(e.Defn("ml_clearmarked"), func(x *Node) bool {
				if at == nil && x.Is("if") {
					at = x
				}
				return at == nil
			})
			return []Frag{{At: e.SpotBefore(at), Src: "linenr_T first = curbuf->b_ml.ml_locked_low;\nif (first < 0)\n{\n    return;\n}\n"}}
		}},
		{"a body", func(e *Editor) []Frag {
			return []Frag{{At: e.SpotBody(e.Defn("ml_clearmarked")), Src: "if (curbuf == nullptr)\n{\n    return;\n}\nlowest_marked = 0;\n"}}
		}},
		{"a top-level form", func(e *Editor) []Frag {
			return []Frag{{At: e.SpotAfter(e.Defn("ml_clearmarked")), Src: "static bhdr_T *whim_first_block(buf_T *buf)\n{\n    struct block_hdr *hp = buf->b_ml.ml_locked;\n    if (hp == nullptr || ml_find_line(buf, 1, ML_FIND) == nullptr)\n    {\n        return nullptr;\n    }\n    return hp;\n}\n"}}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gs [2]*Graph
			var d [2]time.Duration
			for i, whole := range []bool{true, false} {
				g, err := Read(lisp)
				if err != nil {
					t.Fatal(err)
				}
				e := NewEditor(g)
				fragWhole = whole
				start := time.Now()
				_, err = e.SpliceC(c.make(e)...)
				d[i] = time.Since(start)
				fragWhole = false
				if err != nil {
					t.Fatalf("whole %v: %v", whole, err)
				}
				gs[i] = g
			}
			if err := Equal(gs[0], gs[1]); err != nil {
				t.Fatalf("the pared unit's splice is not the whole one's: %v", err)
			}
			t.Logf("whole %v, pared %v", d[0].Round(time.Millisecond), d[1].Round(time.Millisecond))
		})
	}
}
