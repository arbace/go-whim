package sweep

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/internal/cemit"
)

// TestForeign is the library on C that is not vim (testdata/foreign.c): the
// canonical print and the sweep, told the program's own root, must leave a
// program that compiles without a warning and prints what the original did --
// and must take what is dead: a function, an object, a member, a local.  The
// file keeps YELLOW after a dead enumerator, so the sweep's pinning is what
// keeps its value 4.
func TestForeign(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	src, err := os.ReadFile("testdata/foreign.c")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	run := func(name string, c []byte) string {
		p := filepath.Join(dir, name+".c")
		os.WriteFile(p, c, 0o644)
		bin := filepath.Join(dir, name)
		out, err := exec.Command("gcc", "-std=gnu2x", "-Wall", "-Wextra", "-o", bin, p).CombinedOutput()
		if err != nil {
			t.Fatalf("gcc %s: %v\n%s", name, err, out)
		}
		if name == "after" && len(out) > 0 {
			t.Errorf("the swept program compiles with warnings:\n%s", out)
		}
		got, err := exec.Command(bin).Output()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(got)
	}
	before := run("before", src)

	canon, err := cemit.Canonical(filepath.Join(dir, "foreign.c"), src)
	if err != nil {
		t.Fatal(err)
	}
	again, err := cemit.Canonical(filepath.Join(dir, "foreign.c"), canon)
	if err != nil || !bytes.Equal(again, canon) {
		t.Fatalf("the canonical print is not a fixed point (%v)", err)
	}
	swept, _, err := Prune(canon, filepath.Join(dir, "foreign.c"), Options{Roots: []string{"main"}})
	if err != nil {
		t.Fatal(err)
	}
	if after := run("after", swept); after != before {
		t.Fatalf("the swept program prints\n%s\nwhere the original printed\n%s", after, before)
	}
	for _, gone := range []string{"unused_helper", "dead_counter", "never_read", "unused_local", "UNUSED_COLOR"} {
		if strings.Contains(string(swept), gone) {
			t.Errorf("%s survives the sweep", gone)
		}
	}
	if !strings.Contains(string(swept), "YELLOW = 4") {
		t.Errorf("YELLOW is not pinned to 4 after UNUSED_COLOR goes:\n%s", swept)
	}
}
