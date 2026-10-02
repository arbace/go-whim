package togo

import (
	"strings"
	"testing"
)

// tidyOne is scmTidy on one function's text, with mem and fr the
// function's own and the names given reading memory.
func tidyOne(t *testing.T, text string, void bool, memNames ...string) string {
	t.Helper()
	names := map[string]bool{}
	for _, n := range memNames {
		names[n] = true
	}
	out, err := scmTidy(text, names, void, &scmTidyStats{})
	if err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
	return out
}

// squash is text with its runs of white space one space: a comparison of
// forms, not of layout.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestScmTidyRead(t *testing.T) {
	for _, src := range []string{
		`(define (f ed) (g "a \"quoted\" ) ( [ string" #\x28 #\space '() -1 +2 #t))`,
		`(define (f ed) (let ([x (ch #\x5d)]) [x]))`,
	} {
		fs, err := scmRead(src)
		if err != nil || len(fs) != 1 {
			t.Fatalf("%q: %v", src, err)
		}
		if got := fs[0].flat(); got != src {
			t.Errorf("read back\n%s\nas\n%s", src, got)
		}
	}
	for _, bad := range []string{`(f (g)`, `(f))`, `(f [g)]`, `(f ; a comment` + "\n)"} {
		if _, err := scmRead(bad); err == nil {
			t.Errorf("%q read", bad)
		}
	}
}

func TestScmTidyLayout(t *testing.T) {
	long := `(define (f ed p) (if (fx>? p 0) (g ed aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff gggggggggg hhhhhhhhhh) (let loop1 ([p p]) (loop1 (fx+ p 1)))))`
	got := tidyOne(t, long, false)
	for _, l := range strings.Split(got, "\n") {
		if len(l) > scmWidth {
			t.Errorf("a line of %d columns:\n%s", len(l), got)
		}
	}
	// the named let on a line of its own, its body under it
	if !strings.Contains(got, "\n      (let loop1 ([p p])\n        (loop1 (fx+ p 1)))") {
		t.Errorf("the named let not laid out as a body:\n%s", got)
	}
	if squash(got) != squash(long) {
		t.Errorf("the forms moved:\n%s", got)
	}
}
