package togo

import (
	"strings"
	"testing"
)

// tidyOne is scmTidy on one function's text, with mem and fr the
// function's own and the names given reading memory.
func tidyOne(t *testing.T, text string, void bool, memNames ...string) string {
	t.Helper()
	names := map[string]bool{}
	for _, n := range memNames {
		names[n] = true
	}
	out, err := scmTidy(text, names, void, &scmTidyStats{})
	if err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
	return out
}

// squash is text with its runs of white space one space: a comparison of
// forms, not of layout.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestScmTidyRead(t *testing.T) {
	for _, src := range []string{
		`(define (f ed) (g "a \"quoted\" ) ( [ string" #\x28 #\space '() -1 +2 #t))`,
		`(define (f ed) (let ([x (ch #\x5d)]) [x]))`,
	} {
		fs, err := scmRead(src)
		if err != nil || len(fs) != 1 {
			t.Fatalf("%q: %v", src, err)
		}
		if got := fs[0].flat(); got != src {
			t.Errorf("read back\n%s\nas\n%s", src, got)
		}
	}
	for _, bad := range []string{`(f (g)`, `(f))`, `(f [g)]`, `(f ; a comment` + "\n)"} {
		if _, err := scmRead(bad); err == nil {
			t.Errorf("%q read", bad)
		}
	}
}

func TestScmTidyLayout(t *testing.T) {
	long := `(define (f ed p) (if (fx>? p 0) (g ed aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff gggggggggg hhhhhhhhhh) (let loop1 ([p p]) (loop1 (fx+ p 1)))))`
	got := tidyOne(t, long, false)
	for _, l := range strings.Split(got, "\n") {
		if len(l) > scmWidth {
			t.Errorf("a line of %d columns:\n%s", len(l), got)
		}
	}
	// the named let on a line of its own, its body under it
	if !strings.Contains(got, "\n      (let loop1 ([p p])\n        (loop1 (fx+ p 1)))") {
		t.Errorf("the named let not laid out as a body:\n%s", got)
	}
	if squash(got) != squash(long) {
		t.Errorf("the forms moved:\n%s", got)
	}
}

func TestScmTidyJoins(t *testing.T) {
	// join2 is called once: in place, its parameter that the call passes
	// by its own name not bound again, the other bound
	src := `(define (f ed q)
  (let ([mem (ed-mem ed)])
    (define (join2 p n)
      (g ed p n)
      (h ed p))
    (let ([p (fx+ q 1)])
      (when (fx>? p 3) (k ed))
      (join2 p (m ed)))))`
	want := `(define (f ed q)
  (let ([mem (ed-mem ed)])
    (let ([p (fx+ q 1)])
      (when (fx>? p 3) (k ed))
      (let ([n (m ed)])
        (g ed p n)
        (h ed p)))))
`
	if got := tidyOne(t, src, false); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	// called from two places, or where a binding would capture what its
	// body names: it stays
	for _, src := range []string{
		`(define (f ed q) (define (join2 p) (g ed p)) (if (fx>? q 0) (join2 q) (join2 0)))`,
		`(define (f ed q) (define (join2 p) (g ed p q)) (let ([q 5]) (join2 q)))`,
	} {
		if got := tidyOne(t, src, false); !strings.Contains(got, "(define (join2 p)") {
			t.Errorf("inlined:\n%s", got)
		}
	}
	// several forms as an if's arm: the if a cond
	src = `(define (f ed q) (define (join2) (g ed) (h ed)) (if (fx>? q 0) (join2) (k ed)))`
	if got := tidyOne(t, src, false); !strings.Contains(squash(got), "(cond [(fx>? q 0) (g ed) (h ed)] [else (k ed)])") {
		t.Errorf("got\n%s", got)
	}
}

func TestScmTidyInvariants(t *testing.T) {
	src := `(define (f ed s n) (let loop1 ([s s] [n n] [i 0]) (if (fx<? i n) (loop1 s n (fx+ i (g ed s))) i)))`
	if got := squash(tidyOne(t, src, false)); got != "(define (f ed s n) (let loop1 ([i 0]) (if (fx<? i n) (loop1 (fx+ i (g ed s))) i)))" {
		t.Errorf("got %s", got)
	}
	// n is bound again inside the body before a jump passes it: not the
	// same value
	src = `(define (f ed n) (let loop1 ([n n] [i 0]) (if (fx<? i n) (let ([n (fx- n 1)]) (loop1 n (fx+ i 1))) i)))`
	if got := squash(tidyOne(t, src, false)); !strings.Contains(got, "[n n]") {
		t.Errorf("got %s", got)
	}
	// a name that reads memory is not a binding's value
	src = `(define (f ed) (let loop1 ([vcol vcol] [i 0]) (if (fx<? i 3) (loop1 vcol (fx+ i 1)) i)))`
	if got := squash(tidyOne(t, src, false, "vcol")); !strings.Contains(got, "[vcol vcol]") {
		t.Errorf("got %s", got)
	}
}

// scmTidyC is C whose Scheme every rule of scm_tidy.go rewrites: a tail
// two branches share, written once and then in place; loops whose
// parameters do not change; *d++ = *s++; a switch whose cases share their
// code; conditions of conditions; comparisons with zero.
const scmTidyC = javaHost + `
static int calls;
static void note(int v) { calls += v; }
static char *copy(char *d, const char *s, unsigned long n) { char *r = d; while (n-- > 0) *d++ = *s++; return r; }
static int tail(int a, int b) {
    int r = a;
    if (a > b) { r = a - b; note(1); }
    r = r * 2;
    note(r);
    return r + b;
}
static int tail2(int a, int b) { int r = a + b; if (a > b) note(1); r = r * 2; note(r); return r; }
static int sumk(int a, int n) { int k = a * 3, s = 0; note(k); for (int i = 0; i < n; i++) s += k + i; return s + k; }
static int count(const char *s, int c, int lim) {
    int n = 0;
    for (int i = 0; s[i] != 0 && i < lim; i++) if (s[i] == c) n++;
    return n;
}
static int kind(int c) {
    switch (c) {
    case 'a': case 'e': return 1;
    case 'b': note(2); return 2;
    case 'c': note(2); return 2;
    case 'x': return 9;
    default: return 9;
    }
}
static int cond(int a, int b, int c) { return (a == 0 && b != 0 && c > 1) || (a != 0 && (b == 0 || c == 0)); }
static void fill(char *p, int n) { int i = 0; while (i < n) { p[i] = 'a' + i; i++; } p[i] = 0; if (n == 0) note(5); }
void run(void) {
    char buf[16], dst[16];
    fill(buf, 6); outs(buf);
    fill(buf, 0); out(calls);
    fill(buf, 9);
    copy(dst, buf, 10); outs(dst);
    out(tail(7, 3)); out(tail(2, 5)); out(calls);
    out(tail2(7, 3)); out(tail2(2, 5)); out(calls); out(sumk(4, 5)); out(sumk(-2, 0));
    out(count("abcabca", 'a', 100)); out(count("abcabca", 'a', 3));
    out(kind('a') + kind('e') + kind('b') + kind('c') + kind('x') + kind('z')); out(calls);
    out(cond(0, 1, 2)); out(cond(0, 0, 2)); out(cond(1, 0, 5)); out(cond(1, 1, 1)); out(cond(1, 1, 0));
}
`

func TestScmTidy(t *testing.T) {
	prog := scmSame(t, scmTidyC, Profile{}, javaHarnessC)
	// sumk's k, which the loop never changes, is not the loop's
	for _, want := range []string{"(case ", "(let loop1 ([s 0] [i 0])"} {
		if !strings.Contains(prog, want) {
			t.Errorf("no %q in the Scheme", want)
		}
	}
}
