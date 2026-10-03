package graph

import (
	"strings"
	"testing"
)

// The samples of DeleteForHeader: a core that declares malloc itself, a
// host below <stdlib.h> that calls it too.
const (
	b3dcTypedef = "typedef unsigned long usize;\n"
	b3dcProto   = "void *malloc(usize n);\n"
	b3dcCoreUse = "static void *core_get(usize n) { return malloc(n); }\n"
	b3dcCore    = "static int core_fn(int a) { return a + 1; }\n"
	b3dcStdlib  = "#include <stdlib.h>\n"
	b3dcHost    = "static void *host_get(usize n) { void *p = malloc(n + 1); return p ? p : malloc(1); }\n"
	b3dcMain    = "int main(void) { return core_fn(1) + (host_get(1) != 0); }\n"
	b3dcMainUse = "int main(void) { return (core_get(1) != 0) + (host_get(1) != 0); }\n"
)

func b3dcProtoOf(t *testing.T, e *Editor) *Node {
	t.Helper()
	for _, f := range e.Graph().Forms {
		if topName(f) == "malloc" {
			return f
		}
	}
	t.Fatal("no prototype of malloc")
	return nil
}

func TestDeleteForHeader(t *testing.T) {
	src := incSample(b3dcTypedef, b3dcProto, b3dcCore, b3dcStdlib, b3dcHost, b3dcMain)
	path, g, e := incGraph(t, src)
	more := "static void *host_more(usize n) { return malloc(n); }\n"
	rebound, dangling, err := e.DeleteForHeader([]*Node{b3dcProtoOf(t, e)}, false,
		func() []Frag { return []Frag{{At: e.SpotBefore(e.Defn("main")), Src: more}} })
	if err != nil {
		t.Fatal(err)
	}
	if len(rebound) != 2 || len(dangling) != 0 {
		t.Fatalf("%d rebound, %d dangling; want 2 and 0", len(rebound), len(dangling))
	}
	for _, u := range rebound {
		if u.Atom != "malloc" || u.Refs[0].Head() != "extern" {
			t.Errorf("a use made again is %s", Lisp(u))
		}
	}
	after := incSample(b3dcTypedef, b3dcCore, b3dcStdlib, b3dcHost, more, b3dcMain)
	incWant(t, path, g, after)
	_, _, h := importSample(t, after)
	if err := SameGraph(g, h); err != nil {
		t.Fatalf("not the import of the text after: %v", err)
	}
}

func TestDeleteForHeaderRefused(t *testing.T) {
	src := incSample(b3dcTypedef, b3dcProto, b3dcCoreUse, b3dcStdlib, b3dcHost, b3dcMainUse)
	_, g, e := incGraph(t, src)
	before, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = e.DeleteForHeader([]*Node{b3dcProtoOf(t, e)}, false, nil)
	if err == nil || !strings.Contains(err.Error(), "above every include") {
		t.Fatalf("a use above the include: %v", err)
	}
	if now, _ := g.C(); string(now) != string(before) {
		t.Fatal("a refusal changed the graph")
	}
	// another declaration of the name: refused
	src2 := incSample(b3dcTypedef, b3dcProto, b3dcProto, b3dcCore, b3dcStdlib, b3dcHost, b3dcMain)
	_, _, e2 := incGraph(t, src2)
	if _, _, err := e2.DeleteForHeader([]*Node{b3dcProtoOf(t, e2)}, false, nil); err == nil || !strings.Contains(err.Error(), "again") {
		t.Fatalf("a second declaration: %v", err)
	}
	// a definition is not a declaration for the header
	_, _, e3 := incGraph(t, incSample(b3dcTypedef, b3dcProto, b3dcCoreUse, b3dcStdlib, b3dcHost, b3dcMainUse))
	if _, _, err := e3.DeleteForHeader([]*Node{e3.Defn("core_get")}, false, nil); err == nil {
		t.Fatal("a definition deleted for the header")
	}
}

func TestDeleteForHeaderDangle(t *testing.T) {
	src := incSample(b3dcTypedef, b3dcProto, b3dcCoreUse, b3dcStdlib, b3dcHost, b3dcMainUse)
	_, _, e := incGraph(t, src)
	rebound, dangling, err := e.DeleteForHeader([]*Node{b3dcProtoOf(t, e)}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rebound) != 2 || len(dangling) != 1 || e.Function(dangling[0]) != e.Defn("core_get") {
		t.Fatalf("%d rebound, %d dangling", len(rebound), len(dangling))
	}
}
