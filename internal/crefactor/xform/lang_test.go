package xform

import (
	"os/exec"
	"strings"
	"testing"
)

// A small list library's NULL and size_t are the language's nullptr and
// usize; the NULL in its message stays, and so does the one in a literal it
// was not told of when told none.
func TestNullptrUsize(t *testing.T) {
	src := `#include <stddef.h>
#include <stdio.h>

struct node
{
    struct node *next;
    size_t len;
};

size_t
length(struct node *n)
{
    size_t k = 0;
    while (n != NULL)
    {
        k += (size_t)n->len;
        n = n->next;
    }
    return k;
}

void *
first(struct node *n)
{
    return n ? n->next : (void *)NULL;
}

void
show(struct node *n)
{
    printf("%s\n", n == NULL ? "NULL list" : "list");
}
`
	want := `#include <stddef.h>
#include <stdio.h>

typedef typeof(sizeof(0)) usize;

struct node
{
    struct node *next;
    usize len;
};

usize
length(struct node *n)
{
    usize k = 0;
    while (n != nullptr)
    {
        k += (usize)n->len;
        n = n->next;
    }
    return k;
}

void *
first(struct node *n)
{
    return n ? n->next : nullptr;
}

void
show(struct node *n)
{
    printf("%s\n", n == nullptr ? "NULL list" : "list");
}
`
	got := run(t, NullptrUsize(NullptrKnobs{}), src)
	same(t, got, want)
	compiles(t, got)
	run(t, NullptrUsize(NullptrKnobs{NullLiterals: []string{`"NULL list"`}}), src, "--casts", "1")
	refuses(t, NullptrUsize(NullptrKnobs{NullLiterals: []string{}}), src, "not the 0")
	refuses(t, NullptrUsize(NullptrKnobs{}), src, "at least 2", "--casts", "2")
	refuses(t, NullptrUsize(NullptrKnobs{}), strings.Replace(src, "k += (size_t)n->len;", "k += 3 * size_t;", 1), "neither a cast")
}

// Unused on a definition's parameters goes, a GNU fallthrough is C23's, a
// format attribute stays; an attribute of another kind refuses.
func TestAttrs(t *testing.T) {
	src := `#include <stdarg.h>
#include <stdio.h>

int logf_(const char *fmt, ...) __attribute__((format(printf, 1, 2)));

int
logf_(const char *fmt, ...)
{
    va_list ap;
    va_start(ap, fmt);
    int n = vprintf(fmt, ap);
    va_end(ap);
    return n;
}

int
step(int state, int ctx __attribute__((unused)))
{
    switch (state)
    {
    case 0:
        state++;
        __attribute__((fallthrough));
    case 1:
        return state;
    }
    return -1;
}
`
	want := strings.Replace(strings.Replace(src, " __attribute__((unused))", "", 1),
		"__attribute__((fallthrough));", "[[fallthrough]];", 1)
	got := run(t, Attrs(), src)
	same(t, got, want)
	compiles(t, got)
	refuses(t, Attrs(), strings.Replace(src, "((format(printf, 1, 2)))", "((noreturn))", 1), "never looked at")
}

// A tagged value's one-member union is that member; the two-member one stays.
func TestUnions(t *testing.T) {
	src := `struct value
{
    int tag;
    union
    {
        long i;
        double d;
    } num;
    union
    {
        char *s;
    } str;
};

long
get(struct value *v)
{
    if (v->tag)
    {
        return v->num.i;
    }
    return v->str.s[0];
}

#include <stdio.h>
`
	want := `struct value
{
    int tag;
    union
    {
        long i;
        double d;
    } num;
    char *str;
};

long
get(struct value *v)
{
    if (v->tag)
    {
        return v->num.i;
    }
    return v->str[0];
}

#include <stdio.h>
`
	got := run(t, Unions(), src, "--degenerate", "1", "--genuine", "1")
	same(t, got, want)
	compiles(t, got)
	refuses(t, Unions(), src, "at least 1 and 2", "--degenerate", "1", "--genuine", "2")
	refuses(t, Unions(), strings.Replace(src, "return v->str.s[0];", "return sizeof v->str;", 1), "neither its own declaration")
}

// The compiler says which headers a file needs: math.h and string.h go,
// stdio.h stays.
func TestIncludes(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	s := Silent{Cmd: "gcc", Flags: []string{"-fsyntax-only", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter"}}
	src := `#include <math.h>
#include <stdio.h>
#include <string.h>

int
main(void)
{
    puts("hi");
    return 0;
}
`
	want := `#include <stdio.h>

int
main(void)
{
    puts("hi");
    return 0;
}
`
	same(t, run(t, Includes(s), src), want)
	refuses(t, Includes(s), "#define X 1\n"+src, "a directive other than #include")
	refuses(t, Includes(s), strings.Replace(src, "return 0;", "int unused;\n    return 0;", 1), "does not compile silently")
}
