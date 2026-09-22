package p081

// Whim phase 81, the check -- one line, one command.
// See phase/081/edit.go, and GOALS.md.
//
// Runs after phase/081/edit.go and the sweep tools/phaserun.sh runs between
// them, and reads nothing from the edit's shell -- only the work tree and the state
// directory, as tools/phaserun.sh describes.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim81", Check) }

// Whim81 is phase 81's check: one line, one command.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim81 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "onecommand", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src := []byte(check.ReadFile(f))
	for _, g := range []string{"comment_start", "starts_with_colon"} {
		if n := check.CountWord(src, g); n != 0 {
			r.Say("%s still has %d mentions", g, n)
			return harness.ErrReported
		}
	}
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	if err := check.Run(w, "sh", "tools/phasebuild.sh", work, beforeLines); err != nil {
		return harness.ErrReported
	}

	d, err := os.MkdirTemp("", "whim81")
	if err != nil {
		return err
	}
	defer os.RemoveAll(d)
	home, _ := os.MkdirTemp(d, "onecommand-home-")
	env := check.WhimEnv(home)
	oldV, e1 := harness.Stage(filepath.Join(state, "old"))
	newV, e2 := harness.Stage(filepath.Join(work, "whim-vim"))
	if e1 != nil || e2 != nil {
		return fmt.Errorf("staging the two binaries failed")
	}
	const (
		bar   = "a bar is argument text"
		quote = "a quote is argument text"
		esc   = "a backslash before a bar stays"
	)
	type cse struct {
		cmds []string
		why  string
	}
	differ := []cse{
		{[]string{"%s/a/X/|%s/b/Y/"}, bar}, {[]string{"set ts=3|%s/a/X/"}, bar}, {[]string{"1d|1d"}, bar},
		{[]string{"2|"}, bar}, {[]string{"|"}, bar}, {[]string{"1a|new"}, bar},
		{[]string{"map Q A|b", "normal Q"}, bar}, {[]string{"nmap Q A|b", "normal Q"}, bar},
		{[]string{"\" a comment"}, quote}, {[]string{"\""}, quote}, {[]string{"set ts=3 \" a comment"}, quote},
		{[]string{"%s/a/X/ \" a comment"}, quote}, {[]string{"1d \" a comment"}, quote},
		{[]string{"map Q A\\|b", "normal Q"}, esc},
	}
	same := [][]string{
		{"%s/a/X/"}, {"%s/a/X/", "%s/b/Y/"}, {"%s/a\\|b/Q/g"}, {"g/a\\|c/d"}, {"g/b/s/a/Z/"},
		{"%s/a/\"/"}, {"%s/\"/q/"}, {"normal! A\"x"}, {"normal! A|x"}, {"map Q AX", "normal Q"},
		{"map Q A\"b", "normal Q"}, {"map Q A\x16|b", "normal Q"}, {"set ts=3"}, {"2,3d"}, {"2d 2"},
		{"%s/a/X/\n%s/b/Y/"}, {"1d\n1d"}, {"$"}, {"2"}, {"%p"}, {"1a"}, {"%j"},
		{"map Q A\\\"b", "normal Q"}, {"let x = 1"}, {"echo \"x\""}, {"@\""}, {"2*"}, {"undo"}, {"map Q A b ", "normal Q"},
	}
	key := func(c []string) string { return strings.Join(c, "\x00") }
	all := map[string][]string{}
	isDiffer := map[string]bool{}
	for _, c := range differ {
		all[key(c.cmds)] = c.cmds
		isDiffer[key(c.cmds)] = true
	}
	for _, c := range same {
		all[key(c)] = c
	}
	// Python sorts the tuples themselves: element by element, a shorter tuple
	// first when it is a prefix.
	var todo [][]string
	for _, c := range all {
		todo = append(todo, c)
	}
	less := func(a, b []string) bool {
		for i := 0; i < len(a) && i < len(b); i++ {
			if a[i] != b[i] {
				return a[i] < b[i]
			}
		}
		return len(a) < len(b)
	}
	sort.Slice(todo, func(i, j int) bool { return less(todo[i], todo[j]) })
	argv := func(c []string) []string {
		var a []string
		for _, x := range c {
			a = append(a, "+"+x)
		}
		return append(a, "+w! out.txt", "+q!")
	}
	oldR, newR := make([]check.WhimRes, len(todo)), make([]check.WhimRes, len(todo))
	check.WhimPool(len(todo), func(i int) { oldR[i] = check.WhimRun(oldV, d, env, argv(todo[i]), "out.txt") })
	check.WhimPool(len(todo), func(i int) { newR[i] = check.WhimRun(newV, d, env, argv(todo[i]), "out.txt") })
	idx := map[string]int{}
	var gotSet = map[string]bool{}
	for i, c := range todo {
		idx[key(c)] = i
		if !oldR[i].Eq(newR[i], false) {
			gotSet[key(c)] = true
		}
	}
	mismatch := len(gotSet) != len(isDiffer)
	for k := range gotSet {
		if !isDiffer[k] {
			mismatch = true
		}
	}
	tupleRepr := func(c []string) string {
		q := make([]string, len(c))
		for i, x := range c {
			q[i] = check.PyRepr(x)
		}
		if len(q) == 1 {
			return "(" + q[0] + ",)"
		}
		return "(" + strings.Join(q, ", ") + ")"
	}
	resRepr := func(x check.WhimRes) string {
		if x.Timeout {
			return "('TIMEOUT',)"
		}
		b := "None"
		if x.Body != nil {
			b = check.PyRepr(*x.Body)
		}
		return fmt.Sprintf("(%s, %q, %s)", x.Rc, x.Stderr, b)
	}
	if mismatch {
		fmt.Fprintf(w, "  onecommand   old and new binaries differ on %d cases, expected %d:\n", len(gotSet), len(differ))
		for i, c := range todo {
			k := key(c)
			if gotSet[k] != isDiffer[k] {
				fmt.Fprintf(w, "                 %-36s old %s\n", tupleRepr(c), resRepr(oldR[i]))
				fmt.Fprintf(w, "                 %-36s new %s\n", "", resRepr(newR[i]))
			}
		}
		return harness.ErrReported
	}
	for _, p := range []struct {
		c    []string
		Body string
	}{{[]string{"map Q A|b", "normal Q"}, "a\nba\nca|b\n"}, {[]string{"map Q A\\|b", "normal Q"}, "a\nba\nca\\|b\n"}} {
		n := newR[idx[key(p.c)]]
		if n.Body == nil || *n.Body != p.Body {
			b := "None"
			if n.Body != nil {
				b = check.PyRepr(*n.Body)
			}
			r.Say("%s wrote %s, expected %s", tupleRepr(p.c), b, check.PyRepr(p.Body))
			return harness.ErrReported
		}
	}
	bi := idx[key([]string{"%s/a/X/|%s/b/Y/"})]
	if o, n := oldR[bi], newR[bi]; o.Body == nil || *o.Body != "X\nYX\ncX\n" || n.Body == nil || *n.Body != "a\nba\nca\n" {
		fmt.Fprintf(w, "  onecommand   the bar case did not show a split before and none after: %s -> %s\n", check.OptRepr(o.Body), check.OptRepr(n.Body))
		return harness.ErrReported
	}
	r.Say("%d cases through both binaries: %d differ exactly as declared, %d identical", len(todo), len(differ), len(same))
	return nil
}
