package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

var (
	// A printf conversion: % then flags, width, precision, then the letter.
	floatConv = regexp.MustCompile(`%[-+ #0']*[0-9*]*(?:\.[0-9*]*)?[fFeEgG]`)
	// A C string literal, escapes included.
	cLiteral = regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"`)
	// Terminfo capability strings use % as an operator language of their own
	// -- %p1 pushes a parameter, %{1} a constant, %? %t %e %; are its
	// conditional.  `\033[?1006;1000%?%p1%{1}%=%th%el%;` is not a printf
	// format and its %e is an `else`.  Raw text is worse still:
	// `indent % get_sw_value(curbuf)` is C.
	terminfo    = regexp.MustCompile(`%[p{?;]|%t[^a-zA-Z]`)
	looseFormat = regexp.MustCompile(`vim_v?snprintf[_a-z]*\([^,]*,[^,]*, *[A-Za-z_][\w>.\-]*[,)]`)
	libmCall    = regexp.MustCompile(`\b(?:ceil|floor|log10)\s*\(`)
)

// nofloatCuts are TYPE_FLOAT's three arms and the walker's six labels: case
// labels, each dropped with the run it heads alone (DropCase), in the
// function each is in.
var nofloatCuts = []struct {
	what, fn string
	labels   []string
}{
	{"format_typeof's float arm", "format_typeof", floatLetters},
	{"the argument walker's six labels", "parse_fmt_types", floatLetters},
	{"format_typename's float arm", "format_typename", []string{"(case TYPE_FLOAT)"}},
	{"the va_arg walker's float arm", "skip_to_arg", []string{"(case TYPE_FLOAT)"}},
}

// floatLetters are the six conversions' case labels.
var floatLetters = []string{"(case 'f')", "(case 'F')", "(case 'e')", "(case 'E')", "(case 'g')", "(case 'G')"}

// checkNoFloatFormats refuses to cut unless nothing can reach the branch being
// cut.
//
// SCANNING EVERY LITERAL IS THE COMPLETE CHECK, and that is worth saying
// because twelve call sites pass a format that is not a literal.  None of them
// CONSTRUCTS one: smsg() and semsg() forward the format parameter they were
// given, vim_snprintf() forwards to vim_vsnprintf(), and the three remaining
// locals are assigned from literals a few lines above.  So every format that
// can reach the branch originates as a literal in this file, and every literal
// in this file has just been read.
func checkNoFloatFormats(text []byte, w io.Writer) error {
	var bad []string
	lits := cLiteral.FindAllIndex(text, -1)
	for _, m := range lits {
		lit := text[m[0]:m[1]]
		if terminfo.Match(lit) {
			continue
		}
		if floatConv.Match(lit) {
			line := bytes.Count(text[:m[0]], []byte{'\n'}) + 1
			s := string(lit)
			if len(s) > 70 {
				s = s[:70]
			}
			bad = append(bad, fmt.Sprintf("%d: %s", line, s))
		}
	}
	if len(bad) > 0 {
		show := bad
		if len(show) > 5 {
			show = show[:5]
		}
		return fmt.Errorf("nofloat: %d string literals carry a float conversion, so the "+
			"%%f branch IS reachable and must not be removed:\n    %s",
			len(bad), strings.Join(show, "\n    "))
	}
	fmt.Fprintf(w, "  nofloat      no float conversion in any of %d string literals; the "+
		"%d forwarded formats all originate in one\n",
		len(lits), len(looseFormat.FindAll(text, -1)))
	return nil
}

// NoFloat removes the float conversion nothing can reach, and the libm calls
// that were the only other floating point in the file.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the conversion is the run of
// items from its first label through its block, replaced by nothing
// (Splice), in vim_vsnprintf_typval; the arms are case labels dropped
// (DropCase); the string literals and the libm calls are counted on the C
// view by the text version's own expressions, so the numbers are its
// numbers (history keeps it).  The arms' acts report nothing of their own,
// as the text's did not: one line says all four.
func NoFloat(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nofloat", e, w)
	if err := checkNoFloatFormats(v.Text(), w); err != nil {
		return err
	}

	// The fuzzy matcher, whose score rounding called ceil and floor, went
	// with every completion context but files at phase 4 (whim4f, phase 4f's
	// program, which runs before this phase now).

	// The conversion: its six labels and the block they share, the run
	// that ends where `default:` begins.  It is the one that calls log10.
	quiet := graph.NewVerbs("nofloat", e, io.Discard)
	lines := 0
	quiet.InFunction("vim_vsnprintf_typval", func(q *graph.Verbs) {
		block := "(block (def f double) _*)"
		if q.One(block, "the float conversion case") == nil {
			return
		}
		q.Expect(q.Count("(call log10 _*)") > 0, "the float conversion case is not where this expects")
		before := bytes.Count(q.Text(), []byte{'\n'})
		q.Splice("(case 'f')", block, "", "the float conversion case")
		lines = before - bytes.Count(q.Text(), []byte{'\n'})
	})
	if err := quiet.Done(); err != nil {
		return err
	}
	v.Sayf("the %%f conversion, %d lines nothing can reach", lines)

	for _, c := range nofloatCuts {
		quiet.InFunction(c.fn, func(q *graph.Verbs) {
			for _, l := range c.labels {
				q.DropCase(l, 1, c.what)
			}
		})
	}
	if err := quiet.Done(); err != nil {
		return err
	}

	// TYPE_FLOAT itself and typename_float, now unreferenced, are the
	// collection's.
	v.Say("TYPE_FLOAT's three arms")

	v.Sayf("%d libm calls left", v.TextCount(libmCall.String()))
	return v.Done()
}
