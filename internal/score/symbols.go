package score

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/build"
)

// SymbolCache is where the libc surface is kept, keyed by the source's own
// content: the first 32 hex digits of its sha256, `.u` the undefined names and
// `.d` the external ones.
const SymbolCache = ".cache/symbols"

// Symbols writes <out>/undefined and <out>/external: the libc surface of a
// source file, and what it defines externally.  It prints nothing.
//
// A phase asks this twice -- once of its input, to report what the phase cost
// in dependencies, and once of its output, to check it -- and THE INPUT OF ONE
// PHASE IS THE OUTPUT OF THE ONE BEFORE: the same bytes, compiled twice, a
// phase apart.  So the answer is cached under the file's own sha256: not by
// time, not by path, by content.  A phase's second question is the next
// phase's first, and the second time it is free.  Nothing can go stale,
// because a different file has a different key.
func Symbols(src, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// The object is compiled with the one line's CFLAGS, and they are part of
	// the key: an answer cached under other flags is another answer.
	cflags, _, _ := build.FlagsFor(0)
	sum := sha256.Sum256(append(append([]byte(strings.Join(cflags, " ")), 0), data...))
	sha := hex.EncodeToString(sum[:])[:32]
	if err := os.MkdirAll(SymbolCache, 0o755); err != nil {
		return err
	}
	cu := filepath.Join(SymbolCache, sha+".u")
	cd := filepath.Join(SymbolCache, sha+".d")
	if isFile(cu) && isFile(cd) {
		if err := copyFile(cu, filepath.Join(out, "undefined")); err != nil {
			return err
		}
		return copyFile(cd, filepath.Join(out, "external"))
	}

	obj := filepath.Join(out, ".symbols.o")
	args := append(append([]string{"-c"}, cflags...), "-o", obj, src)
	if err := exec.Command("gcc", args...).Run(); err != nil {
		return fmt.Errorf("gcc %s: %w", strings.Join(args, " "), err)
	}
	undef, ext := surface(os.Stderr, obj)
	os.Remove(obj)
	for _, wr := range []struct {
		path string
		b    []byte
	}{
		{filepath.Join(out, "undefined"), undef}, {filepath.Join(out, "external"), ext},
		{cu, undef}, {cd, ext},
	} {
		if err := os.WriteFile(wr.path, wr.b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// surface is an object's undefined names (`nm -u | awk '{print $2}' | sort`)
// and its external ones (`nm --extern-only --defined-only | awk '{print $NF}' |
// sort`), each a sorted file's bytes.  Byte order is sort's here: the locale is
// C.UTF-8, whose collation is the code point, which UTF-8 keeps in byte order.
func surface(errw io.Writer, obj string) (undef, ext []byte) {
	return sortedLines(awkColumn(nmOut(errw, obj, "-u"), 2)),
		sortedLines(awkColumn(nmOut(errw, obj, "--extern-only", "--defined-only"), -1))
}

// isFile is the shell's `[ -f path ]`: a regular file, through a symlink.
func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

func sortedLines(names []string) []byte {
	sort.Strings(names)
	var b bytes.Buffer
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// awkColumn is `awk '{print $N}'` over text, one entry per line: field n
// (1-based), or with n < 0 the last ($NF) -- which on a line with no fields is
// the line itself, as awk's $0.  Fields are split on blanks and tabs.
func awkColumn(text []byte, n int) []string {
	var col []string
	for _, l := range textLines(text) {
		f := strings.FieldsFunc(l, func(r rune) bool { return r == ' ' || r == '\t' })
		switch {
		case n < 0 && len(f) == 0:
			col = append(col, l)
		case n < 0:
			col = append(col, f[len(f)-1])
		case n <= len(f):
			col = append(col, f[n-1])
		default:
			col = append(col, "")
		}
	}
	return col
}

// nmOut is nm's standard output; its errors go to errw and its status is not
// consulted, as the shell's pipeline into awk did not consult it.
func nmOut(errw io.Writer, obj string, args ...string) []byte {
	var stdout bytes.Buffer
	cmd := exec.Command("nm", append(args, obj)...)
	cmd.Stdout, cmd.Stderr = &stdout, errw
	cmd.Run()
	return stdout.Bytes()
}

// textLines is a file's lines as grep and awk see them: split at newlines, a
// last line without one still a line, and no line after a final newline.  So
// len(textLines(b)) is what `grep -c` counts with an empty pattern.
func textLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	s := strings.TrimSuffix(string(b), "\n")
	return strings.Split(s, "\n")
}
