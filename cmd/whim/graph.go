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
func runGraph(args []string) int {
	out, mode, files := "", "", []string(nil)
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
	if len(files) == 0 || mode != "--check" && len(files) > 1 || mode == "--check" && out != "" {
		return graphUsage()
	}
	if mode == "--check" {
		bad := 0
		for _, f := range files {
			if !graphCheck(f) {
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
	fmt.Fprintln(os.Stderr, "usage: whim graph [-o OUT] FILE | whim graph --check FILE... | whim graph --collect [-o OUT] FILE")
	return 2
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
