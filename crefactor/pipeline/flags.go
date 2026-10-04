package pipeline

import (
	"fmt"
	"strconv"
)

// Flags reads a step's counted arguments: each `--name N` a count, and
// nothing else -- a count a phase refuses under is an argument in the plan
// (`--at-least N`), so that a foreign code base passes nothing and gets no
// floor.  A name not in want refuses, and so does a count that is not a
// number.  It was crefactor/xform's, the text transforms' parser, until
// xform was deleted (doc/GRAPH-MIGRATION.md, *Fin as built*).
func Flags(tag string, args []string, want ...string) (map[string]int, error) {
	out := map[string]int{}
	for i := 0; i < len(args); i++ {
		ok := false
		for _, n := range want {
			ok = ok || args[i] == n
		}
		if !ok || i+1 == len(args) {
			return nil, fmt.Errorf("%s: unexpected argument %q (want %v, each with a count)", tag, args[i], want)
		}
		n, err := strconv.Atoi(args[i+1])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("%s: %s wants a count, not %q", tag, args[i], args[i+1])
		}
		out[args[i]] = n
		i++
	}
	return out, nil
}
