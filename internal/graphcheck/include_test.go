package graphcheck

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
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

// Phase 88 asks gcc which headers can go; the graph asks the headers. On
// q087 the rule finds every header gcc removed spare but two: <stdlib.h>
// and <stdint.h>, which provide EXIT_FAILURE and SIZE_MAX to two of phase
// 43's static_asserts in the host -- assertions of the core's own
// enumerators against the headers. With them gone the asserts compile
// silently and compare each enumerator with itself, which gcc's silence
// accepts and the rule refuses. DeleteIncludeRebind makes the deletion as
// gcc's silence does, the two tokens made uses of the core's enumerators;
// then the graph's C view is q088.c byte for byte.
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
	spare, err := e.SpareIncludes()
	if err != nil {
		t.Fatal(err)
	}
	var rule []string
	for _, inc := range spare {
		rule = append(rule, graph.IncludeSpec(inc))
	}
	var differ []string
	for _, s := range gone {
		if !slices.Contains(rule, s) {
			differ = append(differ, s)
		}
	}
	if len(gone) != 31 || len(rule) != 29 || !slices.Equal(differ, []string{"<stdlib.h>", "<stdint.h>"}) {
		t.Fatalf("gcc removed %d, the rule %d; gcc's not the rule's: %v", len(gone), len(rule), differ)
	}
	var all []*graph.Node
	for _, s := range gone {
		all = append(all, includeNamed(t, e, s))
	}
	miss, err := e.Missing(all...)
	if err != nil {
		t.Fatal(err)
	}
	var lost []string
	for _, u := range miss {
		if !u.Macro || !u.First.Is("static_assert") {
			t.Errorf("without gcc's 31: %v", u)
		}
		lost = append(lost, u.Name)
	}
	sort.Strings(lost)
	if !slices.Equal(lost, []string{"EXIT_FAILURE", "SIZE_MAX"}) {
		t.Errorf("without gcc's 31, unprovided: %v", miss)
	}
	// each deletion where the rule allows it, and the rebinding where not
	var rebound []string
	for _, s := range gone {
		inc := includeNamed(t, e, s)
		if slices.Contains(rule, s) {
			if err := e.DeleteInclude(inc); err == nil {
				continue
			} else if _, ok := err.(*graph.CollisionError); ok {
				t.Fatal(err)
			}
		}
		rb, err := e.DeleteIncludeRebind(inc)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range rb {
			if r := n.Ref(); r == nil || !e.InCore(r) {
				t.Errorf("%s: %s rebound to %v, not the core's", s, n.Atom, r)
			}
			rebound = append(rebound, n.Atom)
		}
	}
	sort.Strings(rebound)
	if !slices.Equal(rebound, []string{"EXIT_FAILURE", "SIZE_MAX"}) {
		t.Errorf("rebound %v", rebound)
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
}

// Phase 73 moves <fcntl.h> to beside <termios.h> by two literals on the
// text; on the graph it is one move of the include form, its id kept.
func TestIncludesPhase73(t *testing.T) {
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 72)
	te := edit.New("selfpipe", in, io.Discard)
	te.Literal("#include <fcntl.h>\n", "", 1, "<fcntl.h> out")
	te.Literal("#include <termios.h>\n", "#include <termios.h>\n#include <fcntl.h>\n", 1, "<fcntl.h> beside <termios.h>")
	want, err := te.Done()
	if err != nil {
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

// Phase 99 adds <stdlib.h> after <stddef.h> by a literal on the text; on
// the graph it is an inserted include form with a fresh id. Phase 88 had
// made the host's EXIT_FAILURE assert a use of the core's enumerator
// (TestIncludesPhase88); <stdlib.h> makes it the header's macro again, which
// the rule refuses as a collision and InsertIncludeRebind makes: the use
// becomes the macro's token, as an import of q099.c has it.
func TestIncludesPhase99(t *testing.T) {
	dir, _, _, _ := setup(t)
	in := snapOf(t, dir, 98)
	te := edit.New("pinnedtime", in, io.Discard)
	te.Literal("#include <stddef.h>\n", "#include <stddef.h>\n#include <stdlib.h>\n", 1, "<stdlib.h>")
	want, err := te.Done()
	if err != nil {
		t.Fatal(err)
	}
	g, e := readBack(t, in)
	stddef := includeNamed(t, e, "<stddef.h>")
	_, err = e.InsertIncludeAfter(stddef, "<stdlib.h>")
	var ce *graph.CollisionError
	if !errors.As(err, &ce) || !slices.Equal(ce.Names(), []string{"EXIT_FAILURE"}) || !ce.Collisions[0].Use {
		t.Fatalf("<stdlib.h> under the rule: %v", err)
	}
	inc, toks, err := e.InsertIncludeRebind(stddef, "<stdlib.h>", true)
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
	if got := cView(t, g); !bytes.Equal(got, want) {
		t.Fatalf("the graph's C view is not the text's: %d bytes against %d", len(got), len(want))
	}
	// as an import of the text after has it: the assert's EXIT_FAILURE the macro
	w, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), want)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range w.Forms {
		if f.Is("static_assert") {
			graph.Walk(f, func(n *graph.Node) bool {
				if !n.IsList() && n.Atom == "EXIT_FAILURE" && n.Ref() != nil {
					t.Errorf("q099.c imported: the assert's EXIT_FAILURE refers to %v", n.Ref())
				}
				return true
			})
		}
	}
	// read back, the same graph
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Equal(g, h); err != nil {
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
