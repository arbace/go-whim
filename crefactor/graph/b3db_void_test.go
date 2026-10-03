package graph

import "testing"

// A `(void)` function is, in cc's type and so in the importer's, one void
// parameter.  RetypeResult on one, and a PARAM drop that empties a list,
// leave the type an import of the C view gives (B3d: they wrote no
// parameter, a type the importer never interns).
func TestVoidParamsAsImported(t *testing.T) {
	src := `static int now(void);

static int add(int a);

static int now(void)
{
    return 1;
}

static int add(int a)
{
    return a;
}

int main(void)
{
    return add(2) == now();
}
`
	path, _, e := fragOn(t, src)
	if _, err := e.RetypeResult("now", "long"); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, nil)
	_, _, e = fragOn(t, src)
	if _, err := e.DropParam("add", "a", ParamOptions{}); err == nil {
		t.Fatal("a used parameter dropped")
	}
	path, _, e = fragOn(t, `static int one(int a);

static int one(int a)
{
    return 1;
}

int main(void)
{
    return one(2);
}
`)
	if _, err := e.DropParam("one", "a", ParamOptions{}); err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, nil)
}
