// Package pipeline is tools/pipeline.sh: the pipelines, and the only place
// that knows how they differ.
//
//	slim-vim.c = F(upstream@sha)   ten phases, work in upstream/
//	whim-vim.c = G(slim-vim.c)     work in .tmp/whim-stage/
//
// Both are the same construct -- a phase is a function of the tree it is
// handed, memoized in three tiers -- so the driver, the boundaries, the oracle
// and the synthesiser are shared and this is the whole of the parameterisation.
// A second copy of the memoize would be a second place for it to be subtly
// wrong.
package pipeline

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// P is the pipeline's parameters.
type P struct {
	Tag    string // q -- the boundary tag
	Doc    string
	Work   string // the work directory
	Build  string // where boundaries are recorded
	Source string // the one file a sweep runs on
	Delta  string // the declared-delta checker
	Phases []int
}

// Get returns the pipeline.
//
// ONE PIPELINE, so this takes no name: the tools take no pipeline argument
// either (tools/pipeline.sh).  The boundary tag stays q, so that a stray p3
// from arbace/slim-vim's own pipeline is obviously wrong rather than plausibly
// right.
//
// The phase list is NOT written here, and that is measured rather than tidy:
// tools/pipeline.sh is hashed into every stage's key and every edit's, so a
// byte changed there re-keys the whole pipeline, and a list written there would
// do that every time a phase was added.  It is the `phases` line of
// phase/stages, which no key reads, and this reads it from there too.
func Get() (P, error) {
	ph, err := manifestPhases("phase/stages")
	if err != nil {
		return P{}, err
	}
	return P{
		Tag: "q", Doc: "GOALS.md",
		Work: ".tmp/whim-stage", Build: ".build", Source: "whim-vim.c",
		Delta: "tools/whimdelta.sh", Phases: ph,
	}, nil
}

func seq(a, b int) []int {
	out := make([]int, 0, b-a+1)
	for i := a; i <= b; i++ {
		out = append(out, i)
	}
	return out
}

func manifestPhases(manifest string) ([]int, error) {
	f, err := os.Open(manifest)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 || fields[0] != "phases" {
			continue
		}
		out := make([]int, 0, len(fields)-1)
		for _, f := range fields[1:] {
			n, err := strconv.Atoi(f)
			if err != nil {
				continue
			}
			out = append(out, n)
		}
		return out, nil
	}
	return nil, s.Err()
}

// Parts is tools/phaserun.sh --parts: the programs a unit runs, in order.  A
// unit is a phase N or a stage A-B.  A split phase contributes its edit and
// its check; a whole phase contributes its one file; a phase with neither
// contributes nothing, and a unit with no programs at all is an agent's.
func (p P) Parts(unit string) []string {
	first, last := unit, unit
	if i := strings.Index(unit, "-"); i >= 0 {
		first, last = unit[:i], unit[i+1:]
	}
	a, err1 := strconv.Atoi(first)
	b, err2 := strconv.Atoi(last)
	if err1 != nil || err2 != nil {
		return nil
	}
	var out []string
	for n := a; n <= b; n++ {
		edit := fmt.Sprintf("phase/%03d/edit.sh", n)
		check := fmt.Sprintf("phase/%03d/check.sh", n)
		whole := fmt.Sprintf("phase/%03d/make.sh", n)
		if exists(edit) && exists(check) {
			out = append(out, edit, check)
		} else if exists(whole) {
			out = append(out, whole)
		}
	}
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
