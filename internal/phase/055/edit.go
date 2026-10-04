package p055

// Whim phase 55 (formerly 127) -- de-page the leaf.  See GOAL.md.
//
// A DATA BLOCK STOPS BEING A PAGE OF BYTES AND BECOMES AN ARRAY OF LINE RECORDS.
// Until this phase a leaf of the memline tree is a header, an index of byte offsets
// growing UP from the header, and a text arena growing DOWN from the end of the
// page, with the two meeting at `db_free` bytes of gap.  A line's text lives inside
// the block, so inserting a line in the middle memmoves the arena and rewrites every
// index below it; a line that grows past the gap is appended-and-deleted into another
// block; and a line longer than a page makes the block two pages.  After this phase
// the leaf is
//
// struct data_line { char_u *dl_text; colnr_T dl_len; char dl_marked; };
// struct data_block { short_u db_id; linenr_T db_line_count;
// DATA_LN db_line[DB_LINE_MAX]; };
//
// and a line's text is its own allocation.  Inserting a line shifts records, not
// bytes; replacing a line stores a pointer.
//
// WHY IT IS CHEAP NOW AND WAS NOT BEFORE.  The arena exists for exactly one reason,
// to avoid a malloc per line, and GOALS.md's charter has retired that reason: "A
// GARBAGE COLLECTOR IS ASSUMED FROM HERE ON".  Phase 52 made host_alloc a bump
// allocator and host_free a return, so a per-line allocation costs a pointer bump and
// freeing costs nothing.  This phase spends that.
//
// AND WHAT IT SPENDS IS MEASURED, by the check's own counter and on both sides:
// the heaviest memline session asks the host for 201,927,792 bytes where the input
// asks 200,438,864, +0.7%.  A leaf page per 64 lines costs about what a leaf page per
// 78 lines and its arena cost, and the per-line text is what the arena used to hold
// inside the page.  With nothing freed that figure is a session's TRAFFIC and not its
// live data, which is why it is two hundred megabytes and why phase 52's arena
// is a gigabyte -- a number that phase and this one arrived at from opposite ends
// with instruments written apart, agreeing to the byte.
//
// THE TARGET REPRESENTATION IS TODAY'S DIRTY-LINE PATH MADE PERMANENT, which is why
// the rewrite can be this small.  `buf->b_ml.ml_line_ptr` under ML_LINE_DIRTY is
// ALREADY a separately allocated `char_u *` with `ml_line_len` beside it -- that is
// what ml_replace_len() builds and what del_bytes() edits in place when
// ml_line_alloced() is true.  ml_flush_line()'s job was to copy that buffer back into
// the page; here it stores the pointer, and the sixty-line "does the new text still
// fit" branch under it -- the memmove, the index fixup and the append-then-delete
// fallback -- has nothing left to decide.
//
// WHAT GOES, EVERY ONE OF IT MEASURED ON THE INPUT AND ASSERTED AS A PARTITION AND
// NOT AS A COUNT (below, `classify`):
//
// db_free           14 mentions      the gap, in bytes
// db_txt_start      29               the arena's low water
// db_txt_end         6               the arena's high water
// db_index          34               the offset index, declared `unsigned [1]`
// the top bit       17               DB_MARKED, stolen from an offset, now a field
// (char_u *)dp +    14               every interior pointer into the page
// offsetof(DATA_BL)  2               there is nothing left to measure
// ML_APPEND_MARK     5               its one caller was the fallback that goes
//
// THE THIRD offsetof IS NOT THIS PHASE'S.  ml_new_ptr()'s
// `offsetof(PTR_BL, pb_pointer)` measures a POINTER block, which is still a page of
// entries and is not the leaf.  De-paging the branch is a phase of its own and would
// change the tree's fanout on purpose; this one leaves it alone and says so.
//
// DB_LINE_MAX IS A FREE PARAMETER NOW AND IT IS CHOSEN BY MEASUREMENT.  A leaf used
// to hold as many lines as fitted in a page -- 78 of `zmemline`'s 47-byte
// lines on the boundary this was first written against -- and nothing decides it any
// more, so the value is a tuning knob.  The corpus CANNOT SEE IT: measured,
// DB_LINE_MAX of 32, 64, 128 and even 1 all record the 102 screen cases and the 16
// memline cases byte for byte, so no argument from "the recording agrees" is worth
// anything here.  What it does decide is how much of the tree the corpus REACHES, and
// that is measured, with record 123's own markers, ON THIS PHASE'S ACTUAL INPUT:
//
// DB_LINE_MAX   SPLITDATA  SPLITPTR  SPLITROOT  IDXNZ  DEEP
// 32           16         5          5        16     5
// 64           16         1          1        16     1
// 128           16         0          0        16     0
// 255           14         0          0        14     0
// q054, the input  16         1          1        16     1
//
// 64 IS TAKEN BECAUSE IT REACHES EXACTLY WHAT THE INPUT REACHES, and 255 -- the value
// that would fill the page -- is the one that must not be chosen: it reaches no
// pointer-block split at all, so the natural-looking choice, the one that wastes
// nothing, would blind the instrument on the very phase that rewrites the tree.
//
// THE MARGIN IS ONE CASE AND IT HAS BEEN NARROWING UNDER THIS PHASE, which is worth
// writing down rather than discovering.  The same table taken on q123 of the old numbering read 6/5/1/0 in
// the SPLITROOT column, so 128 was a live choice then and reaches ZERO now: zero
// phase 53 took `pe_old_lnum` out of PTR_EN and phase 54 took the block number and the
// page count, and pb_count_max has gone 127 -> 170 -> 255 while the corpus's buffer
// sizes have not moved.  Measured directly on q054: mem_deep_jumps makes 321 data
// blocks on the input and 391 here, against a pb_count_max of 255, and no other case
// reaches 255 on either side.  So a PTR_EN of 8 bytes would put pb_count_max at 511
// and take even 64 to zero -- at which point the corpus needs resizing or DB_LINE_MAX
// needs lowering (32 reaches 5), and that is the phase that shrinks PTR_EN to decide,
// not this one.
//
// THE LEAF IS STILL ALLOCATED AS ONE MEMFILE PAGE and that is deliberate scope.
// sizeof(DATA_BL) is 1,040 bytes of a 4,096-byte page, which the edit asserts with a
// static_assert rather than leaving to be discovered.  Allocating a block at its own
// size means giving memfile a byte size where it has a page count, which is block
// NUMBERING as well as block size -- the machinery phase 54 has just rewritten --
// and a phase that replaced the leaf's representation and changed how blocks are
// allocated in one act would have two claims and one set of evidence.  It is named
// here so it is not lost: it would take the leaf from 112 bytes a line to 64, and it
// would make an off-by-one in the capacity bound VISIBLE, which today it is not (the
// check measures that and reports it).  Two findings of the same neighbourhood go with
// it, and phase 54 has already taken one of them from the other side: `pe_page_count`
// and `bh_page_count` were constant 1 after this phase and are gone before it.
//
// NOTHING IS FREED, AND THAT IS THE LIFETIME RULE.  A record owns its text and never
// gives it back: ml_flush_line() stores the replacement and drops the old pointer,
// ml_delete_int() drops a record's, and neither calls vim_free().  So
//
// A POINTER RETURNED BY ml_get*() IS VALID FOR THE LIFETIME OF THE PROCESS.
//
// which is strictly weaker than what 341 call sites needed before, where the pointer
// was into a page and any insert or delete in the same block, any flush of any line
// in it, and any split invalidated it.  The check states the rule as a partition over
// every assignment to `dl_text` in the output, and probes it with a build that
// poisons the text a record stops owning.
//
// HOW THE EDIT IS WRITTEN, because phases 53 and 54 rewrite the same functions.
// Nothing here is anchored to a line this phase does not itself replace: every region
// is found by the function it is in and by its own first and last line, every call
// whose arity changes is rewritten by DROPPING ITS LAST ARGUMENT rather than by
// matching the argument, every `ml_flags |=` statement inside a replaced region is
// carried forward as it was found, and every local that the rewrite stops using is
// removed by COMPUTING that its name is left mentioned once.  A phase that wrote
// `ML_LOCKED_DIRTY` out would break on phase 53, which removes it.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, *R1 as built*): the leaf's new
// types and ml_alloc_line() are C spliced in ONE synthesized import (FRAG),
// the record array a member inserted, and every region the text replaced
// is the statements it held, cut, rewritten in place or built where they
// stood -- the ones the text carried kept with their nodes -- with
// ml_new_data() losing its page count by PARAM, at every call.  The
// partitions are the text's, on the lines of the forms that say the names
// (graph.FormLines) and, for what the output must not say, on the C view of
// the forms that could say it (graph.FormsWith): no whole C view is printed;
// the line counts its report gave were the whole file's, and went with it,
// as did the state files nothing read (*Fin as built*).

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim55", Edit) }

const w55DbLineMax = 64

// w55Bit is the mark, which used to be the top bit of an offset.  Phase 31's
// macro expansion left it spelled Out, and this is THE TEXT and not a description
// of it.
const w55Bit = "((unsigned)1 << ((sizeof(unsigned) * 8) - 1))"

// w55Gone are the five names the leaf stops having, and the one flag whose only
// caller goes with ml_flush_line()'s fallback.
var w55Gone = []string{"db_free", "db_txt_start", "db_txt_end", "db_index", "ML_APPEND_MARK"}

// w55Homes is where a mention of a gone name is allowed to be in the INPUT.  A
// mention anywhere else is a rule this edit does not have, and it refuses rather
// than leaving it.
var w55Homes = []string{"<file scope>", "ml_open", "ml_get_buf", "ml_append_int", "ml_delete_int",
	"ml_setmarked", "ml_firstmarked", "ml_clearmarked", "ml_flush_line", "ml_new_data"}

// w55Types are the leaf's new types and the function that allocates a
// line, C spliced where the text wrote them.
const (
	w55Line = `enum { DB_LINE_MAX = 64 };

struct data_line
{
    char_u      *dl_text;
    colnr_T     dl_len;
    char        dl_marked;
};
`
	w55Assert = `static_assert(sizeof(DATA_BL) <= MEMFILE_PAGE_SIZE, "a leaf is one memfile page");`
	w55Alloc  = `    static char_u *
ml_alloc_line(char_u *line, colnr_T len)
{
    char_u      *text;

    text = alloc((usize)len);
    if (text != nullptr)
    {
         musl_memmove((char *)(text), (char *)(line), (usize)(len)) ;
    }

    return text;
}
`
)

// Edit de-pages the leaf: a data block stops being an index of byte offsets
// over a text arena and becomes `DATA_LN db_line[DB_LINE_MAX]`, so a line's text
// is its own allocation valid for the lifetime of the process.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("  leaf         usage: edit whim55 <file>")
	}
	// die here prints on stdout, which is the heredoc's own `die`: this
	// phase writes its refusals into the report and not onto stderr.
	die := func(format string, a ...interface{}) error {
		fmt.Fprintf(w, "  leaf         %s\n", fmt.Sprintf(format, a...))
		return fmt.Errorf("")
	}
	say := func(format string, a ...interface{}) {
		fmt.Fprintf(w, "  leaf         %s\n", fmt.Sprintf(format, a...))
	}
	v := graph.NewVerbs("leaf", e, io.Discard)
	failed := func() error {
		if v.Err != nil {
			return die("%s", strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v.Err.Error()), "leaf")))
		}
		return nil
	}
	decls := func(name string) []*graph.Node {
		var ds []*graph.Node
		for _, d := range append(e.Decls(name), e.MemberDecls(name)...) {
			if e.Live(d) {
				ds = append(ds, d)
			}
		}
		return ds
	}
	// build makes the C-lisp items src where at stands, in fn.
	build := func(at *graph.Node, src string) []*graph.Node {
		if v.Err != nil {
			return nil
		}
		ns, err := e.Build(at, src, nil)
		if err != nil {
			v.Die("%v", err)
		}
		return ns
	}
	// region replaces the items from first through last of their list by
	// the items with.
	region := func(first, last *graph.Node, with []*graph.Node, what string) {
		if v.Err != nil {
			return
		}
		if first == nil || last == nil {
			v.Die("%s is not the region this edit replaces", what)
			return
		}
		var err error
		if first == last {
			err = e.Replace(first, with...)
		} else {
			err = e.ReplaceRun(first, last, with...)
		}
		if err != nil {
			v.Die("%s -- %v", what, err)
		}
	}
	one := func(fn, pat, what string) *graph.Node {
		var n *graph.Node
		v.InFunction(fn, func(v *graph.Verbs) { n = v.One(pat, what) })
		return n
	}
	items := func(b *graph.Node) []*graph.Node { return b.Args() }

	// --- the partition, before anything is changed ---------------------------
	var ns []*graph.Node
	for _, name := range w55Gone {
		ns = append(ns, e.AndUses(decls(name)...)...)
	}
	ls, err := e.FormLines(ns...)
	if err != nil {
		return die("%v", err)
	}
	var tb strings.Builder
	for _, l := range ls {
		tb.WriteString(l.Text)
		tb.WriteByte('\n')
	}
	forms := tb.String()
	before := map[string]int{}
	var strays []string
	for _, name := range append(append([]string{}, w55Gone...), w55Bit) {
		pat := `\b` + regexp.QuoteMeta(name) + `\b`
		if name == w55Bit {
			pat = regexp.QuoteMeta(name)
		}
		re := regexp.MustCompile(pat)
		hits := 0
		for _, l := range ls {
			if re.MatchString(l.Text) {
				hits++
				who := l.Func
				if who == "" {
					who = "<file scope>"
				}
				if !edit.Contains(w55Homes, who) {
					strays = append(strays, fmt.Sprintf("%s in %s", name, who))
				}
			}
		}
		if hits == 0 {
			return die("%s is not in the input at all, so this phase has already run or the "+
				"leaf is not the one it was written against", name)
		}
		if name == w55Bit {
			before[name] = strings.Count(forms, w55Bit)
		} else {
			before[name] = edit.MentionCount([]byte(forms), name)
		}
	}
	if len(strays) > 0 {
		return die("a name this phase removes is mentioned where it has no rule: %s",
			strings.Join(edit.First(strays, 5), "; "))
	}
	sum := 0
	for _, n := range w55Gone {
		sum += before[n]
	}
	say("the input mentions %s -- %d times between them, the top bit %d more -- and every "+
		"mention is in the struct, the enumerator or one of the eight memline functions "+
		"this edit rewrites", strings.Join(w55Gone, ", "), sum, before[w55Bit])

	// --- 1. the typedef, the record and the block ----------------------------
	var dataBL, dataBlock, openDefn *graph.Node
	for _, f := range e.Graph().Forms {
		switch {
		case f.Is("typedef") && graph.DeclName(f) == "DATA_BL":
			dataBL = f
		case f.Is("struct") && graph.Tag(f) == "data_block":
			dataBlock = f
		}
	}
	openDefn = e.Defn("ml_open")
	if dataBL == nil || dataBlock == nil || openDefn == nil {
		return die("struct data_block, its typedef or ml_open() is not in the file")
	}
	var members []string
	for _, m := range graph.Members(dataBlock) {
		members = append(members, m.Head())
	}
	if len(members) != 6 || members[5] != "db_index" {
		return die("struct data_block is not the header-index-arena block this phase replaces: %s",
			strings.Join(members, " "))
	}
	if _, err := e.SpliceC(
		graph.Frag{At: e.SpotBefore(dataBlock), Src: w55Line},
		graph.Frag{At: e.SpotAfter(dataBlock), Src: w55Assert},
		graph.Frag{At: e.SpotBefore(openDefn), Src: w55Alloc}); err != nil {
		return die("the record and one line's allocation -- %v", err)
	}
	// the typedef names the record, so it is spliced once the record is in:
	// a unit's fragments name no other fragment's declarations
	if _, err := e.SpliceC(graph.Frag{At: e.SpotAfter(dataBL), Src: "typedef struct data_line        DATA_LN;"}); err != nil {
		return die("the record's typedef -- %v", err)
	}
	var lineCount *graph.Node
	for _, m := range graph.Members(dataBlock) {
		if m.Head() == "db_line_count" {
			lineCount = m
		}
	}
	if _, err := e.InsertMember(lineCount, true, "(db_line (array DB_LINE_MAX DATA_LN))"); err != nil {
		return die("the block's records -- %v", err)
	}

	// --- 2. one line's text is its own allocation: ml_open's empty line ------
	if at := one("ml_open", "(= (-> dp db_line_count) 1)", "ml_open's one empty line"); at != nil {
		first := one("ml_open", "(= (index (-> dp db_index) 0) (pre-- (-> dp db_txt_start)))", "ml_open's one empty line")
		last := one("ml_open", "(= (deref (+ (cast (ptr char_u) dp) (-> dp db_txt_start))) NUL)", "ml_open's one empty line")
		with := build(first, `(= (. (index (-> dp db_line) 0) dl_text) (call ml_alloc_line (cast (ptr char_u) "") 1))
			(if (== (. (index (-> dp db_line) 0) dl_text) nullptr) (block (goto error)))
			(= (. (index (-> dp db_line) 0) dl_len) 1)`)
		region(first, last, append(with, at), "ml_open's one empty line")
	}

	// --- 3. ml_new_data has no page count ------------------------------------
	nd := e.Defn("ml_new_data")
	if nd == nil || e.ParamIndex("ml_new_data", "page_count") != 1 {
		return die("ml_new_data's last parameter is not the page count")
	}
	if c := one("ml_new_data", "(call mf_new mfp page_count)", "ml_new_data hands its page count to mf_new"); c != nil {
		if err := e.Replace(c.Kids[3], graph.Literal(1)); err != nil {
			return die("%v", err)
		}
	}
	v.InFunction("ml_new_data", func(v *graph.Verbs) {
		v.CutRun("ml_new_data's arena", "(= (-> dp db_txt_start) _)", "(= (-> dp db_free) _)")
	})
	if err := failed(); err != nil {
		return err
	}
	if _, err := e.DropParam("ml_new_data", "page_count", graph.ParamOptions{}); err != nil {
		return die("ml_new_data has no page count -- %v", err)
	}

	// --- 5. ml_get_buf reads a record ----------------------------------------
	v.InFunction("ml_get_buf", func(v *graph.Verbs) {
		v.CutRun("ml_get_buf reads a record", "(= start _)", "(if (== idx 0) _ _)")
	})

	for _, r := range [][2]string{
		{"(= (. (-> buf b_ml) ml_line_ptr) (+ (cast (ptr char_u) dp) start))", "(. (index (-> dp db_line) idx) dl_text)"},
		{"(= (. (-> buf b_ml) ml_line_len) (- end start))", "(. (index (-> dp db_line) idx) dl_len)"},
	} {
		if at := one("ml_get_buf", r[0], "ml_get_buf reads a record"); at != nil {
			region(at.Kids[2], at.Kids[2], build(at.Kids[2], r[1]), "ml_get_buf reads a record")
		}
	}

	// --- 6. ml_append_int ----------------------------------------------------
	if at := one("ml_append_int", "(def line_count int)", "ml_append_int's text"); at != nil {
		if err := e.InsertAfter(at, build(at, "(def text (ptr char_u))")...); err != nil {
			v.Die("%v", err)
		}
	}
	if at := one("ml_append_int", "(= space_needed (+ len (paren (sizeof-type unsigned))))", "ml_append_int's text"); at != nil {
		region(at, at, build(at, "(= text (call ml_alloc_line line len)) (if (== text nullptr) (block (goto theend)))"),
			"ml_append_int's text")
	}
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.Rewrite("(< (cast long (-> dp db_free)) space_needed)", "(>= (-> dp db_line_count) DB_LINE_MAX)", 1,
			"the next block's room")
	})
	if big := one("ml_append_int", "(if (>= (cast long (-> dp db_free)) space_needed) _ _)", "the room in the block"); big != nil {
		in := items(big.Kids[2])
		pre := one("ml_append_int", "(pre++ (paren (-> dp db_line_count)))", "the room in the block")
		with := build(in[0], `(if (> line_count (+ db_idx 1))
				(block (call musl_memmove
					(cast (ptr char) (paren (addr (index (-> dp db_line) (+ db_idx 2)))))
					(cast (ptr char) (paren (addr (index (-> dp db_line) (+ db_idx 1)))))
					(* (cast usize (- line_count db_idx 1)) (sizeof-type DATA_LN)))))
			(= (. (index (-> dp db_line) (+ db_idx 1)) dl_text) text)
			(= (. (index (-> dp db_line) (+ db_idx 1)) dl_len) len)
			(= (. (index (-> dp db_line) (+ db_idx 1)) dl_marked) FALSE)`)
		region(in[0], in[len(in)-1], append(with, pre), "the room in the block")
		if v.Err == nil {
			cond := build(big.Kids[1], "(< (-> dp db_line_count) DB_LINE_MAX)")
			if v.Err == nil {
				if err := e.Replace(big.Kids[1], cond...); err != nil {
					v.Die("%v", err)
				}
			}
		}
	}
	if at := one("ml_append_int", "(if (== lines_moved 0) _ _)", "the split arm"); at != nil {
		region(at, at, build(at, "(= in_left (paren (!= lines_moved 0)))"), "the split arm")
	}
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.Cut("(= page_count (/ _ page_size))", 1, "the split arm")
	})
	stores := func(d, i string) string {
		return fmt.Sprintf("(= (. (index (-> %[1]s db_line) %[2]s) dl_text) text) (= (. (index (-> %[1]s db_line) %[2]s) dl_len) len)"+
			" (= (. (index (-> %[1]s db_line) %[2]s) dl_marked) FALSE)", d, i)
	}
	if s := one("ml_append_int", "(if (! in_left) _)", "the split arm"); s != nil {
		in := items(s.Kids[2])
		region(in[0], in[len(in)-2], build(in[0], stores("dp_right", "0")), "the split arm")
	}
	if s := one("ml_append_int", "(if lines_moved _)", "the split arm"); s != nil {
		in := items(s.Kids[2])
		region(in[0], in[len(in)-3], build(in[0], `(call musl_memmove
			(cast (ptr char) (paren (addr (index (-> dp_right db_line) line_count_right))))
			(cast (ptr char) (paren (addr (index (-> dp db_line) (+ db_idx 1)))))
			(* (cast usize (paren lines_moved)) (sizeof-type DATA_LN)))`), "the split arm")
	}
	if s := one("ml_append_int", "(if in_left _)", "the split arm"); s != nil {
		in := items(s.Kids[2])
		region(in[0], in[len(in)-2], build(in[0], stores("dp_left", "line_count_left")), "the split arm")
	}

	// --- 7. ml_delete_int ----------------------------------------------------
	v.InFunction("ml_delete_int", func(v *graph.Verbs) {
		v.CutRun("the line_size computation", "(= line_start _)", "(if (== idx 0) _ _)")
	})
	if at := one("ml_delete_int", "(= text_start (-> dp db_txt_start))", "ml_delete_int's records"); at != nil {
		blk := e.Parent(at)
		in := items(blk)
		region(in[0], in[len(in)-2], build(in[0], `(if (< idx (- count 1))
			(block (call musl_memmove
				(cast (ptr char) (paren (addr (index (-> dp db_line) idx))))
				(cast (ptr char) (paren (addr (index (-> dp db_line) (+ idx 1)))))
				(* (cast usize (- count idx 1)) (sizeof-type DATA_LN)))))`), "ml_delete_int's records")
	}

	// --- 8. the mark is a field ----------------------------------------------
	v.InFunction("ml_setmarked", func(v *graph.Verbs) {
		v.Rewrite("(|= (index (-> dp db_index) ?i) _)", "(= (. (index (-> dp db_line) ?i) dl_marked) TRUE)", 1, "the mark is a field")
	})
	for _, fn := range []string{"ml_firstmarked", "ml_clearmarked"} {
		v.InFunction(fn, func(v *graph.Verbs) {
			v.Rewrite("(& (paren (index (-> dp db_index) i)) _)", "(. (index (-> dp db_line) i) dl_marked)", 1, "the mark is a field")
			v.Rewrite("(&= (paren (index (-> dp db_index) i)) _)", "(= (. (index (-> dp db_line) i) dl_marked) FALSE)", 1, "the mark is a field")
		})
	}

	// --- 9. ml_flush_line stores the pointer ---------------------------------
	if first := one("ml_flush_line", "(= start _)", "ml_flush_line stores the pointer"); first != nil {
		last := one("ml_flush_line", "(if (>= (cast int (-> dp db_free)) extra) _ _)", "ml_flush_line stores the pointer")
		region(first, last, build(first, "(= (. (index (-> dp db_line) idx) dl_text) new_line)"+
			" (= (. (index (-> dp db_line) idx) dl_len) (. (-> buf b_ml) ml_line_len))"), "ml_flush_line stores the pointer")
	}
	// THE FREE AND NOTHING ELSE.  The line under it is `entered = FALSE;` --
	// the re-entrancy guard's only clear, which stays.
	v.InFunction("ml_flush_line", func(v *graph.Verbs) {
		v.Cut("(call vim_free new_line)", 1, "the free of the line flushed")
	})
	if err := failed(); err != nil {
		return err
	}

	// the members nothing names now
	for _, m := range []string{"db_free", "db_txt_start", "db_txt_end", "db_index"} {
		ms := e.MemberDecls(m)
		if len(ms) != 1 {
			return die("%s is declared %d times", m, len(ms))
		}
		if err := e.Delete(ms[0]); err != nil {
			return die("struct data_block -- %v", err)
		}
	}

	// --- 10. ML_APPEND_MARK has no caller left -------------------------------
	// Its enumerator is left for the sweep, which takes one nothing names.
	for _, d := range e.Decls("ML_APPEND_MARK") {
		if k := len(e.Uses(d)); k != 0 {
			return die("ML_APPEND_MARK is still mentioned %d times and not only by its own "+
				"enumerator, so the flag has a caller this edit did not see", k+1)
		}
	}

	// --- the partition again, on the output ----------------------------------
	// Each question is asked of the forms that could answer it, found in one
	// walk (graph.FormsWith): a mention holds its name, the top bit `sizeof`,
	// an offsetof its type's name, each within one token; an interior pointer
	// is a cast of an identifier `dp...`, a token holding `dp`.
	landed := []string{"dl_text", "dl_len", "dl_marked", "DB_LINE_MAX", "ml_alloc_line"}
	saying := e.FormsWith(append(append([]string{"sizeof", "DATA_BL", "PTR_BL"}, w55Gone...), landed...)...)
	textOf := func(forms []*graph.Node) []byte {
		if v.Err != nil {
			return nil
		}
		t, err := graph.FormsC(forms)
		if err != nil {
			v.Die("%v", err)
		}
		return t
	}
	textWith := func(word string) []byte { return textOf(saying[word]) }
	for _, name := range w55Gone {
		out := textWith(name)
		want := 0
		if name == "ML_APPEND_MARK" {
			want = 1 // its enumerator, the sweep's
		}
		if k := edit.MentionCount(out, name); k != want {
			return die("%s survives the edit with %d mentions", name, k)
		}
	}
	if k := strings.Count(string(textWith("sizeof")), w55Bit); k != 0 {
		return die("the stolen top bit survives the edit %d times", k)
	}
	if interiorRe.MatchString(strings.ReplaceAll(string(textOf(e.FormsWhere(func(a string) bool { return strings.Contains(a, "dp") }))), "(char *)dp", "(char_u *)dp")) {
		return die("an interior pointer into a data block survives the edit")
	}
	if strings.Contains(string(textWith("DATA_BL")), "offsetof(DATA_BL") {
		return die("a data block is still being measured with offsetof")
	}
	if strings.Count(string(textWith("PTR_BL")), "offsetof(PTR_BL") != 1 {
		return die("ml_new_ptr's offsetof is not where it was: a POINTER block is still a " +
			"page and is not this phase's")
	}
	for _, name := range landed {
		if edit.MentionCount(textWith(name), name) == 0 {
			return die("%s is not in the output, so the replacement did not land", name)
		}
	}
	if err := failed(); err != nil {
		return err
	}

	// The lifetime rule, as a partition over every assignment to a record's text.
	dl, err := e.FormLines(e.AndUses(e.MemberDecls("dl_text")...)...)
	if err != nil {
		return die("%v", err)
	}
	got := map[string]int{}
	total := 0
	for _, l := range dl {
		if dlTextRe.MatchString(l.Text) {
			who := l.Func
			if who == "" {
				who = "<file scope>"
			}
			got[who]++
			total++
		}
	}
	want := map[string]int{"ml_open": 1, "ml_append_int": 3, "ml_flush_line": 1}
	if !w55SameCount(got, want) {
		return die("a record's text is assigned in %s, and the lifetime rule this phase pins "+
			"says it is assigned in exactly %s", w55PyDict(got), w55PyDict(want))
	}
	var wk []string
	for k := range want {
		wk = append(wk, k)
	}
	sort.Strings(wk)
	var shown []string
	for _, k := range wk {
		shown = append(shown, fmt.Sprintf("%s x%d", k, want[k]))
	}
	say("a record's text is assigned in exactly %d places -- %s -- and freed in none, "+
		"so a pointer ml_get() returned stays readable for the life of the process",
		total, strings.Join(shown, ", "))

	say("the leaf is an array of %d records and a line's text is its own allocation",
		w55DbLineMax)
	return nil
}

func w55SameCount(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// w55PyDict is Python's str() of a {str: int} dict, which the refusal quotes.
func w55PyDict(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	Out := make([]string, len(keys))
	for i, k := range keys {
		Out[i] = fmt.Sprintf("'%s': %d", k, m[k])
	}
	return "{" + strings.Join(Out, ", ") + "}"
}
