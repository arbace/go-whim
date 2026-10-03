package graph

// crefactor/xform's DropCalls and NeverNull tests (core_test.go), moved with
// the transforms they test (doc/GRAPH-MIGRATION.md, B3g): the source
// imported and read back from its Lisp, the core the forms above its first
// include, the C view held to the text step's result printed canonically.
// The counts the text steps took as arguments are the phases' now.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// b3gCore runs edit on src's graph, read back, its core the forms above
// the first include, and returns the C view.
func b3gCore(t *testing.T, src string, edit func(e *Editor, in func(*Node) bool) error) string {
	t.Helper()
	g, _, err := Import(filepath.Join(t.TempDir(), "p.c"), []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if g, err = Read(g.Lisp()); err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	core := map[*Node]bool{}
	for _, f := range e.Core() {
		core[f] = true
	}
	if err := edit(e, func(f *Node) bool { return core[f] }); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	out, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// b3gCoreErr is b3gCore's refusal.
func b3gCoreErr(t *testing.T, src string, edit func(e *Editor, in func(*Node) bool) error) error {
	t.Helper()
	dir := t.TempDir()
	g, _, err := Import(filepath.Join(dir, "p.c"), []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	core := map[*Node]bool{}
	for _, f := range e.Core() {
		core[f] = true
	}
	_ = os.Remove(dir)
	return edit(e, func(f *Node) bool { return core[f] })
}

// Calls to two functions that do nothing go from the core, one keeping its
// --, and the local only passed to them goes with its store; the host's call
// to the core's function is redirected to its own.
func TestDropCalls(t *testing.T) {
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
	o := DropCallsOptions{Funcs: []string{"forget", "release"}, Redirect: [][2]string{{"forget", "release"}}}
	var r DropCallsReport
	got := b3gCore(t, src, func(e *Editor, in func(*Node) bool) (err error) {
		o.In = in
		r, err = e.DropCalls(o)
		return err
	})
	gtSame(t, got, want)
	gtCompiles(t, got)
	if r.Calls != 4 || r.Steps != 1 || r.Redirected != 1 || strings.Join(r.Locals, " ") != "q" {
		t.Errorf("report: %+v", r)
	}
	err := b3gCoreErr(t, strings.Replace(src, "forget(q);", "forget(next());", 1), func(e *Editor, in func(*Node) bool) error {
		o.In = in
		_, err := e.DropCalls(o)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "cannot keep") {
		t.Errorf("want a refusal saying it cannot keep the call, got %v", err)
	}
}

// dup3 never returns NULL, being xmalloc's; the tests of its result and of
// xmalloc's fold (the kept branch is left at its depth, for the canonical
// print), the label only a folded branch reached goes, and find(), which no
// root vouches for, keeps its test.
func TestNeverNull(t *testing.T) {
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
	var r NeverNullReport
	got := b3gCore(t, src, func(e *Editor, in func(*Node) bool) (err error) {
		r, err = e.NeverNull(NeverNullOptions{In: in, Roots: []string{"xmalloc"}})
		return err
	})
	gtSame(t, got, want)
	if r.Folded != 2 || r.Never != 2 || len(r.Left) != 0 {
		t.Errorf("report: %+v", r)
	}
}
