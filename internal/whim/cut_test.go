package whim

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Cut on the tracked product: the core ends where the first #include is, no
// directive before it; and a directive before the first #include is
// refused.
func TestCut(t *testing.T) {
	c, err := os.ReadFile(filepath.Join("..", "..", "src", "whim-vim.c"))
	if err != nil {
		t.Skip(err)
	}
	got, err := Cut(c)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(c, []byte("\n#include "))
	if i < 0 || !bytes.HasPrefix(c, bytes.TrimRight(got, "\n")) || len(bytes.TrimRight(got, "\n")) != len(bytes.TrimRight(c[:i], "\n")) {
		t.Errorf("the core is not everything before the first #include")
	}
	if _, err := Cut([]byte("int x;\n#define Y 1\n#include <stdio.h>\n")); err == nil {
		t.Error("a directive before the first #include was not refused")
	}
}
