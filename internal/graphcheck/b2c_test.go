package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	p057 "github.com/arbace/go-whim/internal/phase/057"
	p066 "github.com/arbace/go-whim/internal/phase/066"
	p078 "github.com/arbace/go-whim/internal/phase/078"
	p080 "github.com/arbace/go-whim/internal/phase/080"
	p083 "github.com/arbace/go-whim/internal/phase/083"
	"github.com/arbace/go-whim/internal/whim"
)

// PARAM, RETYPE and MOVE (doc/GRAPH-MIGRATION.md, B2c) on the real
// snapshots: whole phases whose rows need them, written on the graph
// outside the plan (no phase is converted here) and held, on q(N-1)'s
// graph read back from its Lisp, to the text program twice -- before the
// sweep, the graph's C view against the text program's output printed
// canonically, byte for byte; and collected, against qN.c.  Each act is
// asserted where it is made (the counts the text program asserted), and
// nothing is left untyped where the edit's types follow plainly.

// b2cPhase holds one phase: text is its text program, onGraph the same
// acts on the graph.
func b2cPhase(t *testing.T, n int, text func([]byte, io.Writer) ([]byte, error), onGraph func(t *testing.T, e *graph.Editor, v *graph.Verbs)) {
	dir, _, _, _ := setup(t)
	in, want := snapOf(t, dir, n-1), snapOf(t, dir, n)
	path := filepath.Join(t.TempDir(), "whim-vim.c")
	out, err := text(in, io.Discard)
	if err != nil {
		t.Fatalf("the text program: %v", err)
	}
	pre, err := cemit.Canonical(path, out)
	if err != nil {
		t.Fatalf("the text program's output does not print: %v", err)
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
	v := graph.NewVerbs(fmt.Sprint(n), e, io.Discard)
	t0 := time.Now()
	onGraph(t, e, v)
	t.Logf("phase %d on the graph: %v, %d acts logged", n, time.Since(t0).Round(time.Millisecond), len(e.Log))
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	got, err := h.C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pre) {
		t.Fatalf("before the sweep: the graph's C view is not the text program's (%d bytes against %d)\n%s", len(got), len(pre), diffAtB2c(got, pre))
	}
	// read back, the same graph
	r, err := graph.Read(h.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Equal(h, r); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if _, err := e.Collect(whim.GraphCollect()); err != nil {
		t.Fatal(err)
	}
	got, err = h.C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("collected: %d bytes, q%03d.c %d\n%s", len(got), n, len(want), diffAtB2c(got, want))
	}
}

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

// drop drops the parameters named of the function fn, as one edit.
func drop(t *testing.T, e *graph.Editor, fn string, names ...string) graph.ParamStats {
	t.Helper()
	var ds []graph.ParamDrop
	d := e.FileDecls(fn)
	if len(d) == 0 {
		t.Fatalf("no %s", fn)
	}
	for _, name := range names {
		i := e.ParamIndex(fn, name)
		if i < 0 {
			t.Fatalf("%s has no parameter %s", fn, name)
		}
		ds = append(ds, graph.ParamDrop{Decl: d[0], I: i})
	}
	st, err := e.DropParams(ds, graph.ParamOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// Phase 66: five functions lose the parameters that carry an eval value,
// each test of them folded first.
func TestB2cPhase66(t *testing.T) {
	b2cPhase(t, 66, p066.Edit, func(t *testing.T, e *graph.Editor, v *graph.Verbs) {
		v.InFunction("vim_regsub_both", func(v *graph.Verbs) {
			v.Rewrite("(|| (paren (&& ?s (== expr nullptr))) ?d)", "(|| ?s ?d)", 1, "a NULL source is refused")
			v.Rewrite("(|| (!= expr nullptr) (paren ?c))", "?c", 1, "only a \\= source is an expression")
		})
		v.InFunction("cursor_pos_info", func(v *graph.Verbs) {
			v.FoldAlways("(== dict nullptr)", 3, "cursor_pos_info() always gives its message")
		})
		v.InFunction("vim_vsnprintf_typval", func(v *graph.Verbs) {
			v.FoldNever("(!= tvs nullptr)", 2, "the formatter always clamps")
			v.FoldNever("(&& (!= tvs nullptr) _)", 1, "and never counts arguments left over")
		})
		if v.Failed() {
			return
		}
		// no number in a format is read from a list: 10 arguments
		var tests []*graph.Node
		for _, fn := range []string{"parse_fmt_types", "vim_vsnprintf_typval"} {
			v.InFunction(fn, func(v *graph.Verbs) {
				for _, n := range v.Find("(!= tvs nullptr)") {
					if p := e.Parent(n); p != nil && p.Is("call") {
						tests = append(tests, n)
					}
				}
			})
		}
		if len(tests) != 10 {
			t.Fatalf("%d tests of tvs as arguments, the text's 10", len(tests))
		}
		for _, n := range tests {
			p := e.Parent(n)
			f, err := e.Build(n, "FALSE", nil)
			if err == nil {
				err = e.Replace(n, f...)
			}
			if err != nil {
				t.Fatal(err)
			}
			e.Rederive(p) // FALSE is an int's enumerator: the call's type again

		}
		drop(t, e, "vim_regsub_both", "expr")
		drop(t, e, "match_add", "pos_list")
		drop(t, e, "cursor_pos_info", "dict")
		// the formatter and the parse it hands tvs to: one edit
		st, err := e.DropParams([]graph.ParamDrop{
			{Decl: e.FileDecls("vim_vsnprintf_typval")[0], I: e.ParamIndex("vim_vsnprintf_typval", "tvs")},
			{Decl: e.FileDecls("parse_fmt_types")[0], I: e.ParamIndex("parse_fmt_types", "tvs")},
		}, graph.ParamOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if st.Args != 2 {
			t.Errorf("tvs: %s", st)
		}
		drop(t, e, "find_ex_command", "lookup", "cctx")
		n := v.Mentions("tvs")
		v.Expect(n == 0, "tvs has %d mentions left", n)
		if len(e.Untyped) != 0 {
			t.Errorf("%d untyped", len(e.Untyped))
		}
	})
}

// Phase 83: the line getters' cookie -- five functions, the pointers of
// their type (three parameters and a member) and the calls through them,
// one family.
func TestB2cPhase83(t *testing.T) {
	b2cPhase(t, 83, p083.Edit, func(t *testing.T, e *graph.Editor, v *graph.Verbs) {
		v.InFunction("do_one_cmd", func(v *graph.Verbs) {
			v.Cut("(= (. ea cookie) cookie)", 1, "do_one_cmd() keeps none")
		})
		if v.Failed() {
			return
		}
		getline := v.One("(ea_getline _)", "exarg_T's line getter")
		if getline == nil {
			return
		}
		fp := func(fn, name string) graph.ParamDrop {
			p := paramOf(e.Defn(fn), name)
			if p == nil {
				t.Fatalf("%s has no %s", fn, name)
			}
			return graph.ParamDrop{Decl: p, I: 1}
		}
		fn := func(name string) graph.ParamDrop {
			return graph.ParamDrop{Decl: e.FileDecls(name)[0], I: e.ParamIndex(name, "cookie")}
		}
		st, err := e.DropParams([]graph.ParamDrop{
			fn("do_cmdline"), fn("getline_equal"), fn("do_one_cmd"), fn("getexline"), fn("getcmdkeycmd"),
			fp("do_cmdline", "fgetline"), fp("getline_equal", "fgetline"), fp("getline_equal", "func"),
			fp("do_one_cmd", "fgetline"), {Decl: getline, I: 1},
		}, graph.ParamOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// 9 cookies and 9 getter types; 6 + 1 + 4 + 1 + 1 calls
		if st.Params != 18 || st.Calls != 13 || st.Args != 13 || len(e.Untyped) != 0 {
			t.Errorf("%s; %d untyped", st, len(e.Untyped))
		}
		n := v.Mentions("cookie")
		v.Expect(n == 1, "cookie is still named %d times, beside exarg_T's member", n-1)
		// the member and the functions it is set to have one type
		if getline.Type == nil || getline.Type.Kids[1].Type != e.Defn("getexline").Type {
			t.Errorf("ea_getline is typed %v, getexline %v", getline.Type, e.Defn("getexline").Type)
		}
	})
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
	text := func(in []byte, w io.Writer) ([]byte, error) {
		x := edit.New("deadstore", in, w)
		x.Literal("cmdline_handle_ctrl_bsl(int c, int *gotesc)\n{\n",
			"cmdline_handle_ctrl_bsl(int *gotesc)\n{\n    int c;\n", 1, "c is a local")
		x.Literal("cmdline_handle_ctrl_bsl(c, &gotesc)", "cmdline_handle_ctrl_bsl(&gotesc)", 1, "its one caller")
		return x.Done()
	}
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 85)
	path := filepath.Join(t.TempDir(), "whim-vim.c")
	out, err := text(in, io.Discard)
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

// Phase 57: p_emoji is an int.
func TestB2cPhase57(t *testing.T) {
	b2cPhase(t, 57, p057.Edit, func(t *testing.T, e *graph.Editor, v *graph.Verbs) {
		ds := e.FileDecls("p_emoji")
		if len(ds) != 1 {
			t.Fatalf("%d declarations of p_emoji", len(ds))
		}
		st, err := e.Retype(ds[0], "int")
		if err != nil {
			t.Fatal(err)
		}
		if st.Decls != 1 || len(e.Untyped) != 0 {
			t.Errorf("p_emoji: %s; %d left untyped", st, len(e.Untyped))
		}
	})
}

// Phase 80: get_register() returns a yankreg_T * and put_register() takes
// one; the two casts go and the two locals between them are retyped.
func TestB2cPhase80(t *testing.T) {
	b2cPhase(t, 80, p080.Edit, func(t *testing.T, e *graph.Editor, v *graph.Verbs) {
		if _, err := e.RetypeResult("get_register", "(ptr yankreg_T)"); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Retype(paramOf(e.Defn("put_register"), "reg"), "(ptr yankreg_T)"); err != nil {
			t.Fatal(err)
		}
		v.InFunction("get_register", func(v *graph.Verbs) {
			v.Rewrite("(return (cast (ptr void) ?r))", "(return ?r)", 1, "which returns the register without a cast")
		})
		v.InFunction("put_register", func(v *graph.Verbs) {
			v.Rewrite("(deref (cast (ptr yankreg_T) ?r))", "(deref ?r)", 1, "which copies it without a cast")
		})
		for _, name := range []string{"reg1", "reg2"} {
			d := v.One("(def "+name+" (ptr void) nullptr)", name)
			if d == nil {
				return
			}
			if _, err := e.Retype(d, "(ptr yankreg_T)"); err != nil {
				t.Fatal(err)
			}
		}
		for _, u := range e.Untyped {
			t.Errorf("untyped: %s", graph.Lisp(u))
		}
	})
}

// Phase 78: call arguments with effects are evaluated in gcc's order -- 11
// expressions moved into new locals' values, MoveTo's, the places they
// leave taken by the locals.
func TestB2cPhase78(t *testing.T) {
	b2cPhase(t, 78, p078.Edit, func(t *testing.T, e *graph.Editor, v *graph.Verbs) {
		// hoist moves x, a node of the statement s, into a new local of type
		// typ declared in a block around s.
		hoist := func(s, x *graph.Node, name, typ string) {
			blk, err := e.Build(s, "(block (def "+name+" "+typ+" 0) ?s)", graph.Bindings{"s": s})
			if err == nil {
				err = e.Replace(s, blk...)
			}
			if err != nil {
				t.Fatal(err)
			}
			def := blk[0].Kids[1]
			fill, err := e.Build(x, name, nil)
			if err != nil {
				t.Fatal(err)
			}
			id := x.ID
			if err := e.MoveTo(x, def.Kids[len(def.Kids)-1], fill[0]); err != nil {
				t.Fatal(err)
			}
			if x.ID != id || !e.Live(x) {
				t.Errorf("#%d moved, now #%d", id, x.ID)
			}
		}
		// the 9 copies of a line with its length
		var saves []*graph.Node
		pat, err := clisp.Pattern("(= ?x (call vim_strnsave ?g ?l))")
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range v.Find("(= ?x (call vim_strnsave ?g ?l))") {
			b, _ := graph.Match(pat, s)
			g, l := b["g"], b["l"]
			if b["x"].IsList() || !g.Is("call") || !l.Is("call") || g.Kids[1].IsList() || l.Kids[1].IsList() ||
				!strings.HasPrefix(g.Kids[1].Atom, "ml_get") || l.Kids[1].Atom != g.Kids[1].Atom+"_len" ||
				!sameForms(g.Kids[2:], l.Kids[2:]) || !isItem(e, s) {
				continue
			}
			saves = append(saves, s)
		}
		if len(saves) != 9 {
			t.Fatalf("%d copies of a line, the text's 9", len(saves))
		}
		for _, s := range saves {
			call := s.Kids[2]
			hoist(s, call.Kids[3], "len", "colnr_T")
		}
		// cursor_pos_info's width before its length
		v.InFunction("cursor_pos_info", func(v *graph.Verbs) {
			s := v.One("(call col_print buf2 _ (call ml_get_curline_len) (call linetabsize_str p))", "the column")
			if s != nil {
				hoist(s, s.Kids[5], "vcol", "int")
			}
		})
		// fileinfo's new-file message before the modified flag
		v.InFunction("fileinfo", func(v *graph.Verbs) {
			q := v.One(`(? (paren (& (-> curbuf b_flags) BF_NEW)) (call new_file_message) "")`, "the message")
			if q != nil {
				hoist(e.Item(q), q, "new_msg", "(ptr char)")
			}
		})
		if len(e.Untyped) != 0 {
			t.Errorf("%d untyped", len(e.Untyped))
		}
	})
}

func isItem(e *graph.Editor, n *graph.Node) bool { return e.Item(n) == n }

// sameForms says two runs of forms spell the same, ids aside.
func sameForms(a, b []*graph.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].IsList() != b[i].IsList() || a[i].Atom != b[i].Atom || !sameForms(a[i].Kids, b[i].Kids) {
			return false
		}
	}
	return true
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
