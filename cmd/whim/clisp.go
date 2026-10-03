package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
)

// runC2lisp converts C to its s-expressions (crefactor/clisp, doc/C-LISP.md).
//
//	whim c2lisp [FILE]                the .lc on stdout (stdin with no FILE)
//	whim c2lisp -o OUT FILE           written to OUT
//	whim c2lisp --check FILE...       there and back: OK when lisp2c(c2lisp(F))
//	                                  is F byte for byte, for each F
//
// --check of a text that is not in cemit's canonical spelling says so, and
// whether the round trip gives the canonical text instead -- which is all it
// can give, since the forms hold the program and not its spacing.
func runC2lisp(args []string) int {
	out, check, files := "", false, []string(nil)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			check = true
		case "-o":
			i++
			if i >= len(args) {
				return c2lispUsage()
			}
			out = args[i]
		default:
			files = append(files, args[i])
		}
	}
	if check {
		if len(files) == 0 || out != "" {
			return c2lispUsage()
		}
		bad := 0
		for _, f := range files {
			if !c2lispCheck(f) {
				bad++
			}
		}
		if bad > 0 {
			fmt.Fprintf(os.Stderr, "  c2lisp       %d of %d differ\n", bad, len(files))
			return 1
		}
		return 0
	}
	if len(files) > 1 {
		return c2lispUsage()
	}
	path, src, err := readArg(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	lc, err := clisp.ToLisp(path, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  c2lisp       %v\n", err)
		return 1
	}
	return emit(out, lc)
}

func c2lispUsage() int {
	fmt.Fprintln(os.Stderr, "usage: whim c2lisp [-o OUT] [FILE] | whim c2lisp --check FILE...")
	return 2
}

// c2lispCheck converts one file there and back and reports.
func c2lispCheck(path string) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return false
	}
	t0 := time.Now()
	lc, err := clisp.ToLisp(path, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  c2lisp       %s: %v\n", path, err)
		return false
	}
	t1 := time.Now()
	back, err := clisp.ToC(lc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  c2lisp       %s: lisp2c: %v\n", path, err)
		return false
	}
	t2 := time.Now()
	times := fmt.Sprintf("%s to %s, %s and %s", size(len(src)), size(len(lc)),
		t1.Sub(t0).Round(time.Millisecond), t2.Sub(t1).Round(time.Millisecond))
	if bytes.Equal(back, src) {
		fmt.Printf("  c2lisp       %s OK: byte for byte, %d lines, %d lines of forms; %s\n", path, lines(src), lines(lc), times)
		return true
	}
	canon, cerr := cemit.Canonical(path, src)
	switch {
	case cerr == nil && !bytes.Equal(canon, src) && bytes.Equal(back, canon):
		fmt.Printf("  c2lisp       %s is not canonical; the round trip gives cemit's canonical text, byte for byte (%d lines); %s\n", path, lines(canon), times)
		return true
	case cerr == nil && !bytes.Equal(canon, src):
		fmt.Fprintf(os.Stderr, "  c2lisp       %s is not canonical, and the round trip is not its canonical text: %s\n", path, firstDiff(canon, back))
	default:
		fmt.Fprintf(os.Stderr, "  c2lisp       %s DIFFERS: %s\n", path, firstDiff(src, back))
	}
	return false
}

// runLisp2c prints s-expressions as C.
//
//	whim lisp2c [FILE.lc]             the C on stdout (stdin with no FILE)
//	whim lisp2c -o OUT FILE.lc        written to OUT
//	whim lisp2c --check FILE.lc FILE.c   OK when lisp2c(FILE.lc) is FILE.c byte
//	                                  for byte, and c2lisp of it is FILE.lc
func runLisp2c(args []string) int {
	out, check, files := "", false, []string(nil)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			check = true
		case "-o":
			i++
			if i >= len(args) {
				return lisp2cUsage()
			}
			out = args[i]
		default:
			files = append(files, args[i])
		}
	}
	if check {
		if len(files) != 2 || out != "" {
			return lisp2cUsage()
		}
		return lisp2cCheck(files[0], files[1])
	}
	if len(files) > 1 {
		return lisp2cUsage()
	}
	_, src, err := readArg(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	c, err := clisp.ToC(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  lisp2c       %v\n", err)
		return 1
	}
	return emit(out, c)
}

func lisp2cUsage() int {
	fmt.Fprintln(os.Stderr, "usage: whim lisp2c [-o OUT] [FILE.lc] | whim lisp2c --check FILE.lc FILE.c")
	return 2
}

func lisp2cCheck(lcPath, cPath string) int {
	lc, err := os.ReadFile(lcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	want, err := os.ReadFile(cPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	c, err := clisp.ToC(lc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  lisp2c       %s: %v\n", lcPath, err)
		return 1
	}
	if !bytes.Equal(c, want) {
		fmt.Fprintf(os.Stderr, "  lisp2c       %s is NOT %s: %s\n", lcPath, cPath, firstDiff(want, c))
		return 1
	}
	again, err := clisp.ToLisp(cPath, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  lisp2c       %s: c2lisp: %v\n", cPath, err)
		return 1
	}
	if !bytes.Equal(again, lc) {
		fmt.Fprintf(os.Stderr, "  lisp2c       %s gives %s, whose forms are not %s: %s\n", lcPath, cPath, lcPath, firstDiff(lc, again))
		return 1
	}
	fmt.Printf("  lisp2c       %s OK: it is %s byte for byte, and %s's forms are it byte for byte\n", lcPath, cPath, cPath)
	return 0
}

// readArg reads the one file named, or stdin.
func readArg(files []string) (string, []byte, error) {
	if len(files) == 0 {
		b, err := io.ReadAll(os.Stdin)
		return "<stdin>", b, err
	}
	b, err := os.ReadFile(files[0])
	return files[0], b, err
}

func emit(out string, data []byte) int {
	if out == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "whim: %v\n", err)
			return 1
		}
		return 0
	}
	if err := writeFile(out, data); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	return 0
}

// firstDiff names the first line two texts differ on, and shows both.
func firstDiff(want, got []byte) string {
	a, b := bytes.Split(want, []byte("\n")), bytes.Split(got, []byte("\n"))
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y []byte
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if i >= len(a) || i >= len(b) || !bytes.Equal(x, y) {
			return fmt.Sprintf("line %d: want %q, got %q", i+1, clip(x), clip(y))
		}
	}
	return "same lines, different bytes"
}

func clip(b []byte) string {
	if len(b) > 100 {
		return string(b[:100]) + "..."
	}
	return string(b)
}

func size(n int) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", n>>10)
}
