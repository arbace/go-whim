117 DECLARES NOTHING AT ALL, AND IT IS THE ONE KIND THIS PIPELINE HAS RARELY HAD: the
code runs, it runs constantly, and the recording is byte-identical because the rewrite
IS the same function.  It is not phase 92's kind (code that could not run), not 95's or
96's or 113's (code the instrument cannot see), not 99's or 106's (a byte-identical
binary) and not 112's (different answers no record holds).  ga_grow_inner() is on the
path of every growarray in the editor and a recording drives it 4,289 times in 104 of
the 106 records -- so "two recordings are byte-identical" is STRONG evidence here, and
the control says how strong: the same rewrite with the copy length set to 0 moves 102
of the 102 screen cases.

WHAT IT DOES.  `realloc` had three call sites and two of them were the core's --
`ga_grow_inner()` and `get_keystroke()`.  Both now say `malloc`, a `musl_memcpy` of the
OLD size, and `free`, and `void *realloc(void *p, usize n);` leaves the core's libc
prototype block.  The third site, `adjust_types()`, is below the first `#include` and
is the HOST's, so `realloc` STAYS in `nm -u` -- 17 names either side, the same set --
which is an equality the phase declares rather than a symbol it frees, exactly as
phase 111 did for `gettimeofday` and phase 113 for the four stdio names.

WHY THE REWRITE AND NOT A VENDORING.  `realloc` CANNOT BE IMPLEMENTED from `malloc` and
`free`: to move the old contents it needs the old size, and its interface does not
carry one -- musl reads it back out of the chunk header below the pointer, which is a
fact about musl's heap and not about C.  So there is no `musl_realloc` to write beside
phase 97's sixteen, and the only route is the call sites, each with the size IT knows.
Both do: ga_grow_inner already computes `ga_itemsize * ga_maxlen` on the next line, to
zero the tail, and get_keystroke's old size is `buflen` before the `+= 100` that sits
immediately above the call.

THE FOUR TRAPS ARE MEMORY BUGS AND NOT DIFFERENCES, which is why the phase owes a unit
harness and not probes.  A recording cannot see a leak, a double free, or an overread
whose bytes are overwritten before anything reads them.  phase/117/check.sh extracts
both rewritten sites from the input source and from the output, runs them through one
AddressSanitizer driver -- eight doublings from empty, six independent first grows, a
failed allocation, and the 100-byte extension -- and requires the two transcripts to be
identical with no finding.  Six of seven controls move, each with its own named
finding, and the seventh is the interesting one: dropping the `gap->ga_data != nullptr`
guard changes NOTHING measurable, because a null ga_data implies ga_maxlen == 0 implies
a copy of length zero and musl_memcpy is a plain `for (; n; n--)` loop.  The guard is
kept for what GOALS.md II.4c asks of the core -- that its meaning be on the page -- and
the check proves it with a driver whose musl_memcpy announces a null source: 0 from the
output, 8 from the unguarded control.

AND get_keystroke's EXTENSION IS UNREACHABLE IN A RECORDING, measured: an instrumented
build of the input marks each of the five `continue` paths inside its loop at 0 of 106
records, so `len` never exceeds one ui_inchar() and `maxlen` never falls below 10.  A
pty session feeding a partial escape sequence sixty times does not reach it either.
The unit harness is the only instrument that can drive it, and it drives both versions.
