package edit

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// Blank keeps the length and every newline, keeps a literal's quotes and
// blanks what is between them, escapes included, and blanks a comment whole,
// delimiters and all.
func TestBlank(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`f("a(b");`, `f("   ");`},
		{`c = '{';`, `c = ' ';`},
		{`c = '\'';`, `c = '  ';`},
		{`s = "q\"(";`, `s = "    ";`},
		{"a /* ( */ b", "a         b"},
		{"a // {\nb", "a     \nb"},
		{"a /* x\ny */ b", "a     \n     b"},
		{"s = \"x\\\ny\";", "s = \"  \n \";"},
		{"a // x \\\n still\nb", "a       \n      \nb"},
	} {
		got := Blank([]byte(c.in))
		if string(got) != c.want {
			t.Errorf("Blank(%q) = %q, want %q", c.in, got, c.want)
		}
		if len(got) != len(c.in) {
			t.Errorf("Blank(%q) changed the length", c.in)
		}
	}
}

// Match and RMatch pair an opener with its closer, over blanked text, so a
// bracket inside a literal or a comment does not count; unbalanced text is -1;
// a non-bracket at the index is the caller's bug, and panics.
func TestMatchRMatch(t *testing.T) {
	src := []byte(`f(a, "(", g[')'], { /* } */ h(b) });`)
	b := Blank(src)
	for _, c := range []struct {
		open byte
		nth  int // which occurrence of the opener, in the blanked text
		want string
	}{
		{'(', 0, `(a, "(", g[')'], { /* } */ h(b) })`},
		{'[', 0, `[')']`},
		{'{', 0, `{ /* } */ h(b) }`},
		{'(', 1, `(b)`},
	} {
		i := -1
		for k := 0; k <= c.nth; k++ {
			i += 1 + bytes.IndexByte(b[i+1:], c.open)
		}
		j := Match(b, i)
		if j < 0 {
			t.Fatalf("Match(%c #%d) = -1", c.open, c.nth)
		}
		if got := string(src[i : j+1]); got != c.want {
			t.Errorf("Match(%c #%d) spans %q, want %q", c.open, c.nth, got, c.want)
		}
		if k := RMatch(b, j); k != i {
			t.Errorf("RMatch(%d) = %d, want %d", j, k, i)
		}
	}
	if j := Match([]byte("((a)"), 0); j != -1 {
		t.Errorf("Match on unbalanced text = %d, want -1", j)
	}
	if j := RMatch([]byte("(a))"), 3); j != -1 {
		t.Errorf("RMatch on unbalanced text = %d, want -1", j)
	}
	for name, f := range map[string]func(){
		"Match":  func() { Match([]byte("a)"), 0) },
		"RMatch": func() { RMatch([]byte("a("), 1) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s at a non-bracket does not panic", name)
				}
			}()
			f()
		}()
	}
}

// Depths is the depth BEFORE each byte: an opener at its context's depth, its
// closer one deeper.
func TestDepths(t *testing.T) {
	got := Depths([]byte("a(b[c])d"))
	want := []int{0, 0, 1, 1, 2, 2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Depths = %v, want %v", got, want)
	}
}

const defs = `static int helper(int);

static int
helper(int x)
{
    const char *s = "other(1) { }";
    return x + 1;
}

int
other(int y)
{
    int helper_count = helper(y);
    return helper_count;
}
`

// FindDefinition finds a definition by name at depth 0 -- its specifiers'
// line with it, its closing brace and newline -- and not a prototype, a call,
// a longer name, or the name inside a string.
func TestFindDefinition(t *testing.T) {
	s := []byte(defs)
	b := Blank(s)
	a, z, ok := FindDefinition(s, b, "helper")
	if !ok {
		t.Fatal("helper is not found")
	}
	want := "static int\nhelper(int x)\n{\n    const char *s = \"other(1) { }\";\n    return x + 1;\n}\n"
	if got := defs[a:z]; got != want {
		t.Errorf("helper spans\n%q\nwant\n%q", got, want)
	}
	a, z, ok = FindDefinition(s, b, "other")
	if !ok || !strings.HasPrefix(defs[a:z], "int\nother(int y)\n{") || !strings.HasSuffix(defs[a:z], "}\n") {
		t.Errorf("other spans %q", defs[a:z])
	}
	for _, name := range []string{"helper_count", "missing", "help", "x"} {
		if _, _, ok := FindDefinition(s, b, name); ok {
			t.Errorf("%s is found as a definition", name)
		}
		if HasDefinition(s, name) {
			t.Errorf("HasDefinition(%s)", name)
		}
	}
	if !HasDefinition(s, "helper") || !HasDefinition(s, "other") {
		t.Errorf("HasDefinition misses a definition")
	}
}

// DeleteDefinition takes the definition and leaves the prototype and the
// callers; a name not defined is reported, and the text is left.
func TestDeleteDefinition(t *testing.T) {
	out, ok := DeleteDefinition([]byte(defs), "helper")
	if !ok {
		t.Fatal("helper was not deleted")
	}
	want := strings.Replace(defs, "static int\nhelper(int x)\n{\n    const char *s = \"other(1) { }\";\n    return x + 1;\n}\n", "", 1)
	if string(out) != want {
		t.Errorf("after the deletion\n%s\nwant\n%s", out, want)
	}
	out, ok = DeleteDefinition([]byte(defs), "missing")
	if ok || string(out) != defs {
		t.Errorf("deleting a missing definition: %v, text moved %v", ok, string(out) != defs)
	}
}

// Normalize drops whitespace except between two word characters, where one
// space stands for the run, and leaves a literal's contents as they are.
func TestNormalize(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a  +\n b", "a+b"},
		{"unsigned   int x", "unsigned int x"},
		{"sizeof (\"a  b\")", "sizeof(\"a  b\")"},
		{"c = ' ' ;", "c=' ';"},
		{"  f ( x , y ) ", "f(x,y)"},
	} {
		if got := string(Normalize([]byte(c.in)).Text); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A needle matches modulo whitespace, the text's literals excepted: the
// spans are source offsets, and whitespace at the needle's edge is required
// and is part of the span.  A space INSIDE a literal is content, so the
// needle's layout still finds `"f(a,b)"`'s contents and not `"f(a, b)"`'s,
// and a needle that is itself a literal must say its spaces exactly.
func TestSpansCountIndex(t *testing.T) {
	src := []byte("x = f( a,b );\ny = f(a , b);\nz = \"f(a,b)\";\nw = \"f(a, b)\";\n")
	n := Normalize(src)
	spans := n.Spans([]byte("f(a, b)"))
	var got []string
	for _, s := range spans {
		got = append(got, string(src[s[0]:s[1]]))
	}
	want := []string{"f( a,b )", "f(a , b)", "f(a,b)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Spans = %q, want %q", got, want)
	}
	if k := Count(src, []byte("f(a,b)")); k != 3 {
		t.Errorf("Count = %d, want 3", k)
	}
	for needle, want := range map[string]int{`"f(a, b)"`: 1, `"f(a,b)"`: 1, `"f(a,  b)"`: 0} {
		if k := Count(src, []byte(needle)); k != want {
			t.Errorf("Count(%s) = %d, want %d: a literal's spaces are content", needle, k, want)
		}
	}
	if !ContainsNorm(src, []byte("y=f(a,b)")) || ContainsNorm(src, []byte("q = f")) {
		t.Errorf("ContainsNorm answers wrong")
	}
	if i := IndexNorm(src, []byte("f (a,b)")); i != 4 {
		t.Errorf("IndexNorm = %d, want 4", i)
	}
	if i := IndexNorm(src, []byte("nothing")); i != -1 {
		t.Errorf("IndexNorm of an absent needle = %d", i)
	}
	if i := IndexNormFrom(src, []byte("f(a,b)"), 5); i != 18 {
		t.Errorf("IndexNormFrom = %d, want 18", i)
	}

	// Edge whitespace: " || b" needs a space before it, and takes it.
	e := []byte("if (a || b)\n")
	sp := Normalize(e).Spans([]byte(" || b"))
	if len(sp) != 1 || string(e[sp[0][0]:sp[0][1]]) != " || b" {
		t.Errorf("an edge space is not part of the span: %v", sp)
	}
	if k := Count([]byte("if (a|| b)"), []byte(" || b")); k != 0 {
		t.Errorf("a needle wanting a space matched where there is none")
	}
	// Two words stay two words.
	if k := Count([]byte("int x;"), []byte("intx;")); k != 0 {
		t.Errorf("`intx` matched `int x`")
	}
}

// ReplaceFirst, ReplaceN and ReplaceAll rewrite modulo whitespace, leaving the
// text when the needle is absent; ReplaceFirst keeps the whitespace around the
// tokens it matched.
func TestReplaceNorm(t *testing.T) {
	src := []byte("a = f( 1 );\nb = f(1);\nc = f (1);\n")
	for _, c := range []struct {
		n    int
		want string
	}{
		{1, "a = g;\nb = f(1);\nc = f (1);\n"},
		{2, "a = g;\nb = g;\nc = f (1);\n"},
		{-1, "a = g;\nb = g;\nc = g;\n"},
		{9, "a = g;\nb = g;\nc = g;\n"},
	} {
		if got := string(ReplaceN(src, []byte("f(1)"), []byte("g"), c.n)); got != c.want {
			t.Errorf("ReplaceN(%d) = %q, want %q", c.n, got, c.want)
		}
	}
	if got := string(ReplaceAll(src, []byte("f(1)"), []byte("g"))); got != "a = g;\nb = g;\nc = g;\n" {
		t.Errorf("ReplaceAll = %q", got)
	}
	if got := ReplaceN(src, []byte("h(2)"), []byte("g"), -1); !bytes.Equal(got, src) {
		t.Errorf("ReplaceN of an absent needle moved the text")
	}
	blk := []byte("{\n    x = 1;\n}\n\nnext;\n")
	if got := string(ReplaceFirst(blk, []byte("x = 1;\n}\n\n"), []byte("y;"))); got != "{\n    y;\n\nnext;\n" {
		t.Errorf("ReplaceFirst took the whitespace around its match: %q", got)
	}
}

// An anchor is matched exactly wherever it occurs exactly, and modulo
// whitespace only when it occurs nowhere: so a spaced site is not counted
// while an exact one exists.
func TestAnchorsExactFirst(t *testing.T) {
	text := "p = q + 1;\np = q  +  1;\n"
	if k := CountAnchor(text, "p = q + 1;"); k != 1 {
		t.Errorf("CountAnchor with an exact site = %d, want 1", k)
	}
	if k := CountAnchor(text, "p=q+1;"); k != 2 {
		t.Errorf("CountAnchor with no exact site = %d, want 2", k)
	}
	if got := ReplaceAnchor(text, "p = q + 1;", "r;", -1); got != "r;\np = q  +  1;\n" {
		t.Errorf("ReplaceAnchor exact = %q", got)
	}
	if got := ReplaceAnchor(text, "p=q+1;", "r;", 1); got != "r;\np = q  +  1;\n" {
		t.Errorf("ReplaceAnchor modulo whitespace, once = %q", got)
	}
	if got := ReplaceAnchor(text, "p=q+1;", "r;", -1); got != "r;\nr;\n" {
		t.Errorf("ReplaceAnchor modulo whitespace, all = %q", got)
	}
	if k := CountAnchorB([]byte(text), "absent"); k != 0 {
		t.Errorf("CountAnchorB of an absent anchor = %d", k)
	}
}

// Line is whole lines after their indentation, quoted; Head is one line up to
// its end and not past it.
func TestLineHead(t *testing.T) {
	if got, want := Line("f(x);"), `(?m)^[ \t]*f\(x\);\n`; got != want {
		t.Errorf("Line = %s, want %s", got, want)
	}
	if got, want := Head("if (x)"), `(?m)^[ \t]*if \(x\)$`; got != want {
		t.Errorf("Head = %s, want %s", got, want)
	}
	text := []byte("    if (x)\n    {\n        f(x);\n    }\n    if (x) y();\n")
	for _, c := range []struct {
		pattern string
		n       int
	}{
		{Line("f(x);"), 1},
		{Line("if (x)", "{"), 1},
		{Line("if (x)", "f(x);"), 0},
		{Head("if (x)"), 1},
		{Line("if (x) y();"), 1},
	} {
		if _, err := ReplacePattern(text, c.pattern, "", c.n); err != nil {
			t.Errorf("%s: %v", c.pattern, err)
		}
	}
}

// CollapseWS folds whitespace runs to one space outside literals only.
func TestCollapseWS(t *testing.T) {
	if got := string(CollapseWS([]byte("a  =\n\t\"x  y\" ;"))); got != `a = "x  y" ;` {
		t.Errorf("CollapseWS = %q", got)
	}
}
