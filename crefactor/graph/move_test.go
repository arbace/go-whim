package graph

import (
	"slices"
	"testing"
)

// A local moved to the outer block: its id, its uses' edges, kept.
func TestMoveLocal(t *testing.T) {
	src := `int
f(int a)
{
    int r = 0;
    if (a)
    {
        int t = a * 2;
        r = t;
    }
    return r;
}
int main(void) { return f(1); }
`
	e, v, path := b2c(t, src)
	tdef := v.One("(def t int (* a 2))", "t")
	iff := v.One("(if a _)", "the if")
	id, uses := tdef.ID, e.Uses(tdef)
	if err := e.MoveBefore(tdef, iff); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `int
f(int a)
{
    int r = 0;
    int t = a * 2;
    if (a)
    {
        r = t;
    }
    return r;
}
int main(void) { return f(1); }
`)
	act := e.Log[len(e.Log)-1]
	if tdef.ID != id || !slices.Equal(e.Uses(tdef), uses) || act.Op != "move" || !slices.Contains(act.Moved, id) || len(act.Gone) != 0 {
		t.Errorf("ids: #%d (was #%d), act %+v", tdef.ID, id, act)
	}
}

// What MOVE refuses, the graph as it was: a use before its declaration, a
// declaration that would hide another a use names, one declared twice, a
// break that would bind elsewhere, a goto out of its function, a node into
// itself.
func TestMoveRefuses(t *testing.T) {
	src := `static int t;
int
f(int a)
{
    int r = 0;
    while (a)
    {
        if (a > 1)
        {
            int t = a * 2;
            r = t;
            break;
        }
        a--;
    }
    if (r)
    {
        int r2 = 1;
        goto out;
    }
out:
    return r + t;
}
int g(int b) { int r = b; return r; }
int main(void) { return f(1) + g(2); }
`
	e, v, _ := b2c(t, src)
	before := cview(t, e)
	tdef := v.One("(def t int (* a 2))", "t")
	ret := v.One("(return (+ r t))", "the return")
	rdef := v.One("(def r int 0)", "r")
	brk := v.One("(break)", "the break")
	wh := v.One("(while a _)", "the while")
	got := v.One("(goto out)", "the goto")
	gret := v.One("(return r)", "g's return")
	inner := v.One("(if (> a 1) _)", "the inner if")
	unchanged(t, e, before, e.MoveAfter(tdef, ret), "would resolve to")
	unchanged(t, e, before, e.MoveBefore(tdef, wh), "would resolve to #")
	unchanged(t, e, before, e.MoveBefore(tdef, gret), "would resolve to #1 (def t)") // f's uses of t
	unchanged(t, e, before, e.MoveBefore(rdef, gret), "is declared again")
	unchanged(t, e, before, e.MoveBefore(brk, wh), "would bind to")
	unchanged(t, e, before, e.MoveBefore(got, gret), "would leave the function of its label")
	unchanged(t, e, before, e.MoveBefore(inner, brk), "into what it holds")
}

// A run of statements moved into another block, and a function moved
// before its first use; moved above its prototype, the uses go to it, the
// first declaration, as the importer would have them.
func TestMoveRunAndFunctions(t *testing.T) {
	src := `static int g(int);
int
main(void)
{
    int a = 1;
    int b = 2;
    a++;
    b++;
    if (a)
    {
        b = 0;
    }
    return g(a + b);
}
int g(int x) { return x; }
`
	e, v, path := b2c(t, src)
	inc := v.One("(post++ a)", "a++")
	incb := v.One("(post++ b)", "b++")
	store := v.One("(= b 0)", "b = 0")
	if err := e.MoveRun(inc, incb, store, true); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `static int g(int);
int
main(void)
{
    int a = 1;
    int b = 2;
    if (a)
    {
        b = 0;
        a++;
        b++;
    }
    return g(a + b);
}
int g(int x) { return x; }
`)
	g := e.Defn("g")
	proto := e.FileDecls("g")[0]
	call := v.One("(call g _)", "the call")
	if err := e.MoveBefore(g, e.Defn("main")); err != nil {
		t.Fatal(err)
	}
	if call.Kids[1].Ref() != proto {
		t.Errorf("the call refers to %s, not the prototype", label(call.Kids[1].Ref()))
	}
	if err := e.MoveBefore(g, proto); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `int g(int x) { return x; }
static int g(int);
int
main(void)
{
    int a = 1;
    int b = 2;
    if (a)
    {
        b = 0;
        a++;
        b++;
    }
    return g(a + b);
}
`)
	if call.Kids[1].Ref() != g {
		t.Errorf("the call refers to %s, not the definition, now the first declaration", label(call.Kids[1].Ref()))
	}
	// a function moved after its only use with no prototype before it
	e, v, _ = b2c(t, `int g(int x) { return x; }
int main(void) { return g(1); }
`)
	before := cview(t, e)
	unchanged(t, e, before, e.MoveAfter(e.Defn("g"), e.Defn("main")), "`g` at #")
	_ = v
}

// Phase 78's shape: an argument moved into a new local's value, the place
// it leaves taken by a use of the local -- the argument's ids kept, the
// placeholder superseded, the fill given, nothing untyped.
func TestMoveTo(t *testing.T) {
	src := `static char *get(void);
static int get_len(void);
static char *save(char *p, int n);
int
main(void)
{
    char *s;
    s = save(get(), get_len());
    return s != 0;
}
char *get(void) { return 0; }
int get_len(void) { return 0; }
char *save(char *p, int n) { return p; }
`
	e, v, path := b2c(t, src)
	stmt := v.One("(= s (call save _ _))", "the store")
	arg := v.One("(call get_len)", "the argument")
	blk, err := e.Build(stmt, "(block (def len int 0) ?s)", Bindings{"s": stmt})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Replace(stmt, blk...); err != nil {
		t.Fatal(err)
	}
	def := v.One("(def len int 0)", "the local")
	fill, err := e.Build(arg, "len", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, saveCall := arg.ID, v.One("(call save _ _)", "save")
	before := cview(t, e)
	unchanged(t, e, before, e.MoveTo(arg, def.Kids[3], nil), "say what fills it")
	if err := e.MoveTo(arg, def.Kids[3], fill[0]); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `static char *get(void);
static int get_len(void);
static char *save(char *p, int n);
int
main(void)
{
    char *s;
    {
        int len = get_len();
        s = save(get(), len);
    }
    return s != 0;
}
char *get(void) { return 0; }
int get_len(void) { return 0; }
char *save(char *p, int n) { return p; }
`)
	if arg.ID != id || def.Kids[3] != arg || saveCall.Type == nil || len(e.Untyped) != 0 || fill[0].ID == 0 {
		t.Errorf("arg #%d (was #%d), save typed %v, %d untyped", arg.ID, id, saveCall.Type != nil, len(e.Untyped))
	}
	// a local's use moved where the local is not visible
	use := v.One("(call save (call get) len)", "the call").Kids[3]
	other := v.One("(!= s 0)", "the test").Kids[2]
	before = cview(t, e)
	fill2, _ := e.Build(use, "0", nil)
	unchanged(t, e, before, e.MoveTo(use, other, fill2[0]), "would resolve to nothing")
}
