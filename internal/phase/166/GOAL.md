# Phase 166 — the system headers nothing needs

The last phase, and the only one that removes an `#include`. The file opens
with the 41 `#include`s slim-vim.c has. Each phase before this one takes away
some of what they were for: the directory walker, the locale, the password
file, dlopen, setjmp, the maths library, the ctype functions (97-98), the stdio
stream (104). A header that supplies nothing the file names -- no function, no
type, no constant -- is a dependency on the host that buys nothing. This phase
drops every such header at once. It is the `includes` step, which phase 82 used
to run.

**The set is computed, not listed.** A header's name says what it is for, not
what this file uses from it. `<sys/types.h>` may be the only thing declaring a
type the code names, and musl's headers include one another, so one that looks
dead can be carrying another. So each `#include` is tried: delete it and
require the compile to stay SILENT under `-Wall -Wextra`. gcc compiles C23,
where a call to an undeclared function is an error and an unknown type is an
error, so "silent" means "nothing this header provided was used". The
candidates are tried one at a time in parallel, then all together. If the set
fails together -- two headers each covering for the other -- they are removed
one by one from the bottom, each kept only while the build stays silent.

**Why last, and all together.** Phases 82, 99 and 104 each used to drop the
headers that had just become unused, and about twenty phases between asserted
the count they were handed: eighteen, twelve, eleven. The dropping is one rule,
so it runs once, here. The phases before it now assert only what does not
depend on it: every directive is an `#include` of a system header, the
directives are one contiguous block, and a phase adds and removes none (phase
147 excepted, which needs `<fcntl.h>` and puts it beside `<termios.h>`).

**Measured:** it drops 31 of the 41 headers where 82, 99 and 104 between them
dropped 29, leaving 10 `#include`s instead of 12. The two more are `<stdlib.h>`
and `<stdint.h>`: when those phases ran, each still carried something, and by
the end everything it declared arrives through the headers that remain. The
file compiles silently without them, and the binary is BYTE-IDENTICAL to the one
the twelve gave (`SOURCE_DATE_EPOCH=0`, the one compile line). Every phase from
82 to 165 was checked against its new input -- the committed boundary with all
41 headers in place -- and gives the committed boundary with all 41 in place,
once the five phases below were fixed.

**What moving it cost the phases before it.** Beyond the directive counts, a
header's `#include` line spells words (`select`, `time`, `ioctl`), and mention
counts saw them. So every mention count skips `#include` lines now
(`edit.MentionCount`): a directive names a header, not an identifier. Two
counts that had counted a kept header's line on purpose drop by one: 103's
`ioctl` and 115's `time`. Phases 106 and 110 found the block by its fixed size
(the typedef on line 13, the first twelve lines lifted), and now use the size
they were handed. Phase 147 moves `<fcntl.h>` beside `<termios.h>` instead of
adding it.
