package dead

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

// The fixture, testdata/reach.c, is a program in the shape the canonical
// print leaves -- the name at column 0, the return type on the line above --
// with a function main calls, one reached only through a table, one nothing
// calls, and two external functions that call only each other: a dead cycle,
// which gcc cannot see (nothing external is unused to it) and reachability
// can.

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/reach.c")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func needGcc(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
	}
}

// Every definition is found with its return type's line, and nothing that
// is not a definition: a prototype, a call, the table.
func TestFuncDefinitionsFindsEveryDefinitionWithItsType(t *testing.T) {
	src := fixture(t)
	defs := FuncDefinitions(src, edit.Blank(src))
	if len(defs) != 6 {
		t.Errorf("%d definitions, want 6: %v", len(defs), defs)
	}
	for name, head := range map[string]string{
		"helper": "static int\nhelper(int x)\n{",
		"ping":   "int\nping(int n)\n{",
		"orphan": "static int\norphan(void)\n{",
		"main":   "int\nmain(void)\n{",
	} {
		s, ok := defs[name]
		if !ok {
			t.Errorf("%s is not found", name)
			continue
		}
		body := string(src[s[0]:s[1]])
		if !strings.HasPrefix(body, head) || !strings.HasSuffix(body, "}") {
			t.Errorf("%s spans %q", name, body)
		}
	}
}

// A definition whose line above ends a statement or a block does not take
// that line as its type, so two spans never overlap; a brace in a string
// or a comment does not end a body.
func TestFuncDefinitionsTakesOnlyATypeLine(t *testing.T) {
	src := []byte("int x;\nf(void)\n{\n    return \"}\";\n}\ng(void)\n{\n    /* } */\n    return 0;\n}\n")
	defs := FuncDefinitions(src, edit.Blank(src))
	if got := string(src[defs["f"][0]:defs["f"][1]]); got != "f(void)\n{\n    return \"}\";\n}" {
		t.Errorf("f spans %q", got)
	}
	if got := string(src[defs["g"][0]:defs["g"][1]]); got != "g(void)\n{\n    /* } */\n    return 0;\n}" {
		t.Errorf("g spans %q", got)
	}
}

// Reachability from the roots: what main calls, what the table outside every
// body names, and not what only a prototype names -- so the dead cycle is
// dead.  The roots are the caller's: another root keeps what it reaches.
func TestFuncReach(t *testing.T) {
	src := fixture(t)
	for _, tc := range []struct {
		roots     []string
		reachable int
		dead      string
		lines     int
	}{
		{[]string{"main"}, 3, "orphan ping pong", 12},
		{[]string{"main", "ping"}, 5, "orphan", 4},
		{[]string{"orphan"}, 3, "main ping pong", 13},
		// no root: only what the table names
		{nil, 1, "helper main orphan ping pong", 21},
	} {
		defs, reachable, dead, lines := FuncReach(src, tc.roots)
		if len(defs) != 6 || reachable != tc.reachable || strings.Join(dead, " ") != tc.dead || lines != tc.lines {
			t.Errorf("roots %v: %d defs, %d reachable, dead %v in %d lines; want %d, %q, %d",
				tc.roots, len(defs), reachable, dead, lines, tc.reachable, tc.dead, tc.lines)
		}
	}
}

// DeleteFuncs takes the dead definitions and the blank lines after them, and
// what is left compiles without a word and prints what the original did.
func TestDeleteFuncsLeavesAProgramThatCompilesSilently(t *testing.T) {
	needGcc(t)
	src := fixture(t)
	defs, _, dead, _ := FuncReach(src, []string{"main"})
	out := DeleteFuncs(src, defs, dead)
	for _, gone := range []string{"orphan", "ping(int n)\n{", "pong(int n)\n{"} {
		if strings.Contains(string(out), gone) {
			t.Errorf("%q survives the deletion", gone)
		}
	}
	if strings.Contains(string(out), "}\n\n\n") {
		t.Errorf("a deletion left a double blank line:\n%s", out)
	}
	if again, _, dead, _ := FuncReach(out, []string{"main"}); len(dead) != 0 || len(again) != 3 {
		t.Errorf("after the deletion, %d defs and dead %v", len(again), dead)
	}
	dir := t.TempDir()
	run := func(name string, text []byte) string {
		p, bin := filepath.Join(dir, name+".c"), filepath.Join(dir, name)
		os.WriteFile(p, text, 0o644)
		b, err := exec.Command("gcc", "-std=gnu2x", "-Wall", "-Wextra", "-o", bin, p).CombinedOutput()
		if err != nil || (name == "after" && len(b) > 0) {
			t.Fatalf("gcc %s: %v\n%s", name, err, b)
		}
		got, err := exec.Command(bin).Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(got)
	}
	if before, after := run("before", src), run("after", out); after != before || after != "8 1\n" {
		t.Errorf("the program printed %q, and after the deletion %q", before, after)
	}
}

// GccWarnings reads gcc's call graph: the static function nothing calls is
// named, by its line -- and the dead external cycle is not, which is why
// FuncReach exists.  A file with nothing unused gives nothing.
func TestGccWarningsNamesTheUnusedStatic(t *testing.T) {
	needGcc(t)
	w, err := GccWarnings("testdata/reach.c", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 1 || w[0].Line != 25 || !strings.Contains(w[0].Text, "'orphan' defined but not used") {
		t.Errorf("warnings %v", w)
	}
	src := fixture(t)
	defs, _, dead, _ := FuncReach(src, []string{"main"})
	p := filepath.Join(t.TempDir(), "after.c")
	os.WriteFile(p, DeleteFuncs(src, defs, dead), 0o644)
	if w, err := GccWarnings(p, ""); err != nil || len(w) != 0 {
		t.Errorf("after the deletion: %v %v", w, err)
	}
}

// A file-scope object nothing reads is the other warning read; gcc's status
// is not consulted, so a file that fails still answers.
func TestGccWarningsAnswersForAFileThatFails(t *testing.T) {
	needGcc(t)
	p := filepath.Join(t.TempDir(), "bad.c")
	os.WriteFile(p, []byte("static int unread;\nstatic void\nnobody(void)\n{\n}\nint\nmain(void)\n{\n    return missing;\n}\n"), 0o644)
	w, err := GccWarnings(p, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, x := range w {
		got = append(got, x.Text)
	}
	s := strings.Join(got, "\n")
	if !strings.Contains(s, "'nobody' defined but not used") || !strings.Contains(s, "'unread' defined but not used") {
		t.Errorf("warnings %q", got)
	}
}

// With keep, what gcc said is kept beside the digest of the file it said it
// of, and nothing else.
func TestGccWarningsKeepsStderrAndDigest(t *testing.T) {
	needGcc(t)
	keep := filepath.Join(t.TempDir(), "keep")
	if _, err := GccWarnings("testdata/reach.c", keep); err != nil {
		t.Fatal(err)
	}
	txt, err := os.ReadFile(filepath.Join(keep, "last.txt"))
	if err != nil || !strings.Contains(string(txt), "orphan") {
		t.Errorf("last.txt is %q (%v)", txt, err)
	}
	sum := sha256.Sum256(fixture(t))
	if sha, _ := os.ReadFile(filepath.Join(keep, "last.sha")); string(sha) != hex.EncodeToString(sum[:])+"\n" {
		t.Errorf("last.sha is %q", sha)
	}
	if ents, _ := os.ReadDir(keep); len(ents) != 2 {
		t.Errorf("keep holds %d files", len(ents))
	}
}
