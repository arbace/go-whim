package graph

import (
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

const runSample = `int g(int);
int
f(int a, int b)
{
    switch (a)
    {
    case 1:
        {
            b = 2;
            continue_here:
            b++;
        }
    case 2:
        {
            b = 3;
            break;
        }
    case 3:
        b = g(b);
        b++;
        break;
    }
    b = 1;
    a = 2;
    if (a)
    {
        return 1;
    }
    a = 3;
    return a;
}
`

// TestDropCaseRun: a case after a block that ends in a break is cut with
// its run, where DropCase refuses; a case after a block that does not end
// in a jump is refused by both.
func TestDropCaseRun(t *testing.T) {
	sameAsText(t, runSample, func(b []byte) ([]byte, error) {
		return edit.ReplaceLiteral(b, "    case 3:\n        b = g(b);\n        b++;\n        break;\n", "", 1)
	}, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.DropCaseRun("(case 3)", 1, "case 3") })
	})
	refuses(t, runSample, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.DropCase("(case 3)", 1, "case 3") })
	}, "falls into it")
	refuses(t, runSample, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.DropCaseRun("(case 2)", 1, "case 2") })
	}, "falls into it")
}

// TestSpliceFirst: from one item through the first after it of a shape.
func TestSpliceFirst(t *testing.T) {
	sameAsText(t, runSample, func(b []byte) ([]byte, error) {
		return edit.ReplaceLiteral(b, "    b = 1;\n    a = 2;\n    if (a)\n    {\n        return 1;\n    }\n", "    b = 4;\n", 1)
	}, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.SpliceFirst("(= b 1)", "(if _*)", "(= b 4)", "the run") })
	})
	refuses(t, runSample, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.SpliceFirst("(= b 1)", "(while _*)", "", "the run") })
	}, "no item after")
}

// TestRun: consecutive items by their patterns, and the refusals.
func TestRun(t *testing.T) {
	_, _, v, _ := verbsOn(t, runSample)
	v.InFunction("f", func(v *Verbs) {
		if r := v.Run("the run", "(= b 1)", "(= a 2)", "(if a _)"); len(r) != 3 {
			t.Errorf("run of %d", len(r))
		}
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	refuses(t, runSample, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.Run("the run", "(= b 1)", "(= a 3)") })
	}, "occurs 0 times")
	sameAsText(t, runSample, func(b []byte) ([]byte, error) {
		return edit.ReplaceLiteral(b, "    b = 1;\n    a = 2;\n", "", 1)
	}, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.CutRun("the run", "(= b 1)", "(= a 2)") })
	})
	refuses(t, runSample, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.Run("the run", "(post++ b)") })
	}, "occurs 2 times")
}

// TestDropOperandAsText: the operand left keeps the parentheses it stood
// in, as the text's cut of the other leaves them; where it needed none, as
// DropOperand.
func TestDropOperandAsText(t *testing.T) {
	const src = `int g(int);
int
f(int a, int b)
{
    while ((a = g(a), a > 1) && b)
    {
        b--;
    }
    if (a && b > 2)
    {
        return 1;
    }
    return 0;
}
`
	sameAsText(t, src, func(b []byte) ([]byte, error) {
		b, err := edit.ReplaceLiteral(b, ") && b)", "))", 1)
		if err != nil {
			return nil, err
		}
		return edit.ReplaceLiteral(b, "a && b > 2", "b > 2", 1)
	}, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) {
			v.DropOperandAsText("b", 1, "the comma's partner")
			v.DropOperandAsText("a", 1, "a plain operand")
		})
	})
}

// TestHeadFold: the head `if (C)` folds the plain ifs and not the else-if
// arm of the same condition, `else if (C)` the arm alone; a count of -1 is
// every one, inner first, reported with how many.
func TestHeadFold(t *testing.T) {
	const src = `int
f(int a, int b)
{
    if (a)
    {
        b = 1;
        if (a)
        {
            b = 2;
        }
    }
    else if (b)
    {
        b = 3;
    }
    if (b)
    {
        a = 4;
    }
    return a;
}
`
	_, _, v, log := verbsOn(t, src)
	v.InFunction("f", func(v *Verbs) { v.HeadFold("never", false, "a", -1, "the a ifs") })
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	if got := log.String(); got != "  tiny         the a ifs (2)\n" {
		t.Errorf("report %q", got)
	}
	refuses(t, src, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.HeadFold("never", false, "b", 2, "the b ifs") })
	}, "matched 1 times, expected 2")
	sameAsText(t, src, func(b []byte) ([]byte, error) {
		return edit.ReplaceLiteral(b, "    else if (b)\n    {\n        b = 3;\n    }\n", "", 1)
	}, func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) { v.HeadFold("never", true, "b", 1, "the b arm") })
	})
}
