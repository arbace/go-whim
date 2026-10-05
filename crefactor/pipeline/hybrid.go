package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
)

// THE HYBRID DRIVER.  A phase's steps are text steps and graph steps
// (Step.Graph), and the program is held as one or the other -- or as both,
// in step, at a phase's start:
//
//   - a text step runs on the text: when the graph is ahead of it, the
//     graph's C view first (cemit's canonical print, byte for byte), and the
//     graph is dropped after, the text being ahead;
//   - a graph step runs on the graph, through one editor: when there is
//     none, the text is imported first (crefactor/graph's Import: cc's
//     parse and type check, 2 to 3 times cc's parse);
//   - a `sweep` step is the collection when the program is held as a graph
//     (through the editor, so that the steps after it edit on the same
//     index: Editor.Collect), and crefactor/sweep's on the text otherwise;
//   - the phase's finish likewise: a phase that ends on the graph is
//     collected and printed by the C view -- no sweep, no canonical print --
//     and one that ends on text is swept and printed canonically.
//
// Between phases the text is always there (a boundary is C: the snapshots,
// the line counts), and the graph is handed on only to a phase that BEGINS
// ON THE GRAPH -- whose first step, sweeps aside, is a graph step -- so that
// a phase takes the same path whether it is run in order or from its
// snapshot in the parallel check: a phase that begins on text gets text
// either way.  The graph handed on gets a fresh editor (an editor's records
// -- what was written, emptied, removed -- belong to one phase's closure),
// and a whole run writes it beside the boundary it was printed as, qNNN.g,
// for the check to read back instead of importing (the read is a tenth of
// cc's parse).

// prog is the program as a phase holds it.
type prog struct {
	text []byte        // the text; nil while the graph is ahead of it
	ed   *graph.Editor // the graph, and the editor on it; nil while the text is ahead or there is none
	conv Conv
}

// Conv is what one phase spent between the two kinds, and what its graph
// steps did: the imports and C views made, the collections, and the
// editor's log -- its acts, the ids they superseded and the ids they gave.
type Conv struct {
	Imports, Views, Collections int
	Import, View, Collect       time.Duration
	Graph                       bool // the phase ran a graph step, or held a graph
	Acts, Gone, New             int
	FromSnapshot                bool // the check began it on the graph read from its snapshot
}

func (c Conv) String() string {
	if !c.Graph {
		return ""
	}
	var parts []string
	if c.FromSnapshot {
		parts = append(parts, "read from the store")
	}
	if c.Imports > 0 {
		parts = append(parts, fmt.Sprintf("%d import %s", c.Imports, ms(c.Import)))
	}
	if c.Views > 0 {
		parts = append(parts, fmt.Sprintf("%d C view %s", c.Views, ms(c.View)))
	}
	if c.Collections > 0 {
		parts = append(parts, fmt.Sprintf("%d collection %s", c.Collections, ms(c.Collect)))
	}
	parts = append(parts, fmt.Sprintf("%d acts, %d ids superseded, %d given", c.Acts, c.Gone, c.New))
	return "graph: " + strings.Join(parts, ", ")
}

func ms(d time.Duration) string { return fmt.Sprintf("%dms", d.Milliseconds()) }

// tally adds the editor's log to the phase's, once: an editor is tallied
// when it is dropped or the phase ends.
func (pr *prog) tally() {
	if pr.ed == nil {
		return
	}
	for _, a := range pr.ed.Log {
		pr.conv.Acts++
		pr.conv.Gone += len(a.Gone)
		pr.conv.New += len(a.New)
	}
	pr.ed.Log = nil
}

// editor is the graph, imported from the text when there is none.
func (c *Config) editor(pr *prog, scratch string) (*graph.Editor, error) {
	pr.conv.Graph = true
	if pr.ed != nil {
		return pr.ed, nil
	}
	start := time.Now()
	g, _, err := graph.Import(filepath.Join(scratch, c.WorkName), pr.text)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	pr.ed = graph.NewEditor(g)
	pr.conv.Imports++
	pr.conv.Import += time.Since(start)
	return pr.ed, nil
}

// textOf is the text, printed from the graph when the graph is ahead.
func (pr *prog) textOf() ([]byte, error) {
	if pr.text != nil {
		return pr.text, nil
	}
	start := time.Now()
	t, err := pr.ed.Graph().C()
	if err != nil {
		return nil, fmt.Errorf("the C view: %w", err)
	}
	pr.text = t
	pr.conv.Views++
	pr.conv.View += time.Since(start)
	return t, nil
}

// collect is the sweep on the graph, through its editor.
func (c *Config) collect(pr *prog, w io.Writer) error {
	start := time.Now()
	// the typed edges the phase's edits cleared or left stale, given back
	// (doc/GRAPH.md, step 6), so that what the phase hands on is what an
	// import of its C view would be
	rs := pr.ed.Recheck()
	st, err := pr.ed.Collect(c.Collect)
	if err != nil {
		return fmt.Errorf("collection: %w", err)
	}
	d := time.Since(start)
	pr.text = nil
	pr.conv.Collections++
	pr.conv.Collect += d
	fmt.Fprintf(w, "  collect      %s; recheck: %s; %dms\n", st, rs, d.Milliseconds())
	return nil
}

// BeginsOnGraph says the phase's first step, sweeps aside, is a graph step:
// the phase is handed the graph, in order and in the check alike.
func BeginsOnGraph(p Phase) bool {
	for _, s := range p.Steps {
		if s.Op != "sweep" {
			return s.Graph
		}
	}
	return false
}

// GRAPH SNAPSHOTS.  The graph a whole run handed from boundary NNN to the
// phase after it, which begins on the graph, is kept in SnapDir's store of
// graphs (graph.Store): its Lisp, headed by a comment naming the digest of
// qNNN.c, the text it was printed as, put as the graph qNNN -- the units of
// its Lisp, each once, and a manifest qNNN.gm of their keys.  The reader
// ignores the comment; the check reads it first, and takes the graph only
// when it is qNNN.c's.  A graph the store cannot give whole (a manifest, an
// index or a pack missing, cut short, or not the bytes put), or one of
// another text (left by a run of another plan, or of another input), is not
// read: the phase imports qNNN.c instead, which proves the same thing more
// slowly, and the check says why.

const graphSnapHead = ";; the graph of q%03d.c, sha256 %s\n"

func graphSnapName(n int) string { return fmt.Sprintf("q%03d", n) }

func (o *Options) writeGraphSnap(n int, text []byte, g *graph.Graph) error {
	return o.store.Put(graphSnapName(n), fmt.Appendf(nil, graphSnapHead, n, digestOf(text)), g)
}

// readGraphSnap is boundary n's graph, when the store holds it and it is
// text's; nil and why otherwise (nil and nil: there is none).
func (c *Config) readGraphSnap(n int, text []byte) (*graph.Graph, error) {
	if c.SnapDir == "" {
		return nil, nil
	}
	return GraphSnapshotIn(c.SnapDir, n, text)
}

// GraphSnapshotIn is readGraphSnap on a directory of snapshots: boundary
// n's graph from dir's store, when it holds one and it is text's.
func GraphSnapshotIn(dir string, n int, text []byte) (*graph.Graph, error) {
	if !graph.StoreHas(dir, graphSnapName(n)) {
		return nil, nil
	}
	b, err := graph.ReadStoreLisp(dir, graphSnapName(n))
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(b, fmt.Appendf(nil, graphSnapHead, n, digestOf(text))) {
		return nil, fmt.Errorf("q%03d's graph is not q%03d.c's", n, n)
	}
	return graph.Read(b)
}
