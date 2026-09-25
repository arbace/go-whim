package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// C's `do { ... } while (0)` -- a block a break can leave -- is Go's
// `for { ...; break }`: a break in it leaves it, a break in a switch or loop
// inside it stays theirs, a continue leaves it by its label, and a body
// that ends in a return needs none.  Any other do-while keeps its test.
const onceC = `int once(int a, int b)
{
    int r = 0;
    do
    {
        int k = a * 2;
        if (a < 0)
            break;
        r += k;
        if (b == 1)
            continue;
        switch (b)
        {
        case 2:
            r += 100;
            break;
        default:
            r += 1;
        }
        for (int i = 0; i < b; i++)
        {
            if (i == 3)
                break;
            r += i;
        }
        r += 1000;
    }
    while (0);
    return r;
}
int twice(int a)
{
    do
    {
        if (a > 5)
            break;
        return a;
    }
    while (0);
    return -a;
}
int loop(int n)
{
    int t = 0;
    do
    {
        t += n;
        n--;
    }
    while (n > 0);
    return t;
}
`

// onceMain calls the three on the same arguments in either language.
const onceMainC = `#include <stdio.h>
int main(void)
{
    for (int a = -2; a < 8; a++)
        for (int b = 0; b < 6; b++)
            printf("%d %d %d\n", once(a, b), twice(a), loop(b));
    return 0;
}
`

const onceMainGo = `
func main() {
	for a := int32(-2); a < 8; a++ {
		for b := int32(0); b < 6; b++ {
			fmt.Printf("%d %d %d\n", once(a, b), twice(a), loop(b))
		}
	}
}
`

func TestDoWhileZero(t *testing.T) {
	dir := t.TempDir()
	c := filepath.Join(dir, "once.c")
	if err := os.WriteFile(c, []byte(onceC), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "once.go")
	if rc := Run([]string{c, dir, "-editor", out}, io.Discard, Profile{Header: "package main\n\nimport \"fmt\"\n\n"}); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	for _, want := range []string{
		"\tfor {\n\t\tk := a * 2\n",            // the body runs once: k declared where C does
		"\t\tif b == 1 {\n\t\t\tbreak once1\n", // continue leaves the labeled for
		"once1:\n\tfor {\n",
		"\tfor {\n\t\tif a > 5 {\n\t\t\tbreak\n\t\t}\n\t\treturn a\n\t}\n\treturn -a", // no break after a return
		"\t\tif !(n > 0) {\n\t\t\tbreak\n",                                            // a loop's test stays
	} {
		if !strings.Contains(g, want) {
			t.Errorf("no %q in:\n%s", want, g)
		}
	}
	if strings.Contains(g, "!false") || strings.Contains(g, "0 != 0") {
		t.Errorf("the constant condition is printed:\n%s", g)
	}

	// the control: the Go program prints what the C one does
	for _, tool := range []string{"gcc", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	if err := os.WriteFile(c, []byte(onceC+onceMainC), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "once")
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
		if err := os.WriteFile(filepath.Join(gdir, "main.go"), []byte(src+onceMainGo), 0o644); err != nil {
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
	if got := goRun(goProgram(g)); got != string(want) {
		t.Errorf("the Go prints\n%s\nthe C\n%s", got, want)
	}
	// and a wrong translation is seen: the continue that falls into the switch
	if goRun(strings.NewReplacer("break once1\n", "\n", "once1:\n", "").Replace(goProgram(g))) == string(want) {
		t.Error("the control: a do-while(0) whose continue does not leave it prints the same")
	}
}

// goProgram is the generated file cut to what a main package can hold: its
// functions, without the instance type an empty profile still writes.
func goProgram(g string) string {
	if i := strings.Index(g, "\n// Editor is one instance"); i >= 0 {
		g = g[:i+1]
	}
	return g
}
