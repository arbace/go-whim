package whim

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// includeLine is where the core ends: the first #include, whitespace after
// the # being insignificant to C.
var includeLine = regexp.MustCompile(`^ *# *include `)

// Cut is the core half of a whim-vim.c: every line before the first
// #include, which is the line between the core and its host (GOALS.md
// II.4c), trailing blank lines dropped -- and an error when what is left
// holds a directive, since the core has none, so a cut that kept one found
// the wrong line.  The core does not compile on its own (it calls the musl_
// functions the host defines) and is not meant to; it parses, and it is what
// every translation is written from: the Go, the Java and the Clojure
// editors each cut it for themselves (`go tool whim cut` prints it).
func Cut(c []byte) ([]byte, error) {
	lines := strings.SplitAfter(string(c), "\n")
	last := -1
	for i, l := range lines {
		if includeLine.MatchString(l) {
			break
		}
		if strings.TrimSpace(l) != "" {
			last = i
		}
		if strings.HasPrefix(strings.TrimLeft(l, " "), "#") {
			return nil, fmt.Errorf("the cut holds a directive at line %d, so it found the wrong line", i+1)
		}
	}
	if last < 0 {
		return nil, fmt.Errorf("no core before the first #include")
	}
	var b bytes.Buffer
	for _, l := range lines[:last+1] {
		b.WriteString(strings.TrimSuffix(l, "\n"))
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}
