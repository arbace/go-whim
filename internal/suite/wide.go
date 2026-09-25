package suite

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/cmdtab"
)

// The wide suite: what the quick one (cases.md) does not reach, run on demand
// (`whim test --wide`, `make whim-test-wide`).  Four groups:
//
//	keys  102 keystroke cases with their startup arguments (wide-cases.md)
//	ex    every Ex command, by name, from the reference's and the candidate's tables
//	argv  30 command lines (wide-argv.md)
//	pty   the editor on a pseudo-terminal: sizes, TERM, raw typing (wide-pty.md)
//
// Every case is compared as the quick suite compares: the candidate's C
// against the reference's, and the Go editor against the candidate's C, with
// the control required to move something.

//go:embed wide-cases.md
var wideCasesMD string

//go:embed wide-argv.md
var wideArgvMD string

//go:embed wide-pty.md
var widePtyMD string

// WideCase is one case of the wide suite.
type WideCase struct {
	Group, Name string
	Args        []string
	Keys        []byte
	pty         *ptySpec
}

// fenced is the lines of md's fenced block, empty ones included.
func fenced(md, file string) ([]string, error) {
	i := strings.Index(md, "```\n")
	j := strings.LastIndex(md, "\n```")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("%s has no fenced block", file)
	}
	return strings.Split(md[i+4:j], "\n"), nil
}

// args is a `|`-separated list, each part unescaped; empty is none.
func parseArgs(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var out []string
	for _, a := range strings.Split(s, "|") {
		b, err := unescape(a)
		if err != nil {
			return nil, err
		}
		out = append(out, string(b))
	}
	return out, nil
}

// WideCases is the keys, argv and pty groups; the ex group is made from the
// sources being compared (exCases).
func WideCases() ([]WideCase, error) {
	var out []WideCase
	lines, err := fenced(wideCasesMD, "wide-cases.md")
	if err != nil {
		return nil, err
	}
	for _, l := range lines {
		f := strings.Split(l, "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("wide-cases.md: %q is not name, args and keys", l)
		}
		args, err := parseArgs(f[1])
		if err != nil {
			return nil, fmt.Errorf("wide-cases.md %s: %v", f[0], err)
		}
		keys, err := unescape(f[2])
		if err != nil {
			return nil, fmt.Errorf("wide-cases.md %s: %v", f[0], err)
		}
		out = append(out, WideCase{Group: "keys", Name: f[0], Args: args, Keys: keys})
	}
	if lines, err = fenced(wideArgvMD, "wide-argv.md"); err != nil {
		return nil, err
	}
	for i, l := range lines {
		args, err := parseArgs(l)
		if err != nil {
			return nil, fmt.Errorf("wide-argv.md line %d: %v", i+1, err)
		}
		name := strings.Join(args, " ")
		if name == "" {
			name = "(none)"
		}
		out = append(out, WideCase{Group: "argv", Name: name, Args: args, Keys: []byte("\x1b:q!\r")})
	}
	if lines, err = fenced(widePtyMD, "wide-pty.md"); err != nil {
		return nil, err
	}
	for _, l := range lines {
		f := strings.Split(l, "\t")
		if len(f) != 5 {
			return nil, fmt.Errorf("wide-pty.md: %q is not name, rows, columns, TERM and keys", l)
		}
		rows, err1 := strconv.Atoi(f[1])
		cols, err2 := strconv.Atoi(f[2])
		keys, err3 := unescape(f[4])
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, fmt.Errorf("wide-pty.md %s: a size or the keys do not parse", f[0])
		}
		out = append(out, WideCase{Group: "pty", Name: f[0], Keys: keys, pty: &ptySpec{rows, cols, f[3]}})
	}
	return out, nil
}

// exCases is every Ex command name in either source's table, typed at `:`.
// Commands the phases removed answer `E492` or `not implemented`, and that is
// the behaviour recorded.
func exCases(srcs ...[]byte) ([]WideCase, error) {
	seen := map[string]bool{}
	var names []string
	for i, s := range srcs {
		ns, err := cmdtab.CommandNamesIn(s, fmt.Sprintf("source %d", i))
		if err != nil {
			return nil, err
		}
		for _, n := range ns {
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	sort.Strings(names)
	var out []WideCase
	for _, n := range names {
		out = append(out, WideCase{Group: "ex", Name: n, Keys: []byte(":" + n + "\r\x1b:q!\r")})
	}
	return out, nil
}

// runWide runs one case on bin.
func runWide(bin string, c WideCase) ([]byte, int, error) {
	if c.pty != nil {
		return RunPty(bin, c.Args, c.Keys, *c.pty)
	}
	return RunArgs(bin, c.Args, c.Keys)
}

// compareWide runs every case on a and b, as many at once as there are
// cores, and returns the cases whose output or status differ, by group.
func compareWide(cases []WideCase, a, b string) (map[string][]string, error) {
	type res struct {
		c    WideCase
		same bool
		err  error
	}
	results := make([]res, len(cases))
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
				results[i] = res{c: c, err: fmt.Errorf("%s %s on %s: %w", c.Group, c.Name, a, err)}
				return
			}
			ob, sb, err := runWide(b, c)
			if err != nil {
				results[i] = res{c: c, same: false}
				return
			}
			results[i] = res{c: c, same: sa == sb && bytes.Equal(oa, ob)}
		}(i, c)
	}
	wg.Wait()
	diff := map[string][]string{}
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		if !r.same {
			diff[r.c.Group] = append(diff[r.c.Group], r.c.Name)
		}
	}
	return diff, nil
}

var wideGroups = []string{"keys", "ex", "argv", "pty"}

// Wide runs the wide suite: the candidate's C against rev's, and the Go editor
// against the candidate's C, on every group, with the control required to be
// seen.
func Wide(w io.Writer, rev, candSrc string) error {
	start := time.Now()
	cases, err := WideCases()
	if err != nil {
		return err
	}
	b, err := prepare(rev, candSrc)
	if err != nil {
		return err
	}
	defer os.RemoveAll(b.dir)
	ex, err := exCases(b.refSrc, b.candSrc)
	if err != nil {
		return err
	}
	cases = append(cases, ex...)
	count := map[string]int{}
	for _, c := range cases {
		count[c.Group]++
	}

	seen, err := compareWide(cases, b.ref, b.ctl)
	if err != nil {
		return err
	}
	nSeen := 0
	for _, g := range seen {
		nSeen += len(g)
	}
	if nSeen == 0 {
		return fmt.Errorf("suite: THE CONTROL WENT UNSEEN in the wide suite -- %s changed to %s and no case noticed", controlOld, controlNew)
	}
	cDiff, err := compareWide(cases, b.ref, b.cand)
	if err != nil {
		return err
	}
	goDiff, err := compareWide(cases, b.cand, b.goBin)
	if err != nil {
		return err
	}
	fail := false
	for _, g := range wideGroups {
		line := fmt.Sprintf("  wide %-5s   %3d cases", g, count[g])
		switch {
		case len(cDiff[g]) > 0:
			line += fmt.Sprintf(": %d behave differently from %s: %s", len(cDiff[g]), rev, strings.Join(cDiff[g], " "))
			fail = true
		case len(goDiff[g]) > 0:
			line += fmt.Sprintf(": the Go editor differs from the C on %d: %s", len(goDiff[g]), strings.Join(goDiff[g], " "))
			fail = true
		default:
			line += fmt.Sprintf(" as %s does, and the Go editor as the C; the control seen by %d", rev, len(seen[g]))
		}
		fmt.Fprintln(w, line)
	}
	if fail {
		return fmt.Errorf("suite: behaviour moved in the wide suite")
	}
	fmt.Fprintf(w, "  wide         %d cases in %d groups; %dms\n", len(cases), len(wideGroups), time.Since(start).Milliseconds())
	return nil
}
