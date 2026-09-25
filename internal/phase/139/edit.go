package p139

// Whim phase 139 -- the core sorts and searches typed arrays.  See GOAL.md.
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

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim139", Edit) }

// W139Search is a typed binary search over an array of T, compared by cmp: the
// vendored musl_bsearch() line for line, on a T * instead of a char * stepped
// by a width.  Exported so the check requires the identical text.  The probe
// order is musl's -- the middle of what is left, then the upper half past it
// or the lower half -- so a comparator that matches a prefix finds the entry
// it found before.
func W139Search(name, t string) string {
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

// W139SortBody is sort_strings()'s Body, inside its braces: an insertion sort
// of the pointers by strcmp() of what they point to.  Two strings that compare
// equal are equal byte for byte, so any order of them prints the same.
const W139SortBody = `    int         i;
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

// w139Site is a search through musl_bsearch(), after the cast of its result.
const w139Site = `musl_bsearch\(&target, &(\w+), \((sizeof\(\w+\) / sizeof\(\(\w+\)\[0\]\))\), sizeof\(\w+\[0\]\), (\w+)\)`

// Whim139 sorts and searches typed arrays.
//
// The core sorted one array and searched four through the vendored musl_qsort()
// and musl_bsearch(), which see an array as a void * stepped by a byte width
// and hand each element to the comparator as a const void *.  The Go
// transpilation could not follow a pointer through void * -- sort_strings()'s
// first Go signature was wrong -- and typed each by hand (internal/gen/FINDINGS.md, 7).
// Here the four searches call a typed copy of musl's search, one per element
// type, the comparators take the type they always cast to, and :undolist's one
// sort is an insertion sort; the sweep takes musl_qsort(), musl_bsearch() and
// sort_compare().
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("typed", text, w)
	kvCast := "    keyvalue_T *kv1 = (keyvalue_T *)a;\n    keyvalue_T *kv2 = (keyvalue_T *)b;\n"
	e.Literal("(const void *a, const void *b);\n\nstatic int cmp_keyvalue_value_i(const void *a, const void *b);\n\nstatic int cmp_keyvalue_value_ni(const void *a, const void *b);\n", "(keyvalue_T *kv1, keyvalue_T *kv2);\n\nstatic int cmp_keyvalue_value_i(keyvalue_T *kv1, keyvalue_T *kv2);\n\nstatic int cmp_keyvalue_value_ni(keyvalue_T *kv1, keyvalue_T *kv2);\n\nstatic keyvalue_T *keyvalue_bsearch(keyvalue_T *key, keyvalue_T *base, usize nel, int (*cmp)(keyvalue_T *, keyvalue_T *));\n", 1,
		"the keyvalue_T comparators take keyvalue_T: their prototypes, and the typed search's")
	e.Literal("(const void *a, const void *b)\n{\n"+kvCast, "(keyvalue_T *kv1, keyvalue_T *kv2)\n{\n", 3,
		"their definitions, without the casts")
	e.Literal("cmp_key_name_entry(const void *a, const void *b)\n", "cmp_key_name_entry(struct key_name_entry *a, struct key_name_entry *b)\n", 1,
		"the key name comparator takes a key_name_entry")
	e.Literal("((struct key_name_entry *)a)->name.string;", "a->name.string;", 1,
		"and reads it without a cast")
	e.Literal("((struct key_name_entry *)b)->name.string;", "b->name.string;", 1,
		"twice")
	// the typed searches, each after the comparators of its type
	e.Sub(`(?s)\ncmp_keyvalue_value_ni\(.*?\n\}\n`, "${0}\n"+W139Search("keyvalue_bsearch", "keyvalue_T"), 1,
		"keyvalue_bsearch() is musl_bsearch() on a keyvalue_T *")
	e.Sub(`(?s)\ncmp_key_name_entry\(.*?\n\}\n`, "${0}\n"+W139Search("key_name_bsearch", "struct key_name_entry"), 1,
		"key_name_bsearch() on a struct key_name_entry *")
	e.Sub(`\(keyvalue_T \*\)`+w139Site, "keyvalue_bsearch(&target, ${1}, ${2}, ${3})", 3, "three of the four searches call keyvalue_bsearch()")
	e.Sub(`\(struct key_name_entry \*\)`+w139Site, "key_name_bsearch(&target, ${1}, ${2}, ${3})", 1, "and one key_name_bsearch()")
	e.Literal("    musl_qsort((void *)files, (usize)count, sizeof(char_u *), sort_compare);\n", W139SortBody+"\n", 1,
		"sort_strings() sorts the pointers itself")
	return e.Done()
}
