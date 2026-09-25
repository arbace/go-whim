package xform

import "github.com/arbace/go-whim/internal/cc"

// Terminates says control never runs off the end of the block item: a
// statement that does not (StmtTerminates).  A nil item does not.
func Terminates(it *cc.BlockItem) bool {
	return it != nil && it.Case == cc.BlockItemStmt && StmtTerminates(it.Statement)
}

// StmtTerminates is C's analogue of Go's terminating statement: a jump
// (return, goto, break, continue), an if whose two branches both terminate,
// or a block whose last item does.
func StmtTerminates(s *cc.Statement) bool {
	switch s.Case {
	case cc.StatementJump:
		return true
	case cc.StatementCompound:
		var last *cc.BlockItem
		for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
			last = l.BlockItem
		}
		return Terminates(last)
	case cc.StatementSelection:
		sel := s.SelectionStatement
		return sel.Case == cc.SelectionStatementIfElse && StmtTerminates(sel.Statement) && StmtTerminates(sel.Statement2)
	}
	return false
}
