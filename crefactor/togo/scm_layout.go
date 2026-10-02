package togo

// scm_layout.go is what the Scheme backend says about what it wrote: the
// layout it read and wrote the memory by, which a test holds to gcc's, and
// the counts of its shapes.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// scmStats are the counts of what the printer wrote.
type scmStats struct {
	framed, joins, loops, cases, values, tuples int
}

// layout lists every struct and union C can name at file scope -- its
// size, and each member's offset, through the members of a type that has
// no name of its own -- as the front end lays them out, which is the
// layout every load and store of the library is at:
//
//	size C SIZE
//	offset C PATH OFFSET
//
// a test prints the same with gcc's sizeof and offsetof and requires the
// two to agree.
func (s *sgen) layout() string {
	seen := map[string]bool{}
	type named struct {
		c string
		t cc.Type
	}
	var all []named
	var visit func(t cc.Type, depth int)
	visit = func(t cc.Type, depth int) {
		if t == nil || depth > 8 {
			return
		}
		switch t.Kind() {
		case cc.Ptr, cc.Array:
			visit(elemOf(t), depth+1)
			return
		case cc.Struct, cc.Union:
		default:
			return
		}
		fs := scmFields(t)
		if len(fs) == 0 {
			return
		}
		key := fmt.Sprintf("%p", fs[0])
		if seen[key] {
			return
		}
		seen[key] = true
		if c := scmCSpelling(t); c != "" && !scmBlockScope(t) {
			all = append(all, named{c, t})
		}
		for _, f := range fs {
			visit(f.Type(), depth+1)
		}
	}
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if d, ok := n.(*cc.Declarator); ok {
			visit(d.Type(), 0)
		}
		walkChildrenFn(n, rec)
	}
	for tu := s.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		rec(tu.ExternalDeclaration)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].c < all[j].c })
	var b strings.Builder
	var walk func(c, path string, t cc.Type, base int64)
	walk = func(c, path string, t cc.Type, base int64) {
		for _, f := range scmFields(t) {
			if f.Name() == "" && !isAggr(f.Type()) {
				continue
			}
			p := f.Name()
			if path != "" && p != "" {
				p = path + "." + p
			} else if p == "" {
				p = path
			}
			if f.Name() != "" {
				fmt.Fprintf(&b, "offset %s %s %d\n", strings.ReplaceAll(c, " ", "~"), p, base+f.Offset())
			}
			if ft := f.Type(); isAggr(ft) && scmCSpelling(ft) == "" {
				walk(c, p, ft, base+f.Offset())
			}
		}
	}
	for _, n := range all {
		fmt.Fprintf(&b, "size %s %d\n", strings.ReplaceAll(n.c, " ", "~"), n.t.Size())
		walk(n.c, "", n.t, 0)
	}
	return b.String()
}

// scmFields are a struct's or a union's members.
func scmFields(t cc.Type) []*cc.Field {
	var fs []*cc.Field
	switch x := t.(type) {
	case *cc.StructType:
		for i := 0; i < x.NumFields(); i++ {
			fs = append(fs, x.FieldByIndex(i))
		}
	case *cc.UnionType:
		for i := 0; i < x.NumFields(); i++ {
			fs = append(fs, x.FieldByIndex(i))
		}
	}
	return fs
}

// scmCSpelling is a struct's or a union's name in C: its typedef's, or its
// tag's; "" for a type with neither.
func scmCSpelling(t cc.Type) string {
	if td := t.Typedef(); td != nil && td.Name() != "" {
		return td.Name()
	}
	var tag string
	switch x := t.(type) {
	case *cc.StructType:
		tk := x.Tag()
		tag = tk.SrcStr()
	case *cc.UnionType:
		tk := x.Tag()
		tag = tk.SrcStr()
	}
	if tag == "" {
		return ""
	}
	if t.Kind() == cc.Union {
		return "union " + tag
	}
	return "struct " + tag
}

// scmBlockScope says a struct or a union is declared in a function, where
// C names it and nowhere else.
func scmBlockScope(t cc.Type) bool {
	var sc *cc.Scope
	switch x := t.(type) {
	case *cc.StructType:
		sc = x.LexicalScope()
	case *cc.UnionType:
		sc = x.LexicalScope()
	}
	return sc != nil && sc.Parent != nil
}
