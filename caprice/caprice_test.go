package caprice

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Editors are instances: testdata/instances/Main.hs runs four at once in one
// process, each on a host of its own, and requires each to exit 0 and its
// screen to show its own text and no other's -- editor/host_test.go's
// TestEditorsAreInstances. It compiles against the build `make bin/caprice`
// leaves in lib/caprice, whose objects it shares; without one it skips.
func TestEditorsAreInstances(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "lib", "caprice"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "Caprice", "Editor.hs")); err != nil {
		t.Skip("no caprice build in lib/caprice (make bin/caprice)")
	}
	if _, err := exec.LookPath("ghc"); err != nil {
		t.Skip("no ghc")
	}
	main, err := filepath.Abs(filepath.Join("testdata", "instances", "Main.hs"))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := CompileMain(dir, filepath.Join(t.TempDir(), "instances"), main)
	if err != nil {
		t.Fatal(err)
	}
	out, err := command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Logf("%s", out)
}
