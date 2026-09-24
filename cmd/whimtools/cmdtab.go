package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arbace/go-whim/internal/cmdtab"
)

// runCmdnames is create_cmdidxs.names(): the Ex command table, in order.  It
// is exposed so the parse can be compared against the Python's without
// running the editor six hundred times.
func runCmdnames(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: whimtools cmdnames <file>")
		return 1
	}
	names, err := cmdtab.CommandNames(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return 0
}

// runCmdidxs is tools/create_cmdidxs.py.  No flag prints the generated block,
// --check requires the file's block to match it, --update rewrites it.
func runCmdidxs(args []string) int {
	var path, mode string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			mode = a
		} else if path == "" {
			path = a
		}
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "usage: whimtools cmdidxs <file> [--check|--update]")
		return 1
	}
	switch mode {
	case "--check":
		if err := cmdtab.CheckCmdIdxs(path); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		fmt.Printf("%s: ex_cmdidxs block reproduces byte for byte\n", path)
	case "--update":
		if err := cmdtab.UpdateCmdIdxs(path); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	default:
		names, err := cmdtab.CommandNames(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		fmt.Print(cmdtab.GenerateCmdIdxs(names))
	}
	return 0
}
