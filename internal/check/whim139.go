package check

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim139", Whim139) }

// w139Musl is the input's musl_bsearch() body; the typed searches are it with
// the pointer steps typed, and that correspondence is checked line by line.
const w139Musl = `{
    void *tryp;
    int sign;

    while (nel > 0)
    {
        tryp = (char *)base + width * (nel / 2);
        sign = cmp(key, tryp);
        if (sign < 0)
        {
            nel /= 2;
        }
        else if (sign > 0)
        {
            base = (char *)tryp + width;
            nel -= nel / 2 + 1;
        }
        else
        {
            return tryp;
        }
    }
    return nullptr;
}`

// Whim139 is phase 139's check: the core sorts and searches typed arrays.
//
//  1. THE SEARCH IS MUSL'S: the input's musl_bsearch() is w139Musl, and
//     rewriting its two byte steps as element steps -- `(char *)base + width *
//     (nel / 2)` is `base + nel / 2`, `(char *)tryp + width` is `tryp + 1` --
//     gives each typed search's body.  Same probes, same order, same entry.
//  2. THE CUT: both typed searches are edit.W139Search exactly; musl_qsort,
//     musl_bsearch and sort_compare have no mention; each of the four searches
//     calls a typed one with the table and comparator it had; sort_strings() is
//     edit.W139SortBody.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES, one per search and one for the sort: :hi with attributes, a
//     colour name, a key name in a mapping, a character class in a pattern,
//     and :undolist over two branches.  Each writes the same bytes on both
//     binaries, and each CONTROL moves.
func Whim139(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim139", "typed")
	if err != nil {
		return err
	}
	r := c.r
	o, cl, found, _ := cutil.Body([]byte(c.old), "musl_bsearch")
	if !found || c.old[o:cl+1] != w139Musl {
		r.bad("the input's musl_bsearch() is not the search this phase copies")
	}
	typed := strings.NewReplacer(
		"    void *tryp;\n    int sign;", "    T *tryp;\n    int         sign;",
		"(char *)base + width * (nel / 2)", "base + nel / 2",
		"(char *)tryp + width", "tryp + 1").Replace(w139Musl)
	for _, s := range [][2]string{{"keyvalue_bsearch", "keyvalue_T"}, {"key_name_bsearch", "struct key_name_entry"}} {
		want := edit.W139Search(s[0], s[1])
		body := want[strings.Index(want, "\n{\n")+1 : len(want)-1]
		if strings.ReplaceAll(body, s[1]+" *tryp;", "T *tryp;") != typed {
			r.bad("%s's body is not musl_bsearch()'s with its steps typed", s[0])
		}
		if strings.Count(c.new, want) != 1 {
			r.bad("%s is not defined once, as this phase writes it", s[0])
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("keyvalue_bsearch() and key_name_bsearch() are the input's musl_bsearch() with its byte steps made element steps: the same probes in the same order")

	for _, n := range []string{"musl_qsort", "musl_bsearch", "sort_compare"} {
		if k := word(c.new, n); k != 0 {
			r.bad("%s has %d mentions left", n, k)
		}
	}
	for _, s := range []string{
		"keyvalue_bsearch(&target, highlight_tab, sizeof(highlight_tab) / sizeof((highlight_tab)[0]), cmp_keyvalue_value_ni);",
		"keyvalue_bsearch(&target, color_name_tab, sizeof(color_name_tab) / sizeof((color_name_tab)[0]), cmp_keyvalue_value_i);",
		"key_name_bsearch(&target, key_names_table, sizeof(key_names_table) / sizeof((key_names_table)[0]), cmp_key_name_entry);",
		"keyvalue_bsearch(&target, char_class_tab, sizeof(char_class_tab) / sizeof((char_class_tab)[0]), cmp_keyvalue_value_n);",
	} {
		if strings.Count(c.new, s) != 1 {
			r.bad("the search %q is not there once", s)
		}
		tab := s[strings.Index(s, ", ")+2:]
		tab = tab[:strings.Index(tab, ",")]
		cmp := s[strings.LastIndex(s, ", ")+2 : len(s)-2]
		if !strings.Contains(c.old, "musl_bsearch(&target, &"+tab+", ") || !strings.Contains(c.old, "sizeof("+tab+"[0]), "+cmp+");") {
			r.bad("the input did not search %s with %s", tab, cmp)
		}
	}
	so, sc, sf, _ := cutil.Body([]byte(c.new), "sort_strings")
	if !sf || c.new[so:sc+1] != "{\n"+edit.W139SortBody+"\n}" {
		r.bad("sort_strings()'s body is not the one this phase writes")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("the four searches call a typed search with the table and comparator they had; sort_strings() sorts in place; musl_qsort, musl_bsearch and sort_compare are gone")
	if err := c.gate(true); err != nil {
		return err
	}

	ob, nb := c.bins()
	seed := []byte("iabc 123\x1b")
	probes := []struct{ what, keys, ctl string }{
		{"highlight attributes", ":hi Search cterm=bold,underline\r:hi Search\r", ":hi Search cterm=reverse\r:hi Search\r"},
		{"a colour name", ":hi Search ctermfg=Red\r:hi Search\r", ":hi Search ctermfg=Blue\r:hi Search\r"},
		{"a key name", ":map <F5> x\r:map\r", ":map <F6> x\r:map\r"},
		{"a character class", "gg0/[[:digit:]]\r", "gg0/[[:space:]]\r"},
		{"the undo list", "a one\x1ba two\x1bua three\x1b:undolist\r", "a one\x1ba two\x1b:undolist\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr.keys), []byte(":q!\r")}
		s1, _, e1 := stream(ob, keys, nil)
		s2, _, e2 := stream(nb, keys, nil)
		s3, _, e3 := stream(nb, [][]byte{seed, []byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.bad("%s is written differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: highlight attributes, a colour name, a key name, a character class and :undolist over two branches write the same bytes on both binaries; each CONTROL moves")
	return nil
}
