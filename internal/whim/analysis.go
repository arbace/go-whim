package whim

import "github.com/arbace/go-whim/internal/reach"

// What the analysis tools are told about vim (doc/VIM-VS-GENERIC.md, step 7):
// internal/dead's funcreach and internal/reach's closure.  Each of them hard-coded these names before; none of them names
// anything in vim now.

// allocators are vim's allocators, whose void * result is fresh memory; the
// core's host_alloc is one more where the analysis reads the whole core.
var allocators = []string{"alloc", "alloc_clear", "lalloc", "lalloc_clear"}

// Dead is what internal/dead's funcreach is told: vim's one entry point.  It
// was hard-coded in internal/dead/funcreach.go.
var Dead = struct{ Roots []string }{Roots: []string{"main"}}

// Reach is what internal/reach's closure is told: ml_recover, whose
// definition means the editor still reads swap files, so a struct layout is a
// disk format; and the allocators, a cast of whose result is no pun.  They
// were hard-coded in internal/reach/reach.go and cast.go.
var Reach = reach.Options{
	FreezeLayoutIf: []string{"ml_recover"},
	Allocators:     append(append([]string{}, allocators...), "host_alloc"),
}
