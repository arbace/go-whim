package graph

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// LocalOut against crefactor/xform's text step of that name (Step6): the
// text step's own sample -- each shape it rewrites and each reason it holds
// a parameter back -- printed canonically, run through the text step and
// printed again, recorded below before the text step was deleted.  The
// graph's step, on the sample's graph read back from its Lisp, gives that
// C byte for byte, with the text's counts, a graph that is the import of
// its C view, nothing left untyped, and a program that prints what the
// sample printed.

const s6loIn = `int printf(const char *, ...);

typedef unsigned char char_u;

static int gv;

static int *kept;

static void split(int v, int *q, int *r);

static void advance(char_u **);

    static void
split(int v, int *q, int *r)
{
    *q = v / 10;
    *r = v % 10;
}

    static void
bump(int *n)
{
    *n += 1;
}

    static int
maybe_len(const char *s, int *lenp)
{
    int n = 0;
    while (s[n])
    {
        n++;
    }
    if (lenp != 0)
    {
        *lenp = n;
    }
    return n > 3;
}

    static int
pick(int a, int *out)
{
    if (!out)
    {
        return -1;
    }
    if (a > 2)
    {
        *out = a * 2;
        return 1;
    }
    return 0;
}

    static char_u *
skip(char_u *p, int *count)
{
    while (*p == ' ')
    {
        p++;
        (*count)++;
    }
    return p;
}

    static void
advance(char_u **pp)
{
    (*pp)++;
}

    static void
keep(int *p)
{
    kept = p;
}

    static void
twice(int *n)
{
    bump(n);
    bump(n);
}

    static void
global_bump(int *n)
{
    *n += 5;
}

    static int
both(int *a, int *b)
{
    *a = 1;
    *b = 2;
    return 3;
}

    static void
early(int k, int *o)
{
    if (k)
    {
        return;
    }
    *o = 7;
}

    static void
sink(int *o)
{
    *o = 9;
}

    static int
setv(int k, int *o)
{
    *o = k * 2;
    return k;
}

    int
main(void)
{
    int q;
    int r;
    int len = -1;
    int n = 5;
    int m = 0;
    int k = 0;
    int h = 0;
    int cnt = 0;
    int a;
    int b;
    int e = 0;
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
    {
        printf("%d %d %d\n", a, b, e);
    }
    int kept_in = 4;
    int unread = 0;
    early(1, &kept_in);
    early(0, &kept_in);
    sink(&unread);
    printf("%d\n", kept_in);
    int kv = 1;
    int kr = 0;
    kr = setv(2, &kv);
    kv = 5;
    printf("%d %d\n", kr, kv);
    return 0;
}
`

const s6loWant = `int printf(const char *, ...);

typedef unsigned char char_u;

static int gv;

static int *kept;

typedef struct
{
    int q;
    int r;
} split__out_T;

static split__out_T split(int v);

static char_u *advance(char_u *);

    static split__out_T
split(int v)
{
    int q;
    int r;
    split__out_T out__;
    q = v / 10;
    r = v % 10;
    {
        out__.q = q;
        out__.r = r;
        return out__;
    }
}

    static void
bump(int *n)
{
    *n += 1;
}

    static int
maybe_len(const char *s, int *lenp)
{
    int n = 0;
    while (s[n])
    {
        n++;
    }
    if (lenp != 0)
    {
        *lenp = n;
    }
    return n > 3;
}

    static int
pick(int a, int *out)
{
    if (!out)
    {
        return -1;
    }
    if (a > 2)
    {
        *out = a * 2;
        return 1;
    }
    return 0;
}

typedef struct
{
    char_u *r__;
    int count;
} skip__out_T;

    static skip__out_T
skip(char_u *p, int count)
{
    skip__out_T out__;
    while (*p == ' ')
    {
        p++;
        (count)++;
    }
    {
        out__.r__ = (p);
        out__.count = count;
        return out__;
    }
}

    static char_u *
advance(char_u *pp)
{
    (pp)++;
    return pp;
}

    static void
keep(int *p)
{
    kept = p;
}

    static void
twice(int *n)
{
    bump(n);
    bump(n);
}

    static void
global_bump(int *n)
{
    *n += 5;
}

typedef struct
{
    int r__;
    int a;
    int b;
} both__out_T;

    static both__out_T
both(void)
{
    int a;
    int b;
    both__out_T out__;
    a = 1;
    b = 2;
    {
        out__.r__ = (3);
        out__.a = a;
        out__.b = b;
        return out__;
    }
}

    static int
early(int k, int o)
{
    if (k)
    {
        return o;
    }
    o = 7;
    return o;
}

    static int
sink(void)
{
    int o;
    o = 9;
    return o;
}

typedef struct
{
    int r__;
    int o;
} setv__out_T;

    static setv__out_T
setv(int k)
{
    int o;
    setv__out_T out__;
    o = k * 2;
    {
        out__.r__ = (k);
        out__.o = o;
        return out__;
    }
}

    int
main(void)
{
    both__out_T both__o;
    setv__out_T setv__o;
    skip__out_T skip__o;
    split__out_T split__o;
    int q;
    int r;
    int len = -1;
    int n = 5;
    int m = 0;
    int k = 0;
    int h = 0;
    int cnt = 0;
    int a;
    int b;
    int e = 0;
    char_u buf[] = "  word";
    char_u *w = buf;
    (split__o = split(47), q = split__o.q, r = split__o.r, (void)0);
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
    w = (skip__o = skip(w, cnt), cnt = skip__o.count, skip__o.r__);
    printf("%s %d\n", (char *)w, cnt);
    (w = advance(w));
    printf("%s\n", (char *)w);
    keep(&k);
    twice(&h);
    global_bump(&gv);
    printf("%d %d\n", h, gv);
    if ((both__o = both(), a = both__o.a, b = both__o.b, both__o.r__) == 3 && (e = a + b) > 0)
    {
        printf("%d %d %d\n", a, b, e);
    }
    int kept_in = 4;
    int unread = 0;
    (kept_in = early(1, kept_in));
    (kept_in = early(0, kept_in));
    sink();
    printf("%d\n", kept_in);
    int kv = 1;
    int kr = 0;
    kr = (setv__o = setv(2), setv__o.r__);
    kv = 5;
    printf("%d %d\n", kr, kv);
    return 0;
}
`

func TestLocalOutAsTheText(t *testing.T) {
	g, _, err := Import("/x/t.c", []byte(s6loIn))
	if err != nil {
		t.Fatal(err)
	}
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	st, err := e.LocalOut()
	if err != nil {
		t.Fatal(err)
	}
	if st != (LocalOutStats{Params: 9, Funcs: 7, DeadIn: 6}) {
		t.Errorf("%+v, the text's 9 parameters of 7 functions, 6 taking no value in", st)
	}
	got, err := h.C()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != s6loWant {
		t.Fatalf("not the text step's C:\n%s", got)
	}
	if len(e.Untyped) != 0 {
		t.Errorf("%d left untyped: %s", len(e.Untyped), Lisp(e.Untyped[0]))
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	i, _, err := Import("/x/t.c", got)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(h, i); err != nil {
		t.Fatalf("not the import of its C view: %v", err)
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	if a, b := s6loRun(t, s6loIn), s6loRun(t, string(got)); a != b {
		t.Errorf("the program printed\n%s\nand after the step\n%s", a, b)
	}
}

// TestLocalOutNothing: a file with no out-parameter is left as it is.
func TestLocalOutNothing(t *testing.T) {
	src := "static int f(int a)\n{\n    return a;\n}\n\n    int\nmain(void)\n{\n    return f(0);\n}\n"
	g, _, err := Import("/x/t.c", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	st, err := e.LocalOut()
	if err != nil || st.Params != 0 || len(e.Log) != 0 {
		t.Fatalf("%+v %v: %d acts", st, err, len(e.Log))
	}
}

// s6loRun is what a C program prints.
func s6loRun(t *testing.T, src string) string {
	dir := t.TempDir()
	c := filepath.Join(dir, "p.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "p")
	if o, err := exec.Command("gcc", "-w", "-o", bin, c).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s\n%s", err, o, src)
	}
	var out bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// TestRetypeKeywordTypedef: a type form naming `usize`, a word BUILD's
// keywords list, is the file's typedef of that name where it has one (a
// parameter `usize *p` retyped `usize`, as LocalOut does on whim-vim.c).
func TestRetypeKeywordTypedef(t *testing.T) {
	src := "typedef typeof(sizeof 0) usize;\nstatic int f(usize *p)\n{\n    *p = 1;\n    return 0;\n}\n"
	g, _, err := Import("/x/t.c", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	p := paramElems(defType(e.Defn("f")))[0]
	if _, err := e.Retype(p, "usize"); err != nil {
		t.Fatal(err)
	}
	if p.Type == nil || p.Type != g.Forms[0].Type {
		t.Fatalf("the parameter is not typed as the typedef")
	}
}
