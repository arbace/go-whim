package suite

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Clojure editor's hookup, on cljeditor's stand-in namespace (no
// editor, but every host function reached through the glue): the stand-in
// and its control built as `whim test --clojure` builds them, the control
// required to move its answers on the cases that type an i, and the
// launcher's report of an exception read as the report reads it -- the
// exception and the core's function it came from.
func TestClojureStandIn(t *testing.T) {
	for _, tool := range []string{"javac", "java", "clojure"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "cljeditor", "testdata", "standin", "whim", "editor.clj"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cljOut := filepath.Join(dir, "clj", "editor.clj")
	if err := os.MkdirAll(filepath.Dir(cljOut), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cljOut, src, 0o644); err != nil {
		t.Fatal(err)
	}
	ctlSrc, err := writeControl(src, "editor.clj", filepath.Join(dir, "clj-control"))
	if err != nil {
		t.Fatal(err)
	}
	e := &jvmEditor{name: "Clojure", where: "cljeditor/", file: "editor.clj", launcher: "whim-clj",
		frame: regexp.MustCompile(`^\tat whim\.editor\$([A-Za-z0-9_]+)`)}
	if _, err := compileClojure(e, cljOut, ctlSrc, dir); err != nil {
		t.Fatal(err)
	}
	cases := []WideCase{
		{Group: "keys", Name: "typed", Keys: []byte("ihello\x1b:q!\r")},
		{Group: "keys", Name: "untyped", Keys: []byte("x")},
	}
	rs, err := compareEach(cases, e.bin, e.ctl)
	if err != nil {
		t.Fatal(err)
	}
	if rs[0].same || !rs[1].same {
		t.Errorf("the control moved %v and %v; want the typed case only", !rs[0].same, !rs[1].same)
	}
	out, code, err := RunArgs(e.bin, nil, []byte("T"))
	if err != nil {
		t.Fatal(err)
	}
	got := failures([]result{{c: cases[0], outB: out}}, e)
	want := "1 thrown in vim_main: java.lang.IllegalStateException: the stand-in was told to throw"
	if code != 70 || len(got) != 1 || got[0] != want {
		t.Errorf("status %d, failures %q; want 70 and %q\n%s", code, got, want, out)
	}
	if !strings.Contains(e.label("wide "), "wide clj") {
		t.Errorf("label %q", e.label("wide "))
	}
}
