package c23conf

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
)

// expected is the files expected to fail, each with its reason.  A file listed
// here that passes fails the test.
var expected = map[string]string{}

// stdFlags is the dialect every gcc run here is told.
var stdFlags = []string{"-std=c23"}

// objFlags is the one compile line's code generation (the Makefile's CFLAGS,
// whim's internal/build/compile.go), for an object: the link's -static
// -no-pie -s do not apply to one.
var objFlags = []string{"-O0", "-fno-stack-protector", "-c"}

// A result is one file's answer: the first stage it failed at, "" when none,
// and what cc.Translate said of it.
type result struct {
	stage, why string
	check      string
}

func needGCC(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc is not on PATH: the oracle is absent")
	}
}

// gccAccepts runs gcc -std=c23 -fsyntax-only on src, written as name in a
// directory of its own; the status is the answer, never the output.
func gccAccepts(dir, name string, src []byte) error {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, src, 0o644); err != nil {
		return err
	}
	args := append(append([]string{}, stdFlags...), "-fsyntax-only", p)
	if out, err := exec.Command("gcc", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("gcc refuses it: %v\n%s", err, out)
	}
	return nil
}

// object compiles src, as name in dir, to an object file and returns its
// bytes.  The same base name in two directories gives the same STT_FILE
// symbol, so two objects of one program are the same bytes.
func object(dir, name string, src []byte) ([]byte, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, src, 0o644); err != nil {
		return nil, err
	}
	o := strings.TrimSuffix(p, ".c") + ".o"
	args := append(append(append([]string{}, stdFlags...), objFlags...), "-o", o, p)
	cmd := exec.Command("gcc", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gcc -c: %v\n%s", err, out)
	}
	return os.ReadFile(o)
}

// samePrograms is stage 5: the original and its print have the same
// preprocessing tokens and compile to the same object.
func samePrograms(dir, name string, orig, print []byte) error {
	a, err := ppTokens(orig)
	if err != nil {
		return fmt.Errorf("the original's tokens: %v", err)
	}
	b, err := ppTokens(print)
	if err != nil {
		return fmt.Errorf("the print's tokens: %v", err)
	}
	if d := tokenDiff(labelNulls(a), labelNulls(b)); d != "" {
		return fmt.Errorf("the print's tokens differ: %s", d)
	}
	oa, err := object(filepath.Join(dir, "a"), name, orig)
	if err != nil {
		return err
	}
	ob, err := object(filepath.Join(dir, "b"), name, print)
	if err != nil {
		return err
	}
	if !bytes.Equal(oa, ob) {
		return fmt.Errorf("the print compiles to another object (%d bytes against %d)", len(ob), len(oa))
	}
	return nil
}

// labelNulls is the tokens without the null statement a label may have: a `;`
// right after a `:`, which in C is only ever a labeled null statement (a
// conditional's or a bit-field's colon is followed by an operand, an asm
// operand list's by an operand or a parenthesis).  C23 6.8.2 lets a label
// stand before a declaration or a block's closing brace with no statement,
// and says such a label is as if followed by a null statement; cemit prints
// that null statement, the spelling every C standard accepts, and whim's
// pipeline texts are written in it.  The one canonical rewrite the token
// judge allows, and the object judge still sees everything.
func labelNulls(toks []string) []string {
	out := toks[:0:0]
	for i, s := range toks {
		if s == ";" && i > 0 && toks[i-1] == ":" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func tokenDiff(a, b []string) string {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			lo := max(0, i-3)
			return fmt.Sprintf("token %d: %q where the original has %q (original %q, print %q)",
				i, y, x, a[lo:min(len(a), i+4)], b[lo:min(len(b), i+4)])
		}
	}
	return ""
}

// typeCheck is what cc.Translate, the type checker, says of src: "ok" or its
// first error.  A panic is returned as an error of its own.
func typeCheck(name string, src []byte) (s string, panicked error) {
	defer func() {
		if r := recover(); r != nil {
			panicked = fmt.Errorf("cc.Translate panics: %v", r)
		}
	}()
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return "", err
	}
	_, err = cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: name, Value: string(src)},
	})
	if err != nil {
		first, _, _ := strings.Cut(err.Error(), "\n")
		if i := strings.Index(first, name+":"); i >= 0 {
			first = first[i+len(name)+1:]
		}
		return "error: " + first, nil
	}
	return "ok", nil
}

// conform runs stages 1-6 on one file, and the type check beside them.
func conform(t *testing.T, name string, src []byte) result {
	dir := t.TempDir()
	if err := gccAccepts(filepath.Join(dir, "."), name, src); err != nil {
		t.Fatalf("%s: the oracle refuses the test's own file: %v", name, err)
	}
	var r result
	var err error
	r.check, err = typeCheck(name, src)
	if err != nil {
		t.Errorf("%s: %v", name, err)
	}
	if _, _, err := cemit.Parse(name, src); err != nil {
		return result{"parse", firstLine(err), r.check}
	}
	print, err := cemit.Canonical(name, src)
	if err != nil {
		return result{"print", firstLine(err), r.check}
	}
	pdir := filepath.Join(dir, "print")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := gccAccepts(pdir, name, print); err != nil {
		return result{"print", err.Error() + "\n" + string(print), r.check}
	}
	again, err := cemit.Canonical(name, print)
	if err != nil {
		return result{"fixpoint", firstLine(err), r.check}
	}
	if !bytes.Equal(again, print) {
		return result{"fixpoint", "cemit of its print differs:\n" + string(print) + "--\n" + string(again), r.check}
	}
	if err := samePrograms(filepath.Join(dir, "obj"), name, src, print); err != nil {
		return result{"same", err.Error() + "\n" + string(print), r.check}
	}
	lc, err := clisp.ToLisp(name, src)
	if err != nil {
		return result{"clisp", firstLine(err), r.check}
	}
	back, err := clisp.ToC(lc)
	if err != nil {
		return result{"clisp", "ToC: " + firstLine(err) + "\n" + string(lc), r.check}
	}
	if !bytes.Equal(back, print) {
		return result{"clisp", "the forms print back otherwise:\n" + string(lc) + "--\n" + string(back), r.check}
	}
	lc2, err := clisp.ToLisp(name, print)
	if err != nil || !bytes.Equal(lc2, lc) {
		return result{"clisp", fmt.Sprintf("the print's forms differ (%v):\n%s--\n%s", err, lc, lc2), r.check}
	}
	return r
}

func firstLine(err error) string {
	s, _, _ := strings.Cut(err.Error(), "\n")
	return s
}

func files(t *testing.T) []string {
	t.Helper()
	fs, err := filepath.Glob("testdata/*.c")
	if err != nil || len(fs) == 0 {
		t.Fatalf("no test files: %v", err)
	}
	sort.Strings(fs)
	return fs
}

// TestC23 is the conformance test: every file through every stage, the
// expected failures held to their list, and the score.
func TestC23(t *testing.T) {
	needGCC(t)
	fs := files(t)
	var mu sync.Mutex
	got := map[string]result{}
	t.Run("files", func(t *testing.T) {
		for _, f := range fs {
			name := filepath.Base(f)
			t.Run(strings.TrimSuffix(name, ".c"), func(t *testing.T) {
				t.Parallel()
				src, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				r := conform(t, name, src)
				mu.Lock()
				got[name] = r
				mu.Unlock()
				why, xfail := expected[name]
				switch {
				case r.stage != "" && !xfail:
					t.Errorf("%s fails at %s: %s", name, r.stage, r.why)
				case r.stage == "" && xfail:
					t.Errorf("%s passes, but is listed as an expected failure (%s): take it off the list", name, why)
				}
			})
		}
	})
	pass := 0
	var b strings.Builder
	for _, f := range fs {
		name := filepath.Base(f)
		r, ok := got[name]
		if !ok {
			continue
		}
		verdict := "pass"
		if r.stage != "" {
			verdict = "FAIL at " + r.stage
			if why, ok := expected[name]; ok {
				verdict += " (expected: " + why + ")"
			}
		} else {
			pass++
		}
		fmt.Fprintf(&b, "  %-22s %-12s type check: %s\n", name, verdict, r.check)
	}
	for _, f := range fs {
		if r := got[filepath.Base(f)]; r.stage != "" {
			fmt.Fprintf(&b, "\n%s, at %s:\n%s\n", filepath.Base(f), r.stage, r.why)
		}
	}
	t.Logf("C23 conformance: %d of %d\n%s", pass, len(fs), b.String())
}

// TestExpectedListed holds the expected failures to files that exist.
func TestExpectedListed(t *testing.T) {
	for name, why := range expected {
		if why == "" {
			t.Errorf("%s: an expected failure needs its reason", name)
		}
		if _, err := os.Stat(filepath.Join("testdata", name)); err != nil {
			t.Errorf("%s: listed as an expected failure, and not a test file", name)
		}
	}
}

// TestControl: a print that is wrong is caught.  Each mutation is of a
// correct print, and gcc accepts every one of them; the judge must still say
// no, and the right stage of it must be the one that does.
func TestControl(t *testing.T) {
	needGCC(t)
	for _, c := range []struct {
		name, src, from, to string
		objectsAlike        bool // the mutation moves the tokens and not the code
	}{
		// A dropped attribute: the code is the same, the program is not.
		{"fallthrough dropped", "int f(int x) { switch (x) { case 1: x++; [[fallthrough]]; default: break; } return x; }\n",
			"[[fallthrough]];", ";", true},
		// A respelled constant: the value is the same, the spelling is not.
		{"separators dropped", "int x = 1'000'000;\n", "1'000'000", "1000000", true},
		// A changed value: both judges see it.
		{"value changed", "int x = 4;\nint y = 0b1010;\n", "0b1010", "0b1011", false},
		// Two tokens that whitespace alone keeps apart: `- -a` is a, `--a`
		// decrements it.
		{"tokens glued", "int f(int a) { return - -a; }\n", "- -a", "--a", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			mutated := strings.Replace(c.src, c.from, c.to, 1)
			if mutated == c.src {
				t.Fatalf("the mutation %q -> %q does not apply", c.from, c.to)
			}
			err := samePrograms(dir, "x.c", []byte(c.src), []byte(mutated))
			if err == nil {
				t.Fatalf("the judge takes a wrong print:\n%s", mutated)
			}
			if !strings.Contains(err.Error(), "tokens differ") {
				t.Errorf("the tokens judge did not see it: %v", err)
			}
			// The objects alone, as the second judge.
			oa, err1 := object(filepath.Join(dir, "a"), "x.c", []byte(c.src))
			ob, err2 := object(filepath.Join(dir, "b"), "x.c", []byte(mutated))
			if err1 != nil || err2 != nil {
				t.Fatalf("gcc: %v %v", err1, err2)
			}
			if bytes.Equal(oa, ob) != c.objectsAlike {
				t.Errorf("objects alike: %v, expected %v", bytes.Equal(oa, ob), c.objectsAlike)
			}
		})
	}
	// And the judge takes a correct print: the same text, respaced.
	if err := samePrograms(t.TempDir(), "x.c", []byte("int x=1'0;\n"), []byte("int x = 1'0;\n")); err != nil {
		t.Errorf("the judge refuses a respacing: %v", err)
	}
}
