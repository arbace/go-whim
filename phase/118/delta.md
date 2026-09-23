118 DECLARES NOTHING AT ALL, and it is the SIXTH kind -- the code runs, the instrument
sees it, and it does the same thing.  The core's last three libc CALLS go to the host:
`malloc`, called by lalloc(); `free`, called by vim_free() and update_wincolor(); and
`write`, called by mch_write() -- and, since phase 117 rewrote `realloc` as a malloc, a
copy and a free, by ga_grow_inner() and get_keystroke() as well.  Above the first
`#include` they are 4, 5 and 2 mentions -- each a declaration and its call sites, and
HOW MANY is read off the text rather than written down -- and all three become 0, with
three `static` prototypes at the end of the block phase 108 made of eleven and three
definitions below the boundary that call the same libc functions with the same
arguments.  `host_alloc(usize n)` returns what malloc() returned, nullptr included, so
lalloc()'s clear_sb_text()/do_outofmem_msg() failure path is reached exactly as before;
`host_free(void *p)` calls free(), so it is null-safe for free()'s own reason;
`host_write(const char *s, int len)` is ONE write(2) to fd 1, no loop, the count cast
to int and handed back for mch_write() to ignore, which is what mch_write() did with it.
79,786 -> 79,804 lines, and the core's block of ordinary declarations is TWO lines
where it was five: `getpid` and `kill` remain, and they are one phase's, not two.

THE BINARY MOVES and that is not aimed for: at -O0 a call to a static function in this
file is not the instruction stream of a call to a libc symbol, and three definitions
arrive.  782,760 bytes either side, 206,588 of them differing.  So the evidence is a
RECORDING -- two full tools/zrecord.sh runs, `diff -r` empty across all 106 records --
and here that is STRONG rather than weak, which is worth saying because an empty
declaration usually is not: lalloc() and vim_free() are on the path of essentially
everything the editor does and mch_write() is every byte it draws, so the corpus
hammers all three.  Measured on the instrumented output over the 102 screen cases and
marking every one of them: 53,848 host_alloc calls, 22,417 host_free calls and 1,012
host_write calls.

THE PHASE FREES NO SYMBOL AND SAYS SO AS AN EQUALITY.  `nm -u` is the same set in and
out, a cmp in both directions, and `malloc`, `free` and `write` are all three still in
it.  That is not a disappointment and it is the general rule of this pipeline read
back: a symbol leaves when its last CALLER leaves the file, and inside one translation
unit moving a call from the core into the host moves no caller out.  Phase 111 is the
contrast -- it freed `gettimeofday` because the last caller went with it.  What DOES
move is the thing `nm -u` cannot show: the editor.c cut's warning set, which IS the
core -> host interface, goes from 14 names to 17.  An implicit libc dependency hidden
in a bare declaration becoming an explicit named call is the point of the boundary, and
an interface that grows by exactly three is what that looks like.

THE CONTROLS ARE ONE PER FUNCTION AND TWO OF THEM MOVE NOTHING, which is reported
rather than hidden.  host_alloc returning nullptr always moves 106 of 106; refusing
only the allocations above 200,000 bytes -- in this editor exactly ONE, the screen at
319,968 -- moves 100 of 102 screen cases, the two survivors being ctrl_c_clean and
ctrl_c_changed, which exit before a key is looked up.  host_write writing HALF THE
BYTES moves 102 of 102, and host_write writing every byte and REPORTING half moves 0 of
102 -- which is the phase's own claim about the wrapper measured rather than argued:
mch_write() assigns the count to vim_ignored and writes no more, so the return value is
inert and a wrapper that LOOPED on a short write would be a behaviour change no
recording could see.  host_free doing NOTHING AT ALL moves 0 of 102, because a leak is
invisible to a 106-record corpus; what says host_free is called is the instrument, and
the check says so in those words rather than presenting a silent control as evidence.
host_free(nullptr) called on every draw moves 0 of 102 and writes nothing, which is
`free(nullptr) is defined and does nothing` measured on the wrapper.  The core does not
rely on it -- 0 of the corpus's 22,417 frees are null, every call site guarding -- but
the wrapper inherits it rather than adding a test of its own.
