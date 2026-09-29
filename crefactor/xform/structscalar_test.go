package xform

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

const structScalarC = `int printf(const char *, ...);
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
int main(void)
{
    move_pos(&gw.cur, 3);
    printf("%ld %d\n", gw.cur.lnum, gw.cur.col);
    printf("%ld %ld %ld %d %ld\n", key(), partial(), ret().lnum, passed(), wonly(&gw.cur));
    keep();
    return 0;
}
`

// StructScalar takes a struct local used by member and copied whole, and
// leaves one returned, one whose address is taken, a static.
func TestStructScalar(t *testing.T) {
	var log bytes.Buffer
	out, err := StructScalar(nil)([]byte(structScalarC), nil, &log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"typeof(((pos_T *)0)->lnum) a_lnum = (*pp).lnum;",
		"a_col += dc;",
		"{ (*pp).lnum = a_lnum; (*pp).col = a_col; }",
		"{ c_lnum = b_lnum; c_col = b_col; }",
		"typeof(((pos_T *)0)->col) z_col = 0;",
		"pos_T r = {1, 1};", // returned whole
		"pos_T k = {9, 9};", // its address taken
		"pos_T q = {2, 3};",
		"typeof(((pos_T *)0)->lnum) w_lnum = (*pp).lnum;", // w.col is only stored
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s\n%s", want, got, log.String())
		}
	}
	if strings.Contains(got, "w_col") {
		t.Errorf("w.col, only stored, is a local still:\n%s", got)
	}
	if _, err := translate(out); err != nil {
		t.Fatalf("the result does not type-check: %v\n%s", err, got)
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	if a, b := gccRun(t, structScalarC), gccRun(t, got); a != b {
		t.Errorf("the program printed\n%s\nand after the step\n%s\n%s", a, b, got)
	}
}
