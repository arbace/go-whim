package togo

// rs.go is the Rust backend's frame (doc/RUST.md): the module a translation
// unit becomes -- whimsy's, the fifth translation.  Rust has C's memory
// natively, so the translation keeps it as the C keeps it, as the Haskell
// does, but with no offsets of its own:
//
//   - every struct and union is a #[repr(C)] type, laid out as the C lays
//     it out (a layout test holds it to gcc's offsetof);
//   - a pointer is a raw pointer, *mut T, walked, compared, subtracted and
//     cast as C does; a function pointer an Option<unsafe fn(...)>; no Rust
//     reference to a C object is ever made: a member is reached through its
//     pointer, `(*p).m`, and an address is `&raw mut`;
//   - the file-scope objects and the block-scope statics are the fields of
//     one #[repr(C)] Editor, made zeroed and never moved, its initial
//     values written into it when it is made (init_globals); a function is
//     an `unsafe fn` of `ed: *mut Editor` and the C's parameters -- the
//     editor only where it reaches the editor, `unsafe` only where it does
//     what Rust calls unsafe (rs_fx.go);
//   - C's control flow is Rust's: return, break and continue as they are, a
//     goto a labeled block it breaks (as the Java's), a switch a match -- or,
//     where a case falls into the next, a ladder of labeled blocks;
//   - an expression's side effects are Rust's blocks, `{ x = v; x }`, which
//     are expressions: what C does inside an expression, in C's order;
//   - C's arithmetic is exact: the widths of C's types, wrapping_* for what
//     can overflow, `as` for C's conversions;
//   - a function declared and not defined is the host's, called in the host
//     module with the editor first; a variadic one takes a slice of VArg.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// rgen is the Rust backend's state for one translation unit.
type rgen struct {
	g       *gen
	host    string // the module the host's functions are called in
	defined map[string]*cc.FunctionDefinition
	hostFns map[string]*cc.Declarator
	fnName  map[string]string // a C function -> its Rust name

	// the types (rs_types.go)
	structNames map[string]string // a struct's identity -> its Rust name
	structTaken map[string]bool
	tagTypedef  map[string]string
	structType  map[string]cc.Type // a Rust struct name -> its C type
	structOrder []string
	aliases     map[string]string // a typedef's Rust alias name -> what it stands for
	aliasOrder  []string
	aliasOf     map[*cc.Declarator]string

	// the values
	consts     map[string]*rconst // an enumerator's name -> its constant
	constOrder []string
	field      map[string]string // an object's key -> its Editor field
	objects    []*robj
	valueTaken map[string]bool // names a local may not take: functions, constants, the runtime's

	// what each function does, as its signature says it (rs_fx.go)
	fx map[string]*rsFx

	// what the functions were written with, for the coverage line
	nMatch, nLadder, nGoto, nLowered int
}

// rconst is an enumerator as a Rust constant.
type rconst struct {
	name string
	ty   string // its Rust type
	v    int64
}

// robj is a file-scope object, or a block-scope static, as a field of the
// Editor.
type robj struct {
	key, field string
	t          cc.Type
	in         *cc.Initializer
	what       string // for the comments and the refusals: its C name
}

// rsReserved are Rust's keywords and the names the generated code uses: a C
// name among them takes a trailing underscore.
var rsReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`as async await break const continue crate dyn else enum extern false fn for if impl in
	let loop match mod move mut pub ref return self Self static struct super trait true type unsafe use where while
	abstract become box do final macro override priv typeof unsized virtual yield try gen union
	i8 i16 i32 i64 i128 isize u8 u16 u32 u64 u128 usize f32 f64 bool char str
	Some None Ok Err Option Result Box Vec String drop
	ed Editor VArg c_void null_mut zeroed at decay pdiff vtruth host_ new_editor init_globals size_of`) {
		rsReserved[w] = true
	}
}

// rsName is a C name as a Rust item, field or local: one Rust reserves takes
// a trailing underscore.
func rsName(s string) string {
	if s == "" {
		return "anon_"
	}
	if strings.Trim(s, "_") == "" {
		s += "x" // `_` is no identifier
	}
	for rsReserved[s] {
		s += "_"
	}
	return s
}

// writeRs writes the translation unit as the Rust module path, and beside
// it path.refused (the functions it could not write, with the reason),
// path.host (the host functions it calls, as Rust signatures) and
// path.layout (each struct's members and their offsets, for the layout
// test).
func (g *gen) writeRs(path string) error {
	r := &rgen{g: g, host: g.p.RsHost, defined: map[string]*cc.FunctionDefinition{}, hostFns: map[string]*cc.Declarator{},
		fnName: map[string]string{}, structNames: map[string]string{}, structTaken: map[string]bool{}, tagTypedef: map[string]string{},
		structType: map[string]cc.Type{}, aliases: map[string]string{}, aliasOf: map[*cc.Declarator]string{},
		consts: map[string]*rconst{}, field: map[string]string{}, valueTaken: map[string]bool{}}
	if r.host == "" {
		r.host = "crate::host"
	}
	var fds []*cc.FunctionDefinition
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			fd := ed.FunctionDefinition
			r.defined[fd.Declarator.Name()] = fd
			fds = append(fds, fd)
		}
	}
	for name, d := range g.a.fnDecls {
		if r.defined[name] == nil && !strings.HasPrefix(name, "__") {
			r.hostFns[name] = d
		}
	}
	for name := range r.defined {
		r.fnName[name] = rsName(name)
		r.valueTaken[r.fnName[name]] = true
	}
	r.tagTypedefs()
	r.enumerators()
	r.objectFields()
	r.typedefAliases()
	r.effects()

	var report strings.Builder
	var funcs []string
	written := 0
	for _, fd := range fds {
		src, why := r.function(fd)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", fd.Declarator.Name(), why)
			funcs = append(funcs, r.stub(fd, why))
			continue
		}
		written++
		funcs = append(funcs, src)
	}
	inits, failed := r.initializers()
	for _, f := range failed {
		fmt.Fprintf(&report, "initial value of %s\n", f)
	}
	editor := r.editorStruct()
	types := r.typeDefs() // after everything has named its types

	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by `go tool whim skel -rs` from a C translation unit; DO NOT EDIT.\n// %d of %d functions written; the rest are stubs that panic, with the reason.\n\n", written, len(fds))
	var body strings.Builder
	body.WriteString(types)
	body.WriteString(r.constDefs())
	body.WriteString(editor)
	body.WriteString(inits)
	for _, f := range funcs {
		body.WriteString("\n" + f)
	}
	b.WriteString(r.prelude(body.String()))
	b.WriteString(body.String())
	nSafe, nNoEd := 0, 0
	for _, fx := range r.fx {
		if !fx.unsafe {
			nSafe++
		}
		if !fx.editor {
			nNoEd++
		}
	}
	fmt.Fprintf(logw, "rs: %d of %d functions written, %d refused, %d from the lowered form; %d safe, %d without the editor; %d switches matches, %d ladders; %d gotos' labeled blocks; %d structs and unions, %d fields of the editor\n",
		written, len(fds), len(fds)-written, r.nLowered, nSafe, nNoEd, r.nMatch, r.nLadder, r.nGoto, len(r.structOrder), len(r.objects))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(path+".host", []byte(r.hostSigs()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(path+".layout", []byte(r.layout()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(path+".refused", []byte(report.String()), 0o644)
}

// rsRuntimeNames are the runtime's names the generated code may use.
var rsRuntimeNames = regexp.MustCompile(`\b(decay|pdiff|fn_addr|str_u8|str_i8|VArg|chunks|Shared)\b`)

// prelude is what the module's body uses of the runtime and of std.
func (r *rgen) prelude(body string) string {
	rt := r.g.p.RsRuntime
	if rt == "" {
		rt = "crate::rt"
	}
	code := rsStrRe.ReplaceAllString(body, `b""`)
	s := "#![allow(non_snake_case, non_camel_case_types, non_upper_case_globals, unused_assignments)]\n\n"
	if rsRuntimeNames.MatchString(code) {
		s += "use " + rt + "::*;\n"
	}
	s += "use core::ffi::c_void;\n"
	if strings.Contains(code, "null_mut") {
		s += "use core::ptr::null_mut;\n"
	}
	return s + "\n"
}

// stub is a refused function: its signature, and a body that panics.
func (r *rgen) stub(fd *cc.FunctionDefinition, why string) string {
	sig := r.signature(fd.Declarator, nil, nil)
	return fmt.Sprintf("// REFUSED: %s\n#[allow(unused_variables)]\n%s {\n    panic!(%q);\n}\n", why, sig, fd.Declarator.Name()+": "+why)
}

// enumerators are the unit's enumerators as constants, each of its own type.
func (r *rgen) enumerators() {
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if e, ok := n.(*cc.Enumerator); ok {
			name := e.Token.SrcStr()
			if _, ok := r.consts[name]; !ok {
				v, known := intValue(e.Value())
				k, okk := scalarKind(e.Type())
				if known && okk {
					v = truncK(v, k)
					c := &rconst{name: rsName(name), ty: rsKind(k), v: v}
					r.consts[name] = c
					r.constOrder = append(r.constOrder, name)
					r.valueTaken[c.name] = true
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	for tu := r.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case == cc.ExternalDeclarationDecl {
			rec(ed.Declaration)
		}
		if ed.Case == cc.ExternalDeclarationFuncDef {
			rec(ed.FunctionDefinition.CompoundStatement)
		}
	}
}

func (r *rgen) constDefs() string {
	var b strings.Builder
	for _, n := range r.constOrder {
		c := r.consts[n]
		fmt.Fprintf(&b, "pub const %s: %s = %s;\n", c.name, c.ty, rsLit(c.v, c.ty))
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	return b.String()
}

// objectFields gives each file-scope object and each block-scope static a
// field of the Editor: its own name, a static its function's and its own.
func (r *rgen) objectFields() {
	taken := map[string]bool{"host": true}
	add := func(key, name string, t cc.Type, in *cc.Initializer, what string) {
		if _, ok := r.field[key]; ok {
			for _, o := range r.objects {
				if o.key == key && o.in == nil {
					o.in = in // the definition after a tentative one
				}
			}
			return
		}
		if t == nil {
			return
		}
		n := rsName(name)
		for taken[n] {
			n += "_"
		}
		taken[n] = true
		r.field[key] = n
		r.objects = append(r.objects, &robj{key: key, field: n, t: t, in: in, what: what})
	}
	for tu := r.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() == nil || d.Type().Kind() == cc.Function {
				continue
			}
			add(r.g.a.declKey(d), d.Name(), d.Type(), l.InitDeclarator.Initializer, d.Name())
		}
	}
	for _, s := range r.g.a.statics {
		add(r.g.a.declKey(s.d), s.fn+"__"+s.d.Name(), s.d.Type(), s.init, s.fn+"."+s.d.Name())
	}
	// a tentative definition's array of unknown size is complete at its
	// definition: the type is the last declaration's
	for tu := r.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() == nil || d.Type().Kind() == cc.Function {
				continue
			}
			for _, o := range r.objects {
				if o.key == r.g.a.declKey(d) && d.Type().Size() > o.t.Size() {
					o.t = d.Type()
				}
			}
		}
	}
}

// editorStruct is the Editor: the objects as its fields, in the C's order,
// and the host's pointer, which the C does not have.
func (r *rgen) editorStruct() string {
	var b strings.Builder
	b.WriteString("/// The core's file-scope objects and block-scope statics: one editor's state.\n#[repr(C)]\npub struct Editor {\n")
	for _, o := range r.objects {
		fmt.Fprintf(&b, "    pub %s: %s,\n", o.field, r.declType(o.t))
	}
	b.WriteString("    /// the host the editor runs on: the hand-written glue's, not the C's\n    pub host: *mut c_void,\n}\n\n")
	b.WriteString("/// A new editor, zeroed and boxed, never to move: its initial values written.\npub fn new_editor() -> *mut Editor {\n" +
		"    unsafe {\n        let ed: *mut Editor = Box::into_raw(Box::<Editor>::new_zeroed().assume_init());\n" +
		"        init_globals(ed);\n        ed\n    }\n}\n\n")
	return b.String()
}

// hostSigs is the Rust signature of each function the module calls in the
// host: what the host module must define.
func (r *rgen) hostSigs() string {
	var names []string
	for n := range r.hostFns {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		d := r.hostFns[n]
		ft, ok := d.Type().(*cc.FunctionType)
		if !ok {
			continue
		}
		ps := []string{"ed: *mut Editor"}
		for i, p := range ft.Parameters() {
			if p.Type() == nil || p.Type().Kind() == cc.Void {
				continue
			}
			t := p.Type()
			if t.Kind() == cc.Array {
				t = t.Decay()
			}
			pn := p.Name()
			if pn == "" {
				pn = fmt.Sprintf("p%d", i)
			}
			ps = append(ps, rsName(pn)+": "+r.declType(t))
		}
		if ft.IsVariadic() {
			ps = append(ps, "args: &[VArg]")
		}
		res := ""
		if rt := ft.Result(); rt != nil && rt.Kind() != cc.Void {
			res = " -> " + r.declType(rt)
		}
		fmt.Fprintf(&b, "pub unsafe fn %s(%s)%s\n", rsName(n), strings.Join(ps, ", "), res)
	}
	return b.String()
}
