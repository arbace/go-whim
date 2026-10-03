package graph

import (
	"fmt"
	"strings"
)

// TYPES UNDER AN EDIT THAT CHANGES ONE (doc/GRAPH-MIGRATION.md, B2c): what
// PARAM, RETYPE and MOVE share.  A declaration whose type an edit changes
// gets the type node its new form names -- found by structure among the
// graph's, or made and interned when the graph has none (a pointer, an
// array, a function type; never a basic type the file does not use) -- and
// the expressions above each of its uses are typed again where the type
// follows plainly from the operands (derive: a call its callee's result, a
// selection its member's, `&x`, `*p`, `p[i]`, an assignment, a comparison,
// a cast), and cleared and listed in Untyped where it does not: unknown,
// never wrong.  The walk up stops at the first expression whose type it
// finds unchanged, and at the statement.  No checker: step 6's.

// A typeTx is the type nodes an edit asks for: the graph's own when it
// holds one of that structure, otherwise a new node, PENDING until the edit
// commits -- so that an edit that refuses leaves the types section as it
// was, and two requests for one structure get one node.
type typeTx struct {
	e       *Editor
	keys    map[string]*Node // the graph's types by structure, built on first use
	pending []*Node

	// What formType says of the forms an edit is about to change: a fn
	// form with these parameters dropped, a typedef with this new type.
	drops   map[*Node][]int
	typedef func(d *Node) *Node
}

func (e *Editor) typeTx() *typeTx { return &typeTx{e: e} }

// internKey is a type node's structure: its words, and its operands by
// identity (a struct's or union's type is its form: identity too).
func internKey(n *Node) string {
	var b strings.Builder
	for _, k := range n.Kids {
		switch {
		case k.list: // a function's parameters
			b.WriteString("(")
			for _, p := range k.Kids {
				if p.Type != nil {
					fmt.Fprintf(&b, "%p ", p.Type)
				} else {
					b.WriteString(p.Atom + " ")
				}
			}
			b.WriteString(")")
		case k.Type != nil:
			fmt.Fprintf(&b, "@%p ", k.Type)
		default:
			b.WriteString(k.Atom + " ")
		}
	}
	return b.String()
}

// intern is the type node of n's structure: the graph's, an earlier
// request's, or n, pending.
func (tx *typeTx) intern(n *Node) *Node {
	if tx.keys == nil {
		tx.keys = map[string]*Node{}
		for _, t := range tx.e.g.Types {
			if t.list && t.Head() != "struct" && t.Head() != "union" && t.Head() != "enum" {
				k := internKey(t)
				if _, ok := tx.keys[k]; !ok {
					tx.keys[k] = t
				}
			}
		}
	}
	k := internKey(n)
	if t, ok := tx.keys[k]; ok {
		return t
	}
	tx.keys[k] = n
	tx.pending = append(tx.pending, n)
	return n
}

// pointer is `(pointer @:t)`.
func (tx *typeTx) pointer(t *Node) *Node {
	if t == nil {
		return nil
	}
	return tx.intern(NewList(NewAtom("pointer"), &Node{Type: t}))
}

// array is `(array N @:t)`, or `(array @:t)` for size "".
func (tx *typeTx) array(size string, t *Node) *Node {
	if t == nil {
		return nil
	}
	n := NewList(NewAtom("array"))
	if size != "" {
		n.Kids = append(n.Kids, NewAtom(size))
	}
	n.Kids = append(n.Kids, &Node{Type: t})
	return tx.intern(n)
}

// function is `(function (@:P ... [...]) @:R)`.
func (tx *typeTx) function(params []*Node, variadic bool, result *Node) *Node {
	if result == nil {
		return nil
	}
	ps := NewList()
	for _, p := range params {
		if p == nil {
			return nil
		}
		ps.Kids = append(ps.Kids, &Node{Type: p})
	}
	if variadic {
		ps.Kids = append(ps.Kids, NewAtom("..."))
	}
	return tx.intern(NewList(NewAtom("function"), ps, &Node{Type: result}))
}

// commit puts the pending nodes in the types section, each with a fresh id,
// and logs them as given.
func (tx *typeTx) commit() {
	if len(tx.pending) == 0 {
		return
	}
	act := Act{Op: "type"}
	for _, n := range tx.pending {
		n.up = tx.e.top[1]
		Walk(n, func(x *Node) bool {
			for _, k := range x.Kids {
				k.up = x
			}
			if x.list {
				tx.e.g.Fresh(x)
				act.New = append(act.New, x.ID)
			}
			return true
		})
		tx.e.g.Types = append(tx.e.g.Types, n)
	}
	tx.pending = nil
	tx.e.Log = append(tx.e.Log, act)
}

// funcParts are a function type node's parameters' types, whether it is
// variadic, and its result.
func funcParts(f *Node) (params []*Node, variadic bool, result *Node, ok bool) {
	if !f.Is("function") || len(f.Kids) != 3 {
		return nil, false, nil, false
	}
	for _, p := range f.Kids[1].Kids {
		if !p.list && p.Atom == "..." {
			variadic = true
			continue
		}
		params = append(params, p.Type)
	}
	return params, variadic, f.Kids[2].Type, true
}

// formType is the type node a declaration's type form names, the nodes the
// graph lacks made in tx: a typedef name its typedef's type, a tag its
// definition, a builtin's words the graph's basic type (nil when the file
// uses none of them), a pointer, an array of a decimal size or none, a
// function.  A parameter's type is adjusted (param): an array is a pointer
// to its element, a function a pointer to it.  nil when the form says what
// it does not follow (typeof, an array of a computed size).
func (tx *typeTx) formType(t *Node, param bool) *Node {
	n := tx.formType1(t)
	if param && n != nil {
		switch {
		case n.Is("array"):
			n = tx.pointer(pointee(n))
		case n.Is("function"):
			n = tx.pointer(n)
		}
	}
	return n
}

func (tx *typeTx) formType1(t *Node) *Node {
	if t == nil {
		return nil
	}
	if !t.list {
		if d := t.Ref(); d != nil {
			if tx.typedef != nil {
				if n := tx.typedef(d); n != nil {
					return n
				}
			}
			return d.Type // a typedef is its type
		}
		return tx.basic([]string{t.Atom})
	}
	switch h := t.Head(); h {
	case "struct", "union", "enum":
		if len(t.Kids) == 2 {
			return t.Kids[1].Ref()
		}
		return nil
	case "ptr":
		return tx.pointer(tx.formType1(t.Kids[1]))
	case "paren", "name-attr":
		return tx.formType1(t.Kids[1])
	case "array":
		elem := tx.formType1(t.Kids[len(t.Kids)-1])
		switch {
		case len(t.Kids) == 2:
			return tx.array("", elem)
		case len(t.Kids) == 3 && !t.Kids[1].list && isDecimalInt(t.Kids[1].Atom):
			return tx.array(t.Kids[1].Atom, elem)
		}
		return nil
	case "fn":
		var params []*Node
		variadic := false
		drop := map[int]bool{}
		for _, j := range tx.drops[t] {
			drop[j] = true
		}
		for j, p := range paramElems(t) {
			if drop[j] {
				continue
			}
			if !p.list && p.Atom == "..." {
				variadic = true
				continue
			}
			params = append(params, tx.formType(paramTypeForm(p), true))
		}
		// `(void)`, written or left by the drops, is one void parameter
		// in cc's type, which the importer interns: say it as it does
		if len(params) == 0 && !variadic && len(t.Kids) >= 2 && t.Kids[1].list &&
			(len(t.Kids[1].Kids) == 1 && !t.Kids[1].Kids[0].list && t.Kids[1].Kids[0].Atom == "void" || len(paramElems(t)) > 0) {
			params = append(params, tx.formType(NewAtom("void"), true))
		}
		return tx.function(params, variadic, tx.formType1(t.Kids[2]))
	case "fn-ids", "typeof", "typeof-type", "typeof_unqual", "typeof_unqual-type", "__typeof__", "__typeof__-type",
		"atomic", "_BitInt":
		return nil
	}
	// specifiers: qualifiers aside, a builtin's words or one typedef name
	var words []string
	var named *Node
	for _, k := range t.Kids {
		switch {
		case k.list:
			if isAttrForm(k) {
				continue
			}
			if len(t.Kids) == 1 || named != nil || len(words) > 0 {
				return nil
			}
			named = k // one specifier's form: a tag
		case k.Atom == "const" || k.Atom == "volatile" || k.Atom == "restrict" || k.Atom == "spec" ||
			k.Atom == "__restrict" || k.Atom == "__restrict__" || prefixWords[k.Atom]:
		case k.Ref() != nil:
			named = k
		default:
			words = append(words, k.Atom)
		}
	}
	if named != nil {
		if len(words) > 0 {
			return nil
		}
		return tx.formType1(named)
	}
	return tx.basic(words)
}

// basic is the `(basic ...)` node of the specifiers spelled: the graph's,
// or made.
func (tx *typeTx) basic(spelled []string) *Node {
	if t := basicType(tx.e.g, spelled); t != nil {
		return t
	}
	words := basicWords(spelled)
	if len(words) == 0 {
		return nil
	}
	n := NewList(NewAtom("basic"))
	for _, w := range words {
		n.Kids = append(n.Kids, NewAtom(w))
	}
	return tx.intern(n)
}

// completed is t, an array of no size, given old's size where old is the
// array its declaration's initialiser completed: `T a[] = {...}`.
func (tx *typeTx) completed(t, old *Node) *Node {
	if t.Is("array") && len(t.Kids) == 2 && old.Is("array") && len(old.Kids) == 3 {
		return tx.array(old.Kids[1].Atom, pointee(t))
	}
	return t
}

// paramElems are a fn form's parameters: the elements of its list, `void`
// alone being none.
func paramElems(fn *Node) []*Node {
	if len(fn.Kids) < 2 {
		return nil
	}
	ps := fn.Kids[1].Kids
	if len(ps) == 1 && !ps[0].list && ps[0].Atom == "void" {
		return nil
	}
	return ps
}

// paramTypeForm is a parameter's type form: `(NAME TYPE ATTR...)`'s TYPE,
// `(TYPE ATTR...)`'s TYPE, a bare atom itself.
func paramTypeForm(p *Node) *Node {
	switch {
	case !p.list:
		return p
	case len(p.Kids) >= 2 && !p.Kids[0].list && !isTypeWordAtom(p.Kids[0]) && !isAttrForm(p.Kids[1]):
		return p.Kids[1]
	case len(p.Kids) >= 1:
		return p.Kids[0]
	}
	return nil
}

// paramName is a named parameter's name, or "".
func paramName(p *Node) string {
	if p.list && len(p.Kids) >= 2 && !p.Kids[0].list && !isTypeWordAtom(p.Kids[0]) && !isAttrForm(p.Kids[1]) {
		return p.Kids[0].Atom
	}
	return ""
}

// isTypeWordAtom says an atom at a parameter's head is a type, not its
// name: a keyword, or a typedef name (an atom with an edge).
func isTypeWordAtom(a *Node) bool { return keywords[a.Atom] || a.Ref() != nil }

// derive is the type an expression form has from its operands' where that
// is plain, made in tx when the graph lacks it; nil when it is not plain.
func (tx *typeTx) derive(n *Node) *Node {
	args := n.Args()
	switch h := n.Head(); h {
	case "->", ".":
		if r := args[len(args)-1].Ref(); r != nil {
			return r.Type
		}
		return nil
	case "cast":
		if len(args) == 2 {
			return tx.formType(args[0], false)
		}
		return nil
	case "addr":
		if t := typeOf(args[0]); t != nil {
			return tx.pointer(t)
		}
		return nil
	case "deref":
		t := typeOf(args[0])
		if t.Is("function") {
			return t // *f of a function is the function
		}
		return pointee(t)
	case "call":
		return resultType(typeOf(args[0]))
	case "sizeof", "sizeof-bare", "sizeof-type", "alignof", "alignof-bare", "alignof-type":
		return n.Type // a size, whatever its operand
	}
	if n.Head() == "" {
		return nil
	}
	return exprType(tx.e, n)
}

// rederive types again the expressions from n up, to the statement: each
// derived where that is plain, else cleared and listed in Untyped; it stops
// at the first one whose type it finds unchanged.
func (tx *typeTx) rederive(n *Node) { tx.rederiveCount(n) }

// Rederive types again the expressions from n up to the statement where
// the type follows plainly from the operands (a call its callee's result,
// a selection its member's, `&x`, `*p`, `p[i]`, an assignment, a
// comparison, a cast), taking them off Untyped, and clears and lists the
// others; it stops at the first it finds unchanged, and says how many it
// changed.  After a Replace that cleared the types above it (one the
// editor could not tell was of the same type), it gives back what follows.
func (e *Editor) Rederive(n *Node) int {
	tx := e.typeTx()
	k := tx.rederiveCount(n)
	tx.commit()
	return k
}

// isTypeContext says a list is a type form (what a declaration or a cast
// spells), whose nodes carry no typed edge.
func isTypeContext(q *Node) bool {
	switch q.Head() {
	case "ptr", "array", "fn", "paren", "spec", "name-attr", "struct", "union", "enum":
		return q.Head() != "paren" || q.Type == nil
	}
	return false
}

// typed takes n off the Untyped list: it has its type again.
func (e *Editor) typed(n *Node) {
	for i, u := range e.Untyped {
		if u == n {
			e.Untyped = append(e.Untyped[:i], e.Untyped[i+1:]...)
			return
		}
	}
}

// typedAll takes n and everything under it off the Untyped list: a type
// form a splice listed, which carries no typed edge by its nature.
func (e *Editor) typedAll(n *Node) {
	in := map[*Node]bool{}
	Walk(n, func(x *Node) bool { in[x] = true; return true })
	out := e.Untyped[:0]
	for _, u := range e.Untyped {
		if !in[u] {
			out = append(out, u)
		}
	}
	e.Untyped = out
}

// isParamList says p is a fn form's parameter list.
func isParamList(p *Node) bool {
	return p != nil && p.up != nil && p.up.Is("fn") && len(p.up.Kids) > 1 && p.up.Kids[1] == p
}

// isMemberForm says p is a member of a struct or union definition.
func isMemberForm(e *Editor, p *Node) bool {
	q := e.Parent(p)
	if q == nil || !(q.Is("struct") || q.Is("union")) {
		return false
	}
	for _, m := range members(q) {
		if m == p {
			return true
		}
	}
	return false
}
