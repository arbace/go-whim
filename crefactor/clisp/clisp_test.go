package clisp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// roundTrip converts src to forms and back, and requires cemit's canonical
// text of src -- which is src when src is canonical -- and the forms back
// from that text, byte for byte both ways.
func roundTrip(t *testing.T, name, src string) []byte {
	t.Helper()
	want, err := cemit.Canonical(name, []byte(src))
	if err != nil {
		t.Fatalf("%s: cemit refused: %v", name, err)
	}
	lc, err := ToLisp(name, []byte(src))
	if err != nil {
		t.Fatalf("%s: ToLisp: %v", name, err)
	}
	c, err := ToC(lc)
	if err != nil {
		t.Fatalf("%s: ToC: %v\n%s", name, err, lc)
	}
	if !bytes.Equal(c, want) {
		t.Fatalf("%s: the round trip is not cemit's text\nforms:\n%s\nwant:\n%s\ngot:\n%s", name, lc, want, c)
	}
	again, err := ToLisp(name, c)
	if err != nil {
		t.Fatalf("%s: ToLisp of the canonical text: %v", name, err)
	}
	if !bytes.Equal(again, lc) {
		t.Fatalf("%s: the canonical text's forms differ\nfirst:\n%s\nagain:\n%s", name, lc, again)
	}
	return lc
}

// Each form, from the C that makes it: the forms ToLisp writes, and the
// round trip byte for byte.
func TestForms(t *testing.T) {
	for _, c := range []struct{ name, src, forms string }{
		{"include", "#include <stdio.h>\n#include <stdlib.h>\nint x;\n",
			"(include \"<stdio.h>\")\n(include \"<stdlib.h>\")\n\n(def x int)\n"},
		{"typedef", "typedef unsigned long ul;", "(typedef ul (unsigned long))\n"},
		{"storage", "static const char *name = \"a\";", "(def static name (ptr (const char)) \"a\")\n"},
		{"typedef name first", "typedef int T; T const x;", "(typedef T int)\n\n(def x (spec T const))\n"},
		{"pointer to array of functions",
			"int (*(*x)[10])(int);", "(def x (ptr (array 10 (ptr (fn (int) int)))))\n"},
		{"const pointer", "char *const p;", "(def p (ptr char const))\n"},
		{"pointer to pointer", "int **const *p;", "(def p (ptr (ptr (ptr int) const)))\n"},
		{"redundant declarator parens", "int (x);", "(def x (paren int))\n"},
		{"function pointer parameter", "void sort(int (*cmp)(const void *, const void *), ...);",
			"(def sort (fn ((cmp (ptr (fn (((ptr (const void))) ((ptr (const void)))) int))) ...) void))\n"},
		{"unspecified parameters", "int f();", "(def f (fn () int))\n"},
		{"struct", "struct pt { int x, y; unsigned f : 3; int : 2; struct pt *next; };",
			"(struct pt (x int) (y int) (f unsigned (bits 3)) (int (bits 2)) (next (ptr (struct pt))))\n"},
		{"anonymous union", "struct s { union { int i; char c; }; };",
			"(struct s ((union (i int) (c char))))\n"},
		{"enum", "enum hue { RED, GREEN = 3, BLUE };", "(enum hue (RED) (GREEN 3) (BLUE))\n"},
		{"enum with a tag and a type", "enum hue : long { RED };", "(enum hue (: long) (RED))\n"},
		{"prefix operators kept apart", "int f(int a) { return - -a + - --a; }", ""},
		{"one-line enum", "enum : long { BIG = 1L << 40 };", "(enum (: long) (BIG (<< 1L 40)))\n"},
		{"designated initializer", "struct pt { int x; int y; }; struct pt o = { .y = 2, [0] = 1, .x = { 1, 2 } };", ""},
		{"array of rows", "static int t[][2] = { {1, 2}, {3, 4} };", "(def static t (array (array 2 int)) (init (init 1 2) (init 3 4)))\n"},
		{"compound literal", "struct pt { int x; }; void f(void) { struct pt p = (struct pt){ .x = 1 }; }", ""},
		{"precedence", "int f(int a, int b, int c) { return (a + b) * c - (a * b) + c + a; }",
			"(defn f (fn ((a int) (b int) (c int)) int)\n  (return (+ (- (* (+ a b) c) (paren (* a b))) c a)))\n"},
		{"assignment chain", "void f(int a, int b) { a = b = 1; a += (b, 2); }", ""},
		{"conditional", "int f(int a) { return a ? a > 1 ? 2 : 3 : (a = 4); }", ""},
		{"unary", "int f(int *p, int a) { return -a + !a + ~a + *p + -(-a) + sizeof a + sizeof(int) + sizeof(p[0]) + sizeof (int){1}; }", ""},
		{"increments", "void f(int *p) { ++*p; (*p)++; p++; --p; }", ""},
		{"members", "struct s { struct s *n; int v; }; int f(struct s *p, struct s q) { return p->n->n->v + q.n->v + (&q)->v; }", ""},
		{"calls and indexing", "int g(int, int); int f(int **a) { return g(a[1][2], (g(1, 2), 3)); }", ""},
		{"casts", "typedef int T[4]; void *f(void *p) { return (char *)p + (long)(T *)p + (long)(int (*)[2])p; }", ""},
		{"strings and chars", "const char *s = \"a\\tb\\\"c\" \"d\"; int c = '\\''; int d = '('; int e = L'x';", ""},
		{"if else chain", "int f(int a) { if (a) return 1; else if (a > 1) { return 2; } else { if (a) return 3; } return 0; }", ""},
		{"loops", "void f(int n) { int i; for (;;) break; for (i = 0; i < n; i++) continue; for (int j = 0; j < n; j++) { } while (n) n--; do n++; while (n < 3); }", ""},
		{"switch and labels", "int f(int a) { switch (a) { case 1: case 2 ... 3: return 1; default: break; } goto out; out: return 0; }", ""},
		{"label at the end", "void f(int a) { if (a) goto end; a++; end: }", ""},
		{"static_assert", "static_assert(sizeof(int) == 4, \"int\");", ""},
		{"attributes", "static int f(const char *, ...) __attribute__((format(printf, 1, 2))); int g(int x __attribute__((unused))) { switch (x) { case 1: x++; __attribute__((fallthrough)); default: break; } return 0; }", ""},
		{"generic", "int f(int x) { return _Generic(x, int: 1, char *: 2, default: 0); }", ""},
		{"typeof", "typedef typeof(sizeof(0)) usize; struct s { int v; }; typeof(((struct s *)0)->v) w;", ""},
		{"alignof", "int a = alignof(long) + alignof(int);", ""},
		{"statement expression", "int f(int a) { return ({ int b = a; b + 1; }); }", ""},
		{"function returning a function pointer", "static int (*pick(int k))(int) { return 0; }", ""},
		{"pointer result", "static char *name(int k) { return k ? \"a\" : \"b\"; }", ""},
		{"nullptr and bool", "bool f(void *p) { return p != nullptr && true; }", ""},
		{"struct in a function", "void f(void) { struct q { int a; } v = { 1 }; enum { K = 2 }; v.a = K; }", ""},
		{"multiple declarators", "int a, *b, c[2] = { 1, 2 };", "(def a int)\n\n(def b (ptr int))\n\n(def c (array 2 int) (init 1 2))\n"},
		{"for with two declarators", "void f(void) { for (int i = 0, j = 1; i < j; i++) { } }", ""},
		{"empty statements", "void f(void) { ; L: ; }", ""},
		{"alignas and atomic", "alignas(16) int a; _Alignas(long) char b; _Atomic(int) c; typeof(int) d;", ""},
		{"asm label", "int x __asm__(\"y\");", ""},
		{"declaration without a declarator", "static struct s { int a; };", ""},
		{"include spelled otherwise", "#include<stddef.h>\nint x;", ""},
		{"attribute spaced", "int g(int x __attribute__ ((unused))) { return 0; }", ""},
		{"computed goto", "void f(void) { void *p = &&L; goto *p; L: return; }", ""},
		{"macro recovery", "#include <errno.h>\n#include <limits.h>\nint f(void) { errno = 0; return INT_MAX + EINTR; }", ""},
	} {
		lc := roundTrip(t, c.name+".c", c.src)
		if c.forms != "" && string(lc) != c.forms {
			t.Errorf("%s: forms\n%s\nwant\n%s", c.name, lc, c.forms)
		}
	}
}

// The reader keeps a literal whole, whatever it holds, and a `;` outside one
// starts a comment.
func TestReader(t *testing.T) {
	forms, err := Read([]byte("(a '(' \")\\\" ;\" L\"x y\" '\\'') ; a comment\n(b)"))
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 2 {
		t.Fatalf("%d forms, want 2", len(forms))
	}
	want := []string{"a", "'('", `")\" ;"`, `L"x y"`, `'\''`}
	for i, w := range want {
		if got := forms[0].List[i].Atom; got != w {
			t.Errorf("atom %d: %q, want %q", i, got, w)
		}
	}
	for _, bad := range []string{"(a", ")", "(\"x)"} {
		if _, err := Read([]byte(bad)); err == nil {
			t.Errorf("%q read without an error", bad)
		}
	}
}

// A form ToC does not know is refused, never printed as something else.
func TestRefusals(t *testing.T) {
	for _, src := range []string{
		"(frobnicate x)",
		"(defn f)",
		"(def x (ptr))",
		"(defn f (fn () int) (return (bogus 1)))",
	} {
		if _, err := ToC([]byte(src)); err == nil {
			t.Errorf("%s printed without an error", src)
		}
	}
}

// One character changed in the forms moves the C: the control the corpus
// check is held to.
func TestControl(t *testing.T) {
	src := "int f(int a) { return a + 1; }"
	lc := roundTrip(t, "control.c", src)
	moved := bytes.Replace(lc, []byte("(+ a 1)"), []byte("(+ a 2)"), 1)
	if bytes.Equal(moved, lc) {
		t.Fatal("the control did not apply")
	}
	c, err := ToC(moved)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := cemit.Canonical("control.c", []byte(src))
	if bytes.Equal(c, want) {
		t.Fatal("a changed form printed the same C")
	}
}

// The corpus, where it is: whim-vim.c from this repository's src/, and any
// file CLISP_CORPUS names (a glob), each byte for byte.  Skipped with -short.
func TestCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus is slow")
	}
	files := []string{filepath.Join("..", "..", "src", "whim-vim.c")}
	if g := os.Getenv("CLISP_CORPUS"); g != "" {
		more, err := filepath.Glob(g)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, more...)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Logf("%s: %v (skipped)", f, err)
			continue
		}
		lc, err := ToLisp(f, src)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		c, err := ToC(lc)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !bytes.Equal(c, src) {
			canon, _ := cemit.Canonical(f, src)
			if !bytes.Equal(c, canon) {
				t.Fatalf("%s: the round trip differs", f)
			}
		}
		t.Logf("%s: %d lines, %d lines of forms, byte for byte", f, bytes.Count(src, []byte("\n")), strings.Count(string(lc), "\n"))
	}
}
