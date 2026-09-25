package xform

import (
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

// The fixtures are a small scanner's code, not any one code base's.  Each
// step's output is compared printed canonically, as the pipeline prints every
// boundary, and the programs are compiled and run: the rewritten one must
// compile silently under -Wall -Wextra and print what the original prints.

const breakSrc = `int printf(const char *, ...);

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

const breakWant = `int printf(const char *, ...);
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
	got := canon(t, run(t, GotoBreak(), breakSrc, "--at-least", "3"))
	same(t, got, canon(t, breakWant))
	sameBehaviour(t, breakSrc, got)
	refuses(t, GotoBreak(), breakSrc, "fewer than the 99", "--at-least", "99")
	refuses(t, GotoBreak(), breakSrc, "unexpected argument", "--floor", "1")
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
        ;
    }
    return r;
}
`
	got := canon(t, run(t, GotoBreak(), src))
	same(t, got, canon(t, want))
	compiles(t, got)
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
	same(t, canon(t, run(t, GotoBreak(), src)), canon(t, src))
	same(t, canon(t, run(t, GotoLoop(), src)), canon(t, src))
}

const loopSrc = `int printf(const char *, ...);

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

const loopWant = `int printf(const char *, ...);
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
	got := canon(t, run(t, GotoLoop(), loopSrc, "--at-least", "4"))
	same(t, got, canon(t, loopWant))
	sameBehaviour(t, loopSrc, got)
	refuses(t, GotoLoop(), loopSrc, "fewer than the 5", "--at-least", "5")
	refuses(t, GotoLoop(), loopSrc, "unexpected argument", "--floor", "1")
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
		{"the label is not a block item", `int f(int a)
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
		out, err := GotoLoop()([]byte(c.src), nil, &log)
		if err != nil {
			t.Fatalf("%s: %v", c.why, err)
		}
		if string(out) != c.src {
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
	once, err := GotoLoop()([]byte(src), nil, &log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "1 its region overlaps another's") {
		t.Errorf("the report: %s", log.String())
	}
	twice := canon(t, run(t, GotoLoop(), string(once)))
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
	same(t, twice, canon(t, want))
	compiles(t, twice)
}

// The control: a rewrite that is wrong -- the two-level goto written as
// break, a retry whose continue a loop inside it takes -- is caught by the
// comparison the tests above make.
func TestGotoControl(t *testing.T) {
	wrong := strings.Replace(breakSrc, "            if (m[i][j] == 0)\n                goto hit;", "            if (m[i][j] == 0)\n                break;", 1)
	if wrong == breakSrc {
		t.Fatal("the control did not apply")
	}
	if a, b := output(t, breakSrc), output(t, wrong); a != "" && a == b {
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
	if a, b := output(t, loopy), output(t, wrong); a != "" && a == b {
		t.Error("a continue a loop inside the region takes was not caught")
	}
}

// canon prints text as the pipeline prints a boundary.
func canon(t *testing.T, text string) string {
	t.Helper()
	out, err := cemit.Canonical(file, []byte(text))
	if err != nil {
		t.Fatalf("canonical: %v\n%s", err, text)
	}
	return string(out)
}

// sameBehaviour requires the rewritten program to compile silently and print
// what the original prints.
func sameBehaviour(t *testing.T, orig, got string) {
	t.Helper()
	if a, b := output(t, orig), output(t, got); a != b {
		t.Errorf("the rewritten program prints\n%s\nthe original\n%s", b, a)
	}
}

// output compiles the program with gcc, requiring silence, and runs it; ""
// when there is no gcc.
func output(t *testing.T, src string) string {
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
