package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/arbace/go-whim/crefactor/cemit"
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
