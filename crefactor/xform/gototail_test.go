package xform

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

// gotoTailSrc is a small program, no one code base's: a tail of three
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
const gotoTailSrc = `int printf(const char *fmt, ...);

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

// gotoTailWant is gotoTailSrc with the six gotos that take their tail
// rewritten: braces where the goto was a statement of its own (an if's or a
// case's), none where it was a block item; scan's label goes with its tail,
// which only the gotos reached; walk's, again's (in twice) and mark's labels
// go and their tails stay, reached by falling in; mark's empty statement goes
// with its label.
const gotoTailWant = `int printf(const char *fmt, ...);

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
	out, err := GotoTail(GotoTailKnobs{Tail: 3})([]byte(gotoTailSrc), []string{"--at-least", "6"}, &report)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	same(t, got, gotoTailWant)
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
	if tailOutput(t, gotoTailSrc) != tailOutput(t, got) {
		t.Errorf("the rewritten program prints something else")
	}
	refuses(t, GotoTail(GotoTailKnobs{Tail: 3}), gotoTailSrc, "fewer than the 7", "--at-least", "7")
	refuses(t, GotoTail(GotoTailKnobs{Tail: 3}), gotoTailSrc, "unexpected argument", "--tail", "3")

	// A bound of 4 takes longtail's goto too; one of 1 leaves scan's.
	if n := strings.Count(run(t, GotoTail(GotoTailKnobs{Tail: 4}), gotoTailSrc), "goto "); n != strings.Count(got, "goto ")-1 {
		t.Errorf("a bound of 4 leaves %d gotos, want one fewer than the %d a bound of 3 does", n, strings.Count(got, "goto "))
	}
	if n := strings.Count(run(t, GotoTail(GotoTailKnobs{Tail: 1}), gotoTailSrc), "goto "); n != strings.Count(got, "goto ")+2 {
		t.Errorf("a bound of 1 leaves %d gotos, want scan's two more than the %d a bound of 3 does", n, strings.Count(got, "goto "))
	}
}

// The control: the rewrites the name rule holds, made anyway and the label
// dropped, compile as silently -- and print something else, which the comparison above sees.
// shadow's copy reads the inner r, cast's the inner T.
func TestGotoTailWrong(t *testing.T) {
	want := tailOutput(t, gotoTailSrc)
	for _, wrong := range []struct{ fn, with string }{
		{"shadow", "{ note(r); return r; }"},
		{"cast", "{ k++; return (T)k * 2; }"},
	} {
		src := gotoTailSrc
		at := strings.Index(src, "\nint "+wrong.fn+"(")
		g := at + strings.Index(src[at:], "goto out;")
		src = src[:g] + wrong.with + src[g+len("goto out;"):]
		l := g + strings.Index(src[g:], "\nout:\n")
		src = src[:l] + src[l+len("\nout:"):]
		if tailOutput(t, src) == want {
			t.Errorf("%s: copying its tail over its goto prints the same, so the test would not see a wrong rewrite", wrong.fn)
		}
	}
}

// tailOutput compiles src with gcc under the sweep's warnings, requires it to
// print nothing, runs it and returns what it prints.  With no gcc the test is
// skipped.
func tailOutput(t *testing.T, src string) string {
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

func TestReturnsVoid(t *testing.T) {
	ast, err := parse([]byte("void a(void) {}\nstatic void b(int x) {}\nvoid *c(void) { return 0; }\nint d(void) { return 0; }\nvoid (*e(void))(int) { return 0; }\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"a": true, "b": true, "c": false, "d": false, "e": false}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Case != cc.ExternalDeclarationFuncDef || ed.Position().Filename != file {
			continue
		}
		name := ed.FunctionDefinition.Declarator.Name()
		if got := returnsVoid(ed.FunctionDefinition); got != want[name] {
			t.Errorf("%s: returnsVoid = %v, want %v", name, got, want[name])
		}
		delete(want, name)
	}
	if len(want) > 0 {
		t.Errorf("not seen: %v", want)
	}
}
