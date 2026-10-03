package graph

// TERMINATES AND THE STATEMENTS AFTER A JUMP, on nodes: crefactor/xform's
// terminates.go and deadstmt.go asked of the graph (doc/GRAPH-MIGRATION.md,
// B1c).  The text rule walks cc's compound statements and their block
// items; here a block's items are C-lisp's, where a label is an item of its
// own before the statement it labels (`(case 1) (return x)` is cc's one
// labeled item `case 1: return x;`).  So what cc calls an item is a run of
// labels and the statement after them, and the rule reads it that way: a
// statement after a label is a labeled statement, which does not terminate,
// and a run of dead statements ends at the first label.

// StmtTerminates is C's analogue of Go's terminating statement: a jump
// (return, goto, goto *, break, continue), an if whose two branches both
// terminate, or a block whose last item does -- crefactor/xform's
// StmtTerminates on a node.
func StmtTerminates(s *Node) bool {
	switch {
	case s == nil || !s.list:
		return false
	case jumps(s):
		return true
	case s.Is("block"):
		items := blockItems(s)
		return ItemTerminates(items, len(items)-1)
	case s.Is("if"):
		return len(s.Kids) == 4 && StmtTerminates(s.Kids[2]) && StmtTerminates(s.Kids[3])
	}
	return false
}

// ItemTerminates says control never runs off the end of items[i], one of a
// block's items: a statement no label stands before (cc's labeled statement
// does not terminate) that terminates.  An index out of range does not.
func ItemTerminates(items []*Node, i int) bool {
	if i < 0 || i >= len(items) || labeledAt(items, i) {
		return false
	}
	return StmtTerminates(items[i])
}

// labeledAt says a label stands before items[i], attributes between.
func labeledAt(items []*Node, i int) bool {
	for k := i - 1; k >= 0; k-- {
		switch {
		case isLabelItem(items[k]):
			return true
		case !items[k].Is("stmt-attr"):
			return false
		}
	}
	return false
}

// isLabelItem says x is a label: a goto's or a switch's.
func isLabelItem(x *Node) bool { return x.Is("label") || isCaseLabel(x) }

// opensLabel says items[j] begins a labeled statement: a label, or the
// attributes before one.
func opensLabel(items []*Node, j int) bool {
	for ; j < len(items); j++ {
		switch {
		case isLabelItem(items[j]):
			return true
		case !items[j].Is("stmt-attr"):
			return false
		}
	}
	return false
}

// isDeclItem says x is a declaration among a block's items, not a
// statement.
func isDeclItem(x *Node) bool {
	switch x.Head() {
	case "def", "typedef", "static_assert", "declare", "macro-decl", "struct", "union", "enum":
		return true
	}
	return false
}

// itemLists calls f with each list of block items under n -- a block's, a
// statement expression's, a function body's -- outer before inner, in the
// file's order.
func itemLists(n *Node, f func(items []*Node)) {
	Walk(n, func(x *Node) bool {
		switch {
		case !x.list:
			return false
		case x.Is("block"):
			f(blockItems(x))
		case x.Is("stmt-expr"):
			f(x.Kids[1:])
		case x.Is("defn"):
			f(Body(x))
		}
		return true
	})
}

// DeadStmt deletes every run of statements no path reaches: those after a
// statement that always jumps (StmtTerminates), up to the next label, which
// is where a path can come in again.  A run holding a declaration is left
// -- a label after it may be reached with that name in scope -- and counted
// in held.  A run inside a run already deleted goes with it and is not
// counted: cut is the outermost runs.  crefactor/xform's DeadStmt, its
// numbers the same.
func (e *Editor) DeadStmt() (cut, held int, err error) {
	type run struct{ first, last *Node }
	var runs []run
	for _, f := range e.g.Forms {
		itemLists(f, func(items []*Node) {
			for i := 0; i < len(items); i++ {
				if !ItemTerminates(items, i) {
					continue
				}
				j := i + 1
				decl := false
				for j < len(items) && !opensLabel(items, j) {
					decl = decl || isDeclItem(items[j])
					j++
				}
				switch {
				case j == i+1:
					continue
				case decl:
					held++
				default:
					runs = append(runs, run{items[i+1], items[j-1]})
				}
				i = j - 1
			}
		})
	}
	// outer runs come first (the walk is preorder): one inside a run already
	// gone is gone with it
	for _, r := range runs {
		if !e.Live(r.first) {
			continue
		}
		if err := e.ReplaceRun(r.first, r.last); err != nil {
			return cut, held, err
		}
		cut++
	}
	return cut, held, nil
}
