package wpp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/internal/whim"
)

// layoutEntry is a line of the backend's layout listing
// (editor.cpp.layout): a struct's size, or a member's offset, as the C
// front end lays them out.
type layoutEntry struct {
	size bool
	c    string
	path string
	n    int64
}

func readLayout(t *testing.T, path string) []layoutEntry {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var es []layoutEntry
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		var e layoutEntry
		var err error
		switch {
		case len(f) == 3 && f[0] == "size":
			e = layoutEntry{size: true, c: strings.ReplaceAll(f[1], "~", " ")}
			e.n, err = strconv.ParseInt(f[2], 10, 64)
		case len(f) == 4 && f[0] == "offset":
			e = layoutEntry{c: strings.ReplaceAll(f[1], "~", " "), path: f[2]}
			e.n, err = strconv.ParseInt(f[3], 10, 64)
		default:
			t.Fatalf("a layout line of no form: %q", sc.Text())
		}
		if err != nil {
			t.Fatal(err)
		}
		es = append(es, e)
	}
	return es
}

// asserts are the listing as static assertions: sizeof and offsetof.
func asserts(es []layoutEntry) string {
	var b strings.Builder
	for i, e := range es {
		if e.size {
			fmt.Fprintf(&b, "static_assert(sizeof(%s) == %d, \"%d\");\n", e.c, e.n, i)
		} else {
			fmt.Fprintf(&b, "static_assert(__builtin_offsetof(%s, %s) == %d, \"%d\");\n", e.c, e.path, e.n, i)
		}
	}
	return b.String()
}

// TestLayout holds the C++ structs to the C's: every struct and union of
// the core that C names at file scope -- its size, and each member's
// offset, through the members of a type with no name of its own -- as the
// C front end lists them, which gcc's sizeof and offsetof on the C must
// give and g++'s on the generated header must give too -- but for the
// structs that hold a pointer to a function, which C++'s pointer to a
// member function makes two words wide (doc/CPP.md).  The control: one
// offset moved by one is refused by both.
func TestLayout(t *testing.T) {
	for _, tool := range []string{"gcc", "g++"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	c, err := os.ReadFile(filepath.Join("..", "src", "whim-vim.c"))
	if err != nil {
		t.Skip("no src/whim-vim.c")
	}
	core, err := whim.Cut(c)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	editorC := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(editorC, core, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "editor.cpp")
	if rc := togo.Run([]string{editorC, dir, "-cpp", out}, io.Discard, whim.Gen); rc != 0 {
		t.Fatalf("the C++ backend: status %d", rc)
	}
	es := readLayout(t, out+".layout")
	// the structs that hold a pointer to a function, whose layout is not
	// C's: C++'s pointer to a member function is two words
	fp, err := os.ReadFile(out + ".fnptrs")
	if err != nil {
		t.Fatal(err)
	}
	fnptrs := map[string]bool{}
	for _, n := range strings.Split(strings.TrimSpace(string(fp)), "\n") {
		fnptrs[n] = true
	}
	var same []layoutEntry
	for _, e := range es {
		if !fnptrs[e.c] {
			same = append(same, e)
		}
	}
	t.Logf("%d structs hold a pointer to a function: %s", len(fnptrs), strings.Join(strings.Fields(strings.ReplaceAll(string(fp), "struct ", "struct~")), ", "))
	structs, members := 0, 0
	for _, e := range es {
		if e.size {
			structs++
		} else {
			members++
		}
	}
	if structs < 100 {
		t.Fatalf("the listing has %d structs: too few for the core", structs)
	}
	t.Logf("%d structs and unions, %d members", structs, members)
	check := func(es, same []layoutEntry) (gccErr, gppErr error) {
		f := filepath.Join(dir, "layout.c")
		if err := os.WriteFile(f, append(append([]byte{}, core...), asserts(es)...), 0o644); err != nil {
			t.Fatal(err)
		}
		if o, err := exec.Command("gcc", "-fsyntax-only", "-w", f).CombinedOutput(); err != nil {
			gccErr = fmt.Errorf("%v\n%s", err, firstLines(o, 20))
		}
		g := filepath.Join(dir, "layout.cpp")
		src := "#include \"editor.hpp\"\nnamespace whimpp {\n" + asserts(same) + "}\n"
		if err := os.WriteFile(g, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("g++", "-std=c++23", "-fsyntax-only", "-w", g)
		cmd.Dir = dir
		if o, err := cmd.CombinedOutput(); err != nil {
			gppErr = fmt.Errorf("%v\n%s", err, firstLines(o, 20))
		}
		return
	}
	if gccErr, gppErr := check(es, same); gccErr != nil || gppErr != nil {
		t.Errorf("the layouts are not the listing's: gcc %v; g++ %v", gccErr, gppErr)
	}
	bad := append([]layoutEntry{}, same...)
	for i := range bad {
		if !bad[i].size && bad[i].n > 0 {
			bad[i].n++
			break
		}
	}
	if gccErr, gppErr := check(bad, bad); gccErr == nil || gppErr == nil {
		t.Error("the control: a wrong offset was taken")
	}
}

func firstLines(b []byte, n int) string {
	ls := strings.SplitN(string(b), "\n", n+1)
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}

// Editors are instances: testdata/instances/main.cpp runs four at once in
// one process, each on a thread and a host of its own, and requires each
// to exit 0 and its screen to show its own text and no other's --
// editor/host_test.go's TestEditorsAreInstances, whimsy's and whiml's.  It
// is compiled against the objects `make bin/whim++` leaves in lib/wpp;
// without them it skips.
func TestEditorsAreInstances(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "lib", "wpp", "src"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "editor.o")); err != nil {
		t.Skip("no whim++ build in lib/wpp (make bin/whim++)")
	}
	dir := t.TempDir()
	prog := filepath.Join(dir, "instances")
	args := append(append([]string{}, Flags...), "-I", src, filepath.Join("testdata", "instances", "main.cpp"))
	for _, u := range units {
		if u != "main" && u != "term" {
			args = append(args, filepath.Join(src, u+".o"))
		}
	}
	args = append(args, "-o", prog)
	if o, err := command("g++", args...).CombinedOutput(); err != nil {
		t.Fatalf("g++: %v\n%s", err, o)
	}
	out, err := command(prog).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Logf("%s", out)
}
