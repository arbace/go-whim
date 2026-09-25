package p112

// Whim phase 112 -- THE CASE TABLES BECOME ONE, AND IT IS THE UNION.
//
// `whim-vim.c` carried TWO complete Unicode simple-case maps and they did the same job.
// vim's own `toUpper[]`/`toLower[]` have been there since whim; phase 98 added
// musl's as `musl_toUpper[]`/`musl_toLower[]`, range-compressed into the same
// `convertStruct` shape and read by the same `utf_convert()`, so that `towupper` and
// `towlower` could leave `nm -u`.  Which of the two the editor consults is decided by
// `'casemap'`: with `internal` set it reads vim's, without it reads musl's.  A core with
// no C library has nothing to choose between, so this phase makes it ONE table -- and
// the table is the UNION.
//
// THE SURVEY THAT PROPOSED THIS SAID THE TWO "DIFFER ON 2 OF 5 PROBES", which was
// accurate about its probe set's reach and says nothing about the truth: five characters
// cannot see 193 codepoints.  Expanded over the whole of 0..0x10FFFF -- which is what
// this edit does, and what internal/phase/112/check.go then does again from this machine's
// libc -- the two disagree at 97 upper codepoints and 96 lower, at NONE of which both
// map to different characters, and the split is lopsided:
//
// * vim maps and musl does not, 96 upper and 96 lower: all of Vithkuqi (U+10570..,
// U+10597..), all of Garay (U+10D50.., U+10D70..), the enclosed Latin letters
// U+24B6..U+24CF and U+24D0..U+24E9, Glagolitic U+2C2F/U+2C5F, the recent Latin
// Extended-D additions (U+A7C0 U+A7C1 U+A7C7 U+A7C8 ...), U+019B, U+0264, U+1C89 and
// U+1C8A.  vim's table is simply NEWER -- it knows Unicode 14's Vithkuqi and Unicode
// 16's Garay, and musl's casemap.h predates both.
// * musl maps and vim does not, EXACTLY ONE: U+00DF -> U+1E9E, the sharp s.
//
// So "delete musl's and use vim's" would lose the sharp s on the non-internal arm and
// "use musl's" would lose ninety-six.  Each table knew something the other did not, and
// the union is the only answer that keeps both.  IT IS COMPUTED HERE AND NOT WRITTEN
// DOWN: the edit expands both tables, refuses on a codepoint they map differently,
// requires that no existing row covers one it is about to insert -- `utf_convert()`
// binary-searches on `rangeEnd`, so a row inside another row is unreachable -- inserts a
// `{c,c,-1,offset}` row at its sorted place, and then re-expands and requires the result
// to be exactly the union, ascending and non-overlapping.
//
// THE ONE ROW IS A DELIBERATE DIVERGENCE FROM UNICODE AND THE USER TOOK IT KNOWINGLY.
// Unicode's SIMPLE uppercase of U+00DF is U+00DF; U+1E9E is musl's tailoring, and
// putting it into vim's own table changes the DEFAULT `'casemap'`, not only the vendored
// arm: `:s/.*/\U&/` on `ß` now draws `ẞ` where it drew `ß`.  What it buys is that the
// file stops contradicting itself.  `swapchar()` has hard-coded `ß -> ẞ` for `gU`, `g~`
// and `~` all along, so today the table and the keystroke give different answers for the
// same character; after this phase they agree.
//
// WHAT IT DOES, in three parts, every one of them computed:
//
// A  the union, INSERTED INTO VIM'S ROWS, as above.
// B  `musl_toUpper[]` and `musl_toLower[]`, read by nothing after C, which the
// sweep deletes.
// C  `musl_towupper()` and `musl_towlower()` repointed at `toUpper[]`/`toLower[]`.
// The two three-line wrappers STAY: they are what the non-internal arm of
// `utf_toupper()`/`utf_tolower()` calls and what the two dead `if (c >= 0x100)`
// arms of `vim_toupper()`/`vim_tolower()` name, and deleting them is a different
// idea.
//
// THE FORMAT IS THE FILE'S, PROVEN AND NOT ASSUMED.  Every one of the four tables is
// parsed and re-emitted before anything is changed, and the edit refuses unless the
// re-emission is byte-identical to the text it came from.  So the rows this phase writes
// are in `tools/canon.sh`'s shape by construction rather than by resemblance.
//
// THE DELTA IS REAL AND IT RUNS ON BOTH ARMS, which is the thing a reader gets wrong
// twice over.  On the NON-INTERNAL arm -- `:set casemap=` or `casemap=keepascii`, which
// read musl's table and now read the union -- 96 upper and 96 lower codepoints gain a
// mapping they never had, and the sharp s keeps the one it had.  On the DEFAULT arm,
// which reads vim's table, the single row arrives.  Six probe sessions move and six do
// not; `internal/phase/112/delta.md` gains no line, because the recorded corpus cannot see any of
// it.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).  The check needs
// the binary this phase was HANDED, to run its twelve probes on both sides.

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim112", Edit) }

// A convertStruct row is canonical text, as every table in the file is: four
// spaces of indent and a space after each comma.
var w112Row = regexp.MustCompile(`^    \{(0x[0-9a-f]+), (0x[0-9a-f]+), (-?\d+), (-?\d+)\},?$`)

const w112RowFormat = "    {0x%x, 0x%x, %d, %d}"

// w112Names are the four convertStruct tables this phase merges into two.
var w112Names = []string{"toUpper", "toLower", "musl_toUpper", "musl_toLower"}

type w112Rec struct{ lo, hi, step, off int }

// Whim112 makes the case tables one, and it is the UNION: vim's toUpper[]/toLower[]
// and the musl_to*[] phase 98 vendored disagreed at 97 upper and 96 lower
// codepoints -- vim's newer by ninety-six and musl's knowing `ß -> ẞ` alone --
// and a core with no C library has nothing for 'casemap' to choose between.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "casemap", W: w}
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 168 drops the unused)
	t := string(text)
	before := strings.Count(t, "\n")

	words := func(s, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(s, -1))
	}
	block := func(name string) ([]int, error) {
		m := regexp.MustCompile(`(?ms)^static convertStruct ` + name + `\[\] =\n\{\n(.*?)\n\};\n`).
			FindStringSubmatchIndex(t)
		if m == nil {
			return nil, p.Die("there is no `static convertStruct %s[]` in the input, so this phase has "+
				"nothing to merge", name)
		}
		return m, nil
	}
	parse := func(name, Body string) ([]w112Rec, error) {
		var rows []w112Rec
		for _, line := range strings.Split(Body, "\n") {
			m := w112Row.FindStringSubmatch(line)
			if m == nil {
				return nil, p.Die("%s has a row this phase cannot read: %s", name, cutil.PyRepr(line))
			}
			lo, _ := strconv.ParseInt(m[1][2:], 16, 64)
			hi, _ := strconv.ParseInt(m[2][2:], 16, 64)
			step, _ := strconv.Atoi(m[3])
			off, _ := strconv.Atoi(m[4])
			rows = append(rows, w112Rec{int(lo), int(hi), step, off})
		}
		return rows, nil
	}
	// The canonical text ends every row of a table with a comma, the last one
	// included.
	emit := func(rows []w112Rec) string {
		Out := make([]string, len(rows))
		for i, r := range rows {
			Out[i] = fmt.Sprintf(w112RowFormat, r.lo, r.hi, r.step, r.off)
		}
		return strings.Join(Out, ",\n") + ","
	}
	// expand is {codepoint: target}, EXACTLY as utf_convert() reads the row.  A
	// row with `step < 0` is how this file spells a single codepoint, and it
	// works because `(a - lo) % step` is 0 for every a when step is -1.
	expand := func(name string, rows []w112Rec) (map[int]int, error) {
		Out := map[int]int{}
		for _, r := range rows {
			if r.step < 0 {
				if r.lo != r.hi {
					return nil, p.Die("%s has a step < 0 row spanning more than one codepoint: "+
						"{0x%x,0x%x,%d,%d}", name, r.lo, r.hi, r.step, r.off)
				}
				Out[r.lo] = r.lo + r.off
			} else {
				for c := r.lo; c <= r.hi; c += r.step {
					Out[c] = c + r.off
				}
			}
		}
		return Out, nil
	}
	ascending := func(name string, rows []w112Rec) error {
		for i := 0; i+1 < len(rows); i++ {
			a, b := rows[i], rows[i+1]
			if a.hi >= b.lo {
				return p.Die("%s is not ascending and non-overlapping at {0x%x,0x%x,%d,%d} / "+
					"{0x%x,0x%x,%d,%d}, and utf_convert() binary-searches on rangeEnd",
					name, a.lo, a.hi, a.step, a.off, b.lo, b.hi, b.step, b.off)
			}
		}
		return nil
	}

	// ---- the four tables, parsed and PROVEN to re-emit as the text they came from
	blocks := map[string][]int{}
	rows := map[string][]w112Rec{}
	maps := map[string]map[int]int{}
	for _, n := range w112Names {
		m, err := block(n)
		if err != nil {
			return nil, err
		}
		blocks[n] = m
		r, err := parse(n, t[m[2]:m[3]])
		if err != nil {
			return nil, err
		}
		rows[n] = r
		if emit(r) != t[m[2]:m[3]] {
			return nil, p.Die("%s does not re-emit as the text it came from, so the rows this phase "+
				"writes would not be in the file's own shape", n)
		}
		if err := ascending(n, r); err != nil {
			return nil, err
		}
		e, err := expand(n, r)
		if err != nil {
			return nil, err
		}
		maps[n] = e
	}
	var fig []interface{}
	for _, n := range w112Names {
		fig = append(fig, len(rows[n]), len(maps[n]))
	}
	p.Sayf("four convertStruct tables read and re-emitted BYTE FOR BYTE as the text they came "+
		"from -- toUpper %d rows / %d codepoints, toLower %d / %d, musl_toUpper %d / %d, "+
		"musl_toLower %d / %d -- so what this phase writes is in the file's shape by "+
		"construction and not by resemblance", fig...)

	// ---- A. the union, computed -----------------------------------------------
	merged := map[string][]w112Rec{}
	type rep struct {
		vimN, muslN string
		nv, nm      int
		muslOnly    []int
		r0, r1      int
	}
	var report []rep
	for _, pr := range []struct{ vimN, muslN string }{
		{"toUpper", "musl_toUpper"}, {"toLower", "musl_toLower"},
	} {
		ev, em := maps[pr.vimN], maps[pr.muslN]
		var clash []int
		for c, v := range ev {
			if m, ok := em[c]; ok && m != v {
				clash = append(clash, c)
			}
		}
		sort.Ints(clash)
		if len(clash) > 0 {
			return nil, p.Die("%s and %s map %d codepoints to DIFFERENT characters (U+%04X -> %04X / "+
				"%04X is the first), and a union is not defined there",
				pr.vimN, pr.muslN, len(clash), clash[0], ev[clash[0]], em[clash[0]])
		}
		var vimOnly, muslOnly []int
		for c := range ev {
			if _, ok := em[c]; !ok {
				vimOnly = append(vimOnly, c)
			}
		}
		for c := range em {
			if _, ok := ev[c]; !ok {
				muslOnly = append(muslOnly, c)
			}
		}
		sort.Ints(vimOnly)
		sort.Ints(muslOnly)
		for _, c := range muslOnly {
			for _, r := range rows[pr.vimN] {
				if r.lo <= c && c <= r.hi {
					return nil, p.Die("%s already has a row covering U+%04X ({0x%x,0x%x,%d,%d}), so a "+
						"single-codepoint row for it would be unreachable",
						pr.vimN, c, r.lo, r.hi, r.step, r.off)
				}
			}
		}
		newRows := append([]w112Rec{}, rows[pr.vimN]...)
		for _, c := range muslOnly {
			newRows = append(newRows, w112Rec{c, c, -1, em[c] - c})
		}
		sort.SliceStable(newRows, func(i, j int) bool { return newRows[i].lo < newRows[j].lo })
		union := map[int]int{}
		for c, v := range ev {
			union[c] = v
		}
		for c, v := range em {
			union[c] = v
		}
		got, err := expand(pr.vimN, newRows)
		if err != nil {
			return nil, err
		}
		if !w112SameMap(got, union) {
			return nil, p.Die("the merged %s does not expand to the union of the two", pr.vimN)
		}
		if err := ascending(pr.vimN, newRows); err != nil {
			return nil, err
		}
		merged[pr.vimN] = newRows
		report = append(report, rep{pr.vimN, pr.muslN, len(vimOnly), len(muslOnly),
			muslOnly, len(rows[pr.vimN]), len(newRows)})
	}
	for _, r := range report {
		only := make([]string, len(r.muslOnly))
		for i, c := range r.muslOnly {
			only[i] = fmt.Sprintf("U+%04X", c)
		}
		shown := strings.Join(only, " ")
		if shown == "" {
			shown = "none"
		}
		p.Sayf("%s: %d codepoints %s maps and %s does not -- they ARRIVE on the non-internal "+
			"arm -- and %d the other way (%s), which ARRIVE on the default one; 0 where both "+
			"map and disagree.  %d rows -> %d",
			r.vimN, r.nv, r.vimN, r.muslN, r.nm, shown, r.r0, r.r1)
	}
	for _, n := range []string{"toUpper", "toLower"} {
		oldText := t[blocks[n][0]:blocks[n][1]]
		if strings.Count(t, oldText) != 1 {
			return nil, p.Die("%s[] is not in the file exactly once", n)
		}
		t = strings.Replace(t, oldText,
			fmt.Sprintf("static convertStruct %s[] =\n{\n%s\n};\n", n, emit(merged[n])), 1)
	}

	// ---- B. musl's two tables are the sweep's -------------------------------
	// Once C repoints the two wrappers, nothing reads musl_toUpper[] or
	// musl_toLower[], and the sweep deletes both.

	// ---- C. the two wrappers, repointed ---------------------------------------
	for _, wp := range []struct{ W, table string }{
		{"musl_towupper", "toUpper"}, {"musl_towlower", "toLower"},
	} {
		oldCall := fmt.Sprintf("    return utf_convert(a, musl_%s, (int)sizeof(musl_%s));\n", wp.table, wp.table)
		newCall := fmt.Sprintf("    return utf_convert(a, %s, (int)sizeof(%s));\n", wp.table, wp.table)
		if strings.Count(t, oldCall) != 1 {
			return nil, p.Die("%s() does not read musl_%s[] in the one shape this phase rewrites",
				wp.W, wp.table)
		}
		t = strings.ReplaceAll(t, oldCall, newCall)
	}
	p.Say("musl_towupper() and musl_towlower() now read toUpper[] and toLower[] -- the two " +
		"wrappers STAY, being what the non-internal arm of utf_toupper()/utf_tolower() " +
		"calls and what the two dead `if (c >= 0x100)` arms of vim_toupper()/vim_tolower() " +
		"name; deleting them is a different idea")

	// ---- what must be true of the result --------------------------------------
	for _, gone := range []string{"musl_toUpper", "musl_toLower"} {
		if words(t, gone) != 1 {
			return nil, p.Die("%s is named other than by its own definition", gone)
		}
	}
	for _, kw := range []struct {
		keep string
		want int
	}{{"musl_towupper", 4}, {"musl_towlower", 4}} {
		if k := words(t, kw.keep); k != kw.want {
			return nil, p.Die("%s has %d mentions, expected %d -- its prototype, its definition, the one "+
				"live call and the dead one", kw.keep, k, kw.want)
		}
	}
	for _, n := range []string{"toUpper", "toLower"} {
		if k := words(t, n); k != 5 {
			return nil, p.Die("%s is named %d times, expected 5 -- its definition, utf_to%s()'s "+
				"utf_convert call and the wrapper's", n, k, strings.ToLower(n[2:]))
		}
	}
	if k := edit.CoreCalls([]byte(t), "utf_convert"); k != 6 {
		return nil, p.Die("utf_convert is called %d times, expected the same 6 -- this phase moves no "+
			"call, it changes what two of them read", k)
	}
	L := strings.Split(t, "\n")
	var directives []int
	for i, l := range L {
		if strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
			directives = append(directives, i)
		}
	}
	inc := regexp.MustCompile(`^ *# *include `)
	okd := len(directives) == nInc
	for _, i := range directives {
		if !inc.MatchString(L[i]) {
			okd = false
		}
	}
	if !okd {
		return nil, p.Die("the directives are no longer the %d #includes it was handed and nothing else", nInc)
	}
	p.Sayf("musl_toUpper and musl_toLower at 1 mention each, their definitions, for the "+
		"sweep, musl_towupper and musl_towlower at "+
		"4 each, toUpper and toLower at 5 each, utf_convert at the same 6 calls, and the "+
		"first #include -- the boundary -- is still line %d with no directive "+
		"above it", directives[0]+1)
	p.Sayf("the file is %d lines and the input was %d", strings.Count(t, "\n"), before)
	return []byte(t), nil
}

func w112SameMap(a, b map[int]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}
