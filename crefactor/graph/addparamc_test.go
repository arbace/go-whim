package graph

import (
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// AddParamC: a pointer to a function, which BUILD does not make, added to a
// function with a prototype and a definition, its argument at the call.
func TestAddParamC(t *testing.T) {
	src := `static void quit(int r);
static int run(int argc);
static void (*on_exit_fn)(int);
static void quit(int r) { (void)r; }
static int
run(int argc)
{
    return argc;
}
int
main(void)
{
    return run(1);
}
`
	path, _, v, _ := verbsOn(t, src)
	e := v.Editor()
	if _, err := e.AddParamC("run", 1, "void (*exit_fn)(int)", func(*Node) (string, Bindings) { return "quit", nil }); err != nil {
		t.Fatal(err)
	}
	want, err := cemit.Canonical(path, []byte(strings.NewReplacer(
		"run(int argc)", "run(int argc, void (*exit_fn)(int))",
		"run(1)", "run(1, quit)").Replace(src)))
	if err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, want)
	if _, err := e.AddParamC("run", 0, "int argc", func(*Node) (string, Bindings) { return "0", nil }); err == nil {
		t.Error("a second parameter argc is refused")
	}
}
