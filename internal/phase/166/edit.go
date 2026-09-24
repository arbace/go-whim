package p166

// Whim phase 166 -- a question returns bool.  See GOAL.md.

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/sweep"
)

func init() { edit.Register("whim166", Edit) }

// Edit retypes `int` to `bool` for every core function that answers a
// question: each of its returns is TRUE, FALSE, a comparison, a logical
// && || !, a ?: of those, or a call to another such function -- found to a
// fixpoint.  Its definition and every prototype change; nothing else does.
//
// It changes no value.  Every return is 0 or 1 already, and a bool converts
// to exactly that wherever an int is wanted, so `count += f()` and
// `x = f()` read the same numbers.  A function whose name is used as
// anything but the callee of a call is left alone (a table or a pointer to it
// wants the type it has), and so is one the host names.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "boolret", W: w}
	cut := bytes.Index(text, []byte("\n#include "))
	if cut < 0 {
		return nil, p.Die("no #include: the core/host line is not where this phase expects it")
	}
	host := text[cut:]
	const path = "whim-vim.c"
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: text},
	})
	if err != nil {
		return nil, p.Die("the input does not parse: %v", err)
	}
	inCore := func(n cc.Node) bool {
		pos := n.Position()
		return pos.Filename == path && pos.Offset < cut
	}

	type fn struct {
		name    string
		intTok  []cc.Token // the `int` of the definition and of every prototype
		returns []cc.ExpressionNode
		decls   int
	}
	fns := map[string]*fn{}
	// the definitions: `int` alone for a type, no pointer
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.FunctionDefinition == nil || !inCore(ed) {
			continue
		}
		fd := ed.FunctionDefinition
		tok, ok := onlyInt(fd.DeclarationSpecifiers)
		if !ok || fd.Declarator.Pointer != nil {
			continue
		}
		f := &fn{name: fd.Declarator.Name(), intTok: []cc.Token{tok}}
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			if j, ok := n.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementReturn {
				f.returns = append(f.returns, j.ExpressionList)
			}
			return true
		})
		fns[f.name] = f
	}
	delete(fns, "main")
	// the prototypes, each of one declarator
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Declaration == nil || ed.Declaration.Case != cc.DeclarationDecl || !inCore(ed) {
			continue
		}
		d := ed.Declaration
		l := d.InitDeclaratorList
		if l == nil || l.InitDeclarator == nil {
			continue
		}
		f := fns[l.InitDeclarator.Declarator.Name()]
		if f == nil {
			continue
		}
		tok, ok := onlyInt(d.DeclarationSpecifiers)
		if !ok || l.InitDeclaratorList != nil || l.InitDeclarator.Declarator.Pointer != nil {
			delete(fns, f.name) // a prototype this cannot retype alone
			continue
		}
		f.intTok = append(f.intTok, tok)
	}
	// a name used as anything but a callee, or named by the host, stays int
	callee := map[*cc.PrimaryExpression]bool{}
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		if c, ok := n.(*cc.PostfixExpression); ok && c.Case == cc.PostfixExpressionCall {
			if pe, ok := c.PostfixExpression.(*cc.PrimaryExpression); ok {
				callee[pe] = true
			}
		}
		return true
	})
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		if pe, ok := n.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && !callee[pe] {
			delete(fns, pe.Token.SrcStr())
		}
		return true
	})
	for name := range fns {
		if edit.MentionCount(host, name) > 0 {
			delete(fns, name)
		}
	}
	// the fixpoint: a function is a question when all its returns are
	yes := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for name, f := range fns {
			if yes[name] || len(f.returns) == 0 {
				continue
			}
			all := true
			for _, r := range f.returns {
				if r == nil || !boolish(r, yes) {
					all = false
					break
				}
			}
			if all {
				yes[name] = true
				grew = true
			}
		}
	}
	names := make([]string, 0, len(yes))
	var toks []cc.Token
	for name := range yes {
		names = append(names, name)
		toks = append(toks, fns[name].intTok...)
	}
	sort.Strings(names)
	sort.Slice(toks, func(i, j int) bool { return toks[i].Position().Offset > toks[j].Position().Offset })
	for _, t := range toks {
		o := t.Position().Offset
		if string(text[o:o+3]) != "int" {
			return nil, p.Die("the `int` of a retyped function is not at %d", o)
		}
		text = append(append(append([]byte{}, text[:o]...), "bool"...), text[o+3:]...)
	}
	p.Say(fmt.Sprintf("%d functions answer a question and return bool now, %d declarations retyped: %s",
		len(names), len(toks), strings.Join(names[:min(len(names), 12)], ", ")+"..."))
	return text, nil
}

// onlyInt is the `int` token of a specifier list whose one type specifier is
// `int` -- storage classes and inline beside it are fine, a qualifier or
// another type word is not.
func onlyInt(ds *cc.DeclarationSpecifiers) (cc.Token, bool) {
	var tok cc.Token
	n := 0
	for ; ds != nil; ds = ds.DeclarationSpecifiers {
		switch ds.Case {
		case cc.DeclarationSpecifiersTypeSpec:
			n++
			if ds.TypeSpecifier.Case != cc.TypeSpecifierInt {
				return tok, false
			}
			tok = ds.TypeSpecifier.Token
		case cc.DeclarationSpecifiersTypeQual:
			return tok, false
		}
	}
	return tok, n == 1
}

// boolish says e's value is 0 or 1 by its form.
func boolish(e cc.ExpressionNode, yes map[string]bool) bool {
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			s := x.Token.SrcStr()
			return s == "TRUE" || s == "FALSE" || s == "true" || s == "false"
		case cc.PrimaryExpressionExpr:
			return boolish(x.ExpressionList, yes)
		}
	case *cc.ExpressionList:
		return x.ExpressionList == nil && boolish(x.AssignmentExpression, yes)
	case *cc.EqualityExpression:
		return x.Case != cc.EqualityExpressionRel
	case *cc.RelationalExpression:
		return x.Case != cc.RelationalExpressionShift
	case *cc.LogicalAndExpression:
		return x.Case == cc.LogicalAndExpressionLAnd
	case *cc.LogicalOrExpression:
		return x.Case == cc.LogicalOrExpressionLOr
	case *cc.UnaryExpression:
		return x.Case == cc.UnaryExpressionNot
	case *cc.ConditionalExpression:
		return x.Case == cc.ConditionalExpressionCond && boolish(x.ExpressionList, yes) && boolish(x.ConditionalExpression, yes)
	case *cc.PostfixExpression:
		if x.Case == cc.PostfixExpressionCall {
			if pe, ok := x.PostfixExpression.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent {
				return yes[pe.Token.SrcStr()]
			}
		}
	}
	return false
}
