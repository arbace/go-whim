package p164

// Whim phase 164 -- no statement follows a jump.  See GOAL.md.

import (
	"fmt"
	"io"
	"sort"

	"github.com/arbace/go-whim/internal/cc"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/sweep"
)

func init() { edit.Register("whim164", Edit) }

// Edit deletes every statement no path reaches: those after a statement that
// always jumps -- return, goto, break, continue, or an if whose branches both
// do, or a block that ends in one -- up to the next label or case, which is
// where a path can come in again.  The tree only LOCATES them (cc.Parse);
// the text is what is cut.
//
// A run holding a declaration is left: a label after it may be reached with
// that name in scope, and C lets a jump pass a declaration.  None does today,
// and the count says so.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "nodeadstmt", W: w}
	const path = "whim-vim.c"
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: text},
	})
	if err != nil {
		return nil, p.Die("the input does not parse: %v", err)
	}
	type cut struct{ a, z int }
	var cuts []cut
	held := 0
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		cs, ok := n.(*cc.CompoundStatement)
		if !ok || cs.Position().Filename != path {
			return true
		}
		var items []*cc.BlockItem
		for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
			items = append(items, l.BlockItem)
		}
		for i := 0; i < len(items); i++ {
			if !terminates(items[i]) {
				continue
			}
			j := i + 1
			decl := false
			for j < len(items) && !labeled(items[j]) {
				decl = decl || items[j].Case == cc.BlockItemDecl
				j++
			}
			if j == i+1 {
				continue
			}
			if decl {
				held++
				i = j - 1
				continue
			}
			a, _ := sweep.Span(items[i+1], path, text)
			_, z := sweep.Span(items[j-1], path, text)
			if z > a {
				cuts = append(cuts, cut{a, z})
			}
			i = j - 1
		}
		return true
	})
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].a > cuts[j].a })
	// A run inside a dead block is dead with it: keep only the outermost cuts.
	var outer []cut
	for i, c := range cuts {
		inner := false
		for k, d := range cuts {
			if k != i && d.a <= c.a && c.z <= d.z && (d.a != c.a || d.z != c.z) {
				inner = true
			}
		}
		if !inner {
			outer = append(outer, c)
		}
	}
	cuts = outer
	for _, c := range cuts {
		text = append(append([]byte{}, text[:c.a]...), text[c.z:]...)
	}
	p.Say(fmt.Sprintf("%d runs of statements after a jump go; %d held for a declaration", len(cuts), held))
	return text, nil
}

func labeled(it *cc.BlockItem) bool {
	return it.Case == cc.BlockItemStmt && it.Statement.Case == cc.StatementLabeled
}

func terminates(it *cc.BlockItem) bool {
	return it.Case == cc.BlockItemStmt && stmtTerminates(it.Statement)
}

// stmtTerminates is C's analogue of Go's terminating statement: control never
// runs off its end.
func stmtTerminates(s *cc.Statement) bool {
	switch s.Case {
	case cc.StatementJump:
		return true
	case cc.StatementCompound:
		var last *cc.BlockItem
		for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
			last = l.BlockItem
		}
		return last != nil && terminates(last)
	case cc.StatementSelection:
		sel := s.SelectionStatement
		return sel.Case == cc.SelectionStatementIfElse && stmtTerminates(sel.Statement) && stmtTerminates(sel.Statement2)
	}
	return false
}
