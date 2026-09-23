package verify

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/harness"
)

// THE DECLARATIONS ARE THE OTHER HALF OF EVERY DELTA, and this is what reads
// them.  It was tools/declared.sh, and the fold below was the awk inside
// tools/whimdelta.sh.
//
// Each phase declares, in advance, how its binary may move: phase/NNN/delta.md,
// a token per way inside a FENCED BLOCK, prose around it saying why, and no
// block at all for a phase that declares nothing (the words are GOALS.md's,
// *What is measured*).  The fence is what separates the data from the prose:
// everything outside one is a note, and a phase that declares nothing is all
// note.

// Declarations is the declared delta of phases from..to, in the one grammar
// both delta checkers read -- `N  tokens` for a phase's first line,
// continuation lines indented, notes and blank lines left out -- so a checker
// reads one list however the phases keep theirs.
//
// from and to are plain integers; the directory is named with %03d, and never
// with arithmetic on a padded name, which a shell would read as octal.
func Declarations(from, to int) (string, error) {
	var b strings.Builder
	for p := from; p <= to; p++ {
		data, err := os.ReadFile(fmt.Sprintf("phase/%03d/delta.md", p))
		if err != nil {
			if os.IsNotExist(err) {
				continue // a phase without a delta.md declares nothing
			}
			return "", err
		}
		fence, seen := false, false
		for _, line := range textLines(string(data)) {
			if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
				fence = !fence
				continue
			}
			if !fence || len(strings.Fields(line)) == 0 {
				continue
			}
			s := strings.TrimLeft(line, " \t")
			if seen {
				fmt.Fprintf(&b, "      %s\n", s)
			} else {
				fmt.Fprintf(&b, "%-6s%s\n", strconv.Itoa(p), s)
				seen = true
			}
		}
	}
	return b.String(), nil
}

// PhaseDeclared is what phase n itself declares, a token per line: the list a
// phase program asserts its own table against.  From build.CoreFrom on the
// tokens are zero's and the reader is the core's, which is the same split
// every other form of the delta makes.
func PhaseDeclared(n int) ([]string, error) {
	if n >= build.CoreFrom {
		text, err := Declarations(build.CoreFrom, n)
		if err != nil {
			return nil, err
		}
		_, own, err := harness.ZDeclaredText(text, n)
		return own, err
	}
	text, err := Declarations(0, n)
	if err != nil {
		return nil, err
	}
	return fold(text, n).own, nil
}

// decl is a run of declarations folded to what holds AT one phase.
//
// THE FOLD IS CUMULATIVE, because the list up to a phase is the whole
// difference from slim at that phase: a command is added by its name and taken
// out again by drop:name, a behaviour case by case:name and drop:case:name,
// and the terminal table by term-moved.  A phase that removes a command and a
// later phase that removes the command that replaced it are two lines, not one
// edited line -- so what a phase declared stays readable after the phase that
// undoes it.
type decl struct {
	cmds  map[string]bool // Ex command names
	cases map[string]bool // behaviour cases
	term  bool            // the terminal table
	own   []string        // what phase n itself declares, in order, commands only
}

// fold reads Declarations' output and folds every line for a phase <= n.
//
// own is the phase's own commands: not case:, not drop: and not term-moved,
// in the order they are written and with duplicates kept, because it is a list
// a phase program reads and not a set.
func fold(text string, n int) decl {
	d := decl{cmds: map[string]bool{}, cases: map[string]bool{}}
	phase := 0
	for _, line := range textLines(text) {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		i := 0
		if line[0] >= '0' && line[0] <= '9' {
			phase = leadingInt(f[0])
			i = 1
		}
		if phase > n {
			continue
		}
		for ; i < len(f); i++ {
			w := f[i]
			if phase == n && !strings.HasPrefix(w, "case:") &&
				!strings.HasPrefix(w, "drop:") && w != "term-moved" {
				d.own = append(d.own, w)
			}
			switch {
			case w == "term-moved":
				d.term = true
			case strings.HasPrefix(w, "drop:case:"):
				delete(d.cases, w[len("drop:case:"):])
			case strings.HasPrefix(w, "drop:"):
				delete(d.cmds, w[len("drop:"):])
			case strings.HasPrefix(w, "case:"):
				d.cases[w[len("case:"):]] = true
			default:
				d.cmds[w] = true
			}
		}
	}
	return d
}

// leadingInt is the number a declaration line opens with.
func leadingInt(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(s[:i])
	return n
}

// textLines is the lines of a file: a trailing newline ends the last line and
// does not begin an empty one.
func textLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}
