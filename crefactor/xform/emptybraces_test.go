package xform

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

// TestEmptyBraces holds the empty-block pattern on the windows around
// emptyBraces to the pattern on the whole text: on edge cases and, with
// $WHIM_C, that file, and that file with every statement line of a block
// taken out, which leaves thousands of blocks empty.
func TestEmptyBraces(t *testing.T) {
	texts := []string{
		"if (a)\n{\n}\n", "    if (a)\n    {\n    }\n    else\n    {\n    }\n",
		"x;\nelse if (b)\n  {\n \t}\n}\n", "{\n}\n", "else\n{\n}", "if (a)\n{\n}\nif (b)\n{\n}\n",
	}
	if f := os.Getenv("WHIM_C"); f != "" {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var cut []string
		for _, l := range strings.Split(string(src), "\n") {
			if !strings.HasSuffix(l, ";") || !strings.HasPrefix(l, "        ") {
				cut = append(cut, l)
			}
		}
		texts = append(texts, string(src), strings.Join(cut, "\n"))
	}
	for _, s := range texts {
		b := []byte(s)
		got := edit.AllSubmatchIndexAround(edit.EmptyGuardedBlock, b, emptyBraces(b), 3)
		want := edit.EmptyGuardedBlock.FindAllSubmatchIndex(b, -1)
		if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
			t.Errorf("%.40q: %d matches, the regexp %d", s, len(got), len(want))
		}
		if len(s) > 1000 {
			t.Logf("%d matches", len(want))
		}
	}
}
