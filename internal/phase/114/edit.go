package p114

// Whim phase 114 -- abs and labs, the two the core took on trust.
// See GOALS.md II.4c, and GOALS.md.
//
// THE CORE IS OPTIMISED FOR TRANSPILATION, NOT FOR PERFORMANCE, AND SO IT MAY NOT
// DEPEND ON LATENT COMPILER BEHAVIOUR (GOALS.md II.4c, the user's rule of
// 2026-09-19).  This phase is the first application of it, and it is the reason the
// phase exists at all -- because by every number this pipeline usually reports, it
// does nothing.
//
// `abs` and `labs` are CALLED by the core, at three sites, and appear in `nm -u`
// ZERO times.  gcc lowers both to inline arithmetic -- measured on the input, `gcc
// -S` of the whole file contains not one mention of either name -- and NOTHING IN
// THE LANGUAGE PROMISES THAT.  A compiler that emitted the calls the source
// literally asks for would silently have added two libc symbols to a file whose
// whole claim is the shortness of that list.  So this phase FREES NO SYMBOL and must
// say so as an equality rather than let a reader expect a vendoring phase to move
// the count: what it removes is a dependence on behaviour nothing states.
//
// WHAT IT DOES, in three parts:
//
// the two prototypes    `long labs(long n);` and `int abs(int n);`, which
// phase 109 wrote into the core's block of libc declarations
// when the headers were still above it.  The block loses two
// of its entries and nothing else changes in it.
// the three call sites  8152 `musl_labs((long)get_cursor_rel_lnum(...))` in the
// number column, 41824 `musl_labs(curwin->w_topline -
// prev_topline)` in scroll_with_sms, and 77379
// `musl_abs(wp->w_height - wp->w_prev_height)` in
// last_status_rec.  The line numbers are where they were
// when this was written and nothing below depends on them.
// the two definitions   `static musl_abs` and `static musl_labs`, in the `musl_`
// block phases 97 and 98 built, immediately above
// `musl_bsearch` so that the four `<stdlib.h>` scalar
// functions the core owns -- musl_atoi, musl_atol, musl_abs,
// musl_labs -- sit together and above every use.
//
// MUSL'S SPELLING IS COPIED AND NOT IMPROVED, which is phase 97's rule applied to
// two more functions.  /root/musl/src/stdlib/abs.c and labs.c are one line each:
//
// int abs(int a) { return a>0 ? a : -a; }
// long labs(long a) { return a>0 ? a : -a; }
//
// `a > 0 ? a : -a` and `a < 0 ? -a : a` ARE THE SAME FUNCTION, and the check proves
// it twice rather than arguing it: the two spellings compile to BYTE-IDENTICAL
// machine code at -O0 and at -O2, and they agree at every one of the 4,294,967,296
// `int` values.  Having established that, the rule is to write what musl writes.
//
// AND THE UNDEFINED BEHAVIOUR IS COPIED WITH IT.  `-a` overflows at `INT_MIN` and at
// `LONG_MIN`, so `musl_abs(INT_MIN)` is undefined -- and so is `abs(INT_MIN)`, and so
// is musl's own `abs`, by exactly the same expression.  THE VENDORED PAIR IS
// FAITHFUL RATHER THAN SAFER, deliberately: a phase that quietly made the core's
// arithmetic differ from the libc it is replacing would be a behaviour change
// wearing a vendoring phase's clothes.  What the check does instead is measure
// whether the three call sites can reach the value, and the answer is no in one case
// by a clamp in this file and in the other two by the arithmetic of line numbers.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and the check needs it for
// more than a comparison: the corpus CANNOT SEE any of the three call sites (measured
// -- an instrumented build enters none of them in 106 records), so this phase owes
// probes of its own, and those probes are run on the binary this phase was handed and
// on its own and required to draw the same screen.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"github.com/arbace/go-whim/crefactor/xform"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// The rule is general, and it is crefactor/xform's Own, built with vim's
// knobs (internal/whim/xform.go): the two bodies are musl's.  Its call counts
// are arguments in the plan.
func init() { phase.RegisterArgs("whim114", xform.Own(whim.Own114).Edit()) }
