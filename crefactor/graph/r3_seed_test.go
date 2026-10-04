package graph

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
)

// The seed's spelling and phase 88's headers on samples read back from
// their Lisp (R3): crefactor/xform's text tests of NullptrUsize, Attrs and
// Includes, held now to the graph's C view and to the import of the
// expected text (SameGraph).

const r3List = `#include <stddef.h>
#include <stdio.h>

struct node
{
    struct node *next;
    size_t len;
};

size_t
length(struct node *n)
{
    size_t k = 0;
    while (n != NULL)
    {
        k += (size_t)n->len;
        n = n->next;
    }
    return k;
}

void *
first(struct node *n)
{
    return n ? n->next : (void *)NULL;
}

void
show(struct node *n)
{
    printf("%s\n", n == NULL ? "NULL list" : "list");
}

int
main(void)
{
    show(first(0));
    return (int)length(0);
}
`

// r3Same holds g to want: its C view the canonical print of want, and the
// graph, read back, the import of it.
func r3Same(t *testing.T, g *Graph, want string) {
	t.Helper()
	_, _, w := importSample(t, want)
	wc, err := w.C()
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.C()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wc) {
		t.Fatalf("got:\n%s\nwant:\n%s", got, wc)
	}
	h, err := Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	if err := SameGraph(h, w); err != nil {
		t.Fatalf("not the import of the text after: %v", err)
	}
}

// A small list library's NULL and size_t are the language's nullptr and
// usize; the NULL in its message stays, and so does the one in a literal it
// was not told of when told none.
func TestR3NullptrUsize(t *testing.T) {
	want := `#include <stddef.h>
#include <stdio.h>

typedef typeof(sizeof(0)) usize;

struct node
{
    struct node *next;
    usize len;
};

usize
length(struct node *n)
{
    usize k = 0;
    while (n != nullptr)
    {
        k += (usize)n->len;
        n = n->next;
    }
    return k;
}

void *
first(struct node *n)
{
    return n ? n->next : nullptr;
}

void
show(struct node *n)
{
    printf("%s\n", n == nullptr ? "NULL list" : "list");
}

int
main(void)
{
    show(first(0));
    return (int)length(0);
}
`
	run := func(k NullptrKnobs, casts int, src string) (*Graph, *Editor, *Verbs, string) {
		_, g, e := incGraph(t, src)
		var w strings.Builder
		v := NewVerbs("language", e, &w)
		v.NullptrUsize(k, casts)
		return g, e, v, w.String()
	}
	g, e, v, rep := run(NullptrKnobs{NullLiterals: []string{`"NULL list"`}}, 1, r3List)
	if v.Err != nil {
		t.Fatal(v.Err)
	}
	if len(e.Untyped) != 0 {
		t.Errorf("left untyped: %d", len(e.Untyped))
	}
	for _, s := range []string{"4 mentions of `size_t`, ALL of them type-name positions: 1 casts and 3 declarations",
		"`NULL` -> `nullptr` at 3 sites and `size_t` -> `usize` at 4", "1 `(void *)nullptr` -> `nullptr`",
		"`usize` is a typedef on line 4"} {
		if !strings.Contains(rep, s) {
			t.Errorf("the report does not say %q:\n%s", s, rep)
		}
	}
	e.Recheck()
	if _, err := e.Collect(CollectOptions{Roots: []string{"main"}}); err != nil {
		t.Fatal(err)
	}
	r3Same(t, g, want)
	for _, c := range []struct {
		k     NullptrKnobs
		casts int
		src   string
		why   string
	}{
		{NullptrKnobs{NullLiterals: []string{}}, 0, r3List, "not the 0"},
		{NullptrKnobs{}, 2, r3List, "at least 2"},
		{NullptrKnobs{}, 0, strings.Replace(r3List, "size_t k = 0;", "size_t k = 0;\n    void *nullptr_ = 0;\n    (void)nullptr_;", 1) + "int usize;\n", "already occurs"},
		{NullptrKnobs{}, 0, strings.Replace(r3List, `"list"`, `"size_t"`, 1), "a literal contains `size_t`"},
	} {
		_, _, v, _ := run(c.k, c.casts, c.src)
		if v.Err == nil || !strings.Contains(v.Err.Error(), c.why) {
			t.Errorf("want a refusal saying %q, got %v", c.why, v.Err)
		}
	}
}

const r3Step = `#include <stdarg.h>
#include <stdio.h>

int logf_(const char *fmt, ...) __attribute__((format(printf, 1, 2)));

int
logf_(const char *fmt, ...)
{
    va_list ap;
    va_start(ap, fmt);
    int n = vprintf(fmt, ap);
    va_end(ap);
    return n;
}

struct ctx
{
    int attr;
};

int
step(int state, int ctx __attribute__((unused)), int attr)
{
    int spare __attribute__((unused)) = attr;
    switch (state)
    {
    case 0:
        state++;
        __attribute__((fallthrough));
    case 1:
        return state;
    }
    return -1;
}

int
main(void)
{
    struct ctx c = {0};
    return step(0, 0, c.attr) + logf_("%d", 1);
}
`

// Unused on a definition's parameter and a local goes, a GNU fallthrough is
// C23's, a format attribute stays -- a parameter and a member named attr
// are no attribute; an attribute of another kind refuses, and so does an
// unused on a field.
func TestR3Attrs(t *testing.T) {
	_, g, e := incGraph(t, r3Step)
	var w strings.Builder
	v := NewVerbs("attrs", e, &w)
	v.Attrs()
	if v.Err != nil {
		t.Fatalf("%v\n%s", v.Err, w.String())
	}
	for _, s := range []string{"4 `__attribute__` in the file, and every one is one of five kinds: unused 2, fallthrough 1, format 1, format_arg 0, cold 0",
		"2 `__attribute__((unused))`: 1 in the parameter list of a function DEFINITION -- 1 header lines, every one followed by `{` -- and 1 on a local's",
		"4 attributes -> 1, the same"} {
		if !strings.Contains(w.String(), s) {
			t.Errorf("the report does not say %q:\n%s", s, w.String())
		}
	}
	want := strings.Replace(strings.ReplaceAll(r3Step, " __attribute__((unused))", ""),
		"__attribute__((fallthrough));", "[[fallthrough]];", 1)
	r3Same(t, g, want)
	for _, c := range []struct{ src, why string }{
		{strings.Replace(r3Step, "((format(printf, 1, 2)))", "((noreturn))", 1), "never looked at"},
		{strings.Replace(r3Step, "    int attr;\n", "    int attr __attribute__((unused));\n", 1), "not on a function DEFINITION's parameter"},
		{strings.Replace(r3Step, "__attribute__((fallthrough));", "[[fallthrough]];", 1), "C23 attribute is in the file already"},
	} {
		_, _, e := incGraph(t, c.src)
		v := NewVerbs("attrs", e, io.Discard)
		v.Attrs()
		if v.Err == nil || !strings.Contains(v.Err.Error(), c.why) {
			t.Errorf("want a refusal saying %q, got %v", c.why, v.Err)
		}
	}
}

// The rule says which headers a file needs: <math.h> and <string.h> go,
// <stdio.h> stays; two headers that each provide what the file takes from
// both can each go alone but not together, and the fold from the bottom
// keeps the upper one.
func TestR3Spares(t *testing.T) {
	src := "#include <math.h>\n#include <stdio.h>\n#include <string.h>\n\nint\nmain(void)\n{\n    return printf(\"x\\n\");\n}\n"
	_, g, e := incGraph(t, src)
	r, err := e.Spares()
	if err != nil {
		t.Fatal(err)
	}
	var spare []string
	for _, inc := range r.Spare {
		spare = append(spare, IncludeSpec(inc))
	}
	if !slices.Equal(spare, []string{"<math.h>", "<string.h>"}) || !r.Together || len(r.Alone) != 2 || len(r.All) != 3 {
		t.Fatalf("spare %v, together %v, alone %d of %d", spare, r.Together, len(r.Alone), len(r.All))
	}
	if err := e.DeleteIncludes(r.All...); err == nil || !strings.Contains(err.Error(), "printf") {
		t.Fatalf("all three deleted: %v", err)
	}
	if err := e.DeleteIncludes(r.Spare...); err != nil {
		t.Fatal(err)
	}
	r3Same(t, g, strings.Replace(strings.Replace(src, "#include <math.h>\n", "", 1), "#include <string.h>\n", "", 1))

	// size_t is <stddef.h>'s and <stdio.h>'s: each can go alone, not both
	src = "#include <stddef.h>\n#include <stdio.h>\n\nsize_t n;\n\nint\nmain(void)\n{\n    return (int)n;\n}\n"
	_, _, e = incGraph(t, src)
	if r, err = e.Spares(); err != nil {
		t.Fatal(err)
	}
	if len(r.Alone) != 2 || r.Together || len(r.Spare) != 1 || IncludeSpec(r.Spare[0]) != "<stdio.h>" {
		t.Fatalf("alone %d, together %v, spare %d", len(r.Alone), r.Together, len(r.Spare))
	}
	if len(r.Unprovided) != 0 || len(r.Collisions) != 0 {
		t.Fatalf("unprovided %v, collisions %v", r.Unprovided, r.Collisions)
	}
}
