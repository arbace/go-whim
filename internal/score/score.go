package score

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/build"
)

// Score prints what each product costs a target: bytes to store, and symbols
// to provide -- slim-vim beside whim-vim, because every number here is a delta
// from it.  It was tools/score.sh, and prints what that printed.
//
// GOALS.md measures phases against the two together, and the second is the
// one that matters.  An embedded target is defined by what it must supply, not
// by what it costs to store, so a phase that shrinks the binary while adding a
// libc call has gone backwards -- and only a report that shows both can say so.
//
// Both are built with the one compile line (build.FlagsFor), applied to the
// object as well as the binary, because __stack_chk_fail is a symbol the
// default CFLAGS put there.
func Score(w io.Writer) {
	c, l, _ := build.FlagsFor(0)
	cflags, ldflags := strings.Join(c, " "), strings.Join(l, " ")
	scoreRow(w, "slim-vim", "src/slim-vim.c", ldflags, cflags)
	scoreRow(w, "whim-vim", "src/whim-vim.c", ldflags, cflags)
}

func scoreRow(w io.Writer, name, src, ldflags, cflags string) {
	sst, err := os.Stat(src)
	if err != nil || !sst.Mode().IsRegular() {
		fmt.Fprintf(w, "  %-10s %s\n", name, "absent")
		return
	}
	// BUILT FRESH, INTO A SCRATCH DIRECTORY, every time.  It reused the binary
	// beside the source when it was not older, and once reported the bytes of a
	// binary eight hours stale; and the binaries beside the sources are make's,
	// stamped with the digest they were built from (the Makefile), which a
	// binary written here would not be.
	data, _ := os.ReadFile(src)
	lines := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lines++
	}
	var size int64
	if dir, err := os.MkdirTemp("", "score-bin."); err == nil {
		bin := filepath.Join(dir, name)
		args := append(append(strings.Fields(cflags), strings.Fields(ldflags)...), "-o", bin, src)
		cmd := exec.Command("gcc", args...)
		cmd.Stdout = w
		if cmd.Run() == nil {
			if st, err := os.Stat(bin); err == nil {
				size = st.Size()
			}
		}
		os.RemoveAll(dir)
	}
	// What it needs from the world: undefined symbols in the object, which is
	// the honest question.  A static binary has resolved them all already, so
	// asking the binary would answer "none" and mean nothing.
	syms := 0
	if dir, err := os.MkdirTemp("", "score."); err == nil {
		obj := filepath.Join(dir, "score.o")
		exec.Command("gcc", append(append([]string{"-c"}, strings.Fields(cflags)...), "-o", obj, src)...).Run()
		var out bytes.Buffer
		nm := exec.Command("nm", "-u", obj)
		nm.Stdout = &out
		nm.Run()
		seen := map[string]bool{}
		for _, l := range strings.Split(out.String(), "\n") {
			f := strings.FieldsFunc(l, func(r rune) bool { return r == ' ' || r == '\t' })
			n := l // awk's $NF on a line with no fields is the line
			if len(f) > 0 {
				n = f[len(f)-1]
			}
			if n != "" && !seen[n] {
				seen[n] = true
				syms++
			}
		}
		os.RemoveAll(dir)
	}
	fmt.Fprintf(w, "  %-10s %9s lines  %11s bytes  %4d libc symbols\n",
		name, thousands(int64(lines)), thousands(size), syms)
}

// thousands is n with a comma between every three digits.
func thousands(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
