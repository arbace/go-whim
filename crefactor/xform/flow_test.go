package xform

import (
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

// GotoTail with a tail of no statements, GotoReturn's rule before this took
// it: a goto to a label that returns is that return; one whose value could be
// shadowed at the goto is held, and so is its label.  A label's return that
// only the gotos reached goes with it; one after a labeled statement stays,
// for the dead-statement rule (crefactor/graph's Editor.DeadStmt) to take.
func TestGotoTailNoStatements(t *testing.T) {
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
	got := run(t, GotoTail(GotoTailKnobs{}), src)
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
