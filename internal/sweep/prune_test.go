package sweep

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// prune runs Prune on src and fails the test on an error.
func prune(t *testing.T, src string) string {
	t.Helper()
	out, _, err := Prune([]byte(src), "t.c", Options{Roots: []string{"main"}, FreezeLayoutIf: []string{"ml_recover"}})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	return string(out)
}

func has(t *testing.T, out, re string, want bool) {
	t.Helper()
	if regexp.MustCompile(re).MatchString(out) != want {
		verb := "lacks"
		if !want {
			verb = "still has"
		}
		t.Errorf("output %s %q:\n%s", verb, re, out)
	}
}

// compiles asks gcc whether the output is C, when there is a gcc.
func compiles(t *testing.T, out string) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "o.c")
	os.WriteFile(p, []byte(out), 0o644)
	if b, err := exec.Command("gcc", "-fsyntax-only", "-std=gnu2x", "-Werror", p).CombinedOutput(); err != nil {
		t.Errorf("gcc refuses the output: %s\n%s", b, out)
	}
}

func TestPruneFunctionsAndObjects(t *testing.T) {
	out := prune(t, `
static int used(void);
static int unused(void);
static int counter;
static int orphan;
int table_fn(void) { return 1; }
static int (*table[])(void) = { table_fn };
static int used(void) { return counter; }
static int unused(void) { return orphan; }
int main(void) { return used(); }
`)
	has(t, out, `\bunused\b`, false)
	has(t, out, `\borphan\b`, false)
	has(t, out, `\btable\b`, false)
	has(t, out, `\btable_fn\b`, false)
	has(t, out, `static int used\(void\);`, true)
	has(t, out, `static int counter;`, true)
	compiles(t, out)
}

func TestPruneDeclaratorLists(t *testing.T) {
	out := prune(t, `
static int a, b, c, d;
int main(void) { return b + d; }
`)
	has(t, out, `static int b, d;`, true)
	compiles(t, out)
}

func TestPruneKeepsTagDroppingTypedef(t *testing.T) {
	out := prune(t, `
typedef struct node { int v; struct node *next; } node_T;
int main(void) { struct node n; n.v = 0; return n.v; }
`)
	has(t, out, `node_T`, false)
	has(t, out, `struct node \{`, true)
	has(t, out, `\bnext\b`, false) // a member nothing names
	compiles(t, out)
}

func TestPruneShadowing(t *testing.T) {
	// A local named like a global does not keep it.
	out := prune(t, `
static int append(int x) { return x; }
int main(void) { int append = 1; return append; }
`)
	has(t, out, `static int append\(`, false)
	compiles(t, out)
}

func TestPruneEnumPinned(t *testing.T) {
	out := prune(t, `
enum { A, B, C, D = 10, E };
int main(void) { return A + C + E; }
`)
	has(t, out, `\bB\b`, false)
	has(t, out, `\bD\b`, false)
	has(t, out, `C = 2`, true)
	has(t, out, `E = 11`, true)
	compiles(t, out)
}

func TestPruneEnumUnpinnableKept(t *testing.T) {
	// sizeof has no value the text can give, so the run before E stays.
	out := prune(t, `
enum { A = sizeof(long), B, C };
int main(void) { return A + C; }
`)
	has(t, out, `\bB\b`, true)
	compiles(t, out)
}

func TestPrunePositional(t *testing.T) {
	out := prune(t, `
struct pair { int a; int b; int c; };
struct inner { int x; int y; };
struct outer { struct inner in; int z; };
static struct pair p = { 1, 2, 3 };
static struct outer o = { .in = { 1, 2 }, .z = 3 };
static struct outer q = { { 1, 2 }, 3 };
int main(void) { return p.a + o.z + q.z; }
`)
	has(t, out, `int b;`, true) // p is filled by position
	has(t, out, `int y;`, true) // inner is filled by position inside outer
	compiles(t, out)
}

func TestPruneNeverEmpties(t *testing.T) {
	out := prune(t, `
struct s { int a; int b; };
int main(void) { struct s v; (void)v; return 0; }
`)
	has(t, out, `int a;`, true)
	has(t, out, `int b;`, true)
	compiles(t, out)
}

func TestPruneUnusedLocal(t *testing.T) {
	out := prune(t, `
static int helper(void) { return 1; }
int main(void)
{
    int keep = 0, drop = 2;
    int gone = helper();
    return keep;
}
`)
	has(t, out, `\bdrop\b`, false)
	has(t, out, `\bgone\b`, false)
	has(t, out, `\bhelper\b`, false) // its one use went with the local
	has(t, out, `int keep = 0;`, true)
	compiles(t, out)
}

func TestPruneStaticAssertRoots(t *testing.T) {
	out := prune(t, `
enum { SIZE = 4 };
static_assert(SIZE == 4, "size");
int main(void) { return 0; }
`)
	has(t, out, `SIZE = 4`, true)
	compiles(t, out)
}

func TestPruneMemberByName(t *testing.T) {
	out := prune(t, `
struct a { int shared; int only_a; };
struct b { int shared; int only_b; };
int main(void) { struct a x; struct b y; x.shared = 1; y.only_b = 2; return x.shared + y.only_b; }
`)
	has(t, out, `only_a`, false)
	// by name: b.shared stays because a.shared is named
	if strings.Count(out, "int shared;") != 2 {
		t.Errorf("both members named shared should stay:\n%s", out)
	}
	compiles(t, out)
}

func TestPruneUnusedLocalShadowed(t *testing.T) {
	// Two locals of one name in two blocks: each is its own, and both go.
	out := prune(t, `
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
`)
	has(t, out, `unblock`, false)
	has(t, out, `int used = 1;`, false)
	has(t, out, `int used = 2;`, true)
	compiles(t, out)
}

func TestPruneRecoverKeepsMembers(t *testing.T) {
	// A struct layout is a disk format while ml_recover is defined.
	src := `
struct block0 { int b0_id; int b0_unused; };
int ml_recover(void) { struct block0 b; b.b0_id = 1; return b.b0_id; }
int main(void) { return ml_recover(); }
`
	out := prune(t, src)
	has(t, out, `int b0_unused;`, true)
	out = prune(t, strings.ReplaceAll(src, "ml_recover", "ml_read"))
	has(t, out, `b0_unused`, false)
	compiles(t, out)
}

func TestPruneRoots(t *testing.T) {
	// The roots are the caller's: a library with no main keeps what its
	// entry points reach, and loses the rest.
	src := `
static int helper(void) { return 1; }
static int unused(void) { return 2; }
int api_entry(void) { return helper(); }
`
	out, _, err := Prune([]byte(src), "t.c", Options{Roots: []string{"api_entry"}})
	if err != nil {
		t.Fatal(err)
	}
	has(t, string(out), `helper`, true)
	has(t, string(out), `unused`, false)
	out, _, _ = Prune([]byte(src), "t.c", Options{Roots: []string{"main"}})
	has(t, string(out), `api_entry`, false) // vim's root on a library takes everything
}
