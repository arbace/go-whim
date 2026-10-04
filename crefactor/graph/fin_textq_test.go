package graph

import (
	"regexp"
	"strings"
	"testing"
)

const finTextqSrc = `#include <stddef.h>
typedef struct mf { int mfp; } mf_T;
static int ml_mfp;
static const char *msg = "no mfp here";
static int other;
    static int
f(mf_T *mfp)
{
    return mfp->mfp + ml_mfp + (int)sizeof(mf_T);
}
    static int
g(void)
{
    return other + 'm' + (int)u8"dp" + (int)offsetof(mf_T, mfp);
}
int
main(void)
{
    return f(0) + g();
}
`

// TestFinFormsWith: the forms a word's mention can be in are found by
// atom -- every form whose C view says the word is among them, and none
// that does not hold it at all; an identifier holds only itself (`mfp`
// not `ml_mfp`), a literal or a macro's text what it contains.  FormsWhere,
// and Quoted: the literals, and the verbatim texts as the C view prints them.
func TestFinFormsWith(t *testing.T) {
	e, _, _ := b2c(t, finTextqSrc)
	forms := e.Graph().Forms
	words := []string{"mfp", "ml_mfp", "sizeof", "dp", "other", "nowhere"}
	got := e.FormsWith(words...)
	for _, w := range words {
		re := regexp.MustCompile(`\b` + w + `\b`)
		in := map[*Node]bool{}
		for _, f := range got[w] {
			in[f] = true
		}
		for _, f := range forms {
			c, err := FormsC([]*Node{f})
			if err != nil {
				t.Fatal(err)
			}
			if re.Match(c) && !in[f] {
				t.Errorf("%s: a form saying it is not found:\n%s", w, c)
			}
			if in[f] && !strings.Contains(string(c), w) {
				t.Errorf("%s: a form not holding it is found:\n%s", w, c)
			}
		}
	}
	names := func(fs []*Node) string {
		var out []string
		for _, f := range fs {
			out = append(out, topName(f))
		}
		return strings.Join(out, " ")
	}
	for w, want := range map[string]string{
		"mfp":     "mf_T msg f g", // the member, the string, the parameter, a macro's text
		"ml_mfp":  "ml_mfp f",
		"dp":      "g",
		"nowhere": "",
	} {
		if s := names(got[w]); s != want {
			t.Errorf("%s: found in %q, want %q", w, s, want)
		}
	}
	if s := names(e.FormsWhere(func(a string) bool { return strings.HasPrefix(a, "ml_") })); s != "ml_mfp f" {
		t.Errorf("FormsWhere: %q", s)
	}
	if s := strings.Join(e.Quoted(), " "); s != `<stddef.h> "no mfp here" 'm' u8"dp" offsetof(mf_T, mfp)` {
		t.Errorf("Quoted: %s", s)
	}
}
