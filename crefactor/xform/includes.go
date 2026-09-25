package xform

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Silent is the compiler question Includes asks: a file is fine when Cmd,
// given Flags and the file, prints NOTHING.  A warning is an answer, so the
// command exiting 0 is not enough -- gcc exits 0 with warnings, and an
// implicit declaration is exactly the warning a removed header produces.
type Silent struct {
	Cmd   string
	Flags []string
}

var includeLine = regexp.MustCompile(`^#include <[^>]+>$`)

// Includes is the step that removes every system header the file does not
// need, asking the compiler rather than a rule: each `#include <...>` line is
// deleted on its own and kept out if the file still compiles silently; the
// survivors are then removed together, and if that is not silent they are
// re-tried one at a time FROM THE BOTTOM.  A directive other than `#include`
// refuses, and so does an input that does not compile silently.
//
// With a directory argument it leaves `total` and `keep` there, for a check
// that wants to know what was asked and what went.
func Includes(s Silent) Step {
	return func(t []byte, args []string, w io.Writer) ([]byte, error) { return includes(s, t, args, w) }
}

func includes(s Silent, t []byte, args []string, w io.Writer) ([]byte, error) {
	lines := strings.Split(string(t), "\n")
	var cand []int
	for i, ln := range lines {
		if includeLine.MatchString(ln) {
			cand = append(cand, i)
		} else if s := strings.TrimLeft(ln, " \t"); strings.HasPrefix(s, "#") {
			return nil, fmt.Errorf("  includes     a directive other than #include is in the file")
		}
	}
	if len(cand) == 0 {
		return nil, fmt.Errorf("  includes     no #include lines found")
	}
	d, err := os.MkdirTemp("", "includes")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(d)

	silent := func(name string, text []byte) (bool, error) {
		p := filepath.Join(d, name)
		if err := os.WriteFile(p, text, 0o644); err != nil {
			return false, err
		}
		out, err := exec.Command(s.Cmd, append(append([]string{}, s.Flags...), p)...).CombinedOutput()
		os.Remove(p)
		return err == nil && len(out) == 0, nil
	}
	if ok, err := silent("in.c", t); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("  includes     the input does not compile silently -- nothing to measure against")
	}

	// Each line on its own, in parallel: the answers do not depend on each other.
	var mu sync.Mutex
	var wg sync.WaitGroup
	var alone []int
	var firstErr error
	sem := make(chan struct{}, 16)
	for _, n := range cand {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ok, err := silent(fmt.Sprintf("t%d.c", n), without(lines, []int{n}))
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if ok {
				alone = append(alone, n)
			}
		}(n)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	sort.Ints(alone)
	fmt.Fprintf(w, "  includes     %d of %d can go on their own\n", len(alone), len(cand))

	keep := alone
	if ok, err := silent("all.c", without(lines, alone)); err != nil {
		return nil, err
	} else if ok {
		fmt.Fprintf(w, "  includes     and all of them together\n")
	} else {
		keep = nil
		for i := len(alone) - 1; i >= 0; i-- { // from the bottom
			try := append(append([]int{}, keep...), alone[i])
			ok, err := silent("t.c", without(lines, try))
			if err != nil {
				return nil, err
			}
			if ok {
				keep = try
			}
		}
		sort.Ints(keep)
		fmt.Fprintf(w, "  includes     together they do not build; %d removed one by one, from the bottom\n", len(keep))
	}
	if len(keep) == 0 {
		fmt.Fprintf(w, "  includes     every header is needed\n")
		return t, nil
	}
	var removed []string
	for _, n := range keep {
		removed = append(removed, strings.TrimSuffix(strings.TrimPrefix(lines[n], "#include <"), ">"))
	}
	if len(args) > 0 && args[0] != "" {
		if err := os.WriteFile(filepath.Join(args[0], "total"),
			[]byte(fmt.Sprintf("%d\n", len(cand))), 0o644); err != nil {
			return nil, err
		}
		var ks strings.Builder
		for _, n := range keep {
			fmt.Fprintf(&ks, "%d\n", n+1) // the phase program counted from 1
		}
		if err := os.WriteFile(filepath.Join(args[0], "keep"), []byte(ks.String()), 0o644); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(w, "  includes     removed: %s \n", strings.Join(removed, " "))
	return without(lines, keep), nil
}

// without is the phase program's `drop`: the text with those lines deleted.
func without(lines []string, drop []int) []byte {
	gone := make(map[int]bool, len(drop))
	for _, n := range drop {
		gone[n] = true
	}
	out := make([]string, 0, len(lines))
	for i, ln := range lines {
		if !gone[i] {
			out = append(out, ln)
		}
	}
	return []byte(strings.Join(out, "\n"))
}
