package reach

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/arbace/go-whim/internal/ccx"
)

// small is one translation unit with every kind live and every kind dead, a
// static_assert, an enum with a fixed underlying type, a renumbering hazard
// and a positional initialiser.
const small = `typedef unsigned long usize;
typedef long hash_T;
enum : usize { SIZE_MAX = (usize)-1 };
enum { A0, A1_DEAD, A2 };
enum colour { RED = 1, BLUE };
struct pair { int left; int right_dead; };
typedef struct { int only; } box_T;
typedef struct
{
    int kind;
    long start_dead;
} req_T;
static req_T r0 = {1, -1};
static int seen;
static int dead_obj;
static int callee(void) { return A2; }
static int dead_callee(void) { return 1; }
static int dead_caller(void) { return dead_callee(); }
static int dead_proto(void);
static_assert(sizeof(usize) == 8, "usize");
int main(void)
{
    struct pair p = {.left = 0};
    box_T b;
    b.only = 0;
    seen = (int)SIZE_MAX + BLUE;
    return callee() + p.left + b.only + r0.kind + seen;
}
`

func analyze(t *testing.T, src string) (*Closure, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "t.c")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ast, err := ccx.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	return Analyze(ast, path, []byte(src)), path
}

func ids(es []*Entity) string {
	var out []string
	for _, e := range es {
		out = append(out, e.ID)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func TestClosure(t *testing.T) {
	c, _ := analyze(t, small)
	want := "F:dead_callee F:dead_caller M:pair.right_dead M:req_T.start_dead N:A0 N:A1_DEAD N:RED O:dead_obj P:dead_proto T:hash_T"
	if got := ids(c.Unreachable()); got != want {
		t.Fatalf("unreachable:\n got %s\nwant %s", got, want)
	}
	if got := strings.Join(c.Roots[ClassAssert], " "); got != "T:usize" {
		t.Errorf("static_assert roots: %s", got)
	}
	why := map[string]string{}
	for _, f := range c.Partition().Left {
		why[f.Fn] = f.What
	}
	// Every unreachable enumerator here has a live implicit one after it:
	// RED = 1 is explicit, but BLUE after it is not.
	for _, n := range []string{"N:A0", "N:A1_DEAD", "N:RED"} {
		if !strings.HasPrefix(why[n], WhyRenumbers) {
			t.Errorf("%s: %s", n, why[n])
		}
	}
	if !strings.HasPrefix(why["M:req_T.start_dead"], WhyPositional) {
		t.Errorf("start_dead: %s", why["M:req_T.start_dead"])
	}
	// p is initialised with a designator, which names left: that is a leftover
	// of the instrument, never a silent pass.
	if !strings.Contains(why[".left"], "designator") {
		t.Errorf("the designator was not refused: %v", why)
	}
}

// The control has to be able to fail.  A function marked used is kept by gcc
// and by nothing the closure knows, so the two instruments disagree on it and
// the agreement must refuse.
func TestAgreementRefuses(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	src := "__attribute__((used)) static int kept(void) { return 0; }\nstatic int dead(void) { return 1; }\nint main(void) { return 0; }\n"
	c, path := analyze(t, src)
	ast, _ := ccx.Parse(path)
	res, _, err := Agreement(c, ast, path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Classes[agreeBoth] != 1 || len(res.Left) != 1 || res.Left[0].Fn != "F:kept" {
		t.Fatalf("want F:dead agreed and F:kept refused, got %+v", res)
	}
}

func TestControls(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	src := strings.Replace(small, "struct pair p = {.left = 0};", "struct pair p; p.left = 0;", 1)
	c, path := analyze(t, src)
	ast, _ := ccx.Parse(path)
	agree, unused, err := Agreement(c, ast, path)
	if err != nil {
		t.Fatal(err)
	}
	// gcc names dead_caller, dead_obj, dead_proto; dead_callee is one level
	// further than it looks.
	if len(agree.Left) != 0 || agree.Classes[agreeBoth] != 3 || agree.Classes[agreeDeeper] != 1 {
		t.Fatalf("agreement: %+v", agree)
	}
	planted, err := Planted(c, []byte(src), unused, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(planted.Left) != 0 || planted.Classes[plantFound] != len(plantIDs) || planted.Classes[plantGcc] != 3 {
		t.Fatalf("planted: %+v", planted)
	}
	pos, err := Positional(c, []byte(src), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// start_dead is req_T's last member and r0 fills it: gcc warns, exit 0.
	if len(pos.Left) != 0 || pos.Classes["deleted, and gcc only WARNS -- excess elements in struct initializer -- and exits 0"] != 1 {
		t.Fatalf("positional: %+v", pos)
	}
}
