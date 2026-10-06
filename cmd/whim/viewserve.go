package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/arbace/go-whim/crefactor/graph/view"
	"github.com/arbace/go-whim/internal/whim"
)

// runViewServe is an editing session on the program's graph for an editor
// to drive (doc/GRAPH.md, *The editing server*): FILE's graph loaded once
// (as whim view loads it), then a request a line on stdin, an answer on
// stdout, the framing view-clj --serve's -- a header, the body, `;;end`:
//
//	ok N [WORDS]        N the body's bytes; then the body, and ;;end
//	error N             the message as the body
//
// Buffers are views' texts being typed into (crefactor/graph/view's
// Buffer), any number open on one session, their edits made in place in it
// and undone in order: `open` gives each a number, B, and a request
// begins with `@B` for buffer B, or is for the last one named.  An edit
// through one prints the others again: `touched B...` in its header names
// those whose text moved.  Words are blank-separated; '...' quotes a word
// as it is, "..." as a Go string (\n, \", \\), for a change's text.
//
//	open [--ids] [--fallout] [--depth N] [--show S] [--stop H] VIEW ARG   a buffer on a view;
//	                        body: its text; header: ok N buf B; --fallout closes over the
//	                        uses a deletion leaves (they are refused without it); `@B open`
//	                        puts buffer B on the view
//	buffers                 the buffers open: a line each, B and its view
//	close                   the buffer closed
//	text                    the buffer's text; header: ok N STATUS [REASON]
//	change FROM TO TEXT [CURSOR]  bytes FROM to TO of the buffer (or LINE:COL)
//	                        replaced by TEXT; header: ok N STATUS CURSOR [REASON] --
//	                        applied, pending, layout or same, CURSOR the change's end
//	                        in the body, or the CURSOR given (a byte of the text after
//	                        the change: the editor's) where it is in the body; an
//	                        applied change that put top-level forms beside the
//	                        view's adds `added NAME...`
//	at POS [VIEW]           the buffer reopened at the cursor: VIEW (the buffer's, by
//	                        default) of what the node at POS is about; header: ok N #ID NAME
//	rename POS NAME         the entity at POS renamed, its every declaration and use, as
//	                        one edit; header: ok N #ID D declarations U uses
//	undo                    the last edit undone; body: the buffer printed again
//	revert                  what is pending dropped; body: the buffer
//	c NAME                  NAME's definition as C, from the graph as it is
//	write FILE              the program's C view written to FILE; header: ok 0 BYTES
//	edits | stats | ping | quit
//
//	whim view-serve [--no-cache] [FILE]
func runViewServe(args []string) int {
	noCache, file := false, "src/whim-vim.c"
	for _, a := range args {
		switch {
		case a == "--no-cache":
			noCache = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintln(os.Stderr, "usage: whim view-serve [--no-cache] [FILE]")
			return 2
		default:
			file = a
		}
	}
	start := time.Now()
	g, how, err := loadGraph(file, noCache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  view-serve   %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "  view-serve   %s %s %dms; a request a line\n", file, how, time.Since(start).Milliseconds())
	fo := whim.GraphFallOut
	view.NewServer(g, file, view.ServerOptions{FallOut: &fo}).Serve(os.Stdin, os.Stdout)
	return 0
}
