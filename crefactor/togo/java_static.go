package togo

// java_static.go is which methods are static, and which private
// (doc/JAVA-IDIOMS.md, item 10).
//
// A method is static when it reaches no field of the editor, directly or
// through what it calls: it names no file-scope object and no hoisted
// static, calls no host method and nothing through a pointer, is used as
// no value (a function reference is `this::f`), is not one the glue calls
// (Profile.JavaGlue) or one whose body is the runtime's, and calls only
// static methods.  Held to what the printer writes: a method thought static
// whose text names an instance field or method all the same is not, with
// every method that calls it, and the methods are printed again -- as the
// Clojure's functions without the editor are (clj_ed.go).
//
// With Profile.JavaPrivate every method is private but the host's, which
// the glue overrides, and the glue's.

import (
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// staticFree is the methods that need no instance, before the printer has
// had its say (staticHeld).
func (j *jgen) staticFree(fds []*cc.FunctionDefinition) map[string]bool {
	p := j.g.p
	needs := map[string]bool{}
	calls := map[string][]string{}
	defined := map[string]bool{}
	for _, fd := range fds {
		defined[fd.Declarator.Name()] = true
	}
	for _, n := range p.JavaGlue {
		needs[n] = true
	}
	for _, rb := range p.RuntimeBodies {
		if rb.Java != nil {
			needs[rb.Name] = true
		}
	}
	for _, fd := range fds {
		name := fd.Declarator.Name()
		if j.g.a.addr[name] {
			needs[name] = true // a value: this::f
		}
		walkNodes(fd.CompoundStatement, func(n cc.Node) {
			if needs[name] {
				return
			}
			switch x := n.(type) {
			case *cc.PrimaryExpression:
				if d := identDecl(x); d != nil && d.Type() != nil && d.Type().Kind() != cc.Function && d.StorageDuration() == cc.Static {
					needs[name] = true // a field
				}
			case *cc.PostfixExpression:
				if x.Case != cc.PostfixExpressionCall {
					return
				}
				d := fnDesignator(x.PostfixExpression)
				switch {
				case d == nil:
					needs[name] = true
				case defined[d.Name()]:
					calls[name] = append(calls[name], d.Name())
				case d.Name() == "__builtin_expect", p.allocators[d.Name()], p.frees[d.Name()],
					p.byteMove(d.Name()), d.Name() == p.Bytes.Set, d.Name() == p.Bytes.Cmp:
				default:
					needs[name] = true // the host
				}
			}
		})
	}
	closeNeeds(needs, calls)
	free := map[string]bool{}
	for _, fd := range fds {
		if n := fd.Declarator.Name(); !needs[n] {
			free[n] = true
		}
	}
	return free
}

var javaIdent = regexp.MustCompile(`(^|[^.\w$])([A-Za-z_$][\w$]*)`)

// staticHeld is free without the methods whose text names an instance field
// or method (inst, and every method not free), and the methods that call
// them; and whether it changed.
func (j *jgen) staticHeld(free map[string]bool, texts map[string]string, inst map[string]bool, fds []*cc.FunctionDefinition) bool {
	needs := map[string]bool{}
	calls := map[string][]string{}
	byJName := map[string]bool{}
	for n := range texts {
		if !free[n] {
			byJName[j.jName(n)] = true
		}
	}
	changed := false
	for _, fd := range fds {
		name := fd.Declarator.Name()
		if !free[name] {
			needs[name] = true
			continue
		}
		for _, m := range javaIdent.FindAllStringSubmatch(texts[name], -1) {
			if id := m[2]; id != j.jName(name) && (inst[id] || byJName[id]) {
				needs[name] = true
				changed = true
				break
			}
		}
		walkNodes(fd.CompoundStatement, func(n cc.Node) {
			if x, ok := n.(*cc.PostfixExpression); ok && x.Case == cc.PostfixExpressionCall {
				if d := fnDesignator(x.PostfixExpression); d != nil {
					calls[name] = append(calls[name], d.Name())
				}
			}
		})
	}
	if !changed {
		return false
	}
	closeNeeds(needs, calls)
	for n := range free {
		if needs[n] {
			delete(free, n)
		}
	}
	return true
}

// mods is a method's modifiers: private (Profile.JavaPrivate, but for the
// glue's), static (staticFree).
func (j *jgen) mods(name string) string {
	m := ""
	if j.g.p.JavaPrivate && !j.glue[name] {
		m += "private "
	}
	if j.statics[name] {
		m += "static "
	}
	return m
}

// instanceNames are the names a static method may not use: the editor's
// fields -- the file-scope objects, the hoisted statics, the function
// references -- and the host's methods.
func (j *jgen) instanceNames() map[string]bool {
	out := map[string]bool{}
	for tu := j.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() != nil && d.Type().Kind() == cc.Function && j.defined[d.Name()] {
				continue // a type, or a prototype of a method: its own rule
			}
			out[j.jName(d.Name())] = true // an object, or the host's method
		}
	}
	for _, s := range j.g.a.statics {
		out[j.staticName(s.fn, s.d.Name())] = true
	}
	for _, fld := range j.fnRefs {
		out[fld] = true
	}
	for n := range j.defined {
		if !j.statics[n] {
			out[j.jName(n)] = true
		}
	}
	return out
}

// suppress is a declaration's @SuppressWarnings, one annotation for what it
// holds: C's fall-throughs, meant and each marked; an array of Ptr<T>,
// which Java creates raw, having no generic arrays.
func suppress(fallthrough_, rawArray bool) string {
	var ws []string
	if fallthrough_ {
		ws = append(ws, `"fallthrough"`)
	}
	if rawArray {
		ws = append(ws, `"rawtypes"`, `"unchecked"`)
	}
	switch len(ws) {
	case 0:
		return ""
	case 1:
		return "    @SuppressWarnings(" + ws[0] + ")\n"
	}
	return "    @SuppressWarnings({" + strings.Join(ws, ", ") + "})\n"
}
