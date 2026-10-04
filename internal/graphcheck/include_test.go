package graphcheck

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/whim"
)

// INCLUDE and the line on the snapshots (doc/GRAPH-MIGRATION.md, B2e): the
// real phases' include edits made on the graph, read back from its Lisp,
// and held byte for byte to the text programs' -- phase 88's deletions, 73's
// move, 99's insertion -- and the boundary queries held to whim.Cut on every
// snapshot.

// readBack is snapshot n's graph, imported and read back from its Lisp: no
// cc node behind it.
func readBack(t *testing.T, in []byte) (*graph.Graph, *graph.Editor) {
	t.Helper()
	g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return h, graph.NewEditor(h)
}

func cView(t *testing.T, g *graph.Graph) []byte {
	t.Helper()
	b, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func includeNamed(t *testing.T, e *graph.Editor, spec string) *graph.Node {
	t.Helper()
	for _, inc := range e.Includes() {
		if graph.IncludeSpec(inc) == spec {
			return inc
		}
	}
	t.Fatalf("no #include %s", spec)
	return nil
}

// Phase 88 runs on the graph (R3): the headers the include rule finds
// spare are deleted, where the text step asked gcc. On q087 the rule names
// exactly the 29 headers q088.c no longer has -- 33 alone, which together
// leave names unprovided, and the fold from the bottom 29 -- and keeps <stdlib.h> and <stdint.h>, which provide
// EXIT_FAILURE and SIZE_MAX to phase 43's static_asserts in the host (the
// finding of B2e that made gcc's question ask the preprocessor too,
// 4f96189); deleting them gives q088.c byte for byte. The control:
// <termios.h> is not spare on q088.
//
// The gcc cross-check stays as this test, not as a step: q088.c compiles
// with nothing printed under the sweep's warnings, and each header it still
// includes, taken out, either makes gcc print something or changes what
// the file's own lines preprocess to -- gcc's question (xform.Includes
// with Silent's Same, deleted with phase 88's text step) answers as the
// rule does on every header left.
func TestIncludesPhase88(t *testing.T) {
	dir, _, _, _ := setup(t)
	in, want := snapOf(t, dir, 87), snapOf(t, dir, 88)
	g, e := readBack(t, in)
	var gone []string
	for _, inc := range e.Includes() {
		if !bytes.Contains(want, []byte("\n#include "+graph.IncludeSpec(inc)+"\n")) {
			gone = append(gone, graph.IncludeSpec(inc))
		}
	}
	r, err := e.Spares()
	if err != nil {
		t.Fatal(err)
	}
	var rule []string
	for _, inc := range r.Spare {
		rule = append(rule, graph.IncludeSpec(inc))
	}
	if len(gone) != 29 || !slices.Equal(rule, gone) || r.Together || len(r.Alone) != 33 {
		t.Fatalf("q088.c went without %d, the rule %d (alone %d, together %v): %v against %v", len(gone), len(rule), len(r.Alone), r.Together, rule, gone)
	}
	if len(r.Unprovided) > 0 || len(r.Collisions) > 0 {
		t.Fatalf("q087 under the rule: %v %v", r.Unprovided, r.Collisions)
	}
	for s, name := range map[string]string{"<stdlib.h>": "EXIT_FAILURE", "<stdint.h>": "SIZE_MAX"} {
		miss, err := e.Missing(append(slices.Clone(r.Spare), includeNamed(t, e, s))...)
		if err != nil || len(miss) != 1 || miss[0].Name != name || !miss[0].First.Is("static_assert") {
			t.Errorf("%s with the spare ones provides %v (%v)", s, miss, err)
		}
	}
	if err := e.DeleteIncludes(r.Spare...); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if got := cView(t, g); !bytes.Equal(got, want) {
		t.Fatalf("the graph's C view is not q088.c: %d bytes against %d", len(got), len(want))
	}
	if miss, err := e.Missing(includeNamed(t, e, "<termios.h>")); err != nil || len(miss) == 0 {
		t.Errorf("the control: <termios.h> spare on q088 (%v)", err)
	}
	// gcc, asked of every header left
	d := t.TempDir()
	if ok, why := r3SilentSame(t, d, "q088.c", want, nil); !ok {
		t.Fatalf("q088.c under gcc: %s", why)
	}
	lines := bytes.Split(want, []byte("\n"))
	var wg sync.WaitGroup
	for i, l := range lines {
		if !bytes.HasPrefix(l, []byte("#include ")) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			without := bytes.Join(append(append([][]byte{}, lines[:i]...), lines[i+1:]...), []byte("\n"))
			if ok, _ := r3SilentSame(t, d, fmt.Sprintf("w%d.c", i), without, want); ok {
				t.Errorf("gcc: q088.c without %s compiles silently to the same tokens -- the rule kept it", l)
			}
		}()
	}
	wg.Wait()
}

// r3SilentSame is phase 88's old compiler question: text compiles with
// nothing printed under the sweep's warnings, and -- ref given -- its own
// lines preprocess to ref's.
func r3SilentSame(t *testing.T, dir, name string, text, ref []byte) (bool, string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, text, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("gcc", "-fsyntax-only", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter", p).CombinedOutput()
	if err != nil || len(out) > 0 {
		return false, fmt.Sprintf("%v %s", err, edit.CoreHead(string(out), 200))
	}
	if ref == nil {
		return true, ""
	}
	own := func(p string, b []byte) string {
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("gcc", "-E", p).Output()
		if err != nil {
			t.Fatal(err)
		}
		var o strings.Builder
		in := false
		for _, ln := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(ln, "# ") {
				f := strings.Fields(ln)
				in = len(f) > 2 && f[2] == strconv.Quote(p)
				continue
			}
			if in && strings.TrimSpace(ln) != "" {
				o.WriteString(ln + "\n")
			}
		}
		return o.String()
	}
	if own(p+".e.c", text) != own(p+".r.c", ref) {
		return false, "its own lines preprocess otherwise"
	}
	return true, ""
}

// Phase 73 moves <fcntl.h> to beside <termios.h> by two literals on the
// text; on the graph it is one move of the include form, its id kept.
func TestIncludesPhase73(t *testing.T) {
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 72)
	// the text's two acts were edit.E's Literal: edit.ReplaceLiteral on the
	// whole text, counted
	want, err := edit.ReplaceLiteral(in, "#include <fcntl.h>\n", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if want, err = edit.ReplaceLiteral(want, "#include <termios.h>\n", "#include <termios.h>\n#include <fcntl.h>\n", 1); err != nil {
		t.Fatal(err)
	}
	g, e := readBack(t, in)
	fcntl := includeNamed(t, e, "<fcntl.h>")
	if err := e.MoveFormsAfter(includeNamed(t, e, "<termios.h>"), fcntl); err != nil {
		t.Fatal(err)
	}
	if act := e.Log[len(e.Log)-1]; act.Op != "move" || !slices.Equal(act.Moved, []graph.ID{fcntl.ID}) {
		t.Errorf("the act %+v", act)
	}
	if got := cView(t, g); !bytes.Equal(got, want) {
		t.Fatalf("the graph's C view is not the text's: %d bytes against %d", len(got), len(want))
	}
	// the control: <termios.h> cannot go below a use of what it alone provides
	host := e.Host()
	if err := e.MoveFormsAfter(host[len(host)-1], includeNamed(t, e, "<termios.h>")); err == nil {
		t.Error("<termios.h> moved to the end of the file")
	}
}

// The rebinds, both ways, on a snapshot: since 4f96189 phase 88 keeps
// <stdlib.h>, so on q098 deleting it is refused (the host's EXIT_FAILURE
// assert would be left unprovided) and DeleteIncludeRebind makes it, the
// token made a use of the core's enumerator; inserting it again where it
// was is refused as a collision of use and InsertIncludeRebind makes it,
// the use the macro's token again: the C view is q098.c byte for byte, and
// the graph, read back, the import's.
func TestIncludesPhase99(t *testing.T) {
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 98)
	g, e := readBack(t, in)
	stdlib := includeNamed(t, e, "<stdlib.h>")
	at := e.Graph().Forms[slices.Index(e.Graph().Forms, stdlib)-1]
	if err := e.DeleteInclude(stdlib); err == nil {
		t.Fatal("<stdlib.h> deleted under the rule")
	}
	rb, err := e.DeleteIncludeRebind(stdlib)
	if err != nil {
		t.Fatal(err)
	}
	if len(rb) != 1 || rb[0].Atom != "EXIT_FAILURE" || rb[0].Ref() == nil || !e.InCore(rb[0].Ref()) {
		t.Fatalf("rebound %v", rb)
	}
	_, err = e.InsertIncludeAfter(at, "<stdlib.h>")
	var ce *graph.CollisionError
	if !errors.As(err, &ce) || !slices.Equal(ce.Names(), []string{"EXIT_FAILURE"}) || !ce.Collisions[0].Use {
		t.Fatalf("<stdlib.h> under the rule: %v", err)
	}
	inc, toks, err := e.InsertIncludeRebind(at, "<stdlib.h>", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(toks) != 1 || toks[0].Atom != "EXIT_FAILURE" || toks[0].Ref() != nil || !e.InHost(toks[0]) {
		t.Errorf("the tokens made the macro: %v", toks)
	}
	if !slices.Contains(e.Log[len(e.Log)-2].New, inc.ID) {
		t.Errorf("the act %+v", e.Log[len(e.Log)-2])
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	if got := cView(t, g); !bytes.Equal(got, in) {
		t.Fatalf("the graph's C view is not q098.c: %d bytes against %d", len(got), len(in))
	}
	e.Recheck()
	w, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.SameGraph(h, w); err != nil {
		t.Fatal(err)
	}
}

// Phase 43 moved the includes below the core because the core's own
// constants are the headers' macros below them; it found the twelve by
// compiling (its GOAL.md: "23 errors naming exactly twelve"). On q043 the
// graph refuses the includes moved back above the core, naming the same
// twelve, and nothing changes.
func TestIncludesPhase43(t *testing.T) {
	dir, _, _, _ := setup(t)
	g, e := readBack(t, snapOf(t, dir, 43))
	before := cView(t, g)
	err := e.MoveFormsBefore(g.Forms[0], e.Includes()...)
	var ce *graph.CollisionError
	if !errors.As(err, &ce) {
		t.Fatalf("the includes moved above the core: %v", err)
	}
	names := ce.Names()
	sort.Strings(names)
	twelve := []string{"EXIT_FAILURE", "INT_MAX", "INT_MIN", "LLONG_MAX", "LLONG_MIN", "LONG_MAX", "LONG_MIN",
		"PATH_MAX", "SIGHUP", "SIGTERM", "SIZE_MAX", "ULLONG_MAX"}
	if !slices.Equal(names, twelve) {
		t.Errorf("the collisions %v, phase 43's %v", names, twelve)
	}
	if !bytes.Equal(cView(t, g), before) {
		t.Error("a refused move changed the graph")
	}
}

// The line on every snapshot: the first include form; the core's C view is
// whim.Cut's text (from q043, where the includes moved below the core;
// before it the first form is an include and Cut finds no core); and
// nothing above the line takes a name from the headers.
func TestTheLine(t *testing.T) {
	dir, phases, _, jobs := setup(t)
	snaps := append([]int{0}, phases...)
	var mu sync.Mutex
	cores := map[int]int{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	for _, n := range slices.Compact(snaps) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			in, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("q%03d.c", n)))
			if err != nil {
				t.Error(err)
				return
			}
			g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
			if err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			e := graph.NewEditor(g)
			if err := lineAgrees(e, in); err != nil {
				t.Errorf("q%03d: %v", n, err)
				return
			}
			mu.Lock()
			cores[n] = len(e.Core())
			mu.Unlock()
		}()
	}
	wg.Wait()
	if c, ok := cores[42]; ok && c != 0 {
		t.Errorf("q042 has a core of %d forms", c)
	}
	if c, ok := cores[43]; ok && c == 0 {
		t.Error("q043 has no core")
	}
	t.Logf("%d snapshots", len(cores))
}

func lineAgrees(e *graph.Editor, text []byte) error {
	first := e.FirstInclude()
	if first == nil {
		return fmt.Errorf("no include form")
	}
	cut, err := whim.Cut(text)
	core := e.Core()
	switch {
	case err != nil && len(core) != 0:
		return fmt.Errorf("whim.Cut finds no core (%v), the graph %d forms", err, len(core))
	case err == nil:
		got, err := graph.FormsC(core)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, cut) {
			return fmt.Errorf("the core's C view is not whim.Cut's: %d bytes against %d", len(got), len(cut))
		}
	}
	if e.Host()[0] != first || !e.InHost(first) {
		return fmt.Errorf("the host does not begin at the first include")
	}
	us, err := e.HeaderUses()
	if err != nil {
		return err
	}
	for _, u := range us {
		if e.InCore(u.First) {
			return fmt.Errorf("the core takes %v", u)
		}
	}
	return nil
}
