package build

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

// TestPlanNumbers holds the plan to its numbering: a phase's N is its place
// in the plan, so the numbers run 0, 1, 2, ... without a gap, and each names
// a directory, internal/phase/NNN/, that says what the phase is.  A program a
// phase's steps call by name is the phase's own, whimN (or a second entry of
// its package: whimNkp), or one of its parts, whimNx in internal/phase/NNN/x/.
// One program is called from an earlier phase: whim18, which phase 2's front
// runs (internal/steps), and which is not in the plan's steps.
func TestPlanNumbers(t *testing.T) {
	name := regexp.MustCompile(`^whim(\d+)([a-z]?)(kp|ep|bl|rows)?$`)
	for k, p := range Plan {
		if p.N != k {
			t.Fatalf("plan entry %d is numbered %d: the numbers are the plan's places", k, p.N)
		}
		dir := fmt.Sprintf("../phase/%03d", p.N)
		if _, err := os.Stat(dir + "/GOAL.md"); err != nil {
			t.Errorf("phase %d: %v", p.N, err)
		}
		for _, s := range p.Steps {
			if s.Op != "edit" {
				continue
			}
			m := name.FindStringSubmatch(s.Args[0])
			if m == nil {
				t.Errorf("phase %d calls %q, which is no phase's program", p.N, s.Args[0])
				continue
			}
			if m[1] != fmt.Sprint(p.N) {
				t.Errorf("phase %d calls %s, another phase's program", p.N, s.Args[0])
			}
			if m[2] != "" {
				if _, err := os.Stat(dir + "/" + m[2] + "/edit.go"); err != nil {
					t.Errorf("phase %d calls its part %s: %v", p.N, s.Args[0], err)
				}
			}
		}
	}
}
