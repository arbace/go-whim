package xform

import (
	"fmt"
	"io"
	"sort"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// DeadStmt is the step that deletes every statement no path reaches: those
// after a statement that always jumps -- return, goto, break, continue, or an
// if whose branches both do, or a block that ends in one -- up to the next
// label or case, which is where a path can come in again.  It takes no knobs
// and no arguments.
//
// A run holding a declaration is left: a label after it may be reached with
// that name in scope, and C lets a jump pass a declaration.  The report counts
// them.
func DeadStmt() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "nodeadstmt", W: w}
		if _, err := flags(p.Tag, args); err != nil {
			return nil, err
		}
		return deadStmt(p, text)
	}
}

func deadStmt(p edit.Ph, text []byte) ([]byte, error) {
	const path = file
	ast, err := parse(text)
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
			if !Terminates(items[i]) {
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
