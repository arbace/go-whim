package graph

import (
	"bytes"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// crefactor/xform's TestDeadStmt, which the text rule was held to, on a
// graph read back from its Lisp: what follows a jump goes up to the next
// label, a run with a declaration is held, and a dead run inside a dead
// block is cut once.  A case's own statement is labeled, and a labeled
// statement is not a jump: what follows `case 1: return 10;` stays.  The
// text's result, printed canonically, is the C view here.
func TestDeadStmt(t *testing.T) {
	src := `int peek(int c)
{
    switch (c)
    {
    case 1:
        return 10;
        c++;
    case 2:
        break;
        c--;
        c--;
    }
    if (c > 3)
    {
        return 1;
    }
    else
    {
        return 2;
    }
    c = 7;
    {
        return c;
    }
again:
    c++;
    return c;
    int held = 3;
    return held;
}

int nest(int c)
{
    return c;
    {
        c++;
        return c;
        c--;
    }
}
`
	want := `int peek(int c)
{
    switch (c)
    {
    case 1:
        return 10;
        c++;
    case 2:
        break;
        c--;
        c--;
    }
    if (c > 3)
    {
        return 1;
    }
    else
    {
        return 2;
    }
again:
    c++;
    return c;
    int held = 3;
    return held;
}

int nest(int c)
{
    return c;
}
`
	path, _, v, _ := verbsOn(t, src)
	canon, err := cemit.Canonical(path, []byte(want))
	if err != nil {
		t.Fatal(err)
	}
	cut, held, err := v.Editor().DeadStmt()
	if err != nil {
		t.Fatal(err)
	}
	if cut != 2 || held != 1 {
		t.Errorf("cut %d, held %d; want 2 and 1 (the run inside nest's dead block goes with it)", cut, held)
	}
	if err := v.Editor().Check(); err != nil {
		t.Fatal(err)
	}
	got, err := v.Editor().Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, canon) {
		t.Errorf("%s\ngot:\n%s\nwant:\n%s", firstDiff(got, canon), got, canon)
	}
}

// crefactor/xform's TestStmtTerminates: an if whose branches both jump
// terminates, one with no else does not; and a statement a label stands
// before is a labeled statement, which does not.
func TestStmtTerminates(t *testing.T) {
	src := "void f(int a)\n{\n    if (a)\n    {\n        return;\n    }\n    else\n    {\n        a++;\n        return;\n    }\n}\n\n" +
		"void g(int a)\n{\n    if (a)\n    {\n        return;\n    }\n}\n\n" +
		"void h(int a)\n{\n    a++;\nl:\n    return;\n}\n"
	_, _, v, _ := verbsOn(t, src)
	for i, name := range []string{"f", "g", "h"} {
		items := Body(v.Editor().Defn(name))
		want := i == 0
		if got := ItemTerminates(items, len(items)-1); got != want {
			t.Errorf("%s: its last item terminates = %v, want %v", name, got, want)
		}
	}
}
