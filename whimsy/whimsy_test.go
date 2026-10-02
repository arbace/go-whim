package whimsy

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

// layoutEntry is a line of the backend's layout listing (editor.rs.layout):
// a struct's size, or a member's offset, as the C front end lays them out.
type layoutEntry struct {
	size         bool
	rust, c      string // the type, as each language names it
	rpath, cpath string // the member's path, as each spells it
	n            int64
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
		case len(f) == 4 && f[0] == "size":
			e = layoutEntry{size: true, rust: f[1], c: strings.ReplaceAll(f[2], "~", " ")}
			e.n, err = strconv.ParseInt(f[3], 10, 64)
		case len(f) == 6 && f[0] == "offset":
			e = layoutEntry{rust: f[1], c: strings.ReplaceAll(f[2], "~", " "), rpath: f[3], cpath: f[4]}
			e.n, err = strconv.ParseInt(f[5], 10, 64)
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

// cAsserts are the listing as gcc's static assertions: sizeof and offsetof
// -- of the types declared at file scope, which C can name there.
func cAsserts(es []layoutEntry) string {
	var b strings.Builder
	for i, e := range es {
		switch {
		case e.c == "-":
		case e.size:
			fmt.Fprintf(&b, "_Static_assert(sizeof(%s) == %d, \"%d\");\n", e.c, e.n, i)
		default:
			fmt.Fprintf(&b, "_Static_assert(__builtin_offsetof(%s, %s) == %d, \"%d\");\n", e.c, e.cpath, e.n, i)
		}
	}
	return b.String()
}

// rsAsserts are the listing as Rust's constant assertions: size_of and
// offset_of!.
func rsAsserts(es []layoutEntry) string {
	var b strings.Builder
	b.WriteString("use crate::editor::*;\n\n")
	for _, e := range es {
		if e.size {
			fmt.Fprintf(&b, "const _: () = assert!(core::mem::size_of::<%s>() == %d);\n", e.rust, e.n)
		} else {
			fmt.Fprintf(&b, "const _: () = assert!(core::mem::offset_of!(%s, %s) == %d);\n", e.rust, e.rpath, e.n)
		}
	}
	return b.String()
}

// stubHost is a host of the backend's signatures (editor.rs.host) whose
// functions do nothing a check needs: the crate type-checks without the
// hand-written host.
func stubHost(sigs string) string {
	var b strings.Builder
	b.WriteString("#![allow(unused_variables)]\nuse crate::editor::*;\nuse crate::rt::VArg;\nuse core::ffi::c_void;\n\n")
	for _, l := range strings.Split(strings.TrimSpace(sigs), "\n") {
		b.WriteString(l + " {\n    unimplemented!()\n}\n")
	}
	return b.String()
}

// TestLayout holds every struct and union of the core to gcc's layout: the
// backend lists each one's size and each member's offset (through the
// members of a type with no name of its own) as the C front end computes
// them, and gcc's sizeof and offsetof, and Rust's size_of and offset_of! on
// the generated #[repr(C)] types, must each agree with the listing --
// compile-time assertions on both sides.  The control: one number of the
// listing changed is refused by both.
func TestLayout(t *testing.T) {
	for _, tool := range []string{"gcc", "rustc"} {
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
	out := filepath.Join(dir, "editor.rs")
	if rc := togo.Run([]string{editorC, dir, "-rs", out}, io.Discard, whim.Gen); rc != 0 {
		t.Fatalf("the Rust backend: status %d", rc)
	}
	es := readLayout(t, out+".layout")
	structs, members, local := 0, 0, 0
	for _, e := range es {
		switch {
		case e.size && e.c == "-":
			local++
			fallthrough
		case e.size:
			structs++
		default:
			members++
		}
	}
	if structs < 100 {
		t.Fatalf("the listing has %d structs: too few for the core", structs)
	}
	t.Logf("%d structs and unions (%d of a function's own, which gcc cannot name at file scope), %d members", structs, local, members)

	sigs, err := os.ReadFile(out + ".host")
	if err != nil {
		t.Fatal(err)
	}
	rt, err := os.ReadFile(filepath.Join("src", "rt.rs"))
	if err != nil {
		t.Fatal(err)
	}
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
	rustCheck := func(es []layoutEntry) error {
		crate := filepath.Join(dir, "crate")
		files := map[string]string{
			"lib.rs":    "pub mod editor;\npub mod host;\npub mod rt;\nmod layout;\n",
			"rt.rs":     string(rt),
			"host.rs":   stubHost(string(sigs)),
			"layout.rs": rsAsserts(es),
		}
		if err := os.MkdirAll(crate, 0o755); err != nil {
			return err
		}
		ed, err := os.ReadFile(out)
		if err != nil {
			return err
		}
		files["editor.rs"] = string(ed)
		for n, s := range files {
			if err := os.WriteFile(filepath.Join(crate, n), []byte(s), 0o644); err != nil {
				return err
			}
		}
		o, err := exec.Command("rustc", "--edition", "2021", "--crate-type", "rlib", "--crate-name", "layout", "--emit=metadata",
			"-A", "warnings", "--out-dir", crate, filepath.Join(crate, "lib.rs")).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v\n%s", err, firstLines(o, 20))
		}
		return nil
	}
	if err := gccCheck(es); err != nil {
		t.Errorf("gcc's layout is not the listing's: %v", err)
	}
	if err := rustCheck(es); err != nil {
		t.Errorf("Rust's layout is not the listing's: %v", err)
	}
	// the control: one offset moved by one
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
	if rustCheck(bad) == nil {
		t.Error("the control: rustc took a wrong offset")
	}
}

func firstLines(b []byte, n int) string {
	ls := strings.SplitN(string(b), "\n", n+1)
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}
