package togo

// ml.go is the OCaml backend's frame (doc/OCAML.md): the module a
// translation unit becomes -- whiml's, the seventh translation, for OCaml
// 5.  It keeps C's memory as the Scheme backend does, and reads the C as
// the Scheme does: it runs the Scheme printer (scm*.go) on every function,
// in the mode that differs where OCaml's integers do (newSgen), reads each
// function back as forms (scm_tidy.go's reader) and prints the forms as
// OCaml (ml_expr.go) -- so every decision about the C, the memory's
// layout, the frames, the out-parameters and struct values, the order of
// evaluation, the joins and loops, is the Scheme's, made once, and what
// is OCaml's is the printing and the types:
//
//   - every C object of an editor lives in one Bytes, a pointer an int
//     offset into it (whiml/rt.ml), but the file-scope scalars whose
//     address nothing takes: those are the fields of a record the editor
//     carries (ed.st.got_int <- 0); the others at constant addresses, read
//     by their names (p_sm ed) and written by set_, their addresses
//     _addr; a struct's members, a module per struct
//     (Win_T.w_cursor_lnum ed wp);
//   - C's integers are OCaml's int, 63 bits: the 8-, 16- and 32-bit types
//     wrapped where C wraps them; long and unsigned long the int itself,
//     an unsigned long its bits as a signed long, ordered and divided by
//     the runtime as unsigned -- exact for every value under 2^62 in
//     magnitude, and for the low 63 bits of a sum, product or shift past
//     it (doc/OCAML.md, *Integers*); a C bool OCaml's bool;
//   - a function is a top-level function of the editor and the C's
//     parameters, the functions in order of their calls, each cycle one
//     let rec; its blocks local functions (let rec ... in), every jump a
//     tail call;
//   - a function pointer is an index into a table, one table for each
//     arity and result -- so every call through one is typed;
//   - the host's functions are a record of functions the editor carries,
//     its type the module's (glue), which the host fills in;
//   - what an OCaml programmer would write differently is written so where
//     it says the same (doc/OCAML-IDIOMS.md): no warning under -w +a, an
//     interface, comparisons turned round, ladders as matches, bytes
//     compared and matched as chars.

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// mlgen is the OCaml backend's state for one translation unit.
type mlgen struct {
	s *sgen

	glob    map[string]string // a Scheme name at file scope -> its OCaml name
	taken   map[string]bool   // the OCaml names at file scope
	renamed map[string]bool   // OCaml names at file scope that are not their C name: a local may not take them
	objs    map[string]*sobj  // a Scheme object name -> the object
	objOf   map[string]*mlObj // its OCaml names
	fns     map[string]string // a Scheme function name -> its C name
	members map[string]*mlMember
	modules map[string]string // a struct's name -> its module
	modTake map[string]bool
	enumVal map[string]string  // a Scheme enumerator -> its value, as OCaml writes it
	variant map[string]*mlEnum // a Scheme enumerator of a variant -> the variant
	ctorOf  map[string]string  // a Scheme enumerator of a variant -> its constructor
	venums  []*mlEnum          // the variants, in the order they are declared
	recs    []*mlRecord        // the records (ml_record.go), made by recordList
	unitFn  map[string]bool    // a Scheme function name -> its value is ()
	tables  map[string]bool    // the call-ptr tables used
	used    map[string]bool    // the names at file scope the functions use
	st      mlStats
}

// use marks the name at file scope n used: the header defines only what is
// (an unused value is a warning).
func (m *mlgen) use(n string) { m.used[n] = true }

// mlObj is a file-scope object's names: its reader (or, an array or a
// struct, its address), its writer, its address -- or, a field of the
// editor's state, the field's.
type mlObj struct {
	read, set, addr string
	agg, field      bool
	kind            string
	variant         *mlEnum // the variant the object's values are, nil for none
	addrAt          int
	written         bool
}

// mlAddressed adds to taken the names x takes the address of, &name: of an
// object, or of a local of the same name, which keeps the object in
// memory too; and to read the names it reads, but as set!'s target.
func mlAddressed(x *sform, taken, read map[string]bool) {
	if !x.isList() {
		if strings.HasPrefix(x.atom, "&") {
			taken[x.atom[1:]] = true
		} else {
			read[x.atom] = true
		}
		return
	}
	for i, k := range x.kids {
		if i == 1 && x.head() == "set!" {
			continue
		}
		mlAddressed(k, taken, read)
	}
}

// mlMember is a member's accessors, in its struct's module.
type mlMember struct {
	module, read, set, addr string
	kind                    string
	off                     int
	variant                 *mlEnum // the variant its values are, nil for none: read and written through the conversions
	record                  bool    // a record's field (ml_record.go), named read: p.read
}

// mlStats counts what the printer wrote.
type mlStats struct {
	functions, recGroups, biggestRec, ignores, clamped, ladders, charMatches, charCmps, strCmps int
}

// mlKeywords are OCaml's reserved words, and the names the generated code
// uses that a C name may not take.
var mlReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`and as assert asr begin class constraint do done downto else end
exception external false for fun function functor if in include inherit initializer land lazy let
lor lsl lsr lxor match method mod module mutable new nonrec object of open or private rec sig struct
then to true try type val virtual when while with effect parser
ed fr sret mem not ignore failwith raise fst snd min max compare ref succ pred abs
glue new_editor data_end host_names chunks fn_ptr fn_index frame_push frame_pop
ld_s8 ld_u8 ld_char ld_s16 ld_u16 ld_s32 ld_u32 ld_s64 ld_u64 ld_ptr ld_bool
st_s8 st_u8 st_s16 st_u16 st_s32 st_u32 st_s64 st_u64 st_ptr st_bool
mem_copy mem_zero mem_fill mem_image to_i8 to_u8 to_i16 to_u16 to_i32 to_u32
i32_shl u32_add u32_sub u32_mul u32_shl u32_not u64_div u64_rem u64_shr u64_lt u64_le u64_gt u64_ge c_str_is has_prefix c_str_is_ci has_prefix_ci`) {
		mlReserved[w] = true
	}
}

// mlMangle is a Scheme identifier spelled as OCaml can: what OCaml cannot
// have in a name respelled, a name with a capital first lowered.
func mlMangle(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ':':
			b.WriteString("__")
		case c == '.':
			b.WriteByte('_')
		case c == '*':
			b.WriteByte('\'')
		case c == '-':
			b.WriteByte('_')
		case c == '&':
			b.WriteString("_addr")
		case c == '?':
			b.WriteString("_p")
		case c == '!' || c == '>':
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '\'':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "_%02x", c)
		}
	}
	n := b.String()
	if n == "" {
		return "x_"
	}
	if n[0] >= 'A' && n[0] <= 'Z' {
		// a constant's (NUL, MODE_INSERT), an object's (IObuff, Rows):
		// lowered whole
		n = strings.ToLower(n)
	}
	if n[0] >= '0' && n[0] <= '9' || n[0] == '\'' {
		n = "x" + n
	}
	if n == "_" {
		n = "x_"
	}
	for mlReserved[n] {
		n += "_"
	}
	return n
}

// writeMl writes the translation unit as the OCaml module at path, and
// beside it path.refused, the functions it could not write, path.host, the
// fields of the host's record, and path.layout, the layout it wrote the
// memory by.
func (g *gen) writeMl(path string) error {
	s := newSgen(g, true)
	fds, failed := s.prepare()
	m := &mlgen{s: s, glob: map[string]string{}, taken: map[string]bool{}, renamed: map[string]bool{},
		objs: map[string]*sobj{}, objOf: map[string]*mlObj{}, fns: map[string]string{}, members: map[string]*mlMember{},
		modules: map[string]string{}, modTake: map[string]bool{}, enumVal: map[string]string{}, unitFn: map[string]bool{},
		tables: map[string]bool{}, used: map[string]bool{}, variant: map[string]*mlEnum{}, ctorOf: map[string]string{}}
	if err := m.variants(); err != nil {
		return err
	}
	if err := m.records(); err != nil {
		return err
	}
	var report strings.Builder
	for _, f := range failed {
		fmt.Fprintf(&report, "initial value of %s\n", f)
	}
	// the Scheme of every function first: it names the objects, members
	// and constants they use
	type fnText struct {
		name string
		form *sform
	}
	var texts []fnText
	for _, fd := range fds {
		src, why := s.function(fd)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", fd.Declarator.Name(), why)
			continue
		}
		forms, err := scmRead(src)
		if err != nil || len(forms) != 1 {
			fmt.Fprintf(&report, "%s: the Scheme does not read back: %v\n", fd.Declarator.Name(), err)
			continue
		}
		texts = append(texts, fnText{fd.Declarator.Name(), forms[0]})
	}
	// the objects whose address the functions take
	taken, read := map[string]bool{}, map[string]bool{}
	for _, t := range texts {
		mlAddressed(t.form, taken, read)
	}
	m.nameAll(taken, read)
	for c, u := range s.unitFns {
		m.unitFn[s.names[c]] = u
	}
	for _, n := range s.hostList {
		m.unitFn[s.names[n]] = scmTypeOf(m.hostType(n).Result()) == "void"
	}

	// each function's OCaml, and the functions it calls
	type fnOut struct {
		name string
		doc  *mdoc
		refs map[string]bool
	}
	var outs []*fnOut
	byName := map[string]*fnOut{}
	for _, t := range texts {
		d, refs, why := m.function(t.form)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", t.name, why)
			continue
		}
		o := &fnOut{name: t.name, doc: d, refs: refs}
		outs = append(outs, o)
		byName[s.names[t.name]] = o
	}
	m.st.functions = len(outs)

	// the table entries name functions too
	entries := m.tableEntries()

	// the functions in order of their calls: each cycle one let rec
	idx := map[string]int{}
	for i, o := range outs {
		idx[s.names[o.name]] = i
	}
	edges := make([][]int, len(outs))
	for i, o := range outs {
		var es []int
		for r := range o.refs {
			if j, ok := idx[r]; ok {
				es = append(es, j)
			}
		}
		sort.Ints(es)
		edges[i] = es
	}
	sccs := mlSCC(len(outs), edges)

	var b strings.Builder
	fmt.Fprintf(&b, "(* Code generated by `go tool whim skel -ml` from a C translation unit; DO NOT EDIT. *)\n(* %d of %d functions written. *)\n\n", len(outs), len(fds))
	b.WriteString("open Rt\n\n")
	m.fixups()
	mli := m.mli() // before the header: it uses what it exports
	b.WriteString(m.header())
	for _, comp := range sccs {
		self := false
		if len(comp) == 1 {
			for _, e := range edges[comp[0]] {
				if e == comp[0] {
					self = true
				}
			}
		}
		var parts []string
		for k, i := range comp {
			kw := "and"
			if k == 0 {
				kw = "let"
				if len(comp) > 1 || self {
					kw = "let rec"
				}
			}
			parts = append(parts, mrender(mcat(mtext(kw+" "), outs[i].doc)))
		}
		if len(comp) > 1 {
			m.st.recGroups++
			m.st.biggestRec = max(m.st.biggestRec, len(comp))
		}
		b.WriteString(strings.Join(parts, "\n\n") + "\n\n")
	}
	b.WriteString(entries)
	b.WriteString(m.image())
	text := b.String()
	fmt.Fprintf(logw, "ml: %d of %d functions written, %d refused, %d lines; %d cycles of calls as let rec (the largest %d functions), %d values ignored, %d constants past 63 bits saturated; %d function-pointer tables; %d ladders of tests a match, %d matches and %d comparisons on a char, %d string comparisons on an OCaml string\n",
		len(outs), len(fds), len(fds)-len(outs), strings.Count(text, "\n"), m.st.recGroups, m.st.biggestRec, m.st.ignores, m.st.clamped, len(m.tables), m.st.ladders, m.st.charMatches, m.st.charCmps, m.st.strCmps)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path+".layout", []byte(s.layout()), 0o644); err != nil {
		return err
	}
	var host []string
	for _, n := range s.hostList {
		host = append(host, m.glob[s.names[n]])
	}
	if err := os.WriteFile(path+".host", []byte(strings.Join(host, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".ml")+".mli", []byte(mli), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(path+".refused", []byte(report.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// hostType is the C type of the host's function n.
func (m *mlgen) hostType(n string) *cc.FunctionType {
	ft, _ := m.s.hostFns[n].Type().(*cc.FunctionType)
	return ft
}

// take is name at file scope, made unique, and marked renamed when it is
// not c.
func (m *mlgen) take(name, c string) string {
	for m.taken[name] {
		name += "'"
	}
	m.taken[name] = true
	if name != c {
		m.renamed[name] = true
	}
	return name
}

// nameAll names everything at file scope: the functions, the host's, the
// objects, the constants, the members' modules.  A scalar object whose
// address nothing takes -- no function, no initial value -- and that the
// host does not read is a field of the editor's state, not memory
// (doc/OCAML-IDIOMS.md, item 3) -- one the functions read: a field only
// written is a warning, and stays in memory.
func (m *mlgen) nameAll(taken, read map[string]bool) {
	s := m.s
	for _, w := range []string{"glue", "new_editor", "data_end"} {
		m.taken[w] = true
	}
	var cs []string
	for c := range s.defined {
		cs = append(cs, c)
	}
	cs = append(cs, s.hostList...)
	sort.Strings(cs)
	for _, c := range cs {
		sn := s.names[c]
		m.glob[sn] = m.take(mlMangle(sn), c)
		m.fns[sn] = c
	}
	var keys []string
	for k, o := range s.objects {
		if o.used {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return s.objects[keys[i]].addr < s.objects[keys[j]].addr })
	for _, k := range keys {
		o := s.objects[k]
		m.objs[o.name] = o
		base := mlMangle(o.name)
		ob := &mlObj{agg: o.kind == "agg", kind: o.kind, addrAt: o.addr, variant: m.variantOfObject(k)}
		if !ob.agg && !taken[o.name] && read[o.name] && !m.imageTakes(o) && !m.exported(o.name) {
			ob.field = true
			ob.read = base
			for mlRtFields[ob.read] || m.hostField(ob.read) {
				ob.read += "'"
			}
			m.objOf[o.name] = ob
			continue
		}
		if ob.agg {
			ob.read = m.take(base, o.name)
			ob.addr = ob.read
		} else {
			ob.read = m.take(base, o.name)
			ob.set = m.take("set_"+base, "")
			ob.addr = m.take(base+"_addr", "")
		}
		m.objOf[o.name] = ob
	}
	var es []string
	for n := range s.enums {
		es = append(es, n)
	}
	sort.Strings(es)
	for _, n := range es {
		if c, ok := m.ctorOf[n]; ok {
			m.glob[n] = c
			continue
		}
		m.glob[n] = m.take(mlMangle(n), n)
		m.enumVal[n] = s.enums[n]
	}
	var ms []string
	for n := range s.members {
		ms = append(ms, n)
	}
	sort.Strings(ms)
	fieldTaken := map[string]map[string]bool{}
	for _, n := range ms {
		sm := s.members[n]
		st, path, _ := strings.Cut(n, ".")
		mod, ok := m.modules[st]
		if !ok {
			mod = mlModule(st)
			for m.modTake[mod] {
				mod += "_"
			}
			m.modTake[mod] = true
			m.modules[st] = mod
			fieldTaken[mod] = map[string]bool{}
		}
		ft := fieldTaken[mod]
		uniq := func(x string) string {
			for ft[x] || mlReserved[x] {
				x += "'"
			}
			ft[x] = true
			return x
		}
		f := mlMangle(path)
		mm := &mlMember{module: mod, kind: sm.kind, off: sm.off, variant: m.variantOfType(sm.ctype)}
		mm.addr = uniq(f + "_addr")
		if sm.kind != "agg" {
			mm.read = uniq(f)
			mm.set = uniq("set_" + f)
		}
		m.members[n] = mm
	}
	// a record's fields are named in the module's one name space of
	// fields: the state's, the runtime's, the host's, the other records'
	recNames := map[string]bool{}
	for _, r := range s.records {
		recNames[r] = true
	}
	fieldNames := map[string]bool{}
	for _, ob := range m.objOf {
		if ob.field {
			fieldNames[ob.read] = true
		}
	}
	for _, n := range ms {
		st, _, _ := strings.Cut(n, ".")
		mm := m.members[n]
		if !recNames[st] || mm.kind == "agg" {
			continue
		}
		mm.record = true
		f := mm.read
		for fieldNames[f] || mlRtFields[f] || m.hostField(f) {
			f = mlVariantType(st) + "_" + mm.read
			for fieldNames[f] {
				f += "'"
			}
		}
		fieldNames[f] = true
		mm.read = f
	}
}

// mlRtFields are the fields of the runtime's records, which a field of the
// state may not take: OCaml finds a field by its name.
var mlRtFields = map[string]bool{"mem": true, "sp": true, "limit": true, "glue": true, "st": true, "shared": true,
	"arena_base": true, "next": true, "arena_end": true, "zeroed": true, "lock": true, "stacks": true,
	"stack_next": true, "stack_end": true}

// hostField says n is a field of the host's record.
func (m *mlgen) hostField(n string) bool {
	for _, h := range m.s.hostList {
		if mlMangle(m.s.names[h]) == n {
			return true
		}
	}
	return false
}

// fields are the objects that are the state's fields, in address order.
func (m *mlgen) fields() []*mlObj {
	var out []*mlObj
	for _, ob := range m.objOf {
		if ob.field {
			out = append(out, ob)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].addrAt < out[j].addrAt })
	return out
}

// initial is the field ob's initial value, read from the image.
func (m *mlgen) initial(ob *mlObj) string {
	at := ob.addrAt - scmBase
	b := func(i int) uint64 {
		if at+i < len(m.s.image) {
			return uint64(m.s.image[at+i])
		}
		return 0
	}
	if ob.variant != nil {
		bits, _ := mlAccessBits(ob.kind)
		var v uint64
		for i := 0; i < bits/8; i++ {
			v |= b(i) << (8 * i)
		}
		c, ok := ob.variant.byVal[int64(int32(v))]
		if !ok {
			panic(unsupported{fmt.Sprintf("a variant's object whose initial value, %d, is no enumerator", v)})
		}
		return c
	}
	if ob.kind == "bool" {
		if b(0) != 0 {
			return "true"
		}
		return "false"
	}
	bits, signed := mlAccessBits(ob.kind)
	var v uint64
	for i := 0; i < bits/8; i++ {
		v |= b(i) << (8 * i)
	}
	if signed || bits == 64 {
		sh := 64 - bits
		return mlIntText(int64(v<<sh) >> sh)
	}
	return strconv.FormatUint(v, 10)
}

// mlAccessBits is an accessor kind's width and signedness: s32, u8, ptr...
// (bool a byte).
func mlAccessBits(k string) (int, bool) {
	switch k {
	case "ptr":
		return 64, true
	case "bool":
		return 8, false
	}
	if len(k) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(k[1:])
	if err != nil {
		return 0, false
	}
	return n, k[0] == 's'
}

// imageTakes says an initial value holds the address of the object o, or
// of a byte of it.
func (m *mlgen) imageTakes(o *sobj) bool {
	size := 1
	if b, _ := mlAccessBits(o.kind); b > 0 {
		size = b / 8
	}
	for _, p := range m.s.imagePtrs {
		if int(p) >= o.addr && int(p) < o.addr+size {
			return true
		}
	}
	return false
}

// exported says the host reads the object of Scheme name n: it stays in
// memory, where its exported reader reads it.
func (m *mlgen) exported(n string) bool {
	for _, e := range m.s.g.p.ScmExports {
		if scmName(e) == n {
			return true
		}
	}
	return false
}

// mlModule is a struct's name as a module's: its first letter a capital.
func mlModule(st string) string {
	n := mlMangle(st)
	if n == "" || !(n[0] >= 'a' && n[0] <= 'z') {
		return "S_" + n
	}
	return strings.ToUpper(n[:1]) + n[1:]
}

// mlKindType is the OCaml type of a C value of Scheme kind st.
func mlKindType(st string) string {
	switch st {
	case "void":
		return "unit"
	case "bool":
		return "bool"
	}
	return "int"
}

// glueType is the host's record and the editor's type.
func (m *mlgen) glueType(mli bool) string {
	s := m.s
	var b strings.Builder
	b.WriteString(m.variantTypes())
	b.WriteString(m.recordTypes())
	b.WriteString("(* The host's functions, as the core calls them: the editor carries them. *)\n")
	b.WriteString("type glue = {\n")
	for _, n := range s.hostList {
		fmt.Fprintf(&b, "  %s : %s;\n", m.glob[s.names[n]], strings.Join(m.hostSig(n), " -> "))
	}
	b.WriteString("}\n\n")
	if mli {
		b.WriteString("(* The C's file-scope objects the editor keeps as its fields. *)\nand state\n\n")
	} else if len(m.fields()) == 0 {
		b.WriteString("(* The C's file-scope objects whose address nothing takes: none. *)\nand state = unit\n\n")
	} else {
		b.WriteString("(* The C's file-scope objects whose address nothing takes: the editor's\n   fields, ed.st.name. *)\nand state = {\n")
		for _, ob := range m.fields() {
			mut := ""
			if ob.written {
				mut = "mutable "
			}
			t := mlKindType(scmKindOf(ob.kind))
			if ob.variant != nil {
				t = ob.variant.typ
			}
			fmt.Fprintf(&b, "  %s%s : %s;\n", mut, ob.read, t)
		}
		b.WriteString("}\n\n")
	}
	b.WriteString("and ed = (glue, state) Rt.ed\n\n")
	return b.String()
}

// hostSig is the OCaml types of the host's function n: the editor, its
// parameters, its result.
func (m *mlgen) hostSig(n string) []string {
	ft := m.hostType(n)
	ts := []string{"ed"}
	if ft == nil {
		return append(ts, "unit")
	}
	for _, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		ts = append(ts, mlKindType(scmTypeOf(p.Type())))
	}
	if ft.IsVariadic() {
		ts = append(ts, "int list")
	}
	return append(ts, mlKindType(scmTypeOf(ft.Result())))
}

// mli is the module's interface: the host's record and the editor's type,
// new_editor, and what the profile exports -- the functions and objects
// the host calls back.
func (m *mlgen) mli() string {
	s := m.s
	var b strings.Builder
	b.WriteString("(* Code generated by `go tool whim skel -ml` from a C translation unit; DO NOT EDIT. *)\n\n")
	b.WriteString("(* The core's interface: what a host makes an editor with, and what it\n   calls back. *)\n\n")
	b.WriteString(m.glueType(true))
	b.WriteString("(* A new editor on the host's functions. *)\nval new_editor : glue -> ed\n")
	for _, n := range s.g.p.ScmExports {
		if d := s.defined[n]; d != nil {
			ft, _ := d.Declarator.Type().(*cc.FunctionType)
			if ft == nil || len(s.facts.outs[n]) > 0 || s.facts.tupleRet(n) || isAggr(ft.Result()) || ft.IsVariadic() {
				panic(unsupported{"an export of a type the interface does not write: " + n})
			}
			var ts []string
			if s.takesEd(n) {
				ts = append(ts, "ed")
			}
			for _, p := range ft.Parameters() {
				if p.Type() == nil || p.Type().Kind() == cc.Void {
					continue
				}
				ts = append(ts, mlKindType(scmTypeOf(p.Type())))
			}
			if len(ts) == 0 {
				ts = append(ts, "unit")
			}
			r := mlKindType(scmTypeOf(ft.Result()))
			if m.unitFn[s.names[n]] {
				r = "unit"
			}
			fmt.Fprintf(&b, "val %s : %s\n", m.glob[s.names[n]], strings.Join(append(ts, r), " -> "))
			continue
		}
		if ob := m.objOf[scmName(n)]; ob != nil {
			if ob.agg {
				fmt.Fprintf(&b, "val %s : int\n", ob.read)
			} else {
				fmt.Fprintf(&b, "val %s : ed -> %s\n", ob.read, mlKindType(scmKindOf(ob.kind)))
			}
			m.use(ob.read)
		}
	}
	return b.String()
}

// scmKindOf is the Scheme kind of an accessor's kind: ptr an address.
func scmKindOf(k string) string {
	if k == "ptr" {
		return "u64"
	}
	return k
}

// header is what the functions are written against: the host's record
// and the editor's type, the memory's end, the objects, the constants,
// the members, the host's functions, the tables of function pointers.
func (m *mlgen) header() string {
	s := m.s
	var b strings.Builder
	m.variantNeeds()
	b.WriteString(m.glueType(false))
	b.WriteString(m.variantConvs())
	end := s.litBase + len(s.pool)
	fmt.Fprintf(&b, "(* The memory: the null page and the function pointers below %d, the\n   file-scope objects from there, the string literals from %d to %d. *)\nlet data_end = %d\n\n", scmBase, s.litBase, end, scmAlign(end, 16))

	// the host's functions
	b.WriteString("(* The host's functions, called through the record the editor carries. *)\n")
	for _, n := range s.hostList {
		ft := m.hostType(n)
		as := []string{"ed"}
		if ft != nil {
			k := 0
			for _, p := range ft.Parameters() {
				if p.Type() == nil || p.Type().Kind() == cc.Void {
					continue
				}
				as = append(as, fmt.Sprintf("a%d", k))
				k++
			}
			if ft.IsVariadic() {
				as = append(as, "args")
			}
		}
		on := m.glob[s.names[n]]
		if !m.used[on] {
			continue
		}
		fmt.Fprintf(&b, "let %s %s = ed.glue.%s %s\n", on, strings.Join(as, " "), on, strings.Join(as, " "))
	}

	// the constants
	var es []string
	for n := range m.enumVal {
		es = append(es, n)
	}
	sort.Strings(es)
	b.WriteString("\n(* The C's named constants the functions use. *)\n")
	for _, n := range es {
		if m.used[m.glob[n]] {
			fmt.Fprintf(&b, "let %s = %s\n", m.glob[n], m.intLit(m.enumVal[n]))
		}
	}

	// the objects
	var objs []*sobj
	for _, o := range m.objs {
		objs = append(objs, o)
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].addr < objs[j].addr })
	b.WriteString("\n(* The file-scope objects the functions name: a scalar read by its name and\n   written by set_, its address _addr; an array or a struct its address. *)\n")
	for _, o := range objs {
		ob := m.objOf[o.name]
		if ob.field {
			continue
		}
		if ob.agg {
			if m.used[ob.read] {
				fmt.Fprintf(&b, "let %s = %d\n", ob.read, o.addr)
			}
			continue
		}
		if m.used[ob.addr] {
			fmt.Fprintf(&b, "let %s = %d\n", ob.addr, o.addr)
		}
		if m.used[ob.read] {
			ld := fmt.Sprintf("ld_%s ed %d", o.kind, o.addr)
			if ob.variant != nil {
				ld = ob.variant.typ + "_of_int (" + ld + ")"
			}
			fmt.Fprintf(&b, "let %s ed = %s\n", ob.read, ld)
		}
		if m.used[ob.set] {
			v := "v"
			if ob.variant != nil {
				v = "(int_of_" + ob.variant.typ + " v)"
			}
			fmt.Fprintf(&b, "let %s ed v = st_%s ed %d %s\n", ob.set, o.kind, o.addr, v)
		}
	}

	// the members, a module for each struct
	b.WriteString("\n(* The members the functions name, a module for each struct: M.f ed p reads\n   one, M.set_f ed p v writes it, M.f_addr p is its address. *)\n")
	var mods []string
	byMod := map[string][]string{}
	for n, mm := range m.members {
		if byMod[mm.module] == nil {
			mods = append(mods, mm.module)
		}
		byMod[mm.module] = append(byMod[mm.module], n)
	}
	sort.Strings(mods)
	for _, mod := range mods {
		ns := byMod[mod]
		sort.Slice(ns, func(i, j int) bool {
			a, c := m.members[ns[i]], m.members[ns[j]]
			if a.off != c.off {
				return a.off < c.off
			}
			return ns[i] < ns[j]
		})
		var defs []string
		for _, r := range m.recordList() {
			if r.module == mod {
				defs = append(defs, m.recordModule(r)...)
			}
		}
		for _, n := range ns {
			mm := m.members[n]
			if mm.record {
				continue
			}
			if m.used[mod+"."+mm.addr] {
				defs = append(defs, fmt.Sprintf("  let %s p = p + %d\n", mm.addr, mm.off))
			}
			if mm.kind != "agg" && m.used[mod+"."+mm.read] {
				ld := fmt.Sprintf("ld_%s ed (p + %d)", mm.kind, mm.off)
				if mm.variant != nil {
					ld = mm.variant.typ + "_of_int (" + ld + ")"
				}
				defs = append(defs, fmt.Sprintf("  let %s ed p = %s\n", mm.read, ld))
			}
			if mm.kind != "agg" && m.used[mod+"."+mm.set] {
				v := "v"
				if mm.variant != nil {
					v = "(int_of_" + mm.variant.typ + " v)"
				}
				defs = append(defs, fmt.Sprintf("  let %s ed p v = st_%s ed (p + %d) %s\n", mm.set, mm.kind, mm.off, v))
			}
		}
		if len(defs) > 0 {
			fmt.Fprintf(&b, "module %s = struct\n%send\n\n", mod, strings.Join(defs, ""))
		}
	}

	// the tables
	b.WriteString("(* The functions whose address is taken, a table for each arity and result:\n   a function pointer is an index here, filled in at the end. *)\n")
	var tks []string
	for k := range m.tableKinds() {
		tks = append(tks, k)
	}
	sort.Strings(tks)
	n := len(s.fnOrder)
	for _, k := range tks {
		ar, void := mlTableKind(k)
		var ts, us, as []string
		ts = append(ts, "ed")
		us = append(us, "_")
		for i := 0; i < ar; i++ {
			ts = append(ts, "int")
			us = append(us, "_")
			as = append(as, fmt.Sprintf("a%d", i))
		}
		r := "int"
		if void {
			r = "unit"
		}
		ts = append(ts, r)
		fmt.Fprintf(&b, "let fn_table_%s : (%s) array =\n  Array.make %d (fun %s -> failwith \"no function of this type at this pointer\")\n", k, strings.Join(ts, " -> "), max(n, 1), strings.Join(us, " "))
		fmt.Fprintf(&b, "let call_ptr%s p ed %s = fn_table_%s.(fn_index p) ed %s\n\n", k, strings.Join(as, " "), k, strings.Join(as, " "))
	}
	return b.String()
}

// mlTableKind is a table's arity, and whether its functions' value is ().
func mlTableKind(k string) (int, bool) {
	void := strings.HasSuffix(k, "v")
	n, _ := strconv.Atoi(strings.TrimSuffix(k, "v"))
	return n, void
}

// tableKinds are the tables: those the calls name, and those of the
// functions whose address is taken.
func (m *mlgen) tableKinds() map[string]bool {
	out := map[string]bool{}
	for k := range m.tables {
		out[k] = true
	}
	for _, name := range m.s.fnOrder {
		if k := m.fnKind(name); k != "" {
			out[k] = true
		}
	}
	return out
}

// fnKind is the table a pointer to the C function name is in.
func (m *mlgen) fnKind(name string) string {
	var ft *cc.FunctionType
	if d := m.s.defined[name]; d != nil {
		ft, _ = d.Declarator.Type().(*cc.FunctionType)
	} else if hd := m.s.hostFns[name]; hd != nil {
		ft, _ = hd.Type().(*cc.FunctionType)
	}
	if ft == nil || ft.IsVariadic() {
		return ""
	}
	n := 0
	for _, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		n++
	}
	k := strconv.Itoa(n)
	if scmTypeOf(ft.Result()) == "void" {
		k += "v"
	}
	return k
}

// tableEntries fill the tables: each function whose address is taken, at
// its index, as the Scheme's table has it -- itself, or a function that
// passes a truth value as a number.
func (m *mlgen) tableEntries() string {
	var b strings.Builder
	b.WriteString("(* The tables of function pointers, filled in. *)\nlet () =\n")
	any := false
	for i, name := range m.s.fnOrder {
		k := m.fnKind(name)
		if k == "" {
			continue
		}
		var ft *cc.FunctionType
		if d := m.s.defined[name]; d != nil {
			ft, _ = d.Declarator.Type().(*cc.FunctionType)
		} else {
			ft = m.hostType(name)
		}
		forms, err := scmRead(m.s.tableEntry(name, ft))
		if err != nil || len(forms) != 1 {
			panic(fmt.Sprintf("the table entry of %s does not read back: %v", name, err))
		}
		f := &mlfn{m: m, refs: map[string]bool{}}
		x := f.expr(forms[0], newMlScope(nil), wantValue)
		f.finish()
		fmt.Fprintf(&b, "  fn_table_%s.(%d) <- %s;\n", k, i, mrender(mlParen(x, precApp).d))
		any = true
	}
	if !any {
		b.WriteString("  ()\n\n")
		return b.String()
	}
	b.WriteString("  ()\n\n")
	return b.String()
}

// fixups write the literals' addresses into the image: it places the
// literals only the initial values name, so it comes before data_end is.
func (m *mlgen) fixups() {
	s := m.s
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
}

// image is new_editor: an editor on the host's record, its memory holding
// the objects' initial values and the literals.
func (m *mlgen) image() string {
	s := m.s
	var b strings.Builder
	b.WriteString("(* A new editor on the host's functions: its memory, with the objects'\n   initial values and the literals. *)\nlet new_editor glue =\n")
	var fs []string
	for _, ob := range m.fields() {
		fs = append(fs, fmt.Sprintf("%s = %s", ob.read, m.initial(ob)))
	}
	if len(fs) == 0 {
		b.WriteString("  let ed = Rt.make_editor glue () data_end in\n")
	} else {
		fmt.Fprintf(&b, "  let st = {\n    %s;\n  } in\n  let ed = Rt.make_editor glue st data_end in\n", strings.Join(fs, ";\n    "))
	}
	for _, l := range mlImage(s.image, scmBase) {
		b.WriteString("  " + l + ";\n")
	}
	for _, l := range mlImage(s.pool, s.litBase) {
		b.WriteString("  " + l + ";\n")
	}
	b.WriteString("  ed\n")
	return b.String()
}

// mlImage are the lines that copy img into memory at base, as scmImage's.
func mlImage(img []byte, base int) []string {
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
		out = append(out, fmt.Sprintf("mem_image ed %d %s", base+i, mlString(img[i:end])))
		i = end
	}
	return out
}

// mlString is bytes as an OCaml string literal.
func mlString(bs []byte) string {
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
			fmt.Fprintf(&b, `\x%02x`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// scmUnstring is the bytes of a Scheme string literal.
func scmUnstring(lit string) []byte {
	s := lit[1 : len(lit)-1]
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			out = append(out, c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			out = append(out, '\n')
		case 't':
			out = append(out, '\t')
		case 'r':
			out = append(out, '\r')
		case 'x':
			j := strings.IndexByte(s[i:], ';')
			v, _ := strconv.ParseUint(s[i+1:i+j], 16, 8)
			out = append(out, byte(v))
			i += j
		default:
			out = append(out, s[i])
		}
	}
	return out
}

// intLit is the decimal integer lit as OCaml's int holds it: its 64 bits
// read as a signed long, saturated to 63 bits (ml63).
func (m *mlgen) intLit(lit string) string {
	v, ok := new(big.Int).SetString(lit, 10)
	if !ok {
		return lit
	}
	if v.IsInt64() {
		k := v.Int64()
		if c := ml63(k); c != k {
			m.st.clamped++
			k = c
		}
		return mlIntText(k)
	}
	// past a signed long: an unsigned long's bits
	two64 := new(big.Int).Lsh(big.NewInt(1), 64)
	w := new(big.Int).Sub(v, two64)
	if !w.IsInt64() {
		panic(unsupported{"a constant past 64 bits: " + lit})
	}
	k := w.Int64()
	if c := ml63(k); c != k {
		m.st.clamped++
		k = c
	}
	return mlIntText(k)
}

func mlIntText(k int64) string {
	switch k {
	case 1<<62 - 1:
		return "max_int"
	case -1 << 62:
		return "min_int"
	}
	return strconv.FormatInt(k, 10)
}

// mlSCC are the strongly connected components of the graph of n nodes,
// each a list of nodes, a component after every one it reaches.
func mlSCC(n int, edges [][]int) [][]int {
	index := make([]int, n)
	low := make([]int, n)
	on := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	var stack []int
	var out [][]int
	next := 0
	type frame struct{ v, e int }
	for root := 0; root < n; root++ {
		if index[root] >= 0 {
			continue
		}
		call := []frame{{root, 0}}
		index[root], low[root] = next, next
		next++
		stack = append(stack, root)
		on[root] = true
		for len(call) > 0 {
			f := &call[len(call)-1]
			if f.e < len(edges[f.v]) {
				w := edges[f.v][f.e]
				f.e++
				if index[w] < 0 {
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					on[w] = true
					call = append(call, frame{w, 0})
				} else if on[w] {
					low[f.v] = min(low[f.v], index[w])
				}
				continue
			}
			v := f.v
			call = call[:len(call)-1]
			if len(call) > 0 {
				u := call[len(call)-1].v
				low[u] = min(low[u], low[v])
			}
			if low[v] == index[v] {
				var comp []int
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					on[w] = false
					comp = append(comp, w)
					if w == v {
						break
					}
				}
				sort.Ints(comp)
				out = append(out, comp)
			}
		}
	}
	return out
}
