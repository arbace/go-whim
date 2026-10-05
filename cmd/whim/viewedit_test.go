package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/graph/view"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/suite"
)

const viewSample = "../../crefactor/graph/view/testdata/sample.c"

// printed is `whim view` of FILE as text, through the code runView runs.
func printed(t *testing.T, file, name, arg string, ids bool) string {
	t.Helper()
	g, _, err := loadGraph(file, true)
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := viewText(view.NewIndex(g), name, []string{arg}, view.Options{}, view.Printer{IDs: ids}, false)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// TestViewEditCLI: whim view-edit on the views' sample -- an edit written
// as C, a refusal that writes nothing -- and --spans' table.
func TestViewEditCLI(t *testing.T) {
	dir := t.TempDir()
	text := printed(t, viewSample, "def", "get", false)
	in := filepath.Join(dir, "edited.lisp")
	out := filepath.Join(dir, "out.c")
	write := func(s string) {
		if err := os.WriteFile(in, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(strings.Replace(text, "(+= (-> b b_ml) 2)", "(+= (-> b b_ml) 3)", 1))
	if rc := runViewEdit([]string{"--no-cache", "-i", in, "-o", out, "def", "get", viewSample}); rc != 0 {
		t.Fatalf("exit %d", rc)
	}
	c, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(c), "b->b_ml += 3;") || strings.Contains(string(c), "b->b_ml += 2;") {
		t.Fatalf("the C written:\n%s", c)
	}
	os.Remove(out)
	write(strings.Replace(text, "(= opt 0)", "(= opt nope)", 1))
	if rc := runViewEdit([]string{"--no-cache", "-i", in, "-o", out, "def", "get", viewSample}); rc != 1 {
		t.Fatalf("a refusal: exit %d", rc)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("a refusal wrote its output")
	}
	sp := filepath.Join(dir, "spans")
	stdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	rc := runView([]string{"--no-cache", "--spans", sp, "def", "get", viewSample})
	os.Stdout = stdout
	table, err := os.ReadFile(sp)
	if rc != 0 || err != nil || !strings.HasPrefix(string(table), ";; start end kind id parent whole\n0 ") {
		t.Fatalf("--spans: exit %d, %v:\n%.200s", rc, err, table)
	}
}

// TestViewEditSuite (WHIM_VIEW_EDIT_SUITE=1): a behaviour-neutral edit
// through a view of src/whim-vim.c -- ml_clearmarked's ++lnum written
// lnum += 1, and its local i renamed k -- compiled and linked with the one
// compile line, and the quick suite run on it against HEAD's.
func TestViewEditSuite(t *testing.T) {
	if os.Getenv("WHIM_VIEW_EDIT_SUITE") == "" {
		t.Skip("WHIM_VIEW_EDIT_SUITE=1 runs it (about 20 s)")
	}
	t.Chdir("../..")
	text := printed(t, "src/whim-vim.c", "def", "ml_clearmarked", false)
	ed := strings.NewReplacer("(pre++ lnum)", "(+= lnum 1)", "(def i int)", "(def k int)", "(= i ", "(= k ",
		"(pre++ i)", "(pre++ k)", "db_line) i)", "db_line) k)").Replace(text)
	dir := t.TempDir()
	in, out := filepath.Join(dir, "edited.lisp"), filepath.Join(dir, "whim-vim.c")
	if err := os.WriteFile(in, []byte(ed), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := runViewEdit([]string{"-i", in, "-o", out, "def", "ml_clearmarked"}); rc != 0 {
		t.Fatalf("view-edit: exit %d", rc)
	}
	c, _ := os.ReadFile(out)
	if !strings.Contains(string(c), "++k, lnum += 1)") {
		t.Fatal("the edit is not in the C")
	}
	g, _, err := graph.Import(out, c)
	if err != nil {
		t.Fatal(err)
	}
	if back, _ := g.C(); string(back) != string(c) {
		t.Fatal("the C written is not canonical")
	}
	cf, lf, _ := build.FlagsFor(103)
	args := append(append(append([]string{}, cf...), lf...), "-o", filepath.Join(dir, "a.out"), out)
	if b, err := exec.Command("gcc", args...).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, b)
	}
	if err := suite.Check(os.Stdout, "HEAD", out, suite.JVM{}); err != nil {
		t.Fatal(err)
	}
}
