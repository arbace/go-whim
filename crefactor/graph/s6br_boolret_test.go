package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// crefactor/xform's BoolRet tests, on the graph: each source imported, read
// back from its Lisp, BoolRet run, its C view held to what the text step
// gave (the same strings, the same program's output), and the graph held
// to the import of its C view (SameGraph): every expression typed as cc
// types it, nothing left untyped.

func brRunOn(t *testing.T, opt BoolRetOptions, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "t.c")
	g, _, err := Import(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	if _, err := e.BoolRet(opt); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	out, err := h.C()
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Untyped) > 0 {
		t.Errorf("%d left untyped: %s", len(e.Untyped), Lisp(e.Untyped[0]))
	}
	i, _, err := Import(path, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(h, i); err != nil {
		t.Errorf("not the import of its C view: %v", err)
	}
	return string(out)
}

// brGccRun is what a C program prints, when there is a gcc.
func brGccRun(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	dir := t.TempDir()
	c := filepath.Join(dir, "p.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "p")
	if o, err := exec.Command("gcc", "-w", "-o", bin, c).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s\n%s", err, o, src)
	}
	o, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(o)
}

// A foreign code base's own truth constants, YES and NO: even and check
// answer, so they and check's local are bool, and check(4) == YES is
// check(4); count and main stay int.
func TestBoolRetGraph(t *testing.T) {
	k := BoolRetOptions{True: []string{"YES"}, False: []string{"NO"}, Keep: []string{"main"}}
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
	got := brRunOn(t, k, src)
	gtSame(t, got, want)
	gtCompiles(t, got)
	// without the truth constants, NO is a code and check stays int
	k.True, k.False = nil, nil
	if got := brRunOn(t, k, src); !strings.Contains(got, "int\ncheck(") || !strings.Contains(got, "bool\neven(") {
		t.Errorf("with no truth constants, check is not left int and even not made bool:\n%s", got)
	}
}

// With Globals, a file-scope int that only ever holds an answer is bool; a
// counter, one whose address is taken, one compared with a code, one sized,
// and one a local shadows stay int; and the program prints what it did.
func TestBoolRetGraphGlobals(t *testing.T) {
	k := BoolRetOptions{True: []string{"YES"}, False: []string{"NO"}, Keep: []string{"main"}, Globals: true}
	src := `int printf(const char *, ...);
enum
{
    NO = 0,
    YES = 1
};
static int active = NO;
static int seen;
static int ready = 0;
static int count = NO;
static int *where;
static int held = NO;
static int mode = NO;
static int sized = NO;
static int shadow = NO;
static int tabled = NO;
static int *table[] = {&tabled};

static void
turn(int n)
{
    active = n > 2;
    seen = YES;
    ready = active && seen;
    count++;
    held = YES;
    where = &held;
    mode = YES;
    sized = NO;
    tabled = YES;
}

static int
look(void)
{
    int shadow = 5;
    return shadow;
}

int
main(void)
{
    turn(3);
    if (active == NO || mode == 2)
    {
        return 1;
    }
    shadow = YES;
    printf("%d %d %d %d %d %d %d %zu %d\n", active, seen, ready, count, *where, mode, look(), sizeof sized, shadow != NO);
    return 0;
}

#include <stdio.h>
`
	want := brGccRun(t, src)
	got := brRunOn(t, k, src)
	for _, w := range []string{"static bool active", "static bool seen", "static bool ready", "if (!(active) ||",
		"static int count", "static int held", "static int mode", "static int sized", "static int shadow", "static int tabled"} {
		if !strings.Contains(got, w) {
			t.Errorf("no %q in\n%s", w, got)
		}
	}
	if o := brGccRun(t, got); o != want {
		t.Errorf("the program prints %q, the original %q\n%s", o, want, got)
	}
	// without Globals, nothing file-scope moves
	k.Globals = false
	if got := brRunOn(t, k, src); strings.Contains(got, "static bool") {
		t.Errorf("without Globals a file-scope object was retyped:\n%s", got)
	}
}

// With Relax: a flag saved in a local and restored, given a literal 0, or
// |= and &= of an answer is bool, the compound written x = E || x; a flag
// given |= of a number, compared with a code, or counted stays int; and the
// program prints what it did.
func TestBoolRetGraphRelax(t *testing.T) {
	k := BoolRetOptions{True: []string{"YES"}, False: []string{"NO"}, Keep: []string{"main"}, Globals: true, Relax: true}
	src := `int printf(const char *, ...);
enum
{
    NO = 0,
    YES = 1
};
static int scroll = NO;
static int broke = NO;
static int seen = NO;
static int mixed = NO;
static int code = NO;
static int counted = NO;
static int calls;

static int
hit(int n)
{
    calls++;
    return n > 1;
}

static void
work(int n)
{
    int save = scroll;
    scroll = YES;
    broke = 0;
    seen |= hit(n);
    seen &= n < 9;
    mixed |= 4;
    code = n > 2;
    counted = YES;
    counted++;
    scroll = save;
}

int
main(void)
{
    work(3);
    work(1);
    if (code == 2)
    {
        return 1;
    }
    printf("%d %d %d %d %d %d %d\n", scroll, broke, seen, mixed, code, counted, calls);
    return 0;
}

#include <stdio.h>
`
	want := brGccRun(t, src)
	got := brRunOn(t, k, src)
	for _, w := range []string{"static bool scroll", "static bool broke", "static bool seen", "seen = (hit(n)) || seen",
		"seen = (n < 9) && seen", "static int mixed", "static int code", "static int counted"} {
		if !strings.Contains(got, w) {
			t.Errorf("no %q in\n%s", w, got)
		}
	}
	if o := brGccRun(t, got); o != want {
		t.Errorf("the program prints %q, the original %q\n%s", o, want, got)
	}
	// without Relax, phase 102's rule: the saved flag and the others stay int
	k.Relax = false
	if got := brRunOn(t, k, src); !strings.Contains(got, "static int scroll") || !strings.Contains(got, "static int seen") {
		t.Errorf("without Relax a flag the old rule keeps int was retyped:\n%s", got)
	}
}
