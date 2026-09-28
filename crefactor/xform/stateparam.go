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

// StateParamKnobs are StateParam's: a set of file-scope objects that are one
// machine's state, the struct they become, and where the state stops being
// a parameter.
type StateParamKnobs struct {
	Core Core
	// Objects are the file-scope objects: each becomes a member of Type, of
	// its own name and type.
	Objects []string
	// Type is the struct's typedef name, Instance the file-scope object of
	// it that holds what the objects held, and Param the parameter a
	// function is handed it by.
	Type, Instance, Param string
	// Roots keep their signatures: each binds Param to &Instance and hands
	// it down.  Every path from a function that names an object up to one
	// nothing in the core calls must pass through one.
	Roots []string
}

// StateParam is the step that makes a set of file-scope objects -- one
// machine's state -- a parameter: they become the members of one struct, of
// which one file-scope instance holds what they held, and every function
// that names one of them, or calls a function that does, is handed a
// pointer to the struct as its first parameter, down from the roots, which
// keep their signatures and bind it to the instance.  An object `o` is
// `P->o` where it was named.
//
// So a caller may run the machine on a struct of its own -- two at once,
// each on its own state -- where the objects allowed one.  Nothing a root's
// caller sees changes: the roots run it on the instance, as before on the
// objects.
//
// It refuses when a function that needs the state is not a root and is
// named other than by a call (its address taken, or named by the host), or
// when nothing but a root bounds the need; when an object has an
// initialiser that is not zero (the instance is zero-initialised); when a
// name it adds is taken; and when what it writes does not type-check.
func StateParam(k StateParamKnobs) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "stateparam", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		out, n, err := stateParam(p, k, text)
		if err != nil {
			return nil, err
		}
		if min, ok := f["--at-least"]; ok && n < min {
			return nil, p.Die("%d functions take the state, fewer than the %d asked for", n, min)
		}
		return out, nil
	}
}

func stateParam(p edit.Ph, k StateParamKnobs, text []byte) ([]byte, int, error) {
	end := len(text)
	if k.Core != nil {
		if e := k.Core(text); e >= 0 {
			end = e
		}
	}
	src := append([]byte(nil), text[:end]...)
	ast, err := translate(append([]byte(nil), src...))
	if err != nil {
		return nil, 0, p.Die("the core does not type-check: %v", err)
	}
	isObj := map[string]bool{}
	for _, o := range k.Objects {
		isObj[o] = true
	}
	isRoot := map[string]bool{}
	for _, r := range k.Roots {
		isRoot[r] = true
	}

	// the definitions, the prototypes, the objects' declarations, and every
	// name the added three could meet
	fns := map[string]*cc.FunctionDefinition{}
	protos := map[string][]*cc.Declarator{}
	objs := map[string]*spObj{}
	taken := map[string]bool{}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Position().Filename != file {
			continue
		}
		switch ed.Case {
		case cc.ExternalDeclarationFuncDef:
			fd := ed.FunctionDefinition
			fns[fd.Declarator.Name()] = fd
		case cc.ExternalDeclarationDecl:
			d := ed.Declaration
			if d == nil || d.Case != cc.DeclarationDecl {
				continue
			}
			for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
				id := l.InitDeclarator
				if id == nil || id.Declarator == nil {
					continue
				}
				dc := id.Declarator
				if dc.Type() != nil && dc.Type().Kind() == cc.Function {
					protos[dc.Name()] = append(protos[dc.Name()], dc)
					continue
				}
				if isObj[dc.Name()] {
					if d.InitDeclaratorList.InitDeclaratorList != nil {
						return nil, 0, p.Die("%s is declared beside another object", dc.Name())
					}
					objs[dc.Name()] = &spObj{ed, d, id}
				}
			}
		}
	}
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		switch x := n.(type) {
		case *cc.Declarator:
			if x.Position().Filename == file {
				taken[x.Name()] = true
			}
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent && x.Position().Filename == file {
				taken[x.Token.SrcStr()] = true
			}
		}
		return true
	})
	for _, name := range []string{k.Type, k.Instance, k.Param} {
		if taken[name] {
			return nil, 0, p.Die("the name %s is taken", name)
		}
	}
	for _, o := range k.Objects {
		od := objs[o]
		if od == nil {
			return nil, 0, p.Die("no file-scope object %s", o)
		}
		if od.id.Declarator.StorageDuration() != cc.Static {
			return nil, 0, p.Die("%s is not a file-scope object", o)
		}
		if od.id.Initializer != nil && !zeroInit(od.id.Initializer, src) {
			return nil, 0, p.Die("%s has an initialiser that is not zero", o)
		}
	}

	objset := map[*cc.Declarator]string{}
	for n, o := range objs {
		objset[o.id.Declarator] = n
	}
	// who names an object, who calls whom, and who is named but not called
	named := map[string]bool{}
	callers := map[string]map[string]bool{}
	called := map[*cc.PrimaryExpression]bool{}
	for name, fd := range fns {
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.PrimaryExpression:
				if o := objRef(x, objset); o != "" {
					named[name] = true
				}
			case *cc.PostfixExpression:
				if x.Case == cc.PostfixExpressionCall {
					if c := calleeName(x); c != "" {
						if callers[c] == nil {
							callers[c] = map[string]bool{}
						}
						callers[c][name] = true
						if pe, ok := unparen(x.PostfixExpression).(*cc.PrimaryExpression); ok {
							called[pe] = true
						}
					}
				}
			}
			return true
		})
	}
	// the objects named outside every function: in another's initialiser
	outside := false
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		if fd, ok := n.(*cc.FunctionDefinition); ok {
			_ = fd
			return false
		}
		if x, ok := n.(*cc.PrimaryExpression); ok && x.Position().Filename == file && objRef(x, objset) != "" {
			outside = true
		}
		return true
	})
	if outside {
		return nil, 0, p.Die("an object is named outside every function")
	}
	need := map[string]bool{}
	var stack []string
	for n := range named {
		need[n] = true
		stack = append(stack, n)
	}
	for len(stack) > 0 {
		g := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if isRoot[g] {
			continue
		}
		for c := range callers[g] {
			if !need[c] {
				need[c] = true
				stack = append(stack, c)
			}
		}
	}
	// a function that needs the state and is not a root must be reached by
	// calls alone
	valued := map[string]bool{}
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		pe, ok := n.(*cc.PrimaryExpression)
		if !ok || pe.Case != cc.PrimaryExpressionIdent || called[pe] || pe.Position().Filename != file {
			return true
		}
		if d, ok := pe.ResolvedTo().(*cc.Declarator); ok && fns[d.Name()] != nil && d.Type() != nil && d.Type().Kind() == cc.Function {
			valued[d.Name()] = true
		}
		return true
	})
	var takers []string
	for n := range need {
		if isRoot[n] {
			continue
		}
		switch {
		case valued[n]:
			return nil, 0, p.Die("%s needs the state and its address is taken", n)
		case mentions(text[end:], n):
			return nil, 0, p.Die("%s needs the state and the host names it", n)
		case len(callers[n]) == 0:
			return nil, 0, p.Die("%s needs the state and nothing in the core calls it: no root bounds it", n)
		}
		if ft, ok := fns[n].Declarator.Type().(*cc.FunctionType); !ok || ft.IsVariadic() {
			return nil, 0, p.Die("%s needs the state and is variadic", n)
		}
		takers = append(takers, n)
	}
	sort.Strings(takers)

	type rewrite struct {
		a, z int
		with string
	}
	var rw []rewrite
	ptr := k.Type + " *" + k.Param
	// a parameter list gets the pointer first
	addParam := func(dc *cc.Declarator) error {
		dd := dc.DirectDeclarator
		for dd != nil && dd.Case != cc.DirectDeclaratorFuncParam {
			dd = dd.DirectDeclarator
		}
		if dd == nil || dd.ParameterTypeList == nil {
			return fmt.Errorf("%s: no parameter list", dc.Name())
		}
		open := dd.Token.Position().Offset
		pds := paramDecls(dc)
		if len(pds) == 1 && pds[0].Declarator == nil && pds[0].AbstractDeclarator == nil {
			if a, z := sweep.Span(pds[0], file, src); strings.TrimSpace(string(src[a:z])) == "void" {
				rw = append(rw, rewrite{a, z, ptr})
				return nil
			}
		}
		if len(pds) == 0 {
			rw = append(rw, rewrite{open + 1, open + 1, ptr})
			return nil
		}
		rw = append(rw, rewrite{open + 1, open + 1, ptr + ", "})
		return nil
	}
	for _, n := range takers {
		if err := addParam(fns[n].Declarator); err != nil {
			return nil, 0, p.Die("%v", err)
		}
		for _, dc := range protos[n] {
			if err := addParam(dc); err != nil {
				return nil, 0, p.Die("%v", err)
			}
		}
	}
	isTaker := map[string]bool{}
	for _, n := range takers {
		isTaker[n] = true
	}
	for n := range need {
		fd := fns[n]
		sweep.Walk(fd.CompoundStatement, func(nd cc.Node) bool {
			switch x := nd.(type) {
			case *cc.PrimaryExpression:
				if o := objRef(x, objset); o != "" {
					a, z := sweep.Span(x, file, src)
					rw = append(rw, rewrite{a, z, k.Param + "->" + o})
				}
			case *cc.PostfixExpression:
				if x.Case == cc.PostfixExpressionCall && isTaker[calleeName(x)] {
					open := x.Token.Position().Offset
					if x.ArgumentExpressionList == nil {
						rw = append(rw, rewrite{open + 1, open + 1, k.Param})
					} else {
						rw = append(rw, rewrite{open + 1, open + 1, k.Param + ", "})
					}
				}
			}
			return true
		})
		if isRoot[n] {
			brace := fd.CompoundStatement.Token.Position().Offset
			rw = append(rw, rewrite{brace + 1, brace + 1, fmt.Sprintf("\n    %s = &%s;", ptr, k.Instance)})
		}
	}

	// the struct, where the first object was; a member's type declared after
	// that is moved up to it
	first := -1
	var members []string
	for _, o := range k.Objects {
		od := objs[o]
		a, z := lineSpan(od.ed, src)
		if first < 0 || a < first {
			first = a
		}
		rw = append(rw, rewrite{a, z, ""})
		sa, sz := sweep.Span(od.d.DeclarationSpecifiers, file, src)
		spec := strings.TrimSpace(strings.Replace(" "+string(src[sa:sz])+" ", " static ", " ", 1))
		da, dz := sweep.Span(od.id.Declarator, file, src)
		members = append(members, fmt.Sprintf("    %s %s;\n", spec, src[da:dz]))
	}
	var moved []string
	movedAt := map[int]bool{}
	for _, o := range k.Objects {
		t := objs[o].id.Declarator.Type()
		for t != nil {
			if td := t.Typedef(); td != nil {
				if off := td.Position().Offset; td.Position().Filename == file && off > first {
					ted := declOf(ast, td)
					if ted == nil {
						return nil, 0, p.Die("%s's type %s: no declaration to move", o, td.Name())
					}
					a, z := lineSpan(ted, src)
					if !movedAt[a] {
						movedAt[a] = true
						rw = append(rw, rewrite{a, z, ""})
						moved = append(moved, string(src[a:z]))
					}
				}
				break
			}
			if at, ok := t.(*cc.ArrayType); ok {
				t = at.Elem()
				continue
			}
			break
		}
	}
	// a function that takes it declared before that: the type is declared
	// first, where the first of them is, and defined at the objects' place
	early := first
	for _, n := range takers {
		for _, dc := range append([]*cc.Declarator{fns[n].Declarator}, protos[n]...) {
			a := declStart(dc, src)
			if a < early {
				early = a
			}
		}
	}
	var decl string
	if early < first {
		tag := strings.TrimSuffix(k.Type, "_T") + "_S"
		rw = append(rw, rewrite{early, early, "typedef struct " + tag + " " + k.Type + ";\n\n"})
		decl = strings.Join(moved, "") + "struct " + tag + "\n{\n" + strings.Join(members, "") + "};\n\nstatic " + k.Type + " " + k.Instance + ";\n\n"
	} else {
		decl = strings.Join(moved, "") + "typedef struct\n{\n" + strings.Join(members, "") + "} " + k.Type + ";\n\nstatic " + k.Type + " " + k.Instance + ";\n\n"
	}
	rw = append(rw, rewrite{first, first, decl})

	sort.SliceStable(rw, func(i, j int) bool { return rw[i].a > rw[j].a || rw[i].a == rw[j].a && rw[i].z > rw[j].z })
	out := append([]byte{}, src...)
	last := len(out) + 1
	for _, x := range rw {
		if x.z > last {
			return nil, 0, p.Die("two rewrites overlap at offset %d", x.a)
		}
		out = append(append(append([]byte{}, out[:x.a]...), x.with...), out[x.z:]...)
		last = x.a
	}
	if _, err := translate(append([]byte(nil), out...)); err != nil {
		return nil, 0, p.Die("what it writes does not type-check: %v", err)
	}
	var roots []string
	for n := range need {
		if isRoot[n] {
			roots = append(roots, n)
		}
	}
	sort.Strings(roots)
	p.Say(fmt.Sprintf("%d objects are %s's members; %d functions take it, from %d roots (%s)", len(k.Objects), k.Type, len(takers), len(roots), strings.Join(roots, ", ")))
	return append(out, text[end:]...), len(takers), nil
}

// spObj is one of StateParam's objects: its declaration.
type spObj struct {
	ed *cc.ExternalDeclaration
	d  *cc.Declaration
	id *cc.InitDeclarator
}

// objRef is the object of set x names, or "".
func objRef(x *cc.PrimaryExpression, set map[*cc.Declarator]string) string {
	if x.Case != cc.PrimaryExpressionIdent {
		return ""
	}
	d, ok := x.ResolvedTo().(*cc.Declarator)
	if !ok {
		return ""
	}
	return set[d]
}

// zeroInit says an initialiser is all zeros: each value a constant zero or
// a null pointer.
func zeroInit(in *cc.Initializer, src []byte) bool {
	zero := true
	sweep.Walk(in, func(n cc.Node) bool {
		x, ok := n.(*cc.Initializer)
		if !ok || x.AssignmentExpression == nil {
			return zero
		}
		a, z := sweep.Span(x.AssignmentExpression, file, src)
		switch v := x.AssignmentExpression.Value().(type) {
		case cc.Int64Value:
			zero = zero && v == 0
		case cc.UInt64Value:
			zero = zero && v == 0
		default:
			zero = zero && strings.TrimSpace(string(src[a:z])) == "nullptr"
		}
		return false
	})
	return zero
}

// lineSpan is a file-scope declaration's whole lines, and the blank line
// after them.
func lineSpan(ed *cc.ExternalDeclaration, src []byte) (int, int) {
	a, z := sweep.Span(ed, file, src)
	for a > 0 && src[a-1] != '\n' {
		a--
	}
	for z < len(src) && src[z] != '\n' {
		z++
	}
	if z < len(src) {
		z++
	}
	if z < len(src) && src[z] == '\n' {
		z++
	}
	return a, z
}

// declOf is the file-scope declaration that declares d.
func declOf(ast *cc.AST, d *cc.Declarator) *cc.ExternalDeclaration {
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			if l.InitDeclarator != nil && l.InitDeclarator.Declarator == d {
				return ed
			}
		}
	}
	return nil
}
