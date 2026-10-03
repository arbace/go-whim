package clisp

import (
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

func forms(t *testing.T, src string) []*Node {
	t.Helper()
	fs, err := Forms("t.c", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func printed(t *testing.T, fs []*Node) string {
	t.Helper()
	b, err := Print(fs)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func canonical(t *testing.T, src string) string {
	t.Helper()
	b, err := cemit.Canonical("t.c", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// An edit on the tree prints as the same edit made to the C, canonically.
func TestTreeEdits(t *testing.T) {
	src := `int a, b;
static void f(int x)
{
    a = x;
    b = x;
    if (x > 2)
    {
        a = 0;
    }
    else
    {
        b = 1;
        a = 1;
    }
    x++;
}
`
	root := Root(forms(t, src))
	// delete every statement assigning b, through cursors collected first
	for _, c := range FindPattern(root, MustPattern("(= b _)")) {
		if !c.IsItem() {
			t.Fatalf("%s is not an item", c.Node())
		}
		c.Delete()
	}
	ifs := Heads(root, "if")
	if len(ifs) != 1 || ifs[0].Function() != "f" {
		t.Fatalf("ifs %d", len(ifs))
	}
	ifs[0].FoldNever()
	// insert before and after, during a walk, and replace
	Walk(root, func(c *Cursor) bool {
		if c.IsItem() && Matches(MustPattern("(post++ x)"), c.Node()) {
			c.InsertBefore(MustPattern("(= a 2)"))
			c.InsertAfter(MustPattern("(return)"))
			c.Replace(MustPattern("(pre++ x)"))
		}
		return true
	}, nil)
	got := printed(t, root.List)
	want := canonical(t, `int a, b;
static void f(int x)
{
    a = x;
    a = 1;
    a = 2;
    ++x;
    return;
}
`)
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// A walk goes on after the node its callback deleted or replaced, and visits
// neither what it deleted nor what it put in its place.
func TestWalkAfterEdits(t *testing.T) {
	root := MustPattern("(block a b c d e)")
	var seen []string
	Walk(root, func(c *Cursor) bool {
		if c.Node().list {
			return true
		}
		seen = append(seen, c.Node().Atom)
		switch c.Node().Atom {
		case "b":
			c.Delete()
		case "c":
			c.Replace(A("x"), A("y"))
		case "d":
			if s := c.SiblingCursor(-1); s == nil || s.Node().Atom != "y" {
				t.Fatalf("d's sibling before is %v", s)
			}
			c.SiblingCursor(1).Delete()
		}
		return true
	}, nil)
	if strings.Join(seen, " ") != "block a b c d" {
		t.Fatalf("seen %v", seen)
	}
	if root.String() != "(block a x y d)" {
		t.Fatalf("got %s", root)
	}
}

func TestPatterns(t *testing.T) {
	n := MustPattern("(= (-> buf b_x) (call f 1 2))")
	for _, c := range []struct {
		p  string
		ok bool
	}{
		{"(= (-> buf ?m) _)", true},
		{"(= (-> curbuf ?m) _)", false},
		{"(= _*)", true},
		{"(= (-> _*) (call f _*))", true},
		{"(= (-> buf b_x))", false},
		{"(= ?a ?a)", false},
	} {
		if got := Matches(MustPattern(c.p), n); got != c.ok {
			t.Errorf("%s: %v", c.p, got)
		}
	}
	b, _ := Match(MustPattern("(= (-> buf ?m) ?v)"), n)
	if b["m"].Atom != "b_x" || b["v"].String() != "(call f 1 2)" {
		t.Fatalf("bindings %v", b)
	}
	if s := Subst(MustPattern("(= (-> curbuf ?m) ?v)"), b).String(); s != "(= (-> curbuf b_x) (call f 1 2))" {
		t.Fatalf("subst %s", s)
	}
	if !Matches(MustPattern("(f ?a ?a)"), MustPattern("(f (g 1) (g 1))")) {
		t.Fatal("a repeated binding of equal trees")
	}
}

// The resolver: scopes, shadowing, the four name spaces.
func TestResolve(t *testing.T) {
	src := `typedef int T;
struct s { int n; struct s *next; };
struct u { int n; };
enum { RED, GREEN = RED + 2 };
static int x;
static int g(int);
static int g(int x)
{
    T y = x;
    {
        int x = y;
        y = x;
    }
    for (int x = 0; x < 3; x++)
    {
        y += x;
    }
    if (y)
    {
        goto out;
    }
    struct s v = {.next = 0};
    y = v.next->n + GREEN;
out:
    return g(x) + y;
}
`
	fs := forms(t, src)
	s := Resolve(fs)
	root := Root(fs)
	// every use of x, with what it resolves to
	var xs []string
	for _, r := range s.Refs {
		if r.Atom.Atom != "x" {
			continue
		}
		d := r.Decl
		if d == nil {
			t.Fatalf("x unresolved")
		}
		xs = append(xs, d.Kind)
	}
	// T y = x (the parameter); y = x (the inner block's local); x < 3, x++
	// and y += x (the for's); g(x) (the parameter again) -- a declarator's
	// own name is not a use
	want := "param local local local local param"
	if strings.Join(xs, " ") != want {
		t.Fatalf("x resolves to %v, want %s", xs, want)
	}
	if d := s.File["x"]; d == nil || d.Kind != "object" || len(s.Uses(d)) != 0 {
		t.Fatalf("file x: %+v uses %d", d, len(s.Uses(d)))
	}
	if d := s.File["g"]; d == nil || d.Kind != "function" || len(d.Nodes) != 2 || len(s.Uses(d)) != 1 {
		t.Fatalf("g: %+v", d)
	}
	if d := s.File["T"]; d == nil || d.Kind != "typedef" || len(s.Uses(d)) != 1 {
		t.Fatalf("T: %+v", d)
	}
	if d := s.File["RED"]; d == nil || d.Kind != "enumerator" || len(s.Uses(d)) != 1 {
		t.Fatalf("RED: %+v", d)
	}
	if d := s.Tags["s"]; d == nil || d.Kind != "struct" || len(s.Uses(d)) != 2 {
		t.Fatalf("tag s: %+v uses %d", d, len(s.Uses(d)))
	}
	// members: next is one member's, n two structs'
	if ms := s.Members["next"]; len(ms) != 1 || len(s.Uses(ms[0])) != 2 {
		t.Fatalf("next: %d", len(ms))
	}
	amb := 0
	for _, r := range s.Refs {
		if r.Space == Member && r.Atom.Atom == "n" && r.Ambiguous {
			amb++
		}
	}
	if amb != 1 {
		t.Fatalf("n's uses ambiguous: %d", amb)
	}
	// the label, used before it is defined
	gotos := Heads(root, "goto")
	if d := s.DeclOf(gotos[0].Node().List[1]); d == nil || d.Kind != "label" {
		t.Fatalf("goto out: %+v", d)
	}
}

// Mentions counts as the C's `\bname\b` does: atoms, designators, and words
// in the quoted ones.
func TestMentions(t *testing.T) {
	fs := forms(t, `struct s { int abc; };
struct s v = {.abc = 1};
char *m = "abc abcd";
int f(void) { return v.abc; }
`)
	if n := Mentions(Root(fs), "abc", true); n != 4 {
		t.Fatalf("with literals %d", n)
	}
	if n := Mentions(Root(fs), "abc", false); n != 3 {
		t.Fatalf("without %d", n)
	}
}
