package togo

// hsnamed.go is what the Haskell printer calls by the C's names where it
// printed numbers (doc/HASKELL-IDIOMS.md, item 3), and the segment's initial
// bytes as one image (item 7):
//
//   - a file-scope object is read, written and addressed by accessors of its
//     name -- `curwin ed'`, `set'curwin ed' p`, `addr'curwin ed'` -- where
//     `rdP (edSeg ed') 2768` was;
//   - a member's offset is a constant of its struct's and its own name,
//     `win_T'w_cursor`, where `16` was: `rdI64 wp (win_T'w_cursor +
//     pos_T'lnum)`;
//   - an enumerator is a pattern synonym of its name, `ESC`, in an
//     expression and in a case, where `(27 :: Int32)` was; a character
//     constant `ch 'x'` (a case label keeps the number, the character beside
//     it);
//   - initGlobals copies the image of the objects' constant initial values
//     into the segment, and writes only the addresses -- of strings, of
//     objects, of functions -- one by one.
//
// A definition is printed for the names the module uses: the accessors,
// offsets and patterns are found in the functions' text (hsTokens).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// hobj is a file-scope object's names.
type hobj struct {
	name     string
	off      int
	t        cc.Type
	exported bool // the host takes its address (Profile.HsExports)
}

// nameObjects names every object of the segment: a file-scope object by its
// C name, a block-scope static by its function's and its own.
func (h *hgen) nameObjects() {
	h.objs = map[string]*hobj{}
	taken := map[string]bool{}
	for _, n := range h.names {
		taken[n] = true
	}
	keys := make([]string, 0, len(h.segOff))
	for k := range h.segOff {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var n string
		switch {
		case strings.HasPrefix(k, "global:"):
			n = hsName(strings.TrimPrefix(k, "global:"))
		case strings.HasPrefix(k, "static:"):
			fn, v, _ := strings.Cut(strings.TrimPrefix(k, "static:"), ".")
			n = strings.TrimSuffix(hsName(fn), "'") + "'" + v
		default:
			continue
		}
		if taken[n] || hsReserved[n] {
			n = "g'" + n
		}
		taken[n] = true
		h.objs[k] = &hobj{name: n, off: h.segOff[k], t: h.segType[k]}
	}
}

// objAddr is the address of the object of key: the object itself, whole.
func (f *hfn) objAddr(key string) (haddr, bool) {
	o := f.h.objs[key]
	if o == nil {
		return haddr{}, false
	}
	return haddr{base: hv{val: "(addr'" + o.name + " ed')", ht: "P"}, obj: o, whole: true}, true
}

// structName is the name a struct's or a union's members are named after:
// its typedef's, else its tag's, one name for each type.
func (h *hgen) structName(t cc.Type) string {
	var tag string
	var first *cc.Field
	switch x := t.(type) {
	case *cc.StructType:
		tk := x.Tag()
		tag = tk.SrcStr()
		first = x.FieldByIndex(0)
	case *cc.UnionType:
		tk := x.Tag()
		tag = tk.SrcStr()
		first = x.FieldByIndex(0)
	default:
		return ""
	}
	if first == nil {
		return ""
	}
	key := fmt.Sprintf("%p", first) // a typedef's clone of the type shares its fields
	if n, ok := h.structNames[key]; ok {
		return n
	}
	n := h.tagTypedef[tag]
	if n == "" && t.Typedef() != nil {
		n = t.Typedef().Name()
	}
	if n == "" {
		n = tag
	}
	if n == "" {
		n = "anon"
	}
	if c := n[0]; c >= 'A' && c <= 'Z' || c == '_' && (len(n) == 1 || n[1] < 'a' || n[1] > 'z') {
		n = "c'" + n
	}
	base := n
	for i := 2; h.structTaken[n]; i++ {
		n = fmt.Sprintf("%s'%d", base, i)
	}
	h.structTaken[n] = true
	h.structNames[key] = n
	return n
}

// memberName is the constant of member fl's offset in container t, or ""
// when the member has no name to give it.
func (h *hgen) memberName(t cc.Type, fl *cc.Field) string {
	if fl == nil || fl.Name() == "" {
		return ""
	}
	if t != nil && (t.Kind() == cc.Ptr || t.Kind() == cc.Array) {
		t = elemOf(t)
	}
	s := h.structName(t)
	if s == "" {
		return ""
	}
	n := s + "'" + fl.Name()
	off := fl.Offset()
	if have, ok := h.memberOff[n]; ok && have != off {
		return "" // one name, two offsets: the number
	}
	h.memberOff[n] = off
	return n
}

// offStr is an address's offset as an argument: its members' names and the
// rest a number.
func offStr(a haddr) string {
	if len(a.syms) == 0 {
		return hsOff(a.off)
	}
	s := strings.Join(a.syms, " + ")
	switch r := a.off - a.symOff; {
	case r > 0:
		s += fmt.Sprintf(" + %d", r)
	case r < 0:
		s += fmt.Sprintf(" - %d", -r)
	}
	if len(a.syms) > 1 || a.off != a.symOff {
		s = "(" + s + ")"
	}
	return s
}

// hsConNames are the constructors in the module's scope, which no pattern
// may be.
var hsConNames = map[string]bool{"True": true, "False": true, "Ptr": true, "VI": true, "VU": true, "VP": true, "Ed": true,
	"IO": true, "Int": true, "Bool": true, "Array": true, "Dynamic": true}

// constSpelling is the C's spelling of a constant expression e of value v,
// when it has one: an enumerator's name (a pattern), a character constant
// `ch 'x'`; and whether the spelling is a name, which needs no parentheses.
func (h *hgen) constSpelling(e cc.ExpressionNode, v int64) (string, bool) {
	for {
		switch x := e.(type) {
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				return "", false
			}
			e = x.AssignmentExpression
			continue
		case *cc.ConstantExpression:
			e = x.ConditionalExpression
			continue
		case *cc.PrimaryExpression:
			switch x.Case {
			case cc.PrimaryExpressionExpr:
				e = x.ExpressionList
				continue
			case cc.PrimaryExpressionIdent:
				if n := h.enumName(x, v); n != "" {
					return n, true
				}
			case cc.PrimaryExpressionChar:
				if c, ok := hsCharLit(v); ok {
					return "ch " + c, false
				}
			}
		}
		return "", false
	}
}

// enumName is the pattern an enumerator x of value v is written as, or "".
func (h *hgen) enumName(x *cc.PrimaryExpression, v int64) string {
	en, ok := x.ResolvedTo().(*cc.Enumerator)
	if !ok {
		return ""
	}
	n := en.Token.SrcStr()
	if n == "" || n[0] < 'A' || n[0] > 'Z' || hsConNames[n] {
		return ""
	}
	if have, ok := h.enumVal[n]; ok && have != v {
		return ""
	}
	h.enumVal[n] = v
	return n
}

// hsCharLit is value v as a Haskell character literal, when it is a
// character a reader reads: printable ASCII, and the escapes C and Haskell
// share.
func hsCharLit(v int64) (string, bool) {
	switch {
	case v == '\'':
		return `'\''`, true
	case v == '\\':
		return `'\\'`, true
	case v == '\n':
		return `'\n'`, true
	case v == '\t':
		return `'\t'`, true
	case v == '\r':
		return `'\r'`, true
	case v >= 0x20 && v < 0x7f:
		return "'" + string(rune(v)) + "'", true
	}
	return "", false
}

// named is a constant of value v, type ht, spelled s (constSpelling).
func named(v int64, ht, s string, atom bool) hv {
	bare := s
	if !atom {
		bare = "(" + s + ")"
	}
	return hv{val: "(" + s + " :: " + ht + ")", bare: bare, ht: ht, konst: true, kv: v, spell: s, spellAtom: atom}
}

// hsLabel is a case value as a pattern: the enumerator its label names, or
// the number, the character beside it when the label is one.
func (h *hgen) hsLabel(v int64, ht string, label cc.ExpressionNode) string {
	num := hsPattern(v, ht)
	if label == nil {
		return num
	}
	ev, ok := intValue(label.Value())
	if !ok {
		return num
	}
	s, atom := h.constSpelling(label, ev)
	switch {
	case s == "":
		return num
	case atom && hsPattern(ev, ht) == num:
		return s
	case !atom:
		return num + " {- " + strings.TrimPrefix(s, "ch ") + " -}"
	}
	return num
}

// definitions are the accessors, the offsets and the patterns the module's
// text uses, each printed once; with the exported objects' addresses unless
// skipExported (the module prints them itself).
func (h *hgen) definitions(text string, skipExported bool) string {
	used := map[string]bool{}
	for _, t := range hsTokens(text) {
		used[t] = true
	}
	var b strings.Builder
	// the objects
	objs := make([]*hobj, 0, len(h.objs))
	for _, o := range h.objs {
		objs = append(objs, o)
	}
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].off < objs[j].off || objs[i].off == objs[j].off && objs[i].name < objs[j].name
	})
	b.WriteString("-- * The file-scope objects: each at its offset in the segment, read, written and addressed by name\n\n")
	for _, o := range objs {
		scalar := o.t != nil && o.t.Kind() != cc.Array && !isAggr(o.t) && o.t.Kind() != cc.Function
		ht := h.hsType(o.t)
		st := h.sigType(o.t)
		if strings.Contains(st, " ") {
			st = "(" + st + ")"
		}
		if scalar && used[o.name] {
			fmt.Fprintf(&b, "%s :: Ed -> IO %s\n%s ed' = rd%s (edSeg ed') %d\n{-# INLINE %s #-}\n", o.name, st, o.name, hsAccess(ht), o.off, o.name)
		}
		if scalar && used["set'"+o.name] {
			fmt.Fprintf(&b, "set'%s :: Ed -> %s -> IO ()\nset'%s ed' = wr%s (edSeg ed') %d\n{-# INLINE set'%s #-}\n", o.name, st, o.name, hsAccess(ht), o.off, o.name)
		}
		// an exported object's address the module may print itself, under
		// its C name; under its own, Defs prints it for the top to wrap
		dup := skipExported && h.exported[o.name]
		if !dup && (used["addr'"+o.name] || o.exported && !skipExported) {
			// an address of any type, as pAdd's: the C converts it freely
			fmt.Fprintf(&b, "addr'%s :: Ed -> Ptr a\naddr'%s ed' = pAdd (edSeg ed') %d\n{-# INLINE addr'%s #-}\n", o.name, o.name, o.off, o.name)
		}
	}
	// the members
	var ms []string
	for n := range h.memberOff {
		if used[n] {
			ms = append(ms, n)
		}
	}
	sort.Strings(ms)
	b.WriteString("\n-- * The members' offsets, as the C lays them out\n\n")
	for _, n := range ms {
		fmt.Fprintf(&b, "%s :: Int\n%s = %d\n", n, n, h.memberOff[n])
	}
	// the enumerators
	var es []string
	for n := range h.enumVal {
		if used[n] {
			es = append(es, n)
		}
	}
	sort.Strings(es)
	b.WriteString("\n-- * The C's named constants: patterns, of any integer type\n\n")
	for _, n := range es {
		fmt.Fprintf(&b, "pattern %s :: (Eq a, Num a) => a\npattern %s = %d\n", n, n, h.enumVal[n])
	}
	// the types: the functions' and the accessors' above
	for _, t := range hsTokens(b.String()) {
		used[t] = true
	}
	tys := h.typeDefs(used)
	sort.Strings(tys)
	b.WriteString("\n-- * What the pointers point at: the structs as types of their own, the C's typedefs as synonyms\n\n")
	for _, d := range tys {
		b.WriteString(d + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// poke writes the n bytes of v, little-endian, into the image at off.
func (f *hfn) poke(off, n int, v uint64) {
	for len(f.image) < off+n {
		f.image = append(f.image, 0)
	}
	for i := 0; i < n; i++ {
		f.image[off+i] = byte(v >> (8 * i))
	}
}

// imageLines copy the image's bytes into the segment: its runs of bytes
// that are not zero, as primitive string literals of at most 1 KB.
func imageLines(img []byte) []string {
	var out []string
	for i := 0; i < len(img); {
		if img[i] == 0 {
			i++
			continue
		}
		// a run: to the next 32 zero bytes, at most 1 KB
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
		out = append(out, fmt.Sprintf("copyMem (pAdd (edSeg ed') %d) %s %d", i, hsString(string(img[i:end])), end-i))
		i = end
	}
	return out
}
