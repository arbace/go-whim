package graph

import (
	"bytes"
	"strings"
	"testing"
)

// crefactor/xform's TestUnions, which the text step was held to, on a
// graph read back from its Lisp: a tagged value's one-member union is that
// member, the two-member one stays; and a `.` chain (`w.v.str.s`) loses
// the one member from the same chain.
const unionSample = `struct value
{
    int tag;
    union
    {
        long i;
        double d;
    } num;
    union
    {
        char *s;
    } str;
};

struct wrap
{
    struct value v;
};

long get(struct value *v, struct wrap w)
{
    if (v->tag)
    {
        return v->num.i;
    }
    return v->str.s[0] + w.v.str.s[1];
}

#include <stdio.h>
`

func TestDegenerateUnions(t *testing.T) {
	path, _, e := fragOn(t, unionSample)
	var log bytes.Buffer
	v := NewVerbs("unions", e, &log)
	v.DegenerateUnions(1, 1)
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	want := `struct value
{
    int tag;
    union
    {
        long i;
        double d;
    } num;
    char *str;
};

struct wrap
{
    struct value v;
};

    long
get(struct value *v, struct wrap w)
{
    if (v->tag)
    {
        return v->num.i;
    }
    return v->str[0] + w.v.str[1];
}

#include <stdio.h>
`
	asImported(t, path, e, []byte(want))
	if !strings.Contains(log.String(), "str 1 + 2") {
		t.Errorf("the report:\n%s", log.String())
	}
}

func TestDegenerateUnionsRefusals(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{unionSample, "at least 1 and 2"},
		{strings.Replace(unionSample, "return v->str.s[0]", "return sizeof v->str", 1), "neither its own declaration"},
		{strings.Replace(unionSample, "if (v->tag)", "if (v->tag && \"str\"[0])", 1), "neither its own declaration"},
	} {
		_, _, e := fragOn(t, c.src)
		v := NewVerbs("unions", e, &bytes.Buffer{})
		g := 1
		if c.want == "at least 1 and 2" {
			g = 2
		}
		v.DegenerateUnions(1, g)
		if err := v.Done(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v", c.want, err)
		}
	}
}
