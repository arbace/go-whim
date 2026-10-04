package graph

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

// FOLDX held to the text's fall-out closure: each sample's graph, read
// back from its Lisp, through FoldX is testdata/foldx/NAME.c byte for byte
// -- what crefactor/xform's FallOut (every unwritten object and member a
// seed) gave on the sample, printed canonically.  The files were written
// once, before xform was deleted (doc/GRAPH-MIGRATION.md, *Fin as built*):
// at 1c227af, by `cd crefactor && go test ./graph -run X` with a test that,
// for each sample, wrote cemit.Canonical(path, xform.FallOut()(canon, nil,
// io.Discard)) -- this file's foldxSame there, its want written out.  Each
// differs from its sample's canonical text: the closure does something on
// every one.

func foldxSame(t *testing.T, name, src string) FoldXStats {
	t.Helper()
	_, canon, g := importSample(t, src)
	want, err := os.ReadFile(filepath.Join("testdata", "foldx", name+".c"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(want, canon) {
		t.Fatalf("%s: the text closure's result is the sample itself", name)
	}
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	st, err := e.FoldX(FoldX{})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if err := e.Check(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	got, err := h.C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: %s\ngraph:\n%s\ntext:\n%s", name, firstDiff(got, want), got, want)
	}
	return st
}

var foldxSamples = map[string]string{
	"sizes": `
#include <stddef.h>
#include <stdint.h>
#include <limits.h>
typedef unsigned long usz;
typedef struct { char c; long l; int a[3]; short s; } T;
union u { char c[5]; int i; };
static int flag;
int use(long);
int main(void)
{
    use(sizeof(T) * (flag + 1));
    use(offsetof(T, a) + flag);
    use(sizeof(int) + flag);
    use(sizeof(union u) + flag);
    use((usz)flag);
    use(_Alignof(T) + flag);
    if ((usz)flag > SIZE_MAX / sizeof(T *))
    {
        use(1);
    }
    if (flag < INT_MAX && flag > INT_MIN)
    {
        use(2);
    }
    return 0;
}
`,
	"truth": `
static int p_tbs;
int p_ic;
int use(int);
struct s { int ic; int headlen; };
struct s *orgpat;
int main(void)
{
    int noic = use(0);
    int findall = use(1);
    int x;
    p_ic = use(2);
    orgpat->ic = ((p_ic || !noic) && (findall || orgpat->headlen == 0 || !p_tbs));
    x = ((p_ic || !noic) && (findall || !p_tbs));
    x = ((p_ic || !noic) && (!p_tbs));
    x = ((p_ic || !noic) && !p_tbs);
    x = (p_ic && !p_tbs);
    x = (p_ic == 1 && !p_tbs);
    x = (p_ic + (p_tbs + 1)) * 2;
    return x;
}
`,
	"parens": `
static long ur;
static int on;
int use(int);
int x, cnt;
int main(void)
{
    if (!(x & 4) && (ur < 0 || cnt <= ur))
    {
        use(1);
    }
    if (x && (ur))
    {
        use(1);
    }
    if (on && (x || cnt))
    {
        use(2);
    }
    use((on + 2) * x);
    use(x * (on ? x + 1 : cnt));
    use(cnt + (on || x));
    return 0;
}
`,
	"seed-if": `
static int flag;
static int other = 3;
int use(int);
int main(void)
{
    if (flag)
    {
        use(1);
    }
    else
    {
        use(2);
    }
    if (!flag && other)
    {
        use(3);
    }
    return other;
}
`,
	"logic": `
static int flag;
int use(int);
int g;
int main(void)
{
    int x = use(0);
    if (x && !flag)
    {
        use(1);
    }
    if (x || flag)
    {
        use(2);
    }
    if (use(3) && flag)
    {
        use(4);
    }
    g = x && !flag;
    g = (x > 1) && !flag;
    if (x && !flag && g)
    {
        use(5);
    }
    if (x && g && flag)
    {
        use(6);
    }
    return 0;
}
`,
	"returns": `
static int flag;
int use(int);
static int enabled(void)
{
    return flag;
}
static void nothing(int a)
{
    if (flag)
    {
        use(a);
    }
}
int main(void)
{
    if (enabled())
    {
        use(1);
    }
    nothing(2);
    nothing(3);
    return 0;
}
`,
	"params": `
static int flag;
int use(int);
static int pick(int a, int b);
static int pick(int a, int b)
{
    return a + b;
}
int main(void)
{
    use(pick(flag, 1));
    use(pick(flag, 2));
    return 0;
}
`,
	"jumps": `
static int flag;
int use(int);
int main(void)
{
    int r = use(0);
    if (!flag)
    {
        return r;
    }
    use(1);
    use(2);
    return 0;
}
`,
	"locals": `
static int flag;
int use(int);
int main(void)
{
    int on = flag;
    int k = 5;
    if (on)
    {
        use(1);
    }
    while (flag)
    {
        use(2);
    }
    k = flag ? 1 : 2;
    return k + on;
}
`,
	"pointers": `
static char *name;
int use(int);
int main(void)
{
    if (name != (void *)0)
    {
        use(1);
    }
    if (name)
    {
        use(2);
    }
    use(name == 0);
    return 0;
}
`,
	"members": `
struct st { int on; int n; };
static struct st s;
static struct st *sp = &s;
int use(int);
int main(void)
{
    sp->n = 3;
    if (sp->on)
    {
        use(1);
    }
    return s.n;
}
`,
	"unsigned": `
static unsigned u;
static int neg = -1;
int use(int);
int main(void)
{
    if (u - 1 > 0)
    {
        use(1);
    }
    if (neg < 0)
    {
        use(2);
    }
    use(neg * 2 + 1);
    use((unsigned char)300 + u);
    return 0;
}
`,
	"labels": `
static int flag;
int use(int);
int main(void)
{
    int i = use(0);
    switch (i)
    {
    case 1:
        if (flag)
        {
            use(1);
        }
        break;
    case 2:
        while (flag)
        {
            use(2);
        }
        break;
    }
    if (flag)
    {
        use(3);
    }
    else if (i)
    {
        use(4);
    }
    if (i)
    {
        use(5);
    }
    else if (flag)
    {
        use(6);
    }
    return 0;
}
`,
}

func TestFoldXAgainstText(t *testing.T) {
	for name, src := range foldxSamples {
		foldxSame(t, name, src)
	}
}

// The seed taken from what a cut leaves: Before, the text's FallOutOf.
func TestFoldXSeedsAfterCut(t *testing.T) {
	src := `
static int a;
static int b;
int use(int);
static void set(void)
{
    a = 1;
}
int main(void)
{
    if (b)
    {
        set();
    }
    if (a)
    {
        use(1);
    }
    return 0;
}
`
	_, _, g := importSample(t, src)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	before := e.Unwritten(nil)
	if !before.Has("b") || before.Has("a") {
		t.Fatalf("before: %v", before.Names())
	}
	// the cut: set() no longer writes a
	set := e.Defn("set")
	if err := e.Delete(Body(set)[0]); err != nil {
		t.Fatal(err)
	}
	st, err := e.FoldX(FoldX{Before: before})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := h.C()
	if strings.Contains(string(out), "if (a)") || !strings.Contains(string(out), "if (b)") {
		t.Errorf("the seed: %v\n%s", st.Report, out)
	}
	if len(st.Report) == 0 || !strings.HasPrefix(st.Report[0], "1 left unwritten by the cut: a") {
		t.Errorf("report: %q", st.Report)
	}
}

// EmptyBlocks held to xform's EmptyBlocksRule: the empty blocks and the
// locals only given values, on the graph read back, against what the text
// rule gave on this sample (recorded when B3g deleted it, its last user
// phase 62 on the graph): 4 blocks, the locals b and p.
func TestEmptyBlocksAgainstText(t *testing.T) {
	src := `
int use(int);
int g;
int main(void)
{
    int a = use(0);
    int b = 0;
    int c = use(1);
    char *p = 0;
    if (a)
    {
    }
    if (a > 1)
    {
    }
    else
    {
    }
    if (use(2))
    {
    }
    if (a)
    {
        use(3);
    }
    else if (a == 2)
    {
    }
    if (c)
    {
    }
    else if (use(4))
    {
    }
    b = a + 1;
    b = 2;
    p = 0;
    g = c;
    while (a)
    {
    }
    return 0;
}
`
	_, _, g := importSample(t, src)
	want := []byte(emptyBlocksTextWant)
	n, took := 4, []string{"b", "p"}
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	st, err := e.EmptyBlocks(EmptyOptions{Cond: edit.PureCond})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	got, _ := h.C()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s\ngraph:\n%s\ntext:\n%s", firstDiff(got, want), got, want)
	}
	if st.Blocks != n || strings.Join(st.Locals, " ") != strings.Join(took, " ") {
		t.Errorf("graph %d blocks, %q; text %d, %q", st.Blocks, st.Locals, n, took)
	}
}

const emptyBlocksTextWant = `int use(int);

int g;

    int
main(void)
{
    int a = use(0);
    int c = use(1);
    if (use(2))
    {
    }
    if (a)
    {
        use(3);
    }
    if (c)
    {
    }
    else if (use(4))
    {
    }
    g = c;
    while (a)
    {
    }
    return 0;
}
`

const moreSample = `
struct opt { int f; int s; };
static struct opt *cur;
static int flag = 1;
static int level = 3;
static int checkclear(int x);
static int checkclear(int x) { return x > 2; }
static void help(int x) { }
int use(int);
int main(void)
{
    int i = use(0);
    if (!checkclear(i))
    {
        help(i);
    }
    if (i)
    {
        goto out;
    }
    use(level);
    if (flag)
    {
        cur->s = 0;
    }
out:
    return cur->f;
}
`

// The rules beside the closure, each opted into: a label an edit took the
// last goto of, a condition kept for its effect (notags' shape: `if
// (!f()) {X}` with X cut is `(void)f();`), an if a store's removal left with
// nothing to do, the value a cut gives what it deletes.  Without them the
// closure leaves each as the text cutters do.
func TestFallOutMore(t *testing.T) {
	for _, c := range []struct {
		opt  FallOutOptions
		want []string
		not  []string
	}{
		{FallOutOptions{KeepEmpty: true},
			[]string{"if (!checkclear(i))\n    {\n    }", "use(3);", "if (flag)\n    {\n    }", "out:"}, nil},
		{FallOutOptions{KeepEmpty: true, KeepCondition: true, StoreIfs: true, Labels: true, Values: map[string]int64{"level": 7}},
			[]string{"    (void)checkclear(i);\n", "use(7);", "    use(7);\n    return cur->f;"}, []string{"if (flag)", "out:", "help"}},
	} {
		_, _, g := importSample(t, moreSample)
		h, err := Read(g.Lisp())
		if err != nil {
			t.Fatal(err)
		}
		e := NewEditor(h)
		for _, name := range []string{"help", "level"} {
			for _, d := range e.FileDecls(name) {
				if err := e.Delete(d); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := e.Delete(member(h, "s")); err != nil {
			t.Fatal(err)
		}
		gt := find(h, func(n *Node) bool {
			return n.Is("if") && len(blockItems(n.Kids[2])) == 1 && blockItems(n.Kids[2])[0].Is("goto")
		})
		if err := e.Delete(gt); err != nil {
			t.Fatal(err)
		}
		st, err := e.FallOut(c.opt)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Check(); err != nil {
			t.Fatal(err)
		}
		if _, err := Collect(h, testCollect); err != nil {
			t.Fatal(err)
		}
		out, _ := h.C()
		for _, w := range c.want {
			if !bytes.Contains(out, []byte(w)) {
				t.Errorf("%+v: no %q in\n%s", c.opt, w, out)
			}
		}
		for _, w := range c.not {
			if bytes.Contains(out, []byte(w)) {
				t.Errorf("%+v: %q left in\n%s", c.opt, w, out)
			}
		}
		if c.opt.KeepCondition && st.Kept != 1 {
			t.Errorf("kept %d conditions", st.Kept)
		}
	}
}

// A cut's own writes as marks: a predicate made `return 0;` by a cut is 0
// at its calls, and what that decides is folded, with no seed at all.
func TestFoldXMarks(t *testing.T) {
	src := `
int use(int);
int x, y;
static int visible(void)
{
    return x > 0;
}
int main(void)
{
    if (visible())
    {
        use(1);
    }
    if (!visible() && y)
    {
        use(2);
    }
    use(x ? 3 : 4);
    return 0;
}
`
	_, _, g := importSample(t, src)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	ret := Body(e.Defn("visible"))[0]
	zero := Literal(0)
	if err := e.Replace(ret.Kids[1], zero); err != nil {
		t.Fatal(err)
	}
	st, err := e.FoldX(FoldX{Marks: []*Node{zero}, NoSeeds: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if _, err := Collect(h, testCollect); err != nil {
		t.Fatal(err)
	}
	out, _ := h.C()
	for _, w := range []string{"    if (y)\n    {\n        use(2);\n    }\n", "use(x ? 3 : 4);"} {
		if !bytes.Contains(out, []byte(w)) {
			t.Errorf("no %q in\n%s\n%v", w, out, st.Report)
		}
	}
	for _, w := range []string{"visible", "use(1)"} {
		if bytes.Contains(out, []byte(w)) {
			t.Errorf("%q left in\n%s", w, out)
		}
	}
}
