package p142

// Whim phase 142, the check -- the version names no build date or time.
// See phase/142/edit.go, and GOALS.md.
//
// phase/142/check.go requires __DATE__ and __TIME__ gone, the output
// built at two SOURCE_DATE_EPOCHs identical where the input's differ, and the
// version line a command-line error prints.

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim142", Check) }

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
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim142", "datetime")
	if err != nil {
		return err
	}
	r := c.R
	for _, n := range []string{"__DATE__", "__TIME__"} {
		if k := check.Word(c.New, n); k != 0 {
			r.Bad("%s has %d mentions left", n, k)
		}
	}
	if !strings.Contains(c.New, `char *msg = _("%s (%s)");`) {
		r.Bad("the version format is not the name and the release date")
	}
	if err := r.Done(); err != nil {
		return err
	}
	a, err1 := c.BuildAt(c.F, "0")
	b, err2 := c.BuildAt(c.F, "1000000000")
	oa, err3 := c.BuildAt(filepath.Join(c.State, "old.c"), "0")
	ob, err4 := c.BuildAt(filepath.Join(c.State, "old.c"), "1000000000")
	for _, e := range []error{err1, err2, err3, err4} {
		if e != nil {
			r.Bad("a build failed: %v", e)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	if a != b {
		r.Bad("the output built at two SOURCE_DATE_EPOCHs differs: it still reads the clock")
	}
	if oa == ob {
		r.Bad("the CONTROL did not move: the input built at two SOURCE_DATE_EPOCHs is the same, so the builds cannot tell a date apart")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("built at SOURCE_DATE_EPOCH 0 and 1000000000 the output is the same %d bytes; the input, the CONTROL, differs", len(a))
	if err := c.Gate(true); err != nil {
		return err
	}
	oldBin, newBin := c.Bins()
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
		r.Bad("a command-line error is headed %q", nl)
	}
	if !strings.HasPrefix(ol, "VIM - Vi IMproved 9.2 (2026 Feb 14, compiled ") {
		r.Bad("the input's command-line error was headed %q, not the dated line this phase shortens", ol)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("`-T` with no argument is headed %q, where the input said %q", nl, ol)
	return nil
}
