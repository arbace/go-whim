// Package reach is a reachability closure over one C translation unit, of
// every kind of thing the old sweep's six deleting tools deleted, and it REPORTS:
// it changes no text.  doc/surveys/REACHABILITY.md, in git history before
// the sweep became a closure (internal/sweep), is the measurement it was
// built from; this is that survey's throwaway instrument made a partition.
//
// An entity is one of eight kinds:
//
//	F  a function definition at file scope
//	O  a file-scope object
//	P  a prototype for a function never defined in the translation unit
//	T  a typedef
//	S  a tagged struct or union definition
//	E  a tagged enum definition
//	M  a struct or union member
//	N  an enumerator
//
// A file-scope NAME merges all its declarators: a use resolves to whichever
// declarator the type checker picked -- in this tree the prototype near the
// top, never the definition -- so the prototype and the definition are one F.
//
// Every reference the type checker resolved is charged, by byte position, to
// the innermost entity span holding it: an identifier to its declarator or
// enumerator, a member access to the *cc.Field and through the field's
// declarator offset to its M, a type specifier to its T, S or E.  A member
// points at the type that owns it, an enumerator at its enum, a typedef at the
// tag it names.
//
// What the closure starts from is not assumed but computed, and every entity
// is put in exactly one class (Partition); a leftover is a finding.
package reach

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"

	"github.com/arbace/go-whim/internal/cc"
	"github.com/arbace/go-whim/internal/ccx"
	"modernc.org/token"
)

// The classes an entity can be in, in the order they are tried.
const (
	ClassExternal = "root: external linkage"
	ClassAssert   = "root: named by a static_assert"
	ClassCut      = "root: named across the core/host cut"
	ClassCast     = "held: a member of a struct or union punned through a pointer cast"
	ClassReached  = "reachable from a root"
)

// Options is what the closure is told about the program rather than knows:
// nothing in this package names anything in it.  whim tells it vim's
// (internal/whim, Reach).
type Options struct {
	// FreezeLayoutIf names the functions whose definition means the program
	// still reads its structs from disk, so that a struct layout is a file
	// format: while one is defined, no member is deleted (WhyFrozen).
	FreezeLayoutIf []string
	// Allocators are the calls whose void * result is fresh memory: a cast
	// of one is an allocation, not a pun (PunStats.FromAllocator).
	Allocators []string
}

// Entity is one thing the closure can keep or not.
type Entity struct {
	Kind string // F O P T S E M N
	ID   string // KIND:name, a member KIND:owner.name
	Line int    // its first declarator's line
	Name string // the bare name: a file-scope name, a tag, a member, an enumerator

	start, end int // the construct's byte span
	key        string
	reached    bool
	roots      []string // every root class that seeds it, in the order above
	referrers  map[string]bool
}

// Reachable says whether the closure reached it.
func (e *Entity) Reachable() bool { return e.reached }

// Referenced says whether anything at all -- reachable or not -- refers to
// it.  An unreachable entity nothing refers to is what gcc can see; one only
// the unreachable refer to is a level gcc does not look past.
func (e *Entity) Referenced() bool { return len(e.referrers) > 0 }

// Closure is one translation unit's entities, the edges between them and what
// the closure reached.
type Closure struct {
	Path     string
	Entities []*Entity // sorted by ID
	byKey    map[string]*Entity

	// Cut is the byte offset of the first `#include` line, the line between
	// the editor core and its host; -1 when the text has none.
	Cut int

	// Roots is every root, by class; a root can be in more than one.
	Roots map[string][]string

	left []ccx.Finding // what the instrument could not place

	// placed: member key -> the lines of the initialiser elements that place
	// a value in it by position, with no designator.
	placed map[string][]int
	// members: a struct or union definition's offset -> its members' keys,
	// in order; structOf the other way.
	members  map[int][]string
	structOf map[string]int
	// renumbers: enumerator key -> the live enumerator whose implicit value
	// deleting it would move.
	renumbers map[string]string
	// frozenBy: the first of Options.FreezeLayoutIf the text defines -- it
	// still reads its structs from disk, so a struct layout is a disk
	// format; "" when it defines none.
	frozenBy string
	// opt is what the closure was told, for the planted copy's.
	opt Options
	// memberDecl: member key -> its StructDeclaration's span and how many
	// members that declaration declares.
	memberDecl map[string][3]int
	// Pun is what the cast guard measured: see cast.go.
	Pun PunStats
}

type enumItem struct {
	key      string
	explicit bool
}

type span struct {
	start, end int
	keys       []string
}

// Analyze builds the closure of one parsed translation unit.  path is the
// name the file was parsed under and src its bytes; opt is what the closure
// is told about the program.
func Analyze(ast *cc.AST, path string, src []byte, opt Options) *Closure {
	c := &Closure{
		Path: path, byKey: map[string]*Entity{}, Cut: -1, opt: opt,
		Roots:  map[string][]string{},
		placed: map[string][]int{}, members: map[int][]string{}, structOf: map[string]int{}, renumbers: map[string]string{},
		memberDecl: map[string][3]int{},
	}
	if i := bytes.Index(src, []byte("\n#include")); i >= 0 {
		c.Cut = i + 1
	} else if bytes.HasPrefix(src, []byte("#include")) {
		c.Cut = 0
	}
	inFile := func(n interface{ Position() token.Position }) bool { return n.Position().Filename == path }
	edges := map[string]map[string]bool{}
	edge := func(from, to string) {
		if from == "" || to == "" || from == to {
			return
		}
		if edges[from] == nil {
			edges[from] = map[string]bool{}
		}
		edges[from][to] = true
	}
	add := func(kind, key, id, name string, line int) *Entity {
		e := c.byKey[key]
		if e == nil {
			e = &Entity{Kind: kind, ID: id, Name: name, Line: line, key: key, referrers: map[string]bool{}}
			c.byKey[key] = e
		}
		return e
	}
	var spans []span

	// ---- the file-scope names: F, O, P, T ----------------------------------
	defined := map[string]bool{}
	for nm, nodes := range ast.Scope.Nodes {
		for _, n := range nodes {
			if d, ok := n.(*cc.Declarator); ok && !d.IsSynthetic() && inFile(d) && d.IsFuncDef() {
				defined[nm] = true
			}
		}
	}
	for nm, nodes := range ast.Scope.Nodes {
		fn, td, ext, seen := false, false, false, false
		line := 1 << 30
		for _, n := range nodes {
			d, ok := n.(*cc.Declarator)
			if !ok || d.IsSynthetic() || !inFile(d) {
				continue
			}
			seen = true
			td = td || d.IsTypename()
			fn = fn || (d.Type() != nil && d.Type().Kind() == cc.Function)
			ext = ext || d.Linkage() == cc.External
			if l := d.Position().Line; l < line {
				line = l
			}
		}
		if !seen {
			continue
		}
		kind := "O"
		switch {
		case td:
			kind = "T"
		case fn && defined[nm]:
			kind = "F"
		case fn:
			kind = "P"
		}
		e := add(kind, "n:"+nm, kind+":"+nm, nm, line)
		if ext {
			c.root(ClassExternal, e)
		}
	}
	for _, nm := range opt.FreezeLayoutIf {
		if e := c.byKey["n:"+nm]; e != nil && e.Kind == "F" {
			c.frozenBy = nm
			break
		}
	}
	name := func(nm string) *Entity { return c.byKey["n:"+nm] }

	// What each top-level declaration declares, so that a member of a struct
	// defined inside it points at the definition typereach would delete whole.
	structOwner := map[*cc.StructOrUnionSpecifier][]string{}
	enumOwner := map[*cc.EnumSpecifier][]string{}
	tagsOf := func(ds cc.Node) (tags []string) {
		walk(ds, func(m cc.Node) {
			switch y := m.(type) {
			case *cc.StructOrUnionSpecifier:
				if y.Case == cc.StructOrUnionSpecifierDef && y.Token.SrcStr() != "" {
					tags = append(tags, "s:"+y.Token.SrcStr())
				}
			case *cc.EnumSpecifier:
				if y.Case == cc.EnumSpecifierDef && y.Token2.SrcStr() != "" {
					tags = append(tags, "e:"+y.Token2.SrcStr())
				}
			}
		})
		return tags
	}
	declared := func(d *cc.Declaration) (keys []string) {
		for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			if l.InitDeclarator == nil || l.InitDeclarator.Declarator == nil {
				continue
			}
			if e := name(l.InitDeclarator.Declarator.Name()); e != nil {
				keys = append(keys, e.key)
			}
		}
		return keys
	}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Declaration == nil {
			continue
		}
		d := ed.Declaration
		owners := append(declared(d), tagsOf(d.DeclarationSpecifiers)...)
		walk(d, func(m cc.Node) {
			switch y := m.(type) {
			case *cc.StructOrUnionSpecifier:
				if y.Case == cc.StructOrUnionSpecifierDef {
					structOwner[y] = owners
				}
			case *cc.EnumSpecifier:
				if y.Case == cc.EnumSpecifierDef {
					enumOwner[y] = owners
				}
			}
		})
	}

	// ---- the definitions: F spans, S, E, M, N, declarations ----------------
	fieldKey := map[int]string{} // a member declarator's offset -> its key
	anonSeq := map[string]int{}
	var asserts [][2]int
	var enums [][]enumItem
	displayed := map[string]int{}
	walk(ast.TranslationUnit, func(n cc.Node) {
		switch x := n.(type) {
		case *cc.FunctionDefinition:
			if e := name(x.Declarator.Name()); e != nil && e.Kind == "F" {
				s := x.DeclarationSpecifiers.Position().Offset
				en := x.CompoundStatement.Token2.Position().Offset + 1
				e.start, e.end = s, en
				spans = append(spans, span{s, en, []string{e.key}})
			}
		case *cc.StructOrUnionSpecifier:
			if x.Case != cc.StructOrUnionSpecifierDef || !inFile(x) {
				return
			}
			tag := x.Token.SrcStr()
			owners := append([]string{}, structOwner[x]...)
			owner := tag
			if owner == "" {
				owner = "<anonymous>"
				if len(owners) > 0 {
					owner = c.byKey[owners[0]].Name
				}
			}
			base := owner
			if k := anonSeq[base]; k > 0 {
				owner = fmt.Sprintf("%s#%d", base, k)
			}
			anonSeq[base]++
			if tag != "" {
				e := add("S", "s:"+tag, "S:"+tag, tag, x.Position().Line)
				e.start, e.end = x.Position().Offset, x.Token3.Position().Offset+1
				spans = append(spans, span{e.start, e.end, []string{e.key}})
				owners = append(owners, e.key)
			}
			for l := x.StructDeclarationList; l != nil; l = l.StructDeclarationList {
				sd := l.StructDeclaration
				if sd == nil {
					continue
				}
				a, b := nodeSpan(sd)
				var keys []string
				for dl := sd.StructDeclaratorList; dl != nil; dl = dl.StructDeclaratorList {
					if dl.StructDeclarator == nil || dl.StructDeclarator.Declarator == nil {
						continue
					}
					d := dl.StructDeclarator.Declarator
					if d.Name() == "" {
						continue
					}
					key := fmt.Sprintf("m:%d", d.Position().Offset)
					id := "M:" + owner + "." + d.Name()
					if k := displayed[id]; k > 0 {
						id = fmt.Sprintf("%s@%d", id, d.Position().Line)
					}
					displayed["M:"+owner+"."+d.Name()]++
					fieldKey[d.Position().Offset] = key
					e := add("M", key, id, d.Name(), d.Position().Line)
					e.start, e.end = a, b
					c.members[x.Position().Offset] = append(c.members[x.Position().Offset], key)
					c.structOf[key] = x.Position().Offset
					keys = append(keys, key)
					for _, o := range owners {
						edge(key, o)
					}
				}
				for _, k := range keys {
					c.memberDecl[k] = [3]int{a, b, len(keys)}
				}
				if len(keys) > 0 {
					spans = append(spans, span{a, b, keys})
				}
			}
		case *cc.EnumSpecifier:
			if x.Case != cc.EnumSpecifierDef || !inFile(x) {
				return
			}
			tag := x.Token2.SrcStr()
			owners := append([]string{}, enumOwner[x]...)
			var etag string
			if tag != "" {
				e := add("E", "e:"+tag, "E:"+tag, tag, x.Position().Line)
				e.start, e.end = x.Position().Offset, x.Token5.Position().Offset+1
				etag = e.key
			}
			var all []string
			var list []enumItem
			for l := x.EnumeratorList; l != nil; l = l.EnumeratorList {
				n := l.Enumerator
				if n == nil || n.Token.SrcStr() == "" {
					continue
				}
				a, b := nodeSpan(n)
				e := add("N", "n:"+n.Token.SrcStr(), "N:"+n.Token.SrcStr(), n.Token.SrcStr(), n.Position().Line)
				e.start, e.end = a, b
				spans = append(spans, span{a, b, []string{e.key}})
				edge(e.key, etag)
				for _, o := range owners {
					edge(e.key, o)
				}
				all = append(all, e.key)
				list = append(list, enumItem{e.key, n.Case == cc.EnumeratorExpr})
			}
			// The enum's head -- a C23 fixed underlying type, `enum : usize
			// {...}` -- belongs to the tag when there is one and to every
			// enumerator when there is not: it is what they all are.
			head := all
			if etag != "" {
				head = []string{etag}
			}
			if a, b := nodeSpan(x); len(head) > 0 {
				spans = append(spans, span{a, b, head})
			}
			enums = append(enums, list)
		case *cc.Declaration:
			if x.Case != cc.DeclarationDecl {
				if x.StaticAssertDeclaration != nil {
					a, b := nodeSpan(x)
					asserts = append(asserts, [2]int{a, b})
				}
				return
			}
			a, b := nodeSpan(x)
			var keys []string
			for l := x.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
				if l.InitDeclarator == nil || l.InitDeclarator.Declarator == nil {
					continue
				}
				d := l.InitDeclarator.Declarator
				if d.LexicalScope() != ast.Scope || !inFile(d) {
					continue
				}
				if e := name(d.Name()); e != nil {
					keys = append(keys, e.key)
					if e.start == 0 && e.end == 0 {
						e.start, e.end = a, b
					}
				}
			}
			if len(keys) > 0 {
				spans = append(spans, span{a, b, keys})
			}
		}
	})
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end-spans[i].start < spans[j].end-spans[j].start
	})
	ownerAt := func(p int) []string {
		best, bl := -1, 0
		for i := range spans {
			s := spans[i]
			if s.start > p {
				break
			}
			if p < s.end && (best < 0 || s.end-s.start < bl) {
				best, bl = i, s.end-s.start
			}
		}
		if best < 0 {
			return nil
		}
		return spans[best].keys
	}
	inAssert := func(p int) bool {
		for _, a := range asserts {
			if a[0] <= p && p < a[1] {
				return true
			}
		}
		return false
	}
	where := func(p token.Position) string { return fmt.Sprintf("%s:%d", p.Filename, p.Line) }

	// ---- the references ----------------------------------------------------
	ref := func(pos token.Position, to *Entity) {
		if to == nil {
			return
		}
		p := pos.Offset
		if c.Cut >= 0 && p >= c.Cut && to.start < c.Cut {
			c.root(ClassCut, to)
		}
		owners := ownerAt(p)
		if len(owners) == 0 {
			if inAssert(p) {
				c.root(ClassAssert, to)
				return
			}
			c.left = append(c.left, ccx.Finding{Fn: to.ID, Where: where(pos),
				What: "a reference charged to no entity, outside every static_assert"})
			return
		}
		for _, o := range owners {
			edge(o, to.key)
		}
	}
	walk(ast.TranslationUnit, func(n cc.Node) {
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if x.Case != cc.PrimaryExpressionIdent || !inFile(x) {
				return
			}
			switch y := x.ResolvedTo().(type) {
			case *cc.Declarator:
				if inFile(y) && y.LexicalScope() == ast.Scope {
					ref(x.Position(), name(y.Name()))
				}
			case *cc.Enumerator:
				if e := name(y.Token.SrcStr()); e != nil && e.Kind == "N" {
					ref(x.Position(), e)
				}
			}
		case *cc.PostfixExpression:
			if (x.Case != cc.PostfixExpressionSelect && x.Case != cc.PostfixExpressionPSelect) || !inFile(x) {
				return
			}
			f := x.Field()
			if f == nil || f.Name() == "" || f.Declarator() == nil {
				return
			}
			d := f.Declarator()
			key, ok := fieldKey[d.Position().Offset]
			if !ok {
				if inFile(d) {
					c.left = append(c.left, ccx.Finding{Fn: f.Name(), Where: where(x.Position()),
						What: "a member access not joined to a member declared in this file"})
				}
				return
			}
			ref(x.Position(), c.byKey[key])
		case *cc.TypeSpecifier:
			if !inFile(x) {
				return
			}
			switch x.Case {
			case cc.TypeSpecifierTypeName:
				if e := name(x.Token.SrcStr()); e != nil && e.Kind == "T" {
					ref(x.Position(), e)
				}
			case cc.TypeSpecifierStructOrUnion:
				if s := x.StructOrUnionSpecifier; s != nil && s.Token.SrcStr() != "" {
					ref(x.Position(), c.byKey["s:"+s.Token.SrcStr()])
				}
			case cc.TypeSpecifierEnum:
				if e := x.EnumSpecifier; e != nil && e.Token2.SrcStr() != "" {
					ref(x.Position(), c.byKey["e:"+e.Token2.SrcStr()])
				}
			}
		case *cc.Designator:
			// `.name =` names a member the closure does not resolve: deleting
			// that member would break the initialiser, so it is not placed.
			if (x.Case == cc.DesignatorField || x.Case == cc.DesignatorField2) && inFile(x) {
				nm := "." + x.Token2.SrcStr()
				if x.Case == cc.DesignatorField2 {
					nm = x.Token.SrcStr() + ":"
				}
				c.left = append(c.left, ccx.Finding{Fn: nm, Where: where(x.Position()),
					What: "a member named by a designator, which the closure does not resolve"})
			}
		case *cc.InitializerList:
			// An element with no designator is placed by position: deleting
			// its member, or any member before it, moves it.
			if x.Designation != nil || x.Initializer == nil || x.Initializer.Field() == nil || !inFile(x) {
				return
			}
			d := x.Initializer.Field().Declarator()
			if d == nil {
				return
			}
			if key, ok := fieldKey[d.Position().Offset]; ok {
				c.placed[key] = append(c.placed[key], x.Position().Line)
			}
		}
	})

	// A typedef names the tag it defines.
	walk(ast.TranslationUnit, func(n cc.Node) {
		d, ok := n.(*cc.Declaration)
		if !ok || d.Case != cc.DeclarationDecl {
			return
		}
		tags := tagsOf(d.DeclarationSpecifiers)
		for _, k := range declared(d) {
			for _, t := range tags {
				edge(k, t)
			}
		}
	})

	// ---- the closure -------------------------------------------------------
	for from, tos := range edges {
		for to := range tos {
			if e := c.byKey[to]; e != nil {
				e.referrers[from] = true
			}
		}
	}
	var stack []string
	for _, e := range c.byKey {
		if len(e.roots) > 0 {
			stack = append(stack, e.key)
		}
	}
	propagate := func(stack []string) {
		for len(stack) > 0 {
			k := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			e := c.byKey[k]
			if e == nil || e.reached {
				continue
			}
			e.reached = true
			for t := range edges[k] {
				stack = append(stack, t)
			}
		}
	}
	propagate(stack)
	// A member of a struct punned through a pointer cast is held, and what
	// it names is reached from it: see cast.go.
	propagate(c.punned(ast, path, fieldKey))
	// Deleting an enumerator moves every implicit value after it, up to the
	// next explicit one; it is a hazard when what moves survives.
	for _, list := range enums {
		for i, d := range list {
			if c.byKey[d.key].reached {
				continue
			}
			for _, s := range list[i+1:] {
				if s.explicit {
					break
				}
				if c.byKey[s.key].reached {
					c.renumbers[d.key] = s.key
					break
				}
			}
		}
	}

	for _, e := range c.byKey {
		c.Entities = append(c.Entities, e)
	}
	sort.Slice(c.Entities, func(i, j int) bool { return c.Entities[i].ID < c.Entities[j].ID })
	for k := range c.Roots {
		sort.Strings(c.Roots[k])
	}
	return c
}

func (c *Closure) root(class string, e *Entity) {
	for _, r := range e.roots {
		if r == class {
			return
		}
	}
	e.roots = append(e.roots, class)
	c.Roots[class] = append(c.Roots[class], e.ID)
}

// Unreachable is every entity the closure did not reach, by ID.
func (c *Closure) Unreachable() []*Entity {
	var out []*Entity
	for _, e := range c.Entities {
		if !e.reached {
			out = append(out, e)
		}
	}
	return out
}

// Partition puts every entity in exactly one class -- the first root class
// that seeds it, else reachable -- and every entity the closure did not reach
// is a finding, with the reason deleting it is not the whole story when there
// is one.  What the instrument itself could not place is a finding too.
func (c *Closure) Partition() ccx.Result {
	res := ccx.Result{Title: "reachability: every entity, by why it is kept", Classes: map[string]int{}}
	for _, cl := range []string{ClassExternal, ClassAssert, ClassCut, ClassCast, ClassReached} {
		res.Classes[cl] = 0
	}
	for _, e := range c.Entities {
		switch {
		case len(e.roots) > 0:
			res.Classes[e.firstRoot()]++
		case e.reached:
			res.Classes[ClassReached]++
		default:
			res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: fmt.Sprintf("%s:%d", c.Path, e.Line), What: c.Why(e)})
		}
	}
	res.Left = append(res.Left, c.left...)
	return res
}

// The ways an unreachable entity is found, and what deleting it would need
// beyond deleting it.  The last three are REACHABILITY.md's guards,
// carried as findings rather than filters: a filter silently keeps a thing,
// and a finding says what it would not touch and why.
const (
	WhyUnreachable = "unreachable"
	WhyFrozen      = "a struct layout is a disk format" // Why says which function froze it
	WhyPositional  = "unreachable, but initialised by position: deleting it moves or drops an initialiser"
	WhyRenumbers   = "unreachable, but deleting it renumbers a live enumerator"
)

// Why says which of the Why classes an unreachable entity is in, with its
// particulars.
func (c *Closure) Why(e *Entity) string {
	switch {
	case e.Kind == "M" && c.frozenBy != "":
		return fmt.Sprintf("unreachable, but %s is defined: %s", c.frozenBy, WhyFrozen)
	case e.Kind == "M" && c.Moves(e) > 0:
		return fmt.Sprintf("%s (%d elements from line %d)", WhyPositional, c.Moves(e), c.firstMoved(e))
	case e.Kind == "N" && c.renumbers[e.key] != "":
		return fmt.Sprintf("%s (%s)", WhyRenumbers, c.byKey[c.renumbers[e.key]].ID)
	}
	return WhyUnreachable
}

// nodeSpan is the byte range a node covers: its first token's offset to its
// last token's end.
func nodeSpan(n cc.Node) (int, int) {
	lo, hi := 1<<62, -1
	walkTok(n, func(t cc.Token) {
		p := t.Position()
		if !p.IsValid() {
			return
		}
		if p.Offset < lo {
			lo = p.Offset
		}
		if e := p.Offset + len(t.SrcStr()); e > hi {
			hi = e
		}
	})
	if hi < 0 {
		return 0, 0
	}
	return lo, hi
}

func walkTok(n cc.Node, f func(cc.Token)) {
	v := reflect.ValueOf(n)
	if n == nil || v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for i := 0; i < e.NumField(); i++ {
		if !e.Type().Field(i).IsExported() {
			continue
		}
		switch x := e.Field(i).Interface().(type) {
		case cc.Token:
			f(x)
		case cc.Node:
			walkTok(x, f)
		}
	}
}

// walk calls f on every node under n, n included, in source order.
func walk(n cc.Node, f func(cc.Node)) {
	v := reflect.ValueOf(n)
	if n == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return
	}
	f(n)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for i := 0; i < e.NumField(); i++ {
		if !e.Type().Field(i).IsExported() {
			continue
		}
		if x, ok := e.Field(i).Interface().(cc.Node); ok {
			walk(x, f)
		}
	}
}

// Moves is how many initialiser elements, placed by position, deleting a
// member would move or drop: those placed in it and in every member after it.
func (c *Closure) Moves(e *Entity) int {
	n, after := 0, false
	for _, k := range c.members[c.structOf[e.key]] {
		after = after || k == e.key
		if after {
			n += len(c.placed[k])
		}
	}
	return n
}

func (c *Closure) firstMoved(e *Entity) int {
	first, after := 0, false
	for _, k := range c.members[c.structOf[e.key]] {
		after = after || k == e.key
		for _, l := range c.placed[k] {
			if after && (first == 0 || l < first) {
				first = l
			}
		}
	}
	return first
}

// FillsLast says whether some initialiser places a value, by position, in
// the last member of e's struct: the one case where deleting e leaves gcc an
// element with nowhere to go, which it reports as a WARNING -- `excess
// elements in struct initializer` -- and exits 0.  Every other move is silent.
func (c *Closure) FillsLast(e *Entity) bool {
	ms := c.members[c.structOf[e.key]]
	return len(ms) > 0 && len(c.placed[ms[len(ms)-1]]) > 0
}

// firstRoot is the first of the root classes, in the order they are tried,
// that seeds e.
func (e *Entity) firstRoot() string {
	for _, cl := range []string{ClassExternal, ClassAssert, ClassCut, ClassCast} {
		for _, r := range e.roots {
			if r == cl {
				return cl
			}
		}
	}
	return ""
}
