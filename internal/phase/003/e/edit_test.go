package p003e

import (
	"bytes"
	"os"
	"regexp"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

// TestRowsSame holds the rows written `[^}\n]*`, replaced on edit's windows,
// to the rows written `[^}]*` and replaced by the regexp on the whole text:
// the same count and the same bytes, on the input, the product, the seed
// and the boundary phase 3 (which runs this program) is handed, when a
// build has left them.
func TestRowsSame(t *testing.T) {
	texts := map[string][]byte{
		"small": []byte("    {'(', nv_brace, 0, (-1)},\n    {')', nv_brace, 0,\n FORWARD},\n" +
			"    {'{', nv_findpar, 0, (-1)},\n{'}', nv_findpar, 0, FORWARD},"),
	}
	for _, f := range []string{"../../../src/slim-vim.c", "../../../src/whim-vim.c",
		"../../../.cache/boundaries/q000.c", "../../../.cache/boundaries/q002.c"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Logf("%s: %v -- skipped", f, err)
			continue
		}
		texts[f] = src
	}
	for f, text := range texts {
		seen := 0
		for _, m := range rows {
			old := regexp.MustCompile(rowPattern(m.key, m.handler, `[^}]*`))
			repl := []byte("${1}nv_error${2}")
			want, wantN := old.ReplaceAll(text, repl), len(old.FindAllIndex(text, -1))
			got, gotN := edit.ReplaceAllCounted(regexp.MustCompile(rowPattern(m.key, m.handler, `[^}\n]*`)), text, repl)
			if gotN != wantN || !bytes.Equal(got, want) {
				t.Errorf("%s: %s: %d replaced, the whole-text regexp %d (same bytes: %v)",
					f, m.What, gotN, wantN, bytes.Equal(got, want))
			}
			seen += wantN
		}
		t.Logf("%s: %d rows", f, seen)
	}
}
