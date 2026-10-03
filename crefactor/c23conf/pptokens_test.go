package c23conf

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// ppTokens splits C source into C23's preprocessing tokens (6.4): the
// identifiers, pp-numbers, character constants, string literals and
// punctuators, with the whitespace and the comments between them gone.  It
// is the test's own lexer, written from the standard and not shared with the
// front end it judges.  The test's files have no directive and no line
// splice, so neither is lexed; a byte it does not know is an error.
func ppTokens(src []byte) ([]string, error) {
	s := string(src)
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++
		case strings.HasPrefix(s[i:], "//"):
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				j = len(s) - i
			}
			i += j
		case strings.HasPrefix(s[i:], "/*"):
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				return nil, fmt.Errorf("a comment not closed at %d", i)
			}
			i += j + 4
		case c == '#':
			return nil, fmt.Errorf("a directive at %d: the test's files have none", i)
		case isDigit(c) || c == '.' && i+1 < len(s) && isDigit(s[i+1]):
			j := ppNumber(s, i)
			out = append(out, s[i:j])
			i = j
		case isIdentStart(c):
			j := i
			for j < len(s) && isIdentCont(s[j]) {
				j++
			}
			// An encoding prefix is part of the literal it begins (6.4.4.5,
			// 6.4.5): u8'a' is one token, not an identifier and a constant.
			if j < len(s) && (s[j] == '\'' || s[j] == '"') && slices.Contains([]string{"L", "u", "U", "u8"}, s[i:j]) {
				k, err := quoted(s, j)
				if err != nil {
					return nil, err
				}
				out = append(out, s[i:k])
				i = k
				break
			}
			out = append(out, s[i:j])
			i = j
		case c == '\'' || c == '"':
			k, err := quoted(s, i)
			if err != nil {
				return nil, err
			}
			out = append(out, s[i:k])
			i = k
		default:
			p := punctuator(s[i:])
			if p == "" {
				return nil, fmt.Errorf("a byte no token begins with, %q at %d", c, i)
			}
			out = append(out, p)
			i += len(p)
		}
	}
	return out, nil
}

// ppNumber is the end of the pp-number at i (6.4.8): a digit or a period and
// a digit, then digits, identifier characters, periods, an exponent's sign
// after e, E, p or P, and C23's digit separator, a ' between two of the
// others.
func ppNumber(s string, i int) int {
	j := i + 1
	for j < len(s) {
		c := s[j]
		switch {
		case (c == '+' || c == '-') && strings.ContainsRune("eEpP", rune(s[j-1])):
			j++
		case c == '\'' && j+1 < len(s) && isIdentCont(s[j+1]):
			j += 2
		case c == '.' || isIdentCont(c):
			j++
		default:
			return j
		}
	}
	return j
}

// quoted is the end of the character constant or string literal whose quote
// is at i.
func quoted(s string, i int) (int, error) {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '\n':
			return 0, fmt.Errorf("a literal not closed at %d", i)
		case q:
			return j + 1, nil
		}
	}
	return 0, fmt.Errorf("a literal not closed at %d", i)
}

// punctuators is 6.4.6's, longest first, the digraphs left out (the files
// spell none); `::` is C23's.
var punctuators = []string{
	"...", "<<=", ">>=",
	"->", "++", "--", "<<", ">>", "<=", ">=", "==", "!=", "&&", "||",
	"*=", "/=", "%=", "+=", "-=", "&=", "^=", "|=", "##", "::",
	"[", "]", "(", ")", "{", "}", ".", "&", "*", "+", "-", "~", "!",
	"/", "%", "<", ">", "^", "|", "?", ":", ";", "=", ",", "#",
}

func punctuator(s string) string {
	for _, p := range punctuators {
		if strings.HasPrefix(s, p) {
			return p
		}
	}
	return ""
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}
func isIdentCont(c byte) bool { return isIdentStart(c) || isDigit(c) }

func TestPPTokens(t *testing.T) {
	for _, c := range []struct {
		src  string
		want []string
	}{
		{"int x = 1'000'000;", []string{"int", "x", "=", "1'000'000", ";"}},
		{"a - -b --c", []string{"a", "-", "-", "b", "--", "c"}},
		{"[[gnu::packed]]", []string{"[", "[", "gnu", "::", "packed", "]", "]"}},
		{"u8'a' u8\"x\" L'\\'' u 'b'", []string{"u8'a'", "u8\"x\"", "L'\\''", "u", "'b'"}},
		{"1.5e+3f 0x1p-2 .5 3uwb 1'0.5'0", []string{"1.5e+3f", "0x1p-2", ".5", "3uwb", "1'0.5'0"}},
		{"a/*x*/b // y\nc", []string{"a", "b", "c"}},
		{"x<<=y>>z...", []string{"x", "<<=", "y", ">>", "z", "..."}},
		{"int café;", []string{"int", "café", ";"}},
	} {
		got, err := ppTokens([]byte(c.src))
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%q: %q, %v; want %q", c.src, got, err, c.want)
		}
	}
}
