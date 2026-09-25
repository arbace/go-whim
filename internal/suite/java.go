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

// THE EDITORS ON THE JVM, on demand: the Java editor (jeditor/,
// doc/JAVA.md), `whim test --java`, and the Clojure editor (cljeditor/,
// doc/CLOJURE.md), `whim test --clojure` (clojure.go).  The candidate's core
// is written by the backend (Editor.java, whim/editor.clj), compiled with
// the editor's runtime, host and glue, and run on the same cases as the Go
// editor, required to answer exactly as the C candidate does.
//
// ITS OWN CONTROL.  The C control is " INSERT" spelled " INSERX" in the C;
// each JVM editor's is the same string changed in its generated file, and it
// must move at least one case of THAT editor's own answers -- not the C's,
// which an editor that answers nothing would differ from anyway.  So an
// editor that stops before it draws cannot pass as having seen it.

// javaControl changes the Java's copy of the C control's string.
const javaControlOld, javaControlNew = `" INSERT"`, `" INSERX"`

// A jvmEditor is an editor on the JVM the suite built from the candidate,
// and its control: the Java's or the Clojure's.
type jvmEditor struct {
	name, where string         // "Java", "jeditor/"
	file        string         // the generated file its control changes
	launcher    string         // the name its launcher reports a failure under
	frame       *regexp.Regexp // a frame of the core; its function the first group
	bin, ctl    string         // the two launchers
}

// buildJava builds the Java editor from candSrc, and its control, under dir.
func buildJava(gen jeditor.Gen, candSrc, dir string) (*jvmEditor, error) {
	e := &jvmEditor{name: "Java", where: "jeditor/", file: "Editor.java", launcher: "whim-java",
		frame: regexp.MustCompile(`^\tat (?:Editor|Whim)\.([A-Za-z0-9_$]+)\(`)}
	jdir, cdir := filepath.Join(dir, "java"), filepath.Join(dir, "java-control")
	bin, err := jeditor.Build(gen, candSrc, jdir, filepath.Join(dir, "whim-java"), nil)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(filepath.Join(jdir, "src", "Editor.java"))
	if err != nil {
		return nil, err
	}
	ctlSrc, err := writeControl(src, "Editor.java", filepath.Join(cdir, "src"))
	if err != nil {
		return nil, err
	}
	ctl, err := jeditor.Compile(ctlSrc, cdir, filepath.Join(dir, "whim-java-control"))
	if err != nil {
		return nil, err
	}
	e.bin, e.ctl = bin, ctl
	return e, nil
}

// writeControl writes src, the generated file name, with the control applied,
// into dir, and returns its path; an error unless the control's string is in
// it exactly once.
func writeControl(src []byte, name, dir string) (string, error) {
	if n := bytes.Count(src, []byte(javaControlOld)); n != 1 {
		return "", fmt.Errorf("suite: the control string %s is in %s %d times, not once", javaControlOld, name, n)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	return p, os.WriteFile(p, bytes.Replace(src, []byte(javaControlOld), []byte(javaControlNew), 1), 0o644)
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

// thrown is what an editor on the JVM reports when an exception stops it, as
// its launcher prints it: its name and a colon ("whim-java: "), the
// exception, and the first frames, a line each.
func thrown(launcher string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(launcher) + `: ([^\n]*)\n((?:\tat [^\n]*(?:\n|$))*)`)
}

// failures counts what stopped the editor e in the cases it differs on: the
// exception and the first frame of the core among those printed --
// "vim_main: refused: ..." and the like -- most frequent first.
func failures(rs []result, e *jvmEditor) []string {
	re := thrown(e.launcher)
	count := map[string]int{}
	for _, r := range rs {
		if r.same {
			continue
		}
		m := re.FindSubmatch(r.outB)
		if m == nil {
			continue
		}
		key := string(m[1])
		for _, f := range strings.Split(string(m[2]), "\n") {
			if fm := e.frame.FindStringSubmatch(f); fm != nil {
				key = fm[1] + ": " + key
				break
			}
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

// checkJVM runs the cases on the editor e against the C candidate, and on
// e's control against e, and reports per group; an error when e differs or
// its control went unseen.
func checkJVM(w io.Writer, label string, groups []string, cases []WideCase, b *builds, e *jvmEditor) error {
	start := time.Now()
	diffRes, err := compareEach(cases, b.cand, e.bin)
	if err != nil {
		return err
	}
	seenRes, err := compareEach(cases, e.bin, e.ctl)
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
			line += fmt.Sprintf(": the %s editor differs from the C on %d: %s", e.name, len(diff[g]), abbrev(diff[g], 12))
		} else {
			line += fmt.Sprintf(": the %s editor (%s) answers all exactly as the C does", e.name, e.where)
		}
		line += fmt.Sprintf("; its control seen by %d", len(seen[g]))
		fmt.Fprintln(w, line)
	}
	for _, f := range failures(diffRes, e) {
		fmt.Fprintf(w, "  %-12s %s\n", label, f)
	}
	fmt.Fprintf(w, "  %-12s %dms\n", label, time.Since(start).Milliseconds())
	if fail {
		return fmt.Errorf("suite: the %s editor differs from the C", e.name)
	}
	if nSeen == 0 {
		return fmt.Errorf("suite: THE %s CONTROL WENT UNSEEN -- %s changed to %s in %s and no case of the %s editor noticed",
			strings.ToUpper(e.name), javaControlOld, javaControlNew, e.file, e.name)
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

// label is how e's lines are headed: its launcher's name without "whim-"
// ("java", "clj"), after prefix ("wide ") when there is one.
func (e *jvmEditor) label(prefix string) string {
	return prefix + strings.TrimPrefix(e.launcher, "whim-")
}
