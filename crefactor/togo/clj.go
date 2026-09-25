package togo

// clj.go is the Clojure backend's frame: the namespace a translation unit
// becomes, its struct types, the editor's state, the initial values, and
// the report of what it refuses.  doc/CLOJURE.md is the design and the
// contract with the hand-written namespaces around it.
//
// It reads the same analysis the Go and the Java are written from, and
// takes the Java backend's decisions whole (jgen: which pointer class, which
// struct class, which member or object is boxed); what it adds is the form
// -- the lowered function (lower.go), printed as Clojure (clj_fn.go,
// clj_expr.go) -- and the representation of what Java has as fields:
//
//   - a struct or union is a deftype whose scalar and pointer members are
//     mutable fields behind an interface of accessors, (.m s) and
//     (.set_m s v), and whose struct, array and boxed members are final
//     fields, (.-m s); it is a whim.rt.Struct (set, zero) for the runtime
//     and has copy and, where compared, eq;
//   - the editor is a deftype of three typed slot arrays, longs for the
//     integers, booleans, and objects for the rest, read and written through
//     the namespace's macros (g ed name) and (g! ed name v): a deftype's
//     constructor takes every field, and the JVM allows a method 255
//     parameter slots, so the 821 file-scope objects cannot be one type's
//     fields;
//   - a C function is a defn of the editor and the C's parameters; a
//     function declared and not defined is the host's, called in the host
//     namespace (whim.cljhost) with the editor first.

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// cljCoreNames are clojure.core's public names that a C identifier can
// spell, and Clojure's special forms: a C name among them takes a trailing
// underscore, as a Java reserved word does in the Java backend.
var cljReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`Inst abs accessor aclone agent aget alength alias alter amap ancestors and
	apply areduce aset assert assoc atom await await1 bases bean bigdec bigint biginteger binding boolean
	booleans butlast byte bytes case cast cat char chars chunk class comment commute comp comparator compare
	compile complement completing concat cond condp conj cons constantly count cycle dec declare dedupe
	definline definterface defmacro defmethod defmulti defn defonce defprotocol defrecord defstruct deftype
	delay deliver denominator deref derive descendants destructure disj dissoc distinct doall dorun doseq
	dosync dotimes doto double doubles drop eduction empty ensure eval extend extenders ffirst filter filterv
	find first flatten float floats flush fn fnext fnil for force format frequencies future gensym get hash
	identity import inc int interleave intern interpose into ints iterate iteration juxt keep key keys keyword
	last let letfn list load locking long longs loop macroexpand map mapcat mapv max memfn memoize merge meta
	methods min mod munge name namespace newline next nfirst nnext not ns nth nthnext nthrest num numerator or
	parents partial partition partitionv pcalls peek pmap pop pr prefers print printf println prn promise
	proxy pvalues quot rand range rationalize read reduce reduced reductions ref refer reify rem remove repeat
	repeatedly replace replicate require resolve rest reverse rseq rsubseq second send seq seque sequence set
	short shorts shuffle slurp some sort spit str struct subs subseq subvec supers symbol sync take test time
	trampoline transduce transient type underive unquote unreduced update use val vals vec vector when while
	zipmap
	catch def do finally fn if monitor new quote recur throw try var
	nil true false
	g e enumerators i8 u8 i16 u16 i32 u32 slots ed st Editor Struct Ptr Rt Ga BytePtr ShortPtr IntPtr LongPtr
	BoolPtr IFn Integer Long Boolean Object String Math System`) {
		cljReserved[w] = true
	}
}

// cgen is the Clojure backend's state for one translation unit.
type cgen struct {
	g           *gen
	j           *jgen // the Java backend's decisions
	ns, hostNS  string
	hostFns     map[string]bool
	slots       map[string]*cslot // by the analysis's key
	slotOrder   []*cslot
	nL, nZ, nO  int
	enums       map[string]string
	adapters    map[string]string
	adapterText []string
	defined     map[string]bool
}

// cslot is a file-scope object's slot in the editor.
type cslot struct {
	name  string // the C name, or <function>_<name> for a hoisted static
	key   string
	kind  byte // 'L', 'Z' or 'O'
	idx   int
	jt    string
	c     cc.Type
	boxed bool
	d     *cc.Declarator
	in    *cc.Initializer
}

// cljName is a C identifier as a Clojure symbol: the profile's renames, then
// a trailing underscore for what Clojure has already.
func (c *cgen) cljName(s string) string {
	if r, ok := c.g.p.Rename[s]; ok {
		s = r
	}
	if cljReserved[s] {
		return s + "_"
	}
	return s
}

func (c *cgen) fnName(s string) string    { return c.cljName(s) }
func (c *cgen) localName(s string) string { return c.cljName(s) }
func (c *cgen) enumName(s string) string  { return c.cljName(s) }

// memberName is a member's name in its deftype: its own, _anonN for an
// unnamed one, with a trailing underscore for a name the deftype's own
// methods or Clojure have.
func (c *cgen) memberName(fl *cc.Field) string {
	if fl.Name() == "" {
		for i, m := range members(fl.ParentType()) {
			if m == fl {
				return fmt.Sprintf("_anon%d", i)
			}
		}
	}
	n := fl.Name()
	switch n {
	case "set", "zero", "copy", "eq":
		return n + "_"
	}
	return c.cljName(n)
}

// slotOf is the slot of a file-scope object or a hoisted static.
func (c *cgen) slotOf(d *cc.Declarator, lf *lfn) *cslot {
	key := c.g.a.declKey(d)
	return c.slots[key]
}

// writeClj writes the translation unit as one Clojure namespace to path,
// and beside it path.refused: every function it refuses, with why.
func (g *gen) writeClj(path string) error {
	j := &jgen{g: g, structs: map[string]string{}, boxed: map[*cc.Declarator]bool{}, boxedGlobal: map[string]bool{},
		enums: map[string]string{}, defined: map[string]bool{},
		classType: map[string]cc.Type{}, needEq: map[string]bool{}, boxedField: map[string]bool{},
		gaUsed: map[string]*gaHelper{}, ifaces: map[string]*jiface{}, ifaceByName: map[string]*jiface{},
		fnRefs: map[string]string{}, fnRefText: map[string]string{}}
	j.boxes()
	ns := g.p.CljNamespace
	if ns == "" {
		ns = "whim.editor"
	}
	host := g.p.CljHost
	if host == "" {
		host = "whim.cljhost"
	}
	c := &cgen{g: g, j: j, ns: ns, hostNS: host, hostFns: map[string]bool{}, slots: map[string]*cslot{},
		enums: map[string]string{}, adapters: map[string]string{}, defined: map[string]bool{}}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			c.defined[ed.FunctionDefinition.Declarator.Name()] = true
			j.defined[ed.FunctionDefinition.Declarator.Name()] = true
		}
	}
	var hostNames []string
	for name := range g.a.fnDecls {
		p := g.p
		if p.allocators[name] || p.frees[name] || p.byteMove(name) || name == p.Bytes.Set || name == p.Bytes.Cmp {
			continue
		}
		if !c.defined[name] && !strings.HasPrefix(name, "__") {
			c.hostFns[name] = true
			hostNames = append(hostNames, name)
		}
	}
	sort.Strings(hostNames)
	c.allocSlots()

	// the functions, written or refused
	var report strings.Builder
	texts := map[string]string{}
	var fds []*cc.FunctionDefinition
	written, replaced, total, structured, machine, split := 0, 0, 0, 0, 0, 0
	whys := map[string]int{}
	shapes := map[string]int{} // why a function is a state machine
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef {
			continue
		}
		fd := ed.FunctionDefinition
		total++
		name := fd.Declarator.Name()
		if j.replaced(name) {
			replaced++
			continue
		}
		fds = append(fds, fd)
		src, why, mode, shape := c.function(fd)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", name, why)
			whys[reasonKey(why)]++
			texts[name] = c.stub(fd.Declarator, why)
			continue
		}
		written++
		switch mode {
		case "structured":
			structured++
		case "machine":
			machine++
			shapes[shape]++
		case "split":
			shapes[shape]++
			machine++
			split++
		}
		texts[name] = "\n" + src
	}
	inits, failed := c.initializers()
	for _, f := range failed {
		fmt.Fprintf(&report, "initial value of %s\n", f)
	}
	j.eqClosure()
	types := c.structTypes()

	var b strings.Builder
	fmt.Fprintf(&b, ";; Code generated by `go tool whim skel -clj` from a C translation unit; DO NOT EDIT.\n")
	fmt.Fprintf(&b, ";; %d of %d functions written (%d structured, %d state machines), %d the runtime's; the rest are stubs that throw, with the reason.\n\n",
		written, total, structured, machine, replaced)
	fmt.Fprintf(&b, "(ns %s\n  (:require [%s])\n  (:import [whim.rt BytePtr ShortPtr IntPtr LongPtr BoolPtr Ptr Rt Ga Struct]))\n\n", ns, host)
	b.WriteString("(set! *warn-on-reflection* true)\n(set! *unchecked-math* true)\n\n")
	b.WriteString(cljPrelude)
	// the functions a function calls before it where they can be: a
	// namespace's top-level forms are one method of its class when it is
	// compiled ahead of time, and a declare of every function would take a
	// third of its 64 KB; and a call to a function defined before it is a
	// direct one
	order, forward := c.fnOrder(fds)
	if len(forward) > 0 {
		b.WriteString("\n(declare")
		for i, n := range forward {
			if i%8 == 0 {
				b.WriteString("\n ")
			}
			b.WriteString(" " + n)
		}
		b.WriteString(")\n\n")
	}
	var enums []string
	for _, e := range c.enums {
		enums = append(enums, e)
	}
	sort.Strings(enums)
	// the enumerators the code names, by name: (e NAME) is its value,
	// written where it is used -- a def each would be a top-level form each,
	// and the namespace's class has one method for all of them
	b.WriteString(";; The enumerators the code names: (e NAME) is the value.\n(def ^:private enumerators\n  (read-string \"{")
	b.WriteString(strings.Join(enums, "\n    "))
	b.WriteString(`}"))

(defmacro ^:private e
  "The value of the enumerator n."
  [n]
  (or (get enumerators n) (throw (IllegalArgumentException. (str "no enumerator " n)))))

`)
	b.WriteString(types)
	head, tail := c.editorText(inits)
	b.WriteString(head)
	b.WriteString(c.gaHelpers())
	for _, a := range c.adapterText {
		b.WriteString(a)
	}
	for _, name := range order {
		b.WriteString(texts[name])
	}
	b.WriteString("\n" + tail)

	var ks []string
	for k := range whys {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(a, b int) bool {
		if whys[ks[a]] != whys[ks[b]] {
			return whys[ks[a]] > whys[ks[b]]
		}
		return ks[a] < ks[b]
	})
	fmt.Fprintf(logw, "clj: %d of %d functions written (%d structured, %d state machines, %d of them split), %d the runtime's, %d refused\n",
		written, total, structured, machine, split, replaced, total-written-replaced)
	for _, k := range ks {
		fmt.Fprintf(logw, "  %5d  %s\n", whys[k], k)
	}
	fmt.Fprintf(logw, "clj: state machines, by the first thing that would not nest:\n")
	var ss []string
	for k := range shapes {
		ss = append(ss, k)
	}
	sort.Slice(ss, func(a, b int) bool {
		if shapes[ss[a]] != shapes[ss[b]] {
			return shapes[ss[a]] > shapes[ss[b]]
		}
		return ss[a] < ss[b]
	})
	for _, k := range ss {
		fmt.Fprintf(logw, "  %5d  %s\n", shapes[k], k)
	}
	if len(failed) > 0 {
		fmt.Fprintf(logw, "clj: %d initial values refused\n", len(failed))
	}
	if err := os.WriteFile(path+".refused", []byte(report.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// cljPrelude is what every generated namespace has: the conversions to the
// C integer types, and the editor's slots.
const cljPrelude = `;; C's conversion of a long to each integer type narrower than 64 bits.
(defmacro ^:private i8 [x] ` + "`" + `(long (unchecked-byte ~x)))
(defmacro ^:private u8 [x] ` + "`" + `(bit-and ~x 0xff))
(defmacro ^:private i16 [x] ` + "`" + `(long (unchecked-short ~x)))
(defmacro ^:private u16 [x] ` + "`" + `(bit-and ~x 0xffff))
(defmacro ^:private i32 [x] ` + "`" + `(long (unchecked-int ~x)))
(defmacro ^:private u32 [x] ` + "`" + `(bit-and ~x 0xffffffff))
`

// stub is a refused function: its parameters, and a body that throws.
func (c *cgen) stub(d *cc.Declarator, why string) string {
	ft, _ := d.Type().(*cc.FunctionType)
	var ps []string
	for i, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		ps = append(ps, fmt.Sprintf("p%d", i))
	}
	return fmt.Sprintf("\n(defn %s [ed%s]\n  (throw (UnsupportedOperationException. %s)))\n",
		c.fnName(d.Name()), strings.Join(append([]string{""}, ps...), " "), cljQuote("refused: "+why))
}

// runtimeBody is the Clojure body the profile gives a function, or nil.
func (c *cgen) runtimeBody(name string) func(string) string {
	for _, rb := range c.g.p.RuntimeBodies {
		if rb.Name == name && rb.Clj != nil {
			return rb.Clj
		}
	}
	return nil
}

// --- the editor's state -------------------------------------------------------

// allocSlots gives every file-scope object and hoisted static its slot.
func (c *cgen) allocSlots() {
	type global struct {
		d  *cc.Declarator
		in *cc.Initializer
	}
	var order []string
	globals := map[string]*global{}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration.Case != cc.DeclarationDecl {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			id := l.InitDeclarator
			d := id.Declarator
			if d.IsTypename() || d.Type().Kind() == cc.Function {
				continue
			}
			gl, ok := globals[d.Name()]
			if !ok {
				gl = &global{d: d}
				globals[d.Name()] = gl
				order = append(order, d.Name())
			}
			if at, ok := gl.d.Type().(*cc.ArrayType); ok && (at.IsIncomplete() || at.Len() <= 0) || id.Initializer != nil {
				gl.d = d
			}
			if id.Initializer != nil {
				gl.in = id.Initializer
			}
		}
	}
	add := func(d *cc.Declarator, name, key string, in *cc.Initializer) {
		t := d.Type()
		s := &cslot{name: name, key: key, c: t, d: d, in: in, boxed: c.j.isBoxed(d)}
		jt, why := c.j.jt(t, key)
		if why != "" {
			jt = "Object"
		}
		s.jt = jt
		_, scalar := scalarKind(t)
		switch {
		case s.boxed || !scalar:
			s.kind, s.idx = 'O', c.nO
			c.nO++
		case t.Kind() == cc.Bool:
			s.kind, s.idx = 'Z', c.nZ
			c.nZ++
		default:
			s.kind, s.idx = 'L', c.nL
			c.nL++
		}
		c.slots[key] = s
		c.slotOrder = append(c.slotOrder, s)
	}
	for _, name := range order {
		gl := globals[name]
		add(gl.d, name, "global:"+name, gl.in)
	}
	for _, s := range c.g.a.statics {
		add(s.d, s.fn+"_"+s.d.Name(), fmt.Sprintf("static:%s.%s", s.fn, s.d.Name()), s.init)
	}
}

// initializers are the initial values as the bodies of functions of a few
// hundred forms each, and the objects the slots start with; and what it
// refused.
func (c *cgen) initializers() ([]string, []string) {
	f := &cfn{c: c, name: "init-globals", vars: map[*lvar]*cvar{}}
	var chunks []string
	var cur []string
	var failed []string
	flush := func() {
		if len(cur) > 0 {
			chunks = append(chunks, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, s := range c.slotOrder {
		if s.in == nil || zeroInit(s.in) || s.jt == "Object" && s.c.Kind() != cc.Ptr {
			continue
		}
		forms := func() (forms []string) {
			defer func() {
				if r := recover(); r != nil {
					u, ok := r.(unsupported)
					if !ok {
						panic(r)
					}
					failed = append(failed, s.name+": "+u.why)
					forms = []string{";; refused: the initial value of " + s.name + ": " + u.why}
				}
			}()
			pre := f.capture(func() { f.initInto(c.slotPlace(f, s), s.in) })
			for _, p := range pre {
				forms = append(forms, p.form)
			}
			return forms
		}()
		cur = append(cur, forms...)
		if len(cur) > 300 {
			flush()
		}
	}
	flush()
	return chunks, failed
}

// slotPlace is a slot as the place an initial value goes.
func (c *cgen) slotPlace(f *cfn, s *cslot) cplace {
	g := "(g ed " + s.name + ")"
	switch {
	case s.boxed:
		k, sc := scalarKind(s.c)
		return cplace{t: s.c, jt: s.jt, key: s.key, set: func(v string) string {
			if sc {
				v = storeK(v, k)
			}
			return "(aset " + g + " 0 " + v + ")"
		}}
	case isAggr(s.c) || s.c.Kind() == cc.Array:
		return cplace{obj: cv{s: g, t: s.jt, c: s.c, typed: true}, t: s.c, jt: s.jt, key: s.key}
	}
	return cplace{t: s.c, jt: s.jt, key: s.key, set: func(v string) string {
		return "(g! ed " + s.name + " " + v + ")"
	}}
}

// editorText is the Editor type, its slots, new-editor, host-of and the
// accessors the host reads.
func (c *cgen) editorText(inits []string) (string, string) {
	var b strings.Builder
	b.WriteString(";; The editor: its host, and the C's file-scope objects in three typed slot arrays.\n")
	b.WriteString("(deftype Editor [host ^longs L ^booleans Z ^objects O])\n\n")
	b.WriteString(";; Each file-scope object's slot: [array index class] -- read from a string, since a\n;; literal map would be built by code in one method of the namespace's class.\n(def ^:private slots\n  (read-string \"{")
	for i, s := range c.slotOrder {
		if i > 0 {
			b.WriteString("\n    ")
		}
		fmt.Fprintf(&b, "%s [%c %d", s.name, s.kind+'a'-'A', s.idx)
		if h := cljHint(s.jt); h != "" && !s.boxed {
			fmt.Fprintf(&b, " %s", h)
		} else if s.boxed {
			fmt.Fprintf(&b, " %s", cljHint(s.jt+"[]"))
		}
		b.WriteString("]")
	}
	b.WriteString("}\"))\n\n")
	b.WriteString(`(defmacro ^:private g
  "The file-scope object n of the editor ed."
  [ed n]
  (let [[k i t] (or (get slots n) (throw (IllegalArgumentException. (str "no file-scope object " n))))]
    (case k
      l (list 'aget (with-meta (list '.-L ed) {:tag 'longs}) i)
      z (list 'aget (with-meta (list '.-Z ed) {:tag 'booleans}) i)
      o (with-meta (list 'aget (with-meta (list '.-O ed) {:tag 'objects}) i) (if t {:tag t} {})))))

(defmacro ^:private g!
  "Set the file-scope object n of the editor ed to v."
  [ed n v]
  (let [[k i] (or (get slots n) (throw (IllegalArgumentException. (str "no file-scope object " n))))]
    (case k
      l (list 'aset (with-meta (list '.-L ed) {:tag 'longs}) i (list 'long v))
      z (list 'aset (with-meta (list '.-Z ed) {:tag 'booleans}) i (list 'boolean v))
      o (list 'aset (with-meta (list '.-O ed) {:tag 'objects}) i v))))

`)
	// the objects the slots hold from the start: arrays, structs, boxes
	f := &cfn{c: c, name: "make-objects", vars: map[*lvar]*cvar{}}
	var objs []string
	for _, s := range c.slotOrder {
		switch {
		case s.boxed:
			objs = append(objs, "(g! ed "+s.name+" "+f.newBoxOf(s.jt)+")")
		case isAggr(s.c) || s.c.Kind() == cc.Array:
			if s.jt == "Object" {
				continue
			}
			objs = append(objs, "(g! ed "+s.name+" "+f.newOf(s.c, s.jt)+")")
		}
	}
	var fnsText strings.Builder
	var calls []string
	chunk := func(name string, forms []string) {
		for i := 0; i < len(forms); i += 300 {
			end := i + 300
			if end > len(forms) {
				end = len(forms)
			}
			n := fmt.Sprintf("%s-%d", name, i/300)
			calls = append(calls, "("+n+" ed)")
			fmt.Fprintf(&fnsText, "(defn- %s [^Editor ed]\n  %s\n  nil)\n\n", n, strings.Join(forms[i:end], "\n  "))
		}
	}
	chunk("make-objects", objs)
	for i, in := range inits {
		n := fmt.Sprintf("init-globals-%d", i)
		calls = append(calls, "("+n+" ed)")
		fmt.Fprintf(&fnsText, "(defn- %s [^Editor ed]\n  %s\n  nil)\n\n", n, strings.ReplaceAll(in, "\n", "\n  "))
	}
	head := b.String()
	b.Reset()
	b.WriteString(fnsText.String())
	fmt.Fprintf(&b, `(defn new-editor
  "An editor on host, a whim.host.Host: the C's file-scope objects as they start."
  [host]
  (let [ed (Editor. host (long-array %d) (boolean-array %d) (object-array %d))]
    %s
    ed))

(defn host-of
  "The host of the editor ed."
  [^Editor ed]
  (.-host ed))

`, c.nL, c.nZ, c.nO, strings.Join(calls, "\n    "))
	for _, name := range c.g.p.CljExports {
		if s := c.slots["global:"+name]; s != nil {
			fmt.Fprintf(&b, "(defn %s\n  \"The file-scope object %s of the editor ed, for the host.\"\n  [^Editor ed]\n  (g ed %s))\n\n", c.cljName(name), name, s.name)
		}
	}
	return head, b.String()
}

// fnOrder is the order the functions are written in -- each after the
// functions it names, where no cycle forbids it -- and the functions named
// before they are defined: the declare.
func (c *cgen) fnOrder(fds []*cc.FunctionDefinition) ([]string, []string) {
	byName := map[string]*cc.FunctionDefinition{}
	for _, fd := range fds {
		byName[fd.Declarator.Name()] = fd
	}
	refs := func(fd *cc.FunctionDefinition) []string {
		var r []string
		seen := map[string]bool{}
		var rec func(cc.Node)
		rec = func(n cc.Node) {
			if n == nil {
				return
			}
			if x, ok := n.(*cc.PrimaryExpression); ok && x.Case == cc.PrimaryExpressionIdent {
				if d, ok := x.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
					if name := d.Name(); byName[name] != nil && !seen[name] {
						seen[name] = true
						r = append(r, name)
					}
				}
			}
			walkChildrenFn(n, rec)
		}
		rec(fd.CompoundStatement)
		return r
	}
	var order []string
	state := map[string]int{} // 1 on the stack, 2 written
	var visit func(name string)
	visit = func(name string) {
		if state[name] != 0 {
			return
		}
		state[name] = 1
		for _, r := range refs(byName[name]) {
			visit(r)
		}
		state[name] = 2
		order = append(order, name)
	}
	for _, fd := range fds {
		visit(fd.Declarator.Name())
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	fwd := map[string]bool{}
	for _, n := range order {
		for _, r := range refs(byName[n]) {
			if pos[r] >= pos[n] {
				fwd[c.fnName(r)] = true
			}
		}
	}
	// what the adapters and the initial values name comes before them too
	for n := range byName {
		fwd[c.fnName(n)] = fwd[c.fnName(n)]
	}
	var forward []string
	for n, f := range fwd {
		if f {
			forward = append(forward, n)
		}
	}
	for _, a := range c.adapterText {
		for n := range byName {
			if strings.Contains(a, "("+c.fnName(n)+" ") && !fwd[c.fnName(n)] {
				fwd[c.fnName(n)] = true
				forward = append(forward, c.fnName(n))
			}
		}
	}
	sort.Strings(forward)
	var names []string
	for _, n := range order {
		names = append(names, n)
	}
	return names, forward
}

// gaHelpers are the growarray's typed accessors: each makes or grows the
// storage as its type (Ga) and keeps it in the array.
func (c *cgen) gaHelpers() string {
	j := c.j
	var ks []string
	for k := range j.gaUsed {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b strings.Builder
	data, maxlen := c.cljName(c.g.p.GrowArray.Data), c.cljName(c.g.p.GrowArray.MaxLen)
	for _, to := range ks {
		h := j.gaUsed[to]
		var mk string
		switch to {
		case "BytePtr":
			mk = "(Ga/bytes (.%[1]s gap) (.%[2]s gap))"
		case "ShortPtr":
			mk = "(Ga/shorts (.%[1]s gap) (.%[2]s gap))"
		case "IntPtr":
			mk = "(Ga/ints (.%[1]s gap) (.%[2]s gap))"
		case "LongPtr":
			mk = "(Ga/longs (.%[1]s gap) (.%[2]s gap))"
		case "BoolPtr":
			mk = "(Ga/bools (.%[1]s gap) (.%[2]s gap))"
		default:
			e := elemJ(to)
			alloc := "(object-array n)"
			if _, ok := j.classType[e]; ok {
				alloc = "(array-" + e + " n)"
			}
			fmt.Fprintf(&b, "(def ^:private ^java.util.function.IntFunction mk-%s\n  (reify java.util.function.IntFunction (apply [_ n] %s)))\n", h.name, alloc)
			mk = "(Ga/ptrs (.%[1]s gap) (.%[2]s gap) mk-" + h.name + ")"
		}
		mk = fmt.Sprintf(mk, data, maxlen)
		fmt.Fprintf(&b, "(defn- %s ^%s [^%s gap]\n  (let [p %s]\n    (.set_%s gap p)\n    p))\n\n", h.name, cljHint(to), h.class, mk, data)
	}
	return b.String()
}

// --- the struct types -------------------------------------------------------------

// structTypes are the deftypes of every struct and union the code reaches, a
// struct held by value before the struct holding it.
func (c *cgen) structTypes() string {
	j := c.j
	f := &cfn{c: c, name: "types", vars: map[*lvar]*cvar{}}
	// every class the members reach, found first: the text of one may name
	// another
	for i := 0; i < len(j.classOrder); i++ {
		for _, fl := range members(j.classType[j.classOrder[i]]) {
			if fl != nil {
				j.jt(fl.Type(), fieldKey(fl))
			}
		}
	}
	j.eqClosure()
	done := map[string]bool{}
	var order []string
	var visit func(name string)
	visit = func(name string) {
		if done[name] {
			return
		}
		done[name] = true
		for _, fl := range members(j.classType[name]) {
			if fl == nil {
				continue
			}
			ft := fl.Type()
			for ft.Kind() == cc.Array {
				ft = ft.(*cc.ArrayType).Elem()
			}
			if isAggr(ft) {
				if n, why := j.structName(ft); why == "" {
					visit(n)
				}
			}
		}
		order = append(order, name)
	}
	for i := 0; i < len(j.classOrder); i++ {
		visit(j.classOrder[i])
	}
	var b strings.Builder
	b.WriteString(";; The struct and union types.\n(declare")
	for _, n := range order {
		b.WriteString(" new-" + n + " array-" + n)
	}
	b.WriteString(")\n\n")
	for _, n := range order {
		b.WriteString(c.structType(f, n, j.classType[n]))
	}
	return b.String()
}

// structType is one struct's interface, deftype and makers.
func (c *cgen) structType(f *cfn, name string, t cc.Type) string {
	j := c.j
	var iface, fields, meths, set, zero, eq, ctor []string
	for _, fl := range members(t) {
		if fl == nil {
			continue
		}
		m := c.memberName(fl)
		key := fieldKey(fl)
		ft, why := j.jt(fl.Type(), key)
		if why != "" {
			// what no code reads: kept, as an Object
			fields = append(fields, "^:unsynchronized-mutable "+m)
			iface = append(iface, "("+m+" [])", "(set_"+m+" [v])")
			meths = append(meths, fmt.Sprintf("(%s [_] %s) (set_%s [_ v__] (set! %s v__) nil)", m, m, m, m))
			set = append(set, fmt.Sprintf("(set! %s (.%s o__))", m, m))
			zero = append(zero, fmt.Sprintf("(set! %s nil)", m))
			ctor = append(ctor, "nil")
			continue
		}
		switch {
		case j.boxedField[key]:
			h := cljHint(ft + "[]")
			fields = append(fields, "^"+h+" "+m)
			set = append(set, fmt.Sprintf("(aset %s 0 (aget ^%s (.-%s o__) 0))", m, h, m))
			zero = append(zero, "(aset "+m+" 0 "+boxZero(fl.Type())+")")
			eq = append(eq, eqForm(fmt.Sprintf("(aget %s 0)", m), fmt.Sprintf("(aget ^%s (.-%s o__) 0)", h, m), fl.Type(), ft, f))
			ctor = append(ctor, f.newBoxOf(ft))
		case isAggr(fl.Type()) || fl.Type().Kind() == cc.Array:
			h := cljHint(ft)
			fields = append(fields, "^"+h+" "+m)
			set = append(set, copyForm(m, fmt.Sprintf("(.-%s o__)", m), fl.Type(), ft, f))
			zero = append(zero, f.zeroInPlace(cv{s: m, t: ft, c: fl.Type(), typed: true}, fl.Type())...)
			eq = append(eq, eqForm(m, fmt.Sprintf("(.-%s o__)", m), fl.Type(), ft, f))
			ctor = append(ctor, f.newOf(fl.Type(), ft))
		default:
			k, scalar := scalarKind(fl.Type())
			switch {
			case scalar && k.boolean:
				fields = append(fields, "^:unsynchronized-mutable ^boolean "+m)
				iface = append(iface, "(^boolean "+m+" [])", "(set_"+m+" [^boolean v])")
				zero = append(zero, fmt.Sprintf("(set! %s (boolean false))", m))
				ctor = append(ctor, "false")
			case scalar:
				fields = append(fields, "^:unsynchronized-mutable ^long "+m)
				iface = append(iface, "(^long "+m+" [])", "(set_"+m+" [^long v])")
				zero = append(zero, fmt.Sprintf("(set! %s 0)", m))
				ctor = append(ctor, "0")
			default:
				fields = append(fields, "^:unsynchronized-mutable "+m)
				iface = append(iface, "("+m+" [])", "(set_"+m+" [v])")
				zero = append(zero, fmt.Sprintf("(set! %s nil)", m))
				ctor = append(ctor, "nil")
			}
			meths = append(meths, fmt.Sprintf("(%s [_] %s) (set_%s [_ v__] (set! %s v__) nil)", m, m, m, m))
			set = append(set, fmt.Sprintf("(set! %s (.%s o__))", m, m))
			eq = append(eq, eqForm(m, fmt.Sprintf("(.%s o__)", m), fl.Type(), ft, f))
		}
	}
	iface = append(iface, "(copy [])", "(^boolean eq [o])")
	var b strings.Builder
	what := "struct"
	if t.Kind() == cc.Union {
		what = "union: every member its own field"
	}
	fmt.Fprintf(&b, ";; C %s %s\n", what, typeName(t))
	fmt.Fprintf(&b, "(definterface I_%s\n  %s)\n", name, strings.Join(iface, "\n  "))
	fmt.Fprintf(&b, "(deftype %s [%s]\n  I_%s\n", name, strings.Join(fields, "\n    "), name)
	for _, m := range meths {
		b.WriteString("  " + m + "\n")
	}
	fmt.Fprintf(&b, "  (copy [this__] (.set ^%s (new-%s) this__))\n", name, name)
	eqBody := "(throw (UnsupportedOperationException. \"eq\"))"
	if j.needEq[name] {
		eqBody = "true"
		if len(eq) > 0 {
			eqBody = "(and " + strings.Join(eq, "\n      ") + ")"
		}
		eqBody = fmt.Sprintf("(let [^%s o__ o__]\n      %s)", name, eqBody)
	}
	fmt.Fprintf(&b, "  (eq [this__ o__] %s)\n", eqBody)
	fmt.Fprintf(&b, "  Struct\n  (set [this__ o__]\n    (let [^%s o__ o__]\n      %s)\n    this__)\n", name, strings.Join(append(set, "nil"), "\n      "))
	fmt.Fprintf(&b, "  (zero [this__]\n    %s\n    this__))\n", strings.Join(append(zero, "nil"), "\n    "))
	fmt.Fprintf(&b, "(defn new-%s ^%s []\n  (%s. %s))\n", name, name, name, strings.Join(ctor, " "))
	fmt.Fprintf(&b, "(defn array-%s ^objects [^long n]\n  (let [a (object-array n)]\n    (dotimes [k n] (aset a k (new-%s)))\n    a))\n\n", name, name)
	return b.String()
}

// boxZero is what a box of a C scalar or pointer of type t holds as zero.
func boxZero(t cc.Type) string {
	if k, ok := scalarKind(t); ok {
		if k.boolean {
			return "false"
		}
		return storeK("0", k)
	}
	return "nil"
}

// copyForm copies a value of C type t from src into dst, which exists.
func copyForm(dst, src string, t cc.Type, jt string, f *cfn) string {
	switch t.Kind() {
	case cc.Struct, cc.Union:
		if !strings.HasPrefix(dst, "^") {
			dst = "^" + jt + " " + dst
		}
		return "(.set " + dst + " " + src + ")"
	case cc.Array:
		at := t.(*cc.ArrayType)
		switch at.Elem().Kind() {
		case cc.Struct, cc.Union, cc.Array:
			k := f.tmpName("k")
			ej := elemJ(jt)
			return fmt.Sprintf("(dotimes [%s %d] %s)", k, at.Len(),
				copyForm("^"+cljHint(ej)+" (aget ^objects "+dst+" "+k+")", "(aget ^objects "+src+" "+k+")", at.Elem(), ej, f))
		}
		return fmt.Sprintf("(System/arraycopy %s 0 %s 0 %d)", src, dst, at.Len())
	}
	return "(set! " + dst + " " + src + ")"
}

// eqForm is the test that two values of C type t are equal: memcmp's answer
// on what C holds.
func eqForm(a, b string, t cc.Type, jt string, f *cfn) string {
	switch t.Kind() {
	case cc.Struct, cc.Union:
		return "(.eq ^" + jt + " " + a + " " + b + ")"
	case cc.Array:
		at := t.(*cc.ArrayType)
		k := f.tmpName("k")
		ej := elemJ(jt)
		h := cljHint(jt)
		return fmt.Sprintf("(every? (fn [^long %s] %s) (range %d))", k,
			eqForm("(aget ^"+h+" "+a+" "+k+")", "(aget ^"+h+" "+b+" "+k+")", at.Elem(), ej, f), at.Len())
	case cc.Bool:
		return "(= " + a + " " + b + ")"
	}
	if _, ok := scalarKind(t); ok {
		return "(== " + a + " " + b + ")"
	}
	switch {
	case scalarPtrElem(jt) != "":
		return "(" + jt + "/eq " + a + " " + b + ")"
	case strings.HasPrefix(jt, "Ptr<"):
		return "(Ptr/eq " + a + " " + b + ")"
	}
	return "(identical? " + a + " " + b + ")"
}
