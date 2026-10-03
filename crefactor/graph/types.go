package graph

import (
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// TYPE NODES.  A type is interned by its structure: one node for every
// `char *`, whatever the declarations that spell it.  A struct, union or
// enum is its definition's own form in the file -- the node its members
// hang from -- or, when the file has none, an external node.  Qualifiers
// and typedef names are the forms' business, not the type's: a typedef is
// its type.

// typeNode is t's node, or nil when cc gave no type.
func (im *importer) typeNode(t cc.Type) *Node {
	if t == nil || t.Kind() == cc.InvalidKind {
		return nil
	}
	if n, ok := im.typeOf[t]; ok {
		return n
	}
	var n *Node
	switch t.Kind() {
	case cc.Ptr:
		p, _ := t.(*cc.PointerType)
		if p == nil {
			return nil
		}
		elem := im.operand(p.Elem())
		n = im.intern("pointer", []string{"pointer"}, elem)
	case cc.Array:
		a, _ := t.(*cc.ArrayType)
		if a == nil {
			return nil
		}
		elem := im.operand(a.Elem())
		words := []string{"array"}
		if !a.IsIncomplete() && a.Len() >= 0 {
			words = append(words, strconv.FormatInt(a.Len(), 10))
		}
		n = im.intern(strings.Join(words, " "), words, elem)
	case cc.Function:
		f, _ := t.(*cc.FunctionType)
		if f == nil {
			return nil
		}
		params := NewList()
		key := []string{"function("}
		for _, p := range f.Parameters() {
			pt := im.operand(p.Type())
			params.Kids = append(params.Kids, &Node{Type: pt})
			key = append(key, im.keyOf[pt])
		}
		if f.IsVariadic() {
			params.Kids = append(params.Kids, NewAtom("..."))
			key = append(key, "...")
		}
		r := im.operand(f.Result())
		k := strings.Join(key, " ") + ")" + im.keyOf[r]
		if old, ok := im.typeKey[k]; ok {
			n = old
			break
		}
		n = NewList(NewAtom("function"), params, &Node{Type: r})
		im.keep(n, k)
	case cc.Struct, cc.Union:
		n = im.structType(t)
	case cc.Enum:
		n = im.enumType(t)
	default:
		words := append([]string{"basic"}, strings.Fields(t.Kind().String())...)
		n = im.intern(strings.Join(words, " "), words, nil)
	}
	im.typeOf[t] = n
	return n
}

// intern is the type node of these words and operand, made once.
func (im *importer) intern(key string, words []string, operand *Node) *Node {
	if operand != nil {
		key += " " + im.keyOf[operand]
	}
	if n, ok := im.typeKey[key]; ok {
		return n
	}
	n := NewList()
	for _, w := range words {
		n.Kids = append(n.Kids, NewAtom(w))
	}
	if operand != nil {
		n.Kids = append(n.Kids, &Node{Type: operand})
	}
	im.keep(n, key)
	return n
}

func (im *importer) keep(n *Node, key string) {
	im.typeKey[key] = n
	im.keyOf[n] = "{" + key + "}"
	im.g.Types = append(im.g.Types, n)
}

type fielder interface {
	NumFields() int
	FieldByIndex(int) *cc.Field
	Tag() cc.Token
}

// structType is a struct or union type's definition in the file -- found by
// a member's declarator, or by its tag -- or an external node.
func (im *importer) structType(t cc.Type) *Node {
	kw := "struct"
	if t.Kind() == cc.Union {
		kw = "union"
	}
	s, ok := t.(fielder)
	if !ok {
		return im.externTag(kw, "")
	}
	for i := 0; i < s.NumFields(); i++ {
		f := s.FieldByIndex(i)
		if f == nil || f.Declarator() == nil {
			continue
		}
		if m := im.decl[f.Declarator()]; m != nil {
			if def := im.owner[m]; def != nil {
				im.named(def)
				return def
			}
		}
	}
	tok := s.Tag()
	tag := tok.SrcStr()
	if defs := im.tagDefs[kw+" "+tag]; tag != "" && len(defs) > 0 {
		im.named(defs[0])
		return defs[0]
	}
	if tag == "" && t.Typedef() != nil {
		tag = t.Typedef().Name()
	}
	return im.externTag(kw, tag)
}

// named records a definition form as a type node's.
func (im *importer) named(def *Node) {
	if _, ok := im.keyOf[def]; !ok {
		im.nstruct++
		im.keyOf[def] = "{def " + strconv.Itoa(im.nstruct) + "}"
	}
}

// enumType is an enum type's definition, found by its enumerators or its
// tag, or an external node.
func (im *importer) enumType(t cc.Type) *Node {
	e, ok := t.(*cc.EnumType)
	if !ok {
		return im.externTag("enum", "")
	}
	for _, x := range e.Enumerators() {
		if m := im.decl[x]; m != nil {
			if def := im.owner[m]; def != nil {
				im.named(def)
				return def
			}
		}
	}
	tok := e.Tag()
	tag := tok.SrcStr()
	if defs := im.tagDefs["enum "+tag]; tag != "" && len(defs) > 0 {
		im.named(defs[0])
		return defs[0]
	}
	return im.externTag("enum", tag)
}

// EXTERNAL NODES: what the headers (or the front end's builtins) declare.

// extern is the external node of a header's declaration.
func (im *importer) extern(d cc.Node, name string) *Node {
	head := "extern"
	var t cc.Type
	switch x := d.(type) {
	case *cc.Declarator:
		if x.IsTypename() {
			head = "extern-typedef"
		}
		t = x.Type()
	case *cc.Enumerator:
		head = "extern-enumerator"
		t = x.Type()
	}
	key := head + " " + name
	if n := im.externs[key]; n != nil {
		return n
	}
	n := NewList(NewAtom(head), NewAtom(name))
	im.externs[key] = n
	im.g.Externs = append(im.g.Externs, n)
	n.Type = im.typeNode(t)
	return n
}

// externTag is a struct, union or enum the file uses and does not define.
func (im *importer) externTag(kw, tag string) *Node {
	key := "extern-" + kw + " " + tag
	if n := im.externs[key]; n != nil {
		return n
	}
	n := NewList(NewAtom("extern-" + kw))
	if tag != "" {
		n.Kids = append(n.Kids, NewAtom(tag))
	}
	im.externs[key] = n
	im.keyOf[n] = "{" + key + "}"
	im.g.Externs = append(im.g.Externs, n)
	return n
}

// externMember is the member of an external struct or union the file
// names: `(member NAME)` in it, typed.
func (im *importer) externMember(s *Node, name string, t cc.Type) *Node {
	for _, m := range s.Kids {
		if m.Is("member") && m.Kids[1].Atom == name {
			return m
		}
	}
	m := NewList(NewAtom("member"), NewAtom(name))
	s.Kids = append(s.Kids, m)
	m.Type = im.typeNode(t)
	return m
}

// undeclared is the external node of a name nothing declares: kind is
// `undeclared` (an identifier or typedef name), `undeclared-label` or
// `unresolved-member` (a member whose selection did not type).
func (im *importer) undeclared(kind, name string) *Node {
	key := kind + " " + name
	if n := im.externs[key]; n != nil {
		return n
	}
	n := NewList(NewAtom(kind), NewAtom(name))
	im.externs[key] = n
	im.g.Externs = append(im.g.Externs, n)
	return n
}

// isUndeclared says t is an external node for what nothing declares.
func isUndeclared(t *Node) bool {
	switch t.Head() {
	case "undeclared", "undeclared-label", "unresolved-member":
		return true
	}
	return false
}

// isExtern says t is an external node.
func isExtern(t *Node) bool {
	return strings.HasPrefix(t.Head(), "extern") || isUndeclared(t) || t.Is("member")
}

// operand is t's node inside another type: `(basic invalid)` where cc gave
// none, so that every operand is an edge.
func (im *importer) operand(t cc.Type) *Node {
	if n := im.typeNode(t); n != nil {
		return n
	}
	return im.intern("basic invalid", []string{"basic", "invalid"}, nil)
}
