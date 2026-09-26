package vijure

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// standIn is the hand-written whim.editor the tests build: what the
// contract says the generated one provides, and no editor.
const standIn = "testdata/standin/whim/editor.clj"

func needTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"javac", "java", "clojure"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// run runs the launcher bin with args, keys on stdin (a file, as the suite
// feeds them), and returns stdout, stderr and the exit status.  It dies
// with the test.
func run(t *testing.T, bin string, keys string, args ...string) (string, string, int) {
	t.Helper()
	in := filepath.Join(t.TempDir(), "keys")
	if err := os.WriteFile(in, []byte(keys), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	cmd.Stdin = f
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), code
}

// The stand-in namespace, built as the generated one is (Compile), runs
// under the launcher and reaches every host function through the glue: the
// arguments' bytes as they were given, the host's answers, host_alloc and
// its arena's exhaustion, vim_snprintf with its variadic arguments and its
// errors (through the core's emsg, by name), the keys read, host_exit, a
// status returned by vim_main, and an exception reported as the Java
// launcher reports one, with status 70.  Then the same as one jar, run by
// java -jar.
func TestStandInRuns(t *testing.T) {
	needTools(t)
	dir := t.TempDir()
	bin, err := Compile(standIn, filepath.Join(dir, "clj"), filepath.Join(dir, "vijure"))
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(dir, "clj", CacheName)) != strings.Contains(string(launcher), "-XX:AOTCache=") {
		t.Errorf("the launcher and the AOT cache disagree:\n%s", launcher)
	}

	out, errOut, code := run(t, bin, "xiiVq", "-R", "a b", "\xff\xfe")
	for _, want := range []string{
		"argc=4 [-R] [a b] [\xff\xfe]\r\n",
		" INSERT INSERT[" + bin + "]",
		"alloc=100 zero=0 ",
		"time=1 now=1\r\n",
		"fmt=-7    ab|42  |ff|A\r\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output has no %q:\n%q", want, out)
		}
	}
	if code != 5 || errOut != "" {
		t.Errorf("q: status %d, stderr %q; want 5 and nothing", code, errOut)
	}

	cases := []struct {
		keys   string
		code   int
		stderr string
	}{
		{"", 0, ""},   // the end of input: vim_main returns 0
		{"iE", 3, ""}, // host_exit through the terminal host
		{"P", 0, "E1504: Positional argument 1 type used inconsistently: int/string\n"},
		{"A", 1, "whim-vim: host arena exhausted: 1073741824 bytes, 112 used, request 2147483648\n"},
		{"T", 70, "vijure: java.lang.IllegalStateException: the stand-in was told to throw\n\tat whim.editor$vim_main"},
	}
	for _, c := range cases {
		_, errOut, code := run(t, bin, c.keys)
		if code != c.code || !strings.HasPrefix(errOut, c.stderr) || (c.stderr == "" && errOut != "") {
			t.Errorf("keys %q: status %d, stderr %q; want %d and %q", c.keys, code, errOut, c.code, c.stderr)
		}
	}

	jar := filepath.Join(dir, "vijure.jar")
	if err := Jar(filepath.Join(dir, "clj"), jar); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(jar)
	if err != nil {
		t.Fatal(err)
	}
	zr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-jar", jar, "x")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	cmd.Stdin = strings.NewReader("iq")
	b, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 5 || !strings.Contains(string(b), "argc=2 [x]") || !strings.Contains(string(b), " INSERT") {
		t.Errorf("java -jar: %v\n%s", err, b)
	}
}

// A reflective call in the namespace is refused, the compiler's warning
// quoted: the contract is that there is none.
func TestReflectionRefused(t *testing.T) {
	needTools(t)
	b, err := os.ReadFile(standIn)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ns := filepath.Join(dir, "editor.clj")
	b = append(b, "\n(defn reflective [x] (.length x))\n"...)
	if err := os.WriteFile(ns, b, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Compile(ns, filepath.Join(dir, "clj"), filepath.Join(dir, "vijure"))
	if err == nil || !strings.Contains(err.Error(), "Reflection warning") || !strings.Contains(err.Error(), "length") {
		t.Errorf("a reflective call was not refused: %v", err)
	}
}

// Math on a boxed number in the namespace is refused, the compiler's warning
// quoted: the arithmetic is primitive, and a change that boxes it is slower
// where nothing in the suite would see it.
func TestBoxedMathRefused(t *testing.T) {
	needTools(t)
	b, err := os.ReadFile(standIn)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ns := filepath.Join(dir, "editor.clj")
	b = append(b, "\n(defn boxed [x] (inc x))\n"...)
	if err := os.WriteFile(ns, b, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Compile(ns, filepath.Join(dir, "clj"), filepath.Join(dir, "vijure"))
	if err == nil || !strings.Contains(err.Error(), "Boxed math warning") || !strings.Contains(err.Error(), "inc") {
		t.Errorf("boxed math was not refused: %v", err)
	}
}

// A generator that writes nothing -- a toolset without the Clojure backend
// -- is named, not taken for an empty namespace.
func TestNoBackend(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "src", "whim-vim.c"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Skip(err)
	}
	_, err = Generate(func(string, string, string) error { return nil }, src, t.TempDir())
	if !errors.Is(err, ErrNoBackend) {
		t.Errorf("Generate: %v, want ErrNoBackend", err)
	}
}
