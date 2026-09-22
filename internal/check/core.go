package check

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/arbace/go-whim/internal/harness"
)

// core is what every check from phase 129 on shares: the phase's source
// before and after, the standard gate -- a silent compile, the linkage, the
// libc surface, the build -- and probes that compare what the binary the
// phase was handed and the binary it made write to the terminal.
//
// Phases 129 onwards turn tx/FINDINGS.md into pipeline phases: each removes
// from the C a construct the Go transpilation (editor/editor.go) had to work
// around, and changes nothing the editor does.  So every one of them declares
// NOTHING in pipes/zero.delta, and the stage's delta check is the recording
// staying byte for byte what it was; the probes here are for what the
// recording cannot see.
type core struct {
	w            io.Writer
	work, state  string
	f            string // whim-vim.c in the work tree
	old, new     string // the source the phase was handed, and the output
	r            *rep
	symbolsInput string // the undefined symbols of the stage's input, saved before phasecheck removes them
}

func newCore(w io.Writer, args []string, name, tag string) (*core, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("usage: check %s <work-dir> <state-dir>", name)
	}
	c := &core{w: w, work: args[0], state: args[1], r: &rep{tag: tag, w: w}}
	c.f = filepath.Join(c.work, "whim-vim.c")
	c.old = readFile(filepath.Join(c.state, "old.c"))
	c.new = readFile(c.f)
	if c.old == "" || c.new == "" {
		return nil, fmt.Errorf("%s: the edit left no old.c beside the output", name)
	}
	c.symbolsInput = readFile(filepath.Join(c.state, "symbols", "undefined"))
	return c, nil
}

// gate is the standard part of every check: tools/phasecheck.sh (a compile with
// no warning, main the only external symbol, nv_cmds indexed, the libc
// surface) and tools/phasebuild.sh (the binary).  sameSymbols requires the
// libc surface to be the one the stage was handed, as a set.
func (c *core) gate(sameSymbols bool) error {
	if err := run(c.w, "sh", "tools/phasecheck.sh", c.work, c.f, filepath.Join(c.state, "symbols")); err != nil {
		return harness.ErrReported
	}
	if sameSymbols {
		after := readFile(".cache/symbols/last/undefined")
		if c.symbolsInput == "" || after != c.symbolsInput {
			c.r.say("the libc surface moved, and this phase frees and needs nothing:\n    before %q\n    after  %q",
				strings.Fields(c.symbolsInput), strings.Fields(after))
			return harness.ErrReported
		}
	}
	before := strings.TrimSpace(readFile(filepath.Join(c.state, "input-lines")))
	if err := run(c.w, "sh", "tools/phasebuild.sh", c.work, before); err != nil {
		return harness.ErrReported
	}
	return nil
}

// bins are the binary the phase was handed ($state/old, built by the edit)
// and the one it made.
func (c *core) bins() (string, string) {
	return filepath.Join(c.state, "old"), filepath.Join(c.work, "whim-vim")
}

// stream runs a binary over keys on the harness's 80x24 terminal and returns
// the digest of every byte it wrote, and its exit status.  Two binaries that
// draw the same thing write the same bytes; the digest is the comparison.
func stream(bin string, keys [][]byte, args []string) (string, int, error) {
	_, out, _, rc, err := harness.ZSession(bin, keys, "xterm", args, 24, 80, 20*time.Second)
	if err != nil {
		return "", rc, err
	}
	s := sha256.Sum256(out)
	return hex.EncodeToString(s[:])[:16], rc, nil
}

// word counts whole-word mentions of name.
func word(text, name string) int {
	return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllStringIndex(text, -1))
}

// linesWith is every line mentioning name as a word, trimmed.
func linesWith(text, name string) []string {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	var r []string
	for _, l := range strings.Split(text, "\n") {
		if re.MatchString(l) {
			r = append(r, strings.TrimSpace(l))
		}
	}
	return r
}

// buildAt builds src with the boundary's CFLAGS and LDFLAGS and
// SOURCE_DATE_EPOCH=epoch, and returns the binary's bytes.
func (c *core) buildAt(src, epoch string) (string, error) {
	mk := readFile(filepath.Join(c.work, "Makefile"))
	flag := func(k string) []string {
		m := regexp.MustCompile(`(?m)^` + k + `  *= *(.*)$`).FindStringSubmatch(mk)
		if m == nil {
			return nil
		}
		return strings.Fields(m[1])
	}
	tmp, err := os.MkdirTemp("", "core-build-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "b")
	cmd := exec.Command("gcc", append(append(flag("CFLAGS"), flag("LDFLAGS")...), "-o", out, src)...)
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH="+epoch)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return readFile(out), nil
}

// sameBinary: both sources built with the boundary's flags, SOURCE_DATE_EPOCH=0,
// are the same bytes.  For a phase whose claim is that it changes no code.
func (c *core) sameBinary() (bool, int, error) {
	a, err := c.buildAt(filepath.Join(c.state, "old.c"), "0")
	if err != nil {
		return false, 0, err
	}
	b, err := c.buildAt(c.f, "0")
	if err != nil {
		return false, 0, err
	}
	return a == b, len(b), nil
}

// swept is text as the pipeline would leave it: written to a scratch file and
// run through tools/sweep.sh, the sweep every stage runs.  A check that states
// its phase's output as "this rule applied to the input" computes it with the
// sweep the pipeline really uses, and not a copy of its rules.
func swept(text string) (string, error) {
	tmp, err := os.MkdirTemp("", "core-swept-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	pf := filepath.Join(tmp, "whim-vim.c")
	if err := os.WriteFile(pf, []byte(text), 0o644); err != nil {
		return "", err
	}
	if out, err := exec.Command("sh", "tools/sweep.sh", pf).CombinedOutput(); err != nil {
		return "", fmt.Errorf("the sweep refused the computed text: %s", strings.TrimSpace(string(out)))
	}
	return readFile(pf), nil
}

// sweptLines is what the sweep took from pre to reach out, beyond structure:
// the non-brace lines of pre that out does not have, in order.
func sweptLines(pre, out string) []string {
	have := map[string]int{}
	for _, l := range strings.Split(out, "\n") {
		have[l]++
	}
	var took []string
	for _, l := range strings.Split(pre, "\n") {
		if have[l] > 0 {
			have[l]--
			continue
		}
		if t := strings.TrimSpace(l); t != "" && t != "{" && t != "}" {
			took = append(took, t)
		}
	}
	return took
}

// firstDiff names the first line two texts disagree on.
func firstDiff(a, b string) (int, string, string) {
	x, y := strings.Split(a, "\n"), strings.Split(b, "\n")
	i := 0
	for i < len(x) && i < len(y) && x[i] == y[i] {
		i++
	}
	return i + 1, at(x, i), at(y, i)
}
