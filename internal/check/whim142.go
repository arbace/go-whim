package check

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() { register("whim142", Whim142) }

// Whim142 is phase 142's check: the version names no build date or time.
//
//  1. THE CUT: __DATE__ and __TIME__ have no mention, and the version format
//     is "%s (%s)".
//  2. THE BUILD IS A FUNCTION OF THE SOURCE: the output built at two different
//     SOURCE_DATE_EPOCHs is the same bytes; the CONTROL, the input built the
//     same two ways, is not -- so the two builds could tell a date apart.
//  3. THE GATE, the libc surface unchanged.
//  4. WHAT IT SAYS: a command-line error prints "VIM - Vi IMproved 9.2 (2026 Feb
//     14)" above it, measured by running the binary, and the input printed the
//     same line with ", compiled ..." before the closing parenthesis.  The
//     recorded invocations that show the line are the declared delta.
func Whim142(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim142", "datetime")
	if err != nil {
		return err
	}
	r := c.r
	for _, n := range []string{"__DATE__", "__TIME__"} {
		if k := word(c.new, n); k != 0 {
			r.bad("%s has %d mentions left", n, k)
		}
	}
	if !strings.Contains(c.new, `char *msg = _("%s (%s)");`) {
		r.bad("the version format is not the name and the release date")
	}
	if err := r.done(); err != nil {
		return err
	}
	a, err1 := c.buildAt(c.f, "0")
	b, err2 := c.buildAt(c.f, "1000000000")
	oa, err3 := c.buildAt(filepath.Join(c.state, "old.c"), "0")
	ob, err4 := c.buildAt(filepath.Join(c.state, "old.c"), "1000000000")
	for _, e := range []error{err1, err2, err3, err4} {
		if e != nil {
			r.bad("a build failed: %v", e)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	if a != b {
		r.bad("the output built at two SOURCE_DATE_EPOCHs differs: it still reads the clock")
	}
	if oa == ob {
		r.bad("the CONTROL did not move: the input built at two SOURCE_DATE_EPOCHs is the same, so the builds cannot tell a date apart")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("built at SOURCE_DATE_EPOCH 0 and 1000000000 the output is the same %d bytes; the input, the CONTROL, differs", len(a))
	if err := c.gate(true); err != nil {
		return err
	}
	oldBin, newBin := c.bins()
	first := func(bin string) (string, error) {
		cmd := exec.Command(bin, "-T")
		var e bytes.Buffer
		cmd.Stderr = &e
		_ = cmd.Run()
		l, _, _ := strings.Cut(e.String(), "\n")
		return l, nil
	}
	nl, _ := first(newBin)
	ol, _ := first(oldBin)
	if nl != "VIM - Vi IMproved 9.2 (2026 Feb 14)" {
		r.bad("a command-line error is headed %q", nl)
	}
	if !strings.HasPrefix(ol, "VIM - Vi IMproved 9.2 (2026 Feb 14, compiled ") {
		r.bad("the input's command-line error was headed %q, not the dated line this phase shortens", ol)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("`-T` with no argument is headed %q, where the input said %q", nl, ol)
	return nil
}
