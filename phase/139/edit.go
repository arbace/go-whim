package p139

// Whim phase 139 -- the core sorts and searches typed arrays.  See GOAL.md.
//
// The four table searches call a typed copy of musl_bsearch() -- the same
// probes in the same order -- the comparators take the type they cast to, and
// :undolist's sort is an insertion sort; the sweep takes musl_qsort(),
// musl_bsearch() and sort_compare() (tx/FINDINGS.md, 7).
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"fmt"
	"io"
	"regexp"

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

var w139Site = regexp.MustCompile(`\((keyvalue_T|struct key_name_entry) \*\)musl_bsearch\(&target, &(\w+),  \((sizeof\(\w+\) / sizeof\(\(\w+\)\[0\]\))\) , sizeof\(\w+\[0\]\), (\w+)\)`)

// Whim139 sorts and searches typed arrays.
//
// The core sorted one array and searched four through the vendored musl_qsort()
// and musl_bsearch(), which see an array as a void * stepped by a byte width
// and hand each element to the comparator as a const void *.  The Go
// transpilation could not follow a pointer through void * -- sort_strings()'s
// first Go signature was wrong -- and typed each by hand (tx/FINDINGS.md, 7).
// Here the four searches call a typed copy of musl's search, one per element
// type, the comparators take the type they always cast to, and :undolist's one
// sort is an insertion sort; the sweep takes musl_qsort(), musl_bsearch() and
// sort_compare().
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "typed", W: w}
	var err error
	kvCast := "    keyvalue_T *kv1 = (keyvalue_T *)a;\n    keyvalue_T *kv2 = (keyvalue_T *)b;\n    "
	steps := []struct {
		Old, New, What string
		n              int
	}{
		{"(const void *a, const void *b);\nstatic int cmp_keyvalue_value_i(const void *a, const void *b);\nstatic int cmp_keyvalue_value_ni(const void *a, const void *b);\n",
			"(keyvalue_T *kv1, keyvalue_T *kv2);\nstatic int cmp_keyvalue_value_i(keyvalue_T *kv1, keyvalue_T *kv2);\nstatic int cmp_keyvalue_value_ni(keyvalue_T *kv1, keyvalue_T *kv2);\nstatic keyvalue_T *keyvalue_bsearch(keyvalue_T *key, keyvalue_T *base, usize nel, int (*cmp)(keyvalue_T *, keyvalue_T *));\n",
			"the keyvalue_T comparators take keyvalue_T: their prototypes, and the typed search's", 1},
		{"(const void *a, const void *b)\n{\n" + kvCast, "(keyvalue_T *kv1, keyvalue_T *kv2)\n{\n", "their definitions, without the casts", 3},
		{"cmp_key_name_entry(const void *a, const void *b)\n", "cmp_key_name_entry(struct key_name_entry *a, struct key_name_entry *b)\n", "the key name comparator takes a key_name_entry", 1},
		{"((struct key_name_entry *)a)->name.string;", "a->name.string;", "and reads it without a cast", 1},
		{"((struct key_name_entry *)b)->name.string;", "b->name.string;", "twice", 1},
	}
	for _, s := range steps {
		if text, err = p.Literal(text, s.Old, s.New, s.What, s.n); err != nil {
			return nil, err
		}
	}
	// the typed searches, each after the comparators of its type
	anchor := func(after, what string) error {
		re := regexp.MustCompile(`(?s)\n` + regexp.QuoteMeta(after) + `.*?\n\}\n`)
		m := re.FindIndex(text)
		if m == nil || len(re.FindAllIndex(text, -1)) != 1 {
			return p.Die("%s: no single definition to follow", after)
		}
		fn := "keyvalue_bsearch"
		t := "keyvalue_T"
		if after == "cmp_key_name_entry(" {
			fn, t = "key_name_bsearch", "struct key_name_entry"
		}
		ins := "\n" + W139Search(fn, t)
		text = append(append(append([]byte{}, text[:m[1]]...), ins...), text[m[1]:]...)
		p.Say(what)
		return nil
	}
	if err := anchor("cmp_keyvalue_value_ni(", "keyvalue_bsearch() is musl_bsearch() on a keyvalue_T *"); err != nil {
		return nil, err
	}
	if err := anchor("cmp_key_name_entry(", "key_name_bsearch() on a struct key_name_entry *"); err != nil {
		return nil, err
	}
	n := 0
	text = w139Site.ReplaceAllFunc(text, func(m []byte) []byte {
		s := w139Site.FindSubmatch(m)
		fn := "keyvalue_bsearch"
		if string(s[1]) != "keyvalue_T" {
			fn = "key_name_bsearch"
		}
		n++
		return []byte(fmt.Sprintf("%s(&target, %s, %s, %s)", fn, s[2], s[3], s[4]))
	})
	if n != 4 {
		return nil, p.Die("%d searches through musl_bsearch(), and this phase was written against 4", n)
	}
	p.Say("the four searches call them")
	if text, err = p.Literal(text, "    musl_qsort((void *)files, (usize)count, sizeof(char_u *), sort_compare);\n", W139SortBody+"\n",
		"sort_strings() sorts the pointers itself", 1); err != nil {
		return nil, err
	}
	return text, nil
}
