package ccx

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

// The fixtures, testdata/*.c, are small self-contained C -- no #include, so
// the parse needs no system header -- each written to put at least one
// occurrence in every class of one check and a known few in none.  A finding
// is named here by its function and line:column, which is what a reader of the
// report acts on.

// parse reads testdata/name.c.
func parse(t *testing.T, name string) *cc.AST {
	t.Helper()
	ast, err := Parse(filepath.Join("testdata", name+".c"))
	if err != nil {
		t.Fatal(err)
	}
	return ast
}

// parseText reads C given inline.
func parseText(t *testing.T, src string) *cc.AST {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.c")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ast, err := Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	return ast
}

// left names each leftover by its function and line:column, sorted.
func left(r Result) []string {
	var s []string
	for _, f := range r.Left {
		w := f.Where
		if i := strings.LastIndex(w, ".c:"); i >= 0 {
			w = w[i+3:]
		}
		s = append(s, f.Fn+" "+w)
	}
	sort.Strings(s)
	return s
}

// sameList fails the test when got is not want.
func sameList(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// sameClasses fails the test when the classes' counts are not want's.
func sameClasses(t *testing.T, r Result, want map[string]int) {
	t.Helper()
	for k, n := range want {
		if r.Classes[k] != n {
			t.Errorf("%s: %q holds %d, want %d", r.Title, k, r.Classes[k], n)
		}
	}
	for k, n := range r.Classes {
		if _, ok := want[k]; !ok && n != 0 {
			t.Errorf("%s: unexpected class %q (%d)", r.Title, k, n)
		}
	}
}

// the profile the fixtures' own allocator and functions of bytes are named in
var castProfile = Profile{Allocators: []string{"pool_get"}, Frees: []string{"pool_put"}, ByteFuncs: []string{"copy_bytes"}}

// Every pointer cast is classified by what it converts; a pun between two
// structs and an integer made a pointer are what no class covers.
func TestCastsClassifyEveryPointerCast(t *testing.T) {
	r := Casts(parse(t, "casts"), castProfile)
	sameClasses(t, r, map[string]int{
		"allocation: void * from an allocator, to the type allocated":              1,
		"bytes: an object handed to a function of bytes (memmove, memset, memcmp)": 2,
		"bytes: between char, unsigned char and signed char pointers":              1,
		"from void * to bytes":                     1,
		"no change: to the type it already has":    2, // a struct to itself, a function to its pointer
		"null: the constant 0 (nullptr)":           1,
		"to void *: handed to a function of bytes": 1,
	})
	sameList(t, "left over", left(r), []string{"casts 15:24", "casts 21:16"})
	if w := r.Left[0].What + r.Left[1].What; !strings.Contains(w, "(pointer to struct circle) of pointer to struct rect") || !strings.Contains(w, "(pointer to int) of a long") {
		t.Errorf("the findings say %q", w)
	}
}

// The profile moves exactly what it names: without it, the allocator's
// result and the two arguments of the function of bytes are left over, and
// nothing else changes.
func TestCastsEmptyProfileLeavesWhatTheProfileNames(t *testing.T) {
	ast := parse(t, "casts")
	full, empty := Casts(ast, castProfile), Casts(ast, Profile{})
	sameList(t, "left over, empty profile", left(empty), []string{"casts 14:22", "casts 15:24", "casts 21:16", "casts 22:16", "casts 22:27"})
	for k, n := range empty.Classes {
		if full.Classes[k] != n {
			t.Errorf("%q moved from %d to %d, and the profile does not name it", k, full.Classes[k], n)
		}
	}
}

// A void * is typed by its owner: a function of bytes, an allocator or a
// free.  Each is a leftover when the profile does not name it.
func TestVoidPtrsByOwner(t *testing.T) {
	ast := parse(t, "casts")
	r := VoidPtrs(ast, castProfile)
	sameClasses(t, r, map[string]int{"a function of bytes": 1, "an allocator": 2})
	sameList(t, "left over", left(r), []string{"casts 12:42", "casts 12:6"})
	empty := VoidPtrs(ast, Profile{})
	sameClasses(t, empty, map[string]int{})
	sameList(t, "left over, empty profile", left(empty), []string{" 2:6", " 3:6", " 4:6", "casts 12:42", "casts 12:6"})
}

// Unsequenced operands collide when both call a function with effects, or
// one writes what the other uses; a pure function is one that writes only
// its own locals, and PureCalls names more.
func TestOrder(t *testing.T) {
	ast := parse(t, "order")
	pure := PureFuncs(ast, nil)
	for fn, want := range map[string]bool{"bump": false, "peek": true, "f": true, "order": false, "ext_len": false} {
		if pure[fn] != want {
			t.Errorf("PureFuncs[%s] is %v", fn, pure[fn])
		}
	}
	if !PureFuncs(ast, []string{"ext_len"})["ext_len"] {
		t.Error("a PureCall is not pure")
	}
	for _, tc := range []struct {
		name    string
		p       Profile
		left    []string
		classed int
	}{
		{"no profile", Profile{}, []string{"order 10:13", "order 12:13", "order 13:5", "order 14:5"}, 1},
		// ext_len named pure: its call beside bump() is sequenced enough
		{"ext_len pure", Profile{PureCalls: []string{"ext_len"}}, []string{"order 10:13", "order 13:5", "order 14:5"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Order(ast, tc.p)
			sameList(t, "left over", left(r), tc.left)
			if n := r.Classes["unsequenced operands, at most one of them calling a function with effects"]; n != tc.classed {
				t.Errorf("%d pairs classified, want %d", n, tc.classed)
			}
		})
	}
	r := Order(ast, Profile{})
	for _, f := range r.Left {
		if strings.HasSuffix(f.Where, ":14:5") && !strings.Contains(f.What, "call arguments: one operand writes i") {
			t.Errorf("f(i++, i) is reported as %q", f.What)
		}
	}
}

// A goto to a label in a block it is in is Go's; a goto into a block is not.
func TestGotos(t *testing.T) {
	r := Gotos(parse(t, "gotos"))
	sameClasses(t, r, map[string]int{"a goto to a label in a block it is in": 2})
	sameList(t, "left over", left(r), []string{"jumps 17:9"})
}

// Function pointers compare in Go only with nil.
func TestFuncCompares(t *testing.T) {
	r := FuncCompares(parse(t, "gotos"))
	sameClasses(t, r, map[string]int{"a function pointer tested against null": 2})
	sameList(t, "left over", left(r), []string{"compare 28:9", "compare 30:12"})
}

// the fixture's tagged union: kind == 1 says n holds, anything else s
var unionProfile = Profile{Unions: UnionProfile{
	Rules: []UnionRule{{Union: "u", Member: map[string]*regexp.Regexp{
		"n": regexp.MustCompile(`^(\+v->kind==1|-v->kind!=1)$`),
		"s": regexp.MustCompile(`^(-v->kind==1|\+v->kind!=1)$`),
	}}},
	Guarded: GuardedCalls{
		Funcs: []GuardedCall{{Func: "read_n", Guard: "+mode==1"}},
		Var:   "mode",
		Class: "read_n where mode names it",
	},
	Discriminants: []Discriminant{{Var: "mode", Guard: regexp.MustCompile(`^[+-]mode==1$`)}},
	Saves:         &SavedVar{Var: "mode", Copy: "mode_save", For: "mode"},
}}

// A union read is classified where its discriminant says the member holds:
// in the then of the test, after an early return on its negation, and in a
// function every call to which is under it.  A read where the discriminant
// names the other member is a pun; a guarded call outside its guard, and a
// discriminant a reader can reach a write of, are findings; a function that
// saves and restores the discriminant is a barrier.
func TestUnions(t *testing.T) {
	ast := parse(t, "unions")
	r := Unions(ast, unionProfile)
	sameClasses(t, r, map[string]int{
		"u.n, where its discriminant says it holds": 3, // as_number, only_numbers, early
		"u.s, where its discriminant says it holds": 1, // as_string's then
		"read_n where mode names it":                2,
		"mode: 2 functions read under it, and nothing they can call before such a read writes it (saved and restored by keeps_mode)": 0,
	})
	sameList(t, "left over", left(r), []string{
		"as_string 23:17", // s read where kind == 1: a pun
		"by_mode_badly ",  // calls set_mode, which writes mode, before read_n
		"read_n 59:17",    // called under mode, not under kind
		"unguarded ",      // read_n called where mode is not tested
	})
	for _, f := range r.Left {
		if f.Fn == "by_mode_badly" && f.What != "mode is written by set_mode, which it can reach: set_mode" {
			t.Errorf("by_mode_badly: %q", f.What)
		}
	}
	// With no profile, every access is left over and nothing else is said.
	empty := Unions(ast, Profile{})
	sameClasses(t, empty, map[string]int{})
	sameList(t, "left over, empty profile", left(empty), []string{
		"as_number 15:21", "as_string 22:21", "as_string 23:17", "early 42:17", "only_numbers 28:17", "read_n 59:17",
	})
}

// A state field moves only within its member's class, or by its push.
func TestUnionsStateField(t *testing.T) {
	ast := parseText(t, `struct item { int state; union { int a; long b; } u; };
void push(struct item *it, int state) { it->state = state; }
void step(struct item *it)
{
    switch (it->state) {
    case 1:
        it->state = 2;
        break;
    case 3:
        it->state = 1;
        break;
    }
}
`)
	p := Profile{Unions: UnionProfile{State: &StateField{
		Member: "state", Push: "push", Param: "state",
		Class:  regexp.MustCompile(`^(1|2)$`),
		Pushed: "pushed", Moved: "moved",
	}}}
	r := Unions(ast, p)
	sameClasses(t, r, map[string]int{"pushed": 1, "moved": 1})
	sameList(t, "left over", left(r), []string{"step 10:19"})
}

// the fixture's growarray, vec_T
var gaProfile = Profile{
	Allocators: []string{"pool_get"}, ByteFuncs: []string{"copy_bytes"},
	GrowArray: GrowArray{Type: "vec_T", Tag: "vec", Data: "data", ItemSize: "itemsize", Init: "vec_init", InitSize: 1, Grow: "vec_grow"},
}

// Every growarray object has one element type, through the pointers it is
// handed to; a second type by way of a pointer, or a growarray copied whole,
// is a finding.
func TestGrowArrays(t *testing.T) {
	src, err := os.ReadFile("testdata/garrays.c")
	if err != nil {
		t.Fatal(err)
	}
	objects := "growarray objects, each of one element type (2; and 1 pointers to them)"
	r := GrowArrays(parse(t, "garrays"), gaProfile)
	sameClasses(t, r, map[string]int{
		objects: 2,
		"uses that give an element type: casts, conversions, itemsizes":  4,
		"uses that give none: tests, stores of storage, vec_grow's copy": 4,
	})
	sameList(t, "left over", left(r), nil)

	pun := bytes.Replace(src, []byte("put_int(&nums, 2);"), []byte("put_int(&nums, 2);\n    put_int(&names, 3);"), 1)
	r = GrowArrays(parseText(t, string(pun)), gaProfile)
	if len(r.Left) != 1 || r.Left[0].What != "names is used as int (in put_int) and pointer to bytes (in setup)" {
		t.Errorf("a growarray of two types: %q", r.Left)
	}

	copied := bytes.Replace(src, []byte("put_int(&nums, 2);"), []byte("put_int(&nums, 2);\n    names = nums;"), 1)
	r = GrowArrays(parseText(t, string(copied)), gaProfile)
	if len(r.Left) < 1 || !strings.HasPrefix(r.Left[0].What, "a growarray copied: names=nums") {
		t.Errorf("a growarray copied: %q", r.Left)
	}

	// And the profile is what makes the storage typed: the casts of it are
	// the growarray's class with one, left over without.
	if n := Casts(parse(t, "garrays"), gaProfile).Classes["growarray: data, to the element type"]; n != 2 {
		t.Errorf("%d casts of the storage classified, want 2", n)
	}
	if n := len(Casts(parse(t, "garrays"), Profile{}).Left); n != 2 {
		t.Errorf("%d casts of the storage left over with no profile, want 2", n)
	}
}

// Print lists the classes in order and says whether nothing was left over.
func TestResultPrint(t *testing.T) {
	var b bytes.Buffer
	ok := Result{Title: "t", Classes: map[string]int{"b": 2, "a": 1}}.Print(&b)
	if !ok || b.String() != "t\n       1  a\n       2  b\n       0  LEFT OVER\n" {
		t.Errorf("printed %q, %v", b.String(), ok)
	}
	b.Reset()
	if (Result{Title: "t", Left: []Finding{{"f", "x.c:1:2", "what"}}}).Print(&b) {
		t.Error("a result with a leftover prints as clean")
	}
	if !strings.Contains(b.String(), "x.c:1:2: f  -- what") {
		t.Errorf("printed %q", b.String())
	}
}
