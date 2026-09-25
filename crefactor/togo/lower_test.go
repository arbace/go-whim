package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The lowered form on C that is not vim: every function of each program is
// lowered to blocks and printed back as C (lower_c.go), and the printed
// program, compiled with gcc, must print what the original prints -- the
// lowering kept what every function does.  The programs are the Java
// backend's tests', with their hosts.

// loweredC lowers src's functions and prints them back as C.
func loweredC(t *testing.T, dir, src string) string {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "lowered.c")
	if rc := Run([]string{c, dir, "-lowerc", out}, io.Discard, Profile{}); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runC compiles src with the harness and returns what it prints.
func runC(t *testing.T, dir, name, src, harness string) string {
	c := filepath.Join(dir, name+".c")
	h := filepath.Join(dir, name+"-harness.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h, []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, name)
	if o, err := exec.Command("gcc", "-w", "-O0", "-Wno-error=incompatible-pointer-types", "-o", exe, c, h).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s\n%s", err, o, numbered(src))
	}
	// a wrong lowering may loop for ever: bounded, and killed with the test
	b, err := bounded(60*time.Second, "", exe)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return string(b)
}

func lowerSame(t *testing.T, src, harness string) string {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	dir := t.TempDir()
	low := loweredC(t, dir, src)
	want := runC(t, dir, "orig", src, harness)
	got := runC(t, dir, "low", low, harness)
	if got != want {
		t.Errorf("the lowered C prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(low))
	}
	return low
}

func TestLowerIntegers(t *testing.T) { lowerSame(t, javaIntsC, javaHarnessC) }
func TestLowerFlow(t *testing.T)     { lowerSame(t, javaFlowC, javaHarnessC) }
func TestLowerStrings(t *testing.T)  { lowerSame(t, javaStringsC, javaHarnessC) }
func TestLowerStructs(t *testing.T)  { lowerSame(t, javaStructsC, javaHarnessC) }
func TestLowerProfile(t *testing.T) {
	lowerSame(t, javaProfileC, "#include <stdlib.h>\n#include <string.h>\nvoid *alloc(unsigned long n) { return calloc(1, n); }\nvoid vim_free(void *p) { free(p); }\n"+javaHarnessC)
}
func TestLowerVarargs(t *testing.T)   { lowerSame(t, javaVarargsC, javaHarnessC) }
func TestLowerGrow(t *testing.T)      { lowerSame(t, javaGrowC, javaGrowHarnessC) }
func TestLowerPointers(t *testing.T)  { lowerSame(t, javaPointersC, javaHarnessC) }
func TestLowerGoto(t *testing.T)      { lowerSame(t, javaGotoC, javaHarnessC) }
func TestLowerDeadLabel(t *testing.T) { lowerSame(t, javaDeadLabelC, javaHarnessC) }

// What the Java tests do not reach and the lowering takes apart: an if-else
// whose arms both go on, ?: and && and || whose later operands do
// something, as values and as statements, a compound assignment whose right
// side calls something that changes its left, an lvalue whose index calls,
// a goto backwards (a loop the Java refuses), a switch inside a loop whose
// cases fall through and continue, and a case label inside a block of its
// switch.
const lowerExtraC = javaHost + `
static int calls, g;
int bump(int k) { calls++; g += k; return k; }
int *slot(int *a, int i) { calls++; return &a[i]; }

int arms(int x)
{
    int r = 0, s = 0;
    if (x > 2)
    {
        r = 1;
        s = 2;
    }
    else if (x > 0)
        r = 3;
    else
    {
        s = 4;
        r = s + 1;
    }
    return r * 10 + s;
}

int values(int x)
{
    int a = 0, b = 0;
    int t = x > 3 ? (a = bump(x)) : (b = bump(-x));
    int u = (x & 1) && (a += bump(1)) > 2;
    int v = (x & 2) || (b = bump(7), b > 3);
    x > 5 ? bump(100) : bump(200);
    (x & 4) && bump(1000);
    (x & 8) || bump(2000);
    return t + u * 3 + v * 5 + a * 7 + b * 11;
}

int compound(void)
{
    g = 5;
    g += bump(3);
    int a[4] = { 1, 2, 3, 4 };
    int i = 0;
    a[i++] += 10;
    *slot(a, 2) *= 3;
    a[bump(1)] -= 1;
    return a[0] * 1000 + a[1] * 100 + a[2] * 10 + a[3] + i;
}

int backwards(int n)
{
    int t = 0;
again:
    t += n;
    if (--n > 0)
        goto again;
    return t;
}

int machine(int n)
{
    int t = 0;
    for (int i = 0; i < n; i++)
    {
        switch (i % 4)
        {
        case 0:
            t += 1;
        case 1:
            t += 10;
            if (i > 5)
                continue;
            break;
        case 2:
            {
                t += 100;
        case 3:
                t += 1000;
            }
        }
        t += 5;
    }
    return t;
}

void run(void)
{
    for (int x = -1; x < 5; x++)
        out(arms(x));
    for (int x = 0; x < 16; x++)
        out(values(x));
    out(g);
    out(compound());
    out(g);
    out(calls);
    out(backwards(4));
    out(machine(12));
}
`

func TestLowerExtra(t *testing.T) { lowerSame(t, lowerExtraC, javaHarnessC) }
