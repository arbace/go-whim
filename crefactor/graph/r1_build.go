package graph

import "fmt"

// BUILD's operands as cc's check types them (doc/GRAPH-MIGRATION.md, *R1
// as built*): an operand of an array type -- a selection of an array
// member, an array's name -- is stored DECAYED, a pointer to its element,
// everywhere but under `&`; and a function's name a pointer to it.  BUILD
// typed them as declared, so a built `p->arr[0]` was not the import of its
// C view (its selection an array, the import's a pointer), which the
// re-check, typing only what is left untyped, did not mend.

// decayOperands types n's operands decayed: the lists (an identifier has
// no typed edge, its declaration has), a pointer type the graph lacks made.
func (b *builder) decayOperands(n *Node) {
	var tx *typeTx
	for _, k := range n.Kids[1:] {
		t := k.Type
		if !k.list || t == nil || !t.Is("array") && !t.Is("function") {
			continue
		}
		el := t
		if t.Is("array") {
			el = pointee(t)
		}
		p := pointerTo(b.e.g, el)
		if p == nil && el != nil {
			if tx == nil {
				tx = b.e.typeTx()
			}
			p = tx.pointer(el)
		}
		if p != nil {
			b.e.g.save(k)
			k.Type = p
		}
	}
	if tx != nil {
		tx.commit()
	}
}

// enumSize is an array's size written as an enumerator -- `db_line[DB_LINE_MAX]`
// -- as the decimal its value is, which is how the importer's type node
// says it; "" for any other size form (RETYPE and InsertMember refuse it,
// as before).
func (tx *typeTx) enumSize(n *Node) string {
	if n.list {
		return ""
	}
	d := n.Ref()
	if d == nil || !tx.e.IsEnumerator(d) {
		return ""
	}
	v, ok := tx.e.EnumValues()[d]
	if !ok || v < 0 {
		return ""
	}
	return fmt.Sprint(v)
}

// decayAt is t as the expression q is typed where it stands: an array a
// pointer to its element and a function a pointer to it, as cc's check
// stores an operand's type, everywhere but under `&` (through the
// parentheses and a comma's last operand between); RETYPE's re-derivation
// typed a selection of an array member as the member.
func (tx *typeTx) decayAt(q, t *Node) *Node {
	if t == nil || !t.Is("array") && !t.Is("function") {
		return t
	}
	n := q
	for {
		p := tx.e.Parent(n)
		if p == nil || IsStatement(p) {
			break
		}
		if p.Is("addr") {
			return t
		}
		if p.Is("paren") || p.Is("comma") && p.Kids[len(p.Kids)-1] == n {
			n = p
			continue
		}
		break
	}
	if t.Is("array") {
		return tx.pointer(pointee(t))
	}
	return tx.pointer(t)
}
