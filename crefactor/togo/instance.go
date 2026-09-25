package togo

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// INSTANCE MODE.  The C keeps its state in file-scope objects, and the Go it
// is translated to keeps it in package variables: one program per process.
// Instance rewrites that Go so the state is ONE STRUCT TYPE'S FIELDS and every
// function that reaches it one of its METHODS, so a process can hold any
// number of the program, and an embedding program owns the one it runs.
//
// It works on the Go, not the C: the generator's output is parsed with
// go/parser, whose resolution says which identifier is a package-level name
// and which a local that shadows it, and the rewrite is a set of insertions
// in the text, which gofmt then prints.  The rules:
//
//   - the package-level variables of the files named in Own are the struct's
//     fields, declared where the first of them was, in their order;
//   - a function becomes a method when it names state or calls a method,
//     directly or through another function: a fixpoint over every file handed
//     in, so a helper that touches neither stays a function;
//   - a reference to state is `recv.x`; a call of a method is `recv.f(...)`;
//     a method named as a value is the method value `recv.f`, bound to the
//     instance, so a function-pointer type keeps its signature and a table of
//     them is filled per instance;
//   - `init` in an Own file is the method Init names, which the embedding
//     code calls once per instance, since Go runs init once per process.
//
// Names outside the files -- state or methods a hand-written file of the same
// package declares -- are told (Fields, Methods), as HandNames reads them.

// Instance is what the instance pass is told.
type Instance struct {
	Type     string   // the struct type: Editor
	Receiver string   // the receiver's name, used by nothing else: ed
	Init     string   // what init becomes: initGlobals
	Embed    string   // a hand-written struct the type embeds, its fields state too; "" is none
	Own      []string // the files whose package-level variables are the fields
	Fields   []string // state declared outside the files (Embed's fields)
	Methods  []string // methods declared outside the files
}

// Rewrite applies the instance pass to files, the package's Go by name, and
// returns every file it changed, printed by gofmt.
func (o Instance) Rewrite(files map[string][]byte) (map[string][]byte, error) {
	fset := token.NewFileSet()
	type file struct {
		name string
		src  []byte
		ast  *ast.File
		own  bool
	}
	var fs []*file
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	own := map[string]bool{}
	for _, n := range o.Own {
		own[n] = true
	}
	for _, n := range names {
		f, err := parser.ParseFile(fset, n, files[n], parser.ParseComments)
		if err != nil {
			return nil, err
		}
		fs = append(fs, &file{n, files[n], f, own[n]})
	}

	state := map[string]bool{}
	methods := map[string]bool{}
	for _, n := range o.Fields {
		state[n] = true
	}
	for _, n := range o.Methods {
		methods[n] = true
	}
	// objects: the package-level objects of each file, so that an identifier
	// resolved to one of them is known to be that name and not a local.
	isTop := func(f *file, id *ast.Ident) bool {
		if id.Obj == nil {
			return true // declared in another file, or the universe
		}
		return f.ast.Scope.Lookup(id.Name) == id.Obj
	}
	var funcs []*ast.FuncDecl
	funcFile := map[*ast.FuncDecl]*file{}
	for _, f := range fs {
		for _, d := range f.ast.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok == token.VAR && f.own {
					for _, s := range d.Specs {
						vs := s.(*ast.ValueSpec)
						if len(vs.Values) > 0 && vs.Type == nil {
							return nil, fmt.Errorf("instance: %s: %s has an initial value and no type; a field needs its type written", f.name, vs.Names[0].Name)
						}
						for _, id := range vs.Names {
							if id.Name != "_" {
								state[id.Name] = true
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv != nil {
					if recvIs(d, o.Type) {
						methods[d.Name.Name] = true
					}
					continue
				}
				if d.Name.Name == "init" && !f.own {
					continue
				}
				funcs = append(funcs, d)
				funcFile[d] = f
			}
		}
	}

	// refs walks the references to package-level names in n: every identifier
	// that is a name as an expression -- not a selector's field, not a key of
	// a composite literal, not a label, not a declaration's own name.
	refs := func(f *file, n ast.Node, visit func(id *ast.Ident)) {
		skip := map[*ast.Ident]bool{}
		ast.Inspect(n, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.SelectorExpr:
				skip[x.Sel] = true
			case *ast.KeyValueExpr:
				if id, ok := x.Key.(*ast.Ident); ok {
					skip[id] = true
				}
			case *ast.BranchStmt:
				if x.Label != nil {
					skip[x.Label] = true
				}
			case *ast.LabeledStmt:
				skip[x.Label] = true
			case *ast.FuncDecl:
				skip[x.Name] = true
			case *ast.Field:
				for _, id := range x.Names {
					skip[id] = true
				}
			case *ast.Ident:
				if !skip[x] && isTop(f, x) {
					visit(x)
				}
			}
			return true
		})
	}

	// the fixpoint: a function that names state or a method is a method.
	for changed := true; changed; {
		changed = false
		for _, fd := range funcs {
			name := fd.Name.Name
			if name == "init" {
				continue // init names what it initialises; it is the Init method anyway
			}
			if methods[name] {
				continue
			}
			hit := false
			refs(funcFile[fd], fd, func(id *ast.Ident) {
				if state[id.Name] || methods[id.Name] {
					hit = true
				}
			})
			if hit {
				methods[name] = true
				changed = true
			}
		}
	}
	for n := range state {
		if methods[n] {
			return nil, fmt.Errorf("instance: %s is both state and a method", n)
		}
	}

	out := map[string][]byte{}
	for _, f := range fs {
		type edit struct {
			at, end int // replace src[at:end]
			s       string
		}
		var edits []edit
		off := func(p token.Pos) int { return fset.Position(p).Offset }
		// the receiver's name must be free in every function it is added to
		conflict := ""
		for _, d := range f.ast.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			isMethod := methods[fd.Name.Name] && fd.Recv == nil && fd.Name.Name != "init"
			isInit := fd.Recv == nil && fd.Name.Name == "init" && f.own
			if isMethod || isInit || (fd.Recv != nil && recvIs(fd, o.Type)) {
				ast.Inspect(fd.Body, func(x ast.Node) bool {
					if id, ok := x.(*ast.Ident); ok && id.Name == o.Receiver && id.Obj != nil && (fd.Recv == nil) {
						conflict = fd.Name.Name
					}
					return true
				})
				if fd.Type.Params != nil && fd.Recv == nil {
					for _, p := range fd.Type.Params.List {
						for _, id := range p.Names {
							if id.Name == o.Receiver {
								conflict = fd.Name.Name
							}
						}
					}
				}
			}
			switch {
			case isInit:
				edits = append(edits, edit{off(fd.Name.Pos()), off(fd.Name.End()), fmt.Sprintf("(%s *%s) %s", o.Receiver, o.Type, o.Init)})
			case isMethod:
				edits = append(edits, edit{off(fd.Name.Pos()), off(fd.Name.Pos()), fmt.Sprintf("(%s *%s) ", o.Receiver, o.Type)})
			}
			if fd.Body != nil {
				refs(f, fd.Body, func(id *ast.Ident) {
					if state[id.Name] || methods[id.Name] {
						edits = append(edits, edit{off(id.Pos()), off(id.Pos()), o.Receiver + "."})
					}
				})
			}
		}
		if conflict != "" {
			return nil, fmt.Errorf("instance: %s: the receiver name %q is already a name in %s", f.name, o.Receiver, conflict)
		}
		// the state's declarations become the struct type
		if f.own {
			var fields strings.Builder
			var assigns []string
			first := -1
			if o.Embed != "" {
				fields.WriteString("\t" + o.Embed + "\n")
			}
			for _, d := range f.ast.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				from := off(gd.Pos())
				if gd.Doc != nil {
					from = off(gd.Doc.Pos())
				}
				for _, s := range gd.Specs {
					vs := s.(*ast.ValueSpec)
					if len(vs.Values) == 0 {
						a, b := off(vs.Pos()), off(vs.End())
						if vs.Doc != nil {
							a = off(vs.Doc.Pos())
						}
						if vs.Comment != nil {
							b = off(vs.Comment.End())
						}
						fields.WriteString("\t" + string(f.src[a:b]) + "\n")
						continue
					}
					// A declaration with a value: the field, and the value
					// assigned first thing in Init.  The value must name no
					// state: it is moved as it is written.
					bad := ""
					for _, v := range vs.Values {
						refs(f, v, func(id *ast.Ident) {
							if state[id.Name] || methods[id.Name] {
								bad = id.Name
							}
						})
					}
					if bad != "" {
						return nil, fmt.Errorf("instance: %s: the value of %s names %s", f.name, vs.Names[0].Name, bad)
					}
					names := make([]string, len(vs.Names))
					for i, id := range vs.Names {
						names[i] = id.Name
					}
					doc := ""
					if vs.Doc != nil {
						doc = string(f.src[off(vs.Doc.Pos()):off(vs.Doc.End())]) + "\n\t"
					}
					cmt := ""
					if vs.Comment != nil {
						cmt = " " + string(f.src[off(vs.Comment.Pos()):off(vs.Comment.End())])
					}
					fields.WriteString("\t" + doc + strings.Join(names, ", ") + " " + string(f.src[off(vs.Type.Pos()):off(vs.Type.End())]) + cmt + "\n")
					lhs := make([]string, len(names))
					for i, n := range names {
						lhs[i] = o.Receiver + "." + n
					}
					vals := make([]string, len(vs.Values))
					for i, v := range vs.Values {
						vals[i] = string(f.src[off(v.Pos()):off(v.End())])
					}
					assigns = append(assigns, strings.Join(lhs, ", ")+" = "+strings.Join(vals, ", "))
				}
				to := off(gd.End())
				if first < 0 {
					first = from
					edits = append(edits, edit{from, to, "\x00STRUCT\x00"})
				} else {
					edits = append(edits, edit{from, to, ""})
				}
			}
			if len(assigns) > 0 {
				var init *ast.FuncDecl
				for _, d := range f.ast.Decls {
					if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "init" {
						init = fd
					}
				}
				if init == nil {
					return nil, fmt.Errorf("instance: %s declares state with values and has no init to assign them in", f.name)
				}
				at := off(init.Body.Lbrace) + 1
				edits = append(edits, edit{at, at, "\n\t// the storage the C declares with the object\n\t" + strings.Join(assigns, "\n\t") + "\n"})
			}
			decl := fmt.Sprintf("// %s is one instance of the program: its state, every file-scope\n// object of the C, as fields.\ntype %s struct {\n%s}", o.Type, o.Type, fields.String())
			if first < 0 {
				edits = append(edits, edit{len(f.src), len(f.src), "\n" + decl + "\n"})
			} else {
				for i := range edits {
					if edits[i].s == "\x00STRUCT\x00" {
						edits[i].s = decl
					}
				}
			}
		}
		if len(edits) == 0 {
			continue
		}
		sort.SliceStable(edits, func(i, j int) bool { return edits[i].at > edits[j].at })
		src := append([]byte(nil), f.src...)
		for _, e := range edits {
			src = append(src[:e.at], append([]byte(e.s), src[e.end:]...)...)
		}
		pretty, err := format.Source(src)
		if err != nil {
			return nil, fmt.Errorf("instance: %s does not print: %v", f.name, err)
		}
		if !bytes.Equal(pretty, f.src) {
			out[f.name] = pretty
		}
	}
	return out, nil
}

// recvIs reports whether fd is a method on *typ.
func recvIs(fd *ast.FuncDecl, typ string) bool {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return false
	}
	st, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := st.X.(*ast.Ident)
	return ok && id.Name == typ
}

// HandNames reads what the hand-written files of a package tell the pass:
// the methods they declare on *typ, and the fields of embed, the struct of
// state they declare.
func HandNames(files map[string][]byte, typ, embed string) (methods, fields []string, err error) {
	fset := token.NewFileSet()
	for n, src := range files {
		f, err := parser.ParseFile(fset, n, src, 0)
		if err != nil {
			return nil, nil, err
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if recvIs(d, typ) {
					methods = append(methods, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok || ts.Name.Name != embed {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						return nil, nil, fmt.Errorf("%s: %s is not a struct", n, embed)
					}
					for _, fl := range st.Fields.List {
						for _, id := range fl.Names {
							fields = append(fields, id.Name)
						}
					}
				}
			}
		}
	}
	sort.Strings(methods)
	sort.Strings(fields)
	return methods, fields, nil
}
