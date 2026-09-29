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

// StructScalar is the step that makes a local struct of scalars one local
// per member: `pos_T a = *pp; a.col += dc; *pp = a;` becomes
//
//	typeof(((pos_T *)0)->lnum) a_lnum = (*pp).lnum; typeof(((pos_T *)0)->col) a_col = (*pp).col;
//	a_col += dc;
//	(*pp).lnum = a_lnum, (*pp).col = a_col;
//
// A target with no struct values -- Java, Clojure -- makes a local struct an
// object, allocated each time its declaration runs and copied field by
// field; the members as locals are neither.  C's copy of a struct is its
// members' copies, so this is exact where the struct is used only so: a
// local -- not static, not a parameter, alone in its declaration -- of a
// struct whose members are all named scalars or pointers, and every use of
// it one of
//
//   - s.m, not its address;
//   - s = E or E = s as a statement, E another such struct, or an
//     expression that does nothing (no call, no store), read once per
//     member where C read it once;
//   - its declaration's initializer: such an E, or braces of values by
//     position (the rest zero);
//   - the initializer of another such struct's declaration.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N
// structs are taken.
func StructScalar(core Core) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "structscalar", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		out, n, err := structScalar(p, core, text)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "  %s: %d struct locals are their members' locals\n", p.Tag, n)
		if min, ok := f["--at-least"]; ok && n < min {
			return nil, p.Die("%d structs taken, fewer than the %d asked for", n, min)
		}
		return out, nil
	}
}

// ssUse is one use of a candidate struct.
type ssUse struct {
	kind  string // "member", "set" (s = E), "store" (E = s), "init" (S t = s)
	node  cc.Node
	other cc.ExpressionNode // E
	peer  *cc.Declarator    // t, when E is another struct: s = t, t = s, S t = s
	stmt  cc.Node           // the statement an assignment is
	wonly bool              // a member use that only stores (s.m = E;): not a read
}

func structScalar(p edit.Ph, core Core, text []byte) ([]byte, int, error) {
	end := len(text)
	if core != nil {
		if e := core(text); e >= 0 {
			end = e
		}
	}
	src := append([]byte(nil), text[:end]...)
	ast, err := translate(append([]byte(nil), src...))
	if err != nil {
		return nil, 0, p.Die("the core does not type-check: %v", err)
	}
	var rws []nrw
	n := 0
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef || ed.Position().Filename != file {
			continue
		}
		r, k := ssFunction(src, ed.FunctionDefinition)
		rws = append(rws, r...)
		n += k
	}
	if n == 0 {
		return text, 0, nil
	}
	out, err := applyNested(src, rws)
	if err != nil {
		return nil, 0, p.Die("%v", err)
	}
	return append(out, text[end:]...), n, nil
}

// ssFunction is one function's rewrites, and how many structs they take.
func ssFunction(src []byte, fd *cc.FunctionDefinition) ([]nrw, int) {
	par := parents(fd.CompoundStatement)
	span := func(n cc.Node) (int, int) { return spanOf(n, src) }
	// the candidates: declared alone, in a block, of a struct of scalars
	decl := map[*cc.Declarator]*cc.Declaration{}
	sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
		d, ok := n.(*cc.Declarator)
		if !ok || d.StorageDuration() != cc.Automatic || d.IsParam() || !scalarStruct(d.Type()) || structName(d.Type()) == "struct " {
			return true
		}
		id, ok := par[d].(*cc.InitDeclarator)
		if !ok {
			return true
		}
		l, ok := par[id].(*cc.InitDeclaratorList)
		if !ok || l.InitDeclaratorList != nil {
			return true
		}
		dl, ok := par[l].(*cc.Declaration)
		if !ok {
			return true
		}
		if _, ok := par[dl].(*cc.BlockItem); !ok {
			return true
		}
		if in := id.Initializer; in != nil && !ssInitOK(in) {
			return true
		}
		decl[d] = dl
		return true
	})
	if len(decl) == 0 {
		return nil, 0
	}
	// the uses
	uses := map[*cc.Declarator][]ssUse{}
	bad := map[*cc.Declarator]bool{}
	sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
		pe, ok := n.(*cc.PrimaryExpression)
		if !ok || pe.Case != cc.PrimaryExpressionIdent {
			return true
		}
		d, _ := pe.ResolvedTo().(*cc.Declarator)
		if decl[d] == nil {
			return true
		}
		u, ok := ssUseOf(par, pe)
		if !ok {
			bad[d] = true
			return true
		}
		uses[d] = append(uses[d], u)
		return true
	})
	for changed := true; changed; {
		changed = false
		for d := range decl {
			if bad[d] {
				continue
			}
			for _, u := range uses[d] {
				// another struct in the pair must be taken too
				if u.peer != nil && (decl[u.peer] == nil || bad[u.peer]) && u.kind == "init" {
					bad[d] = true
					changed = true
					break
				}
			}
			// the declaration's own initializer, when it is another struct
			if id := decl[d].InitDeclaratorList.InitDeclarator; !bad[d] && id.Initializer != nil && id.Initializer.Case == cc.InitializerExpr {
				if t := identOf(id.Initializer.AssignmentExpression); t != nil && (decl[t] == nil || bad[t]) {
					bad[d] = true
					changed = true
				}
			}
		}
	}
	taken := map[*cc.Declarator]bool{}
	for d := range decl {
		if !bad[d] {
			taken[d] = true
		}
	}
	if len(taken) == 0 {
		return nil, 0
	}
	// the members' names
	names := map[*cc.Declarator][]string{}
	for d := range taken {
		st := d.Type().(*cc.StructType)
		for i := 0; i < st.NumFields(); i++ {
			nm := d.Name() + "_" + st.FieldByIndex(i).Name()
			for mentions(src, nm) {
				nm += "_"
			}
			names[d] = append(names[d], nm)
		}
	}
	// what is read of each: a member read, a struct stored whole, a member
	// of a struct copied into another that reads it
	read := map[*cc.Declarator][]bool{}
	for d := range taken {
		read[d] = make([]bool, len(names[d]))
	}
	for changed := true; changed; {
		changed = false
		mark := func(d *cc.Declarator, i int) {
			if !read[d][i] {
				read[d][i] = true
				changed = true
			}
		}
		for d := range taken {
			flds := d.Type().(*cc.StructType)
			for _, u := range uses[d] {
				switch u.kind {
				case "member":
					if !u.wonly {
						mark(d, indexOf(fieldNames(flds), u.node.(*cc.PostfixExpression).Token2.SrcStr()))
					}
				case "store":
					for i := range read[d] {
						if u.peer != nil && taken[u.peer] {
							if read[u.peer][i] {
								mark(d, i)
							}
							continue
						}
						mark(d, i)
					}
				case "init":
					for i := range read[d] {
						if taken[u.peer] && read[u.peer][i] {
							mark(d, i)
						}
					}
				}
			}
			// braces whose values do something keep their members
			if in := decl[d].InitDeclaratorList.InitDeclarator.Initializer; in != nil && in.Case != cc.InitializerExpr {
				i := 0
				for l := in.InitializerList; l != nil && i < len(read[d]); l, i = l.InitializerList, i+1 {
					if effects(l.Initializer.AssignmentExpression) {
						mark(d, i)
					}
				}
			}
		}
	}
	fields := func(d *cc.Declarator) []string {
		st := d.Type().(*cc.StructType)
		var out []string
		for i := 0; i < st.NumFields(); i++ {
			out = append(out, st.FieldByIndex(i).Name())
		}
		return out
	}
	// member i of expression e: another taken struct's local, or (e).m
	memberOf := func(render func(int, int) string, e cc.ExpressionNode, d *cc.Declarator, i int) string {
		if t := identOf(e); t != nil && taken[t] {
			return names[t][i]
		}
		a, z := span(e)
		return "(" + render(a, z) + ")." + fields(d)[i]
	}
	var rws []nrw
	var ds []*cc.Declarator
	for d := range taken {
		ds = append(ds, d)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i].Position().Offset < ds[j].Position().Offset })
	for _, d := range ds {
		d := d
		dl := decl[d]
		a, z := span(dl)
		id := dl.InitDeclaratorList.InitDeclarator
		sn := structName(d.Type())
		rws = append(rws, nrw{a, z, func(render func(int, int) string) string {
			vals := make([]string, len(names[d]))
			switch in := id.Initializer; {
			case in == nil:
			case in.Case == cc.InitializerExpr:
				for i := range vals {
					vals[i] = memberOf(render, in.AssignmentExpression, d, i)
				}
			default:
				i := 0
				for l := in.InitializerList; l != nil && i < len(vals); l, i = l.InitializerList, i+1 {
					ea, ez := span(l.Initializer.AssignmentExpression)
					vals[i] = render(ea, ez)
				}
				for ; i < len(vals); i++ {
					vals[i] = "0"
				}
			}
			var b strings.Builder
			for i, nm := range names[d] {
				if !read[d][i] {
					continue // nothing reads it
				}
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				fmt.Fprintf(&b, "typeof(((%s *)0)->%s) %s", sn, fields(d)[i], nm)
				if vals[i] != "" {
					b.WriteString(" = " + vals[i])
				}
				b.WriteString(";")
			}
			return b.String()
		}})
		for _, u := range uses[d] {
			u, d := u, d
			switch u.kind {
			case "member":
				sel := u.node.(*cc.PostfixExpression)
				i := indexOf(fields(d), sel.Token2.SrcStr())
				if u.wonly && !read[d][i] {
					// a store nothing reads: what the value does, if anything
					a, z := span(u.stmt)
					rhs := u.other
					rws = append(rws, nrw{a, z, func(render func(int, int) string) string {
						if !effects(rhs) {
							return ";"
						}
						ra, rz := span(rhs)
						return render(ra, rz) + ";"
					}})
					continue
				}
				a, z := span(sel)
				rws = append(rws, nrw{a, z, func(func(int, int) string) string { return names[d][i] }})
			case "set", "store":
				// a statement of its own a member, so that the sweep takes
				// one nothing reads
				a, z := span(u.stmt)
				if u.kind == "set" && u.peer != nil && taken[u.peer] && u.peer == d {
					continue
				}
				rws = append(rws, nrw{a, z, func(render func(int, int) string) string {
					var parts []string
					for i, nm := range names[d] {
						if u.kind == "set" && !read[d][i] || u.kind == "store" && u.peer != nil && taken[u.peer] && !read[u.peer][i] {
							continue // nothing reads it
						}
						if u.kind == "set" {
							parts = append(parts, nm+" = "+memberOf(render, u.other, d, i))
							continue
						}
						if t := identOf(u.other); t != nil && taken[t] {
							parts = append(parts, names[t][i]+" = "+nm)
							continue
						}
						oa, oz := span(u.other)
						parts = append(parts, "("+render(oa, oz)+")."+fields(d)[i]+" = "+nm)
					}
					if len(parts) == 0 {
						return ";"
					}
					return "{ " + strings.Join(parts, "; ") + "; }"
				}})
			}
		}
	}
	// an assignment between two taken structs is rewritten from one side
	var out []nrw
	seen := map[[2]int]bool{}
	for _, r := range rws {
		k := [2]int{r.a, r.z}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out, len(taken)
}

func indexOf(xs []string, x string) int {
	for i, s := range xs {
		if s == x {
			return i
		}
	}
	return -1
}

// scalarStruct says t is a struct whose members are all named scalars or
// pointers, none a bit field.
func scalarStruct(t cc.Type) bool {
	st, ok := t.(*cc.StructType)
	if !ok || st.NumFields() == 0 {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		fl := st.FieldByIndex(i)
		ft := fl.Type()
		if fl.Name() == "" || fl.IsBitfield() || ft == nil {
			return false
		}
		switch ft.Kind() {
		case cc.Struct, cc.Union, cc.Array, cc.Function, cc.Void:
			return false
		}
	}
	return true
}

// ssInitOK says a candidate's initializer is one the rewrite writes: an
// expression that does nothing, or braces of values by position.
func ssInitOK(in *cc.Initializer) bool {
	if in.Case == cc.InitializerExpr {
		return !effects(in.AssignmentExpression)
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		if l.Designation != nil || l.Initializer == nil || l.Initializer.Case != cc.InitializerExpr {
			return false
		}
	}
	return true
}

// ssUseOf is the use of a candidate at pe, or false when it is one the
// rewrite cannot write.
func ssUseOf(par map[cc.Node]cc.Node, pe *cc.PrimaryExpression) (ssUse, bool) {
	u := up(par, pe)
	switch x := u.(type) {
	case *cc.PostfixExpression:
		if x.Case != cc.PostfixExpressionSelect || unparen(x.PostfixExpression) != cc.Node(pe) {
			return ssUse{}, false
		}
		if a, ok := up(par, x).(*cc.UnaryExpression); ok && a.Case == cc.UnaryExpressionAddrof {
			return ssUse{}, false
		}
		use := ssUse{kind: "member", node: x}
		if as, ok := up(par, x).(*cc.AssignmentExpression); ok && as.Case == cc.AssignmentExpressionAssign && unparen(as.UnaryExpression) == cc.Node(x) {
			if es, ok := up(par, as).(*cc.ExpressionStatement); ok {
				use.wonly, use.stmt, use.other = true, es, as.AssignmentExpression
			}
		}
		return use, true
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionAssign {
			return ssUse{}, false
		}
		es, ok := up(par, x).(*cc.ExpressionStatement)
		if !ok {
			return ssUse{}, false
		}
		if unparen(x.UnaryExpression) == cc.Node(pe) {
			other := x.AssignmentExpression
			if effects(other) {
				return ssUse{}, false
			}
			return ssUse{kind: "set", node: x, other: other, peer: identOf(other), stmt: es}, true
		}
		other := x.UnaryExpression
		if effects(other) {
			return ssUse{}, false
		}
		return ssUse{kind: "store", node: x, other: other, peer: identOf(other), stmt: es}, true
	case *cc.Initializer:
		// the initializer of another struct's declaration: written there
		if id, ok := par[x].(*cc.InitDeclarator); ok && x.Case == cc.InitializerExpr && id.Declarator != nil {
			return ssUse{kind: "init", node: x, peer: id.Declarator}, true
		}
	}
	return ssUse{}, false
}

func fieldNames(st *cc.StructType) []string {
	var out []string
	for i := 0; i < st.NumFields(); i++ {
		out = append(out, st.FieldByIndex(i).Name())
	}
	return out
}
