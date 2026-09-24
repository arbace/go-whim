package canon

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrFailed is Run's refusal: what went wrong has already been written.
var ErrFailed = errors.New("canon failed")

// Run is the canonicalisers on a file, in place, once or to a fixpoint:
// `go tool whim canon FILE [--once]` at a prompt, and what a phase check calls in
// process to require that canon is a no-op on its output.  It was tools/canon.sh,
// a wrapper that found the binary and ran this; the check that ran the wrapper
// read its combined output, and a caller that wants that passes one writer for
// both.  Its lines are matched exactly, including the plural on "round".
//
// --once runs the passes a single time, for a caller with a fixpoint of its
// own: the sweep loops until a whole round changes nothing and canon is part of
// that round, so proving canon settled separately would prove it twice.
// Exceeding MaxRounds is a hard failure and not a number to raise.
//
// Measured when the passes became Go: 502 of 502 slim inputs identical to the
// Python's output, 494 of them exercising it, and one --once round over a 4.7
// MB file went from 3.577s to 0.314s with byte-identical output.
func Run(stdout, stderr io.Writer, file string, once bool) error {
	src, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(stderr, "whim: %v\n", err)
		return ErrFailed
	}
	out, rounds, changed, converged := Fixpoint(src, once)
	if err := WriteFile(file, out); err != nil {
		fmt.Fprintf(stderr, "whim: %v\n", err)
		return ErrFailed
	}
	if !converged {
		fmt.Fprintf(stdout, "  canon        NOT CONVERGING after %d rounds -- two passes are\n", rounds)
		fmt.Fprintln(stdout, "               undoing each other; that is a bug in one of them,")
		fmt.Fprintln(stdout, "               not a reason to raise the limit.")
		return ErrFailed
	}
	if once {
		if changed {
			fmt.Fprintln(stdout, "canon changed it")
		} else {
			fmt.Fprintln(stdout, "canon settled")
		}
		return nil
	}
	s := "s"
	if rounds == 1 {
		s = ""
	}
	fmt.Fprintf(stdout, "  canon        fixpoint after %d round%s\n", rounds, s)
	return nil
}

// WriteFile writes data over path keeping its permission bits, 0644 for a
// new file -- the rewrite in place every canonicaliser does.
func WriteFile(path string, data []byte) error {
	fi, err := os.Stat(path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = fi.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}
