package xform

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A small tokenizer's code, not any one code base's.  `classify` leaves early
// to one label from three places at two depths, and the label marks the
// function's tail; `sum` jumps to the end of a loop's body, and its label,
// `next: ;`, goes with its empty statement.
const gbTaken = `int classify(const char *s, int n)
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

const gbTakenWant = `int classify(const char *s, int n)
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

const gbTakenMain = `#include <stdio.h>
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
	got := run(t, GotoBlock(), gbTaken, "--at-least", "5")
	same(t, got, gbTakenWant)
	behaves(t, gbTaken, got, gbTakenMain)
	refuses(t, GotoBlock(), gbTaken, "fewer than the 6", "--at-least", "6")
	refuses(t, GotoBlock(), gbTaken, "unexpected argument", "--at-most", "6")

	// the control: a wrong rewrite -- one goto that falls through instead of
	// leaving -- is seen by the comparison
	wrong := strings.Replace(got, "if (t > 100)\n            break;", "if (t > 100)\n            t += 0;", 1)
	if wrong == got {
		t.Fatal("the control's edit did not apply")
	}
	if runC(t, gbTaken+gbTakenMain) == runC(t, wrong+gbTakenMain) {
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
			out, err := GotoBlock()([]byte(src), nil, &b)
			if err != nil {
				t.Fatal(err)
			}
			rep := b.String()
			same(t, string(out), src)
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
	out, err := GotoBlock()([]byte(src), nil, &w)
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
	behaves(t, src, string(out), `#include <stdio.h>
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
	if runC(t, src+main) == runC(t, naive+main) {
		t.Error("the naive rewrite of a goto out of a loop prints the same")
	}
}

// behaves asks gcc to compile before and after, each with main, silently
// under -Wall -Wextra, and requires the two programs to print the same.
func behaves(t *testing.T, before, after, main string) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return
	}
	a, b := runC(t, before+main), runC(t, after+main)
	if a != b {
		t.Errorf("before prints %q, after %q", a, b)
	}
}

// runC compiles src silently and runs it, returning what it prints.
func runC(t *testing.T, src string) string {
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
