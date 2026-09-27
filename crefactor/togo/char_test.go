package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A plain char is signed (gcc, x86-64), and the Go holds it as a byte: widened,
// its value is its int8's.  Every byte through four uses -- a sign test, a
// widening, a comparison with an unsigned char, a conversion to unsigned --
// must print what gcc's build prints; and, the control, the widening written
// without its int8 must not.
const charC = `int neg(char c) { return c < 0; }
int widen(char c) { return c; }
int mix(char c, unsigned char u) { return c == u; }
unsigned int tou(char c) { return c; }
`

const charMainC = `#include <stdio.h>
int main(void)
{
    for (int i = 0; i < 256; i++)
        printf("%d %d %d %u\n", neg((char)i), widen((char)i), mix((char)i, (unsigned char)i), tou((char)i));
    return 0;
}
`

const charMainGo = `
func B2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func main() {
	for i := 0; i < 256; i++ {
		fmt.Printf("%d %d %d %d\n", neg(byte(i)), widen(byte(i)), mix(byte(i), byte(i)), tou(byte(i)))
	}
}
`

func TestPlainCharIsSigned(t *testing.T) {
	for _, tool := range []string{"gcc", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	dir := t.TempDir()
	c := filepath.Join(dir, "char.c")
	if err := os.WriteFile(c, []byte(charC), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "char.go")
	if rc := Run([]string{c, dir, "-editor", out}, io.Discard, Profile{Header: "package main\n\nimport \"fmt\"\n\n"}); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	g := goProgram(string(b))
	if err := os.WriteFile(c, []byte(charC+charMainC), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "char")
	if o, err := exec.Command("gcc", "-w", "-o", exe, c).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, o)
	}
	want, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatal(err)
	}
	gdir := filepath.Join(dir, "g")
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatal(err)
	}
	goRun := func(src string) string {
		if err := os.WriteFile(filepath.Join(gdir, "main.go"), []byte(src+charMainGo), 0o644); err != nil {
			t.Fatal(err)
		}
		run := exec.Command("go", "run", "main.go")
		run.Dir = gdir
		run.Env = append(os.Environ(), "GO111MODULE=off", "GOFLAGS=")
		got, err := run.CombinedOutput()
		if err != nil {
			t.Fatalf("go run: %v\n%s\n%s", err, got, src)
		}
		return string(got)
	}
	if got := goRun(g); got != string(want) {
		t.Errorf("the Go prints\n%s\nthe C\n%s\n%s", got, want, g)
	}
	// the control: the widening without its int8, as the Go printer wrote it
	wrong := strings.ReplaceAll(g, "(int8(c))", "(c)")
	if wrong == g {
		t.Fatalf("the control changed nothing:\n%s", g)
	}
	if got := goRun(wrong); got == string(want) {
		t.Errorf("the control: an unsigned plain char prints what the C prints")
	}
}
