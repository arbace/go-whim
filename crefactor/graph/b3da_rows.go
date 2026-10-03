package graph

import "strconv"

// ArrangeRowsTyped is ArrangeRows that leaves the table and its new rows
// typed as an import of the result types them: where the arrangement gives
// the table a length the graph holds no array type for (ArrangeRows then
// clears its typed edge and lists it in Untyped), the type `(array N
// @:elem)` is made and interned; and a new row's element that is an integer
// constant spelled with a sign, `(- K)`, is given its literal's type (int,
// long, unsigned... by K's spelling and size), as cc types it.  Elements of
// other shapes are as BuildRows left them.
func (e *Editor) ArrangeRowsTyped(def *Node, order []*Node, ix RowIndex) (*RowsDone, error) {
	var elem *Node
	if t := def.Type; t.Is("array") {
		elem = t.Kids[len(t.Kids)-1].Type
	}
	done, err := e.ArrangeRows(def, order, ix)
	if err != nil {
		return nil, err
	}
	for _, r := range done.Inserted {
		Walk(r, func(n *Node) bool {
			if n.Type == nil && n.Is("-") && len(n.Kids) == 2 && !n.Kids[1].list {
				if t := e.literalType(n.Kids[1].Atom); t != nil {
					n.Type = t
					e.typed(n)
				}
			}
			return true
		})
	}
	if def.Type == nil && elem != nil {
		rows := TableInit(def).Args()
		vals := e.EnumValues()
		env := map[string]int64{}
		for n, v := range vals {
			env[EnumeratorName(n)] = v
		}
		pos, ok, err := rowPositions(rows, env)
		if err != nil {
			return done, nil
		}
		var n int64
		for i := range pos {
			if !ok[i] {
				return done, nil
			}
			n = max(n, pos[i]+1)
		}
		tx := e.typeTx()
		def.Type = tx.array(strconv.FormatInt(n, 10), elem)
		tx.commit()
		e.typed(def)
	}
	return done, nil
}

// literalType is the graph's basic type of an integer literal, by C's rule
// for its spelling and value, or nil.
func (e *Editor) literalType(tok string) *Node {
	_, xt, ok := litType(tok)
	if !ok || xt.ptr || xt.float || xt.size == 0 {
		return nil
	}
	var words []string
	switch {
	case xt.size == 4 && xt.signed:
		words = []string{"int"}
	case xt.size == 4:
		words = []string{"unsigned"}
	case xt.size == 8 && xt.signed:
		words = []string{"long"}
	case xt.size == 8:
		words = []string{"unsigned", "long"}
	default:
		return nil
	}
	tx := e.typeTx()
	t := tx.basic(words)
	tx.commit()
	return t
}
