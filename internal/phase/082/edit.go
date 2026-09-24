package p082

// Whim phase 82 -- the system headers nothing needs, and every comment.
// See GOAL.md.
//
// whim-vim.c opens with the 41 #includes slim-vim.c has, and eighty-one phases have
// taken away most of what they were for: the directory walker, the locale,
// the password file, dlopen, setjmp, the maths library, utime, uname.  The object
// now leaves 80 symbols for libc to supply, and a header that supplies none of them
// -- no function, no type, no constant -- is a dependency on the host that buys
// nothing.
//
// THE SET IS COMPUTED, NOT LISTED.  A header's name says what it is for, not what
// this file uses from it: <sys/types.h> may be the only thing declaring a type the
// code names, and musl's headers include one another, so one that looks dead can be
// carrying another.  So each #include is tried: delete it and require the compile
// to stay SILENT under the sweep's flags.  gcc 15 compiles C23, where a call to an
// undeclared function is an error and an unknown type is an error, so "silent" is
// "nothing this header provided was used".  The candidates are tried one at a time
// and in parallel, then all together; if the set fails together -- two headers each
// covering for the other -- it falls back to removing them one by one, keeping each
// removal only while the build stays silent.  ONE BY ONE FROM THE BOTTOM: tried top
// down, the first dry run dropped <string.h> and <stdlib.h>, whose declarations also
// arrive through headers further down, and kept <wchar.h>.  The general headers come
// first in the file, so walking up from the end drops the specific ones and keeps
// the ones everything else leans on.
//
// THE PROOF IS THE BINARY, BYTE FOR BYTE.  A declaration that no longer exists cannot
// change what is compiled -- but a header can also define a function-like macro that
// shadows the function (musl's <ctype.h> does, for isalpha and friends), and losing
// one of those would change code without a word from the compiler if the prototype
// still came from somewhere.  So the input and the output are both built with
// SOURCE_DATE_EPOCH pinned, from the same file name, and must be identical.  That is
// tier 1 of the verification tiers, and it makes the delta "none" a measurement.
//
// EVERY COMMENT GOES TOO: the former-file banners, the notes, and the lines earlier
// phases wrote to explain themselves.  Reasoning lives in the phase programs,
// GOALS.md and the commit messages; whim-vim.c carries code and nothing else, and
// no phase after this one writes a comment into it.  Comments do not reach the
// binary -- nothing uses __LINE__ -- so the byte-for-byte check covers this cut too.
// A comment is found by a scanner that knows string and character literals, since
// "pack/*/start/*" and "://" are data.  A line that was only a comment is deleted; a
// comment after code is cut with the whitespace before it; a blank run the deletions
// create is collapsed.
// The input, for the binary comparison at the end.  Same file name both sides, or
// __FILE__ differs.  Built there, not here in the background: the header trials
// below end in a bare `wait`, which reaps every child, and a later `wait $pid` on a
// reaped child is status 127 -- which is how the first dry run died.
// The block: every #include, and they are the first directives in the file.
// One at a time, all at once.
// All together, and if not, one by one from the bottom.
// --- every comment ---------------------------------------------------------

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

var (
	blankRun3  = regexp.MustCompile(`\n\n\n+`)
	braceBlank = regexp.MustCompile(`\{\n\n`)
)

// commentSpans returns the start and end of every comment, found by a SCANNER
// THAT KNOWS STRING AND CHARACTER LITERALS -- a `//` inside "pack/*/start/*" or
// "://" is not one.  CLAUDE.md records 295 such occurrences on 16 lines of the
// product, every one inside a literal, which is why this cannot be a regex.
func commentSpans(s []byte) [][2]int {
	var Out [][2]int
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case c == '"' || c == '\'':
			i++
			for i < n && s[i] != c {
				if s[i] == '\\' {
					i += 2
				} else {
					i++
				}
			}
			i++
		case bytes.HasPrefix(s[i:], []byte("//")):
			j := bytes.IndexByte(s[i:], '\n')
			if j < 0 {
				Out = append(Out, [2]int{i, n})
				i = n
			} else {
				Out = append(Out, [2]int{i, i + j})
				i += j
			}
		case bytes.HasPrefix(s[i:], []byte("/*")):
			j := bytes.Index(s[i+2:], []byte("*/"))
			if j < 0 {
				Out = append(Out, [2]int{i, n})
				i = n
			} else {
				Out = append(Out, [2]int{i, i + 2 + j + 2})
				i += 2 + j + 2
			}
		default:
			i++
		}
	}
	return Out
}

// Whim82 strips every comment, and asserts the paragraphing it must not change.
//
// THE BLANK-LINE ASSERTIONS ARE THE POINT.  A comment on a line of its own
// becomes nothing, and two such lines either side of a blank one would leave a
// run of three newlines where there was one -- which no verification tier can
// see, since blank lines change neither the binary nor the token stream.  So
// the runs are counted before and after, and a brace followed by a blank line
// is collapsed ONLY if there were none to begin with.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	found := commentSpans(text)
	for _, s := range found {
		if bytes.HasPrefix(text[s[0]:s[1]], []byte("/*")) {
			return nil, fmt.Errorf("  %-12s a block comment -- this program only knows line comments", "nocomments")
		}
	}
	runsBefore := len(regexp.MustCompile(`\n\n\n`).FindAll(text, -1))
	afterBraceBefore := len(braceBlank.FindAll(text, -1))

	lines := strings.Split(string(text), "\n")
	starts := map[int]int{}
	for _, s := range found {
		ln := bytes.Count(text[:s[0]], []byte("\n"))
		col := s[0] - (bytes.LastIndex(text[:s[0]], []byte("\n")) + 1)
		starts[ln] = col
	}
	var Out []string
	gone, trimmed := 0, 0
	for ln, line := range lines {
		if col, ok := starts[ln]; ok {
			code := strings.TrimRight(line[:col], " \t\r\n\v\f")
			if code == "" {
				gone++
				continue
			}
			Out = append(Out, code)
			trimmed++
			continue
		}
		Out = append(Out, line)
	}
	t := []byte(strings.Join(Out, "\n"))
	t = bytes.TrimLeft(t, "\n")
	t = blankRun3.ReplaceAll(t, []byte("\n\n"))
	if afterBraceBefore == 0 {
		t = braceBlank.ReplaceAll(t, []byte("{\n"))
	}
	if n := len(commentSpans(t)); n > 0 {
		return nil, fmt.Errorf("  %-12s %d comments survive", "nocomments", n)
	}
	if len(regexp.MustCompile(`\n\n\n`).FindAll(t, -1)) > runsBefore {
		return nil, fmt.Errorf("  %-12s stripping made a run of blank lines", "nocomments")
	}
	fmt.Fprintf(w, "  %-12s %d comment lines deleted, %d comments cut from the end of a line\n",
		"nocomments", gone, trimmed)
	return t, nil
}

func init() { edit.Register("whim82", Edit) }
