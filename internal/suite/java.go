package suite

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/jeditor"
)

// THE JAVA EDITOR (jeditor/, doc/JAVA.md), on demand: `whim test --java`.
// The candidate's core is written as Editor.java by the Java backend,
// compiled with jeditor's runtime, host and glue, and run on the same cases
// as the Go editor, required to answer exactly as the C candidate does.
//
// ITS OWN CONTROL.  The C control is " INSERT" spelled " INSERX" in the C;
// the Java's is the same string changed in the generated Editor.java, and it
// must move at least one case of the JAVA editor's own answers -- not the
// C's, which an editor that answers nothing would differ from anyway.  So an
// editor that stops before it draws cannot pass as having seen it.

// javaControl changes the Java's copy of the C control's string.
const javaControlOld, javaControlNew = `" INSERT"`, `" INSERX"`

// buildJava builds the Java editor from candSrc, and its control, under dir:
// the two launchers.
func buildJava(gen jeditor.Gen, candSrc, dir string) (bin, ctl string, err error) {
	jdir, cdir := filepath.Join(dir, "java"), filepath.Join(dir, "java-control")
	bin, err = jeditor.Build(gen, candSrc, jdir, filepath.Join(dir, "whim-java"), nil)
	if err != nil {
		return "", "", err
	}
	src, err := os.ReadFile(filepath.Join(jdir, "src", "Editor.java"))
	if err != nil {
		return "", "", err
	}
	if n := bytes.Count(src, []byte(javaControlOld)); n != 1 {
		return "", "", fmt.Errorf("suite: the control string %s is in Editor.java %d times, not once", javaControlOld, n)
	}
	if err := os.MkdirAll(filepath.Join(cdir, "src"), 0o755); err != nil {
		return "", "", err
	}
	ctlSrc := filepath.Join(cdir, "src", "Editor.java")
	if err := os.WriteFile(ctlSrc, bytes.Replace(src, []byte(javaControlOld), []byte(javaControlNew), 1), 0o644); err != nil {
		return "", "", err
	}
	ctl, err = jeditor.Compile(ctlSrc, cdir, filepath.Join(dir, "whim-java-control"))
	return bin, ctl, err
}

// A result is one case run on two editors.
type result struct {
	c     WideCase
	same  bool
	outB  []byte // b's output, kept when it differs
	err   error
	stopB bool // b could not be run to its end (a hang, or no start)
}

// compareEach runs every case on a and b, as many at once as there are cores.
func compareEach(cases []WideCase, a, b string) ([]result, error) {
	results := make([]result, len(cases))
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for i, c := range cases {
		wg.Add(1)
		go func(i int, c WideCase) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			oa, sa, err := runWide(a, c)
			if err != nil {
				results[i] = result{c: c, err: fmt.Errorf("%s %s on %s: %w", c.Group, c.Name, a, err)}
				return
			}
			ob, sb, err := runWide(b, c)
			if err != nil {
				results[i] = result{c: c, outB: ob, stopB: true}
				return
			}
			same := sa == sb && bytes.Equal(oa, ob)
			results[i] = result{c: c, same: same}
			if !same {
				results[i].outB = ob
			}
		}(i, c)
	}
	wg.Wait()
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
	}
	return results, nil
}

// byGroup is the names of the cases that differ, by group.
func byGroup(rs []result) map[string][]string {
	diff := map[string][]string{}
	for _, r := range rs {
		if !r.same {
			diff[r.c.Group] = append(diff[r.c.Group], r.c.Name)
		}
	}
	return diff
}

// thrown is the Java launcher's report of what stopped the editor: the
// exception, and the first frame of the core it came from.
var thrown = regexp.MustCompile(`whim-java: ([^\n]*)\n(?:\tat (?:Editor|Whim)\.([A-Za-z0-9_$]+)\()?`)

// failures counts what stopped the Java editor in the cases it differs on:
// "vim_main: refused: ..." and the like, most frequent first.
func failures(rs []result) []string {
	count := map[string]int{}
	for _, r := range rs {
		if r.same {
			continue
		}
		m := thrown.FindSubmatch(r.outB)
		if m == nil {
			continue
		}
		key := string(m[1])
		if len(m[2]) > 0 {
			key = string(m[2]) + ": " + key
		}
		count[key]++
	}
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if count[keys[i]] != count[keys[j]] {
			return count[keys[i]] > count[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var out []string
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%d thrown in %s", count[k], k))
	}
	return out
}

// checkJava runs the cases on the Java editor against the C candidate, and
// on the Java's control against the Java, and reports per group; an error
// when the Java differs or its control went unseen.
func checkJava(w io.Writer, label string, groups []string, cases []WideCase, b *builds) error {
	start := time.Now()
	diffRes, err := compareEach(cases, b.cand, b.javaBin)
	if err != nil {
		return err
	}
	seenRes, err := compareEach(cases, b.javaBin, b.javaCtl)
	if err != nil {
		return err
	}
	diff, seen := byGroup(diffRes), byGroup(seenRes)
	count := map[string]int{}
	for _, c := range cases {
		count[c.Group]++
	}
	fail := false
	nSeen := 0
	for _, g := range groups {
		nSeen += len(seen[g])
		head := label
		if len(groups) > 1 {
			head = fmt.Sprintf("%s %-5s", label, g)
		}
		line := fmt.Sprintf("  %-12s %3d cases", head, count[g])
		if len(diff[g]) > 0 {
			fail = true
			line += fmt.Sprintf(": the Java editor differs from the C on %d: %s", len(diff[g]), abbrev(diff[g], 12))
		} else {
			line += ": the Java editor (jeditor/) answers all exactly as the C does"
		}
		line += fmt.Sprintf("; its control seen by %d", len(seen[g]))
		fmt.Fprintln(w, line)
	}
	for _, f := range failures(diffRes) {
		fmt.Fprintf(w, "  %-12s %s\n", label, f)
	}
	fmt.Fprintf(w, "  %-12s %dms\n", label, time.Since(start).Milliseconds())
	if fail {
		return fmt.Errorf("suite: the Java editor differs from the C")
	}
	if nSeen == 0 {
		return fmt.Errorf("suite: THE JAVA CONTROL WENT UNSEEN -- %s changed to %s in Editor.java and no case of the Java editor noticed", javaControlOld, javaControlNew)
	}
	return nil
}

// abbrev is names joined, the first n of them and a count of the rest.
func abbrev(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, " ")
	}
	return strings.Join(names[:n], " ") + fmt.Sprintf(" and %d more", len(names)-n)
}
