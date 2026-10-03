package p052

// Whim phase 52 (formerly 124) -- freeing is free.  GOALS.md's charter bullet, "A GARBAGE
// COLLECTOR IS ASSUMED FROM HERE ON".
//
// `host_alloc` BECOMES A BUMP ALLOCATOR AND `host_free` RETURNS WITHOUT DOING ANYTHING.
// Phase 49b moved `malloc` and `free` across the boundary and wrote the two wrappers that
// forwarded to them; this phase changes what is behind those two names and nothing else.
// The charter's words: "No real collector is built: `host_alloc` becomes a bump allocator
// in the host with enough arena for the test suite and `host_free` returns without doing
// anything, which is a change entirely below the boundary and touches no core line."
//
// THE CLAIM IS "FREEING IS NOW FREE" AND NOT "THE CORE STOPPED FREEING".  Every
// `host_free` call the core makes is still there and still made; what it costs is a
// store of a parameter and a return.  Some later phase may delete the calls, and it
// will be able to, which is the point of doing this one first.
//
// WHAT THE CORPUS ASKED FOR, MEASURED AND NOT GUESSED.  The input source was built with
// a counter on host_alloc that totals every request, rounded as the allocator below
// rounds it, and dumped from host_exit() -- which every session reaches.  Over the whole
// of tools/st.sh zrecord, 268 sessions in 122 records, the largest single session asked for
// 200,458,672 bytes, and two separate recordings gave that same number.  It is ONE case:
//
// the heaviest memline case, mem_deep_jumps, 25,000 lines     200,458,672
// the next three memline cases                    53,134,304 / 52,559,280 / 52,506,976
// the heaviest of the 102 screen cases                          1,722,512
// a buffer of 100,000 lines                                    12,862,224
// a buffer of 300,000 lines           35,157,264   (~112 bytes a line, LINEAR)
// 200,000 characters into ONE line                         20,013,114,624 (QUADRATIC)
//
// THE ARENA COULD NOT HAVE BEEN SIZED BEFORE RECORD 123, and that is worth saying
// plainly rather than leaving in the manifest.  The heaviest of the 102 screen cases asks
// for 1,722,512 bytes and the heaviest of record 123's 16 memline cases asks for 115 TIMES
// MORE.  A phase written one boundary earlier would have measured the 102, found 1.7 MB,
// and sized an arena from a corpus that provably cannot reach the text layer at all --
// which is the defect record 123 exists to have ended, arriving one phase later in a shape
// nobody predicted.  A 64 MiB arena was in fact written here first and the recording
// refused it: `THE RECORDING MOVED, in 1 of 122 records: memline/mem_deep_jumps`, the case
// dying with `host arena exhausted: 67108864 bytes, 67058640 used, request 60263`.
//
// THE LAST ROW IS THE REASON A FIXED ARENA IS A STATEMENT ABOUT THE WORKLOAD AND NOT
// ABOUT THE EDITOR, and it is written here rather than discovered later.  With nothing
// freed, what an arena must hold is not the live data but the TRAFFIC, and this editor's
// traffic is quadratic in the length of a single line being typed: `+normal 200000ax`
// wants twenty gigabytes.  No fixed arena covers that, so the choice is not "how big is
// safe" but "which workloads are covered".
//
// THE SIZE IS 1 GiB AND THE FIRST ARGUMENT FOR IT WAS WRONG, WHICH IS RECORDED HERE
// BECAUSE THE MEASUREMENT THAT KILLED IT IS WORTH MORE THAN THE NUMBER.  This phase first
// chose 64 MiB and justified the ceiling by saying the abort path costs the arena times
// the harness's concurrency -- "64 MiB across 64 threads is 4 GiB on a 62 GiB machine".
// That is FALSE except for a runaway session.  An ordinary session's resident memory is
// its TRAFFIC, which the arena does not change; an untouched arena page costs nothing at
// all.  Measured: the same source at 64 MiB and at 1 GiB produces a byte-identical image,
// 772,872 either way, because .bss is NOBITS.  So the size buys exactly one thing -- how
// far a runaway goes before it dies loudly -- and costs exactly one thing, address space.
// At 1,073,741,824 bytes it is 5.36 times the measured high-water, which is what lets the
// check keep a four-times rule with no weakening to justify.
//
// THE ARENA COSTS NOTHING TO STORE, WHICH IS MEASURED TOO AND IS WHY IT CAN BE THIS
// GENEROUS.  It is a file-scope object with no initialiser, so it is `.bss`, which is
// `NOBITS`: the section header records a size and the file holds no bytes.  Measured on
// this pipeline's own compile line, `.bss` goes from 24,600 bytes to 67,133,528 and the
// IMAGE SHRINKS, 781,064 -> 772,872, because musl's allocator is no longer linked in.
//
// FOUR PARTS, AND THE LAST TWO ARE NOT OPTIONAL.
//
// 1  host_alloc   the arena, the offset, and an abort that NAMES the arena, what is
// used and the request that did not fit.  Not nullptr and not silent:
// lalloc()'s out-of-memory path would turn a wall into a message and
// carry on, and a phase whose declared delta is nothing must not have
// a way of quietly doing less.
// 2  host_free    `(void)p;`.
// 3  format_overflow_error()'s `free(argcopy)`     BELOW the boundary, and phase 49b
// 4  adjust_types()'s `realloc(*ap_types, ...)`    left both deliberately.
//
// PARTS 3 AND 4 ARE WHAT MAKES THIS PHASE CORRECT RATHER THAN NEARLY CORRECT, and
// neither is in the core.  The formatter's private island -- the four functions phase 43
// moved BELOW the includes because they need `va_list` -- still called libc's `free` and
// libc's `realloc` directly, on pointers that came from `alloc_clear()`, which is to say
// from `host_alloc`.  Phase 49b saw them and left them, naming `free`'s one below-boundary
// mention as "format_overflow_error() below the boundary", and phase 49a saw the other and
// said in as many words that the remaining `realloc` "is the host's and is not this
// phase's".  Both were right while `host_alloc` WAS `malloc`: the two allocators were one
// allocator.  This phase is where they stop being one, and a `free()` or a `realloc()` of
// an arena pointer is undefined behaviour from the line below.  So they move here, and
// the realloc moves by phase 49a's own rewrite -- allocate, copy, free -- with phase 49a's
// own traps read off this site:
//
// TRAP 1  `realloc(nullptr, n)` is `malloc(n)`.  Not reachable here: the null case is
// the OTHER arm of the same `if`, which calls alloc_clear().
// TRAP 2  on failure `realloc` leaves the old block valid and allocated.  The
// `return FAIL` is above everything this edit adds, so it still does.
// TRAP 3  `realloc` does not initialise what it grows, and this site does not expect
// it to: the loop below fills `[*num_posarg, arg)` itself.  The copy takes
// the `*num_posarg` entries that were there, the loop fills the rest, and the
// contents are what they were.
//
// NEITHER IS REACHED BY THE CORPUS AND ONE CANNOT BE REACHED AT ALL, which is why the
// check owns a probe for them instead of a recording.  `format_overflow_error()` is
// called only when `get_unsigned_int`'s `overflow_err` is true, and that argument is
// `tvs != nullptr`, and `vim_vsnprintf_typval` has ONE caller in this file, passing
// nullptr -- so it is phase 31's and phase 38a's kind, code no build of whim-vim can run.
// `adjust_types()` needs a positional format spec and no string literal in the file holds
// one, so it takes a runtime format to reach; the check reaches it with one.
//
// WHAT THIS PHASE DOES NOT DO, AND IT IS MEASURED RATHER THAN OVERLOOKED.  `malloc`,
// `free` and `realloc` were the only users of `<stdlib.h>`, and with them gone the
// directive is dead: the check builds the output without it and the binary is
// BYTE-IDENTICAL.  It stays.  GOALS.md permits a phase to remove a directive and
// phase 35 is the precedent for declining -- it measured that removing three of them was
// free and wrote "the count stays 18" into its own program.  The eleven stay eleven here
// for the same reason: this phase's subject is the allocator, the removal is free
// whenever somebody asks for it, and a phase that changes two things cannot say which one
// a difference came from.
//
// THE CORE IS NOT TOUCHED, AND THE EDIT ASSERTS IT RATHER THAN CLAIMING IT.  The text
// above the first `#include` must be byte-identical in and out.  The check states the
// same thing the way the project states it -- `cmp` of `make editor.c`'s own cut.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check records from it.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// edit is the program's graph edited through crefactor/graph, its report the
// text version's, which the plan ran until then (history keeps it):
//
//   - THE PARTITION is the uses of the three, by edge: each a callee in one
//     of the four places, found by its form (host_alloc's and host_free's
//     whole bodies, the call `free(argcopy)`, adjust_types()'s realloc in an
//     else followed by its failure test); the mentions in the C view must
//     be exactly those uses, so a word the edges cannot see refuses too;
//   - the C is written by FRAG, in one synthesized import: the arena before
//     host_alloc()'s definition, the two bodies, the two items of
//     adjust_types() and the one of format_overflow_error(), each at its
//     place (editlit.go, the text's literals cut where they go);
//   - the core is held unchanged by its own C view, before and after; the
//     includes by their forms.
//
// The scratch file the text wrote (arena-bytes, the arena's size for a
// check that no longer exists) was read by nothing and is not written: the
// plan's step no longer passes @state.  The text's last check, that the file
// grew by exactly the lines its replacements held, was a count of the
// text's own lines and has no counterpart; the report says the C view's.

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim52", Edit) }

var w52Names = []string{"malloc", "free", "realloc"}

var (
	w52Dir = regexp.MustCompile(`^ *# *`)
	w52Inc = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)
)

// w52Directives are the C view's directive lines, by index.
func w52Directives(view []byte) ([]int, []string) {
	L := strings.Split(string(view), "\n")
	var d []int
	for i, l := range L {
		if w52Dir.MatchString(l) {
			d = append(d, i)
		}
	}
	return d, L
}

// Edit is phase 52 on the graph: host_alloc() hands out an arena, host_free()
// frees nothing, and the two libc calls below the boundary that were not the
// allocator's own become its.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("arena", e, w)
	p := edit.Ph{Tag: "arena", W: io.Discard}
	incs := e.Includes()
	view := v.Text()
	d, L := w52Directives(view)
	linesBefore := len(L) - 1
	if len(d) != len(incs) {
		v.Die("the file holds %d preprocessor directives and this phase was written against "+
			"the eleven `#include`s phase 40 left", len(d))
		return v.Done()
	}
	for i := range d {
		if d[i] != d[0]+i {
			v.Die("the eleven directives are not eleven consecutive lines")
			return v.Done()
		}
		if !w52Inc.MatchString(L[d[i]]) {
			v.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
			return v.Done()
		}
	}
	boundary := d[0]
	coreBefore, err := graph.FormsC(e.Core())
	if err != nil {
		v.Die("the core's C view: %v", err)
		return v.Done()
	}
	v.Sayf("the boundary is line %d, the first of the eleven `#include`s, and there is not a "+
		"directive above it", boundary+1)

	spans, err := edit.LiteralSpansShort(p, view)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	for _, name := range w52Names {
		re := regexp.MustCompile(`\b` + name + `\b`)
		var bad []string
		for _, s := range spans {
			if re.Match(view[s[0]:s[1]]) {
				bad = append(bad, string(view[s[0]:s[1]]))
			}
		}
		if len(bad) > 0 {
			v.Die("a literal holds the name `%s`: %s", name, strings.Join(edit.First(bad, 3), " / "))
			return v.Done()
		}
	}
	v.Sayf("%d string and character literals, and not one of them holds `malloc`, `free` or "+
		"`realloc`, so every mention the partition below classifies is code", len(spans))

	// the four places, by their forms
	alloc, free := e.Defn("host_alloc"), e.Defn("host_free")
	if alloc == nil || free == nil {
		v.Die("host_alloc() and host_free() are not both defined in this file")
		return v.Done()
	}
	var argFree, realloc, failTest *graph.Node
	classes := []struct {
		what string
		ok   bool
	}{
		{"host_alloc()'s body", len(graph.Body(alloc)) == 1},
		{"host_free()'s body", len(graph.Body(free)) == 1},
	}
	v.In(alloc, func(v *graph.Verbs) {
		classes[0].ok = classes[0].ok && graph.Matches(w52Pattern("(return (call malloc n))"), graph.Body(alloc)[0])
	})
	v.In(free, func(v *graph.Verbs) {
		classes[1].ok = classes[1].ok && graph.Matches(w52Pattern("(call free p)"), graph.Body(free)[0])
	})
	v.InFunction("format_overflow_error", func(v *graph.Verbs) {
		ms := v.Find("(call free argcopy)")
		ok := len(ms) == 1 && e.Item(ms[0]) == ms[0]
		if ok {
			argFree = ms[0]
		}
		classes = append(classes, struct {
			what string
			ok   bool
		}{"format_overflow_error()'s free of argcopy", ok})
	})
	v.InFunction("adjust_types", func(v *graph.Verbs) {
		ms := v.Find("(if _ _ (block (= new_types (call realloc (paren (cast _ (deref ap_types))) _))))")
		ok := len(ms) == 1
		if ok {
			realloc = ms[0].Kids[3].Kids[1]
			failTest = e.Sibling(ms[0], 1)
			ok = failTest != nil && graph.Matches(w52Pattern("(if (== new_types nullptr) (block (return FAIL)))"), failTest)
		}
		classes = append(classes, struct {
			what string
			ok   bool
		}{"adjust_types()'s realloc of *ap_types, with the failure test below it", ok})
	})
	if v.Failed() {
		return v.Done()
	}
	for _, c := range classes {
		if !c.ok {
			v.Die("%s is not in the file in the one shape this phase was written for, so this "+
				"phase has not been handed the file it was written for", c.what)
			return v.Done()
		}
	}
	// every use of the three is one of the four, and every mention a use
	callees := map[*graph.Node]bool{}
	for _, f := range []*graph.Node{graph.Body(alloc)[0].Kids[1], graph.Body(free)[0], argFree, realloc.Kids[2]} {
		callees[f.Kids[1]] = true
	}
	total := map[string]int{}
	var stray []string
	for _, name := range w52Names {
		for _, u := range v.UsesOf(name) {
			if !callees[u] {
				where := "?"
				if f := e.TopForm(u); f != nil {
					where = graph.DeclName(f)
				}
				stray = append(stray, name+" in "+where)
				continue
			}
			if e.InCore(u) {
				v.Die("`%s` is mentioned above the boundary, and phases 49a and 49b took the core's "+
					"last one -- this phase changes the host and nothing else", name)
				return v.Done()
			}
			total[name]++
		}
		if m := edit.MentionCount(view, name); m != total[name] && len(stray) == 0 {
			stray = append(stray, name+": "+strconv.Itoa(m-total[name])+" mention(s) no edge accounts for")
		}
	}
	if len(stray) > 0 {
		s := "s"
		if len(stray) == 1 {
			s = ""
		}
		v.Die("%d mention%s of one of the three is in none of the four classes this phase "+
			"rewrites: %s -- this phase will not leave a libc allocator call behind on a "+
			"pointer the arena handed out", len(stray), s, strings.Join(edit.First(stray, 6), " "))
		return v.Done()
	}
	for _, name := range w52Names {
		if edit.MentionCount(coreBefore, name) > 0 {
			v.Die("`%s` is mentioned above the boundary, and phases 49a and 49b took the core's "+
				"last one -- this phase changes the host and nothing else", name)
			return v.Done()
		}
	}
	for _, n := range []string{"host_arena", "host_arena_used", "host_arena_say", "host_arena_num",
		"host_arena_exhausted", "HOST_ARENA_BYTES"} {
		if edit.MentionCount(view, n) > 0 {
			v.Die("`%s` is already a name in this file", n)
			return v.Done()
		}
	}
	v.Sayf("THE PARTITION HOLDS: `malloc` %d mentions, `free` %d and `realloc` %d, every one of "+
		"them below the boundary and inside one of the four runs of text this phase "+
		"rewrites -- host_alloc's body, host_free's body, format_overflow_error's free of "+
		"argcopy and adjust_types's realloc of *ap_types.  Nothing else in the file says "+
		"any of the three", total["malloc"], total["free"], total["realloc"])

	// the four rewrites, one synthesized import
	if _, err := e.SpliceC(
		graph.Frag{At: e.SpotBefore(alloc), Src: w52Arena},
		graph.Frag{At: e.SpotBody(alloc), Src: w52AllocBody},
		graph.Frag{At: e.SpotBody(free), Src: w52FreeBody},
		graph.Frag{At: e.SpotOf(argFree), Src: w52FreeArg},
		graph.Frag{At: e.SpotOf(e.Item(realloc)), Src: w52ReallocStmt},
		graph.Frag{At: e.SpotAfter(failTest), Src: w52CopyOld},
	); err != nil {
		v.Die("the arena, the bodies and the two sites -- %v", err)
		return v.Done()
	}

	view = v.Text()
	d2, L2 := w52Directives(view)
	okd := len(d2) == len(incs) && d2[0] == boundary
	for i := range d2 {
		if d2[i] != d2[0]+i {
			okd = false
		}
	}
	for i, inc := range e.Includes() {
		if i >= len(incs) || inc != incs[i] {
			okd = false
		}
	}
	if !okd {
		v.Die("the output does not have the same eleven contiguous `#include` directives at " +
			"the same line -- this phase adds no directive, removes none and moves none")
		return v.Done()
	}
	for _, name := range w52Names {
		if n := edit.MentionCount(view, name); n != 0 {
			v.Die("`%s` still has %d mentions after every class was rewritten", name, n)
			return v.Done()
		}
	}
	coreAfter, err := graph.FormsC(e.Core())
	if err != nil || string(coreAfter) != string(coreBefore) {
		v.Die("the text above the boundary is not what it was: this phase is below it " +
			"entirely, and a difference there is a bug in one of the four replacements")
		return v.Done()
	}
	v.Sayf("`malloc`, `free` and `realloc` are 0 mentions in the whole file, and the %d lines "+
		"above the boundary are BYTE-IDENTICAL to the input's -- the eleven #includes are "+
		"untouched at line %d", d2[0], d2[0]+1)
	v.Sayf("%d -> %d lines of the C view, %d more", linesBefore, len(L2)-1, len(L2)-1-linesBefore)
	return v.Done()
}

// w52Pattern is a pattern this program writes, which reads.
func w52Pattern(s string) *clisp.Node {
	p, err := clisp.Pattern(s)
	if err != nil {
		panic(err)
	}
	return p
}
