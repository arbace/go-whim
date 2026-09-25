package edit

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/crefactor/text"
)

// The verb set and the helpers the phases share moved to
// internal/crefactor/text (doc/VIM-VS-GENERIC.md section 4, migration step
// 3): E, Ph, the structural verbs, the literal scanners, the dead-store
// fixpoint and the half of shared.go that knows nothing of vim.  Every name
// is forwarded here, so the phase packages that write edit.E and edit.Ph read
// as they did.  New code imports internal/crefactor/text.

type E = text.E
type Ph = text.Ph

var (
	BareDeclOnly  = text.BareDeclOnly
	EnclosingLoop = text.EnclosingLoop
	W134Else      = text.W134Else
	W134Empty     = text.W134Empty
	W134Fn        = text.W134Fn
	W134Write     = text.W134Write
)

func BindsToWalk(raw []byte) string { return text.BindsToWalk(raw) }

func CallsNotAfterWord(t []byte, name string) [][]int { return text.CallsNotAfterWord(t, name) }

func Contains[T comparable](s []T, v T) bool { return text.Contains[T](s, v) }

func ContainsStr(xs []string, x string) bool { return text.ContainsStr(xs, x) }

func CoreCallRe(name string) *regexp.Regexp { return text.CoreCallRe(name) }

func CoreCalls(t []byte, name string) int { return text.CoreCalls(t, name) }

func CoreHead(s string, n int) string { return text.CoreHead(s, n) }

func CountNewlines(b []byte) int { return text.CountNewlines(b) }

func DeadStores(core []byte) ([]byte, []string) { return text.DeadStores(core) }

func First[T any](s []T, n int) []T { return text.First[T](s, n) }

func IncludeCount(t []byte) int { return text.IncludeCount(t) }

func IndexFrom(t, needle []byte, from int) int { return text.IndexFrom(t, needle, from) }

func IndexOf(s []string, v string) int { return text.IndexOf(s, v) }

func IsWordByte(c byte) bool { return text.IsWordByte(c) }

func JoinInts(v []int) string { return text.JoinInts(v) }

func LastNewlineBefore(t []byte, i int) int { return text.LastNewlineBefore(t, i) }

func LiteralSpans(p Ph, t []byte) ([][2]int, error) { return text.LiteralSpans(p, t) }

func LiteralSpansShort(p Ph, t []byte) ([][2]int, error) { return text.LiteralSpansShort(p, t) }

func MentionCount(t []byte, name string) int { return text.MentionCount(t, name) }

func New(tag string, t []byte, w io.Writer) *E { return text.New(tag, t, w) }

func SortedKeys[V any](m map[string]V) []string { return text.SortedKeys[V](m) }

func SubNotAfterWord(t []byte, name, repl string) ([]byte, int) {
	return text.SubNotAfterWord(t, name, repl)
}

func Uniq(in []string) []string { return text.Uniq(in) }

func W134Pure(cond string) bool { return text.W134Pure(cond) }

func WithoutIncludes(t []byte) []byte { return text.WithoutIncludes(t) }

// Line is text.Line: the anchor most patterns are, whole C lines after their
// indentation, spelled as the C is.
func Line(lines ...string) string { return text.Line(lines...) }

// Head is text.Head: one whole C line, up to its end, for a fold's head.
func Head(line string) string { return text.Head(line) }
