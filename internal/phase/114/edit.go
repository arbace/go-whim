package p114

// Whim phase 114 -- abs and labs, the two the core took on trust.
// See GOALS.md II.4c, and GOALS.md.
//
// THE CORE IS OPTIMISED FOR TRANSPILATION, NOT FOR PERFORMANCE, AND SO IT MAY NOT
// DEPEND ON LATENT COMPILER BEHAVIOUR (GOALS.md II.4c, the user's rule of
// 2026-09-19).  This phase is the first application of it, and it is the reason the
// phase exists at all -- because by every number this pipeline usually reports, it
// does nothing.
//
// `abs` and `labs` are CALLED by the core, at three sites, and appear in `nm -u`
// ZERO times.  gcc lowers both to inline arithmetic -- measured on the input, `gcc
// -S` of the whole file contains not one mention of either name -- and NOTHING IN
// THE LANGUAGE PROMISES THAT.  A compiler that emitted the calls the source
// literally asks for would silently have added two libc symbols to a file whose
// whole claim is the shortness of that list.  So this phase FREES NO SYMBOL and must
// say so as an equality rather than let a reader expect a vendoring phase to move
// the count: what it removes is a dependence on behaviour nothing states.
//
// WHAT IT DOES, in three parts:
//
// the two prototypes    `long labs(long n);` and `int abs(int n);`, which
// phase 109 wrote into the core's block of libc declarations
// when the headers were still above it.  The block loses two
// of its entries and nothing else changes in it.
// the three call sites  8152 `musl_labs((long)get_cursor_rel_lnum(...))` in the
// number column, 41824 `musl_labs(curwin->w_topline -
// prev_topline)` in scroll_with_sms, and 77379
// `musl_abs(wp->w_height - wp->w_prev_height)` in
// last_status_rec.  The line numbers are where they were
// when this was written and nothing below depends on them.
// the two definitions   `static musl_abs` and `static musl_labs`, in the `musl_`
// block phases 97 and 98 built, immediately above
// `musl_bsearch` so that the four `<stdlib.h>` scalar
// functions the core owns -- musl_atoi, musl_atol, musl_abs,
// musl_labs -- sit together and above every use.
//
// MUSL'S SPELLING IS COPIED AND NOT IMPROVED, which is phase 97's rule applied to
// two more functions.  /root/musl/src/stdlib/abs.c and labs.c are one line each:
//
// int abs(int a) { return a>0 ? a : -a; }
// long labs(long a) { return a>0 ? a : -a; }
//
// `a > 0 ? a : -a` and `a < 0 ? -a : a` ARE THE SAME FUNCTION, and the check proves
// it twice rather than arguing it: the two spellings compile to BYTE-IDENTICAL
// machine code at -O0 and at -O2, and they agree at every one of the 4,294,967,296
// `int` values.  Having established that, the rule is to write what musl writes.
//
// AND THE UNDEFINED BEHAVIOUR IS COPIED WITH IT.  `-a` overflows at `INT_MIN` and at
// `LONG_MIN`, so `musl_abs(INT_MIN)` is undefined -- and so is `abs(INT_MIN)`, and so
// is musl's own `abs`, by exactly the same expression.  THE VENDORED PAIR IS
// FAITHFUL RATHER THAN SAFER, deliberately: a phase that quietly made the core's
// arithmetic differ from the libc it is replacing would be a behaviour change
// wearing a vendoring phase's clothes.  What the check does instead is measure
// whether the three call sites can reach the value, and the answer is no in one case
// by a clamp in this file and in the other two by the arithmetic of line numbers.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check needs it for
// more than a comparison: the corpus CANNOT SEE any of the three call sites (measured
// -- an instrumented build enters none of them in 106 records), so this phase owes
// probes of its own, and those probes are run on the binary this phase was handed and
// on its own and required to draw the same screen.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim114", Edit) }

var (
	w114Inc  = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)
	w114Word = regexp.MustCompile(`\b(labs|abs)\b`)
	w114Any  = regexp.MustCompile(`\b(abs|labs)\b`)
)

// w114Defs is /root/musl/src/stdlib/abs.c and labs.c, whole, WRITTEN THE WAY THIS
// FILE WRITES A FUNCTION -- the name at column 0 on a line of its own, which is
// what funcreach.py reads a definition by.  MUSL'S TERNARY IS COPIED AND NOT
// TURNED ROUND: `a < 0 ? -a : a` is the same function, so there is nothing to
// gain and one more difference from the source of record to explain.
const w114Defs = `    static int
musl_abs(int a)
{
    return a > 0 ? a : -a;
}

    static long
musl_labs(long a)
{
    return a > 0 ? a : -a;
}

`

const w114Anchor = "    static void *\nmusl_bsearch("

// Whim114 vendors abs and labs -- called by the core, never in `nm -u` because
// gcc lowers both to inline arithmetic, so the phase's whole value is that the
// core stops depending on behaviour nothing states.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 169 drops the unused)
	p := edit.Ph{Tag: "arith", W: w}

	mentions := edit.MentionCount
	directives := func(t []byte) ([]int, []string) {
		lines := strings.Split(string(t), "\n")
		var d []int
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
				d = append(d, i)
			}
		}
		return d, lines
	}
	// contiguous: one block, with nothing but blank lines between its lines --
	// the canonical print puts a blank line between two declarations.
	contiguous := func(ls []string, d []int) bool {
		for i := 1; i < len(d); i++ {
			for j := d[i-1] + 1; j < d[i]; j++ {
				if strings.TrimSpace(ls[j]) != "" {
					return false
				}
			}
		}
		return len(d) > 0
	}
	at := func(d []int, n int) string {
		Out := make([]string, 0, n)
		for i := 0; i < len(d) && i < n; i++ {
			Out = append(Out, strconv.Itoa(d[i]+1))
		}
		return strings.Join(Out, " ")
	}

	beforeLines := strings.Count(string(text), "\n")

	// ---- 0. the file this edit was written against ----------------------------
	// ELEVEN DIRECTIVES, contiguous, every one an `#include` of a system header --
	// and since phase 110 THEY ARE NOT AT THE TOP.  The first of them is the
	// boundary between the core and the host, so everything this phase writes
	// must land ABOVE it.
	d, lines := directives(text)
	if len(d) != nInc || !contiguous(lines, d) {
		return nil, p.Die("the file does not have exactly eleven contiguous preprocessor directives: "+
			"%d at %s", len(d), at(d, 4))
	}
	for _, i := range d {
		if !w114Inc.MatchString(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	cut := d[0]
	for _, name := range []string{"musl_abs", "musl_labs"} {
		if k := mentions(text, name); k != 0 {
			return nil, p.Die("`%s` already occurs %d times -- this phase introduces it, so an existing "+
				"mention means the phase has already run or the name is taken", name, k)
		}
	}
	p.Sayf("eleven contiguous `#include <...>` directives, the first at line %d and the "+
		"boundary between the core and the host; `musl_abs` and `musl_labs` at zero", cut+1)

	// ---- 1. the two prototypes, found as a BLOCK rather than by line number ---
	firstStatic := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "    static") {
			firstStatic = i
			break
		}
	}
	if firstStatic < 0 {
		return nil, p.Die("there is no `    static` line, so the declaration block has no end")
	}
	var proto []int
	for i, l := range lines[:firstStatic] {
		if l == "" || l[0] == ' ' || l[0] == '\t' {
			continue
		}
		if !strings.HasSuffix(l, ";") || !strings.Contains(l, "(") {
			continue
		}
		if strings.HasPrefix(l, "enum") || strings.HasPrefix(l, "typedef") || strings.HasPrefix(l, "static") {
			continue
		}
		proto = append(proto, i)
	}
	if len(proto) == 0 || !contiguous(lines, proto) {
		return nil, p.Die("the core's libc declarations are not one contiguous block above the first "+
			"`static`: %d lines at %s", len(proto), at(proto, 4))
	}
	block := make([]string, len(proto))
	for i, j := range proto {
		block[i] = lines[j]
	}
	for _, line := range []string{"long labs(long n);", "int abs(int n);"} {
		if w114Count(block, line) != 1 {
			return nil, p.Die("`%s` is not in the core's libc declaration block exactly once -- the "+
				"block is: %s", line, strings.Join(block, " | "))
		}
		if strings.Count(string(text), line+"\n") != 1 {
			return nil, p.Die("`%s` is not a line of its own exactly once in the whole file", line)
		}
		text = []byte(strings.Replace(string(text), line+"\n", "", 1))
	}
	p.Sayf("the core declares %d libc functions above the first `static` and will declare "+
		"%d: `long labs(long n);` and `int abs(int n);` go, and they are the two the "+
		"core asked libc for and never got", len(block), len(block)-2)

	// ---- 2. the three call sites, by a literal-aware single pass --------------
	// CLAUDE.md, *Rename a name across the whole file*: literal-aware, because a
	// name in a string is DATA, and single-pass, because a literal span is an
	// OFFSET and every offset after the first replacement is wrong.
	// `\babs\b` does not match inside `musl_abs`: `_` is a word character.
	spans, err := edit.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	var holding []string
	for _, s := range spans {
		if w114Any.Match(text[s[0]:s[1]]) {
			holding = append(holding, string(text[s[0]:s[1]]))
		}
	}
	if len(holding) > 0 {
		return nil, p.Die("a string or character literal mentions `abs` or `labs`, so a rename would "+
			"change what the editor PRINTS: %s", strings.Join(edit.First(holding, 3), " / "))
	}
	inSpan := func(off int) bool {
		lo, hi := 0, len(spans)
		for lo < hi {
			mid := (lo + hi) / 2
			if spans[mid][0] <= off {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		k := lo - 1
		return k >= 0 && spans[k][0] <= off && off < spans[k][1]
	}
	var Out strings.Builder
	last := 0
	count := map[string]int{}
	for _, m := range w114Word.FindAllSubmatchIndex(text, -1) {
		if inSpan(m[0]) {
			continue
		}
		name := string(text[m[2]:m[3]])
		Out.Write(text[last:m[0]])
		Out.WriteString("musl_" + name)
		last = m[1]
		count[name]++
	}
	Out.Write(text[last:])
	text = []byte(Out.String())
	if count["labs"] != 2 || count["abs"] != 1 {
		return nil, p.Die("the rename reached %d `labs` and %d `abs`, and this phase was measured on 2 "+
			"and 1 -- the two prototypes are already gone, so what is left is exactly the "+
			"call sites", count["labs"], count["abs"])
	}
	p.Sayf("the three call sites are the core's own now: two `labs` -- the number column's "+
		"relative line number and scroll_with_sms's topline difference -- and one `abs`, "+
		"last_status_rec's window-height difference.  One pass, outside every literal, "+
		"and %d literals were scanned and none mentions either name", len(spans))

	// ---- 3. the two definitions, copied from musl -----------------------------
	if strings.Count(string(text), w114Anchor) != 1 {
		return nil, p.Die("`musl_bsearch`'s definition is not in the file exactly once, so there is no " +
			"unambiguous place for these two: they belong with musl_atoi and musl_atol, " +
			"the other <stdlib.h> functions the core owns")
	}
	text = []byte(strings.Replace(string(text), w114Anchor, w114Defs+w114Anchor, 1))

	// ---- 4. what the file is now ----------------------------------------------
	d, lines = directives(text)
	if len(d) != nInc || !contiguous(lines, d) {
		return nil, p.Die("the eleven directives are no longer eleven contiguous lines")
	}
	for _, nw := range []struct {
		Name string
		want int
	}{{"abs", 0}, {"labs", 0}, {"musl_abs", 2}, {"musl_labs", 3}} {
		if k := mentions(text, nw.Name); k != nw.want {
			return nil, p.Die("`%s` has %d mentions after the cut, expected %d", nw.Name, k, nw.want)
		}
	}
	for _, name := range []string{"musl_abs", "musl_labs"} {
		var where []int
		for i, l := range lines {
			if strings.HasPrefix(l, name+"(") {
				where = append(where, i)
			}
		}
		if len(where) != 1 {
			return nil, p.Die("`%s` is not defined by exactly one line beginning at column 0, which is "+
				"how tools/funcreach.py reads a definition", name)
		}
		if where[0] > d[0] {
			return nil, p.Die("`%s` is defined BELOW the first `#include`, which is the boundary: it is "+
				"core code and every one of its callers is above the line", name)
		}
	}
	if n := strings.Count(string(text), "\n"); n != beforeLines+10 {
		return nil, p.Die("the file is %d lines and the input was %d -- expected exactly ten more, the "+
			"two five-line definitions and their two blank lines less the two prototypes",
			n, beforeLines)
	}
	p.Sayf("`abs` and `labs` are at 0 mentions in the whole file, `musl_abs` at 2 and "+
		"`musl_labs` at 3 -- a definition and its calls -- both defined at column 0 above "+
		"the boundary, %d -> %d lines and the blank-line runs exactly as before",
		beforeLines, strings.Count(string(text), "\n"))
	return text, nil
}

func w114Count(ss []string, v string) int {
	n := 0
	for _, s := range ss {
		if s == v {
			n++
		}
	}
	return n
}
