package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The struct locals of scalars made their members' locals
// (s6ss_structscalar.go), held to crefactor/xform's StructScalar text step
// on the same program, its output printed canonically (recorded here, the
// text step having gone): five structs taken of xform's own test and five
// more -- a copy between two taken structs, a struct copied into a table's
// element whose subscript reads a taken member, a self-copy, a chain of
// declarations each the other's value -- and one left whose value is a
// file-scope object, one returned, one whose address is taken, a static.
const s6ssSrc = `int printf(const char *, ...);
typedef long linenr_T;
typedef struct { linenr_T lnum; int col; } pos_T;
typedef struct { pos_T cur; int x; } win_T;
static win_T gw = {{7, 2}, 1};
static pos_T *kept;
static void move_pos(pos_T *pp, int dc) { pos_T a = *pp; a.col += dc; a.lnum++; *pp = a; }
static long key(void) { pos_T b = {3, 4}; pos_T c; c = b; c.col = c.col * 2; return c.lnum * 100 + c.col; }
static long partial(void) { pos_T z = {5}; return z.lnum + z.col; }
static int calls;
static int count(void) { return ++calls; }
static long wonly(pos_T *pp) { pos_T w = *pp; w.col = 3; w.col = count(); return w.lnum + calls; }
static pos_T ret(void) { pos_T r = {1, 1}; return r; }
static void keep(void) { static pos_T s; pos_T k = {9, 9}; kept = &k; s = k; }
static int byaddr(pos_T *p) { return p->col; }
static int passed(void) { pos_T q = {2, 3}; return byaddr(&q); }
static pos_T gp = {1, 2};
static int arr[10];
static pos_T ps[3];
static long fromglobal(void) { pos_T g = gp; return g.col; }
static long nested(pos_T *pp) { pos_T a = *pp; pos_T b = {a.lnum, 1}; arr[a.col % 10] = 1; ps[a.lnum % 3] = b; b = b; *pp = b; return b.lnum; }
static long chain(void) { pos_T x = {1, 2}; pos_T y = x; pos_T z; z = y; return z.lnum + y.col; }
int main(void)
{
    printf("%ld %ld %ld %ld\n", fromglobal(), nested(&gw.cur), chain(), ps[1].lnum);
    move_pos(&gw.cur, 3);
    printf("%ld %d\n", gw.cur.lnum, gw.cur.col);
    printf("%ld %ld %ld %d %ld\n", key(), partial(), ret().lnum, passed(), wonly(&gw.cur));
    keep();
    return 0;
}
`

const s6ssWant = `int printf(const char *, ...);

typedef long linenr_T;

typedef struct
{
    linenr_T lnum;
    int col;
} pos_T;

typedef struct
{
    pos_T cur;
    int x;
} win_T;

static win_T gw =
{
    {7, 2},
    1,
};

static pos_T *kept;

    static void
move_pos(pos_T *pp, int dc)
{
    typeof(((pos_T *)0)->lnum) a_lnum = (*pp).lnum;
    typeof(((pos_T *)0)->col) a_col = (*pp).col;
    a_col += dc;
    a_lnum++;
    {
        (*pp).lnum = a_lnum;
        (*pp).col = a_col;
    }
}

    static long
key(void)
{
    typeof(((pos_T *)0)->lnum) b_lnum = 3;
    typeof(((pos_T *)0)->col) b_col = 4;
    typeof(((pos_T *)0)->lnum) c_lnum;
    typeof(((pos_T *)0)->col) c_col;
    {
        c_lnum = b_lnum;
        c_col = b_col;
    }
    c_col = c_col * 2;
    return c_lnum * 100 + c_col;
}

    static long
partial(void)
{
    typeof(((pos_T *)0)->lnum) z_lnum = 5;
    typeof(((pos_T *)0)->col) z_col = 0;
    return z_lnum + z_col;
}

static int calls;

    static int
count(void)
{
    return ++calls;
}

    static long
wonly(pos_T *pp)
{
    typeof(((pos_T *)0)->lnum) w_lnum = (*pp).lnum;
    ;
    count();
    return w_lnum + calls;
}

    static pos_T
ret(void)
{
    pos_T r =
    {
        1,
        1,
    };
    return r;
}

    static void
keep(void)
{
    static pos_T s;
    pos_T k =
    {
        9,
        9,
    };
    kept = &k;
    s = k;
}

    static int
byaddr(pos_T *p)
{
    return p->col;
}

    static int
passed(void)
{
    pos_T q =
    {
        2,
        3,
    };
    return byaddr(&q);
}

static pos_T gp =
{
    1,
    2,
};

static int arr[10];

static pos_T ps[3];

    static long
fromglobal(void)
{
    pos_T g = gp;
    return g.col;
}

    static long
nested(pos_T *pp)
{
    typeof(((pos_T *)0)->lnum) a_lnum = (*pp).lnum;
    typeof(((pos_T *)0)->col) a_col = (*pp).col;
    typeof(((pos_T *)0)->lnum) b_lnum = a_lnum;
    typeof(((pos_T *)0)->col) b_col = 1;
    arr[a_col % 10] = 1;
    {
        (ps[a_lnum % 3]).lnum = b_lnum;
        (ps[a_lnum % 3]).col = b_col;
    }
    {
        b_lnum = b_lnum;
        b_col = b_col;
    }
    {
        (*pp).lnum = b_lnum;
        (*pp).col = b_col;
    }
    return b_lnum;
}

    static long
chain(void)
{
    typeof(((pos_T *)0)->lnum) x_lnum = 1;
    typeof(((pos_T *)0)->col) x_col = 2;
    typeof(((pos_T *)0)->lnum) y_lnum = x_lnum;
    typeof(((pos_T *)0)->col) y_col = x_col;
    typeof(((pos_T *)0)->lnum) z_lnum;
    {
        z_lnum = y_lnum;
    }
    return z_lnum + y_col;
}

    int
main(void)
{
    printf("%ld %ld %ld %ld\n", fromglobal(), nested(&gw.cur), chain(), ps[1].lnum);
    move_pos(&gw.cur, 3);
    printf("%ld %d\n", gw.cur.lnum, gw.cur.col);
    printf("%ld %ld %ld %d %ld\n", key(), partial(), ret().lnum, passed(), wonly(&gw.cur));
    keep();
    return 0;
}
`

func s6ssRun(t *testing.T, src string) (string, int, *Editor) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "p.c")
	g, _, err := Import(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if g, err = Read(g.Lisp()); err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	n, err := e.StructScalars(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	out, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	return string(out), n, e
}

func TestStructScalars(t *testing.T) {
	got, n, e := s6ssRun(t, s6ssSrc)
	if n != 10 {
		t.Errorf("%d structs taken, the text step took 10", n)
	}
	if got != s6ssWant {
		t.Errorf("got:\n%s\nwant:\n%s", got, s6ssWant)
	}
	if len(e.Untyped) > 0 {
		t.Errorf("%d expressions left untyped", len(e.Untyped))
	}
	i, _, err := Import(filepath.Join(t.TempDir(), "p.c"), []byte(got))
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(e.Graph(), i); err != nil {
		t.Errorf("not the import of its C view: %v", err)
	}
	if a, b := s6ssOutput(t, s6ssSrc), s6ssOutput(t, got); a != b {
		t.Errorf("the program printed\n%s\nand after the step\n%s", a, b)
	}
}

// TestStructScalarsNone: a program with no struct to take is left as it
// was, and says 0.
func TestStructScalarsNone(t *testing.T) {
	src := "typedef struct { int a; int b[2]; } arr_T;\nint main(void) { arr_T x = {1}; return x.a; }\n"
	got, n, _ := s6ssRun(t, src)
	want := gtCanon(t, src)
	if n != 0 || got != want {
		t.Errorf("%d taken:\n%s", n, got)
	}
}

// s6ssOutput is what the program prints, compiled by gcc; "" without one.
func s6ssOutput(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return ""
	}
	dir := t.TempDir()
	c, bin := filepath.Join(dir, "p.c"), filepath.Join(dir, "p")
	if err := os.WriteFile(c, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("gcc", "-std=gnu2x", "-o", bin, c).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, b)
	}
	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
