package build

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

// TestScopedPatterns holds crefactor/edit's windowed matchers to the regexp
// on the input and the product, with every pattern of
// testdata/edit-patterns.md: the matches, the first match and the rewritten
// text the same either way.  A file not on disk is skipped (slim-vim.c is
// fetched, not tracked); so is the whole test under -short.
func TestScopedPatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	md, err := os.ReadFile("testdata/edit-patterns.md")
	if err != nil {
		t.Fatal(err)
	}
	_, fence, _ := strings.Cut(string(md), "```\n")
	fence, _, _ = strings.Cut(fence, "```\n")
	var pats []*regexp.Regexp
	for _, q := range strings.Split(strings.TrimSuffix(fence, "\n"), "\n") {
		p, err := strconv.Unquote(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		pats = append(pats, regexp.MustCompile(p))
	}
	if len(pats) < 800 {
		t.Fatalf("only %d patterns in the fence", len(pats))
	}
	for _, f := range []string{"../../src/slim-vim.c", "../../src/whim-vim.c"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Logf("%s: %v -- skipped", f, err)
			continue
		}
		var wg sync.WaitGroup
		ch := make(chan *regexp.Regexp)
		for range runtime.GOMAXPROCS(0) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for re := range ch {
					got, want := edit.AllSubmatchIndex(re, src), re.FindAllSubmatchIndex(src, -1)
					if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
						t.Errorf("%s: %q: %d matches, the regexp %d", f, re, len(got), len(want))
						continue
					}
					if g, w := edit.FirstIndex(re, src), re.FindIndex(src); !reflect.DeepEqual(g, w) {
						t.Errorf("%s: %q: first %v, the regexp %v", f, re, g, w)
					}
					if len(want) > 0 {
						out, n := edit.ReplaceAllCounted(re, src, []byte("<$1>"))
						if n != len(want) || !bytes.Equal(out, re.ReplaceAll(src, []byte("<$1>"))) {
							t.Errorf("%s: %q: not ReplaceAll's text", f, re)
						}
					}
				}
			}()
		}
		for _, re := range pats {
			ch <- re
		}
		close(ch)
		wg.Wait()
		t.Logf("%s: %d patterns", f, len(pats))
	}
}
