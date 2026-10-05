package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/graphcheck"
)

// runGraph is the program as one resolved, typed graph (crefactor/graph,
// doc/GRAPH.md).
//
//	whim graph [-o OUT] FILE          the graph as Lisp: the forms with their
//	                                  ids and edges, the types, the externs
//	whim graph --check FILE...        steps 1 and 2: the C view of the graph is
//	                                  FILE byte for byte, and so is the graph
//	                                  written and read back, which is the same
//	                                  graph; what the import resolved
//	whim graph --collect [-o OUT] FILE  the sweep as garbage collection, vim's
//	                                  roots and guard: the C view collected
//	whim graph --edn [-o OUT] FILE    the graph as EDN (Clojure's reader reads
//	                                  it as it is), FILE C or the graph's Lisp
//	whim graph --edn --check FILE...  FILE's graph written as EDN and read back:
//	                                  the same graph, its C view FILE's text
//	whim graph --changes N ...        what phase N changed, from the hashes
//	whim graph --snapshot N ...       boundary N's graph from the store
//	                                  (graphchanges.go)
func runGraph(args []string) int {
	if len(args) > 0 && (args[0] == "--changes" || args[0] == "--snapshot") {
		return runGraphBoundary(args[0], args[1:])
	}
	out, mode, files, edn := "", "", []string(nil), false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--edn":
			edn = true
		case "--check", "--collect":
			mode = args[i]
		case "-o":
			i++
			if i >= len(args) {
				return graphUsage()
			}
			out = args[i]
		default:
			files = append(files, args[i])
		}
	}
	if len(files) == 0 || mode != "--check" && len(files) > 1 || mode == "--check" && out != "" || edn && mode == "--collect" {
		return graphUsage()
	}
	if mode == "--check" {
		bad := 0
		check := graphCheck
		if edn {
			check = ednCheck
		}
		for _, f := range files {
			if !check(f) {
				bad++
			}
		}
		if bad > 0 {
			fmt.Fprintf(os.Stderr, "  graph        %d of %d differ\n", bad, len(files))
			return 1
		}
		return 0
	}
	src, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	if edn {
		g, err := graphOf(files[0], src)
		var e []byte
		if err == nil {
			e, err = g.EDN()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "  graph        %v\n", err)
			return 1
		}
		return emit(out, e)
	}
	g, _, err := graph.Import(files[0], src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  graph        %v\n", err)
		return 1
	}
	if mode == "--collect" {
		start := time.Now()
		st, err := graph.Collect(g, graphcheck.Options())
		if err != nil {
			fmt.Fprintf(os.Stderr, "  graph        %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "  collect      %s; %dms\n", st, time.Since(start).Milliseconds())
		c, err := g.C()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  graph        %v\n", err)
			return 1
		}
		return emit(out, c)
	}
	return emit(out, g.Lisp())
}

func graphUsage() int {
	fmt.Fprintln(os.Stderr, "usage: whim graph [--edn] [-o OUT] FILE | whim graph [--edn] --check FILE... | whim graph --collect [-o OUT] FILE | whim graph --changes N [--nominal] [--dir D] | whim graph --snapshot N [-o OUT] [--dir D]")
	return 2
}

// graphOf is a file's graph: read when it is a graph's Lisp (a ;; line
// first, as the snapshots and `whim graph` write it), else imported.
func graphOf(path string, src []byte) (*graph.Graph, error) {
	if bytes.HasPrefix(src, []byte(";;")) {
		return graph.Read(src)
	}
	g, _, err := graph.Import(path, src)
	return g, err
}

// ednCheck holds one C file to the EDN's round trip: imported, written as
// EDN, read back -- the same graph, ids and edges, and its C view the file.
func ednCheck(path string) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  graph        %v\n", err)
		return false
	}
	g, err := graphOf(path, src)
	want := src // a C file's C view is the file; a snapshot's, its graph's
	if err == nil && bytes.HasPrefix(src, []byte(";;")) {
		want, err = g.C()
	}
	var e []byte
	if err == nil {
		e, err = g.EDN()
	}
	var read time.Duration
	if err == nil {
		start := time.Now()
		var h *graph.Graph
		h, err = graph.ReadEDN(e)
		read = time.Since(start)
		if err == nil {
			err = graph.Equal(g, h)
		}
		if err == nil {
			var c []byte
			if c, err = h.C(); err == nil && !bytes.Equal(c, want) {
				err = fmt.Errorf("the C view of the EDN read back is not the file's: %s", firstDiff(want, c))
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  graph        %s: %v\n", path, err)
		return false
	}
	fmt.Printf("OK %s: %d bytes of EDN read back in %dms, the same graph, its C view the file's\n", path, len(e), read.Milliseconds())
	return true
}

// graphCheck holds one file to steps 1 and 2 and reports.
func graphCheck(path string) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  graph        %v\n", err)
		return false
	}
	start := time.Now()
	g, rep, err := graph.Import(path, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  graph        %s: %v\n", path, err)
		return false
	}
	imported := time.Since(start)
	c, err := g.C()
	if err == nil && !bytes.Equal(c, src) {
		err = fmt.Errorf("the C view is not the file: %s", firstDiff(src, c))
	}
	text := g.Lisp()
	var read time.Duration
	if err == nil {
		start = time.Now()
		var h *graph.Graph
		h, err = graph.Read(text)
		read = time.Since(start)
		if err == nil {
			err = graph.Equal(g, h)
		}
		if err == nil {
			if c, err = h.C(); err == nil && !bytes.Equal(c, src) {
				err = fmt.Errorf("the C view of the graph read back is not the file: %s", firstDiff(src, c))
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  graph        %s: %v\n", path, err)
		return false
	}
	fmt.Printf("OK %s: imported in %dms, %d bytes of Lisp read back in %dms\n%s",
		path, imported.Milliseconds(), len(text), read.Milliseconds(), rep)
	return true
}
