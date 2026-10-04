package graph

import (
	"fmt"
	"strconv"
)

// DeleteRowsAsWritten deletes the rows of the table def and says no
// position again: a subscript by a constant stays the number it is.  It is
// for a table whose positions the code reads as places, not as rows' names
// -- `options[0]` the first row, whichever it is, `&options[0]` where a
// walk begins -- which DeleteRows refuses where the row at such a position
// goes.  The table is typed as an import types it: the array of its new
// count, made and interned where the graph has none; where a row's
// position cannot be computed, its typed edge is cleared (Untyped).
func (e *Editor) DeleteRowsAsWritten(def *Node, rows []*Node) error {
	init := TableInit(def)
	if init == nil || !e.Live(def) {
		return fmt.Errorf("delete rows of #%d (%s): not an initialised definition in the graph", def.ID, label(def))
	}
	name := topName(def)
	t := defType(def)
	if !t.Is("array") {
		return fmt.Errorf("delete rows of %s: not an array", name)
	}
	if len(t.Kids) > 2 {
		return fmt.Errorf("delete rows of %s: its size is written; say it with its rows", name)
	}
	gone := map[*Node]bool{}
	for _, r := range rows {
		gone[r] = true
	}
	var order []*Node
	for _, r := range init.Args() {
		if gone[r] {
			delete(gone, r)
			continue
		}
		order = append(order, r)
	}
	for r := range gone {
		return fmt.Errorf("delete rows of %s: #%d (%s) is not one of its rows", name, r.ID, label(r))
	}
	var elem *Node
	if dt := def.Type; dt.Is("array") {
		elem = dt.Kids[len(dt.Kids)-1].Type
	}
	if err := e.spliceAs("rows", init, 1, len(init.Kids), order, placeItem); err != nil {
		return err
	}
	env := map[string]int64{}
	for n, v := range e.EnumValues() {
		env[EnumeratorName(n)] = v
	}
	pos, ok, err := rowPositions(order, env)
	known := err == nil && elem != nil
	var n int64
	for i := range pos {
		if !ok[i] {
			known = false
		}
		n = max(n, pos[i]+1)
	}
	if !known {
		if def.Type != nil {
			def.Type = nil
			e.untype(def)
		}
		return nil
	}
	tx := e.typeTx()
	def.Type = tx.array(strconv.FormatInt(n, 10), elem)
	tx.commit()
	e.typed(def)
	return nil
}
