package graph

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/edit"
)

// The verbs held to crefactor/edit's: each case runs the graph verb on the
// graph read back from its Lisp (no cc node behind it), and its C view must
// be, byte for byte, what the text verb gives on the canonical text,
// printed canonically.  Where that text verb is one of crefactor/edit's
// functions (FoldNever, ReplaceLiteral, ...) the test runs it; where it was
// one of the text verb set's (edit.E, deleted after 864655e, the last
// commit that has it), its result is a golden file, testdata/textverbs/
// (textGolden).  The cases are crefactor/edit's own (acts_test.go), then
// one for each verb they do not cover.

// verbsOn is src's graph, read back from its Lisp, with verbs reporting
// into log; path and the canonical text are the text side's.
func verbsOn(t *testing.T, src string) (string, []byte, *Verbs, *bytes.Buffer) {
	t.Helper()
	path, canon, g := importSample(t, src)
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	return path, canon, NewVerbs("tiny", NewEditor(h), &log), &log
}

// sameAsText holds the graph verbs' result to the text verb's.
func sameAsText(t *testing.T, src string, text func([]byte) ([]byte, error), graph func(*Verbs)) {
	t.Helper()
	path, canon, v, _ := verbsOn(t, src)
	out, err := text(canon)
	if err != nil {
		t.Fatalf("the text verb: %v", err)
	}
	want, err := cemit.Canonical(path, out)
	if err != nil {
		t.Fatalf("the text verb's result does not print: %v\n%s", err, out)
	}
	sameAs(t, v, want, "text", graph)
}

// sameAsGolden holds the graph verbs' result on src to the text verb set's,
// recorded in testdata/textverbs/NAME.c (textGolden).
func sameAsGolden(t *testing.T, src, name string, graph func(*Verbs)) {
	t.Helper()
	_, canon, v, _ := verbsOn(t, src)
	sameAs(t, v, textGolden(t, name, canon), "testdata/textverbs/"+name+".c", graph)
}

// textGolden is testdata/textverbs/NAME.c: what crefactor/edit's text verb
// set (edit.E) gave on canon, printed canonically.  Each file was written
// once at 864655e, the last commit with the text verbs, by the acts its
// use's comment lists, run on the sample's canonical text with
// edit.New("tiny", canon, log), Done's result put through cemit.Canonical
// with the sample's path -- the steps this test then ran itself.  Every
// file differs from its sample (every act there changed it), so a golden
// that has become its sample is refused; a byte changed in one fails the
// test that reads it, which names it.
func textGolden(t *testing.T, name string, canon []byte) []byte {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "textverbs", name+".c"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(want, canon) {
		t.Fatalf("testdata/textverbs/%s.c is its sample unchanged", name)
	}
	return want
}

// sameAs runs the graph verbs and holds the C view to want, which is what's.
func sameAs(t *testing.T, v *Verbs, want []byte, what string, graph func(*Verbs)) {
	t.Helper()
	graph(v)
	if err := v.Done(); err != nil {
		t.Fatalf("the graph verb: %v", err)
	}
	if err := v.Editor().Check(); err != nil {
		t.Fatal(err)
	}
	got, err := v.Editor().Graph().C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s\ngraph:\n%s\n%s:\n%s", firstDiff(got, want), got, what, want)
	}
}

// refuses holds a graph verb to its refusal.
func refuses(t *testing.T, src string, graph func(*Verbs), saying string) {
	t.Helper()
	_, _, v, log := verbsOn(t, src)
	graph(v)
	err := v.Done()
	if err == nil || !strings.Contains(err.Error(), saying) {
		t.Errorf("err = %v, want a refusal saying %q", err, saying)
	}
	if log.Len() != 0 {
		t.Errorf("a refusal reported %q", log.String())
	}
	if err := v.Editor().Check(); err != nil {
		t.Fatal(err)
	}
}

// chain is crefactor/edit's acts_test.go sample, as it is there.
const chain = `int
f(int a, int b)
{
    if (a)
    {
        b++;
        if (b)
        {
            b--;
        }
    }
    if (b > 1)
    {
        a = 1;
    }
    else if (b > 2)
    {
        a = 2;
    }
    else
    {
        a = 3;
    }
    return a;
}
`

// crefactor/edit's TestFolds: each shape of if-chain, folded.
func TestVerbFolds(t *testing.T) {
	for _, c := range []struct {
		name  string
		text  func([]byte, string, int) ([]byte, error)
		head  string
		graph func(*Verbs)
	}{
		{"always: keep the body, inner block and all", edit.FoldAlways, "if (a)", func(v *Verbs) { v.FoldAlways("a", 1, "x") }},
		{"drop: the whole block, inner block and all", edit.DropIf, "if (a)", func(v *Verbs) { v.DropIf("a", 1, "x") }},
		{"never, on a plain if", edit.FoldNever, "if (a)", func(v *Verbs) { v.FoldNever("a", 1, "x") }},
		{"never, heading a chain: the else if heads it", edit.FoldNever, "if (b > 1)", func(v *Verbs) { v.FoldNever("(> b 1)", 1, "x") }},
		{"never, inside a chain: the arm goes", edit.FoldNever, "else if (b > 2)", func(v *Verbs) { v.FoldNever("(> b 2)", 1, "x") }},
		{"never, matched as the if", edit.FoldNever, "if (b > 1)", func(v *Verbs) { v.FoldNever("(if (> b 1) _*)", 1, "x") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			sameAsText(t, chain, func(s []byte) ([]byte, error) { return c.text(s, edit.Head(c.head), 1) }, c.graph)
		})
	}
	// if (F) { A } else { C } -> C
	ifElse := "void h(int);\nvoid\ng(int a)\n{\n    if (a)\n    {\n        a = 1;\n    }\n    else\n    {\n        a = 2;\n    }\n    h(a);\n}\n"
	sameAsText(t, ifElse, func(s []byte) ([]byte, error) { return edit.FoldNever(s, edit.Head("if (a)"), 1) },
		func(v *Verbs) { v.FoldNever("a", 1, "x") })
}

// crefactor/edit's TestFoldRefusals: a count that is not the one given, a
// shape a fold does not fold, an else it would orphan.
func TestVerbFoldRefusals(t *testing.T) {
	for _, c := range []struct {
		name    string
		graph   func(*Verbs)
		refusal string
	}{
		{"never, uncounted", func(v *Verbs) { v.FoldNever("(if _*)", 1, "x") }, "matched 4 times, expected 1 -- a fold that is not counted is a guess"},
		{"always, absent", func(v *Verbs) { v.FoldAlways("z", 1, "x") }, "matched 0 times, expected 1"},
		{"always, with an else", func(v *Verbs) { v.FoldAlways("(> b 1)", 1, "x") }, "fold_always: the block has an else"},
		{"always, an else-if", func(v *Verbs) { v.FoldAlways("(> b 2)", 1, "x") }, "only a plain if"},
		{"drop, with an else", func(v *Verbs) { v.DropIf("(> b 1)", 1, "x") }, "block has an else"},
		{"drop, absent", func(v *Verbs) { v.DropIf("z", 1, "x") }, "matched 0 times"},
		{"always-else, no else", func(v *Verbs) { v.FoldAlwaysElse("a", 1, "x") }, "expected an else"},
		{"always-else, an else-if", func(v *Verbs) { v.FoldAlwaysElse("(> b 1)", 1, "x") }, "expected an else"},
		{"a pattern that does not read", func(v *Verbs) { v.FoldNever("(> b", 1, "x") }, "x -- "},
	} {
		t.Run(c.name, func(t *testing.T) { refuses(t, chain, c.graph, c.refusal) })
	}
}

// FoldAlwaysElse and KeepThen against the text's FoldAlwaysElse and the
// cutters' keepThenChain shape.
func TestVerbKeepThen(t *testing.T) {
	src := "void h(int);\nvoid\ng(int a)\n{\n    if (a)\n    {\n        h(1);\n    }\n    else\n    {\n        h(2);\n    }\n    if (a > 1)\n    {\n        h(3);\n    }\n    else if (a > 2)\n    {\n        h(4);\n    }\n    else\n    {\n        h(5);\n    }\n}\n"
	// keepthen-else.c: e.FoldAlwaysElse(edit.Head("if (a)"), 1, "x")
	sameAsGolden(t, src, "keepthen-else", func(v *Verbs) { v.FoldAlwaysElse("a", 1, "x") })
	sameAsText(t, src, func(s []byte) ([]byte, error) {
		return edit.ReplaceLiteral(s, "    if (a > 1)\n    {\n        h(3);\n    }\n    else if (a > 2)\n    {\n        h(4);\n    }\n    else\n    {\n        h(5);\n    }\n", "        h(3);\n", 1)
	}, func(v *Verbs) { v.KeepThen("(> a 1)", 1, "x") })
}

// crefactor/edit's TestEDriver: the acts in order, each reported under its
// tag as it succeeds, the first refusal stopping the rest.
func TestVerbDriver(t *testing.T) {
	src := "static int count;\n\nstatic void\nbump(void)\n{\n    count++;\n    if (count > 9)\n    {\n        count = 0;\n    }\n}\n\nint\nmain(void)\n{\n    bump();\n    return count;\n}\n"
	// driver.c, and textLog what the text verbs reported doing it:
	//
	//	e.Literal("count++;", "count += 2;", 1, "bump steps by two")
	//	e.InFunction("bump", func(e *edit.E) {
	//		e.FoldNever(edit.Head("if (count > 9)"), 1, "no wrap")
	//	})
	//	e.CountIs(`\bcount\b`, 3, "count's mentions")
	//	e.Lines(`bump\(\);`, 1, "main calls nothing")
	//	e.DeleteDefinition("bump", "bump goes")
	textLog := "  tiny         bump steps by two\n  tiny         no wrap\n  tiny         main calls nothing\n  tiny         bump goes\n"
	sameAsGolden(t, src, "driver", func(v *Verbs) {
		v.Rewrite("(post++ count)", "(+= count 2)", 1, "bump steps by two")
		v.InFunction("bump", func(v *Verbs) {
			v.FoldNever("(> count 9)", 1, "no wrap")
		})
		v.TextCountIs(`\bcount\b`, 3, "count's mentions")
		v.Expect(len(v.UsesOf("count")) == 2 && v.Mentions("count") == 3, "count: %d uses, %d mentions", len(v.UsesOf("count")), v.Mentions("count"))
		v.Cut("(call bump)", 1, "main calls nothing")
		v.DeleteDefinition("bump", "bump goes")
		if got, want := v.W.(*bytes.Buffer).String(), textLog; got != want {
			t.Errorf("reported\n%s\nthe text reported\n%s", got, want)
		}
	})

	// the first refusal stops the rest, and nothing after it reports
	_, _, v, log := verbsOn(t, src)
	v.Rewrite("(post++ count)", "(post-- count)", 1, "first")
	v.Rewrite("count", "n", 1, "second, miscounted")
	v.Rewrite("(post-- count)", "(-= count 1)", 1, "third")
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "second, miscounted -- matched 5 times, expected 1") {
		t.Errorf("Done = %v", err)
	}
	if out, _ := v.Editor().Graph().C(); !bytes.Contains(out, []byte("count--;")) || bytes.Contains(out, []byte("count -= 1;")) {
		t.Errorf("the graph is not what the first act left:\n%s", out)
	}
	if log.String() != "  tiny         first\n" {
		t.Errorf("reported after the refusal:\n%s", log.String())
	}
	refuses(t, src, func(v *Verbs) {
		v.InFunction("missing", func(*Verbs) { t.Error("the acts ran on a missing definition") })
	}, "missing is not defined")
	refuses(t, src, func(v *Verbs) { v.DeleteDefinition("bump", "bump goes") }, "bump goes -- `bump` still refers to the definition")
}

const walks = `struct buf { struct buf *next; int n; };
static struct buf *firstbuf;
static struct buf *curbuf;
int
walk(void)
{
    struct buf *bp;
    int total = 0;
    for (bp = firstbuf; bp != 0; bp = bp->next)
    {
        int k = bp->n;
        total += k;
    }
    return total;
}
int
escapes(void)
{
    struct buf *bp;
    for (bp = firstbuf; bp != 0; bp = bp->next)
    {
        if (bp->n)
        {
            break;
        }
        switch (bp->n)
        {
        case 1:
            break;
        }
    }
    return 0;
}
int
clashes(void)
{
    struct buf *bp;
    int k = 0;
    for (bp = firstbuf; bp != 0; bp = bp->next)
    {
        int k = bp->n;
        (void)k;
    }
    return k;
}
`

// FoldWalk and FoldWalks against the text's, their declaring bodies spliced
// as the text splices them; the refusals, a break that would rebind and a
// declaration that would clash.
func TestVerbWalks(t *testing.T) {
	// walks-foldwalk.c:
	//
	//	e.FoldWalk("walk", "bp", "curbuf", "for (bp = firstbuf; bp != 0; bp = bp->next)", 1, "one buffer")
	sameAsGolden(t, walks, "walks-foldwalk", func(v *Verbs) {
		v.InFunction("walk", func(v *Verbs) { v.FoldWalk("(for (= bp firstbuf) _*)", "(= bp curbuf)", 1, "one buffer") })
	})
	// walks-foldwalks.c:
	//
	//	e.InFunction("walk", func(e *edit.E) {
	//		e.FoldWalks(`for \((bp) = firstbuf; bp != 0; bp = bp->next\)`, func([]string) bool { return true },
	//			func(g []string) string { return g[2] + " = curbuf;" }, "one buffer")
	//	})
	sameAsGolden(t, walks, "walks-foldwalks", func(v *Verbs) {
		v.InFunction("walk", func(v *Verbs) {
			v.FoldWalks("(for (= ?v firstbuf) _*)", func(Bindings) bool { return true },
				func(b Bindings) string { return "(= " + b["v"].Atom + " curbuf)" }, "one buffer")
		})
	})
	refuses(t, walks, func(v *Verbs) {
		v.InFunction("escapes", func(v *Verbs) { v.FoldWalk("(for _*)", "(= bp curbuf)", 1, "x") })
	}, "binds to the walk being removed")
	refuses(t, walks, func(v *Verbs) {
		v.InFunction("clashes", func(v *Verbs) { v.FoldWalk("(for _*)", "(= bp curbuf)", 1, "x") })
	}, "`k` is declared again where it would move")
	refuses(t, walks, func(v *Verbs) {
		v.FoldWalks("(for _*)", func(Bindings) bool { return true }, func(Bindings) string { return "(= bp curbuf)" }, "x")
	}, "1 walk(s) still carry an escaping break/continue")
}

const blocks = `void f(void);
void g2(void);
int
h(int a)
{
    if (a)
    {
        f();
    }
    {
        int x;
        g2();
    }
    a = a + 1;
    return a;
}
int
sw(int c)
{
    int x = 0;
    switch (c)
    {
    case 'a':
    case 'b':
        x = 2;
        break;
    case 'n':
        x = 1;
        break;
    case 'f':
        x = 4;
    case 'g':
        x = 5;
        break;
    default:
        x = 3;
    }
    if (c && x > 1 && x < 9)
    {
        x = 0;
    }
    return x;
}
`

// The block verbs: DropBareBlock, Splice, Body and DeleteDefinition, and the
// shapes the cutters wrote by hand -- a case dropped, an operand dropped.
func TestVerbBlocks(t *testing.T) {
	// blocks-dropbareblock.c: e.DropBareBlock("h", "g2();", "the husk")
	sameAsGolden(t, blocks, "blocks-dropbareblock", func(v *Verbs) { v.InFunction("h", func(v *Verbs) { v.DropBareBlock("(call g2)", "the husk") }) })
	// blocks-splice.c: e.Splice("    if (a)\n", "    a = a + 1;\n", "    f();\n", "the run")
	sameAsGolden(t, blocks, "blocks-splice", func(v *Verbs) { v.Splice("(if a _*)", "(= a _)", "(call f)", "the run") })
	// blocks-body.c: e.Body("h", "    f();\n    return 0;\n", "a stub")
	sameAsGolden(t, blocks, "blocks-body", func(v *Verbs) { v.Body("h", "(call f) (return 0)", "a stub") })
	// blocks-cases.c:
	//
	//	e.InFunction("sw", func(e *edit.E) {
	//		e.Lines(`case 'a':`, 1, "a shares b's")
	//		e.Cut(edit.Line("case 'n':", "x = 1;", "break;"), 1, "n's run")
	//		e.Literal("c && x > 1 && x < 9", "c && x < 9", 1, "an operand")
	//	})
	sameAsGolden(t, blocks, "blocks-cases", func(v *Verbs) {
		v.InFunction("sw", func(v *Verbs) {
			v.DropCase("(case 'a')", 1, "a shares b's")
			v.DropCase("(case 'n')", 1, "n's run")
			v.DropOperand("(> x 1)", 1, "an operand")
		})
	})
	sameAsText(t, blocks, func(s []byte) ([]byte, error) {
		return edit.ReplaceLiteral(s, "c && x > 1 && x < 9", "x > 1 && x < 9", 1)
	}, func(v *Verbs) { v.DropOperand("c", 1, "an operand") })
	refuses(t, blocks, func(v *Verbs) { v.DropCase("(case 'g')", 1, "x") }, "falls into it")
	refuses(t, blocks, func(v *Verbs) { v.DropBareBlock("(call f)", "x") }, "the block is if's, not an item")
	refuses(t, blocks, func(v *Verbs) { v.Splice("(= a _)", "(if a _*)", "", "x") }, "its end comes before its start")
	refuses(t, blocks, func(v *Verbs) { v.Body("nowhere", "", "x") }, "nowhere is not defined at file scope")
	refuses(t, blocks, func(v *Verbs) { v.Cut("(= x _)", 2, "x") }, "matched 6 times, expected 2")
	refuses(t, blocks, func(v *Verbs) { v.Cut("(+ a 1)", 1, "x") }, "holds exactly one node")
}

const table = `typedef void (*fn_T)(int);
static void nv_error(int c);
static void nv_op(int c);
static void nv_error(int c) { (void)c; }
static void nv_op(int c) { (void)c; }
static const struct nv_cmd { int c; fn_T f; } nv_cmds[] =
{
    {'a', nv_op},
    {'!', nv_op},
};
static int always(void) { return 1; }
int
main(void)
{
    nv_cmds[1].f(always());
    return 0;
}
`

// A table's row found and one element of it rewritten (the text's
// `${1}nv_error${2}`), and the rows, a row, ConstOf.
func TestVerbTable(t *testing.T) {
	sameAsText(t, table, func(s []byte) ([]byte, error) {
		return edit.ReplacePattern(s, `(?m)^([ \t]*\{'!', )nv_op(\},)$`, "${1}nv_error${2}", 1)
	}, func(v *Verbs) {
		v.InTable("nv_cmds", func(v *Verbs) {
			v.Expect(len(v.Rows()) == 2, "rows: %d", len(v.Rows()))
			v.Expect(v.Row("(init '!' _)", "the ! row") != nil, "no ! row")
			v.RewriteAt("(init '!' ?h)", "h", "nv_error", 1, "the ! row points at nv_error")
		})
		v.ConstOf("always", "1")
	})
	refuses(t, table, func(v *Verbs) { v.ConstOf("always", "0") }, "always returns 1, not 0")
	refuses(t, table, func(v *Verbs) { v.ConstOf("main", "0") }, "main is no longer a one-line stub")
	refuses(t, table, func(v *Verbs) { v.InTable("nv_cmds", func(v *Verbs) { v.Row("(init _ _)", "x") }) }, "2 rows match, expected 1")
	refuses(t, table, func(v *Verbs) { v.InTable("nothing", func(*Verbs) {}) }, "no initialised definition")
}

// FallOut as a verb: the closure's refusal is the acts' refusal.
func TestVerbFallOut(t *testing.T) {
	_, _, v, _ := verbsOn(t, table)
	v.Cut("(defn static always _*)", 1, "always goes")
	v.FallOut(FallOutOptions{})
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "`always` in main refers to") {
		t.Errorf("a call whose value is used: %v", err)
	}
	_, _, v, log := verbsOn(t, blocks)
	v.Cut("(def g2 _)", 1, "g2's prototype")
	st := v.FallOut(FallOutOptions{KeepEmpty: true})
	if err := v.Done(); err != nil || len(st.Removed) != 1 || log.String() != "  tiny         g2's prototype\n" {
		t.Errorf("%v: %d removed, %q", err, len(st.Removed), log.String())
	}
}
