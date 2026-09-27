package braaam

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// `whim java` builds the editor in Java from the tracked product and writes
// a launcher that runs it: the JVM starts, the core's class loads and its
// state is made, and the editor either runs a session to its :q! or stops at
// a function the Java backend has not written yet (milestone 2), which the
// launcher reports on stderr with status 70.
func TestLauncherBuildsAndRuns(t *testing.T) {
	for _, tool := range []string{"javac", "java", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "src", "whim-vim.c")); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	cmd := exec.Command("go", "tool", "whim", "java", "--out", filepath.Join(dir, "java"))
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("whim java: %v\n%s", err, out)
	}
	bin := filepath.Join(dir, "java", "braaam") // with --out DIR, the launcher is DIR's
	run := exec.Command(bin)
	run.Stdin = strings.NewReader(":q!\r")
	out, err := run.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	// Every function of the core is written (JAVA.md, milestone 2): the
	// editor starts, reads the keys and quits with 0.
	if code != 0 {
		t.Errorf("the launcher: status %d\n%s", code, out)
	}
}
