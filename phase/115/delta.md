115 DECLARES NOTHING AT ALL, and it is a SIXTH kind -- the code runs, the instrument
sees it run, and it does the same thing.  The wall clock crosses the boundary: the two
standalone `time(nullptr)` calls inside `ui_focus_change()` become `vim_time()`, and
that wrapper then moves below the first `#include` as `host_time()`, declared in the
core's host block beside host_exit and host_message.  `long time(long *tp);` leaves the
core's libc prototype block -- SEVEN entries to six, phase 114 having vendored `abs`
and `labs` out of it just before -- and the host takes `time` from <time.h>.  The block
is found by its shape and not by a count, which is why the phase needed no edit when
that happened.  Two full recordings, of the binary the phase was handed and of its own, are
BYTE-IDENTICAL across all 106 records; `nm -u` is the same seventeen names, `time`
among them, because the host still calls it and a symbol leaves when its last CALLER
leaves the FILE.

THE RECORDING IS THE WEAKEST PART OF THE EVIDENCE AND THE CHECK SAYS SO.  Not one of
the 102 screen cases reaches `ui_focus_change()`, which is the only function this phase
changes the SPELLING of a read in -- so a byte-identical recording says nothing about
it.  What answers is an INSTRUMENTED PAIR: `write(2, "TICK\n", 5)` at every clock read
on both sides -- three sites on the input (the wrapper, and ui_focus_change's two,
which bypass it) and one on the output, because afterwards there is only one -- with
the two instrumented 102-case recordings required to be byte-identical.  They are, and
the instrument is not silent: it marks 100 of 102 cases with 410 reads in all, the two
it misses being `ctrl_c_clean` and `ctrl_c_changed`, which exit before a key is looked
up.

AND ui_focus_change IS PROBED DIRECTLY, because a keystroke file CAN reach it:
`\033[I` and `\033[O` are KE_FOCUSGAINED and KE_FOCUSLOST and `set_termname()`
registers both unconditionally.  `\033[O \033[I` reads the clock 3 times and
`\033[O \033[I \033[O \033[I` reads it 4, identically on both binaries -- 0 + 2 + 0 + 1
at the four calls plus one for the `:q!`, because `in_focus &&` short-circuits on a
FocusLost and the second FocusGained finds `last_time` fresh.  THE TWO READS ARE STILL
TWO READS, in the same two statements, in the same order, so they straddle a second
neither more nor less often than before.  The control is that question made into a
program: `hoist`, the output with the two reads collapsed into one local, reads once
per call and gives 5 where the product gives 4.

WHAT WAS REMOVED THAT WAS LOAD-BEARING, and what replaces it.  `long time(long *tp);`
was not decoration: `typedef long time_T;` is correct only because gcc compared that
prototype with <time.h>'s below it.  It is replaced by
`static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");` beside
the twelve constants phase 110 put below the includes, and the replacement is STRICTLY
STRONGER -- measured in four compiles.  The input with the prototype written
`int time(int *tp);` is `conflicting types for 'time'`; the input with `time_T`
perturbed to `int` and the prototype LEFT ALONE compiles in SILENCE, so the prototype
pinned `long == time_t` and never `time_T == long`; the output with the assert deleted
and `time_T` perturbed compiles in silence too, which is the regression this phase
would otherwise have shipped; and the output WITH the assert gives
`static assertion failed: "time_T is time_t"` for `int` and for `long long` alike.
