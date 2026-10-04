package whiml

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

// layoutEntry is a line of the backend's layout listing (editor.ml.layout):
// a struct's size, or a member's offset, as the C front end lays them out
// -- the offsets every load and store of the generated module is at.
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

// cAsserts are the listing as gcc's static assertions: sizeof and offsetof.
func cAsserts(es []layoutEntry) string {
	var b strings.Builder
	for i, e := range es {
		if e.size {
			fmt.Fprintf(&b, "_Static_assert(sizeof(%s) == %d, \"%d\");\n", e.c, e.n, i)
		} else {
			fmt.Fprintf(&b, "_Static_assert(__builtin_offsetof(%s, %s) == %d, \"%d\");\n", e.c, e.path, e.n, i)
		}
	}
	return b.String()
}

// TestLayout holds every struct and union of the core that C names at file
// scope to gcc's layout: the backend lists each one's size and each
// member's offset (through the members of a type with no name of its own)
// as the C front end computes them -- the numbers its loads and stores are
// written with -- and gcc's sizeof and offsetof must agree with the
// listing, as compile-time assertions.  The control: one offset of the
// listing moved by one is refused.
func TestLayout(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no gcc")
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
	out := filepath.Join(dir, "editor.ml")
	if rc := togo.Run([]string{editorC, dir, "-ml", out}, io.Discard, whim.Gen); rc != 0 {
		t.Fatalf("the OCaml backend: status %d", rc)
	}
	es := readLayout(t, out+".layout")
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
	gccCheck := func(es []layoutEntry) error {
		f := filepath.Join(dir, "layout.c")
		if err := os.WriteFile(f, append(append([]byte{}, core...), cAsserts(es)...), 0o644); err != nil {
			return err
		}
		o, err := exec.Command("gcc", "-fsyntax-only", "-w", f).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v\n%s", err, firstLines(o, 20))
		}
		return nil
	}
	if err := gccCheck(es); err != nil {
		t.Errorf("gcc's layout is not the listing's: %v", err)
	}
	bad := append([]layoutEntry{}, es...)
	for i := range bad {
		if !bad[i].size && bad[i].n > 0 {
			bad[i].n++
			break
		}
	}
	if gccCheck(bad) == nil {
		t.Error("the control: gcc took a wrong offset")
	}
}

func firstLines(b []byte, n int) string {
	ls := strings.SplitN(string(b), "\n", n+1)
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}

// Editors are instances: testdata/instances/main.ml runs four at once in
// one process, each on a domain and a host of its own, and requires each to
// exit 0 and its screen to show its own text and no other's --
// editor/host_test.go's TestEditorsAreInstances, caprice's, whimsy's and
// whimsical's.  It is compiled against the modules the build `make
// bin/whiml` leaves compiled in lib/whiml; without them it skips.
func TestEditorsAreInstances(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "lib", "whiml", "src"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "editor.cmx")); err != nil {
		t.Skip("no whiml build in lib/whiml (make bin/whiml)")
	}
	if _, err := exec.LookPath("ocamlopt"); err != nil {
		t.Skip("no ocamlopt")
	}
	dir := t.TempDir()
	main, err := os.ReadFile(filepath.Join("testdata", "instances", "main.ml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ml"), main, 0o644); err != nil {
		t.Fatal(err)
	}
	prog := filepath.Join(dir, "instances")
	args := append(append([]string{}, Flags...), "-I", src, "unix.cmxa")
	for _, m := range modules[:len(modules)-1] {
		args = append(args, filepath.Join(src, m+".cmx"))
	}
	args = append(args, filepath.Join(src, "term_stubs.o"), "main.ml", "-o", prog)
	cmd := command("ocamlopt", args...)
	cmd.Dir = dir
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ocamlopt: %v\n%s", err, o)
	}
	out, err := command(prog).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Logf("%s", out)
}
