package graph

import (
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

func TestSetStorage(t *testing.T) {
	src := "int f(void);\nextern int x;\nint x = 1;\nint\nf(void)\n{\n    return x;\n}\nint\nmain(void)\n{\n    return f();\n}\n"
	path, _, v, _ := verbsOn(t, src)
	e := v.Editor()
	if err := e.SetStorage(e.Defn("f"), "static"); err != nil {
		t.Fatal(err)
	}
	var d *Node
	for _, x := range e.FileDecls("x") {
		if x.Is("def") {
			d = x
		}
	}
	if err := e.SetStorage(d, "static"); err != nil {
		t.Fatal(err)
	}
	want, err := cemit.Canonical(path, []byte(strings.NewReplacer("int f", "static int f", "extern int x", "static int x", "int x = 1", "static int x = 1", "int\nf", "static int\nf").Replace(src)))
	if err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, want)
	if err := e.SetStorage(e.Defn("main"), "auto"); err == nil {
		t.Error("`auto` is not a storage class SetStorage gives")
	}
}
