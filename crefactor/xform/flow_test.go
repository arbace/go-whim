package xform

import (
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

// A small parser's code, not any one code base's: what follows a jump goes up
// to the next label, a run with a declaration is held, and a dead run inside a
// dead block is cut once.  A case's own statement is labeled, and a labeled
// statement is not a jump: what follows `case 1: return 10;` stays.
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
`
	got := run(t, DeadStmt(), src)
	same(t, got, want)
	compiles(t, got)
	refuses(t, DeadStmt(), src, "unexpected argument", "--at-least", "3")
}

// A goto to a label that returns is that return; one whose value could be
// shadowed at the goto is held, and so is its label.  A label's return that
// only the gotos reached goes with it; one after a labeled statement stays,
// for DeadStmt to take.
func TestGotoReturn(t *testing.T) {
	src := `int scan(const char *s)
{
    int n = 0;
    if (!s)
        goto fail;
    while (*s)
    {
        if (*s == '#')
            goto done;
        n++;
        s++;
    }
    goto done;
fail:
    return -1;
done:
    return n;
}

int shadow(int k)
{
    int r = k;
    if (k > 2)
    {
        int r = 0;
        goto out;
    }
out:
    return r;
}
`
	want := `int scan(const char *s)
{
    int n = 0;
    if (!s)
        return -1;
    while (*s)
    {
        if (*s == '#')
            return n;
        n++;
        s++;
    }
    return n;

return n;
}

int shadow(int k)
{
    int r = k;
    if (k > 2)
    {
        int r = 0;
        goto out;
    }
out:
    return r;
}
`
	got := run(t, GotoReturn(), src)
	same(t, got, want)
	compiles(t, got)
}

func TestStmtTerminates(t *testing.T) {
	ast, err := parse([]byte("void f(int a) { if (a) return; else { a++; return; } }\nvoid g(int a) { if (a) return; }\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false}
	i := 0
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.FunctionDefinition == nil || ed.Position().Filename != file {
			continue
		}
		fd := ed.FunctionDefinition
		if fd.CompoundStatement.BlockItemList == nil {
			continue
		}
		var last *cc.BlockItem // the parser puts __func__ first
		for l := fd.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
			last = l.BlockItem
		}
		if got := Terminates(last); got != want[i] {
			t.Errorf("function %d: Terminates = %v, want %v", i, got, want[i])
		}
		i++
	}
}
