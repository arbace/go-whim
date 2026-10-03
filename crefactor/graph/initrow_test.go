package graph

import (
	"bytes"
	"strings"
	"testing"
)

// tableOf is the file's initialised definition name.
func tableOf(t *testing.T, e *Editor, name string) *Node {
	t.Helper()
	for _, d := range e.FileDecls(name) {
		if TableInit(d) != nil {
			return d
		}
	}
	t.Fatalf("no table %s", name)
	return nil
}

// rowsAt are the table's rows at the indexes.
func rowsAt(t *testing.T, e *Editor, name string, at ...int) []*Node {
	rows := TableInit(tableOf(t, e, name)).Args()
	var out []*Node
	for _, i := range at {
		out = append(out, rows[i])
	}
	return out
}

const subscripts = `static int t[] = {10, 20, 30, 40};
static int three[] = {1, 2, 3};
static int *p = &t[2];
static const unsigned short perm[] = {3, 0, 2};
int f(int i) { return t[3] + t[0] + t[i] + *p + three[0] + perm[0]; }
`

func TestRowsSubscripts(t *testing.T) {
	e := editorOn(t, subscripts)
	tt := tableOf(t, e, "t")
	ix := RowIndex{Arrays: []*Node{tableOf(t, e, "perm")}}
	done, err := e.DeleteRows(tt, rowsAt(t, e, "t", 1), ix)
	if err != nil {
		t.Fatal(err)
	}
	if len(done.Deleted) != 1 || done.Moved != 2 || len(done.Sites) != 4 {
		t.Fatalf("%+v", done)
	}
	// the array of three ints the graph holds is the table's type now
	if tt.Type == nil || tt.Type != tableOf(t, e, "three").Type || len(e.Untyped) != 0 {
		t.Errorf("the table's type %v; untyped %d", tt.Type, len(e.Untyped))
	}
	isC(t, e, `static int t[] = {10, 30, 40};
static int three[] = {1, 2, 3};
static int *p = &t[1];
static const unsigned short perm[] = {2, 0, 1};
int f(int i) { return t[2] + t[0] + t[i] + *p + three[0] + perm[0]; }
`)
	// a count the graph has no type for: the typed edge cleared, listed
	e = editorOn(t, subscripts)
	tt = tableOf(t, e, "t")
	ix = RowIndex{Arrays: []*Node{tableOf(t, e, "perm")}}
	if _, err := e.DeleteRows(tt, rowsAt(t, e, "t", 1, 2), ix); err == nil {
		t.Fatal("perm names the row at 2, which goes")
	}
	rs, err := e.BuildRows(tt, "50", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.InsertRows(tt, nil, rs, ix); err != nil {
		t.Fatal(err)
	}
	if tt.Type != nil || len(e.Untyped) != 1 || e.Untyped[0] != tt {
		t.Errorf("the table's type %v; untyped %v", tt.Type, e.Untyped)
	}
	isC(t, e, strings.Replace(subscripts, "{10, 20, 30, 40}", "{10, 20, 30, 40, 50}", 1))
}

func TestRowsRefused(t *testing.T) {
	e := editorOn(t, subscripts)
	_, err := e.DeleteRows(tableOf(t, e, "t"), rowsAt(t, e, "t", 0), RowIndex{})
	mustFail(t, err, "t[0] names the row at 0, which goes")
	e = editorOn(t, subscripts)
	_, err = e.DeleteRows(tableOf(t, e, "t"), rowsAt(t, e, "t", 3), RowIndex{})
	mustFail(t, err, "t[3] names the row at 3, which goes")
	e = editorOn(t, "static int s[4] = {1, 2, 3, 4};\nint f(void) { return s[0]; }\n")
	_, err = e.DeleteRows(tableOf(t, e, "s"), rowsAt(t, e, "s", 3), RowIndex{})
	mustFail(t, err, "its size is written")
	e = editorOn(t, "struct pt { int x; int y; };\nstatic struct pt o = {1, 2};\nint f(void) { return o.x; }\n")
	_, err = e.DeleteRows(tableOf(t, e, "o"), rowsAt(t, e, "o", 1), RowIndex{})
	mustFail(t, err, "not an array")
	if err := e.Check(); err != nil || len(e.Log) != 0 {
		t.Fatalf("a refusal edited: %v %+v", err, e.Log)
	}
}

// The table and its index are one thing: argvfront's main_errors[] and
// its ME_* constants, each an enum of its own, an index of a row deleted
// going with it, the ones after it written with their new position.
func TestRowsIndexEnumerators(t *testing.T) {
	src := `enum { ME_UNKNOWN = 0 };
enum { ME_TOO_MANY = 1 };
enum { ME_MISSING = 2 };
enum { ME_EXTRA = 4 };
static char *(errors[]) = { "unknown", "too many", "missing", "garbage", "extra" };
void err(int n);
int f(int n) { err(ME_UNKNOWN); err(ME_EXTRA); return *errors[n]; }
`
	e := editorOn(t, src)
	ix := RowIndex{Enumerators: enumerators(t, e, "ME_UNKNOWN", "ME_TOO_MANY", "ME_MISSING", "ME_EXTRA")}
	done, err := e.DeleteRows(tableOf(t, e, "errors"), rowsAt(t, e, "errors", 1, 2, 3), ix)
	if err != nil {
		t.Fatal(err)
	}
	if len(done.GoneEnumerators) != 2 || len(done.GoneDecls) != 2 || len(done.Renumbered) != 1 || done.Renumbered[0].New != 1 {
		t.Fatalf("%+v", done)
	}
	if len(e.Dangling()) != 0 {
		t.Fatalf("dangling %v", e.Dangling())
	}
	isC(t, e, `enum { ME_UNKNOWN = 0 };
enum { ME_EXTRA = 1 };
static char *(errors[]) = { "unknown", "extra" };
void err(int n);
int f(int n) { err(ME_UNKNOWN); err(ME_EXTRA); return *errors[n]; }
`)
	// implicit indexes: the enum's own renumbering says them, or it is refused
	src = `enum ix { I_A, I_B, I_C };
static char *names[] = { "a", "b", "c" };
int f(void) { return *names[I_C] + I_A; }
`
	e = editorOn(t, src)
	_, err = e.DeleteRows(tableOf(t, e, "names"), rowsAt(t, e, "names", 1), RowIndex{Enumerators: enumerators(t, e, "I_A", "I_B", "I_C")})
	if err != nil {
		t.Fatal(err)
	}
	isC(t, e, `enum ix { I_A, I_C };
static char *names[] = { "a", "c" };
int f(void) { return *names[I_C] + I_A; }
`)
	e = editorOn(t, src)
	_, err = e.DeleteRows(tableOf(t, e, "names"), rowsAt(t, e, "names", 0), RowIndex{Enumerators: enumerators(t, e, "I_B", "I_C")})
	mustFail(t, err, "the index I_B would be 1, not 0: its value is implicit")
	// an index of a deleted row in an enum it does not empty, beside one that is not an index
	e = editorOn(t, "enum ix { I_A, I_B, I_C, I_N };\nstatic char *names[] = { \"a\", \"b\", \"c\" };\nint f(void) { return I_N; }\n")
	_, err = e.DeleteRows(tableOf(t, e, "names"), rowsAt(t, e, "names", 1), RowIndex{Enumerators: enumerators(t, e, "I_A", "I_B", "I_C")})
	mustFail(t, err, "I_N would move from 3 to 2, and it is not an index of the table")
}

// Rows designated by a name keep their place when another goes: nothing to
// say again.
func TestRowsDesignated(t *testing.T) {
	src := `enum cmd { C_a, C_b, C_c, C_SIZE };
struct cmdname { char *name; int len; };
static struct cmdname cmdnames[] = { [C_a] = {"a", 1}, [C_b] = {"b", 1}, [C_c] = {"c", 1} };
int f(void) { return cmdnames[C_c].len + cmdnames[2].len; }
`
	e := editorOn(t, src)
	var log bytes.Buffer
	v := NewVerbs("rows", e, &log)
	v.InTable("cmdnames", func(v *Verbs) {
		done := v.DeleteRows(`(at (idx C_b) _)`, 1, RowIndex{}, ":b's row")
		if done != nil && done.Moved != 0 {
			t.Errorf("%+v", done)
		}
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	isC(t, e, strings.Replace(src, `[C_b] = {"b", 1}, `, "", 1))
	if log.String() != "  rows         :b's row\n" {
		t.Errorf("reported %q", log.String())
	}
	// and the enumerator after it, renumbered, moves the row with it
	if _, err := e.DeleteEnumerators(enumerators(t, e, "C_b"), Renumber); err != nil {
		t.Fatal(err)
	}
	if v := e.EnumValues()[enumerator(t, e, "C_c")]; v != 1 {
		t.Errorf("C_c = %d", v)
	}
}

func TestRowsInsert(t *testing.T) {
	src := `struct key { int code; char *name; };
static struct key keys[] = { {1, "one"}, {3, "three"}, {0, nullptr} };
static struct key *last = &keys[1];
int f(void) { return keys[2].code + last->code; }
`
	e := editorOn(t, src)
	var log bytes.Buffer
	v := NewVerbs("keys", e, &log)
	v.InTable("keys", func(v *Verbs) {
		v.InsertRows(`(init 3 _)`, `(init 2 "two") (init (at .name "four") (at .code 4))`, RowIndex{}, "two keys more")
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	isC(t, e, `struct key { int code; char *name; };
static struct key keys[] = { {1, "one"}, {2, "two"}, {.name = "four", .code = 4}, {3, "three"}, {0, nullptr} };
static struct key *last = &keys[3];
int f(void) { return keys[4].code + last->code; }
`)
	// the designators refer to the members
	rows := TableInit(tableOf(t, e, "keys")).Args()
	if d := rows[2].Kids[1].Kids[1].Ref(); d == nil || d.Kids[0].Atom != "name" {
		t.Errorf("the designator refers to %v", d)
	}
	// at the end, and a member no row's type has
	e = editorOn(t, src)
	rs, err := e.BuildRows(tableOf(t, e, "keys"), `(at .nom "x")`, nil)
	if err == nil {
		t.Fatalf("built %v", rs)
	}
	rs, err = e.BuildRows(tableOf(t, e, "keys"), `(init 9 "nine")`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.InsertRows(tableOf(t, e, "keys"), nil, rs, RowIndex{}); err != nil {
		t.Fatal(err)
	}
	isC(t, e, strings.Replace(src, `{0, nullptr} }`, `{0, nullptr}, {9, "nine"} }`, 1))
}
