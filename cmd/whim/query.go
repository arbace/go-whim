package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arbace/go-whim/internal/phase"
)

func runQuery(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: whim query <phase> <file>\n  queries: %s\n",
			strings.Join(phase.QueryNames(), " "))
		return 1
	}
	f, ok := phase.LookupQuery(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "whim query: no query for phase %q\n  queries: %s\n",
			args[0], strings.Join(phase.QueryNames(), " "))
		return 1
	}
	text, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	if err := f(text, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
