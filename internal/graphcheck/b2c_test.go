package graphcheck

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

// PARAM, RETYPE and MOVE (doc/GRAPH-MIGRATION.md, B2c) on the real
// snapshots: whole phases whose rows need them, written on the graph
// outside the plan (no phase is converted here) and held, on q(N-1)'s
// graph read back from its Lisp, to the text program twice -- before the
// sweep, the graph's C view against the text program's output printed
// canonically, byte for byte; and collected, against qN.c.  Each act is
// asserted where it is made (the counts the text program asserted), and
// nothing is left untyped where the edit's types follow plainly.

// diffAtB2c is the first differing line of a and b, with a line around it.
func diffAtB2c(a, b []byte) string {
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			lo := max(0, i-2)
			return fmt.Sprintf("line %d:\ngraph: %q\ntext:  %q", i+1, al[lo:min(len(al), i+2)], bl[lo:min(len(bl), i+2)])
		}
	}
	return fmt.Sprintf("one is a prefix of the other: %d lines against %d", len(al), len(bl))
}

// paramOf is the parameter named name of the definition f.
func paramOf(f *graph.Node, name string) *graph.Node {
	for _, p := range graph.DeclType(f).Kids[1].Kids {
		if p.IsList() && len(p.Kids) >= 2 && p.Kids[0].Atom == name {
			return p
		}
	}
	return nil
}

// Phase 86's parameter made a local: cmdline_handle_ctrl_bsl's c, held to
// the text program's two literals (the phase's other acts are statements).
func TestB2cParamToLocal(t *testing.T) {
	// the text program's two acts were edit.E's Literal, which is
	// edit.ReplaceLiteral on the whole text, counted, and a line reported
	text := func(in []byte) ([]byte, error) {
		x, err := edit.ReplaceLiteral(in, "cmdline_handle_ctrl_bsl(int c, int *gotesc)\n{\n",
			"cmdline_handle_ctrl_bsl(int *gotesc)\n{\n    int c;\n", 1)
		if err != nil {
			return nil, fmt.Errorf("c is a local -- %v", err)
		}
		return edit.ReplaceLiteral(x, "cmdline_handle_ctrl_bsl(c, &gotesc)", "cmdline_handle_ctrl_bsl(&gotesc)", 1)
	}
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 85)
	path := filepath.Join(t.TempDir(), "whim-vim.c")
	out, err := text(in)
	if err != nil {
		t.Fatal(err)
	}
	want, err := cemit.Canonical(path, out)
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
	c := paramOf(e.Defn("cmdline_handle_ctrl_bsl"), "c")
	id := c.ID
	st, err := e.ParamToLocal("cmdline_handle_ctrl_bsl", "c")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	got, _ := h.C()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s", diffAtB2c(got, want))
	}
	if c.ID != id || !e.Live(c) || st.Calls != 1 || len(e.Untyped) != 0 {
		t.Errorf("c #%d (was #%d), %s, %d untyped", c.ID, id, st, len(e.Untyped))
	}
}

// MOVE on whim-vim.c (q103): a definition moved before the function of its
// first use, and one moved above its own prototype -- the edges that then
// resolve to it, the first declaration, retargeted -- each held to the
// text cut and pasted, printed canonically, and the second to the importer:
// the graph imported from the C view refers each use to the same kind of
// declaration.
func TestB2cMoveFunction(t *testing.T) {
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 103)
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
	text := func() []byte {
		b, err := h.C()
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	// cutPaste is the text with the definition of name moved to at.
	cutPaste := func(s []byte, name string, at int) []byte {
		a, z, ok := edit.FindDefinition(s, edit.Blank(s), name)
		if !ok || at > a {
			t.Fatalf("no definition of %s before which to paste", name)
		}
		out := append([]byte(nil), s[:at]...)
		out = append(out, s[a:z]...)
		out = append(out, '\n')
		out = append(out, s[at:a]...)
		out = append(out, s[z:]...)
		c, err := cemit.Canonical(path, out)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// A: ml_get_buf cannot go before the function of its first use (a
	// typedef its body names is defined between); the first definition
	// that can, in the file's order, goes there
	firstUser := func(f *graph.Node) *graph.Node {
		var first *graph.Node
		for _, d := range e.FileDecls(graph.DeclName(f)) {
			for _, u := range e.Uses(d) {
				if user := e.Function(u); user != nil && user != f && (first == nil || indexOf(e, user) < indexOf(e, first)) {
					first = user
				}
			}
		}
		return first
	}
	before := text()
	f := e.Defn("ml_get_buf")
	if err := e.MoveBefore(f, firstUser(f)); err == nil || !strings.Contains(err.Error(), "`DATA_BL`") {
		t.Fatalf("ml_get_buf: %v", err)
	}
	if !bytes.Equal(text(), before) {
		t.Fatal("a refusal changed the text")
	}
	moved := ""
	for _, form := range append([]*graph.Node(nil), e.Graph().Forms...) {
		user := firstUser(form)
		if !form.Is("defn") || user == nil || indexOf(e, user) > indexOf(e, form) {
			continue
		}
		name := graph.DeclName(form)
		before := text()
		ua, _, _ := edit.FindDefinition(before, edit.Blank(before), graph.DeclName(user))
		if err := e.MoveBefore(form, user); err != nil {
			continue
		}
		if err := e.Check(); err != nil {
			t.Fatal(err)
		}
		got := text()
		if want := cutPaste(before, name, ua); !bytes.Equal(got, want) {
			t.Fatalf("A, %s: %s", name, diffAtB2c(got, want))
		}
		syntax(t, got)
		moved = name
		break
	}
	if moved == "" {
		t.Fatal("A: no definition could go before its first user")
	}
	t.Logf("A: %s before the function of its first use", moved)
	// B: the first definition, in the file's order, that can go above its
	// prototype; those that cannot say why
	refused := 0
	for _, form := range append([]*graph.Node(nil), e.Graph().Forms...) {
		if !form.Is("defn") || refused > 200 {
			continue
		}
		name := graph.DeclName(form)
		ds := e.FileDecls(name)
		if len(ds) < 2 || ds[0] == form {
			continue
		}
		before := text()
		proto := clispLine(ds[0])
		at := bytes.Index(before, proto)
		if at < 0 || bytes.Count(before, proto) != 1 {
			continue
		}
		uses := 0
		for _, d := range ds {
			uses += len(e.Uses(d))
		}
		err := e.MoveBefore(form, ds[0])
		if err != nil {
			refused++
			if !strings.Contains(err.Error(), "would resolve to") && !strings.Contains(err.Error(), "before its definition") {
				t.Fatalf("%s: %v", name, err)
			}
			if got := text(); !bytes.Equal(got, before) {
				t.Fatalf("%s: a refusal changed the text", name)
			}
			continue
		}
		if err := e.Check(); err != nil {
			t.Fatal(err)
		}
		got := text()
		if want := cutPaste(before, name, at); !bytes.Equal(got, want) {
			t.Fatalf("B, %s: %s", name, diffAtB2c(got, want))
		}
		syntax(t, got)
		if n := len(e.Uses(form)); n != uses {
			t.Errorf("B, %s: %d uses refer to the definition, of %d", name, n, uses)
		}
		// the importer's edges on the C view
		re, _, err := graph.Import(path, got)
		if err != nil {
			t.Fatal(err)
		}
		re2 := graph.NewEditor(re)
		if d := re2.Defn(name); d == nil || len(re2.Uses(d)) != uses {
			t.Errorf("B, %s: the import refers %d uses to the definition, the move %d", name, len(re2.Uses(d)), uses)
		}
		t.Logf("B: %s above its prototype, %d uses retargeted to it; %d refused before it", name, uses, refused)
		return
	}
	t.Fatalf("no definition could go above its prototype (%d refused)", refused)
}

// syntax requires gcc to accept the text, -fsyntax-only: a move the graph
// let by is a program.
func syntax(t *testing.T, src []byte) {
	t.Helper()
	cmd := exec.Command("gcc", "-fsyntax-only", "-w", "-x", "c", "-")
	cmd.Stdin = bytes.NewReader(src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}
}

// indexOf is the place of a top-level form in the file.
func indexOf(e *graph.Editor, f *graph.Node) int {
	for i, x := range e.Graph().Forms {
		if x == f {
			return i
		}
	}
	return -1
}

// clispLine is a top-level declaration printed alone, its line.
func clispLine(d *graph.Node) []byte {
	b, err := clisp.Print([]*clisp.Node{graph.Lisp(d)})
	if err != nil {
		return nil
	}
	return b
}
