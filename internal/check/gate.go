package check

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/dead"
	"github.com/arbace/go-whim/internal/harness"
	"github.com/arbace/go-whim/internal/sweep"
)

// The gate: the part of a check every phase has, and the libc surface it
// measures.  PhaseCheck, PhaseBuild and Symbols were tools/phasecheck.sh,
// tools/phasebuild.sh and tools/symbols.sh, and print what those printed, byte
// for byte; `whimtools phasecheck`, `phasebuild` and `symbols` are the same
// functions at a prompt.
//
// A refusal is reported in the output and returned as harness.ErrReported, so
// a caller adds nothing to it -- what check.Run of the script did.  So is a
// failure the script died of under `set -e` (a missing source, a work tree that
// cannot be written): it goes to the error writer and ErrReported comes back.

// SymbolCache is where the libc surface is kept, keyed by the source's own
// content: the first 32 hex digits of its sha256, `.u` the undefined names and
// `.d` the external ones.
const SymbolCache = ".cache/symbols"

// SymbolsLast is where PhaseCheck leaves its answers -- undefined, before and
// after -- for the check that called it.  It is beside the cache and NOT in the
// work tree: the work tree is the phase's output, and nothing that is merely
// how the phase checked itself belongs in it.
const SymbolsLast = SymbolCache + "/last"

// PhaseCheck is one compile, and everything a phase wants to know from it.
//
// A phase used to run gcc over its output three times: once to ask what it
// warned about, once to produce an object for the `nm` linkage check, and once
// more at the start of the NEXT phase to count the libc symbols of the very
// same bytes.  They are all one question.  This compiles once, with the warning
// flags, keeps the object, and reads the answers off it:
//
//   - did it compile, which is asked BEFORE what it complained about -- an
//     error is not a warning, and a sweep that counts lines matching "warning:"
//     finds none in a run that failed outright;
//   - exactly one external symbol, `main`;
//   - the nv_cmds index (dead.NvIdxCheck), which the compiler cannot check;
//   - what libc it still needs, cached by the source's sha256 so that the next
//     phase's first question is a cache hit.
//
// before is the stage's symbol snapshot (its `undefined` is the surface the
// stage was handed); it is REMOVED at the end, as the script removed it, so a
// check that wants it reads it first.  The answers are in SymbolsLast.
//
// Output and error go to w, where check.Run of the script sent both.
func PhaseCheck(w io.Writer, work, src, before string) error {
	return PhaseCheckTo(w, w, work, src, before)
}

// PhaseCheckTo is PhaseCheck with its error stream apart, for the command line.
func PhaseCheckTo(out, errw io.Writer, work, src, before string) error {
	fail := func(err error) error {
		fmt.Fprintf(errw, "phasecheck: %v\n", err)
		return harness.ErrReported
	}
	// A source that cannot be read is not a death here: the shell took its sha
	// through a pipe, so set -e never saw sha256sum fail, the sha was empty, and
	// the compile below is what refused it.
	data, srcSha := readSha(errw, src)

	obj := filepath.Join(work, "phase.o")
	gccTxt := filepath.Join(work, "gcc.txt")

	// THE SWEEP ALREADY COMPILED THIS FILE, twice over.  Its last round is, by
	// definition, a round on a file it then did not change -- so if the source
	// is still what that round saw, two of its answers are the two this compile
	// is for.  The warnings are deadsweep's stderr (last.txt), from a compile
	// that generates no code; the object is the plain -O0 one the sweep builds
	// in the background for PhaseBuild (build.o), and which symbols an object
	// defines and needs does not depend on warning flags.  Both are keyed by
	// content and both must match; anything else compiles as before.
	cd := sweep.CompileDir
	if isFile(filepath.Join(cd, "last.sha")) && isFile(filepath.Join(cd, "last.txt")) &&
		isFile(filepath.Join(cd, "build.sha")) && isFile(filepath.Join(cd, "build.o")) &&
		catFile(filepath.Join(cd, "last.sha")) == srcSha &&
		catFile(filepath.Join(cd, "build.sha")) == srcSha {
		if err := copyFile(filepath.Join(cd, "build.o"), obj); err != nil {
			return fail(err)
		}
		if err := copyFile(filepath.Join(cd, "last.txt"), gccTxt); err != nil {
			return fail(err)
		}
	} else {
		f, err := os.Create(gccTxt)
		if err != nil {
			// The shell's redirection failed inside the elif, which is a
			// compile that failed: the same line, and no error lines to show.
			fmt.Fprintf(errw, "phasecheck: %v\n", err)
			fmt.Fprintln(out, "  compile      FAILED -- the cut did not leave valid C")
			return harness.ErrReported
		}
		cmd := exec.Command("gcc", "-c", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter", "-o", obj, src)
		cmd.Stdout, cmd.Stderr = out, f
		err = cmd.Run()
		f.Close()
		if err != nil {
			fmt.Fprintln(out, "  compile      FAILED -- the cut did not leave valid C")
			n := 0
			for _, l := range textLines(readBytes(gccTxt)) {
				if n == 5 {
					break
				}
				if strings.Contains(l, "error:") {
					fmt.Fprintf(out, "               %s\n", l)
					n++
				}
			}
			return harness.ErrReported
		}
	}

	var warns []string
	for _, l := range textLines(readBytes(gccTxt)) {
		if strings.Contains(l, "warning:") && !strings.Contains(l, "implicit-fallthrough") {
			warns = append(warns, l)
		}
	}
	if len(warns) != 0 {
		fmt.Fprintf(out, "  warnings     %d besides the fall-throughs -- the sweep is not finished\n", len(warns))
		for i, l := range warns {
			if i == 5 {
				break
			}
			fmt.Fprintf(out, "               %s\n", l)
		}
		return harness.ErrReported
	}
	os.Remove(gccTxt)

	var ext []string
	for _, n := range awkColumn(nmOut(errw, obj, "--extern-only", "--defined-only"), -1) {
		if n != "main" {
			ext = append(ext, n)
		}
	}
	// The shell's $( ) dropped the trailing newlines, and kept every other one.
	if e := strings.TrimRight(strings.Join(ext, "\n"), "\n"); e != "" {
		fmt.Fprintf(out, "  linkage      these became external: %s\n", e)
		fmt.Fprintln(out, "               a dropped static declaration is a dropped linkage")
		return harness.ErrReported
	}
	fmt.Fprintln(out, "  linkage      nm on the object still prints exactly main")

	// A table index the compiler cannot check.  See nvidx.
	line, ok := dead.NvIdxCheck(data)
	fmt.Fprintln(out, line)
	if !ok {
		return harness.ErrReported
	}

	undef, extAll := surface(errw, obj)
	sha := srcSha[:32]
	cu, cdef := filepath.Join(SymbolCache, sha+".u"), filepath.Join(SymbolCache, sha+".d")
	if err := os.MkdirAll(SymbolCache, 0o755); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(cu, undef, 0o644); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(cdef, extAll, 0o644); err != nil {
		return fail(err)
	}
	os.Remove(obj)

	if err := os.MkdirAll(SymbolsLast, 0o755); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(SymbolsLast, "undefined"), undef, 0o644); err != nil {
		return fail(err)
	}

	after := len(textLines(undef))
	was := filepath.Join(before, "undefined")
	var first int
	if isFile(was) {
		old := readBytes(was)
		first = len(textLines(old))
		var gone strings.Builder
		for _, n := range comm23(textLines(old), textLines(undef)) {
			gone.WriteString(n + " ")
		}
		if gone.Len() > 0 {
			fmt.Fprintf(out, "  symbols      %d -> %d, gone: %s\n", first, after, gone.String())
		} else {
			fmt.Fprintf(out, "  symbols      %d -> %d\n", first, after)
		}
	} else {
		first = after
		fmt.Fprintf(out, "  symbols      %d\n", after)
	}
	if err := os.WriteFile(filepath.Join(SymbolsLast, "before"), []byte(fmt.Sprintf("%d\n", first)), 0o644); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(SymbolsLast, "after"), []byte(fmt.Sprintf("%d\n", after)), 0o644); err != nil {
		return fail(err)
	}
	os.RemoveAll(before)
	return nil
}

// PhaseBuild builds the phase's binary, work/whim-vim, by linking the object
// the sweep already compiled when there is one for exactly this text.
//
// Every phase used to end with `make clean && make`: a full compile of text the
// sweep's last round had just compiled.  The sweep's own compile cannot be
// reused as it is, and that was measured rather than assumed: -Wall -Wextra
// MOVE THE CODE -- the object built with them has 64 more bytes of .text and
// links to a different binary.  So the sweep also compiles a PLAIN object of
// each round's starting text, and the round that changes nothing is the round
// whose object is of the final text.  Linked with the work makefile's own flags
// it is byte-identical to what `make` produces, in a twentieth of a second.
//
// The object is used only when its recorded sha is the sha of the file NOW.
// Anything else -- a phase with no sweep, an edit made after the sweep -- builds
// the ordinary way, so this can never link a stale object.  before is the line
// count the report measures the phase against, printed as it is given.
func PhaseBuild(w io.Writer, work, before string) error {
	return PhaseBuildTo(w, w, work, before)
}

// PhaseBuildTo is PhaseBuild with its error stream apart, for the command line.
func PhaseBuildTo(out, errw io.Writer, work, before string) error {
	f := filepath.Join(work, "whim-vim.c")
	bin := filepath.Join(work, "whim-vim")
	_, sha := readSha(errw, f) // as in PhaseCheck: an unreadable file has no sha, and make refuses it
	obj := filepath.Join(sweep.CompileDir, "build.o")
	var how string
	if isFile(obj) && catFile(filepath.Join(sweep.CompileDir, "build.sha")) == sha {
		// The flags are read out of the work makefile rather than written here
		// a second time, so the link cannot drift from what `make` would do.
		args := append(MakeFlags(work), "-o", bin, obj)
		cmd := exec.Command("gcc", args...)
		cmd.Stdout, cmd.Stderr = out, errw
		if cmd.Run() != nil {
			fmt.Fprintln(out, "  build        FAILED linking the sweep's object")
			return harness.ErrReported
		}
		how = "linked from the sweep's object"
	} else {
		exec.Command("make", "-C", work, "clean").Run()
		if exec.Command("make", "-C", work).Run() != nil {
			fmt.Fprintf(out, "  build        FAILED -- rerun by hand: make -C %s\n", work)
			return harness.ErrReported
		}
		how = "compiled"
	}
	size := ""
	if st, err := os.Stat(bin); err == nil {
		size = fmt.Sprint(st.Size())
	} else {
		fmt.Fprintf(errw, "phasebuild: %v\n", err)
	}
	fmt.Fprintf(out, "  build        ok, %s -> %d lines, %s bytes, %s\n",
		before, len(textLines(readBytes(f))), size, how)
	return nil
}

// MakeFlags is the work makefile's CFLAGS and then LDFLAGS as make itself
// resolves them (`make -s -C work -p -n`), in the order make prints its
// database, split into words: the compile line is the boundary's, and this
// reads it rather than stating it again.
func MakeFlags(work string) []string {
	var stdout bytes.Buffer
	cmd := exec.Command("make", "-s", "-C", work, "-p", "-n")
	cmd.Stdout = &stdout
	cmd.Run() // the shell piped it to sed and never saw its status
	var words []string
	for _, l := range textLines(stdout.Bytes()) {
		// sed -n 's/^CFLAGS = //p; s/^LDFLAGS = //p': both run on every line
		for _, p := range []string{"CFLAGS = ", "LDFLAGS = "} {
			if strings.HasPrefix(l, p) {
				l = strings.TrimPrefix(l, p)
				words = append(words, strings.Fields(l)...)
			}
		}
	}
	return words
}

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
	sum := sha256.Sum256(data)
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
	if err := exec.Command("gcc", "-c", "-O0", "-o", obj, src).Run(); err != nil {
		return fmt.Errorf("gcc -c -O0 %s: %w", src, err)
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

// nmOut is nm's standard output; its errors go to errw and its status is not
// consulted, as the shell's pipeline into awk did not consult it.
func nmOut(errw io.Writer, obj string, args ...string) []byte {
	var stdout bytes.Buffer
	cmd := exec.Command("nm", append(args, obj)...)
	cmd.Stdout, cmd.Stderr = &stdout, errw
	cmd.Run()
	return stdout.Bytes()
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

func sortedLines(names []string) []byte {
	sort.Strings(names)
	var b bytes.Buffer
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte('\n')
	}
	return b.Bytes()
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

// comm23 is `comm -23 a b`: the lines of a that are not in b, both sorted.
func comm23(a, b []string) []string {
	var r []string
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch c := strings.Compare(a[i], b[j]); {
		case c < 0:
			r = append(r, a[i])
			i++
		case c > 0:
			j++
		default:
			i++
			j++
		}
	}
	return append(r, a[i:]...)
}

// isFile is the shell's `[ -f path ]`: a regular file, through a symlink.
func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// catFile is `$(cat path 2>/dev/null)`: the contents without their trailing
// newlines, and "" for a file that cannot be read.
func catFile(path string) string {
	return strings.TrimRight(string(readBytes(path)), "\n")
}

func readBytes(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

// readSha is a file and its sha256 in hex, or, for a file that cannot be read,
// nothing and "" -- what `$(sha256sum f | cut ...)` left the shell with, the
// failure reported on the error stream and the script going on.
func readSha(errw io.Writer, path string) ([]byte, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		msg := err.Error()
		var pe *os.PathError
		if errors.As(err, &pe) {
			msg = pe.Err.Error() // strerror, which C capitalises
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
		fmt.Fprintf(errw, "sha256sum: %s: %s\n", path, msg)
		return nil, ""
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:])
}
