package xform

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// localOutC has each shape LocalOut rewrites, and each reason it holds a
// parameter back.
const localOutC = `int printf(const char *, ...);
typedef unsigned char char_u;
static int gv;
static int *kept;
static void split(int v, int *q, int *r);
static void advance(char_u **);
static void split(int v, int *q, int *r) { *q = v / 10; *r = v % 10; }
static void bump(int *n) { *n += 1; }
static int maybe_len(const char *s, int *lenp) { int n = 0; while (s[n]) n++; if (lenp != 0) *lenp = n; return n > 3; }
static int pick(int a, int *out) { if (!out) return -1; if (a > 2) { *out = a * 2; return 1; } return 0; }
static char_u *skip(char_u *p, int *count) { while (*p == ' ') { p++; (*count)++; } return p; }
static void advance(char_u **pp) { (*pp)++; }
static void keep(int *p) { kept = p; }
static void twice(int *n) { bump(n); bump(n); }
static void global_bump(int *n) { *n += 5; }
static int both(int *a, int *b) { *a = 1; *b = 2; return 3; }
int main(void)
{
    int q, r, len = -1, n = 5, m = 0, k = 0, h = 0, cnt = 0, a, b, e = 0;
    char_u buf[] = "  word";
    char_u *w = buf;
    split(47, &q, &r);
    printf("%d %d\n", q, r);
    int ok = maybe_len("abcdef", &len);
    printf("%d %d\n", ok, len);
    printf("%d\n", maybe_len("xy", &gv) + gv);
    bump(&n);
    printf("%d\n", n);
    ok = pick(5, &m);
    printf("%d %d\n", ok, m);
    ok = pick(1, &m) + m;
    printf("%d %d\n", ok, m);
    w = skip(w, &cnt);
    printf("%s %d\n", (char *)w, cnt);
    advance(&w);
    printf("%s\n", (char *)w);
    keep(&k);
    twice(&h);
    global_bump(&gv);
    printf("%d %d\n", h, gv);
    if (both(&a, &b) == 3 && (e = a + b) > 0)
        printf("%d %d %d\n", a, b, e);
    return 0;
}
`

func TestLocalOut(t *testing.T) {
	var log bytes.Buffer
	out, err := LocalOut(nil)([]byte(localOutC), nil, &log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"split__o = split(47, q, r), q = split__o.q, r = split__o.r", // void, two: a struct
		"bump(&n)",                    // twice hands its pointer to bump
		"maybe_len(\"abcdef\", &len)", // another call passes a global's address
		"pick(5, &m)",                 // another call reads m unsequenced
		"both__o = both(a, b), a = both__o.a, b = both__o.b, both__o.r__", // read after &&: sequenced
		"(w = advance(w))", // a pointer's pointer
		"keep(&k)",         // keep keeps the pointer
		"twice(&h)",        // twice hands its pointer on
		"global_bump(&gv)", // a file-scope object's address
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s\n%s", want, got, log.String())
		}
	}
	if _, err := translate(out); err != nil {
		t.Fatalf("the result does not type-check: %v\n%s", err, got)
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	if a, b := gccRun(t, localOutC), gccRun(t, got); a != b {
		t.Errorf("the program printed\n%s\nand after the step\n%s\n%s", a, b, got)
	}
}

// gccRun is what a C program prints.
func gccRun(t *testing.T, src string) string {
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
