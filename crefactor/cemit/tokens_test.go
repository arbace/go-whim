package cemit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
)

// TestTokensWalk holds expansions.tokens (ccwalk's generated walk) to the
// reflective walk it replaced as scan's first pass: the same offsets, counts
// and first spellings for every external declaration of every parse -- the
// module's C test files, and $WHIM_C when it is set.
func TestTokensWalk(t *testing.T) {
	files, _ := filepath.Glob("../*/testdata/*.c")
	if f := os.Getenv("WHIM_C"); f != "" {
		files = append(files, f)
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src = stripComments(src)
		cfg, err := cc.NewConfig("linux", "amd64")
		if err != nil {
			t.Fatal(err)
		}
		ast, err := cc.Parse(cfg, []cc.Source{
			{Name: "<predefined>", Value: cfg.Predefined},
			{Name: "<builtin>", Value: cc.Builtin},
			{Name: file, Value: string(src)},
		})
		if err != nil {
			t.Logf("%s: %v (skipped)", file, err)
			continue
		}
		lines := lineIndex(src)
		n := 0
		for l := ast.TranslationUnit; l != nil; l = l.TranslationUnit {
			d := l.ExternalDeclaration
			if d == nil {
				continue
			}
			mk := func() *expansions {
				return &expansions{count: map[int]int{}, says: map[int]string{}, src: src, line: lines}
			}
			a, b := mk(), mk()
			a.walk(reflect.ValueOf(cc.Node(d)), nil)
			b.tokens(d)
			if !reflect.DeepEqual(a.count, b.count) || !reflect.DeepEqual(a.says, b.says) {
				t.Fatalf("%s: %v: the walks disagree", file, d.Position())
			}
			n++
		}
		if n == 0 {
			t.Fatalf("%s: no declarations", file)
		}
	}
}
