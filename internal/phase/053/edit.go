package p053

// Whim phase 53 (formerly 125) -- the swap file's residue.  See GOAL.md and GOALS.md II.4a.
//
// THE FILESYSTEM WENT AT PHASES 28 TO 32 AND THE SWAP FILE'S MACHINERY DID NOT.  memline
// and memfile still keep the bookkeeping of a file that is written to a disk: a header
// block with the editor's version and the buffer's name in it, a translation table for
// blocks that have not been written out yet, a dirtiness state machine, and a record of
// where each block's lines USED to be.  None of it can be reached and none of it is read.
//
// EVERY ONE OF THESE FOUR GROUPS IS INVISIBLE TO EVERY TOOL IN tools/, AND FOR ONE
// REASON: they are all WRITTEN.  tools/deadfields.py takes a field named nowhere outside
// its own type, and each of these is named; tools/deadsweep.py asks gcc, and gcc has no
// warning for a struct member nothing reads, for an enumerator that is only ever OR-ed
// into a word nothing tests, or for a file-scope object that is read and never assigned.
// Run against the input, deadfields.py reports 0 fields.  So this is an EDIT and not a
// sweep, and the division of labour is stated rather than hoped for: the edit takes
// everything that is written, and the check names what is left standing for the sweep and
// what the sweep then found.
//
// 1  BLOCK ZERO.  `struct block0` is the swap file's header.  Its fields -- the two
// identifying bytes, the version string, the page size, the file name and the four
// magic numbers -- are WRITTEN in ml_open() and ml_setflags() and READ NOWHERE, in
// any build of whim-vim, and the edit proves that as a partition over every mention
// of every one of them: a declaration, an assignment, or a call that copies INTO the
// field.  Nothing may yield a value.  The field list is read out of the struct rather
// than typed here, because phase 49 already took `b0_pid` and a typed list would
// be a phase out of date.
//
// THE TWO SURVIVING BLOCKS MOVE DOWN BY ONE.  ml_open() allocated block nr 0 for the
// header, 1 for the root pointer block and 2 for the first data block, and with the
// header gone the pointer block is 0 and the data block 1.  Five numbers say so, and
// two of them are outside ml_open(): ml_find_line() starts its descent at the root,
// and ml_append_int() recognises the root by its block number when a split reaches
// the top of the tree and the root has to be kept where the reader starts.  The
// check's controls are on those two.
//
// 2  NEGATIVE BLOCK NUMBERS.  mf_trans_add() returns before it does anything unless a
// block number is negative, and a block number is negative only if mf_new() is called
// with `negative` TRUE.  THE CHAIN IS COMPUTED HERE AND NOT ASSERTED FROM A SURVEY:
// mf_new()'s callers pass FALSE or ml_new_data()'s own parameter; ml_new_data()'s
// callers pass FALSE or `flags & ML_APPEND_NEW`; ML_APPEND_NEW is set only by
// ml_append()'s `newfile`; and `newfile` is FALSE at every ml_append() call site in
// the file.  So `negative` is FALSE at every reachable call, no block number is ever
// negative, mf_trans_add() is a no-op, the translation table is always empty and
// ml_find_line()'s `bnum < 0` arm is unreachable.  The phase removes the whole
// island: two functions, three memfile fields, the parameter, and the two flags whose
// only purpose was to choose between marking a block dirty and calling the no-op.
//
// 3  THE DIRTINESS, WRITE-ONLY IN ALL THREE OF ITS LAYERS.  `mf_dirty` is a three-valued
// field with six writes and two reads, and EACH READ IS THE CONDITION OF AN `if`
// WHOSE ONLY STATEMENT WRITES THE FIELD AGAIN -- so nothing outside the field ever
// learns its value, which the edit checks structurally.  `BH_DIRTY` is set three
// times and never tested: `bh_flags` is read in exactly one place and that read
// tests BH_LOCKED.  And ML_LOCKED_DIRTY and ML_LOCKED_POS, the memline's own pair,
// are read at exactly one place between them -- the two arguments mf_put() is about
// to stop taking.  mf_put() becomes `mf_put(bhdr_T *hp)`, which clears BH_LOCKED and
// is the whole of what it did that anything reads.
//
// 4  pe_old_lnum, AND THE TWO LOCALS THAT EXIST ONLY TO FEED IT.  Seven writes, no read.
// Four of the seven are the only statement of an `if`, and the two variables those
// `if`s test are computed by a fourteen-line branch and used nowhere else -- so once
// the field goes, gcc says `-Wunused-but-set-variable` for both, which
// tools/deadsweep.py does not act on (CLAUDE.md, *Audit for dead code*).  The same is
// true of ml_find_line()'s `dirty`, which was mf_put()'s third argument.  All three
// are therefore the edit's.
//
// AND mf_dont_release, WHICH WAS A CONSTANT (since the reform's D5 it falls
// out at phase 1, and this phase no longer names it).  `static int
// mf_dont_release = FALSE;`, read twice and ASSIGNED NOWHERE IN THE FILE.  No warning gcc emits covers a
// file-scope object in either direction, so nothing here has ever been able to see
// it.
//
// THE FANOUT CHANGES AND THAT IS NOT A BEHAVIOUR.  `pe_old_lnum` is a member of PTR_EN,
// the pointer-block entry, so taking it makes each entry smaller and more of them fit in
// a page.  The tree the editor builds for a given buffer is therefore shaped differently
// after this phase, which is a representation and not an observable -- and the check does
// not leave that to be believed: it drives both binaries to sixty thousand lines, where
// an instrumented build says the root pointer block really does overflow and the
// root-preserving branch really does run, and requires the two to draw the same screen.
//
// WHAT THIS PHASE DOES NOT TAKE.  `BH_LOCKED` looks like BH_DIRTY's twin and is not: it
// is read, by mf_put()'s `e_block_was_not_locked` assertion.  Measured -- a binary whose
// mf_put() SETS the bit instead of clearing it draws exactly the same 102 screen cases,
// because the only reader is an internal-error test that then never fires -- so it is
// unreachable EVIDENCE and not unreachable code, and the bit stays.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check records from it.
// The edit also leaves its own output in $state/edit.c, so the check can state what the
// EDIT removed and what the SWEEP removed separately rather than as one number.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, *R1 as built*): the acts are the
// graph's -- the preamble a run of ml_open()'s items cut, the numbers and
// the messages rewritten in place, mf_new(), ml_new_data(), ml_append() and
// mf_put() narrowed by PARAM (every call's argument with them), mf_put()'s
// body built, the definitions, the prototype, the typedef and the member
// deleted, the writes cut by pattern -- and every partition is the text's,
// its regular expressions on the lines of the forms that say the name,
// found by edge (graph.FormLines).  The line counts are the C view's.

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim53", Edit) }

type w53Class struct {
	label string
	Pat   *regexp.Regexp
}

// w53Left are the names this edit leaves STANDING for tools/sweep.sh, each a kind
// sweep finds.  Nothing that is WRITTEN is among them: that is the whole division
// of labour, and a write-only field or a set-and-never-tested bit is the edit's
// because no tool in tools/ can see one.
var w53Left = map[string]string{
	"set_b0_fname":             "a static function with no caller left",
	"long_to_char":             "a static function with no caller left",
	"mf_hash_free_all":         "a static function with no caller left",
	"ml_setflags":              "a forward declaration of a function that is gone",
	"Version":                  "a static object nothing reads",
	"e_didnt_get_block_nr_two": "a static object nothing reads",
	"ZERO_BL":                  "a type nothing reaches",
	"NR_TRANS":                 "a type nothing reaches",
	"BH_DIRTY":                 "an enumerator nothing mentions",
	"MFS_ZERO":                 "an enumerator nothing mentions",
	"ML_APPEND_NEW":            "an enumerator nothing mentions",
	"ML_LOCKED_POS":            "an enumerator nothing mentions",
	"ML_LOCKED_DIRTY":          "an enumerator nothing mentions",
	"b0p":                      "a local nothing reads",
	"bnum2":                    "a local nothing reads",
	"dirty":                    "a local nothing reads",
	"lnum_left":                "a local nothing reads",
	"lnum_right":               "a local nothing reads",
	"mf_trans":                 "a member nothing names",
	"mf_blocknr_min":           "a member nothing names",
	"mf_neg_count":             "a member nothing names",
	"pe_old_lnum":              "a member nothing names",
}

// w53Locals are the functions whose locals and parameters a name of this
// phase may be: where its mentions are looked for besides the file scope.
var w53Locals = []string{"ml_open", "set_b0_fname", "ml_find_line", "ml_append_int", "ml_append",
	"mf_new", "ml_new_data", "mf_put"}

// Edit takes the swap file's residue: four groups of bookkeeping that is
// WRITTEN and never read, which is exactly why no tool in tools/ can see any of
// it -- deadfields.py takes a field named nowhere outside its own type, and gcc
// has no warning for a file-scope object in either direction.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	p := edit.Ph{Tag: "swapres", W: w}
	if len(args) != 1 {
		return p.Die("usage: edit whim53 <file> <state-dir>")
	}
	state := args[0]
	v := graph.NewVerbs("swapres", e, io.Discard)

	// decls are a name's declarations: the file's, a member's, a local's or
	// parameter's in w53Locals' functions.
	decls := func(name string) []*graph.Node {
		ds := append(e.FileDecls(name), e.MemberDecls(name)...)
		for _, en := range e.Decls(name) {
			if !e.Live(en) {
				continue
			}
			dup := false
			for _, d := range ds {
				dup = dup || d == en
			}
			if !dup {
				ds = append(ds, en)
			}
		}
		for _, f := range w53Locals {
			if fn := e.Defn(f); fn != nil {
				ds = append(ds, e.LocalDecls(fn, name)...)
			}
		}
		return ds
	}
	// lines are the lines of the forms that say any of the names.
	lines := func(names ...string) ([]graph.FormLine, error) {
		var ns []*graph.Node
		for _, n := range names {
			ns = append(ns, e.AndUses(decls(n)...)...)
		}
		return e.FormLines(ns...)
	}
	mentions := func(name string) (int, error) {
		ls, err := lines(name)
		if err != nil {
			return 0, err
		}
		var b strings.Builder
		for _, l := range ls {
			b.WriteString(l.Text)
			b.WriteByte('\n')
		}
		return edit.WordPatternCount(b.String(), name), nil
	}
	// partition: every line that says `name` falls in exactly one class, and a
	// leftover refuses.
	partition := func(name string, classes []w53Class, what string) (map[string]int, error) {
		word := regexp.MustCompile(`\b(?:` + name + `)\b`)
		ls, err := lines(strings.Split(name, "|")...)
		if err != nil {
			return nil, p.Die("%v", err)
		}
		Out := map[string]int{}
		seen := 0
		for _, l := range ls {
			if !word.MatchString(l.Text) {
				continue
			}
			seen++
			var hit []string
			for _, c := range classes {
				if c.Pat.MatchString(l.Text) {
					hit = append(hit, c.label)
				}
			}
			if len(hit) != 1 {
				var labels []string
				for _, c := range classes {
					labels = append(labels, c.label)
				}
				return nil, p.Die("`%s` -- %s -- falls in %d of the %d classes this phase "+
					"accounts for (%s), and every mention must fall in exactly one",
					name, strings.TrimSpace(l.Text), len(hit), len(classes),
					strings.Join(labels, ", "))
			}
			Out[hit[0]]++
		}
		var parts []string
		for _, c := range classes {
			if Out[c.label] == 0 {
				return nil, p.Die("the class `%s` of `%s` holds no mention of it, so it is not a class of "+
					"this tree", c.label, name)
			}
			parts = append(parts, fmt.Sprintf("%s %d", c.label, Out[c.label]))
		}
		p.Sayf("%s: %d mentions, every one in a class this phase accounts for -- %s",
			what, seen, strings.Join(parts, ", "))
		return Out, nil
	}
	failed := func() error { return v.Err }
	// calls are the calls of the function named, by edge.
	calls := func(name string) []*graph.Node {
		var out []*graph.Node
		for _, d := range e.FileDecls(name) {
			for _, u := range e.Uses(d) {
				if c := e.Parent(u); c != nil && c.Is("call") && c.Kids[1] == u {
					out = append(out, c)
				}
			}
		}
		return out
	}
	drop := func(fn string, params ...string) []graph.ParamDrop {
		var ds []graph.ParamDrop
		for _, prm := range params {
			ds = append(ds, graph.ParamDrop{Decl: e.FileDecls(fn)[0], I: e.ParamIndex(fn, prm)})
		}
		return ds
	}

	t0, err := e.Graph().C()
	if err != nil {
		return err
	}
	linesBefore := strings.Count(string(t0), "\n")
	incs := e.Includes()
	if len(incs) == 0 {
		return p.Die("the input has no #include")
	}
	firstInc := 1 + strings.Count(string(t0[:strings.Index(string(t0), "\n#include ")+1]), "\n")
	p.Sayf("the input is %d lines with %d preprocessor directives, the first at line %d -- "+
		"this phase adds no directive and removes none",
		linesBefore, len(incs), firstInc)

	// ==== PART 1 -- THE SWAP FILE'S HEADER BLOCK ===============================
	// The field list is READ OUT OF THE STRUCT and not written here: phase 49
	// already took `b0_pid`, so a list typed from a survey would be one name long.
	var sb *graph.Node
	for _, f := range e.Graph().Forms {
		if f.Is("struct") && graph.Tag(f) == "block0" {
			if sb != nil {
				return p.Die("`struct block0`: 2 lines where this phase needs 1")
			}
			sb = f
		}
	}
	if sb == nil {
		return p.Die("`struct block0`: 0 lines where this phase needs 1")
	}
	var fields []string
	for _, m := range graph.Members(sb) {
		if !m.IsList() || m.Kids[0].IsList() {
			return p.Die("`struct block0` has a member this phase cannot read")
		}
		fields = append(fields, m.Head())
	}
	okF := len(fields) >= 5
	for _, f := range fields {
		if !strings.HasPrefix(f, "b0_") {
			okF = false
		}
	}
	if !okF {
		return p.Die("`struct block0` has members %s, and this phase was written against a header "+
			"block whose every field is a `b0_`", strings.Join(fields, " "))
	}
	p.Sayf("`struct block0` is the swap file's header block and has %d fields: %s",
		len(fields), strings.Join(fields, " "))
	any := strings.Join(fields, "|")
	cls, err := partition(any, []w53Class{
		{"its declaration", regexp.MustCompile(`^    (char_u|long|int|short) +(` + any + `) *(\[[^]]*\])?;$`)},
		{"assigned", regexp.MustCompile(`^\s*b0p-> *(` + any + `)\b *(\[[^]]*\])? *=[^=]`)},
		{"copied into", regexp.MustCompile(`^\s*musl_(memmove|strncpy)\(\(char \*\)\((b0p->(` + any + `))\b|` +
			`^\s*long_to_char\([^,]+, b0p->(` + any + `)\);$`)},
	}, "the fields of `struct block0`")
	if err != nil {
		return err
	}
	if cls["its declaration"] != len(fields) {
		return p.Die("`struct block0` declares %d of the %d fields this phase was written against",
			cls["its declaration"], len(fields))
	}
	nwrite := cls["assigned"] + cls["copied into"]

	// ml_open()'s preamble: from the mf_new() that took block nr 0 to the
	// item before the ml_new_ptr() allocation, a run of its items.
	open := e.Defn("ml_open")
	if open == nil {
		return p.Die("`ml_open` is not defined")
	}
	var start, stop int = -1, -1
	body := graph.Body(open)
	for i, it := range body {
		if graph.Matches(clisp.MustPattern("(if (== (= hp (call mf_new _*)) nullptr) _*)"), it) {
			if start >= 0 {
				return p.Die("ml_open()'s first block allocation: 2 lines where this phase needs 1")
			}
			start = i
		}
		if graph.Matches(clisp.MustPattern("(if (== (= hp (call ml_new_ptr _*)) nullptr) _*)"), it) {
			if stop >= 0 {
				return p.Die("ml_open()'s pointer-block allocation: 2 lines where this phase needs 1")
			}
			stop = i
		}
	}
	if start < 0 || stop < start {
		return p.Die("ml_open()'s block-zero preamble is not the run this phase was written for")
	}
	pre := body[start:stop]
	preC, err := graph.ItemsC(pre)
	if err != nil {
		return p.Die("%v", err)
	}
	for _, f := range append(append([]string{}, fields...),
		"set_b0_fname", "long_to_char", "mf_sync", "BLOCK0_ID0", "B0_DIRTY") {
		if !regexp.MustCompile(`\b` + f + `\b`).MatchString(preC) {
			return p.Die("ml_open()'s block-zero preamble does not mention `%s`, so "+
				"it is not the region this phase was written for", f)
		}
	}
	b0 := e.LocalDecls(open, "b0p")
	if len(b0) != 1 || !b0[0].Is("def") {
		return p.Die("ml_open()'s `b0p`: %d lines where this phase needs 1", len(b0))
	}
	inPre := map[*graph.Node]bool{}
	for _, it := range pre {
		graph.Walk(it, func(x *graph.Node) bool { inPre[x] = true; return true })
	}
	for _, u := range e.Uses(b0[0]) {
		if !inPre[u] {
			return p.Die("ml_open() says `b0p` outside the preamble")
		}
	}
	npre := strings.Count(preC, "\n")
	if err := e.ReplaceRun(pre[0], pre[len(pre)-1]); err != nil {
		return p.Die("ml_open()'s block-zero preamble -- %v", err)
	}
	p.Sayf("ml_open()'s block-zero preamble is %d lines and they are gone: the mf_new() that "+
		"took block nr 0, %d of the %d header writes, the set_b0_fname() call, the mf_put() "+
		"and the mf_sync()", npre, nwrite-2, nwrite)

	// THE TWO SURVIVING BLOCKS MOVE DOWN BY ONE: the numbers rewritten in
	// place, the messages pointed at the other string.
	num := func(fn, pat string, val int64, what string) {
		v.InFunction(fn, func(v *graph.Verbs) {
			ms := v.Find(pat)
			if len(ms) != 1 {
				v.Die("%s occurs %d times, expected 1", what, len(ms))
				return
			}
			m := ms[0]
			if err := e.Replace(m.Kids[len(m.Kids)-1], graph.Literal(val)); err != nil {
				v.Die("%s -- %v", what, err)
			}
		})
	}
	msg := func(fn, from, to, what string) {
		v.InFunction(fn, func(v *graph.Verbs) {
			ms := v.Find("(call iemsg " + from + ")")
			ds := e.FileDecls(to)
			if len(ms) != 1 || len(ds) != 1 {
				v.Die("%s occurs %d times, expected 1", what, len(ms))
				return
			}
			if err := e.RetargetAs(ms[0].Kids[2], 0, ds[0]); err != nil {
				v.Die("%s -- %v", what, err)
			}
		})
	}
	num("ml_open", "(!= (. (-> hp bh_hashitem) mhi_key) 1)", 0, "ml_open()'s test that the pointer block is block nr 1")
	msg("ml_open", "e_didnt_get_block_nr_one", "e_didnt_get_block_nr_zero", "ml_open()'s test that the pointer block is block nr 1")
	num("ml_open", "(= (. (index (-> pp pb_pointer) 0) pe_bnum) 2)", 1, "ml_open()'s pointer to the first data block")
	num("ml_open", "(!= (. (-> hp bh_hashitem) mhi_key) 2)", 1, "ml_open()'s test that the data block is block nr 2")
	msg("ml_open", "e_didnt_get_block_nr_two", "e_didnt_get_block_nr_one", "ml_open()'s test that the data block is block nr 2")
	// the line must be INSIDE ml_find_line() and nowhere else, so that the
	// replacement cannot land in some other function that spells it the same
	// way
	num("ml_find_line", "(= bnum 1)", 0, "ml_find_line()'s root block number")
	num("ml_append_int", "(!= (. (-> hp bh_hashitem) mhi_key) 1)", 0, "ml_append_int()'s test for the root pointer block")
	if failed() != nil {
		return failed()
	}
	p.Say("the two surviving blocks move down by one: the pointer block is block nr 0 and " +
		"the data block block nr 1, in ml_open()'s three tests, ml_find_line()'s root and " +
		"ml_append_int()'s root split")

	if _, err := partition("ml_setflags", []w53Class{
		{"its forward declaration", regexp.MustCompile(`^static void ml_setflags\(buf_T \*buf\);$`)},
		{"its definition", regexp.MustCompile(`^ml_setflags\(buf_T \*buf\)$`)},
		{"a call site", regexp.MustCompile(`^\s*ml_setflags\((curbuf|buf)\);$`)},
	}, "`ml_setflags`"); err != nil {
		return err
	}
	v.Cut("(call ml_setflags _)", 2, "ml_setflags()'s calls")
	v.DeleteDefinition("ml_setflags", "`ml_setflags`'s definition")
	if failed() != nil {
		return failed()
	}

	// ==== PART 2 -- THE NEGATIVE BLOCK NUMBERS =================================
	mlAppCalls := calls("ml_append")
	bad := 0
	for _, c := range mlAppCalls {
		if len(c.Kids) != 6 || c.Kids[5].Atom != "FALSE" {
			bad++
		}
	}
	if bad > 0 {
		return p.Die("ml_append() is called at %d sites and %d of them do not pass FALSE for "+
			"`newfile`", len(mlAppCalls), bad)
	}
	flagCalls := calls("ml_append_flags")
	if len(flagCalls) != 2 {
		return p.Die("ml_append_flags() has %d call sites and this phase was written against two",
			len(flagCalls))
	}
	p.Sayf("the chain that could make a block number negative, computed: ml_append() has %d "+
		"call sites and ALL %d pass FALSE for `newfile`; ml_append_flags() has %d, one the "+
		"ml_append() that has just been shown FALSE and one ML_APPEND_UNDO; ml_new_data() "+
		"has %d, one FALSE and one `flags & ML_APPEND_NEW`; mf_new() has %d, two FALSE and "+
		"one ml_new_data()'s own parameter.  So `negative` is FALSE at every reachable "+
		"call and mf_trans_add() returns OK before it does anything",
		len(mlAppCalls), len(mlAppCalls), len(flagCalls), len(calls("ml_new_data")), len(calls("mf_new")))

	v.InFunction("mf_new", func(v *graph.Verbs) {
		v.DropOperand("(! negative)", 1, "mf_new()'s test of the free list")
		v.FoldNever("negative", 1, "mf_new()'s negative branch")
	})
	v.InFunction("ml_append", func(v *graph.Verbs) {
		v.Rewrite("(? newfile ML_APPEND_NEW 0)", "0", 1, "ml_append()'s body")
	})
	if failed() != nil {
		return failed()
	}
	st, err := e.DropParams(append(append(drop("mf_new", "negative"), drop("ml_new_data", "negative")...),
		drop("ml_append", "newfile")...), graph.ParamOptions{})
	if err != nil {
		return p.Die("mf_new(), ml_new_data() and ml_append() lose `negative` and `newfile` -- %v", err)
	}
	if n := len(calls("ml_append")); st.Calls < n || n != len(mlAppCalls) {
		return p.Die("%d ml_append() call sites were rewritten and %d were counted", n, len(mlAppCalls))
	}
	p.Sayf("ml_append() loses `newfile` at its declaration, its definition and all %d call "+
		"sites, and ml_append_flags() is called with a flag word that can no longer hold "+
		"ML_APPEND_NEW", len(mlAppCalls))

	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.CutRun("ml_append_int()'s in-place ML_LOCKED_DIRTY and ML_LOCKED_POS",
			"(|= (. (-> buf b_ml) ml_flags) ML_LOCKED_DIRTY)",
			"(if (! (& flags ML_APPEND_NEW)) (block (|= (. (-> buf b_ml) ml_flags) ML_LOCKED_POS)))")
		v.CutRun("ml_append_int()'s split-block ML_LOCKED_DIRTY and ML_LOCKED_POS",
			"(if (|| lines_moved in_left) (block (|= (. (-> buf b_ml) ml_flags) ML_LOCKED_DIRTY)))",
			"(if (&& (! (& flags ML_APPEND_NEW)) (>= db_idx 0) in_left) (block (|= (. (-> buf b_ml) ml_flags) ML_LOCKED_POS)))")
	})
	if failed() != nil {
		return failed()
	}

	// ==== PART 3 -- THE DIRTY STATE MACHINE ====================================
	cls, err = partition("bh_flags", []w53Class{
		{"its declaration", regexp.MustCompile(`^    char bh_flags;$`)},
		{"a write", regexp.MustCompile(`bh_flags (\|)?= `)},
		{"the one read", regexp.MustCompile(`^    flags = hp->bh_flags;$`)},
	}, "`bh_flags`")
	if err != nil {
		return err
	}
	putLines, err := e.FormLines(e.Defn("mf_put"))
	if err != nil {
		return p.Die("%v", err)
	}
	var tests []string
	for _, l := range putLines {
		if strings.Contains(l.Text, "flags & BH_") {
			tests = append(tests, l.Text)
		}
	}
	if len(tests) != 1 || !strings.Contains(tests[0], "BH_LOCKED") {
		return p.Die("the block header flags are tested at %d places and this phase needs exactly "+
			"one, mf_put()'s test of BH_LOCKED", len(tests))
	}
	total := 0
	for _, n := range cls {
		total += n
	}
	p.Sayf("BH_DIRTY IS WRITTEN AND NEVER TESTED: `bh_flags` has %d mentions, %d of them "+
		"writes and ONE read, and that read tests BH_LOCKED and nothing else",
		total, cls["a write"])

	cls, err = partition("mf_dirty", []w53Class{
		{"its declaration", regexp.MustCompile(`^    mfdirty_T mf_dirty;$`)},
		{"a write", regexp.MustCompile(`mf_dirty = MF_DIRTY_`)},
		{"a read", regexp.MustCompile(`mf_dirty (==|!=) MF_DIRTY_`)},
	}, "`mf_dirty`")
	if err != nil {
		return err
	}
	// each read is the condition of an `if` whose only statement writes it
	for _, d := range e.MemberDecls("mf_dirty") {
		for _, u := range e.Uses(d) {
			s := e.Item(u)
			cmp := e.Parent(e.Parent(u))
			if cmp == nil || !(cmp.Is("==") || cmp.Is("!=")) {
				continue
			}
			ok := s != nil && s.Is("if") && len(s.Kids) == 3
			if ok {
				its := s.Kids[2].Args()
				ok = len(its) == 1 && its[0].Is("=") && len(its[0].Kids) == 3 && its[0].Kids[1].Is("->") &&
					its[0].Kids[1].Kids[len(its[0].Kids[1].Kids)-1].Atom == "mf_dirty"
			}
			if !ok {
				return p.Die("the `if` reading `mf_dirty` does not guard exactly one statement, and that " +
					"statement must be another write of `mf_dirty` for the field to be write-only")
			}
		}
	}
	p.Sayf("THE MEMFILE DIRTINESS IS A WRITE-ONLY STATE MACHINE: `mf_dirty` has %d writes and "+
		"%d reads, and each read is the condition of an `if` whose only statement writes "+
		"`mf_dirty` again -- so nothing outside the field ever learns its value",
		cls["a write"], cls["a read"])

	cls, err = partition("ML_LOCKED_DIRTY|ML_LOCKED_POS", []w53Class{
		{"its enumerator", regexp.MustCompile(`^enum \{ ML_LOCKED_(DIRTY|POS) = 0x0[48] \};$`)},
		{"set", regexp.MustCompile(`ml_flags \|= `)},
		{"cleared", regexp.MustCompile(`ml_flags &= ~\(ML_LOCKED_DIRTY \| ML_LOCKED_POS\);$`)},
		{"mf_put()'s two arguments", regexp.MustCompile(`^\s*mf_put\(mfp, buf->b_ml\.ml_locked, `)},
	}, "`ML_LOCKED_DIRTY` and `ML_LOCKED_POS`")
	if err != nil {
		return err
	}
	both := "(paren (| ML_LOCKED_DIRTY ML_LOCKED_POS))"
	nd := v.Count("(|= (. (-> _ b_ml) ml_flags) ML_LOCKED_DIRTY)")
	v.Cut("(|= (. (-> _ b_ml) ml_flags) ML_LOCKED_DIRTY)", nd, "the ML_LOCKED_DIRTY sets")
	v.Cut("(|= (. (-> _ b_ml) ml_flags) "+both+")", cls["set"]-nd, "the ML_LOCKED_DIRTY and ML_LOCKED_POS sets")
	v.Cut("(&= (. (-> buf b_ml) ml_flags) (~ (| ML_LOCKED_DIRTY ML_LOCKED_POS)))", cls["cleared"],
		"the ML_LOCKED_DIRTY and ML_LOCKED_POS clear")
	if failed() != nil {
		return failed()
	}

	// ==== PART 2b -- mf_put() loses both of its state arguments ================
	putCalls := 0
	for _, c := range calls("mf_put") {
		if len(c.Kids) == 6 && (c.Kids[5].Atom == "TRUE" || c.Kids[5].Atom == "FALSE") {
			putCalls++
		}
	}
	v.Body("mf_put", "(if (== (& (-> hp bh_flags) BH_LOCKED) 0) (block (call iemsg e_block_was_not_locked)))"+
		" (&= (-> hp bh_flags) (~ BH_LOCKED))", "mf_put()'s definition")
	if failed() != nil {
		return failed()
	}
	if _, err := e.DropParams(drop("mf_put", "mfp", "dirty", "infile"), graph.ParamOptions{}); err != nil {
		return p.Die("mf_put()'s definition -- %v", err)
	}
	p.Sayf("mf_put() is `mf_put(bhdr_T *hp)`: it clears BH_LOCKED, which is the one bit "+
		"anything tests, and its %d call sites lose the two arguments that chose between "+
		"writing BH_DIRTY and calling mf_trans_add()", putCalls)

	v.InFunction("ml_find_line", func(v *graph.Verbs) {
		v.Cut("(if (< bnum 0) _*)", 1, "ml_find_line()'s translation of a negative block number")
	})
	if failed() != nil {
		return failed()
	}
	for _, d := range e.FileDecls("mf_trans_add") {
		if d.Is("def") {
			if err := e.Delete(d); err != nil {
				return p.Die("mf_trans_add()'s declaration -- %v", err)
			}
		}
	}
	ntrans := 0
	for _, f := range []string{"mf_trans_add", "mf_trans_del"} {
		c, err := graph.FormsC([]*graph.Node{e.Defn(f)})
		if err != nil {
			return p.Die("%v", err)
		}
		ntrans += strings.Count(strings.TrimRight(string(c), "\n"), "\n") + 1
		v.DeleteDefinition(f, "`"+f+"`'s definition")
	}
	if failed() != nil {
		return failed()
	}
	for _, f := range []string{"mf_trans", "mf_blocknr_min", "mf_neg_count"} {
		k, err := mentions(f)
		if err != nil {
			return p.Die("%v", err)
		}
		if k > 4 {
			return p.Die("`%s` has %d mentions and everything that used it has gone", f, k)
		}
	}
	v.InFunction("mf_get", func(v *graph.Verbs) {
		v.Rewrite("(<= nr (-> mfp mf_blocknr_min))", "(< nr 0)", 1, "mf_get()'s bounds test")
	})
	v.InFunction("mf_free", func(v *graph.Verbs) {
		v.FoldNever("(< (. (-> hp bh_hashitem) mhi_key) 0)", 1, "mf_free()'s negative-block arm")
	})
	v.InFunction("mf_open", func(v *graph.Verbs) {
		v.Cut("(call mf_hash_init (addr (-> mfp mf_trans)))", 1, "mf_open()'s initialisation of mf_trans")
		v.Cut("(= (-> mfp mf_blocknr_min) (- 1))", 1, "mf_open()'s initialisation of mf_blocknr_min")
		v.Cut("(= (-> mfp mf_neg_count) 0)", 1, "mf_open()'s initialisation of mf_neg_count")
	})
	v.InFunction("mf_close", func(v *graph.Verbs) {
		v.Cut("(call mf_hash_free_all (addr (-> mfp mf_trans)))", 1, "mf_close()'s free of mf_trans")
	})
	if failed() != nil {
		return failed()
	}
	p.Sayf("the negative-block machinery is gone: mf_trans_add() and mf_trans_del() (%d lines), "+
		"the three memfile fields that served them, mf_get()'s lower bound and mf_free()'s "+
		"negative arm", ntrans)

	v.Cut("(if (!= (. (-> curbuf b_ml) ml_mfp) nullptr) (block (= (-> (. (-> curbuf b_ml) ml_mfp) mf_dirty) MF_DIRTY_YES_NOSYNC)))", 1,
		"the MF_DIRTY_YES_NOSYNC that is set around a buffer reload")
	v.Cut("(if (&& (!= (. (-> curbuf b_ml) ml_mfp) nullptr) (== (-> (. (-> curbuf b_ml) ml_mfp) mf_dirty) MF_DIRTY_YES_NOSYNC)) _*)", 1,
		"the MF_DIRTY_YES_NOSYNC that is read back")
	v.InFunction("mf_open", func(v *graph.Verbs) {
		v.Cut("(= (-> mfp mf_dirty) MF_DIRTY_NO)", 1, "mf_open()'s initialisation of mf_dirty")
	})
	v.InFunction("mf_new", func(v *graph.Verbs) {
		v.DropOperand("BH_DIRTY", 1, "mf_new()'s two dirty marks")
		v.Cut("(= (-> mfp mf_dirty) MF_DIRTY_YES)", 1, "mf_new()'s two dirty marks")
	})
	if failed() != nil {
		return failed()
	}
	for _, m := range e.MemberDecls("mf_dirty") {
		if err := e.Delete(m); err != nil {
			return p.Die("the `mf_dirty` field -- %v", err)
		}
	}
	for _, d := range e.Decls("mfdirty_T") {
		if err := e.Delete(d); err != nil {
			return p.Die("`mfdirty_T` -- %v", err)
		}
	}
	if k, err := mentions("mf_sync"); err != nil || k != 1 {
		return p.Die("`mf_sync` has %d mentions and the edit has left it exactly one, its own "+
			"definition -- the ml_open() call went with the preamble and the ml_setflags() "+
			"call with the function", k)
	}
	v.DeleteDefinition("mf_sync", "`mf_sync`'s definition")
	if failed() != nil {
		return failed()
	}

	if _, err := partition("dirty", []w53Class{
		{"its declaration", regexp.MustCompile(`^    int dirty;$`)},
		{"a write", regexp.MustCompile(`^\s*dirty = (TRUE|FALSE);$`)},
	}, "ml_find_line()'s `dirty`"); err != nil {
		return err
	}
	v.InFunction("ml_find_line", func(v *graph.Verbs) {
		v.Cut("(= dirty TRUE)", v.Count("(= dirty TRUE)"), "ml_find_line()'s writes of `dirty`")
		v.Cut("(= dirty FALSE)", v.Count("(= dirty FALSE)"), "ml_find_line()'s writes of `dirty`")
	})
	if failed() != nil {
		return failed()
	}

	// ==== PART 4 -- pe_old_lnum ================================================
	cls, err = partition("pe_old_lnum", []w53Class{
		{"its declaration", regexp.MustCompile(`^    linenr_T pe_old_lnum;$`)},
		{"a write", regexp.MustCompile(`^\s*(pp|pp_new)->pb_pointer\[[^]]*\]\.pe_old_lnum = \w+;$`)},
	}, "`pe_old_lnum`")
	if err != nil {
		return err
	}
	p.Sayf("`pe_old_lnum` IS WRITE-ONLY: %d writes and not one read.  tools/deadfields.py "+
		"cannot see it -- that tool takes a field named nowhere outside its own type -- so "+
		"the field goes in the EDIT, with its writes", cls["a write"])
	v.Cut("(= (. (index (-> _ pb_pointer) _) pe_old_lnum) _)", cls["a write"], "the writes of `pe_old_lnum`")
	empties := 0
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		for _, c := range []string{"(!= lnum_left 0)", "(!= lnum_right 0)", "lnum_left", "lnum_right"} {
			pat := "(if " + c + " (block))"
			k := v.Count(pat)
			empties += k
			v.Cut(pat, k, "the empty `if (lnum_left|lnum_right)` blocks")
		}
	})
	if failed() != nil {
		return failed()
	}
	if empties != 4 {
		return p.Die("%d `if (lnum_left|lnum_right)` blocks are empty now and this phase was written "+
			"against four", empties)
	}
	if _, err := partition("lnum_left|lnum_right", []w53Class{
		{"its declaration", regexp.MustCompile(`^        linenr_T lnum_(left|right);$`)},
		{"a write", regexp.MustCompile(`^\s*lnum_(left|right) = (lnum \+ [12]|0);$`)},
	}, "ml_append_int()'s `lnum_left` and `lnum_right`"); err != nil {
		return err
	}
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.Cut("(if (< db_idx 0) (block (= lnum_left (+ lnum 1)) (= lnum_right 0)) _)", 1,
			"the branch that computed lnum_left and lnum_right")
	})
	if failed() != nil {
		return failed()
	}
	kl, _ := mentions("lnum_left")
	kr, _ := mentions("lnum_right")
	if kl != 2 || kr != 2 {
		return p.Die("`lnum_left` has %d mentions and `lnum_right` %d, and the branch this edit has "+
			"just taken should leave each with its declaration and its one reset", kl, kr)
	}
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.CutRun("the reset of lnum_left and lnum_right", "(= lnum_left 0)", "(= lnum_right 0)")
	})
	if failed() != nil {
		return failed()
	}

	// mf_dont_release, read twice and assigned only in mf_close_file(), fell
	// out at phase 1 with the memfile's disk half (nomemfile and noswap, the
	// reform's D5).

	// ---- WHAT IS LEFT FOR THE SWEEP -------------------------------------------
	var leftNames []string
	for name := range w53Left {
		leftNames = append(leftNames, name)
	}
	sort.Strings(leftNames)
	for _, name := range leftNames {
		if k, _ := mentions(name); k == 0 {
			return p.Die("the edit has already taken `%s`, which it leaves for the sweep (%s) -- so "+
				"the two halves of this phase no longer divide as its check states", name, w53Left[name])
		}
	}
	p.Sayf("%d names are left standing for tools/sweep.sh: %s.  Each is a kind that sweep "+
		"finds; nothing that is WRITTEN is among them, because no tool in tools/ can see a "+
		"write", len(w53Left), strings.Join(leftNames, ", "))

	for _, name := range []string{"mf_dirty", "mfdirty_T", "MF_DIRTY_NO", "MF_DIRTY_YES",
		"MF_DIRTY_YES_NOSYNC", "mf_sync", "mf_trans_add", "mf_trans_del", "newfile"} {
		if k, _ := mentions(name); k != 0 {
			return p.Die("`%s` still has %d mentions and the edit owns every one of them", name, k)
		}
	}
	// the locals, members and object left for the sweep are named only where
	// they are declared: nothing reads or writes them any more.  b0p is
	// ml_open()'s declaration and two mentions inside set_b0_fname(), which the
	// sweep takes too.
	for name, k := range map[string]int{"bnum2": 1, "dirty": 1, "lnum_left": 1, "lnum_right": 1,
		"mf_trans": 1, "mf_blocknr_min": 1, "mf_neg_count": 1, "pe_old_lnum": 1,
		"b0p": 3} {
		if n, _ := mentions(name); n != k {
			return p.Die("`%s` has %d mentions, and the edit leaves %d", name, n, k)
		}
	}
	for _, sig := range []struct{ fn, head string }{
		{"mf_new", "mf_new(memfile_T *mfp, int page_count)"},
		{"ml_new_data", "ml_new_data(memfile_T *mfp, int page_count)"},
		{"ml_append", "ml_append(linenr_T lnum, char_u *line, colnr_T len)"},
		{"mf_put", "mf_put(bhdr_T *hp)"}} {
		ls, err := e.FormLines(e.Defn(sig.fn))
		found := 0
		for _, l := range ls {
			if l.Text == sig.head {
				found++
			}
		}
		if err != nil || found != 1 {
			return p.Die("`%s` is not a definition head in the output exactly once, so a "+
				"signature this phase narrowed is not the one it meant", sig.head)
		}
	}

	incs2 := e.Includes()
	okd := len(incs2) == len(incs)
	if okd {
		at := map[*graph.Node]int{}
		for i, f := range e.Graph().Forms {
			at[f] = i
		}
		for i := range incs2 {
			if at[incs2[i]] != at[incs2[0]]+i {
				okd = false
			}
		}
	}
	if !okd {
		return p.Die("the output does not have the same %d contiguous directives the input had -- "+
			"this phase adds none and removes none", len(incs))
	}
	// The check states what the EDIT took and what the SWEEP took, separately, and
	// this is how it can: the text the edit hands on, kept beside the text it was
	// handed.
	t, err := e.Graph().C()
	if err != nil {
		return err
	}
	if err := os.WriteFile(state+"/edit.c", t, 0o644); err != nil {
		return p.Die("%v", err)
	}
	linesAfter := strings.Count(string(t), "\n")
	p.Sayf("%d -> %d lines before the sweep, %d fewer, the %d `#include`s untouched and still "+
		"contiguous",
		linesBefore, linesAfter, linesBefore-linesAfter, len(incs2))
	return nil
}
