package togo

// hs.go is the Haskell backend's frame (doc/HASKELL.md): the module a
// translation unit becomes -- caprice's, the fourth translation.  Where the
// Go, the Java and the Clojure model C's memory in managed objects, the
// Haskell keeps it: every object in raw memory, laid out as the C lays it
// out on amd64 (the C front end's sizes and offsets), and a pointer an
// address (caprice/rt/Caprice/Rt.hs).  So nothing of the pointer analysis
// is needed, and the functions are the lowered form (lower.go) printed as
// Haskell (hs_fn.go, hs_expr.go):
//
//   - the file-scope objects, and the block-scope statics, are one segment
//     per editor, each at its offset; their initial values are written into
//     it when the editor is made (initGlobals);
//   - a function is a top-level function of the editor and the C's
//     parameters, in IO; its blocks are local functions of the variables
//     live at their start -- join points, a jump a tail call -- and a local
//     that is an array, a struct or a union, or whose address is taken,
//     lives in the call's frame;
//   - a function pointer is an index into the table of the functions whose
//     address is taken, at an address no object has;
//   - a function declared and not defined is the host's, called in the host
//     module with the editor first.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// hgen is the Haskell backend's state for one translation unit.
type hgen struct {
	g        *gen
	module   string
	host     string
	defined  map[string]*cc.FunctionDefinition
	hostFns  map[string]*cc.Declarator
	segOff   map[string]int // an object's key -> its offset in the segment
	segSize  int
	fnIdx    map[string]int // a function whose address is taken -> its index
	fnOrder  []string
	names    map[string]string // a C function -> its Haskell name
	variadic map[string]bool   // the host's variadic functions
	taken    map[string]bool   // the names no local may take (topNames)
	fx       map[string]*hsFx  // each function's effects (hseffects.go)
	tyDef    map[string]string // a Haskell type name -> its declaration (hstypes.go)
	tyOf     map[string]string // a C name and a declaration -> the type's name
	tySyn    map[string]string // a synonym -> what it stands for
	outs     map[string][]int  // a function's out-parameters (hsout.go)
	outArg   map[*cc.UnaryExpression]bool
	outLazy  map[*cc.Declarator]bool
	sval     map[*cc.Declarator]bool // struct locals that are values (hsstruct.go)
	bootSigs map[string]string       // an exported function -> its hs-boot signature

	// the names (hsnamed.go)
	segType     map[string]cc.Type // an object's key -> its type
	objs        map[string]*hobj
	exported    map[string]bool // an object whose addr' the exports print
	tagTypedef  map[string]string
	structNames map[string]string
	structTaken map[string]bool
	memberOff   map[string]int64
	enumVal     map[string]int64
}

// hsReserved are the names the generated code uses, and Haskell's keywords:
// a C name among them takes a trailing prime.
var hsReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`case class data default deriving do else foreign if import in infix infixl
	infixr instance let module newtype of then type where forall mdo rec proc family pattern role qualified as hiding
	pure fromIntegral not quot rem negate fmap return id max min otherwise True False
	xor complement shiftL shiftR shift rotate zeroBits bit setBit clearBit complementBit testBit bitSizeMaybe bitSize
	isSigned unsafeShiftL unsafeShiftR rotateL rotateR popCount finiteBitSize countLeadingZeros countTrailingZeros
	toIntegralSized oneBits
	rdI8 rdW8 rdI16 rdW16 rdI32 rdW32 rdI64 rdW64 rdP rdB wrI8 wrW8 wrI16 wrW16 wrI32 wrW32 wrI64 wrW64 wrP wrB
	copyMem fillMem copyMemNo b2i p2i i2p pAdd pSub frame fnPtr fnIndex toRaw fromRaw nullPtr edSeg newEd
	cBytes castPtr plusPtr minusPtr main newEditor initGlobals fnTable callPtr segSize listArray ch`) {
		hsReserved[w] = true
	}
}

// hsName is a C name as a Haskell variable or function: a name Haskell
// could not start with or reserves is changed, the rest kept.
func hsName(s string) string {
	if s == "" {
		return "anon'"
	}
	if c := s[0]; c >= 'A' && c <= 'Z' || c == '_' {
		s = "c'" + s
	}
	if hsReserved[s] {
		s += "'"
	}
	return s
}

// writeHs writes the translation unit as the Haskell module path, and beside
// it path.refused: the functions it could not write, with the reason.
func (g *gen) writeHs(path string) error {
	h := &hgen{g: g, module: g.p.HsModule, host: g.p.HsHost, defined: map[string]*cc.FunctionDefinition{},
		hostFns: map[string]*cc.Declarator{}, segOff: map[string]int{}, fnIdx: map[string]int{},
		names: map[string]string{}, variadic: map[string]bool{}, segType: map[string]cc.Type{},
		exported: map[string]bool{}, memberOff: map[string]int64{}, enumVal: map[string]int64{},
		tyDef: map[string]string{}, tyOf: map[string]string{}, tySyn: map[string]string{}, bootSigs: map[string]string{}}
	if h.module == "" {
		h.module = "Editor"
	}
	if h.host == "" {
		h.host = "Host"
	}
	var fds []*cc.FunctionDefinition
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			fd := ed.FunctionDefinition
			h.defined[fd.Declarator.Name()] = fd
			fds = append(fds, fd)
		}
	}
	for name, d := range g.a.fnDecls {
		if h.defined[name] == nil && !strings.HasPrefix(name, "__") {
			h.hostFns[name] = d
			if ft, ok := d.Type().(*cc.FunctionType); ok && ft.IsVariadic() {
				h.variadic[name] = true
			}
		}
	}
	for name := range h.defined {
		h.names[name] = hsName(name)
	}
	h.layout()
	h.nameObjects()
	h.tagTypedefs()
	h.outParams()
	h.structLocals()
	h.effects()
	if hsCountHook != nil {
		hsCountHook(h)
	}

	var report strings.Builder
	var funcs []string
	written := 0
	for _, fd := range fds {
		src, why := h.function(fd)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", fd.Declarator.Name(), why)
			funcs = append(funcs, h.stub(fd, why))
			continue
		}
		written++
		funcs = append(funcs, src)
	}
	inits, failed := h.initializers()
	for _, f := range failed {
		fmt.Fprintf(&report, "initial value of %s\n", f)
	}

	table := h.table()
	boot, accessors, err := h.exports()
	if err != nil {
		return err
	}
	var init strings.Builder
	fmt.Fprintf(&init, "-- | The bytes of the segment the file-scope objects are in.\nsegSize :: Int\nsegSize = %d\n\n", h.segSize)
	init.WriteString("-- | A new editor on host h: its segment, with the objects' initial values, and its table.\nnewEditor :: Dynamic -> IO Ed\nnewEditor h = do\n  ed' <- newEd segSize h fnTable\n  initGlobals ed'\n  pure ed'\n\n")
	init.WriteString("initGlobals :: Ed -> IO ()\ninitGlobals ed' = do\n")
	for _, l := range inits {
		init.WriteString("  " + l + "\n")
	}
	init.WriteString("  pure ()\n\n")
	init.WriteString(table)

	files := map[string]string{}
	stamp := fmt.Sprintf("-- Code generated by `go tool whim skel -hs` from a C translation unit; DO NOT EDIT.\n-- %d of %d functions written; the rest are stubs that fail, with the reason.\n", written, len(fds))
	if h.g.p.HsParts <= 1 {
		var fb strings.Builder
		for _, f := range funcs {
			fb.WriteString(f + "\n")
		}
		body := init.String() + accessors + h.definitions(strings.Join(inits, "\n")+table+fb.String(), true) + fb.String()
		files[path] = h.header(h.module, "", body) + body
	} else {
		h.split(path, fds, funcs, init.String(), inits, table, files)
	}
	pure, noEd := 0, 0
	for n := range h.defined {
		if h.pure(n) {
			pure++
		}
		if !h.takesEd(n) {
			noEd++
		}
	}
	outs := 0
	for _, is := range h.outs {
		outs += len(is)
	}
	fmt.Fprintf(logw, "hs: %d of %d functions written, %d refused; %d pure, %d without the editor, %d out-parameters in %d functions, %d struct locals as values\n", written, len(fds), len(fds)-written, pure, noEd, outs, len(h.outs), len(h.sval))
	if err := os.WriteFile(path+".host", []byte(h.hostSigs()), 0o644); err != nil {
		return err
	}
	if boot != "" {
		if err := os.WriteFile(strings.TrimSuffix(path, ".hs")+".hs-boot", []byte(boot), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path+".refused", []byte(report.String()), 0o644); err != nil {
		return err
	}
	for p, src := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(stamp+src), 0o644); err != nil {
			return err
		}
	}
	return nil
}

var hsCountHook func(*hgen) // a test's look at the analyses

// layout gives each file-scope object and each block-scope static its
// offset in the segment, aligned as C aligns it.
func (h *hgen) layout() {
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
	for tu := h.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() == nil || d.Type().Kind() == cc.Function {
				continue
			}
			add(h.g.a.declKey(d), d.Type())
		}
	}
	for _, s := range h.g.a.statics {
		add(h.g.a.declKey(s.d), s.d.Type())
	}
	off := 0
	for _, o := range objs {
		al := max(o.t.Align(), 1)
		off = (off + al - 1) / al * al
		h.segOff[o.key] = off
		h.segType[o.key] = o.t
		off += int(max(o.t.Size(), 0))
	}
	h.segSize = off
}

// anon is room in the segment for an object of type t that has no name: a
// compound literal in a file-scope object's initializer.
func (h *hgen) anon(t cc.Type) int {
	al := max(t.Align(), 1)
	off := (h.segSize + al - 1) / al * al
	h.segSize = off + int(max(t.Size(), 1))
	return off
}

// exports is the hs-boot interface of what the host calls back
// (Profile.HsExports), and the accessors of the objects among them.
func (h *hgen) exports() (boot, accessors string, err error) {
	if len(h.g.p.HsExports) == 0 {
		return "", "", nil
	}
	var bb, ab strings.Builder
	fmt.Fprintf(&bb, "module %s where\n\nimport Caprice.Rt\n\n", h.module)
	for _, name := range h.g.p.HsExports {
		if fd := h.defined[name]; fd != nil {
			// in the types' own names: the synonyms are the module's
			sig, _ := h.signature(fd.Declarator, nil)
			sig = h.canon(sig)
			h.bootSigs[name] = sig
			bb.WriteString(sig + "\n")
			continue
		}
		off, ok := h.segOff["global:"+name]
		if !ok {
			return "", "", fmt.Errorf("HsExports: %s is neither a function nor a file-scope object of the unit", name)
		}
		h.exported[name] = true
		h.objs["global:"+name].exported = true
		fmt.Fprintf(&ab, "-- | The address of %s, for the host.\naddr'%s :: Ed -> Ptr a\naddr'%s ed' = pAdd (edSeg ed') %d\n\n", name, name, name, off)
		fmt.Fprintf(&bb, "addr'%s :: Ed -> Ptr a\n", name)
	}
	for _, t := range hsTokens(bb.String()) {
		if d, ok := h.tyDef[t]; ok && strings.HasPrefix(d, "data ") {
			return "", "", fmt.Errorf("HsExports: a signature names %s, a struct: the hs-boot cannot declare it", t)
		}
	}
	return bb.String(), ab.String(), nil
}

// hostSigs is the type of each function the module calls in the host:
// what the host module must define.
func (h *hgen) hostSigs() string {
	var names []string
	for n := range h.hostFns {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		d := h.hostFns[n]
		ft, ok := d.Type().(*cc.FunctionType)
		if !ok {
			continue
		}
		ts := []string{"Ed"}
		for _, p := range ft.Parameters() {
			if p.Type() == nil || p.Type().Kind() == cc.Void {
				continue
			}
			t := p.Type()
			if t.Kind() == cc.Array {
				t = t.Decay()
			}
			ts = append(ts, hostType(t))
		}
		if ft.IsVariadic() {
			ts = append(ts, "[VArg]")
		}
		fmt.Fprintf(&b, "%s :: %s -> IO %s\n", hsName(n), strings.Join(ts, " -> "), hostType(ft.Result()))
	}
	return b.String()
}

// hostType is a type as the host's functions take it: every pointer P, the
// host's own (caprice/host).
func hostType(t cc.Type) string {
	if t != nil && (isAggr(t) || t.Kind() == cc.Ptr || t.Kind() == cc.Array || t.Kind() == cc.Function) {
		return "P"
	}
	return hsScalarType(t)
}

// fnPtrOf is the index of a function whose address is taken, given the
// first time it is asked.
func (h *hgen) fnPtrOf(name string) int {
	if i, ok := h.fnIdx[name]; ok {
		return i
	}
	i := len(h.fnOrder)
	h.fnIdx[name] = i
	h.fnOrder = append(h.fnOrder, name)
	return i
}

// table is the functions whose address is taken, each as a function of the
// editor and its arguments' bits, and the call through a pointer.
func (h *hgen) table() string {
	var b strings.Builder
	b.WriteString("-- | The functions whose address is taken: a function pointer is an index here.\n")
	b.WriteString("fnTable :: Array Int (Ed -> [Word64] -> IO Word64)\n")
	fmt.Fprintf(&b, "fnTable = listArray (0, %d)\n  [", max(len(h.fnOrder)-1, 0))
	if len(h.fnOrder) == 0 {
		b.WriteString(" \\_ _ -> error \"no function\"")
	}
	for i, name := range h.fnOrder {
		if i > 0 {
			b.WriteString("\n  ,")
		}
		d := h.defined[name]
		var ft *cc.FunctionType
		call := ""
		if d != nil {
			ft, _ = d.Declarator.Type().(*cc.FunctionType)
			call = h.names[name]
		} else if hd := h.hostFns[name]; hd != nil {
			ft, _ = hd.Type().(*cc.FunctionType)
			call = h.host + "." + hsName(name)
		}
		if ft == nil {
			fmt.Fprintf(&b, " \\_ _ -> error %q", "no function "+name)
			continue
		}
		var as []string
		n := 0
		for _, p := range ft.Parameters() {
			if p.Type() == nil || p.Type().Kind() == cc.Void {
				continue
			}
			as = append(as, fmt.Sprintf("(fromRaw (a' !! %d))", n))
			n++
		}
		args := "a'"
		if n == 0 {
			args = "_"
		}
		ed := "ed'"
		if d != nil && !h.takesEd(name) {
			ed = ""
		}
		callStr := strings.Join(slices.DeleteFunc([]string{call, ed, strings.Join(as, " ")}, func(s string) bool { return s == "" }), " ")
		edArg := "ed'"
		if ed == "" {
			edArg = "_"
		}
		if d != nil && h.pure(name) {
			fmt.Fprintf(&b, " \\%s %s -> pure (toRaw (%s))", edArg, args, callStr)
		} else {
			fmt.Fprintf(&b, " \\%s %s -> toRaw <$> %s", edArg, args, callStr)
		}
	}
	b.WriteString("\n  ]\n\n")
	return b.String()
}

// stub is a refused function: its type, and a body that fails.
func (h *hgen) stub(fd *cc.FunctionDefinition, why string) string {
	name := h.names[fd.Declarator.Name()]
	sig, params := h.signature(fd.Declarator, nil)
	return fmt.Sprintf("-- REFUSED: %s\n%s\n%s %s= error %q\n", why, sig, name, strings.Join(append(params, ""), " "), fd.Declarator.Name()+": "+why)
}

// signature is a function's type line, and its parameters' names: the
// editor, each C parameter, and the result in IO -- a struct result
// written through a pointer the caller gives, first.
func (h *hgen) signature(d *cc.Declarator, lf *lfn) (string, []string) {
	ft, _ := d.Type().(*cc.FunctionType)
	name := h.names[d.Name()]
	var ts, ps []string
	if h.takesEd(d.Name()) {
		ts = append(ts, "Ed")
		ps = append(ps, "ed'")
	}
	if isAggr(ft.Result()) && !h.tupleRet(d.Name()) {
		ts = append(ts, "Ptr "+h.pointee(ft.Result()))
		ps = append(ps, "sret'")
	}
	i := 0
	for _, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		t := p.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		if h.isOut(d.Name(), i) {
			t = elemOf(t) // a value in, and out with the result
		}
		ts = append(ts, h.sigType(t))
		if lf != nil && i < len(lf.params) {
			ps = append(ps, lf.params[i].name)
		} else {
			ps = append(ps, fmt.Sprintf("p%d'", i))
		}
		i++
	}
	rt := h.sigType(ft.Result())
	if h.tupleRet(d.Name()) {
		rt = h.tupleType(ft.Result())
	}
	if outs := h.outs[d.Name()]; len(outs) > 0 {
		var parts []string
		if rt != "()" {
			parts = append(parts, rt)
		}
		for _, k := range outs {
			parts = append(parts, h.sigType(elemOf(ft.Parameters()[k].Type())))
		}
		rt = strings.Join(parts, ", ")
		if len(parts) > 1 {
			rt = "(" + rt + ")"
		}
	}
	if strings.Contains(rt, " ") && rt[0] != '(' {
		rt = "(" + rt + ")"
	}
	switch {
	case isAggr(ft.Result()) && !h.tupleRet(d.Name()):
		rt = "IO ()"
	case !h.pure(d.Name()):
		rt = "IO " + rt
	}
	return fmt.Sprintf("%s :: %s", name, strings.Join(append(ts, rt), " -> ")), ps
}

// hsParamType is a parameter's Haskell type: a struct passed by value is
// its address, which the callee copies.
func (h *hgen) hsParamType(t cc.Type) string {
	if isAggr(t) {
		return "Ptr " + h.pointee(t)
	}
	return h.hsType(t)
}

// hsType is the Haskell type of a C type's values: a pointer's what it
// points at (hstypes.go); "agg" for a struct or union (whose value is its
// address), "()" for void.
func (h *hgen) hsType(t cc.Type) string {
	if t != nil && (t.Kind() == cc.Ptr || t.Kind() == cc.Array || t.Kind() == cc.Function) {
		return h.ptrType(t)
	}
	return hsScalarType(t)
}

// hsScalarType is the Haskell type of a C scalar type's values.
func hsScalarType(t cc.Type) string {
	if t == nil {
		return "()"
	}
	switch t.Kind() {
	case cc.Void:
		return "()"
	case cc.Ptr, cc.Array, cc.Function:
		return "P"
	case cc.Struct, cc.Union:
		return "agg"
	}
	k, ok := scalarKind(t)
	if !ok {
		return "?" + t.String()
	}
	return hsKindType(k)
}

func hsKindType(k jk) string {
	if k.boolean {
		return "Bool"
	}
	if k.signed {
		return fmt.Sprintf("Int%d", k.size*8)
	}
	return fmt.Sprintf("Word%d", k.size*8)
}

// hsAccess is the suffix of the runtime's reader and writer of a type:
// rdI32, wrP, rdB.
func hsAccess(ht string) string {
	switch {
	case ht == "Bool":
		return "B"
	case isPtrHt(ht):
		return "P"
	}
	return string(ht[0]) + strings.TrimPrefix(strings.TrimPrefix(ht, "Int"), "Word")
}

// initializers are the steps that write the objects' initial values into
// the segment, and the objects whose initializer it cannot write.
func (h *hgen) initializers() ([]string, []string) {
	var lines, failed []string
	f := &hfn{h: h, name: "initGlobals", image: []byte{}}
	one := func(key, name string, t cc.Type, in *cc.Initializer) {
		if in == nil {
			return
		}
		off, ok := h.segOff[key]
		if !ok {
			failed = append(failed, name)
			return
		}
		ls, err := f.catchInit(func() { f.initInto("(edSeg ed')", off, t, in) })
		if err != "" {
			failed = append(failed, name+": "+err)
			return
		}
		lines = append(lines, ls...)
	}
	for tu := h.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() == nil || d.Type().Kind() == cc.Function {
				continue
			}
			one(h.g.a.declKey(d), d.Name(), d.Type(), l.InitDeclarator.Initializer)
		}
	}
	for _, s := range h.g.a.statics {
		one(h.g.a.declKey(s.d), s.fn+"."+s.d.Name(), s.d.Type(), s.init)
	}
	// the constant bytes first, as one image; then the addresses
	return append(imageLines(f.image), lines...), failed
}

// catchInit runs fn, which writes lines, and returns them, or why it
// refused.
func (f *hfn) catchInit(fn func()) (lines []string, why string) {
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(unsupported)
			if !ok {
				panic(r)
			}
			lines, why = nil, u.why
		}
	}()
	f.lines = nil
	fn()
	return f.lines, ""
}
