package graph

import (
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

const substSample = `#include <string.h>
enum { FALSE, TRUE };
static int width;
static int height;
static void redraw(int full);
static void resize(int w, int h);
static int g(int x);
static int
csi(int first, int trail, int argc, int *arg)
{
    int n = 0;
    if (first == '?' && argc == 1)
    {
        n = 1;
    }
    else if (width && argc >= 3 && arg[0] == 48)
    {
        if (arg[1] != height || arg[2] != width)
        {
            resize(arg[2], arg[1]);
        }
        redraw(TRUE);
    }
    else if (first == '?' && trail == 'y' && (arg[0] == 2026 || arg[0] == 2048))
    {
        n = 2;
    }
    n = n * g(argc);
    return n;
}
static void redraw(int full) { width = full; }
static void resize(int w, int h) { width = w; height = h; }
static int g(int x) { return x + (int)strlen("x"); }
int
main(void)
{
    int a[3] = {48, 2, 3};
    return csi(0, 0, 3, a);
}
`

// substText is the text program's substitutions, printed canonically.
func substText(t *testing.T, path string, canon []byte, subs []Subst) []byte {
	t.Helper()
	s := string(canon)
	for _, x := range subs {
		if k := strings.Count(s, x.Old); k != x.N {
			t.Fatalf("the text: %q occurs %d times", x.Old, k)
		}
		s = strings.ReplaceAll(s, x.Old, x.New)
	}
	out, err := cemit.Canonical(path, []byte(s))
	if err != nil {
		t.Fatalf("%v\n%s", err, s)
	}
	return out
}

func TestSubstituteC(t *testing.T) {
	cases := []struct {
		name string
		subs []Subst
		at   []string // the heads of the nodes made, in order
	}{
		{"an else-if's operand", []Subst{{In: "csi", N: 1, What: "R1",
			Old: "    else if (width && argc >= 3 && arg[0] == 48)\n",
			New: "    else if (argc >= 3 && arg[0] == 48)\n"}}, []string{"&&"}},
		{"a statement for a block", []Subst{{In: "csi", N: 1, What: "R2",
			Old: "        if (arg[1] != height || arg[2] != width)\n        {\n            resize(arg[2], arg[1]);\n        }\n",
			New: "        resize(arg[2], arg[1]);\n"}}, []string{"call"}},
		{"a literal in a condition", []Subst{{In: "csi", N: 1, What: "R4",
			Old: "(arg[0] == 2026 || arg[0] == 2048))\n", New: "arg[0] == 2026)\n"}}, []string{"&&"}},
		{"two at once, and a statement gone", []Subst{
			{In: "csi", N: 1, What: "R6", Old: "        redraw(TRUE);\n", New: ""},
			{In: "csi", N: 1, What: "R7", Old: "    n = n * g(argc);\n    return n;\n", New: "    n = n * g(argc) + 1;\n    return n;\n"}},
			[]string{"="}},
		{"a top-level run", []Subst{{Near: "resize", N: 1, What: "P",
			Old: "static void resize(int w, int h);\n\nstatic int g(int x);\n",
			New: "static void resize(int w, int h);\n\nstatic int h(int x);\n\nstatic int g(int x);\n"}}, []string{"def"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path, canon, v, log := verbsOn(t, substSample)
			want := substText(t, path, canon, c.subs)
			before := maxID(v.Editor().Graph())
			kept := One(v.Editor().Defn("csi"), "(return n)")
			v.SubstituteC(c.subs...)
			if err := v.Done(); err != nil {
				t.Fatal(err)
			}
			asImported(t, path, v.Editor(), want)
			if !v.Editor().Live(kept) {
				t.Errorf("a statement the substitutions did not touch lost its node")
			}
			var heads []string
			Walk(v.Editor().Defn("csi"), func(n *Node) bool {
				if n.ID > before && n.list && (v.Editor().Parent(n) == nil || v.Editor().Parent(n).ID <= before) {
					heads = append(heads, n.Head())
				}
				return true
			})
			if c.subs[0].Near != "" {
				heads = nil
				for _, f := range v.Editor().Graph().Forms {
					if f.ID > before {
						heads = append(heads, f.Head())
					}
				}
			}
			if strings.Join(heads, " ") != strings.Join(c.at, " ") {
				t.Errorf("the nodes made are %v, want %v", heads, c.at)
			}
			for _, s := range c.subs {
				if !strings.Contains(log.String(), s.What) {
					t.Errorf("%s is not reported", s.What)
				}
			}
		})
	}
}

func TestSubstituteCRefusals(t *testing.T) {
	for _, c := range []struct {
		name   string
		sub    Subst
		saying string
	}{
		{"a count", Subst{In: "csi", N: 2, What: "x", Old: "redraw(TRUE);", New: ""}, "occurs 1 times, expected 2"},
		{"never another program", Subst{In: "csi", N: 1, What: "x",
			Old: "n = n * g(argc);", New: "n = n * g(argc) + g(1) * 2 - n - 1 + 0 * 0;"}, ""},
		{"a fragment that does not parse", Subst{In: "csi", N: 1, What: "x", Old: "redraw(TRUE);", New: "redraw(TRUE"}, "x --"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.saying == "" {
				// the text's substitution is what is made, or a refusal: never
				// another program
				path, canon, v, _ := verbsOn(t, substSample)
				want := substText(t, path, canon, []Subst{c.sub})
				v.SubstituteC(c.sub)
				if v.Done() == nil {
					asImported(t, path, v.Editor(), want)
				}
				return
			}
			refuses(t, substSample, func(v *Verbs) { v.SubstituteC(c.sub) }, c.saying)
		})
	}
}

// The control: an operand that changes precedence where the node is
// narrowed to is refused by the self-check, not made.
func TestSubstituteCPrecedence(t *testing.T) {
	refuses(t, substSample, func(v *Verbs) {
		v.SubstituteC(Subst{In: "csi", N: 1, What: "x", Old: "n = n * g(argc);", New: "n = n * argc + 1;"})
	}, "did not stand where it was written")
}

// SubstituteSeq: the second substitution's old C is what the first made,
// the third names what the second declares, and the fourth is in the run
// the first changed, apart from it; the result is the text's, one after the
// other, and so is the count of imports it took (the order says three).
func TestSubstituteSeq(t *testing.T) {
	subs := []Subst{
		{In: "csi", N: 1, What: "1", Old: "    else if (width && argc >= 3 && arg[0] == 48)\n", New: "    else if (argc >= 3 && arg[0] == 48)\n"},
		{In: "csi", N: 1, What: "2", Old: "    else if (argc >= 3 && arg[0] == 48)\n", New: "    else if (argc >= 4 && arg[0] == 48)\n"},
		{Near: "height", N: 1, What: "3", Old: "static int height;\n", New: "static int height;\n\nstatic int depth;\n"},
		{In: "g", N: 1, What: "4", Old: "return x + (int)strlen(\"x\");", New: "return x + depth + (int)strlen(\"x\");"},
		{In: "csi", N: 1, What: "5", Old: "(arg[0] == 2026 || arg[0] == 2048)", New: "arg[0] == 2026"},
		{Near: "resize", N: 1, What: "6", Old: "static int g(int x);\n", New: "static int g(int x);\n\nstatic int h(int x);\n"},
	}
	path, canon, v, log := verbsOn(t, substSample)
	want := substText(t, path, canon, subs)
	before := len(v.Editor().Log)
	v.SubstituteSeq(subs...)
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, v.Editor(), want)
	if got := log.String(); strings.Count(got, "tiny") != len(subs) {
		t.Errorf("the report is %q", got)
	}
	_ = before
}

// An insertion after a run's last item keeps the run's nodes.
func TestSubstituteCAfter(t *testing.T) {
	path, canon, v, _ := verbsOn(t, substSample)
	sub := Subst{Near: "g", N: 1, What: "after",
		Old: "static void resize(int w, int h);\n\nstatic int g(int x);\n",
		New: "static void resize(int w, int h);\n\nstatic int g(int x);\n\nstatic int h2(int x);\n"}
	want := substText(t, path, canon, []Subst{sub})
	var kept []*Node
	for _, f := range v.Editor().Graph().Forms {
		if n := DeclName(f); (n == "resize" || n == "g") && f.Is("def") {
			kept = append(kept, f)
		}
	}
	v.SubstituteC(sub)
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, v.Editor(), want)
	for _, f := range kept {
		if !v.Editor().Live(f) {
			t.Errorf("%s's prototype lost its node", DeclName(f))
		}
	}
}

// The control: two substitutions in one run whose old C meet are refused
// together, and made one after the other by SubstituteSeq.
func TestSubstituteCOverlap(t *testing.T) {
	a := Subst{In: "csi", N: 1, What: "a", Old: "arg[0] == 48)", New: "arg[0] == 49)"}
	b := Subst{In: "csi", N: 1, What: "b", Old: "argc >= 3 && arg[0] == 48", New: "argc >= 3 && arg[0] == 48 && n"}
	refuses(t, substSample, func(v *Verbs) { v.SubstituteC(a, b) }, "make them one after the other")
}
