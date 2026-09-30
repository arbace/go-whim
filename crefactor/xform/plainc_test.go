package xform

import (
	"bytes"
	"strings"
	"testing"
)

const plainC = `int printf(const char *, ...);
static inline char *
_(const char *x)
{
    return (char *)x;
}
static char *same(char *x) { return x; }
static char msg[] = "hello";
static int calls;
static int count(void) { return ++calls; }
static int classes(const char *s)
{
    int n = 0;
    for (; *s; s++)
    {
        if ((unsigned)*s - 'A' < 26) n += 1;
        if ((unsigned)*s - 'a' < 26) n += 10;
        if ((unsigned)(*s) - '0' < 10) n += 100;
    }
    return n + ((unsigned)count() - '0' < 10) + calls;
}
static int branch(int x)
{
    if (0) { x = 1; } else { x += 2; }
    if (x > 100) { x = 0; } else if (!1) { x = 5; } else { x *= 3; }
    if (1) { x++; }
    if (0) { x = -1; }
    return x;
}
int main(void)
{
    const char *c = "cst";
    printf("%s %s %s %s\n", _(msg), _(c), _(1 ? "a" : "b"), same(msg));
    printf("%d %d\n", classes("aZ9!b"), branch(4));
    return 0;
}
`

// The three steps keep what the program prints, and write it plainly.
func TestPlainC(t *testing.T) {
	want := gccRun(t, plainC)
	src := []byte(plainC)
	var log bytes.Buffer
	for _, st := range []Step{Identity(nil), AsciiClass(nil), ConstBranch(nil)} {
		out, err := st(src, nil, &log)
		if err != nil {
			t.Fatalf("%v\n%s", err, log.String())
		}
		src = out
	}
	got := string(src)
	for _, w := range []string{
		`printf("%s %s %s %s\n", msg, (char *)c, (1 ? "a" : "b"), msg)`,
		"if (ascii_isupper(*s))", "if (ascii_islower(*s))", "if (ascii_isdigit((*s)))",
		"ascii_isdigit(count())",
		"static inline bool\nascii_isupper(int c)",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("no %q in\n%s", w, got)
		}
	}
	for _, no := range []string{"if (0)", "if (1)", "!1", "_(msg)"} {
		if strings.Contains(got, no) {
			t.Errorf("%q left in\n%s", no, got)
		}
	}
	if o := gccRun(t, got); o != want {
		t.Errorf("the program prints %q, the original %q\n%s", o, want, got)
	}
	t.Log(log.String())
}
