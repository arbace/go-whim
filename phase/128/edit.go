package p128

// Whim phase 128 -- fold the node types.  See GOAL.md.
//
// THE MEMFILE GOES, AND WITH IT THE LAST THING BETWEEN THE TREE AND ITS NODES.
// Until this phase a memline node is TWO allocations: a `bhdr_T` of four members --
// two list pointers, a `char_u *bh_data` and a lock flag -- and, hanging off it, a
// 4,096-byte PAGE that is cast to `PTR_BL *` or `DATA_BL *` depending on the two-byte
// id at its front.  A `memfile_T` of two members owns the list head and the page size.
// After this phase
//
// struct block_hdr    { short_u bh_id; };
// struct pointer_block{ bhdr_T pb_hdr; short_u pb_count; PTR_EN pb_pointer[PB_COUNT_MAX]; };
// struct data_block   { bhdr_T db_hdr; linenr_T db_line_count; DATA_LN db_line[DB_LINE_MAX]; };
//
// and a node is ONE allocation AT ITS OWN SIZE: 1,040 bytes for a leaf and 4,088 for a
// branch, against 4,128 for either of them before.  `bhdr_T` is the node's tag and the
// first member of both, so `(PTR_BL *)hp` and `(bhdr_T *)pp` are the same address and
// the file needs no union; `memfile_T` has nothing left to hold and is gone; and the
// question "does this buffer have a memline" is `ml_root` where it was `ml_mfp`.
//
// THIS IS THE THING PHASE 127 NAMED AND DECLINED, in its own words: "Allocating a
// block at its own size means giving memfile a byte size where it has a page count ...
// It is named here so it is not lost: it would take the leaf from 112 bytes a line to
// 64, and it would MAKE AN OFF-BY-ONE IN THE CAPACITY BOUND VISIBLE, which today it is
// not."  Both halves are measured by the check rather than repeated: the leaf's node
// cost falls from 64.5 bytes a line to 16.25, and phase 127's own `cap` control -- the
// leaf capacity test widened by one -- moves 0 of 118 records on the input and 4 of 118
// here, in one run, with the input's binary built from the source beside it.
//
// WHAT THE FANOUT DOES, WHICH IS THE ONE THING THIS PHASE COULD HAVE DESTROYED.
// `pb_count_max` was computed per block as
// `(mf_page_size - offsetof(PTR_BL, pb_pointer)) / sizeof(PTR_EN)`, which is
// (4096 - 8) / 16 = 255, and it is the tree's fanout.  Phase 123's corpus reaches a
// ROOT SPLIT in exactly one of its sixteen cases, `mem_deep_jumps`, because that case
// builds 391 data blocks and 391 > 255; the other fifteen and all 102 screen cases
// reach none of it.  A phase that took `sizeof(PTR_EN)` to 8 would put the fanout at
// 511, and 391 < 511 would take root-split coverage to ZERO -- silently, because the
// instrument would still run and still pass.  So:
//
// * PTR_EN IS NOT TOUCHED.  It is `{bhdr_T *pe_block; linenr_T pe_line_count;}` here
// exactly as it was there, 16 bytes either side -- and a `static_assert` says so
// rather than leaving it to be rediscovered.
// * THE NEW STRUCT HAS THE SAME OFFSET.  `bhdr_T` is two bytes and `pb_count` two,
// so `pb_pointer` starts at 8 as it did when `pb_id`, `pb_count` and
// `pb_count_max` were three shorts.  PB_COUNT_MAX = 255 is therefore the number the
// input computes and not a number chosen, and the second `static_assert` states it
// that way: `PB_COUNT_MAX == (4096 - 8) / sizeof(PTR_EN)`, which FAILS TO COMPILE
// if a later phase narrows the entry.
//
// The check measures the consequence and not just the arithmetic: the five markers of
// phase 123's instrument, on this phase's output and on its input in the same run,
// case by case.  And a control builds this phase's output with PB_COUNT_MAX = 511 and
// reports what it costs -- 0 of 118 records move and MLSPLITPTR, MLSPLITROOT and MLDEEP
// go 1 -> 0, which is the hazard demonstrated rather than described.
//
// WHAT GOES, MEASURED ON THE INPUT AND ASSERTED AS A PARTITION AND NOT AS A COUNT
// (below, `classify`).  Every mention of every one of these is in file scope, in one of
// the ten `mf_*` functions, in one of the eleven memline functions, or -- for `ml_mfp`
// alone -- in one of the seven places outside the memline that ask whether a buffer has
// one.  A mention anywhere else is a rule this edit does not have, and it refuses.
//
// bh_next bh_prev       15 mentions   the used list, whose one consumer was mf_close
// bh_data               24            the page hanging off the header
// bh_flags               5            the lock: nothing can evict a block
// mf_used_first          6            the list head
// mf_page_size           6            the page size, read in four places
// memfile memfile_T     27            the type and its tag
// ml_mfp                25            the handle, and the "is it open" question
// pb_id db_id            9            two tags where the node has one
// pb_count_max           3            a field written once and read once
// MEMFILE_PAGE_SIZE      3
// the ten mf_* names    47            mf_open mf_close mf_new mf_get mf_put mf_free
// mf_ins_used mf_rem_used mf_alloc_bhdr mf_free_bhdr
// mfp                   55            no function holds a handle on a memfile
// page_count page_size   8
//
// AND WHAT THE SWEEP TAKES, STATED HERE AS THE OTHER HALF OF THE SAME PARTITION, which
// is phase 126's form: `BH_LOCKED`, whose only three readers were mf_new, mf_get and
// mf_put, and `e_block_was_not_locked`, the E293 mf_put raised.  The edit leaves each at
// exactly ONE mention -- its own definition -- and the check requires the sweep to take
// both to zero and to take NOTHING ELSE.
//
// NOTHING IS FREED THAT WAS NOT FREED BEFORE, and the lifetime rule is unchanged.
// `mf_close()` walked the used list at ml_close() and freed every block on it, and the
// used list was exactly the set of live nodes -- so `ml_free_tree()` walks the TREE
// instead and frees the same set.  It is recursive and the depth is the tree's height,
// which is 3 on the heaviest case the corpus has.  `mf_free()`'s two call sites in
// ml_delete_int() become `vim_free(hp)`, one allocation where there were two.  A control
// measures that removing both is invisible, for the reason GOALS.md's charter gives:
// host_free() returns without doing anything.
//
// THE ZEROING IS KEPT AND IT IS LOAD-BEARING ONCE.  mf_new() memset the page to 0 and
// the two constructors call alloc_clear() instead, which is the same act.  It matters in
// exactly one place: ml_open()'s error path runs ml_free_tree() over a root whose single
// pointer entry has not been filled in yet, and a zeroed `pe_block` is the nullptr that
// walk stops on.  A control measures that the host's arena happens to hand out zeroed
// memory anyway -- which is a fact about the host and not a promise to the core.
//
// HOW THE EDIT IS WRITTEN.  Every region is found by the function it is in and by its
// own first and last line; every local the fold stops using is removed by COMPUTING that
// its name is left mentioned once in its own function, never by listing it; and the two
// id constants are carried as they are found rather than spelled, because `(('p' << 8) +
// 't')` is phase 92's macro expansion and not this phase's text.  No line number is
// pinned and no line this phase does not itself replace is quoted.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.RegisterArgs("whim128", Edit) }

// w128Fanout is the fanout, and it is a LITERAL rather than a computation: the
// corpus's root-split coverage is measured against the number the input gives,
// and a later phase that narrowed PTR_EN would take the root split Out of the
// corpus without moving one record.  The input computes it and the output
// asserts it -- and the file gains a static_assert that fails to COMPILE.
const w128Fanout = 255

// w128Gone are the names the fold removes; w128ForSweep the two the EDIT leaves at
// exactly one mention -- their own definition -- for the SWEEP to take, stated
// here so that a sweep which took something else, or nothing, fails in the check.
var w128Gone = []string{"bh_next", "bh_prev", "bh_data", "bh_flags",
	"mf_used_first", "mf_page_size", "memfile", "memfile_T", "ml_mfp",
	"pb_id", "db_id", "pb_count_max", "MEMFILE_PAGE_SIZE",
	"mf_open", "mf_close", "mf_new", "mf_get", "mf_put", "mf_free",
	"mf_ins_used", "mf_rem_used", "mf_alloc_bhdr", "mf_free_bhdr",
	"mfp", "page_count", "page_size"}

var w128ForSweep = []string{"BH_LOCKED", "e_block_was_not_locked"}

// w128Homes: the ten mf_* functions and the eleven memline ones are this phase's
// whole subject; the seven others are the places that ask whether a buffer has a
// memline at all, which is the one question ml_mfp answered for anybody else.
var w128Homes = []string{"<file scope>",
	"mf_open", "mf_close", "mf_new", "mf_get", "mf_put", "mf_free",
	"mf_ins_used", "mf_rem_used", "mf_alloc_bhdr", "mf_free_bhdr",
	"ml_open", "ml_close", "ml_get_buf", "ml_append_int", "ml_delete_int",
	"ml_setmarked", "ml_firstmarked", "ml_clearmarked", "ml_flush_line",
	"ml_new_data", "ml_new_ptr", "ml_find_line", "ml_lineadd",
	"ml_append_flags", "ml_replace_len",
	"buf_clear_file", "buf_freeall", "create_windows", "curbuf_reusable",
	"get_nolist_virtcol", "getout", "open_buffer"}

var (
	w128PageSize = regexp.MustCompile(`enum \{ MEMFILE_PAGE_SIZE = (\d+) \};`)
	w128MfGet    = regexp.MustCompile(`^(\s*)if \(\(hp = mf_get\(mfp, ([a-z>_.\[\]-]+)\)\) == nullptr\)$`)
	w128BhData   = regexp.MustCompile(`\(([A-Za-z_][A-Za-z0-9_]*) \*\)\(([A-Za-z_][A-Za-z0-9_]*)->bh_data\)`)
	w128Ids      = regexp.MustCompile(`\b(pb_id|db_id)\b`)
	w128IdRef    = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*->(?:pb_id|db_id)\b`)
	w128MlMfp    = regexp.MustCompile(`\bml_mfp\b`)
	w128BlankRun = regexp.MustCompile(`\n\n\n`)
)

// Whim128 folds the node types: `bhdr_T` becomes `struct block_hdr { short_u
// bh_id; }`, `memfile_T` goes entirely, and a node is ONE allocation at its own
// size.
func Edit(text []byte, w io.Writer, args []string) ([]byte, error) {
	p := edit.Ph{Tag: "node", W: w}
	if len(args) != 1 {
		return nil, p.Die("usage: edit whim128 <file> <state-dir>")
	}
	state := args[0]
	t0 := string(text)
	lines := strings.Split(t0, "\n")
	nIn := len(lines)

	die := func(format string, a ...interface{}) error {
		fmt.Fprintf(w, "  node         %s\n", fmt.Sprintf(format, a...))
		return fmt.Errorf("")
	}
	say := func(format string, a ...interface{}) {
		fmt.Fprintf(w, "  node         %s\n", fmt.Sprintf(format, a...))
	}
	mentions := func(s, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(s, -1))
	}

	// --- finding things ------------------------------------------------------
	fn := func(name string) (int, int, error) {
		var hits []int
		for i, l := range lines {
			if strings.HasPrefix(l, name+"(") {
				hits = append(hits, i)
			}
		}
		if len(hits) != 1 {
			return 0, 0, die("%s is not a definition head exactly once (%d), so this edit cannot find "+
				"the function it is about", name, len(hits))
		}
		head := hits[0] - 1
		if !strings.HasPrefix(strings.TrimLeft(lines[head], " \t"), "static") {
			return 0, 0, die("%s has no `static` line above its name", name)
		}
		i := hits[0]
		for lines[i] != "{" {
			i++
		}
		depth := 0
		for {
			depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
			if depth == 0 {
				return head, i, nil
			}
			i++
		}
	}
	one := func(lo, hi int, pat string) (int, error) {
		re := regexp.MustCompile(pat)
		var hits []int
		for i := lo; i <= hi; i++ {
			if re.MatchString(lines[i]) {
				hits = append(hits, i)
			}
		}
		if len(hits) != 1 {
			return 0, die("%s matches %d lines where this edit needs exactly one",
				edit.W126PyRepr(pat), len(hits))
		}
		return hits[0], nil
	}
	allOf := func(lo, hi int, pat string) []int {
		re := regexp.MustCompile(pat)
		var Out []int
		for i := lo; i <= hi; i++ {
			if re.MatchString(lines[i]) {
				Out = append(Out, i)
			}
		}
		return Out
	}
	stmtEnd := func(a int) int {
		i := a
		for {
			depth, seen, j := 0, false, i
			for {
				depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				seen = seen || strings.Contains(lines[j], "{")
				if (seen && depth == 0) || (!seen && strings.HasSuffix(strings.TrimRight(lines[j], " \t"), ";")) {
					break
				}
				j++
			}
			k := j + 1
			for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
				k++
			}
			if k < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[k]), "else") {
				i = k
				continue
			}
			return j
		}
	}
	enclosingIn := func(ls []string, i int) string {
		for j := i; j >= 0; j-- {
			if ls[j] == "}" {
				return "<file scope>"
			}
			m := edit.W127FnHead.FindStringSubmatch(ls[j])
			if m != nil && j > 0 && strings.HasPrefix(strings.TrimLeft(ls[j-1], " \t"), "static") {
				return m[1]
			}
		}
		return "<file scope>"
	}
	enclosing := func(i int) string { return enclosingIn(lines, i) }
	defn := func(headPat string) (int, int, error) {
		a, err := one(0, len(lines)-1, headPat)
		if err != nil {
			return 0, 0, err
		}
		i := a
		for lines[i] != "{" {
			i++
		}
		depth := 0
		for {
			depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
			if depth == 0 {
				return a - 1, i, nil
			}
			i++
		}
	}
	structOf := func(name string) (int, int, []string) {
		a := edit.W127Index(lines, "struct "+name)
		b := a
		for lines[b] != "};" {
			b++
		}
		var members []string
		for _, l := range lines[a+2 : b] {
			if strings.TrimSpace(l) != "" {
				members = append(members, strings.TrimSpace(l))
			}
		}
		return a, b, members
	}
	// cut deletes [a,b], and the blank line either side of it that would pair up.
	cut := func(a, b int) {
		if b+1 < len(lines) && strings.TrimSpace(lines[b+1]) == "" {
			b++
		}
		lines = edit.W127Splice(lines, a, b+1, nil)
		if a > 0 && a < len(lines) && strings.TrimSpace(lines[a-1]) == "" &&
			strings.TrimSpace(lines[a]) == "" {
			lines = edit.W127Splice(lines, a, a+1, nil)
		}
	}

	// --- the partition, before anything is changed ---------------------------
	before := map[string]int{}
	var strays []string
	for _, name := range append(append([]string{}, w128Gone...), w128ForSweep...) {
		re := regexp.MustCompile(`\b` + name + `\b`)
		var hits []int
		for i, l := range lines {
			if re.MatchString(l) {
				hits = append(hits, i)
			}
		}
		if len(hits) == 0 {
			return nil, die("%s is not in the input at all, so this phase has already run or the "+
				"memfile is not the one it was written against", name)
		}
		// The COUNT is mentions and the classification is lines: mf_rem_used has
		// two mentions on one line, and the check re-reads the count the same way.
		before[name] = mentions(t0, name)
		for _, i := range hits {
			if !edit.Contains(w128Homes, enclosing(i)) {
				strays = append(strays, fmt.Sprintf("%s in %s (line %d)", name, enclosing(i), i+1))
			}
		}
	}
	if len(strays) > 0 {
		return nil, die("a name this phase removes is mentioned where it has no rule: %s",
			strings.Join(edit.First(strays, 5), "; "))
	}
	sum := 0
	for _, n := range w128Gone {
		sum += before[n]
	}
	say("the input mentions %s -- %d times between them -- and every mention is in file "+
		"scope, in one of the ten mf_* functions, in one of the eleven memline functions, "+
		"or in one of the seven that ask whether a buffer has a memline",
		strings.Join(w128Gone, ", "), sum)

	// The fanout, read off the INPUT rather than written here.
	pm := w128PageSize.FindStringSubmatch(t0)
	if pm == nil {
		return nil, die("the input has no MEMFILE_PAGE_SIZE enumerator")
	}
	page, _ := strconv.Atoi(pm[1])
	_, _, pbm := structOf("pointer_block")
	var tails []string
	for _, m := range pbm {
		f := strings.Fields(m)
		tails = append(tails, f[len(f)-1])
	}
	if strings.Join(tails, " ") != "pb_id; pb_count; pb_count_max; pb_pointer[1];" {
		return nil, die("struct pointer_block is not the page of entries this phase counts: %s",
			strings.Join(pbm, " "))
	}
	if (page-8)/16 != w128Fanout {
		return nil, die("the input computes a fanout of %d and this phase fixes it at %d; the corpus's "+
			"root-split coverage is measured against the first number", (page-8)/16, w128Fanout)
	}
	say("the input's fanout is (%d - 8) / 16 = %d, and that is the number this phase "+
		"fixes: phase 123 reaches a ROOT SPLIT in one of sixteen cases because that "+
		"case builds more data blocks than this", page, w128Fanout)

	// --- 1. struct block_hdr becomes the node's tag, and nothing else --------
	a, b, members := structOf("block_hdr")
	if strings.Join(members, " ") != "bhdr_T      *bh_next; bhdr_T      *bh_prev; "+
		"char_u      *bh_data; char        bh_flags;" {
		return nil, die("struct block_hdr is not the four-member page header this phase folds: %s",
			strings.Join(members, " "))
	}
	lines = edit.W127Splice(lines, a, b+1, w128b0)

	// --- 2. struct memfile has nothing left to hold --------------------------
	a, b, members = structOf("memfile")
	if strings.Join(members, " ") != "bhdr_T      *mf_used_first; unsigned    mf_page_size;" {
		return nil, die("struct memfile is not the two-member one this phase folds away: %s",
			strings.Join(members, " "))
	}
	cut(a, b)
	// The alignment is phase 126's: this typedef is text THAT phase writes, not
	// text phase 0 printed, so it is read here as phase 126 spells it.
	i := edit.W127Index(lines, "typedef struct memfile      memfile_T;")
	lines = edit.W127Splice(lines, i, i+1, nil)

	// --- 3. memline_T loses its handle on one --------------------------------
	i = edit.W127Index(lines, "    memfile_T   *ml_mfp;")
	if lines[i+1] != "    bhdr_T      *ml_root;" {
		return nil, die("ml_mfp is not the line above ml_root, so the memline is not the one this " +
			"edit reads")
	}
	lines = edit.W127Splice(lines, i, i+1, nil)

	// --- 4. the memfile layer itself -----------------------------------------
	i = edit.W127Index(lines, fmt.Sprintf("enum { MEMFILE_PAGE_SIZE = %d };", page))
	if lines[i+2] != "static void mf_ins_used(memfile_T *, bhdr_T *);" {
		return nil, die("the memfile block does not start where this edit expects")
	}
	// The run of prototypes has a blank line between each pair -- the canonical
	// text writes one between two file-scope declarations -- so the walk steps
	// over a blank as well as over a prototype, and stops on the first line that
	// is neither.  That leaves the blank under the last prototype inside the
	// cut, which is where it has to be: the enum above has one over it.
	j := i + 2
	for j < len(lines) && (lines[j] == "" ||
		(strings.HasPrefix(lines[j], "static ") && strings.Contains(lines[j], "mf_"))) {
		j++
	}
	cut(i, j-1)

	for _, headPat := range []string{
		`^mf_open\(void\)$`,
		`^mf_close\(memfile_T \*mfp, int del_file\)$`,
		`^mf_new\(memfile_T \*mfp, int page_count\)$`,
		`^mf_get\(memfile_T \*mfp, bhdr_T \*hp\)$`,
		`^mf_put\(bhdr_T \*hp\)$`,
		`^mf_free\(memfile_T \*mfp, bhdr_T \*hp\)$`,
		`^mf_ins_used\(memfile_T \*mfp, bhdr_T \*hp\)$`,
		`^mf_rem_used\(memfile_T \*mfp, bhdr_T \*hp\)$`,
		`^mf_alloc_bhdr\(memfile_T \*mfp, int page_count\)$`,
		`^mf_free_bhdr\(bhdr_T \*hp\)$`,
	} {
		aa, bb, err := defn(headPat)
		if err != nil {
			return nil, err
		}
		cut(aa, bb)
	}

	// --- 5. a branch is a counted array of entries and not a page ------------
	a, b, _ = structOf("pointer_block")
	lines = edit.W127Splice(lines, a, b+1, w128b1)

	a, b, members = structOf("data_block")
	tails = nil
	for _, m := range members {
		f := strings.Fields(m)
		tails = append(tails, f[len(f)-1])
	}
	if strings.Join(tails, " ") != "db_id; db_line_count; db_line[DB_LINE_MAX];" {
		return nil, die("struct data_block is not the leaf phase 127 left: %s",
			strings.Join(members, " "))
	}
	lines[a+2] = w128s0

	i, err := one(0, len(lines)-1, `^static_assert\(sizeof\(DATA_BL\) <= MEMFILE_PAGE_SIZE,`)
	if err != nil {
		return nil, err
	}
	asserts := append([]string{}, w128b2...)
	asserts[1] = fmt.Sprintf(asserts[1], page)
	lines = edit.W127Splice(lines, i, i+1, asserts)

	// --- 6. the two constructors allocate a node at its own size -------------
	// The id constants are CARRIED Out of the definitions being replaced and
	// never spelled.
	carriedID := func(headPat, field string) (string, error) {
		lo, hi, err := defn(headPat)
		if err != nil {
			return "", err
		}
		k, err := one(lo, hi, `->`+field+` =`)
		if err != nil {
			return "", err
		}
		v := strings.SplitN(lines[k], "=", 2)[1]
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimRight(v, " \t"), ";")), nil
	}
	da, err := carriedID(`^ml_new_data\(memfile_T \*mfp\)$`, "db_id")
	if err != nil {
		return nil, err
	}
	pt, err := carriedID(`^ml_new_ptr\(memfile_T \*mfp\)$`, "pb_id")
	if err != nil {
		return nil, err
	}
	if !strings.Contains(da, "<< 8") || !strings.Contains(pt, "<< 8") || da == pt {
		return nil, die("the two block ids are not the two distinct constants this edit carries: "+
			"%s and %s", edit.W126PyRepr(da), edit.W126PyRepr(pt))
	}
	fill := func(rows []string) []string {
		Out := make([]string, len(rows))
		for i, r := range rows {
			if strings.Contains(r, "%[1]s") || strings.Contains(r, "%[2]s") {
				Out[i] = fmt.Sprintf(r, da, pt)
			} else {
				Out[i] = r
			}
		}
		return Out
	}

	i = edit.W127Index(lines, "static bhdr_T *ml_new_data(memfile_T *);")
	lines[i] = w128s1
	i = edit.W127Index(lines, "static bhdr_T *ml_new_ptr(memfile_T *);")
	lines[i] = w128s2

	aa, bb, err := defn(`^ml_new_data\(memfile_T \*mfp\)$`)
	if err != nil {
		return nil, err
	}
	lines = edit.W127Splice(lines, aa, bb+1, fill(w128b3))
	if aa, bb, err = defn(`^ml_new_ptr\(memfile_T \*mfp\)$`); err != nil {
		return nil, err
	}
	lines = edit.W127Splice(lines, aa, bb+1, fill(w128b4))

	// --- 7. a closed buffer gives its nodes back by walking the tree ---------
	lo, _, err := fn("ml_alloc_line")
	if err != nil {
		return nil, err
	}
	lines = edit.W127Splice(lines, lo, lo, fill(w128b5))

	// --- 8. ml_open opens nothing --------------------------------------------
	lo, hi, err := fn("ml_open")
	if err != nil {
		return nil, err
	}
	if a, err = one(lo, hi, `^    mfp = mf_open\(\);$`); err != nil {
		return nil, err
	}
	b = stmtEnd(a + 1)
	c := b + 1
	for strings.TrimSpace(lines[c]) == "" {
		c++
	}
	if lines[c] != "    buf->b_ml.ml_mfp = mfp;" {
		return nil, die("mf_open is not followed by its failure arm and the assignment to ml_mfp")
	}
	lines = edit.W127Splice(lines, a, c+1, nil)

	if lo, hi, err = fn("ml_open"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^    if \(\(hp = ml_new_ptr\(mfp\)\) == nullptr\)$`); err != nil {
		return nil, err
	}
	lines[i] = w128s3
	if i, err = one(lo, hi, `^    pp = \(PTR_BL \*\)\(hp->bh_data\);$`); err != nil {
		return nil, err
	}
	lines[i] = w128s4
	if i, err = one(lo, hi, `^    mf_put\(hp\);$`); err != nil {
		return nil, err
	}
	lines = edit.W127Splice(lines, i, i+1, nil)

	if lo, hi, err = fn("ml_open"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^    if \(\(hp = ml_new_data\(mfp\)\) == nullptr\)$`); err != nil {
		return nil, err
	}
	lines[i] = w128s5
	if i, err = one(lo, hi, `->pb_pointer\[0\]\.pe_block = hp;$`); err != nil {
		return nil, err
	}
	lines[i] = strings.ReplaceAll(lines[i], "ml_root->bh_data", "ml_root")
	if i, err = one(lo, hi, `^    dp = \(DATA_BL \*\)\(hp->bh_data\);$`); err != nil {
		return nil, err
	}
	lines[i] = w128s6

	if lo, hi, err = fn("ml_open"); err != nil {
		return nil, err
	}
	if a, err = one(lo, hi, `^error:$`); err != nil {
		return nil, err
	}
	if b, err = one(lo, hi, `^    buf->b_ml\.ml_mfp = nullptr;$`); err != nil {
		return nil, err
	}
	lines = edit.W127Splice(lines, a, b+1, w128b6)

	// --- 9. ml_close frees the tree it has ------------------------------------
	if lo, hi, err = fn("ml_close"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^    if \(buf->b_ml\.ml_mfp == nullptr\)$`); err != nil {
		return nil, err
	}
	lines[i] = w128s7
	if i, err = one(lo, hi, `^    mf_close\(buf->b_ml\.ml_mfp, del_file\);$`); err != nil {
		return nil, err
	}
	lines[i] = w128s8
	if i, err = one(lo, hi, `^    buf->b_ml\.ml_mfp = nullptr;$`); err != nil {
		return nil, err
	}
	lines[i] = w128s9

	// --- 10. the three functions that took a handle on the memfile -----------
	if lo, hi, err = fn("ml_append_int"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^    mfp = buf->b_ml\.ml_mfp;$`); err != nil {
		return nil, err
	}
	if !regexp.MustCompile(`^    page_size = mfp->mf_page_size;$`).MatchString(lines[i+1]) {
		return nil, die("the page size is not read where this edit expects")
	}
	lines = edit.W127Splice(lines, i, i+2, nil)
	if strings.TrimSpace(lines[i-1]) == "" && strings.TrimSpace(lines[i]) == "" {
		lines = edit.W127Splice(lines, i, i+1, nil)
	}

	if lo, hi, err = fn("ml_delete_int"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^    mfp = buf->b_ml\.ml_mfp;$`); err != nil {
		return nil, err
	}
	b = stmtEnd(i + 1)
	if !strings.Contains(strings.Join(lines[i:b+1], "\n"), "return FAIL") {
		return nil, die("the memfile null test in ml_delete_int is not the arm this edit rewrites")
	}
	lines = edit.W127Splice(lines, i, b+1, w128b7)

	if lo, hi, err = fn("ml_find_line"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^    mfp = buf->b_ml\.ml_mfp;$`); err != nil {
		return nil, err
	}
	// The statement alone: the canonical text writes no blank line inside a
	// function, so there is none under it to take.
	lines = edit.W127Splice(lines, i, i+1, nil)

	// --- 11. everywhere else, ml_mfp was the question "is this buffer loaded"
	for i, l := range lines {
		if w128MlMfp.MatchString(l) {
			lines[i] = w128MlMfp.ReplaceAllString(l, "ml_root")
		}
	}

	// --- 12. a node is reached without its header ----------------------------
	for i, l := range lines {
		if strings.Contains(l, "->bh_data") {
			lines[i] = w128BhData.ReplaceAllString(l, "($1 *)($2)")
		}
	}

	// --- 13. one tag, in the node --------------------------------------------
	for i, l := range lines {
		if w128Ids.MatchString(l) {
			lines[i] = w128IdRef.ReplaceAllString(l, "hp->bh_id")
		}
	}

	// --- 14. nothing can evict a block, so nothing locks one -----------------
	for _, name := range []string{"ml_append_int", "ml_delete_int", "ml_find_line", "ml_lineadd"} {
		for {
			lo, hi, err = fn(name)
			if err != nil {
				return nil, err
			}
			hits := allOf(lo, hi, `^\s*mf_put\([^)]*\);$`)
			if len(hits) == 0 {
				break
			}
			i = hits[0]
			lines = edit.W127Splice(lines, i, i+1, nil)
			if i > 0 && i < len(lines) && strings.TrimSpace(lines[i-1]) == "" &&
				strings.TrimSpace(lines[i]) == "" {
				lines = edit.W127Splice(lines, i, i+1, nil)
			}
		}
	}

	// --- 15. a block is its own pointer --------------------------------------
	for {
		var hits []int
		mfGet := regexp.MustCompile(`\bmf_get\(`)
		for i, l := range lines {
			if mfGet.MatchString(l) {
				hits = append(hits, i)
			}
		}
		if len(hits) == 0 {
			break
		}
		i = hits[0]
		m := w128MfGet.FindStringSubmatch(lines[i])
		if m == nil {
			return nil, die("an mf_get is not the guarded assignment this edit rewrites: %s", lines[i])
		}
		lines = edit.W127Splice(lines, i, stmtEnd(i)+1, []string{m[1] + "hp = " + m[2] + ";"})
	}

	// --- 16. ml_append_int ----------------------------------------------------
	if lo, hi, err = fn("ml_append_int"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^        if \(\(hp_new = ml_new_data\(mfp\)\) == nullptr\)$`); err != nil {
		return nil, err
	}
	lines[i] = w128s10
	if i, err = one(lo, hi, `if \(pp->pb_count < pp->pb_count_max\)$`); err != nil {
		return nil, err
	}
	lines[i] = strings.ReplaceAll(lines[i], "pp->pb_count_max", "PB_COUNT_MAX")
	if i, err = one(lo, hi, `^                hp_new = ml_new_ptr\(mfp\);$`); err != nil {
		return nil, err
	}
	lines[i] = w128s11
	// The root split copied the whole PAGE, which is how a node's contents moved
	// while its header sat somewhere else.  The header is IN the node now.
	if i, err = one(lo, hi, `^ *musl_memmove\(\(char \*\)\(pp_new\), \(char \*\)\(pp\), \(usize\)page_size\);$`); err != nil {
		return nil, err
	}
	lines = edit.W127Splice(lines, i, i+1, w128b8)

	// --- 17. ml_delete_int releases a node by freeing it ---------------------
	if lo, hi, err = fn("ml_delete_int"); err != nil {
		return nil, err
	}
	freed := allOf(lo, hi, `\bmf_free\(mfp, hp\);$`)
	if len(freed) != 2 {
		return nil, die("ml_delete_int releases a block in %d places and this edit knows two", len(freed))
	}
	for _, i := range freed {
		lines[i] = strings.ReplaceAll(lines[i], "mf_free(mfp, hp);", "vim_free(hp);")
	}

	// --- 18. ml_find_line reads the tag off the node -------------------------
	if lo, hi, err = fn("ml_find_line"); err != nil {
		return nil, err
	}
	if a, err = one(lo, hi, `^\s+dp = \(DATA_BL \*\)\(hp\);$`); err != nil {
		return nil, err
	}
	if !regexp.MustCompile(`if \(hp->bh_id ==\s`).MatchString(lines[a+1]) {
		return nil, die("the leaf test does not follow the cast it replaces: %s", lines[a+1])
	}
	lines = edit.W127Splice(lines, a, a+1, nil)
	if lo, hi, err = fn("ml_find_line"); err != nil {
		return nil, err
	}
	if i, err = one(lo, hi, `^\s+pp = \(PTR_BL \*\)\(dp\);$`); err != nil {
		return nil, err
	}
	lines[i] = strings.ReplaceAll(lines[i], "(dp)", "(hp)")
	if lo, hi, err = fn("ml_find_line"); err != nil {
		return nil, err
	}
	if a, err = one(lo, hi, `^error_block:$`); err != nil {
		return nil, err
	}
	if lines[a+1] != "error_noblock:" {
		return nil, die("error_block and error_noblock are not adjacent once the lock has gone")
	}
	lines = edit.W127Splice(lines, a+1, a+2, nil)

	// --- 19. the locals the fold stopped using -------------------------------
	var dropped []string
	for _, name := range []string{"ml_open", "ml_close", "ml_get_buf", "ml_append_int", "ml_delete_int",
		"ml_setmarked", "ml_firstmarked", "ml_clearmarked", "ml_flush_line",
		"ml_new_data", "ml_new_ptr", "ml_find_line", "ml_lineadd"} {
		for {
			lo, hi, err = fn(name)
			if err != nil {
				return nil, err
			}
			Body := strings.Join(lines[lo:hi+1], "\n")
			found := false
			for i := lo; i <= hi; i++ {
				m := edit.W127Decl.FindStringSubmatch(lines[i])
				if m == nil {
					continue
				}
				if edit.Contains(edit.W127NotDecl, strings.Fields(lines[i])[0]) {
					continue
				}
				if mentions(Body, m[1]) == 1 {
					dropped = append(dropped, name+":"+m[1])
					lines = edit.W127Splice(lines, i, i+1, nil)
					found = true
					break
				}
			}
			if !found {
				break
			}
		}
	}
	say("%d locals the fold stopped using, found by counting their own name: %s",
		len(dropped), strings.Join(dropped, " "))

	// --- the partition again, on the output ----------------------------------
	t := strings.Join(lines, "\n")
	for _, name := range w128Gone {
		if k := mentions(t, name); k != 0 {
			return nil, die("%s survives the edit with %d mentions", name, k)
		}
	}
	for _, name := range w128ForSweep {
		if k := mentions(t, name); k != 1 {
			return nil, die("%s is left at %d mentions and the edit leaves exactly one -- its own "+
				"definition -- for the sweep to take", name, k)
		}
	}
	for _, name := range []string{"PB_COUNT_MAX", "bh_id", "pb_hdr", "db_hdr", "ml_free_tree"} {
		if mentions(t, name) == 0 {
			return nil, die("%s is not in the output, so the replacement did not land", name)
		}
	}
	// THE OPEN-BUFFER PREDICATE MOVED 1:1, stated as a partition over the
	// FUNCTIONS that ask it rather than as a count of mentions.
	askers := func(text, field string) []string {
		ls := strings.Split(text, "\n")
		re := regexp.MustCompile(`\b` + field + `\b *(==|!=) *nullptr`)
		seen := map[string]bool{}
		for i, l := range ls {
			if re.MatchString(l) {
				seen[enclosingIn(ls, i)] = true
			}
		}
		var Out []string
		for k := range seen {
			Out = append(Out, k)
		}
		sort.Strings(Out)
		return Out
	}
	was, now := askers(t0, "ml_mfp"), askers(t, "ml_root")
	if len(askers(t0, "ml_root")) > 0 {
		return nil, die("ml_root is already compared with nullptr in the input, so this edit cannot " +
			"say that the open-buffer question moved onto it")
	}
	wantSet := map[string]bool{"ml_delete_int": true}
	for _, v := range was {
		wantSet[v] = true
	}
	var want []string
	for k := range wantSet {
		want = append(want, k)
	}
	sort.Strings(want)
	if strings.Join(want, "\x00") != strings.Join(now, "\x00") {
		return nil, die("the \"is this buffer's memline open\" question is asked in %s and it was asked "+
			"in %s; the only one this edit adds is ml_delete_int, which asked it through a "+
			"local copy of the handle", edit.W126PyList(now), edit.W126PyList(was))
	}
	say("the question \"does this buffer have a memline\" moved from ml_mfp to ml_root in "+
		"all %d functions that asked it, plus ml_delete_int, which asked it through its own "+
		"copy of the handle -- and ml_root was compared with nullptr in none of them before",
		len(was))
	// THE INPUT IS ASKED FIRST, and that is what `need 128 swept` is.
	if w128BlankRun.MatchString(t0) {
		return nil, die("the input already has a run of two blank lines, so this edit cannot say it " +
			"left none: it needs swept text (phase/STAGES.md, `need 128 swept`)")
	}
	if w128BlankRun.MatchString(t) {
		return nil, die("the edit left a run of two blank lines, which no verification tier can see")
	}

	var gone strings.Builder
	for _, n := range w128Gone {
		fmt.Fprintf(&gone, "%s\t%d\n", n, before[n])
	}
	if err := os.WriteFile(state+"/gone", []byte(gone.String()), 0o644); err != nil {
		return nil, die("%v", err)
	}
	if err := os.WriteFile(state+"/forsweep", []byte(strings.Join(w128ForSweep, "\n")+"\n"), 0o644); err != nil {
		return nil, die("%v", err)
	}
	if err := os.WriteFile(state+"/fanout", []byte(fmt.Sprintf("%d\n", w128Fanout)), 0o644); err != nil {
		return nil, die("%v", err)
	}
	say("%d -> %d lines: a node is ONE allocation at its own size, `bhdr_T` is its tag and "+
		"the first member of both kinds, and there is no memfile", nIn, len(lines))
	return []byte(t), nil
}
