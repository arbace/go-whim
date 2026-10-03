package graph

import (
	"fmt"
	"strings"
)

// SystemIncludeRun is the file's include forms when they are one run of
// consecutive top-level forms, every one an `#include <...>` of a system
// header -- the shape the text programs asserted as "N directives on N
// consecutive lines" -- and the index of the first among the file's forms.
// Refused otherwise, saying where.  (On the graph the include forms are the
// only directives there are: a form is C or an include.)
func (e *Editor) SystemIncludeRun() ([]*Node, int, error) {
	incs := e.Includes()
	if len(incs) == 0 {
		return nil, -1, fmt.Errorf("the file has no include form, so there is no boundary between the core and the host")
	}
	at := e.formAt(incs[0])
	for k, inc := range incs {
		if i := e.formAt(inc); i != at+k {
			return nil, at, fmt.Errorf("the %d include forms are not one run of consecutive forms: the %d-th is form %d, the first form %d", len(incs), k+1, i, at)
		}
		if spec := IncludeSpec(inc); !strings.HasPrefix(spec, "<") {
			return nil, at, fmt.Errorf("an include form is not an `#include <...>` of a system header: %s", spec)
		}
	}
	return incs, at, nil
}
