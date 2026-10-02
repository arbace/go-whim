package edit

import (
	"os"
	"regexp"
	"testing"
)

// TestMentionCountFast holds MentionCount's counter to the regexp it
// replaced, on edge cases and, with $WHIM_C, every identifier of a file.
func TestMentionCountFast(t *testing.T) {
	slow := func(text []byte, name string) int {
		return len(wordRe(name).FindAll(WithoutIncludes(text), -1))
	}
	texts := []string{
		"", "a", "aa a", "a_a a", "#include a\na", "x\n#include a\n a", "#include  a", " #include a",
		"a\n#include <a.h>\nb a ab ba _a a_ 9a a9", "#include a",
	}
	for _, s := range texts {
		for _, nm := range []string{"a", "ab", "a_", "include", "h", "9a"} {
			if got, want := MentionCount([]byte(s), nm), slow([]byte(s), nm); got != want {
				t.Errorf("%q in %q: %d, the regexp %d", nm, s, got, want)
			}
		}
	}
	f := os.Getenv("WHIM_C")
	if f == "" {
		return
	}
	src, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`).FindAll(src, -1) {
		nm := string(m)
		if seen[nm] || len(seen) > 3000 {
			continue
		}
		seen[nm] = true
		if got, want := MentionCount(src, nm), slow(src, nm); got != want {
			t.Errorf("%s: %d, the regexp %d", nm, got, want)
		}
	}
}
