package xform

import (
	"bytes"
	"strings"
	"testing"
)

// core is a foreign code base's core/host line: the first `#include`.
var core Core = func(t []byte) int { return bytes.Index(t, []byte("\n#include ")) }

// An empty if whose condition only reads goes, as do an empty else and an
// empty else-if ending its chain, and then the local only stored; a loop, a
// condition that calls, and the host stay.
func TestEmptyBlocks(t *testing.T) {
	src := `int
clamp(int a)
{
    int seen;
    seen = a + 1;
    if (a > 3)
    {
    }
    if (a)
    {
        a++;
    }
    else
    {
    }
    if (a < 0)
    {
    }
    else if (a > 9)
    {
    }
    while (a > 100)
    {
    }
    if (tick())
    {
    }
    return a;
}

#include <stdio.h>
void
host(void)
{
    if (1)
    {
    }
}
`
	want := `int
clamp(int a)
{
    if (a)
    {
        a++;
    }
    while (a > 100)
    {
    }
    if (tick())
    {
    }
    return a;
}

#include <stdio.h>
void
host(void)
{
    if (1)
    {
    }
}
`
	got := run(t, EmptyBlocks(core), src)
	same(t, got, want)
	run(t, EmptyBlocks(core), src, "--at-least", "4")
	refuses(t, EmptyBlocks(core), src, "fewer than the 5", "--at-least", "5")
	refuses(t, EmptyBlocks(core), "int x;\n", "core does not end")
}

// Calls to two functions that do nothing go from the core, one keeping its
// --, and the local only passed to them goes with its store; the host's call
// to the core's function is redirected to its own.
func TestDropCalls(t *testing.T) {
	k := DropCallsKnobs{Core: core, Funcs: []string{"forget", "release"}, Redirect: [][2]string{{"forget", "release"}}}
	src := `void
release(void *p)
{
}

void
forget(void *p)
{
    if (p)
    {
        release(p);
    }
}

void
reset(void **v, int k)
{
    void *q;
    q = v[0];
    forget(q);
    forget(v[--k]);
    release((char *)v[1]);
}

#include <stdlib.h>
void
host(void *p)
{
    forget(p);
}
`
	want := `void
release(void *p)
{
}

void
forget(void *p)
{
    if (p)
    {
    }
}

void
reset(void **v, int k)
{
    --k;
}

#include <stdlib.h>
void
host(void *p)
{
    release(p);
}
`
	got := run(t, DropCalls(k), src, "--calls", "4", "--redirected", "1")
	same(t, got, want)
	compiles(t, got)
	refuses(t, DropCalls(k), src, "told 5", "--calls", "5")
	refuses(t, DropCalls(k), strings.Replace(src, "forget(q);", "forget(next());", 1), "cannot keep")
}

// dup3 never returns NULL, being xmalloc's; the tests of its result and of
// xmalloc's fold (the kept branch is left at its depth, for the canonical
// print), the label only a folded branch reached goes, and find(), which no
// root vouches for, keeps its test.
func TestNeverNull(t *testing.T) {
	k := NeverNullKnobs{Core: core, Roots: []string{"xmalloc"}}
	src := `void *xmalloc(unsigned long n);

char *
dup3(void)
{
    char *p;
    p = xmalloc(3);
    return p;
}

int
use(void)
{
    char *s = dup3();
    if (s == nullptr)
    {
        goto fail;
    }
    char *t;
    t = (char *)xmalloc(4);
    if (t != nullptr)
    {
        t[0] = 0;
    }
    char *u;
    u = find();
    if (u == nullptr)
    {
        return -2;
    }
    return 0;
fail:
    return -1;
}

#include <stdlib.h>
void *
xmalloc(unsigned long n)
{
    void *p = malloc(n);
    if (p == nullptr)
    {
        abort();
    }
    return p;
}
`
	want := `void *xmalloc(unsigned long n);

char *
dup3(void)
{
    char *p;
    p = xmalloc(3);
    return p;
}

int
use(void)
{
    char *s = dup3();
    char *t;
    t = (char *)xmalloc(4);
        t[0] = 0;
    char *u;
    u = find();
    if (u == nullptr)
    {
        return -2;
    }
    return 0;
    return -1;
}

#include <stdlib.h>
void *
xmalloc(unsigned long n)
{
    void *p = malloc(n);
    if (p == nullptr)
    {
        abort();
    }
    return p;
}
`
	got := run(t, NeverNull(k), src, "--at-least", "2")
	same(t, got, want)
	refuses(t, NeverNull(k), src, "fewer than the 3", "--at-least", "3")
}

// A foreign code base's own truth constants, YES and NO: even and check
// answer, so they and check's local are bool, and check(4) == YES is
// check(4); count and main stay int.
func TestBoolRet(t *testing.T) {
	k := BoolRetKnobs{Core: core, True: []string{"YES"}, False: []string{"NO"}, Keep: []string{"main"}}
	src := `enum
{
    NO = 0,
    YES = 1
};

int even(int n);

int
even(int n)
{
    return n % 2 == 0;
}

int
check(int n)
{
    int ok = NO;
    if (n >= 0)
    {
        ok = even(n);
    }
    return ok;
}

int
count(int n)
{
    return n + 1;
}

int
main(void)
{
    if (check(4) == YES)
    {
        return count(0) == 1;
    }
    return 1;
}

#include <stdio.h>
`
	want := `enum
{
    NO = 0,
    YES = 1
};

bool even(int n);

bool
even(int n)
{
    return n % 2 == 0;
}

bool
check(int n)
{
    bool ok = NO;
    if (n >= 0)
    {
        ok = even(n);
    }
    return ok;
}

int
count(int n)
{
    return n + 1;
}

int
main(void)
{
    if (check(4))
    {
        return count(0) == 1;
    }
    return 1;
}

#include <stdio.h>
`
	got := run(t, BoolRet(k), src)
	same(t, got, want)
	compiles(t, got)
	// without the truth constants, NO is a code and check stays int
	k.True, k.False = nil, nil
	if got := run(t, BoolRet(k), src); !strings.Contains(got, "int\ncheck(") || !strings.Contains(got, "bool\neven(") {
		t.Errorf("with no truth constants, check is not left int and even not made bool:\n%s", got)
	}
}
