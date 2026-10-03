package graph

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// samples are crefactor/sweep's cases, and a few of the graph's own: each
// is held to the sweep, which is the oracle -- Collect's C view must be
// the canonical print of what Prune leaves.
var samples = map[string]string{
	"functions and objects": `
static int used(void);
static int unused(void);
static int counter;
static int orphan;
int table_fn(void) { return 1; }
static int (*table[])(void) = { table_fn };
static int used(void) { return counter; }
static int unused(void) { return orphan; }
int main(void) { return used(); }
`,
	"declarator lists": `
static int a, b, c, d;
int main(void) { return b + d; }
`,
	"tag kept, typedef dropped": `
typedef struct node { int v; struct node *next; } node_T;
int main(void) { struct node n; n.v = 0; return n.v; }
`,
	"shadowing": `
static int append(int x) { return x; }
int main(void) { int append = 1; return append; }
`,
	"enum pinned": `
enum { A, B, C, D = 10, E };
int main(void) { return A + C + E; }
`,
	"enum unpinnable kept": `
enum { A = sizeof(long), B, C };
int main(void) { return A + C; }
`,
	"positional": `
struct pair { int a; int b; int c; };
struct inner { int x; int y; };
struct outer { struct inner in; int z; };
static struct pair p = { 1, 2, 3 };
static struct outer o = { .in = { 1, 2 }, .z = 3 };
static struct outer q = { { 1, 2 }, 3 };
int main(void) { return p.a + o.z + q.z; }
`,
	"never emptied": `
struct s { int a; int b; };
int main(void) { struct s v; (void)v; return 0; }
`,
	"unused locals": `
static int helper(void) { return 1; }
int main(void)
{
    int keep = 0, drop = 2;
    int gone = helper();
    return keep;
}
`,
	"static_assert roots": `
enum { SIZE = 4 };
static_assert(SIZE == 4, "size");
int main(void) { return 0; }
`,
	"members by name": `
struct a { int shared; int only_a; };
struct b { int shared; int only_b; };
int main(void) { struct a x; struct b y; x.shared = 1; y.only_b = 2; return x.shared + y.only_b; }
`,
	"locals shadowed": `
int main(int argc, char **argv)
{
    if (argc > 1)
    {
        int unblock = 0;
        argc++;
    }
    if (argc > 2)
    {
        int unblock = 0;
    }
    int used = 1;
    {
        int used = 2;
        return used;
    }
}
`,
	"recover keeps members": `
struct block0 { int b0_id; int b0_unused; };
int ml_recover(void) { struct block0 b; b.b0_id = 1; return b.b0_id; }
int main(void) { return ml_recover(); }
`,
	"recover gone": `
struct block0 { int b0_id; int b0_unused; };
int ml_read(void) { struct block0 b; b.b0_id = 1; return b.b0_id; }
int main(void) { return ml_read(); }
`,
	"headers, designators, labels": `
#include <string.h>
#include <sys/stat.h>
typedef struct { int len; const char *s; int spare; } str_T;
static str_T greeting = { .len = 5, .s = "hello" };
static int pad;
int main(int argc, char **argv)
{
    struct stat st;
    size_t n = strlen(argv[0]);
    if (argc > 3)
        goto out;
    n += st.st_size + greeting.len;
out:
    return (int)n;
}
`,
	"anonymous members, nested tags": `
struct outer
{
    int kind;
    union
    {
        int i;
        char c;
    };
    struct named { int x; int unused_x; } nm;
    int dead;
};
int main(void) { struct outer o; o.kind = 1; o.i = 2; o.nm.x = 3; return o.kind + o.i + o.nm.x; }
`,
	"orphaned fallthroughs": `
int main(int argc, char **argv) {
	int r = 0;
	(void)argv;
	switch (argc) {
	case 1:
		r = 1;
		[[fallthrough]];
	case 2:
		r++;
		[[fallthrough]];
	L:  default:
		if (r) {
			r = 2;
			[[fallthrough]];
		}
	case 3:
		do {
			r = 3;
			[[fallthrough]];
		} while (0);
	case 4:
		r = 4;
		[[fallthrough]];
		r = 5;
	case 5:
		while (r < 9) {
			r++;
			[[fallthrough]];
		}
		break;
	case 6:
		switch (r) {
		case 0:
			r = 6;
			[[fallthrough]];
		}
		r = 7;
		__attribute__((fallthrough));
	}
	if (r == 99) goto L;
	return r;
}
`,
	"an array of pointers, by position": `
struct pair { int a; int b; };
struct trio { int x; int y; int z; };
static struct pair *(ptrs[2]) = { 0, 0 };
static struct trio (*tp)[2];
int main(void) { return ptrs[0]->a + (*tp)[0].x; }
`,
	"a local's initialiser orphans": `
static int f(void) { return 1; }
static int g(void) { return f(); }
int main(void) { int unused = g(); return 0; }
`,
}

var testCollect = CollectOptions{Roots: []string{"main"}, FreezeLayoutIf: []string{"ml_recover"}}

// oracle is the pipeline's sweep and print of src.
func oracle(t *testing.T, path string, src []byte) []byte {
	t.Helper()
	swept, _, err := sweep.Prune(src, path, sweep.Options{Roots: testCollect.Roots, FreezeLayoutIf: testCollect.FreezeLayoutIf})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	out, err := cemit.Canonical(path, swept)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	return out
}

func importSample(t *testing.T, src string) (string, []byte, *Graph) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.c")
	canon, err := cemit.Canonical(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Import(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return path, canon, g
}

func TestSamples(t *testing.T) {
	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			path, canon, g := importSample(t, src)
			// step 1: the C view is the canonical text
			out, err := g.C()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, canon) {
				t.Fatalf("C view: %s", firstDiff(out, canon))
			}
			if c := g.Count(); c.Dangling != 0 {
				t.Errorf("%d edges dangle", c.Dangling)
			}
			// step 2: written and read back, the same graph
			h, err := Read(g.Lisp())
			if err != nil {
				t.Fatalf("read: %v\n%s", err, g.Lisp())
			}
			if err := Equal(g, h); err != nil {
				t.Fatalf("read back: %v", err)
			}
			// step 3: collected, the sweep's text -- on the graph read back,
			// which no cc node is behind
			if _, err := Collect(h, testCollect); err != nil {
				t.Fatal(err)
			}
			got, err := h.C()
			if err != nil {
				t.Fatal(err)
			}
			if want := oracle(t, path, []byte(src)); !bytes.Equal(got, want) {
				t.Fatalf("collected: %s\ngot:\n%s\nwant:\n%s", firstDiff(got, want), got, want)
			}
			if c := h.Count(); c.Dangling != 0 {
				t.Errorf("after the collection %d edges dangle", c.Dangling)
			}
		})
	}
}

// TestResolution: what the samples' edges say.
func TestResolution(t *testing.T) {
	_, _, g := importSample(t, samples["members by name"])
	// x.shared is struct a's member, y.only_b struct b's: by type.
	var refs []string
	for _, f := range g.Forms {
		Walk(f, func(n *Node) bool {
			if n.Is(".") {
				for _, m := range n.Kids[2:] {
					owner := ""
					for _, s := range g.Forms {
						Walk(s, func(x *Node) bool {
							for _, k := range x.Kids {
								if k == m.Ref() {
									owner = tagOf(x)
								}
							}
							return true
						})
					}
					refs = append(refs, m.Atom+"->"+owner)
				}
			}
			return true
		})
	}
	if got := strings.Join(refs, " "); got != "shared->a only_b->b shared->a only_b->b" {
		t.Errorf("members resolve to %s", got)
	}
	_, _, g = importSample(t, samples["headers, designators, labels"])
	if _, rep, err := Import(filepath.Join(t.TempDir(), "d.c"), []byte(samples["headers, designators, labels"])); err != nil || rep.Designators.File != 2 || rep.Labels.Local != 1 || len(rep.Unresolved) != 0 {
		t.Errorf("designators and labels: %v\n%s", err, rep)
	}
	text := string(g.Lisp())
	for _, want := range []string{"(extern strlen)", "(extern-typedef size_t)", "(extern-struct stat", "(member st_size)"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %s in the externs:\n%s", want, text)
		}
	}
}

// TestControls: each gate catches a deliberate fault.
func TestControls(t *testing.T) {
	_, canon, g := importSample(t, samples["enum pinned"])
	// step 1: a token changed in the graph changes the C view
	Walk(g.Forms[0], func(n *Node) bool {
		if n.Atom == "10" {
			n.Atom = "11"
		}
		return true
	})
	if out, _ := g.C(); bytes.Equal(out, canon) {
		t.Error("step 1's control: a changed token was not seen")
	}
	// step 2: an edge retargeted in the Lisp is not the same graph
	_, _, g = importSample(t, samples["members by name"])
	text := g.Lisp()
	i := bytes.LastIndexByte(text, '@')
	bad := append(append(append([]byte{}, text[:i+1]...), '1'), text[i+1:]...)
	if h, err := Read(bad); err == nil && Equal(g, h) == nil {
		t.Error("step 2's control: a retargeted edge was not seen")
	}
	// step 3: the graph's own member rule is not the sweep's
	path, _, g := importSample(t, samples["members by name"])
	opt := testCollect
	opt.MembersByType = true
	if _, err := Collect(g, opt); err != nil {
		t.Fatal(err)
	}
	if out, _ := g.C(); bytes.Equal(out, oracle(t, path, []byte(samples["members by name"]))) {
		t.Error("step 3's control: members by type collected what the sweep does")
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("TMPDIR") == "" {
		os.Setenv("TMPDIR", os.TempDir())
	}
	os.Exit(m.Run())
}

// TestFallthroughs: the sample's three orphans go, its four stay.
func TestFallthroughs(t *testing.T) {
	_, _, g := importSample(t, samples["orphaned fallthroughs"])
	st, err := Collect(g, testCollect)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := g.C()
	if st.Fallthroughs != 4 || strings.Count(string(out), "[[fallthrough]]") != 4 {
		t.Errorf("%d fallthroughs collected:\n%s", st.Fallthroughs, out)
	}
}
