package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
)

// runGraphBoundary is `whim graph --changes N` and `whim graph --snapshot N`
// (args after the flag): the boundaries' graphs from the store of graphs in
// .cache/boundaries (--dir D another), each imported from its qNNN.c where
// the store holds none of that text.
//
//	whim graph --changes N [--nominal] [--dir D]   what phase N changed: the
//	        top-level forms and external nodes whose content hash moved
//	        from q(N-1) to qN, added, removed and changed; --nominal by the
//	        nominal hashes, a type's definition by its name
//	whim graph --snapshot N [-o OUT] [--dir D]     boundary N's graph as Lisp,
//	        as the store holds it (the qNNN.g of before the store)
func runGraphBoundary(mode string, args []string) int {
	dir, out, nominal, n := ".cache/boundaries", "", false, -1
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--nominal" && mode == "--changes":
			nominal = true
		case (a == "--dir" || a == "-o" && mode == "--snapshot") && i+1 < len(args):
			i++
			if a == "--dir" {
				dir = args[i]
			} else {
				out = args[i]
			}
		default:
			v, err := strconv.Atoi(a)
			if err != nil || n >= 0 || v < 0 {
				return graphUsage()
			}
			n = v
		}
	}
	if n < 0 || mode == "--changes" && n == 0 {
		return graphUsage()
	}
	fail := func(err error) int {
		fmt.Fprintf(os.Stderr, "  graph        %v\n", err)
		return 1
	}
	if mode == "--snapshot" {
		b, err := graph.ReadStoreLisp(dir, fmt.Sprintf("q%03d", n))
		if err != nil {
			return fail(err)
		}
		return emit(out, b)
	}
	start := time.Now()
	a, how, err := boundaryGraph(dir, n-1)
	if err != nil {
		return fail(err)
	}
	b, how2, err := boundaryGraph(dir, n)
	if err != nil {
		return fail(err)
	}
	c, err := graph.Compare(a, b, graph.HashOptions{Nominal: nominal})
	if err != nil {
		return fail(err)
	}
	hashes := "content hashes"
	if nominal {
		hashes = "nominal hashes"
	}
	fmt.Printf("phase %d, q%03d (%s) to q%03d (%s), by %s: %d added, %d removed, %d changed, %d kept; %dms\n",
		n, n-1, how, n, how2, hashes, len(c.Added), len(c.Removed), len(c.Changed), c.Kept, time.Since(start).Milliseconds())
	for _, s := range []struct {
		what string
		l    []graph.Change
	}{{"added", c.Added}, {"removed", c.Removed}, {"changed", c.Changed}} {
		for _, x := range s.l {
			fmt.Printf("  %-8s %s\n", s.what, x.Label)
		}
	}
	return 0
}

// boundaryGraph is boundary n's graph: from the store when it holds the
// graph of qNNN.c, else qNNN.c imported; and which.
func boundaryGraph(dir string, n int) (*graph.Graph, string, error) {
	text, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("q%03d.c", n)))
	if err != nil {
		return nil, "", err
	}
	g, why := pipeline.GraphSnapshotIn(dir, n, text)
	if g != nil {
		return g, "from the store", nil
	}
	if why != nil {
		fmt.Fprintf(os.Stderr, "  graph        q%03d: %v -- importing the text\n", n, why)
	}
	g, _, err = graph.Import(fmt.Sprintf("q%03d.c", n), text)
	return g, "imported", err
}
