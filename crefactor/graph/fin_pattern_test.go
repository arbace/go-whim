package graph

import (
	"io"
	"testing"
)

const finPatternSrc = `int use(int);
int *get(void);
int main(void)
{
    int *p = get();
    int *q = get();
    if (p == nullptr)
    {
        return 1;
    }
    if (q != nullptr)
    {
        use(*q);
    }
    if (use(0) == 0)
    {
        return 2;
    }
    return use((*p)) + use(*q);
}
`

// TestFinShapedPattern: `?name:P` on the graph's nodes -- the node bound
// is the graph's own, found only where P matches it, and a rewrite at a
// shaped binding replaces that node alone.
func TestFinShapedPattern(t *testing.T) {
	e, _, path := b2c(t, finPatternSrc)
	v := NewVerbs("t", e, io.Discard)
	var cs, args []*Node
	v.InFunction("main", func(v *Verbs) {
		cs = v.Query("(if ?c:(== _ nullptr) _*)", "c")
		args = v.Query("(call use ?a:(paren _))", "a")
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || !cs[0].Is("==") || cs[0].Kids[1].Atom != "p" || cs[0].ID == 0 {
		t.Fatalf("the null test: %v", cs)
	}
	if len(args) != 1 || !args[0].Is("paren") {
		t.Fatalf("the parenthesised argument: %v", args)
	}
	v.InFunction("main", func(v *Verbs) {
		v.RewriteAt("(if ?c:(== ?x:_ nullptr) _*)", "c", "(! ?x)", 1, "a null test said with !")
		v.Rewrite("?m:(call use (paren ?e))", "(call use ?e)", 1, "parentheses an argument does not need")
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	r1Same(t, e, path, `int use(int);
int *get(void);
int main(void)
{
    int *p = get();
    int *q = get();
    if (!p)
    {
        return 1;
    }
    if (q != nullptr)
    {
        use(*q);
    }
    if (use(0) == 0)
    {
        return 2;
    }
    return use(*p) + use(*q);
}
`)
}
