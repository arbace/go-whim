package togo

// scm.go is the Scheme backend's frame (doc/SCHEME.md): the R6RS library a
// translation unit becomes -- whimsical's, the sixth translation, for Chez
// Scheme.  As the Haskell does, it keeps C's memory: every object of an
// editor lives in one bytevector, laid out as the C lays it out on amd64
// (the C front end's sizes and offsets), and a pointer is an offset into
// it, a fixnum (whimsical/whimsical/rt.ss).  So nothing of the pointer
// analysis is needed, and the functions are the lowered form (lower.go)
// printed as Scheme (scm_fn.go, scm_expr.go):
//
//   - the bytevector's first 64 KiB are the null page, where the function
//     pointers are -- an index into the table of the functions whose address
//     is taken; the file-scope objects and the block-scope statics follow,
//     each at its offset, and the string literals after them: every address
//     the core names is a constant, and the initial values one image;
//   - a function is a top-level procedure of the editor and the C's
//     parameters; its blocks are local procedures of the variables live at
//     their start -- joins and loops' heads, a jump a tail call -- and a
//     local that is an array, a struct or a union, or whose address is
//     taken, lives in the call's frame on the thread's stack in memory;
//   - a function declared and not defined is the host's, called through
//     the vector of the host's procedures the editor carries.
//
// What the Haskell backend decides about the C and not about Haskell -- the
// segment's layout, the out-parameters (hsout.go), the struct locals that
// are values (hsstruct.go), what each function touches (hseffects.go) -- is
// asked of it, so that the two read the C alike.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// scmBase is where the file-scope objects start: below it, the null page
// and the function pointers.
const scmBase = 65536

// sgen is the Scheme backend's state for one translation unit.
type sgen struct {
	g        *gen
	h        *hgen // the Haskell backend's analyses of the same C
	library  string
	defined  map[string]*cc.FunctionDefinition
	hostFns  map[string]*cc.Declarator
	hostList []string
	names    map[string]string // a C function -> its Scheme name
	fnIdx    map[string]int
	fnOrder  []string

	image   []byte         // from scmBase: the segment's initial bytes
	fixups  map[int]string // an image offset -> the literal whose address is written there
	lits    map[string]int // a string literal -> its offset in the pool
	pool    []byte
	litBase int // where the pool starts
	st      scmStats

	// the names the library defines for what its functions use
	objects     map[string]*sobj // an object's key -> its name
	objNames    map[string]bool
	members     map[string]*smember // an accessor's name -> what it reads
	structNames map[string]string   // a struct type (its first member) -> its name
	structTaken map[string]string   // a name -> the struct type it names
}

// sobj is a file-scope object by name: at addr, of the kind its accessors
// read (s32, ptr...; agg for an array or a struct, whose name is its
// address).
type sobj struct {
	name, kind string
	addr       int
	used       bool
}

// smember is a member's accessor: the kind it reads (agg: its address
// alone) at off from the struct's address.
type smember struct {
	kind string
	off  int
}

// scmReserved are the names a C name may not take: the Scheme the library
// imports, the runtime's, and the generated code's own.
var scmReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(scmRnrs + " " + scmRuntimeNames) {
		scmReserved[w] = true
	}
}

// scmRuntimeNames are the names the generated library uses besides
// (rnrs)'s: whimsical/whimsical/rt.ss's, and its own.
const scmRuntimeNames = `ed ed? ed-mem ed-sp ed-sp-set! ed-glue make-editor ed-fork exit-condition? exit-condition-code raise-exit
arena-alloc ld-s8 ld-u8 ld-s16 ld-u16 ld-s32 ld-u32 ld-s64 ld-u64 ld-ptr ld-bool
st-s8! st-u8! st-s16! st-u16! st-s32! st-u32! st-s64! st-u64! st-ptr! st-bool!
mem-copy! mem-zero! mem-fill! mem-image! mem-bytes mem-string frame-push! frame-pop! c-str
mem-ref mem-set! define-c-object define-c-local define-c-member
->i8 ->u8 ->i16 ->u16 ->i32 ->u32 ->i64 ->u64 b->i
i32+ i32- i32* i32/ i32% i32<< i32>> u32+ u32- u32* u32/ u32% u32<< u32>>
i64+ i64- i64* i64/ i64% i64<< i64>> u64+ u64- u64* u64/ u64% u64<< u64>> u32~ u64~
fn-ptr fn-index void chunks fxquotient
mem fr sret new-editor fn-table host-names call-ptr data-end`

// scmName is a C name as a Scheme identifier: one that Scheme or the
// runtime has, or that R6RS cannot spell, takes a trailing underscore.
func scmName(s string) string {
	if s == "" {
		return "anon_"
	}
	for scmReserved[s] {
		s += "_"
	}
	return s
}

// writeScm writes the translation unit as the R6RS library path, and beside
// it path.refused: the functions it could not write, with the reason; and
// path.host, the host's functions, in the order of the vector the editor
// carries.
func (g *gen) writeScm(path string) error {
	s := &sgen{g: g, library: g.p.ScmLibrary, defined: map[string]*cc.FunctionDefinition{},
		hostFns: map[string]*cc.Declarator{}, names: map[string]string{}, fnIdx: map[string]int{},
		fixups: map[int]string{}, lits: map[string]int{}, objects: map[string]*sobj{}, objNames: map[string]bool{},
		members: map[string]*smember{}, structNames: map[string]string{}, structTaken: map[string]string{}}
	if s.library == "" {
		s.library = "(editor)"
	}
	s.h = s.analyses()
	var fds []*cc.FunctionDefinition
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			fd := ed.FunctionDefinition
			s.defined[fd.Declarator.Name()] = fd
			fds = append(fds, fd)
		}
	}
	for name, d := range g.a.fnDecls {
		if s.defined[name] == nil && !strings.HasPrefix(name, "__") {
			s.hostFns[name] = d
			s.hostList = append(s.hostList, name)
		}
	}
	sort.Strings(s.hostList)
	for name := range s.defined {
		s.names[name] = scmName(name)
	}
	for name := range s.hostFns {
		s.names[name] = scmName(name)
	}

	var report strings.Builder
	// the initial values first: they make the segment's last objects (the
	// compound literals of initializers)
	failed := s.initializers()
	for _, f := range failed {
		fmt.Fprintf(&report, "initial value of %s\n", f)
	}
	s.litBase = scmAlign(scmBase+s.h.segSize, 16)
	var funcs []string
	written := 0
	for _, fd := range fds {
		src, why := s.function(fd)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", fd.Declarator.Name(), why)
			funcs = append(funcs, s.stub(fd, why))
			continue
		}
		written++
		funcs = append(funcs, src)
	}

	var b strings.Builder
	fmt.Fprintf(&b, ";; Code generated by `go tool whim skel -scm` from a C translation unit; DO NOT EDIT.\n;; %d of %d functions written; the rest are stubs that fail, with the reason.\n", written, len(fds))
	b.WriteString(s.header())
	b.WriteString(s.dataDefs())
	b.WriteString(s.hostDefs())
	b.WriteString(s.nameDefs())
	for _, f := range funcs {
		b.WriteString("\n" + f)
	}
	b.WriteString("\n" + s.table())
	b.WriteString(")\n")
	outs := 0
	for _, is := range s.h.outs {
		outs += len(is)
	}
	text := b.String()
	fmt.Fprintf(logw, "scm: %d of %d functions written, %d refused, %d lines; %d with a frame, %d joins and %d loops as local procedures, %d switches a case; %d out-parameters in %d functions and %d struct results as values, %d struct locals as bindings; %d bytes of objects, %d of literals, %d functions in the table\n",
		written, len(fds), len(fds)-written, strings.Count(text, "\n"), s.st.framed, s.st.joins, s.st.loops, s.st.cases, outs, s.st.values, s.st.tuples, len(s.h.sval), s.h.segSize, len(s.pool), len(s.fnOrder))
	if err := os.WriteFile(path+".layout", []byte(s.layout()), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path+".host", []byte(strings.Join(s.hostList, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(path+".refused", []byte(report.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// analyses are the Haskell backend's decisions about the same C, asked with
// the Scheme's exports and runtime bodies in the Haskell's places: the
// layout, the out-parameters, the struct values, the effects.
func (s *sgen) analyses() *hgen {
	p := *s.g.p
	p.HsExports = s.g.p.ScmExports
	p.RuntimeBodies = nil
	for _, rb := range s.g.p.RuntimeBodies {
		if rb.Scm != nil {
			p.RuntimeBodies = append(p.RuntimeBodies, RuntimeBody{Name: rb.Name, Hs: rb.Scm})
		}
	}
	g := *s.g
	g.p = &p
	h := &hgen{g: &g, defined: map[string]*cc.FunctionDefinition{}, hostFns: map[string]*cc.Declarator{},
		segOff: map[string]int{}, fnIdx: map[string]int{}, names: map[string]string{}, variadic: map[string]bool{},
		segType: map[string]cc.Type{}, exported: map[string]bool{}, memberOff: map[string]int64{}, enumVal: map[string]int64{},
		tyDef: map[string]string{}, tyOf: map[string]string{}, tySyn: map[string]string{}, bootSigs: map[string]string{}}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			h.defined[ed.FunctionDefinition.Declarator.Name()] = ed.FunctionDefinition
		}
	}
	for name, d := range g.a.fnDecls {
		if h.defined[name] == nil && !strings.HasPrefix(name, "__") {
			h.hostFns[name] = d
		}
	}
	for name := range h.defined {
		h.names[name] = hsName(name)
	}
	h.layout()
	h.tagTypedefs()
	if !s.g.p.ScmNoOuts {
		h.outParams()
	} else {
		h.outs, h.outArg, h.outLazy = map[string][]int{}, map[*cc.UnaryExpression]bool{}, map[*cc.Declarator]bool{}
	}
	if !s.g.p.ScmNoStructValues {
		h.structLocals()
	} else {
		h.sval = map[*cc.Declarator]bool{}
	}
	h.effects()
	return h
}

func scmAlign(n, a int) int { return (n + a - 1) / a * a }

// header is the library's opening: its name, what it exports and imports.
func (s *sgen) header() string {
	var ex []string
	ex = append(ex, "new-editor", "host-names", "data-end")
	for _, n := range s.g.p.ScmExports {
		if s.defined[n] != nil {
			ex = append(ex, s.names[n])
		} else if off, ok := s.h.segOff["global:"+n]; ok {
			ex = append(ex, "addr:"+n)
			_ = off
		}
	}
	return fmt.Sprintf("(library %s\n  (export %s)\n  (import (rnrs) (whimsical rt))\n\n", s.library, strings.Join(ex, " ")) +
		";; A call through a function pointer: its index in the table.\n(define-syntax call-ptr\n  (syntax-rules () [(_ p ed a ...) ((vector-ref fn-table (fn-index p)) ed a ...)]))\n\n"
}

// dataDefs are the memory's layout and its initial image: what new-editor
// copies into a new editor's bytevector.
func (s *sgen) dataDefs() string {
	// the literals' addresses in the image
	ats := make([]int, 0, len(s.fixups))
	for at := range s.fixups {
		ats = append(ats, at)
	}
	sort.Ints(ats)
	for _, at := range ats {
		addr := uint64(s.literal(s.fixups[at]))
		for i := 0; i < 8; i++ {
			s.poke(at+i, 1, (addr>>(8*i))&0xff)
		}
	}
	end := s.litBase + len(s.pool)
	var b strings.Builder
	fmt.Fprintf(&b, ";; The memory: the null page and the function pointers below %d, the\n;; file-scope objects from there, the string literals from %d to %d.\n", scmBase, s.litBase, end)
	fmt.Fprintf(&b, "(define data-end %d)\n\n", scmAlign(end, 16))
	b.WriteString(";; A new editor on the host's procedures glue (a vector, in host-names'\n;; order): its memory, with the objects' initial values and the literals.\n")
	b.WriteString("(define (new-editor glue)\n  (let* ([ed (make-editor glue data-end)] [mem (ed-mem ed)])\n")
	for _, l := range scmImage(s.image, scmBase) {
		b.WriteString("    " + l + "\n")
	}
	for _, l := range scmImage(s.pool, s.litBase) {
		b.WriteString("    " + l + "\n")
	}
	b.WriteString("    ed))\n\n")
	for _, n := range s.g.p.ScmExports {
		if off, ok := s.h.segOff["global:"+n]; ok && s.defined[n] == nil {
			fmt.Fprintf(&b, ";; The address of %s, for the host.\n(define addr:%s %d)\n\n", n, n, scmBase+off)
		}
	}
	return b.String()
}

// scmImage are the lines that copy img into memory at base: its runs of
// bytes that are not zero, as strings of at most 1 KB, a byte a character.
func scmImage(img []byte, base int) []string {
	var out []string
	for i := 0; i < len(img); {
		if img[i] == 0 {
			i++
			continue
		}
		j, zeros := i, 0
		for j < len(img) && j-i < 1024 && zeros < 32 {
			if img[j] == 0 {
				zeros++
			} else {
				zeros = 0
			}
			j++
		}
		end := j - zeros
		out = append(out, fmt.Sprintf("(mem-image! mem %d %s)", base+i, scmString(img[i:end])))
		i = end
	}
	return out
}

// scmString is bytes as a Scheme string literal, a byte a character.
func scmString(bs []byte) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range bs {
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\t':
			b.WriteString(`\t`)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\x%x;`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// hostDefs are the host's functions: each a procedure that calls the
// host's, from the vector the editor carries.
func (s *sgen) hostDefs() string {
	var b strings.Builder
	b.WriteString(";; The host's functions, in the order of the vector of procedures an\n;; editor is made on.\n")
	fmt.Fprintf(&b, "(define host-names '#(%s))\n", strings.Join(s.hostList, " "))
	for i, n := range s.hostList {
		d := s.hostFns[n]
		ft, _ := d.Type().(*cc.FunctionType)
		var ps []string
		if ft != nil {
			for k, p := range ft.Parameters() {
				if p.Type() == nil || p.Type().Kind() == cc.Void {
					continue
				}
				ps = append(ps, fmt.Sprintf("a%d", k))
			}
			if ft.IsVariadic() {
				ps = append(ps, "args")
			}
		}
		fmt.Fprintf(&b, "(define (%s %s) ((vector-ref (ed-glue ed) %d) %s))\n", s.names[n], strings.Join(append([]string{"ed"}, ps...), " "), i, strings.Join(append([]string{"ed"}, ps...), " "))
	}
	return b.String()
}

// fnPtrOf is the index of a function whose address is taken, given the
// first time it is asked.
func (s *sgen) fnPtrOf(name string) int {
	if i, ok := s.fnIdx[name]; ok {
		return i
	}
	i := len(s.fnOrder)
	s.fnIdx[name] = i
	s.fnOrder = append(s.fnOrder, name)
	if (i+2)*16 > scmBase {
		panic(unsupported{"more functions used as values than the null page holds"})
	}
	return i
}

// table is the functions whose address is taken: a function pointer is an
// index here.  A call through one passes a truth value as a number, and
// takes one back: a function whose C type has a bool is wrapped.
func (s *sgen) table() string {
	var b strings.Builder
	b.WriteString(";; The functions whose address is taken: a function pointer is an index here.\n")
	b.WriteString("(define fn-table\n  (vector")
	for _, name := range s.fnOrder {
		var ft *cc.FunctionType
		if d := s.defined[name]; d != nil {
			ft, _ = d.Declarator.Type().(*cc.FunctionType)
		} else if hd := s.hostFns[name]; hd != nil {
			ft, _ = hd.Type().(*cc.FunctionType)
		}
		if ft == nil {
			fmt.Fprintf(&b, "\n    (lambda args (error 'call-ptr %q))", "no function "+name)
			continue
		}
		b.WriteString("\n    " + s.tableEntry(name, ft))
	}
	b.WriteString("))\n")
	return b.String()
}

// tableEntry is the procedure a pointer to name calls: name itself, or, a
// function with a bool among its parameters or its result, a procedure
// that converts the numbers a call through a pointer passes.
func (s *sgen) tableEntry(name string, ft *cc.FunctionType) string {
	var ps, as []string
	wrap := scmTypeOf(ft.Result()) == "bool"
	k := 0
	for _, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		a := "a" + strconv.Itoa(k)
		k++
		ps = append(ps, a)
		if scmTypeOf(p.Type()) == "bool" {
			wrap = true
			a = "(not (eqv? " + a + " 0))"
		}
		as = append(as, a)
	}
	if ft.IsVariadic() || s.outs(name) != nil || s.h.tupleRet(name) {
		wrap = true // never so in a table, but said
	}
	if !wrap && s.defined[name] != nil && s.takesEd(name) || !wrap && s.defined[name] == nil {
		return s.names[name]
	}
	call := "(" + strings.Join(append([]string{s.names[name], "ed"}, as...), " ") + ")"
	if s.defined[name] != nil && !s.takesEd(name) {
		call = "(" + strings.Join(append([]string{s.names[name]}, as...), " ") + ")"
	}
	if scmTypeOf(ft.Result()) == "bool" {
		call = "(b->i " + call + ")"
	}
	return "(lambda (" + strings.Join(append([]string{"ed"}, ps...), " ") + ") " + call + ")"
}

// outs are the out-parameters of the function name.
func (s *sgen) outs(name string) []int { return s.h.outs[name] }

// takesEd says the function name takes the editor: every function does,
// unless the profile asks for the pure ones without it.
func (s *sgen) takesEd(name string) bool {
	if !s.g.p.ScmPure {
		return true
	}
	return !s.h.pure(name)
}

// stub is a refused function: a procedure that fails.
func (s *sgen) stub(fd *cc.FunctionDefinition, why string) string {
	name := s.names[fd.Declarator.Name()]
	return fmt.Sprintf(";; REFUSED: %s\n(define (%s . args) (error '%s %q))\n", why, name, name, why)
}

// poke writes the n bytes of v, little-endian, into the image at off (an
// offset from scmBase).
func (s *sgen) poke(off, n int, v uint64) {
	for len(s.image) < off+n {
		s.image = append(s.image, 0)
	}
	for i := 0; i < n; i++ {
		s.image[off+i] = byte(v >> (8 * i))
	}
}

// literal is the address of a string literal's bytes, its NUL after them:
// one place for each distinct literal.
func (s *sgen) literal(str string) int {
	if off, ok := s.lits[str]; ok {
		return s.litBase + off
	}
	off := len(s.pool)
	s.lits[str] = off
	s.pool = append(s.pool, str...)
	if !strings.HasSuffix(str, "\x00") {
		s.pool = append(s.pool, 0)
	}
	return s.litBase + off
}

// scmTypeOf is the Scheme kind of a C type's values: i8 to u64, bool, ptr
// (an address: a pointer, an array, a function), agg (a struct or union,
// whose value is its address), void.
func scmTypeOf(t cc.Type) string {
	if t == nil {
		return "void"
	}
	switch t.Kind() {
	case cc.Void:
		return "void"
	case cc.Ptr, cc.Array, cc.Function:
		return "ptr"
	case cc.Struct, cc.Union:
		return "agg"
	}
	k, ok := scalarKind(t)
	if !ok {
		return "?" + t.String()
	}
	return scmKindType(k)
}

func scmKindType(k jk) string {
	if k.boolean {
		return "bool"
	}
	if k.signed {
		return fmt.Sprintf("i%d", k.size*8)
	}
	return fmt.Sprintf("u%d", k.size*8)
}

// scmBits is an integer kind's width and signedness.
func scmBits(st string) (int, bool, bool) {
	switch st {
	case "i8":
		return 8, true, true
	case "u8":
		return 8, false, true
	case "i16":
		return 16, true, true
	case "u16":
		return 16, false, true
	case "i32":
		return 32, true, true
	case "u32":
		return 32, false, true
	case "i64":
		return 64, true, true
	case "u64":
		return 64, false, true
	}
	return 0, false, false
}

// scmWide says st is a 64-bit integer: generic arithmetic, a bignum past
// the fixnums.
func scmWide(st string) bool { return st == "i64" || st == "u64" }

// scmTrunc is v's bits as a value of kind st.
func scmTrunc(v int64, st string) int64 {
	switch st {
	case "bool":
		if v != 0 {
			return 1
		}
		return 0
	case "i8":
		return int64(int8(v))
	case "u8":
		return int64(uint8(v))
	case "i16":
		return int64(int16(v))
	case "u16":
		return int64(uint16(v))
	case "i32":
		return int64(int32(v))
	case "u32":
		return int64(uint32(v))
	}
	return v
}

// scmLit is v as a literal of kind st.
func scmLit(v int64, st string) string {
	switch {
	case st == "bool":
		if v != 0 {
			return "#t"
		}
		return "#f"
	case st == "u64":
		return strconv.FormatUint(uint64(v), 10)
	}
	return strconv.FormatInt(v, 10)
}

// --- the initial values -----------------------------------------------------

// initializers write the objects' initial values into the image, and
// return the objects whose initializer is not a constant.
func (s *sgen) initializers() []string {
	var failed []string
	one := func(key, name string, t cc.Type, in *cc.Initializer) {
		if in == nil {
			return
		}
		off, ok := s.h.segOff[key]
		if !ok {
			failed = append(failed, name)
			return
		}
		if why := s.catchInit(func() { s.initInto(off, t, in) }); why != "" {
			failed = append(failed, name+": "+why)
		}
	}
	for tu := s.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() == nil || d.Type().Kind() == cc.Function {
				continue
			}
			one(s.g.a.declKey(d), d.Name(), d.Type(), l.InitDeclarator.Initializer)
		}
	}
	for _, st := range s.g.a.statics {
		one(s.g.a.declKey(st.d), st.fn+"."+st.d.Name(), st.d.Type(), st.init)
	}
	return failed
}

func (s *sgen) catchInit(fn func()) (why string) {
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(unsupported)
			if !ok {
				panic(r)
			}
			why = u.why
		}
	}()
	fn()
	return ""
}

// initInto writes initializer in, of an object of type t at off in the
// segment, into the image: the C front end gives each initializer its
// offset in the object.
func (s *sgen) initInto(off int, t cc.Type, in *cc.Initializer) {
	if in == nil {
		return
	}
	if in.Case != cc.InitializerExpr {
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			s.initInto(off, t, l.Initializer)
		}
		return
	}
	e := in.AssignmentExpression
	at := off + int(in.Offset())
	it := in.Type()
	if it == nil {
		it = t
	}
	if sv, ok := unparenE(e).Value().(cc.StringValue); ok && it.Kind() == cc.Array {
		// a char array from a string: its bytes, as many as fit
		n := min(int(it.Size()), len(sv))
		for i := 0; i < n; i++ {
			s.poke(at+i, 1, uint64(sv[i]))
		}
		return
	}
	if fl := in.Field(); fl != nil && fl.IsBitfield() {
		panic(unsupported{"a bit field's initializer"})
	}
	if isAggr(it) {
		panic(unsupported{"a struct initialized from an expression"})
	}
	v, lit := s.constOf(e)
	if lit != nil {
		s.poke(at, 8, 0)
		s.fixups[at] = *lit
		return
	}
	st := scmTypeOf(it)
	if st == "ptr" {
		s.poke(at, 8, uint64(v))
		return
	}
	s.poke(at, int(it.Size()), uint64(scmTrunc(v, st)))
}

// constOf is the value of a file-scope initializer's expression: an
// integer, an address of an object, a function's pointer, or a string
// literal (lit, whose address the image gets when the literals are laid
// out).
func (s *sgen) constOf(e cc.ExpressionNode) (int64, *string) {
	if isNullConst(e) {
		return 0, nil
	}
	if sv, ok := unparenE(e).Value().(cc.StringValue); ok {
		str := string(sv)
		return 0, &str
	}
	if v, ok := intValue(e.Value()); ok && !hasEffect(e) {
		return v, nil
	}
	switch x := unparenE(e).(type) {
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return s.constOf(x.CastExpression)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionAddrof {
			if d := fnDesignator(x.CastExpression); d != nil {
				return int64((s.fnPtrOf(d.Name()) + 1) * 16), nil
			}
			return int64(s.constAddr(x.CastExpression)), nil
		}
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			if d := fnDesignator(x); d != nil {
				return int64((s.fnPtrOf(d.Name()) + 1) * 16), nil
			}
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && (d.Type().Kind() == cc.Array || isAggr(d.Type())) {
				return int64(s.constAddr(x)), nil
			}
		}
	case *cc.AdditiveExpression:
		if isPtrish(x.AdditiveExpression.Type()) {
			base, lit := s.constOf(x.AdditiveExpression)
			n, _ := intValue(x.MultiplicativeExpression.Value())
			if lit != nil {
				panic(unsupported{"a string literal's address plus a number"})
			}
			d := n * elemSize(x.AdditiveExpression.Type())
			if x.Case == cc.AdditiveExpressionSub {
				d = -d
			}
			return base + d, nil
		}
	case *cc.PostfixExpression:
		if x.Case == cc.PostfixExpressionComplit {
			t := x.TypeName.Type()
			off := s.h.anon(t)
			in := &cc.Initializer{Case: cc.InitializerInitList, InitializerList: x.InitializerList, Token: x.Token}
			s.initInto(off, t, in)
			return int64(scmBase + off), nil
		}
	}
	panic(unsupported{fmt.Sprintf("an initializer that is not a constant: %T", e)})
}

// constAddr is the address of an lvalue in a file-scope initializer: an
// object, a member of one, an element of one.
func (s *sgen) constAddr(e cc.ExpressionNode) int {
	switch x := unparenE(e).(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
				if off, ok := s.h.segOff[s.g.a.declKey(d)]; ok {
					return scmBase + off
				}
			}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect:
			return s.constAddr(x.PostfixExpression) + int(x.Field().Offset())
		case cc.PostfixExpressionPSelect:
			v, lit := s.constOf(x.PostfixExpression)
			if lit != nil {
				break
			}
			return int(v) + int(x.Field().Offset())
		case cc.PostfixExpressionIndex:
			be, ie := x.PostfixExpression, x.ExpressionList
			if !isPtrish(be.Type()) {
				be, ie = ie, be
			}
			n, ok := intValue(ie.Value())
			if !ok {
				break
			}
			var base int
			if be.Type().Kind() == cc.Array {
				base = s.constAddr(be)
			} else {
				v, lit := s.constOf(be)
				if lit != nil {
					break
				}
				base = int(v)
			}
			return base + int(n*elemSize(be.Type()))
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			v, lit := s.constOf(x.CastExpression)
			if lit == nil {
				return int(v)
			}
		}
	}
	panic(unsupported{fmt.Sprintf("an address that is not a constant: %T", e)})
}

// --- the names ---------------------------------------------------------------

// useObject is the file-scope object of key, by name, marked used.
func (s *sgen) useObject(key string) (*sobj, bool) {
	if o, ok := s.objects[key]; ok {
		o.used = true
		return o, true
	}
	off, ok := s.h.segOff[key]
	if !ok {
		return nil, false
	}
	var n string
	switch {
	case strings.HasPrefix(key, "global:"):
		n = scmName(strings.TrimPrefix(key, "global:"))
	case strings.HasPrefix(key, "static:"):
		fn, v, _ := strings.Cut(strings.TrimPrefix(key, "static:"), ".")
		n = scmName(fn) + ":" + v
	default:
		n = "object:" + strings.ReplaceAll(key, ":", ".")
	}
	for s.objNames[n] {
		n += "*"
	}
	s.objNames[n] = true
	t := s.h.segType[key]
	kind := "agg"
	if t != nil && !isAggr(t) && t.Kind() != cc.Array && t.Kind() != cc.Function {
		kind = scmAccess(scmTypeOf(t))
	}
	o := &sobj{name: n, kind: kind, addr: scmBase + off, used: true}
	s.objects[key] = o
	return o, true
}

// structName is the name a struct's or a union's members are named after:
// its typedef's, else its tag's; "" for a type with neither, or a name
// another type has.
func (s *sgen) structName(t cc.Type) string {
	fs := scmFields(t)
	if len(fs) == 0 {
		return ""
	}
	key := fmt.Sprintf("%p", fs[0]) // a typedef's clone of the type shares its members
	if n, ok := s.structNames[key]; ok {
		return n
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
	n := s.h.tagTypedef[tag]
	if n == "" && t.Typedef() != nil {
		n = t.Typedef().Name()
	}
	if n == "" {
		n = tag
	}
	if have, ok := s.structTaken[n]; n == "" || ok && have != key {
		n = ""
	} else {
		s.structTaken[n] = key
	}
	s.structNames[key] = n
	return n
}

// useMember is the accessor name, defined to read kind st at off, or ""
// when the name reads something else already.
func (s *sgen) useMember(name, st string, off int) string {
	kind := "agg"
	if st != "agg" {
		kind = scmAccess(st)
	}
	m, ok := s.members[name]
	switch {
	case !ok:
		s.members[name] = &smember{kind: kind, off: off}
	case m.off != off:
		return ""
	case kind == "agg":
	case m.kind == "agg":
		m.kind = kind
	case m.kind != kind:
		return ""
	}
	return name
}

// nameDefs are the definitions of the names the functions use: the
// file-scope objects and the members' accessors.
func (s *sgen) nameDefs() string {
	var b strings.Builder
	var objs []*sobj
	for _, o := range s.objects {
		if o.used {
			objs = append(objs, o)
		}
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].addr < objs[j].addr })
	b.WriteString("\n;; The file-scope objects the functions name: a scalar read by its name and\n;; written by set!, an array or a struct its address; &name the address.\n")
	for _, o := range objs {
		fmt.Fprintf(&b, "(define-c-object %s &%s %s %d)\n", o.name, o.name, o.kind, o.addr)
	}
	var ms []string
	for n := range s.members {
		ms = append(ms, n)
	}
	sort.Strings(ms)
	b.WriteString("\n;; The members the functions name, at their offsets from their struct's\n;; address: (name p) reads one, (name-set! p v) writes it, (name& p) is its\n;; address.\n")
	for _, n := range ms {
		m := s.members[n]
		fmt.Fprintf(&b, "(define-c-member %s %s %d)\n", n, m.kind, m.off)
	}
	return b.String()
}
