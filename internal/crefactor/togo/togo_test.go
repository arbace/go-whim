package togo

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A translation unit that is not vim, told nothing: an empty profile names
// no runtime function, allocator or growable array, and the generator still
// writes the whole file -- a pointer that only walks forward a slice.
const small = `typedef struct point { int x; int y; } point_T;
static int total;
static int sum(const char *s)
{
    int n = 0;
    while (*s)
        n += *s++ - '0';
    return n;
}
int add(point_T *p, int k)
{
    p->x += k;
    total += sum("123") + p->y;
    return total;
}
`

func TestEmptyProfile(t *testing.T) {
	dir := t.TempDir()
	c := filepath.Join(dir, "small.c")
	if err := os.WriteFile(c, []byte(small), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "small.go")
	if rc := Run([]string{c, dir, "-editor", out}, io.Discard, Profile{Header: "package main\n\n"}); rc != 0 {
		t.Fatalf("the generator refused a small C file: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type point_T = S_point", "func sum(s []byte) int32", "s = s[1:]", "func add(p *S_point, k int32) int32"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("no %q in:\n%s", want, b)
		}
	}
}
