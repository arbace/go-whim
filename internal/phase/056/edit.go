package p056

// Whim phase 56 (formerly 128) -- fold the node types.  See GOAL.md.
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
// THIS IS THE THING PHASE 55 NAMED AND DECLINED, in its own words: "Allocating a
// block at its own size means giving memfile a byte size where it has a page count ...
// It is named here so it is not lost: it would take the leaf from 112 bytes a line to
// 64, and it would MAKE AN OFF-BY-ONE IN THE CAPACITY BOUND VISIBLE, which today it is
// not."  Both halves are measured by the check rather than repeated: the leaf's node
// cost falls from 64.5 bytes a line to 16.25, and phase 55's own `cap` control -- the
// leaf capacity test widened by one -- moves 0 of 118 records on the input and 4 of 118
// here, in one run, with the input's binary built from the source beside it.
//
// WHAT THE FANOUT DOES, WHICH IS THE ONE THING THIS PHASE COULD HAVE DESTROYED.
// `pb_count_max` was computed per block as
// `(mf_page_size - offsetof(PTR_BL, pb_pointer)) / sizeof(PTR_EN)`, which is
// (4096 - 8) / 16 = 255, and it is the tree's fanout.  Record 123's corpus reaches a
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
// record 123's instrument, on this phase's output and on its input in the same run,
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
// is phase 54's form: `BH_LOCKED`, whose only three readers were mf_new, mf_get and
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
// 't')` is phase 31's macro expansion and not this phase's text.  No line number is
// pinned and no line this phase does not itself replace is quoted.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, *R1 as built*): the node's tag and
// the two headers are members inserted, the fanout a member retyped, and
// the memfile layer the run of forms the text cut; `ml_mfp`'s uses are
// pointed at `ml_root`, each `(T *)(x->bh_data)` gives up its selection,
// each `x->pb_id` and `x->db_id` is `hp->bh_id` built in its place, the
// constructors' bodies are built with the id constants moved in from the
// bodies they replace, and both lose `mfp` by PARAM, at every call.  The
// enumerator and ml_free_tree() are C spliced in one synthesized import
// (FRAG).  The partitions are the text's, on the lines of the forms that
// could say the name, each form apart (graph.FormsWith): no whole C view is
// printed.  The line counts its report gave were the whole file's and went
// with it, as did the state files nothing read and the check that the
// input held no run of two blank lines -- a text's question whether it
// had been swept, which a C view, printed canonically, never holds
// (*Fin as built*).

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim56", Edit) }

// w56Fanout is the fanout, and it is a LITERAL rather than a computation: the
// corpus's root-split coverage is measured against the number the input gives,
// and a later phase that narrowed PTR_EN would take the root split Out of the
// corpus without moving one record.  The input computes it and the output
// asserts it -- and the file gains a static_assert that fails to COMPILE.
const w56Fanout = 255

// w56Gone are the names the fold removes; w56ForSweep the two the EDIT leaves at
// exactly one mention -- their own definition -- for the SWEEP to take, stated
// here so that a sweep which took something else, or nothing, fails in the check.
var w56Gone = []string{"bh_next", "bh_prev", "bh_data", "bh_flags",
	"mf_used_first", "mf_page_size", "memfile", "memfile_T", "ml_mfp",
	"pb_id", "db_id", "pb_count_max", "MEMFILE_PAGE_SIZE",
	"mf_open", "mf_close", "mf_new", "mf_get", "mf_put", "mf_free",
	"mf_ins_used", "mf_rem_used", "mf_alloc_bhdr", "mf_free_bhdr",
	"mfp", "page_count", "page_size"}

var w56ForSweep = []string{"BH_LOCKED", "e_block_was_not_locked"}

// w56Homes: the ten mf_* functions and the eleven memline ones are this phase's
// whole subject; the seven others are the places that ask whether a buffer has a
// memline at all, which is the one question ml_mfp answered for anybody else.
var w56Homes = []string{"<file scope>",
	"mf_open", "mf_close", "mf_new", "mf_get", "mf_put", "mf_free",
	"mf_ins_used", "mf_rem_used", "mf_alloc_bhdr", "mf_free_bhdr",
	"ml_open", "ml_close", "ml_get_buf", "ml_append_int", "ml_delete_int",
	"ml_setmarked", "ml_firstmarked", "ml_clearmarked", "ml_flush_line",
	"ml_new_data", "ml_new_ptr", "ml_find_line", "ml_lineadd",
	"ml_append_flags", "ml_replace_len",
	"buf_clear_file", "buf_freeall", "create_windows", "curbuf_reusable",
	"get_nolist_virtcol", "getout", "open_buffer"}

var (
	w56PageSize = regexp.MustCompile(`enum \{ MEMFILE_PAGE_SIZE = (\d+) \};`)
)

const (
	w56FreeTree = `    static void
ml_free_tree(bhdr_T *hp)
{
    PTR_BL      *pp;
    int         i;

    if (hp == nullptr)
    {
        return;
    }
    if (hp->bh_id == %s)
    {
        pp = (PTR_BL *)(hp);
        for (i = 0; i < (int)pp->pb_count; ++i)
        {
            ml_free_tree(pp->pb_pointer[i].pe_block);
        }
    }
    vim_free(hp);
}
`
)

// w56Saying are the top-level forms saying each of a set of words, found
// in one walk (graph.FormsWith), each printed once.
type w56Saying struct {
	forms   map[string][]*graph.Node
	printed map[*graph.Node][]byte
}

func w56Say(e *graph.Editor, words ...string) *w56Saying {
	return &w56Saying{e.FormsWith(words...), map[*graph.Node][]byte{}}
}

// lines are the lines of each form saying word, as the C view prints it,
// and the C of those forms together.
func (s *w56Saying) lines(word string) ([][]string, string, error) {
	var out [][]string
	var all strings.Builder
	for _, f := range s.forms[word] {
		c, ok := s.printed[f]
		if !ok {
			var err error
			if c, err = graph.FormsC([]*graph.Node{f}); err != nil {
				return nil, "", err
			}
			s.printed[f] = c
		}
		all.Write(c)
		out = append(out, strings.Split(strings.TrimRight(string(c), "\n"), "\n"))
	}
	return out, all.String(), nil
}

// w56Enclosing is the function the line i of ls is in, as the text read it:
// the head above it, `static` on the line over that, before a `}` at column 0.
func w56Enclosing(ls []string, i int) string {
	for j := i; j >= 0; j-- {
		if ls[j] == "}" {
			return "<file scope>"
		}
		m := vimtext.FnHeadRe.FindStringSubmatch(ls[j])
		if m != nil && j > 0 && strings.HasPrefix(strings.TrimLeft(ls[j-1], " \t"), "static") {
			return m[1]
		}
	}
	return "<file scope>"
}

// Edit folds the node types: `bhdr_T` becomes `struct block_hdr { short_u
// bh_id; }`, `memfile_T` goes entirely, and a node is ONE allocation at its own
// size.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("  node         usage: edit whim56 <file>")
	}
	die := func(format string, a ...interface{}) error {
		fmt.Fprintf(w, "  node         %s\n", fmt.Sprintf(format, a...))
		return fmt.Errorf("")
	}
	say := func(format string, a ...interface{}) {
		fmt.Fprintf(w, "  node         %s\n", fmt.Sprintf(format, a...))
	}
	v := graph.NewVerbs("node", e, io.Discard)
	failed := func() error {
		if v.Err != nil {
			return die("%s", strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v.Err.Error()), "node")))
		}
		return nil
	}
	one := func(fn, pat, what string) *graph.Node {
		var n *graph.Node
		v.InFunction(fn, func(v *graph.Verbs) { n = v.One(pat, what) })
		return n
	}
	build := func(at *graph.Node, src string, holes graph.Bindings) []*graph.Node {
		if v.Err != nil {
			return nil
		}
		ns, err := e.Build(at, src, holes)
		if err != nil {
			v.Die("%v", err)
		}
		return ns
	}
	replace := func(old *graph.Node, with []*graph.Node, what string) {
		if v.Err != nil || old == nil {
			return
		}
		if err := e.Replace(old, with...); err != nil {
			v.Die("%s -- %v", what, err)
		}
	}
	member := func(name string) *graph.Node {
		ms := e.MemberDecls(name)
		if len(ms) != 1 {
			v.Die("the member %s is declared %d times", name, len(ms))
			return nil
		}
		return ms[0]
	}
	structOf := func(tag string) *graph.Node {
		for _, f := range e.Graph().Forms {
			if f.Is("struct") && graph.Tag(f) == tag {
				return f
			}
		}
		return nil
	}
	members := func(s *graph.Node) []string {
		var out []string
		for _, m := range graph.Members(s) {
			out = append(out, m.Head())
		}
		return out
	}

	// --- the partition, before anything is changed ---------------------------
	// Each question is asked of the forms that could answer it, each form's
	// lines apart (w56Saying): a mention holds its name within one token.
	said0 := w56Say(e, append(append([]string{"MEMFILE_PAGE_SIZE", "ml_root"}, w56Gone...), w56ForSweep...)...)
	before := map[string]int{}
	var strays []string
	for _, name := range append(append([]string{}, w56Gone...), w56ForSweep...) {
		re := regexp.MustCompile(`\b` + name + `\b`)
		forms, t0, err := said0.lines(name)
		if err != nil {
			return err
		}
		hits := 0
		for _, lines := range forms {
			for i, l := range lines {
				if strings.Contains(l, name) && re.MatchString(l) {
					hits++
					if f := w56Enclosing(lines, i); !edit.Contains(w56Homes, f) {
						strays = append(strays, fmt.Sprintf("%s in %s", name, f))
					}
				}
			}
		}
		if hits == 0 {
			return die("%s is not in the input at all, so this phase has already run or the "+
				"memfile is not the one it was written against", name)
		}
		// The COUNT is mentions and the classification is lines: mf_rem_used has
		// two mentions on one line, and the check re-reads the count the same way.
		before[name] = edit.WordCount(t0, name)
	}
	if len(strays) > 0 {
		return die("a name this phase removes is mentioned where it has no rule: %s",
			strings.Join(edit.First(strays, 5), "; "))
	}
	sum := 0
	for _, n := range w56Gone {
		sum += before[n]
	}
	say("the input mentions %s -- %d times between them -- and every mention is in file "+
		"scope, in one of the ten mf_* functions, in one of the eleven memline functions, "+
		"or in one of the seven that ask whether a buffer has a memline",
		strings.Join(w56Gone, ", "), sum)

	// The fanout, read off the INPUT rather than written here.
	_, tPage, err := said0.lines("MEMFILE_PAGE_SIZE")
	if err != nil {
		return err
	}
	pm := w56PageSize.FindStringSubmatch(tPage)
	if pm == nil {
		return die("the input has no MEMFILE_PAGE_SIZE enumerator")
	}
	page, _ := strconv.Atoi(pm[1])
	pbS := structOf("pointer_block")
	if pbS == nil || strings.Join(members(pbS), " ") != "pb_id pb_count pb_count_max pb_pointer" {
		return die("struct pointer_block is not the page of entries this phase counts")
	}
	if (page-8)/16 != w56Fanout {
		return die("the input computes a fanout of %d and this phase fixes it at %d; the corpus's "+
			"root-split coverage is measured against the first number", (page-8)/16, w56Fanout)
	}
	say("the input's fanout is (%d - 8) / 16 = %d, and that is the number this phase "+
		"fixes: record 123 reaches a ROOT SPLIT in one of sixteen cases because that "+
		"case builds more data blocks than this", page, w56Fanout)

	// THE OPEN-BUFFER PREDICATE, asked of the input before anything changes.
	wasForms, _, err := said0.lines("ml_mfp")
	if err != nil {
		return err
	}
	rootForms, _, err := said0.lines("ml_root")
	if err != nil {
		return err
	}

	// --- 1. struct block_hdr becomes the node's tag, and nothing else --------
	bh := structOf("block_hdr")
	if bh == nil || strings.Join(members(bh), " ") != "bh_next bh_prev bh_data bh_flags" {
		return die("struct block_hdr is not the four-member page header this phase folds")
	}
	if _, err := e.InsertMember(member("bh_flags"), true, "(bh_id short_u)"); err != nil {
		return die("the node's tag -- %v", err)
	}
	ms := structOf("memfile")
	if ms == nil || strings.Join(members(ms), " ") != "mf_used_first mf_page_size" {
		return die("struct memfile is not the two-member one this phase folds away")
	}

	// --- 5. a branch is a counted array of entries; the leaf's tag a header --
	db := structOf("data_block")
	if db == nil || strings.Join(members(db), " ") != "db_id db_line_count db_line" {
		return die("struct data_block is not the leaf phase 55 left")
	}
	if _, err := e.InsertMember(member("pb_id"), false, "(pb_hdr bhdr_T)"); err != nil {
		return die("the branch's header -- %v", err)
	}
	if _, err := e.InsertMember(member("db_id"), false, "(db_hdr bhdr_T)"); err != nil {
		return die("the leaf's header -- %v", err)
	}
	// the id constants are CARRIED out of the definitions being replaced and
	// never spelled
	da := one("ml_new_data", "(= (-> dp db_id) ?v)", "the leaf's id")
	pt := one("ml_new_ptr", "(= (-> pp pb_id) ?v)", "the branch's id")
	if err := failed(); err != nil {
		return err
	}
	daC, _ := graph.ItemsC([]*graph.Node{da.Kids[2]})
	ptC, _ := graph.ItemsC([]*graph.Node{pt.Kids[2]})
	daC, ptC = strings.TrimSuffix(strings.TrimSpace(daC), ";"), strings.TrimSuffix(strings.TrimSpace(ptC), ";")
	if !strings.Contains(daC, "<< 8") || !strings.Contains(ptC, "<< 8") || daC == ptC {
		return die("the two block ids are not the two distinct constants this edit carries: "+
			"%s and %s", vimtext.PyReprMultiline(daC), vimtext.PyReprMultiline(ptC))
	}
	// the fanout's enumerator and ml_free_tree(), one synthesized import
	alloc := e.Defn("ml_alloc_line")
	if alloc == nil {
		return die("ml_alloc_line is not defined")
	}
	if _, err := e.SpliceC(graph.Frag{At: e.SpotBefore(pbS), Src: "enum { PB_COUNT_MAX = 255 };"},
		graph.Frag{At: e.SpotBefore(alloc), Src: fmt.Sprintf(w56FreeTree, ptC)}); err != nil {
		return die("the fanout and ml_free_tree() -- %v", err)
	}
	if _, err := e.Retype(member("pb_pointer"), "(array PB_COUNT_MAX PTR_EN)"); err != nil {
		return die("the branch's entries -- %v", err)
	}
	var oldAssert *graph.Node
	for _, f := range e.Graph().Forms {
		if f.Is("static_assert") {
			if c, _ := graph.FormsC([]*graph.Node{f}); strings.HasPrefix(string(c), "static_assert(sizeof(DATA_BL) <= MEMFILE_PAGE_SIZE,") {
				oldAssert = f
			}
		}
	}
	if oldAssert == nil {
		return die("the leaf's static_assert is not in the file")
	}
	// a static_assert is no form BUILD makes; its condition is an expression
	// it does, and the message a string
	var asserts []*graph.Node
	for _, sa := range [][2]string{
		{"(== (sizeof-type PTR_EN) 16)", `"a pointer entry is one node reference and one line count"`},
		{"(== PB_COUNT_MAX (/ (- " + strconv.Itoa(page) + " 8) (sizeof-type PTR_EN)))", `"the fanout the 4096-byte page gave, kept when the page went"`},
		{"(== (sizeof-type DATA_BL) (+ 16 (* DB_LINE_MAX (sizeof-type DATA_LN))))", `"a leaf is its tag, its count and its records"`},
	} {
		if c := build(oldAssert, sa[0], nil); len(c) == 1 {
			asserts = append(asserts, graph.NewList(graph.NewAtom("static_assert"), c[0], graph.NewAtom(sa[1])))
		}
	}
	replace(oldAssert, asserts, "the static_asserts")

	// --- 6. the two constructors allocate a node at its own size -------------
	ctor := func(fn, local, typ, hdr, keep string, id *graph.Node, extra []string) {
		d := e.Defn(fn)
		if d == nil || v.Err != nil {
			v.Die("%s is not defined", fn)
			return
		}
		var dl, kp *graph.Node
		v.InFunction(fn, func(v *graph.Verbs) {
			dl = v.One("(def "+local+" (ptr "+typ+"))", fn+"()")
			kp = v.One(keep, fn+"()")
			for _, x := range extra {
				v.Cut(x, 1, fn+"()")
			}
		})
		if v.Err != nil {
			return
		}
		at := build(kp, fmt.Sprintf("(= %[1]s (cast (ptr %[2]s) (call alloc_clear (sizeof-type %[2]s))))"+
			" (if (== %[1]s nullptr) (block (return nullptr)))"+
			" (= (. (-> %[1]s %[3]s) bh_id) ?v)", local, typ, hdr), graph.Bindings{"v": id.Kids[2]})
		ret := build(kp, "(return (cast (ptr bhdr_T) "+local+"))", nil)
		if v.Err != nil {
			return
		}
		body := graph.Body(d)
		with := append(append(append([]*graph.Node{dl}, at...), kp), ret...)
		if err := e.ReplaceRun(body[0], body[len(body)-1], with...); err != nil {
			v.Die("%s() -- %v", fn, err)
		}
	}
	ctor("ml_new_data", "dp", "DATA_BL", "db_hdr", "(= (-> dp db_line_count) 0)", da, nil)
	ctor("ml_new_ptr", "pp", "PTR_BL", "pb_hdr", "(= (-> pp pb_count) 0)", pt, []string{"(= (-> pp pb_count_max) _)"})
	if err := failed(); err != nil {
		return err
	}
	for _, fn := range []string{"ml_new_data", "ml_new_ptr"} {
		if _, err := e.DropParam(fn, "mfp", graph.ParamOptions{}); err != nil {
			return die("%s() -- %v", fn, err)
		}
	}

	// --- 8. ml_open opens nothing --------------------------------------------
	v.InFunction("ml_open", func(v *graph.Verbs) {
		v.CutRun("mf_open and its failure arm and the assignment to ml_mfp", "(= mfp (call mf_open))",
			"(if (== mfp nullptr) (block (goto error)))", "(= (. (-> buf b_ml) ml_mfp) mfp)")
	})
	if first := one("ml_open", "(if (!= mfp nullptr) _)", "ml_open's error path"); first != nil {
		last := one("ml_open", "(= (. (-> buf b_ml) ml_mfp) nullptr)", "ml_open's error path")
		with := build(first, "(call ml_free_tree (. (-> buf b_ml) ml_root)) (= (. (-> buf b_ml) ml_root) nullptr)", nil)
		if v.Err == nil {
			if err := e.ReplaceRun(first, last, with...); err != nil {
				v.Die("ml_open's error path -- %v", err)
			}
		}
	}

	v.InFunction("ml_open", func(v *graph.Verbs) {
		v.Cut("(call mf_put hp)", 1, "ml_open's lock")
	})

	// --- 9. ml_close frees the tree it has ------------------------------------
	if c := one("ml_close", "(call mf_close (. (-> buf b_ml) ml_mfp) del_file)", "ml_close frees the tree"); c != nil {
		replace(c, build(c, "(call ml_free_tree (. (-> buf b_ml) ml_root))", nil), "ml_close frees the tree")
	}

	// --- 10. the three functions that took a handle on the memfile -----------
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.CutRun("the page size", "(= mfp (. (-> buf b_ml) ml_mfp))", "(= page_size (-> mfp mf_page_size))")
	})
	if a := one("ml_delete_int", "(= mfp (. (-> buf b_ml) ml_mfp))", "the memfile null test in ml_delete_int"); a != nil {
		b := one("ml_delete_int", "(if (== mfp nullptr) (block (return FAIL)))", "the memfile null test in ml_delete_int")
		with := build(a, "(if (== (. (-> buf b_ml) ml_root) nullptr) (block (return FAIL)))", nil)
		if v.Err == nil {
			if err := e.ReplaceRun(a, b, with...); err != nil {
				v.Die("the memfile null test in ml_delete_int -- %v", err)
			}
		}
	}
	v.InFunction("ml_find_line", func(v *graph.Verbs) {
		v.Cut("(= mfp (. (-> buf b_ml) ml_mfp))", 1, "ml_find_line's handle")
	})
	if err := failed(); err != nil {
		return err
	}

	// --- 4. the memfile layer itself -----------------------------------------
	var pageEnum, last *graph.Node
	for _, f := range e.Graph().Forms {
		if f.Is("enum") {
			if c, _ := graph.FormsC([]*graph.Node{f}); string(c) == fmt.Sprintf("enum { MEMFILE_PAGE_SIZE = %d };\n", page) {
				pageEnum = f
			}
		}
	}
	last = e.Defn("mf_free_bhdr")
	if pageEnum == nil || last == nil {
		return die("the memfile block does not start where this edit expects")
	}
	var run []*graph.Node
	in := false
	for _, f := range e.Graph().Forms {
		if f == pageEnum {
			in = true
		}
		if in {
			run = append(run, f)
		}
		if f == last {
			break
		}
	}
	// --- 11. everywhere else, ml_mfp was the question "is this buffer loaded"
	root := member("ml_root")
	mfpM := member("ml_mfp")
	if err := failed(); err != nil {
		return err
	}
	inRun := map[*graph.Node]bool{}
	for _, f := range run {
		graph.Walk(f, func(x *graph.Node) bool { inRun[x] = true; return true })
	}
	for _, u := range append([]*graph.Node(nil), e.Uses(mfpM)...) {
		if inRun[u] || !e.Live(u) {
			continue
		}
		if err := e.RetargetAs(u, 0, root); err != nil {
			return die("ml_mfp -- %v", err)
		}
	}
	// --- 12. a node is reached without its header ----------------------------
	for _, u := range append([]*graph.Node(nil), e.Uses(member("bh_data"))...) {
		if inRun[u] || !e.Live(u) {
			continue
		}
		sel := e.Parent(u)
		if !sel.Is("->") || len(sel.Kids) != 3 || !e.Parent(sel).Is("paren") || !e.Parent(e.Parent(sel)).Is("cast") {
			return die("a use of bh_data is not the cast of a node this edit rewrites")
		}
		if err := e.Replace(sel, sel.Kids[1]); err != nil {
			return die("bh_data -- %v", err)
		}
	}
	// --- 13. one tag, in the node --------------------------------------------
	for _, m := range []string{"pb_id", "db_id"} {
		for _, u := range append([]*graph.Node(nil), e.Uses(member(m))...) {
			if inRun[u] || !e.Live(u) {
				continue
			}
			sel := e.Parent(u)
			replace(sel, build(sel, "(-> hp bh_id)", nil), "one tag, in the node")
		}
	}
	if err := failed(); err != nil {
		return err
	}
	// --- 14. nothing can evict a block, so nothing locks one -----------------
	for _, fn := range []string{"ml_append_int", "ml_delete_int", "ml_find_line", "ml_lineadd"} {
		v.InFunction(fn, func(v *graph.Verbs) {
			v.Cut("(call mf_put _)", v.Count("(call mf_put _)"), "nothing locks a block")
		})
	}
	// --- 15. a block is its own pointer --------------------------------------
	for _, c := range v.Find("(if (== (= hp (call mf_get mfp ?x)) nullptr) _)") {
		x := c.Kids[1].Kids[1].Kids[2].Kids[3]
		replace(c, build(c, "(= hp ?x)", graph.Bindings{"x": x}), "a block is its own pointer")
	}
	if err := failed(); err != nil {
		return err
	}
	if k := v.Count("(call mf_get _*)"); k != 0 {
		return die("an mf_get is not the guarded assignment this edit rewrites")
	}

	// --- 16. ml_append_int ----------------------------------------------------
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.Rewrite("(-> pp pb_count_max)", "PB_COUNT_MAX", 1, "the fanout")
	})
	// The root split copied the whole PAGE, which is how a node's contents moved
	// while its header sat somewhere else.  The header is IN the node now.
	if c := one("ml_append_int", "(call musl_memmove (cast (ptr char) (paren pp_new)) (cast (ptr char) (paren pp)) (cast usize page_size))", "the root split"); c != nil {
		replace(c, build(c, `(= (-> pp_new pb_count) (-> pp pb_count))
			(call musl_memmove (cast (ptr char) (paren (addr (index (-> pp_new pb_pointer) 0))))
				(cast (ptr char) (paren (addr (index (-> pp pb_pointer) 0))))
				(* (cast usize (-> pp pb_count)) (sizeof-type PTR_EN)))`, nil), "the root split")
	}
	// --- 17. ml_delete_int releases a node by freeing it ---------------------
	v.InFunction("ml_delete_int", func(v *graph.Verbs) {
		v.Rewrite("(call mf_free mfp hp)", "(call vim_free hp)", 2, "ml_delete_int releases a node")
	})
	// --- 18. ml_find_line reads the tag off the node -------------------------
	v.InFunction("ml_find_line", func(v *graph.Verbs) {
		v.Cut("(= dp (cast (ptr DATA_BL) (paren hp)))", 1, "the leaf test's cast")
		v.RewriteAt("(= pp (cast (ptr PTR_BL) (paren ?d)))", "d", "hp", 1, "the branch's cast")
		v.Cut("(label error_noblock)", 1, "error_noblock")
	})
	if err := failed(); err != nil {
		return err
	}

	// the memfile layer, its struct, the memline's handle and the old members
	for _, f := range run {
		if err := e.Delete(f); err != nil {
			return die("the memfile block -- %v", err)
		}
	}
	if err := e.Delete(ms); err != nil {
		return die("struct memfile -- %v", err)
	}
	for _, m := range []string{"ml_mfp", "bh_next", "bh_prev", "bh_data", "bh_flags", "pb_id", "pb_count_max", "db_id"} {
		d := member(m)
		if err := failed(); err != nil {
			return err
		}
		if err := e.Delete(d); err != nil {
			return die("the member %s -- %v", m, err)
		}
	}

	// --- 19. the locals the fold stopped using -------------------------------
	var dropped []string
	for _, name := range []string{"ml_open", "ml_close", "ml_get_buf", "ml_append_int", "ml_delete_int",
		"ml_setmarked", "ml_firstmarked", "ml_clearmarked", "ml_flush_line",
		"ml_new_data", "ml_new_ptr", "ml_find_line", "ml_lineadd"} {
		for {
			fn := e.Defn(name)
			if fn == nil {
				return die("%s is not a definition head exactly once", name)
			}
			c, err := graph.FormsC([]*graph.Node{fn})
			if err != nil {
				return die("%v", err)
			}
			text := string(c)
			if i := strings.Index(text, "\n"+name+"("); i >= 0 {
				text = text[i+1:]
			}
			var gone *graph.Node
			var local string
			graph.Walk(fn, func(x *graph.Node) bool {
				if gone == nil && x.Is("def") {
					if n := graph.DeclName(x); n != "" && edit.WordCount(text, n) == 1 {
						gone, local = x, n
					}
				}
				return gone == nil
			})
			if gone == nil {
				break
			}
			if err := e.Delete(gone); err != nil {
				return die("%s's %s -- %v", name, local, err)
			}
			dropped = append(dropped, name+":"+local)
		}
	}
	say("%d locals the fold stopped using, found by counting their own name: %s",
		len(dropped), strings.Join(dropped, " "))

	// --- the partition again, on the output ----------------------------------
	landed := []string{"PB_COUNT_MAX", "bh_id", "pb_hdr", "db_hdr", "ml_free_tree"}
	said1 := w56Say(e, append(append(append([]string{"ml_root"}, w56Gone...), w56ForSweep...), landed...)...)
	textWith := func(name string) (string, error) {
		_, t, err := said1.lines(name)
		return t, err
	}
	for _, name := range w56Gone {
		t, err := textWith(name)
		if err != nil {
			return err
		}
		want := 0
		if name == "memfile" || name == "memfile_T" {
			want = 1 // the typedef, the sweep's
		}
		if k := edit.WordCount(t, name); k != want {
			return die("%s survives the edit with %d mentions", name, k)
		}
	}
	for _, name := range w56ForSweep {
		t, err := textWith(name)
		if err != nil {
			return err
		}
		if k := edit.WordCount(t, name); k != 1 {
			return die("%s is left at %d mentions and the edit leaves exactly one -- its own "+
				"definition -- for the sweep to take", name, k)
		}
	}
	for _, name := range landed {
		t, err := textWith(name)
		if err != nil {
			return err
		}
		if edit.WordCount(t, name) == 0 {
			return die("%s is not in the output, so the replacement did not land", name)
		}
	}
	// THE OPEN-BUFFER PREDICATE MOVED 1:1, stated as a partition over the
	// FUNCTIONS that ask it rather than as a count of mentions.
	askers := func(forms [][]string, field string) []string {
		re := regexp.MustCompile(`\b` + field + `\b *(==|!=) *nullptr`)
		seen := map[string]bool{}
		for _, ls := range forms {
			for i, l := range ls {
				if re.MatchString(l) {
					seen[w56Enclosing(ls, i)] = true
				}
			}
		}
		var Out []string
		for k := range seen {
			Out = append(Out, k)
		}
		sort.Strings(Out)
		return Out
	}
	nowForms, _, err := said1.lines("ml_root")
	if err != nil {
		return err
	}
	was, now := askers(wasForms, "ml_mfp"), askers(nowForms, "ml_root")
	if len(askers(rootForms, "ml_root")) > 0 {
		return die("ml_root is already compared with nullptr in the input, so this edit cannot " +
			"say that the open-buffer question moved onto it")
	}
	wantSet := map[string]bool{"ml_delete_int": true}
	for _, x := range was {
		wantSet[x] = true
	}
	var want []string
	for k := range wantSet {
		want = append(want, k)
	}
	sort.Strings(want)
	if strings.Join(want, "\x00") != strings.Join(now, "\x00") {
		return die("the \"is this buffer's memline open\" question is asked in %s and it was asked "+
			"in %s; the only one this edit adds is ml_delete_int, which asked it through a "+
			"local copy of the handle", vimtext.PyList(now), vimtext.PyList(was))
	}
	say("the question \"does this buffer have a memline\" moved from ml_mfp to ml_root in "+
		"all %d functions that asked it, plus ml_delete_int, which asked it through its own "+
		"copy of the handle -- and ml_root was compared with nullptr in none of them before",
		len(was))
	say("a node is ONE allocation at its own size, `bhdr_T` is its tag and the first member " +
		"of both kinds, and there is no memfile")
	return nil
}
