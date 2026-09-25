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
