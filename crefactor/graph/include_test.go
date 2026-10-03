package graph

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// The pieces of the include samples: a core above the first include, a
// host below it, as a code base with a line keeps them.  The core declares
// EXIT_FAILURE itself, and the host asserts it against <stdlib.h>'s macro,
// as a core that owns a header's constant does.
const (
	incEnum   = "enum { EXIT_FAILURE = 1 };\n"
	incProto  = "static int twice(int x);\n"
	incCore   = "static int core_fn(int a) { return twice(a) + EXIT_FAILURE; }\n"
	incHelper = "static int helper(int a) { return a + 1; }\n"
	incUser   = "static int user(int a) { return helper(a); }\n"
	incStdlib = "#include <stdlib.h>\n"
	incString = "#include <string.h>\n"
	incCtype  = "#include <ctype.h>\n"
	incTwice  = "static int twice(int x) { return 2 * x; }\n"
	incAssert = "static_assert(1 == EXIT_FAILURE, \"EXIT_FAILURE\");\n"
	incLen    = "static size_t host_len(const char *s) { char *v = getenv(s); return v ? strlen(v) : 0; }\n"
	incMain   = "int main(void) { return core_fn(1) + (int)host_len(\"HOME\") + user(2); }\n"
)

func incSample(parts ...string) string { return strings.Join(parts, "") }

var incFull = incSample(incEnum, incProto, incCore, incHelper, incUser,
	incStdlib, incString, incCtype, incTwice, incAssert, incLen, incMain)

// incGraph is the sample's graph read back from its Lisp, and its editor.
func incGraph(t *testing.T, src string) (string, *Graph, *Editor) {
	t.Helper()
	path, _, g := importSample(t, src)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return path, h, NewEditor(h)
}

// incWant holds the graph's C view to src's canonical print, and the graph
// to its Lisp read back.
func incWant(t *testing.T, path string, g *Graph, src string) {
	t.Helper()
	want, err := cemit.Canonical(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the C view:\n%s\nwant:\n%s", got, want)
	}
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	if err := Equal(g, h); err != nil {
		t.Fatalf("read back: %v", err)
	}
}

func defnNamed(t *testing.T, e *Editor, name string) *Node {
	t.Helper()
	d := e.Defn(name)
	if d == nil {
		t.Fatalf("no definition of %s", name)
	}
	return d
}

func includeOf(t *testing.T, e *Editor, spec string) *Node {
	t.Helper()
	for _, inc := range e.Includes() {
		if IncludeSpec(inc) == spec {
			return inc
		}
	}
	t.Fatalf("no #include %s", spec)
	return nil
}

func useNames(us []HeaderUse) []string {
	var out []string
	for _, u := range us {
		out = append(out, u.Name)
	}
	return out
}

// The line: the first include form, the core above it, the host from it
// on; a node's side by its top-level form; the core's C view the text above
// the first #include.  A file without an include form has no line.
func TestIncludeLine(t *testing.T) {
	path, g, e := incGraph(t, incFull)
	first := e.FirstInclude()
	if IncludeSpec(first) != "<stdlib.h>" || len(e.Includes()) != 3 {
		t.Fatalf("the first include %v of %d", first, len(e.Includes()))
	}
	if len(e.Core()) != 5 || e.Host()[0] != first || len(e.Core())+len(e.Host()) != len(g.Forms) {
		t.Fatalf("core %d forms, host %d", len(e.Core()), len(e.Host()))
	}
	var call *Node
	Walk(defnNamed(t, e, "core_fn"), func(n *Node) bool {
		if call == nil && n.Is("call") {
			call = n
		}
		return true
	})
	twice := defnNamed(t, e, "twice")
	for _, c := range []struct {
		what string
		ok   bool
	}{
		{"core_fn's call is in the core", e.InCore(call) && !e.InHost(call)},
		{"its top-level form is core_fn", e.TopForm(call) == defnNamed(t, e, "core_fn")},
		{"twice's definition is in the host", e.InHost(twice) && !e.InCore(twice)},
		{"the first include is the host's", e.InHost(first)},
		{"an external is in neither", !e.InCore(g.Externs[0]) && !e.InHost(g.Externs[0]) && e.TopForm(g.Externs[0]) == nil},
	} {
		if !c.ok {
			t.Error(c.what)
		}
	}
	core, err := FormsC(e.Core())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := cemit.Canonical(path, []byte(incFull))
	if above, _, _ := bytes.Cut(want, []byte("\n#include")); !bytes.Equal(core, append(above, '\n')) && !bytes.Equal(core, above) {
		t.Errorf("the core's C view:\n%s\nthe text above the first #include:\n%s", core, above)
	}

	_, _, e = incGraph(t, incSample(incEnum, incProto, incCore, incTwice, "int main(void) { return core_fn(1); }\n"))
	if e.FirstInclude() != nil || len(e.Host()) != 0 || len(e.Core()) != len(e.Graph().Forms) {
		t.Error("a file without an include is all core")
	}
	if err := e.MoveToHost(e.Defn("twice")); err == nil {
		t.Error("moved to a host there is not")
	}
}

// What the file takes from the headers, each with the include above its
// first use that provides it: getenv and strlen declared, size_t a
// typedef, EXIT_FAILURE a macro in the host's assert, nothing in the core
// (the core's EXIT_FAILURE is its own enumerator). <ctype.h> provides none.
func TestHeaderUses(t *testing.T) {
	_, _, e := incGraph(t, incFull)
	us, err := e.HeaderUses()
	if err != nil {
		t.Fatal(err)
	}
	from := map[string]string{}
	for _, u := range us {
		if e.InCore(u.First) {
			t.Errorf("%v: a header's name in the core", u)
		}
		from[useKey(u)] = IncludeSpec(u.From)
	}
	want := map[string]string{"getenv": "<stdlib.h>", "strlen": "<string.h>", "size_t": "<stdlib.h>", "macro EXIT_FAILURE": "<stdlib.h>"}
	for k, v := range want {
		if from[k] != v {
			t.Errorf("%s from %q, want %s (all: %v)", k, from[k], v, from)
		}
	}
	if len(from) != len(want) {
		t.Errorf("uses %v, want %v", from, want)
	}
	spare, err := e.SpareIncludes()
	if err != nil {
		t.Fatal(err)
	}
	if len(spare) != 1 || IncludeSpec(spare[0]) != "<ctype.h>" {
		t.Errorf("spare %v", spare)
	}
	miss, err := e.Missing(includeOf(t, e, "<stdlib.h>"))
	if err != nil {
		t.Fatal(err)
	}
	// size_t is <string.h>'s too
	if got := useNames(miss); !slices.Equal(got, []string{"macro EXIT_FAILURE", "getenv"}) && !slices.Equal(got, []string{"EXIT_FAILURE", "getenv"}) {
		t.Errorf("missing without <stdlib.h>: %v", miss)
	}
}

// Deleting an include: refused while a name it alone provides is used,
// named; a spare one goes, the C view the text without its line.
func TestDeleteInclude(t *testing.T) {
	path, g, e := incGraph(t, incFull)
	n := len(e.Log)
	err := e.DeleteInclude(includeOf(t, e, "<string.h>"))
	if err == nil || !strings.Contains(err.Error(), "`strlen`") {
		t.Fatalf("<string.h> deleted while strlen is used: %v", err)
	}
	err = e.DeleteInclude(includeOf(t, e, "<stdlib.h>"))
	if err == nil || !strings.Contains(err.Error(), "`getenv`") || !strings.Contains(err.Error(), "`EXIT_FAILURE`") || strings.Contains(err.Error(), "size_t") {
		t.Fatalf("<stdlib.h> deleted: %v", err)
	}
	if len(e.Log) != n {
		t.Fatal("a refused deletion logged an act")
	}
	ctype := includeOf(t, e, "<ctype.h>")
	if err := e.DeleteInclude(ctype); err != nil {
		t.Fatal(err)
	}
	if e.Live(ctype) || !slices.Contains(e.Log[len(e.Log)-1].Gone, ctype.ID) {
		t.Error("the include's id is not superseded")
	}
	incWant(t, path, g, strings.Replace(incFull, incCtype, "", 1))
}

// A header's macro the file also declares: the deletion refused as the
// rule says, and made by DeleteIncludeRebind with the tokens made uses of
// the file's enumerator -- what an import of the text after makes of them.
func TestDeleteIncludeRebind(t *testing.T) {
	src := incSample(incEnum, incProto, incCore, incStdlib, incString, incTwice, incAssert,
		"int main(void) { return core_fn(1) + (int)strlen(\"x\"); }\n")
	path, g, e := incGraph(t, src)
	stdlib := includeOf(t, e, "<stdlib.h>")
	if err := e.DeleteInclude(stdlib); err == nil || !strings.Contains(err.Error(), "macro `EXIT_FAILURE`") {
		t.Fatalf("deleted under the rule: %v", err)
	}
	rb, err := e.DeleteIncludeRebind(stdlib)
	if err != nil {
		t.Fatal(err)
	}
	if len(rb) != 1 || rb[0].Ref() == nil || !rb[0].Ref().list || rb[0].Ref().Kids[0].Atom != "EXIT_FAILURE" || rb[0].ID == 0 {
		t.Fatalf("rebound %v", rb)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	after := strings.Replace(src, incStdlib, "", 1)
	incWant(t, path, g, after)
	// the import of the text after says the same
	_, _, h := importSample(t, after)
	var asserted *Node
	for _, f := range h.Forms {
		if f.Is("static_assert") {
			Walk(f, func(n *Node) bool {
				if !n.list && n.Atom == "EXIT_FAILURE" {
					asserted = n
				}
				return true
			})
		}
	}
	if asserted == nil || asserted.Ref() == nil || !asserted.Ref().list || asserted.Ref().Kids[0].Atom != "EXIT_FAILURE" {
		t.Errorf("the import of the text after: %v", asserted)
	}
	// a declaration is never rebound
	_, _, e = incGraph(t, incFull)
	if _, err := e.DeleteIncludeRebind(includeOf(t, e, "<string.h>")); err == nil {
		t.Error("strlen's header deleted")
	}
}

// Inserting an include: a fresh form with a fresh id, its line in the C
// view; refused where the file has it already, or where its macros would
// take a name the file declares below it -- EXIT_FAILURE, the core's own,
// were <stdlib.h> put above the core (phase 43's reason the includes are
// below it); a refused insertion leaves the graph as it was.
func TestInsertInclude(t *testing.T) {
	path, g, e := incGraph(t, incFull)
	inc, err := e.InsertIncludeAfter(includeOf(t, e, "<string.h>"), "<stdio.h>")
	if err != nil {
		t.Fatal(err)
	}
	if inc.ID == 0 || !slices.Contains(e.Log[len(e.Log)-1].New, inc.ID) || IncludeSpec(inc) != "<stdio.h>" {
		t.Fatalf("the include %v, the act %+v", inc, e.Log[len(e.Log)-1])
	}
	if _, err := e.InsertIncludeBefore(e.Graph().Forms[0], "<stdio.h>"); err == nil {
		t.Error("included twice")
	}
	incWant(t, path, g, strings.Replace(incFull, incString, incString+"#include <stdio.h>\n", 1))

	src := incSample(incEnum, incProto, incCore, incString, incTwice, "int main(void) { return core_fn(1) + (int)strlen(\"x\"); }\n")
	path, g, e = incGraph(t, src)
	before, _ := g.C()
	n := len(e.Log)
	_, err = e.InsertIncludeBefore(g.Forms[0], "<stdlib.h>")
	var ce *CollisionError
	if !errors.As(err, &ce) || !slices.Equal(ce.Names(), []string{"EXIT_FAILURE"}) || ce.Collisions[0].Use {
		t.Fatalf("<stdlib.h> above the core's EXIT_FAILURE: %v", err)
	}
	if _, _, err := e.InsertIncludeRebind(g.Forms[0], "<stdlib.h>", false); err == nil {
		t.Fatal("a declaration rebound")
	}
	if after, _ := g.C(); !bytes.Equal(before, after) || len(e.Log) != n || len(e.Includes()) != 1 {
		t.Fatal("a refused insertion changed the graph")
	}
	if _, err := e.InsertIncludeBefore(includeOf(t, e, "<string.h>"), "<stdlib.h>"); err != nil {
		t.Fatal(err)
	}
	incWant(t, path, g, strings.Replace(src, incString, incStdlib+incString, 1))
}

// Moving across the line: a host function that uses a header's names
// cannot go above every include; a core function the core still uses
// cannot go below it; the two together can, with their ids; a host
// function that uses no header can go up into the core; an include can
// move where it stays above its names' uses, and not below one.
func TestMoveAcrossLine(t *testing.T) {
	path, g, e := incGraph(t, incFull)
	if err := e.MoveToCore(defnNamed(t, e, "host_len")); err == nil || !strings.Contains(err.Error(), "`getenv`") {
		t.Fatalf("host_len moved above the includes: %v", err)
	}
	helper, user := defnNamed(t, e, "helper"), defnNamed(t, e, "user")
	if err := e.MoveToHost(helper); err == nil || !strings.Contains(err.Error(), "no longer be declared above") {
		t.Fatalf("helper moved below user: %v", err)
	}
	if err := e.MoveToHost(helper, user); err != nil {
		t.Fatal(err)
	}
	act := e.Log[len(e.Log)-1]
	if act.Op != "move" || !slices.Equal(act.Moved, []ID{helper.ID, user.ID}) || !e.Live(helper) || !e.InHost(user) {
		t.Fatalf("the act %+v", act)
	}
	if err := e.MoveToCore(defnNamed(t, e, "twice")); err != nil {
		t.Fatal(err)
	}
	if err := e.MoveFormsAfter(defnNamed(t, e, "host_len"), includeOf(t, e, "<string.h>")); err == nil || !strings.Contains(err.Error(), "`strlen`") {
		t.Fatalf("<string.h> moved below strlen's use: %v", err)
	}
	if err := e.MoveFormsAfter(includeOf(t, e, "<ctype.h>"), includeOf(t, e, "<stdlib.h>")); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	incWant(t, path, g, incSample(incEnum, incProto, incCore, incTwice,
		incString, incCtype, incStdlib, incHelper, incUser, incAssert, incLen, incMain))
}

// A header's macro over a name the file only uses below the include (it
// declares it above): refused under the rule, and made by
// InsertIncludeRebind, the use becoming the macro's token with no edge --
// what an import of the text after makes of it.
func TestInsertIncludeRebind(t *testing.T) {
	src := incSample(incEnum, incProto, incCore, incString, incTwice, incAssert,
		"int main(void) { return core_fn(1) + (int)strlen(\"x\"); }\n")
	path, g, e := incGraph(t, src)
	str := includeOf(t, e, "<string.h>")
	_, err := e.InsertIncludeAfter(str, "<stdlib.h>")
	var ce *CollisionError
	if !errors.As(err, &ce) || !ce.Collisions[0].Use {
		t.Fatalf("under the rule: %v", err)
	}
	inc, toks, err := e.InsertIncludeRebind(str, "<stdlib.h>", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(toks) != 1 || toks[0].Ref() != nil || toks[0].Atom != "EXIT_FAILURE" || IncludeSpec(inc) != "<stdlib.h>" {
		t.Fatalf("the tokens %v", toks)
	}
	if err := e.Check(); err != nil {
		t.Fatal(err)
	}
	after := strings.Replace(src, incString, incString+incStdlib, 1)
	incWant(t, path, g, after)
	_, _, h := importSample(t, after)
	for _, f := range h.Forms {
		if f.Is("static_assert") {
			Walk(f, func(n *Node) bool {
				if !n.list && n.Atom == "EXIT_FAILURE" && n.Ref() != nil {
					t.Errorf("the import of the text after: EXIT_FAILURE refers to %v", n.Ref())
				}
				return true
			})
		}
	}
	if us, _ := e.HeaderUses(); !slices.ContainsFunc(us, func(u HeaderUse) bool {
		return u.Macro && u.Name == "EXIT_FAILURE" && u.From == inc
	}) {
		t.Errorf("EXIT_FAILURE not the new include's: %v", us)
	}
}
