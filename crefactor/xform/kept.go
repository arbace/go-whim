package xform

import (
	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// What fallout.go asks that the typed transforms defined before they moved
// to the graph (step 6: plainc.go's funcName and hasLabel, boolret.go's
// paramDecls).

// funcName says the block item is the front end's __func__.
func funcName(it *cc.BlockItem) bool {
	if it.Case != cc.BlockItemDecl || it.Declaration == nil {
		return false
	}
	l := it.Declaration.InitDeclaratorList
	return l != nil && l.InitDeclaratorList == nil && l.InitDeclarator.Declarator != nil && l.InitDeclarator.Declarator.Name() == "__func__"
}

// hasLabel says s holds a label or a case: a way in from outside it.
func hasLabel(s cc.Node) bool {
	found := false
	sweep.Walk(s, func(n cc.Node) bool {
		if _, ok := n.(*cc.LabeledStatement); ok {
			found = true
		}
		return !found
	})
	return found
}

// paramDecls is a function declarator's parameter declarations, in order;
// nil for a variadic one.  (BoolRet's, which moved to crefactor/graph; the
// closure and the typed transforms still ask it.)
func paramDecls(d *cc.Declarator) []*cc.ParameterDeclaration {
	dd := d.DirectDeclarator
	for dd != nil && dd.Case != cc.DirectDeclaratorFuncParam {
		dd = dd.DirectDeclarator
	}
	if dd == nil || dd.ParameterTypeList == nil || dd.ParameterTypeList.Case != cc.ParameterTypeListList {
		return nil
	}
	var out []*cc.ParameterDeclaration
	for l := dd.ParameterTypeList.ParameterList; l != nil; l = l.ParameterList {
		out = append(out, l.ParameterDeclaration)
	}
	return out
}

// translate type-checks text as one translation unit: the front's closure
// (FallOutOf) asks it every round.
func translate(text []byte) (*cc.AST, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	return cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: file, Value: text},
	})
}
func idByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
