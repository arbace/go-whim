124 DECLARES NOTHING AT ALL, and it is the SIXTH kind -- the code runs, the instrument
sees it, and it does the same thing.  `host_alloc` becomes a bump allocator into a
fixed 1 GiB arena and `host_free` returns without doing anything, which is
GOALS.md's charter bullet "A GARBAGE COLLECTOR IS ASSUMED FROM HERE ON" built.  The
editor asks for exactly the memory it asked for before and is handed as much of it;
what changes is where the bytes come from and that giving them back costs nothing.
79,592 -> 79,660 lines, 68 more, and every one of them below the first `#include`.

THE STRONGEST THING THIS PHASE SAYS IS A `cmp` AND NOT A RECORDING.  It is host-only
from end to end, so `make editor.c` -- the 77,681 lines above the boundary, the part of
this file the project is FOR -- is BYTE-IDENTICAL in and out, 2,064,232 bytes either
side.  That subsumes every screen case, every memline case, every Ex command, every
command line and every pty scenario at once for the core, because the program that
would be transpiled is literally the same text.  The recording below is what says the
HOST still answers the same way.

THE SIZE IS MEASURED AND NOT CHOSEN, AND IT COULD NOT HAVE BEEN MEASURED ONE BOUNDARY
EARLIER.  The input built with a counter on host_alloc, totalling every request as the
allocator rounds it and dumped from host_exit(), over the whole of tools/zrecord.sh:
268 sessions in 122 records, and the largest asks for 200,458,672 bytes -- two
recordings, the same number.  It is ONE case, phase 123's mem_deep_jumps, a
25,000-line buffer churned in the middle; the next three memline cases are near 52
million and the heaviest of the 102 SCREEN cases is 1,722,512, which is 115 times less.
So an arena sized from the 102 alone would have been sized from a corpus that provably
cannot reach the text layer, which is the defect phase 123 exists to have ended.  This
phase found that out the hard way and the record says so: 64 MiB was written first and
the recording refused it -- `THE RECORDING MOVED, in 1 of 122 records:
memline/mem_deep_jumps`, the case dying with `host arena exhausted: 67108864 bytes,
67058640 used, request 60263`, with 0 of 102 screen cases and 0 of the four sweeps
moving.  The check measures the high-water again on this phase's OWN output, over BOTH
corpora, and refuses an arena less than four times it -- and refuses as well if the
heaviest memline case is not heavier than the heaviest screen case, which is that
lesson written down as an assertion rather than as a paragraph.

THE FIRST ARGUMENT FOR THE SIZE WAS WRONG AND THE MEASUREMENT THAT KILLED IT IS WORTH
MORE THAN THE NUMBER.  This phase justified its 64 MiB ceiling by saying the abort
path costs the arena times the harness's concurrency -- "64 MiB across 64 threads is 4
GiB on a 62 GiB machine".  That is false except for a runaway session: an ordinary
session's resident memory is its TRAFFIC, which the arena does not change, and an
untouched arena page costs nothing.  Measured, the same source at 64 MiB and at 1 GiB
gives a BYTE-IDENTICAL image, 772,872 either way, because `.bss` is NOBITS.  So the
size buys one thing, how far a runaway goes before it dies loudly, and costs one thing,
address space.  1,073,741,824 is 5.36 times the measured high-water, and the four-times
rule in the check stands with no weakening to justify.

AND ONE MEASUREMENT SAYS WHY NO NUMBER COULD BE SAFE.  With nothing freed, an arena
must hold a session's whole allocation TRAFFIC and not its live data, and this editor's
traffic is QUADRATIC in the length of a single line being typed: `+normal 200000ax`
asks for 20,013,114,624 bytes across 400,475 calls, and `+normal 500000ax` for
44,075,360,179.  Phase 118's own by-hand probe was `+normal 200000ax`, so this is not a
hypothetical shape -- a later phase that writes a probe like it will hit the wall.
Buffers, by contrast, are LINEAR and cheap: 100,000 lines cost 12,862,224 bytes and
300,000 lines 35,157,264, about 112 bytes a line.  It is churn and not size that fills
an arena.

THE REAL COST IS NOT THE ARENA, IT IS THE RESIDENT MEMORY, and it is a property of the
phase rather than of the size.  A bump allocator makes a session's peak RSS equal to
its traffic.  Measured with getrusage(RUSAGE_CHILDREN) over a whole tools/zmemline.py
run: the worst child peaks at 13.6 MiB on the input and 191.8 MiB here, FOURTEEN TIMES
more, at either arena size.  The 16 memline cases run concurrently, so a single
tools/zrecord.sh run holds about 475 MiB rather than about 30.  A whole
`make whim-verify` with this phase in it -- 42 units at once -- peaks at 13.7 GiB of a
62 GiB machine against 13.1 GiB measured the same way on the boundary before it, which
is 591 MiB and 4.4% more, with 41 GiB still available.  Comfortable, and recorded here
because phases 125 to 128 each change how much the memline allocates: the number to
watch is not the arena, which is free, but this one.

THE ARENA COSTS NOTHING TO STORE AND THE IMAGE SHRINKS.  `.bss` is NOBITS, so the
section header records a size and the file holds no bytes of it: `.bss` goes 24,600 ->
1,073,766,424 and the binary goes 781,064 -> 772,872, eight kilobytes SMALLER, because
musl's allocator is no longer linked in.  The `.bss` growth is the arena LESS 960
bytes, and the 960 is musl's own allocator state -- `__malloc_context` and five smaller
objects, 964 bytes measured on an unstripped pair -- leaving with it.  EXEC, no INTERP,
no dynamic section and no relocation are all still true of a gigabyte object.

IT IS THE FIRST PART II PHASE SINCE 28 TO FREE A SYMBOL, AND IT FREES THREE.  `nm -u` goes
17 -> 14 on the boundary's own flags (18 -> 15 as tools/symbols.sh counts it, which
compiles plain -O0 and so adds __stack_chk_fail), and the set moves by EXACTLY `free`,
`malloc` and `realloc`, a comm empty in the other direction.  Phase 118 freed none and
said so as an equality -- a symbol leaves when its last CALLER leaves the file -- and
this is that rule read the other way: the callers did not move, the calls went.

TWO OF THE FOUR REWRITES ARE NOT IN THE ALLOCATOR, and without them the phase would be
WRONG rather than incomplete.  The formatter's private island -- the four functions
phase 110 moved below the includes because they need `va_list` -- still called libc's
`free` and libc's `realloc` directly, on pointers that came from alloc_clear(), which is
to say from host_alloc.  Phase 118 named one of them in its own program ("and
format_overflow_error() below the boundary") and phase 117 named the other ("`realloc`
call is the host's and is not this phase's").  Both were right while host_alloc WAS
malloc; from this phase a free() or a realloc() of an arena pointer is undefined.  The
realloc moves by phase 117's own pattern -- allocate, copy, free -- with phase 117's three
traps read off this site.

NEITHER IS REACHABLE BY ANY RECORDING AND ONE IS NOT REACHABLE AT ALL, so the check
owes a probe and runs one.  adjust_types() grows *ap_types only for a format string
carrying a positional spec, and NOT ONE STRING LITERAL IN THIS FILE HAS ONE: the same
driver built into the input and into the output runs six ascending positional formats
through it, enters the grow arm 17 times in each, and the two binaries print the same
bytes.  format_overflow_error() cannot be probed because it cannot RUN -- its guard is
`overflow_err`, which is `tvs != nullptr`, and vim_vsnprintf_typval has one caller in
this file passing nullptr.  That is phase 92's and phase 100's kind, and it is rewritten
for the reason a dead branch is kept correct rather than left to rot.

THE CONTROLS, OVER BOTH CORPORA, AND THE ONE THAT MOVES NOTHING IS REPORTED RATHER THAN
HIDDEN.  host_alloc returning nullptr always moves 122 of 122 records.  THE OFFSET
NEVER ADVANCING moves 102 of 102 screen cases and 16 of 16 memline cases, and it is the
only control here that tests the ALLOCATOR rather than the wrapper: a host_alloc that
returned the arena's base for ever would pass the symbol check, the cut and the size
assertion, and this is what says the bookkeeping is live.  host_free doing NOTHING AT
ALL moves 0 of 102 and 0 of 16, and that is PHASE 118'S OWN `cf` CONTROL re-run on this
phase's input rather than a new claim -- on a corpus phase 118 did not have.  A leak is
invisible to this corpus too, so the byte-identical recording is NOT what says the
freeing changed; `free` leaving nm -u is.  And the guard, which no recording can ever
take: with a 256 KiB arena a bare session aborts with `zero-vim: host arena exhausted:
262144 bytes, 113024 used, request 319968` and exits 1 -- the request that did not fit
being the screen, this editor's single largest allocation -- while the identical session
on the 1 GiB output writes nothing to stderr and exits 0.

<stdlib.h> IS NOW DEAD AND IT STAYS, measured rather than argued: malloc, free and
realloc were its only users, and the output built with the directive DELETED is
BYTE-IDENTICAL, 772,872 bytes either way.  GOALS.md lets a phase remove a directive
and phase 96 is the precedent for declining -- it measured that removing three was free
and wrote "the count stays 18" into its own program.  Eleven stays eleven: a phase that
changes two things cannot say which one a difference came from, and the removal is free
for whoever asks for it.
