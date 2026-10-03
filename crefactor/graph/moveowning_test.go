package graph

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A file whose includes are at the top and whose core uses one header's
// macro (INT_MAX, a function's prototype naming no parameter, a use after
// the definition that names them): the includes and the function that uses
// a header's declaration (strlen) move below the core, which owns INT_MAX.
const (
	ownIncs   = "#include <limits.h>\n#include <string.h>\n"
	ownProto  = "static int count(const char *);\n"
	ownCount  = "static int count(const char *s) { return (int)strlen(s); }\n"
	ownCore   = "static int core(const char *s) { return count(s) < INT_MAX ? count(s) : 0; }\n"
	ownMain   = "int main(void) { return core(\"x\"); }\n"
	ownEnum   = "enum : int { INT_MAX = (int)(~0u >> 1) };\n"
	ownBefore = ownIncs + ownProto + ownCount + ownCore + ownMain
)

func TestMoveFormsOwning(t *testing.T) {
	path, g, e := incGraph(t, ownBefore)
	incs := e.Includes()
	host := g.Forms[len(g.Forms)-1] // main
	count := defnNamed(t, e, "count")
	ns := append(append([]*Node{}, incs...), count)

	// the query: what the move would leave unprovided, the graph unchanged
	before, _ := g.C()
	lost, err := e.MoveWouldLose(host, false, ns)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, u := range lost {
		names = append(names, u.String())
	}
	if len(lost) != 1 || lost[0].Name != "INT_MAX" || !lost[0].Macro {
		t.Fatalf("MoveWouldLose: %v", names)
	}
	if after, _ := g.C(); string(after) != string(before) {
		t.Fatal("the query changed the graph")
	}

	// refused: a move that leaves a header's declaration unprovided
	if _, err := e.MoveFormsOwning(host, false, incs, nil); err == nil || !strings.Contains(err.Error(), "strlen") {
		t.Fatalf("a lost declaration: %v", err)
	}
	if after, _ := g.C(); string(after) != string(before) {
		t.Fatal("a refused move changed the graph")
	}

	// made: the core owns INT_MAX, its token a use of the enumerator, the
	// call after count's definition retargeted to the prototype above it
	var seen []HeaderUse
	uses, err := e.MoveFormsOwning(host, false, ns, func(l []HeaderUse) error {
		seen = l
		_, err := e.SpliceC(Frag{At: e.SpotBefore(g.Forms[0]), Src: ownEnum})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || len(uses) != 1 || uses[0].Atom != "INT_MAX" || !uses[0].Ref().Is("INT_MAX") {
		t.Fatalf("own saw %v, %d uses rebound", seen, len(uses))
	}
	want := ownEnum + ownProto + ownCore + ownIncs + ownCount + ownMain
	incWant(t, path, g, want)
	ownSameImport(t, g, want)
	if e.Includes()[0] != incs[0] || !slices.Contains(g.Forms, count) {
		t.Fatal("the moved forms did not keep their nodes")
	}
}

// Without own's declaration the tokens resolve to nothing: refused.
func TestMoveFormsOwningUnowned(t *testing.T) {
	_, g, e := incGraph(t, ownBefore)
	host := g.Forms[len(g.Forms)-1]
	ns := append(append([]*Node{}, e.Includes()...), defnNamed(t, e, "count"))
	_, err := e.MoveFormsOwning(host, false, ns, func([]HeaderUse) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "INT_MAX") {
		t.Fatalf("unowned: %v", err)
	}
	var ce *CollisionError
	if errors.As(err, &ce) {
		t.Fatalf("a collision, not a token naming nothing: %v", err)
	}
}

// ownSameImport holds g to the import of its C view, ids aside: every edge where
// the importer puts it, every typed edge to a type of the same structure.
func ownSameImport(t *testing.T, g *Graph, src string) {
	t.Helper()
	_, _, imp := importSample(t, src)
	if err := SameGraph(g, imp); err != nil {
		t.Fatalf("not the import of its C view: %v", err)
	}
}
