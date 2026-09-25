package xform

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// run applies s to src with args and fails the test on an error.
func run(t *testing.T, s Step, src string, args ...string) string {
	t.Helper()
	out, err := s([]byte(src), args, io.Discard)
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	return string(out)
}

// same fails the test when got is not want.
func same(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// compiles asks gcc whether the output is C, when there is a gcc.
func compiles(t *testing.T, out string) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "o.c")
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("gcc", "-fsyntax-only", "-std=gnu2x", p).CombinedOutput(); err != nil {
		t.Errorf("gcc refuses the output: %s\n%s", b, out)
	}
}

// refuses fails the test unless s refuses src with an error that says why.
func refuses(t *testing.T, s Step, src, why string, args ...string) {
	t.Helper()
	_, err := s([]byte(src), args, io.Discard)
	if err == nil || !strings.Contains(err.Error(), why) {
		t.Errorf("want a refusal saying %q, got %v", why, err)
	}
}
