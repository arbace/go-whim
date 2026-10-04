package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// s6key is a type node's structure: its words and operands, a struct's,
// union's or enum's identity.
func s6key(t *Node) string {
	if t == nil {
		return "nil"
	}
	switch t.Head() {
	case "struct", "union", "enum":
		return fmt.Sprintf("%s#%p", t.Head(), t)
	}
	if !t.list {
		return t.Atom
	}
	var b strings.Builder
	b.WriteString("(")
	for _, k := range t.Kids {
		switch {
		case k.list && k.Type == nil && k.Head() == "":
			b.WriteString("(")
			for _, p := range k.Kids {
				if p.Type != nil {
					b.WriteString(s6key(p.Type) + " ")
				} else {
					b.WriteString(p.Atom + " ")
				}
			}
			b.WriteString(")")
		case k.Type != nil:
			b.WriteString(s6key(k.Type) + " ")
		default:
			b.WriteString(k.Atom + " ")
		}
	}
	b.WriteString(")")
	return b.String()
}

// s6strip clears every expression's typed edge in g and lists it untyped:
// what Recheck must give back, as the import gave it.
func s6strip(g *Graph) (*Editor, map[*Node]*Node) {
	want := map[*Node]*Node{}
	var order []*Node
	for _, f := range g.Forms {
		Walk(f, func(n *Node) bool {
			if n.list && n.Type != nil && isExprForm(n) && !n.Is("macro") && !n.Is("generic") {
				want[n] = n.Type
				order = append(order, n)
			}
			return true
		})
	}
	for _, n := range order {
		n.Type = nil
	}
	e := NewEditor(g)
	e.Untyped = order
	return e, want
}

// TestRecheckEveryExpression: on S6_RECHECK (a C file, or a glob of them --
// the pipeline's boundaries), every expression's typed edge is cleared,
// and Recheck must give each back as the import typed it, or leave it
// untyped -- never typed otherwise.  What it leaves is what a header's
// macro spells (`ICRNL`, `PATH_MAX`, `errno`, `sa_handler`), whose type is
// the header's and not the graph's: 16 expressions of the host from q043
// on, 52-214 before the headers moved below the core.  The census of what
// differs, by head, is the report.
func TestRecheckEveryExpression(t *testing.T) {
	glob := os.Getenv("S6_RECHECK")
	if glob == "" {
		t.Skip("S6_RECHECK is not set")
	}
	files, _ := filepath.Glob(glob)
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			g, _, err := Import(f, src)
			if err != nil {
				t.Fatal(err)
			}
			e, want := s6strip(g)
			st := e.Recheck()
			census := map[string]int{}
			examples := map[string]string{}
			bad, left := 0, 0
			for n, w := range want {
				if s6key(n.Type) != s6key(w) {
					if n.Type == nil {
						left++
					} else {
						bad++
						t.Logf("WRONG %v: got %s want %s", Lisp(n), s6key(n.Type), s6key(w))
					}
					k := n.Head()
					census[k]++
					if _, ok := examples[k]; !ok || len(examples[k]) > 300 {
						s := fmt.Sprint(Lisp(n))
						if len(s) > 200 {
							s = s[:200]
						}
						examples[k] = fmt.Sprintf("%s: got %s want %s", s, s6key(n.Type), s6key(w))
					}
				}
			}
			var ks []string
			for k := range census {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			for _, k := range ks {
				t.Logf("%-8s %5d  e.g. %s", k, census[k], examples[k])
			}
			t.Logf("%d expressions, %s; %d typed otherwise than the import, %d not typed", len(want), st, bad, left)
			if bad > 0 {
				t.Fail()
			}
		})
	}
}

// TestRecheckSample is TestRecheckEveryExpression on a sample of C's
// expression rules, with no corpus: promotions, the usual conversions
// (signed against unsigned, long against unsigned, an enum), pointer
// arithmetic and differences, decay (an array's and a function's, none under
// `&`), `?:`'s cases, members and a narrow bit-field, casts, sizeof, the
// literals' suffixes, strings, nullptr, the comma, the assignments.
func TestRecheckSample(t *testing.T) {
	src := []byte(`enum color { RED, GREEN = 5 };
enum big { BIG = 3000000000 };
struct pt { int x; unsigned f : 3; long y; char name[8]; struct pt *next; };
typedef struct pt pt_T;
static int arr[10];
static int add(int a, int b);
static int add(int a, int b)
{
    return a + b;
}
int main(void)
{
    char c = 'a';
    unsigned u = 1u;
    long l = 2L;
    unsigned long ul = 3ul;
    long long ll = 4ll;
    double d = 1.5;
    float fl = 2.5f;
    short s = 1;
    pt_T p;
    pt_T *pp = &p;
    int (*fp)(int, int) = add;
    int *ip = arr;
    enum color col = GREEN;
    char *str = "abc";
    void *vp = nullptr;
    long diff = ip + 3 - arr;
    int r = c + s + (u > 0) + (int)(l * ul) + (int)(ll >> 1) + -c + ~s + !vp;
    r += (int)(d * fl) + (int)(u + l) + (int)(ul - ll) + col + BIG % 7 + RED;
    r = pp->x + p.f + (int)pp->y + p.name[0] + *str + str[1] + (int)sizeof(p.name) + (int)sizeof p;
    r = (r ? pp : nullptr) == pp->next ? fp(1, 2) : (*fp)(3, 4);
    r = (vp != nullptr, r), r++, --r;
    ip = &arr[2];
    int (*ap)[10] = &arr;
    r += (*ap)[1] + (int)(diff + (r ? 1 : 2u)) + (int)(c ? d : 1) + (int)(str - "x");
    return r + (int)(long)ip;
}
`)
	g, _, err := Import(filepath.Join(t.TempDir(), "s.c"), src)
	if err != nil {
		t.Fatal(err)
	}
	e, want := s6strip(g)
	st := e.Recheck()
	for n, w := range want {
		if s6key(n.Type) != s6key(w) {
			t.Errorf("%v: typed %s, the import %s", Lisp(n), s6key(n.Type), s6key(w))
		}
	}
	if st.Left != 0 || len(e.Untyped) != 0 {
		t.Errorf("%s", st)
	}
	t.Logf("%d expressions, %s", len(want), st)
}
