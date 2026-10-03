package graph

import (
	"bytes"
	"strings"
	"testing"
)

const renames = `struct s;
typedef struct s s_T;
struct s { int x; int y; struct { int in; }; };
static int f(int a);
static int g;
static int f(int a)
{
    int b = a;
again:
    if (b) { b--; goto again; }
    return b + g + sizeof("f");
}
int main(void)
{
    s_T v = { .x = 1, .in = 2 };
    v.x = 2;
    struct s *p = &v;
    return f(p->x) + v.in;
}
`

// rename renames what the declaration of name -- the file's, or the one
// pick chooses -- declares, and holds the C to want.
func TestRename(t *testing.T) {
	for _, c := range []struct {
		what string
		pick func(e *Editor) *Node
		to   string
		want *strings.Replacer
	}{
		{"a function and its prototype", func(e *Editor) *Node { return e.Defn("f") }, "h",
			strings.NewReplacer("int f(", "int h(", "return f(", "return h(")},
		{"an object", func(e *Editor) *Node { return e.FileDecls("g")[0] }, "glob",
			strings.NewReplacer("int g;", "int glob;", "+ g +", "+ glob +")},
		{"a typedef", func(e *Editor) *Node { return e.Decls("s_T")[0] }, "S",
			strings.NewReplacer("s_T", "S")},
		{"a tag, its forward declaration and its uses", func(e *Editor) *Node { return e.tagDef("struct", "s") }, "pt",
			strings.NewReplacer("struct s", "struct pt")},
		{"a member: its selections and designators", func(e *Editor) *Node { return memberNamed(e.tagDef("struct", "s"), "x") }, "x0",
			strings.NewReplacer("int x;", "int x0;", ".x =", ".x0 =", "v.x", "v.x0", "p->x", "p->x0")},
		{"a member of an anonymous struct", func(e *Editor) *Node { return memberNamed(e.tagDef("struct", "s"), "in") }, "inner",
			strings.NewReplacer("int in;", "int inner;", ".in =", ".inner =", "v.in", "v.inner")},
		{"a local", func(e *Editor) *Node { return Body(e.Defn("f"))[0] }, "c",
			strings.NewReplacer("int b = a", "int c = a", "(b)", "(c)", "b--", "c--", "return b +", "return c +")},
		{"a parameter", func(e *Editor) *Node { return paramNamed(e.Defn("f"), "a") }, "n",
			strings.NewReplacer("static int f(int a)\n{", "static int f(int n)\n{", "b = a", "b = n")},
		{"a label", func(e *Editor) *Node { return Body(e.Defn("f"))[1] }, "loop",
			strings.NewReplacer("again", "loop")},
	} {
		t.Run(c.what, func(t *testing.T) {
			e := editorOn(t, renames)
			r, err := e.Rename(c.pick(e), c.to)
			if err != nil {
				t.Fatal(err)
			}
			want := c.want.Replace(renames)
			if c.what == "a parameter" { // the prototype's parameter is a name of its own
				want = strings.Replace(want, "static int f(int n);", "static int f(int a);", 1)
			}
			isC(t, e, want)
			act := e.Log[len(e.Log)-1]
			if act.Op != "rename" || len(act.Gone) != 0 || len(act.New) != 0 || len(act.Moved) != len(r.Decls)+len(r.Uses) {
				t.Errorf("the act %+v", act)
			}
		})
	}
}

func TestRenameRefused(t *testing.T) {
	for _, c := range []struct {
		what, src string
		pick      func(e *Editor) *Node
		to, says  string
	}{
		{"a name the file declares", renames, func(e *Editor) *Node { return e.Defn("f") }, "g", "g is declared at file scope already"},
		{"a local the new name would take a use from",
			"static int x;\nstatic int f(void) { int y = 0; return x + y; }\n",
			func(e *Editor) *Node { return e.FileDecls("x")[0] }, "y", "would name"},
		{"an outer use the renamed local would capture",
			"static int g;\nstatic int f(void) { int x = 1; return x + g; }\n",
			func(e *Editor) *Node { return Body(e.Defn("f"))[0] }, "g", "would name the renamed one"},
		{"a local of the same scope",
			"static int f(void) { int x = 1; int y = 2; return x + y; }\n",
			func(e *Editor) *Node { return Body(e.Defn("f"))[0] }, "y", "declares y in the same scope"},
		{"a parameter of the same function",
			"static int f(int a) { int x = a; return x; }\n",
			func(e *Editor) *Node { return Body(e.Defn("f"))[0] }, "a", "has a parameter a"},
		{"a member of the same struct", renames, func(e *Editor) *Node { return memberNamed(e.tagDef("struct", "s"), "x") }, "in", "member in of the same struct"},
		{"a tag", "struct a { int v; };\nunion b { int w; };\nint f(struct a *p, union b *q) { return p->v + q->w; }\n",
			func(e *Editor) *Node { return e.tagDef("struct", "a") }, "b", "tags share one name space"},
		{"a label", "int f(int n) { a: if (n--) goto a; b: return n; }\n",
			func(e *Editor) *Node { return Body(e.Defn("f"))[0] }, "b", "has a label b already"},
		{"an external", "#include <string.h>\nunsigned long f(char *s) { return strlen(s); }\n",
			func(e *Editor) *Node { return e.g.Externs[0] }, "len", "not a declaration Rename takes"},
		{"not an identifier", renames, func(e *Editor) *Node { return e.Defn("f") }, "int", "not an identifier"},
	} {
		t.Run(c.what, func(t *testing.T) {
			e := editorOn(t, c.src)
			_, err := e.Rename(c.pick(e), c.to)
			mustFail(t, err, c.says)
			if len(e.Log) != 0 {
				t.Errorf("a refusal edited: %+v", e.Log)
			}
		})
	}
}

// A macro's text names the declaration: a respelling cannot reach it.
// The graph is written by hand, as the importer writes a recovered macro.
func TestRenameMacroRefused(t *testing.T) {
	lisp := `#1(defn f
  #2(fn #3(#4(n int)@:10) int)
  #5(return #6(macro "MIN(10, n)")@4@:10))@:11

(types
  #10(basic int)
  #11(function #12(@:10) @:10))

(externs)

(ids 12)
`
	g, err := Read([]byte(lisp))
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(g)
	_, err = e.Rename(paramNamed(e.Defn("f"), "n"), "m")
	mustFail(t, err, "names it in text a respelling cannot reach")
}

func TestRetargetAs(t *testing.T) {
	src := `#include <string.h>
static unsigned long my_strlen(const char *s) { unsigned long n = 0; while (s[n]) n++; return n; }
static unsigned long f(const char *a, const char *b) { return strlen(a) + strlen(b); }
static unsigned long h(const char *my_strlen) { return strlen(my_strlen); }
`
	e := editorOn(t, src)
	var ext *Node
	for _, x := range e.g.Externs {
		if ordinaryName(x) == "strlen" {
			ext = x
		}
	}
	// h's parameter is named my_strlen: refused, all or none
	_, err := e.RetargetUses(ext, e.Defn("my_strlen"))
	mustFail(t, err, "names another declaration")
	if len(e.Log) != 0 {
		t.Fatal("a refusal edited")
	}
	uses := e.Uses(ext)
	for _, u := range uses[:2] {
		if err := e.RetargetAs(u, 0, e.Defn("my_strlen")); err != nil {
			t.Fatal(err)
		}
	}
	mustFail(t, e.RetargetAs(uses[2], 0, e.Defn("my_strlen")), "names #")
	isC(t, e, strings.Replace(src, "strlen(a) + strlen(b)", "my_strlen(a) + my_strlen(b)", 1))
	if e.Log[0].Op != "retarget-as" || uses[0].Ref() != e.Defn("my_strlen") {
		t.Errorf("%+v", e.Log)
	}
}

func TestRespellString(t *testing.T) {
	src := `static char *term = "xterm";
static int n = sizeof("xterm");
void say(const char *);
static void f(void) { say("' not known, defaulting to 'xterm'"); say(("xterm")); say("xterm"); }
`
	e := editorOn(t, src)
	var log bytes.Buffer
	v := NewVerbs("terms", e, &log)
	v.InFunction("f", func(v *Verbs) {
		v.RespellString(`"' not known, defaulting to 'xterm'"`, `"' not known, defaulting to 'xterm-256color'"`, 1, "the message names the fallback")
		v.RespellString(`"xterm"`, `"xterm-256color"`, 2, "and so do the two calls")
	})
	v.RespellString(`"xterm"`, `"xterm-256color"`, 1, "the file's three: refused")
	if err := v.Done(); err == nil || !strings.Contains(err.Error(), "occurs 2 times, expected 1") {
		t.Fatalf("err = %v", err)
	}
	if log.String() != "  terms        the message names the fallback\n  terms        and so do the two calls\n" {
		t.Errorf("reported %q", log.String())
	}
	// the parenthesised literal is typed as the pointer it decays to (cc's
	// type for it, even under sizeof): no array type above it to clear
	if len(e.Untyped) != 0 {
		t.Errorf("untyped %v", e.Untyped)
	}
	isC(t, e, `static char *term = "xterm";
static int n = sizeof("xterm");
void say(const char *);
static void f(void) { say("' not known, defaulting to 'xterm-256color'"); say(("xterm-256color")); say("xterm-256color"); }
`)
	for _, bad := range []string{`xterm`, `"a" "b"`, `"a`, `"a"b"`, `LLL"a"`} {
		if stringToken(bad) {
			t.Errorf("%s is one string literal", bad)
		}
	}
	for _, good := range []string{`"a\"b"`, `L"x"`, `u8"x"`, `""`} {
		if !stringToken(good) {
			t.Errorf("%s is not one string literal", good)
		}
	}
}
