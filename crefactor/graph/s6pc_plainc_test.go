package graph

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// s6pcSrc is crefactor/xform's plainc test's program; s6pcText what the
// text steps (Identity, AsciiClass, ConstBranch) made of it, recorded
// before they were deleted.
const s6pcSrc = `int printf(const char *, ...);
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
        if (!((unsigned)*s - '0' < 10)) n += 1000;
    }
    return n + ((unsigned)count() - '0' < 10) + calls;
}
static int branch(int x)
{
    if (0) { x = 1; } else { x += 2; }
    if (x > 100) { x = 0; } else if (!1) { x = 5; } else { x *= 3; }
    if (1) { x++; }
    if (0) { x = -1; }
    if (1) { if (0) { x = 7; } }
    return x;
}
int main(void)
{
    const char *c = "cst";
    printf("%s %s %s %s %s\n", _(msg), _(c), _(1 ? "a" : "b"), same(msg), _(same(_((char *)c))));
    printf("%d %d\n", classes("aZ9!b"), branch(4));
    return 0;
}
`

const s6pcText = `static inline bool
ascii_isupper(int c)
{
    return (unsigned)c - 'A' < 26;
}

static inline bool
ascii_islower(int c)
{
    return (unsigned)c - 'a' < 26;
}

static inline bool
ascii_isdigit(int c)
{
    return (unsigned)c - '0' < 10;
}

int printf(const char *, ...);
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
        if (ascii_isupper(*s)) n += 1;
        if (ascii_islower(*s)) n += 10;
        if (ascii_isdigit((*s))) n += 100;
        if (!(ascii_isdigit(*s))) n += 1000;
    }
    return n + (ascii_isdigit(count())) + calls;
}
static int branch(int x)
{
    { x += 2; }
    if (x > 100) { x = 0; } else { x *= 3; }
    { x++; }
    {
}
    { {
} }
    return x;
}
int main(void)
{
    const char *c = "cst";
    printf("%s %s %s %s %s\n", msg, (char *)c, (1 ? "a" : "b"), msg, ((char *)c));
    printf("%d %d\n", classes("aZ9!b"), branch(4));
    return 0;
}
`

// TestPlainC: the three edits on the program read back from its Lisp give
// the text steps' C printed canonically, the graph is the import of its C
// view, and the program prints what it printed.
func TestPlainC(t *testing.T) {
	path, canon, g := importSample(t, s6pcSrc)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(h)
	n, f, err := e.PlainIdentity(h.Forms)
	if err != nil || n != 7 || f != 2 {
		t.Fatalf("identity: %d calls of %d functions, %v", n, f, err)
	}
	used, err := e.PlainAsciiClass(h.Forms)
	if err != nil || used["ascii_isdigit"] != 3 || used["ascii_isupper"] != 1 || used["ascii_islower"] != 1 {
		t.Fatalf("asciiclass: %v, %v", used, err)
	}
	k, err := e.PlainConstBranch(h.Forms)
	if err != nil || k != 6 {
		t.Fatalf("constbranch: %d, %v", k, err)
	}
	want, err := cemit.Canonical(path, []byte(s6pcText))
	if err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, want)
	if len(e.Untyped) != 0 {
		t.Fatalf("%d untyped", len(e.Untyped))
	}
	if a, b := s6pcRun(t, canon), s6pcRun(t, want); a != b {
		t.Errorf("the program prints %q, the original %q", b, a)
	}
	// a name the functions would take refuses
	_, _, g2 := importSample(t, "static int ascii_isdigit;\n"+s6pcSrc)
	if _, err := NewEditor(g2).PlainAsciiClass(g2.Forms); err == nil {
		t.Fatal("ascii_isdigit taken, not refused")
	}
}

func s6pcRun(t *testing.T, src []byte) string {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	dir := t.TempDir()
	c := filepath.Join(dir, "p.c")
	if err := os.WriteFile(c, src, 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p")
	if out, err := exec.Command("gcc", "-std=gnu2x", "-O0", "-w", "-o", exe, c).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace(got))
}
