package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Java backend on C that is not vim: each program's functions are
// translated to one Java class, compiled with the runtime (jeditor/rt) and
// run, and must print what the gcc-built C prints.  The C calls two host
// functions it only declares -- out(v) prints a number, outs(s) a string --
// which the C harness defines with printf and the Java one as the abstract
// class's methods: the host behind a line, as the editor's is.

// javaHarnessC is the C side of the host, and main.
const javaHarnessC = `#include <stdio.h>
#include <stdarg.h>
void run(void);
void out(long long v) { printf("%lld\n", v); }
void outs(const char *s) { printf("%s\n", s); }
void outf(const char *fmt, ...) { va_list ap; va_start(ap, fmt); vprintf(fmt, ap); va_end(ap); printf("\n"); }
int main(void) { run(); return 0; }
`

// javaHarness is the Java side: the host as the class's two methods, and main.
const javaHarness = `import whim.rt.*;

public class Main {
    public static void main(String[] args) {
        new Prog() {
            void out(long v) { System.out.println(v); }
            void outs(BytePtr s) { System.out.println(BytePtr.str(s)); }
            // a printf of d, u, x, c, s and l, reading the arguments as the
            // backend boxes them: an int an Integer, an unsigned int or a
            // long a Long, a pointer its class
            void outf(BytePtr fmt, Object... args) {
                StringBuilder b = new StringBuilder();
                String f = BytePtr.str(fmt);
                int k = 0;
                for (int i = 0; i < f.length(); i++) {
                    char c = f.charAt(i);
                    if (c != '%') {
                        b.append(c);
                        continue;
                    }
                    int l = 0;
                    while (f.charAt(++i) == 'l') {
                        l++;
                    }
                    char conv = f.charAt(i);
                    Object a = conv == '%' ? null : args[k++];
                    long v = conv == 's' || conv == '%' ? 0 : ((Number) a).longValue();
                    switch (conv) {
                    case 'd': b.append(l > 0 ? v : (int) v); break;
                    case 'u': b.append(l > 0 ? Long.toUnsignedString(v) : Integer.toUnsignedString((int) v)); break;
                    case 'x': b.append(l > 0 ? Long.toHexString(v) : Integer.toHexString((int) v)); break;
                    case 'c': b.append((char) (v & 0xff)); break;
                    case 's': b.append(BytePtr.str((BytePtr) a)); break;
                    case '%': b.append('%'); break;
                    default: throw new IllegalArgumentException(f);
                    }
                }
                if (k != args.length) {
                    throw new IllegalArgumentException("unused arguments: " + f);
                }
                System.out.println(b);
            }
        }.run();
    }
}
`

const javaHost = `void out(long long v);
void outs(const char *s);
`

// The integer types, signed and unsigned: C's conversions and arithmetic on
// Java's bits.
const javaIntsC = javaHost + `
typedef unsigned char uchar;
static unsigned int ucounter = 4294967290u;
static long long big = -5;
static unsigned short us = 65535;
static signed char sc = -3;
static _Bool flag;

unsigned int uadd(unsigned int a, unsigned int b) { return a + b; }
unsigned int udiv(unsigned int a, unsigned int b) { return a / b; }
unsigned int umod(unsigned int a, unsigned int b) { return a % b; }
unsigned long long uldiv(unsigned long long a, unsigned long long b) { return a / b; }
int ucmp(unsigned int a, int b) { return a < b; }
int ulcmp(unsigned long a, unsigned long b) { return a > b; }
unsigned int ushr(unsigned int a, int n) { return a >> n; }
int sshr(int a, int n) { return a >> n; }
unsigned long ulshr(unsigned long a) { return a >> 60; }
int widen_uchar(uchar c) { return c + 1; }
int widen_char(char c) { return c + 1; }
long long widen_uint(unsigned int u) { return u; }
long long widen_int(int i) { return i; }
unsigned long long widen_neg(int i) { return i; }
uchar narrow(int i) { return (uchar)i; }
short narrows(long l) { return (short)l; }
_Bool truth(int x) { return x; }
int btoi(_Bool b) { return b + 1; }

int sw(unsigned char c)
{
    switch (c)
    {
    case 200:
        return 1;
    case 255:
        {
            int z = 3;
            return z;
        }
    case 'a':
    default:
        break;
    }
    return 0;
}

void ptrops(unsigned char *p, unsigned int *q, short *r)
{
    p[0] += 100;
    *p >>= 1;
    p[1] *= 3;
    p[2]++;
    ++*p;
    q[0] /= 3;
    *q %= 7;
    q[1] -= 1;
    q[1] >>= 4;
    *r -= 2;
    r[1] /= -2;
}

void run(void)
{
    out(uadd(4294967295u, 2));
    out(udiv(4294967295u, 2));
    out(udiv(-6, 4));
    out(umod(-1, 10));
    out(uldiv(-1ull, 3));
    out(ucmp(1, -1));
    out(ulcmp(-1ul, 1));
    out(ushr(0x80000000u, 31));
    out(sshr(-8, 1));
    out(ulshr(-1ul));
    out(widen_uchar(255));
    out(widen_char(-1));
    out(widen_uint(4294967295u));
    out(widen_int(-1));
    out((long long)widen_neg(-1));
    out(narrow(257));
    out(narrow(-1));
    out(narrows(70000));
    out(truth(5));
    out(truth(0));
    out(btoi(truth(7)));
    ucounter += 10;
    out(ucounter);
    ucounter -= 11;
    out(ucounter);
    ucounter /= 2;
    out(ucounter);
    ucounter >>= 3;
    out(ucounter);
    us += 3;
    out(us);
    us--;
    out(us);
    us -= 5;
    out(us);
    us /= 3;
    out(us);
    unsigned char c = 250;
    c += 10;
    out(c);
    c >>= 1;
    out(c);
    c = 200;
    c /= 3;
    out(c);
    c = 255;
    c++;
    out(c);
    out(sc * 100);
    sc = 200;
    out(sc);
    big *= 1000000007;
    big <<= 4;
    out(big);
    big = -7;
    out(big / 2);
    out(big % 2);
    out((unsigned int)big);
    out(-(unsigned int)1 > 5);
    out(~0u);
    out(~(uchar)0);
    out(sizeof(long) + sizeof(short));
    int i = 7;
    out(i++ + ++i);
    out(i--);
    out(--i);
    int x = (i > 3) + (i == 7) * 2;
    out(x);
    x = i > 3 && i < 100 || i == -1;
    out(x);
    out(!x);
    out(!!i);
    flag = i;
    out(flag);
    flag = 0;
    out(flag ? 11 : 22);
    out(i ? i : -1);
    unsigned int u = 3;
    int neg = -1;
    out(u > neg);
    out((long)u * neg);
    out(u * neg);
    out(1 << 31);
    out(1u << 31);
    out(0x7fffffff + 1u);
    out('a' + 1);
    out((char)200);
    out((unsigned char)-56);
    int a, b;
    a = b = 5;
    out(a + b);
    a += b *= 2;
    out(a);
    out(b);
    a = (b = 3, b + 1);
    out(a);
    out(sw(200));
    out(sw(255));
    out(sw('a'));
    out(sw(-56));
    unsigned char ub[3] = { 200, 100, 255 };
    unsigned int uq[2] = { 4000000000u, 0 };
    short sh[2] = { -32767, 7 };
    ptrops(ub, uq, sh);
    out(ub[0]);
    out(ub[1]);
    out(ub[2]);
    out(uq[0]);
    out(uq[1]);
    out(sh[0]);
    out(sh[1]);
}
`

// Control flow: if, while, do, for, switch with fallthrough and default,
// break and continue, conditions that do something, and loops that never end
// but by a return.
const javaFlowC = javaHost + `
enum color { RED, GREEN = 5, BLUE };

int classify(int n)
{
    int r = 0;
    switch (n)
    {
    case 0:
        r += 1;
    case 1:
        r += 10;
        break;
    case GREEN:
    case 6:
        r = 100;
        break;
    default:
        r = -1;
    case 9:
        r += 1000;
    }
    return r;
}

const char *name(enum color c)
{
    switch (c)
    {
    case RED:
        return "red";
    case GREEN:
        return "green";
    default:
        return "other";
    }
}

int loops(int n)
{
    int t = 0;
    int i = 0;
    while (i < n)
    {
        i++;
        if (i % 3 == 0)
            continue;
        if (i > 20)
            break;
        t += i;
    }
    do
    {
        t -= 1;
        if (t % 5 == 0)
            continue;
        t -= 2;
    }
    while (t > 10);
    for (int k = 0, m = 10; k < m; k++, m--)
    {
        if (k == 2)
            continue;
        t += k * m;
    }
    for (;;)
    {
        if (t > 1000)
            break;
        t *= 2;
    }
    return t;
}

int first_over(int *v, int n, int lim)
{
    int i;
    for (i = 0; i < n; i++)
        if (v[i] > lim)
            return i;
    return -1;
}

int forever(int n)
{
    while (1)
    {
        if (n > 50)
            return n;
        n += 7;
    }
}

int once(int a)
{
    int r = 0;
    do
    {
        if (a < 0)
            break;
        r = a * 2;
        if (a == 3)
            continue;
        r += 1;
    }
    while (0);
    return r;
}

static int calls;
int next(void) { return ++calls; }

int sideeffects(int n)
{
    int t = 0;
    int k;
    while ((k = next()) < n)
        t += k;
    do
        t++;
    while ((k = next()) % 4 != 0);
    for (int j = 0; j < 3 && (k = next()) > 0; j += k > 10 ? 2 : 1)
    {
        if (j == 1)
            continue;
        t += j;
    }
    if (n > 3 && next() > 0)
        t += 100;
    return t + calls;
}

void run(void)
{
    for (int n = -1; n < 11; n++)
        out(classify(n));
    outs(name(RED));
    outs(name(GREEN));
    outs(name(BLUE));
    for (int n = 0; n < 30; n += 7)
        out(loops(n));
    int v[5] = { 3, 9, 2, 12, 5 };
    out(first_over(v, 5, 8));
    out(first_over(v, 5, 20));
    out(forever(3));
    out(once(-1));
    out(once(3));
    out(once(4));
    out(sideeffects(5));
    out(sideeffects(2));
}
`

// C strings and arrays: literals, walking, comparing, a char array written
// through a pointer, pointer difference, an array of pointers.
const javaStringsC = javaHost + `
static char buf[32];
static const char *words[] = { "alpha", "beta", "", "gamma" };
static int counts[4] = { 1, 2, 3 };
static char greeting[] = "hi there";

int len(const char *s)
{
    const char *e = s;
    while (*e)
        e++;
    return e - s;
}

int cmp(const char *a, const char *b)
{
    while (*a && *a == *b)
    {
        a++;
        b++;
    }
    return (unsigned char)*a - (unsigned char)*b;
}

char *copy(char *d, const char *s)
{
    char *r = d;
    while ((*d++ = *s++) != 0)
        ;
    return r;
}

int sum(const char *s)
{
    int n = 0;
    while (*s)
        n += *s++ - '0';
    return n;
}

char *find(char *s, int c)
{
    for (; *s; ++s)
        if (*s == c)
            return s;
    return 0;
}

void upper(char *s)
{
    for (int i = 0; s[i] != '\0'; i++)
        if (s[i] >= 'a' && s[i] <= 'z')
            s[i] -= 32;
}

int highbytes(const unsigned char *p, int n)
{
    int k = 0;
    for (int i = 0; i < n; i++)
        if (p[i] >= 0x80)
            k++;
    return k;
}

void run(void)
{
    out(len("hello"));
    out(len(""));
    out(cmp("abc", "abd"));
    out(cmp("abc", "abc"));
    out(cmp("\xff", "a"));
    copy(buf, "copied");
    outs(buf);
    out(len(buf));
    out(sum("12345"));
    char *p = find(buf, 'p');
    out(p - buf);
    outs(p);
    out(find(buf, 'z') == 0);
    upper(buf + 2);
    outs(buf);
    for (int i = 0; i < 4; i++)
    {
        out(len(words[i]));
        out(counts[i]);
    }
    outs(greeting);
    out(sizeof(greeting));
    greeting[2] = '_';
    outs(greeting);
    out(highbytes((const unsigned char *)"a\x80\xff", 3));
    char local[8] = "ab";
    out(local[5]);
    local[5] = 'x';
    out(len(local));
    const char *q = "xyz";
    out(q[1]);
    out(*(q + 2));
    out(q == q + 0);
    out(q + 1 > q);
    char *w = buf;
    w += 3;
    w -= 1;
    out(*w);
    out(w[-1]);
}
`

// Structs: copies by assignment, nested structs and arrays made with their
// struct, pointers to structs plain and walking, an address-taken local.
const javaStructsC = javaHost + `
typedef struct inner { int a; char tag[4]; } inner_T;
typedef struct outer {
    int n;
    inner_T in;
    inner_T many[3];
    struct outer *next;
    const char *name;
} outer_T;

static outer_T g1 = { 7, { 1, "ab" }, { { 2, "c" } }, 0, "g1" };
static outer_T g2;
static outer_T table[4];

int total(outer_T o)
{
    o.n += 1000;
    return o.n + o.in.a + o.many[0].a;
}

outer_T make(int n)
{
    outer_T o = { n };
    o.in.a = n * 2;
    o.name = "made";
    return o;
}

int walk(outer_T *p, int n)
{
    int t = 0;
    for (outer_T *e = p + n; p < e; p++)
        t += p->n;
    return t;
}

void bump(int *p) { *p += 1; }
void setp(const char **pp) { *pp = "set"; }

int chain(outer_T *o)
{
    int k = 0;
    for (; o != 0; o = o->next)
        k += o->n;
    return k;
}

void run(void)
{
    g2 = g1;
    g2.n = 8;
    g2.in.a = 9;
    g2.in.tag[0] = 'z';
    g2.many[0].a = 20;
    out(g1.n);
    out(g1.in.a);
    outs(g1.in.tag);
    out(g1.many[0].a);
    out(g2.n);
    out(g2.in.a);
    outs(g2.in.tag);
    out(g2.many[0].a);
    outs(g2.name);
    out(total(g1));
    out(g1.n);
    outer_T m = make(5);
    out(m.n + m.in.a);
    outs(m.name);
    for (int i = 0; i < 4; i++)
        table[i].n = i * i;
    out(walk(table, 4));
    out(walk(&table[1], 2));
    int x = 41;
    bump(&x);
    bump(&x);
    out(x);
    const char *s = "unset";
    setp(&s);
    outs(s);
    g1.next = &g2;
    g2.next = &m;
    m.next = 0;
    out(chain(&g1));
    inner_T a = { 5, "q" }, b;
    b = a;
    a.a = 6;
    out(a.a * 10 + b.a);
    outer_T *p = &g2;
    p->in = a;
    out(g2.in.a);
    *p = g1;
    out(g2.n);
}
`

// What a profile tells the backend: the allocators, whose storage the
// garbage collector owns, the frees, which are nothing, and the functions of
// bytes.  With them: a list of structs made one at a time, an array of
// structs and one of pointers walked, a global and a parameter whose
// addresses are taken.
const javaProfileC = javaHost + `
typedef unsigned long size_t;
void *alloc(size_t n);
void vim_free(void *p);
void *memmove(void *d, const void *s, size_t n);
void *memset(void *d, int c, size_t n);
int memcmp(const void *a, const void *b, size_t n);

typedef struct node { int v; struct node *next; } node_T;
typedef struct item { int k; char name[8]; } item_T;

static int hits;

node_T *push(node_T *head, int v)
{
    node_T *n = alloc(sizeof(node_T));
    n->v = v;
    n->next = head;
    return n;
}

int sum_list(node_T *n)
{
    int s = 0;
    while (n != 0)
    {
        node_T *next = n->next;
        s += n->v;
        vim_free(n);
        n = next;
    }
    return s;
}

int count(int *where, int k)
{
    *where += k;
    hits++;
    return *where;
}

int twice(int k)
{
    int r = count(&k, k);
    return r + k;
}

void run(void)
{
    node_T *l = 0;
    for (int i = 1; i <= 4; i++)
        l = push(l, i * i);
    out(sum_list(l));
    item_T *items = (item_T *)alloc(3 * sizeof(item_T));
    for (int i = 0; i < 3; i++)
    {
        items[i].k = 10 - i;
        memmove(items[i].name, "item", 5);
        items[i].name[4] = '0' + i;
    }
    for (item_T *p = items; p < items + 3; ++p)
    {
        out(p->k);
        outs(p->name);
    }
    char *s = (char *)alloc(16);
    memset(s, 'x', 3);
    out(memcmp(s, "xxx", 3) == 0);
    out(memcmp(s, "xxy", 3) < 0);
    out(s[3]);
    char **names = (char **)alloc(3 * sizeof(char *));
    names[0] = s;
    names[1] = items[1].name;
    names[2] = 0;
    int n = 0;
    for (char **pp = names; *pp != 0; pp++)
        n += (*pp)[0];
    out(n);
    out(count(&hits, 5));
    out(hits);
    out(twice(7));
    vim_free(items);
}
`

func TestJavaProfile(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(javaProfileC), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Prog.java")
	prof := Profile{Allocators: []string{"alloc"}, Frees: []string{"vim_free"},
		Bytes: ByteFuncs{Move: []string{"memmove"}, Set: "memset", Cmp: "memcmp"}}
	if rc := Run([]string{c, dir, "-java", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, _ := os.ReadFile(out)
	prog := string(b)
	if r, _ := os.ReadFile(out + ".refused"); len(r) > 0 {
		t.Fatalf("refused:\n%s\n%s", r, numbered(prog))
	}
	h := filepath.Join(dir, "harness.c")
	harness := "#include <stdlib.h>\nvoid *alloc(unsigned long n) { return calloc(1, n); }\nvoid vim_free(void *p) { free(p); }\n" + javaHarnessC
	if err := os.WriteFile(h, []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "prog")
	if o, err := exec.Command("gcc", "-w", "-O0", "-o", exe, c, h).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, o)
	}
	want, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := javaOutput(t, dir, prog); got != string(want) {
		t.Errorf("the Java prints\n%s\nthe C\n%s\n%s", diffLines(got, string(want)), want, numbered(prog))
	}
	for _, w := range []string{"new S_node()", "S_item.array(3)", "Rt.memmove(", "int[] hits = new int[1];", "int[] k = new int[] {k_arg};"} {
		if !strings.Contains(prog, w) {
			t.Errorf("no %q in:\n%s", w, numbered(prog))
		}
	}
}

func requireTools(t *testing.T) {
	for _, tool := range []string{"gcc", "javac", "java"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	if _, err := os.Stat(runtimeDir); err != nil {
		t.Skipf("no Java runtime at %s", runtimeDir)
	}
}

// runtimeDir is the Java runtime, jeditor/rt, from crefactor/togo.
var runtimeDir = filepath.Join("..", "..", "jeditor", "rt")

// javaProgram translates src (and nothing else) to Prog.java and returns it,
// with the refusals.
func javaProgram(t *testing.T, dir, src string) (string, string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Prog.java")
	if rc := Run([]string{c, dir, "-java", out}, io.Discard, Profile{}); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.ReadFile(out + ".refused")
	return string(b), string(r)
}

// cOutput is what the gcc-built program prints.
func cOutput(t *testing.T, dir, src string) string {
	c := filepath.Join(dir, "prog.c")
	h := filepath.Join(dir, "harness.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h, []byte(javaHarnessC), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "prog")
	if o, err := exec.Command("gcc", "-w", "-O0", "-o", exe, c, h).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, o)
	}
	b, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// javaOutput compiles prog, the runtime and the harness, runs it and returns
// what it prints.
func javaOutput(t *testing.T, dir, prog string) string {
	jdir := filepath.Join(dir, "java")
	os.RemoveAll(jdir)
	if err := os.MkdirAll(jdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jdir, "Prog.java"), []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jdir, "Main.java"), []byte(javaHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, _ := filepath.Glob(filepath.Join(runtimeDir, "*.java"))
	args := append([]string{"-nowarn", "-d", filepath.Join(jdir, "classes"), filepath.Join(jdir, "Prog.java"), filepath.Join(jdir, "Main.java")}, rt...)
	if o, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s\n%s", err, o, numbered(prog))
	}
	o, err := exec.Command("java", "-cp", filepath.Join(jdir, "classes"), "Main").CombinedOutput()
	if err != nil {
		t.Fatalf("java: %v\n%s\n%s", err, o, numbered(prog))
	}
	return string(o)
}

func numbered(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(strings.Repeat(" ", 4-len(itoa(i+1)))+itoa(i+1)+"  "+lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

// javaSame translates src, requires every function written, and requires
// the Java to print what the C prints; it returns the Java.
func javaSame(t *testing.T, src string) string {
	requireTools(t)
	dir := t.TempDir()
	prog, refused := javaProgram(t, dir, src)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutput(t, dir, src)
	if got := javaOutput(t, dir, prog); got != want {
		t.Errorf("the Java prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(prog))
	}
	return prog
}

// diffLines names the first line where two outputs differ.
func diffLines(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return "line " + itoa(i+1) + ": " + g[i] + " (the C: " + w[i] + ")"
		}
	}
	return "a different number of lines"
}

func TestJavaIntegers(t *testing.T) { javaSame(t, javaIntsC) }
func TestJavaFlow(t *testing.T)     { javaSame(t, javaFlowC) }
func TestJavaStrings(t *testing.T)  { javaSame(t, javaStringsC) }
func TestJavaStructs(t *testing.T)  { javaSame(t, javaStructsC) }

// The control: the comparison sees a translation that is wrong.  Each
// mutation undoes one rule -- unsigned division as Java's signed one, an
// unsigned shift as a signed one, an unsigned char widened with its sign, a
// struct assigned by reference instead of copied -- and each must move the
// output.
func TestJavaControl(t *testing.T) {
	requireTools(t)
	for _, c := range []struct {
		name, src string
		from      *regexp.Regexp
		repl      string
	}{
		{"unsigned division", javaIntsC, regexp.MustCompile(`Integer\.divideUnsigned\(([^,]+), ([^)]+)\)`), "($1 / $2)"},
		{"unsigned shift", javaIntsC, regexp.MustCompile(`>>>`), ">>"},
		{"unsigned widening", javaIntsC, regexp.MustCompile(`\(c & 0xff\) \+ 1`), "c + 1"},
		{"struct copy", javaStructsC, regexp.MustCompile(`g2\.set\(g1\)`), "g2 = g1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			prog, _ := javaProgram(t, dir, c.src)
			wrong := c.from.ReplaceAllString(prog, c.repl)
			if wrong == prog {
				t.Fatalf("the control changed nothing:\n%s", numbered(prog))
			}
			if c.name == "struct copy" {
				// g2 is a field, final: the wrong translation makes it a
				// reference that can be moved
				wrong = strings.Replace(wrong, "final S_outer g2 = ", "S_outer g2 = ", 1)
			}
			want := cOutput(t, dir, c.src)
			if javaOutput(t, dir, wrong) == want {
				t.Errorf("the control: the wrong translation prints what the C prints")
			}
		})
	}
}

// The runtime's own test: jeditor/rt/SelfTest.java.
func TestJavaRuntime(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	rt, _ := filepath.Glob(filepath.Join(runtimeDir, "*.java"))
	if o, err := exec.Command("javac", append([]string{"-Xlint:all", "-Werror", "-d", dir}, rt...)...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, o)
	}
	o, err := exec.Command("java", "-cp", dir, "whim.rt.SelfTest").CombinedOutput()
	if err != nil || strings.TrimSpace(string(o)) != "ok" {
		t.Fatalf("the runtime's self-test: %v\n%s", err, o)
	}
}

// What is refused is refused per function, with its reason, and the class
// still compiles: the function is a stub that throws.  What is left to
// refuse after milestone 2 is what the core does not do: floating point, a
// variadic function of its own, and a goto that is no forward jump to a
// label of a block holding it.
func TestJavaRefuses(t *testing.T) {
	requireTools(t)
	const src = javaHost + `
int half(int x) { double d = x; return d / 2; }
int first(int n, ...) { return n; }
int back(int x) { again: x--; if (x > 0) goto again; return x; }
int fine(int x) { return x + 1; }
void run(void) { out(fine(1)); }
`
	dir := t.TempDir()
	prog, refused := javaProgram(t, dir, src)
	for _, want := range []string{"half: floating point", "first: a variadic function", "back: a goto that is no forward jump"} {
		if !strings.Contains(refused, want) {
			t.Errorf("no %q in the refusals:\n%s", want, refused)
		}
	}
	if strings.Contains(refused, "fine:") || strings.Contains(refused, "run:") {
		t.Errorf("a function refused that is not:\n%s", refused)
	}
	if got := javaOutput(t, dir, prog); got != "2\n" {
		t.Errorf("the Java prints %q", got)
	}
}

// --- milestone 2: the constructs the core needs beyond the slice ------------

// A variadic call: the arguments past the parameters boxed as C promotes
// them -- a char, short or int an Integer, an unsigned int zero-extended to
// a Long, a long a Long, a pointer its class, an array the pointer it
// decays to -- and read back by the host's printf.
const javaVarargsC = javaHost + `void outf(const char *fmt, ...);
static const char *names[] = { "zero", "one" };

void run(void)
{
    unsigned char uc = 200;
    signed char sc = -5;
    short sh = -300;
    unsigned short us = 65000;
    unsigned int u = 4000000000u;
    long l = -1234567890123L;
    unsigned long ul = 18000000000000000000ul;
    char buf[8] = "arr";
    _Bool b = 1;
    outf("%d %d %d %d", uc, sc, sh, us);
    outf("%u %d %x", u, (int)u, u);
    outf("%ld %lu %lx", l, ul, ul);
    outf("%s %s %s", "lit", names[1], buf);
    outf("%c%c%%", 'o', 'k');
    outf("%d %d %d", b, uc > 100, 3 > 2 ? 7 : 8);
    outf("%d", -uc);
    outf("none");
}
`

func TestJavaVarargs(t *testing.T) { javaSame(t, javaVarargsC) }

// The growarray (Profile.GrowArray): a void * that holds storage typed where
// the C casts it -- ints, structs, pointers, bytes, and read uncast where
// it is converted -- grown by a rule of the runtime's (Profile.RuntimeBodies)
// and not by the C's byte copy; with it the functions of bytes on what is
// not bytes (ints, structs, pointers, a fill of 0xff through a struct of
// scalars, memcmp of two structs), a void * that is not the growarray, and
// compound literals.
const javaGrowC = javaHost + `
typedef unsigned long size_t;
void *alloc(size_t n);
void vim_free(void *p);
void *memmove(void *d, const void *s, size_t n);
void *memset(void *d, int c, size_t n);
int memcmp(const void *a, const void *b, size_t n);

typedef struct { int ga_len; int ga_maxlen; int ga_itemsize; int ga_growsize; void *ga_data; } garray_T;
typedef struct pos { long lnum; int col; } pos_T;
typedef struct item { int k; char name[8]; pos_T at; } item_T;
struct holder { char *p; int n; };

void ga_init(garray_T *gap, int itemsize, int growsize)
{
    gap->ga_data = 0;
    gap->ga_maxlen = 0;
    gap->ga_len = 0;
    gap->ga_itemsize = itemsize;
    gap->ga_growsize = growsize;
}

int ga_grow_inner(garray_T *gap, int n)
{
    if (n < gap->ga_growsize)
        n = gap->ga_growsize;
    size_t new_len = (size_t)gap->ga_itemsize * (gap->ga_len + n);
    size_t old_len = (size_t)gap->ga_itemsize * gap->ga_maxlen;
    char *pp = alloc(new_len);
    if (gap->ga_data != 0)
        memmove(pp, gap->ga_data, old_len);
    vim_free(gap->ga_data);
    gap->ga_maxlen = gap->ga_len + n;
    gap->ga_data = pp;
    return 1;
}

int ga_grow(garray_T *gap, int n)
{
    if (gap->ga_maxlen - gap->ga_len < n)
        return ga_grow_inner(gap, n);
    return 1;
}

void ga_clear(garray_T *gap)
{
    vim_free(gap->ga_data);
    ga_init(gap, gap->ga_itemsize, gap->ga_growsize);
}

static garray_T ints, items, strs, bytes;

void run(void)
{
    ga_init(&ints, sizeof(int), 4);
    for (int i = 0; i < 10; i++)
    {
        ga_grow(&ints, 1);
        ((int *)ints.ga_data)[ints.ga_len++] = i * i;
    }
    int *ip = (int *)ints.ga_data;
    out(ip[9] + ip[3]);
    memmove(ip + 1, ip, 5 * sizeof(int));
    for (int i = 0; i < 7; i++)
        out(ip[i]);
    memset(ip, 0xff, 2 * sizeof(int));
    out(ip[0]);
    out(ip[2]);
    memset(ip, 1, sizeof(int));
    out(ip[0]);

    ga_init(&items, sizeof(item_T), 2);
    for (int i = 0; i < 5; i++)
    {
        ga_grow(&items, 1);
        item_T *it = (item_T *)items.ga_data + items.ga_len;
        it->k = i;
        memmove(it->name, "it", 3);
        it->name[2] = '0' + i;
        it->at.lnum = i * 10;
        items.ga_len++;
    }
    item_T *base = (item_T *)items.ga_data;
    memmove(base, base + 2, 3 * sizeof(item_T));
    base[0].k = 99;
    out(base[0].k);
    out(base[2].k);
    outs(base[0].name);
    out(base[0].at.lnum);
    base[2].k = 77;
    out(base[4].k);
    memmove(base + 1, base, 3 * sizeof(item_T));
    out(base[1].k);
    out(base[2].k);
    out(base[3].k);
    item_T one = base[4];
    out(memcmp(&one, &base[4], sizeof(item_T)) == 0);
    one.at.col = 5;
    out(memcmp(&one, &base[4], sizeof one) != 0);
    memset(&one, 0, sizeof(one));
    out(one.k + one.name[0] + one.at.lnum);
    pos_T ps[3];
    memset(ps, 0xff, sizeof(ps));
    out(ps[1].lnum);
    out(ps[2].col);
    memset(ps, 0, sizeof(pos_T) * 2);
    out(ps[1].lnum);
    out(ps[2].col);

    ga_init(&strs, sizeof(char *), 2);
    ga_grow(&strs, 3);
    ((char **)strs.ga_data)[0] = "a";
    ((char **)strs.ga_data)[1] = "bb";
    strs.ga_len = 2;
    char **sp = (char **)strs.ga_data;
    memmove(sp + 1, sp, sizeof(char *));
    outs(sp[1]);
    memset(sp, 0, 2 * sizeof(char *));
    out(sp[0] == 0 && sp[1] == 0);

    ga_init(&bytes, 1, 8);
    ga_grow(&bytes, 4);
    memmove((char *)bytes.ga_data, "abc", 4);
    bytes.ga_len = 3;
    outs(bytes.ga_data);
    char *s = bytes.ga_data;
    s[0] = 'A';
    outs((char *)bytes.ga_data);
    ga_clear(&bytes);
    out(bytes.ga_data == 0);

    void *vp = &base[1];
    item_T *back = (item_T *)vp;
    out(back->k);

    struct holder h = { (char [4]){ 'x', 'y', 0 }, 2 };
    outs(h.p);
    pos_T q = (pos_T){ 3, 4 };
    out(q.lnum + q.col);
}
`

// javaGrowProfile is what javaGrowC's program is told: its allocators,
// frees, functions of bytes and growarray, and ga_grow_inner's Java body.
var javaGrowProfile = Profile{Allocators: []string{"alloc"}, Frees: []string{"vim_free"},
	Bytes:     ByteFuncs{Move: []string{"memmove"}, Set: "memset", Cmp: "memcmp"},
	GrowArray: GrowArray{Data: "ga_data", MaxLen: "ga_maxlen"},
	RuntimeBodies: []RuntimeBody{{Name: "ga_grow_inner", Java: func(string) string {
		return "        if (n < gap.ga_growsize) {\n            n = gap.ga_growsize;\n        }\n" +
			"        gap.ga_maxlen = gap.ga_len + n;\n        return 1;\n"
	}}}}

// javaGrowHarnessC is the C side of javaGrowC's host: calloc and free.
const javaGrowHarnessC = "#include <stdlib.h>\nvoid *alloc(unsigned long n) { return calloc(1, n); }\nvoid vim_free(void *p) { free(p); }\n" + javaHarnessC

func TestJavaGrowArray(t *testing.T) {
	prog := javaSameWith(t, javaGrowC, javaGrowProfile, javaGrowHarnessC)
	for _, w := range []string{"GA_Ptr_S_item(", "GA_IntPtr(", "GA_BytePtr(", "Rt.moveStructs(", "Rt.memmove(", "Rt.fill(", "Rt.zero(", ".eq(", ".zero()"} {
		if !strings.Contains(prog, w) {
			t.Errorf("no %q in:\n%s", w, numbered(prog))
		}
	}
}

// Pointers to members, to functions, and unions: a scalar or pointer member
// whose address is taken is a one-element array its pointer is over (and a
// struct's copy copies its value, not the array); a function used as a value
// is one field per function and interface, so that two of them compare as
// C's do, called through the interface, adapted where the interface's
// classes are not the method's; a union is a class with every member a
// field, copied whole.
const javaPointersC = javaHost + `
typedef struct opts { int ts; long so; char *name; int flags[2]; } opts_T;
typedef struct win { opts_T o; struct win *next; int id; } win_T;
static win_T w1, w2;
static int gval = 3;

void setint(int *p, int v) { *p = v; }
void setstr(char **pp, char *s) { *pp = s; }
long *pick(win_T *wp, int local) { return local ? &wp->o.so : 0; }

struct tab { const char *name; int *var; };
static struct tab tabs[] = { { "ts", &w1.o.ts }, { "id", &w1.id }, { "g", &gval } };

typedef int (*binop_T)(int, int);
int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }
int mul(int a, int b) { return a * b; }
struct opdef { const char *name; binop_T fn; int (*alt)(int, int); };
static struct opdef ops[] = { { "add", add, sub }, { "sub", sub, 0 }, { "mul", mul, add } };
static binop_T current;
int apply(binop_T f, int a, int b) { return f(a, b); }
int apply2(int (*f)(int, int), int a, int b) { return (*f)(a, b); }
binop_T choose(int k) { return k ? mul : add; }
void visit(win_T *wp, void (*cb)(win_T *, int), int n)
{
    for (; wp != 0; wp = wp->next)
        cb(wp, n);
}
void bump(win_T *wp, int n) { wp->id += n; }

typedef union val { long number; int boolean; char *string; } val_T;
typedef struct optset
{
    int kind;
    val_T oldv;
    val_T newv;
    union
    {
        struct { short a, b; } pair;
        long whole;
    } u;
} optset_T;

long describe(optset_T *os)
{
    switch (os->kind)
    {
    case 0:
        return os->oldv.number + os->newv.number;
    case 1:
        return os->oldv.boolean * 10 + os->newv.boolean;
    }
    return os->newv.string[1];
}

void run(void)
{
    setint(&w1.o.ts, 8);
    setstr(&w1.o.name, "win1");
    *pick(&w1, 1) = 9;
    out(pick(&w1, 0) == 0);
    setint(&w1.o.flags[1], 4);
    out(w1.o.ts);
    out(w1.o.so);
    outs(w1.o.name);
    out(w1.o.flags[1]);
    w2 = w1;
    w2.o.ts = 1;
    int *tp = &w2.o.ts;
    *tp += 1;
    out(w1.o.ts);
    out(w2.o.ts);
    for (int i = 0; i < 3; i++)
    {
        *tabs[i].var += 100;
        outs(tabs[i].name);
    }
    out(w1.o.ts);
    out(w1.id);
    out(gval);

    current = add;
    out(current == add);
    out(current == ops[0].fn);
    out(current != ops[1].fn);
    out(ops[1].alt == 0);
    out(choose(1) == mul);
    out(choose(0) == ops[2].alt);
    for (int i = 0; i < 3; i++)
    {
        out(ops[i].fn(7, 3));
        if (ops[i].alt != 0)
            out(ops[i].alt(7, 3));
    }
    out(apply(sub, 10, 4));
    out(apply2(&mul, 6, 7));
    out(apply(choose(1), 2, 5));
    current = 0;
    out(current == 0);
    w1.next = &w2;
    visit(&w1, bump, 5);
    out(w1.id);
    out(w2.id);

    optset_T os = { 0 };
    os.oldv.number = 5;
    os.newv.number = 7;
    out(describe(&os));
    optset_T cp = os;
    cp.newv.number = 1;
    out(os.newv.number);
    out(describe(&cp));
    os.kind = 1;
    os.oldv.boolean = 1;
    os.newv.boolean = 0;
    out(describe(&os));
    os.u.pair.a = 3;
    os.u.pair.b = 4;
    out(os.u.pair.a + os.u.pair.b);
    val_T v = { .string = "str" };
    val_T n = { 42 };
    os.newv = v;
    os.kind = 2;
    out(describe(&os));
    out(n.number);
}
`

func TestJavaPointers(t *testing.T) { javaSame(t, javaPointersC) }

// goto: each a forward jump to a label of a block that holds it -- out of
// nested loops, out of a switch and a loop inside it, past a label to the
// next, into an empty statement at a loop's end, and two whose blocks would
// cross -- a labeled block the goto breaks.
const javaGotoC = javaHost + `
static int grid[3][4] = { { 1, 2, 3, 4 }, { 5, 6, 7, 8 }, { 9, 10, 11, 12 } };

int find(int v)
{
    int r = -1;
    for (int i = 0; i < 3; i++)
        for (int j = 0; j < 4; j++)
            if (grid[i][j] == v)
            {
                r = i * 10 + j;
                goto found;
            }
    out(-100);
    return r;
found:
    out(r);
    return r + 1000;
}

int classify(int n)
{
    int k = 0;
    switch (n)
    {
    case 1:
        k = 1;
        goto done;
    case 2:
        while (1)
        {
            k++;
            if (k > 5)
                goto done;
        }
    default:
        break;
    }
    k = -1;
done:
    return k;
}

int nested(int n)
{
    int t = 0;
    while (n-- > 0)
    {
        if (n == 7)
            goto skip;
        if (n % 2)
            goto odd;
        t += 1;
        goto next;
    odd:
        t += 100;
    next:
        t += 10;
    skip:
        ;
    }
    return t;
}

int crossing(int x)
{
    int r = 0;
    if (x > 5)
        goto la;
    r += 1;
    if (x > 2)
        goto lb;
    r += 2;
la:
    r += 4;
    if (x > 7)
        goto lb;
    r += 16;
lb:
    r += 8;
    return r;
}

void run(void)
{
    out(find(7));
    out(find(12));
    out(find(13));
    for (int n = 0; n < 4; n++)
        out(classify(n));
    out(nested(10));
    for (int x = 0; x < 10; x += 3)
        out(crossing(x));
}
`

func TestJavaGoto(t *testing.T) {
	prog := javaSame(t, javaGotoC)
	if strings.Contains(prog, "goto") || !strings.Contains(prog, "break L_found;") {
		t.Errorf("the gotos are not labeled blocks:\n%s", numbered(prog))
	}
}

// The control for milestone 2's constructs: one rule undone in each, which
// must move the output -- an unsigned char passed to printf with its sign,
// structs moved as references rather than copied, a member's address that
// copies it, a union copied without one of its members, a function pointer
// to another function, and a goto's break that leaves the wrong block.
func TestJavaControl2(t *testing.T) {
	requireTools(t)
	for _, c := range []struct {
		name, src string
		prof      Profile
		harness   string
		from      *regexp.Regexp
		repl      string
	}{
		{"variadic promotion", javaVarargsC, Profile{}, javaHarnessC, regexp.MustCompile(`\(uc & 0xff\)`), "uc"},
		{"struct move", javaGrowC, javaGrowProfile, javaGrowHarnessC, regexp.MustCompile(`Rt\.moveStructs\(`), "Rt.memmove("},
		{"member address", javaPointersC, Profile{}, javaHarnessC, regexp.MustCompile(`new IntPtr\(([\w.]+)\.ts, 0\)`), "new IntPtr(new int[] {$1.ts[0]}, 0)"},
		{"union copy", javaPointersC, Profile{}, javaHarnessC, regexp.MustCompile(`\n\s+number = o\$\.number;`), ""},
		{"function pointer", javaPointersC, Profile{}, javaHarnessC, regexp.MustCompile(`this::sub\b`), "this::mul"},
		{"goto", javaGotoC, Profile{}, javaHarnessC, regexp.MustCompile(`break L_next;`), "break L_skip;"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			prog, refused := javaProgramWith(t, dir, c.src, c.prof)
			if refused != "" {
				t.Fatalf("refused:\n%s", refused)
			}
			wrong := c.from.ReplaceAllString(prog, c.repl)
			if wrong == prog {
				t.Fatalf("the control changed nothing:\n%s", numbered(prog))
			}
			want := cOutputWith(t, dir, c.src, c.harness)
			if javaOutput(t, dir, prog) != want {
				t.Fatalf("the right translation does not print what the C prints")
			}
			if javaOutput(t, dir, wrong) == want {
				t.Errorf("the control: the wrong translation prints what the C prints")
			}
		})
	}
}

// javaProgramWith is javaProgram told a profile.
func javaProgramWith(t *testing.T, dir, src string, prof Profile) (string, string) {
	c := filepath.Join(dir, "prog.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "Prog.java")
	if rc := Run([]string{c, dir, "-java", out}, io.Discard, prof); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.ReadFile(out + ".refused")
	return string(b), string(r)
}

// cOutputWith is cOutput with the C side of the host given.
func cOutputWith(t *testing.T, dir, src, harness string) string {
	c := filepath.Join(dir, "prog.c")
	h := filepath.Join(dir, "harness.c")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h, []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "prog")
	if o, err := exec.Command("gcc", "-w", "-O0", "-o", exe, c, h).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, o)
	}
	b, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// javaSameWith is javaSame told a profile and the C side of the host.
func javaSameWith(t *testing.T, src string, prof Profile, harness string) string {
	requireTools(t)
	dir := t.TempDir()
	prog, refused := javaProgramWith(t, dir, src, prof)
	if refused != "" {
		t.Fatalf("refused:\n%s\n%s", refused, numbered(prog))
	}
	want := cOutputWith(t, dir, src, harness)
	if got := javaOutput(t, dir, prog); got != want {
		t.Errorf("the Java prints\n%s\nthe C\n%s\n%s", diffLines(got, want), want, numbered(prog))
	}
	return prog
}
