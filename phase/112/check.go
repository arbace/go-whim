package p112

// Whim phase 112, the check -- THE CASE TABLES BECOME ONE, AND IT IS THE UNION.
// See phase/112/edit.go, and GOALS.md.
//
// Runs after phase/112/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old.c`, the source this phase was HANDED,
// and `old`, that source built with the boundary's own flags.
//
// WHAT IS CLAIMED, in five parts:
//
// THE UNION     THE STRONGEST THING THIS CHECK SAYS, and it is not a row count.  The
// produced `toUpper[]`/`toLower[]` are expanded over all 1,114,112
// codepoints and required to be EXACTLY the union of the two tables the
// phase was handed, in all three directions: they agree with the INPUT's
// vim table wherever it mapped, with the INPUT's musl table wherever IT
// mapped, and they map nothing that neither did.  A row count cannot say
// any of that, and a merge that dropped Vithkuqi would pass one.
// Perturbing one produced row must break it, so it is proven able to
// fail.
// THE AUTHORITY the musl half is re-derived from THIS MACHINE'S libc through ctypes
// (muslcase's `libc()`), not read out of the table the phase
// deleted.  So the check does not trust the bytes phase 98 shipped
// either.
// THE RULE      nothing this phase changes on the DEFAULT arm may go un-probed, and
// nothing it stops mapping may go un-probed either.  Both sets are
// COMPUTED from the two input tables -- the first is what the union adds
// to vim's own table, the second is what it takes away -- and every
// member of both must appear in the probe text.  That stays true of an
// input this check has never seen, where the number 1 would not.
// THE SOURCE    musl_toUpper and musl_toLower at 0 mentions with the input at 3 each,
// so the assertion is one that can fail; the two wrappers still there at
// 4 mentions and reading vim's tables; utf_convert at the same six
// calls; the boundary still the first of eleven #includes, and the cut
// compiling silently with the SAME thirteen names, read at run time.
// SYMBOLS       `nm -u` is THE SAME SET, as a `comm` empty in both directions, and
// `main` is still the only external symbol.  Changing data frees no libc
// symbol and needs none; the eleven phase 98 freed stay freed.
//
// THE DELTA RUNS ON BOTH ARMS, and getting that wrong is the easiest mistake here.
// `utf_toupper()`/`utf_tolower()` read musl's table whenever `internal` is NOT in
// `'casemap'`, and that arm now reads the union -- so the 96 upper and 96 lower
// codepoints vim knows and musl did not ARRIVE there, and the sharp s KEEPS the mapping
// it had.  The DEFAULT arm reads vim's own table, and the one row the union adds arrives
// there: `:s/.*/\U&/` on `ß` draws `ẞ` where it drew `ß`.  Six probe sessions move and
// six do not.
//
// TWO TRAPS THE PROBES HAD TO GET RIGHT, both measured rather than reasoned.
// `gU`, `g~` and `~` CANNOT show the sharp s at all: `swapchar()` hard-codes
// U+00DF -> U+1E9E before it consults any table, so `gUU` draws `ẞ` under every
// `'casemap'` on both binaries and a probe built on it would have reported nothing.  The
// row is reachable only through `\U`/`\U` in a substitution, which goes `do_upper` ->
// `vim_toupper` -> `utf_toupper` and hits the table directly -- and THAT IS ALSO THE
// ARGUMENT FOR THE ROW: today `swapchar()` and `toUpper[]` give different answers for
// the same character, and after this phase they agree.  And the chartab the 892 startup
// calls of `towupper`/`towlower` build does NOT move, although those calls run with
// `cmp_flags` still 0 and therefore take the non-internal arm: the union equals musl's
// table at every one of 128..255, the two having disagreed below U+0100 at U+00DF alone
// and the union taking musl's answer there.
//
// AND THE RECORDED CORPUS CANNOT SEE ANY OF IT, so `phase/112/delta.md` gains no line.
// All 102 screen cases seed themselves by typing ASCII and none of them touches
// `'casemap'`; two full recordings are byte-identical.  That is phase 85's
// situation -- a blind harness rather than a static phase -- and a phase in it owes
// probes of its own.  These are they.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim112", Check) }

// The probe text, in ONE place: part 1 asserts that every codepoint this
// phase changes on the default arm, and every one it stops mapping, is probed
// -- and part 6 types it.
const (
	z29ProbeText = "ⓐ ⱟ 𐖗 𐵰 ß"
	z29ProbeUp   = "Ⓐ Ⱟ 𐕰 𐵐 ẞ"
)

type z29RowT [4]int

// A convertStruct row is written one of TWO ways in this file, and the edit
// says which and why: vim's own toUpper[]/toLower[] are canonical text -- four
// spaces of indent and a space after each comma -- and the musl_to*[] that
// phase 98 vendored are that phase's INSERTED text, written tight at eight.
// Both shapes are written out here rather than borrowed from the edit: a check
// that read the text through the edit's own regex would agree with it by
// construction, and what is being asserted is what the FILE says.
var (
	z29CheckCanon = regexp.MustCompile(`^    \{(0x[0-9a-f]+), (0x[0-9a-f]+), (-?\d+), (-?\d+)\},?$`)
	z29CheckTight = regexp.MustCompile(`^        \{(0x[0-9a-f]+),(0x[0-9a-f]+),(-?\d+),(-?\d+)\},?$`)
)

// z29Table is rows_of(): the rows of one convertStruct table, or nil when the
// table is not in the text.
//
// IT IS A PARTITION AND NOT A FILTER.  Every line of the body must be a row in
// one of the two shapes and the table must be written one way throughout, so a
// line this reader does not understand refuses instead of being skipped -- a
// regex that silently matched nothing is how a table read as EMPTY and every
// codepoint in it then looked unchanged.
func z29Table(text, name string) ([]z29RowT, bool) {
	re := regexp.MustCompile(`(?ms)^static convertStruct ` + regexp.QuoteMeta(name) + `\[\] =\n\{\n(.*?)\n\};\n`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return nil, false
	}
	var Out []z29RowT
	tight := 0
	for _, line := range strings.Split(m[1], "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		r := z29CheckCanon.FindStringSubmatch(line)
		if r == nil {
			if r = z29CheckTight.FindStringSubmatch(line); r == nil {
				return nil, false
			}
			tight++
		}
		var row z29RowT
		for k := 0; k < 4; k++ {
			v, _ := strconv.ParseInt(r[k+1], 0, 64)
			row[k] = int(v)
		}
		Out = append(Out, row)
	}
	if len(Out) == 0 || (tight != 0 && tight != len(Out)) {
		return nil, false
	}
	return Out, true
}

// z29Expand is {codepoint: target}, exactly as utf_convert() reads the row.
func z29Expand(rows []z29RowT) map[int]int {
	o := map[int]int{}
	for _, r := range rows {
		lo, hi, step, off := r[0], r[1], r[2], r[3]
		if step < 0 {
			o[lo] = lo + off
			continue
		}
		if step == 0 {
			continue
		}
		for c := lo; c <= hi; c += step {
			o[c] = c + off
		}
	}
	return o
}

func z29Get(m map[int]int, c int) int {
	if v, ok := m[c]; ok {
		return v
	}
	return c
}

func z29Sorted(pred func(int) bool, maps ...map[int]int) []int {
	seen := map[int]bool{}
	var Out []int
	for _, m := range maps {
		for c := range m {
			if !seen[c] && pred(c) {
				Out = append(Out, c)
			}
			seen[c] = true
		}
	}
	sort.Ints(Out)
	return Out
}

// z29Words is re.findall(r'\bNAME\b').
func z29Words(text, name string) int {
	return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllStringIndex(text, -1))
}

// z29Calls is `(?<![\w])NAME\s*\(`: RE2 has no lookbehind, so the byte before
// each match is tested instead.
func z29Calls(text, name string) int {
	n := 0
	for _, m := range regexp.MustCompile(regexp.QuoteMeta(name)+`\s*\(`).FindAllStringIndex(text, -1) {
		if m[0] > 0 {
			b := text[m[0]-1]
			if b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b >= 0x80 {
				continue
			}
		}
		n++
	}
	return n
}

func z29Retable(text, name string, rows []z29RowT) string {
	re := regexp.MustCompile(`(?ms)^static convertStruct ` + regexp.QuoteMeta(name) + `\[\] =\n\{\n.*?\n\};\n`)
	head := re.FindString(text)
	var Body []string
	for _, r := range rows {
		Body = append(Body, fmt.Sprintf("        {0x%x,0x%x,%d,%d}", r[0], r[1], r[2], r[3]))
	}
	return strings.ReplaceAll(text, head, "static convertStruct "+name+"[] =\n{\n"+strings.Join(Body, ",\n")+"\n};\n")
}

func z29Hex(cs []int) string {
	var s []string
	for _, c := range cs {
		s = append(s, fmt.Sprintf("U+%04X", c))
	}
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, " ")
}

type z29Sum struct {
	Name                            string
	r0, r1, ncp, nn, arm, ndef, nst int
	defs                            string
}

// Whim112 is phase 112's check: the case tables become one, and it is the union.
func Check(w io.Writer, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: check whim112 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	f := filepath.Join(work, "whim-vim.c")
	oldC := filepath.Join(state, "old.c")
	r := &check.Rep{Tag: "casemap", W: w}
	stop := func(format string, a ...any) error {
		r.Say(format, a...)
		return harness.ErrReported
	}
	beforeLines := strings.TrimRight(check.ReadFile(filepath.Join(state, "input-lines")), "\n")
	tmp, err := os.MkdirTemp("", "whim112-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	var cflags, ldflags []string
	if m := check.Z29CFlags.FindStringSubmatch(mk); m != nil {
		cflags = strings.Fields(m[1])
	}
	if m := check.Z29LDFlags.FindStringSubmatch(mk); m != nil {
		ldflags = strings.Fields(m[1])
	}

	// --- 1. the union, the authority, the rule and the source ----------------
	// tools/create_cmdidxs.py -- named as a PATH so tools/implhash.sh hashes
	// it into this phase's key.  Do not delete it.
	newT := check.ReadFile(f)
	oldT := check.ReadFile(oldC)
	probed := map[int]bool{}
	for _, c := range z29ProbeText + z29ProbeUp {
		probed[int(c)] = true
	}
	type key struct{ Tag, Name string }
	tables := map[key][]z29RowT{}
	have := map[key]bool{}
	for _, x := range []struct{ Text, Tag string }{{oldT, "old"}, {newT, "new"}} {
		for _, name := range []string{"toUpper", "toLower", "musl_toUpper", "musl_toLower"} {
			rows, ok := z29Table(x.Text, name)
			tables[key{x.Tag, name}], have[key{x.Tag, name}] = rows, ok
		}
	}
	var fail []string
	for _, name := range []string{"toUpper", "toLower"} {
		if !have[key{"old", name}] || !have[key{"new", name}] {
			fail = append(fail, fmt.Sprintf("%s[] is missing from one of the two sources", name))
		}
		if !have[key{"old", "musl_" + name}] {
			fail = append(fail, fmt.Sprintf("musl_%s[] was not in the input, so this phase merges nothing and "+
				"the check below proves nothing", name))
		}
		if have[key{"new", "musl_" + name}] {
			fail = append(fail, fmt.Sprintf("musl_%s[] survives in the output", name))
		}
	}

	var summary []z29Sum
	if len(fail) == 0 {
		up, low, err := harness.MuslCaseLibc()
		if err != nil {
			return err
		}
		for _, x := range []struct {
			Name string
			off  map[int]int
		}{{"toUpper", up}, {"toLower", low}} {
			name := x.Name
			libcf := func(c int) int { return c + x.off[c] }
			ev := z29Expand(tables[key{"old", name}])
			em := z29Expand(tables[key{"old", "musl_" + name}])
			en := z29Expand(tables[key{"new", name}])
			union := map[int]int{}
			for c, v := range ev {
				union[c] = v
			}
			for c, v := range em {
				union[c] = v
			}
			// (a) everything the INPUT's vim table mapped, mapped the same
			//     way -- the half a careless merge loses silently.
			lostVim := z29Sorted(func(c int) bool { return z29Get(en, c) != ev[c] }, ev)
			// (b) everything the INPUT's musl table mapped, mapped the same way.
			lostMusl := z29Sorted(func(c int) bool { return z29Get(en, c) != em[c] }, em)
			// (c) nothing neither had.
			invented := z29Sorted(func(c int) bool {
				u, ok := union[c]
				return !ok || en[c] != u
			}, en)
			for _, y := range []struct {
				What string
				s    []int
			}{
				{fmt.Sprintf("is no longer mapped as the input's %s[] mapped it", name), lostVim},
				{fmt.Sprintf("is no longer mapped as the input's musl_%s[] mapped it", name), lostMusl},
				{"is mapped by neither table the phase was handed", invented},
			} {
				if len(y.s) > 0 {
					fail = append(fail, fmt.Sprintf("%d codepoints, U+%04X among them, %s -- the output is not "+
						"the union", len(y.s), y.s[0], y.What))
				}
			}
			// (d) THE AUTHORITY: the musl half again, from this machine's libc
			//     rather than from the table the phase deleted.
			var bad []int
			for c := 0; c < harness.MuslCasePlanes; c++ {
				want := libcf(c)
				if want != c && z29Get(en, c) != want {
					bad = append(bad, c)
					if len(bad) > 4 {
						break
					}
				}
			}
			if len(bad) > 0 {
				fail = append(fail, fmt.Sprintf("%s[] disagrees with THIS MACHINE'S libc at U+%04X (the table "+
					"says %04X, libc says %04X) and %d more",
					name, bad[0], z29Get(en, bad[0]), libcf(bad[0]), len(bad)-1))
			}
			// (e) THE RULE, and it is a rule rather than a number.
			defaultChanges := z29Sorted(func(c int) bool { return en[c] != z29Get(ev, c) }, en)
			stops := z29Sorted(func(c int) bool { _, ok := en[c]; return !ok }, em)
			var unprobed []int
			for _, c := range append(append([]int{}, defaultChanges...), stops...) {
				if !probed[c] {
					unprobed = append(unprobed, c)
				}
			}
			if len(unprobed) > 0 {
				fail = append(fail, fmt.Sprintf("%d codepoint(s) this phase changes on the default arm or stops "+
					"mapping are not in the probe text, U+%04X among them -- nothing "+
					"it moves there may go un-probed", len(unprobed), unprobed[0]))
			}
			// and the non-internal arm, where 96 arrive and no probe text
			// could hold them all: at least one must be looked at.
			armChanges := z29Sorted(func(c int) bool { return z29Get(en, c) != z29Get(em, c) }, en, em)
			anyProbed := false
			for _, c := range armChanges {
				if probed[c] {
					anyProbed = true
				}
			}
			if len(armChanges) > 0 && !anyProbed {
				fail = append(fail, fmt.Sprintf("not one of the %d codepoints that move on the non-internal arm "+
					"is in the probe text", len(armChanges)))
			}
			if len(armChanges) == 0 {
				fail = append(fail, fmt.Sprintf("%s[] and musl_%s[] agreed everywhere, so there was no union to "+
					"take and this phase proves nothing", name, name))
			}
			summary = append(summary, z29Sum{name, len(tables[key{"old", name}]), len(tables[key{"new", name}]),
				len(ev), len(en), len(armChanges), len(defaultChanges), len(stops), z29Hex(defaultChanges)})
		}

		// PROVEN ABLE TO FAIL: perturbing one row of the produced table must
		// break (a), (b) or (d).  Done here, on text, so it costs nothing.
		for _, name := range []string{"toUpper", "toLower"} {
			rows := tables[key{"new", name}]
			if len(rows) == 0 {
				fail = append(fail, fmt.Sprintf("perturbing the first row of the produced %s[] changes no "+
					"codepoint, so the union check cannot fail", name))
				continue
			}
			broken := append([]z29RowT{{rows[0][0], rows[0][1], rows[0][2], rows[0][3] + 1}}, rows[1:]...)
			eb := z29Expand(broken)
			ev := z29Expand(tables[key{"old", name}])
			moved := false
			for c, v := range ev {
				if z29Get(eb, c) != v {
					moved = true
					break
				}
			}
			if !moved {
				fail = append(fail, fmt.Sprintf("perturbing the first row of the produced %s[] changes no "+
					"codepoint, so the union check cannot fail", name))
			}
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	for _, s := range summary {
		r.Say("%s[] %d rows -> %d, %d codepoints -> %d: it maps everything the "+
			"input's %s[] mapped AND everything the input's musl_%s[] mapped AND NOTHING "+
			"NEITHER HAD, over all 1,114,112 codepoints, with the musl half re-derived "+
			"from THIS MACHINE'S libc through ctypes rather than from the table this "+
			"phase deleted.  %d codepoints move on the NON-INTERNAL arm and %d on the "+
			"DEFAULT one (%s), %d stop mapping anywhere -- and every one of the last two "+
			"kinds is in the probe text",
			s.Name, s.r0, s.r1, s.ncp, s.nn, s.Name, s.Name, s.arm, s.ndef, s.defs, s.nst)
	}

	// --- the source, as rules ------------------------------------------------
	var src []string
	for _, name := range []string{"musl_toUpper", "musl_toLower"} {
		if n := z29Words(newT, name); n > 0 {
			src = append(src, fmt.Sprintf("%s is still named %d times", name, n))
		}
		if n := z29Words(oldT, name); n != 3 {
			src = append(src, fmt.Sprintf("the input names %s %d times, not the 3 this phase was written "+
				"against -- its definition and the two in its wrapper's "+
				"utf_convert call", name, n))
		}
	}
	for _, name := range []string{"musl_towupper", "musl_towlower"} {
		nw, ow := z29Words(newT, name), z29Words(oldT, name)
		if nw != ow || nw != 4 {
			src = append(src, fmt.Sprintf("%s has %d mentions and the input had %d; both must be 4 -- its "+
				"prototype, its definition, the live call in utf_to*() and the dead "+
				"one in vim_to*().  THE TWO WRAPPERS STAY", name, nw, ow))
		}
	}
	for _, name := range []string{"toUpper", "toLower"} {
		nw, ow := z29Words(newT, name), z29Words(oldT, name)
		if nw != ow+2 {
			src = append(src, fmt.Sprintf("%s is named %d times and the input named it %d; the repointed "+
				"wrapper adds exactly two", name, nw, ow))
		}
	}
	nCalls, oCalls := z29Calls(newT, "utf_convert"), z29Calls(oldT, "utf_convert")
	if nCalls != oCalls {
		src = append(src, fmt.Sprintf("utf_convert is called %d times and the input called it %d -- this "+
			"phase moves no call, it changes what two of them read", nCalls, oCalls))
	}
	// The two things this phase does NOT touch, and both are load-bearing for
	// its argument: swapchar()'s hard-coded sharp s, and utf_islower()'s
	// `|| a == 0xdf`, now redundant and deliberately left.
	for _, x := range []struct{ Text, Tag string }{{newT, "output"}, {oldT, "input"}} {
		if !strings.Contains(x.Text, "ins_char(0x1E9E);") {
			src = append(src, fmt.Sprintf("swapchar()'s hard-coded sharp s is not in the %s, and it is both why "+
				"gU cannot show this phase's row and what the row makes the table "+
				"agree with", x.Tag))
		}
		if !strings.Contains(x.Text, "(utf_toupper(a) != a) || a == 0xdf") {
			src = append(src, fmt.Sprintf("utf_islower()'s `|| a == 0xdf` is not in the %s", x.Tag))
		}
	}
	L := strings.Split(newT, "\n")
	var d []int
	for i, l := range L {
		if strings.HasPrefix(strings.TrimLeft(l, " \t\n\v\f\r"), "#") {
			d = append(d, i)
		}
	}
	allInc := true
	for _, i := range d {
		if !check.Z29Inc.MatchString(L[i]) {
			allInc = false
		}
	}
	if len(d) != 11 || !allInc {
		src = append(src, "the directives are not the eleven #includes phase 104 left")
	} else {
		for k, i := range d {
			if i != d[0]+k {
				src = append(src, "the eleven #includes are no longer eleven consecutive lines")
				break
			}
		}
	}
	// The two tables this phase must not touch, stated against the INPUT
	// rather than as remembered numbers.
	cmds := len(check.Z29Cmd.FindAllString(newT, -1))
	was := len(check.Z29Cmd.FindAllString(oldT, -1))
	got, _ := harness.CommandNames(f)
	if cmds != was || len(got) != was {
		src = append(src, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; the input had %d and this "+
			"phase touches no row", cmds, len(got), was))
	}
	if check.Z29RowCount(newT) != check.Z29RowCount(oldT) {
		src = append(src, fmt.Sprintf("options[] has %d rows and the input had %d; this phase retires no option",
			check.Z29RowCount(newT), check.Z29RowCount(oldT)))
	}
	if !strings.Contains(newT, `{"casemap"`) {
		src = append(src, "the 'casemap' option row is gone, and retiring it is not this phase's")
	}
	if len(src) > 0 {
		for _, l := range src {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("musl_toUpper and musl_toLower at 0 mentions where the input had 3 each, "+
		"so the assertion is one that can fail; musl_towupper and musl_towlower STILL AT "+
		"4 each and now reading vim's tables; utf_convert at the same %d calls; "+
		"swapchar()'s hard-coded sharp s present in BOTH sources -- it is why gU cannot "+
		"show the row, and it is what the row makes the table agree with -- and "+
		"utf_islower()'s `|| a == 0xdf`, now redundant, deliberately left; eleven "+
		"consecutive #includes and nothing else; cmdnames[] %d and options[] %d, both the "+
		"input's, with 'casemap' still among them", nCalls, cmds, check.Z29RowCount(newT))

	// The two controls, computed from the two sources rather than spelled Out.
	//   vimonly  the output with musl's contribution taken back Out.
	//   vimless  vim's toUpper[] replaced by the input's musl_toUpper[].
	vimonly := newT
	for _, name := range []string{"toUpper", "toLower"} {
		ev := z29Expand(tables[key{"old", name}])
		em := z29Expand(tables[key{"old", "musl_" + name}])
		var keep []z29RowT
		for _, rr := range tables[key{"new", name}] {
			_, inEm := em[rr[0]]
			_, inEv := ev[rr[0]]
			if !(rr[0] == rr[1] && inEm && !inEv) {
				keep = append(keep, rr)
			}
		}
		if len(keep) != len(tables[key{"old", name}]) {
			return stop("taking musl's contribution back out of %s[] leaves %d rows "+
				"where the input had %d, so the `vimonly` control is not the input's "+
				"table", name, len(keep), len(tables[key{"old", name}]))
		}
		vimonly = z29Retable(vimonly, name, keep)
	}
	if vimonly == newT {
		return stop("the `vimonly` control is the output unchanged, so the union added " +
			"nothing and the default-arm probe proves nothing")
	}
	if err := os.WriteFile(filepath.Join(tmp, "vimonly.c"), []byte(vimonly), 0o644); err != nil {
		return err
	}
	vimless := z29Retable(newT, "toUpper", tables[key{"old", "musl_toUpper"}])
	if err := os.WriteFile(filepath.Join(tmp, "vimless.c"), []byte(vimless), 0o644); err != nil {
		return err
	}
	r.Say("two controls written: `vimonly`, the output with musl's contribution " +
		"taken back out -- the union NOT taken -- and `vimless`, the output with toUpper[] " +
		"replaced by the input's musl_toUpper[] -- the merge done the careless way round.  " +
		"One for each half of the union, because \"it kept both\" is the claim a careless " +
		"phase breaks in silence")

	// --- 2. the compile, the linkage and the libc surface ---------------------
	for _, x := range []struct{ src, obj string }{{oldC, "old.o"}, {f, "new.o"}} {
		if Out, err := exec.Command("gcc", "-c", "-O0", "-fno-stack-protector", "-o",
			filepath.Join(tmp, x.obj), x.src).CombinedOutput(); err != nil {
			w.Write(Out)
			return harness.ErrReported
		}
	}
	uOld := check.NmField26(filepath.Join(tmp, "old.o"), []string{"-u"}, 1)
	uNew := check.NmField26(filepath.Join(tmp, "new.o"), []string{"-u"}, 1)
	gone := check.Minus26(uOld, uNew)
	came := check.Minus26(uNew, uOld)
	if len(gone)+len(came) > 0 {
		sp := func(xs []string) string {
			s := ""
			for _, x := range xs {
				s += x + " "
			}
			return s
		}
		return stop("`nm -u` moved: gone [%s] arrived [%s].  CHANGING DATA FREES NO LIBC SYMBOL AND NEEDS NONE -- "+
			"the eleven phase 98 freed were freed by the two wrappers, which this phase keeps", sp(gone), sp(came))
	}
	ext := check.NmField26(filepath.Join(tmp, "new.o"), []string{"--extern-only", "--defined-only"}, 2)
	if extS := strings.Join(ext, " ") + " "; extS != "main " {
		return stop("the output defines external symbols other than main: %s", extS)
	}
	for _, absent := range []string{"towupper", "towlower", "toupper", "tolower", "open", "stat", "fopen"} {
		for _, u := range uNew {
			if u == absent {
				return stop("%s is undefined again, and phase 98 freed it", absent)
			}
		}
	}
	r.Say("`nm -u` is THE SAME SET, %d names, as a `comm` empty in BOTH directions, and `main` is still the "+
		"only external symbol.  towupper and towlower stay freed: what this phase changes is DATA, and the "+
		"two wrappers that replaced the calls are still there", len(uNew))

	// --- 3. the enumerators ---------------------------------------------------
	var wgE sync.WaitGroup
	var errEO error
	wgE.Add(1)
	go func() {
		defer wgE.Done()
		errEO = exec.Command("sh", "tools/enumvals.sh", oldC, filepath.Join(tmp, "ev.old")).Run()
	}()
	errEN := exec.Command("sh", "tools/enumvals.sh", f, filepath.Join(tmp, "ev.new")).Run()
	wgE.Wait()
	if errEN != nil || errEO != nil {
		return harness.ErrReported
	}
	evmap := func(p string) map[string]string {
		m := map[string]string{}
		for _, l := range strings.Split(strings.TrimRight(check.ReadFile(p), "\n"), "\n") {
			if i := strings.LastIndex(l, "="); i >= 0 {
				m[l[:i]] = l[i+1:]
			}
		}
		return m
	}
	eo, en := evmap(filepath.Join(tmp, "ev.old")), evmap(filepath.Join(tmp, "ev.new"))
	var eGone, eCame, eMoved []string
	for k, v := range eo {
		if nv, ok := en[k]; !ok {
			eGone = append(eGone, k)
		} else if nv != v {
			eMoved = append(eMoved, k)
		}
	}
	for k := range en {
		if _, ok := eo[k]; !ok {
			eCame = append(eCame, k)
		}
	}
	sort.Strings(eGone)
	sort.Strings(eCame)
	sort.Strings(eMoved)
	if len(eGone)+len(eCame)+len(eMoved) > 0 {
		for _, y := range []struct {
			What string
			s    []string
		}{{"went", eGone}, {"arrived", eCame}, {"renumbered", eMoved}} {
			if len(y.s) > 0 {
				r.Say("enumerators %s: %s", y.What, strings.Join(y.s, " "))
			}
		}
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d: not one went, arrived or renumbered -- this phase "+
		"edits two array initialisers and touches no enum", len(eo), len(en))

	// --- 4. the structural tools, and the boundary as a compile ---------------
	for _, tool := range []string{"nvidx", "orphanopts"} {
		c := exec.Command("sh", "tools/st.sh", tool, f)
		c.Stdout, c.Stderr = w, w
		if err := c.Run(); err != nil {
			return harness.ErrReported
		}
	}
	cutLines := map[string]int{}
	boundary := map[string]string{}
	for _, x := range []struct{ src, base string }{{f, "whim-vim.c"}, {oldC, "old.c"}} {
		lines := check.Z28Cut(check.ReadFile(x.src))
		text := ""
		for _, l := range lines {
			text += l + "\n"
		}
		cp := filepath.Join(tmp, "cut.c")
		os.WriteFile(cp, []byte(text), 0o644)
		if check.Z29Hash.MatchString(text) {
			return stop("the cut of %s holds a directive", x.src)
		}
		c := exec.Command("gcc", "-O0", "-fno-stack-protector", "-Wall", "-Wextra",
			"-Wno-unused-parameter", "-fsyntax-only", cp)
		var eb strings.Builder
		c.Stderr = &eb
		c.Run()
		wl := strings.Split(eb.String(), "\n")
		var errs, warns []string
		for _, l := range wl {
			if strings.Contains(l, ": error:") {
				errs = append(errs, l)
			}
			if strings.Contains(l, ": warning: ") && !strings.Contains(l, "used but never defined") {
				warns = append(warns, l)
			}
		}
		head3 := func(xs []string) {
			for i, l := range xs {
				if i >= 3 {
					break
				}
				fmt.Fprintf(w, "               %s\n", l)
			}
		}
		if len(errs) > 0 {
			r.Say("the cut of %s does not compile on its own:", x.src)
			head3(errs)
			return harness.ErrReported
		}
		if len(warns) > 0 {
			r.Say("the cut of %s has a warning that is not a boundary name:", x.src)
			head3(warns)
			return harness.ErrReported
		}
		set := map[string]bool{}
		for _, m := range check.Z29Undef.FindAllString(eb.String(), -1) {
			set[m] = true
		}
		names := check.Z27Keys(set)
		bt := ""
		for _, n := range names {
			bt += n + "\n"
		}
		boundary[x.base] = bt
		os.WriteFile(filepath.Join(tmp, "boundary."+x.base), []byte(bt), 0o644)
		cutLines[x.base] = len(lines)
	}
	if boundary["whim-vim.c"] != boundary["old.c"] {
		r.Say("the core -> host interface moved:")
		Out, _ := exec.Command("diff", filepath.Join(tmp, "boundary.old.c"), filepath.Join(tmp, "boundary.whim-vim.c")).Output()
		for i, l := range strings.Split(strings.TrimRight(string(Out), "\n"), "\n") {
			if i >= 6 {
				break
			}
			fmt.Fprintf(w, "               %s\n", l)
		}
		return harness.ErrReported
	}
	r.Say("cut at the first #include, %d lines -> %d, 0 directives, 0 errors and no warning that is not a "+
		"boundary name -- and THE SAME %d NAMES either side, read at run time, so the core -> host interface "+
		"phase 110 drew did not move", cutLines["old.c"], cutLines["whim-vim.c"],
		strings.Count(boundary["whim-vim.c"], "\n"))

	// --- 5. the binary, and the two controls ----------------------------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	hdr, _ := exec.Command("readelf", "-h", bin).Output()
	if !regexp.MustCompile(`Type:.*EXEC`).Match(hdr) {
		return stop("the binary is no longer EXEC")
	}
	if ph, _ := exec.Command("readelf", "-l", bin).Output(); strings.Contains(string(ph), "INTERP") {
		return stop("the binary grew an INTERP")
	}
	if dy, _ := exec.Command("readelf", "-d", bin).Output(); strings.Contains(string(dy), "Dynamic section") {
		return stop("the binary grew a dynamic section")
	}
	link := func(src, Out string) error {
		a := append(append(append([]string{}, cflags...), ldflags...), "-o", Out, src)
		return exec.Command("gcc", a...).Run()
	}
	vimonlyBin, vimlessBin := filepath.Join(tmp, "vimonly"), filepath.Join(tmp, "vimless")
	var wgB sync.WaitGroup
	var errO, errV error
	wgB.Add(2)
	go func() { defer wgB.Done(); errO = link(filepath.Join(tmp, "vimonly.c"), vimonlyBin) }()
	go func() { defer wgB.Done(); errV = link(filepath.Join(tmp, "vimless.c"), vimlessBin) }()
	wgB.Wait()
	if errO != nil {
		return stop("the vimonly control did not build")
	}
	if errV != nil {
		return stop("the vimless control did not build")
	}
	nowB, _ := os.ReadFile(f)
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes against the input's %d -- and it SHRINKS, "+
		"because two tables of 16-byte rows leave and one row arrives",
		beforeLines, check.CountLines(nowB), check.SizeOf(bin), check.SizeOf(filepath.Join(state, "old")))

	// --- 6. twelve probes on both binaries, and the two controls --------------
	oldBin, _ := filepath.Abs(filepath.Join(state, "old"))
	if err := z29Probes(r, oldBin, bin, vimonlyBin, vimlessBin); err != nil {
		return err
	}

	// --- 7. the recording did not move ----------------------------------------
	// A recording that fails says why, and ends the check: see recjob.go.
	var wgR sync.WaitGroup
	var errRO, errRN error
	wgR.Add(2)
	go func() {
		defer wgR.Done()
		errRO = check.RecZ(oldBin, oldC, filepath.Join(tmp, "REC.old"))
	}()
	go func() {
		defer wgR.Done()
		errRN = check.RecZ(bin, f, filepath.Join(tmp, "REC.new"))
	}()
	wgR.Wait()
	if check.RecReport(w, errRO, errRN) {
		return harness.ErrReported
	}
	if dl := check.DiffRQ(filepath.Join(tmp, "REC.old"), filepath.Join(tmp, "REC.new")); len(dl) > 0 {
		r.Say("the declared delta is NOTHING AT ALL and the two recordings differ:")
		for i, l := range dl {
			if i >= 12 {
				break
			}
			fmt.Fprintln(w, l)
		}
		return harness.ErrReported
	}
	r.Say("the declared delta is NOTHING AT ALL and TWO FULL RECORDINGS ARE BYTE-IDENTICAL -- and here that " +
		"is the HARNESS and not the phase.  The six probes above prove both arms of 'casemap' moved; the " +
		"corpus cannot see it, because all 102 screen cases seed themselves by typing ASCII and none of them " +
		"touches 'casemap'")
	return nil
}

type z29Session struct {
	Args []string
	Keys [][]byte
}

type z29Probe struct {
	Name       string
	session    z29Session
	mustDiffer bool
	want       string
}

func z29Typed(seed string, keys ...string) z29Session {
	k := [][]byte{[]byte("i" + seed + "\x1b"), []byte(":set nopaste\r")}
	for _, x := range keys {
		k = append(k, []byte(x))
	}
	k = append(k, []byte("\x1b:q!\r"))
	return z29Session{[]string{"+set paste"}, k}
}

// z29Line is the first line of the last screen the editor drew -- the edited
// text.
func z29Line(rec string) string {
	last, ok := "", false
	for _, p := range harness.SplitRecord(rec) {
		if strings.HasPrefix(p.Head, "snap") {
			last, ok = p.Body, true
		}
	}
	if !ok {
		return "<no screen>"
	}
	return strings.TrimSpace(strings.SplitN(last, "\n", 2)[0])
}

func z29Probes(r *check.Rep, oldBin, newBin, vimonlyBin, vimlessBin string) error {
	low, up, ss := z29ProbeText, z29ProbeUp, "ß"
	const subst = ":s/.*/\\U&/\r"
	// name, session, must-differ, what the NEW record must SHOW.  A record
	// that is equal because both binaries did nothing is two failures
	// agreeing, so every row says what it expects to see.
	probes := []z29Probe{
		// THE FOUR THAT MOVE ON THE NON-INTERNAL ARM.
		{"empty_gUU", z29Typed(low, ":set casemap=\r", "gUU"), true, "Ⓐ Ⱟ 𐕰 𐵐 ẞ"},
		{"empty_guu", z29Typed(up, ":set casemap=\r", "guu"), true, "ⓐ ⱟ 𐖗 𐵰 ß"},
		{"keepascii_gUU", z29Typed(low, ":set casemap=keepascii\r", "gUU"), true, "Ⓐ Ⱟ 𐕰 𐵐 ẞ"},
		{"empty_subst_U", z29Typed(low, ":set casemap=\r", subst), true, "Ⓐ Ⱟ 𐕰 𐵐 ẞ"},
		// AND THE TWO THAT MOVE ON THE DEFAULT ARM, which is the one row,
		// reached through `\U` because swapchar() hard-codes U+00DF.
		{"default_subst_U", z29Typed(low, subst), true, "Ⓐ Ⱟ 𐕰 𐵐 ẞ"},
		{"default_subst_ss", z29Typed(ss, subst), true, "ẞ"},
		// THE SIX THAT MUST NOT MOVE.
		{"default_gUU", z29Typed(low, "gUU"), false, "Ⓐ Ⱟ 𐕰 𐵐 ẞ"},
		{"default_guu", z29Typed(up, "guu"), false, "ⓐ ⱟ 𐖗 𐵰 ß"},
		{"internal_gUU", z29Typed(low, ":set casemap=internal\r", "gUU"), false, "Ⓐ Ⱟ 𐕰 𐵐 ẞ"},
		{"empty_subst_ss", z29Typed(ss, ":set casemap=\r", subst), false, "ẞ"},
		{"tilde_ss", z29Typed(ss, "0", "g~g~"), false, "ẞ"},
		// the chartab the 892 startup calls of towupper/towlower build.
		{"isk_at", z29Typed("café naïve", ":set isk=@\r", "0", "dw"), false, "naïve"},
	}
	controls := []struct{ cname, binary, probe string }{
		{"vimonly", vimonlyBin, "default_subst_ss"},
		{"vimonly", vimonlyBin, "empty_subst_ss"},
		{"vimless", vimlessBin, "default_gUU"},
	}
	session := map[string]z29Session{}
	for _, p := range probes {
		session[p.Name] = p.session
	}
	type job struct{ Name, binary string }
	var jobs []job
	for _, p := range probes {
		jobs = append(jobs, job{p.Name, oldBin}, job{p.Name, newBin})
	}
	for _, c := range controls {
		jobs = append(jobs, job{c.probe, c.binary})
	}
	rec := map[job]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, j := range jobs {
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := session[j.Name]
			t, _ := check.ZRecordStream(j.binary, s.Args, s.Keys, 10*time.Second)
			mu.Lock()
			rec[j] = t
			mu.Unlock()
		}()
	}
	wg.Wait()

	var fail, moved, still []string
	for _, p := range probes {
		o, n := rec[job{p.Name, oldBin}], rec[job{p.Name, newBin}]
		if !strings.Contains(n, p.want) {
			fail = append(fail, fmt.Sprintf("%s: the new binary shows %s and not %s, so the comparison below "+
				"is two failures agreeing", p.Name, check.PyRepr26(z29Line(n)), check.PyRepr26(p.want)))
		}
		if p.mustDiffer && o == n {
			fail = append(fail, fmt.Sprintf("%s DID NOT MOVE, and this phase claims it does: %s",
				p.Name, check.PyRepr26(z29Line(n))))
		}
		if !p.mustDiffer && o != n {
			fail = append(fail, fmt.Sprintf("%s MOVED, and this phase claims it does not: %s -> %s",
				p.Name, check.PyRepr26(z29Line(o)), check.PyRepr26(z29Line(n))))
		}
		if p.mustDiffer {
			moved = append(moved, fmt.Sprintf("%s %s -> %s", p.Name, check.PyRepr26(z29Line(o)), check.PyRepr26(z29Line(n))))
		} else {
			still = append(still, fmt.Sprintf("%s == %s", p.Name, check.PyRepr26(z29Line(n))))
		}
	}
	for _, c := range controls {
		cr := rec[job{c.probe, c.binary}]
		if cr == rec[job{c.probe, newBin}] {
			fail = append(fail, fmt.Sprintf("the `%s` control records the same %s as this phase's own binary, "+
				"so that probe is not being tested", c.cname, c.probe))
		}
		if c.cname == "vimonly" && c.probe == "default_subst_ss" && cr != rec[job{c.probe, oldBin}] {
			fail = append(fail, fmt.Sprintf("the `vimonly` control does not record %s as the INPUT did, so what "+
				"it removes is not exactly what the union adds", c.probe))
		}
	}
	vl := z29Line(rec[job{"default_gUU", vimlessBin}])
	if strings.Contains(vl, "Ⓐ") {
		fail = append(fail, fmt.Sprintf("the `vimless` control still uppercases the circled letter, so it is "+
			"not the careless merge it is meant to be: %s", check.PyRepr26(vl)))
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("SIX PROBES MOVE, and they are the union arriving on BOTH arms -- four on "+
		"'casemap' without `internal`, which read musl's table and now read the union, "+
		"and two on the DEFAULT one, which is the single row: %s", strings.Join(moved, "; "))
	r.Say("SIX DO NOT: the ninety-six PROVING THEY DID NOT REGRESS on the arm that "+
		"always had them, the sharp s KEEPING what it had on the arm that always had it, "+
		"swapchar()'s hard-coded answer, and the chartab the 892 startup calls build -- %s",
		strings.Join(still, "; "))
	r.Say("AND THE TWO CONTROLS FAIL AS THEY MUST.  With musl's one row taken back "+
		"out -- the union NOT taken -- default_subst_ss records exactly what the INPUT "+
		"recorded and empty_subst_ss draws %s where this phase draws %s, which is what the "+
		"row buys.  With toUpper[] replaced by the input's musl_toUpper[] -- the merge "+
		"done the careless way round -- default_gUU draws %s and the circled letter is "+
		"gone, which is the regression no record could report",
		check.PyRepr26(z29Line(rec[job{"empty_subst_ss", vimonlyBin}])),
		check.PyRepr26(z29Line(rec[job{"empty_subst_ss", newBin}])), check.PyRepr26(vl))
	return nil
}
