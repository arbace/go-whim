package build

import (
	"fmt"
	"strings"
)

// THE COMPILE LINE IS THE BOUNDARY'S, and it moves twice: phase 0 starts from
// the input's own line, phase 83 makes the binary an ordinary static
// executable (-no-pie), and phase 84 drops the stack protector.  A binary built
// with any other line is not the binary that boundary means.  The lines were
// makefiles under tools/templates/ while a check ran make in a work tree;
// nothing runs make there any more, so they are data here.
var lines = map[string]struct{ c, l []string }{
	"whim": {c: []string{"-O0"}, l: []string{"-static", "-s"}},            // a static-PIE
	"core": {c: []string{"-O0"}, l: []string{"-static", "-no-pie", "-s"}}, // EXEC, no dynamic section
}

// FlagsFor is the compile line the boundary after phase n carries: the last
// line a phase up to n set (Phase.Line "whim" or "core"), with every CFLAGS
// addition a later phase up to n made ("+flag").
func FlagsFor(n int) (cflags, ldflags []string, err error) {
	var base string
	var adds []string
	for _, p := range Plan {
		if p.N > n {
			break
		}
		switch {
		case p.Line == "":
		case strings.HasPrefix(p.Line, "+"):
			adds = append(adds, p.Line[1:])
		default:
			base, adds = p.Line, nil
		}
	}
	ln, ok := lines[base]
	if !ok {
		return nil, nil, fmt.Errorf("build: no compile line is defined at phase %d", n)
	}
	cflags = append(append([]string{}, ln.c...), adds...)
	return cflags, append([]string{}, ln.l...), nil
}
