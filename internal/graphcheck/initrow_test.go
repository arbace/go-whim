package graphcheck

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/whim"
)

// An initialiser element is a one-node place (doc/GRAPH-MIGRATION.md, B0):
// an nv_cmds[] row's handler pointed at nv_error by Replace -- the row kept,
// its id with it, one atom superseded and one given -- on the real
// snapshots, held to the text programs' substitutions byte for byte: phase
// 15a's `!` operator (on q014) and onebuffer's CTRL-^ (phase 4, on q003),
// each on the graph read back from its Lisp.
func TestInitElementPlace(t *testing.T) {
	dir, _, _, _ := setup(t)
	for _, c := range []struct {
		name   string
		before int
		re     string // the text program's substitution
		row    string // the graph's: the row, its handler bound to h
	}{
		{"15a", 14, `(?m)^([ \t]*\{'!', )nv_operator(, 0, 0\},)$`, "(init '!' ?h 0 0)"},
		{"onebuffer", 3, `(?m)^([ \t]*\{Ctrl_HAT, )nv_hat(, NV_NCW, 0\},)$`, "(init Ctrl_HAT ?h NV_NCW 0)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := snapOf(t, dir, c.before)
			want, err := edit.ReplacePattern(in, c.re, "${1}nv_error${2}", 1)
			if err != nil {
				t.Fatal(err)
			}
			g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
			if err != nil {
				t.Fatal(err)
			}
			h, err := graph.Read(g.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			e := graph.NewEditor(h)
			v := graph.NewVerbs(c.name, e, io.Discard)
			var row *graph.Node
			v.InTable("nv_cmds", func(v *graph.Verbs) {
				row = v.Row(c.row, "the row")
				v.RewriteAt(c.row, "h", "nv_error", 1, "the row points at nv_error")
			})
			if err := v.Done(); err != nil {
				t.Fatal(err)
			}
			if err := e.Check(); err != nil {
				t.Fatal(err)
			}
			act := e.Log[len(e.Log)-1]
			if !e.Live(row) || len(act.Gone) != 1 || len(act.New) != 1 || len(e.Untyped) != 0 {
				t.Errorf("the row live %v; the act %+v; %d untyped", e.Live(row), act, len(e.Untyped))
			}
			if d := row.Kids[2].Ref(); d == nil || graph.DeclName(d) != "nv_error" {
				t.Errorf("the handler refers to %v", d)
			}
			got, err := h.C()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("the graph's C view is not the text's: %d bytes against %d", len(got), len(want))
			}
		})
	}
}

// The verbs on a whole phase, outside the plan: phase 15's two programs
// (whim15a, whim15) written on crefactor/graph's verbs -- a table row's
// element rewritten, a case's run dropped, a DropIf, a FoldNever, the
// mentions asserted -- from q014's graph read back from its Lisp, then
// collected as a phase that ends on the graph is: q015.c byte for byte.
// (B1a converts the phase; this holds the verbs it will be written on.)
func TestVerbsOnPhase15(t *testing.T) {
	dir, _, _, _ := setup(t)
	in, want := snapOf(t, dir, 14), snapOf(t, dir, 15)
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := graph.NewEditor(h)
	var log bytes.Buffer
	v := graph.NewVerbs("filters", e, &log)
	v.InTable("nv_cmds", func(v *graph.Verbs) {
		v.RewriteAt("(init '!' ?h 0 0)", "h", "nv_error", 1, "the ! operator's row points at nv_error")
	})
	v.Tag = "noswapfile"
	noswap := "(& (. cmdmod cmod_flags) CMOD_NOSWAPFILE)"
	v.InFunction("parse_command_modifiers", func(v *graph.Verbs) {
		v.DropCase("(case 'n')", 1, "the :noswapfile modifier")
	})
	v.InFunction("ml_open", func(v *graph.Verbs) { v.DropIf(noswap, 1, "ml_open asking for it") })
	v.InFunction("buf_copy_options", func(v *graph.Verbs) { v.FoldNever(noswap, 1, "buf_copy_options asking for it") })
	n := v.Mentions("CMOD_NOSWAPFILE")
	v.Expect(n == 1, "CMOD_NOSWAPFILE outside its enumerator -- %d mentions, expected 1", n)
	v.Expect(len(v.UsesOf("CMOD_NOSWAPFILE")) == 0, "CMOD_NOSWAPFILE still used")
	if err := v.Done(); err != nil {
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
		t.Fatalf("phase 15 on the verbs: %d bytes, q015.c %d", len(got), len(want))
	}
	wantLog := "  filters      the ! operator's row points at nv_error\n" +
		"  noswapfile   the :noswapfile modifier\n  noswapfile   ml_open asking for it\n  noswapfile   buf_copy_options asking for it\n"
	if log.String() != wantLog {
		t.Errorf("reported\n%s", log.String())
	}
}
