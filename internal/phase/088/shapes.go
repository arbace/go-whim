package p088

import "strings"

// This phase's own shapes: they were in internal/whim/vimtext, which holds
// only what more than one phase uses, and only this phase uses these.

// From phase 88.
// splitLinesKeep is Python's splitlines(keepends=True): each line with its newline.
func splitLinesKeep(s string) []string {
	var Out []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			Out = append(Out, s)
			break
		}
		Out = append(Out, s[:i+1])
		s = s[i+1:]
	}
	return Out
}
