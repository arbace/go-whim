package cut

// optfront.go is the reform's D3 (doc/PIPELINE-REFORM.md §7): every options[]
// row the product has not, dropped on the seed in one cut, where 35 phases
// dropped them a few at a time.

import (
	_ "embed"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

//go:embed optfront.md
var optfrontList string

// optfrontNames is the fenced block of optfront.md: the rows to drop.
func optfrontNames() []string {
	parts := strings.Split(optfrontList, "```")
	if len(parts) < 3 {
		return nil
	}
	return strings.Fields(parts[1])
}

// OptFront drops every row optfront.md lists, and its modeline_whitelist[]
// entry.  No guard is asked: a global its row initialised is left at its
// static zero until the phase that removes its readers, which is what every
// drop without --strict did, and a buffer- or window-local option's field is
// its own phase's.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the rows are options[]'s
// elements whose first element is the name's string (DeleteRowsAsWritten:
// the text said no position again -- options[0] is the first row, whichever
// it is), and the names out of the lists are the elements that are
// that string alone of every declaration's outermost initialiser -- what
// the text's `"name",` beginning a line was, since the canonical print
// writes such a list one element per line (history keeps the text
// version).
func OptFront(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("optfront", e, w)
	names := optfrontNames()
	if len(names) == 0 {
		v.Die("optfront.md lists no rows")
		return v.Done()
	}
	want := map[string]bool{}
	for _, n := range names {
		want[`"`+n+`"`] = true
	}
	v.InTable("options", func(v *graph.Verbs) {
		byName := map[string]*graph.Node{}
		for _, r := range graph.TableInit(v.Scope()).Args() {
			if a := r.Args(); r.Is("init") && len(a) > 0 && !a[0].IsList() && want[a[0].Atom] {
				if _, ok := byName[a[0].Atom]; !ok {
					byName[a[0].Atom] = r
				}
			}
		}
		var rows []*graph.Node
		for _, n := range names {
			r := byName[`"`+n+`"`]
			if r == nil {
				v.Die("no options[] row for '%s'", n)
				return
			}
			rows = append(rows, r)
		}
		if err := e.DeleteRowsAsWritten(v.Scope(), rows); err != nil {
			v.Die("the rows -- %v", err)
		}
	})
	if v.Failed() {
		return v.Done()
	}
	// A name's line goes from every list, not only modeline_whitelist[]:
	// DropOptions has always removed `"name",` wherever it begins a line, and
	// so the value lists of other options lost the words they shared with a
	// dropped option ("key" from 'selectmode''s, "debug" from the history
	// names).  The product keeps that, so this does it the same way.
	whitelisted := 0
	for _, in := range v.Find("(init _*)") {
		def := e.Parent(in)
		if def == nil || graph.TableInit(def) != in {
			continue
		}
		var rows []*graph.Node
		for _, el := range in.Args() {
			if !el.IsList() && want[el.Atom] {
				rows = append(rows, el)
			}
		}
		if len(rows) == 0 {
			continue
		}
		if err := e.DeleteRowsAsWritten(def, rows); err != nil {
			v.Die("the names in %s -- %v", graph.DeclName(def), err)
			return v.Done()
		}
		whitelisted += len(rows)
	}
	v.Sayf("%d options[] rows dropped, %d of their names out of the lists that held them", len(names), whitelisted)
	return v.Done()
}
