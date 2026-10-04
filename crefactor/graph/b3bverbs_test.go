package graph

import (
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

// B3b's verbs (b3bverbs.go) held to the text verbs they stand for, on graphs
// read back from their Lisp.

const b3bSample = `static int x;
static int y;
static int k(int a)
{
    return a + 1;
}
static int vf(const char *f, ...)
{
    return f[0];
}
int
f(int a)
{
    if (x)
    {
        a = 1;
    }
    else if (x)
    {
        a = 2;
    }
    else
    {
        a = 3;
    }
    if (y)
    {
        a = 4;
    }
    while (a)
    {
        if (y)
        {
            break;
        }
        a = a - 1;
    }
    a = 5;
    a = 6;
    {
        a = 5;
    }
    vf("%d %d", a, k(a));
    return a;
}
`

// A fold by place: the text's line regexps tell `if (x)` from `else if (x)`.
func TestB3bFoldPlaced(t *testing.T) {
	sameAsText(t, b3bSample, func(s []byte) ([]byte, error) {
		return edit.FoldNever(s, `(?m)^[ \t]*if \(x\)$`, 1)
	}, func(v *Verbs) { v.FoldNeverAt(IfNotArm, "x", 1, "plain") })
	sameAsText(t, b3bSample, func(s []byte) ([]byte, error) {
		return edit.FoldNever(s, `(?m)^[ \t]*else if \(x\)$`, 1)
	}, func(v *Verbs) { v.FoldNeverAt(IfArm, "x", 1, "arm") })
	sameAsText(t, b3bSample, func(s []byte) ([]byte, error) {
		return edit.FoldAlways(s, `(?m)^    if \(y\)$`, 1)
	}, func(v *Verbs) {
		// the plain `if (y)` outside the loop, scoped to itself: the
		// loop's holds a break
		ms := v.Find("(if y (block (= a 4)))")
		if len(ms) != 1 {
			t.Fatalf("%d", len(ms))
		}
		v.In(ms[0], func(v *Verbs) { v.FoldAlwaysAt("y", 1, true, "always") })
	})
	refuses(t, b3bSample, func(v *Verbs) { v.FoldNeverAt(IfAnywhere, "x", 1, "both") }, "matched 2 times, expected 1")
	refuses(t, b3bSample, func(v *Verbs) {
		v.In(v.Find("(while a _*)")[0], func(v *Verbs) {
			v.FoldAlwaysAt("y", 1, true, "the loop's")
		})
	}, "the body kept by this fold carries a break or continue")
}

// CutWhere and CutRun: the text's literals of whole statements.
func TestB3bCuts(t *testing.T) {
	sameAsText(t, b3bSample, func(s []byte) ([]byte, error) {
		return []byte(strings.Replace(string(s), "\n    a = 5;\n", "\n", 1)), nil
	}, func(v *Verbs) {
		v.CutWhere("(= a 5)", func(x *Node) bool { return v.Editor().Parent(x).Is("defn") }, 1, "the function's own")
	})
	sameAsText(t, b3bSample, func(s []byte) ([]byte, error) {
		return []byte(strings.Replace(string(s), "    a = 6;\n    {\n        a = 5;\n    }\n", "", 1)), nil
	}, func(v *Verbs) { v.CutRun("two", "(= a 6)", "(block (= a 5))") })
	refuses(t, b3bSample, func(v *Verbs) { v.CutRun("not a run", "(= a 6)", "(= a 5)") }, "the run of 2 items occurs 0 times")
	refuses(t, b3bSample, func(v *Verbs) { v.CutRun("twice", "(= a 5)") }, "the run of 1 items occurs 2 times")
}

// Muted: the acts made, nothing reported, a refusal carried.
func TestB3bMuted(t *testing.T) {
	_, _, v, log := verbsOn(t, b3bSample)
	v.Muted(func(v *Verbs) { v.Cut("(= a 6)", 1, "said by nobody") })
	if err := v.Done(); err != nil || log.Len() != 0 || v.Count("(= a 6)") != 0 {
		t.Fatalf("err %v, log %q", err, log.String())
	}
	refuses(t, b3bSample, func(v *Verbs) {
		v.Muted(func(v *Verbs) { v.Cut("(= a 7)", 1, "nothing") })
	}, "nothing -- matched 0 times, expected 1")
}

// DropArgPure: an argument with a call dropped through `...` when the cut
// names the callee as free of side effects, and refused otherwise.
func TestB3bDropArgPure(t *testing.T) {
	sameAsText(t, b3bSample, func(s []byte) ([]byte, error) {
		return []byte(strings.Replace(string(s), `vf("%d %d", a, k(a));`, `vf("%d %d", a);`, 1)), nil
	}, func(v *Verbs) {
		c := v.One(`(call vf _*)`, "the call")
		if err := v.Editor().DropArgPure(c, 2, "k"); err != nil {
			v.Die("%v", err)
		}
	})
	_, _, v, _ := verbsOn(t, b3bSample)
	c := v.One(`(call vf _*)`, "the call")
	if err := v.Editor().DropArg(c, 2); err == nil || !strings.Contains(err.Error(), "side effect") {
		t.Errorf("DropArg: %v", err)
	}
	if err := v.Editor().DropArgPure(c, 2, "vf"); err == nil || !strings.Contains(err.Error(), "side effect") {
		t.Errorf("DropArgPure naming another function: %v", err)
	}
	if err := v.Editor().DropArgPure(c, 0, "k"); err == nil || !strings.Contains(err.Error(), "through `...`") {
		t.Errorf("DropArgPure of a named parameter: %v", err)
	}
}
