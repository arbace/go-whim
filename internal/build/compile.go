package build

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// THE COMPILE LINE IS ONE LINE, for every boundary and for the input and the
// product alike: -O0 -fno-stack-protector -static -no-pie -s -- an ordinary
// static executable (EXEC, no dynamic section, no relocation), no stack
// protector, no -g.  It used to move twice, at record 83 (-no-pie) and 84
// (-fno-stack-protector), which is why those two phases exist; they change
// nothing now.
var (
	cflags  = []string{"-O0", "-fno-stack-protector"}
	ldflags = []string{"-static", "-no-pie", "-s"}
)

// FlagsFor is the compile line the boundary after phase n carries, which is the
// one line; n is kept so a caller says which boundary it means.
func FlagsFor(n int) ([]string, []string, error) {
	return append([]string{}, cflags...), append([]string{}, ldflags...), nil
}

// compileBoundary is Config.Compile: the boundary at path compiled and LINKED
// with the one line, judged by gcc's status -- warnings are allowed, as they
// are for the product, and an error is not.  On an error it answers with the
// compiler's first error lines.
//
// The whole line and not -fsyntax-only, which would have caught the three
// errors the snapshots had (an undeclared function twice, a deleted member)
// but not a function declared and defined nowhere, which only the link sees.
// Measured on the 104 boundaries at 64 jobs: -fsyntax-only 36 s of CPU and
// under a second; the whole line 6.6 min of CPU and 8-9 s, of a check of
// 63-71 s.  Every boundary is warning-free under it, too, but that is not
// held.
func compileBoundary(path, scratch string) error {
	args := append(append(append([]string{}, cflags...), ldflags...), "-o", filepath.Join(scratch, "a.out"), path)
	out, err := exec.Command("gcc", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	var errs []string
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.Contains(ln, "error") {
			errs = append(errs, ln)
		}
	}
	if len(errs) == 0 {
		errs = append(errs, err.Error())
	}
	n := len(errs)
	if n > 3 {
		errs = append(errs[:3], fmt.Sprintf("... %d error lines in all", n))
	}
	return fmt.Errorf("%s", strings.Join(errs, "\n               "))
}
