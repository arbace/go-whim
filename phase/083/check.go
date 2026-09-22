package p083

// Whim phase 83, the check -- the core's compile line, and the baselines it is
// measured against.
//
// THE COMPILE LINE IS THE BOUNDARY'S, and from here it is the core's:
// internal/build writes tools/templates/core.mk over the work tree's makefile,
// `gcc -O0 -static -no-pie -s`, and this requires what that line is for -- a
// binary a host can place without a loader.  `readelf` says EXEC, with no
// INTERP, no dynamic section and no relocations; all four, because any one of
// them alone can hold while the binary is still not absolutely static.
//
// The baselines this phase used to record are `whimtools record` now
// (internal/verify), which builds q82 to record them from the same binary.

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim83", Check) }

var elfType = regexp.MustCompile(`(?m)^\s*Type:\s+(\S+)`)

// Check requires the binary this phase's compile line produces to be
// absolutely static.
func Check(w io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: check whim83 <work-dir> <state-dir>")
	}
	work, state := args[0], ""
	if len(args) > 1 {
		state = args[1]
	}
	r := &check.Rep{Tag: "static", W: w}
	bin := filepath.Join(work, "whim-vim")
	// The line count of the text this phase was handed, which is what
	// tools/phasebuild.sh reports the build against.
	before := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	if before == "" {
		before = "0"
	}
	if err := check.Run(w, "sh", "tools/phasebuild.sh", work, before); err != nil {
		return harness.ErrReported
	}
	ask := func(flag, want string) (int, error) {
		out, err := exec.Command("readelf", flag, bin).CombinedOutput()
		if err != nil {
			return 0, err
		}
		return strings.Count(string(out), want), nil
	}
	head, err := exec.Command("readelf", "-h", bin).CombinedOutput()
	if err != nil {
		return err
	}
	m := elfType.FindStringSubmatch(string(head))
	if m == nil {
		r.Say("readelf -h says nothing about the type of %s", bin)
		return harness.ErrReported
	}
	interp, err := ask("-l", "INTERP")
	if err != nil {
		return err
	}
	dyn, err := ask("-d", "There is no dynamic section in this file.")
	if err != nil {
		return err
	}
	rel, err := ask("-r", "There are no relocations in this file.")
	if err != nil {
		return err
	}
	if m[1] != "EXEC" || interp != 0 || dyn != 1 || rel != 1 {
		r.Say("NOT absolutely static: type %s, INTERP %d, no-dynamic %d, no-relocations %d",
			m[1], interp, dyn, rel)
		r.Cont("the core's compile line is gcc -O0 -static -no-pie -s (tools/templates/core.mk)")
		return harness.ErrReported
	}
	r.Say("EXEC, no INTERP, no dynamic section, 0 relocations, %d bytes", check.SizeOf(bin))
	return nil
}
