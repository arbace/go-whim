package verify

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// whim-vim's compile line is the last boundary's: -no-pie from phase 83,
// -fno-stack-protector from phase 84.  whim.mk states it once, as WHIMCFLAGS
// and WHIMLDFLAGS, and `make score` passes both in the environment; an empty
// one takes the default here, for a run by hand.  The flags are applied to the
// object as well as the binary, because __stack_chk_fail is a symbol the
// default CFLAGS put there.
func Score(w io.Writer, whimCFLAGS, whimLDFLAGS string) {
	if whimCFLAGS == "" {
		whimCFLAGS = "-O0 -fno-stack-protector"
	}
	if whimLDFLAGS == "" {
		whimLDFLAGS = "-static -no-pie -s"
	}
	scoreRow(w, "slim-vim", "slim-vim.c", "slim-vim", "-static -s", "-O0")
	scoreRow(w, "whim-vim", "whim-vim.c", "whim-vim", whimLDFLAGS, whimCFLAGS)
}

func scoreRow(w io.Writer, name, src, bin, ldflags, cflags string) {
	sst, err := os.Stat(src)
	if err != nil || !sst.Mode().IsRegular() {
		fmt.Fprintf(w, "  %-10s %s\n", name, "absent")
		return
	}
	// BUILD WHEN THE BINARY IS MISSING **OR OLDER THAN THE SOURCE**.  Testing
	// only for absence reports the bytes of whatever was lying about: measured,
	// after phases 97-99 this printed 799,816 for a source that builds to
	// 805,544, because the binary on disk predated them by eight hours.  The
	// lines and the symbols were right -- both are recomputed from the source
	// below -- so the one stale column was the plausible-looking one.  "Older"
	// is in whole seconds, as the shell's `-nt` compared it.
	bst, err := os.Stat(bin)
	if err != nil || !bst.Mode().IsRegular() || sst.ModTime().Unix() > bst.ModTime().Unix() {
		args := append(append(strings.Fields(cflags), strings.Fields(ldflags)...), "-o", bin, src)
		cmd := exec.Command("gcc", args...)
		cmd.Stdout = w
		cmd.Run()
	}
	data, _ := os.ReadFile(src)
	lines := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lines++
	}
	var size int64
	if st, err := os.Stat(bin); err == nil && st.Mode().IsRegular() {
		size = st.Size()
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
