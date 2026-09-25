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
		locals  *localFacts
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
		f.locals = factsOf(fd)
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
				if r == nil || !boolish(r, yes, f.locals, 0) {
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
	// and the locals that hold an answer: declared `int` alone, one declarator,
	// only ever given 0 or 1
	nLocals := 0
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.FunctionDefinition == nil || !inCore(ed) {
			continue
		}
		lf := factsOf(ed.FunctionDefinition)
		sweep.Walk(ed.FunctionDefinition.CompoundStatement, func(n cc.Node) bool {
			d, ok := n.(*cc.Declaration)
			if !ok || d.Case != cc.DeclarationDecl || d.InitDeclaratorList == nil || d.InitDeclaratorList.InitDeclaratorList != nil {
				return true
			}
			id := d.InitDeclaratorList.InitDeclarator
			tok, ok := onlyInt(d.DeclarationSpecifiers)
			if !ok || id == nil || id.Declarator.Pointer != nil || id.Declarator.DirectDeclarator.Case != cc.DirectDeclaratorIdent {
				return true
			}
			if lf.boolish(id.Declarator.Name(), yes, 0) {
				toks = append(toks, tok)
				nLocals++
			}
			return true
		})
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
	// ---- the callers: `f() == FAIL` is `!f()`, `f() != FAIL` is `f()` ------
	cmp := 0
	for pass := 0; pass < 4; pass++ {
		n, t2, err := rewriteComparisons(text, cut, yes)
		if err != nil {
			return nil, p.Die("%v", err)
		}
		if n == 0 {
			break
		}
		cmp += n
		text = t2
	}
	p.Say(fmt.Sprintf("%d comparisons of an answer with OK, FAIL, TRUE or FALSE are the answer, or its negation", cmp))
	p.Say(fmt.Sprintf("%d functions answer a question and return bool now, %d declarations retyped: %s",
		len(names), len(toks), strings.Join(names[:min(len(names), 12)], ", ")+"..."))
	p.Say(fmt.Sprintf("%d locals that only ever hold an answer are bool too", nLocals))
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
func boolish(e cc.ExpressionNode, yes map[string]bool, lf *localFacts, depth int) bool {
	if depth > 8 {
		return false // a local assigned from itself, round about
	}
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			s := x.Token.SrcStr()
			switch s {
			case "TRUE", "FALSE", "true", "false", "OK", "FAIL":
				return true // OK is 1 and FAIL 0: success is true
			}
			return lf.boolish(s, yes, depth)
		case cc.PrimaryExpressionExpr:
			return boolish(x.ExpressionList, yes, lf, depth)
		}
	case *cc.ExpressionList:
		return x.ExpressionList == nil && boolish(x.AssignmentExpression, yes, lf, depth)
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
		return x.Case == cc.ConditionalExpressionCond && boolish(x.ExpressionList, yes, lf, depth) && boolish(x.ConditionalExpression, yes, lf, depth)
	case *cc.PostfixExpression:
		if x.Case == cc.PostfixExpressionCall {
			if pe, ok := x.PostfixExpression.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent {
				return yes[pe.Token.SrcStr()]
			}
		}
	}
	return false
}

// localFacts is what a function does with each of its locals, by name: how
// many declarations the name has, every value assigned to it (its initializer
// included), and whether anything else touches it -- its address taken, ++,
// --, a compound assignment, a braced initializer.
type localFacts struct {
	decls   map[string]int
	assigns map[string][]cc.ExpressionNode
	bad     map[string]bool
}

func factsOf(fd *cc.FunctionDefinition) *localFacts {
	lf := &localFacts{decls: map[string]int{}, assigns: map[string][]cc.ExpressionNode{}, bad: map[string]bool{}}
	ident := func(e cc.ExpressionNode) string {
		for {
			switch x := e.(type) {
			case *cc.PrimaryExpression:
				switch x.Case {
				case cc.PrimaryExpressionIdent:
					return x.Token.SrcStr()
				case cc.PrimaryExpressionExpr:
					e = x.ExpressionList
					continue
				}
			case *cc.ExpressionList:
				if x.ExpressionList == nil {
					e = x.AssignmentExpression
					continue
				}
			}
			return ""
		}
	}
	sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
		switch x := n.(type) {
		case *cc.InitDeclarator:
			nm := x.Declarator.Name()
			lf.decls[nm]++
			if x.Initializer != nil {
				if x.Initializer.Case == cc.InitializerExpr {
					lf.assigns[nm] = append(lf.assigns[nm], x.Initializer.AssignmentExpression)
				} else {
					lf.bad[nm] = true
				}
			}
		case *cc.AssignmentExpression:
			if nm := ident(x.UnaryExpression); nm != "" {
				if x.Case == cc.AssignmentExpressionAssign {
					lf.assigns[nm] = append(lf.assigns[nm], x.AssignmentExpression)
				} else if x.Case != cc.AssignmentExpressionCond {
					lf.bad[nm] = true
				}
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionAddrof, cc.UnaryExpressionInc, cc.UnaryExpressionDec:
				if nm := ident(x.CastExpression); nm != "" {
					lf.bad[nm] = true
				}
				if nm := ident(x.UnaryExpression); nm != "" {
					lf.bad[nm] = true
				}
			}
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
				if nm := ident(x.PostfixExpression); nm != "" {
					lf.bad[nm] = true
				}
			}
		}
		return true
	})
	return lf
}

// boolish says the local nm holds only 0 or 1: it is declared once in the
// function, touched only by plain assignment, and every value it is given is.
func (lf *localFacts) boolish(nm string, yes map[string]bool, depth int) bool {
	if lf == nil || lf.decls[nm] != 1 || lf.bad[nm] || len(lf.assigns[nm]) == 0 {
		return false
	}
	for _, e := range lf.assigns[nm] {
		if !boolish(e, yes, lf, depth+1) {
			return false
		}
	}
	return true
}

// rewriteComparisons makes one pass: every comparison in the core of an
// answer -- an expression boolish says holds 0 or 1 -- with OK, FAIL, TRUE or
// FALSE becomes the answer or its negation.  Only the outermost of nested
// ones is rewritten in a pass; the caller parses again for the rest.
func rewriteComparisons(text []byte, cut int, yes map[string]bool) (int, []byte, error) {
	const path = "whim-vim.c"
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return 0, nil, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: text},
	})
	if err != nil {
		return 0, nil, fmt.Errorf("the text after retyping does not parse: %v", err)
	}
	constOf := func(e cc.ExpressionNode) (string, bool) {
		pe, ok := e.(*cc.PrimaryExpression)
		if !ok || pe.Case != cc.PrimaryExpressionIdent {
			return "", false
		}
		switch s := pe.Token.SrcStr(); s {
		case "OK", "TRUE", "true":
			return "yes", true
		case "FAIL", "FALSE", "false":
			return "no", true
		}
		return "", false
	}
	type rw struct {
		a, z int
		with string
	}
	var rws []rw
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		x, ok := n.(*cc.EqualityExpression)
		if !ok || x.Case == cc.EqualityExpressionRel {
			return true
		}
		if pos := x.Position(); pos.Filename != path || pos.Offset >= cut {
			return true
		}
		var e cc.ExpressionNode
		k, isConst := constOf(x.RelationalExpression)
		if isConst {
			e = x.EqualityExpression
		} else if k, isConst = constOf(x.EqualityExpression); isConst {
			e = x.RelationalExpression
		}
		if !isConst || !boolish(e, yes, nil, 0) {
			return true
		}
		a, z := sweep.Span(x, path, text)
		ea, ez := sweep.Span(e, path, text)
		s := string(text[ea:ez])
		negate := (k == "no") == (x.Case == cc.EqualityExpressionEq)
		if negate {
			if _, call := e.(*cc.PostfixExpression); call {
				s = "!" + s
			} else if pe, ok := e.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionExpr {
				s = "!" + s
			} else {
				s = "!(" + s + ")"
			}
		}
		rws = append(rws, rw{a, z, s})
		return false // nested ones wait for the next pass
	})
	sort.Slice(rws, func(i, j int) bool { return rws[i].a > rws[j].a })
	for _, r := range rws {
		text = append(append(append([]byte{}, text[:r.a]...), r.with...), text[r.z:]...)
	}
	return len(rws), text, nil
}
