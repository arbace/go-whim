package graph

import (
	"bytes"
	"strings"
	"testing"
)

const ownSample = `int abs(int n);

long labs(long n);

int puts(const char *s);

static int one(int x)
{
    return x;
}

static int f(int a, long b)
{
    puts("absolute");
    return abs(a) + (int)labs(b) + one(a);
}
#include <stddef.h>
`

var ownKnobs = OwnKnobs{
	Prefix: "my_",
	Funcs: []OwnFunc{
		{Name: "abs", Proto: "int abs(int n);", Def: "static int my_abs(int a)\n{\n    return a > 0 ? a : -a;\n}\n"},
		{Name: "labs", Proto: "long labs(long n);", Def: "static long my_labs(long a)\n{\n    return a > 0 ? a : -a;\n}\n"},
	},
	Before: "one",
}

// Own: the prototypes go, the definitions go before `one`, the calls are
// theirs by edge, and the string that spells both names is untouched; the
// graph is the import of its C view.
func TestOwn(t *testing.T) {
	path, _, e := fragOn(t, ownSample)
	var log bytes.Buffer
	v := NewVerbs("arith", e, &log)
	v.Own(ownKnobs, map[string]int{"abs": 1, "labs": 1})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	want := `int puts(const char *s);

    static int
my_abs(int a)
{
    return a > 0 ? a : -a;
}

    static long
my_labs(long a)
{
    return a > 0 ? a : -a;
}

    static int
one(int x)
{
    return x;
}

    static int
f(int a, long b)
{
    puts("absolute");
    return my_abs(a) + (int)my_labs(b) + one(a);
}

#include <stddef.h>
`
	asImported(t, path, e, []byte(want))
	if n := strings.Count(log.String(), "\n"); n != 4 {
		t.Errorf("the report is %d lines, the text's 4:\n%s", n, log.String())
	}
}

// Own's refusals: a literal that spells a name it owns, a count the plan states that the uses do not have, and
// a prototype that is not in the library block as written.
func TestOwnRefusals(t *testing.T) {
	_, _, e := fragOn(t, ownSample)
	v := NewVerbs("arith", e, &bytes.Buffer{})
	v.Own(ownKnobs, map[string]int{"abs": 2})
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "reached 1 `abs`") {
		t.Fatalf("a wrong count: %v", err)
	}
	_, _, e = fragOn(t, strings.Replace(ownSample, `"absolute"`, `"abs"`, 1))
	v = NewVerbs("arith", e, &bytes.Buffer{})
	v.Own(ownKnobs, nil)
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "PRINTS") {
		t.Fatalf("a literal spelling abs: %v", err)
	}
	_, _, e = fragOn(t, ownSample)
	k := ownKnobs
	k.Funcs = []OwnFunc{{Name: "abs", Proto: "int abs(int a);", Def: "static int my_abs(int a)\n{\n    return a;\n}\n"}}
	v = NewVerbs("arith", e, &bytes.Buffer{})
	v.Own(k, nil)
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "library declaration block") {
		t.Fatalf("a prototype not as written: %v", err)
	}
}
