package togo

// cfacts.go is what the backends that keep C's memory raw -- the Haskell
// (hs*.go, doc/HASKELL.md), the Scheme (scm*.go, doc/SCHEME.md) and the
// Rust (rs*.go, doc/RUST.md) -- decide about the C before any prints a
// line, and decide alike, so that they read the C the same way and none's
// printing moves another's output:
//
//   - the segment: each file-scope object's and block-scope static's
//     offset in the one block of memory the editor's objects are in, as C
//     lays them out (layout, anon);
//   - the typedef that names each struct or union tag (tagTypedefs);
//   - the out-parameters, a value in and a value out (outparams.go);
//   - the struct locals that are values, one binding a member
//     (structvalues.go);
//   - what each function touches: the pure ones, and those that need the
//     editor (effects.go).
//
// The Rust asks only the functions kept, the tags' typedefs and the
// editor's reach; it leaves the out-parameters and the struct values out.
//
// A backend makes them with newCFacts, telling it what is its own: the
// functions and objects its host calls back or addresses, and its runtime
// bodies' text, whose callers are written by hand and keep the whole
// signature; and the analyses it leaves out.

import (
	"regexp"

	"github.com/arbace/go-whim/crefactor/cc"
)

// cfacts are the decisions about one translation unit.
type cfacts struct {
	g       *gen
	defined map[string]*cc.FunctionDefinition
	keep    map[string]bool // the functions whose callers are written by hand

	segOff     map[string]int     // an object's key -> its offset in the segment
	segType    map[string]cc.Type // an object's key -> its type
	segSize    int
	tagTypedef map[string]string // a struct's or a union's tag -> the typedef naming it

	outs    map[string][]int // a function's out-parameters (outparams.go)
	outArg  map[*cc.UnaryExpression]bool
	outLazy map[*cc.Declarator]bool
	sval    map[*cc.Declarator]bool // struct locals that are values (structvalues.go)
	fx      map[string]*cfx         // each function's effects (effects.go)
}

// cfactsOptions are what a backend tells newCFacts.
type cfactsOptions struct {
	// exports are the functions (and objects) the host calls back: their
	// callers are written by hand
	exports []string
	// bodies are the runtime bodies in the backend's language, a function
	// whose body is given: it and every function its text names keep their
	// signatures
	bodies []handBody
	// noOuts and noStructValues leave every out-parameter and every struct
	// local in memory, as C has them
	noOuts, noStructValues bool
}

// handBody is a runtime body: the function's name and its text.
type handBody struct{ name, text string }

// newCFacts decides everything about g's translation unit.
func newCFacts(g *gen, o cfactsOptions) *cfacts {
	c := &cfacts{g: g, defined: map[string]*cc.FunctionDefinition{}, segOff: map[string]int{}, segType: map[string]cc.Type{}}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			c.defined[ed.FunctionDefinition.Declarator.Name()] = ed.FunctionDefinition
		}
	}
	c.keep = c.handWritten(o)
	c.layout()
	c.tagTypedefs()
	if o.noOuts {
		c.outs, c.outArg, c.outLazy = map[string][]int{}, map[*cc.UnaryExpression]bool{}, map[*cc.Declarator]bool{}
	} else {
		c.outParams()
	}
	if o.noStructValues {
		c.sval = map[*cc.Declarator]bool{}
	} else {
		c.structLocals()
	}
	c.effects()
	return c
}

// handWritten are the functions whose callers are written by hand: what
// the host calls back, what a runtime body names.
func (c *cfacts) handWritten(o cfactsOptions) map[string]bool {
	keep := map[string]bool{}
	for _, n := range o.exports {
		keep[n] = true
	}
	for _, b := range o.bodies {
		keep[b.name] = true
		for _, w := range bodyWordRe.FindAllString(b.text, -1) {
			if c.defined[w] != nil {
				keep[w] = true
			}
		}
	}
	return keep
}

// bodyWordRe is a name in a runtime body's text.
var bodyWordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_']*`)

// layout gives each file-scope object and each block-scope static its
// offset in the segment, aligned as C aligns it.
func (c *cfacts) layout() {
	type obj struct {
		key string
		t   cc.Type
	}
	var objs []obj
	seen := map[string]bool{}
	add := func(key string, t cc.Type) {
		if seen[key] || t == nil {
			return
		}
		seen[key] = true
		objs = append(objs, obj{key, t})
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() == nil || d.Type().Kind() == cc.Function {
				continue
			}
			add(c.g.a.declKey(d), d.Type())
		}
	}
	for _, s := range c.g.a.statics {
		add(c.g.a.declKey(s.d), s.d.Type())
	}
	off := 0
	for _, o := range objs {
		al := max(o.t.Align(), 1)
		off = (off + al - 1) / al * al
		c.segOff[o.key] = off
		c.segType[o.key] = o.t
		off += int(max(o.t.Size(), 0))
	}
	c.segSize = off
}

// anon is room in the segment for an object of type t that has no name: a
// compound literal in a file-scope object's initializer.
func (c *cfacts) anon(t cc.Type) int {
	al := max(t.Align(), 1)
	off := (c.segSize + al - 1) / al * al
	c.segSize = off + int(max(t.Size(), 1))
	return off
}

// tagTypedefs is, for each struct or union tag, the typedef that names it.
func (c *cfacts) tagTypedefs() {
	c.tagTypedef = map[string]string{}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || !d.IsTypename() || d.Type() == nil {
				continue
			}
			var tag string
			switch x := d.Type().(type) {
			case *cc.StructType:
				tk := x.Tag()
				tag = tk.SrcStr()
			case *cc.UnionType:
				tk := x.Tag()
				tag = tk.SrcStr()
			}
			if tag != "" && c.tagTypedef[tag] == "" {
				c.tagTypedef[tag] = d.Name()
			}
		}
	}
}
