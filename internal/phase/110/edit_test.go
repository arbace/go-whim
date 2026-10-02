package p110

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestReads pins what the guard compares: the diagnostics the phase reads,
// and nothing it does not.
func TestReads(t *testing.T) {
	errs := "cut.c: In function 'f':\n" +
		"cut.c:3:5: error: 'EOF' undeclared (first use in this function)\n" +
		"    3 |     return \"x.c:1:1: error: no\";\n" +
		"cut.c:9:13: warning: 'g' defined but not used [-Wunused-function]\n" +
		"cut.c:10:12: warning: 'v' defined but not used [-Wunused-variable]\n" +
		"cut.c:11:13: warning: 'h' used but never defined\n" +
		"cut.c:12:7: warning: 'x' may be used uninitialized [-Wmaybe-uninitialized]\n"
	want := []string{
		"defined but not used g", "defined but not used v",
		"error: cut.c:3:5: error: 'EOF' undeclared (first use in this function)",
		"undeclared EOF", "unused function g", "unused variable v", "used but never defined h",
	}
	if got := w110Reads(errs); !reflect.DeepEqual(got, want) {
		t.Errorf("w110Reads:\n%q\nwant\n%q", got, want)
	}
	a, b := w110Differ([]string{"x", "x", "y"}, []string{"x", "z"})
	if !reflect.DeepEqual(a, []string{"x", "y"}) || !reflect.DeepEqual(b, []string{"z"}) {
		t.Errorf("w110Differ: %q and %q", a, b)
	}
}

// TestModes compiles a small cut both ways: the two read the same, and the
// control's flag makes them read differently.
func TestModes(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
	path := filepath.Join(t.TempDir(), "cut.c")
	src := "static int v;\nstatic void g(void) {}\nstatic void h(void);\nint main(void) { h(); return 0; }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	lto := w110Reads(w110Gcc(w110LTO, path))
	if len(lto) != 5 {
		t.Fatalf("the LTO compile read %q, want two unused, one unused function, one unused variable and one never defined", lto)
	}
	if plain := w110Reads(w110Gcc(nil, path)); !reflect.DeepEqual(lto, plain) {
		t.Errorf("the LTO compile reads %q, the plain one %q", lto, plain)
	}
	if plain := w110Reads(w110Gcc([]string{"-Wno-unused-function"}, path)); reflect.DeepEqual(lto, plain) {
		t.Errorf("the control moved nothing: %q", plain)
	}
}

// TestGuardControl runs the phase on the boundary it is handed, q109, as a
// build left it: once as it is, which must pass, and once with the plain
// compile's warnings about unused functions turned off -- the control,
// which the guard must refuse, naming the first cut it holds.
func TestGuardControl(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	text, err := os.ReadFile("../../../.cache/boundaries/q109.c")
	if err != nil {
		t.Skipf("no boundary to run on: %v", err)
	}
	if _, err := Edit(text, io.Discard, []string{t.TempDir()}); err != nil {
		t.Fatalf("the phase refused on its own boundary: %v", err)
	}
	defer func(was []string) { w110Plain = was }(w110Plain)
	w110Plain = []string{"-Wno-unused-function"}
	_, err = Edit(text, io.Discard, []string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "READ DIFFERENTLY on cut0.c") {
		t.Fatalf("the control was not refused: %v", err)
	}
	t.Logf("the control's refusal:\n%v", err)
}
