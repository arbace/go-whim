package graph

import "fmt"

// A PROTOTYPE'S PARAMETER NAMES (B3f).  Rename respells a parameter of a
// function's definition and its uses; a prototype names its parameters too,
// with names of its own that nothing uses.  RenamePrototypeParams respells
// the i-th one in every declaration of a function that is not its
// definition, as a text program rewriting the prototype's line did.

// RenamePrototypeParams names the i-th parameter to in every file-scope
// declaration of the function fn that is not its definition, and returns
// how many it respelled.  A declaration whose parameter has no name, or
// whose list names to already, is refused, and nothing changes; so is a
// name that is no identifier.
func (e *Editor) RenamePrototypeParams(fn string, i int, to string) (int, error) {
	if !isIdent(to) {
		return 0, fmt.Errorf("rename %s's parameter %d: `%s` is not an identifier", fn, i, to)
	}
	var atoms []*Node
	act := Act{Op: "rename"}
	for _, d := range e.FileDecls(fn) {
		if d.Is("defn") {
			continue
		}
		t := defType(d)
		if t == nil || !t.Is("fn") || len(t.Kids) < 2 || i < 0 || i >= len(t.Kids[1].Kids) {
			return 0, fmt.Errorf("rename %s's parameter %d: #%d declares no such parameter", fn, i, d.ID)
		}
		ps := t.Kids[1].Kids
		p := ps[i]
		if !p.list || len(p.Kids) < 2 || p.Kids[0].list {
			return 0, fmt.Errorf("rename %s's parameter %d: #%d does not name it", fn, i, d.ID)
		}
		for k, q := range ps {
			if k != i && q.list && len(q.Kids) >= 2 && !q.Kids[0].list && q.Kids[0].Atom == to {
				return 0, fmt.Errorf("rename %s's parameter %d: #%d names another parameter %s", fn, i, d.ID, to)
			}
		}
		atoms = append(atoms, p.Kids[0])
		act.Moved = append(act.Moved, p.ID)
	}
	if len(atoms) == 0 {
		return 0, fmt.Errorf("rename %s's parameter %d: no declaration of it but its definition", fn, i)
	}
	for _, a := range atoms {
		a.Atom = to
	}
	e.Log = append(e.Log, act)
	return len(atoms), nil
}
