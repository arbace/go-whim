package graphcheck

import (
	"bytes"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/cut"
)

// B2b's capabilities on the real snapshots (doc/GRAPH-MIGRATION.md, *B2b
// as built*): INITROW, RENUM and RENAME doing what three text programs of
// the pipeline do, each on the graph read back from its Lisp, its C view
// held to the text program's result printed canonically, byte for byte.

// lispOf is text's graph, written as Lisp: each fresh editor below reads
// it back, so that one import serves a test's variants.
func lispOf(t *testing.T, text []byte) []byte {
	t.Helper()
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), text)
	if err != nil {
		t.Fatal(err)
	}
	return g.Lisp()
}

// readBack is the graph the Lisp holds, in an editor.
func readBackOne(t *testing.T, lisp []byte) *graph.Editor {
	t.Helper()
	h, err := graph.Read(lisp)
	if err != nil {
		t.Fatal(err)
	}
	return graph.NewEditor(h)
}

// sameC holds the editor's C view to the text program's result, printed
// canonically as the pipeline prints a boundary.
func sameC(t *testing.T, e *graph.Editor, text []byte) {
	t.Helper()
	got, want := views(t, e, text)
	if !bytes.Equal(got, want) {
		t.Fatalf("the graph's C view is not the text program's: %s", firstDiff(got, want))
	}
}

func views(t *testing.T, e *graph.Editor, text []byte) (got, want []byte) {
	t.Helper()
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if d := e.Dangling(); len(d) != 0 {
		t.Fatalf("%d edges dangle, the first from %v", len(d), d[0].Use)
	}
	want, err := cemit.Canonical(filepath.Join(t.TempDir(), "whim-vim.c"), text)
	if err != nil {
		t.Fatal(err)
	}
	if got, err = e.Graph().C(); err != nil {
		t.Fatal(err)
	}
	return got, want
}

func enumeratorsOf(t *testing.T, e *graph.Editor, names ...string) []*graph.Node {
	t.Helper()
	var out []*graph.Node
	for _, n := range names {
		var en *graph.Node
		for _, d := range e.Decls(n) {
			if e.IsEnumerator(d) {
				en = d
			}
		}
		if en == nil {
			t.Fatalf("no enumerator %s", n)
		}
		out = append(out, en)
	}
	return out
}

// argvScan is command_line_scan()'s body as argvfront writes it
// (internal/cut/argvfront.go's argvScanBody), in C-lisp.
const argvScan = `(def argc int (-> parmp argc))
(def argv (ptr (ptr char)) (-> parmp argv))
(def argv_idx int)
(pre-- argc)
(pre++ argv)
(= argv_idx 1)
(while (> argc 0)
  (block
    (if (== (index argv 0 0) '+')
      (block
        (if (>= (-> parmp n_commands) MAX_ARG_CMDS)
          (block (call mainerr ME_EXTRA_CMD nullptr)))
        (= argv_idx (- 1))
        (if (== (index argv 0 1) NUL)
          (block (= (index (-> parmp commands) (post++ (-> parmp n_commands))) (cast (ptr char_u) "$")))
          (block (= (index (-> parmp commands) (post++ (-> parmp n_commands))) (cast (ptr char_u) (addr (paren (index argv 0 1))))))))
      (block (call mainerr ME_UNKNOWN_OPTION (cast (ptr char_u) (index argv 0)))))
    (if (|| (<= argv_idx 0) (== (index argv 0 argv_idx) NUL))
      (block (pre-- argc) (pre++ argv) (= argv_idx 1)))))`

// INITROW with the positions said again: argvfront (phase 1's D1, on the
// seed q000) on the graph -- command_line_scan's new body, the calls and
// the prescan cut, and main_errors[]'s three rows deleted with
// RowIndex{Enumerators: the five ME_* constants}: the three that indexed
// them go with their declarations, ME_EXTRA_CMD is written 1 where it was
// 4, which the text program spelled out as a replacement of both.  The
// control: the same rows deleted with no index said leave ME_EXTRA_CMD at
// 4, and the bytes move.
func TestRowsArgvFront(t *testing.T) {
	dir, _, _, _ := setup(t)
	lisp := lispOf(t, snapOf(t, dir, 0))
	// the cutter itself, on the graph since B4: what the verbs below must give
	prod := readBackOne(t, lisp)
	if err := cut.ArgvFront(prod, io.Discard); err != nil {
		t.Fatal(err)
	}
	want, err := prod.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	run := func(said bool) (*graph.Editor, *graph.RowsDone, string) {
		e := readBackOne(t, lisp)
		var log bytes.Buffer
		v := graph.NewVerbs("argvfront", e, &log)
		v.Body("command_line_scan", argvScan, "command_line_scan is +{command} alone")
		v.InFunction("common_init_2", func(v *graph.Verbs) {
			v.Cut("(call early_arg_scan paramp)", 1, "early_arg_scan's call")
		})
		v.InFunction("main", func(v *graph.Verbs) {
			v.Cut("(call parse_command_name (addr params))", 1, "parse_command_name's call")
			v.Cut("(for (= i 1) (< i argc) (pre++ i) _)", 1, "main's --clean prescan")
		})
		var ix graph.RowIndex
		if said {
			ix.Enumerators = enumeratorsOf(t, e, "ME_UNKNOWN_OPTION", "ME_TOO_MANY_ARGS", "ME_ARG_MISSING", "ME_GARBAGE", "ME_EXTRA_CMD")
		}
		var done *graph.RowsDone
		v.InTable("main_errors", func(v *graph.Verbs) {
			done = v.DeleteRowsEach([]string{`"Too many edit arguments"`, `"Argument missing after"`, `"Garbage after option argument"`},
				ix, "the three errors the command line cannot give")
		})
		v.DeleteDefinition("mainerr_arg_missing", "mainerr_arg_missing, which named one of them")
		v.Cut("(def static mainerr_arg_missing _*)", 1, "its prototype")
		if err := v.Done(); err != nil {
			t.Fatal(err)
		}
		return e, done, log.String()
	}
	e, done, log := run(true)
	if len(done.Deleted) != 3 || len(done.GoneEnumerators) != 3 || len(done.GoneDecls) != 3 ||
		len(done.Renumbered) != 1 || graph.EnumeratorName(done.Renumbered[0].Enumerator) != "ME_EXTRA_CMD" || done.Renumbered[0].New != 1 {
		t.Fatalf("%+v", done)
	}
	sameC(t, e, want)
	if strings.Count(log, "\n") != 7 {
		t.Errorf("reported\n%s", log)
	}
	// the control: the rows without their index
	e, _, _ = run(false)
	for _, n := range []string{"ME_TOO_MANY_ARGS", "ME_ARG_MISSING", "ME_GARBAGE"} {
		if ds := e.Decls(n); len(ds) != 1 {
			t.Errorf("%s: %d declarations", n, len(ds))
		}
	}
	if got, want := views(t, e, want); bytes.Equal(got, want) {
		t.Error("the control: the index unsaid, the bytes did not move")
	}
}

// RENUM: filefront (phase 1's D4) on the graph, on the text extable leaves
// (q000 through argvfront, exfront and extable, as the front runs them):
// thirteen designated cmdnames[] rows deleted -- nothing to say again, a
// name places each -- and their enumerators moved after CMD_SIZE with
// Renumber, every value after the first of them moving, as the text
// program's moved lines move them.
func TestRenumFileFront(t *testing.T) {
	dir, _, _, _ := setup(t)
	t.Setenv("REMOVED", declaredRows(t))
	// argvfront, exfront and extable, on the graph since B4
	pre := readBackOne(t, lispOf(t, snapOf(t, dir, 0)))
	for _, f := range []func(*graph.Editor, io.Writer) error{cut.ArgvFront, cut.ExFront, cut.ExTable} {
		if err := f(pre, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	lisp := pre.Graph().Lisp()
	prod := readBackOne(t, lisp)
	if err := cut.FileFront(prod, io.Discard); err != nil {
		t.Fatal(err)
	}
	want, err := prod.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	cmds := []string{"edit", "enew", "ex", "exit", "file", "read", "saveas", "update", "visual", "view", "write", "wq", "xit"}
	var names, pats []string
	for _, c := range cmds {
		names = append(names, "CMD_"+c)
		pats = append(pats, "(at (idx CMD_"+c+") _)")
	}
	e := readBackOne(t, lisp)
	var log bytes.Buffer
	v := graph.NewVerbs("filefront", e, &log)
	v.InTable("cmdnames", func(v *graph.Verbs) {
		done := v.DeleteRowsEach(pats, graph.RowIndex{}, "the rows of the commands that name a file")
		v.Expect(done != nil && done.Moved == 0, "a designated row moved")
	})
	held := v.MoveEnumerators(names, "CMD_SIZE", graph.HoldValues, "held: refused")
	if v.Err == nil || held != nil || !strings.Contains(v.Err.Error(), "would move") {
		t.Fatalf("held, the move was not refused: %v", v.Err)
	}
	v.Err = nil
	r := v.MoveEnumerators(names, "CMD_SIZE", graph.Renumber, "their enumerators after CMD_SIZE")
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	t.Logf("filefront on the graph: %s", r)
	if len(r.Moved) < 90 || len(r.Deleted) != 0 {
		t.Errorf("%s", r)
	}
	sameC(t, e, want)
}

// RENAME by the edges, on q050: a function (its definition, prototype and
// calls) and mparm_T's member `term`, which phase 51 found no tool could
// tell from the other 124 mentions of `term` (another struct's member,
// locals, parameters).  The text side is the replacement written out and
// asserted where it is: every whole-word mention of the function, and the
// member's declaration and its one use.
func TestRenameOnSnapshot(t *testing.T) {
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 50)
	want := regexp.MustCompile(`\bfind_builtin_term\b`).ReplaceAll(in, []byte("builtin_term_of"))
	for _, r := range [][2]string{
		{"    char_u *term;\n} mparm_T;", "    char_u *term_name;\n} mparm_T;"},
		{"termcapinit(params.term);", "termcapinit(params.term_name);"},
	} {
		if bytes.Count(want, []byte(r[0])) != 1 {
			t.Fatalf("%q is not there once", r[0])
		}
		want = bytes.Replace(want, []byte(r[0]), []byte(r[1]), 1)
	}
	e := readBackOne(t, lispOf(t, in))
	fn, err := e.Rename(e.Defn("find_builtin_term"), "builtin_term_of")
	if err != nil {
		t.Fatal(err)
	}
	var term *graph.Node
	for _, m := range graph.Members(graph.DeclType(e.Decls("mparm_T")[0])) {
		if graph.DeclName(m) == "" && len(m.Kids) > 0 && m.Kids[0].Atom == "term" {
			term = m
		}
	}
	if term == nil {
		t.Fatal("mparm_T has no member term")
	}
	mem, err := e.Rename(term, "term_name")
	if err != nil {
		t.Fatal(err)
	}
	if len(mem.Uses) != 1 || len(fn.Decls)+len(fn.Uses) != bytes.Count(in, []byte("find_builtin_term")) {
		t.Errorf("the function: %d declarations, %d uses; the member: %d uses", len(fn.Decls), len(fn.Uses), len(mem.Uses))
	}
	sameC(t, e, want)
	// the name is taken: refused
	if _, err := e.Rename(e.Defn("builtin_term_of"), "set_termname"); err == nil {
		t.Error("renamed onto set_termname")
	}
}
