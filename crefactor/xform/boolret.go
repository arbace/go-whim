package xform

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// BoolRetKnobs is what BoolRet is told.
type BoolRetKnobs struct {
	// Core is where the core ends; only the core is retyped, and a name the
	// host mentions stays int.
	Core Core
	// True and False are the code base's named truth constants, beside C's
	// true and false: an answer is one of them, a comparison, a logical
	// operator or another answer.  A literal 0 or 1 is not an answer.
	True, False []string
	// Keep are functions never retyped, whatever they return: main.
	Keep []string
	// Layout is what the sweep is told about struct layouts: a member a
	// positional initialiser fills stays int (sweep.PositionalMembers).
	Layout sweep.Options
	// Globals takes the core's file-scope objects too: an `int` object that
	// only ever holds an answer is bool (globalCandidates).
	Globals bool
	// Relax takes what the rule of phases 166 and 183 left int: the fixed
	// point is the greatest (a flag saved in a local and restored from it
	// is an answer), a literal 0 or 1 assigned to a file-scope object is
	// one, and so is `x |= E` or `x &= E` of an answer E into one, which is
	// then written `x = E || x` (`&&`): E evaluated as before, x read.
	Relax bool
}

// boolRet is one run's knobs, as sets.
type boolRet struct {
	k        BoolRetKnobs
	compound []compoundAssign // Relax: the |= and &= of an answer into a candidate
	yes      map[string]bool  // the constants that are true
	no       map[string]bool  // the constants that are false
}

func (b *boolRet) answer(s string) bool { return b.yes[s] || b.no[s] }

// BoolRet is the step that retypes `int` to `bool` for every core function
// that answers a question: each of its returns is a truth constant, a
// comparison, a logical && || !, a ?: of those, or a call to another such
// function -- found to a fixpoint.  Its definition and every prototype
// change; so do the locals, members and parameters that only ever hold an
// answer; and a comparison of an answer with a truth constant or 0 becomes
// the answer or its negation.
//
// It changes no value.  Every return is 0 or 1 already, and a bool converts
// to exactly that wherever an int is wanted.  A function whose name is used as
// anything but the callee of a call is left alone (a table or a pointer to it
// wants the type it has), and so is one the host names or k.Keep does.  It
// takes no arguments.
func BoolRet(k BoolRetKnobs) Step {
	b := &boolRet{k: k, yes: map[string]bool{"true": true}, no: map[string]bool{"false": true}}
	for _, s := range k.True {
		b.yes[s] = true
	}
	for _, s := range k.False {
		b.no[s] = true
	}
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "boolret", W: w}
		if _, err := flags(p.Tag, args); err != nil {
			return nil, err
		}
		cut := k.Core(text)
		if cut < 0 {
			return nil, p.Die("the core does not end where this step was told it does")
		}
		return b.run(p, text, cut)
	}
}

func (b *boolRet) run(p edit.Ph, text []byte, cut int) ([]byte, error) {
	b.compound = nil
	const path = file
	host := text[cut:]
	ast, err := parse(text)
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
		f.locals = b.factsOf(fd)
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			if j, ok := n.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementReturn {
				f.returns = append(f.returns, j.ExpressionList)
			}
			return true
		})
		fns[f.name] = f
	}
	for _, name := range b.k.Keep {
		delete(fns, name)
	}
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
	// plain: every core function called by name alone, for its parameters
	plain := map[string]bool{}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed != nil && ed.FunctionDefinition != nil && inCore(ed) {
			plain[ed.FunctionDefinition.Declarator.Name()] = true
		}
	}
	for _, name := range b.k.Keep {
		delete(plain, name)
	}
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		if pe, ok := n.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && !callee[pe] {
			delete(fns, pe.Token.SrcStr())
			delete(plain, pe.Token.SrcStr())
		}
		return true
	})
	for name := range plain {
		if edit.MentionCount(host, name) > 0 {
			delete(fns, name)
			delete(plain, name)
		}
	}
	params := b.paramCandidates(ast, path, cut, plain)
	// the members that may hold an answer, and every value assigned to each
	fields := b.memberCandidates(ast, path, text, cut, host)
	var globals map[string]*memberFacts
	if b.k.Globals {
		globals = b.globalCandidates(ast, path, cut, host)
	}
	// the fixpoint: a function is a question when all its returns are, and a
	// member, a parameter, a file-scope object holds one when every value it
	// is given is.  Each candidate is a check against what is known.
	allAnswers := func(as []assign, yes map[string]bool) bool {
		for _, a := range as {
			if !b.boolish(a.e, yes, a.lf, 0) {
				return false
			}
		}
		return true
	}
	cands := map[string]func(map[string]bool) bool{}
	for k, pc := range params {
		if !pc.bad && len(pc.assigns) > 0 {
			as := pc.assigns
			cands[k] = func(yes map[string]bool) bool { return allAnswers(as, yes) }
		}
	}
	for m, fc := range fields {
		if !fc.bad && len(fc.assigns) > 0 {
			as := fc.assigns
			cands["."+m] = func(yes map[string]bool) bool { return allAnswers(as, yes) }
		}
	}
	for g, gc := range globals {
		if !gc.bad && len(gc.assigns) > 0 {
			as := gc.assigns
			cands["g:"+g] = func(yes map[string]bool) bool { return allAnswers(as, yes) }
		}
	}
	for name, f := range fns {
		if len(f.returns) > 0 {
			f := f
			cands[name] = func(yes map[string]bool) bool {
				for _, r := range f.returns {
					if r == nil || !b.boolish(r, yes, f.locals, 0) {
						return false
					}
				}
				return true
			}
		}
	}
	yes := map[string]bool{}
	if !b.k.Relax {
		// the least: nothing is an answer until shown one
		for grew := true; grew; {
			grew = false
			for k, check := range cands {
				if !yes[k] && check(yes) {
					yes[k] = true
					grew = true
				}
			}
		}
	} else {
		// the greatest: every candidate is one until shown not -- a flag
		// saved in a local and restored from it holds only what it held
		for k := range cands {
			yes[k] = true
		}
		for shrank := true; shrank; {
			shrank = false
			for k := range yes {
				if !cands[k](yes) {
					delete(yes, k)
					shrank = true
				}
			}
		}
	}
	nFields := 0
	var toks []cc.Token
	nParams := 0
	for k, pc := range params {
		if yes[k] {
			toks = append(toks, pc.toks...)
			nParams++
		}
	}
	for m, fc := range fields {
		if yes["."+m] {
			toks = append(toks, fc.toks...)
			nFields++
		}
	}
	nGlobals := 0
	for g, gc := range globals {
		if yes["g:"+g] {
			toks = append(toks, gc.toks...)
			nGlobals++
		}
	}
	names := make([]string, 0, len(yes))
	for name := range yes {
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "p:") || strings.HasPrefix(name, "g:") {
			continue // a member or a parameter, retyped above
		}
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
		lf := b.factsOf(ed.FunctionDefinition)
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
	type edit1 struct {
		a, z int
		s    string
	}
	var edits []edit1
	for _, t := range toks {
		o := t.Position().Offset
		if string(text[o:o+3]) != "int" {
			return nil, p.Die("the `int` of a retyped function is not at %d", o)
		}
		edits = append(edits, edit1{o, o + 3, "bool"})
	}
	// Relax: a |= or &= of an answer into a bool, written as the bool's own
	nCompound := 0
	for _, c := range b.compound {
		if !yes["g:"+c.name] {
			continue
		}
		a, z := spanOf(c.x, text)
		ra, rz := spanOf(c.x.AssignmentExpression, text)
		op := "||"
		if c.x.Case == cc.AssignmentExpressionAnd {
			op = "&&"
		}
		edits = append(edits, edit1{a, z, c.name + " = (" + string(text[ra:rz]) + ") " + op + " " + c.name})
		nCompound++
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].a > edits[j].a })
	for _, e := range edits {
		text = append(append(append([]byte{}, text[:e.a]...), e.s...), text[e.z:]...)
	}
	// ---- the callers: `f() == false` is `!f()`, `f() != false` is `f()` -----
	cmp := 0
	for pass := 0; pass < 4; pass++ {
		n, t2, err := b.rewriteComparisons(text, cut, yes)
		if err != nil {
			return nil, p.Die("%v", err)
		}
		if n == 0 {
			break
		}
		cmp += n
		text = t2
	}
	p.Say(fmt.Sprintf("%d comparisons of an answer with %s or 0 are the answer, or its negation", cmp, strings.Join(append(append([]string{}, b.k.True...), b.k.False...), ", ")))
	p.Say(fmt.Sprintf("%d functions answer a question and return bool now, %d declarations retyped: %s",
		len(names), len(toks), strings.Join(names[:min(len(names), 12)], ", ")+"..."))
	p.Say(fmt.Sprintf("%d locals that only ever hold an answer are bool too", nLocals))
	p.Say(fmt.Sprintf("%d struct members that only ever hold an answer are bool too", nFields))
	p.Say(fmt.Sprintf("%d parameters that are only ever given an answer are bool too", nParams))
	if b.k.Globals {
		p.Say(fmt.Sprintf("%d file-scope objects that only ever hold an answer are bool too", nGlobals))
	}
	if b.k.Relax {
		p.Say(fmt.Sprintf("%d |= and &= of an answer into one are written x = E || x", nCompound))
	}
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
func (b *boolRet) boolish(e cc.ExpressionNode, yes map[string]bool, lf *localFacts, depth int) bool {
	if depth > 8 {
		return false // a local assigned from itself, round about
	}
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			s := x.Token.SrcStr()
			if b.answer(s) {
				return true // a success is true, a failure false
			}
			if yes["g:"+s] && (lf == nil || lf.decls[s] == 0 && !lf.isParam(s)) {
				return true // a file-scope object retyped bool
			}
			return lf.boolish(s, yes, depth)
		case cc.PrimaryExpressionExpr:
			return b.boolish(x.ExpressionList, yes, lf, depth)
		}
	case *cc.ExpressionList:
		return x.ExpressionList == nil && b.boolish(x.AssignmentExpression, yes, lf, depth)
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
		return x.Case == cc.ConditionalExpressionCond && b.boolish(x.ExpressionList, yes, lf, depth) && b.boolish(x.ConditionalExpression, yes, lf, depth)
	case *cc.PostfixExpression:
		if x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionPSelect {
			return yes["."+x.Token2.SrcStr()] // a member retyped bool
		}
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
	b       *boolRet       // the run they are facts of
	fn      string         // the function they are the locals of
	params  map[string]int // its parameters, by name, and their positions
	decls   map[string]int
	assigns map[string][]cc.ExpressionNode
	bad     map[string]bool
}

func (b *boolRet) factsOf(fd *cc.FunctionDefinition) *localFacts {
	lf := &localFacts{b: b, decls: map[string]int{}, assigns: map[string][]cc.ExpressionNode{}, bad: map[string]bool{}}
	lf.fn = fd.Declarator.Name()
	lf.params = map[string]int{}
	for i, pd := range paramDecls(fd.Declarator) {
		if pd.Declarator != nil {
			lf.params[pd.Declarator.Name()] = i
		}
	}
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
	b.comparedWithCode(fd.CompoundStatement, ident, func(s string) { lf.bad[s] = true })
	return lf
}

// isParam says nm is one of the function's parameters.
func (lf *localFacts) isParam(nm string) bool {
	_, ok := lf.params[nm]
	return ok
}

// boolish says the local nm holds only 0 or 1: it is declared once in the
// function, touched only by plain assignment, and every value it is given is.
func (lf *localFacts) boolish(nm string, yes map[string]bool, depth int) bool {
	if lf != nil {
		if i, ok := lf.params[nm]; ok && lf.decls[nm] == 0 {
			return yes[paramKey(lf.fn, i)] // a parameter retyped bool
		}
	}
	if lf == nil || lf.decls[nm] != 1 || lf.bad[nm] || len(lf.assigns[nm]) == 0 {
		return false
	}
	for _, e := range lf.assigns[nm] {
		if !lf.b.boolish(e, yes, lf, depth+1) {
			return false
		}
	}
	return true
}

// rewriteComparisons makes one pass: every comparison in the core of an
// answer -- an expression boolish says holds 0 or 1 -- with a truth constant
// becomes the answer or its negation.  Only the outermost of nested
// ones is rewritten in a pass; the caller parses again for the rest.
func (b *boolRet) rewriteComparisons(text []byte, cut int, yes map[string]bool) (int, []byte, error) {
	const path = file
	ast, err := parse(text)
	if err != nil {
		return 0, nil, fmt.Errorf("the text after retyping does not parse: %v", err)
	}
	constOf := func(e cc.ExpressionNode) (string, bool) {
		pe, ok := e.(*cc.PrimaryExpression)
		if ok && pe.Case == cc.PrimaryExpressionInt && pe.Token.SrcStr() == "0" {
			return "no", true // an answer compared with 0 is its negation
		}
		if !ok || pe.Case != cc.PrimaryExpressionIdent {
			return "", false
		}
		switch s := pe.Token.SrcStr(); {
		case b.yes[s]:
			return "yes", true
		case b.no[s]:
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
		if !isConst || !b.boolish(e, yes, nil, 0) {
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

// assign is one value given to a member, with what its function does with
// its locals, so the value can be judged where it is written.
type assign struct {
	e  cc.ExpressionNode
	lf *localFacts
}

// memberFacts is one member NAME: every declaration of it (members are named
// by name, so a name is retyped everywhere or nowhere), its `int` tokens, and
// every value assigned to it through `.` or `->`.
type memberFacts struct {
	toks    []cc.Token
	assigns []assign
	bad     bool
}

// memberCandidates collects the members that may hold an answer.  A name is
// ruled out when any member so named is not `int` alone, one declarator, no
// bit-field; when anything takes its address, increments it or updates it in
// place; when a designator names it or a brace initializer fills its struct
// by position (a value no assignment shows); or when the host mentions it.
func (b *boolRet) memberCandidates(ast *cc.AST, path string, text []byte, cut int, host []byte) map[string]*memberFacts {
	out := map[string]*memberFacts{}
	get := func(m string) *memberFacts {
		if out[m] == nil {
			out[m] = &memberFacts{}
		}
		return out[m]
	}
	inCore := func(n cc.Node) bool {
		pos := n.Position()
		return pos.Filename == path && pos.Offset < cut
	}
	// the declarations
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		sd, ok := n.(*cc.StructDeclaration)
		if !ok || sd.StructDeclaratorList == nil || sd.Position().Filename != path {
			return true
		}
		tok, isInt := onlyIntSQ(sd.SpecifierQualifierList)
		for l := sd.StructDeclaratorList; l != nil; l = l.StructDeclaratorList {
			d := l.StructDeclarator
			if d == nil || d.Declarator == nil {
				continue
			}
			f := get(d.Declarator.Name())
			if !isInt || l != sd.StructDeclaratorList || sd.StructDeclaratorList.StructDeclaratorList != nil ||
				d.ConstantExpression != nil || d.Declarator.Pointer != nil ||
				d.Declarator.DirectDeclarator.Case != cc.DirectDeclaratorIdent || !inCore(sd) {
				f.bad = true
				continue
			}
			f.toks = append(f.toks, tok)
		}
		return true
	})
	// what is done to them, function by function
	member := func(e cc.ExpressionNode) string {
		for {
			switch x := e.(type) {
			case *cc.PostfixExpression:
				if x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionPSelect {
					return x.Token2.SrcStr()
				}
			case *cc.PrimaryExpression:
				if x.Case == cc.PrimaryExpressionExpr {
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
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Position().Filename != path {
			continue
		}
		var lf *localFacts
		if ed.FunctionDefinition != nil {
			lf = b.factsOf(ed.FunctionDefinition)
		}
		sweep.Walk(ed, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.AssignmentExpression:
				if m := member(x.UnaryExpression); m != "" {
					if x.Case == cc.AssignmentExpressionAssign {
						get(m).assigns = append(get(m).assigns, assign{x.AssignmentExpression, lf})
					} else if x.Case != cc.AssignmentExpressionCond {
						get(m).bad = true
					}
				}
			case *cc.UnaryExpression:
				switch x.Case {
				case cc.UnaryExpressionAddrof, cc.UnaryExpressionInc, cc.UnaryExpressionDec:
					if m := member(x.CastExpression); m != "" {
						get(m).bad = true
					}
					if m := member(x.UnaryExpression); m != "" {
						get(m).bad = true
					}
				}
			case *cc.PostfixExpression:
				if x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
					if m := member(x.PostfixExpression); m != "" {
						get(m).bad = true
					}
				}
			case *cc.Designator:
				switch x.Case {
				case cc.DesignatorField:
					get(x.Token2.SrcStr()).bad = true
				case cc.DesignatorField2:
					get(x.Token.SrcStr()).bad = true
				}
			}
			return true
		})
		b.comparedWithCode(ed, member, func(s string) { get(s).bad = true })
	}
	for m := range sweep.PositionalMembers(ast, path, text, b.k.Layout) {
		get(m).bad = true
	}
	for m, f := range out {
		if len(f.toks) == 0 || edit.MentionCount(host, m) > 0 {
			f.bad = true
		}
	}
	return out
}

// onlyIntSQ is onlyInt for a member's specifier list.
func onlyIntSQ(sq *cc.SpecifierQualifierList) (cc.Token, bool) {
	var tok cc.Token
	n := 0
	for ; sq != nil; sq = sq.SpecifierQualifierList {
		switch sq.Case {
		case cc.SpecifierQualifierListTypeSpec:
			n++
			if sq.TypeSpecifier.Case != cc.TypeSpecifierInt {
				return tok, false
			}
			tok = sq.TypeSpecifier.Token
		default:
			return tok, false
		}
	}
	return tok, n == 1
}

func paramKey(fn string, i int) string { return fmt.Sprintf("p:%s:%d", fn, i) }

// paramDecls is a function declarator's parameter declarations, in order;
// nil for a variadic one, which this phase does not touch.
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

// paramFacts is one parameter: its `int` tokens (the definition's and every
// prototype's) and every value it is given -- each call's argument, and each
// assignment in its function.
type paramFacts struct {
	toks    []cc.Token
	assigns []assign
	bad     bool
}

// paramCandidates collects the parameters that may hold an answer: `int`
// alone and named in the definition, of a function called only by name from
// the core (callers is the set that holds), in the same place in every
// prototype, never taken the address of, incremented or updated in place.
func (b *boolRet) paramCandidates(ast *cc.AST, path string, cut int, callers map[string]bool) map[string]*paramFacts {
	out := map[string]*paramFacts{}
	inCore := func(n cc.Node) bool {
		pos := n.Position()
		return pos.Filename == path && pos.Offset < cut
	}
	defs := map[string]*cc.FunctionDefinition{}
	lfs := map[string]*localFacts{}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.FunctionDefinition == nil || !inCore(ed) || !callers[ed.FunctionDefinition.Declarator.Name()] {
			continue
		}
		fd := ed.FunctionDefinition
		name := fd.Declarator.Name()
		defs[name] = fd
		lf := b.factsOf(fd)
		lfs[name] = lf
		for i, pd := range paramDecls(fd.Declarator) {
			pf := &paramFacts{}
			out[paramKey(name, i)] = pf
			tok, ok := onlyInt(pd.DeclarationSpecifiers)
			if !ok || pd.Declarator == nil || pd.Declarator.Pointer != nil ||
				pd.Declarator.DirectDeclarator.Case != cc.DirectDeclaratorIdent {
				pf.bad = true
				continue
			}
			pn := pd.Declarator.Name()
			if lf.bad[pn] || lf.decls[pn] != 0 {
				pf.bad = true
				continue
			}
			pf.toks = append(pf.toks, tok)
			for _, e := range lf.assigns[pn] {
				pf.assigns = append(pf.assigns, assign{e, lf})
			}
		}
	}
	// the prototypes: the same parameter must be `int` alone there too
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Declaration == nil || ed.Declaration.Case != cc.DeclarationDecl || !inCore(ed) {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if defs[d.Name()] == nil {
				continue
			}
			for i, pd := range paramDecls(d) {
				pf := out[paramKey(d.Name(), i)]
				if pf == nil {
					continue
				}
				tok, ok := onlyInt(pd.DeclarationSpecifiers)
				if !ok || (pd.Declarator != nil && pd.Declarator.Pointer != nil) {
					pf.bad = true
					continue
				}
				pf.toks = append(pf.toks, tok)
			}
		}
	}
	// the calls: every argument is a value the parameter is given
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || !inCore(ed) {
			continue
		}
		var lf *localFacts
		if ed.FunctionDefinition != nil {
			lf = b.factsOf(ed.FunctionDefinition)
		}
		sweep.Walk(ed, func(n cc.Node) bool {
			c, ok := n.(*cc.PostfixExpression)
			if !ok || c.Case != cc.PostfixExpressionCall {
				return true
			}
			pe, ok := c.PostfixExpression.(*cc.PrimaryExpression)
			if !ok || pe.Case != cc.PrimaryExpressionIdent || defs[pe.Token.SrcStr()] == nil {
				return true
			}
			i := 0
			for a := c.ArgumentExpressionList; a != nil; a = a.ArgumentExpressionList {
				if pf := out[paramKey(pe.Token.SrcStr(), i)]; pf != nil {
					pf.assigns = append(pf.assigns, assign{a.AssignmentExpression, lf})
				}
				i++
			}
			return true
		})
	}
	return out
}

// otherConst says e is a constant that is not an answer: a number or
// character other than 0 and 1, a negative one, or a name in capitals other
// than a truth constant.  Something compared with one is a code, not
// a yes or no, whatever values it happens to be given.
func (b *boolRet) otherConst(e cc.ExpressionNode) bool {
	for {
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			switch x.Case {
			case cc.PrimaryExpressionInt, cc.PrimaryExpressionChar:
				s := x.Token.SrcStr()
				return s != "0" && s != "1"
			case cc.PrimaryExpressionIdent:
				s := x.Token.SrcStr()
				if b.answer(s) {
					return false
				}
				return s == strings.ToUpper(s)
			case cc.PrimaryExpressionExpr:
				e = x.ExpressionList
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				e = x.AssignmentExpression
				continue
			}
		case *cc.UnaryExpression:
			return x.Case == cc.UnaryExpressionMinus
		}
		return false
	}
}

// comparedWithCode calls mark on the name every comparison with a code
// compares: `x == MODE_RAW`, `r != -1`, `p->f < 5`.
func (b *boolRet) comparedWithCode(n cc.Node, name func(cc.ExpressionNode) string, mark func(string)) {
	sweep.Walk(n, func(m cc.Node) bool {
		var l, r cc.ExpressionNode
		switch x := m.(type) {
		case *cc.EqualityExpression:
			if x.Case == cc.EqualityExpressionRel {
				return true
			}
			l, r = x.EqualityExpression, x.RelationalExpression
		case *cc.RelationalExpression:
			if x.Case == cc.RelationalExpressionShift {
				return true
			}
			l, r = x.RelationalExpression, x.ShiftExpression
		default:
			return true
		}
		if b.otherConst(r) {
			if s := name(l); s != "" {
				mark(s)
			}
		}
		if b.otherConst(l) {
			if s := name(r); s != "" {
				mark(s)
			}
		}
		return true
	})
}

// compoundAssign is `name |= E` or `name &= E` into a file-scope object.
type compoundAssign struct {
	name string
	x    *cc.AssignmentExpression
}

// globalCandidates collects the core's file-scope objects that may hold an
// answer, by name, with every value given one: its initializer and each
// assignment in a function that has no local or parameter of that name.
// A name is ruled out when a declaration of it is not `int` alone, one
// declarator, no pointer, no array, or is braced; when anything takes its
// address (in a function or an initializer), increments it or updates it in
// place; when it is compared with a code; when a function declares a local
// or a parameter of that name (the facts go by name); or when the host
// mentions it.  An initializer of a literal 0 or 1 is no value either way:
// the object needs an answer assigned somewhere to be one.
func (b *boolRet) globalCandidates(ast *cc.AST, path string, cut int, host []byte) map[string]*memberFacts {
	out := map[string]*memberFacts{}
	get := func(m string) *memberFacts {
		if out[m] == nil {
			out[m] = &memberFacts{}
		}
		return out[m]
	}
	inCore := func(n cc.Node) bool {
		pos := n.Position()
		return pos.Filename == path && pos.Offset < cut
	}
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
	literal01 := func(e cc.ExpressionNode) bool {
		if pe, ok := e.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionInt {
			return pe.Token.SrcStr() == "0" || pe.Token.SrcStr() == "1"
		}
		return false
	}
	// the declarations
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Declaration == nil || ed.Declaration.Case != cc.DeclarationDecl || ed.Declaration.InitDeclaratorList == nil {
			continue
		}
		dl := ed.Declaration.InitDeclaratorList
		tok, isInt := onlyInt(ed.Declaration.DeclarationSpecifiers)
		for l := dl; l != nil; l = l.InitDeclaratorList {
			id := l.InitDeclarator
			if id == nil || id.Declarator == nil || id.Declarator.Type() != nil && id.Declarator.Type().Kind() == cc.Function {
				continue
			}
			f := get(id.Declarator.Name())
			if !isInt || dl.InitDeclaratorList != nil || id.Declarator.Pointer != nil ||
				id.Declarator.DirectDeclarator.Case != cc.DirectDeclaratorIdent || !inCore(ed) {
				f.bad = true
				continue
			}
			f.toks = append(f.toks, tok)
			if in := id.Initializer; in != nil {
				switch {
				case in.Case != cc.InitializerExpr:
					f.bad = true
				case !literal01(in.AssignmentExpression):
					f.assigns = append(f.assigns, assign{in.AssignmentExpression, nil})
				}
			}
		}
	}
	// an address taken outside a function: a table of pointers to them
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Declaration == nil {
			continue
		}
		sweep.Walk(ed.Declaration, func(n cc.Node) bool {
			if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
				if m := ident(u.CastExpression); m != "" {
					get(m).bad = true
				}
			}
			return true
		})
	}
	// what the functions do to them
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.FunctionDefinition == nil || ed.Position().Filename != path {
			continue
		}
		lf := b.factsOf(ed.FunctionDefinition)
		for nm := range lf.decls {
			get(nm).bad = true // a local of the name: the facts go by name
		}
		for nm := range lf.params {
			get(nm).bad = true
		}
		sweep.Walk(ed.FunctionDefinition.CompoundStatement, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.AssignmentExpression:
				if m := ident(x.UnaryExpression); m != "" {
					switch {
					case x.Case == cc.AssignmentExpressionAssign && b.k.Relax && literal01(x.AssignmentExpression):
						// a 0 or a 1: no answer, and none against
					case x.Case == cc.AssignmentExpressionAssign:
						get(m).assigns = append(get(m).assigns, assign{x.AssignmentExpression, lf})
					case b.k.Relax && (x.Case == cc.AssignmentExpressionOr || x.Case == cc.AssignmentExpressionAnd):
						get(m).assigns = append(get(m).assigns, assign{x.AssignmentExpression, lf})
						b.compound = append(b.compound, compoundAssign{m, x})
					case x.Case != cc.AssignmentExpressionCond:
						get(m).bad = true
					}
				}
			case *cc.UnaryExpression:
				switch x.Case {
				case cc.UnaryExpressionAddrof, cc.UnaryExpressionInc, cc.UnaryExpressionDec:
					if m := ident(x.CastExpression); m != "" {
						get(m).bad = true
					}
					if m := ident(x.UnaryExpression); m != "" {
						get(m).bad = true
					}
				}
			case *cc.PostfixExpression:
				if x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
					if m := ident(x.PostfixExpression); m != "" {
						get(m).bad = true
					}
				}
			}
			return true
		})
		b.comparedWithCode(ed.FunctionDefinition.CompoundStatement, ident, func(s string) { get(s).bad = true })
	}
	// a size or a type taken of one: bool would change it
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		var e cc.Node
		switch x := n.(type) {
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionSizeofExpr {
				e = x.UnaryExpression
			}
		case *cc.TypeSpecifier:
			if x.Case == cc.TypeSpecifierTypeofExpr {
				e = x.ExpressionList
			}
		}
		if e != nil {
			sweep.Walk(e, func(m cc.Node) bool {
				if pe, ok := m.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent {
					get(pe.Token.SrcStr()).bad = true
				}
				return true
			})
		}
		return true
	})
	for m, f := range out {
		if len(f.toks) == 0 || edit.MentionCount(host, m) > 0 {
			f.bad = true
		}
	}
	return out
}
