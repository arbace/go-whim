// Package cutil is crefactor/text under its old name: the C-text
// substrate moved there (doc/VIM-VS-GENERIC.md section 4, migration step 3),
// and every name it had is forwarded here, so the phases, the cutters, the
// sweep and dead-code analysis that are written against cutil read as they
// did.  New code imports crefactor/text.
package cutil

import "github.com/arbace/go-whim/crefactor/text"

type Norm = text.Norm

func Blank(s []byte) []byte { return text.Blank(s) }

func Body(s []byte, name string) (opening, closing int, found, balanced bool) {
	return text.Body(s, name)
}

func CollapseWS(s []byte) []byte { return text.CollapseWS(s) }

func Contains(src, needle []byte) bool { return text.ContainsNorm(src, needle) }

func Count(src, needle []byte) int { return text.Count(src, needle) }

func CountAnchor(t, old string) int { return text.CountAnchor(t, old) }

func CountAnchorB(t []byte, old string) int { return text.CountAnchorB(t, old) }

func DeleteDefinition(s []byte, name string) ([]byte, bool) { return text.DeleteDefinition(s, name) }

func Depths(b []byte) []int { return text.Depths(b) }

func DropIf(s []byte, pattern string, count int) ([]byte, error) {
	return text.DropIf(s, pattern, count)
}

func FindBody(s []byte, name string) (opening, closing int, err error) { return text.FindBody(s, name) }

func FindDefinition(s, b []byte, name string) (start, end int, ok bool) {
	return text.FindDefinition(s, b, name)
}

func FindTop(s, b []byte, ch byte) int { return text.FindTop(s, b, ch) }

func FoldAlways(s []byte, pattern string, count int) ([]byte, error) {
	return text.FoldAlways(s, pattern, count)
}

func FoldNever(s []byte, pattern string, count int) ([]byte, error) {
	return text.FoldNever(s, pattern, count)
}

func Guarded(s, b []byte, m []int) (k, o, c int, head string, err error) {
	return text.Guarded(s, b, m)
}

func HasDefinition(s []byte, name string) bool { return text.HasDefinition(s, name) }

func Head(line string) string { return text.Head(line) }

func Index(src, needle []byte) int { return text.IndexNorm(src, needle) }

func IndexFrom(src, needle []byte, from int) int { return text.IndexNormFrom(src, needle, from) }

func Line(lines ...string) string { return text.Line(lines...) }

func Match(b []byte, i int) int { return text.Match(b, i) }

func Normalize(src []byte) *Norm { return text.Normalize(src) }

func PyRepr(v string) string { return text.PyRepr(v) }

func RMatch(b []byte, i int) int { return text.RMatch(b, i) }

func ReplaceAll(src, old, new []byte) []byte { return text.ReplaceAll(src, old, new) }

func ReplaceAnchor(t, old, new string, n int) string { return text.ReplaceAnchor(t, old, new, n) }

func ReplaceAnchorB(t []byte, old string, new []byte, n int) []byte {
	return text.ReplaceAnchorB(t, old, new, n)
}

func ReplaceBody(s []byte, name, body string) ([]byte, int, error) {
	return text.ReplaceBody(s, name, body)
}

func ReplaceFirst(src, old, new []byte) []byte { return text.ReplaceFirst(src, old, new) }

func ReplaceN(src, old, new []byte, n int) []byte { return text.ReplaceN(src, old, new, n) }

func SplitTop(s, b []byte, op string) [][]byte { return text.SplitTop(s, b, op) }
