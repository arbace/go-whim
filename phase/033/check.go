package p033

// Whim phase 33, the check -- commands whose machinery has already gone.
// See phase/033/edit.go, and GOALS.md.
//
// Runs after phase/033/edit.go and the sweep internal/verify runs between
// them, and reads nothing from the edit's shell -- only the work tree and the state
// directory, which is what internal/verify hands a check.

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim33", Check) }

// Whim33 is phase 33's check: commands whose machinery has already gone.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim33 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "deadcmds", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src := []byte(check.ReadFile(f))
	for _, g := range []string{"ex_shell", "ex_nogui", "ex_digraphs", "ex_redrawtabpanel", "ex_colorscheme", "load_colors"} {
		if n := check.CountWord(src, g); n != 0 {
			r.Say("%s still has %d mentions after the sweep", g, n)
			re := regexp.MustCompile(`\b` + g + `\b`)
			k := 0
			for i, l := range strings.Split(string(src), "\n") {
				if re.MatchString(l) {
					line := fmt.Sprintf("               %d:%s", i+1, l)
					if len(line) > 100 {
						line = line[:100]
					}
					fmt.Fprintln(w, line)
					if k++; k == 3 {
						break
					}
				}
			}
			return harness.ErrReported
		}
	}
	// tools/cutil.py's find_definition, as the Go package the sweep runs.  A
	// definition that is not there is a pass, as it was in the heredoc, whose
	// unpacking of None raised and exited 1 -- which the shell read as "no
	// quickfix command found".
	if a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), "ex_listdo"); ok &&
		regexp.MustCompile(`\bCMD_(cdo|cfdo|ldo|lfdo)\b`).Match(src[a:z]) {
		r.Say("ex_listdo still tests for a quickfix command")
		return harness.ErrReported
	}
	r.Say("no handler that only refused is left, and ex_listdo asks nothing about quickfix")
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	if err := check.Run(w, "sh", "tools/phasebuild.sh", work, beforeLines); err != nil {
		return harness.ErrReported
	}
	return nil
}
