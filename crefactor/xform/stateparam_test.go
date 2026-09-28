package xform

import (
	"bytes"
	"strings"
	"testing"
)

const stateParamSrc = `typedef struct { int n; int *p; } pair;
static int depth;
static pair cur = { 0, nullptr };
static int calls;
static int leaf(int x);
static int leaf(int x)
{
    calls++;
    return x + cur.n;
}
static void mid(void)
{
    ++depth;
    cur.n = leaf(depth);
}
int top(int x)
{
    mid();
    return leaf(x) + depth;
}
int other(void)
{
    return top(1);
}
`

var stateParamKnobs = StateParamKnobs{
	Objects: []string{"depth", "cur", "calls"},
	Type:    "st_T", Instance: "st0", Param: "s",
	Roots: []string{"top"},
}

// StateParam makes the objects one struct's members, hands its pointer to
// every function below the root that needs it -- the prototype too -- and
// has the root bind it to the instance; nothing above the root changes.
func TestStateParam(t *testing.T) {
	var log bytes.Buffer
	out, err := StateParam(stateParamKnobs)([]byte(stateParamSrc), []string{"--at-least", "2"}, &log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"static int leaf(st_T *s, int x);",
		"leaf(st_T *s, int x)\n{\n    s->calls++;\n    return x + s->cur.n;",
		"mid(st_T *s)\n{\n    ++s->depth;\n    s->cur.n = leaf(s, s->depth);",
		"int top(int x)\n{\n    st_T *s = &st0;\n    mid(s);\n    return leaf(s, x) + s->depth;",
		"return top(1);",
		"static st_T st0;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s\n%s", want, got, log.String())
		}
	}
	for _, gone := range []string{"static int depth;", "static int calls;"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q is still there:\n%s", gone, got)
		}
	}
}

// It refuses what it cannot make a parameter: a function that needs the
// state with its address taken; an object with an initialiser that is not
// zero; a name it adds that is taken; a need no root bounds.
func TestStateParamRefuses(t *testing.T) {
	for _, c := range []struct{ name, src, why string }{
		{"address", stateParamSrc + "void *fp = (void *)mid;\n", "address is taken"},
		{"init", strings.Replace(stateParamSrc, "static int calls;", "static int calls = 1;", 1), "not zero"},
		{"taken", strings.Replace(stateParamSrc, "int other(void)", "int st0;\nint other(void)", 1), "is taken"},
		{"unbounded", strings.Replace(stateParamSrc, "return top(1);", "return depth;", 1), "no root bounds it"},
	} {
		var log bytes.Buffer
		_, err := StateParam(stateParamKnobs)([]byte(c.src), nil, &log)
		if err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: want a refusal naming %q, got %v", c.name, c.why, err)
		}
	}
}
