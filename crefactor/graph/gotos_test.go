package graph

// crefactor/xform's goto tests (goto_test.go, gototail_test.go,
// gotoblock_test.go, flow_test.go), moved with the transforms they test
// (doc/GRAPH-MIGRATION.md, B3g): each source imported, read back from its
// Lisp, rewritten on the graph, and its C view held to the text step's
// result printed canonically; the programs compiled and run, the rewritten
// one required to print what the original prints.  The floors the text
// steps took (`--at-least`) are internal/steps' now.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// gtStep is a goto transform on the graph as a function of C text: the
// text imported and read back, rewritten, its C view, the report on w.
func gtStep(tail int, name string) func([]byte, io.Writer) ([]byte, error) {
	return func(src []byte, w io.Writer) ([]byte, error) {
		dir, err := os.MkdirTemp("", "gotos")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(dir)
		g, _, err := Import(filepath.Join(dir, "p.c"), src)
		if err != nil {
			return nil, err
		}
		if g, err = Read(g.Lisp()); err != nil {
			return nil, err
		}
		e := NewEditor(g)
		var lines []string
		switch name {
		case "tail":
			r, err := e.GotoTail(tail)
			if err != nil {
				return nil, err
			}
			lines = r.Lines()
		case "break":
			r, err := e.GotoBreak()
			if err != nil {
				return nil, err
			}
			lines = []string{r.Line()}
		case "loop":
			r, err := e.GotoLoop()
			if err != nil {
				return nil, err
			}
			lines = []string{r.Line()}
		case "block":
			r, err := e.GotoBlock()
			if err != nil {
				return nil, err
			}
			lines = []string{r.Line()}
		}
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
		if err := e.Check(); err != nil {
			return nil, err
		}
		return g.C()
	}
}

func gtRun(t *testing.T, name, src string) string {
	t.Helper()
	out, err := gtStep(3, name)([]byte(src), io.Discard)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return string(out)
}

func gtRunTail(t *testing.T, tail int, src string) string {
	t.Helper()
	out, err := gtStep(tail, "tail")([]byte(src), io.Discard)
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	return string(out)
}

// gtSame fails the test when got is not want, each printed canonically:
// the graph's C view is canonical whatever the text it was imported from.
func gtSame(t *testing.T, got, want string) {
	t.Helper()
	got, want = gtCanon(t, got), gtCanon(t, want)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// gtCompiles asks gcc whether the output is C, when there is a gcc.
func gtCompiles(t *testing.T, out string) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "o.c")
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("gcc", "-fsyntax-only", "-std=gnu2x", p).CombinedOutput(); err != nil {
		t.Errorf("gcc refuses the output: %s\n%s", b, out)
	}
}

// TestGotoReturnsVoid: what a void function's end is a return for.
func TestGotoReturnsVoid(t *testing.T) {
	out, err := gtStep(0, "none")([]byte("void a(void) {}\nstatic void b(int x) {}\nvoid *c(void) { return 0; }\nint d(void) { return 0; }\nvoid (*e(void))(int) { return 0; }\n"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Import(filepath.Join(t.TempDir(), "p.c"), out)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"a": true, "b": true, "c": false, "d": false, "e": false}
	for _, f := range g.Forms {
		if !f.Is("defn") {
			continue
		}
		name := declName(f)
		if got := gtReturnsVoid(f); got != want[name] {
			t.Errorf("%s: returns void = %v, want %v", name, got, want[name])
		}
		delete(want, name)
	}
	if len(want) > 0 {
		t.Errorf("not seen: %v", want)
	}
}

// The fixtures are a small scanner's code, not any one code base's.  Each
// step's output is compared printed canonically, as the pipeline prints every
// boundary, and the programs are compiled and run: the rewritten one must
// compile silently under -Wall -Wextra and print what the original prints.

const gtBreakSrc = `int printf(const char *, ...);

int find(const int *v, int n, int want)
{
    int i;
    int at = -1;
    for (i = 0; i < n; i++)
    {
        if (v[i] == want)
        {
            at = i;
            goto found;
        }
    }
found:
    return at * 10 + i;
}

int drain(const int *q, int n)
{
    int k = 0;
    if (n > 0)
    {
        while (k < n)
        {
            if (q[k] < 0)
                goto out;
            k++;
        }
    }
out:
    k += 100;
    return k;
}

int kind(int c)
{
    int r = 0;
    switch (c)
    {
    case 1:
        r = 10;
        goto done;
    case 2:
        r = 20;
        break;
    }
    r++;
done:
    return r;
}

int clip(int a)
{
    if (a > 9)
    {
        a = 9;
        goto tail;
    }
    else if (a < 0)
        goto tail;
    a *= 2;
tail:
    return a;
}

int grid(int m[3][3])
{
    int r = 0;
    for (int i = 0; i < 3; i++)
    {
        for (int j = 0; j < 3; j++)
        {
            if (m[i][j] == 0)
                goto hit;
            r += m[i][j];
        }
        if (r > 20)
            goto hit;
    }
hit:
    return r;
}

int main(void)
{
    int v[] = {4, 7, -1, 9};
    int m[3][3] = {{1, 2, 3}, {4, 0, 6}, {7, 8, 9}};
    int p[3][3] = {{9, 9, 9}, {9, 9, 9}, {1, 1, 1}};
    int z[3][3] = {{1, 1, 1}, {1, 1, 1}, {1, 1, 1}};
    for (int w = -2; w < 11; w++)
        printf("%d %d %d %d %d\n", find(v, 4, w), drain(v, w % 5), kind(w), clip(w), w);
    printf("%d %d %d\n", grid(m), grid(p), grid(z));
    return 0;
}
`

const gtBreakWant = `int printf(const char *, ...);
int find(const int *v, int n, int want)
{
    int i;
    int at = -1;
    for (i = 0; i < n; i++)
    {
        if (v[i] == want)
        {
            at = i;
            break;
        }
    }
    return at * 10 + i;
}
int drain(const int *q, int n)
{
    int k = 0;
    if (n > 0)
    {
        while (k < n)
        {
            if (q[k] < 0)
            {
                break;
            }
            k++;
        }
    }
    k += 100;
    return k;
}
int kind(int c)
{
    int r = 0;
    switch (c)
    {
    case 1:
        r = 10;
        goto done;
    case 2:
        r = 20;
        break;
    }
    r++;
done:
    return r;
}
int clip(int a)
{
    if (a > 9)
    {
        a = 9;
        goto tail;
    }
    else if (a < 0)
    {
        goto tail;
    }
    a *= 2;
tail:
    return a;
}
int grid(int m[3][3])
{
    int r = 0;
    for (int i = 0; i < 3; i++)
    {
        for (int j = 0; j < 3; j++)
        {
            if (m[i][j] == 0)
            {
                goto hit;
            }
            r += m[i][j];
        }
        if (r > 20)
        {
            break;
        }
    }
hit:
    return r;
}
int main(void)
{
    int v[] = {4, 7, -1, 9};
    int m[3][3] = {{1, 2, 3}, {4, 0, 6}, {7, 8, 9}};
    int p[3][3] = {{9, 9, 9}, {9, 9, 9}, {1, 1, 1}};
    int z[3][3] = {{1, 1, 1}, {1, 1, 1}, {1, 1, 1}};
    for (int w = -2; w < 11; w++)
    {
        printf("%d %d %d %d %d\n", find(v, 4, w), drain(v, w % 5), kind(w), clip(w), w);
    }
    printf("%d %d %d\n", grid(m), grid(p), grid(z));
    return 0;
}
`

func TestGotoBreak(t *testing.T) {
	got := gtCanon(t, gtRun(t, "break", gtBreakSrc))
	gtSame(t, got, gtCanon(t, gtBreakWant))
	gtSameBehaviour(t, gtBreakSrc, got)
}

// A switch's end, where control goes when the switch ends, and a goto that
// falls to its label: a break, and nothing.
func TestGotoBreakFalls(t *testing.T) {
	src := `int kind(int c)
{
    int r = 0;
    switch (c)
    {
    case 1:
        r = 10;
        goto done;
    case 2:
        r = 20;
        break;
    }
done:
    if (r > 9)
    {
        r = 9;
        goto tail;
    }
    else if (r < 0)
        goto tail;
tail:
    return r;
}
`
	want := `int kind(int c)
{
    int r = 0;
    switch (c)
    {
    case 1:
        r = 10;
        break;
    case 2:
        r = 20;
        break;
    }
    if (r > 9)
    {
        r = 9;
    }
    else if (r < 0)
    {
    }
    return r;
}
`
	got := gtCanon(t, gtRun(t, "break", src))
	gtSame(t, got, gtCanon(t, want))
	gtCompiles(t, got)
}

// A function whose jumps the walk does not model is left as it is: here a
// computed goto.
func TestGotoOdd(t *testing.T) {
	src := `int jump(int c)
{
    void *t = &&two;
    for (;;)
    {
        if (c)
        {
            goto out;
        }
        goto *t;
    }
two:
out:
    return c;
}
`
	gtSame(t, gtCanon(t, gtRun(t, "break", src)), gtCanon(t, src))
	gtSame(t, gtCanon(t, gtRun(t, "loop", src)), gtCanon(t, src))
}

const gtLoopSrc = `int printf(const char *, ...);

int digits(const char *s, int *n)
{
    int k = 0;
again:
    if (*s == ' ')
    {
        s++;
        goto again;
    }
    while (*s >= '0' && *s <= '9')
    {
        k = k * 10 + (*s - '0');
        s++;
    }
    if (*s == ',')
    {
        s++;
        *n += k;
        k = 0;
        goto again;
    }
    return k;
}

int settle(int x)
{
    int tries = 0;
top:
    x = x / 2 + 1;
    tries++;
    if (x > 3 && tries < 10)
        goto top;
    return x * 100 + tries;
}

int spin(int x)
{
    int d = 0;
round:
    if (x > 1)
    {
        d++;
        x = x % 2 ? 3 * x + 1 : x / 2;
        goto round;
    }
    else
    {
        return d;
    }
}

int main(void)
{
    int n = 0;
    int last = digits("  12, 3,  45 ,6", &n);
    printf("%d %d\n", n, last);
    for (int x = 0; x < 40; x += 3)
        printf("%d %d\n", settle(x), spin(x));
    return 0;
}
`

const gtLoopWant = `int printf(const char *, ...);
int digits(const char *s, int *n)
{
    int k = 0;
    for (;;)
    {
        if (*s == ' ')
        {
            s++;
            continue;
        }
        while (*s >= '0' && *s <= '9')
        {
            k = k * 10 + (*s - '0');
            s++;
        }
        if (*s == ',')
        {
            s++;
            *n += k;
            k = 0;
            continue;
        }
        break;
    }
    return k;
}
int settle(int x)
{
    int tries = 0;
    do
    {
        x = x / 2 + 1;
        tries++;
    }
    while (x > 3 && tries < 10);
    return x * 100 + tries;
}
int spin(int x)
{
    int d = 0;
    for (;;)
    {
        if (x > 1)
        {
            d++;
            x = x % 2 ? 3 * x + 1 : x / 2;
            continue;
        }
        else
        {
            return d;
        }
    }
}
int main(void)
{
    int n = 0;
    int last = digits("  12, 3,  45 ,6", &n);
    printf("%d %d\n", n, last);
    for (int x = 0; x < 40; x += 3)
    {
        printf("%d %d\n", settle(x), spin(x));
    }
    return 0;
}
`

func TestGotoLoop(t *testing.T) {
	got := gtCanon(t, gtRun(t, "loop", gtLoopSrc))
	gtSame(t, got, gtCanon(t, gtLoopWant))
	gtSameBehaviour(t, gtLoopSrc, got)
}

// Every reason a backward goto is held, one function each: each is left as
// it is.
func TestGotoLoopHolds(t *testing.T) {
	for _, c := range []struct{ why, src string }{
		{"a goto comes from before its label", `int f(int a)
{
    if (a)
    {
        goto L;
    }
    a++;
L:
    a += 2;
    if (a < 10)
    {
        goto L;
    }
    return a;
}
`},
		{"a loop or switch between goto and label", `int f(int a)
{
L:
    a++;
    while (a < 5)
    {
        if (a == 3)
        {
            goto L;
        }
        a += 2;
    }
    return a;
}
`},
		{"a loop or switch between goto and label", `int f(int a)
{
L:
    a++;
    switch (a)
    {
    case 1:
        goto L;
    default:
        break;
    }
    return a;
}
`},
		{"a break or continue leaves the region", `int f(int a)
{
    while (a < 50)
    {
    L:
        a++;
        if (a == 7)
        {
            break;
        }
        if (a % 2)
        {
            goto L;
        }
        a *= 3;
    }
    return a;
}
`},
		{"a break or continue leaves the region", `int f(int a)
{
    while (a < 50)
    {
    L:
        a++;
        if (a == 7)
        {
            continue;
        }
        if (a % 2)
        {
            goto L;
        }
        a *= 3;
    }
    return a;
}
`},
		{"a goto enters the region", `int f(int a)
{
    if (a > 5)
    {
        goto M;
    }
L:
    a++;
M:
    a *= 2;
    if (a < 40)
    {
        goto L;
    }
    return a;
}
`},
		{"a case label of a switch around the region is in it", `int f(int a)
{
    switch (a)
    {
    case 0:
        a = 1;
    L:
        a++;
    case 1:
        a += 2;
        if (a < 9)
        {
            goto L;
        }
    }
    return a;
}
`},
		{"a declaration in the region is named after it", `int f(int a)
{
L:
    a++;
    int b = a * 2;
    if (b < 20)
    {
        goto L;
    }
    return b;
}
`},
		{"another label marks the statement", `int f(int a)
{
M:
L:
    a++;
    if (a < 5)
    {
        goto L;
    }
    if (a < 9)
    {
        goto M;
    }
    return a;
}
`},
		// every body is braced on the graph (and in canonical text), so a
		// label under an if is its block's item, and the goto outside that
		// block is what holds it; the text step said "the label is not a
		// block item" of this source as written, unbraced
		{"a goto is not in the label's block", `int f(int a)
{
    if (a)
    L:
        a++;
    if (a < 5)
    {
        goto L;
    }
    return a;
}
`},
	} {
		var log strings.Builder
		out, err := gtStep(3, "loop")([]byte(c.src), &log)
		if err != nil {
			t.Fatalf("%s: %v", c.why, err)
		}
		if gtCanon(t, string(out)) != gtCanon(t, c.src) {
			t.Errorf("%s: rewritten:\n%s", c.why, out)
		}
		if !strings.Contains(log.String(), c.why) {
			t.Errorf("%s: the report does not say so: %s", c.why, log.String())
		}
	}
}

// Two retry loops, one inside the other's region: the outer is taken, the
// inner held for overlapping it, and a second run takes the inner too.
func TestGotoLoopNested(t *testing.T) {
	src := `int f(int a, int b)
{
L1:
    a++;
L2:
    b++;
    if (b < a)
    {
        goto L2;
    }
    if (a < 5)
    {
        goto L1;
    }
    return a * 100 + b;
}
`
	var log strings.Builder
	once, err := gtStep(3, "loop")([]byte(src), &log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "1 its region overlaps another's") {
		t.Errorf("the report: %s", log.String())
	}
	twice := gtCanon(t, gtRun(t, "loop", string(once)))
	want := `int f(int a, int b)
{
    do
    {
        a++;
        do
        {
            b++;
        }
        while (b < a);
    }
    while (a < 5);
    return a * 100 + b;
}
`
	gtSame(t, twice, gtCanon(t, want))
	gtCompiles(t, twice)
}

// The control: a rewrite that is wrong -- the two-level goto written as
// break, a retry whose continue a loop inside it takes -- is caught by the
// comparison the tests above make.
func TestGotoControl(t *testing.T) {
	wrong := strings.Replace(gtBreakSrc, "            if (m[i][j] == 0)\n                goto hit;", "            if (m[i][j] == 0)\n                break;", 1)
	if wrong == gtBreakSrc {
		t.Fatal("the control did not apply")
	}
	if a, b := gtOutput(t, gtBreakSrc), gtOutput(t, wrong); a != "" && a == b {
		t.Error("a break for a goto that leaves two loops was not caught")
	}
	loopy := `int printf(const char *, ...);
int f(int a)
{
L:
    a++;
    while (a < 5)
    {
        if (a == 3)
        {
            goto L;
        }
        a += 2;
    }
    return a;
}
int main(void)
{
    for (int a = -3; a < 6; a++)
        printf("%d\n", f(a));
    return 0;
}
`
	wrong = strings.Replace(strings.Replace(strings.Replace(loopy, "L:\n", "for (;;) {\n", 1), "goto L;", "continue;", 1), "    return a;\n}\nint main", "    break; }\n    return a;\n}\nint main", 1)
	if a, b := gtOutput(t, loopy), gtOutput(t, wrong); a != "" && a == b {
		t.Error("a continue a loop inside the region takes was not caught")
	}
}

// canon prints text as the pipeline prints a boundary.
func gtCanon(t *testing.T, text string) string {
	t.Helper()
	out, err := cemit.Canonical("p.c", []byte(text))
	if err != nil {
		t.Fatalf("canonical: %v\n%s", err, text)
	}
	return string(out)
}

// sameBehaviour requires the rewritten program to compile silently and print
// what the original prints.
func gtSameBehaviour(t *testing.T, orig, got string) {
	t.Helper()
	if a, b := gtOutput(t, orig), gtOutput(t, got); a != b {
		t.Errorf("the rewritten program prints\n%s\nthe original\n%s", b, a)
	}
}

// output compiles the program with gcc, requiring silence, and runs it; ""
// when there is no gcc.
func gtOutput(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return ""
	}
	dir := t.TempDir()
	c, bin := filepath.Join(dir, "p.c"), filepath.Join(dir, "p")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("gcc", "-std=gnu2x", "-Wall", "-Wextra", "-o", bin, c).CombinedOutput(); err != nil || len(b) > 0 {
		t.Fatalf("gcc is not silent: %v\n%s\n%s", err, b, src)
	}
	// a wrong rewrite may loop for ever: a run that fails or takes too long
	// prints that, which differs from any output
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Stderr = io.Discard
	// and if this test process dies first -- a `go test` killed from outside
	// -- the context never fires: the kernel kills the program with it, or a
	// control that loops for ever outlives the test (one ran for three hours).
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("the program fails: %v\n", err)
	}
	return string(out)
}

// gtGotoTailSrc is a small program, no one code base's: a tail of three
// statements reached from a loop and a switch (the statement before its label
// returns, so the tail goes with the label), a void function's end reached
// from two loops deep, an empty statement at a void end, and one function
// for each hold -- a name an inner block shadows (shadow), a typedef name one
// does (cast), a name declared at the top after the goto (late), a label in
// the tail (twice), a case in it (pick), a declaration in it (decl), a
// statement expression in it (stmtexpr) -- and the tails that are none: one
// statement too long (longtail), one that reaches the end of a loop's body
// (inner), and a label whose address is taken, whose goto is rewritten and
// which stays (addr).  main runs them all and prints what they return.
const gtGotoTailSrc = `int printf(const char *fmt, ...);

typedef int T;

int trace[16];
int ntrace;
int m = 7;

static void note(int v)
{
    trace[ntrace++ % 16] = v;
}

int scan(const char *s, int *len, int *last)
{
    int n = 0;
    int c = 0;
    while (*s)
    {
        c = *s++;
        switch (c)
        {
        case '#':
            goto out;
        case '!':
            n = -n;
            break;
        default:
            n++;
        }
        if (n > 4)
            goto out;
    }
    return n;
out:
    *len = n;
    *last = c;
    note(n);
    return n + 100;
}

static int depth;

void walk(int k)
{
    depth++;
    for (int i = 0; i < k; i++)
    {
        for (int j = 0; j < i; j++)
        {
            if (i * j > 6)
            {
                goto done;
            }
        }
    }
    note(k);
done:
    depth--;
}

void mark(int *p)
{
    while (*p)
    {
        if (*p < 0)
        {
            goto end;
        }
        note(*p);
        p++;
    }
end:
    ;
}

int shadow(int k)
{
    int r = k;
    if (k > 2)
    {
        int r = 0;
        note(r);
        goto out;
    }
    r++;
out:
    note(r);
    return r;
}

int cast(int k)
{
    if (k > 2)
    {
        typedef char T;
        T small = (T)k;
        note(small);
        goto out;
    }
out:
    k++;
    return (T)k * 2;
}

int late(int k)
{
    if (k > 5)
    {
        goto out;
    }
    static int m = 1;
    m += k;
out:
    k++;
    return k + m;
}

int twice(int k)
{
    if (k < 0)
    {
        goto out;
    }
    k *= 2;
    goto again;
out:
    k = -k;
again:
    k++;
    return k;
}

int pick(int k)
{
    switch (k)
    {
    case 0:
        if (ntrace > 3)
        {
            goto out;
        }
        k = 5;
    out:
        k++;
        __attribute__((fallthrough));
    case 1:
        return k;
    }
    return -1;
}

int decl(int k)
{
    if (k > 3)
    {
        goto out;
    }
    k *= 3;
out:
    k--;
    int twice = k * 2;
    return twice;
}

int stmtexpr(int k)
{
    if (k > 3)
    {
        goto out;
    }
    k += 10;
out:
    k = ({ int t = k * 2; t + 1; });
    return k;
}

int longtail(int k)
{
    if (k > 3)
    {
        goto out;
    }
    k += 1;
out:
    k++;
    k++;
    k++;
    k++;
    return k;
}

int inner(int k)
{
    while (k < 100)
    {
        if (k % 7 == 3)
        {
            goto next;
        }
        k += 2;
    next:
        k += 5;
    }
    return k;
}

int addr(int k)
{
    void *where = &&out;
    if (k > 1)
    {
        goto *where;
    }
    if (k)
    {
        goto out;
    }
    k = 9;
out:
    note(k);
    return k;
}

int main(void)
{
    int len = 0, last = 0;
    int a[] = {3, 1, -2, 5, 0};
    const char *words[] = {"ab!c#d", "abcdefg", "ab"};
    for (int i = 0; i < 3; i++)
    {
        int n = scan(words[i], &len, &last);
        printf("%d %d %d\n", n, len, last);
    }
    for (int k = -3; k < 400; k += 37)
    {
        walk(k % 9);
        mark(a + (k & 3));
        int v[10];
        v[0] = shadow(k);
        v[1] = cast(k);
        v[2] = late(k);
        v[3] = twice(k);
        v[4] = pick(k & 1);
        v[5] = decl(k);
        v[6] = stmtexpr(k);
        v[7] = longtail(k);
        v[8] = inner(k);
        v[9] = addr(k & 3);
        printf("%d", depth);
        for (int i = 0; i < 10; i++)
        {
            printf(" %d", v[i]);
        }
        printf(" %d\n", ntrace);
    }
    for (int i = 0; i < 16; i++)
    {
        printf("%d ", trace[i]);
    }
    printf("\n");
    return 0;
}
`

// gtGotoTailWant is gtGotoTailSrc with the six gotos that take their tail
// rewritten: braces where the goto was a statement of its own (an if's or a
// case's), none where it was a block item; scan's label goes with its tail,
// which only the gotos reached; walk's, again's (in twice) and mark's labels
// go and their tails stay, reached by falling in; mark's empty statement goes
// with its label.
const gtGotoTailWant = `int printf(const char *fmt, ...);

typedef int T;

int trace[16];
int ntrace;
int m = 7;

static void note(int v)
{
    trace[ntrace++ % 16] = v;
}

int scan(const char *s, int *len, int *last)
{
    int n = 0;
    int c = 0;
    while (*s)
    {
        c = *s++;
        switch (c)
        {
        case '#':
            { *len = n; *last = c; note(n); return n + 100; }
        case '!':
            n = -n;
            break;
        default:
            n++;
        }
        if (n > 4)
            { *len = n; *last = c; note(n); return n + 100; }
    }
    return n;

}

static int depth;

void walk(int k)
{
    depth++;
    for (int i = 0; i < k; i++)
    {
        for (int j = 0; j < i; j++)
        {
            if (i * j > 6)
            {
                depth--; return;
            }
        }
    }
    note(k);
depth--;
}

void mark(int *p)
{
    while (*p)
    {
        if (*p < 0)
        {
            return;
        }
        note(*p);
        p++;
    }

}

int shadow(int k)
{
    int r = k;
    if (k > 2)
    {
        int r = 0;
        note(r);
        goto out;
    }
    r++;
out:
    note(r);
    return r;
}

int cast(int k)
{
    if (k > 2)
    {
        typedef char T;
        T small = (T)k;
        note(small);
        goto out;
    }
out:
    k++;
    return (T)k * 2;
}

int late(int k)
{
    if (k > 5)
    {
        goto out;
    }
    static int m = 1;
    m += k;
out:
    k++;
    return k + m;
}

int twice(int k)
{
    if (k < 0)
    {
        goto out;
    }
    k *= 2;
    k++; return k;
out:
    k = -k;
k++;
    return k;
}

int pick(int k)
{
    switch (k)
    {
    case 0:
        if (ntrace > 3)
        {
            goto out;
        }
        k = 5;
    out:
        k++;
        __attribute__((fallthrough));
    case 1:
        return k;
    }
    return -1;
}

int decl(int k)
{
    if (k > 3)
    {
        goto out;
    }
    k *= 3;
out:
    k--;
    int twice = k * 2;
    return twice;
}

int stmtexpr(int k)
{
    if (k > 3)
    {
        goto out;
    }
    k += 10;
out:
    k = ({ int t = k * 2; t + 1; });
    return k;
}

int longtail(int k)
{
    if (k > 3)
    {
        goto out;
    }
    k += 1;
out:
    k++;
    k++;
    k++;
    k++;
    return k;
}

int inner(int k)
{
    while (k < 100)
    {
        if (k % 7 == 3)
        {
            goto next;
        }
        k += 2;
    next:
        k += 5;
    }
    return k;
}

int addr(int k)
{
    void *where = &&out;
    if (k > 1)
    {
        goto *where;
    }
    if (k)
    {
        note(k); return k;
    }
    k = 9;
out:
    note(k);
    return k;
}

int main(void)
{
    int len = 0, last = 0;
    int a[] = {3, 1, -2, 5, 0};
    const char *words[] = {"ab!c#d", "abcdefg", "ab"};
    for (int i = 0; i < 3; i++)
    {
        int n = scan(words[i], &len, &last);
        printf("%d %d %d\n", n, len, last);
    }
    for (int k = -3; k < 400; k += 37)
    {
        walk(k % 9);
        mark(a + (k & 3));
        int v[10];
        v[0] = shadow(k);
        v[1] = cast(k);
        v[2] = late(k);
        v[3] = twice(k);
        v[4] = pick(k & 1);
        v[5] = decl(k);
        v[6] = stmtexpr(k);
        v[7] = longtail(k);
        v[8] = inner(k);
        v[9] = addr(k & 3);
        printf("%d", depth);
        for (int i = 0; i < 10; i++)
        {
            printf(" %d", v[i]);
        }
        printf(" %d\n", ntrace);
    }
    for (int i = 0; i < 16; i++)
    {
        printf("%d ", trace[i]);
    }
    printf("\n");
    return 0;
}
`

func TestGotoTail(t *testing.T) {
	var report bytes.Buffer
	out, err := gtStep(3, "tail")([]byte(gtGotoTailSrc), &report)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	gtSame(t, got, gtGotoTailWant)
	for _, want := range []string{
		"6 gotos to a tail of at most 3 statements and a return take the tail; 7 held (3 for a name, 1 for a label, 1 for a case, 1 for a declaration, 1 for a statement expression); 2 to a label that marks no such tail stay",
		"4 labels go, 1 with a tail nothing else reaches; 1 that no goto reaches stay, for their address is taken",
		"3 of the 13 functions with a goto are left with none: mark scan walk",
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, report.String())
		}
	}
	// the gcc control: both compile silently, and print the same
	if gtTailOutput(t, gtGotoTailSrc) != gtTailOutput(t, got) {
		t.Errorf("the rewritten program prints something else")
	}

	// A bound of 4 takes longtail's goto too; one of 1 leaves scan's.
	if n := strings.Count(gtRunTail(t, 4, gtGotoTailSrc), "goto "); n != strings.Count(got, "goto ")-1 {
		t.Errorf("a bound of 4 leaves %d gotos, want one fewer than the %d a bound of 3 does", n, strings.Count(got, "goto "))
	}
	if n := strings.Count(gtRunTail(t, 1, gtGotoTailSrc), "goto "); n != strings.Count(got, "goto ")+2 {
		t.Errorf("a bound of 1 leaves %d gotos, want scan's two more than the %d a bound of 3 does", n, strings.Count(got, "goto "))
	}
}

// The control: the rewrites the name rule holds, made anyway and the label
// dropped, compile as silently -- and print something else, which the comparison above sees.
// shadow's copy reads the inner r, cast's the inner T.
func TestGotoTailWrong(t *testing.T) {
	want := gtTailOutput(t, gtGotoTailSrc)
	for _, wrong := range []struct{ fn, with string }{
		{"shadow", "{ note(r); return r; }"},
		{"cast", "{ k++; return (T)k * 2; }"},
	} {
		src := gtGotoTailSrc
		at := strings.Index(src, "\nint "+wrong.fn+"(")
		g := at + strings.Index(src[at:], "goto out;")
		src = src[:g] + wrong.with + src[g+len("goto out;"):]
		l := g + strings.Index(src[g:], "\nout:\n")
		src = src[:l] + src[l+len("\nout:"):]
		if gtTailOutput(t, src) == want {
			t.Errorf("%s: copying its tail over its goto prints the same, so the test would not see a wrong rewrite", wrong.fn)
		}
	}
}

// tailOutput compiles src with gcc under the sweep's warnings, requires it to
// print nothing, runs it and returns what it prints.  With no gcc the test is
// skipped.
func gtTailOutput(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	dir := t.TempDir()
	c, bin := filepath.Join(dir, "p.c"), filepath.Join(dir, "p")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command("gcc", "-std=gnu2x", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter", "-o", bin, c).CombinedOutput()
	if err != nil || len(b) > 0 {
		t.Fatalf("gcc is not silent: %v\n%s", err, b)
	}
	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("the program fails: %v", err)
	}
	return string(out)
}

// A small tokenizer's code, not any one code base's.  `classify` leaves early
// to one label from three places at two depths, and the label marks the
// function's tail; `sum` jumps to the end of a loop's body, and its label,
// `next: ;`, goes with its empty statement.
const gtGbTaken = `int classify(const char *s, int n)
{
    int kind = 0;
    if (!s)
        goto done;
    if (n > 3)
    {
        kind = 1;
        if (s[0] == '#')
        {
            kind = 2;
            goto done;
        }
        goto done;
    }
    kind = 3;
done:
    kind *= 10;
    return kind;
}

int sum(const int *v, int n)
{
    int t = 0;
    for (int i = 0; i < n; i++)
    {
        if (v[i] < 0)
            goto next;
        t += v[i];
        if (t > 100)
            goto next;
        t++;
    next: ;
    }
    return t;
}
`

const gtGbTakenWant = `int classify(const char *s, int n)
{
    int kind = 0;
    do {
if (!s)
        break;
    if (n > 3)
    {
        kind = 1;
        if (s[0] == '#')
        {
            kind = 2;
            break;
        }
        break;
    }
    kind = 3;
} while (0);
kind *= 10;
    return kind;
}

int sum(const int *v, int n)
{
    int t = 0;
    for (int i = 0; i < n; i++)
    {
        do {
if (v[i] < 0)
            break;
        t += v[i];
        if (t > 100)
            break;
        t++;
    } while (0);

    }
    return t;
}
`

const gtGbTakenMain = `#include <stdio.h>
int main(void)
{
    const char *ss[] = {0, "ab", "#abcd", "abcd", "x#yz"};
    int v[] = {3, -1, 50, 60, 7, -2, 9};
    for (int i = 0; i < 5; i++)
        for (int n = 0; n < 6; n++)
            printf("%d ", classify(ss[i], n));
    for (int n = 0; n <= 7; n++)
        printf("%d ", sum(v, n));
    printf("\n");
    return 0;
}
`

func TestGotoBlock(t *testing.T) {
	got := gtRun(t, "block", gtGbTaken)
	gtSame(t, got, gtGbTakenWant)
	gtBehaves(t, gtGbTaken, got, gtGbTakenMain)

	// the control: a wrong rewrite -- one goto that falls through instead of
	// leaving -- is seen by the comparison
	wrong := got
	if i := strings.Index(got, "if (t > 100)"); i >= 0 {
		wrong = got[:i] + strings.Replace(got[i:], "break;", "t += 0;", 1)
	}
	if wrong == got {
		t.Fatal("the control's edit did not apply")
	}
	if gtRunC(t, gtGbTaken+gtGbTakenMain) == gtRunC(t, wrong+gtGbTakenMain) {
		t.Error("the control: a goto that does not leave prints the same")
	}
}

// Every reason a label is held, one function each; none of them is touched.
func TestGotoBlockHolds(t *testing.T) {
	cases := map[string]string{
		// a retry: the goto is after its label
		"backward": `int f(int n)
{
again:
    n--;
    if (n > 3)
        goto again;
    return n;
}
`,
		// a goto in a loop: break would leave the loop, not the region
		"loop": `int f(const int *v, int n)
{
    int t = 0;
    for (int i = 0; i < n; i++)
        if (v[i] < 0)
            goto out;
        else
            t += v[i];
    t = -t;
out:
    return t;
}
`,
		// a goto in a switch: break would leave the switch
		"switch": `int f(int c)
{
    int r = 0;
    switch (c)
    {
    case 1:
        goto out;
    default:
        r = 2;
    }
    r++;
out:
    return r;
}
`,
		// the region has a break of its own, for the loop it is in
		"stray break": `int f(const int *v, int n)
{
    int t = 0;
    for (int i = 0; i < n; i++)
    {
        if (v[i] == 0)
            goto skip;
        if (v[i] < 0)
            break;
        t += v[i];
    skip:
        t++;
    }
    return t;
}
`,
		// and a continue
		"stray continue": `int f(const int *v, int n)
{
    int t = 0;
    for (int i = 0; i < n; i++)
    {
        if (v[i] == 0)
            goto skip;
        if (v[i] < 0)
            continue;
        t += v[i];
    skip:
        t++;
    }
    return t;
}
`,
		// the region holds a case of the switch it is the body of
		"case": `int f(int c)
{
    int r = 0;
    switch (c)
    {
        if (c > 9)
            goto out;
    case 1:
        r = 1;
    out:
        r++;
    }
    return r;
}
`,
		// the region declares what the label's statement reads
		"declaration": `int f(int n)
{
    if (n < 0)
        goto out;
    int k = n * 2;
    n += k;
out:
    k = 1;
    return n + k;
}
`,
		// a goto from before the region to a label inside it (and that
		// label is held, its goto being in a loop)
		"into": `int f(int n)
{
    while (n > 5)
        if (--n == 7)
            goto mid;
    n++;
    if (n < 0)
        goto out;
    n *= 2;
mid:
    n += 3;
out:
    return n;
}
`,
		// the label is inside an if, not a statement of its block
		"nested": `int f(int n)
{
    if (n < 0)
        goto out;
    n++;
    if (n > 1)
    out:
        n = 0;
    return n;
}
`,
		// a label's address is taken
		"computed": `int f(int n)
{
    void *p = &&out;
    if (n < 0)
        goto *p;
    if (n > 3)
        goto out;
    n++;
out:
    return n;
}
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			out, err := gtStep(3, "block")([]byte(src), &b)
			if err != nil {
				t.Fatal(err)
			}
			rep := b.String()
			gtSame(t, string(out), src)
			if !strings.Contains(rep, "0 gotos become") || name == "into" && !strings.Contains(rep, "1 (1 gotos) for a jump into the region") {
				t.Errorf("report: %s", rep)
			}
		})
	}
}

// Two labels whose regions cross: `a`'s goto is in `b`'s region.  One round
// takes `b`, the first in the text; the next finds `a`'s goto inside b's new
// do-while, and holds it.  Behaviour is kept.
func TestGotoBlockRounds(t *testing.T) {
	src := `int f(int n)
{
    int r = 0;
    if (n == 1)
        goto b;
    if (n == 2)
        goto a;
    r += 5;
b:
    r += 7;
a:
    return r;
}
`
	var w bytes.Buffer
	out, err := gtStep(3, "block")([]byte(src), &w)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.String(), "1 gotos become a break out of a do-while(0), and their 1 labels go, in 1 rounds that rewrote") ||
		!strings.Contains(w.String(), "1 (1 gotos) for a loop or switch between") {
		t.Errorf("report: %s", w.String())
	}
	if !strings.Contains(string(out), "goto a;") || strings.Contains(string(out), "goto b;") {
		t.Errorf("out:\n%s", out)
	}
	gtBehaves(t, src, string(out), `#include <stdio.h>
int main(void) { for (int n = 0; n < 4; n++) printf("%d ", f(n)); printf("\n"); return 0; }
`)
}

// The hold for a loop is needed: the naive rewrite of "loop" -- its goto a
// break -- leaves the for instead of the region, and prints otherwise.
func TestGotoBlockLoopControl(t *testing.T) {
	src := `int f(const int *v, int n)
{
    int t = 0;
    for (int i = 0; i < n; i++)
        if (v[i] < 0)
            goto out;
        else
            t += v[i];
    t = -t;
out:
    return t;
}
`
	naive := strings.Replace(strings.Replace(strings.Replace(src,
		"    for", "    do {\n    for", 1), "goto out;", "break;", 1), "out:", "} while (0);", 1)
	main := `#include <stdio.h>
int main(void) { int v[] = {1, 2, -3, 4}; printf("%d\n", f(v, 4)); return 0; }
`
	if gtRunC(t, src+main) == gtRunC(t, naive+main) {
		t.Error("the naive rewrite of a goto out of a loop prints the same")
	}
}

// behaves asks gcc to compile before and after, each with main, silently
// under -Wall -Wextra, and requires the two programs to print the same.
func gtBehaves(t *testing.T, before, after, main string) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return
	}
	a, b := gtRunC(t, before+main), gtRunC(t, after+main)
	if a != b {
		t.Errorf("before prints %q, after %q", a, b)
	}
}

// runC compiles src silently and runs it, returning what it prints.
func gtRunC(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	dir := t.TempDir()
	c := filepath.Join(dir, "p.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p")
	out, err := exec.Command("gcc", "-std=gnu2x", "-O0", "-Wall", "-Wextra", "-o", exe, c).CombinedOutput()
	if err != nil || len(out) > 0 {
		t.Fatalf("gcc is not silent: %v\n%s\n%s", err, out, src)
	}
	got, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

// GotoTail with a tail of no statements, GotoReturn's rule before this took
// it: a goto to a label that returns is that return; one whose value could be
// shadowed at the goto is held, and so is its label.  A label's return that
// only the gotos reached goes with it; one after a labeled statement stays,
// for the dead-statement rule (crefactor/graph's Editor.DeadStmt) to take.
func TestGotoTailNoStatements(t *testing.T) {
	src := `int scan(const char *s)
{
    int n = 0;
    if (!s)
        goto fail;
    while (*s)
    {
        if (*s == '#')
            goto done;
        n++;
        s++;
    }
    goto done;
fail:
    return -1;
done:
    return n;
}

int shadow(int k)
{
    int r = k;
    if (k > 2)
    {
        int r = 0;
        goto out;
    }
out:
    return r;
}
`
	want := `int scan(const char *s)
{
    int n = 0;
    if (!s)
        return -1;
    while (*s)
    {
        if (*s == '#')
            return n;
        n++;
        s++;
    }
    return n;

return n;
}

int shadow(int k)
{
    int r = k;
    if (k > 2)
    {
        int r = 0;
        goto out;
    }
out:
    return r;
}
`
	got := gtRunTail(t, 0, src)
	gtSame(t, got, want)
	gtCompiles(t, got)
}
