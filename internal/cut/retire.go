package cut

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// cmdRows indexes cmdnames[]'s rows by the command each names: the string
// a row begins with, `(cast (ptr char_u) "name")`, its length said
// `sizeof("name") - 1` or a number.  A name two rows give is refused.
func cmdRows(e *graph.Editor) (map[string][]*graph.Node, []*graph.Node, error) {
	var init *graph.Node
	for _, d := range e.FileDecls("cmdnames") {
		if i := graph.TableInit(d); i != nil {
			init = i
		}
	}
	if init == nil {
		return nil, nil, fmt.Errorf("cmdnames[] is not an initialised table")
	}
	by := map[string][]*graph.Node{}
	var rows []*graph.Node
	for _, r := range init.Args() {
		name := cmdRowName(r)
		if name == "" {
			return nil, nil, fmt.Errorf("a cmdnames[] row does not have the expected shape")
		}
		by[name] = append(by[name], r)
		rows = append(rows, r)
	}
	return by, rows, nil
}

// cmdRowInit is a row's (init ...), under its designator.
func cmdRowInit(r *graph.Node) *graph.Node {
	if r.Is("at") && len(r.Args()) == 2 {
		r = r.Args()[1]
	}
	if !r.Is("init") || len(r.Args()) != 5 {
		return nil
	}
	return r
}

// cmdRowName is the command a row names, "" when it is not a row's shape.
func cmdRowName(r *graph.Node) string {
	in := cmdRowInit(r)
	if in == nil {
		return ""
	}
	s := in.Args()[0]
	if !s.Is("cast") || len(s.Args()) != 2 || s.Args()[1].IsList() || !strings.HasPrefix(s.Args()[1].Atom, `"`) {
		return ""
	}
	return strings.Trim(s.Args()[1].Atom, `"`)
}

// cmdRowHandler is the handler a row names: its third element.
func cmdRowHandler(r *graph.Node) *graph.Node { return cmdRowInit(r).Args()[2] }

// ExFront points every row phase 1 declares (internal/phase/001/delta.md,
// handed as REMOVED) at ex_ni, so the command answers "not implemented":
// the 489 commands the product has not, 271 of them stubs in the seed
// already.  A row ex_script_ni or ex_ni already is a stub as it stands and
// is counted apart; a name with no row, or two, is refused, because a phase
// that retires nothing still reports success otherwise.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the rows found by the string
// they begin with, each handler replaced by a use of ex_ni, its typed edge
// kept (history keeps the text version, Retire's regexps).
func ExFront(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("exfront", e, w)
	names := strings.Fields(os.Getenv("REMOVED"))
	if len(names) == 0 {
		return fmt.Errorf("exfront: no rows declared")
	}
	by, _, err := cmdRows(e)
	if err != nil {
		return fmt.Errorf("exfront: %v", err)
	}
	var exNi *graph.Node
	if ds := e.FileDecls("ex_ni"); len(ds) > 0 {
		exNi = ds[0]
	} else {
		return fmt.Errorf("exfront: ex_ni is not declared")
	}
	done, already := 0, 0
	for _, n := range names {
		rows := by[n]
		if len(rows) == 0 {
			return fmt.Errorf("retire: no row for :%s -- the table has moved, and a phase that "+
				"retires nothing still reports success", n)
		}
		h := cmdRowHandler(rows[0])
		if h.Atom == "ex_ni" || h.Atom == "ex_script_ni" {
			already++
			continue
		}
		if len(rows) != 1 {
			return fmt.Errorf("retire: %d rows for :%s, expected one", len(rows), n)
		}
		use := e.RefTo(exNi)
		use.Type = h.Type
		if err := e.Replace(h, use); err != nil {
			return fmt.Errorf("exfront: :%s's handler -- %v", n, err)
		}
		done++
	}
	v.Sayf("%d commands retired, %d were stubs already", done, already)
	return v.Done()
}
