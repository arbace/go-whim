package togo

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A continue that must run something before the next turn -- a do-while's
// condition, a for's increment of two expressions -- with a declaration after
// it, a break of the loop's own, and a switch whose break is the switch's.
// Written as a goto to the loop's end, the continue jumps over `int k`, and
// Go refuses the program; written as loopOnce writes it, it compiles and
// prints what the C prints.
const loopContC = `int dw(int n)
{
    int t = 0;
    do
    {
        n--;
        if (n % 3 == 0)
            continue;
        int k = n * 2;
        switch (k % 4)
        {
        case 0:
            t += 1;
            break;
        default:
            t += k;
        }
        if (t > 50)
            break;
    }
    while (n > 0);
    return t;
}

int fc(int n)
{
    int t = 0;
    int i;
    int j;
    for (i = 0, j = n; i < n; i++, j--)
    {
        if (i % 2)
            continue;
        int k = i + j;
        if (k > 12 && i > 4)
            break;
        t += k;
    }
    return t;
}
`

const loopContMainC = `#include <stdio.h>
int main(void)
{
    for (int n = 0; n < 40; n++)
        printf("%d %d\n", dw(n), fc(n));
    return 0;
}
`

const loopContMainGo = `
func main() {
	for n := int32(0); n < 40; n++ {
		fmt.Printf("%d %d\n", dw(n), fc(n))
	}
}
`

func TestContinueRunsTheLoopsEnd(t *testing.T) {
	dir := t.TempDir()
	c := filepath.Join(dir, "loop.c")
	if err := os.WriteFile(c, []byte(loopContC), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "loop.go")
	if rc := Run([]string{c, dir, "-editor", out}, io.Discard, Profile{Header: "package main\n\nimport \"fmt\"\n\n"}); rc != 0 {
		t.Fatalf("the generator refused: %d", rc)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	g := goProgram(string(b))
	if strings.Contains(g, "goto") {
		t.Errorf("a goto is written:\n%s", g)
	}
	for _, want := range []string{
		"\t\t\t\tbreak cont", // a continue leaves the once-body, to the condition or the increment
		"\t\t\t\tbreak loop", // the loop's own break leaves the loop
		"k := n * 2",         // declared where C declares it
		"\t\t\tswitch ",      // the switch's break stays its own
	} {
		if !strings.Contains(g, want) {
			t.Errorf("no %q in:\n%s", want, g)
		}
	}

	for _, tool := range []string{"gcc", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	if err := os.WriteFile(c, []byte(loopContC+loopContMainC), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "loop")
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
		if err := os.WriteFile(filepath.Join(gdir, "main.go"), []byte(src+loopContMainGo), 0o644); err != nil {
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
		t.Errorf("the Go prints\n%s\nthe C\n%s", got, want)
	}
	// the control: the loop's own breaks written bare leave only the
	// once-body, and the loop runs on -- which the output shows
	wrong := g
	for _, l := range []string{"loop1", "loop2", "loop3", "loop4", "loop5", "loop6"} {
		wrong = strings.ReplaceAll(wrong, "break "+l+"\n", "break\n")
		wrong = strings.ReplaceAll(wrong, l+":\n", "")
	}
	if wrong == g {
		t.Fatal("the control changed nothing")
	}
	if goRun(wrong) == string(want) {
		t.Error("the control: a bare break for the loop's own prints the same")
	}
}
