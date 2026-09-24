package p082

// Whim phase 82 -- every comment.  See GOAL.md.  (The system headers nothing
// needs were this phase's too; phase 167 drops them now, last and together.)
//
// EVERY COMMENT GOES: the former-file banners, the notes, and the lines earlier
// phases wrote to explain themselves.  Reasoning lives in the phase programs,
// GOALS.md and the commit messages; whim-vim.c carries code and nothing else, and
// no phase after this one writes a comment into it.  Comments do not reach the
// binary -- nothing uses __LINE__ -- so the byte-for-byte check covers this cut too.
// A comment is found by a scanner that knows string and character literals, since
// "pack/*/start/*" and "://" are data.  A line that was only a comment is deleted; a
// comment after code is cut with the whitespace before it; a blank run the deletions
// create is collapsed.
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
