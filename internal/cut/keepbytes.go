package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

var badCharLine = regexp.MustCompile(`(?m)^[^\n]*\bbad_char\b[^\n]*$`)

// KeepBytes makes an invalid byte kept, with nothing able to ask otherwise.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): every `++bad` arm folded,
// however many but at least one, and `++enc`'s value kept with the chain
// after it gone (KeepThen); the text version did both by brace matching
// (history keeps it).  The last check is the text's own, on the C view: no
// line reads eap->bad_char but its declaration, its stores and get_bad_opt.
func KeepBytes(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("keepbytes", e, w)
	// readfile went at phase 1 (readfront, phase 31's move)
	v.InFunction("getargopt", func(v *graph.Verbs) {
		bad := `(== (call strncmp (cast (ptr char) (paren arg)) (cast (ptr char) (paren "bad")) (paren 3)) 0)`
		n := v.Count("(if " + bad + " _*)")
		if n == 0 {
			v.Die("++bad -- no occurrence")
			return
		}
		q := graph.NewVerbs("keepbytes", e, io.Discard)
		q.In(v.Scope(), func(q *graph.Verbs) { q.FoldNever(bad, n, "++bad") })
		if q.Err != nil {
			v.Err = q.Err
			return
		}
		v.Sayf("++bad (%d)", n)
		v.KeepThen("(== pp (addr (-> eap force_enc)))", 1, "++enc's value, the only one left to check")
	})
	if v.Failed() {
		return v.Done()
	}

	// The Python writes this as `\bbad_char\b(?!_)`, and the lookahead is
	// REDUNDANT rather than unspellable: `_` is a word character, so a word
	// boundary after "bad_char" already cannot occur inside
	// "bad_char_behavior".  Measured on the whole corpus, the two agree.
	text := v.Text()
	if edit.WordCount(bytes.ReplaceAll(text, []byte("int bad_char;"), nil), "bad_char") > 0 {
		var live []string
		for _, m := range badCharLine.FindAll(text, -1) {
			line := string(m)
			if strings.Contains(line, "int bad_char;") ||
				strings.Contains(line, "get_bad_opt") {
				continue
			}
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "eap->bad_char =") {
				continue
			}
			live = append(live, line)
		}
		if len(live) > 0 {
			return fmt.Errorf("keepbytes: eap->bad_char still read: %s", pyList(live))
		}
	}

	v.Say("an invalid byte is kept, and nothing can ask otherwise")
	return v.Done()
}
