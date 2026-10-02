package sweep

import (
	"os"
	"reflect"
	"testing"
)

// TestIdentSpans holds identSpans to the regexp it replaced, identRe, on edge
// cases and, with $WHIM_C, on a whole file -- the input and the product, run
// once each.
func TestIdentSpans(t *testing.T) {
	texts := [][]byte{
		[]byte(""), []byte("a"), []byte("9"), []byte("0x1F a_b"), []byte("123abc _9 ä z"),
		[]byte("p->q.r(s, \"t u\") /* v */"), []byte("_\n_a\t__b9;"),
	}
	if f := os.Getenv("WHIM_C"); f != "" {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, src)
	}
	for _, b := range texts {
		var got [][]int
		identSpans(b, func(s, e int) { got = append(got, []int{s, e}) })
		want := identRe.FindAllIndex(b, -1)
		if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
			t.Errorf("%.40q: %d spans, the regexp %d", b, len(got), len(want))
		}
		for _, w := range []string{"a", "b", "_a", "int", "struct", "main"} {
			slow := false
			for _, m := range want {
				if string(b[m[0]:m[1]]) == w {
					slow = true
					break
				}
			}
			if hasWord(b, w, nil, 0) != slow {
				t.Errorf("%.40q: hasWord %q is %v, the regexp %v", b, w, !slow, slow)
			}
		}
	}
}
