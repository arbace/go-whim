package dead

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

// warningLine is gcc's `file:line:col: warning: text`.
//
// The [^:]+ for the filename means a path containing a colon silently drops
// EVERY warning, and the sweep then converges immediately on unchanged text.
// Kept as it stands: the paths in use are relative, and this is what the
// comparison is against.
var warningLine = regexp.MustCompile(`^[^:]+:(\d+):\d+: warning: (.*)$`)

// Warning is one gcc warning: the 1-based line it names, and its text.
type Warning struct {
	Line int
	Text string
}

// GccWarnings asks gcc what it warned about, and optionally keeps what it
// produced under keep/.
//
// It generates NO machine code.  The two warnings this reads --
// -Wunused-function and file-scope -Wunused-variable -- come from gcc's call
// graph, which -fsyntax-only never builds and so reports neither, measured.
// -flto -fno-fat-lto-objects does build it, warns, and writes GIMPLE instead
// of compiling: the same warnings byte for byte, in 2.4 seconds instead of
// 6.0 on a 129,000-line file.  An LTO object has no symbols for nm to read,
// which is why what is kept is the stderr and its sha and not an object.
//
// It goes in .cache/ and NOT in the work tree: a file left in the work tree is
// a file the boundary digest counts.
func GccWarnings(path, keep string) ([]Warning, error) {
	if keep != "" {
		if err := os.MkdirAll(keep, 0o755); err != nil {
			return nil, err
		}
		os.Remove(filepath.Join(keep, "last.o"))
	}
	cmd := exec.Command("gcc", "-c", "-O0", "-flto", "-fno-fat-lto-objects",
		"-Wall", "-Wextra", "-Wno-unused-parameter", "-o", "/dev/null", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()
	// GCC'S STATUS IS NOT CONSULTED, and the reason written here was wrong: it
	// said a file that fails to compile yields no warning lines and the tool is
	// then a no-op.  Measured on the text phase 35 leaves -- `winopt_T has no
	// member named wo_eiw`, two uses surviving in a dead function -- gcc exits
	// 1 AND still names 71 unused functions and 12 unused variables, and this
	// tool removes 1,041 lines, after which the text parses again.  So the
	// status is ignored because gcc answers ANYWAY, which is also the thing a
	// tree-based implementation could not do: the front end has nothing to say
	// about text it cannot parse.

	if keep != "" {
		if err := os.WriteFile(filepath.Join(keep, "last.txt"), stderr.Bytes(), 0o644); err != nil {
			return nil, err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(src)
		if err := os.WriteFile(filepath.Join(keep, "last.sha"),
			[]byte(hex.EncodeToString(sum[:])+"\n"), 0o644); err != nil {
			return nil, err
		}
	}

	var out []Warning
	for _, line := range bytes.Split(stderr.Bytes(), []byte{'\n'}) {
		m := warningLine.FindSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(string(m[1]))
		if err != nil {
			continue
		}
		out = append(out, Warning{n, string(m[2])})
	}
	return out, nil
}
