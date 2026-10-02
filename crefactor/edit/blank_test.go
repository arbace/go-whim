package edit

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
)

// TestBlankSame holds Blank to the byte loop it replaced, blankBytewise
// below: on edge cases, on random texts of quotes, slashes, stars,
// backslashes and newlines, and with $WHIM_C on that file.
func TestBlankSame(t *testing.T) {
	texts := []string{
		"", "\"", "'", "/", "\\", "a/b", "\"a\\\"b\" c", "'\\''", "/* x */y", "/* x", "// x\ny",
		"// x\\\ny\nz", "\"a\\\nb\"", "x = '/' / 2; /* \" */ \"/*\" // '\n", "\"abc\\", "a\"",
	}
	r := rand.New(rand.NewSource(1))
	alpha := []byte("ab\"'/*\\\n ")
	for k := 0; k < 20000; k++ {
		b := make([]byte, r.Intn(40))
		for i := range b {
			b[i] = alpha[r.Intn(len(alpha))]
		}
		texts = append(texts, string(b))
	}
	if f := os.Getenv("WHIM_C"); f != "" {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, string(src))
	}
	for _, s := range texts {
		if got, want := Blank([]byte(s)), blankBytewise([]byte(s)); !bytes.Equal(got, want) {
			t.Fatalf("%.60q: %.60q, the byte loop %.60q", s, got, want)
		}
	}
}

func blankBytewise(s []byte) []byte {
	out := make([]byte, len(s))
	n := len(s)
	for i := 0; i < n; {
		c := s[i]

		if c == '"' || c == '\'' {
			q := c
			out[i] = c
			i++
			for i < n {
				// An escape consumes two bytes, and a backslash-newline
				// keeps its newline so that line numbers still hold.
				if s[i] == '\\' && i+1 < n {
					out[i] = ' '
					if s[i+1] == '\n' {
						out[i+1] = '\n'
					} else {
						out[i+1] = ' '
					}
					i += 2
					continue
				}
				if s[i] == q {
					out[i] = q
					i++
					break
				}
				out[i] = keepNewline(s[i])
				i++
			}
			continue
		}

		if c == '/' && i+1 < n && s[i+1] == '*' {
			j := bytes.Index(s[i+2:], []byte("*/"))
			if j < 0 {
				j = n
			} else {
				j = i + 2 + j + 2
			}
			for k := i; k < j; k++ {
				out[k] = keepNewline(s[k])
			}
			i = j
			continue
		}

		if c == '/' && i+1 < n && s[i+1] == '/' {
			j := i
			for j < n && s[j] != '\n' {
				// A // comment continues across a spliced line break.
				if s[j] == '\\' && j+1 < n && s[j+1] == '\n' {
					j += 2
					continue
				}
				j++
			}
			for k := i; k < j; k++ {
				out[k] = keepNewline(s[k])
			}
			i = j
			continue
		}

		out[i] = c
		i++
	}
	return out
}
