package p124

// Whim phase 124 -- freeing is free.  GOALS.md's charter bullet, "A GARBAGE
// COLLECTOR IS ASSUMED FROM HERE ON".
//
// `host_alloc` BECOMES A BUMP ALLOCATOR AND `host_free` RETURNS WITHOUT DOING ANYTHING.
// Phase 118 moved `malloc` and `free` across the boundary and wrote the two wrappers that
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
// THE ARENA COULD NOT HAVE BEEN SIZED BEFORE PHASE 123, and that is worth saying
// plainly rather than leaving in the manifest.  The heaviest of the 102 screen cases asks
// for 1,722,512 bytes and the heaviest of phase 123's 16 memline cases asks for 115 TIMES
// MORE.  A phase written one boundary earlier would have measured the 102, found 1.7 MB,
// and sized an arena from a corpus that provably cannot reach the text layer at all --
// which is the defect phase 123 exists to have ended, arriving one phase later in a shape
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
// 3  format_overflow_error()'s `free(argcopy)`     BELOW the boundary, and phase 118
// 4  adjust_types()'s `realloc(*ap_types, ...)`    left both deliberately.
//
// PARTS 3 AND 4 ARE WHAT MAKES THIS PHASE CORRECT RATHER THAN NEARLY CORRECT, and
// neither is in the core.  The formatter's private island -- the four functions phase 110
// moved BELOW the includes because they need `va_list` -- still called libc's `free` and
// libc's `realloc` directly, on pointers that came from `alloc_clear()`, which is to say
// from `host_alloc`.  Phase 118 saw them and left them, naming `free`'s one below-boundary
// mention as "format_overflow_error() below the boundary", and phase 117 saw the other and
// said in as many words that the remaining `realloc` "is the host's and is not this
// phase's".  Both were right while `host_alloc` WAS `malloc`: the two allocators were one
// allocator.  This phase is where they stop being one, and a `free()` or a `realloc()` of
// an arena pointer is undefined behaviour from the line below.  So they move here, and
// the realloc moves by phase 117's own rewrite -- allocate, copy, free -- with phase 117's
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
// nullptr -- so it is phase 92's and phase 100's kind, code no build of whim-vim can run.
// `adjust_types()` needs a positional format spec and no string literal in the file holds
// one, so it takes a runtime format to reach; the check reaches it with one.
//
// WHAT THIS PHASE DOES NOT DO, AND IT IS MEASURED RATHER THAN OVERLOOKED.  `malloc`,
// `free` and `realloc` were the only users of `<stdlib.h>`, and with them gone the
// directive is dead: the check builds the output without it and the binary is
// BYTE-IDENTICAL.  It stays.  GOALS.md permits a phase to remove a directive and
// phase 96 is the precedent for declining -- it measured that removing three of them was
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

import (
	"fmt"
	"github.com/arbace/go-whim/internal/cutil"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.RegisterArgs("whim124", Edit) }

// w124Names are the three libc allocators.  `\b` does not match inside
// `host_free`, `vim_free` or `realloc_cmdbuff` -- `_` is a word character --
// so these are the bare libc names.
var w124Names = []string{"malloc", "free", "realloc"}

var (
	w124Dir = regexp.MustCompile(`^ *# *`)
	w124Inc = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)
)

// Whim124 makes freeing free: host_alloc() becomes a bump allocator into a 1 GiB
// arena and host_free() a function that returns.
//
// IT TOUCHES NO CORE LINE, and that is the phase's central claim: the text above
// the first `#include` must be BYTE-IDENTICAL in and Out.
func Edit(text []byte, w io.Writer, args []string) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 167 drops the unused)
	p := edit.Ph{Tag: "arena", W: w}
	if len(args) != 1 {
		return nil, p.Die("usage: edit whim124 <file> <state-dir>")
	}
	state := args[0]
	t := string(text)

	swap := func(old, new, what, why string) error {
		if c := cutil.CountAnchor(t, old); c != 1 {
			return p.Die("%s occurs %d times, expected 1 -- %s", what, c, why)
		}
		t = cutil.ReplaceAnchor(t, old, new, 1)
		return nil
	}

	L := strings.Split(t, "\n")
	linesBefore := len(L) - 1

	// ---- 0. the boundary, and the file this edit was written against ---------
	var directives []int
	for i, l := range L {
		if w124Dir.MatchString(l) {
			directives = append(directives, i)
		}
	}
	if len(directives) != nInc {
		return nil, p.Die("the file holds %d preprocessor directives and this phase was written against "+
			"the eleven `#include`s phase 104 left", len(directives))
	}
	for i := range directives {
		if directives[i] != directives[0]+i {
			return nil, p.Die("the eleven directives are not eleven consecutive lines")
		}
	}
	for _, i := range directives {
		if !w124Inc.MatchString(L[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	boundary := directives[0]
	coreBefore := strings.Join(L[:boundary], "\n")
	p.Sayf("the boundary is line %d, the first of the eleven `#include`s, and there is not a "+
		"directive above it", boundary+1)

	// ---- 1. no literal holds any of the three names --------------------------
	spans, err := edit.LiteralSpansShort(p, text)
	if err != nil {
		return nil, err
	}
	for _, name := range w124Names {
		re := regexp.MustCompile(`\b` + name + `\b`)
		var bad []string
		for _, s := range spans {
			if re.MatchString(t[s[0]:s[1]]) {
				bad = append(bad, t[s[0]:s[1]])
			}
		}
		if len(bad) > 0 {
			return nil, p.Die("a literal holds the name `%s`: %s", name,
				strings.Join(edit.First(bad, 3), " / "))
		}
	}
	p.Sayf("%d string and character literals, and not one of them holds `malloc`, `free` or "+
		"`realloc`, so every mention the partition below classifies is code", len(spans))

	// ---- 2. THE PARTITION ----------------------------------------------------
	// A PARTITION AND NOT A COUNT.  How many mentions there are is read off the
	// text and never written down; what is written down is that every one falls
	// in a class this phase rewrites, and that a mention in no class REFUSES.
	classes := []struct{ What, Text string }{
		{"host_alloc()'s body", w124AllocOld},
		{"host_free()'s body", w124FreeOld},
		{"format_overflow_error()'s free of argcopy", w124OverflowOld},
		{"adjust_types()'s realloc of *ap_types, with the failure test below it", w124ReallocOld},
	}
	var covered [][2]int
	for _, c := range classes {
		if k := strings.Count(t, c.Text); k != 1 {
			return nil, p.Die("%s is in the file %d times and must be there exactly once, so this phase "+
				"has not been handed the file it was written for", c.What, k)
		}
		a := strings.Index(t, c.Text)
		covered = append(covered, [2]int{a, a + len(c.Text)})
	}
	total := map[string]int{}
	var stray []string
	for _, name := range w124Names {
		n := 0
		for _, m := range regexp.MustCompile(`\b`+name+`\b`).FindAllStringIndex(t, -1) {
			in := false
			for _, c := range covered {
				if c[0] <= m[0] && m[1] <= c[1] {
					in = true
					break
				}
			}
			if in {
				n++
			} else {
				stray = append(stray, fmt.Sprintf("%s:%d", name, strings.Count(t[:m[0]], "\n")+1))
			}
		}
		total[name] = n
	}
	if len(stray) > 0 {
		s := "s"
		if len(stray) == 1 {
			s = ""
		}
		return nil, p.Die("%d mention%s of one of the three is in none of the four classes this phase "+
			"rewrites: %s -- this phase will not leave a libc allocator call behind on a "+
			"pointer the arena handed out", len(stray), s, strings.Join(edit.First(stray, 6), " "))
	}
	// AND ALL OF THEM ARE BELOW THE BOUNDARY, which is the host-only claim stated
	// as a place before it is stated as a `cmp`.
	for _, name := range w124Names {
		if regexp.MustCompile(`\b` + name + `\b`).MatchString(coreBefore) {
			return nil, p.Die("`%s` is mentioned above the boundary, and phases 117 and 118 took the core's "+
				"last one -- this phase changes the host and nothing else", name)
		}
	}
	for _, n := range []string{"host_arena", "host_arena_used", "host_arena_say", "host_arena_num",
		"host_arena_exhausted", "HOST_ARENA_BYTES"} {
		if regexp.MustCompile(`\b` + n + `\b`).MatchString(t) {
			return nil, p.Die("`%s` is already a name in this file", n)
		}
	}
	p.Sayf("THE PARTITION HOLDS: `malloc` %d mentions, `free` %d and `realloc` %d, every one of "+
		"them below the boundary and inside one of the four runs of text this phase "+
		"rewrites -- host_alloc's body, host_free's body, format_overflow_error's free of "+
		"argcopy and adjust_types's realloc of *ap_types.  Nothing else in the file says "+
		"any of the three", total["malloc"], total["free"], total["realloc"])

	// ---- 3 to 6. the four replacements ---------------------------------------
	for _, e := range []struct{ Old, New, What, why string }{
		{w124AllocOld, w124AllocNew, "host_alloc()'s definition",
			"the arena, its offset, the two message helpers and the abort go where the one " +
				"call of malloc() was, so the allocator and its data stay one paragraph of the " +
				"launcher region"},
		{w124FreeOld, w124FreeNew, "host_free()'s definition",
			"this is the whole of \"freeing is now free\": the parameter is named so the " +
				"signature does not move and discarded so the build is silent"},
		{w124OverflowOld, w124lit2, "format_overflow_error()'s free of argcopy",
			"argcopy came from alloc_clear(), which is host_alloc(), and from the line above " +
				"this one libc's free() of that pointer is undefined.  It is the same rename " +
				"phase 118 made at every core site, made at the one site below the boundary"},
		{w124ReallocOld, w124ReallocNew, "adjust_types()'s realloc of *ap_types",
			"phase 117's rewrite at the one site phase 117 left: allocate, copy what was there, " +
				"free the old block.  The old size is `*num_posarg` entries, which the function " +
				"already has, and the guard is the same `*ap_types != nullptr` the arm above tests"},
	} {
		if err := swap(e.Old, e.New, e.What, e.why); err != nil {
			return nil, err
		}
	}

	// ---- 7. what the file is now ---------------------------------------------
	L = strings.Split(t, "\n")
	var d []int
	for i, l := range L {
		if w124Dir.MatchString(l) {
			d = append(d, i)
		}
	}
	okd := len(d) == nInc && d[0] == boundary
	for i := range d {
		if d[i] != d[0]+i {
			okd = false
		}
	}
	if !okd {
		return nil, p.Die("the output does not have the same eleven contiguous `#include` directives at " +
			"the same line -- this phase adds no directive, removes none and moves none")
	}
	for _, name := range w124Names {
		if n := len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(t, -1)); n != 0 {
			return nil, p.Die("`%s` still has %d mentions after every class was rewritten", name, n)
		}
	}
	// THE CORE IS BYTE-IDENTICAL, which is this phase's central claim and is
	// asserted here before the check states it as `make editor.c`'s own `cmp`.
	if strings.Join(L[:d[0]], "\n") != coreBefore {
		return nil, p.Die("the text above the boundary is not what it was: this phase is below it " +
			"entirely, and a difference there is a bug in one of the four replacements")
	}
	p.Sayf("`malloc`, `free` and `realloc` are 0 mentions in the whole file, and the %d lines "+
		"above the boundary are BYTE-IDENTICAL to the input's -- the eleven #includes are "+
		"untouched at line %d", d[0], d[0]+1)

	// The arithmetic, computed rather than written: the four replacements' own
	// line counts.
	grew := 0
	for _, e := range [][2]string{
		{w124AllocOld, w124AllocNew}, {w124FreeOld, w124FreeNew},
		{w124OverflowOld, w124lit2}, {w124ReallocOld, w124ReallocNew},
	} {
		grew += len(strings.Split(e[1], "\n")) - len(strings.Split(e[0], "\n"))
	}
	if len(L)-1 != linesBefore+grew {
		return nil, p.Die("the file is %d lines and the input was %d -- the four replacements are %d lines "+
			"more between them", len(L)-1, linesBefore, grew)
	}
	p.Sayf("%d -> %d lines, %d more, which is exactly what the four replacements are worth; no "+
		"run of two blank lines", linesBefore, len(L)-1, grew)

	if err := os.WriteFile(state+"/arena-bytes",
		[]byte(fmt.Sprintf("%d\n", 1024*1024*1024)), 0o644); err != nil {
		return nil, p.Die("%v", err)
	}
	return []byte(t), nil
}
