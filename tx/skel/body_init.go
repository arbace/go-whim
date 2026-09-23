package main

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// initializers writes the initial values of every file-scope object and every
// hoisted block-scope static whose C initializer is not all zeros, as
// assignments inside init(): the tables name functions that name the tables,
// which Go calls an initialization cycle at package level and not in init().
// A character array is storage already made (Mk) that its string is copied
// into; any other array or struct is a composite literal.
func (g *gen) initializers() (string, []string) {
	f := &fnEmit{g: g, name: "init", ft: nil, byDecl: map[*cc.Declarator]*local{}, taken: map[string]bool{}, gotos: map[string]bool{}}
	var out strings.Builder
	f.out = &out
	f.indent = 1
	var failed []string
	one := func(name, key string, d *cc.Declarator, in *cc.Initializer) {
		if in == nil || zeroInit(in) {
			return
		}
		before := out.Len()
		defer func() {
			if r := recover(); r != nil {
				u, ok := r.(unsupported)
				if !ok {
					panic(r)
				}
				s := out.String()[:before]
				out.Reset()
				out.WriteString(s)
				failed = append(failed, name+": "+u.why)
			}
		}()
		f.initObject(name, key, d.Type(), in)
	}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration.Case != cc.DeclarationDecl {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			id := l.InitDeclarator
			d := id.Declarator
			if d.IsTypename() || d.Type().Kind() == cc.Function {
				continue
			}
			one(GoName(d.Name()), "global:"+d.Name(), d, id.Initializer)
		}
	}
	for _, s := range g.a.statics {
		key := fmt.Sprintf("static:%s.%s", s.fn, s.d.Name())
		one(g.globalName[key], key, s.d, s.init)
	}
	return out.String(), failed
}

// initObject writes one object's initial value.
func (f *fnEmit) initObject(name, key string, t cc.Type, in *cc.Initializer) {
	typ := f.objType(t, key)
	at, isArray := t.(*cc.ArrayType)
	switch {
	case isArray && in.Case == cc.InitializerExpr:
		// a character array from a string
		sv, ok := unparenE(in.AssignmentExpression).Value().(cc.StringValue)
		if !ok {
			f.no(in, "an array initialized from an expression")
		}
		s := strings.TrimSuffix(string(sv), "\x00")
		if s == "" {
			return
		}
		if strings.HasPrefix(typ, "Ptr[") {
			f.line("copy(%s.Slice(%d), %s)", name, len(s), goQuote(s))
		} else {
			f.line("copy(%s[:], %s)", name, goQuote(s))
		}
	case isArray && strings.HasPrefix(typ, "Ptr["):
		et := elemOfGo(typ)
		for i, it := range listItems(f, in) {
			if zeroInit(it) {
				continue
			}
			if it.Case == cc.InitializerExpr && isByteType(at.Elem().Undecay()) && at.Elem().Kind() == cc.Array {
				f.no(in, "an array of character arrays")
			}
			f.line("%s.Set(%d, %s)", name, i, f.initValue(et, at.Elem(), "elem:"+key, it))
		}
	default:
		f.line("%s = %s", name, f.initValue(typ, t, key, in))
	}
}
