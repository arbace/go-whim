package editor

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// chunkLeast is the fewest lines a chunk is given: below it a goroutine
// costs more than it saves.
const chunkLeast = 64

// Chunks runs work over [0, n) in chunks, on goroutines, and says whether
// every chunk's work did: the parallel body of a function the C writes as
// one loop over the range (match_lines, internal/whim/gen.go).  There are
// about four chunks a CPU, so that one slow chunk does not hold the rest,
// and none smaller than chunkLeast; a range of one chunk runs on the
// caller's goroutine, and a chunk that did not stops the chunks not yet
// started.  The work must be the loop's over its part and write
// nothing another part reads.
func Chunks(n int, work func(from, to int) bool) bool {
	size := max(chunkLeast, (n+4*runtime.GOMAXPROCS(0)-1)/(4*runtime.GOMAXPROCS(0)))
	if size >= n {
		return work(0, n)
	}
	var wg sync.WaitGroup
	var failed atomic.Bool
	for from := 0; from < n; from += size {
		wg.Add(1)
		go func(from, to int) {
			defer wg.Done()
			if failed.Load() || !work(from, to) {
				failed.Store(true)
			}
		}(from, min(n, from+size))
	}
	wg.Wait()
	return !failed.Load()
}
