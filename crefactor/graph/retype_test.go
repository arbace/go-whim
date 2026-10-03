package graph

import (
	"strings"
	"testing"
)

// An object's type: every declaration's form, its typed edge, and the
// expressions above its uses typed again -- a comparison stays int, a sum
// is cleared and listed.
func TestRetypeObject(t *testing.T) {
	src := `extern char *flag;
char *flag;
int main(void) { long k = 0; int z = flag == 0; k = flag != 0; return z + (int)k; }
`
	e, v, path := b2c(t, src)
	d := e.FileDecls("flag")[1]
	st, err := e.Retype(d, "int")
	if err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, strings.ReplaceAll(src, "char *flag;", "int flag;"))
	intT := basicType(e.Graph(), []string{"int"})
	if st.Forms != 2 || e.FileDecls("flag")[0].Type != intT || d.Type != intT || len(e.Untyped) != 0 {
		t.Errorf("stats %s; %d untyped", st, len(e.Untyped))
	}
	for _, c := range v.Find("(== flag 0)") {
		if c.Type != intT {
			t.Errorf("%s typed %s", Lisp(c), Lisp(c.Type))
		}
	}

	// a sum is not plain: cleared, listed
	e, v, path = b2c(t, `int main(void) { int k = 1; long z = k + 1; return (int)z; }
`)
	k := v.One("(def k int 1)", "k")
	sum := v.One("(+ k 1)", "the sum")
	if _, err := e.Retype(k, "long"); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, `int main(void) { long k = 1; long z = k + 1; return (int)z; }
`)
	if sum.Type != nil || len(e.Untyped) != 1 || e.Untyped[0] != sum {
		t.Errorf("the sum: %s, untyped %d", Lisp(sum), len(e.Untyped))
	}
	if k.Type == nil || k.Type.Kids[1].Atom != "long" {
		t.Errorf("k typed %s", Lisp(k.Type))
	}
}

// Phase 80's shape: a function's result and a parameter, every declaration,
// the calls typed with the new result, the locals retyped one by one.
func TestRetypeFunction(t *testing.T) {
	src := `typedef struct { int n; } reg_T;
static reg_T regs[2];
static void *get_register(int name);
static void put_register(int name, void *reg);
void *
get_register(int name)
{
    reg_T *reg = &regs[name];
    return (void *)reg;
}
void
put_register(int name, void *reg)
{
    regs[name] = *(reg_T *)reg;
}
int
main(void)
{
    void *r1 = 0;
    r1 = get_register(1);
    put_register(0, r1);
    return 0;
}
`
	e, v, path := b2c(t, src)
	if _, err := e.RetypeResult("get_register", "(ptr reg_T)"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Retype(paramNamed(e.Defn("put_register"), "reg"), "(ptr reg_T)"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Retype(v.One("(def r1 (ptr void) 0)", "r1"), "(ptr reg_T)"); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, strings.NewReplacer("void *get", "reg_T *get", "void *\nget", "reg_T *\nget", "void *reg", "reg_T *reg", "void *r1", "reg_T *r1").Replace(src))
	reg := v.One("(def reg (ptr reg_T) _)", "reg")
	if reg == nil {
		t.Fatal(v.Done())
	}
	call := v.One("(call get_register 1)", "the call")
	if call.Type != reg.Type || e.Defn("get_register").Type != e.FileDecls("get_register")[0].Type || len(e.Untyped) != 0 {
		t.Errorf("the call typed %s, reg_T * is %s; %d untyped", Lisp(call.Type), Lisp(reg.Type), len(e.Untyped))
	}
	ps, _, _, _ := funcParts(e.Defn("put_register").Type)
	if len(ps) != 2 || ps[1] != reg.Type {
		t.Errorf("put_register's type: %s", Lisp(e.Defn("put_register").Type))
	}
	// the store's type is the left side's, r1's, now reg_T *
	if st := v.One("(= r1 _)", "the store"); st.Type != reg.Type {
		t.Errorf("the store typed %s", Lisp(st.Type))
	}

	// refused: a function whose address is taken
	e, _, _ = b2c(t, `static int f(int x);
static int (*fp)(int) = f;
int f(int x) { return x; }
int main(void) { return fp(1); }
`)
	before := cview(t, e)
	_, err := e.RetypeResult("f", "long")
	unchanged(t, e, before, err, "other than as a callee")
	_, err = e.Retype(e.Defn("f"), "long")
	unchanged(t, e, before, err, "RetypeResult's")
}

// A member, and a typedef reaching every declaration that names it: the
// selections typed again, a function's type with its parameter's.
func TestRetypeMemberAndTypedef(t *testing.T) {
	src := `typedef int T;
struct s { T m; int k; };
static int f(T x);
int f(T x) { return x; }
int main(void) { struct s v; T a = 1; v.m = a; v.k = v.m + 1; return f(a) + (int)(T)a; }
`
	e, v, path := b2c(t, src)
	m := v.One("(m T)", "the member")
	if _, err := e.Retype(m, "long"); err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, strings.Replace(src, "T m;", "long m;", 1))
	longT := m.Type
	if longT == nil || longT.Kids[1].Atom != "long" {
		t.Fatalf("the member typed %s", Lisp(longT))
	}
	for _, s := range v.Find("(. v m)") {
		if s.Type != longT {
			t.Errorf("%s typed %s", Lisp(s), Lisp(s.Type))
		}
	}
	if s := v.One("(+ (. v m) 1)", "the sum"); s.Type != nil {
		t.Errorf("the sum kept a type")
	}

	e, v, path = b2c(t, src)
	var td *Node
	for _, f := range e.Graph().Forms {
		if topName(f) == "T" {
			td = f
		}
	}
	st, err := e.Retype(td, "long")
	if err != nil {
		t.Fatal(err)
	}
	holds(t, e, path, strings.Replace(src, "typedef int T;", "typedef long T;", 1))
	longT = td.Type
	a := v.One("(def a T 1)", "a")
	x := paramNamed(e.Defn("f"), "x")
	ps, _, _, _ := funcParts(e.Defn("f").Type)
	if a.Type != longT || x.Type != longT || len(ps) != 1 || ps[0] != longT || e.FileDecls("f")[0].Type != e.Defn("f").Type {
		t.Errorf("a %s, x %s, f %s (stats %s)", Lisp(a.Type), Lisp(x.Type), Lisp(e.Defn("f").Type), st)
	}
	if c := v.One("(cast T a)", "the cast"); c.Type != longT {
		t.Errorf("the cast typed %s", Lisp(c.Type))
	}
}
