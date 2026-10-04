package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/graph"
)

// Canonical is the canonical print as a step: the text in the one C23
// spelling per construct, with a line of report.
func Canonical(t []byte, args []string, w io.Writer) ([]byte, error) {
	f, err := os.CreateTemp("", "cemit.*.c")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(t); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	out, err := cemit.Canonical(f.Name(), t)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(w, "  canonical    %d lines from %d, one form per construct\n",
		bytes.Count(out, []byte("\n")), bytes.Count(t, []byte("\n")))
	return out, nil
}

// Seed is what phase 0 hands the pipeline: the input IN CANONICAL FORM.  Every
// later phase reads that form -- one C23 spelling per construct, so an anchor
// matches what it means rather than what the input happened to write.
//
// IT IS A FUNCTION SO THAT THERE IS ONE OF IT: when a second caller seeded by
// reading the file, it ran every phase after 0 on the residue spelling, a
// pipeline the build does not run.
func Seed(src []byte, w io.Writer) ([]byte, error) {
	if w == nil {
		w = io.Discard
	}
	out, err := Canonical(src, nil, w)
	if err != nil {
		return nil, fmt.Errorf("cemit: %w", err)
	}
	return out, nil
}

// SeedGraph is the seed of a plan whose phase 0 begins on the graph: the
// input IMPORTED, not printed and imported again -- the importer's C view
// is cemit's canonical print, byte for byte (measured on slim-vim.c), so
// the text it returns is Seed's and the graph is the import of it.  path
// is the name the input is parsed under.
func SeedGraph(src []byte, path string, w io.Writer) (*graph.Graph, []byte, time.Duration, error) {
	if w == nil {
		w = io.Discard
	}
	start := time.Now()
	g, _, err := graph.Import(path, src)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("import: %w", err)
	}
	d := time.Since(start)
	out, err := g.C()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("the C view: %w", err)
	}
	fmt.Fprintf(w, "  canonical    %d lines from %d, one form per construct: the import's C view; %dms\n",
		bytes.Count(out, []byte("\n")), bytes.Count(src, []byte("\n")), time.Since(start).Milliseconds())
	return g, out, d, nil
}
