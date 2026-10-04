package graph

import (
	"bytes"
	"strings"
	"testing"
)

// B3f's verbs and the fix to SpliceC: each result held to the text verb's
// C printed canonically and to the import of its C view (asImported).

const b3fSample = `#include <stddef.h>
static int total = 0;
static int g(int x, int y);
static int g(int x, int y)
{
    return x + y;
}
static void h(void)
{
    total = 1;
}
int
f(int a)
{
    int k = a;
    if (a > 1 && a < 9)
    {
        k = g(k, 2);
    }
    g(k, 3);
    total = k;
    return total;
}
`

// One unit whose fragments name each other whichever is spliced first: a
// local declared by one and used by another of the same list, and an object
// declared at file scope used by a function another fragment defines --
// what SpliceC refused before (a refers edge to a node not yet held).
func TestFragPendingEdges(t *testing.T) {
	path, canon, e := fragOn(t, b3fSample)
	// frag-pending-edges.c:
	//
	//	x.Literal("static int total = 0;\n", "static int total = 0;\nstatic int seen = 0;\n", 1, "seen")
	//	x.Literal("    static void\nh(void)\n", "    static void\nmark(void)\n{\n    seen = 1;\n}\n\n    static void\nh(void)\n", 1, "mark")
	//	x.Literal("    int k = a;\n", "    int k = a;\n    int twice = k * 2;\n", 1, "twice")
	//	x.Literal("    total = k;\n", "    mark();\n    total = twice;\n", 1, "uses")
	want := textGolden(t, "frag-pending-edges", canon)
	f := e.Defn("f")
	ds := Find(f, func(n *Node) bool { return n.Is("def") && DeclName(n) == "k" })
	tot := Find(f, func(n *Node) bool {
		return n.Is("=") && len(n.Kids) == 3 && n.Kids[2].Atom == "k" && n.Kids[1].Atom == "total"
	})
	if len(ds) != 1 || len(tot) != 1 {
		t.Fatalf("the sample: %d k, %d stores", len(ds), len(tot))
	}
	if _, err := e.SpliceC(
		Frag{At: e.SpotOf(e.Item(tot[0])), Src: "mark();\ntotal = twice;\n"},
		Frag{At: e.SpotAfter(ds[0]), Src: "int twice = k * 2;\n"},
		Frag{At: e.SpotBefore(e.Defn("h")), Src: "static void mark(void) { seen = 1; }\n"},
		Frag{At: e.SpotAfter(e.FileDecls("total")[0]), Src: "static int seen = 0;\n"},
	); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, want)
	if e.pending != nil {
		t.Error("the pending nodes outlive the splice")
	}
}

// LiteralExprC, ReplaceEachC, AfterEachC, BeforeEachC, WrapEachC and FragAt
// against the text's literals.
func TestFragMoreVerbs(t *testing.T) {
	path, canon, e := fragOn(t, b3fSample)
	// frag-more-verbs.c:
	//
	//	x.Literal("if (a > 1 && a < 9)", "if (total == 0 && a > 1 && a < 9)", 1, "cond")
	//	x.Literal("k = g(k, 2);", "k = g(k, 2);\n        total++;", 1, "after")
	//	x.Literal("    g(k, 3);\n", "    if (k)\n    {\n        g(k, 3);\n    }\n", 1, "wrapped")
	//	x.Literal("    total = k;\n", "    total--;\n    total = k;\n", 1, "before")
	//	x.Literal("    return x + y;\n", "    return x - y;\n", 1, "each")
	//	x.Literal("    static void\nh(void)\n", "static int seen;\n\n    static void\nh(void)\n", 1, "at")
	want := textGolden(t, "frag-more-verbs", canon)
	v := NewVerbs("tiny", e, &bytes.Buffer{})
	v.Together(func(v *Verbs) {
		v.InFunction("f", func(v *Verbs) {
			v.LiteralExprC("(&& _ _)", "a > 1 && a < 9", "total == 0 && a > 1 && a < 9", 1, "cond")
			v.AfterEachC(v.Find("(= k (call g k 2))"), "total++;", 1, "after")
			v.BeforeEachC(v.Find("(= total k)"), "total--;", 1, "before")
			v.WrapEachC(v.Find("(call g k 3)"), "c", "if (k) { $c; }", 1, "wrapped")
		})
		v.InFunction("g", func(v *Verbs) { v.ReplaceEachC(v.Find("(+ x y)"), "x - y", 1, "each") })
		v.FragAt(e.SpotBefore(e.Defn("h")), "static int seen;\n", "at")
	})
	if err := v.Done(); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, want)
	// the counts refuse
	v = NewVerbs("tiny", e, &bytes.Buffer{})
	v.InFunction("f", func(v *Verbs) { v.LiteralExprC("(&& _ _)", "a > 2", "a", 1, "absent") })
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "matched 0 times") {
		t.Errorf("an absent literal: %v", err)
	}
	v = NewVerbs("tiny", e, &bytes.Buffer{})
	v.ReplaceEachC(nil, "0", 1, "none")
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "0 places, expected 1") {
		t.Errorf("no places: %v", err)
	}
}

// A prototype's parameter names respelled: every declaration but the
// definition, held to the text's; refused where the list names the new name
// already, or the declaration names no such parameter.
func TestRenamePrototypeParams(t *testing.T) {
	path, canon, e := fragOn(t, b3fSample)
	// rename-prototype-params.c:
	//	x.Literal("static int g(int x, int y);", "static int g(int lhs, int y);", 1, "proto")
	want := textGolden(t, "rename-prototype-params", canon)
	if _, err := e.RenamePrototypeParams("g", 1, "x"); err == nil {
		t.Error("a name the list has already was taken")
	}
	if _, err := e.RenamePrototypeParams("g", 2, "z"); err == nil {
		t.Error("a parameter g does not have was renamed")
	}
	if _, err := e.RenamePrototypeParams("g", 0, "if"); err == nil {
		t.Error("a keyword was taken for a name")
	}
	n, err := e.RenamePrototypeParams("g", 0, "lhs")
	if err != nil || n != 1 {
		t.Fatalf("%d, %v", n, err)
	}
	asImported(t, path, e, want)
	if _, err := e.RenamePrototypeParams("h", 0, "q"); err == nil {
		t.Error("h has no prototype, and was renamed")
	}
}
