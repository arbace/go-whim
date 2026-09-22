# Phase 142 — the version names no build date or time

`init_longVersion()` put `__DATE__ " " __TIME__` into the version line that
`mainerr()` prints above a command-line error: `VIM - Vi IMproved 9.2 (2026
Feb 14, compiled <date> <time>)`. So the core's text depended on when it was
compiled, and two builds of the same source differed unless
`SOURCE_DATE_EPOCH` pinned them. The Go transpilation had no such clock and
wrote in the constant the pinned build produces (finding 13). The version is
now its name and its release date, `VIM - Vi IMproved 9.2 (2026 Feb 14)`, and
the binary depends on the source alone.

**Declared delta: nothing**, and not because nothing changed. The version
line is written to stderr, and phase 85's `stderr-moved` excludes stderr from
every comparison, so the recording cannot see this change. The check
therefore measures it itself. It requires `__DATE__` and `__TIME__` gone. It builds the output at two `SOURCE_DATE_EPOCH`s and requires
the same bytes; the input built the same two ways, the control, differs. It
runs `-T` without its argument and reads the version line from both
binaries: the new one, and the input's with `, compiled Jan  1 1970 00:00:00`.
