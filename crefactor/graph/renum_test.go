package graph

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// B2b's capabilities -- RENUM, INITROW, RENAME -- on graphs read back from
// their Lisp (no cc node behind them), each result held to the C written by
// hand, printed canonically, or to the sweep's own output, byte for byte.

// editorOn is src's graph, read back from its Lisp, in an editor.
func editorOn(t *testing.T, src string) *Editor {
	t.Helper()
	_, _, g := importSample(t, src)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return NewEditor(h)
}

// isC holds the editor's C view to want, printed canonically.
func isC(t *testing.T, e *Editor, want string) {
	t.Helper()
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	canon, err := cemit.Canonical(filepath.Join(t.TempDir(), "w.c"), []byte(want))
	if err != nil {
		t.Fatalf("the expected C does not print: %v", err)
	}
	got, err := e.Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, canon) {
		t.Fatalf("%s\ngraph:\n%s\nwant:\n%s", firstDiff(got, canon), got, canon)
	}
	// and written and read back, the same C
	h, err := Read(e.Graph().Lisp())
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := h.C(); !bytes.Equal(again, got) {
		t.Fatal("read back, the C view moved")
	}
}

// enumerator is the file's enumerator name.
func enumerator(t *testing.T, e *Editor, name string) *Node {
	t.Helper()
	for _, d := range e.Decls(name) {
		if e.IsEnumerator(d) {
			return d
		}
	}
	t.Fatalf("no enumerator %s", name)
	return nil
}

func enumerators(t *testing.T, e *Editor, names ...string) []*Node {
	var out []*Node
	for _, n := range names {
		out = append(out, enumerator(t, e, n))
	}
	return out
}

func mustFail(t *testing.T, err error, saying string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), saying) {
		t.Fatalf("err = %v, want a refusal saying %q", err, saying)
	}
}

const abcd = `enum e { A, B, C, D = 10, E };
int f(void) { return A + C + E; }
`

func TestRenumHold(t *testing.T) {
	e := editorOn(t, abcd)
	_, err := e.DeleteEnumerators(enumerators(t, e, "B"), HoldValues)
	mustFail(t, err, "the value of C would move from 2 to 1 (hold)")
	if err := e.Check(); err != nil || len(e.Log) != 0 {
		t.Fatalf("a refusal edited: %v, %d acts", err, len(e.Log))
	}
	// what Delete asks for last-first, as one act; E's use is left to
	// dangle, for the closure or the cut
	r, err := e.DeleteEnumerators(enumerators(t, e, "D", "E"), HoldValues)
	if err != nil {
		t.Fatal(err)
	}
	if d := e.Dangling(); len(r.Deleted) != 2 || len(d) != 1 || d[0].Use.Atom != "E" {
		t.Fatalf("%s; dangling %v", r, d)
	}
}

func TestRenumHoldSet(t *testing.T) {
	e := editorOn(t, `enum e { A, B = 5, C, D };
int f(void) { return A + D; }
`)
	// B and C together: D's predecessor moves, its value too -- refused
	_, err := e.DeleteEnumerators(enumerators(t, e, "B", "C"), HoldValues)
	mustFail(t, err, "D would move from 7 to 1")
	// the run at the end, which nothing after follows: held
	e = editorOn(t, `enum e { A, B = 5, C };
int f(void) { return A; }
`)
	r, err := e.DeleteEnumerators(enumerators(t, e, "B", "C"), HoldValues)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Deleted) != 2 || len(r.Moved) != 0 || len(e.Log) != 1 || len(e.Log[0].Gone) != 2 {
		t.Fatalf("%v; log %+v", r, e.Log)
	}
	isC(t, e, "enum e { A };\nint f(void) { return A; }\n")
}

func TestRenumber(t *testing.T) {
	e := editorOn(t, abcd+"enum g { G = C + 1 };\n")
	r, err := e.DeleteEnumerators(enumerators(t, e, "B"), Renumber)
	if err != nil {
		t.Fatal(err)
	}
	var moved []string
	for _, m := range r.Moved {
		moved = append(moved, EnumeratorName(m.Enumerator)+":"+i64(m.Old)+"->"+i64(m.New))
	}
	if strings.Join(moved, " ") != "C:2->1 G:3->2" || r.String() != "1 deleted, 2 values moved" {
		t.Fatalf("moved %v; %s", moved, r)
	}
	isC(t, e, "enum e { A, C, D = 10, E };\nint f(void) { return A + C + E; }\nenum g { G = C + 1 };\n")
}

func i64(v int64) string { return strconv.FormatInt(v, 10) }

// PinValues is the sweep's rule: the survivor after a deleted run is
// written with its value -- held to crefactor/sweep's Prune, which deletes
// what main does not reach, and to the graph's own collection.
func TestRenumPinIsTheSweeps(t *testing.T) {
	for _, c := range []struct {
		name, src string
		gone      []string
	}{
		{"after a run", "enum { A, B, C, D = 10, E, F };\nint main(void) { return A + D + F; }\n", []string{"B", "C", "E"}},
		{"the value it had already", "enum { A, B = 0, C };\nint main(void) { return A + C; }\n", []string{"B"}},
		{"the first", "enum { A, B, C };\nint main(void) { return C; }\n", []string{"A", "B"}},
		{"hexadecimal", "enum { A = 65535, B, C };\nint main(void) { return A + C; }\n", []string{"B"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			path, _, _ := importSample(t, c.src)
			want := oracle(t, path, []byte(c.src))
			e := editorOn(t, c.src)
			r, err := e.DeleteEnumerators(enumerators(t, e, c.gone...), PinValues)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Moved) != 0 {
				t.Errorf("pinned, yet %v moved", r.Moved)
			}
			got, _ := e.Graph().C()
			if !bytes.Equal(got, want) {
				t.Fatalf("%s\ngraph:\n%s\nsweep:\n%s", firstDiff(got, want), got, want)
			}
			// and the graph's collection of the same text
			_, _, g := importSample(t, c.src)
			if _, err := Collect(g, testCollect); err != nil {
				t.Fatal(err)
			}
			if coll, _ := g.C(); !bytes.Equal(coll, got) {
				t.Fatalf("the collection pins otherwise:\n%s", coll)
			}
		})
	}
	// a value that cannot be computed cannot be pinned (the sweep keeps
	// the run before it)
	e := editorOn(t, "enum { A = sizeof(long), B, C };\nint main(void) { return A + C; }\n")
	_, err := e.DeleteEnumerators(enumerators(t, e, "B"), PinValues)
	mustFail(t, err, "C's value cannot be computed")
}

func TestRenumMoveInsert(t *testing.T) {
	src := `enum cmd { C_a, C_b, C_c, C_SIZE, C_x = 7 };
int f(void) { return C_b + C_SIZE + C_x; }
`
	e := editorOn(t, src)
	b := enumerator(t, e, "C_b")
	r, err := e.MoveEnumerators([]*Node{b}, enumerator(t, e, "C_SIZE"), Renumber)
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != "3 values moved" {
		t.Fatalf("%s: %v", r, r.Moved)
	}
	if !e.Live(b) || len(e.Log) != 1 || len(e.Log[0].Gone) != 0 || len(e.Log[0].Moved) != 5 {
		t.Fatalf("the move: %+v", e.Log)
	}
	isC(t, e, `enum cmd { C_a, C_c, C_SIZE, C_b, C_x = 7 };
int f(void) { return C_b + C_SIZE + C_x; }
`)
	// held, the same move is refused
	e = editorOn(t, src)
	_, err = e.MoveEnumerators([]*Node{enumerator(t, e, "C_b")}, enumerator(t, e, "C_SIZE"), HoldValues)
	mustFail(t, err, "C_c would move from 2 to 1")
	// inserted at the end, held; in the middle, refused; a clash refused
	e = editorOn(t, src)
	val, err := e.BuildValue(enumerator(t, e, "C_x"), "(+ C_x 1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.BuildValue(enumerator(t, e, "C_a"), "(+ C_x 1)"); err == nil {
		t.Error("C_x is visible after C_a")
	}
	r, err = e.InsertEnumerators(enumerator(t, e, "C_x"), []*Node{NewEnumerator("C_y", val), NewEnumerator("C_z", nil)}, HoldValues)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Inserted) != 2 || len(e.Log[0].New) != 4 {
		t.Fatalf("%s %+v", r, e.Log)
	}
	isC(t, e, `enum cmd { C_a, C_b, C_c, C_SIZE, C_x = 7, C_y = C_x + 1, C_z };
int f(void) { return C_b + C_SIZE + C_x; }
`)
	if v := e.EnumValues()[enumerator(t, e, "C_z")]; v != 9 {
		t.Errorf("C_z = %d", v)
	}
	e = editorOn(t, src)
	_, err = e.InsertEnumerators(enumerator(t, e, "C_a"), []*Node{NewEnumerator("C_n", nil)}, HoldValues)
	mustFail(t, err, "C_b would move from 1 to 2")
	_, err = e.InsertEnumerators(enumerator(t, e, "C_a"), []*Node{NewEnumerator("f", nil)}, Renumber)
	mustFail(t, err, "f is declared already")
	_, err = e.ArrangeEnum(e.Parent(enumerator(t, e, "C_a")), nil, Renumber)
	mustFail(t, err, "keeps one enumerator")
}

// The verbs: by name, reported, the first refusal stopping the rest.
func TestRenumVerbs(t *testing.T) {
	e := editorOn(t, abcd)
	var log bytes.Buffer
	v := NewVerbs("renum", e, &log)
	v.DeleteEnumerators([]string{"B"}, Renumber, "B goes, C follows A")
	v.DeleteEnumerators([]string{"Z"}, Renumber, "no such")
	v.DeleteEnumerators([]string{"A"}, Renumber, "never reached")
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "0 enumerators named Z") {
		t.Fatalf("err = %v", err)
	}
	if log.String() != "  renum        B goes, C follows A\n" {
		t.Errorf("reported %q", log.String())
	}
}
