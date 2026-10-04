package graph

// crefactor/xform's StateParam tests (stateparam_test.go), moved with the
// transform (doc/GRAPH-MIGRATION.md, Step6): each source imported and read
// back from its Lisp, its core the forms above its first include, the C
// view held to the text step's result printed canonically (recorded from
// xform.StateParam before it went), the graph to the import of that C
// (SameGraph), nothing left untyped.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

const s6spSrc = `typedef struct { int n; int *p; } pair;
static int depth;
static pair cur = { 0, nullptr };
static int calls;
static int leaf(int x);
static int leaf(int x)
{
    calls++;
    return x + cur.n;
}
static void mid(void)
{
    ++depth;
    cur.n = leaf(depth);
}
int top(int x)
{
    mid();
    return leaf(x) + depth;
}
int other(void)
{
    return top(1);
}
#include <stdio.h>
int main(void) { return other(); }
`

var s6spOpts = StateParamOptions{
	Core:    true,
	Objects: []string{"depth", "cur", "calls"},
	Type:    "st_T", Instance: "st0", Param: "s",
	Roots: []string{"top"},
}

const s6spWant = `typedef struct
{
    int n;
    int *p;
} pair;

typedef struct
{
    int depth;
    pair cur;
    int calls;
} st_T;

static st_T st0;

static int leaf(st_T *s, int x);

    static int
leaf(st_T *s, int x)
{
    s->calls++;
    return x + s->cur.n;
}

    static void
mid(st_T *s)
{
    ++s->depth;
    s->cur.n = leaf(s, s->depth);
}

    int
top(int x)
{
    st_T *s = &st0;
    mid(s);
    return leaf(s, x) + s->depth;
}

    int
other(void)
{
    return top(1);
}

#include <stdio.h>

    int
main(void)
{
    return other();
}
`

// The early declaration of the type (a function that takes it declared
// before the objects), a typedef of a member's type moved up to the
// struct, an array member decayed where the import decays it and not
// under `&`.
const s6spSrc2 = `typedef int unused_T;
static int step(int k);
static long tab[4];
typedef struct { long lo; long hi; } span;
static span cur;
static int used = 0;
static int step(int k)
{
    long *p = tab;
    long (*a)[4] = &tab;
    tab[k] = cur.hi + (long)sizeof(tab) + (*a)[0];
    used = p != nullptr;
    return used;
}
int run(int k)
{
    return step(k) + step(k + 1);
}
#include <stdio.h>
int main(void) { return run(1); }
`

const s6spWant2 = `typedef int unused_T;

typedef struct eng_S eng_T;

static int step(eng_T *en, int k);

typedef struct
{
    long lo;
    long hi;
} span;

struct eng_S
{
    span cur;
    long tab[4];
    int used;
};

static eng_T eng;

    static int
step(eng_T *en, int k)
{
    long *p = en->tab;
    long (*a)[4] = &en->tab;
    en->tab[k] = en->cur.hi + (long)sizeof(en->tab) + (*a)[0];
    en->used = p != nullptr;
    return en->used;
}

    int
run(int k)
{
    eng_T *en = &eng;
    return step(en, k) + step(en, k + 1);
}

#include <stdio.h>

    int
main(void)
{
    return run(1);
}
`

func s6spRun(t *testing.T, src string, o StateParamOptions) (string, StateParamResult, error) {
	t.Helper()
	dir := t.TempDir()
	canon, err := cemit.Canonical(filepath.Join(dir, "p.c"), []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Import(filepath.Join(dir, "p.c"), canon)
	if err != nil {
		t.Fatal(err)
	}
	if g, err = Read(g.Lisp()); err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	r, err := e.StateParam(o)
	if err != nil {
		return "", r, err
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if len(e.Untyped) > 0 {
		t.Errorf("%d left untyped: %s", len(e.Untyped), Lisp(e.Untyped[0]))
	}
	out, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	h, _, err := Import(filepath.Join(dir, "q.c"), out)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(g, h); err != nil {
		t.Errorf("not the import of its C view: %v", err)
	}
	return string(out), r, nil
}

// StateParam makes the objects one struct's members, hands its pointer to
// every function below the root that needs it -- the prototype too -- and
// has the root bind it to the instance; nothing above the root changes.
func TestStateParamGraph(t *testing.T) {
	for _, c := range []struct {
		src, want, line string
		o               StateParamOptions
	}{
		{s6spSrc, s6spWant, "3 objects are st_T's members; 2 functions take it, from 1 roots (top)", s6spOpts},
		{s6spSrc2, s6spWant2, "3 objects are eng_T's members; 1 functions take it, from 1 roots (run)", StateParamOptions{
			Core: true, Objects: []string{"cur", "tab", "used"}, Type: "eng_T", Instance: "eng", Param: "en", Roots: []string{"run"}}},
	} {
		got, r, err := s6spRun(t, c.src, c.o)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("got\n%s\nwant\n%s", got, c.want)
		}
		if l := r.Line(c.o.Type); l != c.line {
			t.Errorf("report %q, want %q", l, c.line)
		}
	}
}

// It refuses what it cannot make a parameter: a function that needs the
// state with its address taken; an object with an initialiser that is not
// zero; a name it adds that is taken; a need no root bounds.
func TestStateParamGraphRefuses(t *testing.T) {
	for _, c := range []struct{ name, src, why string }{
		{"address", strings.Replace(s6spSrc, "#include", "void *fp = (void *)mid;\n#include", 1), "address is taken"},
		{"init", strings.Replace(s6spSrc, "static int calls;", "static int calls = 1;", 1), "not zero"},
		{"taken", strings.Replace(s6spSrc, "int other(void)", "int st0;\nint other(void)", 1), "is taken"},
		{"unbounded", strings.Replace(strings.Replace(s6spSrc, "return top(1);", "return depth;", 1), "return other(); }", "return 0; }", 1), "no root bounds it"},
		{"named by the host", strings.Replace(s6spSrc, "return top(1);", "return depth;", 1), "the host names it"},
		{"host", strings.Replace(s6spSrc, "return other(); }", "return other(); }\nvoid (*hp)(void) = mid;", 1), "address is taken"},
	} {
		_, _, err := s6spRun(t, c.src, s6spOpts)
		if err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: want a refusal naming %q, got %v", c.name, c.why, err)
		}
	}
}
