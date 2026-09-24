package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arbace/go-whim/internal/dead"
)

// runFuncreach is the one dead-code tool left from the sweep's six: the
// function-level reachability a phase step still asks for in the middle of
// its own edit (internal/steps' "funcreach").  The sweep itself is
// internal/sweep's closure, and the other five went with the loop that ran
// them.
//
// Its <100-definition floor exits 1 with a message on stderr: finding fewer
// means the shape it matches has changed, and acting on the answer would
// delete most of the program.
func runFuncreach(args []string) int {
	del := false
	var files []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			if a == "--delete" {
				del = true
			}
			continue
		}
		files = append(files, a)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "usage: whim funcreach <file> [--delete]")
		return 1
	}
	text, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defs, reachable, deadNames, deadLines := dead.FuncReach(text)
	if len(defs) < dead.MinDefinitions {
		fmt.Fprintf(os.Stderr, "funcreach: only %d definitions found, which cannot be right "+
			"for this file -- the shape it matches has changed, and acting "+
			"on the answer would delete most of the program\n", len(defs))
		return 1
	}
	fmt.Printf("  funcreach    %d definitions, %d reachable, %d not (%d lines)\n",
		len(defs), reachable, len(deadNames), deadLines)
	if len(deadNames) > 0 && del {
		if err := writeFile(files[0], dead.DeleteFuncs(text, defs, deadNames)); err != nil {
			fmt.Fprintf(os.Stderr, "whim: %v\n", err)
			return 1
		}
		n := len(deadNames)
		if n > 6 {
			n = 6
		}
		tail := ""
		if len(deadNames) > 6 {
			tail = "..."
		}
		fmt.Printf("  funcreach    deleted: %s%s\n", strings.Join(deadNames[:n], ", "), tail)
	}
	return 0
}
