package p117

// Whim phase 117 -- the core stops reallocating.
// See GOALS.md II.4c, whose transpilation rule is this phase's justification.
//
// THERE IS NO `musl_realloc` TO VENDOR, AND THAT IS THE WHOLE REASON THIS PHASE IS A
// REWRITE AND NOT A VENDORING.  Phases 97 and 98 gave the core its own definitions of
// sixteen mem*/str* functions and of qsort/bsearch, because each of those is a function
// of its arguments.  `realloc` is NOT: to move the old contents it must know how many
// bytes the old block held, and its interface -- `void *realloc(void *p, usize n)` --
// does not carry that number.  musl reads it back out of the chunk header two words
// below `p`, which is a fact about musl's heap and not about C.  So a core-owned
// `musl_realloc(void *p, usize n)` written over `malloc`, a copy and `free` CANNOT BE
// WRITTEN AT ALL: there is nothing to give the copy for a length.
//
// The only route left is to rewrite each call site with the size IT knows, and the
// phase exists because both core sites know it.  GOALS.md II.4c's rule -- the core is
// optimised for transpilation, so its meaning must be on the page -- is what makes that
// worth doing: on a JVM there is no `realloc`, and `malloc` + a copy + `free` is an
// array allocation and an arraycopy, which is exactly what these two sites now say.
//
// THREE CALL SITES, AND ONLY TWO OF THEM ARE THE CORE'S.  Every count is re-measured
// from the input here, `grep -ow realloc` above and below the first `#include`:
//
// ga_grow_inner()   the core's.  Its old size is on the NEXT LINE already --
// `old_len = (usize)gap->ga_itemsize * gap->ga_maxlen;`, which the
// function computes to zero the tail.  That IS the old allocation.
// get_keystroke()   the core's.  Its old size is `buflen` BEFORE the `buflen += 100;`
// immediately above the call, and the rewrite saves it in `t_buflen`
// beside the `t_buf` the input already saves, rather than writing
// `buflen - 100` and asking a reader to do the arithmetic.
// adjust_types()    THE HOST'S, below the boundary, in the formatter island phase 110
// moved down.  It is left exactly as it is, and `realloc` therefore
// STAYS in `nm -u` -- an equality this phase declares rather than a
// symbol it frees.  See the check.
//
// THE FOUR TRAPS, each of which would be a silent memory bug and not a failure.
//
// 1  `realloc(nullptr, n)` IS `malloc(n)`.  `ga_grow_inner` is called with
// `gap->ga_data == nullptr` on a growarray's first grow, and MEASURED on a full
// recording of the input that is 2,739 of its 4,289 calls -- the majority case,
// not an edge.  The rewrite must not copy from null and must not free null, so the
// copy and the free are inside `if (gap->ga_data != nullptr)`.  The guard is what
// makes the rewrite FAITHFUL rather than merely similar, and the check measures
// exactly how much it is worth: on THIS implementation, nothing -- see the check's
// `c_noguard`, which is reported rather than hidden.
//
// 2  ON FAILURE `realloc` LEAVES THE OLD BLOCK VALID AND ALLOCATED.  Both sites rely
// on it.  `ga_grow_inner` returns FAIL with `ga_data` untouched; `get_keystroke`
// frees the old block itself, with `vim_free`.  So in `ga_grow_inner` the `return
// FAIL` comes BEFORE anything is freed, and in `get_keystroke` the free of the old
// block moves into an `else` and the existing `vim_free(t_buf)` stays exactly where
// it was.  The success-path free is `free()` and NOT `vim_free()`: `vim_free` does
// nothing while `really_exiting`, and `realloc` frees regardless.
//
// 3  `ga_grow_inner` ZEROES `new_len - old_len` BYTES AFTER THE COPY.  That statement
// is not moved and not rewritten; the copy is inserted above it, so the order is
// copy-then-zero as it was realloc-then-zero.
//
// 4  SHRINKING.  Neither site can ask for less than it has, and it is MEASURED, not
// assumed.  `ga_grow_inner`'s only caller is `ga_grow`, which calls it only when
// `gap->ga_maxlen - gap->ga_len < n`, and the three statements above the allocation
// only raise `n` -- so `new_len > old_len` always.  The input's own
// `musl_memset(pp + old_len, 0, new_len - old_len)` already asserts it, `new_len -
// old_len` being unsigned.  An instrumented build of the input marks a shrink at 0
// of 106 records.  `get_keystroke` does `buflen += 100` immediately above, so the
// new size exceeds the old by exactly 100.  `musl_memcpy` therefore copies the old
// size at both sites and no minimum is needed -- which the check states as the
// reason rather than as an omission.
//
// THE PROTOTYPE GOES WITH THE LAST CORE CALL.  `void *realloc(void *p, usize n);` is one
// of the nine plain libc prototypes phase 109 wrote below the `usize` typedef and phase 110
// carried above the includes; with no core call left it declares nothing the core uses,
// and `adjust_types()` takes its declaration from <stdlib.h>, which is above it.  Eight
// prototypes remain.  Phases 114 and 118 also shrink this block, and the check computes
// the count from the input rather than stating it, so this phase composes with either.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim117", Edit) }

var (
	whim117Directive = regexp.MustCompile(`^ *#`)
	whim117Inc       = regexp.MustCompile(`^ *# *include <([A-Za-z0-9_/.]+)>$`)
	// `\brealloc\b` does NOT match inside `realloc_cmdbuff` -- `_` is a word
	// character -- so this counts the libc name alone, which is what the
	// phase is about.
	whim117Realloc = regexp.MustCompile(`\brealloc\b`)
)

// ga_grow_inner: the old allocation is `ga_itemsize * ga_maxlen`, which the
// input computes to zero the tail.  It is hoisted ABOVE the allocation so the
// copy can use it, and the zeroing statement is left exactly as it was, BELOW
// the copy.
//
// THE GUARD IS TRAP 1.  On a first grow `ga_data` is null and `ga_maxlen` is
// 0; `realloc(nullptr, n)` is `malloc(n)` and frees nothing, so the rewrite
// must neither copy from null nor free null.
//
// THE `return FAIL` IS TRAP 2, and its position is the whole of it: nothing is
// freed above it, so a failed allocation leaves `gap->ga_data` pointing at a
// block that is still valid and still allocated, which is what `realloc`
// guaranteed and what the caller relies on.
const whim117OldGa = `    new_len = (usize)gap->ga_itemsize * (gap->ga_len + n);
    pp = realloc((gap->ga_data), (new_len));
    if (pp == nullptr)
    {
        return FAIL;
    }
    old_len = (usize)gap->ga_itemsize * gap->ga_maxlen;
`

const whim117NewGa = `    new_len = (usize)gap->ga_itemsize * (gap->ga_len + n);
    old_len = (usize)gap->ga_itemsize * gap->ga_maxlen;
    pp = malloc(new_len);
    if (pp == nullptr)
    {
        return FAIL;
    }
    if (gap->ga_data != nullptr)
    {
        musl_memcpy(pp, gap->ga_data, old_len);
        free(gap->ga_data);
    }
`

const whim117Zero = "    musl_memset((pp + old_len), (0), (new_len - old_len));\n"

// get_keystroke: `t_buflen` is declared beside the `t_buf` the input already
// keeps, so the two halves of what `realloc`'s interface does not carry -- the
// old pointer and the old size -- are saved together and on adjacent lines.
// `buflen - 100` would be the same number and a worse text.
//
// TRAP 2 AGAIN, and the other shape of it: here the caller frees the old block
// ITSELF on failure, so `vim_free(t_buf)` stays exactly where it is and the
// success-path free goes in an `else`.  It is `free()` and not `vim_free()`
// because `vim_free` declines while `really_exiting` and `realloc` freed
// regardless.
const whim117OldKs = `            char_u *t_buf = buf;
            buflen += 100;
            buf = realloc((buf), (buflen));
            if (buf == nullptr)
            {
                vim_free(t_buf);
            }
`

const whim117NewKs = `            char_u *t_buf = buf;
            int t_buflen = buflen;
            buflen += 100;
            buf = malloc(buflen);
            if (buf == nullptr)
            {
                vim_free(t_buf);
            }
            else
            {
                musl_memcpy(buf, t_buf, t_buflen);
                free(t_buf);
            }
`

const whim117Proto = "void *realloc(void *p, usize n);\n"

// Whim117 stops the core reallocating: `realloc` is rewritten at its two core
// sites as an allocation, a copy of the OLD size and a free.
//
// It is the one libc function that cannot be vendored at all -- to move the
// old contents it needs a length its interface does not carry -- so the route
// is each call site supplying the length it already knows.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "realloc", W: w}
	lines := bytes.Split(text, []byte{'\n'})
	linesBefore := len(lines)

	// ---- 0. the file this edit was written against -----------------------
	// The boundary is the first `#include` and nothing else marks it.  The
	// core is everything above; the host is everything from there down, and
	// the third `realloc` call is the host's and is not this phase's.
	var d []int
	for i, l := range lines {
		if whim117Directive.Match(l) {
			d = append(d, i)
		}
	}
	if len(d) == 0 {
		return nil, p.Die("the file has no preprocessor directive, so there is no boundary and no way to " +
			"tell a core call from a host one")
	}
	for k, i := range d {
		if i != d[0]+k {
			return nil, p.Die("the %d directives are not on consecutive lines, so the first `#include` is not "+
				"a boundary", len(d))
		}
	}
	for _, i := range d {
		if !whim117Inc.Match(lines[i]) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header")
		}
	}
	bound := d[0]
	core := bytes.Join(lines[:bound], []byte{'\n'})
	host := bytes.Join(lines[bound:], []byte{'\n'})
	p.Sayf("%d directives on consecutive lines from %d, every one an `#include <...>`; the "+
		"core is the %d lines above the first of them", len(d), bound+1, bound)

	// ---- 1. the inventory, as a partition and not a list -----------------
	nc := len(whim117Realloc.FindAll(core, -1))
	nh := len(whim117Realloc.FindAll(host, -1))
	if nc != 3 || nh != 1 {
		return nil, p.Die("`realloc` occurs %d times in the core and %d in the host, where this phase was "+
			"written against 3 and 1: the prototype and two calls above the boundary, and "+
			"adjust_types() below it", nc, nh)
	}
	p.Say("`realloc` is 3 in the core -- the prototype, ga_grow_inner's call and " +
		"get_keystroke's -- and 1 in the host, adjust_types(), which is NOT this phase's " +
		"and is why the symbol does not leave")

	// NO LITERAL MAY HOLD THE NAME.  Phase 106 was caught Out by three string
	// literals holding `NULL`; the lesson is applied rather than assumed.
	spans, err := edit.LiteralSpansShort(p, text)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, s := range spans {
		if whim117Realloc.Match(text[s[0]:s[1]]) {
			bad = append(bad, string(text[s[0]:s[1]]))
		}
	}
	if len(bad) > 0 {
		return nil, p.Die("a literal holds the name `realloc`: %s", strings.Join(bad, " / "))
	}
	p.Sayf("%d string and character literals, none holding `realloc`, so both substitutions "+
		"below are over code", len(spans))

	// ---- 2. ga_grow_inner -- the size is already on the next line --------
	if bytes.Count(text, []byte(whim117OldGa)) != 1 {
		return nil, p.Die("ga_grow_inner's realloc and the two lines either side are not in the file " +
			"exactly once, so this phase cannot tell what the old size is")
	}
	if bytes.Count(text, []byte(whim117Zero)) != 1 {
		return nil, p.Die("ga_grow_inner's tail-zeroing statement is not in the file exactly once, and " +
			"trap 3 is that it must survive the rewrite unmoved and BELOW the copy")
	}
	text = bytes.Replace(text, []byte(whim117OldGa), []byte(whim117NewGa), 1)
	if bytes.Index(text, []byte(whim117NewGa)) >= bytes.Index(text, []byte(whim117Zero)) {
		return nil, p.Die("the copy did not land above the tail-zeroing statement")
	}
	p.Say("ga_grow_inner: `realloc(ga_data, new_len)` -> `malloc(new_len)` with the copy and " +
		"the free GUARDED by `ga_data != nullptr`, and `old_len` hoisted above the " +
		"allocation -- it is the old size and the function already computed it.  The " +
		"`return FAIL` still comes before anything is freed, so a failed grow leaves the " +
		"old block valid and ga_data untouched; the tail-zeroing statement is unmoved and " +
		"below the copy")

	// ---- 3. get_keystroke -- the size is `buflen` before the `+= 100` ----
	if bytes.Count(text, []byte(whim117OldKs)) != 1 {
		return nil, p.Die("get_keystroke's realloc, the `buflen += 100` above it and the `vim_free` " +
			"below it are not in the file exactly once")
	}
	text = bytes.Replace(text, []byte(whim117OldKs), []byte(whim117NewKs), 1)
	p.Say("get_keystroke: `realloc(buf, buflen)` -> `malloc(buflen)` with the old size saved " +
		"as `t_buflen` beside the old pointer `t_buf`, one line above the `buflen += 100`.  " +
		"The failure path is untouched -- `vim_free(t_buf)`, which is what this site always " +
		"did for itself -- and the success path frees with `free()`, not `vim_free()`, " +
		"because vim_free declines while really_exiting and realloc did not")

	// ---- 4. the prototype, which now declares nothing the core uses ------
	if bytes.Count(text, []byte(whim117Proto)) != 1 {
		return nil, p.Die("`%s` is not in the file exactly once -- it is one of the plain libc prototypes "+
			"phase 109 wrote and phase 110 carried above the includes", strings.TrimSpace(whim117Proto))
	}
	text = bytes.Replace(text, []byte(whim117Proto), nil, 1)
	p.Say("`void *realloc(void *p, usize n);` is gone from the core's libc prototype block.  " +
		"adjust_types() takes its declaration from <stdlib.h>, which is above it")

	// ---- 5. what the file is now -----------------------------------------
	L := bytes.Split(text, []byte{'\n'})
	const added = 5 + 6 - 1
	if len(L) != linesBefore+added {
		return nil, p.Die("the file is %d lines and the input was %d -- this phase adds exactly %d: 5 at "+
			"ga_grow_inner, 6 at get_keystroke and -1 for the prototype",
			len(L)-1, linesBefore-1, added)
	}
	var nd []int
	for i, l := range L {
		if whim117Directive.Match(l) {
			nd = append(nd, i)
		}
	}
	if len(nd) != len(d) {
		return nil, p.Die("the file has %d directives and had %d: this phase adds none and removes none",
			len(nd), len(d))
	}
	if nd[0]-d[0] != len(L)-linesBefore {
		return nil, p.Die("the boundary moved by %d lines and the file by %d: every line this phase "+
			"touches is above the first `#include`", nd[0]-d[0], len(L)-linesBefore)
	}
	ncore := bytes.Join(L[:nd[0]], []byte{'\n'})
	nhost := bytes.Join(L[nd[0]:], []byte{'\n'})
	if whim117Realloc.Match(ncore) {
		return nil, p.Die("`realloc` still occurs %d times in the core, and the phase's whole product is "+
			"that it is 0", len(whim117Realloc.FindAll(ncore, -1)))
	}
	if len(whim117Realloc.FindAll(nhost, -1)) != nh {
		return nil, p.Die("the host's %d `realloc` did not survive: adjust_types() is the host's and no "+
			"phase of this pipeline has taken it", nh)
	}
	// A cut between two blank lines leaves two in a row now that the canonical
	// print separates every declaration, and the print at the end of the phase
	// collapses them; the blank runs are layout, and not this phase's to count.
	p.Sayf("the core does not name `realloc` at all: 3 -> 0 above the boundary, 1 -> 1 below "+
		"it, %d directives unmoved relative to the text, and the blank-line runs unchanged "+
		"at %d", len(nd), p.BlankRuns(text))
	return text, nil
}
