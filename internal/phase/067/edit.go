package p067

// Whim phase 67 (formerly 139) -- the core sorts and searches typed arrays.  See GOAL.md.
//
// The four table searches call a typed copy of musl_bsearch() -- the same
// probes in the same order -- the comparators take the type they cast to, and
// :undolist's sort is an insertion sort; the sweep takes musl_qsort(),
// musl_bsearch() and sort_compare() (internal/gen/FINDINGS.md, 7).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"fmt"
	"io"

	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim67", Edit) }

// W67Search is a typed binary search over an array of T, compared by cmp: the
// vendored musl_bsearch() line for line, on a T * instead of a char * stepped
// by a width.  Exported so the check requires the identical text.  The probe
// order is musl's -- the middle of what is left, then the upper half past it
// or the lower half -- so a comparator that matches a prefix finds the entry
// it found before.
func W67Search(name, t string) string {
	return fmt.Sprintf(`    static %[2]s *
%[1]s(%[2]s *key, %[2]s *base, usize nel, int (*cmp)(%[2]s *, %[2]s *))
{
    %[2]s *tryp;
    int         sign;

    while (nel > 0)
    {
        tryp = base + nel / 2;
        sign = cmp(key, tryp);
        if (sign < 0)
        {
            nel /= 2;
        }
        else if (sign > 0)
        {
            base = tryp + 1;
            nel -= nel / 2 + 1;
        }
        else
        {
            return tryp;
        }
    }
    return nullptr;
}
`, name, t)
}

// W67SortBody is sort_strings()'s Body, inside its braces: an insertion sort
// of the pointers by strcmp() of what they point to.  Two strings that compare
// equal are equal byte for byte, so any order of them prints the same.
const W67SortBody = `    int         i;
    int         j;
    char_u      *s;

    for (i = 1; i < count; ++i)
    {
        s = files[i];
        for (j = i; j > 0 && musl_strcmp((char *)files[j - 1], (char *)s) > 0; --j)
        {
            files[j] = files[j - 1];
        }
        files[j] = s;
    }`

// Edit sorts and searches typed arrays.
//
// The core sorted one array and searched four through the vendored musl_qsort()
// and musl_bsearch(), which see an array as a void * stepped by a byte width
// and hand each element to the comparator as a const void *.  The Go
// transpilation could not follow a pointer through void * -- sort_strings()'s
// first Go signature was wrong -- and typed each by hand (internal/gen/FINDINGS.md, 7).
// Here the four searches call a typed copy of musl's search, one per element
// type, the comparators take the type they always cast to, and :undolist's one
// sort is an insertion sort; the collection takes musl_qsort(), musl_bsearch() and
// sort_compare().
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): a comparator's type changes
// while its address is a value, which RETYPE refuses, so the comparators'
// prototypes and definitions are written anew by FRAG -- each the C of the
// form it replaces with the text program's literals applied to that form
// alone -- in ONE unit with the two typed searches, which retargets every use
// of them; then the four calls are rebuilt by form (BUILD) and sort_strings()'s
// call replaced by its loop (FRAG); history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("typed", e, w)
	// the forms the text program's literals rewrote, each found once
	form := func(name string, defn bool) *graph.Node {
		var out []*graph.Node
		for _, d := range e.FileDecls(name) {
			if d.Is("defn") == defn {
				out = append(out, d)
			}
		}
		if len(out) != 1 {
			v.Die("%s: %d %s, expected 1", name, len(out), map[bool]string{true: "definitions", false: "prototypes"}[defn])
			return nil
		}
		return out[0]
	}
	// lit is the text program's Literal on the C of some forms alone
	lit := func(c, old, new string, n int, what string) string {
		if k := strings.Count(c, old); k != n {
			v.Die("%s -- occurs %d times, expected %d", what, k, n)
			return c
		}
		return strings.ReplaceAll(c, old, new)
	}
	cOf := func(fs ...*graph.Node) string {
		b, err := graph.FormsC(fs)
		if err != nil {
			v.Die("%v", err)
		}
		return string(b)
	}
	kv := []string{"cmp_keyvalue_value_n", "cmp_keyvalue_value_i", "cmp_keyvalue_value_ni"}
	var protos, defs []*graph.Node
	for _, n := range kv {
		protos = append(protos, form(n, false))
		defs = append(defs, form(n, true))
	}
	kn := form("cmp_key_name_entry", true)
	if v.Failed() {
		return v.Done()
	}
	kvCast := "    keyvalue_T *kv1 = (keyvalue_T *)a;\n    keyvalue_T *kv2 = (keyvalue_T *)b;\n"
	var fs []graph.Frag
	pc := lit(cOf(protos...), "(const void *a, const void *b);\n\nstatic int cmp_keyvalue_value_i(const void *a, const void *b);\n\nstatic int cmp_keyvalue_value_ni(const void *a, const void *b);\n", "(keyvalue_T *kv1, keyvalue_T *kv2);\n\nstatic int cmp_keyvalue_value_i(keyvalue_T *kv1, keyvalue_T *kv2);\n\nstatic int cmp_keyvalue_value_ni(keyvalue_T *kv1, keyvalue_T *kv2);\n\nstatic keyvalue_T *keyvalue_bsearch(keyvalue_T *key, keyvalue_T *base, usize nel, int (*cmp)(keyvalue_T *, keyvalue_T *));\n", 1,
		"the keyvalue_T comparators take keyvalue_T: their prototypes, and the typed search's")
	fs = append(fs, graph.Frag{At: e.SpotRun(protos[0], protos[2]), Src: pc})
	for i, d := range defs {
		c := lit(cOf(d), "(const void *a, const void *b)\n{\n"+kvCast, "(keyvalue_T *kv1, keyvalue_T *kv2)\n{\n", 1,
			"their definitions, without the casts")
		if i == 2 {
			c += "\n" + W67Search("keyvalue_bsearch", "keyvalue_T")
		}
		fs = append(fs, graph.Frag{At: e.SpotOf(d), Src: c})
	}
	c := lit(cOf(kn), "cmp_key_name_entry(const void *a, const void *b)\n", "cmp_key_name_entry(struct key_name_entry *a, struct key_name_entry *b)\n", 1,
		"the key name comparator takes a key_name_entry")
	c = lit(c, "((struct key_name_entry *)a)->name.string;", "a->name.string;", 1, "and reads it without a cast")
	c = lit(c, "((struct key_name_entry *)b)->name.string;", "b->name.string;", 1, "twice")
	fs = append(fs, graph.Frag{At: e.SpotOf(kn), Src: c + "\n" + W67Search("key_name_bsearch", "struct key_name_entry")})
	if v.Failed() {
		return v.Done()
	}
	if _, err := e.SpliceC(fs...); err != nil {
		v.Die("the comparators take their type -- %v", err)
		return v.Done()
	}
	v.Say("the keyvalue_T comparators take keyvalue_T: their prototypes, and the typed search's")
	v.Say("their definitions, without the casts")
	v.Say("the key name comparator takes a key_name_entry, and reads it without a cast")
	v.Say("keyvalue_bsearch() is musl_bsearch() on a keyvalue_T *")
	v.Say("key_name_bsearch() on a struct key_name_entry *")
	v.Rewrite("(cast (ptr keyvalue_T) (call musl_bsearch (addr target) (addr ?t) (paren ?n) (sizeof (index ?t 0)) ?c))",
		"(call keyvalue_bsearch (addr target) ?t ?n ?c)", 3, "three of the four searches call keyvalue_bsearch()")
	v.Rewrite("(cast (ptr (struct key_name_entry)) (call musl_bsearch (addr target) (addr ?t) (paren ?n) (sizeof (index ?t 0)) ?c))",
		"(call key_name_bsearch (addr target) ?t ?n ?c)", 1, "and one key_name_bsearch()")
	v.InFunction("sort_strings", func(v *graph.Verbs) {
		v.ReplaceC("(call musl_qsort (cast (ptr void) files) (cast usize count) (sizeof-type (ptr char_u)) sort_compare)",
			W67SortBody+"\n", 1, "sort_strings() sorts the pointers itself")
	})
	return v.Done()
}
