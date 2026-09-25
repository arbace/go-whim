package suite

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/jeditor"
)

// termProbe compiles jeditor's host with testdata/TermProbe.java and returns
// a launcher that runs the probe as the editor's launcher runs the editor.
func termProbe(t *testing.T) string {
	for _, tool := range []string{"javac", "java"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	dir := t.TempDir()
	files, err := jeditor.WriteSources(filepath.Join(dir, "src"))
	if err != nil {
		t.Fatal(err)
	}
	var host []string
	for _, f := range files {
		if !strings.HasSuffix(f, "Whim.java") { // the glue needs Editor.java
			host = append(host, f)
		}
	}
	probe, _ := filepath.Abs("testdata/TermProbe.java")
	classes := filepath.Join(dir, "classes")
	args := append([]string{"-nowarn", "-d", classes, probe}, host...)
	if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	bin := filepath.Join(dir, "probe")
	if _, err := jeditor.WriteLauncher(bin, classes); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.Replace(string(b), " Whim ", " TermProbe ", 1))
	if err := os.WriteFile(bin, b, 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// The terminal host in Java on a real pseudo-terminal, as the pty cases run
// the editor: its size, its modes set and restored, input with and without a
// timeout, the four signals the editor reads as input or dies of -- raised,
// and delivered by kill(1) to the JVM -- a delay, a message on stderr, and
// the exit status.
func TestJavaTermHost(t *testing.T) {
	bin := termProbe(t)
	out, code, err := RunPty(bin, nil, []byte("ab"), ptySpec{rows: 30, cols: 100, term: "xterm"})
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	want := strings.Join([]string{
		"winsize 30x100",
		"ttykeys 127 3 false true", // the suite's raw slave: ICRNL off, ONLCR left
		"raw false false",          // TermStart's raw mode: both off
		"wait true",
		"read 2 ab",
		"idle false true", // nothing typed: the whole timeout, and no
		"winch true 16 ^[[48;30;100;0;0t",
		"sigwinch true 16 ^[[48;30;100;0;0t",
		"sigint true 1 ^C",
		"sigtstp true 5 ^[[?1z",
		"sigterm 15",
		"delay true",
		"suspend false",
		"to stderr",
		"",
	}, "\r\n")
	if string(out) != want || code != 3 {
		t.Errorf("on a terminal: status %d, want 3; printed\n%q\nwant\n%q", code, out, want)
	}

	// On a file, as the quick suite runs the editor: no terminal, so no size
	// and no modes; the keys at once, and then the end of input.
	out, code, err = Run(bin, []byte("ab"))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	want = strings.Join([]string{
		"winsize none",
		"ttykeys none",
		"raw none",
		"wait true",
		"read 2 ab",
		"idle true false", // a file at its end is readable at once
		"winch true 0 ",   // no size to report: the read finds the end
		"sigwinch true 0 ",
		"sigint true 1 ^C",
		"sigtstp true 5 ^[[?1z",
		"sigterm 15",
		"delay true",
		"suspend true 0 ", // continued: SIGCONT, and no size to report
		"to stderr",
		"",
	}, "\r\n")
	if string(out) != want || code != 3 {
		t.Errorf("on a file: status %d, want 3; printed\n%q\nwant\n%q", code, out, want)
	}
}
