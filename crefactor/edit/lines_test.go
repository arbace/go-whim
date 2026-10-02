package edit

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"testing"
)

// scopedCheck holds every windowed helper to the regexp method it stands
// for, on one pattern and one text.
func scopedCheck(t *testing.T, re *regexp.Regexp, text []byte) {
	t.Helper()
	same := func(what string, a, b any) {
		t.Helper()
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s %q on %.40q: %v, the regexp %v", what, re, text, a, b)
		}
	}
	norm := func(ms [][]int) [][]int {
		if len(ms) == 0 {
			return nil
		}
		return ms
	}
	same("AllSubmatchIndex", norm(AllSubmatchIndex(re, text)), norm(re.FindAllSubmatchIndex(text, -1)))
	same("AllIndex", norm(AllIndex(re, text)), norm(re.FindAllIndex(text, -1)))
	same("AllIndexN", norm(AllIndexN(re, text, 2)), norm(re.FindAllIndex(text, 2)))
	same("CountMatches", CountMatches(re, text), len(re.FindAll(text, -1)))
	same("FirstIndex", FirstIndex(re, text), re.FindIndex(text))
	same("FirstSubmatchIndex", FirstSubmatchIndex(re, text), re.FindSubmatchIndex(text))
	for _, repl := range []string{"", "<$0>", "${1}x"} {
		out, n := ReplaceAllCounted(re, text, []byte(repl))
		same("ReplaceAllCounted", n, len(re.FindAll(text, -1)))
		if !bytes.Equal(out, re.ReplaceAll(text, []byte(repl))) {
			t.Errorf("ReplaceAllCounted %q %q on %.40q: not ReplaceAll's text", re, repl, text)
		}
	}
}

// TestScoped holds the windowed matches (lines.go) to the regexp's own, on
// patterns of every shape the scope reads -- a word, a line, lines, a
// boundary at a window's edge, the ones it refuses -- and, with $WHIM_C, on
// that file.  internal/build's TestScopedPatterns runs every pattern the
// build's edits ran on a whole text through the same comparison.
func TestScoped(t *testing.T) {
	text := []byte("static int x;\n\nint\nf(int a)\n{\n    if (a)\n    {\n        x = 1;\n    }\n" +
		"    else\n    {\n        x = 2;\n    }\n    return x;\n}\nx\nxx = 1;\n" +
		"aa aaa\n\nfoo(bar)\nfoo (bar);\n#include <a.h>\n    foo(1);\nfoo")
	pats := []string{
		`(?m)^[ \t]*if \(a\)\n[ \t]*\{\n`,
		`(?m)^[ \t]*x = (\d);\n`,
		`\bx\b`,
		`x\b`,
		`\bx = `,
		`(?m)^x$`,
		`(?m)^foo\([^\n]*\n`,
		`(?m)[ \t]*foo\((\w+)\);?$`,
		`foo\s*\(`,
		`aa`,
		`(?m)^[ \t]*\}\n[ \t]*else\n`,
		`(?m)^    return x;\n\}\n\b`,
		`(?m)\n$`,
		`(?s)static.*return`,
		`\Astatic`,
		`foo\z`,
		`(?i)FOO`,
		`(?:aa)+ `,
		`(?m)(?:^[ \t]*x = \d;\n){1,2}`,
		`(?m)^(?:[^\n]*\n)*?    \}\n`,
		`[ \t]*\{\n(?:[ \t]+[^\n]*\n){0,3}`,
	}
	for _, p := range pats {
		scopedCheck(t, regexp.MustCompile(p), text)
	}
	f := os.Getenv("WHIM_C")
	if f == "" {
		return
	}
	src, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pats {
		scopedCheck(t, regexp.MustCompile(p), src)
	}
}

// TestScopeOf pins what the scope reads off a pattern: the literal and the
// newlines, or nothing where it must run on the whole text.
func TestScopeOf(t *testing.T) {
	for _, c := range []struct {
		re  string
		lit string
		k   int
	}{
		{`(?m)^[ \t]*if \(p_xyz\) \{\n`, "if (p_xyz) {\n", 1},
		{`(?m)^[ \t]*case[^\n]*\n[ \t]*return x;\n`, "return x;\n", 2},
		{`(?m)^(static\s+int\s*p_sb);$`, "", 0},
		{`\bfoo\b`, "foo", 0},
		{`(?:ab)+c`, "ab", 0},
		{`(?m)(?:^x = \d;\n){1,3}`, "x = ", 3},
		{`(?s)a.*bc`, "", 0},
		{`\Aabc`, "", 0},
		{`(?i)abc`, "", 0},
		{`x`, "", 0},
	} {
		s := scopeOf(regexp.MustCompile(c.re))
		if string(s.lit) != c.lit || (s.lit != nil && s.k != c.k) {
			t.Errorf("%s: %q %d, want %q %d", c.re, s.lit, s.k, c.lit, c.k)
		}
	}
}
