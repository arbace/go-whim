package xform

import (
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

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
