# Phase 82 — the system headers nothing needs, and every comment

`whim-vim.c` opened with the same 41 `#include`s as `slim-vim.c`, and eighty-one
phases had taken away most of what they were for — the directory walker, the locale,
the password file, `dlopen`, `setjmp`, the maths library, `utime`, `uname`. The
object leaves 80 symbols for musl to supply, and a header that provides none of them
is a dependency on the host that buys nothing.

**The set is computed, not listed.** A header's name says what it is for, not what
this file takes from it, and musl's headers include one another. So each `#include`
is deleted in turn and the compile must stay **silent** under the sweep's flags; gcc
15 compiles C23, where an undeclared function or an unknown type is an error, so
silence means nothing the header provided was used. 28 of 41 can go on their own, in
parallel. Together they do not build — some pairs each carry what the other declares
— so they are removed one at a time, keeping each removal only while the build stays
silent.

**From the bottom, and the first dry run is why.** Walked top down, it dropped
`<string.h>` and `<stdlib.h>`, whose declarations happen to arrive through headers
further down, and kept `<wchar.h>`. The general headers come first, so walking up
from the end drops the specific ones and keeps what everything else leans on.
`<iconv.h>` stays: no iconv function is called, but `iconv_t` is still named.

**The proof is the binary, byte for byte.** A header can define a function-like
macro that shadows a function — musl's `<ctype.h>` does — and losing one would change
code silently if the prototype still came from elsewhere. So the input and output are
both built with `SOURCE_DATE_EPOCH` pinned, from the same file name, and must be
identical. They are, 955,976 bytes, which makes "no delta" a measurement.

Removed, 23: `limits.h` `sys/types.h` `dirent.h` `sys/time.h` `pwd.h` `sys/file.h`
`strings.h` `setjmp.h` `locale.h` `float.h` `math.h` `inttypes.h` `stdbool.h`
`sys/select.h` `wchar.h` `utime.h` `langinfo.h` `sys/sysinfo.h` `sys/wait.h`
`stropts.h` `sys/utsname.h` `dlfcn.h` `sys/resource.h`. Left, 18: `stdio.h` `ctype.h`
`sys/stat.h` `stdlib.h` `unistd.h` `sys/param.h` `time.h` `signal.h` `string.h`
`errno.h` `stdint.h` `wctype.h` `stdarg.h` `stddef.h` `fcntl.h` `iconv.h`
`sys/ioctl.h` `termios.h`.

## Every comment

The same phase strips every comment: the former-file banners, the seven notes, and
the lines earlier whim phases wrote to explain themselves — 314 lines, and the blank
lines around the banners that would otherwise have doubled up. `whim-vim.c` carries
code and nothing else from here, and **no later phase writes a comment into it**
(rule 5); the reasoning lives in the phase programs, this file and the commit
messages. Comments are found by a scanner that knows string and character literals,
because `"pack/*/start/*"` and `"://"` are data. Comments never reach the binary —
nothing uses `__LINE__` — so the byte-for-byte check covers this cut too, and the
paragraphing is checked separately: no run of blank lines, and the counts of blank
lines after `{` and before `}` unchanged.

Measured: 87,107 → **86,614 lines**.

## `arrowcheck.py` retired

The pty check that the arrow keys still move the cursor ran in 46 phases, from 24
on, at about **20 seconds of wall time each** — for most phases more than the
phase's own work. It guarded against one accident: the mouse phase deleting
`nv_cmds[]` rows under a precomputed index, which `tools/nvidxcheck.py` now catches
structurally, in `phasecheck.sh`, in no measurable time. One accident does not buy
a pty session per phase for ever, so the call went from every phase program and the
tool was deleted. Phase 32's own check keeps its completion half.
