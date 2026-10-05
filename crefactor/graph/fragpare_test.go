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
		{"a local shadowing one used after it", func(e *Editor) []Frag {
			// in ml_clearmarked's inner loop, before the statement using
			// lnum: a new lnum, which every later use in the block is then
			var at *Node
			Walk(e.Defn("ml_clearmarked"), func(x *Node) bool {
				if at == nil && x.Is("block") && len(x.Kids) > 2 {
					for _, k := range x.Kids[1:] {
						if k.Is("=") && len(k.Kids) == 3 && k.Kids[1].Atom == "dp" {
							at = k
						}
					}
				}
				return at == nil
			})
			return []Frag{{At: e.SpotBefore(at), Src: "linenr_T lnum = 0;\n"}}
		}},
		{"a goto to a label of the function", func(e *Editor) []Frag {
			// a goto before the label, in its block: the statements
			// between them left out, the label's item printed
			var lbl *Node
			for _, f := range e.g.Forms {
				if f.Is("defn") && lbl == nil {
					Walk(f, func(x *Node) bool {
						if lbl == nil && x.Is("label") && e.Parent(x) != nil && itemsFrom(e.Parent(x)) > 0 {
							lbl = x
						}
						return lbl == nil
					})
				}
			}
			b := e.Parent(lbl)
			first := b.Kids[itemsFrom(b)]
			return []Frag{{At: e.SpotBefore(first), Src: "if (0)\n{\n    goto " + lbl.Kids[1].Atom + ";\n}\n"}}
		}},
		{"a statement in a switch's case", func(e *Editor) []Frag {
			var at *Node
			for _, f := range e.g.Forms {
				if !f.Is("defn") || at != nil {
					continue
				}
				Walk(f, func(x *Node) bool {
					if at == nil && x.Is("case") && e.Parent(x) != nil && itemsFrom(e.Parent(x)) > 0 {
						at = x
					}
					return at == nil
				})
			}
			return []Frag{{At: e.SpotAfter(at), Src: "got_int = got_int;\n"}}
		}},
		{"an expression in ex_substitute", func(e *Editor) []Frag {
			var at *Node
			Walk(e.Defn("ex_substitute"), func(x *Node) bool {
				if at == nil && x.Is("=") && len(x.Kids) == 3 && x.Kids[1].Atom == "which_pat" {
					at = x.Kids[2]
				}
				return at == nil
			})
			return []Frag{{At: e.SpotOf(at), Src: "RE_LAST + 0"}}
		}},
		{"a function rewritten beside a splice in its caller", func(e *Editor) []Frag {
			// a function rewritten by a top-level fragment beside a splice
			// in its caller, after the call.  The case the colliding rule's
			// paring guard was written for is phase 40's (a text
			// substitution of report_term_error, header and body, beside a
			// splice in set_termname), held by whim-build-check: here the
			// editor's replacement retargets the call by itself, and the
			// case passes with the guard or without it
			// after the call: before ex_global's last item
			g := e.Defn("ex_global")
			last := g.Kids[len(g.Kids)-1]
			return []Frag{
				{At: e.SpotOf(e.Defn("ml_clearmarked")), Src: "static void\nml_clearmarked(void)\n{\n    lowest_marked = 0;\n}\n"},
				{At: e.SpotBefore(last), Src: "got_int = got_int;\n"},
			}
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
