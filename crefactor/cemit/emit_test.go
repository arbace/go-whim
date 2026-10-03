package cemit

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// canon is Canonical on a snippet, failing the test on a refusal.
func canon(t *testing.T, src string) string {
	t.Helper()
	out, err := Canonical("snippet.c", []byte(src))
	if err != nil {
		t.Fatalf("Canonical refused:\n%s\n-- %v", src, err)
	}
	return string(out)
}

// Each construct comes out in its one spelling, whatever the spacing it went
// in with: one declarator per declaration, braces always, one statement per
// line, a definition's name at column 0 under its specifiers, a member,
// enumerator or initialiser per line with a trailing comma, a cast and sizeof
// with no space after the parenthesis.  Each is a fixed point as well.
func TestOneSpelling(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"declarators split in a block", "void f(void){int a=1,*b , c[2];}", `    void
f(void)
{
    int a = 1;
    int *b;
    int c[2];
}
`},
		{"struct", "struct pt{int x,y;};", `struct pt
{
    int x;
    int y;
};
`},
		{"typedef of a union", "typedef union  { int i; char c [4]; } cell_t;", `typedef union
{
    int i;
    char c[4];
} cell_t;
`},
		{"enum", "enum hue{RED,GREEN=3 ,BLUE};", `enum hue
{
    RED,
    GREEN = 3,
    BLUE,
};
`},
		{"array initialiser", "static int t[]={1,2 ,3};", `static int t[] =
{
    1,
    2,
    3,
};
`},
		{"designated initialiser", "struct pt{int x;int y;}; struct pt o={.y=2,.x=1};", `struct pt
{
    int x;
    int y;
};

struct pt o =
{
    .y = 2,
    .x = 1,
};
`},
		{"function pointer", "static int (* pick) ( int ,int );", "static int (*pick)(int, int);\n"},
		{"array of function pointers", "void (*hooks[2])(void);", "void (*hooks[2])(void);\n"},
		{"definition", "static const char *name(int k){return k?\"a\":\"b\";}", `    static const char *
name(int k)
{
    return k ? "a" : "b";
}
`},
		{"if/else chain gets braces", "int f(int v){if(v<0)return -1;else if(v==0)return 0;else return 1;}", `    int
f(int v)
{
    if (v < 0)
    {
        return -1;
    }
    else if (v == 0)
    {
        return 0;
    }
    else
    {
        return 1;
    }
}
`},
		{"loops", "void f(int n){int i;for(i=0;i<n;i++)n--;while(n)n--;do n++;while(n<3);for(;;)break;}", `    void
f(int n)
{
    int i;
    for (i = 0; i < n; i++)
    {
        n--;
    }
    while (n)
    {
        n--;
    }
    do
    {
        n++;
    }
    while (n < 3);
    for (;;)
    {
        break;
    }
}
`},
		{"switch", "int f(int n){switch(n){case 0:return 1;case 1:case 2:n++;break;default:n--;}return n;}", `    int
f(int n)
{
    switch (n)
    {
    case 0:
        return 1;
    case 1:
    case 2:
        n++;
        break;
    default:
        n--;
    }
    return n;
}
`},
		{"casts and sizeof", "long f(int v){return (long) v+(unsigned char)  v+sizeof (int)+sizeof v;}", `    long
f(int v)
{
    return (long)v + (unsigned char)v + sizeof(int) + sizeof v;
}
`},
		{"enum with a tag and a type", "enum hue:long{RED};", "enum hue : long { RED };\n"},
		{"prefix operators kept apart", "int f(int a){return - -a+ - --a+ +(+a);}", `    int
f(int a)
{
    return - -a + - --a + +(+a);
}
`},
		{"operators spaced one way", "int f(int a,int b){a+=b<<2;return a&&!b||a%b==0?a:-b;}", `    int
f(int a, int b)
{
    a += b << 2;
    return a && !b || a % b == 0 ? a : -b;
}
`},
		{"goto and a label", "int f(int a){if(a)goto out;a++;out:return a;}", `    int
f(int a)
{
    if (a)
    {
        goto out;
    }
    a++;
out:
    return a;
}
`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := canon(t, c.src)
			if got != c.want {
				t.Errorf("printed\n%s\nwant\n%s", got, c.want)
			}
			if again := canon(t, got); again != got {
				t.Errorf("not a fixed point: a second print gives\n%s", again)
			}
		})
	}
}

// Every comment goes -- line, block, trailing, inside a block, one continued
// by a backslash -- and a comment's delimiters inside a string or character
// literal are content and stay.
func TestCommentsDropped(t *testing.T) {
	src := `/* a header
   comment */
int a; // trailing
int b /* inside */ = 2;
// continued \
int gone;
const char *s = "/* kept */ // kept";
char q = '"';
int
f(void)
{
    /* in a body */
    return a; // after
}
`
	got := canon(t, src)
	for _, bad := range []string{"header", "trailing", "inside", "continued", "gone", "in a body", "after"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q survives the print:\n%s", bad, got)
		}
	}
	for _, keep := range []string{`"/* kept */ // kept"`, `'"'`, "int b = 2;"} {
		if !strings.Contains(got, keep) {
			t.Errorf("%s is lost:\n%s", keep, got)
		}
	}
}

// A directive other than #include is refused, naming its line, because the
// front end would expand it and the tree would print without it.  #include
// passes, with spaces after the `#` or not.
func TestDirectives(t *testing.T) {
	for _, c := range []struct {
		src    string
		refuse bool
	}{
		{"#define N 3\nint a[N];\n", true},
		{"int a;\n  #if 1\nint b;\n#endif\n", true},
		{"#pragma once\nint a;\n", true},
		{"int a;\n#undef X\n", true},
		{"#include <stddef.h>\nsize_t n;\n", false},
		{"#  include <stddef.h>\nsize_t n;\n", false},
	} {
		_, err := Canonical("d.c", []byte(c.src))
		switch {
		case c.refuse && err == nil:
			t.Errorf("accepted\n%s", c.src)
		case c.refuse && !strings.Contains(err.Error(), "d.c:"):
			t.Errorf("the refusal names no line: %v", err)
		case !c.refuse && err != nil:
			t.Errorf("refused\n%s-- %v", c.src, err)
		}
	}
}

// A file-scope declaration of several declarators prints as one declaration
// each, separated as any two file-scope declarations are, so a second print
// -- which reads them as separate external declarations -- moves nothing.  It
// printed them adjacent once, and the fixed point took two passes.  In a
// block the split is a fixed point too (TestOneSpelling).
func TestFixedPointFileScopeDeclarators(t *testing.T) {
	once := canon(t, "int a, *b;\n")
	if twice := canon(t, once); twice != once {
		t.Errorf("a second print moves the text:\n%s\n-- was --\n%s", twice, once)
	}
}

// A text that does not parse is an error, not a partial print.
func TestParseErrorRefused(t *testing.T) {
	if out, err := Canonical("bad.c", []byte("int f(void) { return 1;\n")); err == nil {
		t.Errorf("printed an unbalanced text:\n%s", out)
	}
}

// The includes are written back where they were -- what was declared above
// the first stays above them, the rest follows -- and not expanded: the line
// is the only trace of the header.
func TestIncludesStayWhereTheyWere(t *testing.T) {
	src := "int core;\nint\nf(void)\n{\n    return core;\n}\n#include <stdio.h>\n#include <stdlib.h>\nint host;\n"
	want := `int core;

    int
f(void)
{
    return core;
}

#include <stdio.h>
#include <stdlib.h>

int host;
`
	if got := canon(t, src); got != want {
		t.Errorf("printed\n%s\nwant\n%s", got, want)
	}
}

// The canonical form is a fixed point: printing what was printed gives the
// same bytes, on the whole program (each snippet in TestOneSpelling checks
// its own).
func TestFixedPoint(t *testing.T) {
	src, err := os.ReadFile("testdata/shapes.c")
	if err != nil {
		t.Fatal(err)
	}
	once := canon(t, string(src))
	if twice := canon(t, once); twice != once {
		t.Fatalf("a second print moves the text:\n%s\n-- was --\n%s", twice, once)
	}
	if once == string(src) {
		t.Fatalf("testdata/shapes.c is already canonical, so it proves nothing")
	}
}

// The printed program is the program: gcc compiles both without a word, and
// the two binaries print the same thing.
func TestPrintedCompilesAndRunsTheSame(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	src, err := os.ReadFile("testdata/shapes.c")
	if err != nil {
		t.Fatal(err)
	}
	printed, err := Canonical("shapes.c", src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	run := func(name string, c []byte) string {
		p := filepath.Join(dir, name+".c")
		if err := os.WriteFile(p, c, 0o644); err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(dir, name)
		out, err := exec.Command("gcc", "-std=gnu2x", "-Wall", "-o", bin, p).CombinedOutput()
		if err != nil || len(out) > 0 {
			t.Fatalf("gcc %s: %v\n%s", name, err, out)
		}
		got, err := exec.Command(bin).Output()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(got)
	}
	before, after := run("before", src), run("after", printed)
	if before != after {
		t.Fatalf("the printed program prints\n%s\nwhere the original printed\n%s", after, before)
	}
	if !bytes.HasPrefix([]byte(before), []byte("-1 0 ")) {
		t.Fatalf("the fixture ran, but not as written: %q", before)
	}
}

// The constructs C-lisp once refused, each printed in its one spelling, a
// fixed point, and accepted by gcc -std=c23: an attribute on a struct or
// union, before its tag and after its body, and on a member; a parameter's
// array declarator with `static`, qualifiers or `*`; the qualifiers of a
// pointer in their order, and an attribute among them; an old-style (K&R)
// definition and identifier list; `__auto_type`, C23's `auto`, and the
// `__typeof__` spellings.  Each printed wrong or not at all before: the
// leading struct attribute and the member's were dropped, `[static 3]`
// refused, `*const volatile` reversed, `f(a, b)` printed as `f(,)`,
// `__auto_type y = 3;` as `y = 3;`, and a K&R definition refused.
func TestDeclaratorsAndAttributes(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"struct attribute before the tag", "struct __attribute__((packed)) s { int a; char b; };", `struct __attribute__((packed)) s
{
    int a;
    char b;
};
`},
		{"struct attribute after the body", "struct s { int a; char b; } __attribute__((packed));", `struct s
{
    int a;
    char b;
} __attribute__((packed));
`},
		{"union attributes both sides", "union __attribute__((aligned(8))) { int a; } __attribute__((may_alias)) u;", `union __attribute__((aligned(8)))
{
    int a;
} __attribute__((may_alias)) u;
`},
		{"struct attribute on a tag", "struct __attribute__((packed)) s; struct __attribute__((packed)) s *p;", "struct __attribute__((packed)) s;\n\nstruct __attribute__((packed)) s *p;\n"},
		{"member attributes", "struct s { int a __attribute__((aligned(8))); char b, c __attribute__((aligned(4))); unsigned f : 3 __attribute__((packed)); };", `struct s
{
    int a __attribute__((aligned(8)));
    char b;
    char c __attribute__((aligned(4)));
    unsigned f : 3 __attribute__((packed));
};
`},
		{"array parameters", "void f(int a[static 3], int b[const 4], int c[static const 5], int d[const static 6], int e[restrict], int n, int g[const *], int h[restrict volatile n]);",
			"void f(int a[static 3], int b[const 4], int c[static const 5], int d[const static 6], int e[restrict], int n, int g[const *], int h[restrict volatile n]);\n"},
		{"abstract array parameters", "void g(int[*], int[const 4], int[const static 5], int[static const 6]);",
			"void g(int [*], int [const 4], int [const static 5], int [static const 6]);\n"},
		{"pointer qualifiers in order", "char *const volatile p; char *restrict const *volatile q;",
			"char *const volatile p;\n\nchar *restrict const *volatile q;\n"},
		{"pointer attribute", "char * __attribute__((aligned(8))) p;", "char *__attribute__((aligned(8))) p;\n"},
		{"old-style definition", "int f(a, b, p) int a; register char b, *p; { return a + b; }", `    int
f(a, b, p)
    int a;
    register char b;
    register char *p;
{
    return a + b;
}
`},
		{"__auto_type and auto", "void f(void) { __auto_type x = 1; auto y = 2; const auto z = 3; x = y + z; }", `    void
f(void)
{
    __auto_type x = 1;
    auto y = 2;
    const auto z = 3;
    x = y + z;
}
`},
		{"typeof spellings", "__typeof__(1) a; __typeof(1) b; __typeof__(int *) c; typeof(int) d;",
			"__typeof__(1) a;\n\n__typeof(1) b;\n\n__typeof__(int *) c;\n\ntypeof(int) d;\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := canon(t, c.src)
			if got != c.want {
				t.Errorf("printed\n%s\nwant\n%s", got, c.want)
			}
			if again := canon(t, got); again != got {
				t.Errorf("not a fixed point: a second print gives\n%s", again)
			}
			gccAccepts(t, got)
		})
	}
	// An identifier list outside a definition is not C (gcc refuses it), but
	// the front end parses it, and it is printed as written.
	if got := canon(t, "int f(a, b);"); got != "int f(a, b);\n" {
		t.Errorf("an identifier list printed as %q", got)
	}
}

// gccAccepts requires gcc -std=c23 to take src without an error.
func gccAccepts(t *testing.T, src string) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "a.c")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("gcc", "-std=c23", "-fsyntax-only", p).CombinedOutput(); err != nil {
		t.Errorf("gcc -std=c23 refuses the printed text: %v\n%s\n%s", err, out, src)
	}
}

// A FILE-SCOPE DECLARATION IS CONVERTED ONCE.  File asked a declaration
// whether it printed anything by printing it, then printed it again; a
// member-designator macro (`st_mtime`, which is `st_mtim.tv_sec`) is printed
// by the first selection that reaches it and recorded, so the second print
// dropped it: `sizeof(s.st_mtime)` came out `sizeof(s)`.  And a file-scope
// declaration that is wholly a macro's invocation, the last of its
// declaration, ends where its arguments do -- it ran to the end of the file.
func TestMacrosAtFileScope(t *testing.T) {
	src := "#include \"testdata/macros.h\"\nstruct st s;\nlong t = sizeof(s.st_mtime);\nlong u = s.st_mtime;\nDECLARE(x)\nint y;\n"
	want := "#include \"testdata/macros.h\"\n\nstruct st s;\n\nlong t = sizeof(s.st_mtime);\n\nlong u = s.st_mtime;\n\nDECLARE(x)\n\nint y;\n"
	got := canon(t, src)
	if got != want {
		t.Errorf("printed\n%s\nwant\n%s", got, want)
	}
	if again := canon(t, got); again != got {
		t.Errorf("not a fixed point: a second print gives\n%s", again)
	}
}

// C23'S ATTRIBUTE STATEMENTS ARE PRINTED.  `[[fallthrough]];` reached the
// printer as `;` while the front end discarded the attributes of such a
// statement, and whim's boundaries carried a bare `;` in its 37 places; it
// holds them now, and so does the print -- C23's, GNU's, and a statement's
// attributes before it alike.
func TestAttributeStatements(t *testing.T) {
	src := "int g(int x);\nint f(int x) { switch (x) { case 1: x++; [[fallthrough]]; case 2: x--; __attribute__((fallthrough)); default: [[gnu::musttail]] return g(x); } }\n"
	def := canon(t, src)
	for _, want := range []string{"[[fallthrough]];", "__attribute__((fallthrough));", "[[gnu::musttail]]"} {
		if !strings.Contains(def, want) {
			t.Errorf("the print drops %s:\n%s", want, def)
		}
	}
}
