package p126

// Whim phase 126 -- a block number becomes a reference.
//
// THE MEMLINE STOPS NAMING ITS BLOCKS BY NUMBER AND HOLDS THEM.  `pe_bnum` and
// `ip_bnum` become `bhdr_T *`, `memline_T` gains `ml_root`, and `mf_get(mfp, nr,
// page_count)` becomes `mf_get(mfp, hp)`.  The hash table that turned an integer block
// number into a page then has nothing left to look up, so it goes -- with the free list
// it was keyed alongside, with `mf_blocknr_max` that handed the numbers out, and with
// `pe_page_count`, whose one reader was the argument `mf_get` no longer takes.
//
// THIS IS THE PHASE THAT BUYS THE PORT THE MOST, AND IT IS WORTH SAYING WHY IN ONE
// SENTENCE: an integer key into a side hash table becomes an object reference, which is
// the one thing a JVM has and C does not make it say.  GOALS.md II.4d lists what a port
// would have to be told about rather than translate, and the memline page is the whole
// of the list; this removes the outer half of it -- the indirection BETWEEN pages.  It
// does NOT remove the inner half: `db_index[1]` indexed to the line count, the fourteen
// `(char_u *)dp + start` interior pointers and the page arithmetic are untouched and are
// phase 127's.  A reference to a block whose innards are still a byte array is halfway.
//
// WHAT WAS THERE.  `mf_new()` handed every block an integer from a counter, inserted it
// in `mf_hash` under that integer, and the tree stored the integer; `mf_get()` took the
// integer back and hashed it to the page.  Nothing had been written to a disk since zero
// phase 89 and nothing could be read from one since phase 92, so the hash had held every
// live block for thirty-four phases and a lookup could not miss.  Four measurements say
// that in the text rather than as a story, and they are this edit's first act:
//
// * every block `mf_new()` makes is inserted in the hash, and the only thing that ever
// removes one is `mf_free()`, which removes it from the used list in the same breath;
// * `mf_get()`'s two ways of failing are `nr >= mf_blocknr_max || nr < 0` and a miss,
// and no caller passes anything but a number the tree stored;
// * the used list is not an ordering anything reads.  `mf_used_last` is WRITE-ONLY
// here -- phase 125 took `ml_setflags()`, its last reader -- and there is no release
// path left to walk it: `mf_release_all` is WHIM'S, three mentions in `slim-vim.c`
// and none in `whim-vim.c`, and phase 125 showed `mf_dont_release` to be a constant.
// So moving a block to the head of the list is bookkeeping nothing observes, and
// this edit keeps it anyway;
// * `pe_page_count` is read ONCE, into the argument `mf_get()` is about to lose.
//
// SO THE HASH IS A MAP FROM A NUMBER THIS FILE INVENTS TO A POINTER IT ALREADY HAD.
//
// WHAT A WRITE-ONLY FIELD COSTS, AND WHY FOUR OF THEM ARE IN THE EDIT.  tools/sweep.sh
// finds a function nothing calls, a type nothing names and a field named nowhere outside
// its own type; it finds none of `mf_used_last`, `bh_page_count`, `pe_page_count` or
// `pe_bnum`, because every one of them is WRITTEN.  tools/deadfields.py reports 0 fields
// in this region for exactly that reason, and gcc has no warning for a struct member in
// either direction.  Phase 103's trap is the other half of it: remove a member and leave
// its initialiser and the compile says `excess elements in struct initializer`, which is
// a correct phase failing.  Every field here goes WITH its writes, in this edit, and
// every mention of every one of them is partitioned below by the function it sits in --
// a partition and not a count, because a count is a bet on the phase before this one.
//
// THE ONE STRING THIS PHASE CHANGES, SAID HERE AND NOT BURIED.  `E323: Line count wrong
// in block %ld` is the only message in the file that printed a block number, and there
// is no number left to print.  It becomes `E323: Line count wrong in block`.  It is an
// `iemsg` on the arm of `ml_find_line()` that runs when the line counts under a pointer
// block do not add up to the tree's own line count -- an integrity check, reachable only
// from a corrupt tree -- and the check measures that no record in a whole recording
// reaches it, on the binary this phase was handed, and exhibits both messages from a
// build that forces the arm.  `E298: Didn't get block nr 0?` and `E298: Didn't get block
// nr 1?` are not changed but DELETED, with the two tests that were the only thing that
// could raise them: ml_open() asked whether the first two blocks came back numbered 0
// and 1, and the question has no meaning once there are no numbers.  Both strings are
// left standing for tools/sweep.sh, which is where an unreferenced object belongs.
//
// THE ROOT IS THE ONE BLOCK THE TREE CANNOT REACH BY DESCENT, so it needs a name.
// `ml_open()` used to rely on the first block it made being number 0 and `ml_find_line()`
// started every descent at 0; `ml_append_int()` asked `mhi_key != 0` to know whether the
// block that had just overflowed was the root.  `memline_T.ml_root` is that fact written
// down, set once in `ml_open()` and never again -- the root block's IDENTITY does not
// change when the root splits, which is what makes one field enough: the split copies the
// root's contents into a NEW block and leaves the root holding one entry that points at
// it, so `ml_root` is still the root and the stack entry above it is still right.
// The flags are the boundary makefile's and are not written here a second time
// (GOALS.md core rule 8).  The check needs the binary this phase was HANDED, for the
// instrumented pair and for the two recordings, so it is built here and left in the
// state directory (what passes between the parts is files).
// NOT create_cmdidxs --check, for phase/085/edit.go's reason: the derived
// first-two-letters index went with the command table whim's phase 80 reduced, and the
// tool raises rather than reporting nothing.  Nothing here touches the command table.
//

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.RegisterArgs("whim126", Edit) }

var (
	w126Lit     = regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'`)
	w126Head    = regexp.MustCompile(`^([A-Za-z_]\w*)\s*\(`)
	w126IncLine = regexp.MustCompile(`^ *# *include `)
	w126DirLine = regexp.MustCompile(`^ *#`)
)

// w126Names are the names this edit partitions, plus the three it introduces.
var w126Names = []string{"blocknr_T", "mf_hashitem_T", "mf_hashtab_T", "mhi_key", "mhi_next",
	"mhi_prev", "bh_hashitem", "pe_bnum", "ip_bnum", "pe_page_count",
	"bh_page_count", "mf_blocknr_max", "mf_free_first", "mf_used_last",
	"mf_hash", "ml_root", "pe_block", "ip_block"}

var (
	w126HashImpl = []string{"mf_hash_init", "mf_hash_free", "mf_hash_find", "mf_hash_add_item",
		"mf_hash_rem_item", "mf_hash_grow"}
	w126HashWrap = []string{"mf_ins_hash", "mf_rem_hash", "mf_find_hash"}
	w126FreeList = []string{"mf_ins_free", "mf_rem_free"}
)

// Whim126 turns a block number into a reference: `pe_bnum` and `ip_bnum` become
// `bhdr_T *`, `memline_T` gains `ml_root`, and the hash table that turned an
// integer into a page goes with the free list and `mf_blocknr_max`.
func Edit(text []byte, w io.Writer, args []string) ([]byte, error) {
	p := edit.Ph{Tag: "refblocks", W: w}
	if len(args) != 1 {
		return nil, p.Die("usage: edit whim126 <file> <state-dir>")
	}
	state := args[0]
	t := string(text)
	nIn := strings.Count(t, "\n")

	lines := func() []string { return strings.Split(t, "\n") }
	mentions := func(s, name string) int {
		return len(regexp.MustCompile(`\b(?:`+name+`)\b`).FindAllString(s, -1))
	}
	blankRuns := func(s string) int {
		L := strings.Split(s, "\n")
		n := 0
		for i := 1; i < len(L); i++ {
			if L[i] == "" && L[i-1] == "" {
				n++
			}
		}
		return n
	}
	swap := func(old, new, what, why string) error {
		c := strings.Count(t, old)
		if c != 1 {
			return p.Die("%s occurs %d times, expected %d -- %s", what, c, 1, why)
		}
		t = strings.ReplaceAll(t, old, new)
		return nil
	}
	// heads: every definition in this tree's ONE shape -- a name at column 0 with
	// `(` after it and `{` at column 0 on the next line, closed by `}` at column 0.
	type head struct {
		a, b int
		Name string
	}
	headsOf := func(src string) []head {
		L := strings.Split(src, "\n")
		var Out []head
		for i, l := range L {
			m := w126Head.FindStringSubmatch(l)
			if m != nil && i+1 < len(L) && L[i+1] == "{" {
				end := i + 1
				for end < len(L) && L[end] != "}" {
					end++
				}
				if end < len(L) {
					Out = append(Out, head{i, end, m[1]})
				}
			}
		}
		return Out
	}
	// owners: {function name: how many of its lines say `name`}.  THIS IS THE
	// PHASE'S UNIT OF ASSERTION -- WHERE a name is said and not how often, because
	// the phase before this one moves the counts.
	owners := func(name string) map[string]int {
		L := lines()
		hs := headsOf(t)
		who := func(i int) string {
			for _, h := range hs {
				if h.a <= i && i <= h.b {
					return h.Name
				}
			}
			return "<file scope>"
		}
		re := regexp.MustCompile(`\b` + name + `\b`)
		d := map[string]int{}
		for i, l := range L {
			if re.MatchString(l) {
				d[who(i)]++
			}
		}
		return d
	}
	places := func(name string, expect []string, what string) error {
		got := owners(name)
		var gk []string
		for k := range got {
			gk = append(gk, k)
		}
		sort.Strings(gk)
		ek := append([]string{}, expect...)
		sort.Strings(ek)
		if strings.Join(gk, "\x00") != strings.Join(ek, "\x00") {
			return p.Die("`%s` is said in %s and this phase accounts for %s -- %s",
				name, edit.W126PyList(gk), edit.W126PyList(ek), what)
		}
		var parts []string
		for _, k := range gk {
			parts = append(parts, fmt.Sprintf("%s %d", k, got[k]))
		}
		p.Sayf("`%s` is said in %d places and nowhere else: %s",
			name, len(got), strings.Join(parts, ", "))
		return nil
	}
	cut := func(a, b, what string) (int, error) {
		for _, s := range []struct{ v, side string }{{a, "start"}, {b, "end"}} {
			if c := strings.Count(t, s.v); c != 1 {
				return 0, p.Die("the %s of %s occurs %d times and a cut needs exactly one",
					s.side, what, c)
			}
		}
		i, j := strings.Index(t, a), strings.Index(t, b)
		if i >= j {
			return 0, p.Die("%s: the start is not above the end", what)
		}
		n := strings.Count(t[i:j], "\n")
		t = t[:i] + t[j:]
		return n, nil
	}

	// ---- 0. the file this edit is handed --------------------------------------
	literals := w126Lit.FindAllString(t, -1)
	var inlit []string
	for _, n := range w126Names {
		re := regexp.MustCompile(`\b` + n + `\b`)
		for _, s := range literals {
			if re.MatchString(s) {
				inlit = append(inlit, n)
				break
			}
		}
	}
	if len(inlit) > 0 {
		return nil, p.Die("%s appear inside a string literal, so a line-oriented partition would read "+
			"data as code", strings.Join(inlit, ", "))
	}
	for _, n := range []string{"ml_root", "pe_block", "ip_block"} {
		if k := mentions(t, n); k != 0 {
			return nil, p.Die("`%s` is already said %d times, and this phase is what introduces it", n, k)
		}
	}
	p.Sayf("%d string and character literals, and not one of them holds any of the %d names "+
		"this edit partitions or the 3 it introduces", len(literals), len(w126Names)-3)

	var incs, directives []int
	for i, l := range lines() {
		if w126IncLine.MatchString(l) {
			incs = append(incs, i)
		}
		if w126DirLine.MatchString(l) {
			directives = append(directives, i)
		}
	}
	if len(incs) != len(directives) || len(incs) == 0 {
		return nil, p.Die("the input has %d preprocessor directives and %d of them are `#include`, and "+
			"this phase adds none and removes none", len(directives), len(incs))
	}
	boundary := incs[0]
	p.Sayf("the input is %d lines with %d `#include`s and no other directive, the first at "+
		"line %d -- the line between the core and the host", nIn, len(incs), boundary+1)

	// ---- 1. where every name this phase removes is said -----------------------
	for _, pl := range []struct {
		Name   string
		expect []string
		What   string
	}{
		{"mhi_key", []string{"<file scope>", "mf_new", "ml_open", "ml_append_int",
			"mf_hash_find", "mf_hash_add_item", "mf_hash_rem_item", "mf_hash_grow"},
			"it is the hash key, the number mf_new() hands out and the number the tree stored"},
		{"bh_hashitem", []string{"<file scope>", "mf_new", "ml_open", "ml_append_int"},
			"it is the hash item embedded in every block header"},
		{"pe_bnum", []string{"<file scope>", "ml_open", "ml_find_line", "ml_append_int"},
			"it is the block number a pointer entry stores"},
		{"ip_bnum", []string{"<file scope>", "ml_find_line", "ml_append_int", "ml_delete_int",
			"ml_lineadd"}, "it is the block number a stack entry remembers"},
		{"pe_page_count", []string{"<file scope>", "ml_open", "ml_find_line", "ml_append_int"},
			"it is mf_get()'s third argument, stored so the lookup could size the page"},
		{"bh_page_count", []string{"<file scope>", "mf_new", "mf_alloc_bhdr", "ml_append_int"},
			"it is the page count a block header carries"},
		{"mf_blocknr_max", []string{"<file scope>", "mf_open", "mf_new", "mf_get"},
			"it is the counter the block numbers came from"},
		{"mf_free_first", append([]string{"<file scope>", "mf_open", "mf_close", "mf_new"}, w126FreeList...),
			"it is the head of the free list, which is keyed by block number"},
		{"mf_used_last", []string{"<file scope>", "mf_open", "mf_ins_used", "mf_rem_used"},
			"it is the tail of the used list"},
	} {
		if err := places(pl.Name, pl.expect, pl.What); err != nil {
			return nil, err
		}
	}

	L := lines()
	peRe := regexp.MustCompile(`\bpe_page_count\b`)
	peWr := regexp.MustCompile(`\bpe_page_count\s*=`)
	var peReads []int
	for i, l := range L {
		if peRe.MatchString(l) && !peWr.MatchString(l) && !strings.Contains(l, "int pe_page_count;") {
			peReads = append(peReads, i)
		}
	}
	if len(peReads) != 1 || !strings.Contains(L[peReads[0]], "page_count =") {
		return nil, p.Die("`pe_page_count` has %d readers and this phase rests on its having one, the "+
			"argument mf_get() is about to lose", len(peReads))
	}
	p.Sayf("`pe_page_count` is read ONCE in the whole file -- %s -- and that read is the third "+
		"argument of mf_get(); every other mention of it is a write", strings.TrimSpace(L[peReads[0]]))

	ulRe := regexp.MustCompile(`\bmf_used_last\b`)
	ulWr := regexp.MustCompile(`\bmf_used_last\s*=`)
	var lastw, lastr []int
	for i, l := range L {
		if ulRe.MatchString(l) {
			lastw = append(lastw, i)
			if !ulWr.MatchString(l) && !strings.Contains(l, "bhdr_T *mf_used_last;") {
				lastr = append(lastr, i)
			}
		}
	}
	if len(lastr) > 0 {
		var shown []string
		for _, i := range lastr {
			shown = append(shown, strings.TrimSpace(L[i]))
		}
		return nil, p.Die("`mf_used_last` is read at %s, and this phase removes it as a write-only field",
			strings.Join(shown, ", "))
	}
	p.Sayf("`mf_used_last` IS WRITE-ONLY: %d mentions, its declaration and %d writes and not "+
		"one read -- phase 125 took ml_setflags(), which was the last thing that walked the "+
		"used list backwards.  No warning gcc emits covers a struct member in either "+
		"direction, so it goes in this edit with its writes", len(lastw), len(lastw)-1)

	callIn := func(name string) []string {
		hs := headsOf(t)
		who := func(i int) string {
			for _, h := range hs {
				if h.a <= i && i <= h.b {
					return h.Name
				}
			}
			return "<file scope>"
		}
		re := regexp.MustCompile(`\b` + name + `\s*\(`)
		seen := map[string]bool{}
		for i, l := range lines() {
			if re.MatchString(l) {
				seen[who(i)] = true
			}
		}
		delete(seen, "<file scope>")
		delete(seen, name)
		var Out []string
		for k := range seen {
			Out = append(Out, k)
		}
		sort.Strings(Out)
		return Out
	}
	insIn, remIn := callIn("mf_ins_hash"), callIn("mf_rem_hash")
	if strings.Join(insIn, ",") != "mf_get,mf_new" || strings.Join(remIn, ",") != "mf_free,mf_get" {
		return nil, p.Die("the hash is inserted into from %s and removed from from %s, and this phase "+
			"rests on mf_new() and mf_get() being the only insertions and mf_free() and "+
			"mf_get() the only removals", edit.W126PyList(insIn), edit.W126PyList(remIn))
	}
	p.Sayf("THE HASH HOLDS EVERY LIVE BLOCK: it is inserted into by %s and removed from by "+
		"%s, and mf_get() does both in one breath to move a block to the head of the used "+
		"list -- so a lookup by a number the tree stored cannot miss, and the pointer it "+
		"would have returned is the same answer",
		strings.Join(insIn, " and "), strings.Join(remIn, " and "))

	// ---- 2. the types ---------------------------------------------------------
	for _, s := range []struct{ Old, New, What, why string }{
		{w126s0Old, w126s0New, w126s0What, w126s0Why},
		{w126s1Old, w126s1New, w126s1What, w126s1Why},
		{w126s2Old, w126s2New, w126s2What, w126s2Why},
		{w126s3Old, w126s3New, w126s3What, w126s3Why},
		{w126s4Old, w126s4New, w126s4What, w126s4Why},
	} {
		if err := swap(s.Old, s.New, s.What, s.why); err != nil {
			return nil, err
		}
	}

	// ---- 3. the memfile -------------------------------------------------------
	// Eleven functions go, and they go HERE and not to tools/sweep.sh: every one
	// names a type or a field removed above, so leaving them for the sweep would
	// leave a file that does not compile for the sweep to ask gcc about.
	all := append(append(append([]string{}, w126HashWrap...), w126FreeList...), w126HashImpl...)
	for _, name := range all {
		// WITH THE BLANK LINE UNDER IT.  The canonical text puts one between two
		// file-scope declarations, so taking the line alone would leave the
		// blank behind and the paragraph check below would refuse -- eleven
		// times over, in two runs.
		re := regexp.MustCompile(`(?m)^static [\w *]*` + name + `\([^\n]*\);\n\n`)
		m := re.FindStringIndex(t)
		if m == nil {
			return nil, p.Die("`%s` has no forward declaration in the one shape this tree writes them", name)
		}
		t = t[:m[0]] + t[m[1]:]
	}
	p.Sayf("%d forward declarations go: %s", len(all), strings.Join(all, ", "))

	for _, s := range []struct{ Old, New, What, why string }{
		{w126s5Old, w126s5New, w126s5What, w126s5Why},
		{w126s6Old, w126s6New, w126s6What, w126s6Why},
		{w126s7Old, w126s7New, w126s7What, w126s7Why},
		{w126s8Old, w126s8New, w126s8What, w126s8Why},
		{w126s9Old, w126s9New, w126s9What, w126s9Why},
	} {
		if err := swap(s.Old, s.New, s.What, s.why); err != nil {
			return nil, err
		}
	}
	n, err := cut(w126c10A, w126c10B, w126c10What)
	if err != nil {
		return nil, err
	}
	p.Sayf("the three one-line wrappers go, %d lines: %s", n, strings.Join(w126HashWrap, ", "))

	for _, s := range []struct{ Old, New, What, why string }{
		{w126s11Old, w126s11New, w126s11What, w126s11Why},
		{w126s12Old, w126s12New, w126s12What, w126s12Why},
		{w126s13Old, w126s13New, w126s13What, w126s13Why},
	} {
		if err := swap(s.Old, s.New, s.What, s.why); err != nil {
			return nil, err
		}
	}
	n, err = cut(w126c14A, w126c14B, w126c14What)
	if err != nil {
		return nil, err
	}
	p.Sayf("the free list and the hash implementation go, %d lines: %s, the two MHT_ "+
		"enumerators and %s", n, strings.Join(w126FreeList, ", "), strings.Join(w126HashImpl, ", "))

	// ---- 4. the memline -------------------------------------------------------
	for _, s := range []struct{ Old, New, What, why string }{
		{w126s15Old, w126s15New, w126s15What, w126s15Why},
		{w126s16Old, w126s16New, w126s16What, w126s16Why},
		{w126s17Old, w126s17New, w126s17What, w126s17Why},
		{w126s18Old, w126s18New, w126s18What, w126s18Why},
		{w126s19Old, w126s19New, w126s19What, w126s19Why},
		{w126s20Old, w126s20New, w126s20What, w126s20Why},
		{w126s21Old, w126s21New, w126s21What, w126s21Why},
		{w126s22Old, w126s22New, w126s22What, w126s22Why},
		{w126s23Old, w126s23New, w126s23What, w126s23Why},
		{w126s24Old, w126s24New, w126s24What, w126s24Why},
		{w126s25Old, w126s25New, w126s25What, w126s25Why},
		{w126s26Old, w126s26New, w126s26What, w126s26Why},
		{w126s27Old, w126s27New, w126s27What, w126s27Why},
		{w126s28Old, w126s28New, w126s28What, w126s28Why},
		{w126s29Old, w126s29New, w126s29What, w126s29Why},
		{w126s30Old, w126s30New, w126s30What, w126s30Why},
		{w126s31Old, w126s31New, w126s31What, w126s31Why},
	} {
		if err := swap(s.Old, s.New, s.What, s.why); err != nil {
			return nil, err
		}
	}

	// The seven remaining stores, as a partition over the two labels: every store
	// of a pointer entry's block is one of these two, and a leftover refuses.
	for _, e := range []struct{ Old, New, side string }{
		{"pe_bnum = bnum_left;", "pe_block = bp_left;", "left"},
		{"pe_bnum = bnum_right;", "pe_block = bp_right;", "right"},
	} {
		c := strings.Count(t, e.Old)
		if c < 1 {
			return nil, p.Die("no store of the %s label is left, and a split has two sides", e.side)
		}
		t = strings.ReplaceAll(t, e.Old, e.New)
		p.Sayf("%d stores of the %s label", c, e.side)
	}
	for _, old := range []string{w126pc1, w126pc2, w126pc3} {
		if strings.Count(t, old) < 1 {
			return nil, p.Die("a page-count store this phase accounts for is not there: %s",
				edit.W126PyRepr(old))
		}
		t = regexp.MustCompile(`\n *`+regexp.QuoteMeta(strings.TrimRight(old, "\n"))).
			ReplaceAllString(t, "")
	}
	c := strings.Count(t, "mf_get(mfp, ip->ip_bnum, 1)")
	if c < 1 {
		return nil, p.Die("no stack-walk fetch is left, and the three loops that climb the tree all make one")
	}
	t = strings.ReplaceAll(t, "mf_get(mfp, ip->ip_bnum, 1)", "mf_get(mfp, ip->ip_block)")
	p.Sayf("%d stack-walk fetches become mf_get(mfp, ip->ip_block)", c)

	// ---- 5. what is left, as a partition --------------------------------------
	gone := append([]string{"blocknr_T", "mf_hashitem_T", "mf_hashtab_T", "mhi_key", "mhi_next", "mhi_prev",
		"mht_mask", "mht_count", "mht_buckets", "mht_small_buckets", "mht_fixed",
		"MHT_INIT_SIZE", "MHT_LOG_LOAD_FACTOR", "MHT_GROWTH_FACTOR", "bh_hashitem",
		"pe_bnum", "ip_bnum", "pe_page_count", "bh_page_count", "mf_blocknr_max",
		"mf_free_first", "mf_used_last", "mf_hash", "page_count_left",
		"page_count_right", "bnum_left", "bnum_right"}, all...)
	var left []string
	for _, n := range gone {
		if k := mentions(t, n); k > 0 {
			left = append(left, fmt.Sprintf("%s %d", n, k))
		}
	}
	if len(left) > 0 {
		lead := "names this phase removes are"
		if len(left) == 1 {
			lead = "a name this phase removes is"
		}
		return nil, p.Die("%s still said: %s.  If they are `blocknr_T`, `mf_hashitem_T` and "+
			"`mf_hashtab_T` at one mention each, the input is UNSWEPT: phase 125 leaves "+
			"`mf_hash_free_all` standing for tools/sweep.sh and its forward declaration "+
			"names all three (`need 126 swept` in phase/STAGES.md)",
			lead, strings.Join(left, ", "))
	}
	p.Sayf("%d names are gone from the whole file: the two hash types and blocknr_T, their "+
		"nine fields and three enumerators, the four block-number fields, the two page "+
		"counts, the free list, the used tail and the eleven functions", len(gone))

	for _, pl := range []struct {
		Name   string
		expect []string
		What   string
	}{
		{"ml_root", []string{"<file scope>", "ml_open", "ml_append_int", "ml_find_line"},
			"the root is set in ml_open(), tested in ml_append_int() and is where every " +
				"descent starts"},
		{"pe_block", []string{"<file scope>", "ml_open", "ml_find_line", "ml_append_int"},
			"exactly where pe_bnum was"},
		{"ip_block", []string{"<file scope>", "ml_find_line", "ml_append_int", "ml_delete_int",
			"ml_lineadd"}, "exactly where ip_bnum was"},
	} {
		if err := places(pl.Name, pl.expect, pl.What); err != nil {
			return nil, err
		}
	}

	standing := []string{"e_didnt_get_block_nr_zero", "e_didnt_get_block_nr_one"}
	for _, name := range standing {
		if k := mentions(t, name); k != 1 {
			return nil, p.Die("`%s` has %d mentions and this edit leaves it at one, its own definition, "+
				"which is what the sweep takes", name, k)
		}
	}
	p.Sayf("%d names are left standing for tools/sweep.sh: %s -- each is an unreferenced "+
		"file-scope object, which is -Wunused-variable and the one kind of dead thing in "+
		"this phase that a tool can see", len(standing), strings.Join(standing, ", "))

	// ---- 6. the shape of what is written Out ----------------------------------
	if r := blankRuns(t); r > 0 {
		var at []string
		L := strings.Split(t, "\n")
		for i := 1; i < len(L) && len(at) < 20; i++ {
			if L[i] == "" && L[i-1] == "" {
				at = append(at, fmt.Sprintf("%d", i+1))
			}
		}
		return nil, p.Die("%d runs of two blank lines at %s -- no verification tier can see "+
			"paragraphing (CLAUDE.md, *Verification tiers*)", r, strings.Join(at, " "))
	}
	var incs2, dirs2 []int
	for i, l := range lines() {
		if w126IncLine.MatchString(l) {
			incs2 = append(incs2, i)
		}
		if w126DirLine.MatchString(l) {
			dirs2 = append(dirs2, i)
		}
	}
	if len(incs2) != len(incs) || len(dirs2) != len(incs2) {
		return nil, p.Die("the output has %d directives and %d of them are `#include`, against %d and %d",
			len(dirs2), len(incs2), len(directives), len(incs))
	}
	for i := range incs2 {
		if incs2[i] != incs2[0]+i {
			return nil, p.Die("the eleven `#include`s are not contiguous any more")
		}
	}
	nOut := strings.Count(t, "\n")
	p.Sayf("%d -> %d lines before the sweep, %d fewer, the %d `#include`s untouched and still "+
		"contiguous, and no run of two blank lines", nIn, nOut, nIn-nOut, len(incs2))

	// The edit leaves its own output beside the input, so the check can say what
	// the EDIT removed and what the SWEEP removed separately.
	if err := os.WriteFile(state+"/edit.c", []byte(t), 0o644); err != nil {
		return nil, p.Die("%v", err)
	}
	if err := os.WriteFile(state+"/boundary-in", []byte(fmt.Sprintf("%d\n", boundary+1)), 0o644); err != nil {
		return nil, p.Die("%v", err)
	}
	return []byte(t), nil
}
