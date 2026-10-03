package p049a

// Whim phase 49a (formerly 117) -- the core stops reallocating.
// See GOALS.md II.4c, whose transpilation rule is this phase's justification.
//
// THERE IS NO `musl_realloc` TO VENDOR, AND THAT IS THE WHOLE REASON THIS PHASE IS A
// REWRITE AND NOT A VENDORING.  Phases 36 and 37 gave the core its own definitions of
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
// adjust_types()    THE HOST'S, below the boundary, in the formatter island phase 43
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
// of the nine plain libc prototypes phase 42 wrote below the `usize` typedef and phase 43
// carried above the includes; with no core call left it declares nothing the core uses,
// and `adjust_types()` takes its declaration from <stdlib.h>, which is above it.  Eight
// prototypes remain.  Phases 47 and 49b also shrink this block, and the check computes
// the count from the input rather than stating it, so this phase composes with either.
//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The
// cut is FRAG's two literal runs and a deletion for the header, on the
// program's graph, and its report is the text version's, which the plan
// ran until then (history keeps it):
//
//   - the include checks are the include forms (vimtext.IncludeRun): every
//     one an `#include <...>`, on consecutive forms;
//   - "realloc is 3 in the core and 1 in the host" is asked twice: of the
//     edges -- the core's prototype, its two calls (ga_grow_inner,
//     get_keystroke) and adjust_types()'s, which cc resolves to that same
//     prototype -- and of the C view, `\brealloc\b` above and below the
//     first include line, as the text counted it;
//   - the two rewrites are LiteralC, each run of whole items found once in
//     its function (the text found it once in the file);
//   - the prototype goes by DeleteForHeader: adjust_types()'s call is made
//     again, as an import of the text after makes it, a use of <stdlib.h>'s
//     realloc -- the text's "takes its declaration from <stdlib.h>", now an
//     edge;
//   - the text-only checks (the line counts, the boundary's line moving by
//     the file's, the blank-line runs) are dropped.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim49a", Edit) }

var whim49aRealloc = regexp.MustCompile(`\brealloc\b`)

const whim49aOldGa = `    new_len = (usize)gap->ga_itemsize * (gap->ga_len + n);
    pp = realloc((gap->ga_data), (new_len));
    if (pp == nullptr)
    {
        return FAIL;
    }
    old_len = (usize)gap->ga_itemsize * gap->ga_maxlen;
`

const whim49aNewGa = `    new_len = (usize)gap->ga_itemsize * (gap->ga_len + n);
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

const whim49aZero = "musl_memset((pp + old_len), (0), (new_len - old_len));"

const whim49aOldKs = `            char_u *t_buf = buf;
            buflen += 100;
            buf = realloc((buf), (buflen));
            if (buf == nullptr)
            {
                vim_free(t_buf);
            }
`

const whim49aNewKs = `            char_u *t_buf = buf;
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

// Edit is part 49a on the graph: realloc's two core calls become malloc, a
// copy and a free, and its prototype goes for <stdlib.h>'s.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("realloc", e, w)
	incs, err := vimtext.IncludeRun(e)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	text := v.Text()
	core, host, bound := vimtext.SplitCore(text)
	v.Sayf("%d directives on consecutive lines from %d, every one an `#include <...>`; the "+
		"core is the %d lines above the first of them", len(incs), bound+1, bound)

	// the prototype, and its uses by edge
	var proto *graph.Node
	for _, d := range e.Decls("realloc") {
		if e.InCore(d) && vimtext.IsOrdinaryDecl(d) && proto == nil {
			proto = d
		} else {
			v.Die("`realloc` is declared other than by the core's one prototype: %s", graph.Lisp(d))
			return v.Done()
		}
	}
	if proto == nil {
		v.Die("the core does not declare `realloc`")
		return v.Done()
	}
	var coreFns, hostFns []string
	for _, u := range e.Uses(proto) {
		name := "(outside a function)"
		if f := e.Function(u); f != nil {
			name = graph.DeclName(f)
		}
		if e.InCore(u) {
			coreFns = append(coreFns, name)
		} else {
			hostFns = append(hostFns, name)
		}
	}
	nc, nh := len(whim49aRealloc.FindAll(core, -1)), len(whim49aRealloc.FindAll(host, -1))
	if nc != 3 || nh != 1 || strings.Join(coreFns, " ") != "ga_grow_inner get_keystroke" ||
		strings.Join(hostFns, " ") != "adjust_types" {
		v.Die("`realloc` occurs %d times in the core and %d in the host, its prototype's uses in %s "+
			"and %s, where this phase was written against 3 and 1: the prototype and two calls "+
			"above the boundary (ga_grow_inner, get_keystroke), and adjust_types() below it",
			nc, nh, vimtext.JoinOrNone(coreFns), vimtext.JoinOrNone(hostFns))
		return v.Done()
	}
	v.Say("`realloc` is 3 in the core -- the prototype, ga_grow_inner's call and " +
		"get_keystroke's -- and 1 in the host, adjust_types(), which is NOT this phase's " +
		"and is why the symbol does not leave")

	strs := v.Strings()
	var bad []string
	for _, s := range strs {
		if whim49aRealloc.MatchString(s.Atom) {
			bad = append(bad, s.Atom)
		}
	}
	if len(bad) > 0 {
		v.Die("a literal holds the name `realloc`: %s", strings.Join(bad, " / "))
		return v.Done()
	}
	v.Sayf("%d string literals, none holding `realloc`, so both substitutions "+
		"below are over code", len(strs))

	v.Together(func(v *graph.Verbs) {
		v.InFunction("ga_grow_inner", func(v *graph.Verbs) {
			v.TextCountIs(regexp.QuoteMeta(whim49aZero), 1, "ga_grow_inner's tail-zeroing statement, "+
				"which must survive the rewrite unmoved and BELOW the copy (trap 3)")
			v.LiteralC(whim49aOldGa, whim49aNewGa, 1, "ga_grow_inner's realloc and the two lines either side")
		})
		v.InFunction("get_keystroke", func(v *graph.Verbs) {
			v.LiteralC(whim49aOldKs, whim49aNewKs, 1,
				"get_keystroke's realloc, the `buflen += 100` above it and the `vim_free` below it")
		})
	})
	if v.Failed() {
		return v.Done()
	}
	v.InFunction("ga_grow_inner", func(v *graph.Verbs) {
		t := string(v.Text())
		copyAt := strings.Index(t, "musl_memcpy(pp, gap->ga_data, old_len);")
		if copyAt < 0 || copyAt >= strings.Index(t, whim49aZero) {
			v.Die("the copy did not land above the tail-zeroing statement")
		}
	})
	if v.Failed() {
		return v.Done()
	}
	v.Say("ga_grow_inner: `realloc(ga_data, new_len)` -> `malloc(new_len)` with the copy and " +
		"the free GUARDED by `ga_data != nullptr`, and `old_len` hoisted above the " +
		"allocation -- it is the old size and the function already computed it.  The " +
		"`return FAIL` still comes before anything is freed, so a failed grow leaves the " +
		"old block valid and ga_data untouched; the tail-zeroing statement is unmoved and " +
		"below the copy")

	v.Say("get_keystroke: `realloc(buf, buflen)` -> `malloc(buflen)` with the old size saved " +
		"as `t_buflen` beside the old pointer `t_buf`, one line above the `buflen += 100`.  " +
		"The failure path is untouched -- `vim_free(t_buf)`, which is what this site always " +
		"did for itself -- and the success path frees with `free()`, not `vim_free()`, " +
		"because vim_free declines while really_exiting and realloc did not")

	rebound, _, err := e.DeleteForHeader([]*graph.Node{proto}, false, nil)
	if err != nil {
		v.Die("%v", err)
		return v.Done()
	}
	if len(rebound) != 1 {
		v.Die("%d uses of `realloc` went to the header's declaration, where adjust_types()'s one was expected",
			len(rebound))
		return v.Done()
	}
	v.Say("`void *realloc(void *p, usize n);` is gone from the core's libc prototype block.  " +
		"adjust_types() takes its declaration from <stdlib.h>, which is above it")

	if n := len(e.Includes()); n != len(incs) {
		v.Die("the file has %d include forms and had %d: this phase adds none and removes none", n, len(incs))
		return v.Done()
	}
	core, host, _ = vimtext.SplitCore(v.Text())
	if k := len(whim49aRealloc.FindAll(core, -1)); k > 0 {
		v.Die("`realloc` still occurs %d times in the core, and the phase's whole product is that it is 0", k)
		return v.Done()
	}
	if k := len(whim49aRealloc.FindAll(host, -1)); k != nh {
		v.Die("the host's %d `realloc` did not survive (%d now): adjust_types() is the host's and no "+
			"phase of this pipeline has taken it", nh, k)
		return v.Done()
	}
	v.Sayf("the core does not name `realloc` at all: 3 -> 0 above the boundary, 1 -> 1 below "+
		"it, the %d include forms unmoved", len(incs))
	return v.Done()
}
