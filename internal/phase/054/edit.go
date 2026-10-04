package p054

// Whim phase 54 (formerly 126) -- a block number becomes a reference.
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
// phase 55's.  A reference to a block whose innards are still a byte array is halfway.
//
// WHAT WAS THERE.  `mf_new()` handed every block an integer from a counter, inserted it
// in `mf_hash` under that integer, and the tree stored the integer; `mf_get()` took the
// integer back and hashed it to the page.  Nothing had been written to a disk since zero
// phase 28 and nothing could be read from one since phase 31, so the hash had held every
// live block for thirty-four phases and a lookup could not miss.  Four measurements say
// that in the text rather than as a story, and they are this edit's first act:
//
// * every block `mf_new()` makes is inserted in the hash, and the only thing that ever
// removes one is `mf_free()`, which removes it from the used list in the same breath;
// * `mf_get()`'s two ways of failing are `nr >= mf_blocknr_max || nr < 0` and a miss,
// and no caller passes anything but a number the tree stored;
// * the used list is not an ordering anything reads.  `mf_used_last` is WRITE-ONLY
// here -- phase 53 took `ml_setflags()`, its last reader -- and there is no release
// path left to walk it: `mf_release_all` is WHIM'S, three mentions in `slim-vim.c`
// and none in `whim-vim.c`, and phase 53 showed `mf_dont_release` to be a constant.
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
// either direction.  Phase 39's trap is the other half of it: remove a member and leave
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
// NOT create_cmdidxs --check, for internal/phase/004/e/edit.go's reason: the derived
// first-two-letters index went with the command table whim's phase 26 reduced, and the
// tool raises rather than reporting nothing.  Nothing here touches the command table.
//

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, *R1 as built*): the two block
// numbers are RETYPED and RENAMED members (`pe_block`, `ip_block`), and so
// are the locals that carried them and mf_get()'s parameter, so that every
// use keeps its node; `ml_root` is a member inserted; the statements that
// changed are rewritten in place or built where they stand; mf_get()
// loses its page count by PARAM, at every call; the wrappers, the free
// list and the hash go as the runs of forms the text cut; the types and
// members last, once nothing names them.  Every partition is the text's,
// on the lines of the forms that say the name (graph.FormLines); the
// literals and the line counts are the C view's.

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim54", Edit) }

var w54Lit = regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'`)

// w54Names are the names this edit partitions, plus the three it introduces.
var w54Names = []string{"blocknr_T", "mf_hashitem_T", "mf_hashtab_T", "mhi_key", "mhi_next",
	"mhi_prev", "bh_hashitem", "pe_bnum", "ip_bnum", "pe_page_count",
	"bh_page_count", "mf_blocknr_max", "mf_free_first", "mf_used_last",
	"mf_hash", "ml_root", "pe_block", "ip_block"}

var (
	w54HashImpl = []string{"mf_hash_init", "mf_hash_free", "mf_hash_find", "mf_hash_add_item",
		"mf_hash_rem_item", "mf_hash_grow"}
	w54HashWrap = []string{"mf_ins_hash", "mf_rem_hash", "mf_find_hash"}
	w54FreeList = []string{"mf_ins_free", "mf_rem_free"}
	// w54Protos are the forward declarations this edit leaves for the sweep:
	// their functions go here, and they name no type that goes.
	w54Protos = []string{"mf_ins_hash", "mf_rem_hash", "mf_ins_free", "mf_rem_free"}
)

// w54Locals are the functions whose locals and parameters this phase's
// names may be.
var w54Locals = []string{"ml_find_line", "ml_append_int", "mf_get", "mf_new", "ml_open"}

// Edit turns a block number into a reference: `pe_bnum` and `ip_bnum` become
// `bhdr_T *`, `memline_T` gains `ml_root`, and the hash table that turned an
// integer into a page goes with the free list and `mf_blocknr_max`.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	p := edit.Ph{Tag: "refblocks", W: w}
	if len(args) != 1 {
		return p.Die("usage: edit whim54 <file> <state-dir>")
	}
	state := args[0]
	v := graph.NewVerbs("refblocks", e, io.Discard)
	failed := func() error { return v.Err }

	decls := func(name string) []*graph.Node {
		ds := append(e.Decls(name), e.MemberDecls(name)...)
		for _, f := range w54Locals {
			if fn := e.Defn(f); fn != nil {
				ds = append(ds, e.LocalDecls(fn, name)...)
			}
		}
		var out []*graph.Node
		for _, d := range ds {
			if e.Live(d) {
				out = append(out, d)
			}
		}
		return out
	}
	lines := func(name string) ([]graph.FormLine, error) {
		return e.FormLines(e.AndUses(decls(name)...)...)
	}
	// owners: {function name: how many of its lines say `name`}.  THIS IS THE
	// PHASE'S UNIT OF ASSERTION -- WHERE a name is said and not how often, because
	// the phase before this one moves the counts.
	owners := func(name string) (map[string]int, error) {
		ls, err := lines(name)
		if err != nil {
			return nil, err
		}
		re := regexp.MustCompile(`\b` + name + `\b`)
		d := map[string]int{}
		for _, l := range ls {
			if re.MatchString(l.Text) {
				who := l.Func
				if who == "" {
					who = "<file scope>"
				}
				d[who]++
			}
		}
		return d, nil
	}
	places := func(name string, expect []string, what string) error {
		got, err := owners(name)
		if err != nil {
			return p.Die("%v", err)
		}
		var gk []string
		for k := range got {
			gk = append(gk, k)
		}
		sort.Strings(gk)
		ek := append([]string{}, expect...)
		sort.Strings(ek)
		if strings.Join(gk, "\x00") != strings.Join(ek, "\x00") {
			return p.Die("`%s` is said in %s and this phase accounts for %s -- %s",
				name, vimtext.PyList(gk), vimtext.PyList(ek), what)
		}
		var parts []string
		for _, k := range gk {
			parts = append(parts, fmt.Sprintf("%s %d", k, got[k]))
		}
		p.Sayf("`%s` is said in %d places and nowhere else: %s",
			name, len(got), strings.Join(parts, ", "))
		return nil
	}
	mentions := func(name string) int {
		ls, err := lines(name)
		if err != nil {
			return -1
		}
		var b strings.Builder
		for _, l := range ls {
			b.WriteString(l.Text)
			b.WriteByte('\n')
		}
		return edit.WordPatternCount(b.String(), name)
	}
	// forms are the top-level forms from the one declaring from, up to and
	// not including the one declaring to: a run the text cut, and its lines
	// with the blank one after each.
	forms := func(from, to *graph.Node) ([]*graph.Node, int, error) {
		var run []*graph.Node
		in := false
		for _, f := range e.Graph().Forms {
			if f == from {
				in = true
			}
			if f == to {
				break
			}
			if in {
				run = append(run, f)
			}
		}
		if from == nil || to == nil || len(run) == 0 {
			return nil, 0, fmt.Errorf("the run of forms is not in the file")
		}
		c, err := graph.FormsC(run)
		if err != nil {
			return nil, 0, err
		}
		return run, strings.Count(string(c), "\n") + 1, nil
	}
	cutForms := func(run []*graph.Node, what string) error {
		for _, f := range run {
			if err := e.Delete(f); err != nil {
				return p.Die("%s -- %v", what, err)
			}
		}
		return nil
	}
	member := func(name string) *graph.Node {
		ms := e.MemberDecls(name)
		if len(ms) != 1 {
			v.Die("the member %s is declared %d times", name, len(ms))
			return nil
		}
		return ms[0]
	}
	local := func(fn, name string) *graph.Node {
		ds := e.LocalDecls(e.Defn(fn), name)
		if len(ds) != 1 {
			v.Die("%s's %s is declared %d times", fn, name, len(ds))
			return nil
		}
		return ds[0]
	}
	retype := func(d *graph.Node, typ, to, what string) {
		if v.Err != nil || d == nil {
			return
		}
		if _, err := e.Retype(d, typ); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
		if to != "" {
			if _, err := e.Rename(d, to); err != nil {
				v.Die("%s -- %v", what, err)
			}
		}
	}
	// build puts the C-lisp src in at's place, or after it.
	build := func(fn, pat, src string, after bool, what string) {
		v.InFunction(fn, func(v *graph.Verbs) {
			at := v.One(pat, what)
			if at == nil {
				return
			}
			ns, err := e.Build(at, src, nil)
			if err == nil {
				if after {
					err = e.InsertAfter(at, ns...)
				} else {
					err = e.Replace(at, ns...)
				}
			}
			if err != nil {
				v.Die("%s -- %v", what, err)
			}
		})
	}
	// right puts the C-lisp src in the place of the right side of the one
	// assignment pat matches.
	right := func(fn, pat, src, what string) {
		v.InFunction(fn, func(v *graph.Verbs) {
			at := v.One(pat, what)
			if at == nil {
				return
			}
			ns, err := e.Build(at.Kids[2], src, nil)
			if err == nil {
				err = e.Replace(at.Kids[2], ns...)
			}
			if err != nil {
				v.Die("%s -- %v", what, err)
			}
		})
	}

	// ---- 0. the file this edit is handed --------------------------------------
	t0, err := e.Graph().C()
	if err != nil {
		return err
	}
	t := string(t0)
	nIn := strings.Count(t, "\n")
	literals := w54Lit.FindAllString(t, -1)
	var inlit []string
	for _, n := range w54Names {
		re := regexp.MustCompile(`\b` + n + `\b`)
		for _, s := range literals {
			if re.MatchString(s) {
				inlit = append(inlit, n)
				break
			}
		}
	}
	if len(inlit) > 0 {
		return p.Die("%s appear inside a string literal, so a line-oriented partition would read "+
			"data as code", strings.Join(inlit, ", "))
	}
	for _, n := range []string{"ml_root", "pe_block", "ip_block"} {
		if k := edit.WordPatternCount(t, n); k != 0 {
			return p.Die("`%s` is already said %d times, and this phase is what introduces it", n, k)
		}
	}
	p.Sayf("%d string and character literals, and not one of them holds any of the %d names "+
		"this edit partitions or the 3 it introduces", len(literals), len(w54Names)-3)

	incs := e.Includes()
	if len(incs) == 0 {
		return p.Die("the input has no preprocessor directive, and this phase adds none and removes none")
	}
	boundary := strings.Count(t[:strings.Index(t, "\n#include ")+1], "\n")
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
		{"mf_free_first", append([]string{"<file scope>", "mf_open", "mf_close", "mf_new"}, w54FreeList...),
			"it is the head of the free list, which is keyed by block number"},
		{"mf_used_last", []string{"<file scope>", "mf_open", "mf_ins_used", "mf_rem_used"},
			"it is the tail of the used list"},
	} {
		if err := places(pl.Name, pl.expect, pl.What); err != nil {
			return err
		}
	}

	ls, err := lines("pe_page_count")
	if err != nil {
		return p.Die("%v", err)
	}
	peRe := regexp.MustCompile(`\bpe_page_count\b`)
	peWr := regexp.MustCompile(`\bpe_page_count\s*=`)
	var peReads []string
	for _, l := range ls {
		if peRe.MatchString(l.Text) && !peWr.MatchString(l.Text) && !strings.Contains(l.Text, "int pe_page_count;") {
			peReads = append(peReads, l.Text)
		}
	}
	if len(peReads) != 1 || !strings.Contains(peReads[0], "page_count =") {
		return p.Die("`pe_page_count` has %d readers and this phase rests on its having one, the "+
			"argument mf_get() is about to lose", len(peReads))
	}
	p.Sayf("`pe_page_count` is read ONCE in the whole file -- %s -- and that read is the third "+
		"argument of mf_get(); every other mention of it is a write", strings.TrimSpace(peReads[0]))

	if ls, err = lines("mf_used_last"); err != nil {
		return p.Die("%v", err)
	}
	ulRe := regexp.MustCompile(`\bmf_used_last\b`)
	ulWr := regexp.MustCompile(`\bmf_used_last\s*=`)
	var lastw int
	var lastr []string
	for _, l := range ls {
		if ulRe.MatchString(l.Text) {
			lastw++
			if !ulWr.MatchString(l.Text) && !strings.Contains(l.Text, "bhdr_T *mf_used_last;") {
				lastr = append(lastr, strings.TrimSpace(l.Text))
			}
		}
	}
	if len(lastr) > 0 {
		return p.Die("`mf_used_last` is read at %s, and this phase removes it as a write-only field",
			strings.Join(lastr, ", "))
	}
	p.Sayf("`mf_used_last` IS WRITE-ONLY: %d mentions, its declaration and %d writes and not "+
		"one read -- phase 53 took ml_setflags(), which was the last thing that walked the "+
		"used list backwards.  No warning gcc emits covers a struct member in either "+
		"direction, so it goes in this edit with its writes", lastw, lastw-1)

	callIn := func(name string) []string {
		ls, _ := lines(name)
		re := regexp.MustCompile(`\b` + name + `\s*\(`)
		seen := map[string]bool{}
		for _, l := range ls {
			if re.MatchString(l.Text) && l.Func != "" && l.Func != name {
				seen[l.Func] = true
			}
		}
		var Out []string
		for k := range seen {
			Out = append(Out, k)
		}
		sort.Strings(Out)
		return Out
	}
	insIn, remIn := callIn("mf_ins_hash"), callIn("mf_rem_hash")
	if strings.Join(insIn, ",") != "mf_get,mf_new" || strings.Join(remIn, ",") != "mf_free,mf_get" {
		return p.Die("the hash is inserted into from %s and removed from from %s, and this phase "+
			"rests on mf_new() and mf_get() being the only insertions and mf_free() and "+
			"mf_get() the only removals", vimtext.PyList(insIn), vimtext.PyList(remIn))
	}
	p.Sayf("THE HASH HOLDS EVERY LIVE BLOCK: it is inserted into by %s and removed from by "+
		"%s, and mf_get() does both in one breath to move a block to the head of the used "+
		"list -- so a lookup by a number the tree stored cannot miss, and the pointer it "+
		"would have returned is the same answer",
		strings.Join(insIn, " and "), strings.Join(remIn, " and "))

	// what the text's summary lines count, before the acts change it
	leftStores := v.Count("(= (. (index (-> _ pb_pointer) _) pe_bnum) bnum_left)")
	rightStores := v.Count("(= (. (index (-> _ pb_pointer) _) pe_bnum) bnum_right)")
	fetches := v.Count("(call mf_get mfp (-> ip ip_bnum) 1)")

	// ---- 2. the types: the two numbers are blocks, the memline names its root
	retype(member("pe_bnum"), "(ptr bhdr_T)", "pe_block", "the pointer entry")
	retype(member("ip_bnum"), "(ptr bhdr_T)", "ip_block", "the stack entry's block")
	if failed() != nil {
		return failed()
	}
	if _, err := e.InsertMember(member("ml_mfp"), true, "(ml_root (ptr bhdr_T))"); err != nil {
		return p.Die("the memline -- %v", err)
	}

	// ---- 3. the memfile -------------------------------------------------------
	// Eleven functions go, and they go HERE and not to tools/sweep.sh: every one
	// names a type or a field removed above, so leaving them for the sweep would
	// leave a file that does not compile for the sweep to ask gcc about.  Seven
	// of their forward declarations name such a type too and go here; the other
	// four (w54Protos) name none, and the sweep takes them with their
	// definitions.
	all := append(append(append([]string{}, w54HashWrap...), w54FreeList...), w54HashImpl...)
	var cutProtos []string
	for _, name := range all {
		if edit.ContainsStr(w54Protos, name) {
			continue
		}
		var proto *graph.Node
		for _, d := range e.FileDecls(name) {
			if d.Is("def") {
				proto = d
			}
		}
		if proto == nil {
			return p.Die("`%s` has no forward declaration in the one shape this tree writes them", name)
		}
		if err := e.Delete(proto); err != nil {
			return p.Die("`%s`'s forward declaration -- %v", name, err)
		}
		cutProtos = append(cutProtos, name)
	}
	p.Sayf("%d forward declarations go: %s", len(cutProtos), strings.Join(cutProtos, ", "))

	v.InFunction("mf_open", func(v *graph.Verbs) {
		v.Cut("(= (-> mfp mf_free_first) nullptr)", 1, "mf_open()'s body")
		v.Cut("(= (-> mfp mf_used_last) nullptr)", 1, "mf_open()'s body")
		v.Cut("(call mf_hash_init (addr (-> mfp mf_hash)))", 1, "mf_open()'s body")
		v.Cut("(= (-> mfp mf_blocknr_max) 0)", 1, "mf_open()'s body")
	})
	v.InFunction("mf_close", func(v *graph.Verbs) {
		v.Cut("(while (!= (-> mfp mf_free_first) nullptr) _)", 1, "mf_close()'s tail")
		v.Cut("(call mf_hash_free (addr (-> mfp mf_hash)))", 1, "mf_close()'s tail")
	})
	v.InFunction("mf_new", func(v *graph.Verbs) {
		v.Cut("(def freep (ptr bhdr_T))", 1, "mf_new()")
		v.Cut("(def p (ptr char_u))", 1, "mf_new()")
		v.Cut("(= hp nullptr)", 1, "mf_new()")
		v.Cut("(= freep (-> mfp mf_free_first))", 1, "mf_new()")
		v.Rewrite("(if (&& (!= freep nullptr) (>= (-> freep bh_page_count) page_count)) _ _)",
			"(if (== (= hp (call mf_alloc_bhdr mfp page_count)) nullptr) (block (return nullptr)))", 1, "mf_new()")
		v.Cut("(= (-> hp bh_page_count) page_count)", 1, "mf_new()")
		v.Cut("(call mf_ins_hash mfp hp)", 1, "mf_new()")
	})
	if failed() != nil {
		return failed()
	}
	// mf_get(mfp, nr, page_count) is mf_get(mfp, hp): its parameter the block
	nr := local("mf_get", "nr")
	retype(nr, "(ptr bhdr_T)", "", "mf_get()")
	v.Body("mf_get", "(if (== nr nullptr) (block (return nullptr))) (call mf_rem_used mfp nr)"+
		" (|= (-> nr bh_flags) BH_LOCKED) (call mf_ins_used mfp nr) (return nr)", "mf_get()")
	if failed() != nil {
		return failed()
	}
	if _, err := e.Rename(nr, "hp"); err != nil {
		return p.Die("mf_get() -- %v", err)
	}
	if _, err := e.DropParam("mf_get", "page_count", graph.ParamOptions{}); err != nil {
		return p.Die("mf_get() -- %v", err)
	}
	v.InFunction("mf_free", func(v *graph.Verbs) {
		v.Cut("(call vim_free (-> hp bh_data))", 1, "mf_free()")
		v.Cut("(call mf_rem_hash mfp hp)", 1, "mf_free()")
		v.Rewrite("(call mf_ins_free mfp hp)", "(call mf_free_bhdr hp)", 1, "mf_free()")
	})
	if failed() != nil {
		return failed()
	}
	run, n, err := forms(e.Defn("mf_ins_hash"), e.Defn("mf_ins_used"))
	if err != nil {
		return p.Die("the three hash wrappers -- %v", err)
	}
	if err := cutForms(run, "the three hash wrappers"); err != nil {
		return err
	}
	p.Sayf("the three one-line wrappers go, %d lines: %s", n, strings.Join(w54HashWrap, ", "))

	v.InFunction("mf_ins_used", func(v *graph.Verbs) {
		v.Rewrite("(if (== (-> hp bh_next) nullptr) (block (= (-> mfp mf_used_last) hp)) ?b)",
			"(if (!= (-> hp bh_next) nullptr) ?b)", 1, "mf_ins_used()'s tail")
	})
	v.InFunction("mf_rem_used", func(v *graph.Verbs) {
		v.Rewrite("(if (== (-> hp bh_next) nullptr) (block (= (-> mfp mf_used_last) (-> hp bh_prev))) ?b)",
			"(if (!= (-> hp bh_next) nullptr) ?b)", 1, "mf_rem_used()'s head")
	})
	v.InFunction("mf_alloc_bhdr", func(v *graph.Verbs) {
		v.Cut("(= (-> hp bh_page_count) page_count)", 1, "mf_alloc_bhdr()'s page count")
	})
	if failed() != nil {
		return failed()
	}
	var ptrBL *graph.Node
	for _, d := range e.Decls("PTR_BL") {
		if d.Is("typedef") {
			ptrBL = d
		}
	}
	if run, n, err = forms(e.Defn("mf_ins_free"), ptrBL); err != nil {
		return p.Die("the free list and the whole hash implementation -- %v", err)
	}
	if err := cutForms(run, "the free list and the whole hash implementation"); err != nil {
		return err
	}
	p.Sayf("the free list and the hash implementation go, %d lines: %s, the two MHT_ "+
		"enumerators and %s", n, strings.Join(w54FreeList, ", "), strings.Join(w54HashImpl, ", "))

	// ---- 4. the memline -------------------------------------------------------
	v.InFunction("ml_open", func(v *graph.Verbs) {
		v.Rewrite("(if (!= (. (-> hp bh_hashitem) mhi_key) 0) _)", "(= (. (-> buf b_ml) ml_root) hp)", 1, "ml_open()'s two blocks")
		v.Cut("(= (. (index (-> pp pb_pointer) 0) pe_block) 1)", 1, "ml_open()'s two blocks")
		v.Cut("(= (. (index (-> pp pb_pointer) 0) pe_page_count) 1)", 1, "ml_open()'s two blocks")
		v.Rewrite("(if (!= (. (-> hp bh_hashitem) mhi_key) 1) _)",
			"(= (. (index (-> (cast (ptr PTR_BL) (paren (-> (. (-> buf b_ml) ml_root) bh_data))) pb_pointer) 0) pe_block) hp)", 1,
			"ml_open()'s two blocks")
	})
	build("ml_open", "(= (. (-> buf b_ml) ml_stack_size) 0)", "(= (. (-> buf b_ml) ml_root) nullptr)", true, "ml_open()'s preamble")
	retype(local("ml_find_line", "bnum"), "(ptr bhdr_T)", "bp", "ml_find_line()'s cursor")
	right("ml_find_line", "(= bp 0)", "(. (-> buf b_ml) ml_root)", "where a descent starts")
	v.InFunction("ml_find_line", func(v *graph.Verbs) {
		v.Cut("(= page_count 1)", 1, "where a descent starts")
		v.Cut("(= page_count (. (index (-> pp pb_pointer) idx) pe_page_count))", 1, "the step down")
		v.Cut("(call vim_snprintf (cast (ptr char) IObuff) (call emsg_iobuff_room) e_line_count_wrong_in_block_nr bp)", 1, "E323")
		v.Rewrite("(call iemsg (call iobuff_or e_line_count_wrong_in_block_nr))", "(call iemsg e_line_count_wrong_in_block_nr)", 1, "E323")
	})
	if failed() != nil {
		return failed()
	}
	e323 := e.FileDecls("e_line_count_wrong_in_block_nr")
	if len(e323) != 1 {
		return p.Die("E323's text -- declared %d times", len(e323))
	}
	if err := e.Replace(graph.DeclInit(e323[0]), graph.NewAtom(`"E323: Line count wrong in block"`)); err != nil {
		return p.Die("E323's text -- %v", err)
	}
	if _, err := e.Rename(e323[0], "e_line_count_wrong_in_block"); err != nil {
		return p.Die("E323's text -- %v", err)
	}
	retype(local("ml_append_int", "bnum_left"), "(ptr bhdr_T)", "bp_left", "ml_append_int()'s two labels")
	retype(local("ml_append_int", "bnum_right"), "(ptr bhdr_T)", "bp_right", "ml_append_int()'s two labels")
	right("ml_append_int", "(= bp_left (. (-> hp_left bh_hashitem) mhi_key))", "hp_left", "what a data-block split labels its halves with")
	right("ml_append_int", "(= bp_right (. (-> hp_right bh_hashitem) mhi_key))", "hp_right", "what a data-block split labels its halves with")
	right("ml_append_int", "(= (. (index (-> pp pb_pointer) 0) pe_block) (. (-> hp_new bh_hashitem) mhi_key))", "hp_new", "the root split")
	right("ml_append_int", "(= bp_left (. (-> hp bh_hashitem) mhi_key))", "hp", "what a pointer-block split labels its halves with")
	right("ml_append_int", "(= bp_right (. (-> hp_new bh_hashitem) mhi_key))", "hp_new", "what a pointer-block split labels its halves with")
	v.InFunction("ml_append_int", func(v *graph.Verbs) {
		v.Cut("(= page_count_left (-> hp_left bh_page_count))", 1, "what a data-block split labels its halves with")
		v.Cut("(= page_count_right (-> hp_right bh_page_count))", 1, "what a data-block split labels its halves with")
		v.Rewrite("(!= (. (-> hp bh_hashitem) mhi_key) 0)", "(!= hp (. (-> buf b_ml) ml_root))", 1, "the root test")
		v.Cut("(= (. (index (-> pp pb_pointer) 0) pe_page_count) 1)", 1, "the root split")
		v.Cut("(= page_count_left 1)", 1, "what a pointer-block split labels its halves with")
		v.Cut("(= page_count_right 1)", 1, "what a pointer-block split labels its halves with")
	})
	if failed() != nil {
		return failed()
	}

	// The seven remaining stores, as a partition over the two labels: every store
	// of a pointer entry's block is one of these two (renamed with the labels).
	for _, s := range []struct {
		n    int
		side string
	}{{leftStores, "left"}, {rightStores, "right"}} {
		if s.n < 1 {
			return p.Die("no store of the %s label is left, and a split has two sides", s.side)
		}
		p.Sayf("%d stores of the %s label", s.n, s.side)
	}
	for _, pc := range []string{
		"(= (. (index (-> pp pb_pointer) pb_idx) pe_page_count) page_count_left)",
		"(= (. (index (-> pp pb_pointer) (+ pb_idx 1)) pe_page_count) page_count_right)",
		"(= (. (index (-> pp_new pb_pointer) 0) pe_page_count) page_count_right)",
	} {
		k := v.Count(pc)
		if k < 1 {
			return p.Die("a page-count store this phase accounts for is not there: %s", pc)
		}
		v.Cut(pc, k, "a page-count store")
	}
	if fetches < 1 {
		return p.Die("no stack-walk fetch is left, and the three loops that climb the tree all make one")
	}
	p.Sayf("%d stack-walk fetches become mf_get(mfp, ip->ip_block)", fetches)

	// the members and types nothing names now
	for _, m := range []string{"bh_hashitem", "bh_page_count", "mf_free_first", "mf_used_last", "mf_hash",
		"mf_blocknr_max", "pe_page_count"} {
		d := member(m)
		if failed() != nil {
			return failed()
		}
		if err := e.Delete(d); err != nil {
			return p.Die("the block header and the hash types -- %v", err)
		}
	}
	for _, f := range []string{"mf_hashtab_T", "MHT_INIT_SIZE", "mf_hashitem_T", "blocknr_T"} {
		for _, d := range e.Decls(f) {
			if q := e.Parent(d); q != nil && q.Is("enum") {
				d = e.TopForm(d)
			}
			if err := e.Delete(d); err != nil {
				return p.Die("the block header and the hash types -- %v", err)
			}
		}
	}
	for _, f := range e.Graph().Forms {
		if f.Is("struct") && graph.Tag(f) == "mf_hashitem_S" {
			if err := e.Delete(f); err != nil {
				return p.Die("the block header and the hash types -- %v", err)
			}
			break
		}
	}

	// ---- 5. what is left, as a partition --------------------------------------
	gone := append([]string{"blocknr_T", "mf_hashitem_T", "mf_hashtab_T", "mhi_key", "mhi_next", "mhi_prev",
		"mht_mask", "mht_count", "mht_buckets", "mht_small_buckets", "mht_fixed",
		"MHT_INIT_SIZE", "MHT_LOG_LOAD_FACTOR", "MHT_GROWTH_FACTOR", "bh_hashitem",
		"pe_bnum", "ip_bnum", "pe_page_count", "bh_page_count", "mf_blocknr_max",
		"mf_free_first", "mf_used_last", "mf_hash", "bnum_left", "bnum_right"}, all...)
	// left for the sweep, each named once: the four forward declarations, and
	// ml_append_int()'s page-count locals, declared and no longer written or
	// read (ml_find_line()'s page_count is left the same way).
	once := append([]string{"page_count_left", "page_count_right"}, w54Protos...)
	var left []string
	for _, n := range once {
		if k := mentions(n); k != 1 {
			return p.Die("`%s` has %d mentions, and this edit leaves it one, its declaration, "+
				"for the sweep", n, k)
		}
	}
	for _, n := range gone {
		if edit.ContainsStr(w54Protos, n) {
			continue
		}
		if k := mentions(n); k > 0 {
			left = append(left, fmt.Sprintf("%s %d", n, k))
		}
	}
	if len(left) > 0 {
		return p.Die("names this phase removes are still said: %s", strings.Join(left, ", "))
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
			return err
		}
	}

	standing := []string{"e_didnt_get_block_nr_zero", "e_didnt_get_block_nr_one"}
	for _, name := range standing {
		if k := mentions(name); k != 1 {
			return p.Die("`%s` has %d mentions and this edit leaves it at one, its own definition, "+
				"which is what the sweep takes", name, k)
		}
	}
	p.Sayf("%d names are left standing for tools/sweep.sh: %s -- each is an unreferenced "+
		"file-scope object, which is -Wunused-variable and the one kind of dead thing in "+
		"this phase that a tool can see", len(standing), strings.Join(standing, ", "))

	// ---- 6. the shape of what is written Out ----------------------------------
	incs2 := e.Includes()
	at := map[*graph.Node]int{}
	for i, f := range e.Graph().Forms {
		at[f] = i
	}
	if len(incs2) != len(incs) {
		return p.Die("the output has %d `#include`s, against %d", len(incs2), len(incs))
	}
	for i := range incs2 {
		if at[incs2[i]] != at[incs2[0]]+i {
			return p.Die("the eleven `#include`s are not contiguous any more")
		}
	}
	out, err := e.Graph().C()
	if err != nil {
		return err
	}
	nOut := strings.Count(string(out), "\n")
	p.Sayf("%d -> %d lines before the sweep, %d fewer, the %d `#include`s untouched and still "+
		"contiguous", nIn, nOut, nIn-nOut, len(incs2))

	// The edit leaves its own output beside the input, so the check can say what
	// the EDIT removed and what the SWEEP removed separately.
	if err := os.WriteFile(state+"/edit.c", out, 0o644); err != nil {
		return p.Die("%v", err)
	}
	if err := os.WriteFile(state+"/boundary-in", []byte(fmt.Sprintf("%d\n", boundary+1)), 0o644); err != nil {
		return p.Die("%v", err)
	}
	return nil
}
