package togo

// hstypes.go is what a pointer points at, in the Haskell's types (doc/
// HASKELL-IDIOMS.md, item 4): `Ptr Win_T` where every pointer was `Ptr ()`.
// A struct or a union is a phantom type, `data Win_T` -- no values, no cost
// at run time: the memory is still raw, and read and written at offsets --
// and a scalar the C names by a typedef is a synonym of its Haskell type,
// `type Char_u = Word8`, `type Linenr_T = Int64`, an enumeration's too,
// `type State_E = Int32` (item 10's part worth doing: the names in the
// types, the pattern synonyms in the values).  So GHC checks what the C
// checks: a pointer passed where another kind is wanted is an error, unless
// the C converts it, where the printer writes castPtr.  The runtime's
// readers and writers take a pointer of any type.

import (
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// isPtrHt says ht is a pointer's Haskell type.
func isPtrHt(ht string) bool { return ht == "P" || strings.HasPrefix(ht, "Ptr ") }

// ptrType is the Haskell type of a pointer, an array (its first element's
// address) or a function (an index in the table: a pointer to nothing).
func (h *hgen) ptrType(t cc.Type) string {
	if t.Kind() == cc.Function {
		return "Ptr ()"
	}
	return "Ptr " + h.pointee(elemOf(t))
}

// pointee is what a pointer to e points at, as a Haskell type argument.
func (h *hgen) pointee(e cc.Type) string {
	switch {
	case e == nil || e.Kind() == cc.Void || e.Kind() == cc.Function:
		return "()"
	case e.Kind() == cc.Ptr || e.Kind() == cc.Array:
		return "(" + h.ptrType(e) + ")"
	case isAggr(e):
		s := h.structName(e)
		if s == "" {
			return "()"
		}
		return h.typeName(strings.TrimPrefix(s, "c'"), "data %s")
	}
	return h.scalarName(e)
}

// scalarName is a scalar type as a signature says it: its typedef's name,
// a synonym, where the C gives it one.
func (h *hgen) scalarName(t cc.Type) string {
	base := hsScalarType(t)
	if td := t.Typedef(); td != nil && td.Name() != "" && !strings.HasPrefix(base, "?") {
		return h.typeName(td.Name(), "type %s = "+base)
	}
	return base
}

// sigType is a type as a signature says it: a scalar its typedef's
// synonym, a struct passed by value its address.
func (h *hgen) sigType(t cc.Type) string {
	switch {
	case t == nil || t.Kind() == cc.Void:
		return "()"
	case isAggr(t):
		return "Ptr " + h.pointee(t)
	case t.Kind() == cc.Ptr || t.Kind() == cc.Array || t.Kind() == cc.Function:
		return h.ptrType(t)
	}
	return h.scalarName(t)
}

// hsTypeReserved are the type names in the module's scope.
var hsTypeReserved = map[string]bool{"Int": true, "Int8": true, "Int16": true, "Int32": true, "Int64": true, "Word": true,
	"Word8": true, "Word16": true, "Word32": true, "Word64": true, "Bool": true, "Ptr": true, "IO": true, "Ed": true,
	"P": true, "Array": true, "Dynamic": true, "VArg": true, "Char": true, "Eq": true, "Num": true, "Raw": true}

// typeName is the Haskell type named for C's name c, defined as def says
// (a format of the name): a type name starts upper-case.
func (h *hgen) typeName(c, def string) string {
	if n, ok := h.tyOf[c+"\x00"+def]; ok {
		return n
	}
	n := c
	switch {
	case n == "":
		n = "Anon"
	case n[0] == '_':
		n = "T" + n
	case n[0] >= 'a' && n[0] <= 'z':
		n = strings.ToUpper(n[:1]) + n[1:]
	}
	for hsTypeReserved[n] || h.tyDef[n] != "" && h.tyDef[n] != strings.Replace(def, "%s", n, 1) {
		n += "'"
	}
	d := strings.Replace(def, "%s", n, 1)
	h.tyDef[n] = d
	h.tyOf[c+"\x00"+def] = n
	if strings.HasPrefix(d, "type ") {
		h.tySyn[n] = strings.TrimSpace(d[strings.Index(d, "=")+1:])
	}
	return n
}

var hsTyWordRe = regexp.MustCompile(`[A-Z][A-Za-z0-9_']*`)

// canon is a Haskell type with its synonyms expanded: two types are one when
// their canons are.
func (h *hgen) canon(ht string) string {
	return hsTyWordRe.ReplaceAllStringFunc(ht, func(w string) string {
		if b, ok := h.tySyn[w]; ok {
			return b
		}
		return w
	})
}

// typeDefs are the phantom types and the synonyms the module's text uses.
func (h *hgen) typeDefs(used map[string]bool) []string {
	var out []string
	for n, d := range h.tyDef {
		if used[n] {
			out = append(out, d)
		}
	}
	return out
}
