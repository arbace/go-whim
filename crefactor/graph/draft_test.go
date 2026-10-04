package graph

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// diffLines' hunks, applied to a, give b; random edits included.
func TestDiffLines(t *testing.T) {
	apply := func(a, b []string, hs []lineHunk) string {
		var out []string
		k := 0
		for _, h := range hs {
			out = append(out, a[k:h.OA]...)
			out = append(out, b[h.NA:h.NB]...)
			k = h.OB
		}
		return strings.Join(append(out, a[k:]...), "")
	}
	cases := [][2]string{
		{"", ""}, {"a\n", ""}, {"", "a\n"}, {"a\nb\nc\n", "a\nc\n"},
		{"a\nb\nc\n", "a\nx\nb\nc\ny\n"}, {"}\n}\n}\n", "}\nx\n}\n"}, {"a\nb", "a\nc"},
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		var a, b []string
		for j := 0; j < 30; j++ {
			l := string(rune('a'+r.Intn(4))) + "\n"
			a = append(a, l)
			switch r.Intn(5) {
			case 0:
			case 1:
				b = append(b, l, string(rune('a'+r.Intn(4)))+"\n")
			default:
				b = append(b, l)
			}
		}
		cases = append(cases, [2]string{strings.Join(a, ""), strings.Join(b, "")})
	}
	for _, c := range cases {
		a, b := splitLines(c[0]), splitLines(c[1])
		hs := diffLines(a, b)
		if got := apply(a, b, hs); got != c[1] {
			t.Fatalf("%q -> %q: the hunks %v give %q", c[0], c[1], hs, got)
		}
		for i := 1; i < len(hs); i++ {
			if hs[i].OA < hs[i-1].OB {
				t.Fatalf("hunks out of order: %v", hs)
			}
		}
	}
}

const draftSample = `#include <stddef.h>
enum { FALSE, TRUE };
typedef struct blk blk_T;
typedef long num_T;
struct blk { blk_T *next; num_T key; int count; char flags; };
struct pair { struct blk *a; int n; };
static int total = 0;
static int get(blk_T *bp, num_T nr, int pages);
static int put(blk_T *bp, int dirty);
static int
get(blk_T *bp, num_T nr, int pages)
{
    if (bp->key != nr)
    {
        return FALSE;
    }
    total += pages;
    return put(bp, FALSE);
}
static int
put(blk_T *bp, int dirty)
{
    bp->flags = 0;
    if (dirty)
    {
        bp->count++;
        if (total > 3)
        {
            return get(bp, bp->key, 1);
        }
    }
    return bp->count;
}
int
user(struct pair *p)
{
    int k = p->n;
    if (k > 0)
    {
        k += p->a->count;
        p->a->flags = 1;
    }
    return k + put(p->a, TRUE);
}
`

// draftCase runs edit on the sample's draft and holds the result to the
// text it wrote, printed canonically, and to the import of its C view.
func draftCase(t *testing.T, edit func(string) string) (*Editor, DraftStats) {
	t.Helper()
	path, _, e := fragOn(t, draftSample)
	d, err := e.Draft()
	if err != nil {
		t.Fatal(err)
	}
	text := edit(d.Text())
	want, err := cemit.Canonical(path, []byte(text))
	if err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
	st, err := d.Commit(text)
	if err != nil {
		t.Fatal(err)
	}
	asImported(t, path, e, want)
	return e, st
}

func replaceOnce(t *testing.T, s, old, new string) string {
	t.Helper()
	if strings.Count(s, old) != 1 {
		t.Fatalf("%q occurs %d times", old, strings.Count(s, old))
	}
	return strings.Replace(s, old, new, 1)
}

// A statement in a nested block: the one item is written anew, the rest
// keeps its ids.
func TestDraftNested(t *testing.T) {
	var keep *Node
	e, st := draftCase(t, func(s string) string {
		return replaceOnce(t, s, "            return get(bp, bp->key, 1);\n", "            return get(bp, bp->key, 2);\n")
	})
	keep = e.Defn("user")
	if st.Replaced != 1 || st.Items != 1 || keep == nil || keep.ID == 0 {
		t.Fatalf("%v", st)
	}
	for _, a := range e.Log {
		for _, id := range a.Gone {
			if id == keep.ID {
				t.Fatal("user() was superseded")
			}
		}
	}
}

// A struct written anew with a member fewer, a member's type changed and
// one more: the uses outside the runs carried to the new members; a
// function's head, its prototype and a call elsewhere; deletions; an
// insertion at the end of a block and of the file.
func TestDraftStruct(t *testing.T) {
	_, st := draftCase(t, func(s string) string {
		s = replaceOnce(t, s, "struct blk\n{\n    blk_T *next;\n    num_T key;\n    int count;\n    char flags;\n};\n",
			"struct blk\n{\n    blk_T *next;\n    int count;\n    char flags;\n    blk_T *root;\n};\n")
		s = replaceOnce(t, s, "typedef long num_T;\n\n", "")
		s = replaceOnce(t, s, "static int get(blk_T *bp, num_T nr, int pages);", "static int get(blk_T *bp, int pages);")
		s = replaceOnce(t, s, "get(blk_T *bp, num_T nr, int pages)\n{\n    if (bp->key != nr)\n    {\n        return FALSE;\n    }\n",
			"get(blk_T *bp, int pages)\n{\n")
		s = replaceOnce(t, s, "return get(bp, bp->key, 1);", "return get(bp->root, 1);")
		s = replaceOnce(t, s, "    return bp->count;\n}\n", "    bp->root = bp;\n    return bp->count;\n}\n")
		return s + "\nint\nlast(void)\n{\n    return total;\n}\n"
	})
	if st.Carried == 0 || st.Deleted != 1 || st.Inserted != 2 || st.Replaced == 0 {
		t.Fatalf("%v", st)
	}
}

// Two functions that call each other, both written anew: each fragment
// refers to the other's nodes.
func TestDraftMutual(t *testing.T) {
	draftCase(t, func(s string) string {
		s = replaceOnce(t, s, "    total += pages;\n", "    total += pages + 1;\n")
		s = replaceOnce(t, s, "static int\nput(blk_T *bp, int dirty)\n", "static int\nput(blk_T *bp, int dirty)\n")
		s = replaceOnce(t, s, "    bp->flags = 0;\n", "    bp->flags = 2;\n")
		s = replaceOnce(t, s, "static int get(blk_T *bp, num_T nr, int pages);\n", "static int get(blk_T *bp, num_T nr, int pages);\nstatic int other;\n")
		return replaceOnce(t, s, "        return FALSE;\n", "        return put(bp, TRUE);\n")
	})
}

// A struct's definition deleted while a typedef still names its tag: the
// tag resolves to an external one, as an import resolves it.
func TestDraftTagLeft(t *testing.T) {
	draftCase(t, func(s string) string {
		s = replaceOnce(t, s, "struct pair\n{\n    struct blk *a;\n    int n;\n};\n", "typedef struct pair pair_T;\n")
		return replaceOnce(t, s, "int\nuser(struct pair *p)\n{\n    int k = p->n;\n    if (k > 0)\n    {\n        k += p->a->count;\n        p->a->flags = 1;\n    }\n    return k + put(p->a, TRUE);\n}\n",
			"int\nuser(pair_T *p)\n{\n    return p != nullptr;\n}\n")
	})
}

// Nothing changed is nothing made; a member still named by a form the text
// did not touch, with nothing of its name in the new struct, is refused.
func TestDraftRefusals(t *testing.T) {
	_, _, e := fragOn(t, draftSample)
	d, err := e.Draft()
	if err != nil {
		t.Fatal(err)
	}
	if st, err := d.Commit(d.Text()); err != nil || st.Hunks != 0 || len(e.Log) != 0 {
		t.Fatalf("%v %v %d", st, err, len(e.Log))
	}
	s := replaceOnce(t, d.Text(), "    char flags;\n", "")
	s = replaceOnce(t, s, "    bp->flags = 0;\n", "")
	if _, err := d.Commit(s); err == nil || !strings.Contains(err.Error(), "still named") {
		t.Fatalf("%v, want a refusal: user() still names flags", err)
	}
}
