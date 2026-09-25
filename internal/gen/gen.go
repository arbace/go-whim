// Package gen is the generator of editor/editor.go: internal/crefactor/togo,
// the C-to-Go translator, told what it must know about vim's core
// (internal/whim, Gen).  CONVENTIONS.md states the translation's rules,
// FINDINGS.md what it had to work around, and sigs.md is every signature it
// writes; pre/ and splice/ are the checks and the measurement beside it.
package gen

import (
	"io"

	"github.com/arbace/go-whim/internal/crefactor/togo"
	"github.com/arbace/go-whim/internal/whim"
)

// Run is the program, called as `go tool whim <name> ARGS`: args are its
// arguments, and what it used to print on stderr goes to errw.  It returns
// the exit status.
//
//	skel <editor.c> <outdir> [-bodies | -editor <editor.go>]
func Run(args []string, errw io.Writer) int { return togo.Run(args, errw, whim.Gen) }
