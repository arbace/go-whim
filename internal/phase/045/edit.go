package p045

// Whim phase 45 (formerly 112) -- THE CASE TABLES BECOME ONE, AND IT IS THE UNION.
//
// `whim-vim.c` carried TWO complete Unicode simple-case maps and they did the same job.
// vim's own `toUpper[]`/`toLower[]` have been there since whim; phase 37 added
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
// this edit does, and what internal/phase/045/check.go then does again from this machine's
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
// not; `internal/phase/045/delta.md` gains no line, because the recorded corpus cannot see any of
// it.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).  The check needs
// the binary this phase was HANDED, to run its twelve probes on both sides.

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// tables are read as VALUES from their rows' forms, each row's four atoms
// required to be spelled as the rows this phase writes (hexadecimal ends,
// decimal step and offset: the text's "re-emitted byte for byte"); the
// merged table is the vim table's rows kept, with the musl-only codepoints'
// rows built (INITROW's BuildRows) and placed by ArrangeRows, which says
// the table's new length; the two wrappers' uses of musl_toUpper[] and
// musl_toLower[] are retargeted to toUpper[] and toLower[] (RENAME's
// RetargetAs).  The counts after are the text's own on the C view, and the
// six calls of utf_convert are asked of the edges.  The report is the text
// version's (history keeps it) but for the line counts, which are the
// text's layout and are dropped.

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim45", Edit) }

var w45Names = []string{"toUpper", "toLower", "musl_toUpper", "musl_toLower"}

type w45Rec struct{ lo, hi, step, off int }

// w45Form is a row as this phase writes it: the ends hexadecimal, the step
// and the offset decimal, a negative one `(- N)`.
func w45Form(r w45Rec) string {
	num := func(n int) string {
		if n < 0 {
			return fmt.Sprintf("(- %d)", -n)
		}
		return strconv.Itoa(n)
	}
	return fmt.Sprintf("(init 0x%x 0x%x %s %s)", r.lo, r.hi, num(r.step), num(r.off))
}

// w45Value is an element's integer: a decimal or hexadecimal atom, or
// `(- atom)`.
func w45Value(n *graph.Node) (int, bool) {
	neg := false
	if n.IsList() {
		if n.Head() != "-" || len(n.Kids) != 2 || n.Kids[1].IsList() {
			return 0, false
		}
		neg, n = true, n.Kids[1]
	}
	v, err := strconv.ParseInt(n.Atom, 0, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		v = -v
	}
	return int(v), true
}

// Edit merges each of vim's case tables with musl's: the union, read from
// the rows.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("casemap", e, w)
	incs := e.Includes()
	// The text counted `utf_convert(` six times: five calls and the
	// definition's head.  The calls are asked of the edges, before and after.
	calls0 := w45Calls(v, "utf_convert")
	v.Expect(calls0 == 5, "utf_convert is called %d times, where this phase was written against 5", calls0)
	words := func(name string) int { return edit.MentionCount(v.Text(), name) }

	defs := map[string]*graph.Node{}
	rowNodes := map[string][]*graph.Node{}
	parse := func(name string) []w45Rec {
		var d *graph.Node
		for _, x := range e.Decls(name) {
			if graph.TableInit(x) != nil {
				d = x
			}
		}
		if d == nil {
			v.Die("there is no `static convertStruct %s[]` in the input, so this phase has nothing to merge", name)
			return nil
		}
		defs[name] = d
		var rows []w45Rec
		for _, r := range graph.TableInit(d).Kids[1:] {
			var x [4]int
			ok := r.Head() == "init" && len(r.Kids) == 5
			for i := 0; ok && i < 4; i++ {
				x[i], ok = w45Value(r.Kids[i+1])
			}
			if !ok {
				v.Die("%s has a row this phase cannot read: %s", name, graph.Lisp(r))
				return nil
			}
			rec := w45Rec{x[0], x[1], x[2], x[3]}
			if w45Form(rec) != graph.Lisp(r).String() {
				v.Die("%s's row %s is not spelled as the rows this phase writes (%s), so what it writes "+
					"would not be in the file's own shape", name, graph.Lisp(r), w45Form(rec))
				return nil
			}
			rows = append(rows, rec)
			rowNodes[name] = append(rowNodes[name], r)
		}
		return rows
	}
	expand := func(name string, rows []w45Rec) map[int]int {
		out := map[int]int{}
		for _, r := range rows {
			if r.step < 0 {
				if r.lo != r.hi {
					v.Die("%s has a step < 0 row spanning more than one codepoint: "+
						"{0x%x,0x%x,%d,%d}", name, r.lo, r.hi, r.step, r.off)
					return nil
				}
				out[r.lo] = r.lo + r.off
			} else {
				for c := r.lo; c <= r.hi; c += r.step {
					out[c] = c + r.off
				}
			}
		}
		return out
	}
	ascending := func(name string, rows []w45Rec) {
		for i := 0; i+1 < len(rows); i++ {
			a, b := rows[i], rows[i+1]
			if a.hi >= b.lo {
				v.Die("%s is not ascending and non-overlapping at {0x%x,0x%x,%d,%d} / "+
					"{0x%x,0x%x,%d,%d}, and utf_convert() binary-searches on rangeEnd",
					name, a.lo, a.hi, a.step, a.off, b.lo, b.hi, b.step, b.off)
				return
			}
		}
	}

	rows := map[string][]w45Rec{}
	maps := map[string]map[int]int{}
	for _, n := range w45Names {
		rows[n] = parse(n)
		if v.Failed() {
			return v.Done()
		}
		ascending(n, rows[n])
		maps[n] = expand(n, rows[n])
		if v.Failed() {
			return v.Done()
		}
	}
	var fig []any
	for _, n := range w45Names {
		fig = append(fig, len(rows[n]), len(maps[n]))
	}
	v.Sayf("four convertStruct tables read as values and every row spelled as the rows this "+
		"phase writes -- toUpper %d rows / %d codepoints, toLower %d / %d, musl_toUpper %d / %d, "+
		"musl_toLower %d / %d -- so what this phase writes is in the file's shape by "+
		"construction and not by resemblance", fig...)

	type rep struct {
		vimN, muslN string
		nv, nm      int
		muslOnly    []int
		r0, r1      int
	}
	var report []rep
	order := map[string][]*graph.Node{}
	for _, pr := range []struct{ vimN, muslN string }{
		{"toUpper", "musl_toUpper"}, {"toLower", "musl_toLower"},
	} {
		ev, em := maps[pr.vimN], maps[pr.muslN]
		var clash []int
		for c, x := range ev {
			if m, ok := em[c]; ok && m != x {
				clash = append(clash, c)
			}
		}
		sort.Ints(clash)
		if len(clash) > 0 {
			v.Die("%s and %s map %d codepoints to DIFFERENT characters (U+%04X -> %04X / "+
				"%04X is the first), and a union is not defined there",
				pr.vimN, pr.muslN, len(clash), clash[0], ev[clash[0]], em[clash[0]])
			return v.Done()
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
					v.Die("%s already has a row covering U+%04X ({0x%x,0x%x,%d,%d}), so a "+
						"single-codepoint row for it would be unreachable",
						pr.vimN, c, r.lo, r.hi, r.step, r.off)
					return v.Done()
				}
			}
		}
		type placed struct {
			rec  w45Rec
			node *graph.Node
		}
		var all []placed
		for i, r := range rows[pr.vimN] {
			all = append(all, placed{r, rowNodes[pr.vimN][i]})
		}
		for _, c := range muslOnly {
			rec := w45Rec{c, c, -1, em[c] - c}
			built, err := e.BuildRows(defs[pr.vimN], w45Form(rec), nil)
			if err != nil || len(built) != 1 {
				v.Die("the row %s for %s could not be built: %v", w45Form(rec), pr.vimN, err)
				return v.Done()
			}
			all = append(all, placed{rec, built[0]})
		}
		sort.SliceStable(all, func(i, j int) bool { return all[i].rec.lo < all[j].rec.lo })
		var newRows []w45Rec
		for _, p := range all {
			newRows = append(newRows, p.rec)
			order[pr.vimN] = append(order[pr.vimN], p.node)
		}
		union := map[int]int{}
		for c, x := range ev {
			union[c] = x
		}
		for c, x := range em {
			union[c] = x
		}
		got := expand(pr.vimN, newRows)
		if v.Failed() {
			return v.Done()
		}
		if !w45SameMap(got, union) {
			v.Die("the merged %s does not expand to the union of the two", pr.vimN)
			return v.Done()
		}
		ascending(pr.vimN, newRows)
		if v.Failed() {
			return v.Done()
		}
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
		v.Sayf("%s: %d codepoints %s maps and %s does not -- they ARRIVE on the non-internal "+
			"arm -- and %d the other way (%s), which ARRIVE on the default one; 0 where both "+
			"map and disagree.  %d rows -> %d",
			r.vimN, r.nv, r.vimN, r.muslN, r.nm, shown, r.r0, r.r1)
	}
	for _, n := range []string{"toUpper", "toLower"} {
		if _, err := e.ArrangeRowsTyped(defs[n], order[n], graph.RowIndex{}); err != nil {
			v.Die("%s[]'s rows: %v", n, err)
			return v.Done()
		}
	}

	for _, wp := range []struct{ w, table string }{
		{"musl_towupper", "toUpper"}, {"musl_towlower", "toLower"},
	} {
		v.InFunction(wp.w, func(v *graph.Verbs) {
			m := "(return (call utf_convert a ?t (cast int (sizeof ?s))))"
			ms := v.Find(m)
			ok := len(ms) == 1
			var b graph.Bindings
			if ok {
				b, _ = graph.Match(clisp.MustPattern(m), ms[0])
				ok = b["t"].Atom == "musl_"+wp.table && b["s"].Atom == "musl_"+wp.table
			}
			if !ok {
				v.Die("%s() does not read musl_%s[] in the one shape this phase rewrites", wp.w, wp.table)
				return
			}
			for _, u := range []*graph.Node{b["t"], b["s"]} {
				if err := e.RetargetAs(u, 0, defs[wp.table]); err != nil {
					v.Die("%s()'s read of musl_%s[]: %v", wp.w, wp.table, err)
					return
				}
			}
		})
	}
	if v.Failed() {
		return v.Done()
	}
	v.Say("musl_towupper() and musl_towlower() now read toUpper[] and toLower[] -- the two " +
		"wrappers STAY, being what the non-internal arm of utf_toupper()/utf_tolower() " +
		"calls and what the two dead `if (c >= 0x100)` arms of vim_toupper()/vim_tolower() " +
		"name; deleting them is a different idea")

	for _, gone := range []string{"musl_toUpper", "musl_toLower"} {
		v.Expect(words(gone) == 1, "%s is named other than by its own definition", gone)
		v.Expect(len(v.UsesOf(gone)) == 0, "%s is still used", gone)
	}
	for _, kw := range []string{"musl_towupper", "musl_towlower"} {
		k := words(kw)
		v.Expect(k == 4, "%s has %d mentions, expected 4 -- its prototype, its definition, the one "+
			"live call and the dead one", kw, k)
	}
	for _, n := range []string{"toUpper", "toLower"} {
		k := words(n)
		v.Expect(k == 5, "%s is named %d times, expected 5 -- its definition, utf_to%s()'s "+
			"utf_convert call and the wrapper's", n, k, strings.ToLower(n[2:]))
	}
	calls := w45Calls(v, "utf_convert")
	v.Expect(calls == calls0, "utf_convert is called %d times, expected the same %d -- this phase moves no "+
		"call, it changes what two of them read", calls, calls0)
	after := e.Includes()
	host := e.Host()
	okd := len(after) == len(incs) && len(host) >= len(after)
	for i := 0; okd && i < len(after); i++ {
		okd = host[i] == after[i]
	}
	v.Expect(okd, "the include forms are no longer the %d it was handed, contiguous", len(incs))
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("musl_toUpper and musl_toLower at 1 mention each, their definitions, for the "+
		"collection, musl_towupper and musl_towlower at 4 each, toUpper and toLower at 5 each, "+
		"utf_convert at the same 5 calls, and the first include form -- the boundary -- is "+
		"still form %d with no directive above it", len(e.Core())+1)
	return v.Done()
}

func w45SameMap(a, b map[int]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, x := range a {
		if y, ok := b[k]; !ok || y != x {
			return false
		}
	}
	return true
}

// w45Calls is how many of the uses of name are a call's callee.
func w45Calls(v *graph.Verbs, name string) int {
	n := 0
	for _, u := range v.UsesOf(name) {
		if p := v.Editor().Parent(u); p != nil && p.Head() == "call" && p.Kids[1] == u {
			n++
		}
	}
	return n
}
