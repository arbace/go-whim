// Command whim is the editor on a terminal: package editor, run with the
// terminal host.  make bin/whim builds it.
package main

import (
	"os"

	"github.com/arbace/go-whim/editor"
	"github.com/arbace/go-whim/editor/term"
)

func main() { os.Exit(editor.Main(term.New(), os.Args)) }
